// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package common

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sync"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// defaultDeviceSecret is the fallback secret used when [client] device_secret is not configured; for local development only.
const defaultDeviceSecret = `goteams-default-device-secret`

// deviceSecretWarnOnce ensures the fallback-secret warning is printed only once to avoid log flooding.
var deviceSecretWarnOnce sync.Once

// WSRequestLogMiddleware logs WebSocket handshake request details (redacted) to help diagnose rejected handshakes.
// Placed at the front of the auth chain so the request is logged even if a later middleware aborts.
func WSRequestLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		hdr := c.Request.Header.Clone()
		for _, k := range []string{"Authorization", "Cookie", "X-GoTeams-Device-Credential"} {
			if _, ok := hdr[k]; ok {
				hdr.Set(k, "***")
			}
		}
		logs.Info("[WS] 收到握手请求 method=%s path=%s remote=%s headers=%v query=%s",
			c.Request.Method, c.Request.URL.Path, c.ClientIP(), hdr, c.Request.URL.RawQuery)
		c.Next()
	}
}

// ClientVersionMiddleware validates the client version header.
func ClientVersionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientVersion := c.GetHeader(`X-GoTeams-Client-Version`)
		if clientVersion == `` {
			clientVersion = c.Query(`client_version`)
		}
		c.Set(`clientVersion`, clientVersion)
		protocolVersion := cast.ToInt(c.GetHeader(`X-GoTeams-Protocol-Version`))
		if protocolVersion == 0 {
			protocolVersion = 1
		}
		c.Set(`protocolVersion`, protocolVersion)
		c.Next()
	}
}

// DeviceCredentialMiddleware validates the device credential.
func DeviceCredentialMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := lib_i18n.GetLang(c)
		deviceUUID := c.GetHeader(`X-GoTeams-Device-ID`)
		deviceCredential := c.GetHeader(`X-GoTeams-Device-Credential`)
		credentialVersion := cast.ToInt(c.GetHeader(`X-GoTeams-Device-Credential-Version`))

		if deviceUUID == `` || deviceCredential == `` {
			c.JSON(http.StatusUnauthorized, define.Response{
				Code:    http.StatusUnauthorized,
				Message: lib_i18n.Show(lang, define.MsgClientDeviceCredentialInvalid),
			})
			c.Abort()
			return
		}

		// Get the tenant from the JWT.
		adminId := c.GetInt(`adminId`)
		if adminId == 0 {
			c.JSON(http.StatusUnauthorized, define.Response{
				Code:    http.StatusUnauthorized,
				Message: lib_i18n.Show(lang, define.MsgClientDeviceCredentialInvalid),
			})
			c.Abort()
			return
		}

		// Query the device record.
		m := msql.Model(define.TableClientDevice, define.Postgres)
		row, err := m.Where2([][]string{
			{`admin_id`, cast.ToString(adminId)},
			{`device_uuid`, deviceUUID},
		}).Find()
		if err != nil || len(row) == 0 {
			c.JSON(http.StatusUnauthorized, define.Response{
				Code:    http.StatusUnauthorized,
				Message: lib_i18n.Show(lang, define.MsgClientDeviceCredentialInvalid),
			})
			c.Abort()
			return
		}

		// Validate the device status.
		if cast.ToInt(row[`credential_status`]) == 0 {
			c.JSON(http.StatusForbidden, define.Response{
				Code:    http.StatusForbidden,
				Message: lib_i18n.Show(lang, define.MsgClientDeviceRevoked),
			})
			c.Abort()
			return
		}

		// Validate the credential fingerprint.
		expectedFingerprint := ComputeDeviceFingerprint(DeviceSecret(), deviceCredential)
		if !hmac.Equal([]byte(row[`credential_fingerprint`]), []byte(expectedFingerprint)) {
			c.JSON(http.StatusUnauthorized, define.Response{
				Code:    http.StatusUnauthorized,
				Message: lib_i18n.Show(lang, define.MsgClientDeviceCredentialInvalid),
			})
			c.Abort()
			return
		}

		// Validate tenant consistency.
		if cast.ToInt(row[`admin_id`]) != adminId {
			c.JSON(http.StatusForbidden, define.Response{
				Code:    http.StatusForbidden,
				Message: lib_i18n.Show(lang, define.MsgClientTenantMismatch),
			})
			c.Abort()
			return
		}

		c.Set(`deviceId`, cast.ToInt(row[`id`]))
		c.Set(`deviceUUID`, deviceUUID)
		c.Set(`deviceCredentialVersion`, credentialVersion)
		c.Next()
	}
}

// DeviceSecret returns the device credential signing secret.
// When [client] device_secret is not configured, it falls back to the built-in default secret:
// all deployments then share one secret, making fingerprint verification meaningless, so an error log is emitted on first use.
func DeviceSecret() string {
	if secret := define.Config.Client[`device_secret`]; secret != `` {
		return secret
	}
	deviceSecretWarnOnce.Do(func() {
		logs.Error(`[client] device_secret is not configured, falling back to the built-in default secret; device credential verification provides no real protection`)
	})
	return defaultDeviceSecret
}

// ComputeDeviceFingerprint computes the device credential fingerprint (HMAC-SHA256 + hex).
// Shared by the client auth middleware and the device registration business logic to avoid algorithm divergence.
func ComputeDeviceFingerprint(secret, credential string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(credential))
	return hex.EncodeToString(mac.Sum(nil))
}

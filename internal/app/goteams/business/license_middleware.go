// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
	"net/http"

	"github.com/gin-gonic/gin"
)

// commercialLicensePublicWhitelist contains exact endpoints needed before the first license.
var commercialLicensePublicWhitelist = map[string]map[string]struct{}{
	http.MethodGet: {
		`/api/ping`:                  {},
		`/api/site/branding`:         {},
		`/api/system/license/status`: {},
	},
	http.MethodPost: {
		`/api/system/license/online-verification-code`: {},
		`/api/system/license/activate-online`:          {},
		`/api/system/license/activate-offline`:         {},
	},
}

// commercialLicenseManagementWhitelist contains the minimum authenticated shell required to
// enter the enterprise version tab and let the tenant owner renew an installed blocked license.
var commercialLicenseManagementWhitelist = map[string]map[string]struct{}{
	http.MethodGet: {
		`/api/user/profile`:          {},
		`/api/user/permissions`:      {},
		`/api/enterprise/plan/info`:  {},
		`/api/enterprise/plan/usage`: {},
	},
	http.MethodPost: {
		`/api/auth/login`:                        {},
		`/api/user/logout`:                       {},
		`/api/enterprise/license/install`:        {},
		`/api/enterprise/license/refresh-online`: {},
	},
}

// commercialDisabledRoutes are refused unconditionally in the commercial build (self-service
// registration is closed; tenants are provisioned by bootstrap and operations).
var commercialDisabledRoutes = map[string]struct{}{
	http.MethodPost + ` /api/auth/register`:           {},
	http.MethodPost + ` /api/auth/send-register-code`: {},
}

// CommercialLicenseMiddleware is the global authorization gate. It runs right after CORS and
// in front of every route middleware. Valid licenses pass normally; installed blocked licenses
// expose only a minimal authenticated renewal shell while every other business API stays denied.
func CommercialLicenseMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestKey := c.Request.Method + ` ` + c.Request.URL.Path
		if _, disabled := commercialDisabledRoutes[requestKey]; disabled {
			c.JSON(http.StatusForbidden, define.Response{
				Code:    http.StatusForbidden,
				Message: lib_i18n.Show(lib_i18n.GetLang(c), define.MsgFeatureDisabled),
			})
			c.Abort()
			return
		}
		snapshot := GetLicenseSnapshot()
		if define.LicenseStateCanLogin(snapshot.State) {
			c.Next()
			return
		}
		if _, allowed := commercialLicensePublicWhitelist[c.Request.Method][c.Request.URL.Path]; allowed {
			c.Next()
			return
		}
		state := snapshot.State
		if state == `` {
			state = define.LicenseStateNotActivated
		}
		managementOnly := define.LicenseStateAllowsManagementLogin(state)
		if managementOnly {
			if _, allowed := commercialLicenseManagementWhitelist[c.Request.Method][c.Request.URL.Path]; allowed {
				c.Next()
				return
			}
		}
		redirect := `/license`
		if managementOnly {
			redirect = `/members?tab=version`
		}
		c.JSON(http.StatusForbidden, define.Response{
			Code:    http.StatusForbidden,
			Message: lib_i18n.Show(lib_i18n.GetLang(c), define.LicenseMsgForState(state)),
			Data: gin.H{
				`reason`:          define.LicenseReasonForState(state),
				`redirect`:        redirect,
				`management_only`: managementOnly,
			},
		})
		c.Abort()
	}
}

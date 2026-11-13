// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/define"
)

// templateExportTokenPayloadPrefix namespaces the signed payload, so a token cannot be replayed
// against any other feature that happens to sign with the same secret.
const templateExportTokenPayloadPrefix = `workspace-template-export|v1|`

// GetTemplateExportTokenSecret returns the shared secret that signs template-export tokens. An
// empty value means the deployment cannot export at all (the endpoint answers 403).
func GetTemplateExportTokenSecret() string {
	return strings.TrimSpace(define.Config.Template[`export_secret`])
}

// VerifyTemplateExportToken accepts `<expiry>.<hex hmac>` signed with the configured secret and
// rejects a token whose expiry has passed. Only the offline export script signs (its CLI entry is
// `scripts/template-export.bat`, implemented in `scripts/ps1/template-export.ps1`); the deployment
// only verifies, so the secret never has to leave the config on either side.
func VerifyTemplateExportToken(token string, now int) bool {
	secret := GetTemplateExportTokenSecret()
	expiry, signature, found := strings.Cut(token, `.`)
	if secret == `` || !found {
		return false
	}
	exp, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil || exp < int64(now) {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(templateExportTokenPayload(exp)))
	return hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(signature))
}

// templateExportTokenPayload is the signed string. The Node script signs the same prefix plus the
// same decimal expiry, so a change here must be mirrored there and bumps the version in the prefix.
func templateExportTokenPayload(exp int64) string {
	return templateExportTokenPayloadPrefix + strconv.FormatInt(exp, 10)
}

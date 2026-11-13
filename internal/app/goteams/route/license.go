// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegLicenseRoutes registers commercial license routes. Public endpoints are also kept in the
// global license gate whitelist (business/license_middleware.go); owner endpoints require only
// a session because they must stay reachable right after expiry to install a renewed key.
func RegLicenseRoutes(r lib_web.Route) {
	r[http.MethodGet][`/api/system/license/status`] = controller.LicenseStatus
	r[http.MethodPost][`/api/system/license/online-verification-code`] = controller.LicenseSendVerifyCode
	r[http.MethodPost][`/api/system/license/activate-online`] = controller.LicenseActivateOnline
	r[http.MethodPost][`/api/system/license/activate-offline`] = controller.LicenseActivateOffline
	r[http.MethodPost][`/api/enterprise/license/install`] = wrapAction(withAuth(), controller.LicenseInstall)
	r[http.MethodPost][`/api/enterprise/license/refresh-online`] = wrapAction(withAuth(), controller.LicenseRefreshOnline)
}

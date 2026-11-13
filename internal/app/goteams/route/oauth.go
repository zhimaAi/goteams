// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegOauthRoutes registers third-party scan-code login routes (DingTalk / Feishu).
func RegOauthRoutes(r lib_web.Route) {
	// Public: scan-code entry points used by the sign-in page.
	r[http.MethodGet][`/api/auth/oauth/providers`] = controller.GetOauthProviders
	r[http.MethodGet][`/api/auth/oauth/authorize`] = controller.GetOauthAuthorize
	r[http.MethodPost][`/api/auth/oauth/exchange`] = controller.ExchangeOauthTicket
	r[http.MethodPost][`/api/auth/oauth/bind`] = controller.BindOauthAccount
	r[http.MethodPost][`/api/auth/oauth/register`] = controller.RegisterOauthAccount

	// Third-party callbacks. The path must stay identical to the redirect URI registered
	// in the provider console, so it is built from define.OauthCallbackPath.
	r[http.MethodGet][define.OauthCallbackPath(define.OauthProviderDingtalk)] = controller.DingtalkCallback
	r[http.MethodGet][define.OauthCallbackPath(define.OauthProviderFeishu)] = controller.FeishuCallback

	// Login required: binding management on the personal settings page.
	auth := withAuth()
	r[http.MethodGet][`/api/user/oauth/authorize`] = wrapAction(auth, controller.GetUserOauthAuthorize)
	r[http.MethodGet][`/api/user/oauth/bindings`] = wrapAction(auth, controller.GetUserOauthBindings)
	r[http.MethodDelete][`/api/user/oauth/bindings/:provider`] = wrapAction(auth, controller.DeleteUserOauthBinding)
}

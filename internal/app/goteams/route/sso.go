// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegSsoRoutes 注册企业 SSO 路由：配置接口走权限，登录链路的入口与回调全部公开。
func RegSsoRoutes(r lib_web.Route) {
	auth := withPerm("sso:setting")

	r[http.MethodGet][`/api/enterprise/sso`] = wrapAction(auth, controller.GetSsoConfig)
	r[http.MethodPut][`/api/enterprise/sso`] = wrapAction(auth, controller.SaveSsoConfig)
	r[http.MethodPost][`/api/enterprise/sso/test`] = wrapAction(auth, controller.TestSsoConnection)
	r[http.MethodPut][`/api/enterprise/sso/sync`] = wrapAction(auth, controller.SaveSsoSyncConfig)
	r[http.MethodPost][`/api/enterprise/sso/sync/run`] = wrapAction(auth, controller.RunSsoSync)

	// 公开：入口探测与授权跳转。全部落在 /api/ 下，nginx 现有规则即可，无需部署变更。
	r[http.MethodGet][`/api/sso/entry`] = controller.SsoEntry
	r[http.MethodGet][`/api/sso/authorize`] = controller.SsoAuthorize
	// 回调路径必须与登记进 IdP 后台的地址逐字符一致：{key} 是该租户恒定的回调标识。
	r[http.MethodGet][`/api/sso/callback/:provider/:key`] = controller.SsoCallback
	r[http.MethodGet][`/api/sso/callback/:provider/:key/bind`] = controller.SsoBindCallback
}

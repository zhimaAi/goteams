package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegPersonalizationRoutes 注册设置权限接口与只读品牌入口。
func RegPersonalizationRoutes(r lib_web.Route) {
	auth := withPerm("personalization:setting")
	r[http.MethodGet]["/api/enterprise/personalization"] = wrapAction(auth, controller.EnterpriseCapabilityDisabled)
	r[http.MethodPut]["/api/enterprise/personalization"] = wrapAction(auth, controller.EnterpriseCapabilityDisabled)
	r[http.MethodPost]["/api/enterprise/personalization/logo"] = wrapAction(auth, controller.EnterpriseCapabilityDisabled)
	r[http.MethodGet]["/api/site/branding"] = controller.GetPublicBranding
	r[http.MethodGet]["/api/user/branding"] = wrapAction(withAuth(), controller.GetTenantBranding)
}

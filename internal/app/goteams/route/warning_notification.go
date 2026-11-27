package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegWarningRoutes 注册工作空间预警通知接口。
func RegWarningRoutes(r lib_web.Route) {
	p := "/api/workspaces/:wsId/warning-notifications"
	read := withAnyPerm("warning_notification:view", "warning_notification:manage")
	write := withPerm("warning_notification:manage")
	r[http.MethodGet][p] = wrapAction(read, controller.ListWarningRules)
	r[http.MethodGet][p+"/metadata"] = wrapAction(read, controller.GetWarningMetadata)
	r[http.MethodPost][p] = wrapAction(write, controller.SaveWarningRule)
	r[http.MethodPut][p+"/:ruleId"] = wrapAction(write, controller.SaveWarningRule)
	r[http.MethodPatch][p+"/:ruleId/enabled"] = wrapAction(write, controller.SetWarningRuleEnabled)
	r[http.MethodDelete][p+"/:ruleId"] = wrapAction(write, controller.DeleteWarningRule)
}

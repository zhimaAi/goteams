package route

import (
	"net/http"

	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
)

// RegAutomationRoutes registers project automation rule and execution routes.
func RegAutomationRoutes(r lib_web.Route) {
	prefix := `/api/workspaces/:wsId/automation`
	rules := prefix + `/rules`
	executions := prefix + `/executions`

	r[http.MethodGet][rules] = wrapAction(withAnyPerm("automation:view", "automation:manage"), controller.ListAutomationRules)
	r[http.MethodPost][rules] = wrapAction(withPerm("automation:manage"), controller.CreateAutomationRule)
	r[http.MethodGet][rules+`/:ruleId`] = wrapAction(withAnyPerm("automation:view", "automation:manage"), controller.GetAutomationRule)
	r[http.MethodPut][rules+`/:ruleId`] = wrapAction(withPerm("automation:manage"), controller.UpdateAutomationRule)
	r[http.MethodPost][rules+`/:ruleId/copy`] = wrapAction(withPerm("automation:manage"), controller.CopyAutomationRule)
	r[http.MethodGet][prefix+`/copy-targets`] = wrapAction(withPerm("automation:manage"), controller.ListAutomationCopyTargets)
	r[http.MethodPatch][rules+`/:ruleId/enabled`] = wrapAction(withPerm("automation:manage"), controller.SetAutomationRuleEnabled)
	r[http.MethodDelete][rules+`/:ruleId`] = wrapAction(withPerm("automation:manage"), controller.DeleteAutomationRule)
	r[http.MethodGet][executions] = wrapAction(withAnyPerm("automation:view", "automation:manage"), controller.ListAutomationExecutions)
	r[http.MethodGet][executions+`/:executionId`] = wrapAction(withAnyPerm("automation:view", "automation:manage"), controller.GetAutomationExecution)
	r[http.MethodGet][prefix+`/metadata`] = wrapAction(withAnyPerm("automation:view", "automation:manage"), controller.GetAutomationMetadata)
	r[http.MethodGet][prefix+`/user-mappings`] = wrapAction(withPerm("automation:manage"), controller.ListAutomationUserMappings)
	r[http.MethodPut][prefix+`/user-mappings`] = wrapAction(withPerm("automation:manage"), controller.SaveAutomationUserMappings)
}

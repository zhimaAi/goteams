package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegSatisfactionRoutes 只注册本系统配置、只读结果及客户提交，无外部邀评接口。
func RegSatisfactionRoutes(r lib_web.Route) {
	prefix := "/api/workspaces/:wsId/issue-satisfaction"
	r[http.MethodGet][prefix+"/config"] = wrapAction(withAnyPerm("satisfaction:view", "satisfaction:manage"), controller.GetSatisfactionConfig)
	r[http.MethodPut][prefix+"/config"] = wrapAction(withPerm("satisfaction:manage"), controller.SaveSatisfactionConfig)
	r[http.MethodGet]["/api/workspaces/:wsId/requirements/:reqId/satisfaction"] = wrapAction(withPerm("requirement:view"), controller.GetSatisfaction)
	r[http.MethodPost]["/api/workspaces/:wsId/issue-statistics/satisfaction"] = wrapAction(withAllPerm("issue_statistics:view", "requirement:view"), controller.QuerySatisfactionStatistics)
	public := "/api/public/issue-invites/:inviteKey"
	r[http.MethodGet][public+"/satisfactions"] = controller.ListPublicSatisfactionStates
	r[http.MethodGet][public+"/requirements/:requirementId/satisfaction"] = controller.GetSatisfaction
	r[http.MethodPost][public+"/requirements/:requirementId/satisfaction"] = controller.SubmitSatisfaction
}

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegSLARoutes 注册工单工作空间的 SLA 配置接口。
func RegSLARoutes(r lib_web.Route) {
	prefix := "/api/workspaces/:wsId/sla"
	r[http.MethodGet][prefix+"/metadata"] = wrapAction(withAnyPerm("sla:view", "sla:manage"), controller.GetSLAMetadata)
	r[http.MethodPost][prefix+"/action-executions/:actionId/retry"] = wrapAction(withPerm("sla:manage"), controller.RetrySLAAction)
	r[http.MethodGet][prefix+"/work-items"] = wrapAction(withPerm("requirement:view"), controller.GetSLASummaries)
	r[http.MethodGet]["/api/workspaces/:wsId/requirements/:reqId/sla"] = wrapAction(withPerm("requirement:view"), controller.GetRequirementSLA)
	r[http.MethodGet][prefix+"/settings"] = wrapAction(withAnyPerm("sla:view", "sla:manage"), controller.GetSLASettings)
	r[http.MethodPut][prefix+"/settings"] = wrapAction(withPerm("sla:manage"), controller.SaveSLASettings)
	r[http.MethodGet][prefix+"/stages"] = wrapAction(withAnyPerm("sla:view", "sla:manage"), controller.ListSLAStages)
	r[http.MethodPost][prefix+"/stages"] = wrapAction(withPerm("sla:manage"), controller.SaveSLAStage)
	r[http.MethodPut][prefix+"/stages/:stageId"] = wrapAction(withPerm("sla:manage"), controller.SaveSLAStage)
	r[http.MethodDelete][prefix+"/stages/:stageId"] = wrapAction(withPerm("sla:manage"), controller.ArchiveSLADefinition)
	r[http.MethodGet][prefix+"/policies"] = wrapAction(withAnyPerm("sla:view", "sla:manage"), controller.ListSLAPolicies)
	r[http.MethodGet][prefix+"/policies/:policyId"] = wrapAction(withAnyPerm("sla:view", "sla:manage"), controller.GetSLAPolicy)
	r[http.MethodPost][prefix+"/policies"] = wrapAction(withPerm("sla:manage"), controller.SaveSLAPolicy)
	r[http.MethodPut][prefix+"/policies/:policyId"] = wrapAction(withPerm("sla:manage"), controller.SaveSLAPolicy)
	r[http.MethodDelete][prefix+"/policies/:policyId"] = wrapAction(withPerm("sla:manage"), controller.ArchiveSLADefinition)
	r[http.MethodPut][prefix+"/policy-order"] = wrapAction(withPerm("sla:manage"), controller.SaveSLAPolicyOrder)
}

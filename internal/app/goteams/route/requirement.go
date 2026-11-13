// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegRequirementRoutes registers requirement routes (login + permission required).
func RegRequirementRoutes(r lib_web.Route) {
	prefix := `/api/workspaces/:wsId/requirements`
	categoryPrefix := `/api/workspaces/:wsId/requirement-categories`
	excelPrefix := `/api/workspaces/:wsId/work-items`
	actionPrefix := prefix + `/:reqId`

	// Requirement categories are visible with requirement:view; category structure writes require requirement:manage_category.
	r[http.MethodGet][categoryPrefix] = wrapAction(withPerm("requirement:view"), controller.ListRequirementCategories)
	r[http.MethodPost][categoryPrefix] = wrapAction(withPerm("requirement:manage_category"), controller.CreateRequirementCategory)
	r[http.MethodPut][categoryPrefix+`/:categoryId`] = wrapAction(withPerm("requirement:manage_category"), controller.UpdateRequirementCategory)
	r[http.MethodDelete][categoryPrefix+`/:categoryId`] = wrapAction(withPerm("requirement:manage_category"), controller.DeleteRequirementCategory)
	r[http.MethodPut][categoryPrefix+`/:categoryId/move`] = wrapAction(withPerm("requirement:manage_category"), controller.MoveRequirementCategory)

	// Work-item rich-text uploads support both requirement and defect editors.
	r[http.MethodPost][prefix+`/image`] = wrapAction(withAnyPerm("requirement:edit", "defect:edit"), controller.UploadImage)
	r[http.MethodPost][prefix+`/file`] = wrapAction(withAnyPerm("requirement:edit", "defect:edit"), controller.UploadEditorFile)
	r[http.MethodPost][prefix+`/video`] = wrapAction(withAnyPerm("requirement:edit", "defect:edit"), controller.UploadEditorVideo)

	r[http.MethodGet][prefix] = wrapAction(withAnyPerm("requirement:view", "defect:view"), controller.ListRequirements)
	r[http.MethodGet][`/api/workspaces/:wsId/gantt/requirements`] = wrapAction(withPerm("gantt:view"), controller.ListGanttRequirements)
	r[http.MethodPost][prefix] = wrapAction(withAnyPerm("requirement:create", "defect:create"), controller.CreateRequirement)
	r[http.MethodPut][prefix+`/batch`] = wrapAction(withAnyPerm("requirement:edit", "defect:edit"), controller.BatchUpdateRequirements)
	r[http.MethodDelete][prefix+`/batch`] = wrapAction(withAnyPerm("requirement:delete", "defect:delete"), controller.BatchDeleteRequirements)
	r[http.MethodGet][prefix+`/:reqId`] = wrapAction(withAnyPerm("requirement:view", "defect:view"), controller.GetRequirement)
	r[http.MethodPut][prefix+`/:reqId`] = wrapAction(withAnyPerm("requirement:edit", "defect:edit"), controller.UpdateRequirement)
	r[http.MethodPut][actionPrefix+`/category`] = wrapAction(withPerm("requirement:edit"), controller.AssignRequirementCategory)
	r[http.MethodPost][prefix+`/:reqId/status-transitions`] = wrapAction(withAnyPerm("requirement:edit", "defect:edit"), controller.TransitionWorkItemStatus)
	r[http.MethodGet][`/api/my-work`] = wrapAction(withAuth(), controller.ListMyWork)
	r[http.MethodGet][`/api/my-work/recent-visits`] = wrapAction(withAuth(), controller.ListRecentVisits)
	// Visit tracking is the only writer of the recent-visit list; view permission is enough to record one.
	r[http.MethodPost][prefix+`/:reqId/visit`] = wrapAction(withAnyPerm("requirement:view", "defect:view"), controller.RecordRequirementVisit)
	r[http.MethodDelete][prefix+`/:reqId`] = wrapAction(withAnyPerm("requirement:delete", "defect:delete"), controller.DeleteRequirement)
	r[http.MethodGet][prefix+`/:reqId/histories`] = wrapAction(withAnyPerm("requirement:view", "defect:view"), controller.ListRequirementHistories)
	// Agent tab of the requirement/defect detail drawer: client execution records pushed
	// by TeamsBoard. Visibility follows the work-item view permission, not pipeline:view.
	r[http.MethodGet][prefix+`/:reqId/agent-tasks`] = wrapAction(withWorkspaceMemberAndAnyPerm("requirement:view", "defect:view"), controller.ListRequirementAgentTasks)
	r[http.MethodGet][prefix+`/:reqId/agent-tasks/:taskId`] = wrapAction(withWorkspaceMemberAndAnyPerm("requirement:view", "defect:view"), controller.GetRequirementAgentTaskDetail)
	r[http.MethodGet][prefix+`/:reqId/aging`] = wrapAction(withAnyPerm("requirement:view", "defect:view"), controller.GetRequirementAging)
	r[http.MethodGet][prefix+`/:reqId/histories/:historyId`] = wrapAction(withAnyPerm("requirement:view", "defect:view"), controller.GetRequirementHistory)
	// Restoring a description rewrites the work item, so it needs the edit permission of the item type.
	r[http.MethodPost][prefix+`/:reqId/histories/:historyId/restore`] = wrapAction(withAnyPerm("requirement:edit", "defect:edit"), controller.RestoreRequirementHistory)
	r[http.MethodGet][prefix+`/:reqId/work-logs`] = wrapAction(withPerm("requirement:view"), controller.ListRequirementWorkLogs)
	r[http.MethodPost][prefix+`/:reqId/work-logs`] = wrapAction(withPerm("requirement:edit"), controller.CreateRequirementWorkLog)
	r[http.MethodPut][prefix+`/:reqId/work-logs/:logId`] = wrapAction(withPerm("requirement:edit"), controller.UpdateRequirementWorkLog)
	r[http.MethodDelete][prefix+`/:reqId/work-logs/:logId`] = wrapAction(withPerm("requirement:edit"), controller.DeleteRequirementWorkLog)
	// Action controllers validate source and target type-specific permissions because target projects are selected at runtime.
	r[http.MethodGet][`/api/work-item-action-targets`] = wrapAction(withActionTargetPerm(), controller.ListWorkItemActionTargets)
	r[http.MethodPost][actionPrefix+`/move`] = wrapAction(withAnyPerm("requirement:edit", "defect:edit"), controller.MoveWorkItem)
	r[http.MethodPost][actionPrefix+`/convert`] = wrapAction(withAnyPerm("requirement:edit", "defect:edit"), controller.ConvertWorkItem)
	r[http.MethodGet][actionPrefix+`/merge-targets`] = wrapAction(withPerm("defect:delete"), controller.ListWorkItemMergeTargets)
	r[http.MethodPost][actionPrefix+`/merge`] = wrapAction(withPerm("defect:delete"), controller.MergeWorkItem)
	r[http.MethodGet][excelPrefix+`/import-template`] = wrapAction(withAnyPerm("requirement:import", "defect:import"), controller.DownloadWorkItemImportTemplate)
	r[http.MethodGet][excelPrefix+`/export`] = wrapAction(withAnyPerm("requirement:export", "defect:export"), controller.ExportWorkItems)
	r[http.MethodPost][excelPrefix+`/import-preview`] = wrapAction(withAnyPerm("requirement:import", "defect:import"), controller.PreviewWorkItemImport)
	r[http.MethodPost][excelPrefix+`/import`] = wrapAction(withAnyPerm("requirement:import", "defect:import"), controller.ImportWorkItems)
}

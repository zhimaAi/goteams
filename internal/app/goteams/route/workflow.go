// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// planningStatusPerm maps the planning status config permission.
var planningStatusPerm = map[string]string{define.TypeKeyPlanning: `planning:edit`}

// RegWorkflowRoutes registers workflow status/priority CRUD routes.
func RegWorkflowRoutes(r lib_web.Route) {
	statusPrefix := `/api/workspaces/:wsId/statuses`
	prioPrefix := `/api/workspaces/:wsId/priorities`
	metadataPrefix := `/api/workspaces/:wsId/metadata`

	// Aggregate read of project metadata (requirements + defects).
	r[http.MethodGet][metadataPrefix] = wrapAction(withWorkspaceMember(), controller.GetWorkspaceMetadata)

	// Status CRUD.
	r[http.MethodGet][statusPrefix] = wrapAction(withWorkspaceMember(), controller.ListStatuses)
	r[http.MethodPost][statusPrefix] = wrapAction(withTypeScopedPerm(planningStatusPerm,
		"requirement_setting:status", "defect_setting:status"), controller.CreateStatus)
	r[http.MethodPut][statusPrefix+`/:statusId`] = wrapAction(withTypeScopedPerm(planningStatusPerm,
		"requirement_setting:status", "defect_setting:status"), controller.UpdateStatus)
	r[http.MethodDelete][statusPrefix+`/:statusId`] = wrapAction(withTypeScopedPerm(planningStatusPerm,
		"requirement_setting:status", "defect_setting:status"), controller.DeleteStatus)
	r[http.MethodPut][statusPrefix+`/reset`] = wrapAction(withTypeScopedPerm(planningStatusPerm,
		"requirement_setting:status", "defect_setting:status"), controller.ResetStatuses)
	r[http.MethodPut][statusPrefix+`/order`] = wrapAction(withTypeScopedPerm(map[string]string{
		define.TypeKeyRequirement: "requirement_setting:status",
		define.TypeKeyDefect:      "defect_setting:status",
		define.TypeKeyPlanning:    "planning:edit",
	}), controller.ReorderStatuses)

	// Priority CRUD.
	r[http.MethodGet][prioPrefix] = wrapAction(withWorkspaceMember(), controller.ListPriorities)
	r[http.MethodPost][prioPrefix] = wrapAction(withTypeScopedPerm(planningStatusPerm,
		"requirement_setting:priority", "defect_setting:priority"), controller.CreatePriority)
	r[http.MethodPut][prioPrefix+`/default`] = wrapAction(withTypeScopedPerm(planningStatusPerm,
		"requirement_setting:priority", "defect_setting:priority"), controller.SetDefaultPriority)
	r[http.MethodPut][prioPrefix+`/:priorityId`] = wrapAction(withTypeScopedPerm(planningStatusPerm,
		"requirement_setting:priority", "defect_setting:priority"), controller.UpdatePriority)
	r[http.MethodDelete][prioPrefix+`/:priorityId`] = wrapAction(withTypeScopedPerm(planningStatusPerm,
		"requirement_setting:priority", "defect_setting:priority"), controller.DeletePriority)
	r[http.MethodPut][prioPrefix+`/reset`] = wrapAction(withTypeScopedPerm(planningStatusPerm,
		"requirement_setting:priority", "defect_setting:priority"), controller.ResetPriorities)
	r[http.MethodPut][prioPrefix+`/order`] = wrapAction(withTypeScopedPerm(map[string]string{
		define.TypeKeyRequirement: "requirement_setting:priority",
		define.TypeKeyDefect:      "defect_setting:priority",
		define.TypeKeyPlanning:    "planning:edit",
	}), controller.ReorderPriorities)

	severityPrefix := `/api/workspaces/:wsId/severities`
	manageSeverity := withPerm("defect_setting:priority")
	r[http.MethodGet][severityPrefix] = wrapAction(withWorkspaceMember(), controller.ListSeverities)
	r[http.MethodPost][severityPrefix] = wrapAction(manageSeverity, controller.SaveSeverity)
	r[http.MethodPut][severityPrefix+`/:severityId`] = wrapAction(manageSeverity, controller.SaveSeverity)
	r[http.MethodDelete][severityPrefix+`/:severityId`] = wrapAction(manageSeverity, controller.DeleteSeverity)
	r[http.MethodPut][severityPrefix+`/default`] = wrapAction(manageSeverity, controller.SetDefaultSeverity)
	r[http.MethodPut][severityPrefix+`/reset`] = wrapAction(manageSeverity, controller.ResetSeverities)
	r[http.MethodPut][severityPrefix+`/order`] = wrapAction(manageSeverity, controller.ReorderSeverities)

	// Start/End status configuration.
	settingsPrefix := `/api/workspaces/:wsId/settings`
	r[http.MethodGet][settingsPrefix] = wrapAction(withWorkspaceMember(), controller.GetWorkspaceSettings)
	r[http.MethodPut][settingsPrefix] = wrapAction(withTypeScopedPerm(planningStatusPerm,
		"requirement_setting:status", "defect_setting:status"), controller.UpdateWorkspaceSettings)

	statusActions := `/api/workspaces/:wsId/status-action-settings`
	manageStatusActions := withPerm("requirement_setting:status")
	r[http.MethodGet][statusActions] = wrapAction(manageStatusActions, controller.GetWorkspaceStatusActions)
	r[http.MethodPut][statusActions] = wrapAction(manageStatusActions, controller.SetWorkspaceStatusActionsEnabled)
	r[http.MethodPost][statusActions+`/actions`] = wrapAction(manageStatusActions, controller.SaveWorkspaceStatusAction)
	r[http.MethodPut][statusActions+`/actions/:actionId`] = wrapAction(manageStatusActions, controller.SaveWorkspaceStatusAction)
	r[http.MethodDelete][statusActions+`/actions/:actionId`] = wrapAction(manageStatusActions, controller.SaveWorkspaceStatusAction)
	r[http.MethodPost][statusActions+`/mappings`] = wrapAction(manageStatusActions, controller.SaveWorkspaceStatusActionMapping)
	r[http.MethodPut][statusActions+`/mappings/:mappingId`] = wrapAction(manageStatusActions, controller.SaveWorkspaceStatusActionMapping)
	r[http.MethodDelete][statusActions+`/mappings/:mappingId`] = wrapAction(manageStatusActions, controller.SaveWorkspaceStatusActionMapping)
}

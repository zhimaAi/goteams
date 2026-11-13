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

	// Start/End status configuration.
	settingsPrefix := `/api/workspaces/:wsId/settings`
	r[http.MethodGet][settingsPrefix] = wrapAction(withWorkspaceMember(), controller.GetWorkspaceSettings)
	r[http.MethodPut][settingsPrefix] = wrapAction(withTypeScopedPerm(planningStatusPerm,
		"requirement_setting:status", "defect_setting:status"), controller.UpdateWorkspaceSettings)
}

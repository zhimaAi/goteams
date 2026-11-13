// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegPlanningRoutes registers planning group and planning row routes.
// Read operations require planning:view; all write operations require planning:edit.
func RegPlanningRoutes(r lib_web.Route) {
	groupPrefix := `/api/workspaces/:wsId/planning-groups`
	planningPrefix := `/api/workspaces/:wsId/plannings`

	// Planning groups.
	r[http.MethodGet][groupPrefix] = wrapAction(withPerm("planning:view"), controller.ListPlanningGroups)
	r[http.MethodPost][groupPrefix] = wrapAction(withPerm("planning:edit"), controller.CreatePlanningGroup)
	r[http.MethodPut][groupPrefix+`/:groupId`] = wrapAction(withPerm("planning:edit"), controller.UpdatePlanningGroup)
	r[http.MethodDelete][groupPrefix+`/:groupId`] = wrapAction(withPerm("planning:edit"), controller.DeletePlanningGroup)

	// Planning rows.
	r[http.MethodGet][planningPrefix] = wrapAction(withPerm("planning:view"), controller.ListPlannings)
	r[http.MethodPost][planningPrefix] = wrapAction(withPerm("planning:edit"), controller.CreatePlanning)
	r[http.MethodPost][planningPrefix+`/batch`] = wrapAction(withPerm("planning:edit"), controller.BatchCreatePlannings)
	r[http.MethodPut][planningPrefix+`/:planningId`] = wrapAction(withPerm("planning:edit"), controller.UpdatePlanning)
	r[http.MethodDelete][planningPrefix+`/:planningId`] = wrapAction(withPerm("planning:edit"), controller.DeletePlanning)
	r[http.MethodPut][planningPrefix+`/:planningId/move`] = wrapAction(withPerm("planning:edit"), controller.MovePlanning)
	r[http.MethodPut][planningPrefix+`/:planningId/relations`] = wrapAction(withPerm("planning:edit"), controller.SavePlanningRelations)

	// Recycle bin: browsing needs view, restore and permanent delete need edit.
	r[http.MethodGet][planningPrefix+`/recycle-bin`] = wrapAction(withPerm("planning:view"), controller.ListDeletedPlannings)
	r[http.MethodPost][planningPrefix+`/recycle-bin/restore`] = wrapAction(withPerm("planning:edit"), controller.RestorePlannings)
	r[http.MethodPost][planningPrefix+`/recycle-bin/purge`] = wrapAction(withPerm("planning:edit"), controller.PurgePlannings)

	// Excel import / export.
	r[http.MethodGet][planningPrefix+`/import-template`] = wrapAction(withPerm("planning:edit"), controller.DownloadPlanningImportTemplate)
	r[http.MethodGet][planningPrefix+`/export`] = wrapAction(withPerm("planning:view"), controller.ExportPlannings)
	r[http.MethodPost][planningPrefix+`/import-preview`] = wrapAction(withPerm("planning:edit"), controller.PreviewPlanningImport)
	r[http.MethodPost][planningPrefix+`/import`] = wrapAction(withPerm("planning:edit"), controller.ImportPlannings)
}

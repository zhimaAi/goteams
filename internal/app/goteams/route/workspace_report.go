package route

import (
	"net/http"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
)

// RegWorkspaceReportRoutes registers report generation, history and export for a workspace.
func RegWorkspaceReportRoutes(r lib_web.Route) {
	prefix := `/api/workspaces/:wsId/reports/:reportType`
	view := withPerm(business.ReportView)
	r[http.MethodGet][prefix+`/preview`] = wrapAction(view, controller.GetReportPreview)
	r[http.MethodGet][prefix+`/preview/items`] = wrapAction(view, controller.ListReportPreviewItems)
	r[http.MethodGet][prefix+`/preview/export`] = wrapAction(withPerm(business.ReportExport), controller.ExportReportPreview)
	r[http.MethodGet][prefix+`/options`] = wrapAction(view, controller.GetReportOptions)
	r[http.MethodPost][prefix+`/runs`] = wrapAction(withPerm(business.ReportGenerate), controller.GenerateReport)
	r[http.MethodGet][prefix+`/runs`] = wrapAction(view, controller.ListReports)
	r[http.MethodGet][prefix+`/runs/:runId`] = wrapAction(view, controller.GetReport)
	r[http.MethodGet][prefix+`/runs/:runId/items`] = wrapAction(view, controller.ListReportItems)
	r[http.MethodGet][prefix+`/runs/:runId/export`] = wrapAction(withPerm(business.ReportExport), controller.ExportReport)
}

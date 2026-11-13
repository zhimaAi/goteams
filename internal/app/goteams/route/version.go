// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegVersionRoutes registers version iteration routes (login + permission required).
func RegVersionRoutes(r lib_web.Route) {
	prefix := `/api/workspaces/:wsId/versions`

	r[http.MethodGet][prefix] = wrapAction(withPerm("version:view"), controller.ListVersions)
	r[http.MethodGet][prefix+`/workspace-options`] = wrapAction(withAnyPerm("version:create", "version:edit"), controller.ListVersionWorkspaceOptions)
	r[http.MethodPost][prefix] = wrapAction(withPerm("version:create"), controller.CreateVersion)
	r[http.MethodGet][prefix+`/:versionId/work-items`] = wrapAction(withPerm("version:view"), controller.ListVersionWorkItems)
	r[http.MethodGet][prefix+`/:versionId/filter-options`] = wrapAction(withPerm("version:view"), controller.GetIterationFilterOptions)
	r[http.MethodPut][prefix+`/:versionId`] = wrapAction(withPerm("version:edit"), controller.UpdateVersion)
	r[http.MethodPut][prefix+`/:versionId/ended`] = wrapAction(withPerm("version:edit"), controller.UpdateVersionEnded)
	r[http.MethodDelete][prefix+`/:versionId`] = wrapAction(withPerm("version:delete"), controller.DeleteVersion)
}

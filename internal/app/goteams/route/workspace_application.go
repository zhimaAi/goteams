// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegWorkspaceApplicationRoutes registers workspace application market routes.
func RegWorkspaceApplicationRoutes(r lib_web.Route) {
	r[http.MethodGet][`/api/workspaces/:wsId/applications`] = wrapAction(withAnyPerm("application_market:view", "application_market:manage"), controller.GetWorkspaceApplications)
	r[http.MethodPut][`/api/workspaces/:wsId/applications/:appKey`] = wrapAction(withPerm("application_market:manage"), controller.UpdateWorkspaceApplication)
}

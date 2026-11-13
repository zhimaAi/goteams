package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegWorkspaceNavigationRoutes registers project navigation settings endpoints.
func RegWorkspaceNavigationRoutes(r lib_web.Route) {
	r[http.MethodGet][`/api/workspaces/:wsId/navigation-settings`] = wrapAction(withWorkspaceMember(), controller.GetWorkspaceNavigationSettings)
	r[http.MethodPut][`/api/workspaces/:wsId/navigation-settings/me`] = wrapAction(withWorkspaceMember(), controller.SaveMyWorkspaceNavigationSettings)
	r[http.MethodPut][`/api/workspaces/:wsId/navigation-settings`] = wrapAction(withPerm("project_info:edit"), controller.SaveWorkspaceNavigationSettings)
}

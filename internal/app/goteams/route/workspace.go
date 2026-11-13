// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegWorkspaceRoutes registers workspace routes (login + permission required, except the public
// template avatar endpoint).
func RegWorkspaceRoutes(r lib_web.Route) {
	auth := withAuth()

	r[http.MethodGet][`/api/workspaces`] = wrapAction(auth, controller.ListWorkspaces)
	r[http.MethodPost][`/api/workspaces`] = wrapAction(withPerm("project:create"), controller.CreateWorkspace)
	r[http.MethodGet][`/api/workspace-types`] = wrapAction(withPerm("project:create"), controller.ListWorkspaceTypes)
	r[http.MethodPost][`/api/workspace-types`] = wrapAction(withPerm("project:create"), controller.CreateWorkspaceType)
	r[http.MethodGet][`/api/workspace-templates`] = wrapAction(withPerm("project:create"), controller.ListWorkspaceTemplates)
	// The export is token-gated inside the controller (a signed, short-lived `?token=`), not
	// session-gated: a browser address bar and a headless curl can attach a query string but not
	// the Authorization header. A deployment without a template account answers 403 too.
	// The icon endpoint stays public on purpose: the create-project dialog loads the card images as
	// plain <img> requests, which cannot carry the Authorization header.
	r[http.MethodGet][`/api/workspace-templates/export`] = controller.ExportWorkspaceTemplates
	r[http.MethodGet][`/api/workspace-templates/:wsId/icon`] = controller.GetWorkspaceTemplateIcon
	r[http.MethodGet][`/api/workspaces/:wsId`] = wrapAction(withWorkspaceMember(), controller.GetWorkspace)
	r[http.MethodGet][`/api/workspaces/:wsId/access`] = wrapAction(auth, controller.GetWorkspaceAccess)
	r[http.MethodPost][`/api/workspaces/:wsId/access-requests`] = wrapAction(auth, controller.RequestWorkspaceAccess)
	r[http.MethodPut][`/api/workspaces/:wsId`] = wrapAction(withPerm("project_info:edit"), controller.UpdateWorkspace)
	r[http.MethodGet][`/api/workspaces/:wsId/icon`] = wrapAction(withWorkspaceMember(), controller.GetWorkspaceIcon)
	r[http.MethodPost][`/api/workspaces/:wsId/icon`] = wrapAction(withPerm("project_info:edit"), controller.UploadWorkspaceIcon)
	r[http.MethodDelete][`/api/workspaces/:wsId/icon`] = wrapAction(withPerm("project_info:edit"), controller.DeleteWorkspaceIcon)
	r[http.MethodDelete][`/api/workspaces/:wsId`] = wrapAction(withPerm("project:manage"), controller.DeleteWorkspace)
	// Ending/reopening a project is available to project admins inside project settings and to project managers in enterprise management.
	r[http.MethodPut][`/api/workspaces/:wsId/ended`] = wrapAction(withAnyPerm("workspace:end", "project:manage"), controller.UpdateWorkspaceEnded)
	r[http.MethodGet][`/api/enterprise/projects`] = wrapAction(withPerm("project:manage"), controller.ListEnterpriseProjects)
	r[http.MethodPut][`/api/enterprise/projects/sort`] = wrapAction(withAuth(), controller.SortEnterpriseProjects)
}

// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegWorkspaceMemberRoutes registers workspace member management routes.
func RegWorkspaceMemberRoutes(r lib_web.Route) {
	prefix := `/api/workspaces/:wsId/members`

	r[http.MethodGet][prefix] = wrapAction(withWorkspaceMember(), controller.ListMembers)
	r[http.MethodGet][prefix+`/:userId/avatar`] = wrapAction(withPerm("workspace:view"), controller.GetMemberAvatar)
	r[http.MethodGet][prefix+`/candidates`] = wrapAction(withPerm("project_member:add"), controller.ListMemberCandidates)
	r[http.MethodPost][prefix] = wrapAction(withPerm("project_member:add"), controller.AddMember)
	// Copying another project also creates its missing roles here, so both keys are required.
	r[http.MethodPost][prefix+`/copy`] = wrapAction(withAllPerm("project_member:add", "project_role:add"), controller.CopyProjectMembers)
	r[http.MethodPut][prefix+`/:userId/role`] = wrapAction(withPerm("project_member:change_role"), controller.UpdateMemberRole)
	r[http.MethodDelete][prefix+`/:userId`] = wrapAction(withPerm("project_member:remove"), controller.RemoveMember)
	r[http.MethodGet][prefix+`/:userId/removal-preview`] = wrapAction(withPerm("project_member:remove"), controller.PreviewWorkspaceMemberRemoval)
	// Default project admin transfer: guarded by identity inside the handler (project owner, or the
	// tenant owner as the only fallback), never by a permission key. Kept off the members prefix so
	// the static segment cannot collide with the :userId wildcard.
	r[http.MethodPost][`/api/workspaces/:wsId/owner/transfer`] = wrapAction(withWorkspaceMember(), controller.TransferWorkspaceOwner)
}

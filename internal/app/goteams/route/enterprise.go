// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegEnterpriseRoutes registers enterprise-level routes (login + permission required).
func RegEnterpriseRoutes(r lib_web.Route) {
	// Permissions list (for role config page).
	r[http.MethodGet][`/api/permissions`] = wrapAction(withAuth(), controller.ListPermissions)

	// Enterprise members (require member:view / member:invite / member:edit / member:remove / member:reset_password).
	r[http.MethodPost][`/api/upload/avatar`] = wrapAction(withAnyPerm("member:invite", "member:edit"), controller.UploadEnterpriseAvatar)
	r[http.MethodGet][`/api/enterprise/members`] = wrapAction(withAnyPerm(
		"member:view",
		"member:view_role_list",
		"member:edit_role_permissions",
		"external_notification:manage",
		// 流水线和专家团管理弹窗需要选择适用成员。
		"pipeline:manage",
	), controller.ListEnterpriseMembers)
	r[http.MethodPost][`/api/enterprise/members`] = wrapAction(withPerm("member:invite"), controller.CreateEnterpriseMember)
	r[http.MethodPut][`/api/enterprise/members/:userId`] = wrapAction(withPerm("member:edit"), controller.UpdateEnterpriseMember)
	r[http.MethodDelete][`/api/enterprise/members/:userId`] = wrapAction(withPerm("member:remove"), controller.RemoveEnterpriseMember)
	r[http.MethodGet][`/api/enterprise/members/:userId/removal-preview`] = wrapAction(withPerm("member:remove"), controller.PreviewEnterpriseMemberRemoval)
	r[http.MethodPut][`/api/enterprise/members/:userId/password`] = wrapAction(withPerm("member:reset_password"), controller.ResetEnterpriseMemberPassword)
	r[http.MethodPut][`/api/enterprise/members/:userId/status`] = wrapAction(withPerm("member:edit"), controller.UpdateEnterpriseMemberStatus)
	r[http.MethodGet][`/api/enterprise/member-logs`] = wrapAction(withPerm("member:view"), controller.ListMemberLogs)

	// Member invitation link (manage: member:invite; public endpoints authenticated by invitation token).
	r[http.MethodPost][`/api/enterprise/invitation`] = wrapAction(withPerm("member:invite"), controller.CreateInvitation)
	r[http.MethodGet][`/api/enterprise/invitation`] = wrapAction(withPerm("member:invite"), controller.GetInvitation)
	r[http.MethodDelete][`/api/enterprise/invitation`] = wrapAction(withPerm("member:invite"), controller.StopInvitation)
	r[http.MethodGet][`/api/invite/:token`] = controller.GetInviteInfo
	r[http.MethodPost][`/api/invite/:token/avatar`] = controller.UploadInviteAvatar
	r[http.MethodPost][`/api/invite/:token/email-code`] = controller.SendInviteEmailCode
	r[http.MethodPost][`/api/invite/:token/accept`] = controller.AcceptInvite

	// Enterprise departments (view opens up to anyone who can see the member/role pages,
	// e.g. resolving department names on the role member list; manage reuses member:edit).
	// external_notification:manage is included because the enterprise notification strategy
	// member picker builds its organization tree from this list.
	// 流水线和专家团管理弹窗同样需要选择适用部门。
	r[http.MethodGet][`/api/enterprise/departments`] = wrapAction(withAnyPerm("member:view", "member:view_role_list", "member:edit_role_permissions", "external_notification:manage", "pipeline:manage"), controller.ListDepartments)
	r[http.MethodPost][`/api/enterprise/departments`] = wrapAction(withPerm("member:edit"), controller.CreateDepartment)
	r[http.MethodPut][`/api/enterprise/departments/:departmentId`] = wrapAction(withPerm("member:edit"), controller.RenameDepartment)
	r[http.MethodDelete][`/api/enterprise/departments/:departmentId`] = wrapAction(withPerm("member:edit"), controller.DeleteDepartment)

	// Default system administrator transfer: guarded by identity inside the handler (only the current
	// owner may transfer), never by a permission key, or any enterprise admin could seize the tenant.
	// Note: /api/enterprise/name is registered in route/plan.go on this line, so it is not repeated here.
	r[http.MethodPost][`/api/enterprise/owner/transfer`] = wrapAction(withAuth(), controller.TransferTenantOwner)

	// Enterprise member Excel import/export (import creates members: member:invite; export follows the list view).
	r[http.MethodGet][`/api/enterprise/members/import-template`] = wrapAction(withPerm("member:invite"), controller.DownloadEnterpriseMemberImportTemplate)
	r[http.MethodGet][`/api/enterprise/members/export`] = wrapAction(withPerm("member:view"), controller.ExportEnterpriseMembers)
	r[http.MethodPost][`/api/enterprise/members/import-preview`] = wrapAction(withPerm("member:invite"), controller.PreviewEnterpriseMemberImport)
	r[http.MethodPost][`/api/enterprise/members/import`] = wrapAction(withPerm("member:invite"), controller.ImportEnterpriseMembers)

	// Enterprise roles.
	// Role list is also used by the members page to render role names and role selectors.
	r[http.MethodGet][`/api/enterprise/roles`] = wrapAction(withAnyPerm(
		"member:view",
		"member:invite",
		"member:edit",
		"member:view_role_list",
		"member:create_role",
		"member:edit_role_permissions",
		"member:delete_role",
		// The enterprise notification strategy member picker groups members by enterprise role.
		"external_notification:manage",
	), controller.ListEnterpriseRoles)
	r[http.MethodPost][`/api/enterprise/roles`] = wrapAction(withPerm("member:create_role"), controller.CreateEnterpriseRole)
	r[http.MethodPut][`/api/enterprise/roles/:roleId`] = wrapAction(withPerm("member:edit_role_permissions"), controller.UpdateEnterpriseRoleBasic)
	r[http.MethodPut][`/api/enterprise/roles/:roleId/permissions`] = wrapAction(withPerm("member:edit_role_permissions"), controller.UpdateEnterpriseRolePermissions)
	r[http.MethodPut][`/api/enterprise/roles/:roleId/team-work-scope`] = wrapAction(withPerm("member:edit_role_permissions"), controller.UpdateEnterpriseRoleTeamWorkScope)
	r[http.MethodDelete][`/api/enterprise/roles/:roleId`] = wrapAction(withPerm("member:delete_role"), controller.DeleteEnterpriseRole)

	// Project roles for a workspace (for role config page).
	r[http.MethodGet][`/api/workspaces/:wsId/roles`] = wrapAction(withAnyPerm("workspace:view", "project_role:add", "project_role:edit", "project_role:delete", "project_member:add", "project_member:change_role"), controller.ListProjectRoles)
	r[http.MethodGet][`/api/workspaces/:wsId/roles/copy-targets`] = wrapAction(withPerm("project_role:edit"), controller.ListProjectRoleCopyTargets)
	r[http.MethodPost][`/api/workspaces/:wsId/roles`] = wrapAction(withPerm("project_role:add"), controller.CreateProjectRole)
	r[http.MethodPost][`/api/workspaces/:wsId/roles/:roleId/copy`] = wrapAction(withPerm("project_role:edit"), controller.CopyProjectRole)
	r[http.MethodPut][`/api/workspaces/:wsId/roles/:roleId`] = wrapAction(withPerm("project_role:edit"), controller.UpdateProjectRoleBasic)
	r[http.MethodPut][`/api/workspaces/:wsId/roles/:roleId/permissions`] = wrapAction(withPerm("project_role:edit"), controller.UpdateProjectRolePermissions)
	r[http.MethodPut][`/api/workspaces/:wsId/roles/:roleId/data-view-scope`] = wrapAction(withPerm("project_role:edit"), controller.UpdateProjectRoleDataViewScope)
	r[http.MethodDelete][`/api/workspaces/:wsId/roles/:roleId`] = wrapAction(withPerm("project_role:delete"), controller.DeleteProjectRole)
}

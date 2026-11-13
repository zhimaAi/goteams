// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"slices"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/permission"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

func workspacePermissionApplication(groupKey string) string {
	switch groupKey {
	case permission.GroupKeyDefect, permission.GroupKeyDefectSetup:
		return define.AppKeyDefect
	case permission.GroupKeyVersion:
		return define.AppKeyIteration
	case permission.GroupKeyPlanning:
		return define.AppKeyPlanning
	case permission.GroupKeyGantt:
		return define.AppKeyGantt
	case permission.GroupKeyKnowledgeBase:
		return define.AppKeyKnowledgeBase
	case permission.GroupKeyTicket:
		return define.AppKeyIssue
	case permission.GroupKeyDashboard:
		return define.AppKeyDashboard
	default:
		return ""
	}
}

// IsWorkspacePermissionGroupEnabled 按空间实际启用的应用过滤权限目录，不修改角色授权。
func IsWorkspacePermissionGroupEnabled(groupKey string, enabledApplications []string) bool {
	appKey := workspacePermissionApplication(groupKey)
	return appKey == "" || slices.Contains(enabledApplications, appKey)
}

func filterIssueWorkspacePermissions(adminId int, wsId string, keys []string) []string {
	if wsId == "" {
		return keys
	}
	issueWorkspace, err := IsIssueWorkspace(adminId, cast.ToInt(wsId))
	if err != nil {
		logs.Error("load workspace type before permissions failed, err:%s", err.Error())
		return nil
	}
	return slices.DeleteFunc(keys, func(key string) bool {
		p, ok := permission.GetPermission(key)
		if ok && p.Group == permission.GroupKeySLA {
			return !issueWorkspace
		}
		return ok && !workspaceAppAvailable(workspacePermissionApplication(p.Group), issueWorkspace)
	})
}

// ======================== Permission Check (permission key driven) ========================

// enterprisePermGroups maps permission group keys to enterprise scope.
// Permissions in these groups are checked against the user's enterprise role (users.role_id).
var enterprisePermGroups = map[string]bool{
	permission.GroupKeyTeamWork:         true,
	permission.GroupKeyProject:          true,
	permission.GroupKeyPlan:             true,
	permission.GroupKeyMemberEnterprise: true,
	permission.GroupKeyMemberRoles:      true,
	permission.GroupKeyStorage:          true,
	// SSO 在企业经营设置页配置，该上下文没有 wsId，因此该组必须按 users.role_id 判定，
	// 而不是 workspace_members.role_id。
	permission.GroupKeySso:             true,
	permission.GroupKeyPersonalization: true,
	// Pipeline permissions are enterprise-scope: pipeline routes carry no wsId, so they must
	// be resolved against users.role_id instead of workspace_members.role_id.
	permission.GroupKeyPipeline:       true,
	permission.GroupKeyExternalNotify: true,
}

// tenantUserIdentity carries the enterprise-side identity of a tenant user: the transferable
// owner flag and the enterprise role id, both living on the same users row.
type tenantUserIdentity struct {
	IsOwner bool
	RoleId  int
}

// getTenantUserIdentity loads users.is_owner and users.role_id in one query (admin_id isolated).
func getTenantUserIdentity(adminId int, userId int) tenantUserIdentity {
	if adminId <= 0 || userId <= 0 {
		return tenantUserIdentity{}
	}
	m := msql.Model(define.TableUsers, define.Postgres)
	row, err := m.Where2([][]string{
		{`id`, cast.ToString(userId)},
		{`admin_id`, cast.ToString(adminId)},
	}).Field(`is_owner, role_id`).Find()
	if err != nil {
		logs.Error("get tenant user identity error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return tenantUserIdentity{}
	}
	return tenantUserIdentity{IsOwner: row[`is_owner`] == `1`, RoleId: cast.ToInt(row[`role_id`])}
}

// IsTenantOwner reports whether the user is the tenant's default system administrator.
// Ownership is transferable, so it lives on users.is_owner and is no longer implied by id == admin_id.
func IsTenantOwner(adminId int, userId int) bool {
	return getTenantUserIdentity(adminId, userId).IsOwner
}

// HasPermission checks whether a user has a specific permission.
// Permission checking logic:
//  1. permKey is empty → public endpoint.
//  2. users.is_owner → tenant owner, all permissions granted.
//  3. permKey belongs to enterprise group (member:*) → check enterprise role (users.role_id).
//  4. permKey belongs to project group → check project role (workspace_members.role_id).
func HasPermission(adminId int, userId int, wsId string, permKey string) bool {
	// Empty key means public endpoint; checked first so it costs no query.
	if len(permKey) == 0 {
		return true
	}
	perm, ok := permission.GetPermission(permKey)
	if !ok {
		return false
	}
	if perm.Group == permission.GroupKeySLA {
		issueWorkspace, err := IsIssueWorkspace(adminId, cast.ToInt(wsId))
		if err != nil {
			logs.Error("load workspace type for SLA permission failed, err:%s", err.Error())
			return false
		}
		if !issueWorkspace {
			return false
		}
	}
	// 类型限制先于 owner 放行，防止通过旧接口使用工单空间未提供的模块。
	if !workspaceAppAvailable(workspacePermissionApplication(perm.Group), true) {
		issueWorkspace, err := IsIssueWorkspace(adminId, cast.ToInt(wsId))
		if err != nil {
			logs.Error("load workspace type before module permission failed, err:%s", err.Error())
			return false
		}
		if issueWorkspace {
			return false
		}
	}
	if permKey == define.PermissionTicketCopyLink || permKey == define.PermissionTicketManage {
		enabled, err := IsWorkspaceApplicationEnabled(adminId, cast.ToInt(wsId), define.AppKeyIssue)
		if err != nil {
			logs.Error("check issue application before permission failed, err:%s", err.Error())
			return false
		}
		if !enabled {
			return false
		}
	}
	if permKey == define.PermissionDashboardView || permKey == define.PermissionDashboardCreate {
		enabled, err := IsWorkspaceApplicationEnabled(adminId, cast.ToInt(wsId), define.AppKeyDashboard)
		if err != nil {
			logs.Error("check dashboard application before permission failed, err:%s", err.Error())
			return false
		}
		if !enabled {
			return false
		}
	}

	// Tenant owner has all permissions.
	identity := getTenantUserIdentity(adminId, userId)
	if identity.IsOwner {
		return true
	}

	// Determine permission scope: enterprise or project.
	if enterprisePermGroups[perm.Group] {
		// Enterprise permission: role id already loaded above.
		if identity.RoleId == 0 {
			return false
		}
		return hasRolePermission(adminId, identity.RoleId, permKey)
	}

	// Project permission: check workspace_members.role_id.
	roleId := getProjectRoleId(adminId, userId, wsId)
	if roleId == 0 {
		return false
	}
	return hasRolePermission(adminId, roleId, permKey)
}

// GetUserPermissions returns all permission keys for a user.
// Merges enterprise permissions + project permissions for the given workspace.
func GetUserPermissions(adminId int, userId int, wsId string) []string {
	identity := getTenantUserIdentity(adminId, userId)
	if identity.IsOwner {
		keys := make([]string, 0, len(permission.AllPermissions))
		for _, p := range permission.AllPermissions {
			keys = append(keys, p.Key)
		}
		return filterIssueWorkspacePermissions(adminId, wsId, keys)
	}

	permSet := make(map[string]bool)

	// Enterprise permissions (from users.role_id).
	if identity.RoleId > 0 {
		for _, k := range getRolePermissions(adminId, identity.RoleId) {
			permSet[k] = true
		}
	}

	// Project permissions (from workspace_members.role_id).
	if wsId != `` {
		projectRoleId := getProjectRoleId(adminId, userId, wsId)
		if projectRoleId > 0 {
			projectPerms := getRolePermissions(adminId, projectRoleId)
			for _, k := range projectPerms {
				permSet[k] = true
			}
		}
	}

	if len(permSet) == 0 {
		return nil
	}

	keys := make([]string, 0, len(permSet))
	for k := range permSet {
		keys = append(keys, k)
	}
	return filterIssueWorkspacePermissions(adminId, wsId, keys)
}

// GetUserPermissionGroups returns the permission groups for a user in a workspace.
func GetUserPermissionGroups(adminId int, userId int, wsId string, lang string) []permission.Group {
	userPerms := GetUserPermissions(adminId, userId, wsId)
	if len(userPerms) == 0 {
		return nil
	}

	var enabledApplications []string
	if wsId != "" {
		var err error
		enabledApplications, err = GetEnabledApplicationKeys(adminId, cast.ToInt(wsId))
		if err != nil {
			logs.Error("load applications before permission groups failed, err:%s", err.Error())
			return nil
		}
	}

	permSet := make(map[string]bool, len(userPerms))
	for _, k := range userPerms {
		permSet[k] = true
	}

	var result []permission.Group
	for _, group := range permission.AllGroups {
		if !IsWorkspacePermissionGroupEnabled(group.Key, enabledApplications) {
			continue
		}
		var filtered []permission.Permission
		for _, p := range group.Permissions {
			if permSet[p.Key] {
				p.Name = permission.GetPermissionName(p.Key, lang)
				filtered = append(filtered, p)
			}
		}
		if len(filtered) > 0 {
			result = append(result, permission.Group{
				Key:         group.Key,
				Name:        permission.GetGroupName(group.Key, lang),
				Permissions: filtered,
			})
		}
	}
	return result
}

// ======================== Project Permission ========================

// getProjectRoleId retrieves the user's project role ID in a workspace.
func getProjectRoleId(adminId int, userId int, wsId string) int {
	if wsId == `` || userId == 0 {
		return 0
	}
	m := msql.Model(define.TableWorkspaceMembers, define.Postgres)
	roleId, err := m.Where2([][]string{
		{`user_id`, cast.ToString(userId)},
		{`workspace_id`, wsId},
		{`admin_id`, cast.ToString(adminId)},
	}).Value(`role_id`)
	if err != nil || len(roleId) == 0 {
		return 0
	}
	return cast.ToInt(roleId)
}

// ======================== Role Management ========================

// GetRolePermissions returns the permission key list for a given role (admin_id isolated).
func GetRolePermissions(adminId int, roleId int) []string {
	return getRolePermissions(adminId, roleId)
}

// SetRolePermissions batch-sets permissions for a role (admin_id isolated).
func SetRolePermissions(adminId int, roleId int, permKeys []string) error {
	deleteM := msql.Model(define.TableRolePermissions, define.Postgres)
	_, err := deleteM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`role_id`, cast.ToString(roleId)},
	}).Delete()
	if err != nil {
		logs.Error("delete role_permissions error: %s", err.Error())
		return err
	}
	for _, key := range permKeys {
		if _, ok := permission.GetPermission(key); !ok {
			continue
		}
		now := tool.Time2Int()
		insertM := msql.Model(define.TableRolePermissions, define.Postgres)
		_, err = insertM.Insert(msql.Datas{
			`admin_id`:       adminId,
			`role_id`:        roleId,
			`permission_key`: key,
			`created_at`:     now,
			`updated_at`:     now,
		})
		if err != nil {
			logs.Error("insert role_permissions error: %s", err.Error())
		}
	}
	return nil
}

// ======================== Internal Functions ========================

// hasRolePermission checks if a role has a specific permission (admin_id isolated).
func hasRolePermission(adminId int, roleId int, permKey string) bool {
	m := msql.Model(define.TableRolePermissions, define.Postgres)
	id, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`role_id`, cast.ToString(roleId)},
		{`permission_key`, permKey},
	}).Value(`id`)
	if err != nil {
		logs.Error("check permission error: %s", err.Error())
		return false
	}
	return len(id) > 0
}

// getRolePermissions returns all permission keys for a role (admin_id isolated).
func getRolePermissions(adminId int, roleId int) []string {
	m := msql.Model(define.TableRolePermissions, define.Postgres)
	list, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`role_id`, cast.ToString(roleId)},
	}).Select()
	if err != nil {
		logs.Error("get role permissions error: %s", err.Error())
		return nil
	}
	keys := make([]string, 0, len(list))
	for _, row := range list {
		keys = append(keys, row[`permission_key`])
	}
	return keys
}

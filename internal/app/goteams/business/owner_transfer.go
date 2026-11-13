// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"errors"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/permission"
	"goteams/internal/pkg/lib_i18n"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// Ownership transfer errors (tenant owner and project owner).
var (
	ErrOwnerTransferForbidden = errors.New("only the current owner can transfer ownership")
	ErrOwnerTransferTarget    = errors.New("invalid ownership transfer target")
	ErrProjectOwnerProtected  = errors.New("default project admin cannot be removed")
)

// WorkspaceBriefModel identifies a workspace by id and name, used where only a label is needed.
type WorkspaceBriefModel struct {
	Id   int
	Name string
}

// enterprisePresetRoleId returns the tenant's preset enterprise role id for a preset role name
// (e.g. perm_role_enterprise_admin). Preset roles carry is_system = 1 and cannot be deleted.
func enterprisePresetRoleId(adminId int, presetName string) (int, error) {
	m := msql.Model(define.TableRoles, define.Postgres)
	id, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`scope`, permission.ScopeEnterprise},
		{`name`, lib_i18n.WrapKey(presetName)},
		{`is_system`, `1`},
	}).Value(`id`)
	if err != nil {
		logs.Error("get enterprise preset role error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return 0, err
	}
	if len(id) == 0 {
		return 0, ErrEnterpriseRoleNotFound
	}
	return cast.ToInt(id), nil
}

// workspacePresetRoleId returns a workspace's preset project role id for a preset role name
// (e.g. perm_role_project_admin).
func workspacePresetRoleId(adminId, wsId int, presetName string) (int, error) {
	m := msql.Model(define.TableRoles, define.Postgres)
	id, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, cast.ToString(wsId)},
		{`scope`, permission.ScopeProject},
		{`name`, lib_i18n.WrapKey(presetName)},
		{`is_system`, `1`},
	}).Value(`id`)
	if err != nil {
		logs.Error("get project preset role error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return 0, err
	}
	if len(id) == 0 {
		return 0, ErrWorkspaceRoleNotFound
	}
	return cast.ToInt(id), nil
}

// transferableTenantMember reports whether the user can receive ownership: same tenant, enabled
// and not removed. Disabled or removed members are never valid transfer targets.
func transferableTenantMember(adminId, userId int) (bool, error) {
	if adminId <= 0 || userId <= 0 {
		return false, nil
	}
	m := msql.Model(define.TableUsers, define.Postgres)
	id, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(userId)},
		{`deleted_at`, `0`},
		{`status`, `1`},
	}).Value(`id`)
	if err != nil {
		logs.Error("check ownership transfer target error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return false, err
	}
	return len(id) > 0, nil
}

// TransferTenantOwnerModel describes the transfer-default-system-administrator use case.
type TransferTenantOwnerModel struct {
	AdminId        int
	OperatorUserId int
	TargetUserId   int
}

// TransferTenantOwner hands the default system administrator role to another enterprise member.
// Only the current owner may run it (no permission key can grant it, otherwise any enterprise admin
// could seize the tenant). The previous owner drops to the preset enterprise member role and the
// new owner takes the preset enterprise admin role; admin_id stays untouched on both rows.
func TransferTenantOwner(model TransferTenantOwnerModel) error {
	if !IsTenantOwner(model.AdminId, model.OperatorUserId) {
		return ErrOwnerTransferForbidden
	}
	if model.TargetUserId <= 0 || model.TargetUserId == model.OperatorUserId {
		return ErrOwnerTransferTarget
	}
	valid, err := transferableTenantMember(model.AdminId, model.TargetUserId)
	if err != nil {
		return err
	}
	if !valid {
		return ErrOwnerTransferTarget
	}

	adminRoleId, err := enterprisePresetRoleId(model.AdminId, "perm_role_enterprise_admin")
	if err != nil {
		return err
	}
	memberRoleId, err := enterprisePresetRoleId(model.AdminId, "perm_role_enterprise_member")
	if err != nil {
		return err
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin transfer tenant owner error: %s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := tool.Time2Int()
	if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableUsers+`
		SET is_owner = 0, role_id = $1, updated_at = $2 WHERE admin_id = $3 AND id = $4`,
		tx, memberRoleId, now, model.AdminId, model.OperatorUserId); err != nil {
		logs.Error("demote previous tenant owner error: %s", err.Error())
		return err
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableUsers+`
		SET is_owner = 1, role_id = $1, updated_at = $2 WHERE admin_id = $3 AND id = $4 AND deleted_at = 0`,
		tx, adminRoleId, now, model.AdminId, model.TargetUserId); err != nil {
		logs.Error("promote new tenant owner error: %s", err.Error())
		return err
	}

	if err = tx.Commit(); err != nil {
		logs.Error("commit transfer tenant owner error: %s", err.Error())
		return err
	}
	return nil
}

// GetWorkspaceOwnerId returns the workspace's default project admin.
// owner_user_id can be 0 for workspaces created while the environment ran a version line without
// this feature; such rows fall back to created_by and are healed in place so the fallback runs once.
func GetWorkspaceOwnerId(adminId, wsId int) (int, error) {
	m := msql.Model(define.TableWorkspaces, define.Postgres)
	row, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(wsId)},
	}).Field(`owner_user_id, created_by`).Find()
	if err != nil {
		logs.Error("get workspace owner error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return 0, err
	}
	if len(row) == 0 {
		return 0, ErrWorkspaceNotFound
	}
	if ownerId := cast.ToInt(row[`owner_user_id`]); ownerId > 0 {
		return ownerId, nil
	}

	ownerId := cast.ToInt(row[`created_by`])
	if ownerId <= 0 {
		return 0, nil
	}
	if err = setWorkspaceOwner(nil, adminId, wsId, ownerId); err != nil {
		return 0, err
	}
	return ownerId, nil
}

// setWorkspaceOwner writes the workspace's default project admin. tx may be nil.
func setWorkspaceOwner(tx *sql.Tx, adminId, wsId, ownerUserId int) error {
	_, err := msql.RawExec(define.Postgres, `UPDATE `+define.TableWorkspaces+`
		SET owner_user_id = $1, updated_at = $2 WHERE admin_id = $3 AND id = $4`,
		tx, ownerUserId, tool.Time2Int(), adminId, wsId)
	if err != nil {
		logs.Error("set workspace owner error: ws=%d, err=%s", wsId, err.Error())
	}
	return err
}

// ListOwnedWorkspaces returns the active workspaces where the user is the default project admin.
func ListOwnedWorkspaces(adminId, userId int) ([]WorkspaceBriefModel, error) {
	if adminId <= 0 || userId <= 0 {
		return nil, nil
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT id, name FROM `+define.TableWorkspaces+`
		WHERE admin_id = $1 AND owner_user_id = $2 AND COALESCE(ended_at, 0) = 0 ORDER BY id`,
		nil, adminId, userId)
	if err != nil {
		logs.Error("list owned workspaces error: %s", err.Error())
		return nil, err
	}
	items := make([]WorkspaceBriefModel, 0, len(rows))
	for _, row := range rows {
		items = append(items, WorkspaceBriefModel{Id: cast.ToInt(row[`id`]), Name: row[`name`]})
	}
	return items, nil
}

// ensureWorkspaceMembership makes sure the user belongs to the workspace with at least the given
// preset role. Transfer targets are not necessarily members of the projects they receive, and an
// owner or assignee outside the member list would be invisible in the UI.
func ensureWorkspaceMembership(tx *sql.Tx, adminId, wsId, userId int, presetName string) error {
	roleId, err := workspacePresetRoleId(adminId, wsId, presetName)
	if err != nil {
		return err
	}
	now := tool.Time2Int()
	_, err = msql.RawExec(define.Postgres, `INSERT INTO `+define.TableWorkspaceMembers+`
		(admin_id, workspace_id, user_id, role_id, joined_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5, $5)
		ON CONFLICT (workspace_id, user_id) DO UPDATE SET role_id = $4, updated_at = $5`,
		tx, adminId, wsId, userId, roleId, now)
	if err != nil {
		logs.Error("ensure workspace membership error: ws=%d, user=%d, err=%s", wsId, userId, err.Error())
		return err
	}
	const updateQuery = `UPDATE ` + define.TableWorkspaceAccessRequests + ` SET status = 'joined', updated_at = $1
		WHERE admin_id = $2 AND workspace_id = $3 AND user_id = $4 AND status = 'pending'`
	_, err = msql.RawExec(define.Postgres, updateQuery, tx, now, adminId, wsId, userId)
	if err != nil {
		logs.Error("sql:%s, err:%s", updateQuery, err.Error())
	}
	return err
}

// ensureWorkspaceMembershipIfMissing adds the user to the workspace as a plain project member when
// they are not a member yet, leaving an existing membership and its role untouched.
func ensureWorkspaceMembershipIfMissing(tx *sql.Tx, adminId, wsId, userId int) error {
	roleId, err := workspacePresetRoleId(adminId, wsId, "perm_role_project_member")
	if err != nil {
		return err
	}
	now := tool.Time2Int()
	_, err = msql.RawExec(define.Postgres, `INSERT INTO `+define.TableWorkspaceMembers+`
		(admin_id, workspace_id, user_id, role_id, joined_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5, $5)
		ON CONFLICT (workspace_id, user_id) DO NOTHING`,
		tx, adminId, wsId, userId, roleId, now)
	if err != nil {
		logs.Error("ensure workspace membership error: ws=%d, user=%d, err=%s", wsId, userId, err.Error())
		return err
	}
	const updateQuery = `UPDATE ` + define.TableWorkspaceAccessRequests + ` SET status = 'joined', updated_at = $1
		WHERE admin_id = $2 AND workspace_id = $3 AND user_id = $4 AND status = 'pending'`
	_, err = msql.RawExec(define.Postgres, updateQuery, tx, now, adminId, wsId, userId)
	if err != nil {
		logs.Error("sql:%s, err:%s", updateQuery, err.Error())
	}
	return err
}

// transferWorkspaceOwnership moves one workspace's default project admin to another user inside tx:
// the receiver joins as project admin and the previous owner, if still a member, drops to project member.
func transferWorkspaceOwnership(tx *sql.Tx, adminId, wsId, fromUserId, toUserId int, demotePrevious bool) error {
	if err := ensureWorkspaceMembership(tx, adminId, wsId, toUserId, "perm_role_project_admin"); err != nil {
		return err
	}
	if demotePrevious && fromUserId > 0 {
		memberRoleId, err := workspacePresetRoleId(adminId, wsId, "perm_role_project_member")
		if err != nil {
			return err
		}
		if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableWorkspaceMembers+`
			SET role_id = $1, updated_at = $2 WHERE admin_id = $3 AND workspace_id = $4 AND user_id = $5`,
			tx, memberRoleId, tool.Time2Int(), adminId, wsId, fromUserId); err != nil {
			logs.Error("demote previous project owner error: ws=%d, err=%s", wsId, err.Error())
			return err
		}
	}
	return setWorkspaceOwner(tx, adminId, wsId, toUserId)
}

// TransferWorkspaceOwnerModel describes the transfer-default-project-admin use case.
type TransferWorkspaceOwnerModel struct {
	AdminId        int
	WorkspaceId    int
	OperatorUserId int
	TargetUserId   int
}

// TransferWorkspaceOwner hands a project's default project admin role to another project member.
// Allowed for the current project owner and for the tenant owner, who is the only fallback when the
// project owner is unavailable. The previous owner stays in the project as a plain project member.
func TransferWorkspaceOwner(model TransferWorkspaceOwnerModel) error {
	currentOwnerId, err := GetWorkspaceOwnerId(model.AdminId, model.WorkspaceId)
	if err != nil {
		return err
	}
	if model.OperatorUserId != currentOwnerId && !IsTenantOwner(model.AdminId, model.OperatorUserId) {
		return ErrOwnerTransferForbidden
	}
	if model.TargetUserId <= 0 || model.TargetUserId == currentOwnerId {
		return ErrOwnerTransferTarget
	}
	// The dialog picks from the project member list, so the target must already be a member.
	if !IsWorkspaceMember(model.AdminId, model.TargetUserId, cast.ToString(model.WorkspaceId)) {
		return ErrOwnerTransferTarget
	}
	valid, err := transferableTenantMember(model.AdminId, model.TargetUserId)
	if err != nil {
		return err
	}
	if !valid {
		return ErrOwnerTransferTarget
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin transfer workspace owner error: %s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err = transferWorkspaceOwnership(tx, model.AdminId, model.WorkspaceId, currentOwnerId, model.TargetUserId, true); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		logs.Error("commit transfer workspace owner error: %s", err.Error())
		return err
	}
	return nil
}

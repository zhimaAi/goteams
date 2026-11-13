// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"errors"
	"fmt"
	"sort"
	"strconv"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/permission"

	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

var (
	// ErrProjectRoleCopyTargetInvalid indicates an empty, malformed, or source-equal target workspace.
	ErrProjectRoleCopyTargetInvalid = errors.New("invalid project role copy target")
	// ErrProjectRoleCopyTargetForbidden indicates the user cannot add roles to a target workspace.
	ErrProjectRoleCopyTargetForbidden = errors.New("project role copy target forbidden")
)

// ProjectRoleCopyTargetModel describes a workspace eligible for receiving a copied role.
type ProjectRoleCopyTargetModel struct {
	Id   int
	Name string
}

// ListProjectRoleCopyTargetsModel describes the copy-target list use case.
type ListProjectRoleCopyTargetsModel struct {
	AdminId           int
	UserId            int
	SourceWorkspaceId int
	Items             []ProjectRoleCopyTargetModel
}

// CopyProjectRoleModel describes the copy-project-role use case.
type CopyProjectRoleModel struct {
	AdminId            int
	UserId             int
	SourceWorkspaceId  int
	RoleId             int
	TargetWorkspaceIds []int
}

// ListProjectRoleCopyTargets lists active workspaces where the user can add project roles.
func ListProjectRoleCopyTargets(model ListProjectRoleCopyTargetsModel) (ListProjectRoleCopyTargetsModel, error) {
	if model.AdminId <= 0 || model.UserId <= 0 || model.SourceWorkspaceId <= 0 {
		return ListProjectRoleCopyTargetsModel{}, ErrProjectRoleCopyTargetInvalid
	}

	var (
		rows []msql.Params
		err  error
	)
	if IsTenantOwner(model.AdminId, model.UserId) {
		rows, err = msql.RawValues(define.Postgres, `SELECT id, name FROM `+define.TableWorkspaces+`
			WHERE admin_id = $1 AND id <> $2 AND COALESCE(ended_at, 0) = 0 ORDER BY id DESC`, nil,
			model.AdminId, model.SourceWorkspaceId)
	} else {
		rows, err = msql.RawValues(define.Postgres, `SELECT w.id, w.name
			FROM `+define.TableWorkspaceMembers+` wm
			JOIN `+define.TableRolePermissions+` rp
				ON rp.admin_id = wm.admin_id AND rp.role_id = wm.role_id
			JOIN `+define.TableWorkspaces+` w
				ON w.admin_id = wm.admin_id AND w.id = wm.workspace_id
			WHERE wm.admin_id = $1 AND wm.user_id = $2 AND wm.workspace_id <> $3
				AND rp.permission_key = $4 AND COALESCE(w.ended_at, 0) = 0
			ORDER BY w.id DESC`, nil,
			model.AdminId, model.UserId, model.SourceWorkspaceId, "project_role:add")
	}
	if err != nil {
		logs.Error("list project role copy targets error: %s", err.Error())
		return ListProjectRoleCopyTargetsModel{}, err
	}

	model.Items = make([]ProjectRoleCopyTargetModel, 0, len(rows))
	for _, row := range rows {
		item, convErr := toProjectRoleCopyTargetModel(row)
		if convErr != nil {
			logs.Error("convert project role copy target error: %s", convErr.Error())
			return ListProjectRoleCopyTargetsModel{}, convErr
		}
		model.Items = append(model.Items, item)
	}
	return model, nil
}

// CopyProjectRole copies one custom role's name, description, and saved permissions to target workspaces.
func CopyProjectRole(model CopyProjectRoleModel) error {
	if model.AdminId <= 0 || model.UserId <= 0 || model.RoleId <= 0 {
		return ErrProjectRoleCopyTargetInvalid
	}
	targetWorkspaceIds, err := normalizeProjectRoleCopyTargetIds(model.SourceWorkspaceId, model.TargetWorkspaceIds)
	if err != nil {
		return err
	}

	targets, err := ListProjectRoleCopyTargets(ListProjectRoleCopyTargetsModel{
		AdminId:           model.AdminId,
		UserId:            model.UserId,
		SourceWorkspaceId: model.SourceWorkspaceId,
	})
	if err != nil {
		return err
	}
	allowedTargets := make(map[int]bool, len(targets.Items))
	for _, target := range targets.Items {
		allowedTargets[target.Id] = true
	}
	for _, workspaceId := range targetWorkspaceIds {
		if !allowedTargets[workspaceId] {
			return ErrProjectRoleCopyTargetForbidden
		}
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin project role copy transaction error: %s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()

	sourceRows, err := msql.RawValues(define.Postgres, `SELECT name, description, is_system
		FROM `+define.TableRoles+`
		WHERE admin_id = $1 AND workspace_id = $2 AND id = $3 AND scope = $4
		FOR UPDATE`, tx, model.AdminId, model.SourceWorkspaceId, model.RoleId, permission.ScopeProject)
	if err != nil {
		logs.Error("get source project role for copy error: %s", err.Error())
		return err
	}
	if len(sourceRows) == 0 {
		return ErrProjectRoleNotFound
	}
	isSystem, err := strconv.Atoi(sourceRows[0][`is_system`])
	if err != nil {
		convErr := fmt.Errorf("parse source project role system flag: %w", err)
		logs.Error("convert source project role for copy error: %s", convErr.Error())
		return convErr
	}
	if isSystem == 1 {
		return ErrProjectRoleSystem
	}
	roleName := sourceRows[0][`name`]
	roleDescription := sourceRows[0][`description`]

	for _, workspaceId := range targetWorkspaceIds {
		conflicts, queryErr := msql.RawValues(define.Postgres, `SELECT id FROM `+define.TableRoles+`
			WHERE admin_id = $1 AND workspace_id = $2 AND name = $3 AND scope = $4 LIMIT 1`, tx,
			model.AdminId, workspaceId, roleName, permission.ScopeProject)
		if queryErr != nil {
			logs.Error("check copied project role name error: %s", queryErr.Error())
			return queryErr
		}
		if len(conflicts) > 0 {
			return ErrProjectRoleNameExists
		}
	}

	permissionRows, err := msql.RawValues(define.Postgres, `SELECT permission_key FROM `+define.TableRolePermissions+`
		WHERE admin_id = $1 AND role_id = $2 ORDER BY id ASC`, tx, model.AdminId, model.RoleId)
	if err != nil {
		logs.Error("list source project role permissions for copy error: %s", err.Error())
		return err
	}
	permissionKeys := make([]string, 0, len(permissionRows))
	for _, row := range permissionRows {
		permissionKeys = append(permissionKeys, row[`permission_key`])
	}
	permissionKeys = filterProjectPermissionKeys(permissionKeys)

	now := tool.Time2Int()
	for _, workspaceId := range targetWorkspaceIds {
		inserted, insertErr := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableRoles+`
			(admin_id, workspace_id, name, description, is_system, scope, data_view_scope, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 0, $5, $6, $7, $7) RETURNING id`, tx,
			model.AdminId, workspaceId, roleName, roleDescription, permission.ScopeProject, DataViewScopeSelf, now)
		if insertErr != nil {
			logs.Error("insert copied project role error: %s", insertErr.Error())
			return insertErr
		}
		if len(inserted) == 0 {
			return errors.New("copied project role insert returned no row")
		}
		roleId, convErr := strconv.Atoi(inserted[0][`id`])
		if convErr != nil {
			parseErr := fmt.Errorf("parse copied project role id: %w", convErr)
			logs.Error("convert copied project role id error: %s", parseErr.Error())
			return parseErr
		}
		for _, permissionKey := range permissionKeys {
			if _, insertErr = msql.RawExec(define.Postgres, `INSERT INTO `+define.TableRolePermissions+`
				(admin_id, role_id, permission_key, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $4)`, tx,
				model.AdminId, roleId, permissionKey, now); insertErr != nil {
				logs.Error("insert copied project role permission error: %s", insertErr.Error())
				return insertErr
			}
		}
	}

	if err = tx.Commit(); err != nil {
		logs.Error("commit project role copy transaction error: %s", err.Error())
	}
	return err
}

func toProjectRoleCopyTargetModel(row msql.Params) (ProjectRoleCopyTargetModel, error) {
	id, err := strconv.Atoi(row[`id`])
	if err != nil {
		return ProjectRoleCopyTargetModel{}, fmt.Errorf("parse project role copy target id: %w", err)
	}
	return ProjectRoleCopyTargetModel{Id: id, Name: row[`name`]}, nil
}

func normalizeProjectRoleCopyTargetIds(sourceWorkspaceId int, targetWorkspaceIds []int) ([]int, error) {
	if sourceWorkspaceId <= 0 || len(targetWorkspaceIds) == 0 {
		return nil, ErrProjectRoleCopyTargetInvalid
	}
	seen := make(map[int]bool, len(targetWorkspaceIds))
	result := make([]int, 0, len(targetWorkspaceIds))
	for _, workspaceId := range targetWorkspaceIds {
		if workspaceId <= 0 || workspaceId == sourceWorkspaceId {
			return nil, ErrProjectRoleCopyTargetInvalid
		}
		if seen[workspaceId] {
			continue
		}
		seen[workspaceId] = true
		result = append(result, workspaceId)
	}
	sort.Ints(result)
	return result, nil
}

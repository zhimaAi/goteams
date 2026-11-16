// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"errors"
	"slices"
	"strconv"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/permission"

	"github.com/lib/pq"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// ErrTemplateVisibleRolesInvalid 表示可见角色不属于当前工作空间或格式无效。
var ErrTemplateVisibleRolesInvalid = errors.New("invalid template visible roles")

// TemplateVisibleToRole 按当前工作空间角色判断模板是否可用。
func TemplateVisibleToRole(visibleRoleIDs []int, roleID int) bool {
	return len(visibleRoleIDs) == 0 || (roleID > 0 && slices.Contains(visibleRoleIDs, roleID))
}

// TemplateViewerRole 读取当前有效的工作空间角色，无角色时返回 0。
func TemplateViewerRole(adminID, workspaceID, userID int) (int, error) {
	query := `SELECT r.id FROM ` + define.TableWorkspaceMembers + ` m
		JOIN ` + define.TableRoles + ` r
			ON r.admin_id = m.admin_id AND r.workspace_id = m.workspace_id AND r.id = m.role_id
		WHERE m.admin_id = $1 AND m.workspace_id = $2 AND m.user_id = $3 AND r.scope = $4`
	rows, err := msql.RawValues(define.Postgres, query, nil, adminID, workspaceID, userID, permission.ScopeProject)
	if err != nil {
		logs.Error("load template viewer role failed, sql:%s, err:%s", query, err.Error())
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return strconv.Atoi(rows[0][`id`])
}

// TemplateRoleOption 是可见角色选择器使用的工作空间角色。
type TemplateRoleOption struct {
	ID   int
	Name string
}

// ListTemplateRoleOptions 返回指定工作空间内可供模板设置选择的角色。
func ListTemplateRoleOptions(adminID, workspaceID int) ([]TemplateRoleOption, error) {
	query := `SELECT id, name FROM ` + define.TableRoles + `
		WHERE admin_id = $1 AND workspace_id = $2 AND scope = $3 ORDER BY id`
	rows, err := msql.RawValues(define.Postgres, query, nil, adminID, workspaceID, permission.ScopeProject)
	if err != nil {
		logs.Error("list template role options failed, sql:%s, err:%s", query, err.Error())
		return nil, err
	}
	options := make([]TemplateRoleOption, 0, len(rows))
	for _, row := range rows {
		id, err := strconv.Atoi(row[`id`])
		if err != nil {
			return nil, err
		}
		options = append(options, TemplateRoleOption{ID: id, Name: row[`name`]})
	}
	return options, nil
}

func validatedTemplateVisibleRoles(tx *sql.Tx, adminID, workspaceID int, roleIDs []int) ([]int, error) {
	if len(roleIDs) > 1000 {
		return nil, ErrTemplateVisibleRolesInvalid
	}
	normalized := append([]int{}, roleIDs...)
	slices.Sort(normalized)
	normalized = slices.Compact(normalized)
	if len(normalized) == 0 {
		return normalized, nil
	}
	if normalized[0] <= 0 || normalized[len(normalized)-1] > 2147483647 {
		return nil, ErrTemplateVisibleRolesInvalid
	}
	query := `SELECT id FROM ` + define.TableRoles + `
		WHERE admin_id = $1 AND workspace_id = $2 AND scope = $3 AND id = ANY($4) FOR KEY SHARE`
	rows, err := msql.RawValues(define.Postgres, query, tx, adminID, workspaceID, permission.ScopeProject, pq.Array(normalized))
	if err != nil {
		logs.Error("validate template visible roles failed, sql:%s, err:%s", query, err.Error())
		return nil, err
	}
	if len(rows) != len(normalized) {
		return nil, ErrTemplateVisibleRolesInvalid
	}
	return normalized, nil
}

package business

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
	"goteams/internal/app/goteams/define"
)

// ErrNavigationLocked indicates that members cannot change the project navigation order.
var ErrNavigationLocked = errors.New("workspace navigation editing is locked")

var validNavigationKeys = map[string]bool{
	"planning": true, "requirement": true, "defect": true, "iteration": true,
	"gantt": true, "dashboard": true, "knowledge": true, "issue-invite": true,
}

// WorkspaceNavigationSettings is the effective navigation configuration for one user.
type WorkspaceNavigationSettings struct {
	MainNavOrder      []string
	NavStyle          string
	PreventMemberEdit bool
	IsAdmin           bool
}

// isProjectAdmin reports whether the user has the workspace's preset project-admin role.
// A custom role with project_info:edit is intentionally not treated as a project administrator.
func isProjectAdmin(adminId, userId, workspaceId int) bool {
	roleId, err := workspacePresetRoleId(adminId, workspaceId, "perm_role_project_admin")
	if err != nil {
		logs.Error("get navigation project admin role failed: %s", err.Error())
		return false
	}
	return cast.ToInt(GetMemberRole(adminId, userId, strconv.Itoa(workspaceId))) == roleId
}

func normalizeNavigationOrder(order []string) ([]string, error) {
	result := make([]string, 0, len(order))
	seen := map[string]bool{}
	for _, key := range order {
		if !validNavigationKeys[key] {
			return nil, fmt.Errorf("invalid navigation key")
		}
		if !seen[key] {
			seen[key] = true
			result = append(result, key)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("navigation order cannot be empty")
	}
	return result, nil
}

func normalizeNavigationStyle(style string) (string, error) {
	if style == "" {
		return "simple", nil
	}
	if style != "simple" && style != "minimal" {
		return "", fmt.Errorf("invalid navigation style")
	}
	return style, nil
}

func defaultNavigationOrder() []string {
	return []string{"planning", "requirement", "defect", "iteration", "gantt", "knowledge", "issue-invite"}
}

func decodeNavigationOrder(raw string) []string {
	var order []string
	if json.Unmarshal([]byte(raw), &order) != nil {
		return defaultNavigationOrder()
	}
	normalized, err := normalizeNavigationOrder(order)
	if err != nil {
		return defaultNavigationOrder()
	}
	return normalized
}

// GetWorkspaceNavigationSettings loads the project policy and the user's effective override.
func GetWorkspaceNavigationSettings(adminId, userId, workspaceId int) (WorkspaceNavigationSettings, error) {
	project, err := msql.Model(define.TableWorkspaceNavigationSettings, define.Postgres).Where2([][]string{{"admin_id", strconv.Itoa(adminId)}, {"workspace_id", strconv.Itoa(workspaceId)}}).Field("main_nav_order, nav_style, prevent_member_edit").Find()
	if err != nil {
		logs.Error("get navigation settings failed: %s", err.Error())
		return WorkspaceNavigationSettings{}, err
	}
	settings := WorkspaceNavigationSettings{MainNavOrder: defaultNavigationOrder(), NavStyle: "simple"}
	if len(project) > 0 {
		settings.MainNavOrder = decodeNavigationOrder(project["main_nav_order"])
		if project["nav_style"] == "minimal" {
			settings.NavStyle = "minimal"
		}
		settings.PreventMemberEdit = project["prevent_member_edit"] == "1"
	}
	settings.IsAdmin = isProjectAdmin(adminId, userId, workspaceId)
	// Project administrators always read the project policy. Personal snapshots
	// belong to ordinary members and must never change an administrator's view.
	if !settings.IsAdmin && !settings.PreventMemberEdit {
		member, e := msql.Model(define.TableUserWorkspaceNavigationSettings, define.Postgres).Where2([][]string{{"admin_id", strconv.Itoa(adminId)}, {"workspace_id", strconv.Itoa(workspaceId)}, {"user_id", strconv.Itoa(userId)}}).Field("main_nav_order, nav_style").Find()
		if e != nil {
			logs.Error("get member navigation settings failed: %s", e.Error())
			return WorkspaceNavigationSettings{}, e
		}
		if len(member) > 0 {
			settings.MainNavOrder = decodeNavigationOrder(member["main_nav_order"])
			if member["nav_style"] != "" {
				settings.NavStyle = member["nav_style"]
			}
		}
	}
	return settings, nil
}

// SaveMemberNavigationSettings stores the requesting member's navigation override.
func SaveMemberNavigationSettings(adminId, userId, workspaceId int, order []string, style string) error {
	normalized, err := normalizeNavigationOrder(order)
	if err != nil {
		return err
	}
	style, err = normalizeNavigationStyle(style)
	if err != nil {
		return err
	}
	project, err := GetWorkspaceNavigationSettings(adminId, userId, workspaceId)
	if err != nil {
		return err
	}
	if project.PreventMemberEdit && !project.IsAdmin {
		return ErrNavigationLocked
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		logs.Error("encode member navigation settings failed: %s", err.Error())
		return err
	}
	now := tool.Time2Int()
	result, err := msql.RawExec(define.Postgres, `UPDATE user_workspace_navigation_settings SET main_nav_order=$1, nav_style=$2, updated_at=$3 WHERE admin_id=$4 AND workspace_id=$5 AND user_id=$6`, nil, string(raw), style, now, adminId, workspaceId, userId)
	if err == nil {
		var affected int64
		affected, err = result.RowsAffected()
		if err == nil && affected == 0 {
			_, err = msql.RawExec(define.Postgres, `INSERT INTO user_workspace_navigation_settings (admin_id,workspace_id,user_id,main_nav_order,nav_style,updated_at,created_at) VALUES ($1,$2,$3,$4,$5,$6,$6)`, nil, adminId, workspaceId, userId, string(raw), style, now)
		}
	}
	if err != nil {
		logs.Error("save member navigation settings failed: %s", err.Error())
	}
	return err
}

// SaveWorkspaceNavigationSettings stores the project policy and optionally applies it to every member.
func SaveWorkspaceNavigationSettings(adminId, userId, workspaceId int, order []string, style string, prevent, applyAll bool) error {
	if !isProjectAdmin(adminId, userId, workspaceId) {
		return ErrNavigationLocked
	}
	normalized, err := normalizeNavigationOrder(order)
	if err != nil {
		return err
	}
	style, err = normalizeNavigationStyle(style)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		logs.Error("encode workspace navigation settings failed: %s", err.Error())
		return err
	}
	now := tool.Time2Int()
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin navigation settings transaction failed: %s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Preserve the previous project policy for members without a personal snapshot.
	// A normal administrator edit must not implicitly become an "apply to all" action.
	previousRaw, previousStyle := mustNavigationDefaults()
	previous, queryErr := msql.RawValues(define.Postgres, `SELECT main_nav_order,nav_style FROM workspace_navigation_settings WHERE admin_id=$1 AND workspace_id=$2 FOR UPDATE`, tx, adminId, workspaceId)
	if queryErr != nil {
		logs.Error("load previous workspace navigation settings failed: %s", queryErr.Error())
		return queryErr
	}
	if len(previous) > 0 {
		previousRaw = previous[0]["main_nav_order"]
		if previous[0]["nav_style"] != "" {
			previousStyle = previous[0]["nav_style"]
		}
	}
	result, err := msql.RawExec(define.Postgres, `UPDATE workspace_navigation_settings SET main_nav_order=$1,nav_style=$2,prevent_member_edit=$3,updated_by=$4,updated_at=$5 WHERE admin_id=$6 AND workspace_id=$7`, tx, string(raw), style, boolInt(prevent), userId, now, adminId, workspaceId)
	if err != nil {
		logs.Error("save workspace navigation settings failed: %s", err.Error())
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		logs.Error("check workspace navigation settings update failed: %s", err.Error())
		return err
	}
	if affected == 0 {
		_, err = msql.RawExec(define.Postgres, `INSERT INTO workspace_navigation_settings (admin_id,workspace_id,main_nav_order,nav_style,prevent_member_edit,updated_by,updated_at,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$7)`, tx, adminId, workspaceId, string(raw), style, boolInt(prevent), userId, now)
		if err != nil {
			logs.Error("insert workspace navigation settings failed: %s", err.Error())
			return err
		}
	}
	if applyAll {
		_, err = msql.RawExec(define.Postgres, `UPDATE user_workspace_navigation_settings SET main_nav_order=$1,nav_style=$2,updated_at=$3 WHERE admin_id=$4 AND workspace_id=$5 AND user_id IN (SELECT wm.user_id FROM workspace_members wm WHERE wm.admin_id=$4 AND wm.workspace_id=$5 AND wm.user_id<>$6)`, tx, string(raw), style, now, adminId, workspaceId, userId)
		if err == nil {
			_, err = msql.RawExec(define.Postgres, `INSERT INTO user_workspace_navigation_settings (admin_id,workspace_id,user_id,main_nav_order,nav_style,updated_at,created_at) SELECT $1,$2,wm.user_id,$3,$4,$5,$5 FROM workspace_members wm LEFT JOIN user_workspace_navigation_settings uwns ON uwns.admin_id=$1 AND uwns.workspace_id=$2 AND uwns.user_id=wm.user_id WHERE wm.admin_id=$1 AND wm.workspace_id=$2 AND wm.user_id<>$6 AND uwns.id IS NULL`, tx, adminId, workspaceId, string(raw), style, now, userId)
		}
		if err != nil {
			logs.Error("apply navigation settings to members failed: %s", err.Error())
			return err
		}
	} else {
		// Materialize the old policy only for members that have never customized or
		// previously received an applied snapshot. Existing member choices win.
		_, err = msql.RawExec(define.Postgres, `INSERT INTO user_workspace_navigation_settings (admin_id,workspace_id,user_id,main_nav_order,nav_style,updated_at,created_at) SELECT $1,$2,wm.user_id,$3,$4,$5,$5 FROM workspace_members wm LEFT JOIN user_workspace_navigation_settings uwns ON uwns.admin_id=$1 AND uwns.workspace_id=$2 AND uwns.user_id=wm.user_id WHERE wm.admin_id=$1 AND wm.workspace_id=$2 AND wm.user_id<>$6 AND uwns.id IS NULL`, tx, adminId, workspaceId, previousRaw, previousStyle, now, userId)
		if err != nil {
			logs.Error("preserve member navigation settings failed: %s", err.Error())
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit navigation settings transaction failed: %s", err.Error())
		return err
	}
	return nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func mustNavigationDefaults() (string, string) {
	raw, err := json.Marshal(defaultNavigationOrder())
	if err != nil {
		return `[]`, "simple"
	}
	return string(raw), "simple"
}

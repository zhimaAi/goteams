// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"errors"

	"goteams/internal/app/goteams/define"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// ErrWorkspaceApplicationNotFound indicates an unknown application key.
var ErrWorkspaceApplicationNotFound = errors.New(`workspace application not found`)

// WorkspaceAppDefinition describes one application card in the app market;
// Name and Description are i18n keys translated by the controller layer.
// ComingSoon marks an app whose switch is disabled in the UI ("coming soon").
type WorkspaceAppDefinition struct {
	AppKey      string
	Name        string
	Description string
	ComingSoon  bool
}

// workspaceAppDefinitions is the server-side app directory (display order as rendered).
// Planning/iteration default to enabled (see migration 20260828122233_seed_planning_iteration_apps.sql);
// defect defaults to enabled as well (see migration 20260910163114_seed_defect_app.sql) because
// it used to be a built-in capability without a switch row, and a missing row reads as disabled.
// dashboard is a coming-soon placeholder (the invite_ticket placeholder was removed because it
// duplicated the functional issue-invite app).
var workspaceAppDefinitions = []WorkspaceAppDefinition{
	{AppKey: define.AppKeyDefect, Name: `app_defect`, Description: `app_defect_desc`},
	{AppKey: define.AppKeyPlanning, Name: `app_planning`, Description: `app_planning_desc`},
	{AppKey: define.AppKeyIteration, Name: `app_iteration`, Description: `app_iteration_desc`},
	{AppKey: define.AppKeyGantt, Name: `app_gantt`, Description: `app_gantt_desc`},
	{AppKey: define.AppKeyKnowledgeBase, Name: `app_knowledge_base`, Description: `app_knowledge_base_desc`},
	{AppKey: define.AppKeyDashboard, Name: `app_dashboard`, Description: `app_dashboard_desc`},
	{AppKey: define.AppKeyIssue, Name: `app_issue_invite`, Description: `app_issue_invite_desc`},
}

// GetWorkspaceAppDefinitions returns the full app directory (code-defined).
func GetWorkspaceAppDefinitions(issueWorkspace bool) []WorkspaceAppDefinition {
	defs := make([]WorkspaceAppDefinition, 0, len(workspaceAppDefinitions))
	for _, def := range workspaceAppDefinitions {
		if workspaceAppAvailable(def.AppKey, issueWorkspace) {
			defs = append(defs, def)
		}
	}
	return defs
}

// 工单类型不提供缺陷和迭代；只过滤可用能力，不改写历史应用配置。
func workspaceAppAvailable(appKey string, issueWorkspace bool) bool {
	return !issueWorkspace || (appKey != define.AppKeyDefect && appKey != define.AppKeyIteration)
}

// IsWorkspaceAppKeyRegistered reports whether the app key exists in the app directory.
func IsWorkspaceAppKeyRegistered(appKey string) bool {
	for _, def := range workspaceAppDefinitions {
		if def.AppKey == appKey {
			return true
		}
	}
	return false
}

// IsWorkspaceAppComingSoon reports whether an app key is a coming-soon placeholder
// whose enabled state cannot be toggled yet.
func IsWorkspaceAppComingSoon(appKey string) bool {
	for _, def := range workspaceAppDefinitions {
		if def.AppKey == appKey {
			return def.ComingSoon
		}
	}
	return false
}

// WorkspaceApplicationModel is the business model of a workspace application config row.
type WorkspaceApplicationModel struct {
	AdminId     int
	WorkspaceId int
	AppKey      string
	Enabled     int
	EnabledBy   int
	EnabledAt   int64
	UpdatedAt   int64
}

// GetWorkspaceApplication returns the application config of a workspace; no row means disabled.
func GetWorkspaceApplication(adminId, workspaceId int, appKey string) (WorkspaceApplicationModel, error) {
	m := msql.Model(define.TableWorkspaceApplications, define.Postgres)
	row, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, cast.ToString(workspaceId)},
		{`app_key`, appKey},
	}).Field(`admin_id, workspace_id, app_key, enabled, enabled_by, enabled_at, updated_at`).Find()
	if err != nil {
		logs.Error("get workspace application sql:%s, err:%s", m.GetLastSql(), err.Error())
		return WorkspaceApplicationModel{}, err
	}
	if len(row) == 0 {
		return WorkspaceApplicationModel{AdminId: adminId, WorkspaceId: workspaceId, AppKey: appKey}, nil
	}
	return WorkspaceApplicationModel{
		AdminId:     cast.ToInt(row[`admin_id`]),
		WorkspaceId: cast.ToInt(row[`workspace_id`]),
		AppKey:      row[`app_key`],
		Enabled:     cast.ToInt(row[`enabled`]),
		EnabledBy:   cast.ToInt(row[`enabled_by`]),
		EnabledAt:   cast.ToInt64(row[`enabled_at`]),
		UpdatedAt:   cast.ToInt64(row[`updated_at`]),
	}, nil
}

// ListWorkspaceApplications returns the config rows of a workspace (may be fewer than the app directory).
func ListWorkspaceApplications(adminId, workspaceId int) ([]WorkspaceApplicationModel, error) {
	m := msql.Model(define.TableWorkspaceApplications, define.Postgres)
	rows, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, cast.ToString(workspaceId)},
	}).Field(`admin_id, workspace_id, app_key, enabled, enabled_by, enabled_at, updated_at`).Order(`app_key asc`).Select()
	if err != nil {
		logs.Error("list workspace applications sql:%s, err:%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	items := make([]WorkspaceApplicationModel, 0, len(rows))
	for _, row := range rows {
		items = append(items, WorkspaceApplicationModel{
			AdminId:     cast.ToInt(row[`admin_id`]),
			WorkspaceId: cast.ToInt(row[`workspace_id`]),
			AppKey:      row[`app_key`],
			Enabled:     cast.ToInt(row[`enabled`]),
			EnabledBy:   cast.ToInt(row[`enabled_by`]),
			EnabledAt:   cast.ToInt64(row[`enabled_at`]),
			UpdatedAt:   cast.ToInt64(row[`updated_at`]),
		})
	}
	return items, nil
}

// GetEnabledApplicationKeys returns the enabled app keys of a workspace, e.g. ["knowledge_base"].
func GetEnabledApplicationKeys(adminId, workspaceId int) ([]string, error) {
	apps, err := ListWorkspaceApplications(adminId, workspaceId)
	if err != nil {
		return nil, err
	}
	issueWorkspace, err := IsIssueWorkspace(adminId, workspaceId)
	if err != nil {
		return nil, err
	}
	return enabledWorkspaceApplicationKeys(apps, issueWorkspace), nil
}

func enabledWorkspaceApplicationKeys(apps []WorkspaceApplicationModel, issueWorkspace bool) []string {
	keys := make([]string, 0, len(apps))
	for _, app := range apps {
		if app.Enabled == 1 && workspaceAppAvailable(app.AppKey, issueWorkspace) {
			keys = append(keys, app.AppKey)
		}
	}
	return keys
}

// IsWorkspaceApplicationEnabled reports whether an app is enabled in a workspace.
func IsWorkspaceApplicationEnabled(adminId, workspaceId int, appKey string) (bool, error) {
	if !workspaceAppAvailable(appKey, true) {
		issueWorkspace, err := IsIssueWorkspace(adminId, workspaceId)
		if err != nil || issueWorkspace {
			return false, err
		}
	}
	app, err := GetWorkspaceApplication(adminId, workspaceId, appKey)
	if err != nil {
		return false, err
	}
	return app.Enabled == 1, nil
}

// UpdateWorkspaceApplication upserts the enabled state of a workspace application (idempotent).
func UpdateWorkspaceApplication(model WorkspaceApplicationModel) (WorkspaceApplicationModel, error) {
	workspaceM := msql.Model(define.TableWorkspaces, define.Postgres)
	workspace, err := workspaceM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)}, {`id`, cast.ToString(model.WorkspaceId)},
	}).Field(`id`).Find()
	if err != nil {
		logs.Error("check application workspace sql:%s, err:%s", workspaceM.GetLastSql(), err.Error())
		return WorkspaceApplicationModel{}, err
	}
	if len(workspace) == 0 {
		return WorkspaceApplicationModel{}, ErrWorkspaceNotFound
	}
	if model.AppKey == define.AppKeyIssue {
		return UpdateIssueWorkspaceApplication(model)
	}
	if model.AppKey == define.AppKeyDashboard {
		return UpdateDashboardWorkspaceApplication(model)
	}
	now := int64(tool.Time2Int())
	enabled := 0
	if model.Enabled == 1 {
		enabled = 1
	}
	enabledAt := model.EnabledAt
	if enabled == 1 && enabledAt == 0 {
		enabledAt = now
	}
	_, err = msql.RawExec(define.Postgres, `INSERT INTO workspace_applications
		(admin_id, workspace_id, app_key, enabled, enabled_by, enabled_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (admin_id, workspace_id, app_key) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			enabled_by = EXCLUDED.enabled_by,
			enabled_at = CASE WHEN EXCLUDED.enabled = 1 THEN EXCLUDED.enabled_at ELSE workspace_applications.enabled_at END,
			updated_at = EXCLUDED.updated_at`, nil,
		model.AdminId, model.WorkspaceId, model.AppKey, enabled, model.EnabledBy, enabledAt, now)
	if err != nil {
		logs.Error("upsert workspace application failed, err:%s", err.Error())
		return WorkspaceApplicationModel{}, err
	}
	return WorkspaceApplicationModel{
		AdminId: model.AdminId, WorkspaceId: model.WorkspaceId, AppKey: model.AppKey,
		Enabled: enabled, EnabledBy: model.EnabledBy, EnabledAt: enabledAt, UpdatedAt: now,
	}, nil
}

// UpdateDashboardWorkspaceApplication atomically changes the dashboard application switch and
// creates the initial dashboard plus project announcement on first enable. Disabling only changes
// the switch, preserving all dashboard data for a later re-enable.
func UpdateDashboardWorkspaceApplication(model WorkspaceApplicationModel) (WorkspaceApplicationModel, error) {
	enabled := 0
	if model.Enabled == 1 {
		enabled = 1
	}
	now := int64(tool.Time2Int())
	enabledAt := model.EnabledAt
	if enabled == 1 && enabledAt == 0 {
		enabledAt = now
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin dashboard application transaction failed, err:%s", err.Error())
		return WorkspaceApplicationModel{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if enabled == 1 {
		if err = EnsureDefaultDashboardForWorkspaceTx(tx, model.AdminId, model.WorkspaceId, model.EnabledBy); err != nil {
			logs.Error("ensure default dashboard failed, err:%s", err.Error())
			return WorkspaceApplicationModel{}, err
		}
	}

	if _, err = msql.RawExec(define.Postgres, `INSERT INTO `+define.TableWorkspaceApplications+`
		(admin_id, workspace_id, app_key, enabled, enabled_by, enabled_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (admin_id, workspace_id, app_key) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			enabled_by = EXCLUDED.enabled_by,
			enabled_at = CASE WHEN EXCLUDED.enabled = 1 THEN EXCLUDED.enabled_at ELSE `+define.TableWorkspaceApplications+`.enabled_at END,
			updated_at = EXCLUDED.updated_at`, tx,
		model.AdminId, model.WorkspaceId, model.AppKey, enabled, model.EnabledBy, enabledAt, now); err != nil {
		logs.Error("upsert dashboard application failed, err:%s", err.Error())
		return WorkspaceApplicationModel{}, err
	}

	if err = tx.Commit(); err != nil {
		logs.Error("commit dashboard application transaction failed, err:%s", err.Error())
		return WorkspaceApplicationModel{}, err
	}

	return WorkspaceApplicationModel{
		AdminId: model.AdminId, WorkspaceId: model.WorkspaceId, AppKey: model.AppKey,
		Enabled: enabled, EnabledBy: model.EnabledBy, EnabledAt: enabledAt, UpdatedAt: now,
	}, nil
}

// SeedDefectApplication enables the defect application for a workspace. Defect defaults
// to enabled per the product rules, and a missing row reads as disabled, so every newly
// created workspace must be seeded explicitly; existing ones are backfilled by migration.
// enabled_by stays 0 to mark a system-seeded switch instead of a user action.
func SeedDefectApplication(adminId, workspaceId int) error {
	_, err := UpdateWorkspaceApplication(WorkspaceApplicationModel{
		AdminId: adminId, WorkspaceId: workspaceId, AppKey: define.AppKeyDefect, Enabled: 1,
	})
	return err
}

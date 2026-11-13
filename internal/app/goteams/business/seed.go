// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/permission"
	"goteams/internal/pkg/lib_i18n"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// ======================== Preset Data Definitions ========================
// These define the default workflow configuration for new workspaces.

type presetStatus struct {
	TypeKey   string
	Key       string
	Name      string
	Color     string
	SortOrder int
	IsDefault int
}

type presetPriority struct {
	TypeKey   string
	Key       string
	Name      string
	Color     string
	SortOrder int
	IsDefault int
}

var presetStatuses = []presetStatus{
	{define.TypeKeyRequirement, define.StatusPendingPlan, lib_i18n.WrapKey("workitem_status_pending_plan"), "#8c8c8c", 1, 1},
	{define.TypeKeyRequirement, define.StatusPendingAssign, lib_i18n.WrapKey("workitem_status_pending_assign"), "#0d68ff", 2, 1},
	{define.TypeKeyRequirement, define.StatusInProgress, lib_i18n.WrapKey("workitem_status_in_progress"), "#fa8c16", 3, 1},
	{define.TypeKeyRequirement, define.StatusDone, lib_i18n.WrapKey("workitem_status_done"), "#52c41a", 4, 1},
	{define.TypeKeyRequirement, define.StatusClosed, lib_i18n.WrapKey("workitem_status_closed"), "#8c8c8c", 5, 1},
	{define.TypeKeyRequirement, define.StatusRejected, lib_i18n.WrapKey("workitem_status_rejected"), "#ff4d4f", 6, 1},
	{define.TypeKeyDefect, define.StatusPendingPlan, lib_i18n.WrapKey("workitem_status_pending_plan"), "#8c8c8c", 1, 1},
	{define.TypeKeyDefect, define.StatusPendingAssign, lib_i18n.WrapKey("workitem_status_pending_assign"), "#0d68ff", 2, 1},
	{define.TypeKeyDefect, define.StatusInProgress, lib_i18n.WrapKey("workitem_status_in_progress"), "#fa8c16", 3, 1},
	{define.TypeKeyDefect, define.StatusDone, lib_i18n.WrapKey("workitem_status_resolved"), "#52c41a", 4, 1},
	{define.TypeKeyDefect, define.StatusClosed, lib_i18n.WrapKey("workitem_status_closed"), "#8c8c8c", 5, 1},
	{define.TypeKeyDefect, define.StatusRejected, lib_i18n.WrapKey("workitem_status_rejected"), "#ff4d4f", 6, 1},
	// Planning statuses: To Be Planned → Designed → Implemented → Accepted → Closed.
	{define.TypeKeyPlanning, define.StatusPlanned, lib_i18n.WrapKey("workitem_status_planned"), "#bfbfbf", 1, 1},
	{define.TypeKeyPlanning, define.StatusDesigned, lib_i18n.WrapKey("workitem_status_designed"), "#faad14", 2, 1},
	{define.TypeKeyPlanning, define.StatusImplemented, lib_i18n.WrapKey("workitem_status_implemented"), "#1677ff", 3, 1},
	{define.TypeKeyPlanning, define.StatusAccepted, lib_i18n.WrapKey("workitem_status_accepted"), "#52c41a", 4, 1},
	{define.TypeKeyPlanning, define.StatusPlanningClosed, lib_i18n.WrapKey("workitem_status_closed"), "#8c8c8c", 5, 1},
}

var presetPriorities = []presetPriority{
	{define.TypeKeyRequirement, "urgent", lib_i18n.WrapKey("workitem_priority_urgent"), "#e54545", 1, 0},
	{define.TypeKeyRequirement, "high", lib_i18n.WrapKey("workitem_priority_high"), "#ff8c00", 2, 0},
	{define.TypeKeyRequirement, "medium", lib_i18n.WrapKey("workitem_priority_medium"), "#2d8cf0", 3, 0},
	{define.TypeKeyRequirement, "low", lib_i18n.WrapKey("workitem_priority_low"), "#808695", 4, 0},
	{define.TypeKeyDefect, "urgent", lib_i18n.WrapKey("workitem_priority_urgent"), "#e54545", 1, 0},
	{define.TypeKeyDefect, "high", lib_i18n.WrapKey("workitem_priority_high"), "#ff8c00", 2, 0},
	{define.TypeKeyDefect, "medium", lib_i18n.WrapKey("workitem_priority_medium"), "#2d8cf0", 3, 0},
	{define.TypeKeyDefect, "low", lib_i18n.WrapKey("workitem_priority_low"), "#808695", 4, 0},
	// Planning priorities reuse the requirement/defect colors so that "urgent" is always the same color across the system.
	// key 沿用同一套稳定键，去重按 (type_key, key) 区分，不会与需求/缺陷冲突。
	{define.TypeKeyPlanning, "urgent", lib_i18n.WrapKey("workitem_priority_urgent"), "#e54545", 1, 0},
	{define.TypeKeyPlanning, "high", lib_i18n.WrapKey("workitem_priority_high"), "#ff8c00", 2, 0},
	{define.TypeKeyPlanning, "medium", lib_i18n.WrapKey("workitem_priority_medium"), "#2d8cf0", 3, 0},
	{define.TypeKeyPlanning, "low", lib_i18n.WrapKey("workitem_priority_low"), "#808695", 4, 0},
}

// ======================== Enterprise Role Seeding (per-tenant) ========================

// SeedEnterpriseRoles creates enterprise-level roles (enterprise_admin + enterprise_member)
// and their permissions for the given tenant admin.
// Idempotent: skips roles that already exist.
func SeedEnterpriseRoles(adminId int) (adminRoleId, memberRoleId int, err error) {
	now := tool.Time2Int()

	adminRoleId, err = ensureEnterpriseRole(adminId, lib_i18n.WrapKey("perm_role_enterprise_admin"), lib_i18n.WrapKey("role_desc_all"), now)
	if err != nil {
		return 0, 0, err
	}
	memberRoleId, err = ensureEnterpriseRole(adminId, lib_i18n.WrapKey("perm_role_enterprise_member"), lib_i18n.WrapKey("role_desc_enterprise_member"), now)
	if err != nil {
		return 0, 0, err
	}

	// Assign permissions from preset definitions.
	for _, preset := range permission.PresetRoles {
		if preset.Scope != permission.ScopeEnterprise {
			continue
		}
		var roleId int
		switch preset.Name {
		case "perm_role_enterprise_admin":
			roleId = adminRoleId
		case "perm_role_enterprise_member":
			roleId = memberRoleId
		default:
			continue
		}
		for _, permKey := range preset.Permissions {
			if err := ensureRolePermission(adminId, roleId, permKey, now); err != nil {
				logs.Error("assign enterprise permission [%s] to role [%d] failed: %s", permKey, roleId, err.Error())
			}
		}
	}

	logs.Info("enterprise roles seeded for admin_id=%d: admin=%d, member=%d", adminId, adminRoleId, memberRoleId)
	return adminRoleId, memberRoleId, nil
}

func ensureEnterpriseRole(adminId int, name string, description string, now int) (int, error) {
	viewScope := TeamWorkViewScopeSelf
	if name == lib_i18n.WrapKey("perm_role_enterprise_admin") {
		viewScope = TeamWorkViewScopeAll
	}
	queryM := msql.Model(define.TableRoles, define.Postgres)
	exist, err := queryM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`name`, name},
		{`scope`, permission.ScopeEnterprise},
		{`is_system`, `1`},
	}).Value(`id`)
	if err != nil {
		logs.Error("ensureEnterpriseRole query error: sql=%s, err=%s", queryM.GetLastSql(), err.Error())
		return 0, err
	}
	if len(exist) > 0 {
		updateM := msql.Model(define.TableRoles, define.Postgres)
		if _, err := updateM.Where2([][]string{
			{`admin_id`, cast.ToString(adminId)},
			{`id`, exist},
		}).Update(msql.Datas{
			`description`:          description,
			`team_work_view_scope`: viewScope,
			`updated_at`:           now,
		}); err != nil {
			logs.Error("ensureEnterpriseRole update description error: sql=%s, err=%s", updateM.GetLastSql(), err.Error())
		}
		return cast.ToInt(exist), nil
	}

	insertM := msql.Model(define.TableRoles, define.Postgres)
	id, err := insertM.Insert(msql.Datas{
		`admin_id`:             adminId,
		`workspace_id`:         0,
		`name`:                 name,
		`description`:          description,
		`is_system`:            1,
		`scope`:                permission.ScopeEnterprise,
		`team_work_view_scope`: viewScope,
		`created_at`:           now,
		`updated_at`:           now,
	}, `id`)
	if err != nil {
		logs.Error("ensureEnterpriseRole insert error: sql=%s, err=%s", insertM.GetLastSql(), err.Error())
		return 0, err
	}
	logs.Info("ensureEnterpriseRole inserted: sql=%s, returnedId=%v, adminId=%d", insertM.GetLastSql(), id, adminId)
	return cast.ToInt(id), nil
}

func ensureRolePermission(adminId int, roleId int, permKey string, now int) error {
	queryM := msql.Model(define.TableRolePermissions, define.Postgres)
	exist, err := queryM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`role_id`, cast.ToString(roleId)},
		{`permission_key`, permKey},
	}).Value(`id`)
	if err != nil {
		logs.Error("ensureRolePermission query error: sql=%s, err=%s", queryM.GetLastSql(), err.Error())
		return err
	}
	if len(exist) > 0 {
		return nil
	}
	insertM := msql.Model(define.TableRolePermissions, define.Postgres)
	_, err = insertM.Insert(msql.Datas{
		`admin_id`:       adminId,
		`role_id`:        roleId,
		`permission_key`: permKey,
		`created_at`:     now,
		`updated_at`:     now,
	})
	if err != nil {
		logs.Error("ensureRolePermission insert error: sql=%s, err=%s", insertM.GetLastSql(), err.Error())
	}
	return err
}

// ======================== Helpers ========================

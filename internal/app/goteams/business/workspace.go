// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sort"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/permission"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"
	"goteams/internal/pkg/lib_storage"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

var ErrWorkspaceSlugExists = errors.New("workspace slug already exists")

// ErrWorkspaceNotFound is returned when the workspace does not exist in the current tenant.
var ErrWorkspaceNotFound = errors.New("workspace not found")

// ErrWorkspaceOrderInvalid is returned when a project order contains unknown or duplicate IDs.
var ErrWorkspaceOrderInvalid = errors.New("invalid workspace order")

// ErrWorkspaceOrderForbidden is returned when a user tries to apply an order to all members.
var ErrWorkspaceOrderForbidden = errors.New("workspace order permission denied")

const maxWorkspaceIconSize int64 = 100 * 1024

var (
	ErrWorkspaceIconInvalid  = errors.New(`invalid workspace icon`)
	ErrWorkspaceIconTooLarge = errors.New(`workspace icon too large`)
	ErrWorkspaceIconNotFound = errors.New(`workspace icon not found`)
)

// WorkspaceModel describes the workspace business data used by project lists.
type WorkspaceModel struct {
	Id              int
	WorkspaceType   string
	Name            string
	Slug            string
	Description     string
	Color           string
	Icon            string
	ActiveVersionId int
	CreatedBy       int
	SortOrder       int
	CreatedAt       int64
	UpdatedAt       int64
}

// ListWorkspacesModel describes the list-workspaces use case.
type ListWorkspacesModel struct {
	AdminId int
	UserId  int
	Items   []WorkspaceModel
}

// listWorkspaceFields are the workspace columns required by the project list, avoiding SELECT *.
const listWorkspaceFields = `workspaces.id, workspaces.workspace_type, workspaces.name, workspaces.slug, workspaces.description, workspaces.color, ` +
	`workspaces.icon, workspaces.active_version_id, workspaces.created_by, workspaces.sort_order, workspaces.created_at, workspaces.updated_at`

// toWorkspaceModel converts a workspace row into the business model.
func toWorkspaceModel(row msql.Params) (WorkspaceModel, error) {
	if row[`workspace_type`] != "" {
		if err := validate.ValidateWorkspaceType(row[`workspace_type`]); err != nil {
			return WorkspaceModel{}, err
		}
	}
	id, err := metadataRowInt(row, `id`)
	if err != nil {
		return WorkspaceModel{}, err
	}
	activeVersionId, err := metadataRowInt(row, `active_version_id`)
	if err != nil {
		return WorkspaceModel{}, err
	}
	createdBy, err := metadataRowInt(row, `created_by`)
	if err != nil {
		return WorkspaceModel{}, err
	}
	sortOrder, err := metadataRowInt(row, `sort_order`)
	if err != nil {
		return WorkspaceModel{}, err
	}
	createdAt, err := metadataRowInt64(row, `created_at`)
	if err != nil {
		return WorkspaceModel{}, err
	}
	updatedAt, err := metadataRowInt64(row, `updated_at`)
	if err != nil {
		return WorkspaceModel{}, err
	}
	return WorkspaceModel{
		WorkspaceType: row[`workspace_type`],
		Id:            id, Name: row[`name`], Slug: row[`slug`], Description: row[`description`], Color: row[`color`],
		Icon: row[`icon`], ActiveVersionId: activeVersionId, CreatedBy: createdBy,
		SortOrder: sortOrder, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}

// ListWorkspaces returns the workspaces visible to the user in the sidebar (isolated by admin_id).
// Ended projects are hidden here; they stay reachable by URL and are listed in enterprise project management.
// The tenant super admin sees every workspace of the tenant; members only see the ones they joined.
func ListWorkspaces(model ListWorkspacesModel) (ListWorkspacesModel, error) {
	m := msql.Model(define.TableWorkspaces, define.Postgres)
	query := m.Where(`workspaces.admin_id`, cast.ToString(model.AdminId)).
		Where(`COALESCE(workspaces.ended_at, 0) = 0`)
	if !IsTenantOwner(model.AdminId, model.UserId) {
		m = msql.Model(define.TableWorkspaceMembers, define.Postgres)
		query = m.Where2([][]string{
			{`workspace_members.user_id`, cast.ToString(model.UserId)},
			{`workspace_members.admin_id`, cast.ToString(model.AdminId)},
		}).Join(define.TableWorkspaces, `workspace_members.workspace_id=workspaces.id`, `left`).
			Where(`COALESCE(workspaces.ended_at, 0) = 0`)
	}
	list, err := query.Field(listWorkspaceFields).
		Order(`workspaces.sort_order asc, workspaces.id desc`).
		Select()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return ListWorkspacesModel{}, err
	}
	model.Items = make([]WorkspaceModel, 0, len(list))
	for _, row := range list {
		item, err := toWorkspaceModel(row)
		if err != nil {
			logs.Error("convert workspace row failed: %s", err.Error())
			return ListWorkspacesModel{}, err
		}
		model.Items = append(model.Items, item)
	}
	order, err := workspaceOrder(model.AdminId, model.UserId)
	if err != nil {
		return ListWorkspacesModel{}, err
	}
	model.Items = sortWorkspaceModels(model.Items, order)
	return model, nil
}

func workspaceOrder(adminId, userId int) ([]int, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT user_id, workspace_order FROM `+define.TableUserWorkspaceOrderSettings+`
		WHERE admin_id = $1 AND user_id IN (0, $2) ORDER BY user_id DESC`, nil, adminId, userId)
	if err != nil {
		logs.Error("load workspace order failed, err:%s", err.Error())
		return nil, err
	}
	for _, row := range rows {
		var order []int
		if err := json.Unmarshal([]byte(row[`workspace_order`]), &order); err == nil && len(order) > 0 {
			return order, nil
		}
	}
	return nil, nil
}

func sortWorkspaceModels(items []WorkspaceModel, order []int) []WorkspaceModel {
	rank := make(map[int]int, len(order))
	for index, id := range order {
		if _, exists := rank[id]; !exists {
			rank[id] = index
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		left, leftFound := rank[items[i].Id]
		right, rightFound := rank[items[j].Id]
		if leftFound != rightFound {
			return leftFound
		}
		if !leftFound {
			return false
		}
		return left < right
	})
	return items
}

// GetWorkspaceById returns workspace details (admin_id isolated).
func GetWorkspaceById(adminId int, wsId string) (msql.Params, error) {
	m := msql.Model(define.TableWorkspaces, define.Postgres)
	return m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, wsId},
	}).Find()
}

// CreateWorkspaceModel 保存创建空间时选定的类型，与之后的应用开关独立。
type CreateWorkspaceModel struct {
	AdminId             int
	Name                string
	Description         string
	Slug                string
	Color               string
	CreatedBy           int
	WorkspaceType       string
	TemplateWorkspaceId int
}

// CreateWorkspace creates a workspace and returns the workspace ID.
// Also initializes preset project roles (admin + member) in the roles table for this workspace.
func CreateWorkspace(model CreateWorkspaceModel) (int, error) {
	adminId, name, description, slug, color, createdBy := model.AdminId, model.Name, model.Description, model.Slug, model.Color, model.CreatedBy
	templateTypeResolved := false
	if model.TemplateWorkspaceId > 0 {
		pkg, err := LoadTemplatePackage()
		if err != nil {
			// 模板不可用时沿用创建后的种子配置降级，类型仍保存用户选定值。
			logTemplatePackageUnavailable(`resolve workspace type`, err)
		} else {
			for _, row := range pkg.Workspaces() {
				if cast.ToInt(row[`id`]) == model.TemplateWorkspaceId && row[`workspace_type`] != "" {
					model.WorkspaceType = row[`workspace_type`]
					templateTypeResolved = true
					break
				}
			}
		}
	}
	if err := validate.ValidateWorkspaceType(model.WorkspaceType); err != nil {
		return 0, ErrWorkspaceTypeInvalid
	}
	if !templateTypeResolved {
		configured, err := workspaceTypeConfigured(model.WorkspaceType)
		if err != nil {
			return 0, err
		}
		if !configured {
			return 0, ErrWorkspaceTypeInvalid
		}
	}
	if err := CheckQuota(adminId, define.QuotaProject); err != nil {
		return 0, err
	}
	m := msql.Model(define.TableWorkspaces, define.Postgres)
	if len(slug) == 0 {
		slug = name
	}
	// Slug stays unique per tenant (workspaces_slug_tenant_key); deleted projects release their slug.
	if workspaceSlugExists(adminId, slug) {
		return 0, ErrWorkspaceSlugExists
	}
	now := tool.Time2Int()
	data := msql.Datas{
		`workspace_type`: model.WorkspaceType,
		`name`:           name,
		`description`:    description,
		`slug`:           slug,
		`color`:          color,
		`created_by`:     createdBy,
		// The creator starts as the default project admin; transferable afterwards.
		`owner_user_id`: createdBy,
		`admin_id`:      adminId,
		`created_at`:    now,
		`updated_at`:    now,
	}
	id, err := m.Insert(data, `id`)
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		if strings.Contains(err.Error(), `workspaces_slug_active_key`) {
			return 0, ErrWorkspaceSlugExists
		}
		return 0, err
	}

	wsId := cast.ToString(id)

	// Seed workflow configuration (statuses, priorities, transitions) for this workspace.
	if err := seedWorkspaceWorkflow(adminId, wsId, now); err != nil {
		logs.Error("seed workspace workflow failed: %s", err.Error())
		return 0, err
	}

	// 工单空间默认启用新 SLA，创建时固定时区并退出旧时效机制。
	if model.WorkspaceType == define.WorkspaceTypeIssue {
		if err := slaExec(nil, "INSERT INTO "+define.TableSLASettings+" (admin_id,workspace_id,enabled,timezone,activated_at,created_at,updated_at) VALUES($1,$2,TRUE,'Asia/Shanghai',$3,$3,$3) ON CONFLICT(admin_id,workspace_id) DO NOTHING", adminId, cast.ToInt(id), now); err != nil {
			return 0, err
		}
	}

	// Seed built-in field configs (requirement + defect + planning) for this workspace.
	if err := seedWorkspaceFieldConfigs(adminId, wsId, now); err != nil {
		logs.Error("seed workspace field configs failed: %s", err.Error())
		return 0, err
	}

	// Seed the default planning group (keep at least one group).
	if err := seedPlanningDefaultGroup(adminId, wsId, now); err != nil {
		logs.Error("seed planning default group failed: %s", err.Error())
		return 0, err
	}

	// Seed builtin requirement/defect list views (system views visible to every member).
	if err := seedWorkspaceViews(adminId, wsId, now); err != nil {
		logs.Error("seed workspace views failed: %s", err.Error())
		return 0, err
	}

	// Defect is an app-market module that defaults to enabled, and a missing row reads
	// as disabled, so seed it here or a new project would start with every defect entry
	// (app market card, settings menu, top tab) hidden.
	if err := SeedDefectApplication(adminId, cast.ToInt(wsId)); err != nil {
		logs.Error("seed defect application failed: %s", err.Error())
		return 0, err
	}

	// Seed preset project roles for this workspace (project_admin + project_member).
	projectAdminRoleId, _, err := seedWorkspaceRoles(adminId, wsId, now)
	if err != nil {
		logs.Error("seed workspace roles failed: %s", err.Error())
		return 0, err
	}

	// Creator automatically becomes workspace project admin.
	memberM := msql.Model(define.TableWorkspaceMembers, define.Postgres)
	_, err = memberM.Insert(msql.Datas{
		`workspace_id`: wsId,
		`user_id`:      createdBy,
		`role_id`:      cast.ToString(projectAdminRoleId),
		`admin_id`:     adminId,
		`joined_at`:    now,
		`created_at`:   now,
		`updated_at`:   now,
	})
	if err != nil {
		logs.Error("sql:%s, err:%s", memberM.GetLastSql(), err.Error())
		return 0, err
	}

	return cast.ToInt(id), nil
}

// IsIssueWorkspace 仅按已保存的项目类型识别工单空间，不推断存量空间。
func IsIssueWorkspace(adminId, workspaceId int) (bool, error) {
	m := msql.Model(define.TableWorkspaces, define.Postgres)
	row, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)}, {`id`, cast.ToString(workspaceId)},
	}).Field(`workspace_type`).Find()
	if err != nil {
		logs.Error("get workspace type sql:%s, err:%s", m.GetLastSql(), err.Error())
		return false, err
	}
	return row[`workspace_type`] == define.WorkspaceTypeIssue, nil
}

// workspaceSlugExists checks whether the project slug already exists within the same tenant.
func workspaceSlugExists(adminId int, slug string) bool {
	m := msql.Model(define.TableWorkspaces, define.Postgres)
	id, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`slug`, slug},
	}).Value(`id`)
	if err != nil {
		logs.Error("check workspace slug exists sql:%s, err:%s", m.GetLastSql(), err.Error())
		return false
	}
	return len(id) > 0
}

// seedWorkspaceRoles creates the preset project roles (admin + member) for a workspace.
// Returns the DB role IDs for project_admin and project_member.
func seedWorkspaceRoles(adminId int, wsId string, now int) (adminRoleId int, memberRoleId int, err error) {
	// Create project_admin role (data view scope defaults to all members).
	adminRoleId, err = insertWorkspaceRole(adminId, wsId, lib_i18n.WrapKey("perm_role_project_admin"), lib_i18n.WrapKey("role_desc_project_admin"), permission.ScopeProject, DataViewScopeAll, now)
	if err != nil {
		return 0, 0, err
	}

	// Create project_member role (data view scope defaults to self).
	memberRoleId, err = insertWorkspaceRole(adminId, wsId, lib_i18n.WrapKey("perm_role_project_member"), lib_i18n.WrapKey("role_desc_project_member"), permission.ScopeProject, DataViewScopeSelf, now)
	if err != nil {
		return 0, 0, err
	}

	// Assign permissions to project_admin (all project permissions).
	if err := assignRolePermissions(adminId, adminRoleId, getPresetProjectAdminPerms()); err != nil {
		return 0, 0, err
	}

	// Assign permissions to project_member (subset of project permissions).
	if err := assignRolePermissions(adminId, memberRoleId, getPresetProjectMemberPerms()); err != nil {
		return 0, 0, err
	}

	logs.Info("workspace [%s] project roles seeded: admin=%d, member=%d", wsId, adminRoleId, memberRoleId)
	return adminRoleId, memberRoleId, nil
}

// insertWorkspaceRole inserts a single role record and returns its ID.
// description is a wrapped i18n key (e.g. ${role_desc_project_admin}), translated by the API per language before returning.
// dataViewScope is the default list data view scope (all / partial / self).
func insertWorkspaceRole(adminId int, wsId string, name, description, scope string, dataViewScope string, now int) (int, error) {
	m := msql.Model(define.TableRoles, define.Postgres)
	id, err := m.Insert(msql.Datas{
		`workspace_id`:    wsId,
		`name`:            name,
		`description`:     description,
		`is_system`:       1,
		`scope`:           scope,
		`data_view_scope`: dataViewScope,
		`admin_id`:        adminId,
		`created_at`:      now,
		`updated_at`:      now,
	}, `id`)
	if err != nil {
		logs.Error("insert workspace role [%s] failed: %s", name, err.Error())
		return 0, err
	}
	return cast.ToInt(id), nil
}

// assignRolePermissions inserts role_permissions for a role (admin_id isolated).
func assignRolePermissions(adminId int, roleId int, permKeys []string) error {
	now := tool.Time2Int()
	for _, key := range permKeys {
		insertM := msql.Model(define.TableRolePermissions, define.Postgres)
		_, err := insertM.Insert(msql.Datas{
			`admin_id`:       adminId,
			`role_id`:        roleId,
			`permission_key`: key,
			`created_at`:     now,
			`updated_at`:     now,
		})
		if err != nil {
			logs.Error("assign permission [%s] to role [%d] failed: %s", key, roleId, err.Error())
			return err
		}
	}
	return nil
}

// getPresetProjectAdminPerms returns all project-level permission keys for project admin.
func getPresetProjectAdminPerms() []string {
	for _, preset := range permission.PresetRoles {
		if preset.Scope == permission.ScopeProject && preset.Name == "perm_role_project_admin" {
			return preset.Permissions
		}
	}
	return nil
}

// getPresetProjectMemberPerms returns project member permission keys.
func getPresetProjectMemberPerms() []string {
	for _, preset := range permission.PresetRoles {
		if preset.Scope == permission.ScopeProject && preset.Name == "perm_role_project_member" {
			return preset.Permissions
		}
	}
	return nil
}

// ======================== Workflow Seeding (per-workspace) ========================

// seedWorkspaceWorkflow initializes default statuses, priorities, and transitions for a workspace.
// It also writes the workspace settings (initial/final status config) based on seeded statuses.
func seedWorkspaceWorkflow(adminId int, wsId string, now int) error {
	// Track the initial status and all built-in final status IDs per type for workspaces_settings.
	initialStatusId := map[string]int{
		define.TypeKeyRequirement: 0,
		define.TypeKeyDefect:      0,
		define.TypeKeyPlanning:    0,
	}
	completedStatusIds := map[string][]int{}
	finalStatusIds := map[string][]int{
		define.TypeKeyRequirement: {},
		define.TypeKeyDefect:      {},
		define.TypeKeyPlanning:    {},
	}
	for _, status := range presetStatuses {
		id, err := ensureWorkspaceStatus(adminId, wsId, status, now)
		if err != nil {
			return err
		}
		if isInitialPresetStatus(status) {
			initialStatusId[status.TypeKey] = id
		}
		if status.Key == define.StatusDone {
			completedStatusIds[status.TypeKey] = append(completedStatusIds[status.TypeKey], id)
		}
		if isFinalPresetStatus(status) {
			finalStatusIds[status.TypeKey] = append(finalStatusIds[status.TypeKey], id)
		}
	}
	for _, priority := range presetPriorities {
		if err := ensureWorkspacePriority(adminId, wsId, priority, now); err != nil {
			return err
		}
	}
	// Write workspace settings: initial status = pending_plan, final statuses = done, closed and rejected.
	settings := msql.Datas{
		`admin_id`:                         adminId,
		`workspace_id`:                     wsId,
		`requirement_initial_status_id`:    initialStatusId[define.TypeKeyRequirement],
		`defect_initial_status_id`:         initialStatusId[define.TypeKeyDefect],
		`planning_initial_status_id`:       initialStatusId[define.TypeKeyPlanning],
		`requirement_final_status_ids`:     strings.Join(cast.ToStringSlice(finalStatusIds[define.TypeKeyRequirement]), ","),
		`defect_final_status_ids`:          strings.Join(cast.ToStringSlice(finalStatusIds[define.TypeKeyDefect]), ","),
		`planning_final_status_ids`:        strings.Join(cast.ToStringSlice(finalStatusIds[define.TypeKeyPlanning]), ","),
		`requirement_completed_status_ids`: joinIntSlice(completedStatusIds[define.TypeKeyRequirement]),
		`defect_completed_status_ids`:      joinIntSlice(completedStatusIds[define.TypeKeyDefect]),
		`created_at`:                       now,
		`updated_at`:                       now,
	}
	if _, err := msql.Model(define.TableWorkspaceSettings, define.Postgres).Insert(settings); err != nil {
		logs.Error("seed workspace settings failed: %s", err.Error())
		return err
	}
	logs.Info("workspace [%s] workflow seeded", wsId)
	return nil
}

// isInitialPresetStatus reports whether a preset status is the initial status of its type.
func isInitialPresetStatus(status presetStatus) bool {
	if status.TypeKey == define.TypeKeyPlanning {
		return status.Key == define.StatusPlanned
	}
	return status.Key == define.StatusPendingPlan
}

// isFinalPresetStatus reports whether a preset status is a terminal status of its type.
func isFinalPresetStatus(status presetStatus) bool {
	if status.TypeKey == define.TypeKeyPlanning {
		return status.Key == define.StatusAccepted || status.Key == define.StatusPlanningClosed
	}
	switch status.Key {
	case define.StatusDone, define.StatusClosed, define.StatusRejected:
		return true
	default:
		return false
	}
}

// ensureWorkspaceStatus creates a workspace-level status if it does not exist, and returns its ID.
func ensureWorkspaceStatus(adminId int, wsId string, status presetStatus, now int) (int, error) {
	queryM := msql.Model(define.TableWorkItemStatuses, define.Postgres)
	exist, err := queryM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, wsId},
		{`type_key`, status.TypeKey},
		{`name`, status.Name},
	}).Value(`id`)
	if err != nil {
		return 0, err
	}
	if len(exist) > 0 {
		return cast.ToInt(exist), nil
	}
	insertM := msql.Model(define.TableWorkItemStatuses, define.Postgres)
	id, err := insertM.Insert(msql.Datas{
		`admin_id`:     adminId,
		`workspace_id`: wsId,
		`type_key`:     status.TypeKey,
		`name`:         status.Name,
		`color`:        status.Color,
		`sort_order`:   status.SortOrder,
		`is_default`:   status.IsDefault,
		`created_at`:   now,
		`updated_at`:   now,
	}, `id`)
	if err != nil {
		return 0, err
	}
	return cast.ToInt(id), nil
}

func ensureWorkspacePriority(adminId int, wsId string, priority presetPriority, now int) error {
	queryM := msql.Model(define.TableWorkItemPriorities, define.Postgres)
	exist, err := queryM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, wsId},
		{`type_key`, priority.TypeKey},
		{`key`, priority.Key},
	}).Value(`id`)
	if err != nil {
		return err
	}
	if len(exist) > 0 {
		return nil
	}
	insertM := msql.Model(define.TableWorkItemPriorities, define.Postgres)
	_, err = insertM.Insert(msql.Datas{
		`admin_id`:     adminId,
		`workspace_id`: wsId,
		`type_key`:     priority.TypeKey,
		`key`:          priority.Key,
		`name`:         priority.Name,
		`color`:        priority.Color,
		`sort_order`:   priority.SortOrder,
		`is_default`:   priority.IsDefault,
		`created_at`:   now,
		`updated_at`:   now,
	})
	return err
}

// ResetStatusesModel describes restoring default statuses.
type ResetStatusesModel struct {
	AdminId int
	WsId    string
	TypeKey string
}

// ResetStatuses restores the statuses of the given type under a workspace to the built-in defaults.
// It clears user-created/deleted statuses of that type and re-seeds them, and resets the initial/terminal status fields for that type in workspaces_settings.
func ResetStatuses(model ResetStatusesModel) error {
	now := tool.Time2Int()
	// Delete the existing statuses of this type.
	delStatusM := msql.Model(define.TableWorkItemStatuses, define.Postgres)
	if _, err := delStatusM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WsId},
		{`type_key`, model.TypeKey},
	}).Delete(); err != nil {
		logs.Error("reset statuses delete sql:%s, err:%s", delStatusM.GetLastSql(), err.Error())
		return err
	}

	// Re-seed the built-in statuses and record the initial/terminal status IDs.
	initialStatusId := 0
	finalStatusIds := []int{}
	completedStatusIds := []int{}
	for _, status := range presetStatuses {
		if status.TypeKey != model.TypeKey {
			continue
		}
		id, err := ensureWorkspaceStatus(model.AdminId, model.WsId, status, now)
		if err != nil {
			return err
		}
		if isInitialPresetStatus(status) {
			initialStatusId = id
		}
		if status.Key == define.StatusDone {
			completedStatusIds = append(completedStatusIds, id)
		}
		if isFinalPresetStatus(status) {
			finalStatusIds = append(finalStatusIds, id)
		}
	}

	// Reset the initial/terminal status fields for this type in workspaces_settings.
	fieldInit, fieldFinal := settingsStatusFields(model.TypeKey)
	settingsM := msql.Model(define.TableWorkspaceSettings, define.Postgres)
	exist, err := settingsM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WsId},
	}).Value(`id`)
	if err != nil {
		logs.Error("reset statuses settings check sql:%s, err:%s", settingsM.GetLastSql(), err.Error())
		return err
	}
	data := msql.Datas{
		fieldInit:    cast.ToString(initialStatusId),
		fieldFinal:   joinIntSlice(finalStatusIds),
		`updated_at`: now,
	}
	if model.TypeKey != define.TypeKeyPlanning {
		data[model.TypeKey+"_completed_status_ids"] = joinIntSlice(completedStatusIds)
	}
	if len(exist) > 0 {
		if _, err = settingsM.Where2([][]string{
			{`admin_id`, cast.ToString(model.AdminId)},
			{`workspace_id`, model.WsId},
		}).Update(data); err != nil {
			logs.Error("reset statuses settings update sql:%s, err:%s", settingsM.GetLastSql(), err.Error())
			return err
		}
	} else {
		data[`admin_id`] = model.AdminId
		data[`workspace_id`] = model.WsId
		data[`created_at`] = now
		if _, err = settingsM.Insert(data); err != nil {
			logs.Error("reset statuses settings insert sql:%s, err:%s", settingsM.GetLastSql(), err.Error())
			return err
		}
	}
	logs.Info("workspace [%s] %s statuses reset to default", model.WsId, model.TypeKey)
	if err := ResetDisplaySettingStatuses(model.AdminId, model.WsId, model.TypeKey); err != nil {
		logs.Error("reset display setting statuses failed, err:%s", err.Error())
		return err
	}
	return nil
}

// settingsStatusFields returns the initial/terminal status column names for the given work item type in workspaces_settings.
func settingsStatusFields(typeKey string) (initialField, finalField string) {
	switch typeKey {
	case define.TypeKeyDefect:
		return `defect_initial_status_id`, `defect_final_status_ids`
	case define.TypeKeyPlanning:
		return `planning_initial_status_id`, `planning_final_status_ids`
	default:
		return `requirement_initial_status_id`, `requirement_final_status_ids`
	}
}

// ======================== Built-in Field Config Seeding (per-workspace) ========================

// presetField defines the default configuration of a built-in field.
type presetField struct {
	Key       string // Field key, corresponding to a requirements table column or a derived field.
	Enabled   int    // whether display is enabled by default: 1=yes
	Editable  int    // whether editable by default: 1=yes
	Locked    int    // whether locked (cannot be closed): 1=yes
	SortOrder int    // List sort order.
}

// presetFields holds the default built-in field configuration; array order is the default display order.
var requirementPresetFields = []presetField{
	{Key: `id`, Enabled: 1, Editable: 0, Locked: 1, SortOrder: 0},                 // ID: must be enabled, read-only, locked (cannot be closed), not modifiable.
	{Key: `title`, Enabled: 1, Editable: 0, Locked: 1, SortOrder: 1},              // Title: must be enabled, not editable, locked.
	{Key: `status`, Enabled: 1, Editable: 0, Locked: 1, SortOrder: 2},             // Status: must be enabled, not editable, locked (cannot be closed).
	{Key: `version`, Enabled: 1, Editable: 1, Locked: 0, SortOrder: 3},            // Iteration.
	{Key: `category`, Enabled: 1, Editable: 1, Locked: 0, SortOrder: 4},           // Requirement category.
	{Key: `priority`, Enabled: 1, Editable: 1, Locked: 0, SortOrder: 5},           // Priority.
	{Key: `planned_start_date`, Enabled: 1, Editable: 1, Locked: 0, SortOrder: 6}, // Planned start: enabled by default.
	{Key: `planned_end_date`, Enabled: 1, Editable: 1, Locked: 0, SortOrder: 7},   // Planned end: enabled by default.
	{Key: `assignee`, Enabled: 1, Editable: 1, Locked: 0, SortOrder: 8},           // Assignee.
	{Key: `created_at`, Enabled: 1, Editable: 0, Locked: 0, SortOrder: 9},         // Created at: read-only.
	{Key: `created_by`, Enabled: 1, Editable: 0, Locked: 0, SortOrder: 10},        // Created by: read-only.
	{Key: `estimated_work_hours`, Enabled: 0, Editable: 1, Locked: 0, SortOrder: 11},
	{Key: `work_progress`, Enabled: 0, Editable: 0, Locked: 0, SortOrder: 12},
	{Key: `completed_work_hours`, Enabled: 0, Editable: 1, Locked: 0, SortOrder: 13},
	{Key: `remaining_work_hours`, Enabled: 0, Editable: 0, Locked: 0, SortOrder: 14},
	{Key: `overtime_work_hours`, Enabled: 0, Editable: 0, Locked: 0, SortOrder: 15},
}

var defectPresetFields = []presetField{
	{Key: `id`, Enabled: 1, Editable: 0, Locked: 1, SortOrder: 0},
	{Key: `title`, Enabled: 1, Editable: 0, Locked: 1, SortOrder: 1},
	{Key: `status`, Enabled: 1, Editable: 0, Locked: 1, SortOrder: 2},
	{Key: `version`, Enabled: 1, Editable: 1, Locked: 0, SortOrder: 3},
	{Key: `priority`, Enabled: 1, Editable: 1, Locked: 0, SortOrder: 4},
	{Key: `planned_start_date`, Enabled: 1, Editable: 1, Locked: 0, SortOrder: 5},
	{Key: `planned_end_date`, Enabled: 1, Editable: 1, Locked: 0, SortOrder: 6},
	{Key: `assignee`, Enabled: 1, Editable: 1, Locked: 0, SortOrder: 7},
	{Key: `created_at`, Enabled: 1, Editable: 0, Locked: 0, SortOrder: 8},
	{Key: `created_by`, Enabled: 1, Editable: 0, Locked: 0, SortOrder: 9},
}

// planningPresetFields are the built-in fields for planning: row index, planning name, and status are locked (cannot be closed).
var planningPresetFields = []presetField{
	{Key: `row_index`, Enabled: 1, Editable: 0, Locked: 1, SortOrder: 0},
	{Key: `title`, Enabled: 1, Editable: 0, Locked: 1, SortOrder: 1},
	{Key: `priority`, Enabled: 1, Editable: 1, Locked: 0, SortOrder: 2},
	{Key: `related_items`, Enabled: 1, Editable: 1, Locked: 0, SortOrder: 3},
	{Key: `status`, Enabled: 1, Editable: 0, Locked: 1, SortOrder: 4},
	{Key: `assignee`, Enabled: 1, Editable: 1, Locked: 0, SortOrder: 5},
	{Key: `planned_start_date`, Enabled: 1, Editable: 1, Locked: 0, SortOrder: 6},
	{Key: `planned_end_date`, Enabled: 1, Editable: 1, Locked: 0, SortOrder: 7},
}

// presetFields indexes the default built-in field configuration by work item type.
var presetFields = map[string][]presetField{
	define.TypeKeyRequirement: requirementPresetFields,
	define.TypeKeyDefect:      defectPresetFields,
	define.TypeKeyPlanning:    planningPresetFields,
}

// seedWorkspaceFieldConfigs initializes the enabled configuration for the built-in field sets of a workspace.
func seedWorkspaceFieldConfigs(adminId int, wsId string, now int) error {
	for _, typeKey := range []string{define.TypeKeyRequirement, define.TypeKeyDefect, define.TypeKeyPlanning} {
		for _, f := range presetFields[typeKey] {
			queryM := msql.Model(define.TableWorkItemFieldConfigs, define.Postgres)
			exist, err := queryM.Where2([][]string{
				{`admin_id`, cast.ToString(adminId)},
				{`workspace_id`, wsId},
				{`type_key`, typeKey},
				{`field_key`, f.Key},
			}).Value(`id`)
			if err != nil {
				return err
			}
			if len(exist) > 0 {
				continue
			}
			insertM := msql.Model(define.TableWorkItemFieldConfigs, define.Postgres)
			_, err = insertM.Insert(msql.Datas{
				`admin_id`:     adminId,
				`workspace_id`: wsId,
				`type_key`:     typeKey,
				`field_key`:    f.Key,
				`is_enabled`:   f.Enabled,
				`is_editable`:  f.Editable,
				`is_locked`:    f.Locked,
				`sort_order`:   f.SortOrder,
				`created_at`:   now,
				`updated_at`:   now,
			})
			if err != nil {
				return err
			}
		}
	}
	logs.Info("workspace [%s] field configs seeded", wsId)
	return nil
}

// UpdateWorkspace updates workspace information (admin_id isolated).
func UpdateWorkspace(adminId int, wsId string, name, description, color string) error {
	m := msql.Model(define.TableWorkspaces, define.Postgres)
	now := tool.Time2Int()
	data := msql.Datas{
		`name`:        name,
		`description`: description,
		`color`:       color,
		`updated_at`:  now,
	}
	_, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, wsId},
	}).Update(data)
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
	}
	return err
}

// SaveWorkspaceIcon validates and saves the workspace icon, returning the relative path stored in the database.
func SaveWorkspaceIcon(adminId int, wsId string, file *multipart.FileHeader) (string, error) {
	if file == nil || file.Size <= 0 {
		return ``, ErrWorkspaceIconInvalid
	}
	if file.Size > maxWorkspaceIconSize {
		return ``, ErrWorkspaceIconTooLarge
	}

	opened, err := file.Open()
	if err != nil {
		return ``, err
	}
	defer func() { _ = opened.Close() }()

	header := make([]byte, 512)
	n, readErr := io.ReadFull(opened, header)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return ``, readErr
	}
	ext := ``
	switch http.DetectContentType(header[:n]) {
	case `image/jpeg`:
		ext = `.jpg`
	case `image/png`:
		ext = `.png`
	default:
		return ``, ErrWorkspaceIconInvalid
	}
	if _, err = opened.Seek(0, io.SeekStart); err != nil {
		return ``, err
	}

	workspace, err := GetWorkspaceById(adminId, wsId)
	if err != nil {
		return ``, err
	}
	if len(workspace) == 0 {
		return ``, ErrWorkspaceIconNotFound
	}

	storedPath, err := lib_storage.M.Upload(adminId, lib_storage.BusinessWorkspace, ext, opened)
	if err != nil {
		return ``, err
	}
	oldIcon := workspace[`icon`]
	m := msql.Model(define.TableWorkspaces, define.Postgres)
	_, err = m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, wsId},
	}).Update(msql.Datas{`icon`: storedPath, `updated_at`: tool.Time2Int()})
	if err != nil {
		_ = lib_storage.M.Delete(storedPath)
		logs.Error("update workspace icon sql:%s, err:%s", m.GetLastSql(), err.Error())
		return ``, err
	}
	removeWorkspaceIconFile(adminId, wsId, oldIcon, storedPath)
	return storedPath, nil
}

// GetWorkspaceIconPath returns the workspace icon's stored_path (for Serve).
func GetWorkspaceIconPath(adminId int, wsId string) (string, error) {
	workspace, err := GetWorkspaceById(adminId, wsId)
	if err != nil {
		return ``, err
	}
	if len(workspace) == 0 || workspace[`icon`] == `` {
		return ``, ErrWorkspaceIconNotFound
	}
	icon := workspace[`icon`]
	if !lib_storage.M.IsOwned(adminId, lib_storage.BusinessWorkspace, icon) || !lib_storage.M.Exists(icon) {
		return ``, ErrWorkspaceIconNotFound
	}
	return icon, nil
}

// DeleteWorkspaceIcon removes the workspace icon and restores the text icon.
func DeleteWorkspaceIcon(adminId int, wsId string) error {
	workspace, err := GetWorkspaceById(adminId, wsId)
	if err != nil {
		return err
	}
	if len(workspace) == 0 {
		return ErrWorkspaceIconNotFound
	}
	oldIcon := workspace[`icon`]
	if oldIcon == `` {
		return nil
	}

	m := msql.Model(define.TableWorkspaces, define.Postgres)
	_, err = m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, wsId},
	}).Update(msql.Datas{`icon`: ``, `updated_at`: tool.Time2Int()})
	if err != nil {
		logs.Error("delete workspace icon sql:%s, err:%s", m.GetLastSql(), err.Error())
		return err
	}
	removeWorkspaceIconFile(adminId, wsId, oldIcon, ``)
	return nil
}

func removeWorkspaceIconFile(adminId int, wsId string, oldIcon string, currentIcon string) {
	if oldIcon == `` || oldIcon == currentIcon {
		return
	}
	if err := lib_storage.M.Delete(oldIcon); err != nil {
		logs.Error("remove workspace icon failed, err:%s", err.Error())
	}
}

// settingsFields are the workspaces_settings query columns, to avoid SELECT *.
const settingsFields = `requirement_initial_status_id,defect_initial_status_id,planning_initial_status_id,` +
	`requirement_final_status_ids,defect_final_status_ids,planning_final_status_ids,requirement_completed_status_ids,defect_completed_status_ids,` +
	`requirement_status_transition,defect_status_transition,requirement_aging_control`

// WorkspaceSettingsModel holds the workspace initial/terminal status configuration business data.
type WorkspaceSettingsModel struct {
	AdminId                       int
	WsId                          string
	RequirementInitialStatusId    int
	DefectInitialStatusId         int
	PlanningInitialStatusId       int
	RequirementFinalStatusIds     []int
	RequirementCompletedStatusIds []int
	RequirementClosedStatusIds    []int
	DefectFinalStatusIds          []int
	DefectCompletedStatusIds      []int
	DefectClosedStatusIds         []int
	PlanningFinalStatusIds        []int
	RequirementStatusTransition   StatusTransitionFormModel
	DefectStatusTransition        StatusTransitionFormModel
	RequirementAgingControl       AgingControlForm
	SLAEnabled                    bool
	SLAActivatedAt                int64
}

// UpdateWorkspaceSettingsModel is the partial-update model for workspace initial/terminal status config; nil means the field is not updated this time.
type UpdateWorkspaceSettingsModel struct {
	AdminId                       int
	WsId                          string
	RequirementInitialStatusId    *int
	DefectInitialStatusId         *int
	PlanningInitialStatusId       *int
	RequirementFinalStatusIds     *[]int
	RequirementCompletedStatusIds *[]int
	RequirementClosedStatusIds    *[]int
	DefectFinalStatusIds          *[]int
	DefectCompletedStatusIds      *[]int
	DefectClosedStatusIds         *[]int
	PlanningFinalStatusIds        *[]int
	RequirementStatusTransition   *StatusTransitionFormModel
	DefectStatusTransition        *StatusTransitionFormModel
	RequirementAgingControl       *AgingControlForm
}

// GetWorkspaceSettings reads the workspace initial/final state config (admin_id isolation).
func GetWorkspaceSettings(model WorkspaceSettingsModel) (WorkspaceSettingsModel, error) {
	m := msql.Model(define.TableWorkspaceSettings, define.Postgres)
	row, err := m.Field(settingsFields).Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WsId},
	}).Find()
	if err != nil {
		logs.Error("get workspace settings sql:%s, err:%s", m.GetLastSql(), err.Error())
		return WorkspaceSettingsModel{}, err
	}
	if len(row) == 0 {
		model.RequirementStatusTransition = defaultStatusTransitionForm()
		model.DefectStatusTransition = defaultStatusTransitionForm()
		model.RequirementAgingControl = defaultAgingControlForm()
		return model, nil
	}
	model.RequirementInitialStatusId = cast.ToInt(row[`requirement_initial_status_id`])
	model.DefectInitialStatusId = cast.ToInt(row[`defect_initial_status_id`])
	model.PlanningInitialStatusId = cast.ToInt(row[`planning_initial_status_id`])
	model.RequirementFinalStatusIds = toIntSlice(row[`requirement_final_status_ids`])
	model.DefectFinalStatusIds = toIntSlice(row[`defect_final_status_ids`])
	model.PlanningFinalStatusIds = toIntSlice(row[`planning_final_status_ids`])
	model.RequirementCompletedStatusIds = toIntSlice(row[`requirement_completed_status_ids`])
	model.DefectCompletedStatusIds = toIntSlice(row[`defect_completed_status_ids`])
	model.RequirementClosedStatusIds = statusDifference(model.RequirementFinalStatusIds, model.RequirementCompletedStatusIds)
	model.DefectClosedStatusIds = statusDifference(model.DefectFinalStatusIds, model.DefectCompletedStatusIds)
	model.RequirementStatusTransition = parseStatusTransitionForm(row[`requirement_status_transition`])
	model.DefectStatusTransition = parseStatusTransitionForm(row[`defect_status_transition`])
	model.RequirementAgingControl = parseAgingControlForm(row[`requirement_aging_control`])
	slaSettings, err := getSLASettingsTx(nil, model.AdminId, cast.ToInt(model.WsId), false)
	if err != nil {
		return WorkspaceSettingsModel{}, err
	}
	model.SLAEnabled, model.SLAActivatedAt = slaSettings.Enabled, slaSettings.ActivatedAt
	return model, nil
}

// UpdateWorkspaceSettings updates the workspace initial/final state config (admin_id isolation).
func UpdateWorkspaceSettings(model UpdateWorkspaceSettingsModel) error {
	adminId, wsId := model.AdminId, model.WsId
	recalcVersionCounts := model.RequirementFinalStatusIds != nil || model.DefectFinalStatusIds != nil || model.PlanningFinalStatusIds != nil || model.RequirementCompletedStatusIds != nil || model.DefectCompletedStatusIds != nil || model.RequirementClosedStatusIds != nil || model.DefectClosedStatusIds != nil
	data := msql.Datas{}
	if model.RequirementInitialStatusId != nil {
		data[`requirement_initial_status_id`] = cast.ToString(*model.RequirementInitialStatusId)
	}
	if model.DefectInitialStatusId != nil {
		data[`defect_initial_status_id`] = cast.ToString(*model.DefectInitialStatusId)
	}
	if model.PlanningInitialStatusId != nil {
		data[`planning_initial_status_id`] = cast.ToString(*model.PlanningInitialStatusId)
	}
	if model.RequirementFinalStatusIds != nil {
		data[`requirement_final_status_ids`] = joinIntSlice(*model.RequirementFinalStatusIds)
	}
	if model.DefectFinalStatusIds != nil {
		data[`defect_final_status_ids`] = joinIntSlice(*model.DefectFinalStatusIds)
	}
	if model.PlanningFinalStatusIds != nil {
		data[`planning_final_status_ids`] = joinIntSlice(*model.PlanningFinalStatusIds)
	}
	if model.RequirementStatusTransition != nil {
		encoded, encodeErr := encodeStatusTransitionForm(*model.RequirementStatusTransition)
		if encodeErr != nil {
			return encodeErr
		}
		data[`requirement_status_transition`] = encoded
	}
	if model.DefectStatusTransition != nil {
		encoded, encodeErr := encodeStatusTransitionForm(*model.DefectStatusTransition)
		if encodeErr != nil {
			return encodeErr
		}
		data[`defect_status_transition`] = encoded
	}
	if model.RequirementAgingControl != nil {
		encoded, encodeErr := encodeAgingControlForm(*model.RequirementAgingControl)
		if encodeErr != nil {
			return encodeErr
		}
		data[`requirement_aging_control`] = encoded
	}
	if len(data) == 0 && !recalcVersionCounts {
		return nil
	}
	data[`updated_at`] = tool.Time2Int()
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			logs.Error("rollback workspace completion settings: %s", rollbackErr.Error())
		}
	}()
	rows, err := msql.RawValues(define.Postgres, `SELECT id,requirement_final_status_ids,defect_final_status_ids,requirement_completed_status_ids,defect_completed_status_ids FROM `+define.TableWorkspaceSettings+` WHERE admin_id=$1 AND workspace_id=$2 FOR UPDATE`, tx, adminId, cast.ToInt(wsId))
	if err != nil {
		return err
	}
	if model.RequirementAgingControl != nil {
		settings, err := getSLASettingsTx(tx, adminId, cast.ToInt(wsId), false)
		if err != nil {
			return err
		}
		if settings.ActivatedAt > 0 {
			return ErrSLAConflict
		}
	}
	var old msql.Params
	if len(rows) > 0 {
		old = rows[0]
	}
	for _, group := range []workspaceStatusPatch{
		{define.TypeKeyRequirement, model.RequirementInitialStatusId, model.RequirementFinalStatusIds, model.RequirementCompletedStatusIds, model.RequirementClosedStatusIds},
		{define.TypeKeyDefect, model.DefectInitialStatusId, model.DefectFinalStatusIds, model.DefectCompletedStatusIds, model.DefectClosedStatusIds},
	} {
		if err = applyWorkspaceStatusPatch(tx, adminId, cast.ToInt(wsId), old, group, data); err != nil {
			return err
		}
	}
	columns := make([]string, 0, len(data))
	for key := range data {
		columns = append(columns, key)
	}
	sort.Strings(columns)
	args := []any{adminId, cast.ToInt(wsId)}
	values := []string{}
	assignments := []string{}
	for _, key := range columns {
		args = append(args, data[key])
		placeholder := fmt.Sprintf("$%d", len(args))
		values = append(values, placeholder)
		assignments = append(assignments, key+"="+placeholder)
	}
	query := `UPDATE ` + define.TableWorkspaceSettings + ` SET ` + strings.Join(assignments, ",") + ` WHERE admin_id=$1 AND workspace_id=$2`
	if len(rows) == 0 {
		query = `INSERT INTO ` + define.TableWorkspaceSettings + ` (created_at,admin_id,workspace_id,` + strings.Join(columns, ",") + `) VALUES (` + fmt.Sprintf("$%d", 3+sort.SearchStrings(columns, "updated_at")) + `,$1,$2,` + strings.Join(values, ",") + `)`
	}
	if _, err = msql.RawExec(define.Postgres, query, tx, args...); err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return err
	}
	if model.RequirementInitialStatusId != nil || model.RequirementFinalStatusIds != nil || model.RequirementCompletedStatusIds != nil || model.RequirementClosedStatusIds != nil {
		if err = checkSLAWorkspaceStatusUpdateTx(tx, adminId, cast.ToInt(wsId)); err != nil {
			return err
		}
	}
	if len(rows) > 0 {
		for _, typ := range []string{"requirement", "defect"} {
			field := typ + "_completed_status_ids"
			raw, changed := data[field]
			if !changed {
				continue
			}
			oldIds, newIds := toIntSlice(rows[0][field]), toIntSlice(cast.ToString(raw))
			// 新设为完成状态时无法推断历史完成日期，保留为未定日期；取消完成属性不删除历史时间。
			changedIds := []int{}
			for _, id := range newIds {
				if !intSliceContains(oldIds, id) {
					changedIds = append(changedIds, id)
				}
			}
			if len(changedIds) == 0 {
				continue
			}
			query = `UPDATE ` + define.TableRequirements + ` SET completed_at=0 WHERE admin_id=$1 AND workspace_id=$2 AND type=$3 AND status_id=ANY($4::INTEGER[]) AND completed_at>0`
			if _, err = msql.RawExec(define.Postgres, query, tx, adminId, cast.ToInt(wsId), typ, pq.Array(changedIds)); err != nil {
				logs.Error("sql:%s, err:%s", query, err.Error())
				return err
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	// Changing the end status configuration alters the calculation basis of completed_count.
	if recalcVersionCounts {
		if err := RecalcWorkspaceVersionCounts(adminId, cast.ToInt(wsId)); err != nil {
			logs.Error("RecalcWorkspaceVersionCounts after settings update failed, err:%s", err.Error())
		}
		// 公开详情页缓存的是算好的 can_recall / can_modify，结束状态一改这些标志就失效
		if err := RotateIssueInviteRenderVersion(adminId, cast.ToInt(wsId)); err != nil {
			logs.Error("rotate issue invite render version after settings update failed, err:%s", err.Error())
		}
	}
	return nil
}

// toIntSlice parses a comma-separated ID string into an int slice.
func toIntSlice(s string) []int {
	if len(s) == 0 {
		return []int{}
	}
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		if id := cast.ToInt(p); id > 0 {
			out = append(out, id)
		}
	}
	return out
}

// joinIntSlice joins an int slice into a comma-separated string.
func joinIntSlice(ids []int) string {
	strs := make([]string, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			strs = append(strs, cast.ToString(id))
		}
	}
	return strings.Join(strs, ",")
}

func removeIntValue(ids []int, target int) []int {
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if id != target {
			out = append(out, id)
		}
	}
	return out
}

// deletedStatusSettingsData builds the initial/final status cleanup for a deleted status.
func deletedStatusSettingsData(settings WorkspaceSettingsModel, typeKey string, statusId int) msql.Datas {
	initialField, finalField := settingsStatusFields(typeKey)
	initialStatusId := settings.RequirementInitialStatusId
	finalStatusIds := settings.RequirementFinalStatusIds
	switch typeKey {
	case define.TypeKeyDefect:
		initialStatusId = settings.DefectInitialStatusId
		finalStatusIds = settings.DefectFinalStatusIds
	case define.TypeKeyPlanning:
		initialStatusId = settings.PlanningInitialStatusId
		finalStatusIds = settings.PlanningFinalStatusIds
	}
	data := msql.Datas{}
	if initialStatusId == statusId {
		data[initialField] = 0
	}
	if typeKey != define.TypeKeyPlanning {
		completed := settings.RequirementCompletedStatusIds
		if typeKey == define.TypeKeyDefect {
			completed = settings.DefectCompletedStatusIds
		}
		if next := removeIntValue(completed, statusId); len(next) != len(completed) {
			data[typeKey+"_completed_status_ids"] = joinIntSlice(next)
		}
	}
	filtered := removeIntValue(finalStatusIds, statusId)
	if len(filtered) != len(finalStatusIds) {
		data[finalField] = joinIntSlice(filtered)
	}
	return data
}

// DeleteWorkspaceModel describes the delete-workspace use case.
type DeleteWorkspaceModel struct {
	AdminId int
	WsId    int
}

// ======================== Enterprise Project Management ========================

// EnterpriseProjectModel describes one project row of the enterprise project management list.
type EnterpriseProjectModel struct {
	Id          int
	Name        string
	Color       string
	Icon        string
	EndedAt     int64
	SortOrder   int
	CreatedBy   int
	CreatedAt   int64
	CreatorName string
	AdminNames  []string
	MemberCount int
}

// ListEnterpriseProjectsModel describes the enterprise project list use case.
type ListEnterpriseProjectsModel struct {
	AdminId int
	UserId  int
	Keyword string
	Items   []EnterpriseProjectModel
}

// SetWorkspaceEndedModel describes ending or reopening a project.
type SetWorkspaceEndedModel struct {
	AdminId int
	WsId    int
	Ended   bool
}

// SortWorkspacesModel describes one account's project order and optional tenant-wide application.
type SortWorkspacesModel struct {
	AdminId    int
	UserId     int
	WsIds      []int
	ApplyToAll bool
}

// enterpriseProjectFields are the workspace columns required by the enterprise project list.
const enterpriseProjectFields = `id, name, color, icon, ended_at, sort_order, created_by, created_at`

// toEnterpriseProjectModel converts a workspace row into the enterprise project model.
func toEnterpriseProjectModel(row msql.Params) (EnterpriseProjectModel, error) {
	id, err := metadataRowInt(row, `id`)
	if err != nil {
		return EnterpriseProjectModel{}, err
	}
	sortOrder, err := metadataRowInt(row, `sort_order`)
	if err != nil {
		return EnterpriseProjectModel{}, err
	}
	createdBy, err := metadataRowInt(row, `created_by`)
	if err != nil {
		return EnterpriseProjectModel{}, err
	}
	endedAt, err := metadataRowInt64(row, `ended_at`)
	if err != nil {
		return EnterpriseProjectModel{}, err
	}
	createdAt, err := metadataRowInt64(row, `created_at`)
	if err != nil {
		return EnterpriseProjectModel{}, err
	}
	return EnterpriseProjectModel{
		Id: id, Name: row[`name`], Color: row[`color`], Icon: row[`icon`], EndedAt: endedAt,
		SortOrder: sortOrder, CreatedBy: createdBy, CreatedAt: createdAt, AdminNames: []string{},
	}, nil
}

// ListEnterpriseProjects returns every tenant project with its creator, project admins and member count.
func ListEnterpriseProjects(model ListEnterpriseProjectsModel) (ListEnterpriseProjectsModel, error) {
	conditions := [][]string{{`admin_id`, cast.ToString(model.AdminId)}}
	if keyword := strings.TrimSpace(model.Keyword); len(keyword) > 0 {
		conditions = append(conditions, []string{`name`, `like`, `%` + keyword + `%`})
	}
	m := msql.Model(define.TableWorkspaces, define.Postgres)
	rows, err := m.Where2(conditions).
		Field(enterpriseProjectFields).Order(`sort_order asc, id desc`).Select()
	if err != nil {
		logs.Error("list enterprise projects sql:%s, err:%s", m.GetLastSql(), err.Error())
		return ListEnterpriseProjectsModel{}, err
	}

	userNames, err := tenantUserNames(model.AdminId)
	if err != nil {
		return ListEnterpriseProjectsModel{}, err
	}
	memberCounts, adminNames, err := workspaceMemberStats(model.AdminId, userNames)
	if err != nil {
		return ListEnterpriseProjectsModel{}, err
	}

	model.Items = make([]EnterpriseProjectModel, 0, len(rows))
	for _, row := range rows {
		item, err := toEnterpriseProjectModel(row)
		if err != nil {
			logs.Error("convert enterprise project row failed: %s", err.Error())
			return ListEnterpriseProjectsModel{}, err
		}
		item.CreatorName = userNames[item.CreatedBy]
		item.MemberCount = memberCounts[item.Id]
		if names := adminNames[item.Id]; len(names) > 0 {
			item.AdminNames = names
		}
		model.Items = append(model.Items, item)
	}
	order, err := workspaceOrder(model.AdminId, model.UserId)
	if err != nil {
		return ListEnterpriseProjectsModel{}, err
	}
	rank := make(map[int]int, len(order))
	for index, id := range order {
		if _, exists := rank[id]; !exists {
			rank[id] = index
		}
	}
	sort.SliceStable(model.Items, func(i, j int) bool {
		left, leftFound := rank[model.Items[i].Id]
		right, rightFound := rank[model.Items[j].Id]
		if leftFound != rightFound {
			return leftFound
		}
		if !leftFound {
			return false
		}
		return left < right
	})
	return model, nil
}

// tenantUserNames returns the user id → display name map of the tenant.
func tenantUserNames(adminId int) (map[int]string, error) {
	m := msql.Model(define.TableUsers, define.Postgres)
	rows, err := m.Where(`admin_id`, cast.ToString(adminId)).Field(`id, name`).Select()
	if err != nil {
		logs.Error("list tenant users sql:%s, err:%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	names := make(map[int]string, len(rows))
	for _, row := range rows {
		names[cast.ToInt(row[`id`])] = row[`name`]
	}
	return names, nil
}

// workspaceMemberStats returns member counts and project admin names grouped by workspace id.
func workspaceMemberStats(adminId int, userNames map[int]string) (map[int]int, map[int][]string, error) {
	adminRoleIds, err := projectAdminRoleIds(adminId)
	if err != nil {
		return nil, nil, err
	}
	m := msql.Model(define.TableWorkspaceMembers, define.Postgres)
	rows, err := m.Where(`admin_id`, cast.ToString(adminId)).Field(`workspace_id, user_id, role_id`).Select()
	if err != nil {
		logs.Error("list tenant workspace members sql:%s, err:%s", m.GetLastSql(), err.Error())
		return nil, nil, err
	}
	counts := make(map[int]int, len(rows))
	admins := make(map[int][]string, len(rows))
	for _, row := range rows {
		wsId := cast.ToInt(row[`workspace_id`])
		counts[wsId]++
		if !adminRoleIds[cast.ToInt(row[`role_id`])] {
			continue
		}
		if name := userNames[cast.ToInt(row[`user_id`])]; len(name) > 0 {
			admins[wsId] = append(admins[wsId], name)
		}
	}
	return counts, admins, nil
}

// projectAdminRoleIds returns the preset project admin role ids of the tenant.
func projectAdminRoleIds(adminId int) (map[int]bool, error) {
	m := msql.Model(define.TableRoles, define.Postgres)
	rows, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`scope`, permission.ScopeProject},
		{`name`, lib_i18n.WrapKey("perm_role_project_admin")},
	}).Field(`id`).Select()
	if err != nil {
		logs.Error("list project admin roles sql:%s, err:%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	ids := make(map[int]bool, len(rows))
	for _, row := range rows {
		ids[cast.ToInt(row[`id`])] = true
	}
	return ids, nil
}

// SetWorkspaceEnded ends or reopens a project (admin_id isolated).
// Ended projects disappear from the sidebar project list but stay reachable and editable.
func SetWorkspaceEnded(model SetWorkspaceEndedModel) error {
	now := tool.Time2Int()
	endedAt := 0
	if model.Ended {
		endedAt = now
	}
	m := msql.Model(define.TableWorkspaces, define.Postgres)
	affected, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`id`, cast.ToString(model.WsId)},
	}).Update(msql.Datas{`ended_at`: endedAt, `updated_at`: now})
	if err != nil {
		logs.Error("set workspace ended sql:%s, err:%s", m.GetLastSql(), err.Error())
		return err
	}
	if affected == 0 {
		return ErrWorkspaceNotFound
	}
	return nil
}

// SortWorkspaces saves one account's project order and can apply it to all active tenant members.
func SortWorkspaces(model SortWorkspacesModel) error {
	canManage := HasPermission(model.AdminId, model.UserId, ``, `project:manage`)
	if model.ApplyToAll && !canManage {
		return ErrWorkspaceOrderForbidden
	}
	allowed, err := workspaceOrderTargets(model.AdminId, model.UserId, canManage)
	if err != nil {
		return err
	}
	order, err := normalizeWorkspaceOrder(model.WsIds, allowed)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(order)
	if err != nil {
		return err
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("sort workspaces begin tx err:%s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := tool.Time2Int()
	if model.ApplyToAll {
		_, err = msql.RawExec(define.Postgres, `INSERT INTO `+define.TableUserWorkspaceOrderSettings+`
			(admin_id, user_id, workspace_order, created_at, updated_at)
			SELECT $1, member_id, $2, $3, $3 FROM (
				SELECT 0 AS member_id UNION SELECT id FROM users
				WHERE admin_id = $1 AND COALESCE(status, 1) = 1 AND deleted_at = 0
			) members
			ON CONFLICT (admin_id, user_id) DO UPDATE
			SET workspace_order = EXCLUDED.workspace_order, updated_at = EXCLUDED.updated_at`, tx, model.AdminId, string(raw), now)
	} else {
		_, err = msql.RawExec(define.Postgres, `INSERT INTO `+define.TableUserWorkspaceOrderSettings+`
			(admin_id, user_id, workspace_order, created_at, updated_at) VALUES ($1, $2, $3, $4, $4)
			ON CONFLICT (admin_id, user_id) DO UPDATE
			SET workspace_order = EXCLUDED.workspace_order, updated_at = EXCLUDED.updated_at`, tx,
			model.AdminId, model.UserId, string(raw), now)
	}
	if err != nil {
		logs.Error("save workspace order failed, err:%s", err.Error())
		return err
	}
	return tx.Commit()
}

func normalizeWorkspaceOrder(submitted, allowed []int) ([]int, error) {
	allowedSet := make(map[int]bool, len(allowed))
	for _, id := range allowed {
		allowedSet[id] = true
	}
	seen := make(map[int]bool, len(submitted))
	order := make([]int, 0, len(allowed))
	for _, id := range submitted {
		if !allowedSet[id] || seen[id] {
			return nil, ErrWorkspaceOrderInvalid
		}
		seen[id] = true
		order = append(order, id)
	}
	for _, id := range allowed {
		if !seen[id] {
			order = append(order, id)
		}
	}
	return order, nil
}

func workspaceOrderTargets(adminId, userId int, canManage bool) ([]int, error) {
	if canManage {
		rows, err := msql.RawValues(define.Postgres, `SELECT id FROM workspaces WHERE admin_id = $1
			ORDER BY sort_order ASC, id DESC`, nil, adminId)
		if err != nil {
			logs.Error("list workspace order targets failed, err:%s", err.Error())
			return nil, err
		}
		ids := make([]int, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, cast.ToInt(row[`id`]))
		}
		return ids, nil
	}
	listed, err := ListWorkspaces(ListWorkspacesModel{AdminId: adminId, UserId: userId})
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(listed.Items))
	for _, item := range listed.Items {
		ids = append(ids, item.Id)
	}
	return ids, nil
}

// workspaceTablesByWorkspaceId are the project tables cleaned up by admin_id + workspace_id.
var workspaceTablesByWorkspaceId = append([]string{
	define.TableSLAActions,
	define.TableSLACycles,
	define.TableSLASettings,
	define.TableRequirementComments,
	define.TableRequirementHistories,
	define.TableRequirementWorkLogs,
	define.TableRequirements,
	define.TablePlanningRelations,
	define.TablePlanningGroups,
	define.TableVersions,
	define.TableWorkspaceMembers,
	define.TableWorkItemStatuses,
	define.TableWorkItemPriorities,
	define.TableWorkItemFieldConfigs,
	define.TableCustomFields,
	define.TableWorkspaceSettings,
	define.TableWorkspaceApplications,
	define.TableDashboardVisibilityRoles,
	define.TableDashboardVisibilityMembers,
	define.TableDashboardEditors,
	define.TableDashboardCards,
	define.TableDashboardAnnouncements,
	define.TableDashboardUserStates,
	define.TableWorkspaceDashboards,
	define.TableWorkspaceWorkItemTemplates,
	define.TableRoles,
	define.TablePipelineTask,
	define.TableAutomationExecutions,
	define.TableAutomationRules,
	define.TableWorkspaceViews,
	define.TableUserViewSettings,
	define.TableRequirementCategories,
}, define.AllEnterpriseFileTables()...)

// workspaceTablesByRequirementId are the tables that only reference work items, cleaned up by requirement ids.
var workspaceTablesByRequirementId = []string{
	define.TableCustomFieldValues,
	define.TableRequirementVisits,
}

// workspaceTablesByRoleId are the tables that reference the project roles being removed.
var workspaceTablesByRoleId = []string{
	define.TableRolePermissions,
	define.TableRoleDataViewMembers,
	define.TableRoleTeamWorkMembers,
}

// workspaceTablesByPipelineTaskId are the pipeline tables that reference the project's pipeline tasks.
var workspaceTablesByPipelineTaskId = []string{
	define.TablePipelineTaskStep,
	define.TablePipelineTaskSession,
}

// DeleteWorkspace permanently removes a project together with all of its data (admin_id isolated).
// Rows are removed in one transaction; stored files are removed afterwards because they cannot be rolled back.
func DeleteWorkspace(model DeleteWorkspaceModel) error {
	workspace, err := GetWorkspaceById(model.AdminId, cast.ToString(model.WsId))
	if err != nil {
		return err
	}
	if len(workspace) == 0 {
		return ErrWorkspaceNotFound
	}

	// Deprecated tables (e.g. workspace_status_transitions) are already dropped in some environments,
	// and deleting from a missing table would abort the whole transaction.
	exists, err := existingTables(workspaceCleanupTables())
	if err != nil {
		return err
	}

	attachmentM := msql.Model(define.GetEnterpriseFileTable(model.AdminId), define.Postgres)
	attachments, err := attachmentM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, cast.ToString(model.WsId)},
	}).Field(`stored_path`).Select()
	if err != nil {
		logs.Error("list workspace attachments sql:%s, err:%s", attachmentM.GetLastSql(), err.Error())
		return err
	}
	fileTable := define.GetEnterpriseFileTable(model.AdminId)
	fileRows, err := msql.RawValues(define.Postgres,
		fmt.Sprintf(`SELECT stored_path, file_size FROM %s WHERE admin_id = $1 AND workspace_id = $2`, fileTable), nil,
		model.AdminId, model.WsId)
	if err != nil {
		logs.Error("list workspace enterprise files failed: %s", err.Error())
		return err
	}
	attachments = append(attachments, fileRows...)
	var workspaceFilesSize int64
	for _, row := range fileRows {
		workspaceFilesSize += cast.ToInt64(row[`file_size`])
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("delete workspace begin tx err:%s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = getSLASettingsTx(tx, model.AdminId, model.WsId, true); err != nil {
		return err
	}

	// Child ids must be collected before their parent rows are removed.
	requirementIds, err := workspaceChildIds(tx, define.TableRequirements, model)
	if err != nil {
		return err
	}
	roleIds, err := workspaceChildIds(tx, define.TableRoles, model)
	if err != nil {
		return err
	}
	pipelineTaskIds, err := workspaceChildIds(tx, define.TablePipelineTask, model)
	if err != nil {
		return err
	}

	if err = deleteWorkspaceRowsByIds(tx, workspaceTablesByRequirementId, exists, `requirement_id`, model.AdminId, requirementIds); err != nil {
		return err
	}
	if err = deleteWorkspaceRowsByIds(tx, workspaceTablesByRoleId, exists, `role_id`, model.AdminId, roleIds); err != nil {
		return err
	}
	if err = deleteWorkspaceRowsByIds(tx, workspaceTablesByPipelineTaskId, exists, `pipeline_task_id`, model.AdminId, pipelineTaskIds); err != nil {
		return err
	}
	for _, table := range workspaceTablesByWorkspaceId {
		if !exists[table] {
			continue
		}
		if _, err = msql.RawExec(define.Postgres, fmt.Sprintf(`DELETE FROM %s WHERE admin_id = $1 AND workspace_id = $2`, table), tx,
			model.AdminId, model.WsId); err != nil {
			logs.Error("delete workspace rows from [%s] failed: %s", table, err.Error())
			return err
		}
	}
	if _, err = msql.RawExec(define.Postgres,
		fmt.Sprintf(`DELETE FROM %s WHERE admin_id = $1 AND workspace_id = $2`, fileTable), tx,
		model.AdminId, model.WsId); err != nil {
		logs.Error("delete workspace enterprise files failed: %s", err.Error())
		return err
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM workspaces WHERE admin_id = $1 AND id = $2`, tx,
		model.AdminId, model.WsId); err != nil {
		logs.Error("delete workspace failed: %s", err.Error())
		return err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("delete workspace commit failed: %s", err.Error())
		return err
	}
	// The ledger rows of this workspace were deleted inside the transaction; keep the cached usage in sync.
	if workspaceFilesSize > 0 {
		adjustStorageUsedCache(model.AdminId, -workspaceFilesSize)
	}

	// ponytail: only the project icon and attachment files are tracked here; rich-text inline images keep the
	// existing work item deletion behaviour and remain in storage.
	removeWorkspaceStoredFiles(workspace[`icon`], attachments)
	logs.Info("workspace [%d] permanently deleted by tenant [%d]", model.WsId, model.AdminId)
	return nil
}

// workspaceCleanupTables returns every table involved in project deletion.
func workspaceCleanupTables() []string {
	tables := make([]string, 0, len(workspaceTablesByWorkspaceId)+len(workspaceTablesByRequirementId)+
		len(workspaceTablesByRoleId)+len(workspaceTablesByPipelineTaskId))
	tables = append(tables, workspaceTablesByWorkspaceId...)
	tables = append(tables, workspaceTablesByRequirementId...)
	tables = append(tables, workspaceTablesByRoleId...)
	tables = append(tables, workspaceTablesByPipelineTaskId...)
	return tables
}

// existingTables reports which of the given tables exist in the current database.
func existingTables(tables []string) (map[string]bool, error) {
	rows, err := msql.RawValues(define.Postgres,
		`SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_name = ANY($1)`,
		nil, pq.Array(tables))
	if err != nil {
		logs.Error("check existing tables failed: %s", err.Error())
		return nil, err
	}
	exists := make(map[string]bool, len(rows))
	for _, row := range rows {
		exists[row[`table_name`]] = true
	}
	return exists, nil
}

// workspaceChildIds collects the ids of a project child table inside the deletion transaction.
func workspaceChildIds(tx *sql.Tx, table string, model DeleteWorkspaceModel) ([]int, error) {
	rows, err := msql.RawValues(define.Postgres,
		fmt.Sprintf(`SELECT id FROM %s WHERE admin_id = $1 AND workspace_id = $2`, table), tx,
		model.AdminId, model.WsId)
	if err != nil {
		logs.Error("collect ids from [%s] failed: %s", table, err.Error())
		return nil, err
	}
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, cast.ToInt(row[`id`]))
	}
	return ids, nil
}

// deleteWorkspaceRowsByIds removes rows of the given tables matched by a foreign key id list.
func deleteWorkspaceRowsByIds(tx *sql.Tx, tables []string, exists map[string]bool, foreignKey string, adminId int, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	for _, table := range tables {
		if !exists[table] {
			continue
		}
		if _, err := msql.RawExec(define.Postgres,
			fmt.Sprintf(`DELETE FROM %s WHERE admin_id = $1 AND %s = ANY($2)`, table, foreignKey), tx,
			adminId, pq.Array(ids)); err != nil {
			logs.Error("delete rows from [%s] failed: %s", table, err.Error())
			return err
		}
	}
	return nil
}

// removeWorkspaceStoredFiles removes the project icon and attachment files after the rows are deleted.
func removeWorkspaceStoredFiles(icon string, attachments []msql.Params) {
	if len(icon) > 0 {
		if err := lib_storage.M.Delete(icon); err != nil {
			logs.Error("delete workspace icon file failed, path:%s, err:%s", icon, err.Error())
		}
	}
	for _, row := range attachments {
		if stored := row[`stored_path`]; len(stored) > 0 {
			if err := lib_storage.M.Delete(stored); err != nil {
				logs.Error("delete workspace attachment file failed, path:%s, err:%s", stored, err.Error())
			}
		}
	}
}

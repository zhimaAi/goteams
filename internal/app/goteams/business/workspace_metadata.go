// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"encoding/json"
	"fmt"
	"strconv"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// WorkspaceMetadataStatusModel holds status info in the metadata.
type WorkspaceMetadataStatusModel struct {
	Id        int
	Key       string
	Name      string
	Color     string
	SortOrder int
	IsDefault int
}

// WorkspaceMetadataPriorityModel holds priority info in the metadata.
type WorkspaceMetadataPriorityModel struct {
	Id        int
	Key       string // 跨工作空间映射使用的稳定优先级键（自动化规则引用）
	Label     string
	Color     string
	SortOrder int
	IsDefault int
}

// WorkspaceMetadataFieldConfigModel holds built-in field config in the metadata.
type WorkspaceMetadataFieldConfigModel struct {
	Key       string
	Label     string
	Enabled   int
	Editable  int
	Locked    int
	SortOrder int
}

// WorkspaceMetadataCustomFieldOptionModel holds custom field options in the metadata.
type WorkspaceMetadataCustomFieldOptionModel struct {
	Key   string
	Name  string
	Color string
}

// WorkspaceMetadataCustomFieldModel holds custom field info in the metadata.
type WorkspaceMetadataCustomFieldModel struct {
	Id          int
	Key         string
	Name        string
	FieldType   string
	IsRequired  int
	SortOrder   int
	IsEnabled   int
	Options     []WorkspaceMetadataCustomFieldOptionModel
	IntegerOnly int
	MemberScope string
	RoleIds     []int
	Cascader    []CustomFieldCascaderNode
}

// WorkspaceMetadataGroupModel is the metadata group for a single work item type.
type WorkspaceMetadataGroupModel struct {
	Statuses     []WorkspaceMetadataStatusModel
	Priorities   []WorkspaceMetadataPriorityModel
	FieldConfigs []WorkspaceMetadataFieldConfigModel
	CustomFields []WorkspaceMetadataCustomFieldModel
}

// WorkspaceMetadataSettingsModel holds the workflow initial/terminal status config in the metadata.
type WorkspaceMetadataSettingsModel struct {
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

// WorkspaceMetadataWorkspaceModel holds the basic workspace info in the metadata.
type WorkspaceMetadataWorkspaceModel struct {
	Id              int
	WorkspaceType   string
	AdminId         int
	Name            string
	Slug            string
	Description     string
	Color           string
	Icon            string
	EndedAt         int64
	ActiveVersionId int
	CreatedBy       int
	CreatedAt       int64
	UpdatedAt       int64
}

// WorkspaceMetadataCustomFieldMemberModel holds the minimal tenant member data needed by member-type custom fields.
type WorkspaceMetadataCustomFieldMemberModel struct {
	UserId            int
	EnterpriseRoleId  int
	UserName          string
	UserUsername      string
	UserAvatar        string
	UserStatus        int
	IsWorkspaceMember bool
	Removed           bool
}

// WorkspaceMetadataModel describes the workspace metadata use case.
type WorkspaceMetadataModel struct {
	AdminId             int
	WorkspaceId         string
	Lang                string
	IncludeVersions     bool
	Requirement         WorkspaceMetadataGroupModel
	Defect              WorkspaceMetadataGroupModel
	Planning            WorkspaceMetadataGroupModel
	Settings            WorkspaceMetadataSettingsModel
	Workspace           *WorkspaceMetadataWorkspaceModel
	Members             []WorkspaceMemberModel
	CustomFieldMembers  []WorkspaceMetadataCustomFieldMemberModel
	Versions            []VersionModel
	Permissions         []string
	EnabledApplications []string
}

// GetWorkspaceMetadata returns the basic metadata for requirements and defects under a workspace.
func GetWorkspaceMetadata(model WorkspaceMetadataModel) (WorkspaceMetadataModel, error) {
	model.Requirement = newWorkspaceMetadataGroup()
	model.Defect = newWorkspaceMetadataGroup()
	model.Planning = newWorkspaceMetadataGroup()
	model.Members = []WorkspaceMemberModel{}
	model.CustomFieldMembers = []WorkspaceMetadataCustomFieldMemberModel{}
	model.Versions = []VersionModel{}
	model.Permissions = []string{}

	statusM := msql.Model(define.TableWorkItemStatuses, define.Postgres)
	statusRows, err := statusM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WorkspaceId},
	}).Field(`id, type_key, name, color, sort_order, is_default`).Order(`type_key asc, sort_order asc, id asc`).Select()
	if err != nil {
		logs.Error("sql:%s, err:%s", statusM.GetLastSql(), err.Error())
		return WorkspaceMetadataModel{}, err
	}
	for _, row := range statusRows {
		group := workspaceMetadataGroup(&model, row[`type_key`])
		if group == nil {
			continue
		}
		id, err := metadataRowInt(row, `id`)
		if err != nil {
			return WorkspaceMetadataModel{}, err
		}
		sortOrder, err := metadataRowInt(row, `sort_order`)
		if err != nil {
			return WorkspaceMetadataModel{}, err
		}
		isDefault, err := metadataRowInt(row, `is_default`)
		if err != nil {
			return WorkspaceMetadataModel{}, err
		}
		key := ``
		if lib_i18n.IsWrappedKey(row[`name`]) {
			key = lib_i18n.UnwrapKey(row[`name`])
		}
		group.Statuses = append(group.Statuses, WorkspaceMetadataStatusModel{
			Id: id, Key: key, Name: lib_i18n.ShowStored(model.Lang, row[`name`]), Color: row[`color`], SortOrder: sortOrder, IsDefault: isDefault,
		})
	}

	priorityM := msql.Model(define.TableWorkItemPriorities, define.Postgres)
	priorityRows, err := priorityM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WorkspaceId},
	}).Field(`id, type_key, key, name, color, sort_order, is_default`).Order(`type_key asc, sort_order asc, id asc`).Select()
	if err != nil {
		logs.Error("sql:%s, err:%s", priorityM.GetLastSql(), err.Error())
		return WorkspaceMetadataModel{}, err
	}
	for _, row := range priorityRows {
		group := workspaceMetadataGroup(&model, row[`type_key`])
		if group == nil {
			continue
		}
		id, err := metadataRowInt(row, `id`)
		if err != nil {
			return WorkspaceMetadataModel{}, err
		}
		sortOrder, err := metadataRowInt(row, `sort_order`)
		if err != nil {
			return WorkspaceMetadataModel{}, err
		}
		isDefault, err := metadataRowInt(row, `is_default`)
		if err != nil {
			return WorkspaceMetadataModel{}, err
		}
		group.Priorities = append(group.Priorities, WorkspaceMetadataPriorityModel{
			Id: id, Key: row[`key`], Label: lib_i18n.ShowStored(model.Lang, row[`name`]), Color: row[`color`], SortOrder: sortOrder, IsDefault: isDefault,
		})
	}

	fieldM := msql.Model(define.TableWorkItemFieldConfigs, define.Postgres)
	fieldRows, err := fieldM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WorkspaceId},
	}).Field(`type_key, field_key, is_enabled, is_editable, is_locked, sort_order`).Order(`type_key asc, sort_order asc, id asc`).Select()
	if err != nil {
		logs.Error("sql:%s, err:%s", fieldM.GetLastSql(), err.Error())
		return WorkspaceMetadataModel{}, err
	}
	for _, row := range fieldRows {
		group := workspaceMetadataGroup(&model, row[`type_key`])
		if group == nil {
			continue
		}
		enabled, err := metadataRowInt(row, `is_enabled`)
		if err != nil {
			return WorkspaceMetadataModel{}, err
		}
		editable, err := metadataRowInt(row, `is_editable`)
		if err != nil {
			return WorkspaceMetadataModel{}, err
		}
		locked, err := metadataRowInt(row, `is_locked`)
		if err != nil {
			return WorkspaceMetadataModel{}, err
		}
		sortOrder, err := metadataRowInt(row, `sort_order`)
		if err != nil {
			return WorkspaceMetadataModel{}, err
		}
		group.FieldConfigs = append(group.FieldConfigs, WorkspaceMetadataFieldConfigModel{
			Key: row[`field_key`], Label: fieldLabel(model.Lang, row[`type_key`], row[`field_key`]), Enabled: enabled, Editable: editable, Locked: locked, SortOrder: sortOrder,
		})
	}

	customFieldM := msql.Model(define.TableCustomFields, define.Postgres)
	customFieldRows, err := customFieldM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WorkspaceId},
	}).Field(`id, type_key, key, name, field_type, is_required, sort_order, is_enabled, config_json`).Order(`type_key asc, sort_order asc, id asc`).Select()
	if err != nil {
		logs.Error("sql:%s, err:%s", customFieldM.GetLastSql(), err.Error())
		return WorkspaceMetadataModel{}, err
	}
	hasMemberCustomField := false
	for _, row := range customFieldRows {
		group := workspaceMetadataGroup(&model, row[`type_key`])
		if group == nil {
			continue
		}
		field, err := toWorkspaceMetadataCustomFieldModel(row)
		if err != nil {
			return WorkspaceMetadataModel{}, err
		}
		group.CustomFields = append(group.CustomFields, field)
		if customFieldIsMember(field.FieldType) {
			hasMemberCustomField = true
		}
	}

	settings, err := GetWorkspaceSettings(WorkspaceSettingsModel{AdminId: model.AdminId, WsId: model.WorkspaceId})
	if err != nil {
		return WorkspaceMetadataModel{}, err
	}
	model.Settings = WorkspaceMetadataSettingsModel{
		RequirementInitialStatusId:    settings.RequirementInitialStatusId,
		DefectInitialStatusId:         settings.DefectInitialStatusId,
		PlanningInitialStatusId:       settings.PlanningInitialStatusId,
		RequirementFinalStatusIds:     settings.RequirementFinalStatusIds,
		RequirementCompletedStatusIds: settings.RequirementCompletedStatusIds,
		RequirementClosedStatusIds:    settings.RequirementClosedStatusIds,
		DefectFinalStatusIds:          settings.DefectFinalStatusIds,
		DefectCompletedStatusIds:      settings.DefectCompletedStatusIds,
		DefectClosedStatusIds:         settings.DefectClosedStatusIds,
		PlanningFinalStatusIds:        settings.PlanningFinalStatusIds,
		RequirementStatusTransition:   settings.RequirementStatusTransition,
		DefectStatusTransition:        settings.DefectStatusTransition,
		RequirementAgingControl:       settings.RequirementAgingControl,
		SLAEnabled:                    settings.SLAEnabled,
		SLAActivatedAt:                settings.SLAActivatedAt,
	}

	workspace, err := getWorkspaceMetadataWorkspace(model.AdminId, model.WorkspaceId)
	if err != nil {
		return WorkspaceMetadataModel{}, err
	}
	model.Workspace = workspace

	members, err := ListMembersByWorkspace(ListWorkspaceMembersModel{AdminId: model.AdminId, WorkspaceId: model.WorkspaceId})
	if err != nil {
		return WorkspaceMetadataModel{}, err
	}
	// Removed members ride along flagged: work items still reference them (assignees, user-type
	// custom fields), so the frontend needs their name and avatar to render history. Pickers drop
	// them by the flag, keeping "a removed member can never be assigned again" intact.
	removed, err := ListRemovedTenantMembers(model.AdminId)
	if err != nil {
		return WorkspaceMetadataModel{}, err
	}
	model.Members = append(members.Items, removed...)

	// Same reason for members that only left this project: their users row is untouched, so the
	// list above misses them and their historical assignments would render as an empty cell.
	knownUserIds := make(map[int]bool, len(model.Members))
	for _, member := range model.Members {
		knownUserIds[member.UserId] = true
	}
	formerAssignees, err := ListWorkspaceFormerAssignees(model.AdminId, model.WorkspaceId, knownUserIds)
	if err != nil {
		return WorkspaceMetadataModel{}, err
	}
	model.Members = append(model.Members, formerAssignees...)

	if hasMemberCustomField {
		model.CustomFieldMembers, err = listWorkspaceMetadataCustomFieldMembers(model.AdminId, members.Items, model.Members)
		if err != nil {
			return WorkspaceMetadataModel{}, err
		}
	}

	if model.IncludeVersions {
		model.Versions, err = ListVersionsByWorkspace(model.AdminId, cast.ToInt(model.WorkspaceId))
		if err != nil {
			return WorkspaceMetadataModel{}, err
		}
	}

	model.EnabledApplications, err = GetEnabledApplicationKeys(model.AdminId, cast.ToInt(model.WorkspaceId))
	if err != nil {
		return WorkspaceMetadataModel{}, err
	}
	return model, nil
}

func listWorkspaceMetadataCustomFieldMembers(adminId int, workspaceMembers, references []WorkspaceMemberModel) ([]WorkspaceMetadataCustomFieldMemberModel, error) {
	joined := make(map[int]bool, len(workspaceMembers))
	for _, member := range workspaceMembers {
		joined[member.UserId] = true
	}

	enterprise, err := ListEnterpriseMembers(ListEnterpriseMembersModel{AdminId: adminId})
	if err != nil {
		return nil, err
	}
	items := make([]WorkspaceMetadataCustomFieldMemberModel, 0, len(enterprise.Items))
	seen := make(map[int]bool, len(enterprise.Items))
	for _, member := range enterprise.Items {
		seen[member.Id] = true
		items = append(items, WorkspaceMetadataCustomFieldMemberModel{
			UserId: member.Id, EnterpriseRoleId: member.RoleId, UserName: member.Name,
			UserUsername: member.Username, UserAvatar: member.Avatar, UserStatus: member.Status,
			IsWorkspaceMember: joined[member.Id],
		})
	}
	// Tenant members removed later remain resolvable in historical custom-field values, but are not selectable.
	for _, member := range references {
		if seen[member.UserId] {
			continue
		}
		seen[member.UserId] = true
		items = append(items, WorkspaceMetadataCustomFieldMemberModel{
			UserId: member.UserId, EnterpriseRoleId: member.EnterpriseRoleId, UserName: member.UserName,
			UserUsername: member.UserUsername, UserAvatar: member.UserAvatar, UserStatus: member.UserStatus,
			Removed: true,
		})
	}
	return items, nil
}

// getWorkspaceMetadataWorkspace reads the basic workspace info needed for metadata.
func getWorkspaceMetadataWorkspace(adminId int, workspaceId string) (*WorkspaceMetadataWorkspaceModel, error) {
	m := msql.Model(define.TableWorkspaces, define.Postgres)
	row, err := m.Where2([][]string{{`admin_id`, cast.ToString(adminId)}, {`id`, workspaceId}}).
		Field(`id, admin_id, workspace_type, name, slug, description, color, icon, ended_at, active_version_id, created_by, created_at, updated_at`).Find()
	if err != nil {
		logs.Error("get workspace metadata workspace error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	if len(row) == 0 {
		return nil, nil
	}
	id, err := metadataRowInt(row, `id`)
	if err != nil {
		return nil, err
	}
	rowAdminId, err := metadataRowInt(row, `admin_id`)
	if err != nil {
		return nil, err
	}
	endedAt, err := metadataRowInt64(row, `ended_at`)
	if err != nil {
		return nil, err
	}
	activeVersionId, err := metadataRowInt(row, `active_version_id`)
	if err != nil {
		return nil, err
	}
	createdBy, err := metadataRowInt(row, `created_by`)
	if err != nil {
		return nil, err
	}
	createdAt, err := metadataRowInt64(row, `created_at`)
	if err != nil {
		return nil, err
	}
	updatedAt, err := metadataRowInt64(row, `updated_at`)
	if err != nil {
		return nil, err
	}
	return &WorkspaceMetadataWorkspaceModel{
		WorkspaceType: row[`workspace_type`],
		Id:            id, AdminId: rowAdminId, Name: row[`name`], Slug: row[`slug`], Description: row[`description`],
		Color: row[`color`], Icon: row[`icon`], EndedAt: endedAt, ActiveVersionId: activeVersionId,
		CreatedBy: createdBy, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}

// toWorkspaceMetadataCustomFieldModel converts a custom field data row into the metadata model.
func toWorkspaceMetadataCustomFieldModel(row msql.Params) (WorkspaceMetadataCustomFieldModel, error) {
	id, err := metadataRowInt(row, `id`)
	if err != nil {
		return WorkspaceMetadataCustomFieldModel{}, err
	}
	isRequired, err := metadataRowInt(row, `is_required`)
	if err != nil {
		return WorkspaceMetadataCustomFieldModel{}, err
	}
	sortOrder, err := metadataRowInt(row, `sort_order`)
	if err != nil {
		return WorkspaceMetadataCustomFieldModel{}, err
	}
	isEnabled, err := metadataRowInt(row, `is_enabled`)
	if err != nil {
		return WorkspaceMetadataCustomFieldModel{}, err
	}
	var config CustomFieldConfig
	if row[`config_json`] != `` {
		if err := json.Unmarshal([]byte(row[`config_json`]), &config); err != nil {
			return WorkspaceMetadataCustomFieldModel{}, fmt.Errorf("parse workspace metadata custom field config: %w", err)
		}
	}
	options := make([]WorkspaceMetadataCustomFieldOptionModel, 0, len(config.Options))
	for _, option := range config.Options {
		options = append(options, WorkspaceMetadataCustomFieldOptionModel{Key: option.Key, Name: option.Name, Color: option.Color})
	}
	if config.Options == nil {
		options = nil
	}
	return WorkspaceMetadataCustomFieldModel{
		Id: id, Key: row[`key`], Name: row[`name`], FieldType: row[`field_type`], IsRequired: isRequired,
		SortOrder: sortOrder, IsEnabled: isEnabled, Options: options,
		IntegerOnly: config.IntegerOnly, MemberScope: config.MemberScope, RoleIds: config.RoleIds, Cascader: config.Cascader,
	}, nil
}

func metadataRowInt(row msql.Params, key string) (int, error) {
	value, err := strconv.Atoi(row[key])
	if err != nil {
		return 0, fmt.Errorf("parse workspace metadata %s: %w", key, err)
	}
	return value, nil
}

func metadataRowInt64(row msql.Params, key string) (int64, error) {
	value, err := strconv.ParseInt(row[key], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse workspace metadata %s: %w", key, err)
	}
	return value, nil
}

func newWorkspaceMetadataGroup() WorkspaceMetadataGroupModel {
	return WorkspaceMetadataGroupModel{
		Statuses:     []WorkspaceMetadataStatusModel{},
		Priorities:   []WorkspaceMetadataPriorityModel{},
		FieldConfigs: []WorkspaceMetadataFieldConfigModel{},
		CustomFields: []WorkspaceMetadataCustomFieldModel{},
	}
}

func workspaceMetadataGroup(model *WorkspaceMetadataModel, typeKey string) *WorkspaceMetadataGroupModel {
	switch typeKey {
	case define.TypeKeyRequirement:
		return &model.Requirement
	case define.TypeKeyDefect:
		return &model.Defect
	case define.TypeKeyPlanning:
		return &model.Planning
	default:
		return nil
	}
}

// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/lib/pq"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// ApiWorkspaceGroupModel 是开放 API 暴露的一组同类型工作项可选项（需求或缺陷）。
// 只含 Agent 写工作项字段需要的三类：状态、优先级、自定义字段。
type ApiWorkspaceGroupModel struct {
	Statuses     []WorkspaceMetadataStatusModel
	Priorities   []WorkspaceMetadataPriorityModel
	CustomFields []WorkspaceMetadataCustomFieldModel
}

// ApiWorkspaceMemberModel 是开放 API 暴露的可指派成员：Agent 拿 user_id 填 assignee_ids。
type ApiWorkspaceMemberModel struct {
	UserId       int
	UserName     string
	UserUsername string
}

// ApiWorkspaceSettingsModel 是开放 API 暴露的状态配置：初始状态与终态。
// 终态列表让 Agent 知道哪些状态代表流程结束，不必靠状态名猜。
type ApiWorkspaceSettingsModel struct {
	RequirementInitialStatusId    int
	DefectInitialStatusId         int
	RequirementFinalStatusIds     []int
	RequirementCompletedStatusIds []int
	RequirementClosedStatusIds    []int
	DefectFinalStatusIds          []int
	DefectCompletedStatusIds      []int
	DefectClosedStatusIds         []int
}

// ApiWorkspaceMetadataModel 是开放 API 里单个项目的可选项，Agent 靠它拿到写工作项字段需要的 ID。
type ApiWorkspaceMetadataModel struct {
	WorkspaceId int
	Requirement ApiWorkspaceGroupModel
	Defect      ApiWorkspaceGroupModel
	Versions    []VersionModel
	Members     []ApiWorkspaceMemberModel
	Settings    ApiWorkspaceSettingsModel
}

// ListApiWorkspaceMetadataModel 是开放 API 项目可选项用例的输入与结果。
type ListApiWorkspaceMetadataModel struct {
	AdminId      int
	Lang         string
	WorkspaceIds []int
	Items        []ApiWorkspaceMetadataModel
}

// ListApiWorkspaceMetadata 批量取多个项目的可选项。每张表只发一条 SQL，用 workspace_id = ANY(...)
// 覆盖全部项目：项目列表接口若按项目逐个查，查询次数会随项目数线性放大。
// Items 顺序与 WorkspaceIds 一致，调用方按 WorkspaceId 取用即可。
func ListApiWorkspaceMetadata(model ListApiWorkspaceMetadataModel) (ListApiWorkspaceMetadataModel, error) {
	model.Items = make([]ApiWorkspaceMetadataModel, 0, len(model.WorkspaceIds))
	if len(model.WorkspaceIds) == 0 {
		return model, nil
	}
	position := make(map[int]int, len(model.WorkspaceIds))
	for _, workspaceId := range model.WorkspaceIds {
		position[workspaceId] = len(model.Items)
		model.Items = append(model.Items, ApiWorkspaceMetadataModel{
			WorkspaceId: workspaceId,
			Requirement: newApiWorkspaceGroup(),
			Defect:      newApiWorkspaceGroup(),
			Versions:    []VersionModel{},
			Members:     []ApiWorkspaceMemberModel{},
		})
	}
	fills := []func(*ListApiWorkspaceMetadataModel, map[int]int) error{
		fillApiWorkspaceStatuses,
		fillApiWorkspacePriorities,
		fillApiWorkspaceCustomFields,
		fillApiWorkspaceVersions,
		fillApiWorkspaceMembers,
		fillApiWorkspaceSettings,
	}
	for _, fill := range fills {
		if err := fill(&model, position); err != nil {
			return ListApiWorkspaceMetadataModel{}, err
		}
	}
	return model, nil
}

// fillApiWorkspaceStatuses 取各项目需求与缺陷的状态定义。
func fillApiWorkspaceStatuses(model *ListApiWorkspaceMetadataModel, position map[int]int) error {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, workspace_id, type_key, name, color, sort_order, is_default
		FROM `+define.TableWorkItemStatuses+`
		WHERE admin_id = $1 AND workspace_id = ANY($2)
		ORDER BY workspace_id, type_key, sort_order, id`, nil, model.AdminId, pq.Array(model.WorkspaceIds))
	if err != nil {
		logs.Error("list api workspace statuses failed: admin_id=%d, err=%s", model.AdminId, err.Error())
		return err
	}
	for _, row := range rows {
		item, err := apiWorkspaceItem(row, model, position)
		if err != nil {
			return err
		}
		if item == nil {
			continue
		}
		group := apiWorkspaceGroup(item, row[`type_key`])
		if group == nil {
			continue
		}
		id, err := metadataRowInt(row, `id`)
		if err != nil {
			return err
		}
		sortOrder, err := metadataRowInt(row, `sort_order`)
		if err != nil {
			return err
		}
		isDefault, err := metadataRowInt(row, `is_default`)
		if err != nil {
			return err
		}
		// 状态名可能存的是 i18n key，key 只在能反解出来时才有值，不对外编造。
		key := ``
		if lib_i18n.IsWrappedKey(row[`name`]) {
			key = lib_i18n.UnwrapKey(row[`name`])
		}
		group.Statuses = append(group.Statuses, WorkspaceMetadataStatusModel{
			Id: id, Key: key, Name: lib_i18n.ShowStored(model.Lang, row[`name`]),
			Color: row[`color`], SortOrder: sortOrder, IsDefault: isDefault,
		})
	}
	return nil
}

// fillApiWorkspacePriorities 取各项目需求与缺陷的优先级定义。
func fillApiWorkspacePriorities(model *ListApiWorkspaceMetadataModel, position map[int]int) error {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, workspace_id, type_key, key, name, color, sort_order, is_default
		FROM `+define.TableWorkItemPriorities+`
		WHERE admin_id = $1 AND workspace_id = ANY($2)
		ORDER BY workspace_id, type_key, sort_order, id`, nil, model.AdminId, pq.Array(model.WorkspaceIds))
	if err != nil {
		logs.Error("list api workspace priorities failed: admin_id=%d, err=%s", model.AdminId, err.Error())
		return err
	}
	for _, row := range rows {
		item, err := apiWorkspaceItem(row, model, position)
		if err != nil {
			return err
		}
		if item == nil {
			continue
		}
		group := apiWorkspaceGroup(item, row[`type_key`])
		if group == nil {
			continue
		}
		id, err := metadataRowInt(row, `id`)
		if err != nil {
			return err
		}
		sortOrder, err := metadataRowInt(row, `sort_order`)
		if err != nil {
			return err
		}
		isDefault, err := metadataRowInt(row, `is_default`)
		if err != nil {
			return err
		}
		group.Priorities = append(group.Priorities, WorkspaceMetadataPriorityModel{
			Id: id, Key: row[`key`], Label: lib_i18n.ShowStored(model.Lang, row[`name`]),
			Color: row[`color`], SortOrder: sortOrder, IsDefault: isDefault,
		})
	}
	return nil
}

// fillApiWorkspaceCustomFields 取各项目的自定义字段定义，Agent 靠字段 ID 填 custom_fields。
func fillApiWorkspaceCustomFields(model *ListApiWorkspaceMetadataModel, position map[int]int) error {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, workspace_id, type_key, key, name, field_type,
		is_required, sort_order, is_enabled, config_json
		FROM `+define.TableCustomFields+`
		WHERE admin_id = $1 AND workspace_id = ANY($2)
		ORDER BY workspace_id, type_key, sort_order, id`, nil, model.AdminId, pq.Array(model.WorkspaceIds))
	if err != nil {
		logs.Error("list api workspace custom fields failed: admin_id=%d, err=%s", model.AdminId, err.Error())
		return err
	}
	for _, row := range rows {
		item, err := apiWorkspaceItem(row, model, position)
		if err != nil {
			return err
		}
		if item == nil {
			continue
		}
		group := apiWorkspaceGroup(item, row[`type_key`])
		if group == nil {
			continue
		}
		field, err := toWorkspaceMetadataCustomFieldModel(row)
		if err != nil {
			return err
		}
		group.CustomFields = append(group.CustomFields, field)
	}
	return nil
}

// fillApiWorkspaceVersions 取各项目挂载的迭代。跨项目迭代会在每个所属项目各出现一次，
// 版本行本身只转换一次，再按项目分发。
func fillApiWorkspaceVersions(model *ListApiWorkspaceMetadataModel, position map[int]int) error {
	rows, err := msql.RawValues(define.Postgres, `SELECT v.id, v.admin_id, v.workspace_id, v.name,
		v.start_date, v.end_date, v.ended_at, v.requirement_count, v.completed_count, v.created_at, v.updated_at,
		vw.workspace_id AS listed_workspace_id
		FROM versions v
		JOIN version_workspaces vw ON vw.admin_id = v.admin_id AND vw.version_id = v.id
		WHERE v.admin_id = $1 AND vw.workspace_id = ANY($2)
		ORDER BY vw.workspace_id, v.id DESC`, nil, model.AdminId, pq.Array(model.WorkspaceIds))
	if err != nil {
		logs.Error("list api workspace versions failed: admin_id=%d, err=%s", model.AdminId, err.Error())
		return err
	}
	order := make([]int, 0, len(rows))
	byVersion := make(map[int]VersionModel, len(rows))
	listed := make(map[int][]int, len(rows))
	for _, row := range rows {
		version, err := toVersionModel(row)
		if err != nil {
			return err
		}
		listedWorkspaceId, err := metadataRowInt(row, `listed_workspace_id`)
		if err != nil {
			return err
		}
		if _, ok := byVersion[version.Id]; !ok {
			order = append(order, version.Id)
			byVersion[version.Id] = version
		}
		listed[listedWorkspaceId] = append(listed[listedWorkspaceId], version.Id)
	}
	versions := make([]VersionModel, 0, len(order))
	for _, id := range order {
		versions = append(versions, byVersion[id])
	}
	if err = fillVersionWorkspaces(model.AdminId, order, versions); err != nil {
		return err
	}
	for _, version := range versions {
		byVersion[version.Id] = version
	}
	for workspaceId, ids := range listed {
		index, ok := position[workspaceId]
		if !ok {
			continue
		}
		for _, id := range ids {
			model.Items[index].Versions = append(model.Items[index].Versions, byVersion[id])
		}
	}
	return nil
}

// fillApiWorkspaceMembers 取各项目的成员，供 Agent 解析负责人 ID。
func fillApiWorkspaceMembers(model *ListApiWorkspaceMetadataModel, position map[int]int) error {
	rows, err := msql.RawValues(define.Postgres, `SELECT wm.workspace_id, wm.user_id,
		u.name AS user_name, u.username AS user_username
		FROM `+define.TableWorkspaceMembers+` wm
		LEFT JOIN users u ON u.id = wm.user_id AND u.admin_id = wm.admin_id
		WHERE wm.admin_id = $1 AND wm.workspace_id = ANY($2)
		ORDER BY wm.workspace_id, wm.id`, nil, model.AdminId, pq.Array(model.WorkspaceIds))
	if err != nil {
		logs.Error("list api workspace members failed: admin_id=%d, err=%s", model.AdminId, err.Error())
		return err
	}
	for _, row := range rows {
		item, err := apiWorkspaceItem(row, model, position)
		if err != nil {
			return err
		}
		if item == nil {
			continue
		}
		userId, err := metadataRowInt(row, `user_id`)
		if err != nil {
			return err
		}
		item.Members = append(item.Members, ApiWorkspaceMemberModel{
			UserId: userId, UserName: row[`user_name`], UserUsername: row[`user_username`],
		})
	}
	return nil
}

// fillApiWorkspaceSettings 取各项目的初始状态与终态。没有配置行时留零值，表示沿用默认流程。
func fillApiWorkspaceSettings(model *ListApiWorkspaceMetadataModel, position map[int]int) error {
	rows, err := msql.RawValues(define.Postgres, `SELECT workspace_id, requirement_initial_status_id,
		defect_initial_status_id, requirement_final_status_ids, defect_final_status_ids, requirement_completed_status_ids, defect_completed_status_ids
		FROM `+define.TableWorkspaceSettings+`
		WHERE admin_id = $1 AND workspace_id = ANY($2)`, nil, model.AdminId, pq.Array(model.WorkspaceIds))
	if err != nil {
		logs.Error("list api workspace settings failed: admin_id=%d, err=%s", model.AdminId, err.Error())
		return err
	}
	for _, row := range rows {
		item, err := apiWorkspaceItem(row, model, position)
		if err != nil {
			return err
		}
		if item == nil {
			continue
		}
		initialStatusId, err := metadataRowInt(row, `requirement_initial_status_id`)
		if err != nil {
			return err
		}
		defectInitialStatusId, err := metadataRowInt(row, `defect_initial_status_id`)
		if err != nil {
			return err
		}
		item.Settings = ApiWorkspaceSettingsModel{
			RequirementInitialStatusId:    initialStatusId,
			DefectInitialStatusId:         defectInitialStatusId,
			RequirementFinalStatusIds:     toIntSlice(row[`requirement_final_status_ids`]),
			RequirementCompletedStatusIds: toIntSlice(row[`requirement_completed_status_ids`]),
			RequirementClosedStatusIds:    statusDifference(toIntSlice(row[`requirement_final_status_ids`]), toIntSlice(row[`requirement_completed_status_ids`])),
			DefectFinalStatusIds:          toIntSlice(row[`defect_final_status_ids`]),
			DefectCompletedStatusIds:      toIntSlice(row[`defect_completed_status_ids`]),
			DefectClosedStatusIds:         statusDifference(toIntSlice(row[`defect_final_status_ids`]), toIntSlice(row[`defect_completed_status_ids`])),
		}
	}
	return nil
}

// apiWorkspaceItem 按行里的 workspace_id 定位结果项；行落在本次请求的项目之外时返回 nil 由调用方跳过。
func apiWorkspaceItem(row msql.Params, model *ListApiWorkspaceMetadataModel, position map[int]int) (*ApiWorkspaceMetadataModel, error) {
	workspaceId, err := metadataRowInt(row, `workspace_id`)
	if err != nil {
		return nil, err
	}
	index, ok := position[workspaceId]
	if !ok {
		return nil, nil
	}
	return &model.Items[index], nil
}

// apiWorkspaceGroup 按工作项类型取分组；开放 API 只对外暴露需求与缺陷，规划不在此列。
func apiWorkspaceGroup(item *ApiWorkspaceMetadataModel, typeKey string) *ApiWorkspaceGroupModel {
	switch typeKey {
	case define.TypeKeyRequirement:
		return &item.Requirement
	case define.TypeKeyDefect:
		return &item.Defect
	default:
		return nil
	}
}

func newApiWorkspaceGroup() ApiWorkspaceGroupModel {
	return ApiWorkspaceGroupModel{
		Statuses:     []WorkspaceMetadataStatusModel{},
		Priorities:   []WorkspaceMetadataPriorityModel{},
		CustomFields: []WorkspaceMetadataCustomFieldModel{},
	}
}

// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"encoding/json"
	"errors"

	"goteams/internal/app/goteams/define"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// ErrApiWorkItemNoFields 表示 PATCH 没有携带任何可修改字段。
var ErrApiWorkItemNoFields = errors.New(`api work item has no fields to update`)

// ApiWorkItemMaxPageSize 是开放 API 列表的单页上限，controller 据此拒绝超限请求，business 兜底回默认值。
const ApiWorkItemMaxPageSize = 100

// ApiMyWorkItemsModel 是开放 API「我的工作列表」用例的输入与结果。
type ApiMyWorkItemsModel struct {
	AdminId int
	UserId  int
	Page    int
	Size    int
	Items   []RequirementModel
	Total   int
}

// ApiWorkItemModel 是开放 API 对外暴露的工作项详情。
type ApiWorkItemModel struct {
	Id                 int
	WorkspaceId        int
	ParentId           int
	Title              string
	Description        string
	DescriptionMode    string
	Type               string
	StatusId           int
	StatusName         string
	Priority           int
	PriorityName       string
	VersionId          int
	VersionName        string
	AssigneeIds        []int
	AssigneeNames      []string
	PlannedStartDate   int64
	PlannedEndDate     int64
	EstimatedWorkHours float64
	CompletedWorkHours float64
	CommentCount       int
	CreatedBy          int
	CreatorName        string
	CreatedAt          int64
	UpdatedAt          int64
	CustomFields       CustomFieldValues
}

// ApiWorkItemUpdateModel 是开放 API 修改工作项用例的输入。
// 指针字段区分「未传」与显式置零，未传的列不参与 UPDATE。
type ApiWorkItemUpdateModel struct {
	AdminId            int
	WorkspaceId        int
	ItemId             int
	UserId             int
	Title              *string
	Description        *string
	DescriptionMode    *string
	StatusId           *int
	Priority           *int
	ParentId           *int
	VersionId          *int
	AssigneeIds        *[]int
	PlannedStartDate   *int64
	PlannedEndDate     *int64
	EstimatedWorkHours *float64
	CustomFields       CustomFieldValues
	// ImageFileIds 是本次要回绑的富文本素材台账 id；单独传它会被 ErrApiWorkItemNoFields 拒掉（400），
	// 必须跟其它可改字段一起传，回绑才与那次修改一起落库。
	ImageFileIds []int
}

// ListApiMyWorkItems 分页返回当前用户负责的工作项，与界面「我的工作」共用同一条 SQL 和可见范围口径。
func ListApiMyWorkItems(model ApiMyWorkItemsModel) (ApiMyWorkItemsModel, error) {
	if model.Page < 1 {
		model.Page = 1
	}
	if model.Size < 1 || model.Size > ApiWorkItemMaxPageSize {
		model.Size = 20
	}
	query := buildWorkItemListSQL(``, ``, `r.assignee_ids @> ARRAY[$2]`, myWorkOrderBySQL)

	counts, err := msql.RawValues(define.Postgres, `SELECT COUNT(*) AS total FROM (`+query+`) counted`, nil, model.AdminId, model.UserId)
	if err != nil {
		logs.Error("count api my work items failed: admin_id=%d, err=%s", model.AdminId, err.Error())
		return ApiMyWorkItemsModel{}, err
	}
	model.Total = cast.ToInt(counts[0][`total`])

	rows, err := msql.RawValues(define.Postgres, query+` LIMIT $3 OFFSET $4`, nil,
		model.AdminId, model.UserId, model.Size, (model.Page-1)*model.Size)
	if err != nil {
		logs.Error("list api my work items failed: admin_id=%d, err=%s", model.AdminId, err.Error())
		return ApiMyWorkItemsModel{}, err
	}
	model.Items = make([]RequirementModel, 0, len(rows))
	for _, row := range rows {
		item, convertErr := toRequirementModel(row)
		if convertErr != nil {
			logs.Error("convert api my work item failed: admin_id=%d, err=%s", model.AdminId, convertErr.Error())
			return ApiMyWorkItemsModel{}, convertErr
		}
		model.Items = append(model.Items, item)
	}
	return model, nil
}

// GetApiWorkItem 返回单个工作项详情；数据查看范围之外与不存在统一按 ErrRequirementNotFound 处理。
func GetApiWorkItem(adminId, workspaceId, itemId, userId int) (ApiWorkItemModel, error) {
	wsId, reqId := cast.ToString(workspaceId), cast.ToString(itemId)
	row, err := GetVisibleRequirementById(adminId, wsId, reqId, userId)
	if err != nil {
		logs.Error("get api work item failed: admin_id=%d, item_id=%d, err=%s", adminId, itemId, err.Error())
		return ApiWorkItemModel{}, err
	}
	if len(row) == 0 {
		return ApiWorkItemModel{}, ErrRequirementNotFound
	}
	model, err := apiWorkItemFromRow(row)
	if err != nil {
		return ApiWorkItemModel{}, err
	}
	values, err := ListCustomFieldValues(adminId, itemId)
	if err != nil {
		logs.Error("list api work item custom fields failed: item_id=%d, err=%s", itemId, err.Error())
		return ApiWorkItemModel{}, err
	}
	model.CustomFields = make(CustomFieldValues, len(values))
	for fieldId, value := range values {
		model.CustomFields[fieldId] = value
	}
	return model, nil
}

// UpdateApiWorkItem 把类型化的开放 API 入参翻译成 UpdateRequirement 需要的列集合。
// map[string]any 只允许存在于这一层，controller 不见 Map。
func UpdateApiWorkItem(model ApiWorkItemUpdateModel) error {
	updates := make(map[string]any, 11)
	if model.Title != nil {
		updates[`title`] = *model.Title
	}
	if model.Description != nil {
		updates[`description`] = *model.Description
	}
	if model.DescriptionMode != nil {
		updates[`description_mode`] = *model.DescriptionMode
	}
	if model.StatusId != nil {
		updates[`status_id`] = *model.StatusId
	}
	if model.Priority != nil {
		updates[`priority`] = *model.Priority
	}
	if model.ParentId != nil {
		updates[`parent_id`] = *model.ParentId
	}
	if model.VersionId != nil {
		updates[`version_id`] = *model.VersionId
	}
	if model.AssigneeIds != nil {
		updates[`assignee_ids`] = *model.AssigneeIds
	}
	if model.PlannedStartDate != nil {
		updates[`planned_start_date`] = *model.PlannedStartDate
	}
	if model.PlannedEndDate != nil {
		updates[`planned_end_date`] = *model.PlannedEndDate
	}
	if model.EstimatedWorkHours != nil {
		updates[`work_hours`] = *model.EstimatedWorkHours
	}
	if len(updates) == 0 && len(model.CustomFields) == 0 {
		return ErrApiWorkItemNoFields
	}
	return UpdateRequirement(model.AdminId, cast.ToString(model.WorkspaceId), cast.ToString(model.ItemId),
		updates, model.CustomFields, model.UserId, model.ImageFileIds)
}

// apiWorkItemFromRow 把详情行转成开放 API 模型；数字列解析失败即整行失败，不静默给零值。
func apiWorkItemFromRow(row msql.Params) (ApiWorkItemModel, error) {
	model := ApiWorkItemModel{
		Title:           row[`title`],
		Description:     row[`description`],
		DescriptionMode: row[`description_mode`],
		Type:            row[`type`],
		StatusName:      row[`status_name`],
		PriorityName:    row[`priority_name`],
		VersionName:     row[`version_name`],
		CreatorName:     row[`creator_name`],
	}
	var err error
	number := func(column string) int64 {
		if err != nil {
			return 0
		}
		value, parseErr := parseRequirementInt64(row, column)
		if parseErr != nil {
			err = parseErr
		}
		return value
	}
	model.Id = int(number(`id`))
	model.WorkspaceId = int(number(`workspace_id`))
	model.ParentId = int(number(`parent_id`))
	model.StatusId = int(number(`status_id`))
	model.Priority = int(number(`priority`))
	model.VersionId = int(number(`version_id`))
	model.CommentCount = int(number(`comment_count`))
	model.CreatedBy = int(number(`created_by`))
	model.PlannedStartDate = number(`planned_start_date`)
	model.PlannedEndDate = number(`planned_end_date`)
	model.CreatedAt = number(`created_at`)
	model.UpdatedAt = number(`updated_at`)
	if err != nil {
		return ApiWorkItemModel{}, err
	}
	if model.EstimatedWorkHours, err = parseRequirementFloat64(row, `work_hours`); err != nil {
		return ApiWorkItemModel{}, err
	}
	if model.CompletedWorkHours, err = parseRequirementFloat64(row, `completed_work_hours`); err != nil {
		return ApiWorkItemModel{}, err
	}
	if err = json.Unmarshal([]byte(row[`assignee_ids`]), &model.AssigneeIds); err != nil {
		return ApiWorkItemModel{}, err
	}
	if err = json.Unmarshal([]byte(row[`assignee_names`]), &model.AssigneeNames); err != nil {
		return ApiWorkItemModel{}, err
	}
	return model, nil
}

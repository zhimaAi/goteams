// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"
	"goteams/internal/pkg/lib_storage"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

// apiWorkItemResponse 是开放 API 里工作项的通用表示；列表类端点与详情共用同一份对外契约并同步演进，
// 详情在其上追加只在单条读取时返回的字段。ID 字段一律是内部自增主键，0 表示未设置。
type apiWorkItemResponse struct {
	Id                 int      `json:"id,omitempty"`
	WorkspaceId        int      `json:"workspace_id,omitempty"`
	ParentId           int      `json:"parent_id,omitempty"`
	Title              string   `json:"title"`
	Type               string   `json:"type"`
	StatusId           int      `json:"status_id,omitempty"`
	StatusName         string   `json:"status_name"`
	PriorityId         int      `json:"priority_id,omitempty"`
	PriorityName       string   `json:"priority_name"`
	VersionId          int      `json:"version_id,omitempty"`
	VersionName        string   `json:"version_name,omitempty"`
	AssigneeIds        []int    `json:"assignee_ids,omitempty"`
	AssigneeNames      []string `json:"assignee_names,omitempty"`
	PlannedStartDate   int64    `json:"planned_start_date"`
	PlannedEndDate     int64    `json:"planned_end_date"`
	EstimatedWorkHours float64  `json:"estimated_work_hours"`
	CompletedWorkHours float64  `json:"completed_work_hours"`
	WorkspaceName      string   `json:"workspace_name,omitempty"`
	CreatedAt          int64    `json:"created_at"`
	UpdatedAt          int64    `json:"updated_at"`
}

type apiWorkItemDetailResponse struct {
	apiWorkItemResponse
	Description     string            `json:"description"`
	DescriptionMode string            `json:"description_mode"`
	CommentCount    int               `json:"comment_count"`
	CreatorName     string            `json:"creator_name"`
	CustomFields    map[string]string `json:"custom_fields,omitempty"`
}

// apiWorkItemListResponse 是分页工作项列表的对外契约，「我的工作」与「项目内列表」共用并同步演进。
type apiWorkItemListResponse struct {
	List  []apiWorkItemResponse `json:"list"`
	Total int                   `json:"total"`
	Page  int                   `json:"page"`
	Size  int                   `json:"size"`
}

type apiWorkItemPageQuery struct {
	Page int `form:"page"`
	Size int `form:"size"`
}

// apiWorkItemListQuery 是项目内工作项列表的查询参数，type 为空表示不限类型。
type apiWorkItemListQuery struct {
	Type string `form:"type"`
	Page int    `form:"page"`
	Size int    `form:"size"`
}

type apiWorkspaceResponse struct {
	Id          int                          `json:"id"`
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Color       string                       `json:"color"`
	Icon        string                       `json:"icon"`
	CreatedAt   int64                        `json:"created_at"`
	UpdatedAt   int64                        `json:"updated_at"`
	Metadata    apiWorkspaceMetadataResponse `json:"metadata"`
}

// apiWorkspaceStatusResponse 是一个状态选项；id 是写工作项 status_id 时要回填的状态 ID。
type apiWorkspaceStatusResponse struct {
	Id        int    `json:"id"`
	Key       string `json:"key,omitempty"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
	IsDefault bool   `json:"is_default"`
}

// apiWorkspacePriorityResponse 是一个优先级选项；id 是写工作项 priority_id 时要回填的优先级 ID。
type apiWorkspacePriorityResponse struct {
	Id        int    `json:"id"`
	Key       string `json:"key,omitempty"`
	Label     string `json:"label"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
	IsDefault bool   `json:"is_default"`
}

type apiWorkspaceCustomFieldOptionResponse struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// apiWorkspaceCustomFieldResponse 是一个自定义字段定义；id 是 custom_fields 的键，options 是可选值。
type apiWorkspaceCustomFieldResponse struct {
	Id         int                                     `json:"id"`
	Key        string                                  `json:"key"`
	Name       string                                  `json:"name"`
	FieldType  string                                  `json:"field_type"`
	IsRequired bool                                    `json:"is_required"`
	IsEnabled  bool                                    `json:"is_enabled"`
	Options    []apiWorkspaceCustomFieldOptionResponse `json:"options,omitempty"`
}

// apiWorkspaceGroupResponse 是一种工作项类型（需求或缺陷）的可选项集合。
type apiWorkspaceGroupResponse struct {
	Statuses     []apiWorkspaceStatusResponse      `json:"statuses"`
	Priorities   []apiWorkspacePriorityResponse    `json:"priorities"`
	CustomFields []apiWorkspaceCustomFieldResponse `json:"custom_fields"`
}

// apiWorkspaceVersionResponse 是一个迭代选项；id 是写工作项 version_id 时要回填的迭代 ID。
type apiWorkspaceVersionResponse struct {
	Id            int    `json:"id"`
	Name          string `json:"name"`
	StartDate     int64  `json:"start_date"`
	EndDate       int64  `json:"end_date"`
	IsEnded       bool   `json:"is_ended"`
	IterationType string `json:"iteration_type"`
}

// apiWorkspaceMemberResponse 是一个可指派成员；user_id 是写工作项 assignee_ids 时要回填的用户 ID。
type apiWorkspaceMemberResponse struct {
	UserId       int    `json:"user_id"`
	UserName     string `json:"user_name"`
	UserUsername string `json:"user_username,omitempty"`
}

// apiWorkspaceSettingsResponse 是状态配置：初始状态与终态，终态列表即流程结束的状态。
type apiWorkspaceSettingsResponse struct {
	RequirementInitialStatusId    int   `json:"requirement_initial_status_id,omitempty"`
	DefectInitialStatusId         int   `json:"defect_initial_status_id,omitempty"`
	RequirementFinalStatusIds     []int `json:"requirement_final_status_ids,omitempty"`
	RequirementCompletedStatusIds []int `json:"requirement_completed_status_ids,omitempty"`
	RequirementClosedStatusIds    []int `json:"requirement_closed_status_ids,omitempty"`
	DefectFinalStatusIds          []int `json:"defect_final_status_ids,omitempty"`
	DefectCompletedStatusIds      []int `json:"defect_completed_status_ids,omitempty"`
	DefectClosedStatusIds         []int `json:"defect_closed_status_ids,omitempty"`
}

// apiWorkspaceMetadataResponse 是项目列表里每条项目带的可选项，Agent 据此拿到写工作项字段所需的全部 ID。
type apiWorkspaceMetadataResponse struct {
	Requirement apiWorkspaceGroupResponse     `json:"requirement"`
	Defect      apiWorkspaceGroupResponse     `json:"defect"`
	Versions    []apiWorkspaceVersionResponse `json:"versions"`
	Members     []apiWorkspaceMemberResponse  `json:"members"`
	Settings    apiWorkspaceSettingsResponse  `json:"settings"`
}

type apiIdResponse struct {
	Id int `json:"id,omitempty"`
}

// createApiWorkItemRequest 的 ID 字段一律是内部自增主键，0 表示未提供。
// image_file_ids 是本 Key 先调素材上传接口拿到的素材 ID：先上传再新建，文件才会回绑到新工作项上。
type createApiWorkItemRequest struct {
	Title              string            `json:"title" binding:"required"`
	Description        string            `json:"description"`
	DescriptionMode    string            `json:"description_mode"`
	Type               string            `json:"type"`
	StatusId           int               `json:"status_id"`
	PriorityId         int               `json:"priority_id"`
	ParentId           int               `json:"parent_id"`
	VersionId          int               `json:"version_id"`
	AssigneeIds        []int             `json:"assignee_ids"`
	PlannedStartDate   int64             `json:"planned_start_date"`
	PlannedEndDate     int64             `json:"planned_end_date"`
	EstimatedWorkHours float64           `json:"estimated_work_hours"`
	CustomFields       map[string]string `json:"custom_fields"`
	ImageFileIds       []int             `json:"image_file_ids"`
}

// updateApiWorkItemRequest 全部使用指针以区分「未传」与显式置零；type 不可修改，故不在契约内。
type updateApiWorkItemRequest struct {
	Title              *string           `json:"title"`
	Description        *string           `json:"description"`
	DescriptionMode    *string           `json:"description_mode"`
	StatusId           int               `json:"status_id"`
	PriorityId         int               `json:"priority_id"`
	ParentId           int               `json:"parent_id"`
	VersionId          int               `json:"version_id"`
	AssigneeIds        *[]int            `json:"assignee_ids"`
	PlannedStartDate   *int64            `json:"planned_start_date"`
	PlannedEndDate     *int64            `json:"planned_end_date"`
	EstimatedWorkHours *float64          `json:"estimated_work_hours"`
	CustomFields       map[string]string `json:"custom_fields"`
	ImageFileIds       []int             `json:"image_file_ids"`
}

type createApiWorkItemCommentRequest struct {
	Content string `json:"content" binding:"required"`
}

// ListApiMyWork 返回 token 归属用户负责的工作项，跨项目，与界面「我的工作」同口径分页。
func ListApiMyWork(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var q apiWorkItemPageQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `api.my_work.query`, err)
		return
	}
	if q.Size > business.ApiWorkItemMaxPageSize || q.Size < 0 || q.Page < 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `api.my_work.page`, nil)
		return
	}
	if !common.ApiQuotaCharge(c) {
		return
	}
	model, err := business.ListApiMyWorkItems(business.ApiMyWorkItemsModel{
		AdminId: c.GetInt(`adminId`),
		UserId:  c.GetInt(`userId`),
		Page:    q.Page,
		Size:    q.Size,
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `api.my_work.list`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: toApiWorkItemListResponse(toApiWorkItemResponses(model.Items, lang), model.Total, model.Page, model.Size),
	})
}

// ListApiWorkspaceWorkItems 分页返回指定项目内的工作项，叠加项目成员与按类型的角色查看权限。
// 数据范围由 business.ListRequirements 按项目角色的查看范围收窄，只能看到该用户有权查看的工作项。
func ListApiWorkspaceWorkItems(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, ok := apiWorkspaceParam(c, lang)
	if !ok {
		return
	}
	if !apiEnsureWorkspaceMember(c, lang, wsId) {
		return
	}
	var q apiWorkItemListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `api.work_items.query`, err)
		return
	}
	if q.Size > business.ApiWorkItemMaxPageSize || q.Size < 0 || q.Page < 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `api.work_items.page`, nil)
		return
	}
	// 未指定类型时按「任一类型可见」放行，与界面列表的 withAnyPerm 口径一致。
	adminId, userId, wsIdStr := c.GetInt(`adminId`), c.GetInt(`userId`), strconv.Itoa(wsId)
	if q.Type == `` {
		if !hasWorkItemPermission(adminId, userId, wsIdStr, define.TypeKeyRequirement, `view`) &&
			!hasWorkItemPermission(adminId, userId, wsIdStr, define.TypeKeyDefect, `view`) {
			abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
			return
		}
	} else if !hasWorkItemPermission(adminId, userId, wsIdStr, q.Type, `view`) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}

	if !common.ApiQuotaCharge(c) {
		return
	}
	model, err := business.ListRequirements(business.ListRequirementsModel{
		AdminId:     adminId,
		UserId:      userId,
		WorkspaceId: wsId,
		Type:        q.Type,
		Page:        q.Page,
		Size:        q.Size,
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `api.work_items.list`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: toApiWorkItemListResponse(toApiWorkItemResponses(model.Items, lang), model.Total, model.Page, model.Size),
	})
}

// ListApiWorkspaces 返回 token 归属用户可见的项目列表，每条项目带上写工作项字段所需的选项 ID。
func ListApiWorkspaces(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	if !common.ApiQuotaCharge(c) {
		return
	}
	model, err := business.ListWorkspaces(business.ListWorkspacesModel{
		AdminId: c.GetInt(`adminId`),
		UserId:  c.GetInt(`userId`),
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `api.workspaces.list`, err)
		return
	}
	workspaceIds := make([]int, 0, len(model.Items))
	for _, item := range model.Items {
		workspaceIds = append(workspaceIds, item.Id)
	}
	metadata, err := business.ListApiWorkspaceMetadata(business.ListApiWorkspaceMetadataModel{
		AdminId:      c.GetInt(`adminId`),
		Lang:         lang,
		WorkspaceIds: workspaceIds,
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `api.workspaces.metadata`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: toApiWorkspaceResponses(model.Items, metadata.Items, lang),
	})
}

// toApiWorkspaceResponses 按 workspace_id 把可选项并回项目列表，两者的顺序不保证一致。
func toApiWorkspaceResponses(items []business.WorkspaceModel, metadata []business.ApiWorkspaceMetadataModel, lang string) []apiWorkspaceResponse {
	byWorkspace := make(map[int]business.ApiWorkspaceMetadataModel, len(metadata))
	for _, item := range metadata {
		byWorkspace[item.WorkspaceId] = item
	}
	list := make([]apiWorkspaceResponse, 0, len(items))
	for _, item := range items {
		list = append(list, apiWorkspaceResponse{
			Id: item.Id, Name: item.Name, Description: item.Description,
			Color: item.Color, Icon: item.Icon, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
			Metadata: toApiWorkspaceMetadataResponse(byWorkspace[item.Id]),
		})
	}
	return list
}

func toApiWorkspaceMetadataResponse(model business.ApiWorkspaceMetadataModel) apiWorkspaceMetadataResponse {
	versions := make([]apiWorkspaceVersionResponse, 0, len(model.Versions))
	for _, version := range model.Versions {
		versions = append(versions, apiWorkspaceVersionResponse{
			Id: version.Id, Name: version.Name, StartDate: version.StartDate,
			EndDate: version.EndDate, IsEnded: version.EndedAt > 0, IterationType: version.IterationType(),
		})
	}
	members := make([]apiWorkspaceMemberResponse, 0, len(model.Members))
	for _, member := range model.Members {
		members = append(members, apiWorkspaceMemberResponse{
			UserId: member.UserId, UserName: member.UserName, UserUsername: member.UserUsername,
		})
	}
	return apiWorkspaceMetadataResponse{
		Requirement: toApiWorkspaceGroupResponse(model.Requirement),
		Defect:      toApiWorkspaceGroupResponse(model.Defect),
		Versions:    versions,
		Members:     members,
		Settings: apiWorkspaceSettingsResponse{
			RequirementInitialStatusId:    model.Settings.RequirementInitialStatusId,
			DefectInitialStatusId:         model.Settings.DefectInitialStatusId,
			RequirementFinalStatusIds:     model.Settings.RequirementFinalStatusIds,
			RequirementCompletedStatusIds: model.Settings.RequirementCompletedStatusIds,
			RequirementClosedStatusIds:    model.Settings.RequirementClosedStatusIds,
			DefectFinalStatusIds:          model.Settings.DefectFinalStatusIds,
			DefectCompletedStatusIds:      model.Settings.DefectCompletedStatusIds,
			DefectClosedStatusIds:         model.Settings.DefectClosedStatusIds,
		},
	}
}

func toApiWorkspaceGroupResponse(model business.ApiWorkspaceGroupModel) apiWorkspaceGroupResponse {
	statuses := make([]apiWorkspaceStatusResponse, 0, len(model.Statuses))
	for _, status := range model.Statuses {
		statuses = append(statuses, apiWorkspaceStatusResponse{
			Id: status.Id, Key: status.Key, Name: status.Name, Color: status.Color,
			SortOrder: status.SortOrder, IsDefault: status.IsDefault == 1,
		})
	}
	priorities := make([]apiWorkspacePriorityResponse, 0, len(model.Priorities))
	for _, priority := range model.Priorities {
		priorities = append(priorities, apiWorkspacePriorityResponse{
			Id: priority.Id, Key: priority.Key, Label: priority.Label, Color: priority.Color,
			SortOrder: priority.SortOrder, IsDefault: priority.IsDefault == 1,
		})
	}
	customFields := make([]apiWorkspaceCustomFieldResponse, 0, len(model.CustomFields))
	for _, field := range model.CustomFields {
		options := make([]apiWorkspaceCustomFieldOptionResponse, 0, len(field.Options))
		for _, option := range field.Options {
			options = append(options, apiWorkspaceCustomFieldOptionResponse{
				Key: option.Key, Name: option.Name, Color: option.Color,
			})
		}
		customFields = append(customFields, apiWorkspaceCustomFieldResponse{
			Id: field.Id, Key: field.Key, Name: field.Name, FieldType: field.FieldType,
			IsRequired: field.IsRequired == 1, IsEnabled: field.IsEnabled == 1, Options: options,
		})
	}
	return apiWorkspaceGroupResponse{Statuses: statuses, Priorities: priorities, CustomFields: customFields}
}

// GetApiWorkItem 返回单条工作项详情，叠加项目成员、角色权限与数据查看范围三层校验。
func GetApiWorkItem(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, itemId, ok := apiWorkItemParams(c, lang)
	if !ok {
		return
	}
	if !apiEnsureWorkspaceMember(c, lang, wsId) {
		return
	}
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	model, err := business.GetApiWorkItem(adminId, wsId, itemId, userId)
	if err != nil {
		abortApiWorkItemError(c, lang, `api.work_item.get`, err)
		return
	}
	if !hasWorkItemPermission(adminId, userId, strconv.Itoa(wsId), model.Type, `view`) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}
	if !common.ApiQuotaCharge(c) {
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: toApiWorkItemDetailResponse(model, lang),
	})
}

// CreateApiWorkItem 在指定项目内创建工作项，以 token 归属用户身份执行，权限与其本人一致。
func CreateApiWorkItem(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, ok := apiWorkspaceParam(c, lang)
	if !ok {
		return
	}
	if !apiEnsureWorkspaceMember(c, lang, wsId) {
		return
	}
	var req createApiWorkItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), `api.work_item.create.bind`, err)
		return
	}
	customFields, err := apiParseCustomFields(req.CustomFields)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgApiCustomFieldInvalid), `api.work_item.create.custom_fields`, err)
		return
	}
	if err = business.ValidateApiWorkItemPendingFiles(c.GetInt(`adminId`), wsId, c.GetInt(`userId`), req.ImageFileIds); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `api.work_item.create.image_file_ids`, err)
		return
	}
	descriptionMode := req.DescriptionMode
	if descriptionMode == `` {
		descriptionMode = define.DescriptionModeRichText
	}
	itemType := req.Type
	if itemType == `` {
		itemType = define.TypeKeyRequirement
	}
	if err = validate.ValidateRequirementCreate(req.Title, req.Type, req.PriorityId); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `api.work_item.create.validate`, err)
		return
	}
	if err = validate.ValidateRequirementDescriptionMode(descriptionMode); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `api.work_item.create.description_mode`, err)
		return
	}
	if err = validate.ValidateRequirementWorkHours(req.EstimatedWorkHours); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgRequirementWorkHoursInvalid), `api.work_item.create.work_hours`, err)
		return
	}
	if !hasWorkItemPermission(c.GetInt(`adminId`), c.GetInt(`userId`), strconv.Itoa(wsId), itemType, `create`) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}

	if !common.ApiQuotaCharge(c) {
		return
	}
	model, err := business.CreateRequirement(business.CreateRequirementModel{
		AdminId:            c.GetInt(`adminId`),
		UserId:             c.GetInt(`userId`),
		WorkspaceId:        wsId,
		Title:              req.Title,
		Description:        req.Description,
		DescriptionMode:    descriptionMode,
		Type:               req.Type,
		Priority:           apiOptionalInt(req.PriorityId),
		StatusId:           req.StatusId,
		ParentId:           req.ParentId,
		VersionId:          req.VersionId,
		AssigneeIds:        req.AssigneeIds,
		PlannedStartDate:   req.PlannedStartDate,
		PlannedEndDate:     req.PlannedEndDate,
		EstimatedWorkHours: req.EstimatedWorkHours,
		CustomFields:       customFields,
		ImageFileIds:       req.ImageFileIds,
	})
	if err != nil {
		abortApiWorkItemWriteError(c, lang, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: apiIdResponse{Id: model.Id},
	})
}

// UpdateApiWorkItem 修改工作项，只更新请求里出现的字段。
func UpdateApiWorkItem(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, itemId, ok := apiWorkItemParams(c, lang)
	if !ok {
		return
	}
	if !apiEnsureWorkspaceMember(c, lang, wsId) {
		return
	}
	var req updateApiWorkItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), `api.work_item.update.bind`, err)
		return
	}
	// 写路径与界面一致：先确认条目存在，再按条目类型校验编辑权限。
	if !ensureRequirementPermission(c, lang, wsId, itemId, `edit`) {
		return
	}
	if !common.ApiQuotaCharge(c) {
		return
	}
	customFields, err := apiParseCustomFields(req.CustomFields)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgApiCustomFieldInvalid), `api.work_item.update.custom_fields`, err)
		return
	}
	if err = business.ValidateApiWorkItemPendingFiles(c.GetInt(`adminId`), wsId, c.GetInt(`userId`), req.ImageFileIds); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `api.work_item.update.image_file_ids`, err)
		return
	}
	if req.DescriptionMode != nil {
		if err = validate.ValidateRequirementDescriptionMode(*req.DescriptionMode); err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `api.work_item.update.description_mode`, err)
			return
		}
	}
	if req.EstimatedWorkHours != nil {
		if err = validate.ValidateRequirementWorkHours(*req.EstimatedWorkHours); err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgRequirementWorkHoursInvalid), `api.work_item.update.work_hours`, err)
			return
		}
	}

	model := business.ApiWorkItemUpdateModel{
		AdminId:            c.GetInt(`adminId`),
		UserId:             c.GetInt(`userId`),
		WorkspaceId:        wsId,
		ItemId:             itemId,
		Title:              req.Title,
		Description:        req.Description,
		DescriptionMode:    req.DescriptionMode,
		StatusId:           apiOptionalInt(req.StatusId),
		Priority:           apiOptionalInt(req.PriorityId),
		ParentId:           apiOptionalInt(req.ParentId),
		VersionId:          apiOptionalInt(req.VersionId),
		AssigneeIds:        req.AssigneeIds,
		PlannedStartDate:   req.PlannedStartDate,
		PlannedEndDate:     req.PlannedEndDate,
		EstimatedWorkHours: req.EstimatedWorkHours,
		CustomFields:       customFields,
		ImageFileIds:       req.ImageFileIds,
	}
	if err = business.UpdateApiWorkItem(model); err != nil {
		abortApiWorkItemWriteError(c, lang, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// CreateApiWorkItemComment 为工作项添加评论，权限口径与界面评论一致（需要条目编辑权限）。
func CreateApiWorkItemComment(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, itemId, ok := apiWorkItemParams(c, lang)
	if !ok {
		return
	}
	if !apiEnsureWorkspaceMember(c, lang, wsId) {
		return
	}
	var req createApiWorkItemCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), `api.comment.create.bind`, err)
		return
	}
	if strings.TrimSpace(req.Content) == `` {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `api.comment.create.content`, nil)
		return
	}
	if !ensureRequirementPermission(c, lang, wsId, itemId, `edit`) {
		return
	}
	if !common.ApiQuotaCharge(c) {
		return
	}
	model, err := business.CreateComment(business.CreateCommentModel{
		AdminId:       c.GetInt(`adminId`),
		WorkspaceId:   wsId,
		RequirementId: itemId,
		UserId:        c.GetInt(`userId`),
		Content:       req.Content,
	})
	if err != nil {
		abortApiWorkItemWriteError(c, lang, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: apiIdResponse{Id: model.Id},
	})
}

// UploadApiWorkItemImage 上传富文本图片。与界面新建工作项时插入图片走同一条链路：
// 返回素材地址与素材 ID，地址写进 description，素材 ID 放进新建/修改的 image_file_ids。
func UploadApiWorkItemImage(c *gin.Context) {
	uploadApiWorkItemAsset(c, business.ApiWorkItemAssetImage)
}

// UploadApiWorkItemFile 上传富文本文件。
func UploadApiWorkItemFile(c *gin.Context) {
	uploadApiWorkItemAsset(c, business.ApiWorkItemAssetFile)
}

// UploadApiWorkItemVideo 上传富文本视频。
func UploadApiWorkItemVideo(c *gin.Context) {
	uploadApiWorkItemAsset(c, business.ApiWorkItemAssetVideo)
}

// apiAssetResponse 是素材上传的返回：地址写进正文，素材 ID 放进 image_file_ids。
type apiAssetResponse struct {
	FileId int    `json:"file_id"`
	Url    string `json:"url"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
}

// uploadApiWorkItemAsset 三类素材共用的处理：校验 → 占额度 → 落存储。
// 先校验再计费：类型与大小不对属于参数错误（400），不该扣次数；额度不足（429）时不落任何文件，不留孤儿。
func uploadApiWorkItemAsset(c *gin.Context, kind string) {
	lang := lib_i18n.GetLang(c)
	wsId, ok := apiWorkspaceParam(c, lang)
	if !ok {
		return
	}
	if !apiEnsureWorkspaceMember(c, lang, wsId) {
		return
	}
	file, err := c.FormFile(`file`)
	if err != nil || file == nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `api.asset.form`, err)
		return
	}
	missing, tooLarge, typeInvalid := apiAssetMessages(kind)
	if err = business.ValidateApiWorkItemAsset(kind, file); err != nil {
		switch {
		case errors.Is(err, business.ErrApiAssetMissing):
			abortBadRequest(c, lib_i18n.Show(lang, missing), `api.asset.missing`, err)
		case errors.Is(err, business.ErrApiAssetTooLarge):
			abortBadRequest(c, lib_i18n.Show(lang, tooLarge), `api.asset.size`, err)
		case errors.Is(err, business.ErrApiAssetType):
			abortBadRequest(c, lib_i18n.Show(lang, typeInvalid), `api.asset.type`, err)
		default:
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `api.asset.kind`, err)
		}
		return
	}
	// 上传算一次接口调用：占租户当日额度，成功后计入该 token 的今日/昨日/累计。
	if !common.ApiQuotaCharge(c) {
		return
	}
	assetUrl, fileId, err := business.UploadApiWorkItemAsset(c.GetInt(`adminId`), wsId, c.GetInt(`userId`), file)
	if err != nil {
		// 存储额度不足与界面同一套语义（403 + 配额维度，前端据此弹升级）。
		if abortQuotaExceeded(c, lib_i18n.Show(lang, define.MsgQuotaExceeded), `api.asset.upload`, err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `api.asset.upload`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: apiAssetResponse{FileId: fileId, Url: assetUrl, Name: file.Filename, Size: file.Size},
	})
}

// apiAssetMessages 返回某类素材在缺失、超限、类型不符时对应的 i18n key，与界面提示同一套。
func apiAssetMessages(kind string) (missing, tooLarge, typeInvalid string) {
	switch kind {
	case business.ApiWorkItemAssetFile:
		return define.MsgFileRequired, define.MsgFileTooLarge, define.MsgFileTypeInvalid
	case business.ApiWorkItemAssetVideo:
		return define.MsgVideoRequired, define.MsgVideoTooLarge, define.MsgVideoTypeInvalid
	default:
		return define.MsgImageRequired, define.MsgImageTooLarge, define.MsgImageTypeInvalid
	}
}

// DownloadApiWorkItemFile 下载工作项里的图片/文件/视频。入参是工作项 ID 与素材地址，
// 地址就是详情 description 里 img/video 的 src（也是附件列表返回的 url），原样传入即可，不用自己拼域名。
// 鉴权口径与详情一致：work_item:view + 项目成员 + 条目类型对应的角色查看权限。
func DownloadApiWorkItemFile(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, itemId, ok := apiWorkItemParams(c, lang)
	if !ok {
		return
	}
	if !apiEnsureWorkspaceMember(c, lang, wsId) {
		return
	}
	src := strings.TrimSpace(c.Query(`src`))
	if src == `` {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `api.work_item.file.src`, nil)
		return
	}
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	// 先确认条目可见，与详情同一套判定：越权一律 404/403，不泄露条目是否存在。
	model, err := business.GetApiWorkItem(adminId, wsId, itemId, userId)
	if err != nil {
		abortApiWorkItemError(c, lang, `api.work_item.file`, err)
		return
	}
	if !hasWorkItemPermission(adminId, userId, strconv.Itoa(wsId), model.Type, `view`) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}
	// 先解析素材再计费：地址不属于本工作项（404）与越权一样不占额度，口径同 ApiQuotaCharge 的说明。
	file, err := business.GetApiWorkItemFile(adminId, wsId, itemId, src)
	if err != nil {
		if errors.Is(err, business.ErrAttachmentNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgAttachmentNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `api.work_item.file.query`, err)
		return
	}
	// 再校验一次存储归属：即使地址里的路径被伪造，也读不到不属于本租户的对象。
	if !lib_storage.M.IsOwned(adminId, lib_storage.BusinessRequirement, file.StoredPath) {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgAttachmentNotFound))
		return
	}
	// 下载算一次接口调用：占租户当日额度，成功后计入该 token 的今日/昨日/累计。
	if !common.ApiQuotaCharge(c) {
		return
	}
	c.Header(`Content-Disposition`, `attachment; filename*=UTF-8''`+url.PathEscape(file.Name))
	if file.ContentType != `` {
		c.Header(`Content-Type`, file.ContentType)
	}
	if serveErr := lib_storage.M.Serve(file.StoredPath, c); serveErr != nil {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgAttachmentNotFound))
	}
}

// apiWorkspaceParam 解析路径里的项目 ID，非十进制或非正数一律映射 api_id_invalid。
func apiWorkspaceParam(c *gin.Context, lang string) (int, bool) {
	wsId, ok := apiParsePathId(c.Param(`wsId`))
	if !ok {
		abortApiIdInvalid(c, lang, `api.workspace.param`)
		return 0, false
	}
	return wsId, true
}

// apiWorkItemParams 解析项目与工作项两个路径 ID。
func apiWorkItemParams(c *gin.Context, lang string) (int, int, bool) {
	wsId, ok := apiWorkspaceParam(c, lang)
	if !ok {
		return 0, 0, false
	}
	itemId, ok := apiParsePathId(c.Param(`itemId`))
	if !ok {
		abortApiIdInvalid(c, lang, `api.work_item.param`)
		return 0, 0, false
	}
	return wsId, itemId, true
}

// apiEnsureWorkspaceMember 校验 token 归属用户是该项目的 owner 或成员；这与角色权限、token scope 正交，
// 目的是挡住拿 token 者枚举非本人项目的条目。
// 超管对全部项目放行，成员校验在这里不再兜底，所以必须单独确认项目存在：否则不存在的 wsId 会在
// 列表返回空页、写路径一路插到数据库才失败（500），调用方无法区分「项目不存在」与「项目是空的」。
// 非超管仍只返回 403，不区分「项目不存在」与「不是成员」，避免拿 token 探测同租户里有哪些项目。
func apiEnsureWorkspaceMember(c *gin.Context, lang string, wsId int) bool {
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	wsIdStr := strconv.Itoa(wsId)
	if business.IsTenantOwner(adminId, userId) {
		workspace, err := business.GetWorkspaceById(adminId, wsIdStr)
		if err != nil {
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `api.workspace.exists`, err)
			return false
		}
		if len(workspace) == 0 {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgWorkspaceNotFound))
			return false
		}
		return true
	}
	if business.IsWorkspaceMember(adminId, userId, wsIdStr) {
		return true
	}
	abortForbidden(c, lib_i18n.Show(lang, define.MsgApiNotInWorkspace))
	return false
}

// apiParsePathId 解析路径里的十进制 ID。路径参数只接受裸主键，非数字或非正数都算 ID 非法。
func apiParsePathId(text string) (int, bool) {
	id, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// errApiCustomFieldKey 表示 custom_fields 的键不是十进制字段 ID。JSON 对象的键只能是字符串，
// 所以字段 ID 在 custom_fields 里以十进制字符串出现，解析失败按「自定义字段无效」返回。
var errApiCustomFieldKey = errors.New(`api: custom field key is not a numeric field id`)

// apiParseCustomFields 把对外的自定义字段值还原成内部 field_id → 值 的映射。
func apiParseCustomFields(values map[string]string) (business.CustomFieldValues, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make(business.CustomFieldValues, len(values))
	for key, value := range values {
		fieldId, ok := apiParsePathId(key)
		if !ok {
			return nil, errApiCustomFieldKey
		}
		out[strconv.Itoa(fieldId)] = value
	}
	return out, nil
}

// apiOptionalInt 把「未提供」与显式 0 统一按未提供处理：关系类 ID 为 0 在业务侧本就表示不设置。
func apiOptionalInt(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}

func abortApiIdInvalid(c *gin.Context, lang, stage string) {
	abortBadRequest(c, lib_i18n.Show(lang, define.MsgApiIdInvalid), stage, nil)
}

// abortApiWorkItemError 处理读取用例：数据查看范围之外与不存在统一 404，不泄漏条目是否存在。
func abortApiWorkItemError(c *gin.Context, lang, stage string, err error) {
	if errors.Is(err, business.ErrRequirementNotFound) {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
		return
	}
	abortInternal(c, lib_i18n.Show(lang, define.MsgError), stage, err)
}

// abortApiWorkItemWriteError 把写入用例的业务错误映射到稳定的对外状态，与界面用例同一套口径。
func abortApiWorkItemWriteError(c *gin.Context, lang string, err error) {
	stage := `api.work_item.write`
	if abortQuotaExceeded(c, lib_i18n.Show(lang, define.MsgQuotaExceeded), stage, err) {
		return
	}
	switch {
	case errors.Is(err, business.ErrApiWorkItemNoFields):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage, err)
	case errors.Is(err, business.ErrRequirementNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
	case errors.Is(err, validate.ErrPlannedDateRangeInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgPlannedDateRangeInvalid), stage, err)
	case errors.Is(err, validate.ErrRequirementWorkHoursInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgRequirementWorkHoursInvalid), stage, err)
	case errors.Is(err, business.ErrRequirementRelationInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgRequirementRelationInvalid), stage, err)
	case errors.Is(err, business.ErrRequirementCategoryNotFound):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgRequirementCategoryNotFound), stage, err)
	case errors.Is(err, business.ErrPriorityNotFound):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgPriorityNotFound), stage, err)
	case errors.Is(err, business.ErrStatusNotFound):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgStatusNotFound), stage, err)
	case errors.Is(err, business.ErrVersionNotFound):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgVersionNotFound), stage, err)
	case errors.Is(err, business.ErrVersionEnded):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgVersionEnded), stage, err)
	case errors.Is(err, business.ErrCustomFieldValueInvalid):
		abortBadRequest(c, business.CustomFieldValidationMessage(lang, err), stage, err)
	case errors.Is(err, business.ErrRequirementAssigneesInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage, err)
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), stage, err)
	}
}

func toApiWorkItemResponses(items []business.RequirementModel, lang string) []apiWorkItemResponse {
	out := make([]apiWorkItemResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toApiWorkItemResponse(item, lang))
	}
	return out
}

func toApiWorkItemListResponse(list []apiWorkItemResponse, total, page, size int) apiWorkItemListResponse {
	return apiWorkItemListResponse{List: list, Total: total, Page: page, Size: size}
}

func toApiWorkItemResponse(model business.RequirementModel, lang string) apiWorkItemResponse {
	return apiWorkItemResponse{
		Id:                 model.Id,
		WorkspaceId:        model.WorkspaceId,
		ParentId:           model.ParentId,
		Title:              model.Title,
		Type:               model.Type,
		StatusId:           model.StatusId,
		StatusName:         lib_i18n.ShowStored(lang, model.StatusName),
		PriorityId:         model.Priority,
		PriorityName:       lib_i18n.ShowStored(lang, model.PriorityName),
		VersionId:          model.VersionId,
		VersionName:        model.VersionName,
		AssigneeIds:        model.AssigneeIds,
		AssigneeNames:      model.AssigneeNames,
		PlannedStartDate:   model.PlannedStartDate,
		PlannedEndDate:     model.PlannedEndDate,
		EstimatedWorkHours: model.EstimatedWorkHours,
		CompletedWorkHours: model.CompletedWorkHours,
		WorkspaceName:      model.WorkspaceName,
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
	}
}

func toApiWorkItemDetailResponse(model business.ApiWorkItemModel, lang string) apiWorkItemDetailResponse {
	// 自定义字段值由 field_id → 文本 的开放契约暴露，键就是字段 ID 的十进制字符串（JSON 对象的键只能是字符串）。
	customFields := make(map[string]string, len(model.CustomFields))
	for fieldId, value := range model.CustomFields {
		customFields[fieldId] = cast.ToString(value)
	}
	detail := apiWorkItemDetailResponse{
		apiWorkItemResponse: apiWorkItemResponse{
			Id:                 model.Id,
			WorkspaceId:        model.WorkspaceId,
			ParentId:           model.ParentId,
			Title:              model.Title,
			Type:               model.Type,
			StatusId:           model.StatusId,
			StatusName:         lib_i18n.ShowStored(lang, model.StatusName),
			PriorityId:         model.Priority,
			PriorityName:       lib_i18n.ShowStored(lang, model.PriorityName),
			VersionId:          model.VersionId,
			VersionName:        model.VersionName,
			AssigneeIds:        model.AssigneeIds,
			AssigneeNames:      model.AssigneeNames,
			PlannedStartDate:   model.PlannedStartDate,
			PlannedEndDate:     model.PlannedEndDate,
			EstimatedWorkHours: model.EstimatedWorkHours,
			CompletedWorkHours: model.CompletedWorkHours,
			CreatedAt:          model.CreatedAt,
			UpdatedAt:          model.UpdatedAt,
		},
		Description:     model.Description,
		DescriptionMode: model.DescriptionMode,
		CommentCount:    model.CommentCount,
		CreatorName:     model.CreatorName,
		CustomFields:    customFields,
	}
	return detail
}

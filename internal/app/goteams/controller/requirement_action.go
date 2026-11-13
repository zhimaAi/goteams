// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"net/http"
	"strconv"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
)

type listWorkItemActionTargetsQuery struct {
	SourceWorkspaceId int    `form:"sourceWsId" binding:"required,gt=0"`
	Type              string `form:"type" binding:"required,oneof=requirement defect"`
	Action            string `form:"action" binding:"required,oneof=move convert"`
}

type workItemActionTargetResponse struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type workItemMergeTargetResponse struct {
	Id            int    `json:"id"`
	WorkspaceId   int    `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	Title         string `json:"title"`
}

type workItemActionResponse struct {
	Id          int    `json:"id"`
	WorkspaceId int    `json:"workspace_id"`
	Type        string `json:"type"`
}

type moveWorkItemRequest struct {
	TargetWorkspaceId int `json:"target_workspace_id" binding:"required,gt=0"`
}

type convertWorkItemRequest struct {
	TargetWorkspaceId int    `json:"target_workspace_id" binding:"required,gt=0"`
	TargetType        string `json:"target_type" binding:"required,oneof=requirement defect"`
}

type mergeWorkItemRequest struct {
	TargetWorkspaceId int `json:"target_workspace_id" binding:"required,gt=0"`
	TargetId          int `json:"target_id" binding:"required,gt=0"`
}

// ListWorkItemActionTargets lists projects available to move or conversion actions.
func ListWorkItemActionTargets(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var query listWorkItemActionTargetsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_action.targets.bind`, err)
		return
	}
	model, err := business.ListWorkItemActionTargets(business.ListWorkItemActionTargetsModel{
		AdminId: c.GetInt(`adminId`), UserId: c.GetInt(`userId`), SourceWorkspaceId: query.SourceWorkspaceId, Type: query.Type, Action: query.Action,
	})
	if err != nil {
		respondWorkItemActionError(c, lang, `work_item_action.targets`, err)
		return
	}
	items := make([]workItemActionTargetResponse, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, workItemActionTargetResponse{Id: item.Id, Name: item.Name})
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: items})
}

// ListWorkItemMergeTargets lists editable defects available as merge targets.
func ListWorkItemMergeTargets(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceId, requirementId, ok := workItemActionParams(c, lang, `work_item_action.merge_targets`)
	if !ok {
		return
	}
	model, err := business.ListWorkItemMergeTargets(business.ListWorkItemMergeTargetsModel{
		AdminId: c.GetInt(`adminId`), UserId: c.GetInt(`userId`), SourceWorkspaceId: workspaceId, SourceId: requirementId,
	})
	if err != nil {
		respondWorkItemActionError(c, lang, `work_item_action.merge_targets`, err)
		return
	}
	items := make([]workItemMergeTargetResponse, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, workItemMergeTargetResponse{Id: item.Id, WorkspaceId: item.WorkspaceId, WorkspaceName: item.WorkspaceName, Title: item.Title})
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: items})
}

// MoveWorkItem moves a work item or requirement tree to another project.
func MoveWorkItem(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceId, requirementId, ok := workItemActionParams(c, lang, `work_item_action.move`)
	if !ok {
		return
	}
	var req moveWorkItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_action.move.bind`, err)
		return
	}
	source, err := business.GetRequirementById(c.GetInt(`adminId`), stringInt(workspaceId), stringInt(requirementId))
	if err != nil {
		respondWorkItemActionError(c, lang, `work_item_action.move.source`, err)
		return
	}
	if len(source) == 0 {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
		return
	}
	result, err := business.TransferWorkItem(business.TransferWorkItemModel{
		AdminId: c.GetInt(`adminId`), UserId: c.GetInt(`userId`), SourceWorkspaceId: workspaceId,
		RequirementId: requirementId, TargetWorkspaceId: req.TargetWorkspaceId, TargetType: source[`type`],
	})
	if err != nil {
		respondWorkItemActionError(c, lang, `work_item_action.move`, err)
		return
	}
	respondWorkItemAction(c, lang, result)
}

// ConvertWorkItem converts a requirement/tree or defect to the opposite type.
func ConvertWorkItem(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceId, requirementId, ok := workItemActionParams(c, lang, `work_item_action.convert`)
	if !ok {
		return
	}
	var req convertWorkItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_action.convert.bind`, err)
		return
	}
	result, err := business.TransferWorkItem(business.TransferWorkItemModel{
		AdminId: c.GetInt(`adminId`), UserId: c.GetInt(`userId`), SourceWorkspaceId: workspaceId,
		RequirementId: requirementId, TargetWorkspaceId: req.TargetWorkspaceId, TargetType: req.TargetType, Convert: true,
	})
	if err != nil {
		respondWorkItemActionError(c, lang, `work_item_action.convert`, err)
		return
	}
	respondWorkItemAction(c, lang, result)
}

// MergeWorkItem merges the current defect into a selected target defect.
func MergeWorkItem(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceId, requirementId, ok := workItemActionParams(c, lang, `work_item_action.merge`)
	if !ok {
		return
	}
	var req mergeWorkItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_action.merge.bind`, err)
		return
	}
	result, err := business.MergeWorkItem(business.MergeWorkItemModel{
		AdminId: c.GetInt(`adminId`), UserId: c.GetInt(`userId`), SourceWorkspaceId: workspaceId,
		SourceId: requirementId, TargetWorkspaceId: req.TargetWorkspaceId, TargetId: req.TargetId,
	})
	if err != nil {
		respondWorkItemActionError(c, lang, `work_item_action.merge`, err)
		return
	}
	respondWorkItemAction(c, lang, result)
}

func workItemActionParams(c *gin.Context, lang, stage string) (int, int, bool) {
	workspaceId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage+`.workspace_id`, err)
		return 0, 0, false
	}
	requirementId, err := positiveParam(c, `reqId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage+`.requirement_id`, err)
		return 0, 0, false
	}
	return workspaceId, requirementId, true
}

func respondWorkItemAction(c *gin.Context, lang string, result business.WorkItemActionResultModel) {
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: workItemActionResponse{
		Id: result.Id, WorkspaceId: result.WorkspaceId, Type: result.Type,
	}})
}

func respondWorkItemActionError(c *gin.Context, lang, stage string, err error) {
	switch {
	case errors.Is(err, business.ErrWorkItemActionForbidden):
		respondLoggedError(c, http.StatusForbidden, http.StatusForbidden, lib_i18n.Show(lang, define.MsgForbidden), stage, err)
	case errors.Is(err, business.ErrRequirementNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
	case errors.Is(err, business.ErrWorkItemActionInvalid), errors.Is(err, business.ErrWorkItemMergeTargetInvalid),
		errors.Is(err, business.ErrStatusNotFound):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage, err)
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), stage, err)
	}
}

func stringInt(value int) string {
	return strconv.Itoa(value)
}

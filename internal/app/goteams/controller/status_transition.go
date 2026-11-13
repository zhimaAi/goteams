// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"net/http"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

type transitionCustomFieldValues map[string]any

type transitionWorkItemStatusRequest struct {
	StatusId         int                         `json:"status_id"`
	Comment          string                      `json:"comment"`
	ImageFileIds     []int                       `json:"image_file_ids"`
	AssigneeIds      *[]int                      `json:"assignee_ids"`
	VersionId        *int                        `json:"version_id"`
	Priority         *int                        `json:"priority"`
	PlannedStartDate *int64                      `json:"planned_start_date"`
	PlannedEndDate   *int64                      `json:"planned_end_date"`
	CustomFields     transitionCustomFieldValues `json:"custom_fields"`
}

func toTransitionWorkItemStatusModel(c *gin.Context, workspaceID, workItemID int, req transitionWorkItemStatusRequest) business.TransitionWorkItemStatusModel {
	customFields := req.CustomFields
	if customFields == nil {
		customFields = transitionCustomFieldValues{}
	}
	return business.TransitionWorkItemStatusModel{
		AdminId:          c.GetInt(`adminId`),
		UserId:           c.GetInt(`userId`),
		WorkspaceId:      cast.ToString(workspaceID),
		WorkItemId:       workItemID,
		StatusId:         req.StatusId,
		Comment:          req.Comment,
		ImageFileIds:     req.ImageFileIds,
		AssigneeIds:      req.AssigneeIds,
		VersionId:        req.VersionId,
		Priority:         req.Priority,
		PlannedStartDate: req.PlannedStartDate,
		PlannedEndDate:   req.PlannedEndDate,
		CustomFields:     business.CustomFieldValues(customFields),
	}
}

// TransitionWorkItemStatus applies a status change with the advanced transition form fields.
func TransitionWorkItemStatus(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `status_transition.workspace_id`, err)
		return
	}
	workItemID, err := positiveParam(c, `reqId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `status_transition.work_item_id`, err)
		return
	}
	var req transitionWorkItemStatusRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `status_transition.bind`, err)
		return
	}
	if req.StatusId <= 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `status_transition.status_id`, errors.New(`status_id required`))
		return
	}

	if err = business.TransitionWorkItemStatus(toTransitionWorkItemStatusModel(c, workspaceID, workItemID, req)); err != nil {
		abortStatusTransitionError(c, lang, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

func abortStatusTransitionError(c *gin.Context, lang string, err error) {
	switch {
	case errors.Is(err, business.ErrRequirementNotFound), errors.Is(err, business.ErrPlanningNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
	case errors.Is(err, business.ErrStatusTransitionForbidden):
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
	case errors.Is(err, business.ErrStatusTransitionFieldRequired):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgStatusTransitionFieldRequired), `status_transition.required`, err)
	case errors.Is(err, business.ErrStatusTransitionFormInvalid), errors.Is(err, business.ErrCustomFieldValueInvalid),
		errors.Is(err, business.ErrRequirementAssigneesInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `status_transition.invalid`, err)
	case errors.Is(err, business.ErrCommentAttachmentsInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `status_transition.comment_attachments`, err)
	case errors.Is(err, business.ErrStatusNotFound):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgStatusNotFound), `status_transition.status`, err)
	case errors.Is(err, business.ErrPriorityNotFound):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgPriorityNotFound), `status_transition.priority`, err)
	case errors.Is(err, business.ErrVersionNotFound):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgVersionNotFound), `status_transition.version`, err)
	case errors.Is(err, business.ErrVersionEnded):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgVersionEnded), `status_transition.version_ended`, err)
	case errors.Is(err, business.ErrInvalidStatusTransition):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgStatusTransitionInvalid), `status_transition.rule`, err)
	case errors.Is(err, validate.ErrPlannedDateRangeInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgPlannedDateRangeInvalid), `status_transition.planned_dates`, err)
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `status_transition.apply`, err)
	}
}

// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"net/http"
	"strconv"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
)

type requirementWorkLogRequest struct {
	WorkDate    *int64   `json:"work_date"`
	Hours       *float64 `json:"hours"`
	Description string   `json:"description"`
}

type requirementWorkHoursSummaryResponse struct {
	EstimatedWorkHours string `json:"estimated_work_hours"`
	CompletedWorkHours string `json:"completed_work_hours"`
	RemainingWorkHours string `json:"remaining_work_hours"`
	OvertimeWorkHours  string `json:"overtime_work_hours"`
	WorkProgress       int    `json:"work_progress"`
}

type requirementWorkLogResponseItem struct {
	Id            int    `json:"id"`
	RequirementId int    `json:"requirement_id"`
	WorkDate      int64  `json:"work_date"`
	Hours         string `json:"hours"`
	Description   string `json:"description"`
	CreatedBy     int    `json:"created_by"`
	CreatorName   string `json:"creator_name"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

type listRequirementWorkLogsResponse struct {
	List    []requirementWorkLogResponseItem    `json:"list"`
	Summary requirementWorkHoursSummaryResponse `json:"summary"`
}

type saveRequirementWorkLogResponse struct {
	Item    requirementWorkLogResponseItem      `json:"item"`
	Summary requirementWorkHoursSummaryResponse `json:"summary"`
}

func formatRequirementWorkHours(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}

func toRequirementWorkHoursSummaryResponse(summary business.RequirementWorkHoursSummary) requirementWorkHoursSummaryResponse {
	return requirementWorkHoursSummaryResponse{
		EstimatedWorkHours: formatRequirementWorkHours(summary.Estimated),
		CompletedWorkHours: formatRequirementWorkHours(summary.Completed),
		RemainingWorkHours: formatRequirementWorkHours(summary.Remaining),
		OvertimeWorkHours:  formatRequirementWorkHours(summary.Overtime),
		WorkProgress:       summary.Progress,
	}
}

func toRequirementWorkLogResponse(item business.RequirementWorkLogModel) requirementWorkLogResponseItem {
	return requirementWorkLogResponseItem{
		Id:            item.Id,
		RequirementId: item.RequirementId,
		WorkDate:      item.WorkDate,
		Hours:         formatRequirementWorkHours(item.Hours),
		Description:   item.Description,
		CreatedBy:     item.CreatedBy,
		CreatorName:   item.CreatorName,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

func bindRequirementWorkLogRequest(c *gin.Context, lang string) (requirementWorkLogRequest, bool) {
	var req requirementWorkLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_work_log.bind`, err)
		return requirementWorkLogRequest{}, false
	}
	if req.WorkDate == nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_work_log.work_date_required`, validate.ErrRequirementWorkDateInvalid)
		return requirementWorkLogRequest{}, false
	}
	if err := validate.ValidateRequirementWorkDate(*req.WorkDate); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_work_log.work_date`, err)
		return requirementWorkLogRequest{}, false
	}
	if req.Hours == nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgRequirementWorkHoursInvalid), `requirement_work_log.hours_required`, validate.ErrRequirementWorkHoursInvalid)
		return requirementWorkLogRequest{}, false
	}
	if err := validate.ValidateRequirementWorkHours(*req.Hours); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgRequirementWorkHoursInvalid), `requirement_work_log.hours`, err)
		return requirementWorkLogRequest{}, false
	}
	return req, true
}

func handleRequirementWorkLogError(c *gin.Context, lang, stage string, err error) {
	switch {
	case errors.Is(err, business.ErrRequirementNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
	case errors.Is(err, business.ErrRequirementWorkLogNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementWorkLogNotFound))
	case errors.Is(err, business.ErrRequirementWorkLogDateExists):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgRequirementWorkLogDateExists), stage, err)
	case errors.Is(err, business.ErrRequirementWorkHoursTotalInvalid), errors.Is(err, validate.ErrRequirementWorkHoursInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgRequirementWorkHoursInvalid), stage, err)
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), stage, err)
	}
}

// ListRequirementWorkLogs returns visible work logs and the current work-hour summary.
func ListRequirementWorkLogs(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_work_log.list.workspace_id`, err)
		return
	}
	requirementId, err := positiveParam(c, `reqId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_work_log.list.requirement_id`, err)
		return
	}
	model, err := business.ListRequirementWorkLogs(business.ListRequirementWorkLogsModel{
		AdminId: c.GetInt(`adminId`), UserId: c.GetInt(`userId`), WorkspaceId: workspaceId, RequirementId: requirementId,
	})
	if err != nil {
		handleRequirementWorkLogError(c, lang, `requirement_work_log.list`, err)
		return
	}
	items := make([]requirementWorkLogResponseItem, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, toRequirementWorkLogResponse(item))
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: listRequirementWorkLogsResponse{
		List: items, Summary: toRequirementWorkHoursSummaryResponse(model.Summary),
	}})
}

// CreateRequirementWorkLog creates a requirement work log.
func CreateRequirementWorkLog(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_work_log.create.workspace_id`, err)
		return
	}
	requirementId, err := positiveParam(c, `reqId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_work_log.create.requirement_id`, err)
		return
	}
	req, ok := bindRequirementWorkLogRequest(c, lang)
	if !ok {
		return
	}
	model, err := business.CreateRequirementWorkLog(business.SaveRequirementWorkLogModel{
		AdminId: c.GetInt(`adminId`), UserId: c.GetInt(`userId`), WorkspaceId: workspaceId, RequirementId: requirementId,
		WorkDate: *req.WorkDate, Hours: *req.Hours, Description: req.Description,
	})
	if err != nil {
		handleRequirementWorkLogError(c, lang, `requirement_work_log.create`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: saveRequirementWorkLogResponse{
		Item: toRequirementWorkLogResponse(model.Item), Summary: toRequirementWorkHoursSummaryResponse(model.Summary),
	}})
}

// UpdateRequirementWorkLog updates a scoped requirement work log.
func UpdateRequirementWorkLog(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_work_log.update.workspace_id`, err)
		return
	}
	requirementId, err := positiveParam(c, `reqId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_work_log.update.requirement_id`, err)
		return
	}
	workLogId, err := positiveParam(c, `logId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_work_log.update.work_log_id`, err)
		return
	}
	req, ok := bindRequirementWorkLogRequest(c, lang)
	if !ok {
		return
	}
	model, err := business.UpdateRequirementWorkLog(business.SaveRequirementWorkLogModel{
		AdminId: c.GetInt(`adminId`), UserId: c.GetInt(`userId`), WorkspaceId: workspaceId, RequirementId: requirementId,
		WorkLogId: workLogId, WorkDate: *req.WorkDate, Hours: *req.Hours, Description: req.Description,
	})
	if err != nil {
		handleRequirementWorkLogError(c, lang, `requirement_work_log.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: saveRequirementWorkLogResponse{
		Item: toRequirementWorkLogResponse(model.Item), Summary: toRequirementWorkHoursSummaryResponse(model.Summary),
	}})
}

// DeleteRequirementWorkLog deletes a scoped requirement work log.
func DeleteRequirementWorkLog(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_work_log.delete.workspace_id`, err)
		return
	}
	requirementId, err := positiveParam(c, `reqId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_work_log.delete.requirement_id`, err)
		return
	}
	workLogId, err := positiveParam(c, `logId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_work_log.delete.work_log_id`, err)
		return
	}
	model, err := business.DeleteRequirementWorkLog(business.DeleteRequirementWorkLogModel{
		AdminId: c.GetInt(`adminId`), WorkspaceId: workspaceId, RequirementId: requirementId, WorkLogId: workLogId,
	})
	if err != nil {
		handleRequirementWorkLogError(c, lang, `requirement_work_log.delete`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: toRequirementWorkHoursSummaryResponse(model.Summary)})
}

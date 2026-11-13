// Copyright © 2026 Wuhan Sesame Small Customer Service Network Co., Ltd.

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
	"github.com/spf13/cast"
)

// ======================== Pipeline CRUD ========================

// ListPipelines returns the pipeline list.
func ListPipelines(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))

	keyword := c.Query(`keyword`)
	statusStr := c.Query(`status`)
	page := cast.ToInt(c.Query(`page`))
	size := cast.ToInt(c.Query(`size`))

	list, total, err := business.ListPipelines(adminId, userId, keyword, statusStr, page, size)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `pipeline.list`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: define.PipelineListResp{
			List:  list,
			Total: total,
			Page:  page,
			Size:  size,
		},
	})
}

// GetPipeline returns the pipeline details.
func GetPipeline(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	pipelineId, err := positiveParam(c, `pipelineId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.get.param`, err)
		return
	}

	detail, err := business.GetPipeline(adminId, pipelineId)
	if err != nil {
		if errors.Is(err, business.ErrPipelineNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgPipelineNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `pipeline.get`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    detail,
	})
}

// CreatePipeline creates a pipeline.
func CreatePipeline(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))

	var req define.CreatePipelineReq
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.create.bind`, err)
		return
	}

	if err := validate.ValidatePipelineCreate(req.Name, len(req.Steps)); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, err.Error()), `pipeline.create.validate`, err)
		return
	}
	if req.Scope != `` {
		if err := validate.ValidatePipelineScope(req.Scope, len(req.MemberIds), len(req.DepartmentIds)); err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, err.Error()), `pipeline.create.scope`, err)
			return
		}
	}

	pipelineId, err := business.CreatePipeline(adminId, userId, req)
	if err != nil {
		if errors.Is(err, business.ErrPipelineManageForbidden) {
			abortForbidden(c, lib_i18n.Show(lang, define.MsgPipelineManageForbidden))
			return
		}
		if errors.Is(err, business.ErrPipelineMemberInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgPipelineMemberInvalid), `pipeline.create.members`, err)
			return
		}
		if errors.Is(err, business.ErrPipelineStepPromptRequired) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgPipelineStepPromptRequired), `pipeline.create.prompt`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `pipeline.create`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    map[string]int{`id`: pipelineId},
	})
}

// UpdatePipeline updates basic pipeline info and members.
func UpdatePipeline(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))
	pipelineId, err := positiveParam(c, `pipelineId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.update.param`, err)
		return
	}

	var req define.UpdatePipelineReq
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.update.bind`, err)
		return
	}

	if err := validate.ValidatePipelineUpdate(req.Name); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgPipelineNameRequired), `pipeline.update.validate`, err)
		return
	}
	if req.Scope != `` {
		if err := validate.ValidatePipelineScope(req.Scope, len(req.MemberIds), len(req.DepartmentIds)); err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, err.Error()), `pipeline.update.scope`, err)
			return
		}
	}

	if err := business.UpdatePipeline(adminId, userId, pipelineId, req); err != nil {
		if errors.Is(err, business.ErrPipelineManageForbidden) {
			abortForbidden(c, lib_i18n.Show(lang, define.MsgPipelineManageForbidden))
			return
		}
		if errors.Is(err, business.ErrPipelineNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgPipelineNotFound))
			return
		}
		if errors.Is(err, business.ErrPipelineMemberInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgPipelineMemberInvalid), `pipeline.update.members`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `pipeline.update`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// UpdatePipelineWorkflow updates the pipeline workflow.
func UpdatePipelineWorkflow(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))
	pipelineId, err := positiveParam(c, `pipelineId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.workflow.param`, err)
		return
	}

	var req define.UpdatePipelineWorkflowReq
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.workflow.bind`, err)
		return
	}

	if err := validate.ValidatePipelineWorkflow(len(req.Steps)); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, err.Error()), `pipeline.workflow.validate`, err)
		return
	}

	if err := business.UpdatePipelineWorkflow(adminId, userId, pipelineId, req); err != nil {
		if errors.Is(err, business.ErrPipelineManageForbidden) {
			abortForbidden(c, lib_i18n.Show(lang, define.MsgPipelineManageForbidden))
			return
		}
		if errors.Is(err, business.ErrPipelineNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgPipelineNotFound))
			return
		}
		if errors.Is(err, business.ErrPipelineVersionConflict) {
			abortConflict(c, lib_i18n.Show(lang, define.MsgPipelineWorkflowVersionConflict))
			return
		}
		if errors.Is(err, business.ErrPipelineStepPromptRequired) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgPipelineStepPromptRequired), `pipeline.workflow.prompt`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `pipeline.workflow`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// DeletePipeline soft-deletes a pipeline.
func DeletePipeline(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))
	pipelineId, err := positiveParam(c, `pipelineId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.delete.param`, err)
		return
	}

	if err := business.DeletePipeline(adminId, userId, pipelineId); err != nil {
		if errors.Is(err, business.ErrPipelineManageForbidden) {
			abortForbidden(c, lib_i18n.Show(lang, define.MsgPipelineManageForbidden))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `pipeline.delete`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// UploadPipelineIcon uploads the pipeline icon.
func UploadPipelineIcon(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	userId := cast.ToInt(c.GetInt(`userId`))
	pipelineId, err := positiveParam(c, `pipelineId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.icon.param`, err)
		return
	}
	file, err := c.FormFile(`file`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgPipelineIconInvalid), `pipeline.icon.form_file`, err)
		return
	}
	icon, err := business.SavePipelineIcon(adminId, userId, pipelineId, file)
	if err != nil {
		switch {
		case errors.Is(err, business.ErrPipelineManageForbidden):
			abortForbidden(c, lib_i18n.Show(lang, define.MsgPipelineManageForbidden))
		case errors.Is(err, business.ErrPipelineNotFound), errors.Is(err, business.ErrPipelineIconNotFound):
			abortNotFound(c, lib_i18n.Show(lang, define.MsgPipelineNotFound))
		case errors.Is(err, business.ErrPipelineIconInvalid):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgPipelineIconInvalid), `pipeline.icon.invalid`, err)
		case errors.Is(err, business.ErrPipelineIconTooLarge):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgPipelineIconTooLarge), `pipeline.icon.too_large`, err)
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `pipeline.icon.save`, err)
		}
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    map[string]string{`icon`: icon},
	})
}

// ======================== Pipeline task API ========================

// ListPipelineTasks returns pipeline task statistics and the list.
func ListPipelineTasks(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	userId := cast.ToInt(c.GetInt(`userId`))

	keyword := c.Query(`keyword`)
	period := c.Query(`period`)
	if period == `` {
		period = define.PipelineTaskPeriodAll
	}
	if err := validate.ValidatePipelineTaskPeriod(period); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.tasks.list.period`, err)
		return
	}
	userIDs, startAt, endAt, err := validate.ParsePipelineTaskListFilters(
		c.Query(`user_ids`), c.Query(`start_at`), c.Query(`end_at`),
	)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.tasks.list.filters`, err)
		return
	}
	page := cast.ToInt(c.Query(`page`))
	size := cast.ToInt(c.Query(`size`))

	list, total, stats, members, err := business.ListPipelineTasks(
		adminId, userId, keyword, period, userIDs, startAt, endAt, page, size,
	)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `pipeline.tasks.list`, err)
		return
	}
	for _, task := range list {
		task[`status_name`] = lib_i18n.ShowStored(lang, cast.ToString(task[`status_name`]))
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: define.PipelineTaskListResp{
			List:    list,
			Total:   total,
			Stats:   stats,
			Members: members,
			Page:    page,
			Size:    size,
		},
	})
}

// GetPipelineTaskDetail returns the pipeline task details.
func GetPipelineTaskDetail(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	taskId, err := strconv.ParseInt(c.Param(`taskId`), 10, 64)
	if err != nil || taskId <= 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.task.detail.param`, errInvalidID)
		return
	}

	detail, err := business.GetPipelineTaskDetail(adminId, cast.ToInt(c.GetInt(`userId`)), taskId)
	if err != nil {
		if errors.Is(err, business.ErrPipelineTaskNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgPipelineTaskNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `pipeline.task.detail`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    detail,
	})
}

// ======================== Work-item agent tab ========================

// agentTabWorkItemType resolves the work item type of an agent-tab request. Both
// requirement and defect details share the /requirements/:reqId route, so the
// type travels as the `type` query parameter (default requirement).
func agentTabWorkItemType(c *gin.Context) string {
	if c.Query(`type`) == define.TypeKeyDefect {
		return define.TypeKeyDefect
	}
	return define.TypeKeyRequirement
}

// ListRequirementAgentTasks returns the execution records of a work item grouped
// by executor, for the Agent tab of the requirement/defect detail drawer.
func ListRequirementAgentTasks(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.agent_tasks.ws`, err)
		return
	}
	adminId := cast.ToInt(c.GetInt(`adminId`))
	reqId := cast.ToInt(c.Param(`reqId`))
	if reqId <= 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.agent_tasks.id`, nil)
		return
	}

	executors, err := business.ListWorkItemAgentTasks(adminId, cast.ToInt(c.GetInt(`userId`)), wsId, agentTabWorkItemType(c), reqId)
	if err != nil {
		if errors.Is(err, business.ErrPipelineTaskNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgPipelineTaskNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `pipeline.agent_tasks.list`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    executors,
	})
}

// GetRequirementAgentTaskDetail returns one execution record of a work item for
// the Agent tab detail view (steps, conversations and run statistics).
func GetRequirementAgentTaskDetail(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.agent_task_detail.ws`, err)
		return
	}
	adminId := cast.ToInt(c.GetInt(`adminId`))
	reqId := cast.ToInt(c.Param(`reqId`))
	taskId, err := strconv.ParseInt(c.Param(`taskId`), 10, 64)
	if reqId <= 0 || err != nil || taskId <= 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.agent_task_detail.param`, errInvalidID)
		return
	}

	detail, err := business.GetWorkItemAgentTaskDetail(adminId, cast.ToInt(c.GetInt(`userId`)), wsId, agentTabWorkItemType(c), reqId, taskId)
	if err != nil {
		if errors.Is(err, business.ErrPipelineTaskNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgPipelineTaskNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `pipeline.agent_task_detail`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    detail,
	})
}

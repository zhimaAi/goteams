// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"net/http"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

// ======================== Workflow DTO ========================

// listStatusesResponseItem is an item in the status list.
type listStatusesResponseItem struct {
	Id        int    `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
	IsDefault int    `json:"is_default"`
}

// createStatusRequest is the request to create a status.
type createStatusRequest struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
}

// createStatusResponse is the response after creating a status.
type createStatusResponse struct {
	Id int `json:"id"`
}

// updateStatusRequest is the request to update a status.
type updateStatusRequest struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
}

// listPrioritiesResponseItem is an item in the priority list.
type listPrioritiesResponseItem struct {
	Id        int    `json:"id"`
	Label     string `json:"label"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
	IsDefault int    `json:"is_default"`
}

// createPriorityRequest is the request to create a priority.
type createPriorityRequest struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
}

// createPriorityResponse is the response after creating a priority.
type createPriorityResponse struct {
	Id int `json:"id"`
}

// updatePriorityRequest is the request to update a priority.
type updatePriorityRequest struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
}

// setDefaultPriorityRequest is the request to set the default priority; 0 clears the default.
type setDefaultPriorityRequest struct {
	PriorityId *int `json:"priority_id"`
}

type statusTransitionFieldResponse struct {
	Key      string `json:"key"`
	Required int    `json:"required"`
}

type statusTransitionFormResponse struct {
	Enabled int                             `json:"enabled"`
	Fields  []statusTransitionFieldResponse `json:"fields"`
}

type statusTransitionFieldRequest struct {
	Key      string `json:"key"`
	Required int    `json:"required"`
}

type statusTransitionFormRequest struct {
	Enabled int                            `json:"enabled"`
	Fields  []statusTransitionFieldRequest `json:"fields"`
}

type agingRuleResponse struct {
	Enabled        int  `json:"enabled"`
	TimeoutMinutes int  `json:"timeout_minutes"`
	NotifyEnabled  int  `json:"notify_enabled"`
	WarnMinutes    *int `json:"warn_minutes"`
}

type agingStatusRuleResponse struct {
	StatusID    int    `json:"status_id"`
	StatusName  string `json:"status_name"`
	StatusColor string `json:"status_color"`
	agingRuleResponse
}

type agingControlFormResponse struct {
	MasterEnabled bool                      `json:"master_enabled"`
	TotalDuration agingRuleResponse         `json:"total_duration"`
	StatusRules   []agingStatusRuleResponse `json:"status_rules"`
}

type agingRuleRequest struct {
	Enabled        int  `json:"enabled"`
	TimeoutMinutes int  `json:"timeout_minutes"`
	NotifyEnabled  int  `json:"notify_enabled"`
	WarnMinutes    *int `json:"warn_minutes"`
}

type agingStatusRuleRequest struct {
	StatusID    int    `json:"status_id"`
	StatusName  string `json:"status_name"`
	StatusColor string `json:"status_color"`
	agingRuleRequest
}

type agingControlFormRequest struct {
	MasterEnabled bool                     `json:"master_enabled"`
	TotalDuration agingRuleRequest         `json:"total_duration"`
	StatusRules   []agingStatusRuleRequest `json:"status_rules"`
}

// workspaceSettingsResponse is the response for workspace initial/final state config.
type workspaceSettingsResponse struct {
	SLAEnabled                    bool                         `json:"sla_enabled"`
	SLAActivatedAt                int64                        `json:"sla_activated_at"`
	RequirementInitialStatusId    int                          `json:"requirement_initial_status_id"`
	DefectInitialStatusId         int                          `json:"defect_initial_status_id"`
	PlanningInitialStatusId       int                          `json:"planning_initial_status_id"`
	RequirementFinalStatusIds     []int                        `json:"requirement_final_status_ids"`
	RequirementCompletedStatusIds []int                        `json:"requirement_completed_status_ids"`
	RequirementClosedStatusIds    []int                        `json:"requirement_closed_status_ids"`
	DefectFinalStatusIds          []int                        `json:"defect_final_status_ids"`
	DefectCompletedStatusIds      []int                        `json:"defect_completed_status_ids"`
	DefectClosedStatusIds         []int                        `json:"defect_closed_status_ids"`
	PlanningFinalStatusIds        []int                        `json:"planning_final_status_ids"`
	RequirementStatusTransition   statusTransitionFormResponse `json:"requirement_status_transition"`
	DefectStatusTransition        statusTransitionFormResponse `json:"defect_status_transition"`
	RequirementAgingControl       agingControlFormResponse     `json:"requirement_aging_control"`
}

// updateWorkspaceSettingsRequest is the partial-update request for workspace initial/final state config.
type updateWorkspaceSettingsRequest struct {
	RequirementInitialStatusId    *int                         `json:"requirement_initial_status_id"`
	DefectInitialStatusId         *int                         `json:"defect_initial_status_id"`
	PlanningInitialStatusId       *int                         `json:"planning_initial_status_id"`
	RequirementFinalStatusIds     *[]int                       `json:"requirement_final_status_ids"`
	RequirementCompletedStatusIds *[]int                       `json:"requirement_completed_status_ids"`
	RequirementClosedStatusIds    *[]int                       `json:"requirement_closed_status_ids"`
	DefectFinalStatusIds          *[]int                       `json:"defect_final_status_ids"`
	DefectCompletedStatusIds      *[]int                       `json:"defect_completed_status_ids"`
	DefectClosedStatusIds         *[]int                       `json:"defect_closed_status_ids"`
	PlanningFinalStatusIds        *[]int                       `json:"planning_final_status_ids"`
	RequirementStatusTransition   *statusTransitionFormRequest `json:"requirement_status_transition"`
	DefectStatusTransition        *statusTransitionFormRequest `json:"defect_status_transition"`
	RequirementAgingControl       *agingControlFormRequest     `json:"requirement_aging_control"`
}

// toListStatusesResponse converts the status model to the response.
func toListStatusesResponse(model business.ListStatusesModel) []listStatusesResponseItem {
	items := make([]listStatusesResponseItem, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, listStatusesResponseItem{
			Id: item.Id, Name: item.Name, Color: item.Color, SortOrder: item.SortOrder, IsDefault: item.IsDefault,
		})
	}
	return items
}

// toListPrioritiesResponse converts the priority model to the response.
func toListPrioritiesResponse(model business.ListPrioritiesModel) []listPrioritiesResponseItem {
	items := make([]listPrioritiesResponseItem, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, listPrioritiesResponseItem{
			Id: item.Id, Label: item.Label, Color: item.Color, SortOrder: item.SortOrder, IsDefault: item.IsDefault,
		})
	}
	return items
}

// toWorkspaceSettingsResponse converts the workspace config model to the response.
func toWorkspaceSettingsResponse(model business.WorkspaceSettingsModel) workspaceSettingsResponse {
	return workspaceSettingsResponse{SLAEnabled: model.SLAEnabled, SLAActivatedAt: model.SLAActivatedAt,
		RequirementInitialStatusId:    model.RequirementInitialStatusId,
		DefectInitialStatusId:         model.DefectInitialStatusId,
		PlanningInitialStatusId:       model.PlanningInitialStatusId,
		RequirementFinalStatusIds:     model.RequirementFinalStatusIds,
		RequirementCompletedStatusIds: model.RequirementCompletedStatusIds,
		RequirementClosedStatusIds:    model.RequirementClosedStatusIds,
		DefectFinalStatusIds:          model.DefectFinalStatusIds,
		DefectCompletedStatusIds:      model.DefectCompletedStatusIds,
		DefectClosedStatusIds:         model.DefectClosedStatusIds,
		PlanningFinalStatusIds:        model.PlanningFinalStatusIds,
		RequirementStatusTransition:   toStatusTransitionFormResponse(model.RequirementStatusTransition),
		DefectStatusTransition:        toStatusTransitionFormResponse(model.DefectStatusTransition),
		RequirementAgingControl:       toAgingControlFormResponse(model.RequirementAgingControl),
	}
}

func toStatusTransitionFormResponse(model business.StatusTransitionFormModel) statusTransitionFormResponse {
	fields := make([]statusTransitionFieldResponse, 0, len(model.Fields))
	for _, item := range model.Fields {
		fields = append(fields, statusTransitionFieldResponse{Key: item.Key, Required: item.Required})
	}
	return statusTransitionFormResponse{Enabled: model.Enabled, Fields: fields}
}

func toStatusTransitionFormModel(req *statusTransitionFormRequest) *business.StatusTransitionFormModel {
	if req == nil {
		return nil
	}
	fields := make([]business.StatusTransitionFieldModel, 0, len(req.Fields))
	for _, item := range req.Fields {
		fields = append(fields, business.StatusTransitionFieldModel{Key: item.Key, Required: item.Required})
	}
	return &business.StatusTransitionFormModel{Enabled: req.Enabled, Fields: fields}
}

func toAgingControlFormResponse(model business.AgingControlForm) agingControlFormResponse {
	rules := make([]agingStatusRuleResponse, 0, len(model.StatusRules))
	for _, item := range model.StatusRules {
		rules = append(rules, agingStatusRuleResponse{
			StatusID:    item.StatusID,
			StatusName:  item.StatusName,
			StatusColor: item.StatusColor,
			agingRuleResponse: agingRuleResponse{
				Enabled:        item.Enabled,
				TimeoutMinutes: item.TimeoutMinutes,
				NotifyEnabled:  item.NotifyEnabled,
				WarnMinutes:    item.WarnMinutes,
			},
		})
	}
	return agingControlFormResponse{
		MasterEnabled: model.MasterEnabled,
		TotalDuration: agingRuleResponse{
			Enabled:        model.TotalDuration.Enabled,
			TimeoutMinutes: model.TotalDuration.TimeoutMinutes,
			NotifyEnabled:  model.TotalDuration.NotifyEnabled,
			WarnMinutes:    model.TotalDuration.WarnMinutes,
		},
		StatusRules: rules,
	}
}

func toAgingControlFormModel(req *agingControlFormRequest) *business.AgingControlForm {
	if req == nil {
		return nil
	}
	rules := make([]business.AgingStatusRule, 0, len(req.StatusRules))
	for _, item := range req.StatusRules {
		rules = append(rules, business.AgingStatusRule{
			StatusID:    item.StatusID,
			StatusName:  item.StatusName,
			StatusColor: item.StatusColor,
			AgingRule: business.AgingRule{
				Enabled:        item.Enabled,
				TimeoutMinutes: item.TimeoutMinutes,
				NotifyEnabled:  item.NotifyEnabled,
				WarnMinutes:    item.WarnMinutes,
			},
		})
	}
	return &business.AgingControlForm{
		MasterEnabled: req.MasterEnabled,
		TotalDuration: business.AgingRule{
			Enabled:        req.TotalDuration.Enabled,
			TimeoutMinutes: req.TotalDuration.TimeoutMinutes,
			NotifyEnabled:  req.TotalDuration.NotifyEnabled,
			WarnMinutes:    req.TotalDuration.WarnMinutes,
		},
		StatusRules: rules,
	}
}

// ======================== Workflow Status CRUD ========================

// ListStatuses returns the status list of a specified type under the workspace.
func ListStatuses(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	model, err := business.ListStatuses(business.ListStatusesModel{
		AdminId: c.GetInt(`adminId`),
		WsId:    c.Param(`wsId`),
		TypeKey: c.DefaultQuery(`type`, define.TypeKeyRequirement),
		Lang:    lang,
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `status.list`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toListStatusesResponse(model),
	})
}

// CreateStatus creates a work-item status.
func CreateStatus(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req createStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `status.create.bind`, err)
		return
	}
	if len(req.Name) == 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `status.create.value`, nil)
		return
	}
	model, err := business.CreateStatus(business.CreateStatusModel{
		AdminId:   c.GetInt(`adminId`),
		WsId:      c.Param(`wsId`),
		TypeKey:   c.DefaultQuery(`type`, define.TypeKeyRequirement),
		Name:      req.Name,
		Color:     req.Color,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `status.create`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    createStatusResponse{Id: model.Id},
	})
}

// UpdateStatus edits a work-item status.
func UpdateStatus(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `status.update.bind`, err)
		return
	}
	if err := business.UpdateStatus(business.UpdateStatusModel{
		AdminId:   c.GetInt(`adminId`),
		WsId:      c.Param(`wsId`),
		TypeKey:   c.DefaultQuery(`type`, define.TypeKeyRequirement),
		StatusId:  cast.ToInt(c.Param(`statusId`)),
		Name:      req.Name,
		Color:     req.Color,
		SortOrder: req.SortOrder,
	}); err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `status.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// DeleteStatus deletes a work-item status.
func DeleteStatus(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	if err := business.DeleteStatus(business.DeleteStatusModel{
		AdminId:  c.GetInt(`adminId`),
		WsId:     c.Param(`wsId`),
		TypeKey:  c.DefaultQuery(`type`, define.TypeKeyRequirement),
		StatusId: cast.ToInt(c.Param(`statusId`)),
	}); err != nil {
		if business.IsSLAReferenceConflict(err) {
			abortConflict(c, lib_i18n.Show(lang, define.MsgSLAConflict))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `status.delete`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// ResetStatuses restores the statuses of a specified type under the workspace to built-in defaults.
func ResetStatuses(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	if err := business.ResetStatuses(business.ResetStatusesModel{
		AdminId: c.GetInt(`adminId`),
		WsId:    c.Param(`wsId`),
		TypeKey: c.DefaultQuery(`type`, define.TypeKeyRequirement),
	}); err != nil {
		if business.IsSLAReferenceConflict(err) {
			abortConflict(c, lib_i18n.Show(lang, define.MsgSLAConflict))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `status.reset`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// ======================== Workflow Priority CRUD ========================

// ListPriorities returns the priority list of a specified type under the workspace.
func ListPriorities(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	model, err := business.ListPriorities(business.ListPrioritiesModel{
		AdminId: c.GetInt(`adminId`),
		WsId:    c.Param(`wsId`),
		TypeKey: c.DefaultQuery(`type`, define.TypeKeyRequirement),
		Lang:    lang,
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `priority.list`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toListPrioritiesResponse(model),
	})
}

// CreatePriority creates a work-item priority.
func CreatePriority(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req createPriorityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `priority.create.bind`, err)
		return
	}
	if len(req.Name) == 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `priority.create.value`, nil)
		return
	}
	model, err := business.CreatePriority(business.CreatePriorityModel{
		AdminId:   c.GetInt(`adminId`),
		WsId:      c.Param(`wsId`),
		TypeKey:   c.DefaultQuery(`type`, define.TypeKeyRequirement),
		Name:      req.Name,
		Color:     req.Color,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `priority.create`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    createPriorityResponse{Id: model.Id},
	})
}

// UpdatePriority edits a work-item priority.
func UpdatePriority(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req updatePriorityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `priority.update.bind`, err)
		return
	}
	if err := business.UpdatePriority(business.UpdatePriorityModel{
		AdminId:    c.GetInt(`adminId`),
		WsId:       c.Param(`wsId`),
		TypeKey:    c.DefaultQuery(`type`, define.TypeKeyRequirement),
		PriorityId: cast.ToInt(c.Param(`priorityId`)),
		Name:       req.Name,
		Color:      req.Color,
		SortOrder:  req.SortOrder,
	}); err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `priority.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// SetDefaultPriority sets or clears the default priority for a work-item type.
func SetDefaultPriority(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId := c.GetInt(`adminId`)
	userId := c.GetInt(`userId`)
	typeKey := c.DefaultQuery(`type`, define.TypeKeyRequirement)

	// Precisely validate the config permission by work-item type (route middleware uses OR semantics; tightened here).
	permissionKey := `requirement_setting:priority`
	if typeKey == define.TypeKeyDefect {
		permissionKey = `defect_setting:priority`
	} else if typeKey != define.TypeKeyRequirement {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `priority.default.type`, nil)
		return
	}
	if !business.HasPermission(adminId, userId, wsId, permissionKey) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}

	var req setDefaultPriorityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `priority.default.bind`, err)
		return
	}
	if req.PriorityId == nil || *req.PriorityId < 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `priority.default.value`, nil)
		return
	}
	if err := business.SetDefaultPriority(business.SetDefaultPriorityModel{
		AdminId:    adminId,
		WsId:       wsId,
		TypeKey:    typeKey,
		PriorityId: *req.PriorityId,
	}); err != nil {
		if errors.Is(err, business.ErrPriorityNotFound) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgPriorityNotFound), `priority.default.not_found`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `priority.default`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// DeletePriority deletes a work-item priority.
func DeletePriority(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	if err := business.DeletePriority(business.DeletePriorityModel{
		AdminId:    c.GetInt(`adminId`),
		WsId:       c.Param(`wsId`),
		TypeKey:    c.DefaultQuery(`type`, define.TypeKeyRequirement),
		PriorityId: cast.ToInt(c.Param(`priorityId`)),
	}); err != nil {
		if business.IsSLAReferenceConflict(err) {
			abortConflict(c, lib_i18n.Show(lang, define.MsgSLAConflict))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `priority.delete`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// ResetPriorities restores the priorities of a specified type under the workspace to built-in defaults.
func ResetPriorities(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	if err := business.ResetPriorities(business.ResetPrioritiesModel{
		AdminId: c.GetInt(`adminId`),
		WsId:    c.Param(`wsId`),
		TypeKey: c.DefaultQuery(`type`, define.TypeKeyRequirement),
	}); err != nil {
		if business.IsSLAReferenceConflict(err) {
			abortConflict(c, lib_i18n.Show(lang, define.MsgSLAConflict))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `priority.reset`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// ======================== Workspace Start/End Status Configuration ========================

// GetWorkspaceSettings reads the workspace initial/final state config.
func GetWorkspaceSettings(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	model, err := business.GetWorkspaceSettings(business.WorkspaceSettingsModel{
		AdminId: c.GetInt(`adminId`),
		WsId:    c.Param(`wsId`),
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `settings.get`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toWorkspaceSettingsResponse(model),
	})
}

// UpdateWorkspaceSettings updates the workspace initial/final state config.
func UpdateWorkspaceSettings(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req updateWorkspaceSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `settings.update.bind`, err)
		return
	}
	adminId, userId, wsId := c.GetInt(`adminId`), c.GetInt(`userId`), c.Param(`wsId`)
	permissionChecks := []struct {
		touched       bool
		permissionKey string
	}{
		{req.RequirementInitialStatusId != nil || req.RequirementFinalStatusIds != nil || req.RequirementCompletedStatusIds != nil || req.RequirementClosedStatusIds != nil || req.RequirementStatusTransition != nil, `requirement_setting:status`},
		{req.DefectInitialStatusId != nil || req.DefectFinalStatusIds != nil || req.DefectCompletedStatusIds != nil || req.DefectClosedStatusIds != nil || req.DefectStatusTransition != nil, `defect_setting:status`},
		{req.PlanningInitialStatusId != nil || req.PlanningFinalStatusIds != nil, `planning:edit`},
	}
	for _, check := range permissionChecks {
		if check.touched && !business.HasPermission(adminId, userId, wsId, check.permissionKey) {
			abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
			return
		}
	}
	if err := business.UpdateWorkspaceSettings(business.UpdateWorkspaceSettingsModel{
		AdminId:                       adminId,
		WsId:                          wsId,
		RequirementInitialStatusId:    req.RequirementInitialStatusId,
		DefectInitialStatusId:         req.DefectInitialStatusId,
		PlanningInitialStatusId:       req.PlanningInitialStatusId,
		RequirementFinalStatusIds:     req.RequirementFinalStatusIds,
		RequirementCompletedStatusIds: req.RequirementCompletedStatusIds,
		RequirementClosedStatusIds:    req.RequirementClosedStatusIds,
		DefectFinalStatusIds:          req.DefectFinalStatusIds,
		DefectCompletedStatusIds:      req.DefectCompletedStatusIds,
		DefectClosedStatusIds:         req.DefectClosedStatusIds,
		PlanningFinalStatusIds:        req.PlanningFinalStatusIds,
		RequirementStatusTransition:   toStatusTransitionFormModel(req.RequirementStatusTransition),
		DefectStatusTransition:        toStatusTransitionFormModel(req.DefectStatusTransition),
		RequirementAgingControl:       toAgingControlFormModel(req.RequirementAgingControl),
	}); err != nil {
		if business.IsSLAReferenceConflict(err) {
			abortConflict(c, lib_i18n.Show(lang, define.MsgSLAConflict))
			return
		}
		if errors.Is(err, business.ErrStatusGroupsInvalid) || errors.Is(err, business.ErrStatusTransitionFormInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `settings.update.transition`, err)
			return
		}
		if errors.Is(err, business.ErrAgingFormInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `settings.update.aging`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `settings.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

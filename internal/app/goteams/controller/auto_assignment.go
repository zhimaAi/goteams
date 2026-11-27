package controller

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
	"net/http"
)

type autoAssignmentRequest struct {
	Revision int             `json:"revision"`
	Config   json.RawMessage `json:"config"`
}
type autoAssignmentDepartmentResponse struct {
	ID       int    `json:"id"`
	ParentID int    `json:"parent_id"`
	Name     string `json:"name"`
}
type autoAssignmentResponse struct {
	Revision    int                                `json:"revision"`
	Config      json.RawMessage                    `json:"config"`
	Departments []autoAssignmentDepartmentResponse `json:"departments,omitempty"`
}

// GetAutoAssignment 返回当前工作空间配置及可选分组。
func GetAutoAssignment(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	ws, ok := automationWorkspaceParam(c, lang)
	if !ok {
		return
	}
	model, err := business.GetAutoAssignment(business.AutoAssignmentModel{AdminID: c.GetInt("adminId"), WorkspaceID: ws})
	if err != nil {
		automationRuleError(c, lang, "auto_assignment.get", err)
		return
	}
	groups, err := business.ListDepartments(c.GetInt("adminId"))
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), "auto_assignment.groups", err)
		return
	}
	response := autoAssignmentResponse{Revision: model.Revision, Config: model.Config, Departments: []autoAssignmentDepartmentResponse{}}
	for _, g := range groups {
		response.Departments = append(response.Departments, autoAssignmentDepartmentResponse{ID: g.Id, ParentID: g.ParentId, Name: g.Name})
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Data: response})
}

// SaveAutoAssignment 替换有序规则集，修订冲突时保留已有配置。
func SaveAutoAssignment(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	ws, ok := automationWorkspaceParam(c, lang)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAutomationRequestBody)
	var request autoAssignmentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "auto_assignment.bind", err)
		return
	}
	model, err := business.SaveAutoAssignment(business.AutoAssignmentModel{AdminID: c.GetInt("adminId"), WorkspaceID: ws, UserID: c.GetInt("userId"), Revision: request.Revision, Config: request.Config})
	if errors.Is(err, business.ErrAutoAssignmentConflict) {
		abortConflict(c, lib_i18n.Show(lang, define.MsgAutoAssignmentConflict))
		return
	}
	if err != nil {
		automationRuleError(c, lang, "auto_assignment.save", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Data: autoAssignmentResponse{Revision: model.Revision, Config: model.Config}})
}

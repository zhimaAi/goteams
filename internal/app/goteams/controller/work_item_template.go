// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
)

type listWorkItemTemplatesQuery struct {
	Type string `form:"type"`
}

type listWorkItemTemplateLinkageResponse struct {
	Value     json.RawMessage `json:"value"`
	FieldKeys []string        `json:"field_keys"`
}

type listWorkItemTemplateFieldResponse struct {
	FieldKey     string                                `json:"field_key"`
	DefaultValue json.RawMessage                       `json:"default_value"`
	Required     bool                                  `json:"required"`
	Linkages     []listWorkItemTemplateLinkageResponse `json:"linkages,omitempty"`
}

type listWorkItemTemplatesResponseItem struct {
	Id              int                                 `json:"id"`
	Name            string                              `json:"name"`
	FieldCount      int                                 `json:"field_count"`
	Title           string                              `json:"title"`
	Description     string                              `json:"description"`
	DescriptionMode string                              `json:"description_mode"`
	Fields          []listWorkItemTemplateFieldResponse `json:"fields"`
}

type listWorkItemTemplatesResponse struct {
	Enabled bool                                `json:"enabled"`
	Items   []listWorkItemTemplatesResponseItem `json:"items"`
}

type getWorkItemTemplateLinkageResponse struct {
	Value     json.RawMessage `json:"value"`
	FieldKeys []string        `json:"field_keys"`
}

type getWorkItemTemplateFieldResponse struct {
	FieldKey     string                               `json:"field_key"`
	DefaultValue json.RawMessage                      `json:"default_value"`
	Required     bool                                 `json:"required"`
	Linkages     []getWorkItemTemplateLinkageResponse `json:"linkages,omitempty"`
}

type getWorkItemTemplateResponse struct {
	Id              int                                `json:"id"`
	Name            string                             `json:"name"`
	FieldCount      int                                `json:"field_count"`
	Title           string                             `json:"title"`
	Description     string                             `json:"description"`
	DescriptionMode string                             `json:"description_mode"`
	Fields          []getWorkItemTemplateFieldResponse `json:"fields"`
}

type createWorkItemTemplateLinkageRequest struct {
	Value     json.RawMessage `json:"value"`
	FieldKeys []string        `json:"field_keys"`
}

type createWorkItemTemplateFieldRequest struct {
	FieldKey     string                                 `json:"field_key"`
	DefaultValue json.RawMessage                        `json:"default_value"`
	Required     bool                                   `json:"required"`
	Linkages     []createWorkItemTemplateLinkageRequest `json:"linkages"`
}

type createWorkItemTemplateRequest struct {
	Name            string                               `json:"name"`
	Title           string                               `json:"title"`
	Description     string                               `json:"description"`
	DescriptionMode string                               `json:"description_mode"`
	Fields          []createWorkItemTemplateFieldRequest `json:"fields"`
}

type createWorkItemTemplateResponse struct {
	Id int `json:"id"`
}

type updateWorkItemTemplateLinkageRequest struct {
	Value     json.RawMessage `json:"value"`
	FieldKeys []string        `json:"field_keys"`
}

type updateWorkItemTemplateFieldRequest struct {
	FieldKey     string                                 `json:"field_key"`
	DefaultValue json.RawMessage                        `json:"default_value"`
	Required     bool                                   `json:"required"`
	Linkages     []updateWorkItemTemplateLinkageRequest `json:"linkages"`
}

type updateWorkItemTemplateRequest struct {
	Name            string                               `json:"name"`
	Title           string                               `json:"title"`
	Description     string                               `json:"description"`
	DescriptionMode string                               `json:"description_mode"`
	Fields          []updateWorkItemTemplateFieldRequest `json:"fields"`
}

type updateWorkItemTemplateSettingsRequest struct {
	Enabled bool `json:"enabled"`
}

type workItemDisplayStatusFieldsRequest struct {
	StatusId  int      `json:"status_id"`
	FieldKeys []string `json:"field_keys"`
}

type workItemDisplaySettingsRequest struct {
	EditDisplayMode string                               `json:"edit_display_mode"`
	ViewDisplayMode string                               `json:"view_display_mode"`
	StatusFields    []workItemDisplayStatusFieldsRequest `json:"status_fields"`
}

type workItemDisplayStatusFieldsResponse struct {
	StatusId  int      `json:"status_id"`
	FieldKeys []string `json:"field_keys"`
}

type workItemDisplaySettingsResponse struct {
	TemplateEnabled bool                                  `json:"template_enabled"`
	EditDisplayMode string                                `json:"edit_display_mode"`
	ViewDisplayMode string                                `json:"view_display_mode"`
	DefaultFields   []string                              `json:"default_fields"`
	StatusFields    []workItemDisplayStatusFieldsResponse `json:"status_fields"`
}

type linkWorkItemTemplatesRequest struct {
	Ids []int `json:"ids"`
}

type listWorkItemTemplatePoolQuery struct {
	Type string `form:"type"`
}

type listWorkItemTemplatePoolResponseItem struct {
	Id        int    `json:"id"`
	Name      string `json:"name"`
	BindCount int    `json:"bind_count"`
	CanDelete bool   `json:"can_delete"`
}

func toListWorkItemTemplatesResponse(model business.ListWorkItemTemplatesModel) listWorkItemTemplatesResponse {
	items := make([]listWorkItemTemplatesResponseItem, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, listWorkItemTemplatesResponseItem{
			Id: item.Id, Name: item.Name, FieldCount: item.FieldCount,
			Title: item.Title, Description: item.Description, DescriptionMode: item.DescriptionMode,
			Fields: toListTemplateFields(item.Fields),
		})
	}
	return listWorkItemTemplatesResponse{Enabled: model.Enabled, Items: items}
}

func toWorkItemDisplaySettingsResponse(model business.WorkItemDisplaySettingModel) workItemDisplaySettingsResponse {
	items := make([]workItemDisplayStatusFieldsResponse, 0, len(model.StatusFields))
	for _, item := range model.StatusFields {
		items = append(items, workItemDisplayStatusFieldsResponse{StatusId: item.StatusId, FieldKeys: item.FieldKeys})
	}
	return workItemDisplaySettingsResponse{TemplateEnabled: model.TemplateEnabled, EditDisplayMode: model.EditDisplayMode, ViewDisplayMode: model.ViewDisplayMode, DefaultFields: model.DefaultFields, StatusFields: items}
}

func toListTemplateFields(fields []business.TemplateFieldModel) []listWorkItemTemplateFieldResponse {
	result := make([]listWorkItemTemplateFieldResponse, 0, len(fields))
	for _, field := range fields {
		linkages := make([]listWorkItemTemplateLinkageResponse, 0, len(field.Linkages))
		for _, linkage := range field.Linkages {
			linkages = append(linkages, listWorkItemTemplateLinkageResponse{
				Value: linkage.Value, FieldKeys: linkage.FieldKeys,
			})
		}
		result = append(result, listWorkItemTemplateFieldResponse{
			FieldKey: field.FieldKey, DefaultValue: field.DefaultValue, Required: field.Required,
			Linkages: linkages,
		})
	}
	return result
}

func toGetWorkItemTemplateResponse(item business.WorkItemTemplateModel) getWorkItemTemplateResponse {
	fields := make([]getWorkItemTemplateFieldResponse, 0, len(item.Fields))
	for _, field := range item.Fields {
		linkages := make([]getWorkItemTemplateLinkageResponse, 0, len(field.Linkages))
		for _, linkage := range field.Linkages {
			linkages = append(linkages, getWorkItemTemplateLinkageResponse{
				Value: linkage.Value, FieldKeys: linkage.FieldKeys,
			})
		}
		fields = append(fields, getWorkItemTemplateFieldResponse{
			FieldKey: field.FieldKey, DefaultValue: field.DefaultValue, Required: field.Required,
			Linkages: linkages,
		})
	}
	return getWorkItemTemplateResponse{
		Id: item.Id, Name: item.Name, FieldCount: item.FieldCount,
		Title: item.Title, Description: item.Description, DescriptionMode: item.DescriptionMode,
		Fields: fields,
	}
}

func toListWorkItemTemplatePoolResponse(model business.ListWorkItemTemplatePoolModel) []listWorkItemTemplatePoolResponseItem {
	items := make([]listWorkItemTemplatePoolResponseItem, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, listWorkItemTemplatePoolResponseItem{
			Id: item.Id, Name: item.Name, BindCount: item.BindCount, CanDelete: item.CanDelete,
		})
	}
	return items
}

func createFieldsToModel(fields []createWorkItemTemplateFieldRequest) []business.TemplateFieldModel {
	result := make([]business.TemplateFieldModel, 0, len(fields))
	for _, field := range fields {
		linkages := make([]business.TemplateLinkageModel, 0, len(field.Linkages))
		for _, linkage := range field.Linkages {
			linkages = append(linkages, business.TemplateLinkageModel{
				Value: linkage.Value, FieldKeys: linkage.FieldKeys,
			})
		}
		result = append(result, business.TemplateFieldModel{
			FieldKey: field.FieldKey, DefaultValue: field.DefaultValue, Required: field.Required,
			Linkages: linkages,
		})
	}
	return result
}

func updateFieldsToModel(fields []updateWorkItemTemplateFieldRequest) []business.TemplateFieldModel {
	result := make([]business.TemplateFieldModel, 0, len(fields))
	for _, field := range fields {
		linkages := make([]business.TemplateLinkageModel, 0, len(field.Linkages))
		for _, linkage := range field.Linkages {
			linkages = append(linkages, business.TemplateLinkageModel{
				Value: linkage.Value, FieldKeys: linkage.FieldKeys,
			})
		}
		result = append(result, business.TemplateFieldModel{
			FieldKey: field.FieldKey, DefaultValue: field.DefaultValue, Required: field.Required,
			Linkages: linkages,
		})
	}
	return result
}

func workItemTemplateType(c *gin.Context) string {
	return c.DefaultQuery(`type`, define.TypeKeyRequirement)
}

func abortWorkItemTemplateError(c *gin.Context, lang, scene string, err error) {
	switch {
	case errors.Is(err, business.ErrWorkItemTemplateNotFound), errors.Is(err, business.ErrWorkItemTemplateNotLinked):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgWorkItemTemplateNotFound))
	case errors.Is(err, business.ErrWorkItemTemplateInUse):
		abortConflict(c, lib_i18n.Show(lang, define.MsgWorkItemTemplateInUse))
	case errors.Is(err, business.ErrWorkItemTemplateFieldInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgWorkItemTemplateFieldInvalid), scene, err)
	case errors.Is(err, business.ErrWorkItemDisplaySettingInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), scene, err)
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), scene, err)
	}
}

func validateTemplateMeta(name, title, mode string) error {
	if err := validate.WorkItemTemplateName(name); err != nil {
		return err
	}
	if err := validate.WorkItemTemplateTitle(title); err != nil {
		return err
	}
	return validate.WorkItemTemplateDescriptionMode(mode)
}

// ListWorkItemTemplates returns the workspace template switch and linked templates.
func ListWorkItemTemplates(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var query listWorkItemTemplatesQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.list.bind`, err)
		return
	}
	typeKey := query.Type
	if typeKey == `` {
		typeKey = define.TypeKeyRequirement
	}
	if err := validate.WorkItemTemplateType(typeKey); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.list.type`, err)
		return
	}
	model, err := business.ListWorkItemTemplates(business.ListWorkItemTemplatesModel{
		AdminId: c.GetInt(`adminId`), WorkspaceId: c.Param(`wsId`), TypeKey: typeKey,
	})
	if err != nil {
		abortWorkItemTemplateError(c, lang, `work_item_template.list`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: toListWorkItemTemplatesResponse(model),
	})
}

// GetWorkItemTemplate returns one linked template.
func GetWorkItemTemplate(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	typeKey := workItemTemplateType(c)
	if err := validate.WorkItemTemplateType(typeKey); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.get.type`, err)
		return
	}
	templateId, err := positiveParam(c, `id`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.get.id`, err)
		return
	}
	model, err := business.GetWorkItemTemplate(business.GetWorkItemTemplateModel{
		AdminId: c.GetInt(`adminId`), WorkspaceId: c.Param(`wsId`), TemplateId: templateId, TypeKey: typeKey,
	})
	if err != nil {
		abortWorkItemTemplateError(c, lang, `work_item_template.get`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: toGetWorkItemTemplateResponse(model.Item),
	})
}

// CreateWorkItemTemplate creates a pool template and links it to the workspace.
func CreateWorkItemTemplate(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	typeKey := workItemTemplateType(c)
	if err := validate.WorkItemTemplateType(typeKey); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.create.type`, err)
		return
	}
	var req createWorkItemTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.create.bind`, err)
		return
	}
	if err := validateTemplateMeta(req.Name, req.Title, req.DescriptionMode); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgWorkItemTemplateNameInvalid), `work_item_template.create.meta`, err)
		return
	}
	model, err := business.CreateWorkItemTemplate(business.SaveWorkItemTemplateModel{
		AdminId: c.GetInt(`adminId`), UserId: c.GetInt(`userId`), WorkspaceId: c.Param(`wsId`),
		TypeKey: typeKey, Name: req.Name, Title: req.Title, Description: req.Description,
		DescriptionMode: req.DescriptionMode, Fields: createFieldsToModel(req.Fields),
	})
	if err != nil {
		abortWorkItemTemplateError(c, lang, `work_item_template.create`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: createWorkItemTemplateResponse{Id: model.Id},
	})
}

// UpdateWorkItemTemplate updates a linked pool template.
func UpdateWorkItemTemplate(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	typeKey := workItemTemplateType(c)
	if err := validate.WorkItemTemplateType(typeKey); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.update.type`, err)
		return
	}
	templateId, err := positiveParam(c, `id`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.update.id`, err)
		return
	}
	var req updateWorkItemTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.update.bind`, err)
		return
	}
	if err := validateTemplateMeta(req.Name, req.Title, req.DescriptionMode); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgWorkItemTemplateNameInvalid), `work_item_template.update.meta`, err)
		return
	}
	if err := business.UpdateWorkItemTemplate(business.SaveWorkItemTemplateModel{
		AdminId: c.GetInt(`adminId`), WorkspaceId: c.Param(`wsId`), TemplateId: templateId,
		TypeKey: typeKey, Name: req.Name, Title: req.Title, Description: req.Description,
		DescriptionMode: req.DescriptionMode, Fields: updateFieldsToModel(req.Fields),
	}); err != nil {
		abortWorkItemTemplateError(c, lang, `work_item_template.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// UpdateWorkItemTemplateSettings saves the per-type template switch.
func UpdateWorkItemTemplateSettings(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	typeKey := workItemTemplateType(c)
	if err := validate.WorkItemTemplateType(typeKey); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.settings.type`, err)
		return
	}
	var req updateWorkItemTemplateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.settings.bind`, err)
		return
	}
	if err := business.UpdateWorkItemTemplateSettings(business.UpdateWorkItemTemplateSettingsModel{
		AdminId: c.GetInt(`adminId`), WorkspaceId: c.Param(`wsId`), TypeKey: typeKey, Enabled: req.Enabled,
	}); err != nil {
		abortWorkItemTemplateError(c, lang, `work_item_template.settings`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// GetWorkItemDisplaySettings returns edit/view field display rules.
func GetWorkItemDisplaySettings(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	typeKey := workItemTemplateType(c)
	if err := validate.WorkItemTemplateType(typeKey); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_display_settings.get.type`, err)
		return
	}
	model, err := business.GetWorkItemDisplaySetting(business.WorkItemDisplaySettingModel{AdminId: c.GetInt(`adminId`), WorkspaceId: c.Param(`wsId`), TypeKey: typeKey})
	if err != nil {
		abortWorkItemTemplateError(c, lang, `work_item_display_settings.get`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: toWorkItemDisplaySettingsResponse(model)})
}

// UpdateWorkItemDisplaySettings saves complete edit/view field display rules.
func UpdateWorkItemDisplaySettings(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	typeKey := workItemTemplateType(c)
	if err := validate.WorkItemTemplateType(typeKey); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_display_settings.update.type`, err)
		return
	}
	var req workItemDisplaySettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_display_settings.update.bind`, err)
		return
	}
	statusFields := make([]business.WorkItemStatusFieldsModel, 0, len(req.StatusFields))
	for _, item := range req.StatusFields {
		statusFields = append(statusFields, business.WorkItemStatusFieldsModel{StatusId: item.StatusId, FieldKeys: item.FieldKeys})
	}
	err := business.SaveWorkItemDisplaySetting(business.WorkItemDisplaySettingModel{AdminId: c.GetInt(`adminId`), WorkspaceId: c.Param(`wsId`), TypeKey: typeKey, EditDisplayMode: req.EditDisplayMode, ViewDisplayMode: req.ViewDisplayMode, StatusFields: statusFields})
	if err != nil {
		abortWorkItemTemplateError(c, lang, `work_item_display_settings.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// LinkWorkItemTemplates associates pool templates with the workspace.
func LinkWorkItemTemplates(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	typeKey := workItemTemplateType(c)
	if err := validate.WorkItemTemplateType(typeKey); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.link.type`, err)
		return
	}
	var req linkWorkItemTemplatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.link.bind`, err)
		return
	}
	if err := business.LinkWorkItemTemplates(business.LinkWorkItemTemplatesModel{
		AdminId: c.GetInt(`adminId`), WorkspaceId: c.Param(`wsId`), TypeKey: typeKey, TemplateIds: req.Ids,
	}); err != nil {
		abortWorkItemTemplateError(c, lang, `work_item_template.link`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// DeleteWorkItemTemplate removes the template from the current workspace.
func DeleteWorkItemTemplate(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	typeKey := workItemTemplateType(c)
	if err := validate.WorkItemTemplateType(typeKey); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.delete.type`, err)
		return
	}
	templateId, err := positiveParam(c, `id`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.delete.id`, err)
		return
	}
	if err := business.DeleteWorkItemTemplate(business.DeleteWorkItemTemplateModel{
		AdminId: c.GetInt(`adminId`), WorkspaceId: c.Param(`wsId`), TemplateId: templateId, TypeKey: typeKey,
	}); err != nil {
		abortWorkItemTemplateError(c, lang, `work_item_template.delete`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// UnlinkWorkItemTemplate removes a workspace association.
func UnlinkWorkItemTemplate(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	typeKey := workItemTemplateType(c)
	if err := validate.WorkItemTemplateType(typeKey); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.unlink.type`, err)
		return
	}
	templateId, err := positiveParam(c, `id`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.unlink.id`, err)
		return
	}
	if err := business.UnlinkWorkItemTemplate(business.UnlinkWorkItemTemplateModel{
		AdminId: c.GetInt(`adminId`), WorkspaceId: c.Param(`wsId`), TemplateId: templateId, TypeKey: typeKey,
	}); err != nil {
		abortWorkItemTemplateError(c, lang, `work_item_template.unlink`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// ListWorkItemTemplatePool returns unlinked tenant templates.
func ListWorkItemTemplatePool(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var query listWorkItemTemplatePoolQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.pool.bind`, err)
		return
	}
	typeKey := query.Type
	if typeKey == `` {
		typeKey = define.TypeKeyRequirement
	}
	if err := validate.WorkItemTemplateType(typeKey); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.pool.type`, err)
		return
	}
	model, err := business.ListWorkItemTemplatePool(business.ListWorkItemTemplatePoolModel{
		AdminId: c.GetInt(`adminId`), WorkspaceId: c.Param(`wsId`), TypeKey: typeKey,
	})
	if err != nil {
		abortWorkItemTemplateError(c, lang, `work_item_template.pool`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: toListWorkItemTemplatePoolResponse(model),
	})
}

// DeleteWorkItemTemplatePool deletes a pool template that is not linked anywhere.
func DeleteWorkItemTemplatePool(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	typeKey := workItemTemplateType(c)
	if err := validate.WorkItemTemplateType(typeKey); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.pool.delete.type`, err)
		return
	}
	templateId, err := positiveParam(c, `id`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `work_item_template.pool.delete.id`, err)
		return
	}
	if err := business.DeleteWorkItemTemplatePool(business.DeleteWorkItemTemplatePoolModel{
		AdminId: c.GetInt(`adminId`), WorkspaceId: c.Param(`wsId`), TemplateId: templateId, TypeKey: typeKey,
	}); err != nil {
		abortWorkItemTemplateError(c, lang, `work_item_template.pool.delete`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

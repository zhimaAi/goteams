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

// ======================== Field Config DTO ========================

// listFieldConfigsResponseItem is a built-in field config list item.
type listFieldConfigsResponseItem struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Enabled   int    `json:"enabled"`
	Editable  int    `json:"editable"`
	Locked    int    `json:"locked"`
	SortOrder int    `json:"sort_order"`
}

// saveFieldConfigRequestItem is one item in a batch save of built-in field configs.
type saveFieldConfigRequestItem struct {
	Key       string `json:"key"`
	Enabled   int    `json:"enabled"`
	Editable  int    `json:"editable"`
	SortOrder int    `json:"sort_order"`
}

// saveFieldOrderRequestItem is a single field reference in the unified field order.
type saveFieldOrderRequestItem struct {
	Kind string `json:"kind"`
	Key  string `json:"key"`
	Id   int    `json:"id"`
}

// saveFieldOrderRequest is the request to save the full field order.
type saveFieldOrderRequest struct {
	Items []saveFieldOrderRequestItem `json:"items" binding:"required"`
}

type createCustomFieldOptionRequest struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type updateCustomFieldOptionRequest struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// customFieldOptionResponse is a custom field option.
type customFieldOptionResponse struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type customFieldCascaderRequest struct {
	Key      string                       `json:"key"`
	Name     string                       `json:"name"`
	Children []customFieldCascaderRequest `json:"children"`
}

// listCustomFieldsResponseItem is a custom field list item.
type listCustomFieldsResponseItem struct {
	Id          int                           `json:"id"`
	Key         string                        `json:"key"`
	Name        string                        `json:"name"`
	FieldType   string                        `json:"field_type"`
	IsRequired  int                           `json:"is_required"`
	SortOrder   int                           `json:"sort_order"`
	IsEnabled   int                           `json:"is_enabled"`
	Options     []customFieldOptionResponse   `json:"options"`
	IntegerOnly int                           `json:"integer_only"`
	MemberScope string                        `json:"member_scope,omitempty"`
	RoleIds     []int                         `json:"role_ids,omitempty"`
	Cascader    []customFieldCascaderResponse `json:"cascader,omitempty"`
}

type customFieldCascaderResponse struct {
	Key      string                        `json:"key"`
	Name     string                        `json:"name"`
	Children []customFieldCascaderResponse `json:"children,omitempty"`
}

type listCustomFieldEnterpriseRolesResponseItem struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

// createCustomFieldRequest is the request to create a custom field.
type createCustomFieldRequest struct {
	Name        string                           `json:"name"`
	FieldType   string                           `json:"field_type"`
	IsRequired  int                              `json:"is_required"`
	Options     []createCustomFieldOptionRequest `json:"options"`
	IntegerOnly int                              `json:"integer_only"`
	MemberScope string                           `json:"member_scope"`
	RoleIds     []int                            `json:"role_ids"`
	Cascader    []customFieldCascaderRequest     `json:"cascader"`
}

// createCustomFieldResponse is the response for creating a custom field.
type createCustomFieldResponse struct {
	Id int `json:"id"`
}

// updateCustomFieldRequest is the request to edit a custom field.
type updateCustomFieldRequest struct {
	Name        string                           `json:"name"`
	FieldType   string                           `json:"field_type"`
	IsRequired  int                              `json:"is_required"`
	IsEnabled   int                              `json:"is_enabled"`
	Options     []updateCustomFieldOptionRequest `json:"options"`
	IntegerOnly int                              `json:"integer_only"`
	MemberScope string                           `json:"member_scope"`
	RoleIds     []int                            `json:"role_ids"`
	Cascader    []customFieldCascaderRequest     `json:"cascader"`
}

// toListFieldConfigsResponse converts built-in field config models into the response.
func toListFieldConfigsResponse(model business.ListFieldConfigsModel) []listFieldConfigsResponseItem {
	items := make([]listFieldConfigsResponseItem, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, listFieldConfigsResponseItem{
			Key:       item.Key,
			Label:     item.Label,
			Enabled:   item.Enabled,
			Editable:  item.Editable,
			Locked:    item.Locked,
			SortOrder: item.SortOrder,
		})
	}
	return items
}

// toListCustomFieldsResponse converts custom field models into the response.
func toListCustomFieldsResponse(model business.ListCustomFieldsModel) []listCustomFieldsResponseItem {
	items := make([]listCustomFieldsResponseItem, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, listCustomFieldsResponseItem{
			Id:          item.Id,
			Key:         item.Key,
			Name:        item.Name,
			FieldType:   item.FieldType,
			IsRequired:  item.IsRequired,
			SortOrder:   item.SortOrder,
			IsEnabled:   item.IsEnabled,
			Options:     toCustomFieldOptionsResponse(item.Options),
			IntegerOnly: item.IntegerOnly,
			MemberScope: item.MemberScope,
			RoleIds:     item.RoleIds,
			Cascader:    toCustomFieldCascaderResponse(item.Cascader),
		})
	}
	return items
}

// toCustomFieldOptionsResponse converts custom field options; keeps null when empty to match the existing contract.
func toCustomFieldOptionsResponse(options []business.CustomFieldOption) []customFieldOptionResponse {
	if options == nil {
		return nil
	}
	result := make([]customFieldOptionResponse, 0, len(options))
	for _, option := range options {
		result = append(result, customFieldOptionResponse{Key: option.Key, Name: option.Name, Color: option.Color})
	}
	return result
}

func toCustomFieldCascaderResponse(nodes []business.CustomFieldCascaderNode) []customFieldCascaderResponse {
	if len(nodes) == 0 {
		return nil
	}
	result := make([]customFieldCascaderResponse, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, customFieldCascaderResponse{
			Key:      node.Key,
			Name:     node.Name,
			Children: toCustomFieldCascaderResponse(node.Children),
		})
	}
	return result
}

func toCreateCustomFieldOptionInput(options []createCustomFieldOptionRequest) []business.CustomFieldOptionInput {
	result := make([]business.CustomFieldOptionInput, 0, len(options))
	for _, option := range options {
		result = append(result, business.CustomFieldOptionInput{Key: option.Key, Name: option.Name})
	}
	return result
}

func toUpdateCustomFieldOptionInput(options []updateCustomFieldOptionRequest) []business.CustomFieldOptionInput {
	result := make([]business.CustomFieldOptionInput, 0, len(options))
	for _, option := range options {
		result = append(result, business.CustomFieldOptionInput{Key: option.Key, Name: option.Name})
	}
	return result
}

func toCustomFieldCascaderInput(nodes []customFieldCascaderRequest) []business.CustomFieldCascaderInput {
	result := make([]business.CustomFieldCascaderInput, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, business.CustomFieldCascaderInput{
			Key:      node.Key,
			Name:     node.Name,
			Children: toCustomFieldCascaderInput(node.Children),
		})
	}
	return result
}

func toListCustomFieldEnterpriseRolesResponse(roles []business.EnterpriseRoleBriefModel, lang string) []listCustomFieldEnterpriseRolesResponseItem {
	items := make([]listCustomFieldEnterpriseRolesResponseItem, 0, len(roles))
	for _, role := range roles {
		items = append(items, listCustomFieldEnterpriseRolesResponseItem{
			Id:   role.Id,
			Name: lib_i18n.ShowStored(lang, role.Name),
		})
	}
	return items
}

func customFieldNameValidationMessage(lang string, err error) string {
	switch {
	case errors.Is(err, validate.ErrCustomFieldNameRequired):
		return lib_i18n.Show(lang, define.MsgCustomFieldNameRequired)
	case errors.Is(err, validate.ErrCustomFieldNameTooLong):
		return lib_i18n.Show(lang, define.MsgCustomFieldNameTooLong, 20)
	default:
		return lib_i18n.Show(lang, define.MsgParamInvalid)
	}
}

func abortCustomFieldWriteError(c *gin.Context, lang, where string, err error) bool {
	if errors.Is(err, business.ErrCustomFieldConfigInvalid) {
		abortBadRequest(c, business.CustomFieldValidationMessage(lang, err), where, err)
		return true
	}
	return false
}

// ======================== Built-in Field Config (Attribute Field Switches) ========================

// ListFieldConfigs returns the built-in field config list for the given type under a workspace.
func ListFieldConfigs(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	model, err := business.ListFieldConfigs(business.ListFieldConfigsModel{
		AdminId: c.GetInt(`adminId`),
		WsId:    c.Param(`wsId`),
		TypeKey: c.DefaultQuery(`type`, define.TypeKeyRequirement),
		Lang:    lang,
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `field_config.list`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toListFieldConfigsResponse(model),
	})
}

// SaveFieldConfigs batch-saves built-in field configs of the given type under a workspace.
func SaveFieldConfigs(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req []saveFieldConfigRequestItem
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `field_config.bind`, err)
		return
	}
	items := make([]business.SaveFieldConfigItemModel, 0, len(req))
	for _, item := range req {
		items = append(items, business.SaveFieldConfigItemModel{
			Key:       item.Key,
			Enabled:   item.Enabled,
			Editable:  item.Editable,
			SortOrder: item.SortOrder,
		})
	}
	if err := business.SaveFieldConfigs(business.SaveFieldConfigsModel{
		AdminId: c.GetInt(`adminId`),
		WsId:    c.Param(`wsId`),
		TypeKey: c.DefaultQuery(`type`, define.TypeKeyRequirement),
		Items:   items,
	}); err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `field_config.save`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// SaveFieldOrder saves the unified order of built-in and custom fields for a work item type. // ResetFieldConfigs restores built-in fields of the given type to the default enabled state and order.
func ResetFieldConfigs(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	if err := business.ResetFieldConfigs(business.ResetFieldConfigsModel{
		AdminId: c.GetInt(`adminId`),
		WsId:    c.Param(`wsId`),
		TypeKey: c.DefaultQuery(`type`, define.TypeKeyRequirement),
	}); err != nil {
		if errors.Is(err, business.ErrFieldOrderInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `field_config.reset`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `field_config.reset`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// SaveFieldOrder saves the unified order of built-in and custom fields for a work item type.
func SaveFieldOrder(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req saveFieldOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `field_order.bind`, err)
		return
	}
	items := make([]business.FieldOrderItemModel, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, business.FieldOrderItemModel{Kind: item.Kind, Key: item.Key, Id: item.Id})
	}
	if err := business.SaveFieldOrder(business.SaveFieldOrderModel{
		AdminId: c.GetInt(`adminId`),
		WsId:    c.Param(`wsId`),
		TypeKey: c.DefaultQuery(`type`, define.TypeKeyRequirement),
		Items:   items,
	}); err != nil {
		if errors.Is(err, business.ErrFieldOrderInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `field_order.validate`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `field_order.save`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// ======================== Custom Field CRUD ========================

// ListCustomFields returns the custom field list of a specified type under the workspace.
func ListCustomFields(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	model, err := business.ListCustomFields(business.ListCustomFieldsModel{
		AdminId: c.GetInt(`adminId`),
		WsId:    c.Param(`wsId`),
		TypeKey: c.DefaultQuery(`type`, define.TypeKeyRequirement),
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `custom_field.list`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toListCustomFieldsResponse(model),
	})
}

// ListCustomFieldEnterpriseRoles returns the minimal enterprise role options used by member custom fields.
func ListCustomFieldEnterpriseRoles(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	roles, err := business.ListEnterpriseRoleBriefs(c.GetInt(`adminId`))
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `custom_field.enterprise_roles`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toListCustomFieldEnterpriseRolesResponse(roles, lang),
	})
}

// CreateCustomField creates a custom field.
func CreateCustomField(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req createCustomFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `custom_field.create.bind`, err)
		return
	}
	if err := validate.ValidateCustomFieldName(req.Name); err != nil {
		abortBadRequest(c, customFieldNameValidationMessage(lang, err), `custom_field.create.name`, err)
		return
	}
	if err := validate.ValidateNewCustomFieldType(req.FieldType); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgCustomFieldTypeInvalid, req.Name), `custom_field.create.type`, err)
		return
	}
	model, err := business.CreateCustomField(business.CreateCustomFieldModel{
		AdminId:     c.GetInt(`adminId`),
		WsId:        c.Param(`wsId`),
		TypeKey:     c.DefaultQuery(`type`, define.TypeKeyRequirement),
		Name:        req.Name,
		FieldType:   req.FieldType,
		IsRequired:  req.IsRequired,
		Options:     toCreateCustomFieldOptionInput(req.Options),
		IntegerOnly: req.IntegerOnly,
		MemberScope: req.MemberScope,
		RoleIds:     req.RoleIds,
		Cascader:    toCustomFieldCascaderInput(req.Cascader),
	})
	if err != nil {
		if abortQuotaExceeded(c, lib_i18n.Show(lang, define.MsgQuotaExceeded), `custom_field.create.quota`, err) {
			return
		}
		if abortCustomFieldWriteError(c, lang, `custom_field.create.config`, err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `custom_field.create`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    createCustomFieldResponse{Id: model.Id},
	})
}

// UpdateCustomField edits a custom field.
func UpdateCustomField(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req updateCustomFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `custom_field.update.bind`, err)
		return
	}
	if err := validate.ValidateCustomFieldName(req.Name); err != nil {
		abortBadRequest(c, customFieldNameValidationMessage(lang, err), `custom_field.update.name`, err)
		return
	}
	if err := validate.ValidateCustomFieldType(req.FieldType); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgCustomFieldTypeInvalid, req.Name), `custom_field.update.type`, err)
		return
	}
	if err := business.UpdateCustomField(business.UpdateCustomFieldModel{
		AdminId:     c.GetInt(`adminId`),
		WsId:        c.Param(`wsId`),
		TypeKey:     c.DefaultQuery(`type`, define.TypeKeyRequirement),
		FieldId:     cast.ToInt(c.Param(`fieldId`)),
		Name:        req.Name,
		FieldType:   req.FieldType,
		IsRequired:  req.IsRequired,
		IsEnabled:   req.IsEnabled,
		Options:     toUpdateCustomFieldOptionInput(req.Options),
		IntegerOnly: req.IntegerOnly,
		MemberScope: req.MemberScope,
		RoleIds:     req.RoleIds,
		Cascader:    toCustomFieldCascaderInput(req.Cascader),
	}); err != nil {
		if abortCustomFieldWriteError(c, lang, `custom_field.update.config`, err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `custom_field.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// DeleteCustomField deletes a custom field.
func DeleteCustomField(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	if err := business.DeleteCustomField(business.DeleteCustomFieldModel{
		AdminId: c.GetInt(`adminId`),
		WsId:    c.Param(`wsId`),
		TypeKey: c.DefaultQuery(`type`, define.TypeKeyRequirement),
		FieldId: cast.ToInt(c.Param(`fieldId`)),
	}); err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `custom_field.delete`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

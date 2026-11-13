// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
)

// ======================== Work Item Attribute DTO ========================

// workItemAttributeLimit* mirror the front-end input caps for the attribute form.
const (
	workItemAttributeNameLimit = 20
	workItemAttributeIconLimit = 2
	workItemAttributeDescLimit = 200
	workItemAttributeColorLen  = 7 // #RRGGBB
)

// workItemAttributeResponseItem is one configured attribute row in the response.
type workItemAttributeResponseItem struct {
	TypeKey     string `json:"type_key"`
	Name        string `json:"name"`
	IconColor   string `json:"icon_color"`
	IconText    string `json:"icon_text"`
	Description string `json:"description"`
}

// saveWorkItemAttributeRequest is the request to save one type's attribute config.
type saveWorkItemAttributeRequest struct {
	Name        string `json:"name"`
	IconColor   string `json:"icon_color"`
	IconText    string `json:"icon_text"`
	Description string `json:"description"`
}

// toWorkItemAttributeResponse converts attribute models to the response items.
func toWorkItemAttributeResponse(model business.ListWorkItemAttributesModel) []workItemAttributeResponseItem {
	items := make([]workItemAttributeResponseItem, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, workItemAttributeResponseItem{
			TypeKey: item.TypeKey, Name: item.Name, IconColor: item.IconColor,
			IconText: item.IconText, Description: item.Description,
		})
	}
	return items
}

// isHexColor reports whether s is a #RRGGBB hex color.
func isHexColor(s string) bool {
	if len(s) != workItemAttributeColorLen || s[0] != '#' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			continue
		}
		return false
	}
	return true
}

// ======================== Work Item Attribute Handlers ========================

// GetWorkItemAttributes returns the workspace's configured attribute rows (optionally filtered by ?type=).
func GetWorkItemAttributes(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	typeKey := c.Query(`type`)
	if len(typeKey) > 0 && !validWorkItemType(typeKey) {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workitem_attribute.list.type`, nil)
		return
	}
	model, err := business.ListWorkItemAttributes(business.ListWorkItemAttributesModel{
		AdminId: c.GetInt(`adminId`),
		WsId:    c.Param(`wsId`),
		TypeKey: typeKey,
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workitem_attribute.list`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toWorkItemAttributeResponse(model),
	})
}

// SaveWorkItemAttribute upserts one type's attribute config (scoped to the workspace).
func SaveWorkItemAttribute(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	typeKey := c.DefaultQuery(`type`, define.TypeKeyRequirement)
	if !validWorkItemType(typeKey) {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workitem_attribute.save.type`, nil)
		return
	}
	// The route middleware uses OR semantics; tighten here to the type's own config permission.
	permissionKey := `requirement_setting:attribute`
	if typeKey == define.TypeKeyDefect {
		permissionKey = `defect_setting:attribute`
	}
	if !business.HasPermission(c.GetInt(`adminId`), c.GetInt(`userId`), c.Param(`wsId`), permissionKey) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}
	var req saveWorkItemAttributeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workitem_attribute.save.bind`, err)
		return
	}
	name := strings.TrimSpace(req.Name)
	iconText := strings.TrimSpace(req.IconText)
	description := strings.TrimSpace(req.Description)
	if len(name) == 0 || utf8.RuneCountInString(name) > workItemAttributeNameLimit {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workitem_attribute.save.name`, nil)
		return
	}
	if len(iconText) == 0 || utf8.RuneCountInString(iconText) > workItemAttributeIconLimit {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workitem_attribute.save.icon_text`, nil)
		return
	}
	if !isHexColor(req.IconColor) {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workitem_attribute.save.icon_color`, nil)
		return
	}
	if utf8.RuneCountInString(description) > workItemAttributeDescLimit {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workitem_attribute.save.description`, nil)
		return
	}
	if err := business.SaveWorkItemAttribute(business.SaveWorkItemAttributeModel{
		AdminId:     c.GetInt(`adminId`),
		WsId:        c.Param(`wsId`),
		TypeKey:     typeKey,
		Name:        name,
		IconColor:   req.IconColor,
		IconText:    iconText,
		Description: description,
	}); err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workitem_attribute.save`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// validWorkItemType reports whether the type supports a per-type attribute config.
func validWorkItemType(typeKey string) bool {
	return typeKey == define.TypeKeyRequirement || typeKey == define.TypeKeyDefect
}

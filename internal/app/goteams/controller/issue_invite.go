// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
)

type issueInviteTemplateCreateRequest struct {
	TypeKey          string `json:"type_key"`
	Name             string `json:"name"`
	DescriptionMode  string `json:"description_mode"`
	FieldsJSON       string `json:"fields_json"`
	ThemeImage       string `json:"theme_image"`
	GuideTitle       string `json:"guide_title"`
	GuideDescription string `json:"guide_description"`
	GuideEnabled     int    `json:"guide_enabled"`
	Enabled          int    `json:"enabled"`
	RecallEnabled    int    `json:"recall_enabled"`
	RecallStatusID   int    `json:"recall_status_id"`
}

type issueInviteOrderRequest struct {
	TemplateIDs []int `json:"template_ids"`
}

type issueInviteTemplateUpdateRequest struct {
	Name             string `json:"name"`
	DescriptionMode  string `json:"description_mode"`
	FieldsJSON       string `json:"fields_json"`
	ThemeImage       string `json:"theme_image"`
	GuideTitle       string `json:"guide_title"`
	GuideDescription string `json:"guide_description"`
	GuideEnabled     int    `json:"guide_enabled"`
	Enabled          int    `json:"enabled"`
	RecallEnabled    int    `json:"recall_enabled"`
	RecallStatusID   int    `json:"recall_status_id"`
}

type issueInviteSubmitRequest struct {
	OpenID  string                         `json:"openid"`
	Fields  []define.IssueInviteFieldValue `json:"fields"`
	FileIDs []int                          `json:"file_ids"`
}

type issueInviteCommentRequest struct {
	OpenID  string `json:"openid"`
	Content string `json:"content"`
}

// issueInviteRequirementActionRequest 是公开撤回/催单/重提的请求体：
// 撤回和催单只带 openid，重新提交再多带字段与新增附件。
type issueInviteRequirementActionRequest struct {
	OpenID  string                         `json:"openid"`
	Fields  []define.IssueInviteFieldValue `json:"fields"`
	FileIDs []int                          `json:"file_ids"`
}

type issueInviteTemplateResponse struct {
	ID               int    `json:"id,omitempty"`
	TemplateKey      string `json:"template_key,omitempty"`
	TypeKey          string `json:"type_key"`
	TypeName         string `json:"type_name"`
	TypeIconColor    string `json:"type_icon_color,omitempty"`
	TypeIconText     string `json:"type_icon_text,omitempty"`
	Name             string `json:"name"`
	DescriptionMode  string `json:"description_mode,omitempty"`
	Enabled          int    `json:"enabled"`
	SortOrder        int    `json:"sort_order"`
	ThemeImage       string `json:"theme_image,omitempty"`
	GuideTitle       string `json:"guide_title,omitempty"`
	GuideDescription string `json:"guide_description,omitempty"`
	// 模板自己的邀请文案开关，与项目级配置互不同步
	GuideEnabled int    `json:"guide_enabled"`
	FieldsJSON   string `json:"fields_json,omitempty"`
	// 撤回配置只在管理端响应里出现，见 toIssueInviteManageResponse
	RecallEnabled  int `json:"recall_enabled"`
	RecallStatusID int `json:"recall_status_id"`
}

type issueInviteConfigUpdateRequest struct {
	GuideTitle       string `json:"guide_title"`
	GuideDescription string `json:"guide_description"`
	ThemeImage       string `json:"theme_image"`
	Enabled          int    `json:"enabled"`
}

type issueInviteConfigResponse struct {
	InviteKey        string `json:"invite_key"`
	Enabled          int    `json:"enabled"`
	GuideTitle       string `json:"guide_title,omitempty"`
	GuideDescription string `json:"guide_description,omitempty"`
	ThemeImage       string `json:"theme_image,omitempty"`
}

type issueInviteShareResponse struct {
	InviteKey        string                        `json:"invite_key"`
	InviteURL        string                        `json:"invite_url,omitempty"`
	GuideTitle       string                        `json:"guide_title,omitempty"`
	GuideDescription string                        `json:"guide_description,omitempty"`
	ThemeImage       string                        `json:"theme_image,omitempty"`
	Templates        []issueInviteTemplateResponse `json:"templates"`
}

type publicIssueInviteResponse struct {
	GuideTitle       string                        `json:"guide_title,omitempty"`
	GuideDescription string                        `json:"guide_description,omitempty"`
	ThemeImage       string                        `json:"theme_image,omitempty"`
	Templates        []issueInviteTemplateResponse `json:"templates"`
}

type publicIssueHistoryResponse struct {
	Items    []define.IssueInviteRequirementModel `json:"items"`
	Page     int                                  `json:"page"`
	PageSize int                                  `json:"page_size"`
	Total    int                                  `json:"total"`
}

// issueInviteTemplateTypeName maps a type key to its display name.
func issueInviteTemplateTypeName(lang, typeKey string) string {
	switch typeKey {
	case define.TypeKeyRequirement:
		return lib_i18n.Show(lang, `workitem_type_requirement`)
	case define.TypeKeyDefect:
		return lib_i18n.Show(lang, `workitem_type_defect`)
	default:
		return typeKey
	}
}

// loadIssueInviteTypeAttrs reads the workspace's configured type attributes so public
// template rows can render the same type badges as the workspace pages; a read failure
// degrades to the built-in badge defaults instead of failing the whole public page.
func loadIssueInviteTypeAttrs(stage string, adminID, workspaceID int) map[string]business.WorkItemAttributeModel {
	model, err := business.ListWorkItemAttributes(business.ListWorkItemAttributesModel{
		AdminId: adminID,
		WsId:    cast.ToString(workspaceID),
	})
	if err != nil {
		logs.Error("%s load work item type attrs failed, err:%s", stage, err.Error())
		return nil
	}
	attrs := make(map[string]business.WorkItemAttributeModel, len(model.Items))
	for _, item := range model.Items {
		attrs[item.TypeKey] = item
	}
	return attrs
}

// fillIssueInviteTypeAttrs copies the configured icon color/text onto template type badges.
func fillIssueInviteTypeAttrs(items []issueInviteTemplateResponse, attrs map[string]business.WorkItemAttributeModel) {
	if len(attrs) == 0 {
		return
	}
	for i := range items {
		attr, ok := attrs[items[i].TypeKey]
		if !ok || len(attr.IconColor) == 0 {
			continue
		}
		items[i].TypeIconColor = attr.IconColor
		items[i].TypeIconText = attr.IconText
	}
}

// toIssueInviteTemplateResponse maps one copied/source template to the response DTO. It is
// the public shape too, so the recall config stays out.
func toIssueInviteTemplateResponse(lang string, t define.IssueInviteTemplateModel) issueInviteTemplateResponse {
	return issueInviteTemplateResponse{
		ID:               t.ID,
		TemplateKey:      t.InviteKey,
		TypeKey:          t.TypeKey,
		TypeName:         issueInviteTemplateTypeName(lang, t.TypeKey),
		Name:             t.Name,
		DescriptionMode:  t.DescriptionMode,
		Enabled:          t.Enabled,
		SortOrder:        t.SortOrder,
		ThemeImage:       t.ThemeImage,
		GuideTitle:       t.GuideTitle,
		GuideDescription: t.GuideDescription,
		GuideEnabled:     t.GuideEnabled,
		FieldsJSON:       t.FieldsJSON,
	}
}

// toIssueInviteManageResponse is the admin shape: the template settings page edits the
// recall config, so it has to come back.
func toIssueInviteManageResponse(lang string, t define.IssueInviteTemplateModel) issueInviteTemplateResponse {
	item := toIssueInviteTemplateResponse(lang, t)
	item.RecallEnabled = t.RecallEnabled
	item.RecallStatusID = t.RecallStatusID
	return item
}

// issueInviteTemplateResponseList maps a template list to response items.
func issueInviteTemplateResponseList(lang string, items []define.IssueInviteTemplateModel) []issueInviteTemplateResponse {
	result := make([]issueInviteTemplateResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toIssueInviteTemplateResponse(lang, item))
	}
	return result
}

// issueInviteManageResponseList is issueInviteTemplateResponseList with the recall config.
func issueInviteManageResponseList(lang string, items []define.IssueInviteTemplateModel) []issueInviteTemplateResponse {
	result := make([]issueInviteTemplateResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toIssueInviteManageResponse(lang, item))
	}
	return result
}

// toIssueInviteConfigResponse maps the invitation configuration to the response DTO.
func toIssueInviteConfigResponse(config define.IssueInviteConfigModel) issueInviteConfigResponse {
	return issueInviteConfigResponse{
		InviteKey:        config.InviteKey,
		Enabled:          config.Enabled,
		GuideTitle:       config.GuideTitle,
		GuideDescription: config.GuideDescription,
		ThemeImage:       config.ThemeImage,
	}
}

// abortIssueInviteError maps issue-invitation errors to a controlled HTTP response.
func abortIssueInviteError(c *gin.Context, lang, stage string, err error) {
	switch {
	case errors.Is(err, business.ErrIssueInviteInvalid):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgIssueInviteInvalid))
	case errors.Is(err, business.ErrIssueInviteTemplateNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgIssueInviteTemplateNotFound))
	case errors.Is(err, business.ErrRequirementNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
	case errors.Is(err, business.ErrIssueInviteTemplateDisabled):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgIssueInviteTemplateDisabled), stage, err)
	case errors.Is(err, business.ErrCustomFieldValueInvalid):
		// 自定义字段的校验错误自带字段名和原因，直接渲染，不要压成通用提示
		abortBadRequest(c, business.IssueInviteFieldValidationMessage(lang, err), stage, err)
	case errors.Is(err, business.ErrIssueInviteFieldInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgIssueInviteFieldInvalid), stage, err)
	case errors.Is(err, business.ErrIssueInviteOrderInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgIssueInviteOrderInvalid), stage, err)
	case errors.Is(err, business.ErrIssueInviteRateLimited):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgIssueInviteRateLimited), stage, err)
	case errors.Is(err, business.ErrIssueInviteAppDisabled):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgIssueInviteAppDisabled), stage, err)
	case errors.Is(err, business.ErrIssueInviteRecallStatusInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgIssueInviteRecallStatusInvalid), stage, err)
	case errors.Is(err, business.ErrIssueInviteRecallDisabled):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgIssueInviteRecallDisabled), stage, err)
	case errors.Is(err, business.ErrIssueInviteRecallNotAllowed):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgIssueInviteRecallNotAllowed), stage, err)
	case errors.Is(err, business.ErrIssueInviteResubmitNotAllowed):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgIssueInviteResubmitNotAllowed), stage, err)
	case errors.Is(err, business.ErrIssueInviteUrgeTooSoon):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgIssueInviteUrgeTooSoon), stage, err)
	case errors.Is(err, business.ErrIssueInviteUrgeNotAllowed):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgIssueInviteUrgeNotAllowed), stage, err)
	case errors.Is(err, business.ErrIssueInviteLinkParamInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgIssueInviteLinkParamInvalid), stage, err)
	case errors.Is(err, validate.ErrIssueInviteKeyInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgIssueInviteKeyInvalid), stage, err)
	case errors.Is(err, validate.ErrIssueInviteOpenIDInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgIssueInviteOpenIDInvalid), stage, err)
	case errors.Is(err, validate.ErrIssueInvitePageInvalid),
		errors.Is(err, validate.ErrIssueInviteSubmissionInvalid),
		errors.Is(err, validate.ErrIssueInviteTemplateInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage, err)
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), stage, err)
	}
}

// issueInviteBindParams parses the authenticated workspace scope from route params.
func issueInviteBindParams(c *gin.Context) (adminID, workspaceID int, ok bool) {
	adminID = cast.ToInt(c.GetInt(`adminId`))
	workspaceID, err := positiveParam(c, `wsId`)
	if err != nil {
		lang := lib_i18n.GetLang(c)
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `issue_invite.wsId`, err)
		return 0, 0, false
	}
	return adminID, workspaceID, true
}

// GetIssueInviteShare returns authenticated general/template sharing metadata.
func GetIssueInviteShare(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminID, workspaceID, ok := issueInviteBindParams(c)
	if !ok {
		return
	}
	config, err := business.GetIssueInviteConfig(adminID, workspaceID)
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.share.config`, err)
		return
	}
	// Config-level enabled state; guide fields are returned as-is.
	templates, err := business.ListIssueInviteTemplates(adminID, workspaceID, false)
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.share.templates`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: issueInviteShareResponse{
			InviteKey:        config.InviteKey,
			GuideTitle:       config.GuideTitle,
			GuideDescription: config.GuideDescription,
			ThemeImage:       config.ThemeImage,
			Templates:        issueInviteManageResponseList(lang, templates),
		},
	})
}

// ListIssueInviteTemplates returns authenticated copied templates.
func ListIssueInviteTemplates(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminID, workspaceID, ok := issueInviteBindParams(c)
	if !ok {
		return
	}
	// Public template keys are only exposed to callers who can copy the sharing link.
	enabled, err := business.IsWorkspaceApplicationEnabled(adminID, workspaceID, define.AppKeyIssue)
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.templates.app`, err)
		return
	}
	if !enabled {
		abortIssueInviteError(c, lang, `issue_invite.templates`, business.ErrIssueInviteAppDisabled)
		return
	}
	canCopy := business.HasPermission(adminID, cast.ToInt(c.GetInt(`userId`)), cast.ToString(workspaceID), define.PermissionTicketCopyLink)
	templates, err := business.ListIssueInviteTemplates(adminID, workspaceID, false)
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.templates.list`, err)
		return
	}
	items := issueInviteManageResponseList(lang, templates)
	if !canCopy {
		for i := range items {
			items[i].TemplateKey = ``
		}
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    items,
	})
}

// CreateIssueInviteTemplate creates one independent invitation template.
func CreateIssueInviteTemplate(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminID, workspaceID, ok := issueInviteBindParams(c)
	if !ok {
		return
	}
	var req issueInviteTemplateCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `issue_invite.template.create.bind`, err)
		return
	}
	model := define.IssueInviteTemplateCreateModel{
		AdminID:          adminID,
		WorkspaceID:      workspaceID,
		TypeKey:          req.TypeKey,
		Name:             req.Name,
		DescriptionMode:  req.DescriptionMode,
		FieldsJSON:       req.FieldsJSON,
		ThemeImage:       req.ThemeImage,
		GuideTitle:       req.GuideTitle,
		GuideDescription: req.GuideDescription,
		GuideEnabled:     req.GuideEnabled,
		Enabled:          req.Enabled,
		RecallEnabled:    req.RecallEnabled,
		RecallStatusID:   req.RecallStatusID,
		OperatorID:       cast.ToInt(c.GetInt(`userId`)),
	}
	if err := validate.IssueInviteTemplateCreate(model); err != nil {
		msgKey := define.MsgWorkItemTemplateNameInvalid
		switch {
		case errors.Is(err, validate.ErrIssueInviteTemplateInvalid):
			msgKey = define.MsgWorkItemTemplateFieldInvalid
		case errors.Is(err, validate.ErrIssueInviteGuideTitleInvalid):
			msgKey = define.MsgIssueInviteGuideTitleInvalid
		case errors.Is(err, validate.ErrIssueInviteGuideDescriptionInvalid):
			msgKey = define.MsgIssueInviteGuideDescriptionInvalid
		case errors.Is(err, validate.ErrIssueInviteThemeImageInvalid):
			msgKey = define.MsgIssueInviteThemeImageInvalid
		}
		abortBadRequest(c, lib_i18n.Show(lang, msgKey), `issue_invite.template.create.validate`, err)
		return
	}
	template, err := business.CreateIssueInviteTemplate(model)
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.template.create`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toIssueInviteManageResponse(lang, template),
	})
}

// ReorderIssueInviteTemplates replaces the complete copied-template order.
func ReorderIssueInviteTemplates(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminID, workspaceID, ok := issueInviteBindParams(c)
	if !ok {
		return
	}
	var req issueInviteOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `issue_invite.order.bind`, err)
		return
	}
	err := business.ReorderIssueInviteTemplates(define.IssueInviteTemplateOrderModel{
		AdminID: adminID, WorkspaceID: workspaceID, OrderedTemplateID: req.TemplateIDs,
	})
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.order`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// GetIssueInviteTemplate returns one authenticated copied-template detail.
func GetIssueInviteTemplate(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminID, workspaceID, ok := issueInviteBindParams(c)
	if !ok {
		return
	}
	templateID, err := positiveParam(c, `templateId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `issue_invite.template.id`, err)
		return
	}
	template, err := business.GetIssueInviteTemplate(adminID, workspaceID, templateID)
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.template.get`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toIssueInviteManageResponse(lang, template),
	})
}

// UpdateIssueInviteTemplate updates one copied template.
func UpdateIssueInviteTemplate(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminID, workspaceID, ok := issueInviteBindParams(c)
	if !ok {
		return
	}
	templateID, err := positiveParam(c, `templateId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `issue_invite.template.id`, err)
		return
	}
	var req issueInviteTemplateUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `issue_invite.template.bind`, err)
		return
	}
	model := define.IssueInviteTemplateUpdateModel{
		AdminID:          adminID,
		WorkspaceID:      workspaceID,
		TemplateID:       templateID,
		Name:             req.Name,
		DescriptionMode:  req.DescriptionMode,
		FieldsJSON:       req.FieldsJSON,
		ThemeImage:       req.ThemeImage,
		GuideTitle:       req.GuideTitle,
		GuideDescription: req.GuideDescription,
		GuideEnabled:     req.GuideEnabled,
		Enabled:          req.Enabled,
		RecallEnabled:    req.RecallEnabled,
		RecallStatusID:   req.RecallStatusID,
		OperatorID:       cast.ToInt(c.GetInt(`userId`)),
	}
	if err := validate.IssueInviteTemplateUpdate(model); err != nil {
		// The schema/enabled errors are distinct from the name/title/mode text checks.
		msgKey := define.MsgWorkItemTemplateNameInvalid
		switch {
		case errors.Is(err, validate.ErrIssueInviteTemplateInvalid):
			msgKey = define.MsgWorkItemTemplateFieldInvalid
		case errors.Is(err, validate.ErrIssueInviteGuideTitleInvalid):
			msgKey = define.MsgIssueInviteGuideTitleInvalid
		case errors.Is(err, validate.ErrIssueInviteGuideDescriptionInvalid):
			msgKey = define.MsgIssueInviteGuideDescriptionInvalid
		case errors.Is(err, validate.ErrIssueInviteThemeImageInvalid):
			msgKey = define.MsgIssueInviteThemeImageInvalid
		}
		abortBadRequest(c, lib_i18n.Show(lang, msgKey), `issue_invite.template.validate`, err)
		return
	}
	template, err := business.UpdateIssueInviteTemplate(model)
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.template.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toIssueInviteManageResponse(lang, template),
	})
}

// RemoveIssueInviteTemplate removes one copied template and keeps history snapshots.
func RemoveIssueInviteTemplate(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminID, workspaceID, ok := issueInviteBindParams(c)
	if !ok {
		return
	}
	templateID, err := positiveParam(c, `templateId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `issue_invite.template.id`, err)
		return
	}
	if err := business.RemoveIssueInviteTemplate(adminID, workspaceID, templateID, cast.ToInt(c.GetInt(`userId`))); err != nil {
		abortIssueInviteError(c, lang, `issue_invite.template.remove`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// GetIssueInviteConfig returns the authenticated invitation guide configuration.
func GetIssueInviteConfig(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminID, workspaceID, ok := issueInviteBindParams(c)
	if !ok {
		return
	}
	config, err := business.GetIssueInviteConfig(adminID, workspaceID)
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.config.get`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toIssueInviteConfigResponse(config),
	})
}

// UpdateIssueInviteConfig updates the invitation guide configuration.
func UpdateIssueInviteConfig(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminID, workspaceID, ok := issueInviteBindParams(c)
	if !ok {
		return
	}
	var req issueInviteConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `issue_invite.config.bind`, err)
		return
	}
	model := define.IssueInviteConfigUpdateModel{
		AdminID:          adminID,
		WorkspaceID:      workspaceID,
		GuideTitle:       req.GuideTitle,
		GuideDescription: req.GuideDescription,
		ThemeImage:       req.ThemeImage,
		Enabled:          req.Enabled,
		OperatorID:       cast.ToInt(c.GetInt(`userId`)),
	}
	if err := validate.IssueInviteConfigUpdate(model); err != nil {
		msgKey := define.MsgIssueInviteGuideTitleInvalid
		if errors.Is(err, validate.ErrIssueInviteGuideDescriptionInvalid) {
			msgKey = define.MsgIssueInviteGuideDescriptionInvalid
		}
		abortBadRequest(c, lib_i18n.Show(lang, msgKey), `issue_invite.config.validate`, err)
		return
	}
	config, err := business.UpdateIssueInviteConfig(model)
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.config.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toIssueInviteConfigResponse(config),
	})
}

// UploadIssueInviteThemeImage stores the invitation theme image (750*360, within 500KB).
func UploadIssueInviteThemeImage(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminID, workspaceID, ok := issueInviteBindParams(c)
	if !ok {
		return
	}
	file, err := c.FormFile(`file`)
	if err != nil || file == nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgImageRequired), `issue_invite.theme_image.bind`, err)
		return
	}
	if file.Size == 0 || file.Size > business.IssueInviteThemeImageMaxSize {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgIssueInviteThemeImageTooLarge), `issue_invite.theme_image.size`, nil)
		return
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(file.Filename)), ".")
	if !business.IsAllowedImageExt(ext) {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgIssueInviteThemeImageTypeInvalid), `issue_invite.theme_image.type`, nil)
		return
	}
	url, _, err := business.UploadImage(adminID, workspaceID, c.GetInt(`userId`), file)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgServerError), `issue_invite.theme_image.upload`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    gin.H{`url`: url},
	})
}

// GetPublicIssueInvite returns a public general-link configuration.
func GetPublicIssueInvite(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	inviteKey := c.Param(`inviteKey`)
	openID := c.Query(`openid`)
	if err := validate.IssueInviteOpenID(openID); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgIssueInviteOpenIDInvalid), `issue_invite.public.openid`, err)
		return
	}
	config, templates, err := business.GetPublicIssueInvite(inviteKey, openID)
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.public.config`, err)
		return
	}
	guideTitle := config.GuideTitle
	guideDesc := config.GuideDescription
	themeImg := config.ThemeImage
	if config.Enabled != 1 {
		guideTitle = ""
		guideDesc = ""
		themeImg = ""
	}
	items := issueInviteTemplateResponseList(lang, templates)
	if scope, scopeErr := business.ResolvePublicIssueInvite(inviteKey); scopeErr != nil {
		logs.Error("issue_invite.public.config resolve scope failed, err:%s", scopeErr.Error())
	} else {
		fillIssueInviteTypeAttrs(items, loadIssueInviteTypeAttrs(`issue_invite.public.config`, scope.AdminID, scope.WorkspaceID))
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: publicIssueInviteResponse{
			GuideTitle:       guideTitle,
			GuideDescription: guideDesc,
			ThemeImage:       themeImg,
			Templates:        items,
		},
	})
}
func GetPublicIssueTemplate(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	inviteKey := c.Param(`inviteKey`)
	templateKey := c.Param(`templateKey`)
	openID := c.Query(`openid`)
	if err := validate.IssueInviteOpenID(openID); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgIssueInviteOpenIDInvalid), `issue_invite.public.template.openid`, err)
		return
	}
	template, err := business.GetPublicIssueTemplate(inviteKey, templateKey, openID, lang)
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.public.template`, err)
		return
	}
	item := toIssueInviteTemplateResponse(lang, template)
	// 模板自己的邀请文案开关关着时，公开填写页不渲染文案与主题图；字段保留在库里，重开即恢复。
	if template.GuideEnabled != 1 {
		item.ThemeImage = ``
		item.GuideTitle = ``
		item.GuideDescription = ``
	}
	if scope, scopeErr := business.ResolvePublicIssueInvite(inviteKey); scopeErr != nil {
		logs.Error("issue_invite.public.template resolve scope failed, err:%s", scopeErr.Error())
	} else {
		fillIssueInviteTypeAttrs([]issueInviteTemplateResponse{item},
			loadIssueInviteTypeAttrs(`issue_invite.public.template`, scope.AdminID, scope.WorkspaceID))
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    item,
	})
}

// SubmitPublicIssue creates one public issue.
func SubmitPublicIssue(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	inviteKey := c.Param(`inviteKey`)
	templateKey := c.Param(`templateKey`)
	var req issueInviteSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `issue_invite.public.submit.bind`, err)
		return
	}
	model := define.IssueInviteSubmitModel{
		InviteKey: inviteKey, TemplateKey: templateKey, OpenID: req.OpenID, Fields: req.Fields, FileIDs: req.FileIDs,
	}
	if err := validate.IssueInviteSubmission(model); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `issue_invite.public.submit.validate`, err)
		return
	}
	detail, err := business.SubmitPublicIssue(model)
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.public.submit`, err)
		return
	}
	respondPublicIssueDetail(c, lang, detail)
}

// ListPublicIssueHistory returns one fixed ten-row submitter history page.
func ListPublicIssueHistory(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	inviteKey := c.Param(`inviteKey`)
	openID := c.Query(`openid`)
	page := cast.ToInt(c.DefaultQuery(`page`, `1`))
	history, err := business.ListPublicIssueHistory(inviteKey, openID, page)
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.public.history`, err)
		return
	}
	items := make([]define.IssueInviteRequirementModel, 0, len(history.Items))
	for _, item := range history.Items {
		// DB stores status/priority names as wrapped i18n keys; translate before exposing.
		item.StatusName = lib_i18n.ShowStored(lang, item.StatusName)
		item.PriorityName = lib_i18n.ShowStored(lang, item.PriorityName)
		items = append(items, item)
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: publicIssueHistoryResponse{
			Items: items, Page: history.Page, PageSize: history.PageSize, Total: history.Total,
		},
	})
}

// GetPublicIssueDetail returns one submitter-owned read-only issue detail.
func GetPublicIssueDetail(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	inviteKey := c.Param(`inviteKey`)
	openID := c.Query(`openid`)
	requirementID, err := positiveParam(c, `requirementId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `issue_invite.public.detail.id`, err)
		return
	}
	detail, err := business.GetPublicIssueDetail(inviteKey, openID, requirementID)
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.public.detail`, err)
		return
	}
	respondPublicIssueDetail(c, lang, detail)
}

// respondPublicIssueDetail 返回一张公开工单详情，状态名与优先级名先按请求语言翻好。
func respondPublicIssueDetail(c *gin.Context, lang string, detail define.IssueInviteRequirementModel) {
	// DB stores status/priority names as wrapped i18n keys; translate before exposing.
	detail.StatusName = lib_i18n.ShowStored(lang, detail.StatusName)
	detail.PriorityName = lib_i18n.ShowStored(lang, detail.PriorityName)
	// 内置字段展示值里的优先级同样是 i18n 键，一起翻掉
	for i := range detail.Fields {
		if detail.Fields[i].FieldKey != `priority` {
			continue
		}
		if name, ok := detail.Fields[i].Value.(string); ok {
			detail.Fields[i].Value = lib_i18n.ShowStored(lang, name)
		}
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    detail,
	})
}

// bindIssueInviteRequirementAction 解析公开撤回/催单/重提的入参。
func bindIssueInviteRequirementAction(c *gin.Context, lang, stage string) (define.IssueInviteRequirementActionModel, bool) {
	requirementID, err := positiveParam(c, `requirementId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage+`.id`, err)
		return define.IssueInviteRequirementActionModel{}, false
	}
	var req issueInviteRequirementActionRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage+`.bind`, err)
		return define.IssueInviteRequirementActionModel{}, false
	}
	model := define.IssueInviteRequirementActionModel{
		InviteKey:     c.Param(`inviteKey`),
		OpenID:        req.OpenID,
		RequirementID: requirementID,
		Fields:        req.Fields,
		FileIDs:       req.FileIDs,
	}
	if err = validate.IssueInviteRequirementAction(model); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage+`.validate`, err)
		return define.IssueInviteRequirementActionModel{}, false
	}
	return model, true
}

// RecallPublicIssue lets the submitter withdraw their own issue.
func RecallPublicIssue(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	model, ok := bindIssueInviteRequirementAction(c, lang, `issue_invite.public.recall`)
	if !ok {
		return
	}
	detail, err := business.RecallPublicIssue(model)
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.public.recall`, err)
		return
	}
	respondPublicIssueDetail(c, lang, detail)
}

// UrgePublicIssue lets the submitter urge the assignees of their own issue.
func UrgePublicIssue(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	model, ok := bindIssueInviteRequirementAction(c, lang, `issue_invite.public.urge`)
	if !ok {
		return
	}
	detail, err := business.UrgePublicIssue(model)
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.public.urge`, err)
		return
	}
	respondPublicIssueDetail(c, lang, detail)
}

// ResubmitPublicIssue lets the submitter edit and resubmit their own issue.
func ResubmitPublicIssue(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	model, ok := bindIssueInviteRequirementAction(c, lang, `issue_invite.public.resubmit`)
	if !ok {
		return
	}
	detail, err := business.ResubmitPublicIssue(model)
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.public.resubmit`, err)
		return
	}
	respondPublicIssueDetail(c, lang, detail)
}

type publicIssueHandleRecordResponse struct {
	Type        string                                 `json:"type"`
	Content     string                                 `json:"content,omitempty"`
	ContentHTML string                                 `json:"content_html,omitempty"`
	ActorName   string                                 `json:"actor_name,omitempty"`
	FromName    string                                 `json:"from_name,omitempty"`
	ToName      string                                 `json:"to_name,omitempty"`
	CreatedAt   int64                                  `json:"created_at"`
	Attachments []publicIssueCommentAttachmentResponse `json:"attachments,omitempty"`
}

type publicIssueCommentAttachmentResponse struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
	Kind string `json:"kind"`
	Size int64  `json:"size"`
}

type publicIssueHandleRecordsResponse struct {
	Records []publicIssueHandleRecordResponse `json:"records"`
}

// GetPublicIssueHandleRecords returns the merged submitter handle-record feed.
func GetPublicIssueHandleRecords(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	inviteKey := c.Param(`inviteKey`)
	openID := c.Query(`openid`)
	requirementID, err := positiveParam(c, `requirementId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `issue_invite.public.records.id`, err)
		return
	}
	records, err := business.GetPublicIssueHandleRecords(business.PublicIssueHandleRecordsModel{InviteKey: inviteKey, OpenID: openID, RequirementId: requirementID})
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.public.records`, err)
		return
	}
	response := publicIssueHandleRecordsResponse{Records: make([]publicIssueHandleRecordResponse, 0, len(records.Records))}
	for _, record := range records.Records {
		attachments := make([]publicIssueCommentAttachmentResponse, 0, len(record.Attachments))
		for _, file := range record.Attachments {
			attachments = append(attachments, publicIssueCommentAttachmentResponse{Id: file.Id, Name: file.Name, URL: file.URL, Kind: file.Kind, Size: file.Size})
		}
		response.Records = append(response.Records, publicIssueHandleRecordResponse{
			Type: record.Type, Content: record.Content, ContentHTML: record.ContentHTML, ActorName: record.ActorName,
			FromName: lib_i18n.ShowStored(lang, record.FromName), ToName: lib_i18n.ShowStored(lang, record.ToName), CreatedAt: record.CreatedAt,
			Attachments: attachments,
		})
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response})
}

// CreatePublicIssueComment posts one submitter comment on the public issue detail page.
func CreatePublicIssueComment(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	inviteKey := c.Param(`inviteKey`)
	requirementID, err := positiveParam(c, `requirementId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `issue_invite.public.comment.id`, err)
		return
	}
	var req issueInviteCommentRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `issue_invite.public.comment.bind`, err)
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == `` || utf8.RuneCountInString(content) > define.IssueInviteCommentMaxLength {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `issue_invite.public.comment.validate`, nil)
		return
	}
	commentID, err := business.CreatePublicIssueComment(inviteKey, req.OpenID, requirementID, content)
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.public.comment`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    createCommentResponse{Id: commentID},
	})
}

// UploadPublicIssueAttachment stores one attachment from the public issue form as a pending
// enterprise file record. The work item is not created yet, so the record keeps source_id = 0
// and uploader_id = 0; the submission flow binds it to the created issue afterwards.
func UploadPublicIssueAttachment(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	inviteKey := c.Param(`inviteKey`)
	openID := c.Query(`openid`)
	if err := validate.IssueInviteOpenID(openID); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgIssueInviteOpenIDInvalid), `issue_invite.public.attachment.openid`, err)
		return
	}
	scope, err := business.ResolvePublicIssueInvite(inviteKey)
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.public.attachment.invite`, err)
		return
	}
	file, err := c.FormFile(`file`)
	if err != nil || file == nil || file.Size == 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgFileRequired), `issue_invite.public.attachment.file`, err)
		return
	}
	if file.Size > business.AttachmentMaxSize() {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgAttachmentTooLarge), `issue_invite.public.attachment.size`, nil)
		return
	}
	item, err := business.CreatePendingAttachment(scope.AdminID, scope.WorkspaceID, 0, file)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `issue_invite.public.attachment.create`, err)
		return
	}
	// The ownership cache is the only proof a submitter can present when deleting the
	// still-unbound attachment; roll the ledger row back when it cannot be written.
	attachmentID := cast.ToInt(item[`id`])
	if err = business.CachePendingIssueAttachment(scope.AdminID, scope.WorkspaceID, openID, attachmentID); err != nil {
		if delErr := business.DeletePendingAttachment(scope.AdminID, scope.WorkspaceID, attachmentID); delErr != nil {
			logs.Error("rollback pending attachment after cache failure failed, err:%s", delErr.Error())
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `issue_invite.public.attachment.cache`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: gin.H{
			`id`:   item[`id`],
			`url`:  item[`url`],
			`name`: file.Filename,
			`size`: file.Size,
		},
	})
}

// DeletePublicIssueAttachment removes one still-unbound attachment uploaded from the public
// issue form; only the uploader (proven via the one-day ownership cache) can delete it, and
// already submitted (bound) attachments cannot be removed through this endpoint.
func DeletePublicIssueAttachment(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	inviteKey := c.Param(`inviteKey`)
	openID := c.Query(`openid`)
	if err := validate.IssueInviteOpenID(openID); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgIssueInviteOpenIDInvalid), `issue_invite.public.attachment.openid`, err)
		return
	}
	attachmentID, err := positiveParam(c, `attachmentId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `issue_invite.public.attachment.id`, err)
		return
	}
	scope, err := business.ResolvePublicIssueInvite(inviteKey)
	if err != nil {
		abortIssueInviteError(c, lang, `issue_invite.public.attachment.invite`, err)
		return
	}
	err = business.DeletePendingIssueAttachment(scope.AdminID, scope.WorkspaceID, openID, attachmentID)
	if err != nil {
		if errors.Is(err, business.ErrAttachmentNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgAttachmentNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `issue_invite.public.attachment.delete`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

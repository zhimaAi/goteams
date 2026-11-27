package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
)

type warningRuleRequest struct {
	Name    string          `json:"name"`
	Enabled bool            `json:"enabled"`
	Config  json.RawMessage `json:"config"`
}
type warningRuleResponse struct {
	Id      int             `json:"id"`
	Name    string          `json:"name"`
	Enabled bool            `json:"enabled"`
	Config  json.RawMessage `json:"config"`
}
type warningListResponse struct {
	List  []warningRuleResponse `json:"list"`
	Total int                   `json:"total"`
	Page  int                   `json:"page"`
	Size  int                   `json:"size"`
}
type warningTemplateResponse struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}
type warningOptionResponse struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}
type warningFieldResponse struct {
	Key     string                  `json:"key"`
	Label   string                  `json:"label"`
	Type    string                  `json:"type"`
	Options []warningOptionResponse `json:"options"`
}
type warningMetadataResponse struct {
	Templates []warningTemplateResponse `json:"templates"`
	Fields    []warningFieldResponse    `json:"fields"`
}
type warningListQuery struct {
	Page    int    `form:"page"`
	Size    int    `form:"size"`
	Name    string `form:"name"`
	Keyword string `form:"keyword"`
}
type warningMetadataQuery struct {
	Type       string `form:"type"`
	TemplateId int    `form:"template_id"`
}

func abortWarning(c *gin.Context, err error) {
	lang := lib_i18n.GetLang(c)
	switch {
	case errors.Is(err, business.ErrWarningInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgWarningInvalid), "warning", err)
	case errors.Is(err, business.ErrWarningNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgWarningNotFound))
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), "warning", err)
	}
}
func warningScope(c *gin.Context) (business.WarningRuleModel, bool) {
	ws, err := positiveParam(c, "wsId")
	if err != nil {
		abortWarning(c, business.ErrWarningInvalid)
		return business.WarningRuleModel{}, false
	}
	m := business.WarningRuleModel{AdminId: c.GetInt("adminId"), WorkspaceId: ws}
	if c.Param("ruleId") != "" {
		m.Id, err = positiveParam(c, "ruleId")
		if err != nil {
			abortWarning(c, business.ErrWarningInvalid)
			return m, false
		}
	}
	return m, true
}
func warningResponse(m business.WarningRuleModel) warningRuleResponse {
	return warningRuleResponse{Id: m.Id, Name: m.Name, Enabled: m.Enabled, Config: m.Config}
}

// ListWarningRules 返回当前工作空间的预警规则。
func ListWarningRules(c *gin.Context) {
	scope, ok := warningScope(c)
	if !ok {
		return
	}
	q := warningListQuery{Page: 1, Size: 20}
	if c.ShouldBindQuery(&q) != nil {
		abortWarning(c, business.ErrWarningInvalid)
		return
	}
	m, err := business.ListWarningRules(business.WarningListModel{AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, Page: q.Page, Size: q.Size, Name: q.Name, Keyword: q.Keyword})
	if err != nil {
		abortWarning(c, err)
		return
	}
	items := []warningRuleResponse{}
	for _, item := range m.Items {
		items = append(items, warningResponse(item))
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Data: warningListResponse{List: items, Total: m.Total, Page: m.Page, Size: m.Size}})
}

// SaveWarningRule 创建或更新预警规则。
func SaveWarningRule(c *gin.Context) {
	m, ok := warningScope(c)
	if !ok {
		return
	}
	var req warningRuleRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<17)
	if err := c.ShouldBindJSON(&req); err != nil {
		// 不展开解析错误中的请求值，避免泄露 Webhook 密钥或正文。
		detail := fmt.Sprintf("request JSON binding failed (%T)", err)
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &syntaxErr) {
			detail += fmt.Sprintf(" at byte %d", syntaxErr.Offset)
		} else if errors.As(err, &typeErr) {
			detail += fmt.Sprintf(" at byte %d", typeErr.Offset)
		}
		abortWarning(c, fmt.Errorf("%w: %s", business.ErrWarningInvalid, detail))
		return
	}
	m.Name, m.Enabled, m.Config = req.Name, req.Enabled, req.Config
	m, err := business.SaveWarningRule(m)
	if err != nil {
		abortWarning(c, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Data: warningResponse(m)})
}

// DeleteWarningRule 删除预警规则。
func DeleteWarningRule(c *gin.Context) {
	m, ok := warningScope(c)
	if !ok {
		return
	}
	if err := business.DeleteWarningRule(m); err != nil {
		abortWarning(c, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess})
}

// GetWarningMetadata 返回模块模板和指定模板字段。
func GetWarningMetadata(c *gin.Context) {
	scope, ok := warningScope(c)
	if !ok {
		return
	}
	q := warningMetadataQuery{}
	if c.ShouldBindQuery(&q) != nil {
		abortWarning(c, business.ErrWarningInvalid)
		return
	}
	m, err := business.WarningMetadata(business.WarningMetadataModel{AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, Type: q.Type, TemplateId: q.TemplateId, Lang: lib_i18n.GetLang(c)})
	if err != nil {
		abortWarning(c, err)
		return
	}
	result := warningMetadataResponse{Templates: []warningTemplateResponse{}, Fields: []warningFieldResponse{}}
	for _, t := range m.Templates {
		result.Templates = append(result.Templates, warningTemplateResponse{Id: t.Id, Name: t.Name})
	}
	for _, f := range m.Fields {
		field := warningFieldResponse{Key: f.Key, Label: f.Label, Type: f.Type, Options: []warningOptionResponse{}}
		for _, o := range f.Options {
			field.Options = append(field.Options, warningOptionResponse{Key: o.Key, Name: o.Name})
		}
		result.Fields = append(result.Fields, field)
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Data: result})
}

type warningEnabledRequest struct {
	Enabled *bool `json:"enabled"`
}

// SetWarningRuleEnabled 切换预警规则启停。
func SetWarningRuleEnabled(c *gin.Context) {
	m, ok := warningScope(c)
	if !ok {
		return
	}
	var request warningEnabledRequest
	if c.ShouldBindJSON(&request) != nil || request.Enabled == nil {
		abortWarning(c, business.ErrWarningInvalid)
		return
	}
	m.Enabled = *request.Enabled
	if err := business.SetWarningRuleEnabled(m); err != nil {
		abortWarning(c, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess})
}

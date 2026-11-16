package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
)

type severityResponse struct {
	Id        int    `json:"id"`
	Label     string `json:"label"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
	IsDefault int    `json:"is_default"`
	IsBuiltin bool   `json:"is_builtin"`
}

type saveSeverityRequest struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder *int   `json:"sort_order"`
}

type setDefaultSeverityRequest struct {
	Id *int `json:"severity_id"`
}

func severityResponses(items []business.SeverityModel) []severityResponse {
	out := make([]severityResponse, 0, len(items))
	for _, item := range items {
		entry := severityResponse{Id: item.Id, Label: item.Label, Color: item.Color, SortOrder: item.SortOrder, IsBuiltin: item.IsBuiltin}
		if item.IsDefault {
			entry.IsDefault = 1
		}
		out = append(out, entry)
	}
	return out
}

func severitySettingsModel(c *gin.Context) business.SeveritySettingsModel {
	wsId, _ := strconv.Atoi(c.Param("wsId"))
	return business.SeveritySettingsModel{AdminId: c.GetInt("adminId"), WorkspaceId: wsId, Lang: lib_i18n.GetLang(c)}
}

// ListSeverities 返回工作空间的缺陷严重等级。
func ListSeverities(c *gin.Context) {
	model, err := business.ListSeverities(severitySettingsModel(c))
	if err != nil {
		severitySettingsError(c, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(model.Lang, define.MsgSuccess), Data: severityResponses(model.Items)})
}

// SaveSeverity 创建或编辑工作空间严重等级。
func SaveSeverity(c *gin.Context) {
	model := severitySettingsModel(c)
	if c.Request.Method == http.MethodPut && !readSeverityPathId(c, &model) {
		return
	}
	var req saveSeverityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		severitySettingsError(c, business.ErrSeverityInvalid)
		return
	}
	model.Name = strings.TrimSpace(req.Name)
	model.Color = strings.TrimSpace(req.Color)
	model.SortOrder = req.SortOrder
	if model.Name == "" || utf8.RuneCountInString(model.Name) > 100 ||
		utf8.RuneCountInString(model.Color) > 20 || (req.SortOrder != nil && (*req.SortOrder < 0 || *req.SortOrder > 2147483647)) {
		severitySettingsError(c, business.ErrSeverityInvalid)
		return
	}
	finishSeveritySettings(c, business.SaveSeverity(model))
}

// DeleteSeverity 删除自定义严重等级。
func DeleteSeverity(c *gin.Context) {
	model := severitySettingsModel(c)
	if readSeverityPathId(c, &model) {
		finishSeveritySettings(c, business.DeleteSeverity(model))
	}
}

// SetDefaultSeverity 更新新建缺陷的默认严重等级。
func SetDefaultSeverity(c *gin.Context) {
	var req setDefaultSeverityRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Id == nil || *req.Id < 0 || *req.Id > 2147483647 {
		severitySettingsError(c, business.ErrSeverityInvalid)
		return
	}
	model := severitySettingsModel(c)
	model.Id = *req.Id
	finishSeveritySettings(c, business.SetDefaultSeverity(model))
}

// ResetSeverities 恢复工作空间的预置严重等级。
func ResetSeverities(c *gin.Context) {
	finishSeveritySettings(c, business.ResetSeverities(severitySettingsModel(c)))
}

func readSeverityPathId(c *gin.Context, model *business.SeveritySettingsModel) bool {
	id, err := strconv.ParseInt(c.Param("severityId"), 10, 32)
	if err != nil || id <= 0 {
		severitySettingsError(c, business.ErrSeverityInvalid)
		return false
	}
	model.Id = int(id)
	return true
}

func finishSeveritySettings(c *gin.Context, err error) {
	if err != nil {
		severitySettingsError(c, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lib_i18n.GetLang(c), define.MsgSuccess)})
}

func severitySettingsError(c *gin.Context, err error) {
	lang := lib_i18n.GetLang(c)
	switch {
	case errors.Is(err, business.ErrBuiltinWorkflowItem):
		abortConflict(c, lib_i18n.Show(lang, define.MsgBuiltinWorkflowItemDelete))
	case errors.Is(err, business.ErrSeverityNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgSeverityNotFound))
	case errors.Is(err, business.ErrSeverityInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "severity.input", err)
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), "severity.settings", err)
	}
}

func parseSeveritySelection(raw []byte) (*int, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	value := 0
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	if value < 0 || value > 2147483647 {
		return nil, business.ErrSeverityInvalid
	}
	return &value, nil
}

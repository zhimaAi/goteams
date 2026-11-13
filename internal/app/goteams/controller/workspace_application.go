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

// workspaceApplicationItemResponse is one app card in the app market.
type workspaceApplicationItemResponse struct {
	AppKey      string `json:"app_key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     int    `json:"enabled"`
	ComingSoon  bool   `json:"coming_soon"`
	EnabledBy   int    `json:"enabled_by"`
	EnabledAt   int64  `json:"enabled_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type updateWorkspaceApplicationRequest struct {
	Enabled bool `json:"enabled"`
}

// GetWorkspaceApplications returns the app market cards with current enabled state.
func GetWorkspaceApplications(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId := cast.ToInt(c.GetInt(`adminId`))

	configs, err := business.ListWorkspaceApplications(adminId, cast.ToInt(wsId))
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workspace_application.list`, err)
		return
	}
	enabledByApp := make(map[string]business.WorkspaceApplicationModel, len(configs))
	for _, config := range configs {
		enabledByApp[config.AppKey] = config
	}

	issueWorkspace, err := business.IsIssueWorkspace(adminId, cast.ToInt(wsId))
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workspace_application.type`, err)
		return
	}
	definitions := business.GetWorkspaceAppDefinitions(issueWorkspace)
	items := make([]workspaceApplicationItemResponse, 0, len(definitions))
	for _, def := range definitions {
		config := enabledByApp[def.AppKey]
		items = append(items, workspaceApplicationItemResponse{
			AppKey:      def.AppKey,
			Name:        lib_i18n.Show(lang, def.Name),
			Description: lib_i18n.Show(lang, def.Description),
			Enabled:     config.Enabled,
			ComingSoon:  def.ComingSoon,
			EnabledBy:   config.EnabledBy,
			EnabledAt:   config.EnabledAt,
			UpdatedAt:   config.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    items,
	})
}

// UpdateWorkspaceApplication enables or disables a workspace application (upsert).
func UpdateWorkspaceApplication(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	appKey := c.Param(`appKey`)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	userId := cast.ToInt(c.GetInt(`userId`))

	var req updateWorkspaceApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workspace_application.bind`, err)
		return
	}
	if !business.IsWorkspaceAppKeyRegistered(appKey) {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgWorkspaceApplicationNotFound))
		return
	}
	if appKey == define.AppKeyDefect || appKey == define.AppKeyIteration {
		issueWorkspace, err := business.IsIssueWorkspace(adminId, cast.ToInt(wsId))
		if err != nil {
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workspace_application.type`, err)
			return
		}
		if issueWorkspace {
			abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
			return
		}
	}
	if business.IsWorkspaceAppComingSoon(appKey) {
		// Coming-soon apps cannot be toggled yet; the UI switch is disabled, this is a backend guard.
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgWorkspaceApplicationComingSoon), `workspace_application.coming_soon`, nil)
		return
	}
	enabled := 0
	if req.Enabled {
		enabled = 1
	}
	config, err := business.UpdateWorkspaceApplication(business.WorkspaceApplicationModel{
		AdminId: adminId, WorkspaceId: cast.ToInt(wsId), AppKey: appKey, Enabled: enabled, EnabledBy: userId,
	})
	if err != nil {
		if errors.Is(err, business.ErrWorkspaceNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgWorkspaceNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workspace_application.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: workspaceApplicationItemResponse{
			AppKey: config.AppKey, Enabled: config.Enabled, EnabledBy: config.EnabledBy,
			EnabledAt: config.EnabledAt, UpdatedAt: config.UpdatedAt,
		},
	})
}

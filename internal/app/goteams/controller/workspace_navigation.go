package controller

import (
	"errors"
	"github.com/gin-gonic/gin"
	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
	"net/http"
	"strconv"
)

type navigationSettingsResponse struct {
	MainNav           []string `json:"main_nav"`
	Style             string   `json:"style"`
	PreventMemberEdit bool     `json:"prevent_member_edit"`
	IsAdmin           bool     `json:"is_admin"`
}
type navigationSettingsRequest struct {
	MainNav           []string `json:"main_nav"`
	Style             string   `json:"style"`
	PreventMemberEdit bool     `json:"prevent_member_edit"`
	ApplyToAllMembers bool     `json:"apply_to_all_members"`
}

// GetWorkspaceNavigationSettings returns the effective navigation settings for the current member.
func GetWorkspaceNavigationSettings(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	ws := c.Param("wsId")
	id, err := strconv.Atoi(ws)
	if err != nil || id <= 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "navigation.get.workspace", err)
		return
	}
	s, err := business.GetWorkspaceNavigationSettings(c.GetInt("adminId"), c.GetInt("userId"), id)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), "navigation.get", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: 200, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: navigationSettingsResponse{MainNav: s.MainNavOrder, Style: s.NavStyle, PreventMemberEdit: s.PreventMemberEdit, IsAdmin: s.IsAdmin}})
}

// SaveMyWorkspaceNavigationSettings saves the current member's navigation override.
func SaveMyWorkspaceNavigationSettings(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req navigationSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "navigation.me.bind", err)
		return
	}
	ws, err := strconv.Atoi(c.Param("wsId"))
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "navigation.me.workspace", err)
		return
	}
	if err = business.SaveMemberNavigationSettings(c.GetInt("adminId"), c.GetInt("userId"), ws, req.MainNav, req.Style); err != nil {
		if errors.Is(err, business.ErrNavigationLocked) {
			abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		} else {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "navigation.me.save", err)
		}
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: 200, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// SaveWorkspaceNavigationSettings saves the project navigation policy.
func SaveWorkspaceNavigationSettings(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req navigationSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "navigation.save.bind", err)
		return
	}
	ws, err := strconv.Atoi(c.Param("wsId"))
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "navigation.save.workspace", err)
		return
	}
	if err = business.SaveWorkspaceNavigationSettings(c.GetInt("adminId"), c.GetInt("userId"), ws, req.MainNav, req.Style, req.PreventMemberEdit, req.ApplyToAllMembers); err != nil {
		if errors.Is(err, business.ErrNavigationLocked) {
			abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		} else {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "navigation.save", err)
		}
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: 200, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

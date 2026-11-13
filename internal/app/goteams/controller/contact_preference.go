// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"net/http"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
)

type contactPreferencesResponse struct {
	FavoriteUserIds []int `json:"favorite_user_ids"`
	RecentUserIds   []int `json:"recent_user_ids"`
}

// GetContactPreferences returns the current user's contact preferences.
func GetContactPreferences(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	preferences, err := business.ListContactPreferences(c.GetInt(`adminId`), c.GetInt(`userId`))
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `contact_preferences.list`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: contactPreferencesResponse{
		FavoriteUserIds: preferences.FavoriteUserIds,
		RecentUserIds:   preferences.RecentUserIds,
	}})
}

// UpdateFavoriteContact updates one favorite contact for the current user.
func UpdateFavoriteContact(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	contactUserId, err := positiveParam(c, `contactUserId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `contact_preferences.favorite.param`, err)
		return
	}
	var req struct {
		IsFavorite *bool `json:"is_favorite" binding:"required"`
	}
	if err = c.ShouldBindJSON(&req); err != nil || req.IsFavorite == nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `contact_preferences.favorite.bind`, err)
		return
	}
	if err = business.UpdateFavoriteContact(c.GetInt(`adminId`), c.GetInt(`userId`), contactUserId, *req.IsFavorite); err != nil {
		switch {
		case errors.Is(err, business.ErrContactFavoriteLimit):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgContactFavoriteLimit), `contact_preferences.favorite.limit`, err)
		case errors.Is(err, business.ErrContactInvalid):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgContactInvalid), `contact_preferences.favorite.contact`, err)
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `contact_preferences.favorite.update`, err)
		}
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// RecordRecentContacts records recently selected assignees for the current user.
func RecordRecentContacts(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req struct {
		UserIds []int `json:"user_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `contact_preferences.recent.bind`, err)
		return
	}
	if err := business.RecordRecentContacts(c.GetInt(`adminId`), c.GetInt(`userId`), req.UserIds); err != nil {
		if errors.Is(err, business.ErrContactInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgContactInvalid), `contact_preferences.recent.contact`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `contact_preferences.recent.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

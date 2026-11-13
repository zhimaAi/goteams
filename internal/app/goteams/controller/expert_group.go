// Copyright © 2026 Wuhan Sesame Small Customer Service Network Co., Ltd.

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

// ======================== Expert group CRUD ========================

// ListExpertGroups returns the expert group list.
func ListExpertGroups(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))

	keyword := c.Query(`keyword`)
	statusStr := c.Query(`status`)
	page := cast.ToInt(c.Query(`page`))
	size := cast.ToInt(c.Query(`size`))

	list, total, err := business.ListExpertGroups(adminId, userId, keyword, statusStr, page, size)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `expert_group.list`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: define.ExpertGroupListResp{
			List:  list,
			Total: total,
			Page:  page,
			Size:  size,
		},
	})
}

// GetExpertGroup returns expert group details.
func GetExpertGroup(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	groupId, err := positiveParam(c, `expertGroupId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `expert_group.detail.param`, err)
		return
	}

	detail, err := business.GetExpertGroup(adminId, groupId)
	if err != nil {
		if errors.Is(err, business.ErrExpertGroupNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgExpertGroupNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `expert_group.detail`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    detail,
	})
}

// CreateExpertGroup creates an expert group.
func CreateExpertGroup(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))

	var req define.CreateExpertGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `expert_group.create.bind`, err)
		return
	}
	if err := validate.ValidateExpertGroupCreate(req.Name, len(req.Agents)); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, err.Error()), `expert_group.create.validate`, err)
		return
	}

	groupId, err := business.CreateExpertGroup(adminId, userId, req)
	if err != nil {
		switch {
		case errors.Is(err, business.ErrExpertGroupManageForbidden):
			abortForbidden(c, lib_i18n.Show(lang, define.MsgExpertGroupManageForbidden))
		case errors.Is(err, business.ErrExpertGroupScopeInvalid),
			errors.Is(err, business.ErrExpertGroupMemberInvalid),
			errors.Is(err, business.ErrExpertGroupNoLeader),
			errors.Is(err, business.ErrExpertGroupNoAgent):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `expert_group.create.scope`, err)
		case errors.Is(err, business.ErrExpertGroupAgentPromptRequired):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgExpertGroupAgentPromptRequired), `expert_group.create.prompt`, err)
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `expert_group.create`, err)
		}
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    map[string]interface{}{`id`: groupId},
	})
}

// UpdateExpertGroup updates an expert group.
func UpdateExpertGroup(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))
	groupId, err := positiveParam(c, `expertGroupId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `expert_group.update.param`, err)
		return
	}

	var req define.UpdateExpertGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `expert_group.update.bind`, err)
		return
	}
	if err := validate.ValidateExpertGroupUpdate(req.Name, len(req.Agents)); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, err.Error()), `expert_group.update.validate`, err)
		return
	}

	if err := business.UpdateExpertGroup(adminId, userId, groupId, req); err != nil {
		switch {
		case errors.Is(err, business.ErrExpertGroupNotFound):
			abortNotFound(c, lib_i18n.Show(lang, define.MsgExpertGroupNotFound))
		case errors.Is(err, business.ErrExpertGroupManageForbidden):
			abortForbidden(c, lib_i18n.Show(lang, define.MsgExpertGroupManageForbidden))
		case errors.Is(err, business.ErrExpertGroupScopeInvalid),
			errors.Is(err, business.ErrExpertGroupMemberInvalid),
			errors.Is(err, business.ErrExpertGroupNoLeader),
			errors.Is(err, business.ErrExpertGroupNoAgent):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `expert_group.update.scope`, err)
		case errors.Is(err, business.ErrExpertGroupAgentPromptRequired):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgExpertGroupAgentPromptRequired), `expert_group.update.prompt`, err)
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `expert_group.update`, err)
		}
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// DeleteExpertGroup deletes an expert group.
func DeleteExpertGroup(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))
	groupId, err := positiveParam(c, `expertGroupId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `expert_group.delete.param`, err)
		return
	}

	if err := business.DeleteExpertGroup(adminId, userId, groupId); err != nil {
		if errors.Is(err, business.ErrExpertGroupManageForbidden) {
			abortForbidden(c, lib_i18n.Show(lang, define.MsgExpertGroupManageForbidden))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `expert_group.delete`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// GetClientExpertGroups returns the expert group snapshots available to the current client user.
func GetClientExpertGroups(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))

	list, err := business.GetClientExpertGroups(adminId, userId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `client.expert_groups`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    list,
	})
}

// UploadExpertGroupAvatar saves the expert group avatar and returns the accessible path.
// Mirrors UploadPipelineIcon so the management page handles both consistently.
func UploadExpertGroupAvatar(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	userId := cast.ToInt(c.GetInt(`userId`))
	groupId, err := positiveParam(c, `expertGroupId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `expert_group.avatar.param`, err)
		return
	}
	file, err := c.FormFile(`file`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgExpertGroupAvatarInvalid), `expert_group.avatar.form_file`, err)
		return
	}
	avatar, err := business.SaveExpertGroupAvatar(adminId, userId, groupId, file)
	if err != nil {
		switch {
		case errors.Is(err, business.ErrExpertGroupManageForbidden):
			abortForbidden(c, lib_i18n.Show(lang, define.MsgExpertGroupManageForbidden))
		case errors.Is(err, business.ErrExpertGroupNotFound):
			abortNotFound(c, lib_i18n.Show(lang, define.MsgExpertGroupNotFound))
		case errors.Is(err, business.ErrExpertGroupAvatarInvalid):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgExpertGroupAvatarInvalid), `expert_group.avatar.invalid`, err)
		case errors.Is(err, business.ErrExpertGroupAvatarTooLarge):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgExpertGroupAvatarTooLarge), `expert_group.avatar.too_large`, err)
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `expert_group.avatar.save`, err)
		}
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    map[string]string{`avatar`: avatar},
	})
}

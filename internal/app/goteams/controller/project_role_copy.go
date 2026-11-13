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

type copyProjectRoleRequest struct {
	WorkspaceIds []int `json:"workspaceIds" binding:"required,min=1,dive,gt=0"`
}

type listProjectRoleCopyTargetsResponseItem struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

// ListProjectRoleCopyTargets lists workspaces eligible for receiving a copied project role.
func ListProjectRoleCopyTargets(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	sourceWorkspaceId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `project.role.copy_targets.workspace_id`, err)
		return
	}

	model, err := business.ListProjectRoleCopyTargets(business.ListProjectRoleCopyTargetsModel{
		AdminId:           c.GetInt(`adminId`),
		UserId:            c.GetInt(`userId`),
		SourceWorkspaceId: sourceWorkspaceId,
	})
	if err != nil {
		if errors.Is(err, business.ErrProjectRoleCopyTargetInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `project.role.copy_targets.validate`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `project.role.copy_targets.list`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toListProjectRoleCopyTargetsResponse(model),
	})
}

// CopyProjectRole copies a custom project role to selected workspaces.
func CopyProjectRole(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	sourceWorkspaceId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `project.role.copy.workspace_id`, err)
		return
	}
	roleId, err := positiveParam(c, `roleId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `project.role.copy.role_id`, err)
		return
	}

	var req copyProjectRoleRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `project.role.copy.bind`, err)
		return
	}

	err = business.CopyProjectRole(business.CopyProjectRoleModel{
		AdminId:            c.GetInt(`adminId`),
		UserId:             c.GetInt(`userId`),
		SourceWorkspaceId:  sourceWorkspaceId,
		RoleId:             roleId,
		TargetWorkspaceIds: req.WorkspaceIds,
	})
	if err != nil {
		switch {
		case errors.Is(err, business.ErrProjectRoleCopyTargetForbidden):
			respondLoggedError(c, http.StatusForbidden, http.StatusForbidden, lib_i18n.Show(lang, define.MsgForbidden), `project.role.copy.target_permission`, err)
		case errors.Is(err, business.ErrProjectRoleNameExists):
			respondLoggedError(c, http.StatusConflict, http.StatusConflict, lib_i18n.Show(lang, define.MsgProjectRoleNameExist), `project.role.copy.name_exists`, err)
		case errors.Is(err, business.ErrProjectRoleCopyTargetInvalid),
			errors.Is(err, business.ErrProjectRoleNotFound),
			errors.Is(err, business.ErrProjectRoleSystem):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `project.role.copy.validate`, err)
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `project.role.copy.insert`, err)
		}
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

func toListProjectRoleCopyTargetsResponse(model business.ListProjectRoleCopyTargetsModel) []listProjectRoleCopyTargetsResponseItem {
	items := make([]listProjectRoleCopyTargetsResponseItem, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, listProjectRoleCopyTargetsResponseItem{Id: item.Id, Name: item.Name})
	}
	return items
}

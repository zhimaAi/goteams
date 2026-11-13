package controller

import (
	"errors"
	"net/http"
	"strings"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
)

type workspaceTypeResponse struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type listWorkspaceTypesResponse struct {
	List      []workspaceTypeResponse `json:"list"`
	CanManage bool                    `json:"can_manage"`
}

type createWorkspaceTypeRequest struct {
	Name string `json:"name" binding:"required"`
}

func toWorkspaceTypeResponse(item business.WorkspaceTypeModel, lang string) workspaceTypeResponse {
	name := item.Name
	switch item.Key {
	case define.WorkspaceTypeSoftware, define.WorkspaceTypeAgile, define.WorkspaceTypeIssue:
		name = lib_i18n.Show(lang, "workspace_type_"+item.Key)
	}
	return workspaceTypeResponse{Key: item.Key, Name: name}
}

// ListWorkspaceTypes 返回创建表单所需的类型字典及维护能力。
func ListWorkspaceTypes(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	model, err := business.ListWorkspaceTypes(business.ListWorkspaceTypesModel{AdminId: c.GetInt("adminId"), UserId: c.GetInt("userId")})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), "workspace.types.list", err)
		return
	}
	items := make([]workspaceTypeResponse, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, toWorkspaceTypeResponse(item, lang))
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: listWorkspaceTypesResponse{List: items, CanManage: model.CanManage}})
}

// CreateWorkspaceType 由模板管理员在创建项目时添加类型。
func CreateWorkspaceType(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	if !business.CanManageWorkspaceTypes(c.GetInt("adminId"), c.GetInt("userId")) {
		c.AbortWithStatusJSON(http.StatusForbidden, define.Response{Code: http.StatusForbidden, Message: lib_i18n.Show(lang, define.MsgForbidden)})
		return
	}
	var req createWorkspaceTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "workspace.types.bind", err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if err := validate.ValidateWorkspaceTypeName(req.Name); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), "workspace.types.validate", err)
		return
	}
	item, err := business.CreateWorkspaceType(business.CreateWorkspaceTypeModel{AdminId: c.GetInt("adminId"), UserId: c.GetInt("userId"), Name: req.Name})
	if err != nil {
		if errors.Is(err, business.ErrWorkspaceTypeExists) {
			c.JSON(http.StatusConflict, define.Response{Code: http.StatusConflict, Message: lib_i18n.Show(lang, define.MsgWorkspaceTypeExists)})
			return
		}
		if errors.Is(err, business.ErrWorkspaceTypeForbidden) {
			c.AbortWithStatusJSON(http.StatusForbidden, define.Response{Code: http.StatusForbidden, Message: lib_i18n.Show(lang, define.MsgForbidden)})
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), "workspace.types.create", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: toWorkspaceTypeResponse(item, lang)})
}

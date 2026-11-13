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

// ======================== Enterprise Department Management ========================

type departmentResponseItem struct {
	Id       string `json:"id"`
	ParentId string `json:"parent_id"`
	Name     string `json:"name"`
}

type createDepartmentRequest struct {
	ParentId int    `json:"parentId"`
	Name     string `json:"name" binding:"required"`
}

type renameDepartmentRequest struct {
	Name string `json:"name" binding:"required"`
}

type listDepartmentsResponse struct {
	EnterpriseName string                   `json:"enterprise_name"`
	Items          []departmentResponseItem `json:"items"`
}

// ListDepartments returns the enterprise name (tree root) and all departments; the frontend assembles the tree.
func ListDepartments(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := c.GetInt(`adminId`)
	items, err := business.ListDepartments(adminId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.department.list`, err)
		return
	}
	enterpriseName, err := business.GetEnterpriseName(adminId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.department.list.enterprise_name`, err)
		return
	}
	result := listDepartmentsResponse{
		EnterpriseName: enterpriseName,
		Items:          make([]departmentResponseItem, 0, len(items)),
	}
	for _, item := range items {
		result.Items = append(result.Items, departmentResponseItem{
			Id:       cast.ToString(item.Id),
			ParentId: cast.ToString(item.ParentId),
			Name:     item.Name,
		})
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    result,
	})
}

// CreateDepartment creates a department (parentId 0 = top level).
func CreateDepartment(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req createDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.department.create.bind`, err)
		return
	}
	id, err := business.CreateDepartment(c.GetInt(`adminId`), req.ParentId, req.Name)
	if err != nil {
		abortDepartmentError(c, lang, `enterprise.department.create`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    map[string]any{`id`: cast.ToString(id)},
	})
}

// RenameDepartment renames a department.
func RenameDepartment(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req renameDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `enterprise.department.rename.bind`, err)
		return
	}
	err := business.RenameDepartment(c.GetInt(`adminId`), cast.ToInt(c.Param(`departmentId`)), req.Name)
	if err != nil {
		abortDepartmentError(c, lang, `enterprise.department.rename`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// DeleteDepartment cascade-deletes a department subtree; members are only detached, never removed.
func DeleteDepartment(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	err := business.DeleteDepartment(c.GetInt(`adminId`), cast.ToInt(c.Param(`departmentId`)))
	if err != nil {
		abortDepartmentError(c, lang, `enterprise.department.delete`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

func abortDepartmentError(c *gin.Context, lang, scene string, err error) {
	switch {
	case errors.Is(err, business.ErrDepartmentNotFound):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgDepartmentNotFound), scene+`.not_found`, err)
	case errors.Is(err, business.ErrDepartmentNameExists):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgDepartmentNameExists), scene+`.name_exists`, err)
	case errors.Is(err, business.ErrDepartmentDepthExceeded):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgDepartmentDepthExceeded), scene+`.depth_exceeded`, err)
	case errors.Is(err, business.ErrDepartmentInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgDepartmentInvalid), scene+`.invalid`, err)
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), scene, err)
	}
}

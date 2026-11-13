// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

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

type requirementCategoryResponseItem struct {
	Id        int    `json:"id"`
	ParentId  int    `json:"parent_id"`
	Name      string `json:"name"`
	SortOrder int64  `json:"sort_order"`
	Count     int    `json:"count"`
}

type listRequirementCategoriesResponse struct {
	List               []requirementCategoryResponseItem `json:"list"`
	AllCount           int                               `json:"all_count"`
	UncategorizedCount int                               `json:"uncategorized_count"`
}

type saveRequirementCategoryRequest struct {
	Name string `json:"name"`
}

type createRequirementCategoryRequest struct {
	Name     string `json:"name"`
	ParentId int    `json:"parent_id"`
}

type createRequirementCategoryResponse struct {
	Id int `json:"id"`
}

type moveRequirementCategoryRequest struct {
	ParentId int `json:"parent_id"`
	PrevId   int `json:"prev_id"`
	NextId   int `json:"next_id"`
}

type assignRequirementCategoryRequest struct {
	CategoryId *int `json:"category_id"`
}

// AssignRequirementCategory 更新顶层需求所属需求树的分类。
func AssignRequirementCategory(c *gin.Context) {
	const stage = "requirement_category.assign"
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, "wsId")
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage, err)
		return
	}
	reqId, err := positiveParam(c, "reqId")
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage, err)
		return
	}
	var req assignRequirementCategoryRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage, err)
		return
	}
	if req.CategoryId == nil || *req.CategoryId < 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage, errors.New("category_id must be a non-negative integer"))
		return
	}
	err = business.AssignRequirementCategory(business.AssignRequirementCategoryModel{
		AdminId: c.GetInt("adminId"), WorkspaceId: wsId, RequirementId: reqId,
		UserId: c.GetInt("userId"), CategoryId: *req.CategoryId,
	})
	switch {
	case err == nil:
		c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
	case errors.Is(err, business.ErrRequirementNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
	case errors.Is(err, business.ErrRequirementCategoryAssignmentForbidden):
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
	case errors.Is(err, business.ErrRequirementCategoryAssignmentInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgRequirementCategoryAssignmentInvalid), stage, err)
	default:
		if !abortRequirementCategoryError(c, lang, stage, err) {
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), stage, err)
		}
	}
}

func abortRequirementCategoryError(c *gin.Context, lang, scene string, err error) bool {
	switch {
	case errors.Is(err, business.ErrRequirementCategoryNotFound):
		c.JSON(http.StatusNotFound, define.Response{
			Code: http.StatusNotFound, Message: lib_i18n.Show(lang, define.MsgRequirementCategoryNotFound),
		})
	case errors.Is(err, business.ErrRequirementCategoryNameExists):
		c.JSON(http.StatusConflict, define.Response{
			Code: http.StatusConflict, Message: lib_i18n.Show(lang, define.MsgRequirementCategoryNameExists),
		})
	case errors.Is(err, business.ErrRequirementCategoryTreeInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgRequirementCategoryTreeInvalid), scene, err)
	default:
		return false
	}
	return true
}

// ListRequirementCategories returns the shared category tree and current user's visible counts.
func ListRequirementCategories(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_category.list.workspace_id`, err)
		return
	}
	model, err := business.ListRequirementCategories(business.ListRequirementCategoriesModel{
		AdminId: c.GetInt(`adminId`), UserId: c.GetInt(`userId`), WorkspaceId: wsId,
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement_category.list`, err)
		return
	}
	items := make([]requirementCategoryResponseItem, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, requirementCategoryResponseItem{
			Id: item.Id, ParentId: item.ParentId, Name: item.Name,
			SortOrder: item.SortOrder, Count: item.Count,
		})
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: listRequirementCategoriesResponse{
			List: items, AllCount: model.AllCount, UncategorizedCount: model.UncategorizedCount,
		},
	})
}

// CreateRequirementCategory creates a category under the requested parent.
func CreateRequirementCategory(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_category.create.workspace_id`, err)
		return
	}
	var req createRequirementCategoryRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_category.create.bind`, err)
		return
	}
	if req.ParentId < 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_category.create.validate`, errors.New(`parent_id must not be negative`))
		return
	}
	if err = validate.ValidateRequirementCategoryName(req.Name); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_category.create.validate`, err)
		return
	}
	model, err := business.CreateRequirementCategory(business.CreateRequirementCategoryModel{
		AdminId: c.GetInt(`adminId`), WorkspaceId: wsId, ParentId: req.ParentId, Name: strings.TrimSpace(req.Name),
	})
	if err != nil {
		if abortRequirementCategoryError(c, lang, `requirement_category.create`, err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement_category.create`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: createRequirementCategoryResponse{Id: model.Item.Id},
	})
}

// UpdateRequirementCategory renames a category.
func UpdateRequirementCategory(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_category.update.workspace_id`, err)
		return
	}
	categoryId, err := positiveParam(c, `categoryId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_category.update.category_id`, err)
		return
	}
	var req saveRequirementCategoryRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_category.update.bind`, err)
		return
	}
	if err = validate.ValidateRequirementCategoryName(req.Name); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_category.update.validate`, err)
		return
	}
	if err = business.UpdateRequirementCategory(business.UpdateRequirementCategoryModel{
		AdminId: c.GetInt(`adminId`), WorkspaceId: wsId, CategoryId: categoryId, Name: strings.TrimSpace(req.Name),
	}); err != nil {
		if abortRequirementCategoryError(c, lang, `requirement_category.update`, err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement_category.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// DeleteRequirementCategory deletes a category without deleting requirements.
func DeleteRequirementCategory(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_category.delete.workspace_id`, err)
		return
	}
	categoryId, err := positiveParam(c, `categoryId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_category.delete.category_id`, err)
		return
	}
	if err = business.DeleteRequirementCategory(business.DeleteRequirementCategoryModel{
		AdminId: c.GetInt(`adminId`), WorkspaceId: wsId, CategoryId: categoryId,
	}); err != nil {
		if abortRequirementCategoryError(c, lang, `requirement_category.delete`, err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement_category.delete`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// MoveRequirementCategory changes one category's parent and sibling position.
func MoveRequirementCategory(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_category.move.workspace_id`, err)
		return
	}
	categoryId, err := positiveParam(c, `categoryId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_category.move.category_id`, err)
		return
	}
	var req moveRequirementCategoryRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_category.move.bind`, err)
		return
	}
	if req.ParentId < 0 || req.PrevId < 0 || req.NextId < 0 || req.PrevId == categoryId || req.NextId == categoryId {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `requirement_category.move.validate`, business.ErrRequirementCategoryTreeInvalid)
		return
	}
	if err = business.MoveRequirementCategory(business.MoveRequirementCategoryModel{
		AdminId: c.GetInt(`adminId`), WorkspaceId: wsId, CategoryId: categoryId,
		ParentId: req.ParentId, PrevId: req.PrevId, NextId: req.NextId,
	}); err != nil {
		if abortRequirementCategoryError(c, lang, `requirement_category.move`, err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement_category.move`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

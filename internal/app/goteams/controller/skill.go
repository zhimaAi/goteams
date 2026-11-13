package controller

import (
	"errors"
	"net/http"
	"unicode/utf8"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
)

type createSkillRequest struct {
	Name        string `json:"name" binding:"required,max=100"`
	Remark      string `json:"remark" binding:"max=1000"`
	Icon        string `json:"icon" binding:"max=2048"`
	Description string `json:"description" binding:"max=10000"`
	Sharer      string `json:"sharer" binding:"max=100"`
	SourceURL   string `json:"source_url" binding:"required,max=2048"`
	Source      string `json:"source" binding:"required,oneof=github skillhub clawhub"`
}

type updateSkillRequest struct {
	Name         *string `json:"name" binding:"omitempty,min=1,max=100"`
	Remark       *string `json:"remark" binding:"omitempty,max=1000"`
	Icon         *string `json:"icon" binding:"omitempty,max=2048"`
	Description  *string `json:"description" binding:"omitempty,max=10000"`
	Sharer       *string `json:"sharer" binding:"omitempty,max=100"`
	SourceURL    *string `json:"source_url" binding:"omitempty,min=1,max=2048"`
	Source       *string `json:"source" binding:"omitempty,oneof=github skillhub clawhub"`
	ReviewStatus *string `json:"review_status" binding:"omitempty,oneof=pending approved rejected"`
}

type listSkillsQuery struct {
	Page         int    `form:"page" binding:"min=1"`
	Size         int    `form:"size" binding:"min=1,max=100"`
	Name         string `form:"name" binding:"max=100"`
	ReviewStatus string `form:"review_status" binding:"omitempty,oneof=pending approved rejected"`
}

// skillResponse 是创建后的详情与列表共用的公开技能契约。
type skillResponse struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Remark       string `json:"remark"`
	Icon         string `json:"icon"`
	Description  string `json:"description"`
	Sharer       string `json:"sharer"`
	SourceURL    string `json:"source_url"`
	Source       string `json:"source"`
	ReviewStatus string `json:"review_status"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

type createSkillResponse struct {
	ID int64 `json:"id"`
}

type listSkillsResponse struct {
	List  []skillResponse `json:"list"`
	Total int             `json:"total"`
	Page  int             `json:"page"`
	Size  int             `json:"size"`
}

func toSkillResponse(model business.SkillModel) skillResponse {
	icon := model.Icon
	if icon == "" && model.Name != "" {
		_, size := utf8.DecodeRuneInString(model.Name)
		icon = model.Name[:size]
	}
	return skillResponse{
		ID: model.ID, Name: model.Name, Remark: model.Remark, Icon: icon, Description: model.Description, Sharer: model.Sharer,
		SourceURL: model.SourceURL, Source: model.Source, ReviewStatus: model.ReviewStatus,
		CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt,
	}
}

func respondSkillError(c *gin.Context, stage string, err error) {
	lang := lib_i18n.GetLang(c)
	var fieldErr *validate.FieldError
	switch {
	case errors.Is(err, business.ErrSkillNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgNotFound))
	case errors.Is(err, business.ErrSkillChanged):
		abortConflict(c, lib_i18n.Show(lang, define.MsgSkillReviewConflict))
	case errors.Is(err, business.ErrSkillSourceURLExists):
		abortConflict(c, lib_i18n.Show(lang, define.MsgSkillSourceURLExists))
	case errors.As(err, &fieldErr):
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), stage, err)
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), stage, err)
	}
}

// CreateSkill 创建系统级技能，无需登录。
func CreateSkill(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req createSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "skills.create.bind", err)
		return
	}
	id, err := business.CreateSkill(business.CreateSkillModel{
		Name: req.Name, Remark: req.Remark, Icon: req.Icon, Description: req.Description, Sharer: req.Sharer,
		SourceURL: req.SourceURL, Source: req.Source,
	})
	if err != nil {
		respondSkillError(c, "skills.create", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: createSkillResponse{ID: id}})
}

// ListSkills 返回技能分页列表，支持 name 模糊搜索和 review_status 筛选。
func ListSkills(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	query := listSkillsQuery{Page: 1, Size: 20}
	if err := c.ShouldBindQuery(&query); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "skills.list.bind", err)
		return
	}
	result, err := business.ListSkills(business.ListSkillsModel{Page: query.Page, Size: query.Size, Name: query.Name, ReviewStatus: query.ReviewStatus})
	if err != nil {
		respondSkillError(c, "skills.list", err)
		return
	}
	response := listSkillsResponse{List: make([]skillResponse, 0, len(result.List)), Total: result.Total, Page: result.Page, Size: result.Size}
	for _, item := range result.List {
		response.List = append(response.List, toSkillResponse(item))
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response})
}

// GetSkill 返回技能详情。
func GetSkill(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	id, err := positiveParam(c, "skillId")
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "skills.get.param", err)
		return
	}
	model, err := business.GetSkill(id)
	if err != nil {
		respondSkillError(c, "skills.get", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: toSkillResponse(model)})
}

// UpdateSkill 更新提供的技能字段，支持只传审核状态。
func UpdateSkill(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	id, err := positiveParam(c, "skillId")
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "skills.update.param", err)
		return
	}
	var req updateSkillRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "skills.update.bind", err)
		return
	}
	err = business.UpdateSkill(business.UpdateSkillModel{
		ID: id, Name: req.Name, Remark: req.Remark, Icon: req.Icon, Description: req.Description, Sharer: req.Sharer,
		SourceURL: req.SourceURL, Source: req.Source, ReviewStatus: req.ReviewStatus,
	})
	if err != nil {
		respondSkillError(c, "skills.update", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// DeleteSkill 删除系统级技能。
func DeleteSkill(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	id, err := positiveParam(c, "skillId")
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "skills.delete.param", err)
		return
	}
	if err = business.DeleteSkill(id); err != nil {
		respondSkillError(c, "skills.delete", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

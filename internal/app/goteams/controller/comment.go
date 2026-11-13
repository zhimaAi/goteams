// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

type createCommentRequest struct {
	Content       string `json:"content"`
	AttachmentIds []int  `json:"attachment_ids"`
	ImageFileIds  []int  `json:"image_file_ids"` // 富文本中先行上传的图片/文件/视频，评论保存后回绑
	// ParentId is the replied-to comment id; 0 or omitted means a top-level comment.
	ParentId int `json:"parent_id"`
	// AtInvite marks a comment that mentions the work item's invitation submitter.
	AtInvite bool `json:"at_invite"`
	IsPublic bool `json:"is_public"`
}

type createCommentResponse struct {
	Id int `json:"id"`
}

func validateCreateCommentRequest(req createCommentRequest) error {
	if strings.TrimSpace(req.Content) == "" && len(req.AttachmentIds) == 0 && len(req.ImageFileIds) == 0 {
		return errors.New("comment content or attachment is required")
	}
	if req.ParentId < 0 {
		return errors.New("comment parent id must not be negative")
	}
	if err := validateCommentFileIDs(req.AttachmentIds); err != nil {
		return err
	}
	return validateCommentFileIDs(req.ImageFileIds)
}

func validateCommentFileIDs(ids []int) error {
	seen := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return errors.New("file id must be positive")
		}
		if _, exists := seen[id]; exists {
			return errors.New("file ids must be unique")
		}
		seen[id] = struct{}{}
	}
	return nil
}

func toCreateCommentModel(c *gin.Context, workspaceID, requirementID int, req createCommentRequest) business.CreateCommentModel {
	return business.CreateCommentModel{
		AdminId:       c.GetInt(`adminId`),
		WorkspaceId:   workspaceID,
		RequirementId: requirementID,
		UserId:        c.GetInt(`userId`),
		Content:       strings.TrimSpace(req.Content),
		ParentId:      req.ParentId,
		AtInvite:      req.AtInvite,
		IsPublic:      req.IsPublic,
		AttachmentIds: req.AttachmentIds,
		ImageFileIds:  req.ImageFileIds,
	}
}

func toCreateCommentResponse(model business.CreateCommentModel) createCommentResponse {
	return createCommentResponse{Id: model.Id}
}

// 列表沿用既有数字字符串契约；公开标记是新增布尔字段。
type commentResponse struct {
	Id                    string `json:"id"`
	AdminId               string `json:"admin_id"`
	WorkspaceId           string `json:"workspace_id"`
	RequirementId         string `json:"requirement_id"`
	UserId                string `json:"user_id"`
	Content               string `json:"content"`
	Source                string `json:"source"`
	ParentId              string `json:"parent_id"`
	AtInvite              string `json:"at_invite"`
	IsPublic              bool   `json:"is_public"`
	AutomationRuleId      string `json:"automation_rule_id"`
	AutomationExecutionId string `json:"automation_execution_id"`
	AuthorNameSnapshot    string `json:"author_name_snapshot"`
	AuthorAvatarSnapshot  string `json:"author_avatar_snapshot"`
	UserName              string `json:"user_name"`
	UserAvatar            string `json:"user_avatar"`
	CreatedAt             string `json:"created_at"`
	UpdatedAt             string `json:"updated_at"`
}

func toCommentResponse(model business.CommentModel) commentResponse {
	return commentResponse{
		Id: strconv.Itoa(model.Id), AdminId: strconv.Itoa(model.AdminId),
		WorkspaceId: strconv.Itoa(model.WorkspaceId), RequirementId: strconv.Itoa(model.RequirementId),
		UserId: strconv.Itoa(model.UserId), Content: model.Content, Source: model.Source,
		ParentId: strconv.Itoa(model.ParentId), AtInvite: strconv.Itoa(model.AtInvite), IsPublic: model.IsPublic,
		AutomationRuleId: strconv.Itoa(model.AutomationRuleId), AutomationExecutionId: strconv.FormatInt(model.AutomationExecutionId, 10),
		AuthorNameSnapshot: model.AuthorNameSnapshot, AuthorAvatarSnapshot: model.AuthorAvatarSnapshot,
		UserName: model.UserName, UserAvatar: model.UserAvatar,
		CreatedAt: strconv.FormatInt(model.CreatedAt, 10), UpdatedAt: strconv.FormatInt(model.UpdatedAt, 10),
	}
}

// ensureRequirementPermission checks data visibility and the work-item type permission.
func ensureRequirementPermission(c *gin.Context, lang string, workspaceID, requirementID int, action string) bool {
	adminId := c.GetInt(`adminId`)
	userId := c.GetInt(`userId`)
	item, err := business.GetVisibleRequirementById(adminId, cast.ToString(workspaceID), cast.ToString(requirementID), userId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `requirement.permission`, err)
		return false
	}
	if len(item) == 0 {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
		return false
	}
	if !hasWorkItemPermission(adminId, userId, cast.ToString(workspaceID), item[`type`], action) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return false
	}
	return true
}

// ensureRequirementVisible checks whether the current user may view the work item.
func ensureRequirementVisible(c *gin.Context, lang string, workspaceID, requirementID int) bool {
	return ensureRequirementPermission(c, lang, workspaceID, requirementID, `view`)
}

// ListComments returns the requirement comment list.
func ListComments(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, requirementID, _, ok := commentWorkItem(c, lang, "view")
	if !ok {
		return
	}
	// 评论列表属于读接口，叠加数据查看范围校验。
	if !ensureRequirementVisible(c, lang, workspaceID, requirementID) {
		return
	}
	list, err := business.ListCommentsByRequirement(business.ListCommentsModel{AdminId: c.GetInt(`adminId`), WorkspaceId: workspaceID, RequirementId: requirementID})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `comment.list.query`, err)
		return
	}

	response := make([]commentResponse, 0, len(list))
	for _, comment := range list {
		response = append(response, toCommentResponse(comment))
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response})
}

// CreateComment creates a normal comment.
func CreateComment(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, requirementID, _, ok := commentWorkItem(c, lang, "edit")
	if !ok {
		return
	}
	var req createCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `comment.create.bind`, err)
		return
	}
	if err := validateCreateCommentRequest(req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `comment.create.validate`, err)
		return
	}
	model, err := business.CreateComment(toCreateCommentModel(c, workspaceID, requirementID, req))
	if err != nil {
		if errors.Is(err, business.ErrCommentAttachmentsInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `comment.create.attachments`, err)
			return
		}
		if errors.Is(err, business.ErrCommentPublicUnavailable) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgCommentPublicUnavailable), `comment.create.public`, err)
			return
		}
		if errors.Is(err, business.ErrCommentParentInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgCommentParentInvalid), `comment.create.parent`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `comment.create.create`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: toCreateCommentResponse(model)})
}

type updateCommentRequest struct {
	Content      string `json:"content" binding:"required"`
	ImageFileIds []int  `json:"image_file_ids"`
}

func UpdateComment(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, requirementID, _, ok := commentWorkItem(c, lang, "edit")
	if !ok {
		return
	}
	commentID, err := positiveParam(c, `commentId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `comment.update.comment_id`, err)
		return
	}
	var req updateCommentRequest
	if err = c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `comment.update.bind`, err)
		return
	}
	if err = validateCommentFileIDs(req.ImageFileIds); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `comment.update.files`, err)
		return
	}
	if err = business.UpdateComment(business.UpdateCommentModel{AdminId: c.GetInt(`adminId`), WorkspaceId: workspaceID, RequirementId: requirementID, CommentId: commentID, UserId: c.GetInt(`userId`), Content: req.Content, ImageFileIds: req.ImageFileIds}); err != nil {
		commentMutationError(c, lang, "comment.update", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// DeleteComment deletes the current user's own comment.
func DeleteComment(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, requirementID, _, ok := commentWorkItem(c, lang, "edit")
	if !ok {
		return
	}
	commentID, err := positiveParam(c, `commentId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `comment.delete.comment_id`, err)
		return
	}
	if err = business.DeleteComment(c.GetInt(`adminId`), workspaceID, requirementID, commentID, c.GetInt(`userId`)); err != nil {
		commentMutationError(c, lang, "comment.delete", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// commentWorkItem validates the work item and the current user's operation permission.
func commentWorkItem(c *gin.Context, lang, action string) (int, int, string, bool) {
	workspaceID, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `comment.workspace_id`, err)
		return 0, 0, "", false
	}
	requirementID, err := positiveParam(c, `reqId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `comment.requirement_id`, err)
		return 0, 0, "", false
	}
	item, err := business.GetRequirementById(c.GetInt("adminId"), cast.ToString(workspaceID), cast.ToString(requirementID))
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), "comment.work_item", err)
		return 0, 0, "", false
	}
	if len(item) == 0 {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
		return 0, 0, "", false
	}
	if !business.HasPermission(c.GetInt("adminId"), c.GetInt("userId"), cast.ToString(workspaceID), item["type"]+":"+action) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return 0, 0, "", false
	}
	return workspaceID, requirementID, item["type"], true
}

// commentMutationError converts a comment business error into a unified response.
func commentMutationError(c *gin.Context, lang, stage string, err error) {
	switch {
	case errors.Is(err, business.ErrCommentNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgCommentNotFound))
	case errors.Is(err, business.ErrCommentOwnerRequired):
		abortForbidden(c, lib_i18n.Show(lang, define.MsgCommentOwnerRequired))
	case errors.Is(err, business.ErrCommentAttachmentsInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage, err)
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), stage, err)
	}
}

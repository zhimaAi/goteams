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
	"github.com/spf13/cast"
)

// ======================== Response DTOs ========================

type knowledgeItemResponse struct {
	Id              int    `json:"id"`
	ParentId        int    `json:"parent_id"`
	SortOrder       int    `json:"sort_order"`
	ItemType        string `json:"item_type"`
	Title           string `json:"title"`
	Revision        int64  `json:"revision"`
	VersionSeq      int64  `json:"version_seq"`
	CreatorId       int    `json:"creator_id"`
	CreatorName     string `json:"creator_name"`
	UpdatedBy       int    `json:"updated_by"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
	Favorite        bool   `json:"favorite"`
	ContentMode     string `json:"content_mode,omitempty"`
	SourceUrl       string `json:"source_url,omitempty"`
	Content         string `json:"content,omitempty"`
	ContentHash     string `json:"content_hash,omitempty"`
	SourceType      string `json:"source_type,omitempty"`
	SourceFileName  string `json:"source_file_name,omitempty"`
	SourceMimeType  string `json:"source_mime_type,omitempty"`
	SourceFileSize  int64  `json:"source_file_size,omitempty"`
	SourceExtension string `json:"source_extension,omitempty"`
	FolderTitle     string `json:"folder_title,omitempty"`
	DocumentCount   int    `json:"document_count,omitempty"`
}

type knowledgeVersionResponse struct {
	Id                   int    `json:"id"`
	DocumentId           int    `json:"document_id"`
	VersionNo            int64  `json:"version_no"`
	VersionType          string `json:"version_type"`
	Description          string `json:"description"`
	RestoreFromVersionId int64  `json:"restore_from_version_id"`
	EditSessionId        string `json:"edit_session_id"`
	CreatedBy            int    `json:"created_by"`
	CreatedByName        string `json:"created_by_name"`
	CreatedAt            int64  `json:"created_at"`
	Title                string `json:"title,omitempty"`
	Content              string `json:"content,omitempty"`
	ContentMode          string `json:"content_mode,omitempty"`
	ContentHash          string `json:"content_hash,omitempty"`
	Created              *bool  `json:"created,omitempty"`
}

type knowledgeActivityResponse struct {
	Id              int    `json:"id"`
	ItemId          int    `json:"item_id"`
	ItemTitle       string `json:"item_title"`
	ItemType        string `json:"item_type"`
	UserId          int    `json:"user_id"`
	UserName        string `json:"user_name"`
	Avatar          string `json:"avatar"`
	Action          string `json:"action"`
	CreatedAt       int64  `json:"created_at"`
	SourceExtension string `json:"source_extension,omitempty"`
}

type knowledgeFavoriteHomeResponse struct {
	Folders   []knowledgeItemResponse `json:"folders"`
	Documents []knowledgeItemResponse `json:"documents"`
}

type knowledgePageResponse struct {
	Items []knowledgeItemResponse `json:"items"`
	Total int                     `json:"total"`
}

func toKnowledgeItemResponse(model business.KnowledgeItemModel) knowledgeItemResponse {
	return knowledgeItemResponse{
		Id: model.Id, ParentId: model.ParentId, SortOrder: model.SortOrder, ItemType: model.ItemType, Title: model.Title,
		Revision: model.Revision, VersionSeq: model.VersionSeq, CreatorId: model.CreatorId,
		CreatorName: model.CreatorName, UpdatedBy: model.UpdatedBy, CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt, Favorite: model.Favorite, ContentMode: model.ContentMode,
		SourceUrl: model.SourceUrl, Content: model.Content, ContentHash: model.ContentHash,
		SourceType: model.SourceType, SourceFileName: model.SourceFileName,
		SourceMimeType: model.SourceMimeType, SourceFileSize: model.SourceFileSize,
		SourceExtension: model.SourceExtension,
		FolderTitle:     model.FolderTitle, DocumentCount: model.DocumentCount,
	}
}

func toKnowledgeVersionResponse(model business.KnowledgeVersionModel) knowledgeVersionResponse {
	return knowledgeVersionResponse{
		Id: model.Id, DocumentId: model.DocumentId, VersionNo: model.VersionNo,
		VersionType: model.VersionType, Description: model.Description,
		RestoreFromVersionId: model.RestoreFromVersionId, EditSessionId: model.EditSessionId,
		CreatedBy: model.CreatedBy, CreatedByName: model.CreatedByName, CreatedAt: model.CreatedAt,
		Title: model.Title, Content: model.Content, ContentMode: model.ContentMode, ContentHash: model.ContentHash,
	}
}

func toKnowledgeActivityResponse(model business.KnowledgeActivityModel) knowledgeActivityResponse {
	return knowledgeActivityResponse{
		Id: model.Id, ItemId: model.ItemId, ItemTitle: model.ItemTitle, ItemType: model.ItemType,
		UserId: model.UserId, UserName: model.UserName, Avatar: model.Avatar,
		Action: model.Action, CreatedAt: model.CreatedAt, SourceExtension: model.SourceExtension,
	}
}

// ======================== Shared helpers ========================

// kbBindError maps a business error to the unified response; returns true when handled.
func kbBindError(c *gin.Context, lang string, stage string, err error) bool {
	var conflict *business.KnowledgeRevisionConflictError
	switch {
	case errors.Is(err, business.ErrKnowledgeItemNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgKnowledgeItemNotFound))
		return true
	case errors.Is(err, business.ErrKnowledgeDocumentNotEditable):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgKnowledgeDocumentNotEditable), stage, err)
		return true
	case errors.Is(err, business.ErrKnowledgeFolderDepthExceeded):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgKnowledgeFolderDepthExceeded), stage, err)
		return true
	case errors.Is(err, business.ErrKnowledgeParentNotFound):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage, err)
		return true
	case errors.Is(err, business.ErrKnowledgeInvalidParent):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage, err)
		return true
	case errors.Is(err, validate.ErrKnowledgeTitleInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage, err)
		return true
	case errors.Is(err, validate.ErrKnowledgeContentTooLarge):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgKnowledgeContentTooLarge), stage, err)
		return true
	case errors.Is(err, business.ErrKnowledgeVersionNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgKnowledgeVersionNotFound))
		return true
	case errors.Is(err, business.ErrKnowledgeAttachmentNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgKnowledgeAttachmentNotFound))
		return true
	case errors.Is(err, business.ErrKnowledgeAttachmentTooLarge):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgKnowledgeAttachmentTooLarge), stage, err)
		return true
	case errors.Is(err, business.ErrKnowledgeAttachmentTypeInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgKnowledgeAttachmentTypeInvalid), stage, err)
		return true
	case errors.Is(err, business.ErrForbidden):
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return true
	case errors.As(err, &conflict):
		c.JSON(http.StatusConflict, define.Response{
			Code:    http.StatusConflict,
			Message: lib_i18n.Show(lang, define.MsgKnowledgeRevisionConflict),
			Data: map[string]any{
				`current_revision`: conflict.CurrentRevision,
				`updated_at`:       conflict.UpdatedAt,
				`updated_by`:       conflict.UpdatedBy,
			},
		})
		return true
	}
	abortInternal(c, lib_i18n.Show(lang, define.MsgError), stage, err)
	return true
}

// ======================== Tree / Detail / Search ========================

// GetKnowledgeTree returns the full directory tree metadata.
func GetKnowledgeTree(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)

	items, err := business.GetKnowledgeTree(adminId, cast.ToInt(wsId), userId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `knowledge.tree.list`, err)
		return
	}
	response := make([]knowledgeItemResponse, 0, len(items))
	for _, item := range items {
		response = append(response, toKnowledgeItemResponse(item))
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response})
}

// GetKnowledgeItemDetail returns the item detail with current content.
func GetKnowledgeItemDetail(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	itemId, err := positiveParam(c, `itemId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.detail.param`, err)
		return
	}
	item, err := business.GetKnowledgeItemDetail(adminId, cast.ToInt(wsId), itemId, userId)
	if err != nil {
		kbBindError(c, lang, `knowledge.detail.get`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: toKnowledgeItemResponse(item)})
}

// SearchKnowledgeItems searches by title or creator name.
func SearchKnowledgeItems(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	keyword := strings.TrimSpace(c.Query(`keyword`))
	if keyword == `` {
		c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: []knowledgeItemResponse{}})
		return
	}
	items, err := business.SearchKnowledgeItems(adminId, cast.ToInt(wsId), userId, keyword, 50)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `knowledge.search.list`, err)
		return
	}
	response := make([]knowledgeItemResponse, 0, len(items))
	for _, item := range items {
		response = append(response, toKnowledgeItemResponse(item))
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response})
}

// ======================== Create / Rename / Delete / Copy ========================

// CreateKnowledgeFolder creates a folder.
func CreateKnowledgeFolder(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)

	var req struct {
		ParentId int    `json:"parent_id"`
		Title    string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.folder.bind`, err)
		return
	}
	if err := validate.ValidateKnowledgeTitle(req.Title); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.folder.validate`, err)
		return
	}
	item, err := business.CreateKnowledgeFolder(adminId, cast.ToInt(wsId), userId, req.ParentId, strings.TrimSpace(req.Title))
	if err != nil {
		kbBindError(c, lang, `knowledge.folder.create`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: toKnowledgeItemResponse(item)})
}

// CreateKnowledgeDocument creates a document with the initial v1 version.
func CreateKnowledgeDocument(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)

	var req struct {
		ParentId      int    `json:"parent_id"`
		Title         string `json:"title"`
		Content       string `json:"content"`
		ContentMode   string `json:"content_mode"`
		EditSessionId string `json:"edit_session_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.document.bind`, err)
		return
	}
	if err := validate.ValidateKnowledgeTitle(req.Title); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.document.validate`, err)
		return
	}
	contentMode := req.ContentMode
	if contentMode == `` {
		contentMode = define.KnowledgeContentModeRichText
	}
	if contentMode != define.KnowledgeContentModeRichText && contentMode != define.KnowledgeContentModeMarkdown {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.document.mode`, errors.New(`invalid content mode`))
		return
	}
	item, err := business.CreateKnowledgeDocument(adminId, cast.ToInt(wsId), userId, req.ParentId,
		strings.TrimSpace(req.Title), req.Content, contentMode, req.EditSessionId)
	if err != nil {
		kbBindError(c, lang, `knowledge.document.create`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: toKnowledgeItemResponse(item)})
}

// CreateKnowledgeDocumentFromFile creates a read-only document from any local file.
func CreateKnowledgeDocumentFromFile(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)

	parentId := cast.ToInt(c.PostForm(`parent_id`))
	file, err := c.FormFile(`file`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.document_from_file.file`, err)
		return
	}
	if err = validate.ValidateKnowledgeFileUploadSize(file.Size); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgKnowledgeAttachmentTooLarge), `knowledge.document_from_file.size`, err)
		return
	}
	item, err := business.CreateKnowledgeDocumentFromFile(adminId, cast.ToInt(wsId), userId, parentId, file)
	if err != nil {
		kbBindError(c, lang, `knowledge.document_from_file.create`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: toKnowledgeItemResponse(item)})
}

// CreateKnowledgeLink creates a read-only link node.
func CreateKnowledgeLink(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)

	var req struct {
		ParentId  int    `json:"parent_id"`
		Title     string `json:"title"`
		SourceUrl string `json:"source_url"`
		URL       string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.link.bind`, err)
		return
	}
	req.SourceUrl = strings.TrimSpace(req.SourceUrl)
	if req.SourceUrl == `` {
		req.SourceUrl = strings.TrimSpace(req.URL)
	}
	if req.SourceUrl == `` || !strings.HasPrefix(req.SourceUrl, `http://`) && !strings.HasPrefix(req.SourceUrl, `https://`) {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.link.url`, errors.New(`invalid source url`))
		return
	}
	if req.Title != `` {
		if err := validate.ValidateKnowledgeTitle(req.Title); err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.link.validate`, err)
			return
		}
	}
	title := strings.TrimSpace(req.Title)
	if title == `` {
		titleRunes := []rune(req.SourceUrl)
		if len(titleRunes) > validate.KnowledgeTitleMaxLen {
			titleRunes = titleRunes[:validate.KnowledgeTitleMaxLen]
		}
		title = string(titleRunes)
	}
	if err := validate.ValidateKnowledgeTitle(title); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.link.validate`, err)
		return
	}
	item, err := business.CreateKnowledgeLink(adminId, cast.ToInt(wsId), userId, req.ParentId, title, req.SourceUrl)
	if err != nil {
		kbBindError(c, lang, `knowledge.link.create`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: toKnowledgeItemResponse(item)})
}

// RenameKnowledgeItem renames a folder/document/link.
func RenameKnowledgeItem(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	itemId, err := positiveParam(c, `itemId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.rename.param`, err)
		return
	}
	var req struct {
		Title     string `json:"title"`
		SourceUrl string `json:"source_url"`
	}
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.rename.bind`, err)
		return
	}
	if err = validate.ValidateKnowledgeTitle(req.Title); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.rename.validate`, err)
		return
	}
	req.SourceUrl = strings.TrimSpace(req.SourceUrl)
	if req.SourceUrl != `` && !strings.HasPrefix(req.SourceUrl, `http://`) && !strings.HasPrefix(req.SourceUrl, `https://`) {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.link.url`, errors.New(`invalid source url`))
		return
	}
	if err = business.RenameKnowledgeItem(adminId, cast.ToInt(wsId), userId, itemId, strings.TrimSpace(req.Title), req.SourceUrl); err != nil {
		kbBindError(c, lang, `knowledge.rename.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// MoveKnowledgeItem moves a folder/document/link to another folder (or reorders siblings).
func MoveKnowledgeItem(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	itemId, err := positiveParam(c, `itemId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.move.param`, err)
		return
	}
	var req struct {
		ParentId int `json:"parent_id"`
		// PrevId is the sibling the item is inserted after; 0 means top of the
		// list, -1 means append to the end (or move into a folder's tail).
		PrevId int `json:"prev_id"`
	}
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.move.bind`, err)
		return
	}
	if req.ParentId < 0 || req.PrevId < -1 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.move.validate`, nil)
		return
	}
	if err = business.MoveKnowledgeItem(adminId, cast.ToInt(wsId), userId, itemId, req.ParentId, req.PrevId); err != nil {
		kbBindError(c, lang, `knowledge.move.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// DeleteKnowledgeItem soft-deletes a node (folder delete cascades to descendants).
func DeleteKnowledgeItem(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	itemId, err := positiveParam(c, `itemId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.delete.param`, err)
		return
	}
	if err = business.DeleteKnowledgeItem(adminId, cast.ToInt(wsId), userId, itemId); err != nil {
		kbBindError(c, lang, `knowledge.delete.remove`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// CopyKnowledgeDocument creates a copy of a document with its current content.
func CopyKnowledgeDocument(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	documentId, err := positiveParam(c, `documentId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.copy.param`, err)
		return
	}
	var req struct {
		ParentId      int    `json:"parent_id"`
		EditSessionId string `json:"edit_session_id"`
	}
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.copy.bind`, err)
		return
	}
	item, err := business.CopyKnowledgeDocument(adminId, cast.ToInt(wsId), userId, documentId, req.ParentId, req.EditSessionId)
	if err != nil {
		kbBindError(c, lang, `knowledge.copy.create`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: toKnowledgeItemResponse(item)})
}

// ======================== Favorite / Home ========================

// SetKnowledgeFavorite toggles the favorite state of an item (idempotent).
func SetKnowledgeFavorite(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	itemId, err := positiveParam(c, `itemId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.favorite.param`, err)
		return
	}
	if err = business.SetKnowledgeFavorite(adminId, cast.ToInt(wsId), userId, itemId, true); err != nil {
		kbBindError(c, lang, `knowledge.favorite.set`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// UnsetKnowledgeFavorite removes the favorite state of an item (idempotent).
func UnsetKnowledgeFavorite(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	itemId, err := positiveParam(c, `itemId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.favorite.param`, err)
		return
	}
	if err = business.SetKnowledgeFavorite(adminId, cast.ToInt(wsId), userId, itemId, false); err != nil {
		kbBindError(c, lang, `knowledge.favorite.unset`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// GetKnowledgeHomeActivities returns the paginated activity feed.
func GetKnowledgeHomeActivities(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId := c.GetInt(`adminId`)
	page, size := paginationParams(c)
	items, total, err := business.GetKnowledgeHomeActivities(adminId, cast.ToInt(wsId), page, size)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `knowledge.home.activities`, err)
		return
	}
	response := make([]knowledgeActivityResponse, 0, len(items))
	for _, item := range items {
		response = append(response, toKnowledgeActivityResponse(item))
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: map[string]any{`items`: response, `total`: total}})
}

// GetKnowledgeHomeFavorites returns the user's favorite folders and documents.
func GetKnowledgeHomeFavorites(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	folders, documents, err := business.GetKnowledgeHomeFavorites(adminId, cast.ToInt(wsId), userId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `knowledge.home.favorites`, err)
		return
	}
	folderResponse := make([]knowledgeItemResponse, 0, len(folders))
	for _, item := range folders {
		folderResponse = append(folderResponse, toKnowledgeItemResponse(item))
	}
	documentResponse := make([]knowledgeItemResponse, 0, len(documents))
	for _, item := range documents {
		documentResponse = append(documentResponse, toKnowledgeItemResponse(item))
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: knowledgeFavoriteHomeResponse{Folders: folderResponse, Documents: documentResponse}})
}

// GetKnowledgeHomeCreatedByMe returns the paginated documents/links created by the user.
func GetKnowledgeHomeCreatedByMe(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	page, size := paginationParams(c)
	items, total, err := business.GetKnowledgeHomeCreatedByMe(adminId, cast.ToInt(wsId), userId, page, size)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `knowledge.home.created`, err)
		return
	}
	response := make([]knowledgeItemResponse, 0, len(items))
	for _, item := range items {
		response = append(response, toKnowledgeItemResponse(item))
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: knowledgePageResponse{Items: response, Total: total}})
}

// paginationParams parses the page/size query params with sane defaults.
func paginationParams(c *gin.Context) (page, size int) {
	page = cast.ToInt(c.Query(`page`))
	size = cast.ToInt(c.Query(`size`))
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	return page, size
}

// ======================== Autosave ========================

// SaveKnowledgeDocumentContent persists the document state under the optimistic revision lock.
// A revision mismatch returns 409 with the current server state in Data.
func SaveKnowledgeDocumentContent(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	documentId, err := positiveParam(c, `documentId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.save.param`, err)
		return
	}
	var req struct {
		Revision      int64  `json:"revision"`
		Title         string `json:"title"`
		Content       string `json:"content"`
		ContentMode   string `json:"content_mode"`
		EditSessionId string `json:"edit_session_id"`
	}
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.save.bind`, err)
		return
	}
	if err = validate.ValidateKnowledgeTitle(req.Title); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.save.validate`, err)
		return
	}
	if req.ContentMode != define.KnowledgeContentModeRichText && req.ContentMode != define.KnowledgeContentModeMarkdown {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.save.mode`, errors.New(`invalid content mode`))
		return
	}
	if err = validate.ValidateKnowledgeContentSize(req.Content); err != nil {
		kbBindError(c, lang, `knowledge.save.size`, err)
		return
	}
	result, err := business.SaveKnowledgeDocumentContent(business.SaveKnowledgeDocumentModel{
		AdminId: adminId, WorkspaceId: cast.ToInt(wsId), DocumentId: documentId, UserId: userId,
		Revision: req.Revision, Title: strings.TrimSpace(req.Title), Content: req.Content,
		ContentMode: req.ContentMode, EditSessionId: req.EditSessionId,
	})
	if err != nil {
		kbBindError(c, lang, `knowledge.save.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: map[string]any{
		`unchanged`: result.Unchanged, `revision`: result.NewRevision, `content_hash`: result.ContentHash,
		`updated_at`: result.UpdatedAt, `updated_by`: result.UpdatedBy,
	}})
}

// ConvertKnowledgeDocument converts an uploaded docx document (content_mode=file) into an editable
// rich-text document; the converted item detail is returned so the client can switch to the editor.
func ConvertKnowledgeDocument(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	documentId, err := positiveParam(c, `documentId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.convert.param`, err)
		return
	}
	var req struct {
		Revision int64  `json:"revision"`
		Title    string `json:"title"`
		Content  string `json:"content"`
	}
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.convert.bind`, err)
		return
	}
	if err = validate.ValidateKnowledgeTitle(req.Title); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.convert.validate`, err)
		return
	}
	if err = validate.ValidateKnowledgeContentSize(req.Content); err != nil {
		kbBindError(c, lang, `knowledge.convert.size`, err)
		return
	}
	item, err := business.ConvertKnowledgeUploadDocument(business.ConvertKnowledgeUploadDocumentModel{
		AdminId: adminId, WorkspaceId: cast.ToInt(wsId), DocumentId: documentId, UserId: userId,
		Revision: req.Revision, Title: strings.TrimSpace(req.Title), Content: req.Content,
	})
	if err != nil {
		kbBindError(c, lang, `knowledge.convert.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: toKnowledgeItemResponse(item)})
}

// ======================== Versions ========================

// GetKnowledgeVersions returns the paginated version metadata of a document.
func GetKnowledgeVersions(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId := c.GetInt(`adminId`)
	documentId, err := positiveParam(c, `documentId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.versions.param`, err)
		return
	}
	page, size := paginationParams(c)
	items, total, err := business.GetKnowledgeVersionList(adminId, cast.ToInt(wsId), documentId, page, size)
	if err != nil {
		kbBindError(c, lang, `knowledge.versions.list`, err)
		return
	}
	response := make([]knowledgeVersionResponse, 0, len(items))
	for _, item := range items {
		response = append(response, toKnowledgeVersionResponse(item))
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: map[string]any{`items`: response, `total`: total}})
}

// GetKnowledgeVersionDetail returns one version with its full snapshot.
func GetKnowledgeVersionDetail(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId := c.GetInt(`adminId`)
	documentId, err := positiveParam(c, `documentId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.version.param`, err)
		return
	}
	versionId, err := positiveParam(c, `versionId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.version.param`, err)
		return
	}
	item, err := business.GetKnowledgeVersionDetail(adminId, cast.ToInt(wsId), documentId, versionId)
	if err != nil {
		kbBindError(c, lang, `knowledge.version.detail`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: toKnowledgeVersionResponse(item)})
}

// CreateKnowledgeVersion creates a manual version snapshot of the current state.
func CreateKnowledgeVersion(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	documentId, err := positiveParam(c, `documentId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.version.create.param`, err)
		return
	}
	var req struct {
		Revision      int64  `json:"revision"`
		Description   string `json:"description"`
		EditSessionId string `json:"edit_session_id"`
	}
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.version.create.bind`, err)
		return
	}
	if err = validate.ValidateKnowledgeVersionDescription(req.Description); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.version.create.desc`, err)
		return
	}
	item, err := business.CreateKnowledgeVersion(business.CreateKnowledgeVersionModel{
		AdminId: adminId, WorkspaceId: cast.ToInt(wsId), DocumentId: documentId, UserId: userId,
		Revision: req.Revision, Description: strings.TrimSpace(req.Description), EditSessionId: req.EditSessionId,
	})
	if err != nil {
		kbBindError(c, lang, `knowledge.version.create`, err)
		return
	}
	response := toKnowledgeVersionResponse(item)
	response.Created = &item.Created
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response})
}

// RestoreKnowledgeVersion restores a version snapshot into the current state.
func RestoreKnowledgeVersion(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	documentId, err := positiveParam(c, `documentId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.version.restore.param`, err)
		return
	}
	versionId, err := positiveParam(c, `versionId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.version.restore.param`, err)
		return
	}
	var req struct {
		Revision int64 `json:"revision"`
	}
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.version.restore.bind`, err)
		return
	}
	item, err := business.RestoreKnowledgeVersion(business.RestoreKnowledgeVersionModel{
		AdminId: adminId, WorkspaceId: cast.ToInt(wsId), DocumentId: documentId, VersionId: versionId,
		UserId: userId, Revision: req.Revision,
	})
	if err != nil {
		kbBindError(c, lang, `knowledge.version.restore`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: toKnowledgeVersionResponse(item)})
}

// ======================== Attachments ========================

// uploadKnowledgeAttachment is the shared multipart upload handler.
func uploadKnowledgeAttachment(c *gin.Context, isImage, isVideo bool) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	documentId, err := positiveParam(c, `documentId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.attachment.param`, err)
		return
	}
	file, err := c.FormFile(`file`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.attachment.file`, err)
		return
	}
	attachment, err := business.UploadKnowledgeAttachment(adminId, cast.ToInt(wsId), userId, documentId, file, isImage, isVideo)
	if err != nil {
		kbBindError(c, lang, `knowledge.attachment.upload`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: map[string]any{
		`document_id`: attachment.DocumentId, `file_id`: attachment.FileId,
		`original_name`: attachment.OriginalName,
		`mime_type`:     attachment.MimeType, `file_size`: attachment.FileSize, `url`: attachment.URL,
	}})
}

// UploadKnowledgeImage uploads an image for a document and returns its relative URL.
func UploadKnowledgeImage(c *gin.Context) {
	uploadKnowledgeAttachment(c, true, false)
}

// UploadKnowledgeFile uploads a general file for a document and returns its relative URL.
func UploadKnowledgeFile(c *gin.Context) {
	uploadKnowledgeAttachment(c, false, false)
}

// UploadKnowledgeVideo uploads a video for a document and returns its relative URL.
func UploadKnowledgeVideo(c *gin.Context) {
	uploadKnowledgeAttachment(c, false, true)
}

// GetKnowledgeAttachmentContent streams the attachment file to the client.
func GetKnowledgeAttachmentContent(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId := c.GetInt(`adminId`)
	attachmentId, err := positiveParam(c, `attachmentId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.attachment.content.param`, err)
		return
	}
	if err = business.ServeKnowledgeAttachment(c, adminId, cast.ToInt(wsId), attachmentId); err != nil {
		kbBindError(c, lang, `knowledge.attachment.content`, err)
		return
	}
}

// DownloadKnowledgeDocument downloads an uploaded file in its original format or exports a created document as docx.
func DownloadKnowledgeDocument(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId := c.Param(`wsId`)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	documentId, err := positiveParam(c, `documentId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `knowledge.document.download.param`, err)
		return
	}
	if err = business.ServeKnowledgeDocumentDownload(c, adminId, cast.ToInt(wsId), userId, documentId); err != nil {
		kbBindError(c, lang, `knowledge.document.download`, err)
	}
}

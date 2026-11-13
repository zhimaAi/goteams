// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"net/http"
	"path/filepath"
	"strings"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
)

// UploadImage handles editor image uploads; only authenticated users may upload, and it returns a directly
// accessible URL plus the enterprise file ledger id (so the image counts toward storage before the requirement exists).
func UploadImage(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminID := c.GetInt("adminId")
	wsID, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "upload_image", err)
		return
	}
	userID := c.GetInt("userId")

	file, err := c.FormFile("file")
	if err != nil || file == nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgImageRequired), "upload_image", err)
		return
	}
	// Reject empty files outright to avoid persisting a 0-byte corrupt image.
	if file.Size == 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgImageRequired), "upload_image", nil)
		return
	}
	if file.Size > business.MaxImageSize {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgImageTooLarge), "upload_image", nil)
		return
	}
	// Determine the type by the file extension (a real attribute): the Content-Type header can be spoofed,
	// while a whitelist extension check both prevents spoofing and avoids false 500s when the extension is absent.
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(file.Filename)), ".")
	if !business.IsAllowedImageExt(ext) {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgImageTypeInvalid), "upload_image", nil)
		return
	}
	url, fileID, err := business.UploadImage(adminID, wsID, userID, file)
	if err != nil {
		// Storage exceeded: returns *QuotaExceededError, converted to 403 to trigger frontend "upgrade version" modal.
		if abortQuotaExceeded(c, lib_i18n.Show(lang, define.MsgQuotaExceeded), "upload_image", err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgServerError), "upload_image", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    http.StatusOK,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    gin.H{"url": url, "file_id": fileID},
	})
}

// UploadEditorFile handles document uploads from the rich-text editor, returns url and file ledger id.
func UploadEditorFile(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminID := c.GetInt("adminId")
	wsID, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "upload_editor_file", err)
		return
	}
	userID := c.GetInt("userId")

	file, err := c.FormFile("file")
	if err != nil || file == nil || file.Size == 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgFileRequired), "upload_editor_file", err)
		return
	}
	if file.Size > business.MaxEditorFileSize {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgFileTooLarge), "upload_editor_file", nil)
		return
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(file.Filename)), ".")
	if !business.IsAllowedEditorFileExt(ext) {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgFileTypeInvalid), "upload_editor_file", nil)
		return
	}
	url, fileID, err := business.UploadEditorFile(adminID, wsID, userID, file)
	if err != nil {
		if abortQuotaExceeded(c, lib_i18n.Show(lang, define.MsgQuotaExceeded), "upload_editor_file", err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgServerError), "upload_editor_file", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    http.StatusOK,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: gin.H{
			"url":     url,
			"name":    file.Filename,
			"size":    file.Size,
			"file_id": fileID,
		},
	})
}

// UploadEditorVideo handles local video uploads from the rich-text editor, returns url and file ledger id.
func UploadEditorVideo(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminID := c.GetInt("adminId")
	wsID, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "upload_editor_video", err)
		return
	}
	userID := c.GetInt("userId")

	file, err := c.FormFile("file")
	if err != nil || file == nil || file.Size == 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgVideoRequired), "upload_editor_video", err)
		return
	}
	if file.Size > business.MaxEditorVideoSize {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgVideoTooLarge), "upload_editor_video", nil)
		return
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(file.Filename)), ".")
	if !business.IsAllowedEditorVideoExt(ext) {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgVideoTypeInvalid), "upload_editor_video", nil)
		return
	}
	url, fileID, err := business.UploadEditorVideo(adminID, wsID, userID, file)
	if err != nil {
		if abortQuotaExceeded(c, lib_i18n.Show(lang, define.MsgQuotaExceeded), "upload_editor_video", err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgServerError), "upload_editor_video", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    http.StatusOK,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: gin.H{
			"url":     url,
			"name":    file.Filename,
			"size":    file.Size,
			"file_id": fileID,
		},
	})
}

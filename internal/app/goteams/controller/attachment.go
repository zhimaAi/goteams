// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
	"goteams/internal/pkg/lib_storage"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

// ListAttachments returns requirement attachments.
func ListAttachments(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, requirementID, attachmentID, err := attachmentIDs(c, false)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `attachment.list.id`, err)
		return
	}
	_ = attachmentID
	// 附件列表属于读接口，叠加数据查看范围校验。
	if !ensureRequirementVisible(c, lang, workspaceID, requirementID) {
		return
	}
	list, err := business.ListAttachments(c.GetInt(`adminId`), workspaceID, requirementID)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `attachment.list.query`, err)
		return
	}
	// For local-stored attachments, fill in a publicly accessible URL that needs no auth (OSS stored_path is already a full URL).
	for i := range list {
		if sp := list[i][`stored_path`]; sp != `` {
			list[i][`url`] = sp
		}
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: list})
}

// UploadAttachments uploads one or more requirement attachments.
func UploadAttachments(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, requirementID, _, err := attachmentIDs(c, false)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `attachment.upload.id`, err)
		return
	}
	if !ensureRequirementPermission(c, lang, workspaceID, requirementID, `edit`) {
		return
	}
	form, err := c.MultipartForm()
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `attachment.upload.form`, err)
		return
	}
	commentID := cast.ToInt(c.PostForm(`comment_id`))
	files := form.File[`files`]
	if len(files) == 0 {
		files = form.File[`file`]
	}
	if len(files) == 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `attachment.upload.empty`, errors.New(`file is required`))
		return
	}
	items := make([]any, 0, len(files))
	for _, file := range files {
		item, uploadErr := business.CreateAttachment(c.GetInt(`adminId`), workspaceID, requirementID, commentID, c.GetInt(`userId`), file)
		if uploadErr != nil {
			if uploadErr.Error() == `attachment too large` {
				abortBadRequest(c, lib_i18n.Show(lang, define.MsgAttachmentTooLarge), `attachment.upload.size`, uploadErr)
				return
			}
			// Storage exceeded: returns *QuotaExceededError, converted to 403 to trigger frontend "upgrade version" modal.
			if abortQuotaExceeded(c, lib_i18n.Show(lang, define.MsgQuotaExceeded), `attachment.upload.create`, uploadErr) {
				return
			}
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `attachment.upload.create`, uploadErr)
			return
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: items})
}

// DownloadAttachment downloads a requirement attachment.
func DownloadAttachment(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, requirementID, attachmentID, err := attachmentIDs(c, true)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `attachment.download.id`, err)
		return
	}
	// 附件下载属于读接口，叠加数据查看范围校验。
	if !ensureRequirementVisible(c, lang, workspaceID, requirementID) {
		return
	}
	item, err := business.GetAttachment(c.GetInt(`adminId`), workspaceID, requirementID, attachmentID)
	if err != nil {
		if errors.Is(err, business.ErrAttachmentNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgAttachmentNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `attachment.download.query`, err)
		return
	}
	c.Header(`Content-Disposition`, fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(item[`original_name`])))
	if err := lib_storage.M.Serve(item[`stored_path`], c); err != nil {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgAttachmentNotFound))
	}
}

// DeleteAttachment deletes a requirement attachment.
func DeleteAttachment(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, requirementID, attachmentID, err := attachmentIDs(c, true)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `attachment.delete.id`, err)
		return
	}
	if !ensureRequirementPermission(c, lang, workspaceID, requirementID, `edit`) {
		return
	}
	err = business.DeleteAttachment(c.GetInt(`adminId`), workspaceID, requirementID, attachmentID)
	if err != nil {
		if errors.Is(err, business.ErrAttachmentNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgAttachmentNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `attachment.delete.delete`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

func attachmentIDs(c *gin.Context, needAttachment bool) (int, int, int, error) {
	workspaceID, err := positiveParam(c, `wsId`)
	if err != nil {
		return 0, 0, 0, err
	}
	requirementID, err := positiveParam(c, `reqId`)
	if err != nil {
		return 0, 0, 0, err
	}
	if !needAttachment {
		return workspaceID, requirementID, 0, nil
	}
	attachmentID, err := positiveParam(c, `attachmentId`)
	return workspaceID, requirementID, attachmentID, err
}

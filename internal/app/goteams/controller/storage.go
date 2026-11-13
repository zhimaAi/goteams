package controller

import (
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

// ListEnterpriseFiles returns the paginated file list for the enterprise storage page.
func ListEnterpriseFiles(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminID := c.GetInt(`adminId`)
	workspaceID := cast.ToInt(c.Query(`workspace_id`))
	sourceType := c.Query(`source_type`)
	page := cast.ToInt(c.DefaultQuery(`page`, `1`))
	pageSize := cast.ToInt(c.DefaultQuery(`page_size`, `20`))
	sortField := c.Query(`sort_field`)
	sortOrder := c.Query(`sort_order`)

	result, err := business.ListEnterpriseFiles(adminID, workspaceID, sourceType, page, pageSize, sortField, sortOrder)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `storage.list.query`, err)
		return
	}

	// Supplement project name mapping.
	wsNameMap, _ := business.GetWorkspaceNameMap(adminID)
	// Inject workspace name into each file record.
	type fileWithProject struct {
		business.EnterpriseFile
		ProjectName string `json:"project_name"`
	}
	items := make([]fileWithProject, len(result.List))
	for i, f := range result.List {
		items[i] = fileWithProject{
			EnterpriseFile: f,
			ProjectName:    wsNameMap[f.WorkspaceID],
		}
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: gin.H{
			`list`:       items,
			`total`:      result.Total,
			`total_size`: result.TotalSize,
		},
	})
}

// DownloadEnterpriseFile streams an enterprise file for download.
func DownloadEnterpriseFile(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminID := c.GetInt(`adminId`)
	fileID := cast.ToInt(c.Param(`fileId`))

	file, err := business.GetEnterpriseFileByID(adminID, fileID)
	if err != nil {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgAttachmentNotFound))
		return
	}

	c.Header(`Content-Disposition`, fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(file.FileName)))
	if err := lib_storage.M.Serve(file.StoredPath, c); err != nil {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgAttachmentNotFound))
	}
}

// DeleteEnterpriseFile deletes a single enterprise file (DB record + physical file + requirement_attachments cleanup).
func DeleteEnterpriseFile(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminID := c.GetInt(`adminId`)
	fileID := cast.ToInt(c.Param(`fileId`))

	if err := business.DeleteEnterpriseFile(adminID, fileID); err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `storage.delete.delete`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// BatchDeleteEnterpriseFiles deletes multiple enterprise files.
func BatchDeleteEnterpriseFiles(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminID := c.GetInt(`adminId`)

	var req struct {
		IDs []int `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `storage.batch_delete.bind`, err)
		return
	}

	if err := business.BatchDeleteEnterpriseFiles(adminID, req.IDs); err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `storage.batch_delete.delete`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

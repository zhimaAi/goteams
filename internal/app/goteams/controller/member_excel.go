// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
)

// ======================== Enterprise Member Excel Import/Export ========================

// DownloadEnterpriseMemberImportTemplate downloads the member Excel import template.
func DownloadEnterpriseMemberImportTemplate(c *gin.Context) {
	downloadEnterpriseMemberWorkbook(c, true)
}

// ExportEnterpriseMembers exports all enterprise members as an Excel file (password column stays empty).
func ExportEnterpriseMembers(c *gin.Context) {
	downloadEnterpriseMemberWorkbook(c, false)
}

func downloadEnterpriseMemberWorkbook(c *gin.Context, templateOnly bool) {
	lang := lib_i18n.GetLang(c)
	data, err := business.BuildEnterpriseMemberWorkbook(c.GetInt(`adminId`), lang, c.Query(`timezone`), templateOnly)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.member.excel.build`, err)
		return
	}
	nameKey := `member_excel_export_filename`
	if templateOnly {
		nameKey = `member_excel_template_filename`
	}
	filename := lib_i18n.Show(lang, nameKey)
	c.Header(`Content-Disposition`, `attachment; filename*=UTF-8''`+url.PathEscape(filename))
	c.Data(http.StatusOK, `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`, data)
}

// ImportEnterpriseMembers imports members from Excel: matching accounts update, new accounts are created.
func ImportEnterpriseMembers(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	file, ok := openMemberImportFile(c, `enterprise.member.import`)
	if !ok {
		return
	}
	defer func() { _ = file.Close() }()
	result, err := business.ImportEnterpriseMembers(file, c.GetInt(`adminId`), lang, c.GetInt(`userId`))
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, `member_import_file_invalid`), `enterprise.member.import.parse`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, `member_import_completed`), Data: result})
}

// PreviewEnterpriseMemberImport pre-parses the member Excel file and returns importable row counts.
func PreviewEnterpriseMemberImport(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	file, ok := openMemberImportFile(c, `enterprise.member.import_preview`)
	if !ok {
		return
	}
	defer func() { _ = file.Close() }()
	result, err := business.PreviewEnterpriseMembers(file, c.GetInt(`adminId`), lang, c.GetInt(`userId`))
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, `member_import_file_invalid`), `enterprise.member.import_preview.parse`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: result})
}

func openMemberImportFile(c *gin.Context, scene string) (multipart.File, bool) {
	lang := lib_i18n.GetLang(c)
	fileHeader, err := c.FormFile(`file`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), scene+`.file`, err)
		return nil, false
	}
	if fileHeader.Size <= 0 || fileHeader.Size > workItemExcelMaxFileSize || !strings.EqualFold(strings.TrimPrefix(strings.ToLower(fileExtension(fileHeader.Filename)), `.`), `xlsx`) {
		abortBadRequest(c, lib_i18n.Show(lang, `member_import_file_invalid`), scene+`.file_invalid`, fmt.Errorf("invalid file: %s", fileHeader.Filename))
		return nil, false
	}
	file, err := fileHeader.Open()
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), scene+`.open`, err)
		return nil, false
	}
	return file, true
}

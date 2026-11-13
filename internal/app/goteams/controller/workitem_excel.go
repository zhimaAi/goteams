// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

const workItemExcelMaxFileSize = 100 << 20

// DownloadWorkItemImportTemplate downloads the Excel import template for the current work-item type.
func DownloadWorkItemImportTemplate(c *gin.Context) {
	downloadWorkItemWorkbook(c, true)
}

// ExportWorkItems exports data of the current work-item type.
func ExportWorkItems(c *gin.Context) {
	downloadWorkItemWorkbook(c, false)
}

func downloadWorkItemWorkbook(c *gin.Context, templateOnly bool) {
	lang := lib_i18n.GetLang(c)
	wsId, typeKey, ok := validateWorkItemExcelRequest(c, map[bool]string{true: `import`, false: `export`}[templateOnly])
	if !ok {
		return
	}
	assigneeIdsText := c.Query(`assignee_ids`)
	if assigneeIdsText == `` {
		assigneeIdsText = c.Query(`assignee_id`)
	}
	// root_only=1 is equivalent to fetching only root nodes (parent_id = 0).
	var parentId *int
	if c.Query(`root_only`) == `1` {
		root := 0
		parentId = &root
	}
	ids := parseWorkItemIDs(c.Query(`ids`))
	statusIds, err := parseIntegerIDList(c.Query(`status_ids`), false)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workitem.export.status_ids`, err)
		return
	}
	priorityIds, err := parseIntegerIDList(c.Query(`priority_ids`), true)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workitem.export.priority_ids`, err)
		return
	}
	versionIds, err := parseIntegerIDList(c.Query(`version_ids`), true)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workitem.export.version_ids`, err)
		return
	}
	createdByIds, err := parseIntegerIDList(c.Query(`created_by_ids`), false)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workitem.export.created_by_ids`, err)
		return
	}
	searchFilters, err := parseAndValidateSearchFilters(c.Query(`search_filters`), c.GetInt(`adminId`), wsId, c.GetInt(`userId`), typeKey)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workitem.export.search_filters`, err)
		return
	}
	timezone, err := normalizeRequestTimezone(c.Query(`timezone`))
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workitem.export.timezone`, err)
		return
	}
	data, err := business.BuildWorkItemWorkbook(business.ListRequirementsModel{
		AdminId:       c.GetInt(`adminId`),
		WorkspaceId:   wsId,
		UserId:        c.GetInt(`userId`),
		Type:          typeKey,
		StatusId:      cast.ToInt(c.Query(`status_id`)),
		StatusIds:     statusIds,
		Priority:      cast.ToInt(c.Query(`priority`)),
		PriorityIds:   priorityIds,
		AssigneeIds:   parsePositiveIDList(assigneeIdsText),
		VersionId:     cast.ToInt(c.Query(`version_id`)),
		VersionIds:    versionIds,
		CreatedBy:     cast.ToInt(c.Query(`created_by`)),
		CreatedByIds:  createdByIds,
		ParentId:      parentId,
		Search:        c.Query(`search`),
		TitleSearch:   c.Query(`title_search`),
		SearchFilters: searchFilters,
		Timezone:      timezone,
	}, lang, timezone, ids, templateOnly)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workitem.excel.build`, err)
		return
	}
	nameKey := `workitem_excel_export_filename`
	if templateOnly {
		nameKey = `workitem_excel_template_filename`
	}
	filename := fmt.Sprintf(lib_i18n.Show(lang, nameKey), localizedControllerWorkItemType(typeKey, lang))
	c.Header(`Content-Disposition`, `attachment; filename*=UTF-8''`+url.PathEscape(filename))
	c.Data(http.StatusOK, `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`, data)
}

// ImportWorkItems imports Excel work items, creating rows with empty IDs and updating existing IDs within the current workspace.
func ImportWorkItems(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, typeKey, ok := validateWorkItemExcelRequest(c, `import`)
	if !ok {
		return
	}
	fileHeader, err := c.FormFile(`file`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workitem.import.file`, err)
		return
	}
	if fileHeader.Size <= 0 || fileHeader.Size > workItemExcelMaxFileSize || !strings.EqualFold(strings.TrimPrefix(strings.ToLower(fileExtension(fileHeader.Filename)), `.`), `xlsx`) {
		abortBadRequest(c, lib_i18n.Show(lang, `workitem_import_file_invalid`), `workitem.import.file_invalid`, fmt.Errorf("invalid file: %s", fileHeader.Filename))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workitem.import.open`, err)
		return
	}
	defer func() { _ = file.Close() }()
	result, err := business.ImportWorkItems(file, c.GetInt(`adminId`), cast.ToString(wsId), typeKey, lang, c.PostForm(`timezone`), c.GetInt(`userId`))
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, `workitem_import_file_invalid`), `workitem.import.parse`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, `workitem_import_completed`), Data: result})
}

// PreviewWorkItemImport pre-parses the Excel work-item file and returns the number of importable rows.
func PreviewWorkItemImport(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, typeKey, ok := validateWorkItemExcelRequest(c, `import`)
	if !ok {
		return
	}
	fileHeader, err := c.FormFile(`file`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workitem.import_preview.file`, err)
		return
	}
	if fileHeader.Size <= 0 || fileHeader.Size > workItemExcelMaxFileSize || !strings.EqualFold(strings.TrimPrefix(strings.ToLower(fileExtension(fileHeader.Filename)), `.`), `xlsx`) {
		abortBadRequest(c, lib_i18n.Show(lang, `workitem_import_file_invalid`), `workitem.import_preview.file_invalid`, fmt.Errorf("invalid file: %s", fileHeader.Filename))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workitem.import_preview.open`, err)
		return
	}
	defer func() { _ = file.Close() }()
	result, err := business.PreviewWorkItems(file, c.GetInt(`adminId`), cast.ToString(wsId), typeKey, lang, c.PostForm(`timezone`), c.GetInt(`userId`))
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, `workitem_import_file_invalid`), `workitem.import_preview.parse`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: result})
}

func validateWorkItemExcelRequest(c *gin.Context, action string) (int, string, bool) {
	lang := lib_i18n.GetLang(c)
	wsId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workitem.excel.workspace_id`, err)
		return 0, ``, false
	}
	typeKey := c.Query(`type`)
	if err = validate.ValidateRequirementType(typeKey); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workitem.excel.type`, err)
		return 0, ``, false
	}
	if !hasWorkItemPermission(c.GetInt(`adminId`), c.GetInt(`userId`), cast.ToString(wsId), typeKey, action) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return 0, ``, false
	}
	return wsId, typeKey, true
}

func parseWorkItemIDs(value string) []int {
	ids := []int{}
	seen := map[int]bool{}
	for _, item := range strings.Split(value, `,`) {
		if id, err := strconv.Atoi(strings.TrimSpace(item)); err == nil && id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func fileExtension(filename string) string {
	index := strings.LastIndex(filename, `.`)
	if index < 0 {
		return ``
	}
	return filename[index:]
}

func localizedControllerWorkItemType(typeKey, lang string) string {
	if typeKey == define.TypeKeyDefect {
		return lib_i18n.Show(lang, `workitem_type_defect`)
	}
	return lib_i18n.Show(lang, `workitem_type_requirement`)
}

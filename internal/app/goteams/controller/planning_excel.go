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
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

// planningTitleMaxLen limits a batch-created planning name, in sync with the inline-edit limit.
const planningTitleMaxLen = 255

// planningBatchMaxRows limits how many planning names one batch-create request may carry.
const planningBatchMaxRows = 500

// ======================== Recycle bin DTO ========================

type deletedPlanningResponseItem struct {
	Id            int    `json:"id"`
	Title         string `json:"title"`
	GroupId       int    `json:"group_id"`
	GroupName     string `json:"group_name"`
	DeletedAt     int64  `json:"deleted_at"`
	DeletedBy     int    `json:"deleted_by"`
	DeletedByName string `json:"deleted_by_name"`
}

type recyclePlanningsRequest struct {
	Ids []int `json:"ids"`
}

type recyclePlanningsResponse struct {
	Affected int `json:"affected"`
}

type batchCreatePlanningsRequest struct {
	GroupId int      `json:"group_id"`
	Titles  []string `json:"titles"`
}

type batchCreatePlanningsResponse struct {
	Created int `json:"created"`
}

type buildPlanningWorkbookQuery struct {
	GroupId     int    `form:"group_id"`
	Timezone    string `form:"timezone"`
	Title       string `form:"title"`
	Priority    string `form:"priority"`
	StatusId    string `form:"status_id"`
	AssigneeId  int    `form:"assignee_id"`
	HasRelation string `form:"has_relation"`
	Archived    string `form:"archived"`
}

// ======================== Recycle bin APIs ========================

// ListDeletedPlannings returns the workspace's recycle-bin plannings.
func ListDeletedPlannings(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	model, err := business.ListDeletedPlannings(business.ListDeletedPlanningsModel{
		AdminId:     c.GetInt(`adminId`),
		WorkspaceId: c.Param(`wsId`),
		Lang:        lang,
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `planning.recycle.list`, err)
		return
	}
	items := make([]deletedPlanningResponseItem, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, deletedPlanningResponseItem{
			Id: item.Id, Title: item.Title, GroupId: item.GroupId, GroupName: item.GroupName,
			DeletedAt: item.DeletedAt, DeletedBy: item.DeletedBy, DeletedByName: item.DeletedByName,
		})
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    items,
	})
}

// RestorePlannings restores plannings from the recycle bin; a deleted original group is revived along with them.
func RestorePlannings(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	ids, ok := bindRecyclePlanningIds(c, lang, `planning.recycle.restore`)
	if !ok {
		return
	}
	model, err := business.RestorePlannings(business.RecyclePlanningsModel{
		AdminId:     c.GetInt(`adminId`),
		WorkspaceId: c.Param(`wsId`),
		PlanningIds: ids,
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `planning.recycle.restore`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    recyclePlanningsResponse{Affected: model.Affected},
	})
}

// PurgePlannings permanently deletes plannings from the recycle bin.
func PurgePlannings(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	ids, ok := bindRecyclePlanningIds(c, lang, `planning.recycle.purge`)
	if !ok {
		return
	}
	model, err := business.PurgePlannings(business.RecyclePlanningsModel{
		AdminId:     c.GetInt(`adminId`),
		WorkspaceId: c.Param(`wsId`),
		PlanningIds: ids,
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `planning.recycle.purge`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    recyclePlanningsResponse{Affected: model.Affected},
	})
}

// bindRecyclePlanningIds parses and validates the planning ID list carried by a recycle-bin request.
func bindRecyclePlanningIds(c *gin.Context, lang, scene string) ([]int, bool) {
	var req recyclePlanningsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), scene+`.bind`, err)
		return nil, false
	}
	ids := make([]int, 0, len(req.Ids))
	seen := make(map[int]bool, len(req.Ids))
	for _, id := range req.Ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), scene+`.ids`, nil)
		return nil, false
	}
	return ids, true
}

// ======================== Batch create API ========================

// BatchCreatePlannings creates planning rows in bulk, one name per line.
func BatchCreatePlannings(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req batchCreatePlanningsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning.batch_create.bind`, err)
		return
	}
	if req.GroupId <= 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning.batch_create.group`, nil)
		return
	}
	titles := make([]string, 0, len(req.Titles))
	for _, title := range req.Titles {
		title = strings.TrimSpace(title)
		if title == `` {
			continue
		}
		if len([]rune(title)) > planningTitleMaxLen {
			title = string([]rune(title)[:planningTitleMaxLen])
		}
		titles = append(titles, title)
	}
	if len(titles) == 0 || len(titles) > planningBatchMaxRows {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning.batch_create.titles`, nil)
		return
	}
	drafts := make([]business.PlanningDraftModel, 0, len(titles))
	for _, title := range titles {
		drafts = append(drafts, business.PlanningDraftModel{Title: title})
	}
	model, err := business.BatchCreatePlannings(business.BatchCreatePlanningsModel{
		AdminId:     c.GetInt(`adminId`),
		UserId:      c.GetInt(`userId`),
		WorkspaceId: c.Param(`wsId`),
		GroupId:     req.GroupId,
		Drafts:      drafts,
	})
	if err != nil {
		if abortPlanningError(c, lang, `planning.batch_create`, err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `planning.batch_create`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    batchCreatePlanningsResponse{Created: model.Created},
	})
}

// ======================== Excel APIs ========================

// DownloadPlanningImportTemplate downloads the planning Excel import template.
func DownloadPlanningImportTemplate(c *gin.Context) {
	downloadPlanningWorkbook(c, true)
}

// ExportPlannings exports the current group's planning list as Excel.
func ExportPlannings(c *gin.Context) {
	downloadPlanningWorkbook(c, false)
}

func downloadPlanningWorkbook(c *gin.Context, templateOnly bool) {
	lang := lib_i18n.GetLang(c)
	var query buildPlanningWorkbookQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning.excel.bind`, err)
		return
	}
	priorityIds, err := parsePlanningFilterIDs(query.Priority)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning.excel.priority`, err)
		return
	}
	statusIds, err := parsePlanningFilterIDs(query.StatusId)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning.excel.status`, err)
		return
	}
	archived, err := parsePlanningArchivedFilter(query.Archived)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning.excel.archived`, err)
		return
	}

	model := business.BuildPlanningWorkbookModel{
		AdminId:      c.GetInt(`adminId`),
		WorkspaceId:  c.Param(`wsId`),
		GroupId:      query.GroupId,
		Lang:         lang,
		Timezone:     query.Timezone,
		TemplateOnly: templateOnly,
		Title:        query.Title,
		PriorityIds:  priorityIds,
		StatusIds:    statusIds,
		AssigneeId:   query.AssigneeId,
		Archived:     archived,
	}
	if query.HasRelation != `` {
		hasRelation, err := strconv.ParseBool(query.HasRelation)
		if err != nil {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning.excel.has_relation`, err)
			return
		}
		model.HasRelation = &hasRelation
	}
	if !templateOnly && model.GroupId <= 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `planning.excel.group`, nil)
		return
	}
	data, err := business.BuildPlanningWorkbook(model)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `planning.excel.build`, err)
		return
	}
	nameKey := `planning_excel_export_filename`
	if templateOnly {
		nameKey = `planning_excel_template_filename`
	}
	filename := lib_i18n.Show(lang, nameKey)
	c.Header(`Content-Disposition`, `attachment; filename*=UTF-8''`+url.PathEscape(filename))
	c.Data(http.StatusOK, `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`, data)
}

// ImportPlannings imports plannings from Excel; rows with an empty ID are created in the current group.
func ImportPlannings(c *gin.Context) {
	handlePlanningImport(c, false)
}

// PreviewPlanningImport pre-parses the planning Excel file and returns the number of importable rows.
func PreviewPlanningImport(c *gin.Context) {
	handlePlanningImport(c, true)
}

func handlePlanningImport(c *gin.Context, dryRun bool) {
	lang := lib_i18n.GetLang(c)
	scene := `planning.import`
	if dryRun {
		scene = `planning.import_preview`
	}

	groupId := cast.ToInt(c.Query(`group_id`))
	if groupId <= 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), scene+`.group`, nil)
		return
	}
	fileHeader, err := c.FormFile(`file`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), scene+`.file`, err)
		return
	}
	if fileHeader.Size <= 0 || fileHeader.Size > workItemExcelMaxFileSize ||
		!strings.EqualFold(strings.TrimPrefix(strings.ToLower(fileExtension(fileHeader.Filename)), `.`), `xlsx`) {
		abortBadRequest(c, lib_i18n.Show(lang, `workitem_import_file_invalid`), scene+`.file_invalid`, fmt.Errorf("invalid file: %s", fileHeader.Filename))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), scene+`.open`, err)
		return
	}
	defer func() { _ = file.Close() }()

	model := business.ImportPlanningsModel{
		AdminId:     c.GetInt(`adminId`),
		UserId:      c.GetInt(`userId`),
		WorkspaceId: c.Param(`wsId`),
		GroupId:     groupId,
		Lang:        lang,
		Timezone:    c.PostForm(`timezone`),
	}
	if dryRun {
		preview, previewErr := business.PreviewPlannings(file, model)
		if previewErr != nil {
			abortBadRequest(c, lib_i18n.Show(lang, `workitem_import_file_invalid`), scene+`.parse`, previewErr)
			return
		}
		c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: preview})
		return
	}
	result, err := business.ImportPlannings(file, model)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, `workitem_import_file_invalid`), scene+`.parse`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, `workitem_import_completed`), Data: result})
}

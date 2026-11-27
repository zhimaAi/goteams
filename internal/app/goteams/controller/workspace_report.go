package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"github.com/zhimaAi/go_tools/logs"
)

type generateReportRequest struct {
	RequestId  string   `json:"request_id"`
	Preset     string   `json:"preset"`
	StartDate  string   `json:"start_date"`
	EndDate    string   `json:"end_date"`
	Timezone   string   `json:"timezone"`
	Dimensions []string `json:"dimensions"`
	StatusIds  []int    `json:"status_ids"`
	RoleIds    []int    `json:"role_ids"`
	MemberIds  []int    `json:"member_ids"`
}

type reportOptionResponse struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type reportDimensionResponse struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type reportPageResponse[T any] struct {
	List  []T `json:"list"`
	Total int `json:"total"`
	Page  int `json:"page"`
	Size  int `json:"size"`
}

type reportOptionsResponse struct {
	Dimensions []reportDimensionResponse                `json:"dimensions"`
	Statuses   []reportOptionResponse                   `json:"statuses"`
	Priorities []reportOptionResponse                   `json:"priorities"`
	Roles      []reportOptionResponse                   `json:"roles"`
	Members    reportPageResponse[reportOptionResponse] `json:"members"`
}

type reportRunIdResponse struct {
	Id string `json:"id"`
}

type reportHistoryResponse struct {
	SnapshotVersion int                       `json:"snapshot_version"`
	Id              string                    `json:"id"`
	Creator         string                    `json:"creator"`
	CreatedAt       int64                     `json:"created_at"`
	Start           int64                     `json:"period_start_at"`
	End             int64                     `json:"period_end_at"`
	Timezone        string                    `json:"timezone"`
	Dimensions      []reportDimensionResponse `json:"dimensions"`
	Statuses        []reportOptionResponse    `json:"statuses"`
	Roles           []reportOptionResponse    `json:"roles"`
	Members         []reportOptionResponse    `json:"members"`
}

type reportRowResponse struct {
	Key       string   `json:"key"`
	Dimension string   `json:"dimension"`
	Name      string   `json:"name"`
	Count     int      `json:"count"`
	Hours     *float64 `json:"hours"`
	Values    []string `json:"values,omitempty"`
}

type reportResultResponse struct {
	Report reportHistoryResponse `json:"report"`
	Rows   []reportRowResponse   `json:"rows"`
}

type reportItemResponse struct {
	StatusChangedAt  int64    `json:"status_changed_at,omitempty"`
	CountedHours     *float64 `json:"counted_hours"`
	CountedAssignees []string `json:"counted_assignees,omitempty"`
	Id               int      `json:"id"`
	Title            string   `json:"title"`
	Status           string   `json:"status"`
	Template         string   `json:"template"`
	Iteration        string   `json:"iteration"`
	Category         string   `json:"category"`
	Priority         string   `json:"priority"`
	Severity         string   `json:"severity"`
	Estimated        float64  `json:"estimated"`
	Assignees        string   `json:"assignees"`
	ResponsibleNames []string `json:"responsible_names"`
	CreatedAt        int64    `json:"created_at"`
}

func reportFailure(c *gin.Context, err error) {
	lang := lib_i18n.GetLang(c)
	switch {
	case errors.Is(err, business.ErrReportNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgReportNotFound))
	case errors.Is(err, business.ErrReportInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "report.input", err)
	case errors.Is(err, business.ErrReportConflict):
		abortConflict(c, lib_i18n.Show(lang, define.MsgReportConflict))
	case errors.Is(err, business.ErrReportTooLarge):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgReportTooLarge), "report.size", err)
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), "report.request", err)
	}
}

func reportScope(c *gin.Context) (business.ReportScope, bool) {
	scope := business.ReportScope{AdminId: c.GetInt("adminId"), UserId: c.GetInt("userId"), Type: c.Param("reportType")}
	wsId, err := strconv.Atoi(c.Param("wsId"))
	if err != nil || wsId <= 0 || (scope.Type != business.ReportWorkHours && scope.Type != business.ReportDefects) {
		reportFailure(c, business.ErrReportInvalid)
		return scope, false
	}
	scope.WorkspaceId = wsId
	lang := lib_i18n.GetLang(c)
	enabled, err := business.IsWorkspaceApplicationEnabled(scope.AdminId, scope.WorkspaceId, define.AppKeyReport)
	if err != nil {
		reportFailure(c, err)
		return scope, false
	}
	if !enabled || !business.HasPermission(scope.AdminId, scope.UserId, strconv.Itoa(wsId), business.ReportView) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return scope, false
	}
	if scope.Type == business.ReportWorkHours &&
		business.HasPermission(scope.AdminId, scope.UserId, strconv.Itoa(wsId), "requirement:view") {
		scope.Types = append(scope.Types, define.TypeKeyRequirement)
	}
	enabled, err = business.IsWorkspaceApplicationEnabled(scope.AdminId, scope.WorkspaceId, define.AppKeyDefect)
	if err != nil {
		reportFailure(c, err)
		return scope, false
	}
	if enabled && business.HasPermission(scope.AdminId, scope.UserId, strconv.Itoa(wsId), "defect:view") {
		scope.Types = append(scope.Types, define.TypeKeyDefect)
	}
	if len(scope.Types) == 0 {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return scope, false
	}
	return scope, true
}

type reportPageQuery struct {
	Page int `form:"page"`
	Size int `form:"size"`
}

func reportPage(c *gin.Context, defaultSize int) (reportPageQuery, bool) {
	page := reportPageQuery{Page: 1, Size: defaultSize}
	if err := c.ShouldBindQuery(&page); err != nil || page.Page < 1 || page.Page > 100000 || page.Size < 1 || page.Size > 100 {
		reportFailure(c, business.ErrReportInvalid)
		return reportPageQuery{}, false
	}
	return page, true
}

func reportRecordId(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("runId"), 10, 64)
	if err != nil || id <= 0 {
		reportFailure(c, business.ErrReportInvalid)
		return 0, false
	}
	return id, true
}

func reportSuccess(c *gin.Context, data any) {
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.ShowByContext(c, define.MsgSuccess), Data: data})
}

func reportOptionsForResponse(lang string, options []business.ReportOption) []reportOptionResponse {
	response := make([]reportOptionResponse, 0, len(options))
	for _, option := range options {
		response = append(response, reportOptionResponse{Id: option.Id, Name: lib_i18n.ShowStored(lang, option.Name)})
	}
	return response
}

func reportDimensionsForResponse(lang string, keys []string) []reportDimensionResponse {
	response := make([]reportDimensionResponse, 0, len(keys))
	for _, key := range keys {
		response = append(response, reportDimensionResponse{Key: key, Name: business.ReportDimensionName(lang, key)})
	}
	return response
}

func reportHistoryForResponse(lang string, record business.ReportRecord) reportHistoryResponse {
	var id string
	if record.Id > 0 {
		id = strconv.FormatInt(record.Id, 10)
	}
	return reportHistoryResponse{SnapshotVersion: record.SnapshotVersion, Id: id, Creator: record.Creator, CreatedAt: record.CreatedAt, Start: record.Start, End: record.End, Timezone: record.Timezone,
		Dimensions: reportDimensionsForResponse(lang, record.Filters.Dimensions), Statuses: reportOptionsForResponse(lang, record.Filters.Statuses),
		Roles: reportOptionsForResponse(lang, record.Filters.Roles), Members: reportOptionsForResponse(lang, record.Filters.Members)}
}

func reportRowForResponse(lang string, row business.ReportRow, dimensions []string) reportRowResponse {
	name := lib_i18n.ShowStored(lang, row.Name)
	switch {
	case row.Key == "responsible:-1":
		name = lib_i18n.Show(lang, define.MsgReportUnknownResponsible)
	case row.Key == "responsible:0":
		name = lib_i18n.Show(lang, define.MsgReportNoResponsible)
	case row.Key == row.Dimension+":-1":
		name = lib_i18n.Show(lang, define.MsgReportUnknownAssignees)
	case (row.Dimension == "person" || row.Dimension == "requirement" || row.Dimension == "defect") && row.Key == row.Dimension+":0":
		name = lib_i18n.Show(lang, define.MsgReportNoAssignee)
	case name == "":
		name = lib_i18n.Show(lang, define.MsgReportUnassigned)
	}
	result := reportRowResponse{Key: row.Key, Dimension: row.Dimension, Name: name, Count: row.Count}
	for index, value := range row.Values {
		label := lib_i18n.ShowStored(lang, value.Name)
		switch {
		case dimensions[index] == "responsible" && value.Id == -1:
			label = lib_i18n.Show(lang, define.MsgReportUnknownResponsible)
		case dimensions[index] == "responsible" && value.Id == 0:
			label = lib_i18n.Show(lang, define.MsgReportNoResponsible)
		case dimensions[index] == "category" && value.Id == 0:
			label = lib_i18n.Show(lang, "report_uncategorized")
		case label == "":
			label = lib_i18n.Show(lang, define.MsgReportUnassigned)
		}
		result.Values = append(result.Values, label)
	}
	if len(result.Values) > 0 {
		result.Name = strings.Join(result.Values, " / ")
	}
	if row.Hundredths != nil {
		hours := float64(*row.Hundredths) / 100
		result.Hours = &hours
	}
	return result
}

type reportOptionsQuery struct {
	Search   string `form:"search"`
	ItemType string `form:"item_type"`
	RoleIds  string `form:"role_ids"`
}

// GetReportOptions returns the report filter choices allowed in the workspace.
func GetReportOptions(c *gin.Context) {
	scope, ok := reportScope(c)
	if !ok {
		return
	}
	page, ok := reportPage(c, 50)
	if !ok {
		return
	}
	var query reportOptionsQuery
	if err := c.ShouldBindQuery(&query); err != nil || len(query.Search) > 200 {
		reportFailure(c, business.ErrReportInvalid)
		return
	}
	roleIds, err := parseIntegerIDList(query.RoleIds, false)
	if err != nil {
		reportFailure(c, business.ErrReportInvalid)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	options, err := business.GetReportOptions(ctx, business.GetReportOptionsModel{ReportScope: scope, Search: query.Search, ItemType: query.ItemType, Page: page.Page, Size: page.Size, RoleIds: roleIds})
	if err != nil {
		reportFailure(c, err)
		return
	}
	lang := lib_i18n.GetLang(c)
	reportSuccess(c, reportOptionsResponse{
		Dimensions: reportDimensionsForResponse(lang, scope.Dimensions()),
		Statuses:   reportOptionsForResponse(lang, options.Statuses), Priorities: reportOptionsForResponse(lang, options.Priorities), Roles: reportOptionsForResponse(lang, options.Roles),
		Members: reportPageResponse[reportOptionResponse]{List: reportOptionsForResponse(lang, options.Members), Total: options.MemberTotal, Page: page.Page, Size: page.Size},
	})
}

// GenerateReport creates an immutable report and returns its record ID.
func GenerateReport(c *gin.Context) {
	scope, ok := reportScope(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32*1024)
	var req generateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		reportFailure(c, business.ErrReportInvalid)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
	defer cancel()
	id, err := business.GenerateWorkspaceReport(ctx, business.GenerateReportModel{
		ReportScope: scope, RequestId: req.RequestId, Preset: req.Preset, StartDate: req.StartDate, EndDate: req.EndDate, Timezone: req.Timezone,
		Dimensions: req.Dimensions, StatusIds: req.StatusIds, RoleIds: req.RoleIds, MemberIds: req.MemberIds,
	})
	if err != nil {
		reportFailure(c, err)
		return
	}
	reportSuccess(c, reportRunIdResponse{Id: strconv.FormatInt(id, 10)})
}

// ListReports returns authorized historical report runs.
func ListReports(c *gin.Context) {
	scope, ok := reportScope(c)
	if !ok {
		return
	}
	page, ok := reportPage(c, 20)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	records, total, err := business.ListWorkspaceReports(ctx, scope, page.Page, page.Size)
	if err != nil {
		reportFailure(c, err)
		return
	}
	lang := lib_i18n.GetLang(c)
	items := make([]reportHistoryResponse, 0, len(records))
	for _, record := range records {
		items = append(items, reportHistoryForResponse(lang, record))
	}
	reportSuccess(c, reportPageResponse[reportHistoryResponse]{List: items, Total: total, Page: page.Page, Size: page.Size})
}

func readReportResult(c *gin.Context) (business.ReportScope, business.ReportRecord, bool) {
	scope, ok := reportScope(c)
	if !ok {
		return scope, business.ReportRecord{}, false
	}
	id, ok := reportRecordId(c)
	if !ok {
		return scope, business.ReportRecord{}, false
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	record, err := business.GetWorkspaceReport(ctx, scope, id)
	if err != nil {
		reportFailure(c, err)
		return scope, business.ReportRecord{}, false
	}
	return scope, record, true
}

func readReportPreview(c *gin.Context) (business.ReportScope, business.ReportRecord, bool) {
	scope, ok := reportScope(c)
	if !ok {
		return scope, business.ReportRecord{}, false
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
	defer cancel()
	record, err := business.PreviewWorkspaceReport(ctx, scope, c.Query("timezone"))
	if err != nil {
		reportFailure(c, err)
		return scope, business.ReportRecord{}, false
	}
	return scope, record, true
}

// GetReportPreview returns the default report without creating a history record.
func GetReportPreview(c *gin.Context) {
	_, record, ok := readReportPreview(c)
	if ok {
		c.Header("Cache-Control", "private, no-store")
		sendReportResult(c, record)
	}
}

// GetReport returns a saved report's summary.
func GetReport(c *gin.Context) {
	_, record, ok := readReportResult(c)
	if ok {
		sendReportResult(c, record)
	}
}

func sendReportResult(c *gin.Context, record business.ReportRecord) {
	lang := lib_i18n.GetLang(c)
	rows := make([]reportRowResponse, 0, len(record.Rows))
	for _, row := range record.Rows {
		rows = append(rows, reportRowForResponse(lang, row, record.Filters.Dimensions))
	}
	reportSuccess(c, reportResultResponse{Report: reportHistoryForResponse(lang, record), Rows: rows})
}

type reportItemQuery struct {
	Group          string `form:"group"`
	Search         string `form:"search"`
	Status         int    `form:"status"`
	Priority       int    `form:"priority"`
	Assignee       string `form:"assignee"`
	AssigneeIds    string `form:"assignee_ids"`
	Iteration      string `form:"iteration"`
	StatusIds      string `form:"status_ids"`
	PriorityIds    string `form:"priority_ids"`
	IterationNames string `form:"iteration_names"`
	ItemType       string `form:"item_type"`
}

// ListReportItems returns the paginated snapshot for a report group.
func ListReportItems(c *gin.Context) {
	listReportItems(c, false)
}

// ListReportPreviewItems returns live details for a default report group.
func ListReportPreviewItems(c *gin.Context) {
	listReportItems(c, true)
}

func listReportItems(c *gin.Context, preview bool) {
	scope, ok := reportScope(c)
	if !ok {
		return
	}
	var id int64
	if !preview {
		id, ok = reportRecordId(c)
		if !ok {
			return
		}
	}
	page, ok := reportPage(c, 20)
	if !ok {
		return
	}
	var filters reportItemQuery
	if err := c.ShouldBindQuery(&filters); err != nil || len(filters.Group) == 0 || len(filters.Group) > 200 || len(filters.Search) > 500 || filters.Priority < 0 || len(filters.Assignee) > 100 || len(filters.Iteration) > 200 || filters.Status < 0 {
		reportFailure(c, business.ErrReportInvalid)
		return
	}
	assigneeIds, err := parseIntegerIDList(filters.AssigneeIds, false)
	if err != nil {
		reportFailure(c, business.ErrReportInvalid)
		return
	}
	priorityIds, err := parseIntegerIDList(filters.PriorityIds, true)
	if err != nil {
		reportFailure(c, business.ErrReportInvalid)
		return
	}
	var statusValues, iterationNames []string
	if filters.StatusIds != "" {
		statusValues = strings.Split(filters.StatusIds, ",")
	}
	if filters.IterationNames != "" && (len(filters.IterationNames) > 12000 || json.Unmarshal([]byte(filters.IterationNames), &iterationNames) != nil) {
		reportFailure(c, business.ErrReportInvalid)
		return
	}
	timeout := 10 * time.Second
	if preview {
		timeout = 25 * time.Second
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()
	model := business.ListReportItemsModel{ReportScope: scope, Id: id, Group: filters.Group, Search: filters.Search, Status: filters.Status,
		Priority: filters.Priority, Assignee: filters.Assignee, Iteration: filters.Iteration, Page: page.Page, Size: page.Size, AssigneeIds: assigneeIds,
		StatusValues: statusValues, PriorityIds: priorityIds, IterationNames: iterationNames, ItemType: filters.ItemType}
	var items []business.ReportItem
	var total int
	if preview {
		c.Header("Cache-Control", "private, no-store")
		items, total, err = business.ListWorkspaceReportPreviewItems(ctx, model, c.Query("timezone"))
	} else {
		items, total, err = business.ListWorkspaceReportItems(ctx, model)
	}
	if err != nil {
		reportFailure(c, err)
		return
	}
	lang := lib_i18n.GetLang(c)
	response := make([]reportItemResponse, 0, len(items))
	for _, item := range items {
		status := lib_i18n.ShowStored(lang, item.Status)
		if item.StatusChangedAt > 0 && item.StatusId == 0 {
			status = lib_i18n.Show(lang, define.MsgReportUnknownStatus)
		}
		var countedHours *float64
		if item.CountedHundredths != nil {
			hours := float64(*item.CountedHundredths) / 100
			countedHours = &hours
		}
		response = append(response, reportItemResponse{Id: item.Id, Title: item.Title, Status: status,
			StatusChangedAt: item.StatusChangedAt, CountedHours: countedHours, CountedAssignees: item.CountedAssignees,
			Template: item.Template, Iteration: item.Iteration, Category: item.Category, Priority: lib_i18n.ShowStored(lang, item.Priority),
			Severity: lib_i18n.ShowStored(lang, item.Severity), Estimated: item.Estimated, Assignees: item.Assignees, ResponsibleNames: item.ResponsibleNames, CreatedAt: item.CreatedAt})
	}
	reportSuccess(c, reportPageResponse[reportItemResponse]{List: response, Total: total, Page: page.Page, Size: page.Size})
}

// ExportReport downloads the saved summary as an XLSX workbook.
func ExportReport(c *gin.Context) {
	scope, record, ok := readReportResult(c)
	if ok {
		writeReportWorkbook(c, scope, record)
	}
}

// ExportReportPreview downloads the live report summary without writing history.
func ExportReportPreview(c *gin.Context) {
	scope, record, ok := readReportPreview(c)
	if ok {
		writeReportWorkbook(c, scope, record)
	}
}

func writeReportWorkbook(c *gin.Context, scope business.ReportScope, record business.ReportRecord) {
	lang := lib_i18n.GetLang(c)
	workbook := excelize.NewFile()
	defer func() {
		if err := workbook.Close(); err != nil {
			logs.Error("close report workbook: %s", err.Error())
		}
	}()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	dimensions := record.Filters.Dimensions
	countsOnly := scope.Type == business.ReportDefects
	combinedCounts := countsOnly && record.SnapshotVersion >= business.ReportStatisticsSnapshotVersion
	if combinedCounts {
		dimensions = []string{business.ReportDefects}
	}
	for index, dimension := range dimensions {
		sheetName := business.ReportDimensionName(lang, dimension)
		if combinedCounts {
			sheetName = lib_i18n.Show(lang, "report_defects")
		}
		var err error
		if index == 0 {
			err = workbook.SetSheetName(workbook.GetSheetName(0), sheetName)
		} else {
			_, err = workbook.NewSheet(sheetName)
		}
		if err != nil {
			reportFailure(c, err)
			return
		}
		writer, err := workbook.NewStreamWriter(sheetName)
		if err != nil {
			reportFailure(c, err)
			return
		}
		countKey := "report_requirement_count"
		if scope.Type == business.ReportDefects {
			countKey = "report_defect_count"
		}
		hoursKey := define.MsgReportTotalHours
		if record.SnapshotVersion >= business.ReportEstimateSnapshotVersion {
			hoursKey = define.MsgReportEstimatedTotal
		} else if scope.Type == business.ReportWorkHours {
			hoursKey = define.MsgReportLoggedHours
		}
		header := []any{lib_i18n.Show(lang, define.MsgReportGroup), lib_i18n.Show(lang, countKey), lib_i18n.Show(lang, hoursKey)}
		if scope.Type == business.ReportWorkHours && record.SnapshotVersion >= business.ReportStatisticsSnapshotVersion {
			header[1] = lib_i18n.Show(lang, "report_work_item_count")
		}
		if combinedCounts {
			header = nil
			for _, key := range record.Filters.Dimensions {
				header = append(header, business.ReportDimensionName(lang, key))
			}
			header = append(header, lib_i18n.Show(lang, "report_defect_count"))
		}
		if countsOnly && !combinedCounts {
			header = header[:2]
		}
		err = writer.SetRow("A1", header)
		if err != nil {
			reportFailure(c, err)
			return
		}
		i := 2
		for _, row := range record.Rows {
			if row.Dimension != dimension {
				continue
			}
			if err = ctx.Err(); err != nil {
				reportFailure(c, err)
				return
			}
			r := reportRowForResponse(lang, row, record.Filters.Dimensions)
			var hours any = "—"
			if r.Hours != nil {
				hours = *r.Hours
			}
			values := []any{r.Name, r.Count, hours}
			if combinedCounts {
				values = make([]any, 0, len(r.Values)+1)
				for _, value := range r.Values {
					values = append(values, value)
				}
				values = append(values, r.Count)
			}
			if countsOnly && !combinedCounts {
				values = values[:2]
			}
			if err = writer.SetRow("A"+strconv.Itoa(i), values); err != nil {
				reportFailure(c, err)
				return
			}
			i++
		}
		if err = writer.Flush(); err != nil {
			reportFailure(c, err)
			return
		}
	}
	tz, err := time.LoadLocation(record.Timezone)
	if err != nil {
		reportFailure(c, err)
		return
	}
	titleKey := "report_work_hours"
	if scope.Type == business.ReportDefects {
		titleKey = "report_defects"
	}
	name := fmt.Sprintf("%s_%s_%s.xlsx", lib_i18n.Show(lang, titleKey),
		time.Unix(record.Start, 0).In(tz).Format(time.DateOnly), time.Unix(record.End-1, 0).In(tz).Format(time.DateOnly))
	content, err := workbook.WriteToBuffer()
	if err != nil {
		reportFailure(c, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(name))
	c.Header("Cache-Control", "private, no-store")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content.Bytes())
}

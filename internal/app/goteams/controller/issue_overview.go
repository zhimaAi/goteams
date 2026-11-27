package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
)

type issueOverviewQuery struct {
	Preset          string `form:"preset"`
	From            string `form:"from"`
	To              string `form:"to"`
	StatusId        *int   `form:"status_id"`
	PriorityId      *int   `form:"priority_id"`
	CategoryId      *int   `form:"category_id"`
	Metric          string `form:"metric"`
	PartId          *int   `form:"part_id"`
	Search          string `form:"search"`
	DetailStatusIds string `form:"detail_status_ids"`
	Page            int    `form:"page"`
	Size            int    `form:"size"`
}
type issueOverviewCountsResponse struct {
	Total      int64 `json:"total"`
	Finished   int64 `json:"finished"`
	Pending    int64 `json:"pending"`
	Processing int64 `json:"processing"`
	Completed  int64 `json:"completed"`
	Closed     int64 `json:"closed"`
	Unknown    int64 `json:"unknown"`
}
type issueOverviewGroupResponse struct {
	Id       int    `json:"id"`
	Key      string `json:"key"`
	Name     string `json:"name"`
	Color    string `json:"color"`
	Total    int64  `json:"total"`
	Finished int64  `json:"finished"`
}
type issueOverviewDayResponse struct {
	Date      string `json:"date"`
	Created   int64  `json:"created"`
	Completed int64  `json:"completed"`
	Closed    int64  `json:"closed"`
}
type issueOverviewStageResponse struct {
	Id             int      `json:"id"`
	Name           string   `json:"name"`
	Samples        int64    `json:"samples"`
	AverageSeconds *float64 `json:"average_seconds"`
}
type issueOverviewResponse struct {
	PendingPriorities     []issueOverviewGroupResponse `json:"pending_priorities"`
	EventHistoryUnknown   int64                        `json:"event_history_unknown"`
	EvaluatedAt           int64                        `json:"evaluated_at"`
	Timezone              string                       `json:"timezone"`
	From                  string                       `json:"from"`
	To                    string                       `json:"to"`
	InitialStatusId       int                          `json:"initial_status_id"`
	CompletedStatusIds    []int                        `json:"completed_status_ids"`
	ClosedStatusIds       []int                        `json:"closed_status_ids"`
	All                   issueOverviewCountsResponse  `json:"all"`
	Current               issueOverviewCountsResponse  `json:"current"`
	Completed             int64                        `json:"completed"`
	PreviousCompleted     int64                        `json:"previous_completed"`
	Overdue               int64                        `json:"overdue"`
	PreviousOverdue       int64                        `json:"previous_overdue"`
	SLAEnabled            bool                         `json:"sla_enabled"`
	SLAAchieved           int64                        `json:"sla_achieved"`
	SLAEligible           int64                        `json:"sla_eligible"`
	SLARate               *float64                     `json:"sla_rate"`
	HistoryEligible       int64                        `json:"history_eligible"`
	FirstResolved         int64                        `json:"first_resolved"`
	HistoryUnknown        int64                        `json:"history_unknown"`
	FirstResolutionRate   *float64                     `json:"first_resolution_rate"`
	ResolutionSuccessRate *float64                     `json:"resolution_success_rate"`
	Categories            []issueOverviewGroupResponse `json:"categories"`
	Priorities            []issueOverviewGroupResponse `json:"priorities"`
	Statuses              []issueOverviewGroupResponse `json:"statuses"`
	Members               []issueOverviewGroupResponse `json:"members"`
	Trend                 []issueOverviewDayResponse   `json:"trend"`
	Stages                []issueOverviewStageResponse `json:"stages"`
}
type issueOverviewItemResponse struct {
	Id            int64  `json:"id"`
	Title         string `json:"title"`
	StatusId      int    `json:"status_id"`
	PriorityId    int    `json:"priority_id"`
	CategoryId    int    `json:"category_id"`
	AssigneeIds   []int  `json:"assignee_ids"`
	CreatedAt     int64  `json:"created_at"`
	EventAt       int64  `json:"event_at"`
	EventStatusId int    `json:"event_status_id"`
}
type issueOverviewItemsResponse struct {
	List  []issueOverviewItemResponse `json:"list"`
	Total int                         `json:"total"`
	Page  int                         `json:"page"`
	Size  int                         `json:"size"`
}

func overviewCountsResponse(value business.IssueOverviewCountsModel) issueOverviewCountsResponse {
	return issueOverviewCountsResponse{Total: value.Total, Finished: value.Finished, Pending: value.Pending, Processing: value.Processing, Completed: value.Completed, Closed: value.Closed, Unknown: value.Unknown}
}
func overviewGroupsResponse(values []business.IssueOverviewGroupModel, lang string) []issueOverviewGroupResponse {
	result := make([]issueOverviewGroupResponse, 0, len(values))
	for _, v := range values {
		result = append(result, issueOverviewGroupResponse{Id: v.Id, Key: v.Key, Name: lib_i18n.ShowStored(lang, v.Name), Color: v.Color, Total: v.Total, Finished: v.Finished})
	}
	return result
}
func overviewResponseRate(count, total int64) *float64 {
	if total == 0 {
		return nil
	}
	value := float64(count) * 100 / float64(total)
	return &value
}

// GetIssueOverview 返回独立于SLA启停的工单概览。
func GetIssueOverview(c *gin.Context) { serveIssueOverview(c, false) }

// ListIssueOverviewItems 返回概览数字对应的实时分页明细。
func ListIssueOverviewItems(c *gin.Context) { serveIssueOverview(c, true) }
func serveIssueOverview(c *gin.Context, details bool) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, "issue.overview")
	if !ok {
		return
	}
	var req issueOverviewQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "issue.overview.bind", err)
		return
	}
	if req.Preset == "" {
		req.Preset = "week"
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Size == 0 {
		req.Size = 20
	}
	if details && req.Metric == "" || !details && (req.Metric != "" || req.PartId != nil || req.Search != "" || req.DetailStatusIds != "") || len(req.DetailStatusIds) > 1000 {
		issueStatisticsError(c, lang, business.ErrDashboardInvalid)
		return
	}
	ids := []int{}
	if req.DetailStatusIds != "" {
		for _, raw := range strings.Split(req.DetailStatusIds, ",") {
			id, err := strconv.Atoi(raw)
			if err != nil || id <= 0 {
				issueStatisticsError(c, lang, business.ErrDashboardInvalid)
				return
			}
			ids = append(ids, id)
		}
	}
	result, err := business.QueryIssueOverview(c.Request.Context(), business.IssueOverviewQueryModel{AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId, Lang: lang, Preset: req.Preset, From: req.From, To: req.To, StatusId: req.StatusId, PriorityId: req.PriorityId, CategoryId: req.CategoryId, Metric: req.Metric, PartId: req.PartId, Search: strings.TrimSpace(req.Search), DetailStatusIds: ids, Page: req.Page, Size: req.Size})
	if err != nil {
		issueStatisticsError(c, lang, err)
		return
	}
	if details {
		data := issueOverviewItemsResponse{List: make([]issueOverviewItemResponse, 0, len(result.List)), Total: result.Total, Page: result.Page, Size: result.Size}
		for _, v := range result.List {
			data.List = append(data.List, issueOverviewItemResponse{Id: v.Id, Title: v.Title, StatusId: v.StatusId, PriorityId: v.PriorityId, CategoryId: v.CategoryId, AssigneeIds: append([]int{}, v.AssigneeIds...), CreatedAt: v.CreatedAt, EventAt: v.EventAt, EventStatusId: v.EventStatusId})
		}
		c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: data})
		return
	}
	data := issueOverviewResponse{PendingPriorities: overviewGroupsResponse(result.PendingPriorities, lang), EventHistoryUnknown: result.EventHistoryUnknown, EvaluatedAt: result.EvaluatedAt, Timezone: result.Timezone, From: result.From, To: result.To, InitialStatusId: result.InitialStatusId, CompletedStatusIds: append([]int{}, result.CompletedStatusIds...), ClosedStatusIds: append([]int{}, result.ClosedStatusIds...), All: overviewCountsResponse(result.All), Current: overviewCountsResponse(result.Current), Completed: result.Completed, PreviousCompleted: result.PreviousCompleted, Overdue: result.Overdue, PreviousOverdue: result.PreviousOverdue, SLAEnabled: result.SLAEnabled, SLAAchieved: result.Achieved, SLAEligible: result.Eligible, HistoryEligible: result.HistoryEligible, FirstResolved: result.FirstResolved, HistoryUnknown: result.HistoryUnknown, FirstResolutionRate: overviewResponseRate(result.FirstResolved, result.HistoryEligible), ResolutionSuccessRate: overviewResponseRate(result.Current.Completed, result.Current.Finished), Categories: overviewGroupsResponse(result.Categories, lang), Priorities: overviewGroupsResponse(result.Priorities, lang), Statuses: overviewGroupsResponse(result.Statuses, lang), Members: overviewGroupsResponse(result.Members, lang), Trend: make([]issueOverviewDayResponse, 0, len(result.Trend)), Stages: make([]issueOverviewStageResponse, 0, len(result.Stages))}
	if result.SLAEnabled {
		data.SLARate = overviewResponseRate(result.Achieved, result.Eligible)
	}
	for _, v := range result.Trend {
		data.Trend = append(data.Trend, issueOverviewDayResponse{Date: v.Date, Created: v.Created, Completed: v.Completed, Closed: v.Closed})
	}
	for _, v := range result.Stages {
		data.Stages = append(data.Stages, issueOverviewStageResponse{Id: v.Id, Name: lib_i18n.ShowStored(lang, v.Name), Samples: v.Samples, AverageSeconds: v.AverageSeconds})
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: data})
}

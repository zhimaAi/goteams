package controller

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"
	"net/http"
)

type dashboardChartQueryRequest struct {
	CardId         int                             `json:"card_id"`
	Config         *json.RawMessage                `json:"config"`
	Timezone       string                          `json:"timezone"`
	WorkloadFilter *dashboardWorkloadFilterRequest `json:"workload_filter"`
	CategoryFilter *dashboardCategoryFilterRequest `json:"category_filter"`
	SLARiskFilter  *dashboardSLARiskFilterRequest  `json:"sla_risk_filter"`
}
type dashboardCategoryFilterRequest struct {
	From       int64 `json:"from"`
	To         int64 `json:"to"`
	AssigneeId int   `json:"assignee_id"`
	PriorityId *int  `json:"priority_id"`
}

type dashboardWorkloadFilterRequest struct {
	From       int64 `json:"from"`
	To         int64 `json:"to"`
	AssigneeId int   `json:"assignee_id"`
	PriorityId *int  `json:"priority_id"`
	CategoryId *int  `json:"category_id"`
}

type dashboardWorkloadRowResponse struct {
	Group            string `json:"group"`
	Created          int64  `json:"created"`
	Resolved         int64  `json:"resolved"`
	Closed           int64  `json:"closed"`
	InProgress       int64  `json:"in_progress"`
	PendingAssign    int64  `json:"pending_assign"`
	PendingProcess   int64  `json:"pending_process"`
	Assigned         int64  `json:"assigned"`
	AssignedResolved int64  `json:"assigned_resolved"`
	AssignedClosed   int64  `json:"assigned_closed"`
}

type dashboardCategoryStatResponse struct {
	Id             int    `json:"id"`
	ParentId       int    `json:"parent_id"`
	Name           string `json:"name"`
	Created        int64  `json:"created"`
	Resolved       int64  `json:"resolved"`
	Closed         int64  `json:"closed"`
	InProgress     int64  `json:"in_progress"`
	PendingAssign  int64  `json:"pending_assign"`
	PendingProcess int64  `json:"pending_process"`
}

type dashboardChartBucketResponse struct {
	Id        int    `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	Count     int64  `json:"count"`
	Timestamp int64  `json:"timestamp"`
}
type dashboardSLATrendDayResponse struct {
	Date     string   `json:"date"`
	Rate     *float64 `json:"rate"`
	Achieved int64    `json:"achieved"`
	Eligible int64    `json:"eligible"`
	Started  int64    `json:"started"`
	Overdue  int64    `json:"overdue"`
}

type dashboardSLATrendResponse struct {
	Enabled     bool                           `json:"enabled"`
	EvaluatedAt int64                          `json:"evaluated_at"`
	Days        []dashboardSLATrendDayResponse `json:"days"`
}

type dashboardSLADimensionSeriesResponse struct {
	StageId    int      `json:"stage_id,omitempty"`
	StageName  string   `json:"stage_name,omitempty"`
	MetricType string   `json:"metric_type"`
	Achieved   int64    `json:"achieved"`
	Eligible   int64    `json:"eligible"`
	Rate       *float64 `json:"rate"`
}

type dashboardSLADimensionGroupResponse struct {
	Id     int                                   `json:"id"`
	Name   string                                `json:"name"`
	Series []dashboardSLADimensionSeriesResponse `json:"series"`
}

type dashboardSLADimensionResponse struct {
	Enabled     bool                                 `json:"enabled"`
	EvaluatedAt int64                                `json:"evaluated_at"`
	Groups      []dashboardSLADimensionGroupResponse `json:"groups"`
}

type dashboardChartResponse struct {
	SLADistribution *dashboardSLADistributionResponse `json:"sla_distribution,omitempty"`
	SLARisk         *dashboardSLARiskResponse         `json:"sla_risk,omitempty"`
	SLADimension    *dashboardSLADimensionResponse    `json:"sla_dimension,omitempty"`
	SLATrend        *dashboardSLATrendResponse        `json:"sla_trend,omitempty"`
	Buckets         []dashboardChartBucketResponse    `json:"buckets"`
	UndatedCount    int64                             `json:"undated_count"`
	Workload        []dashboardWorkloadRowResponse    `json:"workload,omitempty"`
	Categories      *[]dashboardCategoryStatResponse  `json:"categories,omitempty"`
}

// QueryDashboardCharts returns typed chart aggregates after workspace and work-item authorization.
func QueryDashboardCharts(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, "dashboard.chart.query")
	if !ok {
		return
	}
	dashboardId, ok := dashboardPathID(c, lang, "dashboard.chart.query")
	if !ok {
		return
	}
	var req dashboardChartQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "dashboard.chart.bind", err)
		return
	}
	if req.CardId <= 0 && req.Config == nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "dashboard.chart.card", errInvalidID)
		return
	}
	var config *string
	if req.Config != nil {
		if err := validate.DashboardConfig(*req.Config); err != nil {
			dashboardError(c, lang, "dashboard.chart.config", err)
			return
		}
		raw := string(*req.Config)
		config = &raw
	}
	var categoryFilter *business.DashboardCategoryFilterModel
	if filter := req.CategoryFilter; filter != nil {
		categoryFilter = &business.DashboardCategoryFilterModel{From: filter.From, To: filter.To, AssigneeId: filter.AssigneeId, PriorityId: filter.PriorityId}
	}
	var workloadFilter *business.DashboardWorkloadFilterModel
	if filter := req.WorkloadFilter; filter != nil {
		workloadFilter = &business.DashboardWorkloadFilterModel{From: filter.From, To: filter.To, AssigneeId: filter.AssigneeId, PriorityId: filter.PriorityId, CategoryId: filter.CategoryId}
	}
	var riskFilter *business.DashboardSLARiskFilterModel
	if filter := req.SLARiskFilter; filter != nil {
		riskFilter = &business.DashboardSLARiskFilterModel{
			DashboardCategoryFilterModel: business.DashboardCategoryFilterModel{From: filter.From, To: filter.To, AssigneeId: filter.AssigneeId, PriorityId: filter.PriorityId},
			Page:                         filter.Page, Size: filter.Size,
		}
	}
	result, err := business.QueryDashboardChart(business.DashboardChartQueryModel{AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId, DashboardId: dashboardId, CardId: req.CardId, Timezone: req.Timezone, Lang: lang, PreviewConfig: config, CategoryFilter: categoryFilter, WorkloadFilter: workloadFilter, SLARiskFilter: riskFilter})
	if err != nil {
		dashboardError(c, lang, "dashboard.chart.query", err)
		return
	}
	response := dashboardChartResponse{Buckets: make([]dashboardChartBucketResponse, 0, len(result.Buckets)), UndatedCount: result.UndatedCount}
	response.SLADistribution = toDashboardSLADistributionResponse(result.SLADistribution)
	response.SLARisk = toDashboardSLARiskResponse(result.SLARisk)
	if trend := result.SLATrend; trend != nil {
		days := make([]dashboardSLATrendDayResponse, 0, len(trend.Trend))
		for _, point := range trend.Trend {
			days = append(days, dashboardSLATrendDayResponse{
				Date: point.Date, Rate: point.Value, Achieved: point.Achieved, Eligible: point.Eligible,
				Started: point.Started, Overdue: point.Overdue,
			})
		}
		response.SLATrend = &dashboardSLATrendResponse{Enabled: trend.Enabled, EvaluatedAt: trend.EvaluatedAt, Days: days}
	}
	if dimension := result.SLADimension; dimension != nil {
		groups := make([]dashboardSLADimensionGroupResponse, 0, len(dimension.Groups))
		for _, group := range dimension.Groups {
			series := make([]dashboardSLADimensionSeriesResponse, 0, len(group.Series))
			for _, item := range group.Series {
				series = append(series, dashboardSLADimensionSeriesResponse{
					MetricType: item.MetricType, StageId: item.StageId, StageName: item.StageName, Achieved: item.Achieved, Eligible: item.Eligible, Rate: item.Rate,
				})
			}
			groups = append(groups, dashboardSLADimensionGroupResponse{Id: group.Id, Name: group.Name, Series: series})
		}
		response.SLADimension = &dashboardSLADimensionResponse{
			Enabled: dimension.Enabled, EvaluatedAt: dimension.EvaluatedAt, Groups: groups,
		}
	}
	for _, b := range result.Buckets {
		response.Buckets = append(response.Buckets, dashboardChartBucketResponse{Id: b.Id, Type: b.Type, Name: b.Name, Color: b.Color, Count: b.Count, Timestamp: b.Timestamp})
	}
	for _, item := range result.Workload {
		response.Workload = append(response.Workload, dashboardWorkloadRowResponse{
			Group: item.Group, Created: item.Created, Resolved: item.Resolved, Closed: item.Closed,
			InProgress: item.InProgress, PendingAssign: item.PendingAssign, PendingProcess: item.PendingProcess,
			Assigned: item.Assigned, AssignedResolved: item.AssignedResolved, AssignedClosed: item.AssignedClosed,
		})
	}
	if result.Categories != nil {
		categories := make([]dashboardCategoryStatResponse, 0, len(result.Categories))
		for _, item := range result.Categories {
			categories = append(categories, dashboardCategoryStatResponse{
				Id: item.Id, ParentId: item.ParentId, Name: item.Name, Created: item.Created,
				Resolved: item.Resolved, Closed: item.Closed, InProgress: item.InProgress,
				PendingAssign: item.PendingAssign, PendingProcess: item.PendingProcess,
			})
		}
		response.Categories = &categories
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response})
}

type copyDashboardCardsRequest struct {
	TargetDashboardIds []int `json:"target_dashboard_ids"`
}
type copyDashboardCardsResponse struct {
	Count int `json:"count"`
}

// CopyDashboardCards atomically copies a saved statistical card to multiple editable dashboards.
func CopyDashboardCards(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, "dashboard.cards.copy")
	if !ok {
		return
	}
	dashboardId, ok := dashboardPathID(c, lang, "dashboard.cards.copy")
	if !ok {
		return
	}
	cardId, ok := dashboardCardPathID(c, lang, "dashboard.cards.copy")
	if !ok {
		return
	}
	var req copyDashboardCardsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "dashboard.cards.copy.bind", err)
		return
	}
	if len(req.TargetDashboardIds) > 100 || validate.DashboardOrder(req.TargetDashboardIds) != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "dashboard.cards.copy.targets", errInvalidID)
		return
	}
	result, err := business.CopyDashboardCards(business.CopyDashboardCardsModel{AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId, DashboardId: dashboardId, CardId: cardId, TargetDashboardIds: req.TargetDashboardIds})
	if err != nil {
		dashboardError(c, lang, "dashboard.cards.copy", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: copyDashboardCardsResponse{Count: len(result)}})
}

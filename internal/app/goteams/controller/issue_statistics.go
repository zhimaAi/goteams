package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"
)

type issueStatisticsMetadataResponse struct {
	slaMetadataResponse
	Enabled  bool                        `json:"enabled"`
	Timezone string                      `json:"timezone"`
	Stages   []dashboardSLAStageResponse `json:"stages"`
}

type issueStatisticsRequest struct {
	ThresholdMinutes *int   `json:"threshold_minutes"`
	Dimension        string `json:"dimension"`
	Preset           string `json:"preset"`
	From             string `json:"from"`
	To               string `json:"to"`
	AssigneeId       int    `json:"assignee_id"`
	PriorityId       *int   `json:"priority_id"`
	CategoryId       *int   `json:"category_id"`
	RoleIds          []int  `json:"role_ids"`
	Page             int    `json:"page"`
	Size             int    `json:"size"`
}

type issueStatisticsStageCountResponse struct {
	Achieved              int64    `json:"achieved"`
	Eligible              int64    `json:"eligible"`
	Rate                  *float64 `json:"rate"`
	AverageOverdueSeconds *float64 `json:"average_overdue_seconds"`
}

type issueStatisticsCountResponse struct {
	Total    int64                               `json:"total"`
	Achieved int64                               `json:"achieved"`
	Eligible int64                               `json:"eligible"`
	Overdue  int64                               `json:"overdue"`
	Rate     *float64                            `json:"rate"`
	Stages   []issueStatisticsStageCountResponse `json:"stages"`
}

type issueStatisticsDetailResponse struct {
	issueStatisticsCountResponse
	Key  string `json:"key"`
	Name string `json:"name"`
}

type issueStatisticsResponse struct {
	Enabled      bool                               `json:"enabled"`
	EvaluatedAt  int64                              `json:"evaluated_at"`
	Stages       []dashboardSLAStageResponse        `json:"stages"`
	Current      *issueStatisticsCountResponse      `json:"current,omitempty"`
	Previous     *issueStatisticsCountResponse      `json:"previous,omitempty"`
	Trend        *dashboardSLATrendResponse         `json:"sla_trend,omitempty"`
	Distribution *dashboardSLADistributionResponse  `json:"sla_distribution,omitempty"`
	Dimension    *dashboardSLADimensionResponse     `json:"sla_dimension,omitempty"`
	Risk         *dashboardSLARiskResponse          `json:"sla_risk,omitempty"`
	Sparklines   [][]dashboardSLATrendPointResponse `json:"sparklines,omitempty"`
	List         []issueStatisticsDetailResponse    `json:"list"`
	Page         int                                `json:"page"`
	Size         int                                `json:"size"`
	Total        int                                `json:"total"`
}

func issueStatisticsError(c *gin.Context, lang string, err error) {
	if errors.Is(err, business.ErrSLAWorkspace) || errors.Is(err, business.ErrDashboardForbidden) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}
	dashboardError(c, lang, "issue.statistics", err)
}

func toIssueStatisticsStages(stages []business.DashboardSLAStageModel) []dashboardSLAStageResponse {
	out := make([]dashboardSLAStageResponse, 0, len(stages))
	for _, stage := range stages {
		out = append(out, dashboardSLAStageResponse{Id: stage.Id, Name: stage.Name})
	}
	return out
}

// GetIssueStatisticsMetadata 返回只读统计所需的选项。
func GetIssueStatisticsMetadata(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, "issue.statistics.metadata")
	if !ok {
		return
	}
	model, err := business.GetIssueStatisticsMetadata(scope.AdminId, scope.WorkspaceId, scope.UserId)
	if err != nil {
		issueStatisticsError(c, lang, err)
		return
	}
	data := issueStatisticsMetadataResponse{
		slaMetadataResponse: slaMetadataResponse{
			Statuses: toSLAOptions(model.Options.Statuses, lang), Priorities: toSLAOptions(model.Options.Priorities, lang),
			Categories: toSLAOptions(model.Options.Categories, lang), Roles: toSLAOptions(model.Options.Roles, lang),
		},
		Enabled: model.Enabled, Timezone: model.Timezone, Stages: toIssueStatisticsStages(model.Stages),
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: data})
}

func toIssueStatisticsCount(count business.IssueStatisticsCountModel) issueStatisticsCountResponse {
	out := issueStatisticsCountResponse{Total: count.Total, Achieved: count.Achieved, Eligible: count.Eligible, Overdue: count.Overdue, Rate: count.Rate,
		Stages: make([]issueStatisticsStageCountResponse, 0, len(count.Stages))}
	for _, stage := range count.Stages {
		out.Stages = append(out.Stages, issueStatisticsStageCountResponse{Achieved: stage.Achieved, Eligible: stage.Eligible, Rate: stage.Rate, AverageOverdueSeconds: stage.AverageOverdueSeconds})
	}
	return out
}

// QueryIssueStatistics 查询独立统计页的固定模块。
func QueryIssueStatistics(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, "issue.statistics.query")
	if !ok {
		return
	}
	var req issueStatisticsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "issue.statistics.bind", err)
		return
	}
	if err := validate.IssueStatisticsThreshold(req.ThresholdMinutes); err != nil {
		issueStatisticsError(c, lang, err)
		return
	}
	view := c.Param("view")
	if req.Preset == "" {
		req.Preset = "week"
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Size == 0 {
		req.Size = 20
	}
	if err := validate.IssueStatisticsQuery(view, req.Dimension, req.Preset, req.From, req.To, req.AssigneeId, req.PriorityId, req.CategoryId, req.RoleIds, req.Page, req.Size); err != nil {
		issueStatisticsError(c, lang, err)
		return
	}
	result, err := business.QueryIssueStatistics(c.Request.Context(), business.IssueStatisticsQueryModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId, Lang: lang, View: view,
		ThresholdMinutes: req.ThresholdMinutes, Dimension: req.Dimension, Preset: req.Preset, From: req.From, To: req.To,
		AssigneeId: req.AssigneeId, PriorityId: req.PriorityId, CategoryId: req.CategoryId, RoleIds: req.RoleIds, Page: req.Page, Size: req.Size,
	})
	if err != nil {
		issueStatisticsError(c, lang, err)
		return
	}
	data := issueStatisticsResponse{
		Enabled: result.Enabled, EvaluatedAt: result.EvaluatedAt, Stages: toIssueStatisticsStages(result.Stages),
		Distribution: toDashboardSLADistributionResponse(result.Distribution), Dimension: toDashboardSLADimensionResponse(result.Dimension),
		Risk: toDashboardSLARiskResponse(result.Risk), List: make([]issueStatisticsDetailResponse, 0, len(result.Details)),
		Page: result.Page, Size: result.Size, Total: result.Total,
	}
	if view == "summary" {
		current, previous := toIssueStatisticsCount(result.Current), toIssueStatisticsCount(result.Previous)
		data.Current, data.Previous = &current, &previous
		trend := &dashboardSLATrendResponse{Enabled: result.Enabled, EvaluatedAt: result.EvaluatedAt, Days: make([]dashboardSLATrendDayResponse, 0, len(result.Trend))}
		for _, day := range result.Trend {
			trend.Days = append(trend.Days, dashboardSLATrendDayResponse{Date: day.Date, Rate: day.Value, Achieved: day.Achieved, Eligible: day.Eligible, Started: day.Started, Overdue: day.Overdue})
		}
		data.Trend = trend
		for _, series := range result.Sparklines {
			points := make([]dashboardSLATrendPointResponse, 0, len(series))
			for _, point := range series {
				points = append(points, dashboardSLATrendPointResponse{Date: point.Date, Value: point.Value})
			}
			data.Sparklines = append(data.Sparklines, points)
		}
	}
	for _, group := range result.Details {
		data.List = append(data.List, issueStatisticsDetailResponse{issueStatisticsCountResponse: toIssueStatisticsCount(group.IssueStatisticsCountModel), Key: group.Key, Name: group.Name})
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: data})
}

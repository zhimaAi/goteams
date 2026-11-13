package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
)

type dashboardSLAStageResponse struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type dashboardSLAOptionsResponse struct {
	Enabled  bool                        `json:"enabled"`
	Timezone string                      `json:"timezone"`
	Stages   []dashboardSLAStageResponse `json:"stages"`
}

// GetDashboardSLAOptions 返回有权编辑的仪表盘的阶段选择数据。
func GetDashboardSLAOptions(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, "dashboard.sla.options")
	if !ok {
		return
	}
	dashboardId, ok := dashboardPathID(c, lang, "dashboard.sla.options")
	if !ok {
		return
	}
	model, err := business.GetDashboardSLAOptions(business.GetDashboardModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId, DashboardId: dashboardId,
	})
	if err != nil {
		dashboardError(c, lang, "dashboard.sla.options", err)
		return
	}
	response := dashboardSLAOptionsResponse{
		Enabled: model.Enabled, Timezone: model.Timezone, Stages: make([]dashboardSLAStageResponse, 0, len(model.Stages)),
	}
	for _, stage := range model.Stages {
		response.Stages = append(response.Stages, dashboardSLAStageResponse{Id: stage.Id, Name: stage.Name})
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response})
}

// dashboardSLAStatsResponse 的空值区别于实际0%，原数字指标响应保持原结构。
type dashboardSLATrendPointResponse struct {
	Date  string   `json:"date"`
	Value *float64 `json:"value"`
}
type dashboardSLAStatsResponse struct {
	StageName        string                           `json:"stage_name,omitempty"`
	ThresholdSeconds int                              `json:"threshold_seconds,omitempty"`
	Started          int64                            `json:"started"`
	Overdue          int64                            `json:"overdue"`
	PreviousValue    *float64                         `json:"previous_value"`
	Change           *float64                         `json:"change"`
	Trend            []dashboardSLATrendPointResponse `json:"trend"`
	Enabled          bool                             `json:"enabled"`
	Achieved         int64                            `json:"achieved"`
	Eligible         int64                            `json:"eligible"`
	Unknown          int64                            `json:"unknown"`
	EvaluatedAt      int64                            `json:"evaluated_at"`
}

func toDashboardSLAStatsResponse(result business.DashboardMetricResultModel) *dashboardSLAStatsResponse {
	if result.SLA == nil {
		return nil
	}
	stats := result.SLA
	response := &dashboardSLAStatsResponse{Enabled: stats.Enabled, Achieved: stats.Achieved, Eligible: stats.Eligible, Unknown: stats.Unknown, EvaluatedAt: stats.EvaluatedAt}
	response.Started = stats.Started
	response.Overdue = stats.Overdue
	response.StageName = stats.StageName
	response.ThresholdSeconds = stats.ThresholdSeconds
	response.PreviousValue = stats.PreviousValue
	response.Change = stats.Change
	response.Trend = make([]dashboardSLATrendPointResponse, 0, len(stats.Trend))
	for _, point := range stats.Trend {
		response.Trend = append(response.Trend, dashboardSLATrendPointResponse{Date: point.Date, Value: point.Value})
	}
	return response
}

func dashboardMetricValue(result business.DashboardMetricResultModel) *float64 {
	if result.SLA != nil && (!result.SLA.Enabled || result.SLA.Eligible == 0) {
		return nil
	}
	return &result.Value
}

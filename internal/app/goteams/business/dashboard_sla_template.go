package business

import (
	"encoding/json"

	"goteams/internal/app/goteams/define"
)

func dashboardSLAStatsTemplateCards(stages []SLADefinitionModel) ([]DashboardCardModel, error) {
	// 图表最多支持 100 个阶段，阶段指标与优先级对比按配置顺序取前两个。
	stageIds := make([]int, 0, min(len(stages), 100))
	for _, stage := range stages[:min(len(stages), 100)] {
		stageIds = append(stageIds, stage.Id)
	}
	featuredStageIds := stageIds[:min(len(stageIds), 2)]
	recentDays := 7
	metrics := make([]DashboardMetricConfig, 0, 4)
	if len(stageIds) > 0 {
		metrics = append(metrics, DashboardMetricConfig{
			MetricType: define.DashboardMetricSLAAchievementRate, SLAStageIds: stageIds, Unit: "%",
		})
		for _, id := range featuredStageIds {
			metrics = append(metrics, DashboardMetricConfig{
				MetricType: define.DashboardMetricSLAStageAchievementRate, SLAStageIds: []int{id}, Unit: "%",
			})
		}
	}
	metrics = append(metrics, DashboardMetricConfig{
		MetricType: define.DashboardMetricSLAOverdueCount, SLALineColor: "#f5222d",
	})
	const metricWidth = define.DashboardGridColumns / 4
	cards := make([]DashboardCardModel, 0, len(metrics)+4)
	for i, config := range metrics {
		config.DataScope = define.DashboardMetricDataScopeRequirement
		config.SLARecentDays = &recentDays
		config.SLAShowComparison = true
		raw, err := json.Marshal(config)
		if err != nil {
			return nil, err
		}
		cards = append(cards, DashboardCardModel{
			CardType: define.DashboardCardTypeMetric, ContentMode: define.DashboardContentModeRichText,
			X: i * metricWidth, W: metricWidth, H: 2, Config: string(raw),
		})
	}

	targetRate := float64(95)
	charts := []struct {
		config     DashboardChartConfig
		x, y, w, h int
	}{
		{config: DashboardChartConfig{
			ChartType: "sla_trend", RecentDays: recentDays, SLATargetRate: &targetRate,
			SLARateColor: "#1664ff", SLAOverdueColor: "#f5222d",
		}, y: 2, w: 8, h: 4},
		{config: DashboardChartConfig{
			ChartType: "sla_dimension", SLADimension: "priority", SLAStageIds: featuredStageIds,
			SLAFirstResponseColor: "#1664ff", SLAResolutionColor: "#1bbf83",
		}, x: 8, y: 2, w: 4, h: 4},
		{config: DashboardChartConfig{
			ChartType: "sla_dimension", SLADimension: "category", SLAStageIds: stageIds,
		}, y: 6, w: 7, h: 4},
		{config: DashboardChartConfig{
			ChartType: "sla_distribution", SLAStageIds: stageIds,
			SLADistributionColors: DashboardSLADistributionColors{
				FirstResponse: "#f5222d", Resolution: "#fa8c16",
			},
		}, x: 7, y: 6, w: 5, h: 4},
	}
	if len(stageIds) == 0 {
		charts = charts[:1]
		charts[0].w = define.DashboardGridColumns
	}
	for _, chart := range charts {
		chart.config.DataScope = define.DashboardMetricDataScopeRequirement
		raw, err := json.Marshal(chart.config)
		if err != nil {
			return nil, err
		}
		cards = append(cards, DashboardCardModel{
			CardType: define.DashboardCardTypeChart, ContentMode: define.DashboardContentModeRichText,
			X: chart.x, Y: chart.y, W: chart.w, H: chart.h, Config: string(raw),
		})
	}
	return cards, nil
}

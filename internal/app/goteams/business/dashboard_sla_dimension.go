package business

import (
	"strconv"

	"goteams/internal/app/goteams/define"
)

// DashboardSLACategoryColors contains per-group overrides keyed by category ID.
type DashboardSLACategoryColors map[int]string

// DashboardSLADimensionSeriesModel retains counts for reconciling each rate.
type DashboardSLADimensionSeriesModel struct {
	MetricType, StageName string
	StageId               int
	Achieved, Eligible    int64
	Rate                  *float64
}

// DashboardSLADimensionGroupModel is a selected priority or category.
type DashboardSLADimensionGroupModel struct {
	Id     int
	Name   string
	Series []DashboardSLADimensionSeriesModel
}

// DashboardSLADimensionModel returns mutually exclusive groups for the chosen dimension.
type DashboardSLADimensionModel struct {
	Enabled     bool
	EvaluatedAt int64
	Groups      []DashboardSLADimensionGroupModel
}

func dashboardSLADimensionGroups(adminId, workspaceId int, lang string, config DashboardChartConfig) ([]DashboardSLADimensionGroupModel, map[int]int, error) {
	// ID 0 represents work items with no current priority or category.
	metadata := []RequirementCategoryModel{{Id: 0}}
	selected := config.SLACategoryIds
	if config.SLADimension == "category" {
		categories, err := listRequirementCategoryItems(adminId, workspaceId)
		if err != nil {
			return nil, nil, err
		}
		metadata = append(categories, metadata...)
	} else {
		selected = config.PriorityIds
		priorities, err := ListPriorities(ListPrioritiesModel{
			AdminId: adminId, WsId: strconv.Itoa(workspaceId), TypeKey: define.TypeKeyRequirement, Lang: lang,
		})
		if err != nil {
			return nil, nil, err
		}
		metadata = make([]RequirementCategoryModel, 0, len(priorities.Items)+1)
		for _, item := range priorities.Items {
			metadata = append(metadata, RequirementCategoryModel{Id: item.Id, Name: item.Label})
		}
		metadata = append(metadata, RequirementCategoryModel{})
	}
	groups, positions := selectDashboardSLAGroups(metadata, selected)
	return groups, positions, nil
}

func selectDashboardSLAGroups(metadata []RequirementCategoryModel, selected []int) ([]DashboardSLADimensionGroupModel, map[int]int) {
	parents := make(map[int]int, len(metadata))
	for _, item := range metadata {
		parents[item.Id] = item.ParentId
	}
	chosen := make(map[int]bool, len(selected))
	for _, id := range selected {
		// A stale or foreign selector matches no workspace group.
		if _, ok := parents[id]; ok {
			chosen[id] = true
		}
	}
	if len(selected) == 0 {
		for _, item := range metadata {
			chosen[item.Id] = item.ParentId == 0
		}
	}

	// A selected ancestor absorbs the entire subtree. Parent and child can both
	// be checked in the editor, but a work item belongs to exactly one result row.
	groupFor := make(map[int]int, len(metadata))
	for _, item := range metadata {
		group := -1
		if chosen[item.Id] {
			group = item.Id
		}
		for parent, depth := item.ParentId, 0; parent > 0 && depth < 2; parent, depth = parents[parent], depth+1 {
			if chosen[parent] {
				group = parent
			}
		}
		groupFor[item.Id] = group
	}
	groups := make([]DashboardSLADimensionGroupModel, 0, len(metadata))
	positions := make(map[int]int, len(metadata))
	for _, item := range metadata {
		if groupFor[item.Id] == item.Id {
			positions[item.Id] = len(groups)
			groups = append(groups, DashboardSLADimensionGroupModel{Id: item.Id, Name: item.Name})
		}
	}
	for id, group := range groupFor {
		index, ok := positions[group]
		if !ok {
			index = -1
		}
		groupFor[id] = index
	}
	return groups, groupFor
}

func queryDashboardSLADimension(model DashboardChartQueryModel, metricQuery DashboardMetricQueryModel, config DashboardChartConfig, where []string, args []any) (*DashboardSLADimensionModel, error) {
	groups, positions, err := dashboardSLADimensionGroups(model.AdminId, model.WorkspaceId, model.Lang, config)
	if err != nil {
		return nil, err
	}
	byPriority := config.SLADimension == "priority"
	series := []DashboardSLADimensionSeriesModel{{MetricType: define.DashboardMetricSLAAchievementRate}}
	if byPriority {
		series = make([]DashboardSLADimensionSeriesModel, 0, len(config.SLAStageIds))
		for _, id := range config.SLAStageIds {
			series = append(series, DashboardSLADimensionSeriesModel{
				MetricType: define.DashboardMetricSLAStageAchievementRate, StageId: id, StageName: config.slaStageNames[id],
			})
		}
	}
	counts := make([]DashboardSLAStatsModel, len(groups)*len(series))
	collect := func(item dashboardSLAItem, stages []SLAStageResultModel) {
		id := item.CategoryId
		if byPriority {
			id = item.PriorityId
		}
		index, exists := positions[id]
		if !exists {
			index = positions[0]
		}
		if index < 0 {
			return
		}
		for i, itemSeries := range series {
			input := stages
			if byPriority {
				input = stages[i : i+1]
			}
			addDashboardSLAItem(&counts[index*len(series)+i], itemSeries.MetricType, item.Resolved, input)
		}
	}
	metric, err := queryDashboardSLAMetric(metricQuery, DashboardMetricConfig{
		MetricType: define.DashboardMetricSLAAchievementRate, DataScope: config.DataScope, SLAStageIds: config.SLAStageIds,
	}, where, args, false, &dashboardSLACollection{Collect: collect})
	if err != nil {
		return nil, err
	}
	result := &DashboardSLADimensionModel{Enabled: metric.SLA.Enabled, EvaluatedAt: metric.SLA.EvaluatedAt, Groups: groups}
	for index := range groups {
		for i, item := range series {
			stats := &counts[index*len(series)+i]
			stats.Enabled = result.Enabled
			item.Achieved, item.Eligible = stats.Achieved, stats.Eligible
			item.Rate = dashboardSLAValue(item.MetricType, stats)
			groups[index].Series = append(groups[index].Series, item)
		}
	}
	return result, nil
}

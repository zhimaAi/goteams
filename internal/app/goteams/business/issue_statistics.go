package business

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/lib/pq"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
)

// IssueStatisticsQueryModel 表示独立工单统计页的访问范围和一次查询。
type IssueStatisticsQueryModel struct {
	AdminId, WorkspaceId, UserId int
	Lang, View, Dimension        string
	Preset, From, To             string
	AssigneeId                   int
	PriorityId, CategoryId       *int
	ThresholdMinutes             *int
	RoleIds                      []int
	Page, Size                   int
}

// IssueStatisticsMetadataModel 提供只读的 SLA 和筛选选项。
type IssueStatisticsMetadataModel struct {
	Options  SLAMetadataModel
	Stages   []DashboardSLAStageModel
	Enabled  bool
	Timezone string
}

// GetIssueStatisticsMetadata 返回固定统计页所需的工作空间选项。
func GetIssueStatisticsMetadata(adminId, workspaceId, userId int) (IssueStatisticsMetadataModel, error) {
	settings, stages, err := issueStatisticsContext(adminId, workspaceId, userId)
	if err != nil {
		return IssueStatisticsMetadataModel{}, err
	}
	options, err := GetSLAMetadata(SLAMetadataModel{AdminId: adminId, WorkspaceId: workspaceId})
	return IssueStatisticsMetadataModel{Options: options, Stages: stages, Enabled: settings.Enabled, Timezone: settings.Timezone}, err
}

func issueStatisticsContext(adminId, workspaceId, userId int) (SLASettingsModel, []DashboardSLAStageModel, error) {
	if err := ensureSLAWorkspace(nil, adminId, workspaceId); err != nil {
		return SLASettingsModel{}, nil, err
	}
	if !HasPermission(adminId, userId, strconv.Itoa(workspaceId), "issue_statistics:view") ||
		!HasPermission(adminId, userId, strconv.Itoa(workspaceId), "requirement:view") {
		return SLASettingsModel{}, nil, ErrDashboardForbidden
	}
	settings, err := getSLASettingsTx(nil, adminId, workspaceId, false)
	if err != nil {
		return settings, nil, err
	}
	if settings.Timezone == "" {
		settings.Timezone = "UTC"
	}
	definitions, err := listSLADefinitionsTx(nil, adminId, workspaceId, true)
	if err != nil {
		return settings, nil, err
	}
	stages := make([]DashboardSLAStageModel, 0, len(definitions))
	for _, definition := range definitions {
		if !definition.Disabled {
			stages = append(stages, DashboardSLAStageModel{Id: definition.Id, Name: definition.Name})
		}
	}
	return settings, stages, nil
}

// IssueStatisticsCountModel 区分可判定分母与有 SLA 周期的工单总数。
type IssueStatisticsCountModel struct {
	Total, Achieved, Eligible, Overdue int64
	Rate                               *float64
	Stages                             []IssueStatisticsStageCountModel
}

// IssueStatisticsStageCountModel 保留阶段达成计数和实际超时样本。
type IssueStatisticsStageCountModel struct {
	Achieved, Eligible, Overdue int64
	Rate, AverageOverdueSeconds *float64
	overdueSeconds              int64
}

// IssueStatisticsGroupModel 是按所选维度归组的一行工单统计。
type IssueStatisticsGroupModel struct {
	Key, Name string
	IssueStatisticsCountModel
	policyStartedAt, policyCycleId int64
}

// IssueStatisticsResultModel 表示一个视图的只读聚合结果。
type IssueStatisticsResultModel struct {
	Enabled           bool
	EvaluatedAt       int64
	Stages            []DashboardSLAStageModel
	Current, Previous IssueStatisticsCountModel
	Trend             []DashboardSLATrendPointModel
	Sparklines        [][]DashboardSLATrendPointModel
	Distribution      *DashboardSLADistributionModel
	Dimension         *DashboardSLADimensionModel
	Risk              *DashboardSLARiskModel
	Details           []IssueStatisticsGroupModel
	Page, Size, Total int
}

func issueStatisticsRange(model IssueStatisticsQueryModel, now time.Time) (time.Time, time.Time, int, error) {
	loc := now.Location()
	today := dashboardChartLocalDay(now)
	from, last := today, today
	switch model.Preset {
	case "today":
	case "week":
		from = today.AddDate(0, 0, -6)
	case "month30":
		from = today.AddDate(0, 0, -29)
	case "month":
		from = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, loc)
	case "custom":
		var err error
		from, err = time.ParseInLocation(time.DateOnly, model.From, loc)
		if err != nil {
			return from, last, 0, ErrDashboardInvalid
		}
		last, err = time.ParseInLocation(time.DateOnly, model.To, loc)
		if err != nil || last.Before(from) || last.After(today) || last.After(from.AddDate(0, 0, 365)) {
			return from, last, 0, ErrDashboardInvalid
		}
	default:
		return from, last, 0, ErrDashboardInvalid
	}
	days := 1
	for day := from; day.Before(last); day = day.AddDate(0, 0, 1) {
		days++
	}
	return from, last.AddDate(0, 0, 1), days, nil
}

func issueStatisticsCategoryIds(adminId, workspaceId, categoryId int) ([]int, error) {
	if categoryId == 0 {
		return []int{0}, nil
	}
	categories, err := listRequirementCategoryItems(adminId, workspaceId)
	if err != nil {
		return nil, err
	}
	_, positions := selectDashboardSLAGroups(categories, []int{categoryId})
	if group, ok := positions[categoryId]; !ok || group != 0 {
		return nil, ErrDashboardInvalid
	}
	ids := make([]int, 0)
	for id, group := range positions {
		if group == 0 {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// QueryIssueStatistics 按工单权限及临时筛选查询固定统计模块。
func QueryIssueStatistics(ctx context.Context, model IssueStatisticsQueryModel) (IssueStatisticsResultModel, error) {
	result := IssueStatisticsResultModel{Page: model.Page, Size: model.Size}
	if err := validate.IssueStatisticsQuery(model.View, model.Dimension, model.Preset, model.From, model.To, model.AssigneeId, model.PriorityId, model.CategoryId, model.RoleIds, model.Page, model.Size); err != nil {
		return result, err
	}
	if err := validate.IssueStatisticsThreshold(model.ThresholdMinutes); err != nil {
		return result, err
	}
	settings, stages, err := issueStatisticsContext(model.AdminId, model.WorkspaceId, model.UserId)
	if err != nil {
		return result, err
	}
	loc, err := time.LoadLocation(settings.Timezone)
	if err != nil {
		return result, fmt.Errorf("load issue statistics timezone: %w", err)
	}
	now := time.Now().In(loc)
	start, end, days, err := issueStatisticsRange(model, now)
	if err != nil {
		return result, err
	}
	result.Enabled, result.Stages, result.EvaluatedAt = settings.Enabled, stages, now.Unix()
	if !settings.Enabled || len(stages) == 0 {
		return result, nil
	}

	stageIds := make([]int, 0, len(stages))
	for _, stage := range stages {
		stageIds = append(stageIds, stage.Id)
	}
	noTopFilters := ""
	query := DashboardMetricQueryModel{
		AdminId: model.AdminId, WorkspaceId: model.WorkspaceId, UserId: model.UserId,
		Timezone: settings.Timezone, TopFilters: &noTopFilters,
	}
	config := DashboardMetricConfig{DataScope: define.DashboardMetricDataScopeRequirement, SLAStageIds: stageIds}
	where, args, err := dashboardWorkItemConditions(query, config.DataScope, "", true)
	if err != nil {
		return result, err
	}
	lower := start
	if model.View == "summary" {
		lower = start.AddDate(0, 0, -days)
	}
	args = append(args, lower.Unix(), end.Unix())
	where = append(where, fmt.Sprintf("r.created_at >= $%d AND r.created_at < $%d", len(args)-1, len(args)))
	if model.PriorityId != nil {
		args = append(args, *model.PriorityId)
		where = append(where, fmt.Sprintf("COALESCE(r.priority,0) = $%d", len(args)))
	}
	if model.CategoryId != nil {
		ids, err := issueStatisticsCategoryIds(model.AdminId, model.WorkspaceId, *model.CategoryId)
		if err != nil {
			return result, err
		}
		args = append(args, pq.Array(ids))
		where = append(where, fmt.Sprintf("COALESCE(r.category_id,0) = ANY($%d::INTEGER[])", len(args)))
	}
	if len(model.RoleIds) > 0 {
		args = append(args, pq.Array(model.RoleIds))
		roleFilter := fmt.Sprintf("EXISTS (SELECT 1 FROM "+define.TableWorkspaceMembers+" wm WHERE wm.admin_id=r.admin_id AND wm.workspace_id=r.workspace_id AND wm.user_id=ANY(r.assignee_ids) AND wm.role_id=ANY($%d::INTEGER[])", len(args))
		if model.AssigneeId > 0 {
			args = append(args, model.AssigneeId)
			roleFilter += fmt.Sprintf(" AND wm.user_id=$%d", len(args))
		}
		where = append(where, roleFilter+")")
	} else if model.AssigneeId > 0 {
		args = append(args, model.AssigneeId)
		where = append(where, fmt.Sprintf("$%d=ANY(r.assignee_ids)", len(args)))
	}
	args = append(args, pq.Array(stageIds))
	where = append(where, fmt.Sprintf("EXISTS (SELECT 1 FROM "+define.TableSLACycles+" cycle WHERE cycle.admin_id=r.admin_id AND cycle.workspace_id=r.workspace_id AND cycle.requirement_id=r.id AND cycle.stage_id=ANY($%d::INTEGER[]))", len(args)))

	switch model.View {
	case "summary", "dimension":
		err = queryIssueStatisticsSummary(ctx, model, query, config, where, args, start, end, &result)
	case "details":
		err = queryIssueStatisticsDetails(ctx, model, query, config, where, args, loc, &result)
	case "risk":
		err = queryIssueStatisticsRisk(ctx, model, query, config, where, args, &result)
	}
	return result, err
}

func (count *IssueStatisticsCountModel) add(item dashboardSLAItem, stages []SLAStageResultModel) {
	count.Total++
	if count.Stages == nil {
		count.Stages = make([]IssueStatisticsStageCountModel, len(stages))
	}
	eligible, achieved, _ := dashboardSLAOutcome(item.Resolved, stages)
	if eligible {
		count.Eligible++
	}
	if achieved {
		count.Achieved++
	}
	overdue := false
	for index, stage := range stages {
		current := &count.Stages[index]
		eligible, achieved, _ := dashboardSLAOutcome(true, stages[index:index+1])
		if eligible {
			current.Eligible++
		}
		if achieved {
			current.Achieved++
		}
		if risk, seconds := dashboardSLAStageRisk(stage, 0); risk == "overdue" {
			current.Overdue++
			current.overdueSeconds += seconds
			overdue = true
		}
	}
	if overdue {
		count.Overdue++
	}
}

func issueStatisticsRate(achieved, eligible int64) *float64 {
	if eligible == 0 {
		return nil
	}
	value := float64(achieved) * 100 / float64(eligible)
	return &value
}

func (count *IssueStatisticsCountModel) finish(stageCount int) {
	count.Rate = issueStatisticsRate(count.Achieved, count.Eligible)
	if count.Stages == nil {
		count.Stages = make([]IssueStatisticsStageCountModel, stageCount)
	}
	for index := range count.Stages {
		stage := &count.Stages[index]
		stage.Rate = issueStatisticsRate(stage.Achieved, stage.Eligible)
		if stage.Overdue > 0 {
			average := float64(stage.overdueSeconds) / float64(stage.Overdue)
			stage.AverageOverdueSeconds = &average
		}
	}
}

func issueStatisticsBucket(stages []SLAStageResultModel) int64 {
	var overdue int64
	unresolved := false
	for _, stage := range stages {
		if risk, _ := dashboardSLAStageRisk(stage, 0); risk == "overdue" {
			if overdue > 0 {
				return -1
			}
			overdue = stage.StageId
		}
		if stage.State != "completed" || stage.CalculationState != "ready" || stage.ElapsedSeconds == nil {
			unresolved = true
		}
	}
	if overdue > 0 {
		return overdue
	}
	if unresolved {
		return -2
	}
	return 0
}

func queryIssueStatisticsSummary(ctx context.Context, model IssueStatisticsQueryModel, query DashboardMetricQueryModel, config DashboardMetricConfig, where []string, args []any, start, end time.Time, result *IssueStatisticsResultModel) error {
	groups, positions, err := dashboardSLADimensionGroups(model.AdminId, model.WorkspaceId, model.Lang, DashboardChartConfig{SLADimension: model.Dimension})
	if err != nil {
		return err
	}
	featured := result.Stages[:min(2, len(result.Stages))]
	groupCounts := make([]IssueStatisticsCountModel, len(groups))
	daily := make(map[string]*IssueStatisticsCountModel)
	buckets := make(map[int64]int64)
	collection := &dashboardSLACollection{Context: ctx, EvaluatedAt: result.EvaluatedAt, Collect: func(item dashboardSLAItem, stages []SLAStageResultModel) {
		if model.View == "summary" {
			if item.CreatedAt < start.Unix() {
				result.Previous.add(item, stages)
				return
			}
			result.Current.add(item, stages)
			date := time.Unix(item.CreatedAt, 0).In(start.Location()).Format(time.DateOnly)
			if daily[date] == nil {
				daily[date] = &IssueStatisticsCountModel{}
			}
			daily[date].add(item, stages)
			buckets[issueStatisticsBucket(stages)]++
		}
		id := item.PriorityId
		if model.Dimension == "category" {
			id = item.CategoryId
		}
		index, ok := positions[id]
		if !ok {
			index, ok = positions[0]
		}
		if ok && index >= 0 {
			groupCounts[index].add(item, stages[:len(featured)])
		}
	}}
	metric, err := queryDashboardSLAMetric(query, config, where, args, false, collection)
	if err != nil {
		return err
	}
	result.Enabled = metric.SLA.Enabled
	for index := range groups {
		count := &groupCounts[index]
		if count.Total == 0 {
			continue
		}
		count.finish(len(featured))
		for stageIndex, stage := range featured {
			value := count.Stages[stageIndex]
			groups[index].Series = append(groups[index].Series, DashboardSLADimensionSeriesModel{StageId: stage.Id, StageName: stage.Name, Achieved: value.Achieved, Eligible: value.Eligible, Rate: value.Rate})
		}
	}
	activeGroups := make([]DashboardSLADimensionGroupModel, 0, len(groups))
	for _, group := range groups {
		if len(group.Series) > 0 {
			activeGroups = append(activeGroups, group)
		}
	}
	result.Dimension = &DashboardSLADimensionModel{Enabled: result.Enabled, EvaluatedAt: result.EvaluatedAt, Groups: activeGroups}
	if model.View != "summary" {
		return nil
	}

	result.Current.finish(len(result.Stages))
	result.Previous.finish(len(result.Stages))
	result.Sparklines = make([][]DashboardSLATrendPointModel, len(featured)+2)
	last := min(end.Unix(), result.EvaluatedAt+1)
	for day := start; day.Unix() < last; day = day.AddDate(0, 0, 1) {
		date := day.Format(time.DateOnly)
		count := daily[date]
		if count == nil {
			count = &IssueStatisticsCountModel{}
		}
		count.finish(len(result.Stages))
		point := DashboardSLATrendPointModel{Date: date, Value: count.Rate, Achieved: count.Achieved, Eligible: count.Eligible, Started: count.Total, Overdue: count.Overdue}
		result.Trend = append(result.Trend, point)
		result.Sparklines[0] = append(result.Sparklines[0], point)
		for index := range featured {
			result.Sparklines[index+1] = append(result.Sparklines[index+1], DashboardSLATrendPointModel{Date: date, Value: count.Stages[index].Rate})
		}
		overdue := float64(count.Overdue)
		lastIndex := len(result.Sparklines) - 1
		result.Sparklines[lastIndex] = append(result.Sparklines[lastIndex], DashboardSLATrendPointModel{Date: date, Value: &overdue})
	}
	result.Distribution = &DashboardSLADistributionModel{Enabled: result.Enabled, EvaluatedAt: result.EvaluatedAt, Total: result.Current.Total}
	for _, stage := range result.Stages {
		result.Distribution.Buckets = append(result.Distribution.Buckets, DashboardSLADistributionBucketModel{Key: "stage", StageId: stage.Id, StageName: stage.Name, Count: buckets[int64(stage.Id)]})
	}
	for _, bucket := range []struct {
		key string
		id  int64
	}{{"multiple", -1}, {"on_time", 0}, {"unknown", -2}} {
		result.Distribution.Buckets = append(result.Distribution.Buckets, DashboardSLADistributionBucketModel{Key: bucket.key, Count: buckets[bucket.id]})
	}
	return nil
}

func queryIssueStatisticsDetails(ctx context.Context, model IssueStatisticsQueryModel, query DashboardMetricQueryModel, config DashboardMetricConfig, where []string, args []any, loc *time.Location, result *IssueStatisticsResultModel) error {
	groups := make(map[string]*IssueStatisticsGroupModel)
	add := func(key string, item dashboardSLAItem, stages []SLAStageResultModel) {
		group := groups[key]
		if group == nil {
			group = &IssueStatisticsGroupModel{Key: key}
			groups[key] = group
		}
		if model.Dimension == "rule" && (item.PolicyStartedAt > group.policyStartedAt ||
			(item.PolicyStartedAt == group.policyStartedAt && item.PolicyCycleId > group.policyCycleId)) {
			group.Name = item.PolicyName
			group.policyStartedAt, group.policyCycleId = item.PolicyStartedAt, item.PolicyCycleId
		}
		group.add(item, stages)
	}
	collection := &dashboardSLACollection{
		Context: ctx, Details: model.Dimension == "member", Policies: model.Dimension == "rule", EvaluatedAt: result.EvaluatedAt,
		Collect: func(item dashboardSLAItem, stages []SLAStageResultModel) {
			switch model.Dimension {
			case "time":
				add(time.Unix(item.CreatedAt, 0).In(loc).Format(time.DateOnly), item, stages)
			case "rule":
				add(strconv.Itoa(item.PolicyId), item, stages)
			case "category":
				add(strconv.Itoa(item.CategoryId), item, stages)
			case "member":
				if len(item.AssigneeIds) == 0 {
					add("0", item, stages)
				}
				seen := make(map[int]bool, len(item.AssigneeIds))
				for _, id := range item.AssigneeIds {
					if !seen[id] {
						seen[id] = true
						add(strconv.Itoa(id), item, stages)
					}
				}
			}
		},
	}
	metric, err := queryDashboardSLAMetric(query, config, where, args, false, collection)
	if err != nil {
		return err
	}
	result.Enabled = metric.SLA.Enabled
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if model.Dimension == "time" {
			return keys[i] > keys[j]
		}
		if left, right := groups[keys[i]].Total, groups[keys[j]].Total; left != right {
			return left > right
		}
		return keys[i] < keys[j]
	})
	result.Total = len(keys)
	offset := min((model.Page-1)*model.Size, len(keys))
	for _, key := range keys[offset:min(offset+model.Size, len(keys))] {
		group := groups[key]
		group.finish(len(result.Stages))
		result.Details = append(result.Details, *group)
	}
	return nil
}

func queryIssueStatisticsRisk(ctx context.Context, model IssueStatisticsQueryModel, query DashboardMetricQueryModel, config DashboardMetricConfig, where []string, args []any, result *IssueStatisticsResultModel) error {
	risk := &DashboardSLARiskModel{Page: model.Page, Size: model.Size, ThresholdSeconds: 30 * 60, EvaluatedAt: result.EvaluatedAt}
	if model.ThresholdMinutes != nil {
		risk.ThresholdSeconds = *model.ThresholdMinutes * 60
	}
	for _, stage := range result.Stages {
		risk.Stages = append(risk.Stages, DashboardSLARiskStageModel{Id: stage.Id, Name: stage.Name, ThresholdSeconds: risk.ThresholdSeconds})
	}
	stats := &DashboardSLAStatsModel{Enabled: result.Enabled}
	offset := int64((model.Page - 1) * model.Size)
	collection := &dashboardSLACollection{Context: ctx, Details: true, EvaluatedAt: result.EvaluatedAt, Collect: func(item dashboardSLAItem, stages []SLAStageResultModel) {
		addDashboardSLAItem(stats, define.DashboardMetricSLAOverdueRate, item.Resolved, stages)
		row := dashboardSLARiskRow(item, stages, risk.Stages)
		if row.Risk == "" {
			return
		}
		if risk.Total >= offset && len(risk.Rows) < model.Size {
			risk.Rows = append(risk.Rows, row)
		}
		risk.Total++
	}}
	metric, err := queryDashboardSLAMetric(query, config, where, args, false, collection)
	if err != nil {
		return err
	}
	result.Enabled, risk.Enabled, stats.Enabled = metric.SLA.Enabled, metric.SLA.Enabled, metric.SLA.Enabled
	risk.Overdue, risk.Started = stats.Overdue, stats.Started
	risk.OverdueRate = dashboardSLAValue(define.DashboardMetricSLAOverdueRate, stats)
	result.Risk = risk
	return nil
}

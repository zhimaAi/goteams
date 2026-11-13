package business

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/zhimaAi/go_tools/msql"
	"goteams/internal/app/goteams/define"
)

// DashboardSLAStatsModel 保留分子、分母和未知数量，避免空样本被解释为已达成。
type DashboardSLAStatsModel struct {
	Enabled                     bool
	Achieved, Eligible, Unknown int64
	EvaluatedAt                 int64
	Started, Overdue, Critical  int64
	ThresholdSeconds            int
	PreviousValue, Change       *float64
	StageName                   string
	Trend                       []DashboardSLATrendPointModel
}

// DashboardSLATrendPointModel 是按创建日期分组的当前SLA结果，不重建历史运行状态。
type DashboardSLATrendPointModel struct {
	Date                                 string
	Value                                *float64
	Achieved, Eligible, Started, Overdue int64
}

func dashboardSLAOutcome(resolved bool, stages []SLAStageResultModel) (eligible, achieved, unknown bool) {
	allCompleted, overdue, runningOverdue := len(stages) > 0, false, false
	for _, stage := range stages {
		if stage.State != "completed" {
			allCompleted = false
		}
		if stage.CalculationState != "ready" || stage.ElapsedSeconds == nil || (stage.State != "completed" && stage.State != "running") {
			unknown = true
			continue
		}
		if *stage.ElapsedSeconds > stage.LimitSeconds {
			overdue = true
			if stage.State == "running" {
				runningOverdue = true
			}
		}
	}
	// 已知运行中超时即确定失败，不因其他阶段未知而把该工单移出严格口径分母。
	eligible = runningOverdue || (resolved && allCompleted && !unknown)
	achieved = eligible && allCompleted && !overdue && !unknown
	return eligible, achieved, unknown
}

func addDashboardSLAItem(stats *DashboardSLAStatsModel, metric string, resolved bool, stages []SLAStageResultModel) {
	if metric == define.DashboardMetricSLAStageAchievementRate || metric == define.DashboardMetricSLAFirstResponseRate || metric == define.DashboardMetricSLAResolutionRate {
		resolved = true
	}
	eligible, achieved, unknown := dashboardSLAOutcome(resolved, stages)
	started, overdue := false, false
	for _, stage := range stages {
		if stage.CalculationState == "ready" && stage.ElapsedSeconds != nil && (stage.State == "running" || stage.State == "completed") {
			started = true
			if *stage.ElapsedSeconds > stage.LimitSeconds {
				overdue = true
			}
		}
	}
	if started {
		stats.Started++
	}
	if overdue {
		stats.Overdue++
	}
	if unknown {
		stats.Unknown++
	}
	if metric == define.DashboardMetricSLAOverdueCount || metric == define.DashboardMetricSLAOverdueRate || metric == define.DashboardMetricSLAStageCriticalCount {
		eligible = started
		achieved = false
	}
	if metric == define.DashboardMetricSLAStageCriticalCount && len(stages) == 1 {
		if risk, _ := dashboardSLAStageRisk(stages[0], dashboardSLADefaultRiskThresholdSeconds); risk == "critical" {
			stats.Critical++
		}
	}
	if eligible {
		stats.Eligible++
	}
	if achieved {
		stats.Achieved++
	}
}

func dashboardSLAValue(metric string, stats *DashboardSLAStatsModel) *float64 {
	if !stats.Enabled || stats.Eligible == 0 {
		return nil
	}
	value := float64(stats.Achieved) * 100 / float64(stats.Eligible)
	if metric == define.DashboardMetricSLAOverdueCount {
		value = float64(stats.Overdue)
	}
	if metric == define.DashboardMetricSLAOverdueRate {
		value = float64(stats.Overdue) * 100 / float64(stats.Eligible)
	}
	if metric == define.DashboardMetricSLAStageCriticalCount {
		value = float64(stats.Critical)
	}
	return &value
}

func normalizeDashboardSLAMetric(config *DashboardMetricConfig) {
	config.RecentDays = 0
	if config.SLARecentDays == nil {
		// 旧卡片可能残留已停用的时间范围和环比，继续按不限时间统计。
		config.SLAShowComparison = false
	}
	switch config.MetricType {
	case define.DashboardMetricSLAFirstResponseRate, define.DashboardMetricSLAResolutionRate:
		config.MetricType = define.DashboardMetricSLAStageAchievementRate
	case define.DashboardMetricSLAOverdueCount, define.DashboardMetricSLAOverdueRate:
		config.SLAStageIds = nil
	}
}

func dashboardSLAStageNames(adminId, workspaceId int, ids []int) (map[int]string, error) {
	if err := ensureSLAWorkspace(nil, adminId, workspaceId); err != nil {
		if err == ErrSLAWorkspace {
			return nil, ErrDashboardInvalidMetric
		}
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	stored, err := loadSLADefinitionsTx(nil, adminId, workspaceId, true)
	if err != nil {
		return nil, err
	}
	available := make(map[int]string, len(stored))
	// 已归档阶段仍可为历史卡片提供阶段名称。
	for _, stage := range stored {
		available[stage.Id] = stage.Name
	}
	selected := make(map[int]string, len(ids))
	for _, id := range ids {
		name, found := available[id]
		if _, seen := selected[id]; id <= 0 || !found || seen {
			return nil, ErrDashboardInvalidMetric
		}
		selected[id] = name
	}
	return selected, nil
}

// dashboardSLATrendDates covers every local date intersecting the rolling chart window.
// Metric cards keep their existing seven-day sparkline.
func dashboardSLATrendDates(start, now int64, loc *time.Location, fullTrend bool) []string {
	today := dashboardChartLocalDay(time.Unix(now, 0).In(loc))
	first := today.AddDate(0, 0, -6)
	if fullTrend {
		first = dashboardChartLocalDay(time.Unix(start, 0).In(loc))
	}
	var dates []string
	for day := first; !day.After(today); day = day.AddDate(0, 0, 1) {
		dates = append(dates, day.Format("2006-01-02"))
	}
	return dates
}

// dashboardSLAItemStages 保持选中阶段顺序，超时指标则使用工单实际进入的所有阶段。
func dashboardSLAItemStages(ids []int, byStage map[int64]SLAStageResultModel) []SLAStageResultModel {
	if len(ids) == 0 {
		stages := make([]SLAStageResultModel, 0, len(byStage))
		for _, stage := range byStage {
			stages = append(stages, stage)
		}
		return stages
	}
	stages := make([]SLAStageResultModel, 0, len(ids))
	for _, id := range ids {
		stages = append(stages, byStage[int64(id)])
	}
	return stages
}

type dashboardSLAItem struct {
	Id                               int64
	PriorityId, CategoryId, StatusId int
	Title                            string
	AssigneeIds                      []int
	Resolved                         bool
}

type dashboardSLACollection struct {
	Details bool
	Collect func(dashboardSLAItem, []SLAStageResultModel)
}

func queryDashboardSLAMetric(model DashboardMetricQueryModel, config DashboardMetricConfig, where []string, args []any, fullTrend bool, collection *dashboardSLACollection) (DashboardMetricResultModel, error) {
	stats := &DashboardSLAStatsModel{EvaluatedAt: time.Now().Unix(), StageName: config.SLAStageName}
	result := DashboardMetricResultModel{MetricType: config.MetricType, DataScope: config.DataScope, SLA: stats}
	if config.MetricType == define.DashboardMetricSLAStageCriticalCount {
		if len(config.SLAStageIds) == 0 {
			return result, nil
		}
		stats.ThresholdSeconds = dashboardSLADefaultRiskThresholdSeconds
	}
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return result, err
	}
	// 分批读取使用同一只读快照，避免分页期间的状态变化使同一次统计前后口径不一致。
	queryContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tx, err := db.BeginTx(queryContext, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback() }()
	settings, err := slaRows(tx, "SELECT enabled,timezone FROM "+define.TableSLASettings+" WHERE admin_id=$1 AND workspace_id=$2", model.AdminId, model.WorkspaceId)
	if err != nil {
		return result, err
	}
	if len(settings) == 0 {
		return result, nil
	}
	stats.Enabled, err = strconv.ParseBool(settings[0]["enabled"])
	if err != nil {
		return result, err
	}
	if !stats.Enabled {
		return result, nil
	}
	pauses, err := loadSLAPausesTx(tx, model.AdminId, model.WorkspaceId)
	if err != nil {
		return result, err
	}
	timezone := model.Timezone
	if timezone == "" {
		timezone = settings[0]["timezone"]
	}
	if timezone == "" {
		timezone = "UTC"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return result, ErrDashboardInvalidMetric
	}
	now := stats.EvaluatedAt
	recentDays := config.RecentDays
	if config.SLARecentDays != nil {
		recentDays = *config.SLARecentDays
	}
	windowSeconds := int64(recentDays) * 86400
	currentStart := int64(0)
	if windowSeconds > 0 {
		currentStart = now - windowSeconds
	}
	queryStart := currentStart
	var previous *DashboardSLAStatsModel
	if config.SLAShowComparison {
		queryStart = currentStart - windowSeconds
		previous = &DashboardSLAStatsModel{Enabled: true}
	}
	var dates []string
	if collection == nil {
		dates = dashboardSLATrendDates(currentStart, now, loc, fullTrend)
	}
	daily := make(map[string]*DashboardSLAStatsModel, len(dates))
	for _, date := range dates {
		daily[date] = &DashboardSLAStatsModel{Enabled: true}
	}
	where = append(append([]string{}, where...), "r.parent_id = 0")
	if len(config.SLAStageIds) == 0 {
		// 无阶段记录的工单不会进入超时指标分母，全量查询可跳过这些工单。
		where = append(where, "EXISTS (SELECT 1 FROM "+define.TableSLACycles+
			" cycle WHERE cycle.admin_id=r.admin_id AND cycle.workspace_id=r.workspace_id AND cycle.requirement_id=r.id)")
	}
	args = append(append([]any{}, args...), queryStart, now)
	where = append(where, fmt.Sprintf("r.created_at >= $%d AND r.created_at <= $%d", len(args)-1, len(args)))
	condition := strings.Join(where, " AND ")
	itemFields := "r.id,r.created_at,COALESCE(s.name,'') AS status_name"
	if collection != nil {
		itemFields += ",COALESCE(r.priority,0) AS priority,COALESCE(r.category_id,0) AS category_id"
		if collection.Details {
			itemFields += ",r.title,COALESCE(r.status_id,0) AS status_id,COALESCE(r.assignee_ids,'{}')::text AS assignee_ids"
		}
	}
	// 按工单 ID 倒序扫描，明细统计仅保留所请求的页。
	for cursor := int64(1<<63 - 1); cursor > 0; {
		if err := queryContext.Err(); err != nil {
			return result, err
		}
		batchArgs := append(append([]any{}, args...), cursor)
		batchCondition := fmt.Sprintf(" AND r.id <= $%d::bigint ORDER BY r.id DESC LIMIT 200", len(args)+1)
		items, err := slaRows(tx, "SELECT "+itemFields+" FROM "+define.TableRequirements+" r LEFT JOIN "+define.TableWorkItemStatuses+" s ON s.admin_id=r.admin_id AND s.workspace_id=r.workspace_id AND s.type_key=r.type AND s.id=r.status_id WHERE "+condition+batchCondition, batchArgs...)
		if err != nil {
			return result, err
		}
		if len(items) == 0 {
			break
		}
		ids := make([]int64, 0, len(items))
		resolved := make(map[int64]bool, len(items))
		created := make(map[int64]int64, len(items))
		for _, item := range items {
			id, err := slaNumber(item, "id")
			if err != nil {
				return result, err
			}
			ids = append(ids, id)
			created[id], err = slaNumber(item, "created_at")
			if err != nil {
				return result, err
			}
			resolved[id] = dashboardCategoryStatusCounts(false, false, item["status_name"], 1).Resolved > 0
		}
		cycleFilter := " WHERE admin_id=$1 AND workspace_id=$2 AND requirement_id=ANY($3)"
		cycleArgs := []any{model.AdminId, model.WorkspaceId, pq.Array(ids)}
		if len(config.SLAStageIds) > 0 {
			cycleFilter += " AND stage_id=ANY($4)"
			cycleArgs = append(cycleArgs, pq.Array(config.SLAStageIds))
		}
		cycles, err := slaRows(tx, "SELECT DISTINCT ON(requirement_id,stage_id) requirement_id,"+slaCycleColumns+" FROM "+define.TableSLACycles+cycleFilter+" ORDER BY requirement_id,stage_id,(state='running') DESC,id DESC", cycleArgs...)
		if err != nil {
			return result, err
		}
		byItem := make(map[int64]map[int64]SLAStageResultModel, len(items))
		for _, row := range cycles {
			id, err := slaNumber(row, "requirement_id")
			if err != nil {
				return result, err
			}
			cycle, err := slaCycleFromRow(row)
			if err != nil {
				return result, err
			}
			projection, err := slaCycleProjection(cycle, pauses, stats.EvaluatedAt)
			if err != nil {
				return result, err
			}
			if byItem[id] == nil {
				byItem[id] = make(map[int64]SLAStageResultModel)
			}
			byItem[id][cycle.StageId] = projection
		}
		for index, id := range ids {
			stages := dashboardSLAItemStages(config.SLAStageIds, byItem[id])
			if collection != nil {
				priorityId, err := strconv.Atoi(items[index]["priority"])
				if err != nil {
					return result, fmt.Errorf("parse SLA work item priority: %w", err)
				}
				categoryId, err := strconv.Atoi(items[index]["category_id"])
				if err != nil {
					return result, fmt.Errorf("parse SLA work item category: %w", err)
				}
				item := dashboardSLAItem{Id: id, PriorityId: priorityId, CategoryId: categoryId, Resolved: resolved[id]}
				if collection.Details {
					row := items[index]
					item.Title = row["title"]
					item.StatusId, err = strconv.Atoi(row["status_id"])
					if err != nil {
						return result, fmt.Errorf("parse SLA work item status: %w", err)
					}
					item.AssigneeIds, err = parsePgIntArray(row["assignee_ids"])
					if err != nil {
						return result, fmt.Errorf("parse SLA work item assignees: %w", err)
					}
				}
				collection.Collect(item, stages)
				continue
			}
			if previous != nil && created[id] < currentStart {
				addDashboardSLAItem(previous, config.MetricType, resolved[id], stages)
				continue
			}
			addDashboardSLAItem(stats, config.MetricType, resolved[id], stages)
			if day := daily[time.Unix(created[id], 0).In(loc).Format("2006-01-02")]; day != nil {
				addDashboardSLAItem(day, config.MetricType, resolved[id], stages)
			}
		}
		cursor = ids[len(ids)-1] - 1
	}
	value := dashboardSLAValue(config.MetricType, stats)
	if value != nil {
		result.Value = *value
	}
	if previous != nil {
		stats.PreviousValue = dashboardSLAValue(config.MetricType, previous)
		if value != nil && stats.PreviousValue != nil {
			change := *value - *stats.PreviousValue
			stats.Change = &change
		}
	}
	for _, date := range dates {
		day := daily[date]
		stats.Trend = append(stats.Trend, DashboardSLATrendPointModel{
			Date: date, Value: dashboardSLAValue(config.MetricType, day),
			Achieved: day.Achieved, Eligible: day.Eligible, Started: day.Started, Overdue: day.Overdue,
		})
	}
	if err = tx.Commit(); err != nil {
		return result, fmt.Errorf("commit dashboard SLA read: %w", err)
	}
	return result, nil
}

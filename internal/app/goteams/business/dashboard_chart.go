package business

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
)

// DashboardChartConfig is the persisted statistical card configuration.
type DashboardChartConfig struct {
	ChartType             string `json:"chart_type"`
	DataScope             string `json:"data_scope"`
	StatusIds             []int  `json:"status_ids"`
	PriorityIds           []int  `json:"priority_ids"`
	SortOrder             string `json:"sort_order"`
	CustomOrder           []int  `json:"custom_order"`
	RequirementColor      string `json:"requirement_color"`
	DefectColor           string `json:"defect_color"`
	Palette               string `json:"palette"`
	DaysFrom              int    `json:"days_from"`
	DaysTo                int    `json:"days_to"`
	MetricType            string `json:"metric_type"`
	Top3Color             string `json:"top3_color"`
	RestColor             string `json:"rest_color"`
	DoneColor             string `json:"done_color"`
	RemainingColor        string `json:"remaining_color"`
	BaselineColor         string `json:"baseline_color"`
	WorkloadDimension     string `json:"workload_dimension,omitempty"`
	AssigneeScope         string `json:"assignee_scope,omitempty"`
	AssigneeScopeIds      []int  `json:"assignee_scope_ids,omitempty"`
	slaStageNames         map[int]string
	SLAStageColors        map[int]string                 `json:"sla_stage_colors,omitempty"`
	SLAStageIds           []int                          `json:"sla_stage_ids,omitempty"`
	RecentDays            int                            `json:"recent_days,omitempty"`
	SLATargetRate         *float64                       `json:"sla_target_rate,omitempty"`
	SLARateColor          string                         `json:"sla_rate_color,omitempty"`
	SLAOverdueColor       string                         `json:"sla_overdue_color,omitempty"`
	SLADimension          string                         `json:"sla_dimension,omitempty"`
	SLACategoryIds        []int                          `json:"sla_category_ids,omitempty"`
	SLACategoryColors     DashboardSLACategoryColors     `json:"sla_category_colors,omitempty"`
	SLAFirstResponseColor string                         `json:"sla_first_response_color,omitempty"`
	SLAResolutionColor    string                         `json:"sla_resolution_color,omitempty"`
	SLADistributionColors DashboardSLADistributionColors `json:"sla_distribution_colors,omitempty"`

	SLARiskThresholdMinutes      *int        `json:"sla_risk_threshold_minutes,omitempty"`
	SLARiskStageThresholdMinutes map[int]int `json:"sla_risk_stage_threshold_minutes,omitempty"`
}

// DashboardChartQueryModel contains the authenticated statistical query context.
type DashboardChartQueryModel struct {
	AdminId, WorkspaceId, UserId, DashboardId, CardId int
	Timezone, Lang                                    string
	PreviewConfig                                     *string
	CategoryFilter                                    *DashboardCategoryFilterModel
	WorkloadFilter                                    *DashboardWorkloadFilterModel
	SLARiskFilter                                     *DashboardSLARiskFilterModel
}

// DashboardChartBucketModel is a typed aggregate with workspace metadata.
type DashboardChartBucketModel struct {
	Id                int
	Type, Name, Color string
	Count             int64
	Timestamp         int64
}

// DashboardChartResultModel contains aggregates and the number of undated completed items.
type DashboardChartResultModel struct {
	Buckets         []DashboardChartBucketModel
	UndatedCount    int64
	Categories      []DashboardCategoryStatModel
	Workload        []DashboardWorkloadRowModel
	SLATrend        *DashboardSLAStatsModel
	SLADimension    *DashboardSLADimensionModel
	SLADistribution *DashboardSLADistributionModel
	SLARisk         *DashboardSLARiskModel
}

func parseDashboardChartConfig(adminId, workspaceId int, raw string) (DashboardChartConfig, error) {
	var c DashboardChartConfig
	if validate.DashboardConfig([]byte(raw)) != nil {
		return c, ErrDashboardInvalid
	}
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return c, ErrDashboardInvalid
	}
	if c.ChartType == "sla_distribution" || c.ChartType == "sla_dimension" {
		if len(c.SLAStageColors) > 100 {
			return c, ErrDashboardInvalidMetric
		}
		for id, color := range c.SLAStageColors {
			if id <= 0 || validate.DashboardChartColors(color) != nil {
				return c, ErrDashboardInvalidMetric
			}
		}
	}
	if c.ChartType == "sla_distribution" || c.ChartType == "sla_risk" {
		if c.DataScope != define.DashboardMetricDataScopeRequirement || len(c.SLAStageIds) > 100 {
			return c, ErrDashboardInvalidMetric
		}
		if c.ChartType == "sla_distribution" && (len(c.SLAStageIds) == 0 || !c.SLADistributionColors.valid()) {
			return c, ErrDashboardInvalidMetric
		}
		stageIds := c.SLAStageIds
		if c.ChartType == "sla_risk" {
			if c.SLARiskThresholdMinutes != nil &&
				(*c.SLARiskThresholdMinutes < 1 || *c.SLARiskThresholdMinutes > 365*24*60) {
				return c, ErrDashboardInvalidMetric
			}
			if len(c.SLARiskStageThresholdMinutes) > 0 {
				stageIds = slices.Clone(stageIds)
			}
			for id, minutes := range c.SLARiskStageThresholdMinutes {
				if id <= 0 || minutes < 1 || minutes > 365*24*60 {
					return c, ErrDashboardInvalidMetric
				}
				if !slices.Contains(c.SLAStageIds, id) {
					stageIds = append(stageIds, id)
				}
			}
		}
		names, err := dashboardSLAStageNames(adminId, workspaceId, stageIds)
		c.slaStageNames = names
		return c, err
	}
	if c.ChartType == "sla_trend" {
		if len(c.SLAStageIds) > 100 || validate.DashboardSLATrend(c.DataScope, c.RecentDays, c.SLATargetRate, c.SLARateColor, c.SLAOverdueColor) != nil {
			return c, ErrDashboardInvalidMetric
		}
		_, err := dashboardSLAStageNames(adminId, workspaceId, c.SLAStageIds)
		return c, err
	}
	if c.ChartType == "sla_dimension" {
		if validate.DashboardSLADimension(c.SLADimension, c.DataScope, c.PriorityIds, c.SLACategoryIds) != nil {
			return c, ErrDashboardInvalidMetric
		}
		if len(c.SLAStageIds) == 0 || len(c.SLAStageIds) > 100 {
			return c, ErrDashboardInvalidMetric
		}
		if len(c.SLACategoryColors) > 500 {
			return c, ErrDashboardInvalidMetric
		}
		for id, color := range c.SLACategoryColors {
			if id < 0 || validate.DashboardChartColors(color) != nil {
				return c, ErrDashboardInvalidMetric
			}
		}
		names, err := dashboardSLAStageNames(adminId, workspaceId, c.SLAStageIds)
		c.slaStageNames = names
		c.RecentDays = 0
		return c, err
	}
	if err := validate.DashboardChart(c.ChartType, c.DataScope, c.SortOrder, c.Palette, c.RequirementColor, c.DefectColor, c.DaysFrom, c.DaysTo, c.StatusIds, c.PriorityIds, c.CustomOrder); err != nil {
		return c, ErrDashboardInvalid
	}
	// 排名与燃烧图的专属字段只在其自身图表类型下校验，其余类型不要求携带。
	switch c.ChartType {
	case "workload_stats":
		if c.WorkloadDimension != "member" && c.WorkloadDimension != "date" {
			return c, ErrDashboardInvalid
		}
		fallthrough
	case "category_stats":
		if c.DataScope != define.DashboardMetricDataScopeRequirement || validate.DashboardCategoryScope(c.AssigneeScope, c.AssigneeScopeIds) != nil {
			return c, ErrDashboardInvalid
		}
		return c, nil
	case "assignee_rank_bar":
		if validate.DashboardChartMetricType(c.MetricType) != nil || validate.DashboardChartColors(c.Top3Color, c.RestColor) != nil {
			return c, ErrDashboardInvalid
		}
	case "iteration_burndown":
		if validate.DashboardChartColors(c.DoneColor, c.RemainingColor, c.BaselineColor) != nil {
			return c, ErrDashboardInvalid
		}
	}
	// Reject foreign workspace IDs before saving or querying a shared card.
	statusIds, priorityIds := map[int]bool{}, map[int]bool{0: true}
	for _, typ := range []string{"requirement", "defect"} {
		if c.DataScope != "all" && c.DataScope != typ {
			continue
		}
		statuses, err := ListStatuses(ListStatusesModel{AdminId: adminId, WsId: strconv.Itoa(workspaceId), TypeKey: typ})
		if err != nil {
			return c, err
		}
		priorities, err := ListPriorities(ListPrioritiesModel{AdminId: adminId, WsId: strconv.Itoa(workspaceId), TypeKey: typ})
		if err != nil {
			return c, err
		}
		for _, item := range statuses.Items {
			statusIds[item.Id] = true
		}
		for _, item := range priorities.Items {
			priorityIds[item.Id] = true
		}
	}
	for _, id := range c.StatusIds {
		if !statusIds[id] {
			return c, ErrDashboardInvalid
		}
	}
	for _, id := range c.PriorityIds {
		if !priorityIds[id] {
			return c, ErrDashboardInvalid
		}
	}

	return c, nil
}

// dashboardChartDayRange resolves a rolling "N days ago — M days ago" window into local [start, end) day boundaries.
func dashboardChartDayRange(loc *time.Location, from, to int) (start, end time.Time) {
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	return today.AddDate(0, 0, -from), today.AddDate(0, 0, 1-to)
}

func dashboardSLAChartDateRange(timezone string, filter *DashboardCategoryFilterModel) (int64, int64, error) {
	if filter != nil && (filter.From != 0 || filter.To != 0) {
		if filter.From <= 0 || filter.To <= filter.From || filter.To-filter.From > 367*86400 {
			return 0, 0, ErrDashboardInvalidMetric
		}
		return filter.From, filter.To, nil
	}
	if timezone == "" {
		timezone = "UTC"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return 0, 0, ErrDashboardInvalidMetric
	}
	start, end := dashboardChartDayRange(loc, 6, 0)
	return start.Unix(), end.Unix(), nil
}

// dashboardChartLocalDay truncates a timestamp to the start of its local calendar day.
func dashboardChartLocalDay(moment time.Time) time.Time {
	return time.Date(moment.Year(), moment.Month(), moment.Day(), 0, 0, 0, 0, moment.Location())
}

// queryDashboardChartDailyCounts groups work items by local calendar day for one timestamp column.
func queryDashboardChartDailyCounts(where []string, args []any, column string, start, end time.Time, timezone string) (map[string]int64, error) {
	args = append(args, start.Unix(), end.Unix(), timezone)
	where = append(where, fmt.Sprintf("r.%s >= $%d AND r.%s < $%d", column, len(args)-2, column, len(args)-1))
	query := `SELECT r.type, to_char(to_timestamp(r.` + column + `) AT TIME ZONE $` + strconv.Itoa(len(args)) + `, 'YYYY-MM-DD') AS day, COUNT(*) AS count FROM ` + define.TableRequirements + ` r WHERE ` + strings.Join(where, ` AND `) + ` GROUP BY r.type, day ORDER BY day,r.type`
	rows, err := msql.RawValues(define.Postgres, query, nil, args...)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return nil, err
	}
	counts := map[string]int64{}
	for _, row := range rows {
		count, err := strconv.ParseInt(row["count"], 10, 64)
		if err != nil {
			return nil, err
		}
		counts[row["type"]+row["day"]] = count
	}
	return counts, nil
}

// appendDashboardChartDayBuckets emits one bucket per day and work item type, filling zero for days without data.
func appendDashboardChartDayBuckets(result *DashboardChartResultModel, counts map[string]int64, start, end time.Time, scope string) {
	for day := start; day.Before(end); day = day.AddDate(0, 0, 1) {
		for _, typ := range []string{"requirement", "defect"} {
			if scope != "all" && scope != typ {
				continue
			}
			result.Buckets = append(result.Buckets, DashboardChartBucketModel{Type: typ, Timestamp: day.Unix(), Count: counts[typ+day.Format("2006-01-02")]})
		}
	}
}

// QueryDashboardChart applies the same permissions and personal filters as numeric metrics.
func QueryDashboardChart(model DashboardChartQueryModel) (DashboardChartResultModel, error) {
	result := DashboardChartResultModel{Buckets: []DashboardChartBucketModel{}}
	dashboard, err := getDashboardForActor(model.AdminId, model.WorkspaceId, model.UserId, false, model.DashboardId)
	if err != nil {
		return result, err
	}
	raw := ""
	if model.PreviewConfig != nil {
		if !dashboard.CanEdit {
			return result, ErrDashboardForbidden
		}
		raw = *model.PreviewConfig
	} else {
		card, err := loadDashboardCard(model.AdminId, model.WorkspaceId, model.DashboardId, model.CardId)
		if err != nil {
			return result, err
		}
		if card.CardType != define.DashboardCardTypeChart {
			return result, ErrDashboardInvalid
		}
		raw = card.Config
	}
	c, err := parseDashboardChartConfig(model.AdminId, model.WorkspaceId, raw)
	if err != nil {
		return result, err
	}
	issueWorkspace, err := IsIssueWorkspace(model.AdminId, model.WorkspaceId)
	if err != nil {
		return result, err
	}
	slaChart := strings.HasPrefix(c.ChartType, "sla_")
	if (c.ChartType == "category_stats" || c.ChartType == "workload_stats" || slaChart) && !issueWorkspace {
		return result, ErrDashboardInvalid
	}
	if slaChart {
		metricQuery := DashboardMetricQueryModel{
			AdminId: model.AdminId, WorkspaceId: model.WorkspaceId, UserId: model.UserId,
			DashboardId: model.DashboardId, Timezone: model.Timezone,
		}
		where, args, err := dashboardWorkItemConditions(metricQuery, c.DataScope, "", issueWorkspace)
		if err != nil {
			return result, err
		}
		if (c.ChartType == "sla_risk" || c.ChartType == "sla_trend") && len(c.SLAStageIds) == 0 {
			stages, err := listSLADefinitionsTx(nil, model.AdminId, model.WorkspaceId, true)
			if err != nil {
				return result, err
			}
			c.slaStageNames = make(map[int]string, len(stages))
			for _, stage := range stages {
				c.SLAStageIds = append(c.SLAStageIds, stage.Id)
				c.slaStageNames[stage.Id] = stage.Name
			}
			if len(stages) == 0 {
				// 避免空阶段列表回退为扫描所有历史 SLA 周期。
				where = append(where, "FALSE")
			}
		}
		if c.ChartType == "sla_dimension" || c.ChartType == "sla_distribution" {
			from, to, err := dashboardSLAChartDateRange(model.Timezone, model.CategoryFilter)
			if err != nil {
				return result, err
			}
			where = append(where, fmt.Sprintf("r.created_at >= $%d AND r.created_at < $%d", len(args)+1, len(args)+2))
			args = append(args, from, to)
		}
		switch c.ChartType {
		case "sla_dimension":
			result.SLADimension, err = queryDashboardSLADimension(model, metricQuery, c, where, args)
			return result, err
		case "sla_distribution":
			result.SLADistribution, err = queryDashboardSLADistribution(metricQuery, c, where, args)
			return result, err
		case "sla_risk":
			result.SLARisk, err = queryDashboardSLARisk(metricQuery, c, model.SLARiskFilter, where, args)
			return result, err
		}
		metric, err := queryDashboardSLAMetric(metricQuery, DashboardMetricConfig{
			MetricType: define.DashboardMetricSLAAchievementRate, DataScope: c.DataScope,
			RecentDays: c.RecentDays, SLAStageIds: c.SLAStageIds,
		}, where, args, true, nil)
		result.SLATrend = metric.SLA
		return result, err
	}
	c.DataScope = dashboardDataScope(c.DataScope, issueWorkspace)
	if issueWorkspace && c.DataScope != "" {
		// 历史混合图表的显示范围可能包含缺陷元数据，取数时仅保留工单选项。
		if len(c.StatusIds) > 0 {
			statuses, err := ListStatuses(ListStatusesModel{AdminId: model.AdminId, WsId: strconv.Itoa(model.WorkspaceId), TypeKey: define.TypeKeyRequirement})
			if err != nil {
				return result, err
			}
			allowed := map[int]bool{}
			for _, status := range statuses.Items {
				allowed[status.Id] = true
			}
			c.StatusIds = slices.DeleteFunc(c.StatusIds, func(id int) bool { return !allowed[id] })
		}
		if len(c.PriorityIds) > 0 {
			priorities, err := ListPriorities(ListPrioritiesModel{AdminId: model.AdminId, WsId: strconv.Itoa(model.WorkspaceId), TypeKey: define.TypeKeyRequirement})
			if err != nil {
				return result, err
			}
			allowed := map[int]bool{0: true}
			for _, priority := range priorities.Items {
				allowed[priority.Id] = true
			}
			c.PriorityIds = slices.DeleteFunc(c.PriorityIds, func(id int) bool { return !allowed[id] })
		}
	}
	// 数据范围全部取消时该卡片不参与统计，返回空结果由前端展示空态。
	if c.DataScope == "" {
		return result, nil
	}
	loc, err := time.LoadLocation(model.Timezone)
	if err != nil {
		return result, ErrDashboardInvalid
	}
	where, args, err := dashboardWorkItemConditions(DashboardMetricQueryModel{AdminId: model.AdminId, WorkspaceId: model.WorkspaceId, UserId: model.UserId, DashboardId: model.DashboardId, Timezone: model.Timezone}, c.DataScope, "", issueWorkspace)
	if err != nil {
		return result, err
	}
	if c.ChartType == "workload_stats" {
		result.Workload, err = queryDashboardWorkload(model, c, where, args, loc)
		return result, err
	}
	if c.ChartType == "category_stats" {
		result.Categories, err = queryDashboardCategoryStats(model, c, where, args, loc)
		return result, err
	}
	if c.ChartType == "completion_line" {
		settings, err := GetWorkspaceSettings(WorkspaceSettingsModel{AdminId: model.AdminId, WsId: strconv.Itoa(model.WorkspaceId)})
		if err != nil {
			return result, err
		}
		args = append(args, pq.Array(settings.RequirementCompletedStatusIds), pq.Array(settings.DefectCompletedStatusIds))
		where = append(where, fmt.Sprintf("((r.type='requirement' AND r.status_id=ANY($%d::INTEGER[])) OR (r.type='defect' AND r.status_id=ANY($%d::INTEGER[])))", len(args)-1, len(args)))
		query := `SELECT COUNT(*) AS count FROM ` + define.TableRequirements + ` r WHERE ` + strings.Join(where, ` AND `) + ` AND r.completed_at=0`
		rows, err := msql.RawValues(define.Postgres, query, nil, args...)
		if err != nil {
			logs.Error("sql:%s, err:%s", query, err.Error())
			return result, err
		}
		if len(rows) != 1 {
			return result, ErrDashboardInvalid
		}
		result.UndatedCount, err = strconv.ParseInt(rows[0]["count"], 10, 64)
		if err != nil {
			return result, err
		}
		start, end := dashboardChartDayRange(loc, c.DaysFrom, c.DaysTo)
		counts, err := queryDashboardChartDailyCounts(where, args, "completed_at", start, end, model.Timezone)
		if err != nil {
			return result, err
		}
		appendDashboardChartDayBuckets(&result, counts, start, end, c.DataScope)
		return result, nil
	}
	// 每日新增趋势与完成趋势同构，区别只在于按创建时间分天且不附加最终状态条件。
	if c.ChartType == "daily_created_line" {
		start, end := dashboardChartDayRange(loc, c.DaysFrom, c.DaysTo)
		counts, err := queryDashboardChartDailyCounts(where, args, "created_at", start, end, model.Timezone)
		if err != nil {
			return result, err
		}
		appendDashboardChartDayBuckets(&result, counts, start, end, c.DataScope)
		return result, nil
	}
	// 数量排名只按处理人统计，不需要区分需求与缺陷两条序列。
	if c.ChartType == "assignee_rank_bar" {
		return queryDashboardChartAssigneeRank(where, args)
	}
	if c.ChartType == "iteration_burndown" {
		return queryDashboardChartBurndown(model, where, args, loc)
	}
	column, selected := "status_id", c.StatusIds
	if c.ChartType == "priority_bar" {
		column, selected = "priority", c.PriorityIds
	}
	if len(selected) > 0 {
		args = append(args, pq.Array(selected))
		where = append(where, fmt.Sprintf("r.%s=ANY($%d::INTEGER[])", column, len(args)))
	}
	query := `SELECT r.type,r.` + column + ` AS id,COUNT(*) AS count FROM ` + define.TableRequirements + ` r WHERE ` + strings.Join(where, ` AND `) + ` GROUP BY r.type,r.` + column
	rows, err := msql.RawValues(define.Postgres, query, nil, args...)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return result, err
	}
	counts := map[string]int64{}
	for _, row := range rows {
		count, err := strconv.ParseInt(row["count"], 10, 64)
		if err != nil {
			return result, err
		}
		counts[row["type"]+":"+row["id"]] = count
	}
	for _, typ := range []string{"requirement", "defect"} {
		if c.DataScope != "all" && c.DataScope != typ {
			continue
		}
		buckets := []DashboardChartBucketModel{}
		if column == "priority" {
			meta, err := ListPriorities(ListPrioritiesModel{AdminId: model.AdminId, WsId: strconv.Itoa(model.WorkspaceId), TypeKey: typ, Lang: model.Lang})
			if err != nil {
				return result, err
			}
			for _, p := range meta.Items {
				buckets = append(buckets, DashboardChartBucketModel{Id: p.Id, Name: p.Label, Color: p.Color, Type: typ})
			}
			buckets = append(buckets, DashboardChartBucketModel{Id: 0, Type: typ})
		} else {
			meta, err := ListStatuses(ListStatusesModel{AdminId: model.AdminId, WsId: strconv.Itoa(model.WorkspaceId), TypeKey: typ, Lang: model.Lang})
			if err != nil {
				return result, err
			}
			for _, p := range meta.Items {
				buckets = append(buckets, DashboardChartBucketModel{Id: p.Id, Name: p.Name, Color: p.Color, Type: typ})
			}
		}
		for _, b := range buckets {
			if len(selected) > 0 && !intSliceContains(selected, b.Id) {
				continue
			}
			b.Count = counts[typ+":"+strconv.Itoa(b.Id)]
			result.Buckets = append(result.Buckets, b)
		}
	}
	return result, nil
}

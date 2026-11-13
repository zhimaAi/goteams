package business

import (
	"fmt"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
)

// DashboardSLADistributionColors 保存首响、解决和未超时桶的颜色配置。
type DashboardSLADistributionColors struct {
	FirstResponse string `json:"first_response"`
	Resolution    string `json:"resolution"`
	OnTime        string `json:"on_time"`
}

func (colors DashboardSLADistributionColors) valid() bool {
	return validate.DashboardMetricStyle("", colors.FirstResponse) == nil &&
		validate.DashboardMetricStyle("", colors.Resolution) == nil &&
		validate.DashboardMetricStyle("", colors.OnTime) == nil
}

// DashboardSLADistributionBucketModel 表示一个所选阶段的超时工单数。
type DashboardSLADistributionBucketModel struct {
	Key, StageName string
	StageId        int
	Count          int64
}

// DashboardSLADistributionModel 按阶段统计超时工单，同一工单可计入多个阶段。
type DashboardSLADistributionModel struct {
	Enabled            bool
	EvaluatedAt, Total int64
	Buckets            []DashboardSLADistributionBucketModel
}

// 未配置阈值的历史明细卡和独立临界指标沿用一小时。
const dashboardSLADefaultRiskThresholdSeconds = 60 * 60

// DashboardSLARiskFilterModel 包含明细临时筛选与分页参数。
type DashboardSLARiskFilterModel struct {
	DashboardCategoryFilterModel
	Page, Size int
}

// DashboardSLARiskRowModel 是工单及其优先展示的风险阶段。
type DashboardSLARiskRowModel struct {
	Id                   int64
	Title, Risk          string
	PriorityId, StatusId int
	AssigneeIds          []int
	Stage                SLAStageResultModel
	StageTimes           []*int64
	StageResults         []DashboardSLARiskStageResultModel
	Seconds              int64
}

// DashboardSLARiskStageResultModel 保存阶段计时及其风险判定。
type DashboardSLARiskStageResultModel struct {
	*SLAStageResultModel
	Risk string
}

// DashboardSLARiskModel 将汇总与分页明细绑定到同一次统计。
type DashboardSLARiskModel struct {
	Stages                  []DashboardSLARiskStageModel
	Enabled                 bool
	EvaluatedAt             int64
	Total, Overdue, Started int64
	OverdueRate             *float64
	Page, Size              int
	ThresholdSeconds        int
	Rows                    []DashboardSLARiskRowModel
}

// DashboardSLARiskStageModel 保存一个所选阶段的名称和临界工单数。
type DashboardSLARiskStageModel struct {
	Id               int
	Name             string
	Critical         int64
	ThresholdSeconds int
}

func dashboardSLAStageRisk(stage SLAStageResultModel, thresholdSeconds int) (kind string, seconds int64) {
	if stage.CalculationState != "ready" || stage.ElapsedSeconds == nil || (stage.State != "running" && stage.State != "completed") {
		return "", 0
	}
	diff := stage.LimitSeconds - *stage.ElapsedSeconds
	if diff < 0 {
		return "overdue", -diff
	}
	if stage.State == "running" && diff <= int64(thresholdSeconds) {
		return "critical", diff
	}
	return "", 0
}

func countDashboardSLAOverdueStages(counts map[int]int64, stages []SLAStageResultModel) {
	for _, stage := range stages {
		if risk, _ := dashboardSLAStageRisk(stage, 0); risk == "overdue" {
			counts[int(stage.StageId)]++
		}
	}
}

func queryDashboardSLADistribution(model DashboardMetricQueryModel, config DashboardChartConfig, where []string, args []any) (*DashboardSLADistributionModel, error) {
	stageCounts := make(map[int]int64)
	result := &DashboardSLADistributionModel{}
	metric, err := queryDashboardSLAMetric(model, DashboardMetricConfig{MetricType: define.DashboardMetricSLAOverdueRate, DataScope: config.DataScope, SLAStageIds: config.SLAStageIds},
		where, args, false, &dashboardSLACollection{Collect: func(_ dashboardSLAItem, stages []SLAStageResultModel) {
			countDashboardSLAOverdueStages(stageCounts, stages)
		}})
	if err != nil {
		return nil, err
	}
	result.Enabled, result.EvaluatedAt = metric.SLA.Enabled, metric.SLA.EvaluatedAt
	for _, id := range config.SLAStageIds {
		count := stageCounts[id]
		result.Total += count
		result.Buckets = append(result.Buckets, DashboardSLADistributionBucketModel{
			Key: "stage", StageId: id, StageName: config.slaStageNames[id], Count: count,
		})
	}
	return result, nil
}

func dashboardSLARiskRow(item dashboardSLAItem, stages []SLAStageResultModel, summary []DashboardSLARiskStageModel) DashboardSLARiskRowModel {
	row := DashboardSLARiskRowModel{Id: item.Id, Title: item.Title, PriorityId: item.PriorityId, StatusId: item.StatusId,
		AssigneeIds: item.AssigneeIds, StageTimes: make([]*int64, len(stages)), StageResults: make([]DashboardSLARiskStageResultModel, len(stages))}
	for index, stage := range stages {
		risk, seconds := dashboardSLAStageRisk(stage, summary[index].ThresholdSeconds)
		if stage.StageId > 0 {
			row.StageResults[index] = DashboardSLARiskStageResultModel{SLAStageResultModel: &stages[index], Risk: risk}
		}
		if stage.CalculationState == "ready" && (stage.State == "running" || stage.State == "completed") {
			row.StageTimes[index] = stage.ElapsedSeconds
		}
		if risk == "critical" {
			summary[index].Critical++
		}
		if risk == "" {
			continue
		}
		higher := row.Risk == "" || (risk == "overdue" && row.Risk != risk)
		if risk == row.Risk {
			higher = (risk == "overdue" && seconds > row.Seconds) || (risk == "critical" && seconds < row.Seconds) ||
				(seconds == row.Seconds && stage.StageId < row.Stage.StageId)
		}
		if higher {
			row.Risk, row.Seconds, row.Stage = risk, seconds, stage
			row.Stage.StageName = summary[index].Name
		}
	}
	return row
}

func queryDashboardSLARisk(model DashboardMetricQueryModel, config DashboardChartConfig, filter *DashboardSLARiskFilterModel, where []string, args []any) (*DashboardSLARiskModel, error) {
	options := DashboardSLARiskFilterModel{}
	if filter != nil {
		options = *filter
	}
	if options.Page == 0 {
		options.Page = 1
	}
	if options.Size == 0 {
		options.Size = 20
	}
	if err := validate.DashboardSLARiskFilter(options.From, options.To, options.AssigneeId, options.PriorityId, options.Page, options.Size); err != nil {
		return nil, err
	}
	if options.From == 0 {
		timezone := model.Timezone
		if timezone == "" {
			timezone = "UTC"
		}
		loc, err := time.LoadLocation(timezone)
		if err != nil {
			return nil, ErrDashboardInvalid
		}
		start, end := dashboardChartDayRange(loc, 6, 0)
		options.From, options.To = start.Unix(), end.Unix()
	}
	where = append(append([]string{}, where...), fmt.Sprintf("r.created_at >= $%d AND r.created_at < $%d", len(args)+1, len(args)+2))
	args = append(append([]any{}, args...), options.From, options.To)
	if options.AssigneeId > 0 {
		args = append(args, options.AssigneeId)
		where = append(where, fmt.Sprintf("$%d = ANY(r.assignee_ids)", len(args)))
	}
	if options.PriorityId != nil {
		args = append(args, *options.PriorityId)
		where = append(where, fmt.Sprintf("COALESCE(r.priority,0) = $%d", len(args)))
	}
	thresholdSeconds := dashboardSLADefaultRiskThresholdSeconds
	if config.SLARiskThresholdMinutes != nil {
		thresholdSeconds = *config.SLARiskThresholdMinutes * 60
	}
	result := &DashboardSLARiskModel{Page: options.Page, Size: options.Size, ThresholdSeconds: thresholdSeconds, Rows: make([]DashboardSLARiskRowModel, 0, options.Size)}
	offset := int64((options.Page - 1) * options.Size)
	for _, id := range config.SLAStageIds {
		stageThreshold := thresholdSeconds
		if minutes, ok := config.SLARiskStageThresholdMinutes[id]; ok {
			stageThreshold = minutes * 60
		}
		result.Stages = append(result.Stages, DashboardSLARiskStageModel{
			Id: id, Name: config.slaStageNames[id], ThresholdSeconds: stageThreshold,
		})
	}
	stats := &DashboardSLAStatsModel{Enabled: true}
	metric, err := queryDashboardSLAMetric(model, DashboardMetricConfig{MetricType: define.DashboardMetricSLAOverdueRate, DataScope: config.DataScope, SLAStageIds: config.SLAStageIds},
		where, args, false, &dashboardSLACollection{Details: true, Collect: func(item dashboardSLAItem, stages []SLAStageResultModel) {
			addDashboardSLAItem(stats, define.DashboardMetricSLAOverdueRate, item.Resolved, stages)
			row := dashboardSLARiskRow(item, stages, result.Stages)
			if row.Risk != "" {
				if result.Total >= offset && len(result.Rows) < options.Size {
					result.Rows = append(result.Rows, row)
				}
				result.Total++
			}
		}})
	if err != nil {
		return nil, err
	}
	result.Enabled, result.EvaluatedAt = metric.SLA.Enabled, metric.SLA.EvaluatedAt
	result.Overdue, result.Started = stats.Overdue, stats.Started
	stats.Enabled = result.Enabled
	result.OverdueRate = dashboardSLAValue(define.DashboardMetricSLAOverdueRate, stats)
	return result, nil
}

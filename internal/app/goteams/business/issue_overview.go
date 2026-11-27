package business

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
)

// IssueOverviewQueryModel 是概览与实时明细的可信查询上下文。
type IssueOverviewQueryModel struct {
	AdminId, WorkspaceId, UserId     int
	Lang, Preset, From, To           string
	StatusId, PriorityId, CategoryId *int
	Metric, Search                   string
	PartId                           *int
	DetailStatusIds                  []int
	Page, Size                       int
}

// IssueOverviewCountsModel 区分存量、创建样本及当前状态。
type IssueOverviewCountsModel struct {
	Total, Finished, Pending, Processing, Completed, Closed, Unknown int64
}

// IssueOverviewGroupModel 保留每个分组的全部与完结数量。
type IssueOverviewGroupModel struct {
	Id               int
	Key, Name, Color string
	Total, Finished  int64
}

// IssueOverviewDayModel 是按真实事件日期补零后的趋势。
type IssueOverviewDayModel struct {
	Date                       string
	Created, Completed, Closed int64
}

// IssueOverviewStageModel 是每工单最新已完成周期的平均有效耗时。
type IssueOverviewStageModel struct {
	Id             int
	Name           string
	Samples        int64
	AverageSeconds *float64
}

// IssueOverviewItemModel 提供实时明细，不保存报表运行记录。
type IssueOverviewItemModel struct {
	Id                               int64
	Title                            string
	StatusId, PriorityId, CategoryId int
	AssigneeIds                      []int
	CreatedAt, EventAt               int64
	EventStatusId                    int
}

// IssueOverviewModel 是概览聚合与有界明细的业务结果。
type IssueOverviewModel struct {
	PendingPriorities                                                  []IssueOverviewGroupModel
	EventHistoryUnknown                                                int64
	EvaluatedAt                                                        int64
	Timezone, From, To                                                 string
	InitialStatusId                                                    int
	CompletedStatusIds, ClosedStatusIds                                []int
	All, Current                                                       IssueOverviewCountsModel
	Completed, PreviousCompleted, Overdue, PreviousOverdue             int64
	Achieved, Eligible, HistoryEligible, FirstResolved, HistoryUnknown int64
	SLAEnabled                                                         bool
	Categories, Priorities, Statuses, Members                          []IssueOverviewGroupModel
	Trend                                                              []IssueOverviewDayModel
	Stages                                                             []IssueOverviewStageModel
	List                                                               []IssueOverviewItemModel
	Total, Page, Size                                                  int
}

type overviewContext struct {
	model                IssueOverviewQueryModel
	settings             WorkspaceSettingsModel
	sla                  SLASettingsModel
	stages               []DashboardSLAStageModel
	options              SLAMetadataModel
	where                []string
	args                 []any
	start, end, previous time.Time
	now                  int64
}

func prepareIssueOverview(model IssueOverviewQueryModel) (overviewContext, error) {
	result := overviewContext{model: model}
	if err := validate.IssueStatisticsQuery("risk", "", model.Preset, model.From, model.To, 0, model.PriorityId, model.CategoryId, nil, model.Page, model.Size); err != nil {
		return result, err
	}
	if model.StatusId != nil && *model.StatusId <= 0 || len(model.Search) > 300 || len(model.DetailStatusIds) > 100 {
		return result, ErrDashboardInvalid
	}
	for _, id := range model.DetailStatusIds {
		if id <= 0 {
			return result, ErrDashboardInvalid
		}
	}
	switch model.Metric {
	case "", "created", "started", "unstarted", "finished", "unfinished", "pending", "processing", "completed":
	default:
		return result, ErrDashboardInvalid
	}
	if model.PartId != nil && (model.Metric != "pending" && model.Metric != "processing" || *model.PartId < 0) {
		return result, ErrDashboardInvalid
	}
	var err error
	result.sla, result.stages, err = issueStatisticsContext(model.AdminId, model.WorkspaceId, model.UserId)
	if err != nil {
		return result, err
	}
	result.settings, err = GetWorkspaceSettings(WorkspaceSettingsModel{AdminId: model.AdminId, WsId: strconv.Itoa(model.WorkspaceId)})
	if err != nil {
		return result, err
	}
	result.options, err = GetSLAMetadata(SLAMetadataModel{AdminId: model.AdminId, WorkspaceId: model.WorkspaceId})
	if err != nil {
		return result, err
	}
	contains := func(options []SLAOptionModel, id int) bool {
		for _, o := range options {
			if o.Id == id {
				return true
			}
		}
		return false
	}
	if model.StatusId != nil && !contains(result.options.Statuses, *model.StatusId) || model.PriorityId != nil && *model.PriorityId > 0 && !contains(result.options.Priorities, *model.PriorityId) {
		return result, ErrDashboardInvalid
	}
	loc, err := time.LoadLocation(result.sla.Timezone)
	if err != nil {
		return result, err
	}
	now := time.Now().In(loc)
	result.now = now.Unix()
	start, end, days, err := issueStatisticsRange(IssueStatisticsQueryModel{Preset: model.Preset, From: model.From, To: model.To}, now)
	if err != nil {
		return result, err
	}
	result.start, result.end, result.previous = start, end, start.AddDate(0, 0, -days)
	empty := ""
	result.where, result.args, err = dashboardWorkItemConditions(DashboardMetricQueryModel{AdminId: model.AdminId, WorkspaceId: model.WorkspaceId, UserId: model.UserId, Timezone: result.sla.Timezone, TopFilters: &empty}, define.DashboardMetricDataScopeRequirement, "", true)
	if err != nil {
		return result, err
	}
	result.where = append(result.where, "r.parent_id=0")
	for _, field := range []struct {
		name  string
		value *int
	}{{"status_id", model.StatusId}, {"priority", model.PriorityId}} {
		if field.value != nil {
			result.args = append(result.args, *field.value)
			result.where = append(result.where, fmt.Sprintf("COALESCE(r.%s,0)=$%d", field.name, len(result.args)))
		}
	}
	if model.CategoryId != nil {
		ids, e := issueStatisticsCategoryIds(model.AdminId, model.WorkspaceId, *model.CategoryId)
		if e != nil {
			return result, e
		}
		result.args = append(result.args, pq.Array(ids))
		result.where = append(result.where, fmt.Sprintf("COALESCE(r.category_id,0)=ANY($%d::int[])", len(result.args)))
	}
	return result, nil
}

func (c overviewContext) group(id int) string {
	if intSliceContains(c.settings.RequirementCompletedStatusIds, id) {
		return "completed"
	}
	if intSliceContains(c.settings.RequirementClosedStatusIds, id) {
		return "closed"
	}
	if id > 0 && id == c.settings.RequirementInitialStatusId {
		return "pending"
	}
	for _, s := range c.options.Statuses {
		if s.Id == id {
			return "processing"
		}
	}
	return "unknown"
}
func overviewAdd(count *IssueOverviewCountsModel, group string, n int64) {
	count.Total += n
	switch group {
	case "completed":
		count.Completed += n
		count.Finished += n
	case "closed":
		count.Closed += n
		count.Finished += n
	case "pending":
		count.Pending += n
	case "processing":
		count.Processing += n
	default:
		count.Unknown += n
	}
}
func overviewSQL(ctx context.Context, tx *sql.Tx, query string, args ...any) (*sql.Rows, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
	}
	return rows, err
}

// QueryIssueOverview 在相同只读快照中聚合可见工单及状态事件。
func QueryIssueOverview(parent context.Context, model IssueOverviewQueryModel) (IssueOverviewModel, error) {
	out := IssueOverviewModel{Page: model.Page, Size: model.Size}
	c, err := prepareIssueOverview(model)
	if err != nil {
		return out, err
	}
	out.EvaluatedAt, out.Timezone, out.From, out.To = c.now, c.sla.Timezone, c.start.Format(time.DateOnly), c.end.AddDate(0, 0, -1).Format(time.DateOnly)
	out.InitialStatusId = c.settings.RequirementInitialStatusId
	out.CompletedStatusIds = c.settings.RequirementCompletedStatusIds
	out.ClosedStatusIds = c.settings.RequirementClosedStatusIds
	out.SLAEnabled = c.sla.Enabled && len(c.stages) > 0
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return out, err
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return out, err
	}
	defer func() { _ = tx.Rollback() }()
	if model.Metric == "" {
		if err = overviewAggregate(ctx, tx, c, &out); err != nil {
			return out, err
		}
	}
	if err = overviewScan(ctx, tx, c, &out); err != nil {
		return out, err
	}
	if err = tx.Commit(); err != nil {
		return out, fmt.Errorf("commit overview read: %w", err)
	}
	return out, nil
}

func overviewAggregate(ctx context.Context, tx *sql.Tx, c overviewContext, out *IssueOverviewModel) error {
	args := append([]any{}, c.args...)
	args = append(args, c.start.Unix(), c.end.Unix(), c.sla.Timezone)
	n := len(c.args)
	query := fmt.Sprintf(`SELECT COALESCE(r.status_id,0),COALESCE(r.priority,0),COALESCE(r.category_id,0),
 CASE WHEN r.created_at >= $%d AND r.created_at < $%d THEN to_char(to_timestamp(r.created_at) AT TIME ZONE $%d,'YYYY-MM-DD') ELSE '' END AS day,COUNT(*)
 FROM %s r WHERE %s GROUP BY 1,2,3,4`, n+1, n+2, n+3, define.TableRequirements, strings.Join(c.where, " AND "))
	rows, err := overviewSQL(ctx, tx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	categories := append([]SLAOptionModel{}, c.options.Categories...)
	categories = append(categories, SLAOptionModel{})
	metadata := make([]RequirementCategoryModel, 0, len(categories))
	for _, v := range categories {
		metadata = append(metadata, RequirementCategoryModel{Id: v.Id, ParentId: v.ParentId, Name: v.Name})
	}
	categoryGroups, positions := selectDashboardSLAGroups(metadata, nil)
	for _, v := range categoryGroups {
		out.Categories = append(out.Categories, IssueOverviewGroupModel{Id: v.Id, Name: v.Name})
	}
	for _, v := range c.options.Priorities {
		out.Priorities = append(out.Priorities, IssueOverviewGroupModel{Id: v.Id, Name: v.Name, Color: v.Color})
	}
	out.Priorities = append(out.Priorities, IssueOverviewGroupModel{})
	out.PendingPriorities = append([]IssueOverviewGroupModel{}, out.Priorities...)
	for _, v := range c.options.Statuses {
		out.Statuses = append(out.Statuses, IssueOverviewGroupModel{Id: v.Id, Key: c.group(v.Id), Name: v.Name, Color: v.Color})
	}
	daily := map[string]int64{}
	for rows.Next() {
		var status, priority, category int
		var day string
		var count int64
		if err = rows.Scan(&status, &priority, &category, &day, &count); err != nil {
			return err
		}
		group := c.group(status)
		overviewAdd(&out.All, group, count)
		if day == "" {
			continue
		}
		overviewAdd(&out.Current, group, count)
		daily[day] += count
		finished := int64(0)
		if group == "completed" || group == "closed" {
			finished = count
		}
		position, ok := positions[category]
		if !ok {
			position = positions[0]
		}
		if position >= 0 && position < len(out.Categories) {
			out.Categories[position].Total += count
			out.Categories[position].Finished += finished
		}
		for i := range out.Priorities {
			if out.Priorities[i].Id == priority {
				out.Priorities[i].Total += count
				out.Priorities[i].Finished += finished
			}
		}
		if group == "pending" {
			for i := range out.PendingPriorities {
				if out.PendingPriorities[i].Id == priority {
					out.PendingPriorities[i].Total += count
				}
			}
		}
		for i := range out.Statuses {
			if out.Statuses[i].Id == status {
				out.Statuses[i].Total += count
				out.Statuses[i].Finished += finished
			}
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for day := c.start; day.Before(c.end); day = day.AddDate(0, 0, 1) {
		date := day.Format(time.DateOnly)
		out.Trend = append(out.Trend, IssueOverviewDayModel{Date: date, Created: daily[date]})
	}
	sort.SliceStable(out.Categories, func(i, j int) bool { return out.Categories[i].Total > out.Categories[j].Total })
	query = fmt.Sprintf(`SELECT a.id,COALESCE(NULLIF(u.name,''),NULLIF(ru.name,''),'#'||a.id::text),COUNT(DISTINCT r.id) FROM %s r
 CROSS JOIN LATERAL (SELECT DISTINCT unnest(r.assignee_ids) AS id) a
 LEFT JOIN %s u ON u.admin_id=r.admin_id AND u.id=a.id
 LEFT JOIN %s ru ON ru.admin_id=r.admin_id AND ru.user_id=a.id
 WHERE %s AND r.created_at >= $%d AND r.created_at < $%d GROUP BY a.id,u.name,ru.name ORDER BY COUNT(DISTINCT r.id) DESC,a.id`, define.TableRequirements, define.TableUsers, define.TableRemovedUsers, strings.Join(c.where, " AND "), n+1, n+2)
	members, err := overviewSQL(ctx, tx, query, args[:n+2]...)
	if err != nil {
		return err
	}
	defer members.Close()
	for members.Next() {
		var item IssueOverviewGroupModel
		if err = members.Scan(&item.Id, &item.Name, &item.Total); err != nil {
			return err
		}
		out.Members = append(out.Members, item)
	}
	return members.Err()
}

type overviewEvent struct {
	At       int64
	From, To int
	Created  bool
	Known    bool
}
type overviewHistoryResult struct {
	Complete, EverCompleted, Direct, Reopened bool
	Events                                    []overviewEvent
}

func overviewHistory(c overviewContext, item IssueOverviewItemModel, events []overviewEvent) overviewHistoryResult {
	result := overviewHistoryResult{Complete: true}
	previous := 0
	created := false
	changed := false
	for _, event := range events {
		if !event.Known {
			result.Complete = false
			previous = 0
			continue
		}
		if c.group(event.To) == "unknown" {
			result.Complete = false
		}
		if event.Created {
			if created || previous > 0 {
				result.Complete = false
			}
			created = true
			previous = event.To
			if c.group(event.To) == "completed" {
				result.EverCompleted = true
				result.Direct = true
				result.Events = append(result.Events, event)
			} else if c.group(event.To) == "closed" {
				result.Events = append(result.Events, event)
			}
			continue
		}
		from := event.From
		if previous > 0 {
			from = previous
		}
		if from <= 0 {
			result.Complete = false
			previous = event.To
			continue
		}
		event.From = from
		previous = event.To
		if from == event.To {
			continue
		}
		fromGroup, toGroup := c.group(from), c.group(event.To)
		if (fromGroup == "completed" || fromGroup == "closed") && toGroup != "completed" && toGroup != "closed" {
			result.Reopened = true
		}
		if toGroup == "completed" {
			result.EverCompleted = true
			if !changed {
				result.Direct = true
			}
		}
		changed = true
		if toGroup == "completed" && fromGroup != "completed" || toGroup == "closed" && fromGroup != "closed" {
			result.Events = append(result.Events, event)
		}
	}
	result.Complete = result.Complete && created && previous == item.StatusId
	return result
}

func overviewReadHistory(ctx context.Context, tx *sql.Tx, c overviewContext, items []IssueOverviewItemModel) (map[int64]overviewHistoryResult, error) {
	ids := make([]int64, len(items))
	for i, item := range items {
		ids[i] = item.Id
	}
	query := `SELECT requirement_id,action,field_name,created_at,COALESCE((processing_snapshot->>'status_id')::int,0),old_value,new_value FROM ` + define.TableRequirementHistories + `
 WHERE admin_id=$1 AND workspace_id=$2 AND requirement_id=ANY($3) AND created_at<=$4
 AND (action IN ('created','status_change') OR (action='issue_recall' AND field_name IN ('status','status_id')) OR field_name IN ('workspace_id','type')) ORDER BY requirement_id,created_at,id`
	rows, err := overviewSQL(ctx, tx, query, c.model.AdminId, c.model.WorkspaceId, pq.Array(ids), c.now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := map[string]int{}
	for _, s := range c.options.Statuses {
		if _, ok := names[s.Name]; ok {
			names[s.Name] = 0
		} else {
			names[s.Name] = s.Id
		}
	}
	timelines := map[int64][]overviewEvent{}
	for rows.Next() {
		var id, at int64
		var action, field, oldValue, newValue string
		var status int
		if err = rows.Scan(&id, &action, &field, &at, &status, &oldValue, &newValue); err != nil {
			return nil, err
		}
		event := overviewEvent{At: at, From: names[oldValue], To: status, Created: action == "created"}
		if event.To == 0 && !event.Created {
			event.To = names[newValue]
		}
		event.Known = event.To > 0 && field != "workspace_id" && field != "type"
		timelines[id] = append(timelines[id], event)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	result := make(map[int64]overviewHistoryResult, len(items))
	for _, item := range items {
		result[item.Id] = overviewHistory(c, item, timelines[item.Id])
	}
	return result, nil
}

func overviewMatches(c overviewContext, item IssueOverviewItemModel, h overviewHistoryResult) (bool, int64, int) {
	if c.model.Search != "" && !strings.Contains(strings.ToLower(item.Title), strings.ToLower(c.model.Search)) {
		return false, 0, 0
	}
	matchesStatus := func(id int) bool {
		return len(c.model.DetailStatusIds) == 0 || intSliceContains(c.model.DetailStatusIds, id)
	}
	if c.model.Metric == "completed" {
		var at int64
		var status int
		for _, e := range h.Events {
			if c.group(e.To) == "completed" && e.At >= c.start.Unix() && e.At < c.end.Unix() && matchesStatus(e.To) {
				at, status = e.At, e.To
			}
		}
		return at > 0, at, status
	}
	if item.CreatedAt < c.start.Unix() || item.CreatedAt >= c.end.Unix() || !matchesStatus(item.StatusId) {
		return false, 0, 0
	}
	group := c.group(item.StatusId)
	match := false
	switch c.model.Metric {
	case "created":
		match = true
	case "finished":
		match = group == "completed" || group == "closed"
	case "unfinished":
		match = group != "completed" && group != "closed"
	case "started":
		match = group != "pending" && group != "unknown"
	case "unstarted", "pending":
		match = group == "pending"
	case "processing":
		match = group == "processing"
	}
	if c.model.PartId != nil {
		if c.model.Metric == "pending" {
			match = match && item.PriorityId == *c.model.PartId
		} else {
			match = match && item.StatusId == *c.model.PartId
		}
	}
	return match, 0, 0
}

func overviewScan(ctx context.Context, tx *sql.Tx, c overviewContext, out *IssueOverviewModel) error {
	stages := make(map[int]int, len(c.stages))
	for _, s := range c.stages {
		stages[s.Id] = len(out.Stages)
		out.Stages = append(out.Stages, IssueOverviewStageModel{Id: s.Id, Name: s.Name})
	}
	sums := make([]int64, len(out.Stages))
	dailyCompleted, dailyClosed := map[string]int64{}, map[string]int64{}
	for cursor := int64(1<<63 - 1); cursor > 0; {
		if err := ctx.Err(); err != nil {
			return err
		}
		args := append([]any{}, c.args...)
		args = append(args, c.previous.Unix(), c.end.Unix(), cursor)
		n := len(c.args)
		candidate := fmt.Sprintf(`(r.created_at >= $%d AND r.created_at < $%d OR EXISTS (SELECT 1 FROM %s h WHERE h.admin_id=r.admin_id AND h.workspace_id=r.workspace_id AND h.requirement_id=r.id AND h.created_at >= $%d AND h.created_at < $%d AND h.action IN ('created','status_change','issue_recall')))`, n+1, n+2, define.TableRequirementHistories, n+1, n+2)
		if c.model.Metric != "" && c.model.Metric != "completed" {
			candidate = fmt.Sprintf("r.created_at >= $%d AND r.created_at < $%d", n+1, n+2)
			args[n] = c.start.Unix()
		}
		query := fmt.Sprintf(`SELECT r.id,r.title,COALESCE(r.status_id,0),COALESCE(r.priority,0),COALESCE(r.category_id,0),COALESCE(r.assignee_ids,'{}'),r.created_at FROM %s r WHERE %s AND %s AND r.id <= $%d::bigint ORDER BY r.id DESC LIMIT 200`, define.TableRequirements, strings.Join(c.where, " AND "), candidate, n+3)
		rows, err := overviewSQL(ctx, tx, query, args...)
		if err != nil {
			return err
		}
		items := make([]IssueOverviewItemModel, 0, 200)
		for rows.Next() {
			var item IssueOverviewItemModel
			var assignees pq.Int64Array
			if err = rows.Scan(&item.Id, &item.Title, &item.StatusId, &item.PriorityId, &item.CategoryId, &assignees, &item.CreatedAt); err != nil {
				rows.Close()
				return err
			}
			for _, id := range assignees {
				item.AssigneeIds = append(item.AssigneeIds, int(id))
			}
			items = append(items, item)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if len(items) == 0 {
			break
		}
		histories := map[int64]overviewHistoryResult{}
		if c.model.Metric == "" || c.model.Metric == "completed" {
			histories, err = overviewReadHistory(ctx, tx, c, items)
			if err != nil {
				return err
			}
		}
		if c.model.Metric == "" && out.SLAEnabled {
			if err = overviewSLA(ctx, tx, c, items, out, stages, sums); err != nil {
				return err
			}
		}
		for _, item := range items {
			h := histories[item.Id]
			if !h.Complete && c.model.Metric == "" {
				out.EventHistoryUnknown++
			}
			if c.model.Metric != "" {
				match, at, status := overviewMatches(c, item, h)
				if match {
					if out.Total >= (c.model.Page-1)*c.model.Size && len(out.List) < c.model.Size {
						item.EventAt, item.EventStatusId = at, status
						out.List = append(out.List, item)
					}
					out.Total++
				}
				continue
			}
			seenCompleted, seenClosed := map[string]bool{}, map[string]bool{}
			completed, previous := false, false
			for _, event := range h.Events {
				if c.group(event.To) == "completed" {
					if event.At >= c.previous.Unix() && event.At < c.start.Unix() {
						previous = true
					}
					if event.At >= c.start.Unix() && event.At < c.end.Unix() {
						completed = true
						date := time.Unix(event.At, 0).In(c.start.Location()).Format(time.DateOnly)
						if !seenCompleted[date] {
							dailyCompleted[date]++
							seenCompleted[date] = true
						}
					}
				} else if c.group(event.To) == "closed" && event.At >= c.start.Unix() && event.At < c.end.Unix() {
					date := time.Unix(event.At, 0).In(c.start.Location()).Format(time.DateOnly)
					if !seenClosed[date] {
						dailyClosed[date]++
						seenClosed[date] = true
					}
				}
			}
			if completed {
				out.Completed++
			}
			if previous {
				out.PreviousCompleted++
			}
			if item.CreatedAt >= c.start.Unix() && item.CreatedAt < c.end.Unix() {
				if !h.Complete {
					out.HistoryUnknown++
				} else if h.EverCompleted {
					out.HistoryEligible++
					if h.Direct && !h.Reopened {
						out.FirstResolved++
					}
				}
			}
		}
		cursor = items[len(items)-1].Id - 1
	}
	for i := range out.Trend {
		out.Trend[i].Completed = dailyCompleted[out.Trend[i].Date]
		out.Trend[i].Closed = dailyClosed[out.Trend[i].Date]
	}
	for i := range out.Stages {
		if out.Stages[i].Samples > 0 {
			value := float64(sums[i]) / float64(out.Stages[i].Samples)
			out.Stages[i].AverageSeconds = &value
		}
	}
	return nil
}

func overviewSLA(ctx context.Context, tx *sql.Tx, c overviewContext, items []IssueOverviewItemModel, out *IssueOverviewModel, positions map[int]int, sums []int64) error {
	ids := []int64{}
	byId := map[int64]IssueOverviewItemModel{}
	for _, item := range items {
		if item.CreatedAt >= c.previous.Unix() && item.CreatedAt < c.end.Unix() {
			ids = append(ids, item.Id)
			byId[item.Id] = item
		}
	}
	if len(ids) == 0 {
		return nil
	}
	stageIds := make([]int, len(c.stages))
	for i, s := range c.stages {
		stageIds[i] = s.Id
	}
	pauses, err := loadSLAPausesTx(tx, c.model.AdminId, c.model.WorkspaceId)
	if err != nil {
		return err
	}
	// 当前SLA与已完成均值分开选周期，运行中周期不能遮掉最近已完成样本。
	for _, completedOnly := range []bool{false, true} {
		filter, order := "", "(state='running') DESC,id DESC"
		if completedOnly {
			filter = " AND state='completed'"
			order = "id DESC"
		}
		query := `SELECT DISTINCT ON(requirement_id,stage_id) requirement_id,` + slaCycleColumns + ` FROM ` + define.TableSLACycles + ` WHERE admin_id=$1 AND workspace_id=$2 AND requirement_id=ANY($3) AND stage_id=ANY($4)` + filter + ` ORDER BY requirement_id,stage_id,` + order
		rows, err := slaRows(tx, query, c.model.AdminId, c.model.WorkspaceId, pq.Array(ids), pq.Array(stageIds))
		if err != nil {
			return err
		}
		current := map[int64]map[int64]SLAStageResultModel{}
		for _, row := range rows {
			id, err := slaNumber(row, "requirement_id")
			if err != nil {
				return err
			}
			cycle, err := slaCycleFromRow(row)
			if err != nil {
				return err
			}
			stage, err := slaCycleProjection(cycle, pauses, c.now)
			if err != nil {
				return err
			}
			if completedOnly {
				if byId[id].CreatedAt >= c.start.Unix() && stage.CalculationState == "ready" && stage.ElapsedSeconds != nil {
					index := positions[int(stage.StageId)]
					out.Stages[index].Samples++
					sums[index] += *stage.ElapsedSeconds
				}
				continue
			}
			if current[id] == nil {
				current[id] = map[int64]SLAStageResultModel{}
			}
			current[id][stage.StageId] = stage
		}
		if completedOnly {
			continue
		}
		for id, values := range current {
			item := byId[id]
			stages := dashboardSLAItemStages(stageIds, values)
			group := c.group(item.StatusId)
			eligible, achieved, _ := dashboardSLAOutcome(group == "completed" || group == "closed", stages)
			overdue := false
			for _, stage := range stages {
				risk, _ := dashboardSLAStageRisk(stage, 0)
				overdue = overdue || risk == "overdue"
			}
			if item.CreatedAt < c.start.Unix() {
				if overdue {
					out.PreviousOverdue++
				}
				continue
			}
			if overdue {
				out.Overdue++
			}
			if eligible {
				out.Eligible++
			}
			if achieved {
				out.Achieved++
			}
		}
	}
	return nil
}

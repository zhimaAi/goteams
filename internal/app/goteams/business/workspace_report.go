package business

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/zhimaAi/go_tools/msql"
)

// Workspace report kinds and permissions.
const (
	ReportWorkHours = "work-hours"
	ReportDefects   = "defects"
	ReportView      = "report:view"
	ReportGenerate  = "report:generate"
	ReportExport    = "report:export"
)

var (
	// ErrReportInvalid 表示条件、日期或请求标识无效。
	ErrReportInvalid = errors.New("invalid report conditions")
	// ErrReportNotFound 表示报表不存在或已不在用户可见范围内。
	ErrReportNotFound = errors.New("report not found")
	// ErrReportConflict 表示同一个请求标识已用于不同条件。
	ErrReportConflict = errors.New("report request already exists")
	// ErrReportTooLarge 表示报表数据超过同步处理上限。
	ErrReportTooLarge = errors.New("report exceeds work item limit")
)

// ReportScope 保存认证后取得的工作区和用户上下文。
type ReportScope struct {
	AdminId, WorkspaceId, UserId int
	Type                         string
	Types                        []string
}

// WorkItemType 返回报表使用的工作项类型。
func (scope ReportScope) WorkItemType() string {
	if scope.Type == ReportDefects {
		return define.TypeKeyDefect
	}
	return define.TypeKeyRequirement
}

// WorkItemTypes 返回认证入口确定的可见工作项类型。
func (scope ReportScope) WorkItemTypes() []string {
	if len(scope.Types) > 0 {
		return scope.Types
	}
	return []string{scope.WorkItemType()}
}

// Dimensions 返回当前权限与应用开关允许的统计维度。
func (scope ReportScope) Dimensions() []string {
	if scope.Type == ReportWorkHours {
		return scope.WorkItemTypes()
	}
	return ReportDimensions(scope.Type)
}

// ReportOption 保存工作区内的筛选项。JSON 标签用于数据库快照格式。
type ReportOption struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

// ReportFilters 保存归一后的筛选项。JSON 标签用于数据库快照格式。
type ReportFilters struct {
	Preset     string         `json:"preset"`
	Dimensions []string       `json:"dimensions"`
	Statuses   []ReportOption `json:"statuses"`
	Roles      []ReportOption `json:"roles"`
	Members    []ReportOption `json:"members"`
}

// ReportRow 保存汇总分组。JSON 标签用于数据库快照格式。
type ReportRow struct {
	Key        string         `json:"key"`
	Dimension  string         `json:"dimension"`
	Name       string         `json:"name"`
	Count      int            `json:"count"`
	Hundredths *int64         `json:"hundredths"`
	Assignees  []string       `json:"assignees,omitempty"`
	Values     []ReportOption `json:"values,omitempty"`
}

// ReportRecord 保存可见的报表记录和汇总。
type ReportRecord struct {
	SnapshotVersion           int
	Id, CreatedAt, Start, End int64
	Creator, Timezone         string
	Filters                   ReportFilters
	Rows                      []ReportRow
}

// ReportItem 保存工作项的必要快照字段。JSON 标签用于数据库快照格式。
type ReportItem struct {
	StatusEventId     int64    `json:"status_event_id,omitempty"`
	StatusChangedAt   int64    `json:"status_changed_at,omitempty"`
	CountedHundredths *int64   `json:"-"`
	CountedAssignees  []string `json:"-"`
	Id                int      `json:"id"`
	Type              string   `json:"type,omitempty"`
	Title             string   `json:"title"`
	Status            string   `json:"status"`
	StatusId          int      `json:"status_id"`
	Template          string   `json:"template"`
	Iteration         string   `json:"iteration"`
	Category          string   `json:"category"`
	Priority          string   `json:"priority"`
	PriorityId        int      `json:"priority_id"`
	Severity          string   `json:"severity"`
	Estimated         float64  `json:"estimated"`
	Assignees         string   `json:"assignees"`
	AssigneeIds       []int    `json:"assignee_ids"`
	ResponsibleNames  []string `json:"responsible_names"`
	CategoryId        int      `json:"-"`
	CreatedAt         int64    `json:"created_at"`
}

// ReportOptions 保存当前报表可用的状态、角色和分页成员选项。
type ReportOptions struct {
	Statuses, Priorities, Roles, Members []ReportOption
	MemberTotal                          int
}

// GenerateReportModel 保存一次生成的原始条件。
type GenerateReportModel struct {
	ReportScope
	RequestId, Preset, StartDate, EndDate, Timezone string
	Dimensions                                      []string
	StatusIds, RoleIds, MemberIds                   []int
}

type reportInput struct {
	filters    ReportFilters
	start, end int64
	hash       [32]byte
}

type reportGroupedItem struct {
	item          ReportItem
	contributions []ReportRow
}

// ReportEstimateSnapshotVersion 标记按状态变更统计的预估工时报表。
const ReportEstimateSnapshotVersion = 2

// ReportStatisticsSnapshotVersion 标记多类型工时与缺陷组合计数快照。
const ReportStatisticsSnapshotVersion = 3

const maxReportItems = 10000
const maxReportSourceRows = 100000

// ReportDimensions 返回当前报表支持的统计维度。
func ReportDimensions(kind string) []string {
	if kind == ReportWorkHours {
		return []string{"requirement", "defect"}
	}
	return []string{"responsible", "severity", "priority", "category"}
}

// ReportDimensionName 返回维度的本地化名称。
func ReportDimensionName(lang, dimension string) string {
	return lib_i18n.Show(lang, "report_dimension_"+dimension)
}

func normalizeReportInput(model GenerateReportModel, now time.Time) (reportInput, error) {
	if len(model.Timezone) > 100 || model.Timezone == "" || model.Timezone == "Local" {
		return reportInput{}, ErrReportInvalid
	}
	tz, err := time.LoadLocation(model.Timezone)
	if err != nil {
		return reportInput{}, ErrReportInvalid
	}
	now = now.In(tz)
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, tz)
	var start, end time.Time
	switch model.Preset {
	case "this-month":
		start = month
		end = time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, tz)
	case "last-month":
		start, end = month.AddDate(0, -1, 0), month
	case "custom":
		start, err = time.ParseInLocation(time.DateOnly, model.StartDate, tz)
		if err != nil {
			return reportInput{}, ErrReportInvalid
		}
		lastDay, parseErr := time.ParseInLocation(time.DateOnly, model.EndDate, tz)
		if parseErr != nil || lastDay.Before(start) || !lastDay.Before(start.AddDate(0, 0, 366)) {
			return reportInput{}, ErrReportInvalid
		}
		end = lastDay.AddDate(0, 0, 1)
	default:
		return reportInput{}, ErrReportInvalid
	}
	if model.Preset != "custom" {
		model.StartDate, model.EndDate = "", ""
	}
	available := model.ReportScope.Dimensions()
	if len(model.Dimensions) == 0 || len(model.Dimensions) > len(available) {
		return reportInput{}, ErrReportInvalid
	}
	var dimensions []string
	for _, dimension := range model.Dimensions {
		if !slices.Contains(available, dimension) {
			return reportInput{}, ErrReportInvalid
		}
	}
	for _, dimension := range available {
		if slices.Contains(model.Dimensions, dimension) {
			dimensions = append(dimensions, dimension)
		}
	}
	model.Dimensions = dimensions
	for _, ids := range []*[]int{&model.StatusIds, &model.RoleIds, &model.MemberIds} {
		if len(*ids) > 500 {
			return reportInput{}, ErrReportInvalid
		}
		*ids = append([]int{}, *ids...)
		slices.Sort(*ids)
		*ids = slices.Compact(*ids)
		if len(*ids) > 0 && (*ids)[0] <= 0 {
			return reportInput{}, ErrReportInvalid
		}
	}
	filters := ReportFilters{Preset: model.Preset, Dimensions: dimensions, Statuses: reportIds(model.StatusIds), Roles: reportIds(model.RoleIds), Members: reportIds(model.MemberIds)}
	fingerprint, err := json.Marshal(struct {
		Filters          ReportFilters
		Start, End, Zone string
	}{filters, model.StartDate, model.EndDate, model.Timezone})
	if err != nil {
		return reportInput{}, err
	}
	return reportInput{filters: filters, start: start.Unix(), end: end.Unix(), hash: sha256.Sum256(fingerprint)}, nil
}

func reportIds(ids []int) []ReportOption {
	options := make([]ReportOption, len(ids))
	for i, id := range ids {
		options[i].Id = id
	}
	return options
}

func reportOptionIds(options []ReportOption) []int {
	ids := make([]int, len(options))
	for i, option := range options {
		ids[i] = option.Id
	}
	return ids
}

const reportStatuses = `SELECT id, COALESCE(name, '') FROM ` + define.TableWorkItemStatuses + ` WHERE admin_id=$1 AND workspace_id=$2 AND type_key=ANY($3::text[])`
const reportPriorities = `SELECT id, COALESCE(name, '') FROM ` + define.TableWorkItemPriorities + ` WHERE admin_id=$1 AND workspace_id=$2 AND type_key=ANY($3::text[])`
const reportRoles = `SELECT id, name FROM ` + define.TableRoles + ` WHERE admin_id=$1 AND workspace_id=$2 AND scope='project'`
const reportMembers = `SELECT wm.user_id, COALESCE(u.name, '') FROM ` + define.TableWorkspaceMembers + ` wm
	JOIN ` + define.TableUsers + ` u ON u.admin_id=wm.admin_id AND u.id=wm.user_id
	WHERE wm.admin_id=$1 AND wm.workspace_id=$2`

type reportQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func readReportOptions(ctx context.Context, db reportQueryer, query string, args ...any) ([]ReportOption, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	options := []ReportOption{}
	for rows.Next() {
		var option ReportOption
		if err = rows.Scan(&option.Id, &option.Name); err != nil {
			return nil, err
		}
		options = append(options, option)
	}
	return options, rows.Err()
}

// GetReportOptionsModel 保存报表筛选项查询、工作项类型与成员角色条件。
type GetReportOptionsModel struct {
	ReportScope
	Search, ItemType string
	Page, Size       int
	RoleIds          []int
}

// GetReportOptions 查询当前工作区的报表筛选项。
func GetReportOptions(ctx context.Context, model GetReportOptionsModel) (ReportOptions, error) {
	if len(model.RoleIds) > 500 || slices.ContainsFunc(model.RoleIds, func(id int) bool { return id <= 0 }) {
		return ReportOptions{}, ErrReportInvalid
	}
	scope := model.ReportScope
	types := scope.WorkItemTypes()
	if model.ItemType != "" {
		if !slices.Contains(types, model.ItemType) {
			return ReportOptions{}, ErrReportInvalid
		}
		types = []string{model.ItemType}
	}
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return ReportOptions{}, err
	}
	var result ReportOptions
	result.Statuses, err = readReportOptions(ctx, db, reportStatuses+` ORDER BY sort_order, id`, scope.AdminId, scope.WorkspaceId, pq.Array(types))
	if err != nil {
		return ReportOptions{}, err
	}
	result.Priorities, err = readReportOptions(ctx, db, reportPriorities+` ORDER BY id`, scope.AdminId, scope.WorkspaceId, pq.Array(types))
	if err != nil {
		return ReportOptions{}, err
	}
	result.Roles, err = readReportOptions(ctx, db, reportRoles+` ORDER BY id`, scope.AdminId, scope.WorkspaceId)
	if err != nil {
		return ReportOptions{}, err
	}
	for _, id := range model.RoleIds {
		if !slices.ContainsFunc(result.Roles, func(role ReportOption) bool { return role.Id == id }) {
			return ReportOptions{}, ErrReportInvalid
		}
	}
	filter := ` AND strpos(lower(COALESCE(u.name, '')), lower($3)) > 0
		AND (COALESCE(cardinality($4::int[]),0)=0 OR wm.role_id=ANY($4))`
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+reportMembers+filter+`) selected`, scope.AdminId, scope.WorkspaceId, model.Search, pq.Array(model.RoleIds)).Scan(&result.MemberTotal)
	if err != nil {
		return ReportOptions{}, err
	}
	result.Members, err = readReportOptions(ctx, db, reportMembers+filter+` ORDER BY u.name, wm.user_id LIMIT $5 OFFSET $6`, scope.AdminId, scope.WorkspaceId, model.Search, pq.Array(model.RoleIds), model.Size, (model.Page-1)*model.Size)
	return result, err
}

func resolveReportFilters(ctx context.Context, tx *sql.Tx, scope ReportScope, filters *ReportFilters) error {
	types := scope.WorkItemTypes()
	if scope.Type == ReportWorkHours {
		types = filters.Dimensions
	}
	for _, lookup := range []struct {
		options *[]ReportOption
		query   string
		args    []any
	}{
		{&filters.Statuses, reportStatuses + ` AND id=ANY($4) ORDER BY id`, []any{scope.AdminId, scope.WorkspaceId, pq.Array(types)}},
		{&filters.Roles, reportRoles + ` AND id=ANY($3) ORDER BY id`, []any{scope.AdminId, scope.WorkspaceId}},
		{&filters.Members, reportMembers + ` AND wm.user_id=ANY($3) ORDER BY wm.user_id`, []any{scope.AdminId, scope.WorkspaceId}},
	} {
		if len(*lookup.options) == 0 {
			continue
		}
		options, err := readReportOptions(ctx, tx, lookup.query, append(lookup.args, pq.Array(reportOptionIds(*lookup.options)))...)
		if err != nil {
			return err
		}
		if len(options) != len(*lookup.options) {
			return ErrReportInvalid
		}
		*lookup.options = options
	}
	return nil
}

const reportSourceFields = `r.id, r.title, event.status_name, event.status_id,
	COALESCE(r.template_name,''), COALESCE(v.name,''), COALESCE(c.name,''),
	COALESCE(p.name,''), COALESCE(sev.name,''), COALESCE(r.work_hours,0),
	COALESCE(array_to_string(ARRAY(SELECT jsonb_array_elements_text(
		COALESCE(event.snapshot->'assignee_names','[]'::jsonb))), ', '), ''),
	r.created_at, COALESCE(r.priority,0), COALESCE(r.severity_id,0),
	event.id, event.created_at,
	COALESCE(assigned.id, CASE WHEN jsonb_typeof(event.snapshot->'assignee_ids')='array' THEN 0 ELSE -1 END),
	COALESCE(NULLIF(assigned.name,''), '#' || assigned.id::text, ''),
	(COALESCE(r.work_hours,0)*100)::bigint,
    COALESCE(event.snapshot->'responsible_ids','null'::jsonb)::text,
    COALESCE(event.snapshot->'responsible_names','null'::jsonb)::text, r.type, COALESCE(r.category_id,0),
    COALESCE(event.snapshot->'assignee_ids','null'::jsonb)::text`

const reportSourceJoins = `
	LEFT JOIN ` + define.TableVersions + ` v ON v.id=r.version_id AND v.admin_id=r.admin_id
	LEFT JOIN ` + define.TableRequirementCategories + ` c ON c.id=r.category_id AND c.admin_id=r.admin_id AND c.workspace_id=r.workspace_id
	LEFT JOIN ` + define.TableWorkItemPriorities + ` p ON p.id=r.priority AND p.admin_id=r.admin_id AND p.workspace_id=r.workspace_id AND p.type_key=r.type
	LEFT JOIN ` + define.TableWorkspaceSeverities + ` sev ON sev.id=r.severity_id AND sev.admin_id=r.admin_id AND sev.workspace_id=r.workspace_id
	JOIN LATERAL (
		SELECT h.id, h.created_at, h.processing_snapshot AS snapshot,
			COALESCE((h.processing_snapshot->>'status_id')::int, legacy.id, 0) AS status_id,
			COALESCE(NULLIF(h.processing_snapshot->>'status_name',''), legacy.name,'') AS status_name
		FROM ` + define.TableRequirementHistories + ` h
		LEFT JOIN LATERAL (
			SELECT min(s.id) AS id, min(s.name) AS name
			FROM ` + define.TableWorkItemStatuses + ` s
			WHERE s.admin_id=r.admin_id AND s.workspace_id=r.workspace_id AND s.type_key=r.type AND s.name=h.new_value
			HAVING count(*)=1
		) legacy ON h.processing_snapshot->>'status_id' IS NULL AND h.action<>'created'
		WHERE h.admin_id=r.admin_id AND h.workspace_id=r.workspace_id AND h.requirement_id=r.id
			AND (h.action IN ('status_change','created')
				OR (h.action='issue_recall' AND h.field_name IN ('status','status_id')))
			AND h.created_at >= $4 AND h.created_at < $5
			AND (cardinality($6::int[])=0 OR COALESCE((h.processing_snapshot->>'status_id')::int, legacy.id)=ANY($6))
		ORDER BY h.created_at DESC, h.id DESC LIMIT 1
	) event ON true`

const reportAssigneeJoin = `
	LEFT JOIN LATERAL (
		SELECT DISTINCT ON (handler.id) handler.id,
			COALESCE(event.snapshot->'assignee_names'->>((handler.ordinality-1)::int), '') AS name
		FROM unnest(ARRAY(SELECT jsonb_array_elements_text(COALESCE(event.snapshot->'assignee_ids','[]'::jsonb))::int))
			WITH ORDINALITY AS handler(id, ordinality)
		ORDER BY handler.id, handler.ordinality
	) assigned ON true`

func collectReport(ctx context.Context, db reportQueryer, scope ReportScope, input reportInput, dataCondition string, dataArgs []any) ([]ReportRow, []*reportGroupedItem, error) {
	types := scope.WorkItemTypes()
	if scope.Type == ReportWorkHours {
		types = input.filters.Dimensions
	}
	peopleJoin := reportAssigneeJoin
	fields := reportSourceFields
	if scope.Type == ReportDefects {
		// 数量报表的人员条件作用于事件中的责任人，避免处理人展开重复计数。
		fields = strings.Replace(fields, "jsonb_typeof(event.snapshot->'assignee_ids')", "jsonb_typeof(event.snapshot->'responsible_ids')", 1)
		peopleJoin = strings.NewReplacer("assignee_ids", "responsible_ids", "assignee_names", "responsible_names").Replace(peopleJoin)
	}
	args := []any{scope.AdminId, scope.WorkspaceId, pq.Array(types), input.start, input.end,
		pq.Array(reportOptionIds(input.filters.Statuses)), pq.Array(reportOptionIds(input.filters.Roles)), pq.Array(reportOptionIds(input.filters.Members))}
	args = append(args, dataArgs...)
	query := `SELECT ` + fields + ` FROM ` + define.TableRequirements + ` r` + reportSourceJoins + peopleJoin + `
		LEFT JOIN ` + define.TableWorkspaceMembers + ` wm ON wm.user_id=assigned.id AND wm.admin_id=r.admin_id AND wm.workspace_id=r.workspace_id
		WHERE r.admin_id=$1 AND r.workspace_id=$2 AND r.type=ANY($3::text[]) AND r.deleted_at=0
			AND (cardinality($7::int[])=0 OR wm.role_id=ANY($7))
			AND (cardinality($8::int[])=0 OR assigned.id=ANY($8))` + dataCondition +
		` ORDER BY r.id, assigned.id LIMIT ` + strconv.Itoa(maxReportSourceRows+1)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	items := make([]*reportGroupedItem, 0)
	var current *reportGroupedItem
	var contributionIndex map[string]int
	for count := 0; rows.Next(); count++ {
		if count >= maxReportSourceRows {
			return nil, nil, ErrReportTooLarge
		}
		var item ReportItem
		var person, severity int
		var name string
		var responsibleIdsJSON, responsibleNamesJSON, assigneeIdsJSON []byte
		var hours int64
		if err = rows.Scan(&item.Id, &item.Title, &item.Status, &item.StatusId, &item.Template, &item.Iteration, &item.Category, &item.Priority, &item.Severity, &item.Estimated, &item.Assignees, &item.CreatedAt, &item.PriorityId, &severity, &item.StatusEventId, &item.StatusChangedAt, &person, &name, &hours, &responsibleIdsJSON, &responsibleNamesJSON, &item.Type, &item.CategoryId, &assigneeIdsJSON); err != nil {
			return nil, nil, err
		}
		if current == nil || current.item.Id != item.Id {
			if len(items) >= maxReportItems {
				return nil, nil, ErrReportTooLarge
			}
			if err = json.Unmarshal(assigneeIdsJSON, &item.AssigneeIds); err != nil {
				return nil, nil, fmt.Errorf("decode report assignee ids: %w", err)
			}
			current = &reportGroupedItem{item: item}
			if scope.Type == ReportDefects {
				if err = json.Unmarshal(responsibleNamesJSON, &current.item.ResponsibleNames); err != nil {
					return nil, nil, fmt.Errorf("decode report responsible names: %w", err)
				}
			}
			items = append(items, current)
			contributionIndex = map[string]int{}
		}
		credit := ReportRow{Key: item.Type + ":" + strconv.Itoa(person), Dimension: item.Type, Name: name, Count: 1}
		if scope.Type == ReportDefects {
			credit = defectReportContribution(input.filters.Dimensions, item, person, name, severity)
		}
		index, seen := contributionIndex[credit.Key]
		if !seen {
			index = len(current.contributions)
			contributionIndex[credit.Key] = index
			current.contributions = append(current.contributions, credit)
		}
		if scope.Type == ReportWorkHours {
			addReportHours(&current.contributions[index], person, name, hours)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, nil, err
	}
	groups := map[string]*ReportRow{}
	for _, item := range items {
		for _, contribution := range item.contributions {
			group := groups[contribution.Key]
			if group == nil {
				group = &ReportRow{Key: contribution.Key, Dimension: contribution.Dimension, Name: contribution.Name, Values: contribution.Values}
				groups[contribution.Key] = group
			}
			group.Count++
			if contribution.Hundredths != nil {
				if group.Hundredths == nil {
					group.Hundredths = new(int64)
				}
				*group.Hundredths += *contribution.Hundredths
			}
		}
	}
	summary := make([]ReportRow, 0, len(groups))
	for _, group := range groups {
		summary = append(summary, *group)
	}
	sort.Slice(summary, func(i, j int) bool {
		if summary[i].Dimension != summary[j].Dimension {
			return slices.Index(input.filters.Dimensions, summary[i].Dimension) < slices.Index(input.filters.Dimensions, summary[j].Dimension)
		}
		if summary[i].Count != summary[j].Count {
			return summary[i].Count > summary[j].Count
		}
		return summary[i].Key < summary[j].Key
	})
	return summary, items, nil
}

func addReportHours(contribution *ReportRow, person int, name string, hours int64) {
	if person < 0 {
		return
	}
	if contribution.Hundredths == nil {
		contribution.Hundredths = new(int64)
	}
	if person > 0 {
		*contribution.Hundredths += hours
		contribution.Assignees = append(contribution.Assignees, name)
	}
}

func defectReportContribution(dimensions []string, item ReportItem, person int, name string, severity int) ReportRow {
	credit := ReportRow{Dimension: ReportDefects, Count: 1}
	parts := []string{ReportDefects}
	for _, dimension := range dimensions {
		option := ReportOption{}
		switch dimension {
		case "responsible":
			option.Id, option.Name = person, name
		case "severity":
			option.Id, option.Name = severity, item.Severity
		case "priority":
			option.Id, option.Name = item.PriorityId, item.Priority
		case "category":
			option.Id, option.Name = item.CategoryId, item.Category
		}
		parts = append(parts, strconv.Itoa(option.Id))
		credit.Values = append(credit.Values, option)
	}
	credit.Key = strings.Join(parts, ":")
	return credit
}

func reportWorkItemScope(scope ReportScope, nextArg int) (string, []any, error) {
	condition, args, err := buildDataViewScopeCondition(scope.AdminId, strconv.Itoa(scope.WorkspaceId), scope.UserId, nextArg)
	if condition != "" {
		condition = ` AND ` + condition
	}
	return condition, args, err
}

// PreviewWorkspaceReport 查询默认报表汇总，不写入生成记录。
func PreviewWorkspaceReport(ctx context.Context, scope ReportScope, timezone string) (ReportRecord, error) {
	record, _, err := collectDefaultWorkspaceReport(ctx, scope, timezone)
	return record, err
}

func collectDefaultWorkspaceReport(ctx context.Context, scope ReportScope, timezone string) (ReportRecord, []*reportGroupedItem, error) {
	model := GenerateReportModel{ReportScope: scope, Preset: "this-month", Dimensions: scope.Dimensions(), Timezone: timezone}
	input, err := normalizeReportInput(model, time.Now())
	if err != nil {
		return ReportRecord{}, nil, err
	}
	condition, args, err := reportWorkItemScope(scope, 9)
	if err != nil {
		return ReportRecord{}, nil, err
	}
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return ReportRecord{}, nil, err
	}
	rows, items, err := collectReport(ctx, db, scope, input, condition, args)
	if err != nil {
		return ReportRecord{}, nil, fmt.Errorf("preview workspace report: %w", err)
	}
	return ReportRecord{SnapshotVersion: ReportStatisticsSnapshotVersion, Start: input.start, End: input.end,
		Timezone: timezone, Filters: input.filters, Rows: rows}, items, nil
}

// GenerateWorkspaceReport 生成并保存同一读取视图内计算出的汇总和工作项快照。
func GenerateWorkspaceReport(ctx context.Context, model GenerateReportModel) (int64, error) {
	requestId, err := uuid.Parse(model.RequestId)
	if err != nil || requestId == uuid.Nil {
		return 0, ErrReportInvalid
	}
	now := time.Now()
	input, err := normalizeReportInput(model, now)
	if err != nil {
		return 0, err
	}
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return 0, err
	}
	findExisting := func() (int64, error) {
		var id int64
		var hash []byte
		query := `SELECT id, request_hash FROM ` + define.TableWorkspaceReports + ` WHERE admin_id=$1 AND workspace_id=$2 AND report_type=$3 AND created_by=$4 AND request_id=$5`
		err := db.QueryRowContext(ctx, query, model.AdminId, model.WorkspaceId, model.Type, model.UserId, requestId.String()).Scan(&id, &hash)
		if err != nil {
			return 0, err
		}
		if !slices.Equal(hash, input.hash[:]) {
			return 0, ErrReportConflict
		}
		_, err = GetWorkspaceReport(ctx, model.ReportScope, id)
		return id, err
	}
	if id, existingErr := findExisting(); !errors.Is(existingErr, sql.ErrNoRows) {
		return id, existingErr
	}
	condition, args, err := reportWorkItemScope(model.ReportScope, 9)
	if err != nil {
		return 0, err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err = resolveReportFilters(ctx, tx, model.ReportScope, &input.filters); err != nil {
		return 0, err
	}
	summary, items, err := collectReport(ctx, tx, model.ReportScope, input, condition, args)
	if err != nil {
		return 0, fmt.Errorf("collect workspace report: %w", err)
	}
	filtersJSON, err := json.Marshal(input.filters)
	if err != nil {
		return 0, err
	}
	summaryJSON, err := json.Marshal(struct {
		Rows []ReportRow `json:"rows"`
	}{summary})
	if err != nil {
		return 0, err
	}
	var id int64
	query := `INSERT INTO ` + define.TableWorkspaceReports + `
		(admin_id,workspace_id,report_type,created_by,created_by_name,created_at,period_start_at,period_end_at,timezone,filters,summary,request_id,request_hash,snapshot_version)
		SELECT w.admin_id,w.id,$3,$4,COALESCE(u.name,''),$5,$6,$7,$8,$9,$10,$11,$12,$13
		FROM ` + define.TableWorkspaces + ` w JOIN ` + define.TableUsers + ` u ON u.admin_id=w.admin_id AND u.id=$4
		WHERE w.admin_id=$1 AND w.id=$2 AND w.workspace_type<>'issue'
		RETURNING id`
	err = tx.QueryRowContext(ctx, query, model.AdminId, model.WorkspaceId, model.Type, model.UserId, now.Unix(), input.start, input.end,
		model.Timezone, string(filtersJSON), string(summaryJSON), requestId.String(), input.hash[:], ReportStatisticsSnapshotVersion).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrReportNotFound
	}
	if err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && (pgErr.Code == "23505" || pgErr.Code == "40001") {
			_ = tx.Rollback()
			if originalId, lookupErr := findExisting(); !errors.Is(lookupErr, sql.ErrNoRows) {
				return originalId, lookupErr
			}
		}
		return 0, err
	}
	if len(items) > 0 {
		ids := make([]int, 0, len(items))
		snapshots := make([]string, 0, len(items))
		contributions := make([]string, 0, len(items))
		for _, item := range items {
			raw, encodeErr := json.Marshal(item.item)
			if encodeErr != nil {
				return 0, encodeErr
			}
			credits, encodeErr := json.Marshal(item.contributions)
			if encodeErr != nil {
				return 0, encodeErr
			}
			ids = append(ids, item.item.Id)
			snapshots = append(snapshots, string(raw))
			contributions = append(contributions, string(credits))
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO `+define.TableWorkspaceReportItems+`
			(admin_id,workspace_id,report_id,work_item_id,item_snapshot,contributions)
			SELECT $1,$2,$3,v.id,v.snapshot::jsonb,v.credits::jsonb FROM unnest($4::int[],$5::text[],$6::text[]) v(id,snapshot,credits)`,
			model.AdminId, model.WorkspaceId, id, pq.Array(ids), pq.Array(snapshots), pq.Array(contributions))
		if err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

const reportHistoryFields = `wr.id,wr.created_by_name,wr.created_at,wr.period_start_at,wr.period_end_at,wr.timezone,wr.filters,wr.snapshot_version`

func visibleReportQuery(scope ReportScope) (string, []any, error) {
	dataScope, dataArgs, err := reportWorkItemScope(scope, 5)
	if err != nil {
		return "", nil, err
	}
	args := append([]any{scope.AdminId, scope.WorkspaceId, scope.Type, pq.Array(scope.WorkItemTypes())}, dataArgs...)
	from := ` FROM ` + define.TableWorkspaceReports + ` wr
		 WHERE wr.admin_id=$1 AND wr.workspace_id=$2 AND wr.report_type=$3
         AND (wr.report_type<>'work-hours' OR NOT EXISTS (
           SELECT 1 FROM jsonb_array_elements_text(wr.filters->'dimensions') dimension
           WHERE NOT dimension=ANY($4::text[])
         ))
		 AND NOT EXISTS (
		   SELECT 1 FROM ` + define.TableWorkspaceReportItems + ` i
		   WHERE i.admin_id=wr.admin_id AND i.workspace_id=wr.workspace_id AND i.report_id=wr.id
		   AND NOT EXISTS (
			 SELECT 1 FROM ` + define.TableRequirements + ` r WHERE r.admin_id=wr.admin_id AND r.workspace_id=wr.workspace_id
			 AND r.id=i.work_item_id AND r.type=ANY($4::text[]) AND r.deleted_at=0` + dataScope + `
		   )
		 )
		 AND NOT EXISTS (SELECT 1 FROM jsonb_to_recordset(wr.filters->'statuses') AS selected(id int)
		   WHERE NOT EXISTS (SELECT 1 FROM ` + define.TableWorkItemStatuses + ` s WHERE s.admin_id=$1 AND s.workspace_id=$2 AND s.id=selected.id AND s.type_key=ANY($4::text[])))
		 AND NOT EXISTS (SELECT 1 FROM jsonb_to_recordset(wr.filters->'roles') AS selected(id int)
		   WHERE NOT EXISTS (SELECT 1 FROM ` + define.TableRoles + ` role WHERE role.admin_id=$1 AND role.workspace_id=$2 AND role.id=selected.id AND role.scope='project'))
		 AND NOT EXISTS (SELECT 1 FROM jsonb_to_recordset(wr.filters->'members') AS selected(id int)
		   WHERE NOT EXISTS (SELECT 1 FROM ` + define.TableWorkspaceMembers + ` wm WHERE wm.admin_id=$1 AND wm.workspace_id=$2 AND wm.user_id=selected.id))`
	return from, args, nil
}

type reportScanner interface{ Scan(...any) error }

func scanReport(scanner reportScanner, withSummary bool) (ReportRecord, error) {
	var record ReportRecord
	var filters, summary []byte
	fields := []any{&record.Id, &record.Creator, &record.CreatedAt, &record.Start, &record.End, &record.Timezone, &filters, &record.SnapshotVersion}
	if withSummary {
		fields = append(fields, &summary)
	}
	if err := scanner.Scan(fields...); err != nil {
		return ReportRecord{}, err
	}
	if err := json.Unmarshal(filters, &record.Filters); err != nil {
		return ReportRecord{}, err
	}
	if withSummary {
		payload := struct {
			Rows []ReportRow `json:"rows"`
		}{}
		if err := json.Unmarshal(summary, &payload); err != nil {
			return ReportRecord{}, err
		}
		record.Rows = payload.Rows
	}
	return record, nil
}

// GetWorkspaceReport 按当前数据可见范围读取历史报表。
func GetWorkspaceReport(ctx context.Context, scope ReportScope, id int64) (ReportRecord, error) {
	from, args, err := visibleReportQuery(scope)
	if err != nil {
		return ReportRecord{}, err
	}
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return ReportRecord{}, err
	}
	args = append(args, id)
	row := db.QueryRowContext(ctx, `SELECT `+reportHistoryFields+`,wr.summary`+from+fmt.Sprintf(` AND wr.id=$%d`, len(args)), args...)
	record, err := scanReport(row, true)
	if errors.Is(err, sql.ErrNoRows) {
		return ReportRecord{}, ErrReportNotFound
	}
	return record, err
}

// ListWorkspaceReports 在历史可见性校验后分页。
func ListWorkspaceReports(ctx context.Context, scope ReportScope, page, size int) ([]ReportRecord, int, error) {
	from, args, err := visibleReportQuery(scope)
	if err != nil {
		return nil, 0, err
	}
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return nil, 0, err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()
	var total int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*)`+from, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, size, (page-1)*size)
	rows, err := tx.QueryContext(ctx, `SELECT `+reportHistoryFields+from+fmt.Sprintf(` ORDER BY wr.created_at DESC,wr.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	records := []ReportRecord{}
	for rows.Next() {
		record, scanErr := scanReport(rows, false)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		records = append(records, record)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}
	return records, total, tx.Commit()
}

// ListReportItemsModel 保存报表抽屉的分组和搜索条件。
type ListReportItemsModel struct {
	ReportScope
	Id                                 int64
	Group, Search, Assignee, Iteration string
	Status, Priority                   int
	Page, Size                         int
	AssigneeIds                        []int
	StatusValues, IterationNames       []string
	StatusIds, PriorityIds             []int
	ItemType                           string
}

func normalizeReportItemFilters(model ListReportItemsModel) (ListReportItemsModel, error) {
	if len(model.AssigneeIds) > 50 || slices.ContainsFunc(model.AssigneeIds, func(id int) bool { return id <= 0 }) ||
		len(model.PriorityIds) > 50 || slices.ContainsFunc(model.PriorityIds, func(id int) bool { return id < 0 }) ||
		len(model.StatusValues) > 50 || len(model.IterationNames) > 50 ||
		slices.ContainsFunc(model.IterationNames, func(name string) bool { return len(name) > 200 }) {
		return model, ErrReportInvalid
	}
	typeKey := model.ItemType
	if typeKey == "" {
		typeKey = model.WorkItemType()
		groupType, _, _ := strings.Cut(model.Group, ":")
		if groupType == define.TypeKeyRequirement || groupType == define.TypeKeyDefect {
			typeKey = groupType
		}
	}
	if !slices.Contains(model.WorkItemTypes(), typeKey) {
		return model, ErrReportInvalid
	}
	for _, value := range model.StatusValues {
		switch value {
		case define.ViewValueAllCompleted, define.ViewValueAllClosed, define.ViewValueAllEnd, define.ViewValueAllNotEnd:
		default:
			if id, err := strconv.Atoi(value); err != nil || id <= 0 {
				return model, ErrReportInvalid
			}
		}
	}
	builder := viewFilterBuilder{adminId: model.AdminId, wsId: strconv.Itoa(model.WorkspaceId), typeKey: typeKey}
	var err error
	model.StatusIds, err = builder.expandStatusValues(model.StatusValues)
	return model, err
}

// ListWorkspaceReportPreviewItems 查询默认报表中指定分组的实时明细。
func ListWorkspaceReportPreviewItems(ctx context.Context, model ListReportItemsModel, timezone string) ([]ReportItem, int, error) {
	model, err := normalizeReportItemFilters(model)
	if err != nil {
		return nil, 0, err
	}
	_, groups, err := collectDefaultWorkspaceReport(ctx, model.ReportScope, timezone)
	if err != nil {
		return nil, 0, err
	}
	items := make([]ReportItem, 0)
	keyword := strings.ToLower(strings.TrimSpace(model.Search))
	assignee := strings.ToLower(model.Assignee)
	// 收集过程按工作项 ID 升序，明细与历史一致按 ID 降序分页。
	for _, group := range slices.Backward(groups) {
		item := group.item
		if !(strings.Contains(strings.ToLower(item.Title), keyword) || strconv.Itoa(item.Id) == keyword) ||
			(model.Status != 0 && item.StatusId != model.Status) ||
			(model.Priority != 0 && item.PriorityId != model.Priority) ||
			(len(model.StatusValues) > 0 && !slices.Contains(model.StatusIds, item.StatusId)) ||
			(len(model.PriorityIds) > 0 && !slices.Contains(model.PriorityIds, item.PriorityId)) ||
			(len(model.IterationNames) > 0 && !slices.Contains(model.IterationNames, item.Iteration)) ||
			!strings.Contains(strings.ToLower(item.Assignees), assignee) ||
			(len(model.AssigneeIds) > 0 && !slices.ContainsFunc(item.AssigneeIds, func(id int) bool { return slices.Contains(model.AssigneeIds, id) })) ||
			(model.Iteration != "" && item.Iteration != model.Iteration) {
			continue
		}
		for _, credit := range group.contributions {
			if credit.Key == model.Group {
				item.CountedHundredths, item.CountedAssignees = credit.Hundredths, credit.Assignees
				items = append(items, item)
				break
			}
		}
	}
	total := len(items)
	start := min((model.Page-1)*model.Size, total)
	end := min(start+model.Size, total)
	return items[start:end], total, nil
}

// ListWorkspaceReportItems 只在所选分组内查询可见报表的明细快照。
func ListWorkspaceReportItems(ctx context.Context, model ListReportItemsModel) ([]ReportItem, int, error) {
	model, err := normalizeReportItemFilters(model)
	if err != nil {
		return nil, 0, err
	}
	from, args, err := visibleReportQuery(model.ReportScope)
	if err != nil {
		return nil, 0, err
	}
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return nil, 0, err
	}
	args = append(args, model.Id)
	visible := `SELECT wr.id` + from + fmt.Sprintf(` AND wr.id=$%d`, len(args))
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()
	var id int64
	if err = tx.QueryRowContext(ctx, visible, args...).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = ErrReportNotFound
		}
		return nil, 0, err
	}
	// 旧报表未保存处理人 ID 时，只读取其已记录事件的快照，不根据姓名或当前处理人猜测。
	query := ` FROM ` + define.TableWorkspaceReportItems + ` i
		JOIN LATERAL jsonb_array_elements(i.contributions) AS selected(credit) ON credit->>'key'=$4::text
		WHERE i.admin_id=$1 AND i.workspace_id=$2 AND i.report_id=$3
		AND ($5='' OR strpos(lower(item_snapshot->>'title'),lower($5))>0 OR work_item_id::text=$5)
		AND ($6=0 OR (item_snapshot->>'status_id')::int=$6)
		AND ($7=0 OR (item_snapshot->>'priority_id')::int=$7)
		AND ($8='' OR strpos(lower(item_snapshot->>'assignees'),lower($8))>0)
		AND ($9='' OR item_snapshot->>'iteration'=$9)
		AND (COALESCE(cardinality($10::int[]),0)=0 OR ARRAY(
			SELECT jsonb_array_elements_text(COALESCE(NULLIF(item_snapshot->'assignee_ids','null'::jsonb),
				(SELECT NULLIF(h.processing_snapshot->'assignee_ids','null'::jsonb)
				 FROM ` + define.TableRequirementHistories + ` h
				 WHERE h.admin_id=i.admin_id AND h.workspace_id=i.workspace_id AND h.requirement_id=i.work_item_id
				   AND h.id=(item_snapshot->>'status_event_id')::bigint), '[]'::jsonb))::int
		) && $10::int[])
		AND (NOT $11::bool OR (item_snapshot->>'status_id')::int=ANY($12::int[]))
		AND (COALESCE(cardinality($13::int[]),0)=0 OR COALESCE((item_snapshot->>'priority_id')::int,0)=ANY($13))
		AND (COALESCE(cardinality($14::text[]),0)=0 OR COALESCE(item_snapshot->>'iteration','')=ANY($14))`
	params := []any{model.AdminId, model.WorkspaceId, id, model.Group, strings.TrimSpace(model.Search), model.Status, model.Priority, model.Assignee, model.Iteration,
		pq.Array(model.AssigneeIds), len(model.StatusValues) > 0, pq.Array(model.StatusIds), pq.Array(model.PriorityIds), pq.Array(model.IterationNames)}
	var total int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*)`+query, params...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT item_snapshot, credit`+query+` ORDER BY work_item_id DESC LIMIT $15 OFFSET $16`, append(params, model.Size, (model.Page-1)*model.Size)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []ReportItem{}
	for rows.Next() {
		var raw, credit []byte
		var item ReportItem
		if err = rows.Scan(&raw, &credit); err != nil {
			return nil, 0, err
		}
		if err = json.Unmarshal(raw, &item); err != nil {
			return nil, 0, err
		}
		var contribution ReportRow
		if err = json.Unmarshal(credit, &contribution); err != nil {
			return nil, 0, err
		}
		item.CountedHundredths, item.CountedAssignees = contribution.Hundredths, contribution.Assignees
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, tx.Commit()
}

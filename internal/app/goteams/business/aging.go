// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// Aging overdue-state enums returned to the frontend (frontend maps them to labels).
const (
	AgingStateWarning    = `warning`
	AgingStateOverdue    = `overdue`
	AgingStateNormal     = `normal`
	AgingStateNotCounted = `not_counted`
)

// AgingStatusNormal / AgingStatusWarning / AgingStatusOverdue are the derived-column values
// stored on requirements. Larger means worse, which is the display priority the list column
// uses: any overdue wins, otherwise any warning, otherwise normal.
const (
	AgingStatusNormal  = 0
	AgingStatusWarning = 1
	AgingStatusOverdue = 2
)

// RequirementAgingNode is one row of the requirement aging detail (the total row or a status span).
type RequirementAgingNode struct {
	IsTotal      bool   `json:"is_total"`
	StatusKey    string `json:"status_key"` // i18n key for the status; empty for the total row
	StatusColor  string `json:"status_color"`
	EnterTime    int64  `json:"enter_time"` // unix seconds; 0 for the total row
	Handler      string `json:"handler"`    // display name(s); "-" for the total row
	DurationSec  int64  `json:"duration_sec"`
	DurationText string `json:"duration_text"`
	OverdueState string `json:"overdue_state"` // overdue | normal | not_counted
	Tooltip      string `json:"tooltip"`
}

// GetRequirementAgingModel carries the inputs for building the aging detail.
type GetRequirementAgingModel struct {
	AdminId        int
	WorkspaceId    string
	RequirementId  string
	Form           AgingControlForm
	FinalStatusIds []int
}

type agingStatus struct {
	id    int
	name  string
	color string
}

// History field_name values persisted on requirement_histories.
const (
	historyFieldStatus   = `status`
	historyFieldAssignee = `assignee`
)

// agingInProgressStatusName is the persisted status name (wrapped i18n key) of the in-progress status.
var agingInProgressStatusName = lib_i18n.WrapKey(`workitem_status_in_progress`)

// assigneePoint is one assignee list as recorded by an assignee_change history row.
type assigneePoint struct {
	time int64
	ids  []int
}

// assigneeAt resolves the assignee ids in effect at time t: the newest recorded change at or
// before t, otherwise initialIDs — the list that was in effect before the first recorded change.
func assigneeAt(initialIDs []int, timeline []assigneePoint, t int64) []int {
	ids := initialIDs
	for _, point := range timeline {
		if point.time > t {
			break
		}
		ids = point.ids
	}
	return ids
}

type agingSpan struct {
	statusKey   string
	statusColor string
	enterTime   int64
	exitTime    int64
}

// agingStatusChange is one status transition on the requirement timeline.
type agingStatusChange struct {
	time      int64
	oldStatus string
	newStatus string
}

// agingSpanDuration is the residence length of one span, never negative.
func agingSpanDuration(span agingSpan) int64 {
	if duration := span.exitTime - span.enterTime; duration > 0 {
		return duration
	}
	return 0
}

// buildAgingSpans splits the residence timeline into one span per status: the interval before the
// first transition belongs to the status that transition left, and the last span belongs to the
// status the item sits in now. No transition means the item never left its creation status.
// timingEnd is where timing stops (the last update time for finished items).
func buildAgingSpans(createdAt, timingEnd int64, changes []agingStatusChange, currentStatusKey string, nameToColor map[string]string) []agingSpan {
	if len(changes) == 0 {
		return []agingSpan{{
			statusKey: currentStatusKey, statusColor: nameToColor[currentStatusKey],
			enterTime: createdAt, exitTime: timingEnd,
		}}
	}
	spans := make([]agingSpan, 0, len(changes)+1)
	statusKey := changes[0].oldStatus
	enterTime := createdAt
	for _, change := range changes {
		spans = append(spans, agingSpan{
			statusKey: statusKey, statusColor: nameToColor[statusKey],
			enterTime: enterTime, exitTime: change.time,
		})
		statusKey = change.newStatus
		enterTime = change.time
	}
	return append(spans, agingSpan{
		statusKey: currentStatusKey, statusColor: nameToColor[currentStatusKey],
		enterTime: enterTime, exitTime: timingEnd,
	})
}

// GetRequirementAging reconstructs the per-status residence detail from requirement
// histories and applies the unified judgement logic (config or fallback) so the
// detail page and the list share the same conclusion.
func GetRequirementAging(model GetRequirementAgingModel) ([]RequirementAgingNode, error) {
	row, err := getRequirementById(nil, model.AdminId, model.WorkspaceId, model.RequirementId)
	if err != nil {
		return nil, err
	}
	if len(row) == 0 {
		return []RequirementAgingNode{}, nil
	}
	createdAt := cast.ToInt64(row[`created_at`])
	updatedAt := cast.ToInt64(row[`updated_at`])
	plannedEnd := cast.ToInt64(row[`planned_end_date`])
	currentStatusKey := row[`status_name`]
	typeKey := row[`type`]
	// 处理人 id 的存储形态可能是 JSON（[1,2]）或 PG 数组（{1,2}），统一用兼容解析。
	currentAssigneeIDs := parseStoredAssigneeIDs(row[`assignee_ids`])

	histories, err := listRequirementHistoryRows(model.AdminId, cast.ToInt(model.WorkspaceId), cast.ToInt(model.RequirementId))
	if err != nil {
		return nil, err
	}
	// histories are returned DESC; reverse to chronological order.
	sort.SliceStable(histories, func(i, j int) bool {
		ti := cast.ToInt64(histories[i][`created_at`])
		tj := cast.ToInt64(histories[j][`created_at`])
		if ti == tj {
			return cast.ToInt(histories[i][`id`]) < cast.ToInt(histories[j][`id`])
		}
		return ti < tj
	})

	index, err := loadAgingStatusIndex(model.AdminId, cast.ToInt(model.WorkspaceId), typeKey, model.FinalStatusIds)
	if err != nil {
		return nil, err
	}

	// Build assignee timeline: time -> assignee id list.
	assigneeTimeline := make([]assigneePoint, 0)
	idNameMap := map[int]string{}
	for _, id := range currentAssigneeIDs {
		idNameMap[id] = ``
	}
	// Creation writes no assignee history row, so the list in effect before the first recorded
	// change comes from that change's old value. Spans that closed before it must show that
	// list instead of today's assignees.
	initialAssigneeIDs := currentAssigneeIDs
	for _, h := range histories {
		if h[`field_name`] != historyFieldAssignee {
			continue
		}
		if len(assigneeTimeline) == 0 {
			initialAssigneeIDs = parseStoredAssigneeIDs(h[`old_value`])
			for _, id := range initialAssigneeIDs {
				idNameMap[id] = ``
			}
		}
		ids := parseStoredAssigneeIDs(h[`new_value`])
		for _, id := range ids {
			idNameMap[id] = ``
		}
		assigneeTimeline = append(assigneeTimeline, assigneePoint{time: cast.ToInt64(h[`created_at`]), ids: ids})
	}
	if len(idNameMap) > 0 {
		fillAssigneeNames(model.AdminId, idNameMap)
	}

	handlerAt := func(t int64) string {
		return handlerNames(assigneeAt(initialAssigneeIDs, assigneeTimeline, t), idNameMap)
	}
	// The assignee list as it stands right now: reassigning updates the row of the status the
	// item currently sits in and the total row, while every earlier row keeps the handler it
	// had when it entered that status.
	currentHandler := handlerNames(currentAssigneeIDs, idNameMap)

	// Build the status timeline from status_change records.
	statusChanges := make([]agingStatusChange, 0)
	for _, h := range histories {
		if h[`field_name`] != historyFieldStatus {
			continue
		}
		statusChanges = append(statusChanges, agingStatusChange{
			time: cast.ToInt64(h[`created_at`]), oldStatus: h[`old_value`], newStatus: h[`new_value`],
		})
	}
	now := int64(tool.Time2Int())
	// Timing stops at the last update time once the requirement reaches a terminal status.
	timingEnd := now
	if index.finalNames[currentStatusKey] && updatedAt > 0 {
		timingEnd = updatedAt
	}
	spans := buildAgingSpans(createdAt, timingEnd, statusChanges, currentStatusKey, index.nameToColor)

	nodes := make([]RequirementAgingNode, 0, len(spans)+1)
	verdict := judgeAgingSpans(spans, index.nameToID, model.Form, index.finalNames)
	var totalDurationSec int64
	for i, span := range spans {
		duration := agingSpanDuration(span)
		totalDurationSec += duration
		handler := handlerAt(span.enterTime)
		if i == len(spans)-1 {
			// The last span is the status the item sits in: it follows the live assignee.
			handler = currentHandler
		}
		nodes = append(nodes, RequirementAgingNode{
			IsTotal:      false,
			StatusKey:    span.statusKey,
			StatusColor:  span.statusColor,
			EnterTime:    span.enterTime,
			Handler:      handler,
			DurationSec:  duration,
			DurationText: formatAgingDuration(duration),
			OverdueState: verdict.spans[i].state,
			Tooltip:      verdict.spans[i].tooltip,
		})
	}

	// The 「总耗时」row answers for the total-duration rule alone: a per-status span that is due soon
	// must not make that row claim the total threshold itself is about to breach. The worst span
	// still shows up in the list column through judgeOverallAging.
	totalState, totalTooltip := judgeTotalDuration(model.Form, totalDurationSec, plannedEnd, timingEnd, verdict.fallbackThresholdSec)
	nodes = append([]RequirementAgingNode{{
		IsTotal:      true,
		StatusKey:    ``,
		StatusColor:  ``,
		EnterTime:    createdAt,      // 总耗时行的进入时间即工作项创建时间
		Handler:      currentHandler, // 总耗时行跟随当前处理人
		DurationSec:  totalDurationSec,
		DurationText: formatAgingDuration(totalDurationSec),
		OverdueState: totalState,
		Tooltip:      totalTooltip,
	}}, nodes...)
	return nodes, nil
}

// judgeSpanAging applies the per-status threshold rule.
func judgeSpanAging(span agingSpan, duration int64, statusID int, form AgingControlForm, finalNames map[string]bool) (string, string) {
	if finalNames[span.statusKey] {
		return AgingStateNotCounted, ``
	}
	rule := form.AgingRuleByStatus(statusID)
	if rule == nil || rule.Enabled != 1 {
		return AgingStateNotCounted, ``
	}
	thresholdSec := int64(rule.TimeoutMinutes) * 60
	label := fmt.Sprintf(`时效上限 %d 分钟`, rule.TimeoutMinutes)
	if duration > thresholdSec {
		return AgingStateOverdue, label
	}
	if inAgingWarnWindow(thresholdSec, duration, rule.WarnMinutes) {
		return AgingStateWarning, label
	}
	return AgingStateNormal, label
}

// agingSpanVerdict is one span's judgement: the state shown in its row and the tooltip behind it.
type agingSpanVerdict struct {
	state   string
	tooltip string
}

// agingSpansVerdict is the judgement of a whole residence timeline. anyOverdue / anyWarning are the
// display priority the overall conclusion is built on (any overdue span wins, otherwise any warning
// span), and fallbackThresholdSec is the sum of the built-in default thresholds the total-duration
// judgement falls back to when the config carries no usable threshold.
type agingSpansVerdict struct {
	spans                []agingSpanVerdict
	anyOverdue           bool
	anyWarning           bool
	fallbackThresholdSec int64
}

// judgeAgingSpans judges every span of a timeline with the per-status rule. The detail page renders
// the per-span verdicts while the SLA worker (the list column) takes the aggregate, so both surfaces
// read one conclusion: they cannot disagree about the same work item.
func judgeAgingSpans(spans []agingSpan, nameToID map[string]int, form AgingControlForm, finalNames map[string]bool) agingSpansVerdict {
	verdict := agingSpansVerdict{spans: make([]agingSpanVerdict, 0, len(spans))}
	for _, span := range spans {
		isFinal := finalNames[span.statusKey]
		verdict.fallbackThresholdSec += int64(agingFallbackHours(span.statusKey, isFinal)) * 3600
		state, tooltip := judgeSpanAging(span, agingSpanDuration(span), nameToID[span.statusKey], form, finalNames)
		if state == AgingStateOverdue {
			verdict.anyOverdue = true
		} else if state == AgingStateWarning {
			verdict.anyWarning = true
		}
		verdict.spans = append(verdict.spans, agingSpanVerdict{state: state, tooltip: tooltip})
	}
	return verdict
}

// agingStatusValue maps an overall conclusion onto the derived-column value stored on requirements.
func agingStatusValue(state string) int {
	switch state {
	case AgingStateOverdue:
		return AgingStatusOverdue
	case AgingStateWarning:
		return AgingStatusWarning
	default:
		return AgingStatusNormal
	}
}

// handlerNames renders an assignee id list as display names; "-" when nobody is resolved.
func handlerNames(ids []int, idNameMap map[int]string) string {
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if n, ok := idNameMap[id]; ok && n != `` {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return `-`
	}
	return strings.Join(names, `,`)
}

// inAgingWarnWindow reports whether elapsed sits inside the warning window of a rule: the
// last warn minutes before the threshold breaches. The SLA worker notifies on exactly the
// same window, so anything shown as "due soon" has also been warned about.
func inAgingWarnWindow(thresholdSec int64, elapsedSec int64, warnMinutes *int) bool {
	if warnMinutes == nil || *warnMinutes <= 0 {
		return false
	}
	return elapsedSec >= thresholdSec-int64(*warnMinutes)*60 && elapsedSec < thresholdSec
}

// judgeOverallAging is the conclusion of the whole work item, used by the list column and the view
// filter: it starts from the total-duration verdict and then applies the display priority, so any
// overdue span lifts it to overdue and otherwise any span inside its warning window lifts it to due
// soon. timingEnd is the end of the timing window: the last update time for finished items, now for
// open ones. The detail 「总耗时」row deliberately does NOT go through here: that row answers only for
// the total-duration rule (judgeTotalDuration), otherwise the worst span would masquerade as a
// breach of the total threshold.
func judgeOverallAging(form AgingControlForm, totalDurationSec, plannedEnd, timingEnd int64, anySegmentOverdue bool, anySegmentWarning bool, fallbackThresholdSec int64) (string, string) {
	state, label := judgeTotalDuration(form, totalDurationSec, plannedEnd, timingEnd, fallbackThresholdSec)
	if anySegmentOverdue {
		return AgingStateOverdue, label
	}
	if anySegmentWarning && state == AgingStateNormal {
		return AgingStateWarning, label
	}
	return state, label
}

// judgeTotalDuration resolves the verdict of the total-duration rule alone: explicit total duration,
// then planned end date, then the per-status rules, then the built-in default sum.
func judgeTotalDuration(form AgingControlForm, totalDurationSec, plannedEnd, timingEnd int64, fallbackThresholdSec int64) (string, string) {
	if form.MasterEnabled && form.TotalDuration.Enabled == 1 {
		thresholdSec := int64(form.TotalDuration.TimeoutMinutes) * 60
		label := fmt.Sprintf(`总时长阈值 %d 分钟`, form.TotalDuration.TimeoutMinutes)
		if totalDurationSec > thresholdSec {
			return AgingStateOverdue, label
		}
		if inAgingWarnWindow(thresholdSec, totalDurationSec, form.TotalDuration.WarnMinutes) {
			return AgingStateWarning, label
		}
		return AgingStateNormal, label
	}
	if form.MasterEnabled && plannedEnd > 0 {
		label := `未设置总时长阈值，按预计结束时间判定`
		if timingEnd > endOfDay(plannedEnd) {
			return AgingStateOverdue, label
		}
		return AgingStateNormal, label
	}
	if form.MasterEnabled {
		return AgingStateNormal, `未设置总时长阈值，按各状态超时判定`
	}
	// Fallback: config disabled, compare total against the sum of built-in default thresholds.
	label := `未设置总时长阈值，按状态默认阈值之和判定`
	if fallbackThresholdSec > 0 && totalDurationSec > fallbackThresholdSec {
		return AgingStateOverdue, label
	}
	return AgingStateNormal, label
}

// agingFallbackHours returns the built-in default timeout (hours) for a status.
func agingFallbackHours(statusKey string, isFinal bool) int {
	if isFinal {
		return 0
	}
	if statusKey == agingInProgressStatusName {
		return AgingFallbackInProgressHours
	}
	return AgingFallbackPlannedHours
}

func endOfDay(unixSec int64) int64 {
	t := time.Unix(unixSec, 0)
	loc := t.Location()
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, loc).Unix()
}

// agingStatusIndex is the workspace status scheme resolved into the lookups the judgement needs:
// name -> color for rendering, name -> id for the per-status rules, and the final-status name set.
type agingStatusIndex struct {
	nameToColor map[string]string
	nameToID    map[string]int
	finalNames  map[string]bool
}

// loadAgingStatusIndex loads the status scheme of one work item type and maps the configured final
// status ids onto their names, so a timeline can be judged without further queries.
func loadAgingStatusIndex(adminId, wsId int, typeKey string, finalStatusIds []int) (agingStatusIndex, error) {
	statuses, err := listAgingStatuses(adminId, wsId, typeKey)
	if err != nil {
		return agingStatusIndex{}, err
	}
	index := agingStatusIndex{
		nameToColor: make(map[string]string, len(statuses)),
		nameToID:    make(map[string]int, len(statuses)),
		finalNames:  map[string]bool{},
	}
	for _, st := range statuses {
		index.nameToColor[st.name] = st.color
		index.nameToID[st.name] = st.id
	}
	for _, id := range finalStatusIds {
		for _, st := range statuses {
			if st.id == id {
				index.finalNames[st.name] = true
			}
		}
	}
	return index, nil
}

// listAgingStatuses loads the workspace status scheme for color/name resolution.
func listAgingStatuses(adminId, wsId int, typeKey string) ([]agingStatus, error) {
	m := msql.Model(define.TableWorkItemStatuses, define.Postgres)
	rows, err := m.Field(`id, name, color`).Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, cast.ToString(wsId)},
		{`type_key`, typeKey},
	}).Order(`sort_order asc, id asc`).Select()
	if err != nil {
		logs.Error("list aging statuses sql:%s, err:%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	out := make([]agingStatus, 0, len(rows))
	for _, r := range rows {
		out = append(out, agingStatus{id: cast.ToInt(r[`id`]), name: r[`name`], color: r[`color`]})
	}
	return out, nil
}

// fillAssigneeNames fills the id->name map for the given assignee ids.
func fillAssigneeNames(adminId int, idNameMap map[int]string) {
	ids := make([]int, 0, len(idNameMap))
	for id := range idNameMap {
		ids = append(ids, id)
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT id, name FROM users WHERE admin_id = $1 AND id = ANY($2)`, nil, adminId, pq.Array(ids))
	if err != nil {
		logs.Error("fill aging assignee names sql err:%s", err.Error())
		return
	}
	for _, r := range rows {
		idNameMap[cast.ToInt(r[`id`])] = r[`name`]
	}
}

// formatAgingDuration renders seconds as "X天X小时X分".
func formatAgingDuration(sec int64) string {
	if sec < 0 {
		sec = 0
	}
	days := sec / 86400
	hours := (sec % 86400) / 3600
	mins := (sec % 3600) / 60
	parts := make([]string, 0, 3)
	if days > 0 {
		parts = append(parts, strconv.FormatInt(days, 10)+`天`)
	}
	if hours > 0 || days > 0 {
		parts = append(parts, strconv.FormatInt(hours, 10)+`小时`)
	}
	parts = append(parts, strconv.FormatInt(mins, 10)+`分`)
	return strings.Join(parts, ``)
}

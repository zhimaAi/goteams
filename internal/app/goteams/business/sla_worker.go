// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

const (
	slaWorkerInterval = 60 * time.Second
	slaBatchSize      = 200
)

var (
	slaWorkerCtx    context.Context
	slaWorkerCancel context.CancelFunc
)

// StartSLAWorkers launches the periodic SLA evaluation worker.
func StartSLAWorkers() {
	if slaWorkerCancel != nil {
		return
	}
	slaWorkerCtx, slaWorkerCancel = context.WithCancel(context.Background())
	ctx := slaWorkerCtx
	go runSLAWebhookWorker(ctx)
	go runSLAWorker(ctx)
	go runSLAWorkerLoop(ctx, time.Second, func() {
		evaluateWorkItemSLAActions(ctx)
	})
	logs.Info("sla worker started")
}

// StopSLAWorkers stops the SLA evaluation worker.
func StopSLAWorkers() {
	if slaWorkerCancel != nil {
		slaWorkerCancel()
		slaWorkerCancel = nil
	}
	logs.Info("sla worker stopped")
}

func runSLAWorker(ctx context.Context) {
	var workItemSLACursor int64
	runSLAWorkerLoop(ctx, slaWorkerInterval, func() {
		evaluateAllWorkspaces(ctx)
		maintainWorkItemSLA(ctx, &workItemSLACursor)
	})
}

func runSLAWorkerLoop(ctx context.Context, interval time.Duration, evaluate func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ctx.Err() != nil {
				return
			}
			evaluate()
		}
	}
}

func evaluateAllWorkspaces(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	rows, err := msql.RawValues(define.Postgres,
		`SELECT admin_id, workspace_id, requirement_aging_control, requirement_final_status_ids
		 FROM `+define.TableWorkspaceSettings+` WHERE NOT EXISTS(SELECT 1 FROM `+define.TableSLASettings+` s WHERE s.admin_id=workspaces_settings.admin_id AND s.workspace_id=workspaces_settings.workspace_id AND s.activated_at>0) AND requirement_aging_control IS NOT NULL AND requirement_aging_control <> ''`, nil)
	if err != nil {
		logs.Error("sla worker load settings err:%s", err.Error())
		return
	}
	for _, row := range rows {
		form := parseAgingControlForm(row[`requirement_aging_control`])
		if !form.MasterEnabled {
			continue
		}
		adminId := cast.ToInt(row[`admin_id`])
		wsId := cast.ToInt(row[`workspace_id`])
		finalIds := parsePositiveInts(row[`requirement_final_status_ids`])
		if err := evaluateWorkspace(adminId, wsId, form, finalIds); err != nil {
			logs.Error("sla worker evaluate workspace %d/%d err:%s", adminId, wsId, err.Error())
		}
	}
}

type slaRequirementRow struct {
	ID          int64
	CreatedAt   int64
	UpdatedAt   int64
	StatusID    int
	StatusName  string
	PlannedEnd  int64
	AssigneeIDs []int
	EnteredAt   int64
	OperatorID  int
	// StatusChanges is the chronological status timeline of the work item: the overall
	// conclusion applies the display priority across every span, not just the current one.
	StatusChanges []agingStatusChange
	// IsFinal marks a requirement sitting in a terminal status: its timing stops at the
	// last update time and it no longer receives SLA notifications.
	IsFinal bool
}

type agingUpdate struct {
	ID          int64
	AgingStatus int
}

func evaluateWorkspace(adminId, wsId int, form AgingControlForm, finalIds []int) error {
	reqRows, err := loadSLARequirements(adminId, wsId, finalIds)
	if err != nil {
		return err
	}
	if len(reqRows) == 0 {
		return nil
	}
	reqIDs := make([]int64, 0, len(reqRows))
	reqByID := make(map[int64]*slaRequirementRow, len(reqRows))
	for i := range reqRows {
		reqIDs = append(reqIDs, reqRows[i].ID)
		reqByID[reqRows[i].ID] = &reqRows[i]
	}

	// Resolve assignees and status-change entry times / operators in bulk.
	if err := fillSLARequirementMeta(adminId, wsId, reqIDs, reqByID); err != nil {
		return err
	}
	index, err := loadAgingStatusIndex(adminId, wsId, define.TypeKeyRequirement, finalIds)
	if err != nil {
		return err
	}

	now := int64(tool.Time2Int())
	updates := make([]agingUpdate, 0, len(reqRows))
	// Time driven external notifications reuse this evaluation: they are collected while the
	// transaction is open and dispatched only after it commits, so a push can never roll back the
	// SLA state and a failed push can never lose the verdict.
	dueEvents := make([]slaDueEvent, 0)
	dueSeen := make(map[string]bool)

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// 首次启用等待旧机制在途写入完成，再永久退出此空间。
	settings, err := getSLASettingsTx(tx, adminId, wsId, false)
	if err != nil {
		return err
	}
	if settings.ActivatedAt > 0 {
		return nil
	}

	for _, req := range reqRows {
		// Timing stops at the last update time once the requirement reaches a terminal status.
		end := now
		if req.IsFinal && req.UpdatedAt > 0 {
			end = req.UpdatedAt
		}
		totalDuration := end - req.CreatedAt
		if totalDuration < 0 {
			totalDuration = 0
		}
		residence := end - req.EnteredAt
		if residence < 0 {
			residence = 0
		}

		// The stored conclusion is the one the detail page shows, including its display priority
		// over the whole timeline: any span already overdue wins, otherwise any span inside its
		// warning window. Judging only the current span would show "未超时" for a work item whose
		// earlier span is due soon.
		spans := buildAgingSpans(req.CreatedAt, end, req.StatusChanges, req.StatusName, index.nameToColor)
		verdict := judgeAgingSpans(spans, index.nameToID, form, index.finalNames)
		state, _ := judgeOverallAging(form, totalDuration, req.PlannedEnd, end,
			verdict.anyOverdue, verdict.anyWarning, verdict.fallbackThresholdSec)
		updates = append(updates, agingUpdate{ID: req.ID, AgingStatus: agingStatusValue(state)})

		// A finished requirement keeps its conclusion but stops receiving notifications.
		if req.IsFinal {
			continue
		}

		// Total-duration rule notifications.
		if form.MasterEnabled && form.TotalDuration.Enabled == 1 {
			totalThresholdSec := int64(form.TotalDuration.TimeoutMinutes) * 60
			breach := req.CreatedAt + totalThresholdSec
			if form.TotalDuration.NotifyEnabled == 1 && totalDuration > totalThresholdSec {
				content := slaNotificationContent{Status: req.StatusName}
				if ensureSLAState(tx, adminId, wsId, req.ID, req.AssigneeIDs, req.OperatorID,
					AgingRuleKeyTotal, define.NotificationEventSlaOverdue, true, content) {
					appendSLAExternalEvent(&dueEvents, dueSeen, req.ID, breach, true)
				}
			} else {
				ensureSLAState(tx, adminId, wsId, req.ID, req.AssigneeIDs, req.OperatorID,
					AgingRuleKeyTotal, define.NotificationEventSlaOverdue, false, slaNotificationContent{})
			}
			active := inAgingWarnWindow(totalThresholdSec, totalDuration, form.TotalDuration.WarnMinutes)
			content := slaNotificationContent{Status: req.StatusName, Minutes: warnMinutesLeft(breach, now)}
			if ensureSLAState(tx, adminId, wsId, req.ID, req.AssigneeIDs, req.OperatorID,
				AgingRuleKeyTotal, define.NotificationEventSlaWarning, active, content) {
				appendSLAExternalEvent(&dueEvents, dueSeen, req.ID, breach, false)
			}
		}

		// Current-status rule notifications.
		rule := form.AgingRuleByStatus(req.StatusID)
		if rule != nil && rule.Enabled == 1 {
			thresholdSec := int64(rule.TimeoutMinutes) * 60
			breach := req.EnteredAt + thresholdSec
			if rule.NotifyEnabled == 1 && residence > thresholdSec {
				content := slaNotificationContent{Status: req.StatusName}
				if ensureSLAState(tx, adminId, wsId, req.ID, req.AssigneeIDs, req.OperatorID,
					agingRuleKeyStatus(req.StatusID), define.NotificationEventSlaOverdue, true, content) {
					appendSLAExternalEvent(&dueEvents, dueSeen, req.ID, breach, true)
				}
			} else {
				ensureSLAState(tx, adminId, wsId, req.ID, req.AssigneeIDs, req.OperatorID,
					agingRuleKeyStatus(req.StatusID), define.NotificationEventSlaOverdue, false, slaNotificationContent{})
			}
			active := inAgingWarnWindow(thresholdSec, residence, rule.WarnMinutes)
			content := slaNotificationContent{Status: req.StatusName, Minutes: warnMinutesLeft(breach, now)}
			if ensureSLAState(tx, adminId, wsId, req.ID, req.AssigneeIDs, req.OperatorID,
				agingRuleKeyStatus(req.StatusID), define.NotificationEventSlaWarning, active, content) {
				appendSLAExternalEvent(&dueEvents, dueSeen, req.ID, breach, false)
			}
		}
	}

	if err := flushAgingStatus(tx, adminId, wsId, updates); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	dispatchSLAExternalNotifications(adminId, wsId, dueEvents, now)
	return nil
}

// slaDueEvent is one 时效管控 verdict handed to the external notification dispatch.
type slaDueEvent struct {
	ReqID    int64
	BreachAt int64
	Overdue  bool
}

// appendSLAExternalEvent records a verdict once per work item and kind: the total-duration rule and
// the current-status rule can both report on the same work item within one cycle.
func appendSLAExternalEvent(events *[]slaDueEvent, seen map[string]bool, reqID, breachAt int64, overdue bool) {
	key := cast.ToString(reqID) + `|` + cast.ToString(overdue)
	if seen[key] {
		return
	}
	seen[key] = true
	*events = append(*events, slaDueEvent{ReqID: reqID, BreachAt: breachAt, Overdue: overdue})
}

// dispatchSLAExternalNotifications pushes the 时效管控 verdicts of one workspace. The operation is
// "sla" so the dispatcher recognises them as time driven instead of a user edit.
func dispatchSLAExternalNotifications(adminId, wsId int, events []slaDueEvent, now int64) {
	if len(events) == 0 {
		return
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("lock legacy SLA notification settings failed: %s", err.Error())
		return
	}
	defer func() { _ = tx.Rollback() }()
	settings, err := getSLASettingsTx(tx, adminId, wsId, false)
	if err != nil || settings.ActivatedAt > 0 {
		return
	}
	// 外部发送完成前保留控制共享锁，避免首次启用越过仍在发送的旧通知。
	for _, item := range events {
		snapshot, err := CaptureAutomationSnapshot(adminId, wsId, int(item.ReqID))
		if err != nil {
			logs.Error("capture sla external notification snapshot err:%s", err.Error())
			continue
		}
		if snapshot == nil {
			continue
		}
		DispatchExternalNotifications(AutomationEvent{AdminID: adminId, WorkspaceID: wsId,
			ItemID: int(item.ReqID), ItemType: snapshot.Type, Operation: "sla", After: snapshot,
			TriggeredAt: now, SLA: &AutomationSLASnapshot{Overdue: item.Overdue, BreachAt: item.BreachAt}})
	}
}

// loadSLARequirements loads every requirement of the workspace, terminal ones included:
// a finished item still needs its conclusion (its timing stops at the last update time),
// while notifications for it are suppressed by the caller.
func loadSLARequirements(adminId, wsId int, finalIds []int) ([]slaRequirementRow, error) {
	finalSet := make(map[int]struct{}, len(finalIds))
	for _, id := range finalIds {
		finalSet[id] = struct{}{}
	}
	rows, err := msql.RawValues(define.Postgres,
		`SELECT r.id, r.created_at, r.updated_at, r.status_id, s.name AS status_name, r.planned_end_date, r.assignee_ids
		 FROM requirements r LEFT JOIN `+define.TableWorkItemStatuses+` s ON s.id = r.status_id AND s.workspace_id = r.workspace_id AND s.admin_id = r.admin_id
		 WHERE r.admin_id = $1 AND r.workspace_id = $2 AND r.type = 'requirement' AND r.deleted_at = 0`, nil, adminId, wsId)
	if err != nil {
		logs.Error("sla load requirements sql err:%s", err.Error())
		return nil, err
	}
	out := make([]slaRequirementRow, 0, len(rows))
	for _, r := range rows {
		statusID := cast.ToInt(r[`status_id`])
		_, isFinal := finalSet[statusID]
		out = append(out, slaRequirementRow{
			ID:          cast.ToInt64(r[`id`]),
			CreatedAt:   cast.ToInt64(r[`created_at`]),
			UpdatedAt:   cast.ToInt64(r[`updated_at`]),
			StatusID:    statusID,
			StatusName:  r[`status_name`],
			PlannedEnd:  cast.ToInt64(r[`planned_end_date`]),
			EnteredAt:   cast.ToInt64(r[`created_at`]),
			IsFinal:     isFinal,
			AssigneeIDs: parseStoredAssigneeIDs(r[`assignee_ids`]),
		})
	}
	return out, nil
}

// fillSLARequirementMeta resolves the status timeline of every loaded requirement in bulk: the
// transitions the overall judgement walks through, plus the entry time / operator of the current
// status that the notifications report on. Assignees are already parsed in loadSLARequirements.
func fillSLARequirementMeta(adminId, wsId int, reqIDs []int64, reqByID map[int64]*slaRequirementRow) error {
	if len(reqIDs) == 0 {
		return nil
	}
	historyRows, err := msql.RawValues(define.Postgres,
		`SELECT requirement_id, created_at, user_id, old_value, new_value
		 FROM requirement_histories
		 WHERE admin_id = $1 AND workspace_id = $2 AND requirement_id = ANY($3::INTEGER[])
		   AND field_name = '`+historyFieldStatus+`' AND action = '`+define.HistoryActionStatusChanged+`'
		 ORDER BY requirement_id ASC, created_at ASC, id ASC`, nil, adminId, wsId, pq.Array(reqIDs))
	if err != nil {
		logs.Error("sla load histories sql err:%s", err.Error())
		return err
	}
	for _, r := range historyRows {
		id := cast.ToInt64(r[`requirement_id`])
		req, ok := reqByID[id]
		if !ok {
			continue
		}
		t := cast.ToInt64(r[`created_at`])
		req.StatusChanges = append(req.StatusChanges, agingStatusChange{
			time: t, oldStatus: r[`old_value`], newStatus: r[`new_value`],
		})
		if t > req.EnteredAt {
			req.EnteredAt = t
			req.OperatorID = cast.ToInt(r[`user_id`])
		}
	}
	return nil
}

// flushAgingStatus persists the derived overall aging conclusion. Parameters live in
// plain `col = ANY($n)` predicates (type-inferable), not in a bare VALUES list, and only
// rows whose conclusion actually flips are written so the periodic worker does not
// churn every requirement row each cycle.
func flushAgingStatus(tx *sql.Tx, adminId, wsId int, updates []agingUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	// Group ids by their target value so every conclusion is written exactly once, including
	// a downgrade from overdue back to due soon when the total duration no longer breaches.
	idsByStatus := make(map[int][]int64, 3)
	for _, u := range updates {
		idsByStatus[u.AgingStatus] = append(idsByStatus[u.AgingStatus], u.ID)
	}
	for status, ids := range idsByStatus {
		if _, err := msql.RawExec(define.Postgres,
			`UPDATE requirements SET aging_status = $1
			 WHERE admin_id = $2 AND workspace_id = $3 AND id = ANY($4::INTEGER[]) AND aging_status <> $1`,
			tx, status, adminId, wsId, pq.Array(ids)); err != nil {
			logs.Error("sla flush aging status err:%s", err.Error())
			return err
		}
	}
	return nil
}

// slaNotificationContent is the structured payload stored in notifications.content for SLA events.
// SLA messages are system notices with a fixed wording, so only the status the work item
// currently sits in and, for warnings, the minutes left before the breach are carried.
// Minutes is frozen when the notice is created: the message reads "overdue in N minutes" as of
// that moment, and a stale N is never recomputed on read.
type slaNotificationContent struct {
	Status  string `json:"status"`
	Minutes int    `json:"minutes,omitempty"`
}

// warnMinutesLeft rounds the time left before the breach up to whole minutes, so the notice never
// promises less time than the work item actually has. 0 means the breach already passed.
func warnMinutesLeft(breachAt int64, now int64) int {
	if breachAt <= now {
		return 0
	}
	return int((breachAt - now + 59) / 60)
}

// ensureSLAState deduplicates SLA notifications: pushes at most one effective notification per
// (requirement, rule, notify_type) while the condition stays active, and clears it when inactive.
// It reports whether this cycle actually reported the condition, which is also what the external
// notification dispatch keys off, so both channels stay in step.
func ensureSLAState(tx *sql.Tx, adminId, wsId int, reqID int64, recipients []int, operatorID int, ruleKey, notifyType string, active bool, content slaNotificationContent) bool {
	now := tool.Time2Int()
	stateRows, err := msql.RawValues(define.Postgres,
		`SELECT active FROM sla_notification_states
		 WHERE admin_id = $1 AND workspace_id = $2 AND requirement_id = $3 AND rule_key = $4 AND notify_type = $5`,
		tx, adminId, wsId, reqID, ruleKey, notifyType)
	if err != nil {
		logs.Error("sla query state err:%s", err.Error())
		return false
	}
	existingActive := 0
	if len(stateRows) > 0 {
		existingActive = cast.ToInt(stateRows[0][`active`])
	}

	if active {
		if existingActive == 1 {
			return false
		}
		contentJSON, _ := json.Marshal(content)
		for _, rec := range recipients {
			if rec <= 0 {
				continue
			}
			if err := CreateNotificationTx(tx, notificationEvent{
				AdminID:     adminId,
				RecipientID: rec,
				WorkspaceID: wsId,
				ItemID:      int(reqID),
				EventType:   notifyType,
				ActorID:     operatorID,
				FieldName:   ``,
				Content:     string(contentJSON),
			}); err != nil {
				logs.Error("sla create notification err:%s", err.Error())
			}
		}
		_, err := msql.RawExec(define.Postgres,
			`INSERT INTO sla_notification_states (admin_id, workspace_id, requirement_id, rule_key, notify_type, active, last_notified_at, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, 1, $6, $6, $6)
			 ON CONFLICT (admin_id, workspace_id, requirement_id, rule_key, notify_type)
			 DO UPDATE SET active = 1, last_notified_at = $6, updated_at = $6`,
			tx, adminId, wsId, reqID, ruleKey, notifyType, now)
		if err != nil {
			logs.Error("sla upsert state err:%s", err.Error())
		}
		return true
	}
	if existingActive == 1 {
		_, err := msql.RawExec(define.Postgres,
			`UPDATE sla_notification_states SET active = 0, updated_at = $1
			 WHERE admin_id = $2 AND workspace_id = $3 AND requirement_id = $4 AND rule_key = $5 AND notify_type = $6`,
			tx, now, adminId, wsId, reqID, ruleKey, notifyType)
		if err != nil {
			logs.Error("sla clear state err:%s", err.Error())
		}
	}
	return false
}

func agingRuleKeyStatus(statusID int) string {
	return `status:` + cast.ToString(statusID)
}

func parsePositiveInts(raw string) []int {
	parts := strings.Split(raw, `,`)
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		if id := cast.ToInt(strings.TrimSpace(p)); id > 0 {
			out = append(out, id)
		}
	}
	return out
}

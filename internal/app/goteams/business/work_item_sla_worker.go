package business

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/lib/pq"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"goteams/internal/app/goteams/define"
)

type slaActionCandidate struct {
	Id, CycleId                         int64
	AdminId, WorkspaceId, RequirementId int
}
type slaNoticeContent struct {
	CycleId        int64  `json:"cycle_id"`
	Stage          string `json:"stage"`
	Position       string `json:"position"`
	OffsetSeconds  int64  `json:"offset_seconds"`
	ElapsedSeconds int64  `json:"elapsed_seconds"`
	LimitSeconds   int64  `json:"limit_seconds"`
}

func slaCandidateFromRow(row msql.Params) (slaActionCandidate, error) {
	var item slaActionCandidate
	values := make([]int64, 5)
	for i, key := range []string{"id", "cycle_id", "admin_id", "workspace_id", "requirement_id"} {
		value, err := slaNumber(row, key)
		if err != nil {
			return item, err
		}
		values[i] = value
	}
	item.Id, item.CycleId, item.AdminId, item.WorkspaceId, item.RequirementId = values[0], values[1], int(values[2]), int(values[3]), int(values[4])
	return item, nil
}

func loadSLACycleForUpdate(tx *sql.Tx, item slaActionCandidate) (*slaCycle, error) {
	rows, err := slaRows(tx, "SELECT "+slaCycleColumns+" FROM "+define.TableSLACycles+" WHERE admin_id=$1 AND workspace_id=$2 AND requirement_id=$3 AND id=$4 FOR UPDATE", item.AdminId, item.WorkspaceId, item.RequirementId, item.CycleId)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	cycle, err := slaCycleFromRow(rows[0])
	return &cycle, err
}

func resolveSLARecipients(ctx context.Context, tx *sql.Tx, item slaActionCandidate, action slaAction) ([]int, error) {
	if len(action.Recipients) == 0 {
		return nil, ErrSLAInvalid
	}
	includeAssignees := false
	roleIds := []int{}
	for _, selector := range action.Recipients {
		switch selector.Type {
		case "assignees":
			includeAssignees = true
		case "workspace_role":
			if selector.RoleId <= 0 {
				return nil, ErrSLAInvalid
			}
			roleIds = append(roleIds, selector.RoleId)
		default:
			return nil, ErrSLAInvalid
		}
	}
	query := "SELECT wm.user_id AS id FROM " + define.TableWorkspaceMembers + " wm JOIN " + define.TableUsers + " u ON u.admin_id=wm.admin_id AND u.id=wm.user_id JOIN " + define.TableRequirements + " r ON r.admin_id=wm.admin_id AND r.workspace_id=wm.workspace_id AND r.id=$3 WHERE wm.admin_id=$1 AND wm.workspace_id=$2 AND u.status=1 AND u.deleted_at=0 AND (($4 AND wm.user_id=ANY(r.assignee_ids)) OR wm.role_id=ANY($5))"
	args := []any{item.AdminId, item.WorkspaceId, item.RequirementId, includeAssignees, pq.Array(roleIds)}
	rows, err := slaRows(tx, query, args...)
	if err != nil {
		return nil, err
	}
	recipients := []int{}
	seen := map[int]bool{}
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		id, err := slaNumber(row, "id")
		if err != nil {
			return nil, err
		}
		userId := int(id)
		if seen[userId] || !HasPermission(item.AdminId, userId, strconv.Itoa(item.WorkspaceId), "requirement:view") {
			continue
		}
		visible, err := CheckRequirementVisible(item.AdminId, strconv.Itoa(item.WorkspaceId), strconv.Itoa(item.RequirementId), userId)
		if err != nil {
			return nil, err
		}
		if visible {
			recipients = append(recipients, userId)
			seen[userId] = true
		}
	}
	return recipients, nil
}

func executeWorkItemSLAAction(ctx context.Context, item slaActionCandidate) error {
	prepareCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(prepareCtx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// 现有计时 helper 使用事务查询；服务端超时约束每次查询和锁等待。
	if _, err = tx.ExecContext(prepareCtx, "SET LOCAL statement_timeout='10s'"); err != nil {
		return err
	}
	settings, err := getSLASettingsTx(tx, item.AdminId, item.WorkspaceId, false)
	if err != nil || !settings.Enabled {
		return err
	}
	_, itemErr := lockSLAWorkItemTx(tx, item.AdminId, item.WorkspaceId, item.RequirementId)
	if itemErr != nil && !errors.Is(itemErr, ErrRequirementNotFound) {
		return itemErr
	}
	cycle, err := loadSLACycleForUpdate(tx, item)
	if err != nil || cycle == nil {
		return err
	}
	rows, err := slaRows(tx, "SELECT state,action_key,trigger_elapsed_seconds,next_attempt_at,delivery_platform FROM "+define.TableSLAActions+" WHERE admin_id=$1 AND workspace_id=$2 AND requirement_id=$3 AND cycle_id=$4 AND id=$5 FOR UPDATE", item.AdminId, item.WorkspaceId, item.RequirementId, item.CycleId, item.Id)
	if err != nil || len(rows) == 0 {
		return err
	}
	row := rows[0]
	if row["state"] != "pending" {
		return nil
	}
	now := time.Now().Unix()
	next, err := slaNumber(row, "next_attempt_at")
	if err != nil {
		return err
	}
	if next > now {
		return nil
	}
	if cycle.State != "running" || itemErr != nil {
		if err = slaExec(tx, "UPDATE "+define.TableSLAActions+" SET state='canceled',reason='stage_completed',updated_at=$1 WHERE admin_id=$2 AND workspace_id=$3 AND id=$4 AND state='pending'", now, item.AdminId, item.WorkspaceId, item.Id); err != nil {
			return err
		}
		return tx.Commit()
	}
	threshold, err := slaNumber(row, "trigger_elapsed_seconds")
	if err != nil {
		return err
	}
	changed, bindErr := bindSLACycleCalendars(&cycle.Snapshot)
	if bindErr != nil {
		logs.Error("SLA calendar configuration unavailable, cycle_id:%d, err:%s", cycle.Id, bindErr.Error())
	}
	pauses, err := loadSLAPausesTx(tx, item.AdminId, item.WorkspaceId)
	if err != nil {
		return err
	}
	elapsed, clockErr := slaCycleElapsed(*cycle, pauses, now)
	var due int64
	if clockErr == nil {
		due, clockErr = slaCycleDeadline(*cycle, pauses, threshold)
	}
	if clockErr != nil && !errors.Is(clockErr, ErrSLACalendar) {
		return clockErr
	}
	if changed {
		data, err := json.Marshal(cycle.Snapshot)
		if err != nil {
			return err
		}
		if err = slaExec(tx, "UPDATE "+define.TableSLACycles+" SET snapshot=$1,updated_at=$2 WHERE admin_id=$3 AND workspace_id=$4 AND id=$5", string(data), now, item.AdminId, item.WorkspaceId, cycle.Id); err != nil {
			return err
		}
	}
	if clockErr != nil {
		if err = slaExec(tx, "UPDATE "+define.TableSLAActions+" SET due_at=NULL,reason='calendar_unavailable',next_attempt_at=$1,updated_at=$2 WHERE admin_id=$3 AND workspace_id=$4 AND id=$5", now+300, now, item.AdminId, item.WorkspaceId, item.Id); err != nil {
			return err
		}
		if err = slaExec(tx, "UPDATE "+define.TableSLACycles+" SET calculation_state='calendar_unavailable' WHERE admin_id=$1 AND workspace_id=$2 AND id=$3", item.AdminId, item.WorkspaceId, cycle.Id); err != nil {
			return err
		}
		return tx.Commit()
	}
	if elapsed < threshold {
		if err = slaExec(tx, "UPDATE "+define.TableSLAActions+" SET due_at=$1,clock_revision=$2,reason='',next_attempt_at=0,updated_at=$3 WHERE admin_id=$4 AND workspace_id=$5 AND id=$6", due, settings.ClockRevision, now, item.AdminId, item.WorkspaceId, item.Id); err != nil {
			return err
		}
		return tx.Commit()
	}
	var action *slaAction
	for i := range cycle.Snapshot.Config.Actions {
		if cycle.Snapshot.Config.Actions[i].Key == row["action_key"] {
			action = &cycle.Snapshot.Config.Actions[i]
			break
		}
	}
	if action == nil {
		return ErrSLAInvalid
	}
	if action.Position == "before_deadline" {
		reason := ""
		// 同时覆盖升级前已生成的重匹配任务和执行延迟导致的过期提醒。
		if len(cycle.Snapshot.ClockPeriods) > 0 && due <= cycle.StartedAt {
			reason = "rematch_reminder_missed"
		} else if elapsed >= cycle.LimitSeconds {
			reason = "deadline_reached"
		}
		if reason != "" {
			if err = slaExec(tx, "UPDATE "+define.TableSLAActions+" SET state='skipped',reason=$1,updated_at=$2 WHERE admin_id=$3 AND workspace_id=$4 AND requirement_id=$5 AND cycle_id=$6 AND id=$7 AND state='pending'", reason, now, item.AdminId, item.WorkspaceId, item.RequirementId, item.CycleId, item.Id); err != nil {
				return err
			}
			return tx.Commit()
		}
	}
	if action.Channel == "webhook" {
		return executeSLAWebhook(ctx, tx, item, *cycle, *action, row["delivery_platform"], now)
	}
	recipients, err := resolveSLARecipients(ctx, tx, item, *action)
	if err != nil {
		return err
	}
	stage, ok := snapshotSLAStage(cycle.Snapshot, cycle.StageId)
	if !ok {
		return ErrSLAInvalid
	}
	content, err := json.Marshal(slaNoticeContent{CycleId: cycle.Id, Stage: stage.Name, Position: action.Position, OffsetSeconds: action.OffsetSeconds, ElapsedSeconds: elapsed, LimitSeconds: cycle.LimitSeconds})
	if err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err = ctx.Err(); err != nil {
			return err
		}
		if err = CreateNotificationTx(tx, notificationEvent{AdminID: item.AdminId, WorkspaceID: item.WorkspaceId, ItemID: item.RequirementId, RecipientID: recipient, EventType: define.NotificationEventWorkItemSLA, Content: string(content)}); err != nil {
			return err
		}
	}
	state, reason := "sent", ""
	var sentAt any = time.Now().Unix()
	if len(recipients) == 0 {
		state, reason, sentAt = "skipped", "no_recipients", nil
	}
	err = slaExec(tx, "UPDATE "+define.TableSLAActions+" SET state=$1,recipient_ids=$2,sent_at=$3,reason=$4,updated_at=$5 WHERE admin_id=$6 AND workspace_id=$7 AND id=$8 AND state='pending'", state, pq.Array(recipients), sentAt, reason, now, item.AdminId, item.WorkspaceId, item.Id)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func recordSLAActionFailure(item slaActionCandidate) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, "SET LOCAL statement_timeout='5s'"); err != nil {
		return err
	}
	settings, err := getSLASettingsTx(tx, item.AdminId, item.WorkspaceId, false)
	if err != nil || !settings.Enabled {
		return err
	}
	err = slaExec(tx, slaActionFailureSQL, time.Now().Unix(), item.AdminId, item.WorkspaceId, item.Id, "")
	if err != nil {
		return err
	}
	return tx.Commit()
}

func maintainWorkItemSLACycle(item slaActionCandidate) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	settings, err := getSLASettingsTx(tx, item.AdminId, item.WorkspaceId, false)
	if err != nil || !settings.Enabled {
		return err
	}
	_, itemErr := lockSLAWorkItemTx(tx, item.AdminId, item.WorkspaceId, item.RequirementId)
	if itemErr != nil && !errors.Is(itemErr, ErrRequirementNotFound) {
		return itemErr
	}
	cycle, err := loadSLACycleForUpdate(tx, item)
	if err != nil || cycle == nil {
		return err
	}
	pauses, err := loadSLAPausesTx(tx, item.AdminId, item.WorkspaceId)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	if itemErr != nil && cycle.State == "running" {
		model := SLAChangeModel{AdminId: item.AdminId, WorkspaceId: item.WorkspaceId, RequirementId: item.RequirementId, OccurredAt: now}
		if err = finishSLACycleTx(tx, model, *cycle, "", "transferred", pauses); err != nil {
			return err
		}
		return tx.Commit()
	}
	changed, bindErr := bindSLACycleCalendars(&cycle.Snapshot)
	if bindErr != nil {
		logs.Error("SLA calendar binding failed, cycle_id:%d, err:%s", cycle.Id, bindErr.Error())
	}
	end := now
	if cycle.State != "running" {
		end = cycle.EndedAt
	}
	elapsed, clockErr := slaCycleElapsed(*cycle, pauses, end)
	if clockErr == nil && cycle.State == "running" {
		_, clockErr = slaCycleDeadline(*cycle, pauses, cycle.LimitSeconds)
	}
	if clockErr != nil && !errors.Is(clockErr, ErrSLACalendar) {
		return clockErr
	}
	state := "ready"
	if clockErr != nil {
		state = "calendar_unavailable"
	}
	if !changed && state == cycle.CalculationState {
		return nil
	}
	data, err := json.Marshal(cycle.Snapshot)
	if err != nil {
		return err
	}
	var duration any
	if cycle.State != "running" && clockErr == nil {
		duration = elapsed
	}
	err = slaExec(tx, "UPDATE "+define.TableSLACycles+" SET snapshot=$1,calculation_state=$2,elapsed_seconds=$3,updated_at=$4 WHERE admin_id=$5 AND workspace_id=$6 AND id=$7", string(data), state, duration, now, item.AdminId, item.WorkspaceId, item.CycleId)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func evaluateWorkItemSLAActions(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	query := "SELECT a.id,a.cycle_id,a.admin_id,a.workspace_id,a.requirement_id FROM " + define.TableSLAActions + " a JOIN " + define.TableSLASettings + " s ON s.admin_id=a.admin_id AND s.workspace_id=a.workspace_id WHERE s.enabled AND a.delivery_platform='' AND a.state='pending' AND a.next_attempt_at<=$1 AND a.due_at<=$1 ORDER BY a.due_at,a.id LIMIT 100"
	rows, err := slaRows(nil, query, time.Now().Unix())
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return
	}
	executeSLAActionRows(ctx, rows)
}

func executeSLAActionRows(ctx context.Context, rows []msql.Params) {
	for _, row := range rows {
		if ctx.Err() != nil {
			return
		}
		item, err := slaCandidateFromRow(row)
		if err != nil {
			logs.Error("parse SLA action: %s", err.Error())
			continue
		}
		executeSLAAction(ctx, item)
	}
}

func executeSLAAction(ctx context.Context, item slaActionCandidate) {
	if ctx.Err() != nil {
		return
	}
	if err := executeWorkItemSLAAction(ctx, item); err != nil {
		if ctx.Err() != nil {
			return
		}
		logs.Error("SLA action failed, action_id:%d, err:%s", item.Id, err.Error())
		if retryErr := recordSLAActionFailure(item); retryErr != nil {
			logs.Error("record SLA retry failed, action_id:%d, err:%s", item.Id, retryErr.Error())
		}
	}
}

func maintainWorkItemSLA(ctx context.Context, cursor *int64) {
	if ctx.Err() != nil {
		return
	}
	// 缺日历重试和周期维护保留分钟频率，避免阻塞已到期通知。
	query := "SELECT a.id,a.cycle_id,a.admin_id,a.workspace_id,a.requirement_id FROM " + define.TableSLAActions + " a JOIN " + define.TableSLASettings + " s ON s.admin_id=a.admin_id AND s.workspace_id=a.workspace_id WHERE s.enabled AND a.delivery_platform='' AND a.state='pending' AND a.next_attempt_at<=$1 AND a.due_at IS NULL ORDER BY a.next_attempt_at,a.id LIMIT 100"
	rows, err := slaRows(nil, query, time.Now().Unix())
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return
	}
	executeSLAActionRows(ctx, rows)
	if ctx.Err() != nil {
		return
	}
	rows, err = slaRows(nil, "SELECT c.id AS cycle_id,c.id,c.admin_id,c.workspace_id,c.requirement_id FROM "+define.TableSLACycles+" c JOIN "+define.TableSLASettings+" s ON s.admin_id=c.admin_id AND s.workspace_id=c.workspace_id WHERE s.enabled AND c.id>$1 AND (c.state='running' OR c.calculation_state='calendar_unavailable') ORDER BY c.id LIMIT 200", *cursor)
	if err != nil {
		return
	}
	if len(rows) == 0 {
		*cursor = 0
		return
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			return
		}
		item, err := slaCandidateFromRow(row)
		if err != nil {
			logs.Error("parse SLA cycle: %s", err.Error())
			continue
		}
		*cursor = item.CycleId
		if err = maintainWorkItemSLACycle(item); err != nil {
			logs.Error("maintain SLA cycle failed, cycle_id:%d, err:%s", item.CycleId, err.Error())
		}
	}
}

// RetrySLAAction 只重试当前空间仍运行周期的失败动作，不恢复已完成或关闭空间的发送。
func RetrySLAAction(model RetrySLAActionModel) error {
	if model.ActionId <= 0 {
		return ErrSLAInvalid
	}
	if err := ensureSLAWorkspace(nil, model.AdminId, model.WorkspaceId); err != nil {
		return err
	}
	rows, err := slaRows(nil, "SELECT id,cycle_id,admin_id,workspace_id,requirement_id FROM "+define.TableSLAActions+" WHERE admin_id=$1 AND workspace_id=$2 AND id=$3", model.AdminId, model.WorkspaceId, model.ActionId)
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return ErrSLANotFound
	}
	item, err := slaCandidateFromRow(rows[0])
	if err != nil {
		return err
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	settings, err := getSLASettingsTx(tx, item.AdminId, item.WorkspaceId, false)
	if err != nil {
		return err
	}
	if !settings.Enabled {
		return ErrSLAConflict
	}
	if _, err = lockSLAWorkItemTx(tx, item.AdminId, item.WorkspaceId, item.RequirementId); err != nil {
		return err
	}
	cycle, err := loadSLACycleForUpdate(tx, item)
	if err != nil {
		return err
	}
	if cycle == nil || cycle.State != "running" {
		return ErrSLAConflict
	}
	rows, err = slaRows(tx, "UPDATE "+define.TableSLAActions+" SET state='pending',attempts=0,reason='',next_attempt_at=0,updated_at=$1 WHERE admin_id=$2 AND workspace_id=$3 AND id=$4 AND state='failed' RETURNING id", time.Now().Unix(), model.AdminId, model.WorkspaceId, model.ActionId)
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return ErrSLAConflict
	}
	return tx.Commit()
}

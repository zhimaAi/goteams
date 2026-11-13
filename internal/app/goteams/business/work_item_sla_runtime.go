package business

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"goteams/internal/app/goteams/define"
)

type slaStoredWorkItemState struct {
	WorkspaceId int `json:"workspace_id"`
	StatusId    int `json:"status_id"`
	PriorityId  int `json:"priority_id"`
	CategoryId  int `json:"category_id"`
}
type slaCycle struct {
	Id, StageId                      int64
	StartedAt, EndedAt, LimitSeconds int64
	State, Reason, CalculationState  string
	ElapsedSeconds                   *int64
	Snapshot                         slaSnapshot
}

func slaStateStorage(state SLAWorkItemStateModel) slaStoredWorkItemState {
	return slaStoredWorkItemState{WorkspaceId: state.WorkspaceId, StatusId: state.StatusId, PriorityId: state.PriorityId, CategoryId: state.CategoryId}
}

func slaStateFromRow(row msql.Params) (SLAWorkItemStateModel, error) {
	var state SLAWorkItemStateModel
	values := make([]int, 4)
	for i, key := range []string{"workspace_id", "status_id", "priority", "category_id"} {
		if key == "priority" && row[key] == "" {
			continue
		}
		value, err := slaNumber(row, key)
		if err != nil {
			return state, err
		}
		values[i] = int(value)
	}
	return SLAWorkItemStateModel{WorkspaceId: values[0], StatusId: values[1], PriorityId: values[2], CategoryId: values[3]}, nil
}

// prepareSLAMutationTx 必须在工单锁之前调用，未启用空间不产生任何运行写入。
func prepareSLAMutationTx(tx *sql.Tx, adminId, workspaceId int) (bool, error) {
	settings, err := getSLASettingsTx(tx, adminId, workspaceId, false)
	return settings.Enabled, err
}

func lockSLAWorkspacesTx(tx *sql.Tx, adminId int, workspaceIds []int) error {
	ids := append([]int{}, workspaceIds...)
	sort.Ints(ids)
	last := 0
	for _, id := range ids {
		if id <= 0 || id == last {
			continue
		}
		last = id
		if _, err := prepareSLAMutationTx(tx, adminId, id); err != nil {
			return err
		}
	}
	return nil
}

func clearSLAItemsTx(tx *sql.Tx, adminId, workspaceId int, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	for _, table := range []string{define.TableSLAActions, define.TableSLACycles} {
		if err := slaExec(tx, "DELETE FROM "+table+" WHERE admin_id=$1 AND workspace_id=$2 AND requirement_id=ANY($3)", adminId, workspaceId, pq.Array(ids)); err != nil {
			return err
		}
	}
	return nil
}

func interruptSLAItemTx(tx *sql.Tx, adminId, workspaceId, requirementId int, reason string) error {
	_, err := prepareSLAMutationTx(tx, adminId, workspaceId)
	if err != nil {
		return err
	}
	cycles, err := runningSLACyclesTx(tx, adminId, workspaceId, requirementId)
	if err != nil {
		return err
	}
	pauses, err := loadSLAPausesTx(tx, adminId, workspaceId)
	if err != nil {
		return err
	}
	model := SLAChangeModel{AdminId: adminId, WorkspaceId: workspaceId, RequirementId: requirementId, OccurredAt: time.Now().Unix()}
	for _, cycle := range cycles {
		if err = finishSLACycleTx(tx, model, cycle, "", reason, pauses); err != nil {
			return err
		}
	}
	return nil
}

func lockSLAWorkItemTx(tx *sql.Tx, adminId, workspaceId, requirementId int) (SLAWorkItemStateModel, error) {
	var state SLAWorkItemStateModel
	rows, err := slaRows(tx, "SELECT workspace_id,status_id,COALESCE(priority,0) AS priority,category_id,created_at FROM "+define.TableRequirements+" WHERE admin_id=$1 AND workspace_id=$2 AND id=$3 AND type='requirement' AND deleted_at=0 FOR UPDATE", adminId, workspaceId, requirementId)
	if err != nil {
		return state, err
	}
	if len(rows) == 0 {
		return state, ErrRequirementNotFound
	}
	keys := []string{"workspace_id", "status_id", "priority", "category_id"}
	values := make([]int, 4)
	for i, key := range keys {
		number, err := slaNumber(rows[0], key)
		if err != nil {
			return state, err
		}
		values[i] = int(number)
	}
	createdAt, err := slaNumber(rows[0], "created_at")
	if err != nil {
		return state, err
	}
	return SLAWorkItemStateModel{WorkspaceId: values[0], StatusId: values[1], PriorityId: values[2], CategoryId: values[3], CreatedAt: createdAt}, nil
}

func resolveSLASelector(selector slaSelector, initial int, finals []int) slaSelector {
	if selector.Mode == "workspace_initial" {
		return slaSelector{Mode: "selected", StatusIds: []int{initial}}
	}
	if selector.Mode == "workspace_final" {
		return slaSelector{Mode: "selected", StatusIds: append([]int{}, finals...)}
	}
	return selector
}

func slaSelectorMatches(selector *slaSelector, statusId int) bool {
	if selector == nil {
		return false
	}
	return selector.Mode == "any" || slaHasId(selector.StatusIds, statusId)
}

func slaTransitionMatches(transition slaStageConfig, before, after int) bool {
	if before == after {
		return false
	}
	from := transition.From.Mode == "any" || slaHasId(transition.From.StatusIds, before)
	to := transition.To.Mode == "any" || slaHasId(transition.To.StatusIds, after)
	return from && to
}

func buildSLASnapshotTx(tx *sql.Tx, settings SLASettingsModel, policy SLADefinitionModel, state SLAWorkItemStateModel) (slaSnapshot, error) {
	snapshot := slaSnapshot{SchemaVersion: 2, PolicyId: policy.Id, Revision: policy.Revision, Name: policy.Name, PriorityId: state.PriorityId, CategoryId: state.CategoryId}
	if err := json.Unmarshal(policy.Config, &snapshot.Config); err != nil {
		return snapshot, err
	}
	snapshot.Config.Calendar.Timezone = settings.Timezone
	if _, err := bindSLAHolidayYears(&snapshot.Config.Calendar); err != nil {
		logs.Error("bind SLA holiday configuration failed, err:%s", err.Error())
	}
	rows, err := slaRows(tx, "SELECT requirement_initial_status_id,requirement_final_status_ids FROM "+define.TableWorkspaceSettings+" WHERE admin_id=$1 AND workspace_id=$2", settings.AdminId, settings.WorkspaceId)
	if err != nil {
		return snapshot, err
	}
	if len(rows) != 1 {
		return snapshot, ErrSLAInvalid
	}
	initial, err := slaNumber(rows[0], "requirement_initial_status_id")
	if err != nil {
		return snapshot, err
	}
	finals := []int{}
	if text := rows[0]["requirement_final_status_ids"]; text != "" {
		for _, raw := range strings.Split(text, ",") {
			value, parseErr := strconv.Atoi(strings.TrimSpace(raw))
			if parseErr != nil || value <= 0 {
				return snapshot, ErrSLAInvalid
			}
			finals = append(finals, value)
		}
	}
	stages, err := listSLADefinitionsTx(tx, settings.AdminId, settings.WorkspaceId, true)
	if err != nil {
		return snapshot, err
	}
	for _, item := range stages {
		var config slaStageConfig
		if err = json.Unmarshal(item.Config, &config); err != nil {
			return snapshot, err
		}
		if err = validateSLAStage(config); err != nil {
			return snapshot, err
		}
		from, to := resolveSLASelector(config.From, int(initial), finals), resolveSLASelector(config.To, int(initial), finals)
		resolved := slaSnapshotStageConfig{From: &from, To: &to}
		snapshot.Stages = append(snapshot.Stages, slaSnapshotStage{Id: item.Id, Revision: item.Revision, Name: item.Name, Config: resolved})
	}
	return snapshot, nil
}

func loadSLAPausesTx(tx *sql.Tx, adminId, workspaceId int) ([]slaPause, error) {
	rows, err := slaRows(tx, "SELECT pauses::text AS pauses FROM "+define.TableSLASettings+" WHERE admin_id=$1 AND workspace_id=$2", adminId, workspaceId)
	if err != nil {
		return nil, err
	}
	pauses := []slaPause{}
	if len(rows) > 0 {
		err = json.Unmarshal([]byte(rows[0]["pauses"]), &pauses)
	}
	return pauses, err
}

func slaCycleFromRow(row msql.Params) (slaCycle, error) {
	var cycle slaCycle
	var err error
	if cycle.Id, err = slaNumber(row, "id"); err != nil {
		return cycle, err
	}
	if cycle.StageId, err = slaNumber(row, "stage_id"); err != nil {
		return cycle, err
	}
	if cycle.StartedAt, err = slaNumber(row, "started_at"); err != nil {
		return cycle, err
	}
	if cycle.EndedAt, err = slaNumber(row, "ended_at"); err != nil {
		return cycle, err
	}
	if cycle.LimitSeconds, err = slaNumber(row, "limit_seconds"); err != nil {
		return cycle, err
	}
	if row["elapsed_seconds"] != "" {
		value, err := slaNumber(row, "elapsed_seconds")
		if err != nil {
			return cycle, err
		}
		cycle.ElapsedSeconds = &value
	}
	cycle.State, cycle.Reason, cycle.CalculationState = row["state"], row["end_reason"], row["calculation_state"]
	if err = json.Unmarshal([]byte(row["snapshot"]), &cycle.Snapshot); err != nil {
		return cycle, err
	}
	return cycle, nil
}

const slaCycleColumns = "id,stage_id,started_at,COALESCE(ended_at,0) AS ended_at,limit_seconds,elapsed_seconds,state,end_reason,calculation_state,snapshot::text AS snapshot"

func runningSLACyclesTx(tx *sql.Tx, adminId, workspaceId, requirementId int) ([]slaCycle, error) {
	rows, err := slaRows(tx, "SELECT "+slaCycleColumns+" FROM "+define.TableSLACycles+" WHERE admin_id=$1 AND workspace_id=$2 AND requirement_id=$3 AND state='running' ORDER BY id FOR UPDATE", adminId, workspaceId, requirementId)
	if err != nil {
		return nil, err
	}
	cycles := make([]slaCycle, 0, len(rows))
	for _, row := range rows {
		cycle, err := slaCycleFromRow(row)
		if err != nil {
			return nil, err
		}
		cycles = append(cycles, cycle)
	}
	return cycles, nil
}

func snapshotSLAStage(snapshot slaSnapshot, id int64) (slaSnapshotStage, bool) {
	for _, stage := range snapshot.Stages {
		if int64(stage.Id) == id {
			return stage, true
		}
	}
	return slaSnapshotStage{}, false
}

func slaCycleElapsed(cycle slaCycle, pauses []slaPause, end int64) (int64, error) {
	var elapsed int64
	for _, period := range cycle.Snapshot.ClockPeriods {
		value, err := slaElapsed(period.Calendar, pauses, period.Start, period.End)
		if err != nil {
			return 0, err
		}
		elapsed += value
	}
	value, err := slaElapsed(cycle.Snapshot.Config.Calendar, pauses, cycle.StartedAt, end)
	return elapsed + value, err
}

func slaCycleDeadline(cycle slaCycle, pauses []slaPause, seconds int64) (int64, error) {
	for _, period := range cycle.Snapshot.ClockPeriods {
		elapsed, err := slaElapsed(period.Calendar, pauses, period.Start, period.End)
		if err != nil {
			return 0, err
		}
		if seconds <= elapsed {
			return slaDeadline(period.Calendar, pauses, period.Start, seconds)
		}
		seconds -= elapsed
	}
	return slaDeadline(cycle.Snapshot.Config.Calendar, pauses, cycle.StartedAt, seconds)
}

func bindSLACycleCalendars(snapshot *slaSnapshot) (bool, error) {
	changed, err := bindSLAHolidayYears(&snapshot.Config.Calendar)
	for i := range snapshot.ClockPeriods {
		bound, bindErr := bindSLAHolidayYears(&snapshot.ClockPeriods[i].Calendar)
		changed = changed || bound
		if err == nil {
			err = bindErr
		}
	}
	return changed, err
}

func finishSLACycleTx(tx *sql.Tx, model SLAChangeModel, cycle slaCycle, eventKey string, reason string, pauses []slaPause) error {
	end := max(model.OccurredAt, cycle.StartedAt)
	if _, err := bindSLACycleCalendars(&cycle.Snapshot); err != nil {
		logs.Error("bind SLA cycle calendar failed, cycle_id:%d, err:%s", cycle.Id, err.Error())
	}
	elapsed, err := slaCycleElapsed(cycle, pauses, end)
	calculation := "ready"
	var duration any = elapsed
	if err != nil {
		if !errors.Is(err, ErrSLACalendar) {
			return err
		}
		calculation = "calendar_unavailable"
		duration = nil
	}
	data, err := json.Marshal(cycle.Snapshot)
	if err != nil {
		return err
	}
	state := "interrupted"
	if reason == "completed" {
		state = "completed"
	}
	err = slaExec(tx, "UPDATE "+define.TableSLACycles+" SET state=$1,end_reason=$2,ended_at=$3,end_mutation_key=$4,elapsed_seconds=$5,calculation_state=$6,snapshot=$7,updated_at=$3 WHERE admin_id=$8 AND workspace_id=$9 AND requirement_id=$10 AND id=$11 AND state='running'", state, reason, end, eventKey, duration, calculation, string(data), model.AdminId, model.WorkspaceId, model.RequirementId, cycle.Id)
	if err != nil {
		return err
	}
	return slaExec(tx, "UPDATE "+define.TableSLAActions+" SET state='canceled',reason=$1,updated_at=$2 WHERE admin_id=$3 AND workspace_id=$4 AND cycle_id=$5 AND state='pending'", reason, end, model.AdminId, model.WorkspaceId, cycle.Id)
}

func startSLACycleTx(tx *sql.Tx, model SLAChangeModel, settings SLASettingsModel, snapshot slaSnapshot, target slaTarget, eventKey, matchedKey string, reason string, pauses []slaPause) error {
	stage, ok := snapshotSLAStage(snapshot, int64(target.StageId))
	if !ok {
		return ErrSLAInvalid
	}
	snapshot.Config.Calendar.Timezone = settings.Timezone
	snapshot.Config.Calendar.HolidayYears = nil
	if _, err := bindSLACycleCalendars(&snapshot); err != nil {
		logs.Error("bind new SLA cycle calendar failed, err:%s", err.Error())
	}
	snapshot.Stages = []slaSnapshotStage{stage}
	snapshot.Config.Targets = []slaTarget{target}
	actions := make([]slaAction, 0)
	for _, action := range snapshot.Config.Actions {
		if action.StageId == target.StageId {
			actions = append(actions, action)
		}
	}
	snapshot.Config.Actions = actions
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	rows, err := slaRows(tx, "INSERT INTO "+define.TableSLACycles+" (admin_id,workspace_id,requirement_id,policy_id,policy_revision,stage_id,stage_revision,start_event_key,matched_key,start_mutation_key,start_reason,state,started_at,limit_seconds,snapshot,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'running',$12,$13,$14,$12,$12) ON CONFLICT(admin_id,workspace_id,requirement_id,stage_id,start_mutation_key) DO NOTHING RETURNING id", model.AdminId, model.WorkspaceId, model.RequirementId, snapshot.PolicyId, snapshot.Revision, target.StageId, stage.Revision, eventKey, matchedKey, model.MutationKey, reason, model.OccurredAt, target.LimitSeconds, string(data))
	if err != nil || len(rows) == 0 {
		return err
	}
	id, err := slaNumber(rows[0], "id")
	if err != nil {
		return err
	}
	cycle := slaCycle{StartedAt: model.OccurredAt, Snapshot: snapshot}
	for _, action := range actions {
		threshold := target.LimitSeconds - action.OffsetSeconds
		if action.Position == "after_deadline" {
			threshold = target.LimitSeconds + max(action.OffsetSeconds, 1)
		}
		due, clockErr := slaCycleDeadline(cycle, pauses, threshold)
		var dueValue any = due
		calculation := "ready"
		reason := ""
		if clockErr != nil {
			if !errors.Is(clockErr, ErrSLACalendar) {
				return clockErr
			}
			dueValue = nil
			calculation = "calendar_unavailable"
			reason = "calendar_unavailable"
		}
		if calculation != "ready" {
			if err = slaExec(tx, "UPDATE "+define.TableSLACycles+" SET calculation_state=$1 WHERE admin_id=$2 AND workspace_id=$3 AND id=$4", calculation, model.AdminId, model.WorkspaceId, id); err != nil {
				return err
			}
		}
		state := "pending"
		// 优先级重匹配保留历史耗时，但不补发历史片段内已错过的提前提醒。
		if clockErr == nil && action.Position == "before_deadline" && len(snapshot.ClockPeriods) > 0 && due <= model.OccurredAt {
			state, reason = "skipped", "rematch_reminder_missed"
		}
		for _, platform := range slaActionPlatforms(action) {
			err = slaExec(tx, "INSERT INTO "+define.TableSLAActions+" (admin_id,workspace_id,requirement_id,cycle_id,action_key,trigger_elapsed_seconds,due_at,clock_revision,reason,next_attempt_at,created_at,updated_at,state,delivery_platform) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11,$12,$13)", model.AdminId, model.WorkspaceId, model.RequirementId, id, action.Key, threshold, dueValue, settings.ClockRevision, reason, model.OccurredAt, model.OccurredAt, state, platform)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// slaLatestMatch 只保存最近一次匹配上下文，完整历史由周期快照承担。
type slaLatestMatch struct {
	MutationKey string                 `json:"mutation_key"`
	Kind        string                 `json:"kind"`
	Source      string                 `json:"source"`
	OccurredAt  int64                  `json:"occurred_at"`
	State       slaStoredWorkItemState `json:"state"`
	Snapshot    *slaSnapshot           `json:"snapshot"`
}

func latestSLAMatchTx(tx *sql.Tx, model SLAChangeModel) (string, *slaSnapshot, *slaStoredWorkItemState, error) {
	rows, err := slaRows(tx, "SELECT sla_match::text AS match FROM "+define.TableRequirements+" WHERE admin_id=$1 AND workspace_id=$2 AND id=$3 AND type='requirement'", model.AdminId, model.WorkspaceId, model.RequirementId)
	if err != nil || len(rows) == 0 {
		return "", nil, nil, err
	}
	if rows[0]["match"] == "" {
		return "", nil, nil, nil
	}
	var match *slaLatestMatch
	if err = json.Unmarshal([]byte(rows[0]["match"]), &match); err != nil {
		return "", nil, nil, err
	}
	if match == nil || match.State.WorkspaceId != model.WorkspaceId {
		return "", nil, nil, nil
	}
	return match.MutationKey, match.Snapshot, &match.State, nil
}

func applySLAChangeTx(tx *sql.Tx, model SLAChangeModel) error {
	settings, err := getSLASettingsTx(tx, model.AdminId, model.WorkspaceId, false)
	if err != nil || !settings.Enabled {
		return err
	}
	if model.Kind != "created" && model.Kind != "status_changed" && model.Kind != "priority_changed" {
		return ErrSLAInvalid
	}
	if model.Kind == "status_changed" && model.Before.StatusId == model.After.StatusId {
		return nil
	}
	if model.Kind == "priority_changed" && model.Before.PriorityId == model.After.PriorityId {
		return nil
	}
	if model.MutationKey == "" {
		return ErrSLAInvalid
	}
	if model.OccurredAt == 0 {
		model.OccurredAt = time.Now().Unix()
	}
	previousKey, _, previous, err := latestSLAMatchTx(tx, model)
	if err != nil {
		return err
	}
	if previousKey == model.MutationKey {
		return nil
	}
	policies, err := listSLADefinitionsTx(tx, model.AdminId, model.WorkspaceId, false)
	if err != nil {
		return err
	}
	policy, err := matchSLAPolicy(policies, model.After)
	if err != nil {
		return err
	}
	var snapshot *slaSnapshot
	if policy != nil {
		built, err := buildSLASnapshotTx(tx, settings, *policy, model.After)
		if err != nil {
			return err
		}
		snapshot = &built
	}
	match := slaLatestMatch{MutationKey: model.MutationKey, Kind: model.Kind, Source: model.Source, OccurredAt: model.OccurredAt, State: slaStateStorage(model.After), Snapshot: snapshot}
	data, err := json.Marshal(match)
	if err != nil {
		return err
	}
	if err = slaExec(tx, "UPDATE "+define.TableRequirements+" SET sla_match=$1 WHERE admin_id=$2 AND workspace_id=$3 AND id=$4 AND type='requirement'", string(data), model.AdminId, model.WorkspaceId, model.RequirementId); err != nil {
		return err
	}
	eventKey := model.MutationKey
	pauses, err := loadSLAPausesTx(tx, model.AdminId, model.WorkspaceId)
	if err != nil {
		return err
	}
	cycles, err := runningSLACyclesTx(tx, model.AdminId, model.WorkspaceId, model.RequirementId)
	if err != nil {
		return err
	}
	active := map[int64]slaCycle{}
	completedStages := map[int64]bool{}
	carried := map[int64][]slaClockPeriod{}
	conditionsChanged := previous != nil && (previous.PriorityId != model.After.PriorityId || previous.CategoryId != model.After.CategoryId)
	for _, cycle := range cycles {
		stage, ok := snapshotSLAStage(cycle.Snapshot, cycle.StageId)
		if !ok {
			return ErrSLAInvalid
		}
		reason := ""
		completed := model.Kind == "status_changed" && slaSelectorMatches(stage.Config.To, model.After.StatusId)
		if cycle.Snapshot.SchemaVersion == 1 {
			// 旧运行周期沿用原终点条件；配置迁移不改写周期结果和既有承诺。
			if stage.Config.End == nil {
				return ErrSLAInvalid
			}
			completed = model.Kind == "status_changed" && slaTransitionMatches(*stage.Config.End, model.Before.StatusId, model.After.StatusId)
		}
		if completed {
			completedStages[cycle.StageId] = true
			reason = "completed"
		} else if model.Source == "resubmit" {
			reason = "resubmitted"
		} else if conditionsChanged && (snapshot == nil || snapshot.PolicyId != cycle.Snapshot.PolicyId || snapshot.PriorityId != cycle.Snapshot.PriorityId) {
			reason = "rematched"
		}
		if reason == "rematched" && model.Before.PriorityId != model.After.PriorityId {
			carried[cycle.StageId] = append(append([]slaClockPeriod{}, cycle.Snapshot.ClockPeriods...), slaClockPeriod{Start: cycle.StartedAt, End: max(model.OccurredAt, cycle.StartedAt), Calendar: cycle.Snapshot.Config.Calendar})
		}
		if reason != "" {
			if err = finishSLACycleTx(tx, model, cycle, eventKey, reason, pauses); err != nil {
				return err
			}
		} else {
			active[cycle.StageId] = cycle
		}
	}
	if snapshot == nil {
		return nil
	}
	for _, target := range snapshot.Config.Targets {
		if target.PriorityId != model.After.PriorityId {
			continue
		}
		stage, ok := snapshotSLAStage(*snapshot, int64(target.StageId))
		if !ok {
			return ErrSLAInvalid
		}
		if !slaSelectorMatches(stage.Config.From, model.After.StatusId) && len(carried[int64(target.StageId)]) == 0 {
			continue
		}
		// 多来源状态间的流转仍属于同次计时；离开后重新进入来源状态才开启新周期。
		cycle, running := active[int64(target.StageId)]
		if running && model.Source != "resubmit" && slaSelectorMatches(stage.Config.From, model.Before.StatusId) {
			continue
		}
		// 来源与目标重叠时，刚完成的阶段不在同一事件立即重启。
		if completedStages[int64(target.StageId)] && model.Source != "resubmit" {
			continue
		}
		if running {
			if err = finishSLACycleTx(tx, model, cycle, eventKey, "restarted", pauses); err != nil {
				return err
			}
		}
		reason := "transition"
		if model.Kind == "created" {
			reason = "created"
		}
		if model.Source == "resubmit" {
			reason = "resubmitted"
		}
		stageSnapshot := *snapshot
		stageSnapshot.ClockPeriods = carried[int64(target.StageId)]
		if err = startSLACycleTx(tx, model, settings, stageSnapshot, target, model.MutationKey, model.MutationKey, reason, pauses); err != nil {
			return err
		}
	}
	return nil
}

func restartSLACyclesOnResubmitTx(tx *sql.Tx, model SLAChangeModel) error {
	settings, err := getSLASettingsTx(tx, model.AdminId, model.WorkspaceId, false)
	if err != nil || !settings.Enabled {
		return err
	}
	matchedKey, snapshot, _, err := latestSLAMatchTx(tx, model)
	if err != nil {
		return err
	}
	if snapshot == nil {
		return nil
	}
	rows, err := slaRows(tx, "SELECT id FROM "+define.TableSLACycles+" WHERE admin_id=$1 AND workspace_id=$2 AND requirement_id=$3 AND start_mutation_key=$4 LIMIT 1", model.AdminId, model.WorkspaceId, model.RequirementId, model.MutationKey)
	if err != nil || len(rows) > 0 {
		return err
	}
	pauses, err := loadSLAPausesTx(tx, model.AdminId, model.WorkspaceId)
	if err != nil {
		return err
	}
	cycles, err := runningSLACyclesTx(tx, model.AdminId, model.WorkspaceId, model.RequirementId)
	if err != nil {
		return err
	}
	for _, cycle := range cycles {
		if err = finishSLACycleTx(tx, model, cycle, "", "resubmitted", pauses); err != nil {
			return err
		}
	}
	for _, target := range snapshot.Config.Targets {
		stage, ok := snapshotSLAStage(*snapshot, int64(target.StageId))
		if !ok {
			return ErrSLAInvalid
		}
		if target.PriorityId == snapshot.PriorityId && slaSelectorMatches(stage.Config.From, model.After.StatusId) {
			if err = startSLACycleTx(tx, model, settings, *snapshot, target, "", matchedKey, "resubmitted", pauses); err != nil {
				return err
			}
		}
	}
	return nil
}

func recordSLACreatedTx(tx *sql.Tx, adminId, workspaceId, requirementId int, source string) error {
	enabled, err := prepareSLAMutationTx(tx, adminId, workspaceId)
	if err != nil || !enabled {
		return err
	}
	state, err := lockSLAWorkItemTx(tx, adminId, workspaceId, requirementId)
	if errors.Is(err, ErrRequirementNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return applySLAChangeTx(tx, SLAChangeModel{AdminId: adminId, WorkspaceId: workspaceId, RequirementId: requirementId, MutationKey: uuid.NewString(), Kind: "created", Source: source, After: state, OccurredAt: state.CreatedAt})
}

func recordSLAUpdatedTx(tx *sql.Tx, adminId, workspaceId, requirementId int, before SLAWorkItemStateModel, source string) error {
	state, err := lockSLAWorkItemTx(tx, adminId, workspaceId, requirementId)
	if err != nil {
		return err
	}
	kind := "status_changed"
	if before.StatusId == state.StatusId {
		if before.PriorityId == state.PriorityId {
			return nil
		}
		kind = "priority_changed"
	}
	return applySLAChangeTx(tx, SLAChangeModel{AdminId: adminId, WorkspaceId: workspaceId, RequirementId: requirementId, MutationKey: uuid.NewString(), Kind: kind, Source: source, Before: before, After: state, OccurredAt: time.Now().Unix()})
}

func recordSLAResubmittedTx(tx *sql.Tx, adminId, workspaceId, requirementId int, before SLAWorkItemStateModel) error {
	state, err := lockSLAWorkItemTx(tx, adminId, workspaceId, requirementId)
	if err != nil {
		return err
	}
	model := SLAChangeModel{AdminId: adminId, WorkspaceId: workspaceId, RequirementId: requirementId, MutationKey: uuid.NewString(), Kind: "status_changed", Source: "resubmit", Before: before, After: state, OccurredAt: time.Now().Unix()}
	if before.StatusId == state.StatusId {
		if before.PriorityId == state.PriorityId {
			return restartSLACyclesOnResubmitTx(tx, model)
		}
		model.Kind = "priority_changed"
	}
	return applySLAChangeTx(tx, model)
}

func validateSLACycleSnapshot(cycle slaCycle) error {
	if (cycle.Snapshot.SchemaVersion != 1 && cycle.Snapshot.SchemaVersion != 2) || cycle.StartedAt <= 0 || cycle.LimitSeconds <= 0 {
		return fmt.Errorf("invalid stored SLA cycle %d", cycle.Id)
	}
	return nil
}

// resumeSLACyclesTx 在空间控制锁内收口关闭期间失效的周期，不回放业务历史。
func resumeSLACyclesTx(tx *sql.Tx, adminId, workspaceId int, now int64) error {
	rows, err := slaRows(tx, "SELECT DISTINCT requirement_id FROM "+define.TableSLACycles+" WHERE admin_id=$1 AND workspace_id=$2 AND state='running' ORDER BY requirement_id", adminId, workspaceId)
	if err != nil {
		return err
	}
	pauses, err := loadSLAPausesTx(tx, adminId, workspaceId)
	if err != nil {
		return err
	}
	for _, row := range rows {
		id, err := slaNumber(row, "requirement_id")
		if err != nil {
			return err
		}
		state, itemErr := lockSLAWorkItemTx(tx, adminId, workspaceId, int(id))
		if itemErr != nil && !errors.Is(itemErr, ErrRequirementNotFound) {
			return itemErr
		}
		cycles, err := runningSLACyclesTx(tx, adminId, workspaceId, int(id))
		if err != nil {
			return err
		}
		model := SLAChangeModel{AdminId: adminId, WorkspaceId: workspaceId, RequirementId: int(id), OccurredAt: now}
		_, _, previous, err := latestSLAMatchTx(tx, model)
		if err != nil {
			return err
		}
		for _, cycle := range cycles {
			reason := "transferred"
			if itemErr == nil {
				reason, err = slaResumeCycleReason(cycle, state.StatusId)
				// 任意终点仍要求真实状态变化，不能把原地暂停当作完成。
				if previous != nil && previous.StatusId == state.StatusId {
					reason = ""
				}
				if err != nil {
					return err
				}
			}
			if reason == "" {
				continue
			}
			if err = finishSLACycleTx(tx, model, cycle, "", reason, pauses); err != nil {
				return err
			}
		}
	}
	return nil
}

func slaResumeCycleReason(cycle slaCycle, statusId int) (string, error) {
	stage, ok := snapshotSLAStage(cycle.Snapshot, cycle.StageId)
	if !ok {
		return "", ErrSLAInvalid
	}
	target := stage.Config.To
	if cycle.Snapshot.SchemaVersion == 1 {
		if stage.Config.End == nil {
			return "", ErrSLAInvalid
		}
		target = &stage.Config.End.To
	}
	if slaSelectorMatches(target, statusId) {
		return "completed", nil
	}
	return "", nil
}

func recordSLATransferredTx(tx *sql.Tx, adminId, workspaceId, requirementId int, now int64) error {
	enabled, err := prepareSLAMutationTx(tx, adminId, workspaceId)
	if err != nil || !enabled {
		return err
	}
	state, err := lockSLAWorkItemTx(tx, adminId, workspaceId, requirementId)
	if err != nil {
		return err
	}
	return applySLAChangeTx(tx, SLAChangeModel{AdminId: adminId, WorkspaceId: workspaceId, RequirementId: requirementId, MutationKey: uuid.NewString(), Kind: "created", Source: "transfer", After: state, OccurredAt: now})
}

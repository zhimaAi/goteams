package business

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/lib/pq"
	"goteams/internal/app/goteams/define"
)

func slaNumberPointer(value int64) *int64 { return &value }

func slaCycleProjection(cycle slaCycle, pauses []slaPause, evaluatedAt int64) (SLAStageResultModel, error) {
	result := SLAStageResultModel{CycleId: cycle.Id, StageId: cycle.StageId, State: cycle.State, Reason: cycle.Reason, StartedAt: cycle.StartedAt, EndedAt: cycle.EndedAt, LimitSeconds: cycle.LimitSeconds, CalculationState: cycle.CalculationState}
	if err := validateSLACycleSnapshot(cycle); err != nil {
		return result, err
	}
	stage, ok := snapshotSLAStage(cycle.Snapshot, cycle.StageId)
	if !ok {
		return result, ErrSLAInvalid
	}
	result.StageName = stage.Name
	if len(cycle.Snapshot.ClockPeriods) > 0 {
		result.StartedAt = cycle.Snapshot.ClockPeriods[0].Start
	}
	var elapsed int64
	var err error
	if cycle.State == "running" {
		elapsed, err = slaCycleElapsed(cycle, pauses, evaluatedAt)
	} else if cycle.ElapsedSeconds != nil {
		elapsed = *cycle.ElapsedSeconds
	} else {
		err = ErrSLACalendar
	}
	if err != nil {
		if !errors.Is(err, ErrSLACalendar) {
			return result, err
		}
		result.CalculationState = "calendar_unavailable"
		return result, nil
	}
	result.CalculationState = "ready"
	result.ElapsedSeconds = slaNumberPointer(elapsed)
	result.RemainingSeconds = slaNumberPointer(max(cycle.LimitSeconds-elapsed, 0))
	result.OverdueSeconds = slaNumberPointer(max(elapsed-cycle.LimitSeconds, 0))
	if cycle.State == "running" {
		deadline, err := slaCycleDeadline(cycle, pauses, cycle.LimitSeconds)
		if err != nil {
			if !errors.Is(err, ErrSLACalendar) {
				return result, err
			}
			result.CalculationState = "calendar_unavailable"
		} else {
			result.DeadlineAt = &deadline
		}
	}
	return result, nil
}

// GetSLASummaries 仅投影最近事件和周期，不重新匹配规则或写入日历快照。
func GetSLASummaries(model SLABatchReadModel) (SLABatchReadModel, error) {
	if len(model.Ids) == 0 || len(model.Ids) > 100 || model.UserId <= 0 {
		return model, ErrSLAInvalid
	}
	if err := ensureSLAWorkspace(nil, model.AdminId, model.WorkspaceId); err != nil {
		return model, err
	}
	seen := map[int]bool{}
	for _, id := range model.Ids {
		if id <= 0 || seen[id] {
			return model, ErrSLAInvalid
		}
		seen[id] = true
	}
	condition, args, err := buildDataViewScopeCondition(model.AdminId, strconv.Itoa(model.WorkspaceId), model.UserId, 4)
	if err != nil {
		return model, err
	}
	query := "SELECT r.id FROM " + define.TableRequirements + " r WHERE r.admin_id=$1 AND r.workspace_id=$2 AND r.id=ANY($3) AND r.type='requirement' AND r.deleted_at=0"
	if condition != "" {
		query += " AND (" + condition + ")"
	}
	queryArgs := []any{model.AdminId, model.WorkspaceId, pq.Array(model.Ids)}
	queryArgs = append(queryArgs, args...)
	rows, err := slaRows(nil, query, queryArgs...)
	if err != nil {
		return model, err
	}
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		id, err := slaNumber(row, "id")
		if err != nil {
			return model, err
		}
		ids = append(ids, int(id))
	}
	settings, err := getSLASettingsTx(nil, model.AdminId, model.WorkspaceId, false)
	if err != nil {
		return model, err
	}
	now := time.Now().Unix()
	model.Items = make([]SLAItemResultModel, 0, len(ids))
	if len(ids) == 0 {
		return model, nil
	}
	if !settings.Enabled {
		for _, id := range ids {
			model.Items = append(model.Items, SLAItemResultModel{RequirementId: id, Result: SLAResultModel{Enabled: false, Paused: settings.ActivatedAt > 0, EvaluatedAt: now, Current: []SLAStageResultModel{}, History: []SLAStageResultModel{}}})
		}
		return model, nil
	}
	pauses, err := loadSLAPausesTx(nil, model.AdminId, model.WorkspaceId)
	if err != nil {
		return model, err
	}
	matches := map[int]*slaSnapshot{}
	rows, err = slaRows(nil, "SELECT id AS requirement_id,CASE WHEN (sla_match->'state'->>'workspace_id')::INTEGER=workspace_id THEN (sla_match->'snapshot')::text ELSE NULL END AS snapshot FROM "+define.TableRequirements+" WHERE admin_id=$1 AND workspace_id=$2 AND id=ANY($3)", model.AdminId, model.WorkspaceId, pq.Array(ids))
	if err != nil {
		return model, err
	}
	for _, row := range rows {
		id, err := slaNumber(row, "requirement_id")
		if err != nil {
			return model, err
		}
		var snapshot *slaSnapshot
		if row["snapshot"] != "" {
			if err = json.Unmarshal([]byte(row["snapshot"]), &snapshot); err != nil {
				return model, err
			}
		}
		matches[int(id)] = snapshot
	}
	cycles := map[int][]slaCycle{}
	rows, err = slaRows(nil, "SELECT DISTINCT ON(requirement_id,stage_id) requirement_id,"+slaCycleColumns+" FROM "+define.TableSLACycles+" WHERE admin_id=$1 AND workspace_id=$2 AND requirement_id=ANY($3) ORDER BY requirement_id,stage_id,(state='running') DESC,id DESC", model.AdminId, model.WorkspaceId, pq.Array(ids))
	if err != nil {
		return model, err
	}
	for _, row := range rows {
		id, err := slaNumber(row, "requirement_id")
		if err != nil {
			return model, err
		}
		cycle, err := slaCycleFromRow(row)
		if err != nil {
			return model, err
		}
		cycles[int(id)] = append(cycles[int(id)], cycle)
	}
	for _, id := range ids {
		result := SLAResultModel{Enabled: true, EvaluatedAt: now, Current: []SLAStageResultModel{}, History: []SLAStageResultModel{}}
		byStage := map[int64]slaCycle{}
		for _, cycle := range cycles[id] {
			byStage[cycle.StageId] = cycle
		}
		if snapshot := matches[id]; snapshot != nil {
			matchedPriority := false
			for _, target := range snapshot.Config.Targets {
				if target.PriorityId == snapshot.PriorityId {
					matchedPriority = true
					break
				}
			}
			result.ConfigurationIncomplete = !matchedPriority
			for _, stage := range snapshot.Stages {
				for _, target := range snapshot.Config.Targets {
					if target.PriorityId != snapshot.PriorityId || target.StageId != stage.Id {
						continue
					}
					if cycle, ok := byStage[int64(stage.Id)]; ok && cycle.Snapshot.PolicyId == snapshot.PolicyId && cycle.Snapshot.PriorityId == snapshot.PriorityId {
						projection, err := slaCycleProjection(cycle, pauses, now)
						if err != nil {
							return model, err
						}
						result.Current = append(result.Current, projection)
						delete(byStage, int64(stage.Id))
					} else {
						result.Current = append(result.Current, SLAStageResultModel{StageId: int64(stage.Id), StageName: stage.Name, State: "not_started", CalculationState: "ready", LimitSeconds: target.LimitSeconds, ElapsedSeconds: slaNumberPointer(0), RemainingSeconds: slaNumberPointer(target.LimitSeconds), OverdueSeconds: slaNumberPointer(0)})
					}
				}
			}
		}
		for _, cycle := range cycles[id] {
			if _, remaining := byStage[cycle.StageId]; remaining && cycle.State == "running" {
				projection, err := slaCycleProjection(cycle, pauses, now)
				if err != nil {
					return model, err
				}
				result.Current = append(result.Current, projection)
			}
		}
		model.Items = append(model.Items, SLAItemResultModel{RequirementId: id, Result: result})
	}
	return model, nil
}

// GetRequirementSLA 返回单工单当前阶段和有界历史，沿用工单数据可见性。
func GetRequirementSLA(model SLAReadModel) (SLAResultModel, error) {
	var result SLAResultModel
	if model.Page < 1 || model.Size < 1 || model.Size > 100 {
		return result, ErrSLAInvalid
	}
	batch, err := GetSLASummaries(SLABatchReadModel{AdminId: model.AdminId, WorkspaceId: model.WorkspaceId, UserId: model.UserId, Ids: []int{model.RequirementId}})
	if err != nil {
		return result, err
	}
	if len(batch.Items) != 1 {
		return result, ErrSLANotFound
	}
	result = batch.Items[0].Result
	if !result.Enabled {
		return result, nil
	}
	counts, err := slaRows(nil, "SELECT COUNT(*) AS total FROM "+define.TableSLACycles+" WHERE admin_id=$1 AND workspace_id=$2 AND requirement_id=$3", model.AdminId, model.WorkspaceId, model.RequirementId)
	if err != nil {
		return result, err
	}
	if len(counts) != 1 {
		return result, ErrSLANotFound
	}
	total, err := slaNumber(counts[0], "total")
	if err != nil {
		return result, err
	}
	result.Total = int(total)
	rows, err := slaRows(nil, "SELECT "+slaCycleColumns+" FROM "+define.TableSLACycles+" WHERE admin_id=$1 AND workspace_id=$2 AND requirement_id=$3 ORDER BY id DESC LIMIT $4 OFFSET $5", model.AdminId, model.WorkspaceId, model.RequirementId, model.Size, (model.Page-1)*model.Size)
	if err != nil {
		return result, err
	}
	pauses, err := loadSLAPausesTx(nil, model.AdminId, model.WorkspaceId)
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		cycle, err := slaCycleFromRow(row)
		if err != nil {
			return result, err
		}
		projection, err := slaCycleProjection(cycle, pauses, result.EvaluatedAt)
		if err != nil {
			return result, err
		}
		result.History = append(result.History, projection)
	}
	return result, nil
}

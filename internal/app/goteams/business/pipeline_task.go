// Copyright © 2026 Wuhan Sesame Small Customer Service Network Co., Ltd.

package business

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_storage"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// ErrPipelineTaskNotFound indicates that the pipeline task does not exist.
var ErrPipelineTaskNotFound = errors.New(`pipeline task not found`)

// ListPipelineTasks returns the pipeline task list, stats, and filter members.
func ListPipelineTasks(adminId, userId int, keyword, period string, userIDs []int, startAt, endAt int64, page, size int) ([]map[string]interface{}, int, define.PipelineTaskStats, []define.PipelineTaskFilterMember, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	offset := (page - 1) * size

	// 页面权限不代表工作项可见；列表、统计和筛选项共用当前用户的可见范围。
	visibility := pipelineWorkItemLogFilter{
		adminID: adminId, viewerUserID: userId, tenantOwner: IsTenantOwner(adminId, userId),
	}

	// 同一需求的多次执行（含切换执行方式后的再次执行）合成一行。
	// conversation_rounds 是这些执行的次数，Token 是它们的合计；其余字段取最近一次执行。
	now := time.Now()
	listFilter := visibility
	listFilter.keyword, listFilter.userIDs = keyword, userIDs
	listFilter.startAt, listFilter.endAt = startAt, endAt
	listFilter.periodStart, listFilter.periodEnd, listFilter.periodEnabled = resolvePipelineTaskPeriodRange(period, now)
	total, err := countAggregatedPipelineWorkItems(listFilter)
	if err != nil {
		return nil, 0, define.PipelineTaskStats{}, nil, err
	}
	rows, err := listAggregatedPipelineWorkItems(listFilter, size, offset)
	if err != nil {
		return nil, 0, define.PipelineTaskStats{}, nil, err
	}

	list := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		list = append(list, map[string]interface{}{
			`id`:                  cast.ToInt64(row[`id`]),
			`user_id`:             cast.ToInt(row[`user_id`]),
			`user_name`:           row[`user_name`],
			`user_avatar`:         row[`user_avatar`],
			`pipeline_id`:         cast.ToInt(row[`pipeline_id`]),
			`pipeline_name`:       row[`pipeline_name_snapshot`],
			`pipeline_color`:      row[`pipeline_color_snapshot`],
			`work_item_type`:      row[`work_item_type`],
			`work_item_id`:        cast.ToInt(row[`work_item_id`]),
			`workspace_id`:        cast.ToInt(row[`workspace_id`]),
			`title`:               row[`title_snapshot`],
			`status_id`:           cast.ToInt(row[`requirement_status_id`]),
			`status_name`:         row[`requirement_status_name`],
			`status_color`:        row[`requirement_status_color`],
			`is_done`:             cast.ToBool(row[`is_done`]),
			`task_status`:         row[`task_status`],
			`execution_status`:    row[`execution_status`],
			`current_step_key`:    row[`current_step_key`],
			`execution_summary`:   row[`execution_summary`],
			`input_tokens`:        cast.ToInt64(row[`input_tokens`]),
			`output_tokens`:       cast.ToInt64(row[`output_tokens`]),
			`total_tokens`:        cast.ToInt64(row[`total_tokens`]),
			`conversation_rounds`: cast.ToInt(row[`conversation_rounds`]),
			`started_at`:          cast.ToInt64(row[`started_at`]),
			`finished_at`:         cast.ToInt64(row[`finished_at`]),
			`device_uuid`:         row[`device_uuid`],
			`last_uploaded_at`:    cast.ToInt64(row[`last_uploaded_at`]),
			`created_at`:          cast.ToInt64(row[`created_at`]),
		})
	}

	// 页签计数与表格一样按需求去重，搜索和成员筛选不计入页签。
	stats := define.PipelineTaskStats{}
	stats.All, err = countAggregatedPipelineWorkItems(visibility)
	if err != nil {
		return nil, 0, define.PipelineTaskStats{}, nil, err
	}

	statsFilter := visibility
	statsFilter.periodStart, statsFilter.periodEnd, statsFilter.periodEnabled = resolvePipelineTaskPeriodRange(define.PipelineTaskPeriodToday, now)
	stats.Today, err = countAggregatedPipelineWorkItems(statsFilter)
	if err != nil {
		return nil, 0, define.PipelineTaskStats{}, nil, err
	}

	statsFilter.periodStart, statsFilter.periodEnd, statsFilter.periodEnabled = resolvePipelineTaskPeriodRange(define.PipelineTaskPeriodYesterday, now)
	stats.Yesterday, err = countAggregatedPipelineWorkItems(statsFilter)
	if err != nil {
		return nil, 0, define.PipelineTaskStats{}, nil, err
	}

	statsFilter.periodStart, statsFilter.periodEnd, statsFilter.periodEnabled = resolvePipelineTaskPeriodRange(define.PipelineTaskPeriodWeek, now)
	stats.Week, err = countAggregatedPipelineWorkItems(statsFilter)
	if err != nil {
		return nil, 0, define.PipelineTaskStats{}, nil, err
	}

	members, err := listPipelineTaskFilterMembers(visibility)
	if err != nil {
		return nil, 0, define.PipelineTaskStats{}, nil, err
	}

	return list, total, stats, members, nil
}

func listPipelineTaskFilterMembers(filter pipelineWorkItemLogFilter) ([]define.PipelineTaskFilterMember, error) {
	where, args := filter.where()
	rows, err := msql.RawValues(define.Postgres, `SELECT DISTINCT t.user_id, u.name, u.username, u.avatar
		FROM `+define.TablePipelineTask+` t
		LEFT JOIN `+define.TableUsers+` u ON t.user_id=u.id AND t.admin_id=u.admin_id
		WHERE `+where+` ORDER BY u.name ASC, u.username ASC, t.user_id ASC`, nil, args...)
	if err != nil {
		return nil, err
	}
	members := make([]define.PipelineTaskFilterMember, 0, len(rows))
	for _, row := range rows {
		members = append(members, define.PipelineTaskFilterMember{
			ID:       cast.ToInt(row[`user_id`]),
			Name:     cast.ToString(row[`name`]),
			Username: cast.ToString(row[`username`]),
			Avatar:   cast.ToString(row[`avatar`]),
		})
	}
	return members, nil
}

func resolvePipelineTaskPeriodRange(period string, now time.Time) (int64, int64, bool) {
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrowStart := todayStart.AddDate(0, 0, 1)
	switch period {
	case define.PipelineTaskPeriodToday:
		return todayStart.Unix(), tomorrowStart.Unix(), true
	case define.PipelineTaskPeriodYesterday:
		return todayStart.AddDate(0, 0, -1).Unix(), todayStart.Unix(), true
	case define.PipelineTaskPeriodWeek:
		return todayStart.AddDate(0, 0, -6).Unix(), tomorrowStart.Unix(), true
	default:
		return 0, 0, false
	}
}

// GetPipelineTaskDetail returns pipeline task details.
// pipeline:view opens the log page; task access is additionally restricted to work items
// visible to the requester in their workspace.
func GetPipelineTaskDetail(adminId, userId int, taskId int64) (map[string]interface{}, error) {
	filter := pipelineWorkItemLogFilter{
		adminID: adminId, viewerUserID: userId, tenantOwner: IsTenantOwner(adminId, userId),
	}
	where, args := filter.where()
	args = append(args, taskId)
	rows, err := msql.RawValues(define.Postgres, `SELECT t.* FROM `+define.TablePipelineTask+` t
		WHERE `+where+` AND t.id=$`+cast.ToString(len(args)), nil, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrPipelineTaskNotFound
	}
	row := rows[0]

	task := map[string]interface{}{
		`id`:                  cast.ToInt64(row[`id`]),
		`user_id`:             cast.ToInt(row[`user_id`]),
		`pipeline_id`:         cast.ToInt(row[`pipeline_id`]),
		`pipeline_name`:       row[`pipeline_name_snapshot`],
		`pipeline_color`:      row[`pipeline_color_snapshot`],
		`work_item_type`:      row[`work_item_type`],
		`work_item_id`:        cast.ToInt(row[`work_item_id`]),
		`workspace_id`:        cast.ToInt(row[`workspace_id`]),
		`title`:               row[`title_snapshot`],
		`task_status`:         row[`task_status`],
		`execution_status`:    row[`execution_status`],
		`current_step_key`:    row[`current_step_key`],
		`execution_summary`:   row[`execution_summary`],
		`input_tokens`:        cast.ToInt64(row[`input_tokens`]),
		`output_tokens`:       cast.ToInt64(row[`output_tokens`]),
		`total_tokens`:        cast.ToInt64(row[`total_tokens`]),
		`conversation_rounds`: cast.ToInt(row[`conversation_rounds`]),
		`started_at`:          cast.ToInt64(row[`started_at`]),
		`finished_at`:         cast.ToInt64(row[`finished_at`]),
		`device_uuid`:         row[`device_uuid`],
		`last_uploaded_at`:    cast.ToInt64(row[`last_uploaded_at`]),
	}

	userM := msql.Model(define.TableUsers, define.Postgres)
	userRow, userErr := userM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(cast.ToInt(row[`user_id`]))},
	}).Find()
	if userErr == nil && len(userRow) > 0 {
		task[`user_name`] = userRow[`name`]
		task[`user_avatar`] = userRow[`avatar`]
	}

	// Query steps
	stepM := msql.Model(define.TablePipelineTaskStep, define.Postgres)
	steps, err := stepM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`pipeline_task_id`, cast.ToString(taskId)},
	}).Order(`sort_order asc, id asc`).Select()
	if err != nil {
		return nil, err
	}
	stepList := make([]map[string]interface{}, 0, len(steps))
	stepNames := make(map[string]string, len(steps))
	stepOrders := make(map[string]int, len(steps))
	for _, s := range steps {
		stepKey := cast.ToString(s[`step_key`])
		stepName := cast.ToString(s[`step_name`])
		stepNames[stepKey] = stepName
		stepOrders[stepKey] = cast.ToInt(s[`sort_order`])
		stepList = append(stepList, map[string]interface{}{
			`step_key`:            stepKey,
			`step_name`:           stepName,
			`sort_order`:          cast.ToInt(s[`sort_order`]),
			`status`:              s[`status`],
			`prompt_summary`:      s[`prompt_summary`],
			`output_summary`:      s[`output_summary`],
			`error_summary`:       s[`error_summary`],
			`input_tokens`:        cast.ToInt64(s[`input_tokens`]),
			`output_tokens`:       cast.ToInt64(s[`output_tokens`]),
			`total_tokens`:        cast.ToInt64(s[`total_tokens`]),
			`conversation_rounds`: cast.ToInt(s[`conversation_rounds`]),
			`started_at`:          cast.ToInt64(s[`started_at`]),
			`finished_at`:         cast.ToInt64(s[`finished_at`]),
			`duration_ms`:         cast.ToInt64(s[`duration_ms`]),
		})
	}
	task[`steps`] = stepList

	// Query step-level sessions: one row per step holding the step's full conversation JSON
	// and aggregated run result, pushed via the client's session sync channel.
	sessionM := msql.Model(define.TablePipelineTaskSession, define.Postgres)
	sessions, err := sessionM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`pipeline_task_id`, cast.ToString(taskId)},
	}).Order(`started_at asc, id asc`).Select()
	if err != nil {
		return nil, err
	}
	conversationList := make([]map[string]interface{}, 0, len(sessions))
	for _, row := range sessions {
		stepKey := cast.ToString(row[`step_key`])
		content := define.PipelineTaskConversationContentSnapshot{
			Rounds: make([]define.PipelineTaskConversationRoundSnapshot, 0),
		}
		// 需求 1896 优化点3：会话内容存于 OSS 对象（信封加密），DB 仅存元数据。
		// 优先读 OSS 解密；历史行（conversation_oss_key 为空）回退读 conversation_json。
		rawContent := ``
		if ossKey := cast.ToString(row[`conversation_oss_key`]); ossKey != `` {
			cipherBytes, readErr := lib_storage.M.Read(ossKey)
			if readErr != nil {
				logs.Error("read session oss object error: task_id=%d, step=%s, err=%s", taskId, stepKey, readErr.Error())
				return nil, readErr
			}
			plain, decErr := decryptSessionContent(adminId, cipherBytes, cast.ToString(row[`conversation_dek_wrapped`]), cast.ToString(row[`conversation_enc`]))
			if decErr != nil {
				logs.Error("decrypt session content error: task_id=%d, step=%s, err=%s", taskId, stepKey, decErr.Error())
				return nil, decErr
			}
			rawContent = string(plain)
		} else {
			rawContent = cast.ToString(row[`conversation_json`])
		}
		if rawContent != `` {
			if err := json.Unmarshal([]byte(rawContent), &content); err != nil {
				return nil, err
			}
		}
		stepName := cast.ToString(row[`step_name`])
		if stepName == `` {
			stepName = stepNames[stepKey]
		}
		// The outer timestamps are Unix seconds for the web frontend; the session row
		// stores millisecond values from the client push.
		conversationList = append(conversationList, map[string]interface{}{
			`conversation_uuid`: stepKey,
			`step_key`:          stepKey,
			`step_name`:         stepName,
			`step_order`:        stepOrders[stepKey],
			`status`:            row[`status`],
			`cli_type`:          row[`cli_type`],
			`model_name`:        row[`model_name`],
			`conversation`:      content,
			`error_summary`:     row[`error_message`],
			`input_tokens`:      cast.ToInt64(row[`input_tokens`]),
			`output_tokens`:     cast.ToInt64(row[`output_tokens`]),
			`total_tokens`:      cast.ToInt64(row[`total_tokens`]),
			`tool_call_count`:   cast.ToInt(row[`tool_call_count`]),
			`files_changed`:     cast.ToInt(row[`files_changed`]),
			`lines_added`:       cast.ToInt(row[`lines_added`]),
			`lines_deleted`:     cast.ToInt(row[`lines_deleted`]),
			`started_at`:        cast.ToInt64(row[`started_at`]) / 1000,
			`finished_at`:       cast.ToInt64(row[`finished_at`]) / 1000,
			`duration_ms`:       cast.ToInt64(row[`duration_ms`]),
		})
	}
	task[`conversations`] = conversationList

	// 产出文档索引：前端据此把沟通记录和正文里的本地路径变成可点击入口，清单外的路径不处理。
	documents, err := PipelineTaskDocumentIndexes(adminId, taskId)
	if err != nil {
		return nil, err
	}
	task[`documents`] = documents

	return task, nil
}

// snapshotUnix converts a client RFC3339 time to Unix seconds.
func snapshotUnix(value *string) int64 {
	if value == nil {
		return 0
	}
	raw := strings.TrimSpace(*value)
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.Unix()
	}
	return 0
}

// RequestTaskSync asks online clients to sync immediately.
// Deprecated: WebSocket sync requests are removed; clients push results incrementally via REST.
func RequestTaskSync(adminId, userId, targetUserId int, deviceUUID, workItemType string, workItemId int) (define.RequestSyncResp, error) {
	return define.RequestSyncResp{}, errors.New(`sync requests via WebSocket are removed`)
}

// validateTaskSnapshot validates stable identifiers and relations in the client's incremental sync payload.
// Steps are incremental: the usage counters are cumulative task-level values provided
// by the client, while the arrays only carry the changed rows, so cross-field equality is not enforced.
func validateTaskSnapshot(data define.PipelineTaskSnapshot) error {
	if strings.TrimSpace(data.LocalTaskUUID) == `` || len(data.LocalTaskUUID) > 64 {
		return errors.New(`local_task_uuid is invalid`)
	}
	if data.LocalRevision < 0 || strings.TrimSpace(data.ProjectionHash) == `` ||
		len(data.ProjectionHash) > 64 {
		return errors.New(`task revision or projection_hash is invalid`)
	}
	if data.WorkItem.ID <= 0 || data.WorkItem.WorkspaceID <= 0 ||
		(data.WorkItem.Type != define.TypeKeyRequirement && data.WorkItem.Type != define.TypeKeyDefect) {
		return errors.New(`work_item is invalid`)
	}
	if data.Pipeline.ID <= 0 {
		return errors.New(`pipeline snapshot is invalid`)
	}
	switch data.ExecutionStatus {
	case define.ExecStatusIdle, define.ExecStatusRunning, define.ExecStatusSuccess,
		define.ExecStatusFailed, define.ExecStatusStopped:
	default:
		return errors.New(`execution_status is invalid`)
	}
	if data.ExecutionIndex < 0 {
		return errors.New(`execution_index is invalid`)
	}
	if data.Usage.InputTokens < 0 || data.Usage.OutputTokens < 0 ||
		data.Usage.TotalTokens < 0 || data.Usage.ConversationRounds < 0 {
		return errors.New(`task usage is invalid`)
	}
	stepKeys := make(map[string]struct{}, len(data.Steps))
	for _, step := range data.Steps {
		if step.StepKey == `` {
			return errors.New(`step_key is required`)
		}
		if _, exists := stepKeys[step.StepKey]; exists {
			return errors.New(`duplicate step_key`)
		}
		stepKeys[step.StepKey] = struct{}{}
	}
	if data.CurrentStepKey != `` {
		if _, exists := stepKeys[data.CurrentStepKey]; !exists {
			return errors.New(`current_step_key is invalid`)
		}
	}
	return nil
}

// SaveTaskSnapshot saves the incremental task sync pushed by the client (called via REST).
// The main table is upserted and steps are upserted by their stable step keys instead of
// being fully replaced; the deleted step list is applied first so a removal always wins.
// Conversation content is no longer part of the task sync channel: the client pushes each
// step's full conversation via the step-level session sync (SaveTaskSession) instead.
func SaveTaskSnapshot(adminId, userId int, data define.PipelineTaskSnapshot) (define.PipelineTaskSyncResp, error) {
	if err := validateTaskSnapshot(data); err != nil {
		return define.PipelineTaskSyncResp{}, err
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return define.PipelineTaskSyncResp{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// Serialize task sync per member to avoid concurrent first inserts of the same local_task_uuid.
	if _, err = msql.RawValues(define.Postgres,
		`SELECT pg_advisory_xact_lock($1, $2)`, tx, adminId, userId); err != nil {
		return define.PipelineTaskSyncResp{}, err
	}

	existing, err := msql.RawValues(define.Postgres, `SELECT id, local_revision, projection_hash, last_uploaded_at
		FROM pipeline_tasks
		WHERE admin_id = $1 AND user_id = $2 AND local_task_uuid = $3 AND execution_index = $4
		FOR UPDATE`, tx, adminId, userId, data.LocalTaskUUID, data.ExecutionIndex)
	if err != nil {
		return define.PipelineTaskSyncResp{}, err
	}

	now := tool.Time2Int()
	startedAt := snapshotUnix(data.StartedAt)
	finishedAt := snapshotUnix(data.FinishedAt)
	clientUpdatedAt := int64(0)
	if data.LastLocalUpdatedAt != `` {
		value := data.LastLocalUpdatedAt
		clientUpdatedAt = snapshotUnix(&value)
	}
	workflowVersion := data.Pipeline.WorkflowVersion
	if workflowVersion <= 0 {
		workflowVersion = 1
	}

	var cloudTaskId int64
	if len(existing) > 0 {
		cloudTaskId = cast.ToInt64(existing[0][`id`])
		existingRevision := cast.ToInt64(existing[0][`local_revision`])
		existingHash := cast.ToString(existing[0][`projection_hash`])
		if data.LocalRevision <= existingRevision {
			if data.LocalRevision == existingRevision && data.ProjectionHash == existingHash {
				return define.PipelineTaskSyncResp{
					CloudTaskID:    cloudTaskId,
					LocalRevision:  existingRevision,
					ProjectionHash: existingHash,
					LastUploadedAt: cast.ToInt64(existing[0][`last_uploaded_at`]),
					Result:         `duplicate`,
				}, nil
			}
			return define.PipelineTaskSyncResp{}, errors.New(`same revision has different projection_hash`)
		}
		if _, err = msql.RawExec(define.Postgres, `UPDATE pipeline_tasks SET
			workspace_id=$1,work_item_id=$2,work_item_type=$3,title_snapshot=$4,
			pipeline_id=$5,pipeline_name_snapshot=$6,pipeline_color_snapshot=$7,cli_type=$8,workflow_version=$9,
			local_revision=$10,projection_hash=$11,device_uuid=$12,task_status=$13,execution_status=$14,
			current_step_key=$15,execution_summary=$16,input_tokens=$17,output_tokens=$18,total_tokens=$19,
			conversation_rounds=$20,started_at=$21,finished_at=$22,client_updated_at=$23,
			last_uploaded_at=$24,updated_at=$24
			WHERE admin_id=$25 AND user_id=$26 AND id=$27`, tx,
			data.WorkItem.WorkspaceID, data.WorkItem.ID, data.WorkItem.Type, data.WorkItem.Title,
			data.Pipeline.ID, data.Pipeline.Name, data.Pipeline.Color, data.Pipeline.CliType, workflowVersion,
			data.LocalRevision, data.ProjectionHash, data.DeviceUUID, data.TaskStatus, data.ExecutionStatus,
			data.CurrentStepKey, data.ExecutionSummary, data.Usage.InputTokens, data.Usage.OutputTokens,
			data.Usage.TotalTokens, data.Usage.ConversationRounds, startedAt, finishedAt, clientUpdatedAt,
			now, adminId, userId, cloudTaskId); err != nil {
			return define.PipelineTaskSyncResp{}, err
		}
	} else {
		rows, insertErr := msql.RawValues(define.Postgres, `INSERT INTO pipeline_tasks(
			admin_id,user_id,workspace_id,work_item_id,work_item_type,title_snapshot,
			pipeline_id,pipeline_name_snapshot,pipeline_color_snapshot,cli_type,workflow_version,
			local_task_uuid,execution_index,local_revision,projection_hash,device_uuid,task_status,execution_status,
			current_step_key,execution_summary,input_tokens,output_tokens,total_tokens,conversation_rounds,
			started_at,finished_at,client_updated_at,last_uploaded_at,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,
				$21,$22,$23,$24,$25,$26,$27,$28,$28,$28)
			RETURNING id`, tx,
			adminId, userId, data.WorkItem.WorkspaceID, data.WorkItem.ID, data.WorkItem.Type,
			data.WorkItem.Title, data.Pipeline.ID, data.Pipeline.Name, data.Pipeline.Color, data.Pipeline.CliType,
			workflowVersion, data.LocalTaskUUID, data.ExecutionIndex, data.LocalRevision, data.ProjectionHash,
			data.DeviceUUID, data.TaskStatus, data.ExecutionStatus, data.CurrentStepKey,
			data.ExecutionSummary, data.Usage.InputTokens, data.Usage.OutputTokens, data.Usage.TotalTokens,
			data.Usage.ConversationRounds, startedAt, finishedAt, clientUpdatedAt, now)
		if insertErr != nil {
			return define.PipelineTaskSyncResp{}, insertErr
		}
		cloudTaskId = cast.ToInt64(rows[0][`id`])
	}

	// Apply deletions first so a removal always wins over a stale re-add in the same payload.
	if len(data.DeletedStepKeys) > 0 {
		if _, err = msql.RawExec(define.Postgres,
			`DELETE FROM pipeline_task_steps WHERE admin_id=$1 AND pipeline_task_id=$2 AND step_key = ANY($3)`,
			tx, adminId, cloudTaskId, pq.Array(data.DeletedStepKeys)); err != nil {
			return define.PipelineTaskSyncResp{}, err
		}
	}

	// Incremental upserts: rows not present in this payload are left untouched on the cloud side.

	// Incremental upserts: rows not present in this payload are left untouched on the cloud side.
	for _, step := range data.Steps {
		if _, err = msql.RawExec(define.Postgres, `INSERT INTO pipeline_task_steps(
			admin_id,pipeline_task_id,step_key,step_name,sort_order,status,prompt_summary,output_summary,
			error_summary,input_tokens,output_tokens,total_tokens,conversation_rounds,started_at,
			finished_at,duration_ms,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$17)
			ON CONFLICT(admin_id, pipeline_task_id, step_key) DO UPDATE SET
			step_name=EXCLUDED.step_name,sort_order=EXCLUDED.sort_order,status=EXCLUDED.status,
			prompt_summary=EXCLUDED.prompt_summary,output_summary=EXCLUDED.output_summary,
			error_summary=EXCLUDED.error_summary,input_tokens=EXCLUDED.input_tokens,
			output_tokens=EXCLUDED.output_tokens,total_tokens=EXCLUDED.total_tokens,
			conversation_rounds=EXCLUDED.conversation_rounds,started_at=EXCLUDED.started_at,
			finished_at=EXCLUDED.finished_at,duration_ms=EXCLUDED.duration_ms,updated_at=EXCLUDED.updated_at`, tx,
			adminId, cloudTaskId, step.StepKey, step.StepName, step.SortOrder, step.Status,
			step.PromptSummary, step.OutputSummary, step.ErrorSummary, step.Usage.InputTokens,
			step.Usage.OutputTokens, step.Usage.TotalTokens, step.Usage.ConversationRounds,
			snapshotUnix(step.StartedAt), snapshotUnix(step.FinishedAt), step.DurationMs, now); err != nil {
			return define.PipelineTaskSyncResp{}, err
		}
	}

	if err = tx.Commit(); err != nil {
		return define.PipelineTaskSyncResp{}, err
	}
	return define.PipelineTaskSyncResp{
		CloudTaskID:    cloudTaskId,
		LocalRevision:  data.LocalRevision,
		ProjectionHash: data.ProjectionHash,
		LastUploadedAt: int64(now),
		Result:         `accepted`,
	}, nil
}

// DeleteTaskByLocalUUID deletes the cloud task projection (steps and conversations) by the client task uuid.
// Triggered by the REST DELETE endpoint. Scoped to admin_id + user_id to ensure tenant and member isolation;
// a missing task is treated as already deleted (idempotent) and does not error.
func DeleteTaskByLocalUUID(adminId, userId int, localTaskUUID string) error {
	if localTaskUUID == `` {
		return errors.New(`local_task_uuid is required`)
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Lock the task by tenant, member, and local UUID; subtables are cleaned up explicitly by business code.
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM pipeline_tasks
		WHERE admin_id=$1 AND user_id=$2 AND local_task_uuid=$3
		FOR UPDATE`, tx, adminId, userId, localTaskUUID)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		// Already gone, treated as delete success (idempotent)
		logs.Info("DeleteTaskByLocalUUID: task not found, admin=%d user=%d uuid=%s", adminId, userId, localTaskUUID)
		return nil
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, cast.ToInt64(row[`id`]))
	}

	// 产出文档随任务一起软删；对象存储对象在响应返回后定向回收，失败由下次启动补做。
	tempPaths, documentObjects, err := softDeleteTaskDocumentsTx(tx, adminId, ids)
	if err != nil {
		return err
	}
	if _, err = msql.RawExec(define.Postgres,
		`DELETE FROM pipeline_task_sessions WHERE admin_id=$1 AND pipeline_task_id = ANY($2)`,
		tx, adminId, pq.Array(ids)); err != nil {
		return err
	}
	if _, err = msql.RawExec(define.Postgres,
		`DELETE FROM pipeline_task_steps WHERE admin_id=$1 AND pipeline_task_id = ANY($2)`,
		tx, adminId, pq.Array(ids)); err != nil {
		return err
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM pipeline_tasks
		WHERE admin_id=$1 AND user_id=$2 AND id = ANY($3)`, tx, adminId, userId, pq.Array(ids)); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	for _, tempPath := range tempPaths {
		RemoveStagedDocumentFile(tempPath)
	}
	scheduleDocumentObjectPurge(documentObjects)

	logs.Info("DeleteTaskByLocalUUID: deleted executions=%d admin=%d user=%d uuid=%s", len(ids), adminId, userId, localTaskUUID)
	return nil
}

// ======================== Work-item agent tab ========================

// ListWorkItemAgentTasks returns the client execution records of one work item,
// grouped by executor. It backs the Agent tab of the requirement/defect detail
// drawer, so access is already gated by the requirement view permission chain.
func ListWorkItemAgentTasks(adminId, viewerUserId, workspaceId int, workItemType string, workItemId int) ([]define.AgentTaskExecutor, error) {
	visible, err := CheckRequirementVisible(adminId, cast.ToString(workspaceId), cast.ToString(workItemId), viewerUserId)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, ErrPipelineTaskNotFound
	}
	rows, err := msql.RawValues(define.Postgres, `
		SELECT t.id, t.user_id, t.pipeline_name_snapshot, t.task_status, t.execution_status,
		       t.current_step_key, t.conversation_rounds, t.total_tokens,
		       t.started_at, t.finished_at, t.last_uploaded_at,
		       u.name AS user_name, u.avatar AS user_avatar
		FROM pipeline_tasks t
		LEFT JOIN users u ON u.id=t.user_id AND u.admin_id=t.admin_id
		WHERE t.admin_id=$1 AND t.workspace_id=$2 AND t.work_item_type=$3 AND t.work_item_id=$4
		ORDER BY t.user_id, COALESCE(NULLIF(t.started_at, 0), t.created_at), t.id`, nil,
		adminId, workspaceId, workItemType, workItemId)
	if err != nil {
		return nil, err
	}

	// The current step display name needs one lookup over all steps of these tasks.
	stepNames := make(map[string]string)
	taskIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		taskIDs = append(taskIDs, cast.ToInt64(row[`id`]))
	}
	if len(taskIDs) > 0 {
		stepRows, stepErr := msql.RawValues(define.Postgres, `
			SELECT pipeline_task_id, step_key, step_name FROM pipeline_task_steps
			WHERE admin_id=$1 AND pipeline_task_id = ANY($2)`, nil, adminId, pq.Array(taskIDs))
		if stepErr != nil {
			return nil, stepErr
		}
		for _, s := range stepRows {
			stepNames[cast.ToString(s[`pipeline_task_id`])+"/"+cast.ToString(s[`step_key`])] = cast.ToString(s[`step_name`])
		}
	}

	executors := make([]define.AgentTaskExecutor, 0)
	executorIndex := make(map[int]int)
	for _, row := range rows {
		userId := cast.ToInt(row[`user_id`])
		index, ok := executorIndex[userId]
		if !ok {
			executors = append(executors, define.AgentTaskExecutor{
				UserId:   userId,
				UserName: cast.ToString(row[`user_name`]),
				Avatar:   cast.ToString(row[`user_avatar`]),
				Tasks:    make([]define.AgentTaskSummary, 0),
			})
			index = len(executors) - 1
			executorIndex[userId] = index
		}
		taskID := cast.ToInt64(row[`id`])
		executors[index].Tasks = append(executors[index].Tasks, define.AgentTaskSummary{
			TaskID:             taskID,
			TaskStatus:         cast.ToString(row[`task_status`]),
			ExecutionStatus:    cast.ToString(row[`execution_status`]),
			PipelineName:       cast.ToString(row[`pipeline_name_snapshot`]),
			CurrentStepName:    stepNames[cast.ToString(taskID)+"/"+cast.ToString(row[`current_step_key`])],
			ConversationRounds: cast.ToInt(row[`conversation_rounds`]),
			TotalTokens:        cast.ToInt64(row[`total_tokens`]),
			StartedAt:          cast.ToInt64(row[`started_at`]),
			FinishedAt:         cast.ToInt64(row[`finished_at`]),
			LastUploadedAt:     cast.ToInt64(row[`last_uploaded_at`]),
		})
	}
	return executors, nil
}

// GetWorkItemAgentTaskDetail returns one execution record detail of a work item.
// The task must belong to the given work item; otherwise the caller gets
// ErrPipelineTaskNotFound even when the task exists under another work item.
func GetWorkItemAgentTaskDetail(adminId, viewerUserId, workspaceId int, workItemType string, workItemId int, taskId int64) (map[string]interface{}, error) {
	m := msql.Model(define.TablePipelineTask, define.Postgres)
	row, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, cast.ToString(workspaceId)},
		{`work_item_type`, workItemType},
		{`work_item_id`, cast.ToString(workItemId)},
		{`id`, cast.ToString(taskId)},
	}).Find()
	if err != nil {
		return nil, err
	}
	if len(row) == 0 {
		return nil, ErrPipelineTaskNotFound
	}
	return GetPipelineTaskDetail(adminId, viewerUserId, taskId)
}

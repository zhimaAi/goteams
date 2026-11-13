// Copyright © 2026 Wuhan Sesame Small Customer Service Network Co., Ltd.

package business

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_storage"

	"github.com/google/uuid"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// ErrPipelineTaskSessionTargetMissing indicates that the task projection of the pushed
// session does not exist on the cloud yet; the client retries the push in a later round.
var ErrPipelineTaskSessionTargetMissing = errors.New(`task projection not found`)

// validateTaskSessionSync validates the stable identifiers, conversation body and payload
// sizes of a step-level session sync push.
func validateTaskSessionSync(data define.PipelineTaskSessionSync) error {
	if strings.TrimSpace(data.LocalTaskUUID) == `` || len(data.LocalTaskUUID) > 64 {
		return errors.New(`local_task_uuid is invalid`)
	}
	if data.ExecutionIndex < 0 {
		return errors.New(`execution_index is invalid`)
	}
	if strings.TrimSpace(data.StepKey) == `` || len(data.StepKey) > 64 {
		return errors.New(`step_key is invalid`)
	}
	if len(data.StepName) > 128 {
		return errors.New(`step_name is invalid`)
	}
	if data.InputTokens < 0 || data.OutputTokens < 0 || data.TotalTokens < 0 ||
		data.ConversationRounds < 0 || data.DurationMs < 0 {
		return errors.New(`session usage is invalid`)
	}
	if data.ToolCallCount < 0 || data.FilesChanged < 0 || data.LinesAdded < 0 || data.LinesDeleted < 0 {
		return errors.New(`session usage is invalid`)
	}
	if len(data.PromptSnapshot) > 128*1024 {
		return errors.New(`prompt_snapshot is too large`)
	}
	if len(data.FinalResult) > 64*1024 {
		return errors.New(`final_result is too large`)
	}
	if len(data.ErrorMessage) > 64*1024 {
		return errors.New(`error_message is too large`)
	}
	if data.Conversation.RoundCount != len(data.Conversation.Rounds) {
		return errors.New(`conversation round_count is invalid`)
	}
	rounds := data.Conversation.Rounds
	if len(rounds) > 0 && len(rounds) != data.ConversationRounds {
		return errors.New(`conversation_rounds mismatch`)
	}
	for index, round := range rounds {
		if round.RoundNo != index+1 || round.SessionUUID == `` {
			return errors.New(`conversation round is invalid`)
		}
		// 用户消息与最终回复都按客户端落库上限放行：final_result 最多 64K 个字符。
		if len(round.UserMessage) > 256*1024 {
			return errors.New(`user message is too large`)
		}
		messageSequences := make(map[int]struct{}, len(round.AssistantMessages))
		for _, message := range round.AssistantMessages {
			if message.Sequence <= 0 || message.Role != `assistant` ||
				strings.TrimSpace(message.Content) == `` || len(message.Content) > 256*1024 {
				return errors.New(`assistant message is invalid`)
			}
			if _, exists := messageSequences[message.Sequence]; exists {
				return errors.New(`duplicate assistant message sequence`)
			}
			messageSequences[message.Sequence] = struct{}{}
		}
		if err := validateProcessEvents(round.Events); err != nil {
			return err
		}
	}
	return nil
}

// Limits stay aligned with the client uploader. Normal process events are stored
// locally without truncation and must pass through; only a pathological dump is cut.
const (
	maxProcessEventsPerRound      = 5000
	maxProcessEventContentBytes   = 8 * 1024 * 1024
	maxProcessEventsBytesPerRound = 32 * 1024 * 1024
)

func validateProcessEvents(events []define.PipelineTaskProcessEventSnapshot) error {
	if len(events) > maxProcessEventsPerRound {
		return errors.New(`process events are invalid`)
	}
	seen := make(map[int]struct{}, len(events))
	total := 0
	for _, event := range events {
		switch event.Type {
		case `thinking`, `message`, `tool_call`, `tool_result`, `permission_request`:
		default:
			return errors.New(`process event is invalid`)
		}
		if event.Sequence <= 0 || len(event.ToolUseID) > 128 {
			return errors.New(`process event is invalid`)
		}
		if _, exists := seen[event.Sequence]; exists {
			return errors.New(`duplicate process event sequence`)
		}
		seen[event.Sequence] = struct{}{}
		size := len(event.Content)
		if size > maxProcessEventContentBytes {
			return errors.New(`process event is too large`)
		}
		total += size
		if total > maxProcessEventsBytesPerRound {
			return errors.New(`process events are too large`)
		}
	}
	return nil
}

// SaveTaskSession upserts the step-level session projection pushed by the client's
// session sync service. One row per step, keyed by (admin_id, pipeline_task_id, step_key):
// the row holds the step's full conversation JSON plus the aggregated run result. A stale
// out-of-order push (smaller client_updated_at than the stored row) never overwrites newer
// data. When the task projection is missing the push is rejected with a non-2xx error so
// the client records the failure and retries after the task sync arrives.
func SaveTaskSession(adminId, userId int, data define.PipelineTaskSessionSync) (define.PipelineTaskSessionSyncResp, error) {
	if err := validateTaskSessionSync(data); err != nil {
		return define.PipelineTaskSessionSyncResp{}, err
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return define.PipelineTaskSessionSyncResp{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// The session row attaches to the task projection pushed via the task sync API.
	tasks, err := msql.RawValues(define.Postgres, `SELECT id FROM pipeline_tasks
		WHERE admin_id=$1 AND user_id=$2 AND local_task_uuid=$3 AND execution_index=$4 FOR UPDATE`,
		tx, adminId, userId, data.LocalTaskUUID, data.ExecutionIndex)
	if err != nil {
		return define.PipelineTaskSessionSyncResp{}, err
	}
	if len(tasks) == 0 {
		return define.PipelineTaskSessionSyncResp{}, ErrPipelineTaskSessionTargetMissing
	}
	cloudTaskId := cast.ToInt64(tasks[0][`id`])

	// Serialize each task's uploads before touching storage. An old retry must not
	// modify either the database pointer or the bytes referenced by a newer row.
	existing, err := msql.RawValues(define.Postgres, `SELECT client_updated_at, last_uploaded_at
		FROM pipeline_task_sessions WHERE admin_id=$1 AND pipeline_task_id=$2 AND step_key=$3`,
		tx, adminId, cloudTaskId, data.StepKey)
	if err != nil {
		return define.PipelineTaskSessionSyncResp{}, err
	}
	if len(existing) > 0 && cast.ToInt64(existing[0][`client_updated_at`]) >= data.ClientUpdatedAt {
		return define.PipelineTaskSessionSyncResp{
			CloudTaskID: cloudTaskId, StepKey: data.StepKey,
			LastUploadedAt: cast.ToInt64(existing[0][`last_uploaded_at`]), Result: `duplicate`,
		}, nil
	}

	conversationJSON, err := json.Marshal(data.Conversation)
	if err != nil {
		return define.PipelineTaskSessionSyncResp{}, err
	}

	// Each upload gets an immutable key. Even an upload/transaction failure must
	// leave the previous ciphertext and its wrapped DEK as a readable pair.
	// 未配置主密钥时 encryptSessionContent 降级为明文（enc=plain），不阻断上报。
	ciphertext, wrappedDek, enc, err := encryptSessionContent(adminId, conversationJSON)
	if err != nil {
		return define.PipelineTaskSessionSyncResp{}, err
	}
	ossKey := fmt.Sprintf(`teamsboard/%d/%s/%d/%s.enc`, adminId, lib_storage.BusinessSessionLog, cloudTaskId, uuid.NewString())
	storedKey, err := lib_storage.M.UploadRaw(ossKey, ciphertext, ``)
	if err != nil {
		return define.PipelineTaskSessionSyncResp{}, err
	}

	now := tool.Time2Int()
	if _, err = msql.RawExec(define.Postgres, `INSERT INTO pipeline_task_sessions(
		admin_id,user_id,pipeline_task_id,step_key,step_name,status,cli_type,model_name,
		prompt_snapshot,result_status,final_result,exit_code,error_code,error_message,
		input_tokens,output_tokens,total_tokens,conversation_json,conversation_rounds,
		tool_call_count,files_changed,lines_added,lines_deleted,
		conversation_oss_key,conversation_dek_wrapped,conversation_enc,
		started_at,finished_at,duration_ms,last_uploaded_at,client_updated_at,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,'null'::jsonb,
			$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$30)
		ON CONFLICT(admin_id, pipeline_task_id, step_key) DO UPDATE SET
		step_name=EXCLUDED.step_name,status=EXCLUDED.status,cli_type=EXCLUDED.cli_type,
		model_name=EXCLUDED.model_name,prompt_snapshot=EXCLUDED.prompt_snapshot,
		result_status=EXCLUDED.result_status,final_result=EXCLUDED.final_result,
		exit_code=EXCLUDED.exit_code,error_code=EXCLUDED.error_code,
		error_message=EXCLUDED.error_message,input_tokens=EXCLUDED.input_tokens,
		output_tokens=EXCLUDED.output_tokens,total_tokens=EXCLUDED.total_tokens,
		conversation_json='null'::jsonb,conversation_rounds=EXCLUDED.conversation_rounds,
		tool_call_count=EXCLUDED.tool_call_count,files_changed=EXCLUDED.files_changed,
		lines_added=EXCLUDED.lines_added,lines_deleted=EXCLUDED.lines_deleted,
		conversation_oss_key=EXCLUDED.conversation_oss_key,
		conversation_dek_wrapped=EXCLUDED.conversation_dek_wrapped,
		conversation_enc=EXCLUDED.conversation_enc,
		started_at=EXCLUDED.started_at,finished_at=EXCLUDED.finished_at,
		duration_ms=EXCLUDED.duration_ms,last_uploaded_at=EXCLUDED.last_uploaded_at,
		client_updated_at=EXCLUDED.client_updated_at,updated_at=EXCLUDED.updated_at
		WHERE pipeline_task_sessions.client_updated_at <= EXCLUDED.client_updated_at`, tx,
		adminId, userId, cloudTaskId, data.StepKey, data.StepName, data.Status, data.CliType,
		data.ModelName, data.PromptSnapshot, data.ResultStatus, data.FinalResult,
		data.ExitCode, data.ErrorCode, data.ErrorMessage,
		data.InputTokens, data.OutputTokens, data.TotalTokens,
		data.ConversationRounds,
		data.ToolCallCount, data.FilesChanged, data.LinesAdded, data.LinesDeleted,
		storedKey, wrappedDek, enc,
		data.StartedAt, data.FinishedAt, data.DurationMs,
		now, data.ClientUpdatedAt, now); err != nil {
		return define.PipelineTaskSessionSyncResp{}, err
	}

	if err = tx.Commit(); err != nil {
		return define.PipelineTaskSessionSyncResp{}, err
	}
	return define.PipelineTaskSessionSyncResp{
		CloudTaskID:    cloudTaskId,
		StepKey:        data.StepKey,
		LastUploadedAt: int64(now),
		Result:         `accepted`,
	}, nil
}

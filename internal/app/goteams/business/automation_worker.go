package business

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

const (
	automationEmptyDisplay      = "\x00"                 // distinguish an empty display value from a missing variable
	automationWorkerCount       = 1                      // single-instance worker count
	automationPollInterval      = 500 * time.Millisecond // idle polling interval
	automationLeaseSeconds      = 60                     // task lease seconds
	automationMaxAttempts       = 3                      // max attempts per node
	automationWebhookLogMaxSize = 32 * 1024              // max request or response body bytes retained in execution logs
)

// automationClaimSQL claims a task with a row lock and prevents a work item from overtaking earlier tasks.
const automationClaimSQL = `WITH candidate AS (
	SELECT e.id FROM automation_executions e
	WHERE (e.status='pending' OR (e.status='running' AND e.lease_until < $1))
	AND NOT EXISTS (
		SELECT 1 FROM automation_executions prior
		WHERE prior.admin_id=e.admin_id AND prior.workspace_id=e.workspace_id
		AND prior.trigger_item_id=e.trigger_item_id AND prior.id < e.id
		AND prior.status IN ('pending','running')
	)
	ORDER BY e.id ASC FOR UPDATE SKIP LOCKED LIMIT 1
) UPDATE automation_executions e SET status='running', started_at=CASE WHEN started_at=0 THEN $1 ELSE started_at END,
	lease_owner=$2, lease_until=$3, updated_at=$1 FROM candidate WHERE e.id=candidate.id
	RETURNING e.id,e.admin_id,e.workspace_id,e.rule_id,e.rule_name_snapshot,e.creator_id_snapshot,
		e.creator_name_snapshot,e.creator_avatar_snapshot,e.locale,e.config_snapshot,e.trigger_snapshot,
		e.node_progress,e.structured_logs`

// automationWorkers holds the lifecycle of workers in the current process.
var automationWorkers struct {
	sync.Mutex
	cancel context.CancelFunc // stop all workers
	wait   sync.WaitGroup     // wait for running workers to exit
}

// automationExecution is the in-memory execution context after a worker claims a task.
type automationExecution struct {
	ID            int64                     // execution id
	AdminID       int                       // tenant admin id
	WorkspaceID   int                       // workspace id
	RuleID        int                       // rule id
	RuleName      string                    // rule name snapshot
	CreatorID     int                       // rule creator id snapshot
	CreatorName   string                    // rule creator name snapshot
	CreatorAvatar string                    // rule creator avatar snapshot
	Locale        string                    // execution locale
	Config        AutomationConfig          // rule config snapshot
	Snapshot      AutomationTriggerSnapshot // trigger event snapshot
	Progress      AutomationNodeProgress    // node checkpoint
	Logs          []AutomationLog           // structured logs
}

// temporaryAutomationError marks a temporary error that may be retried.
type temporaryAutomationError struct{ err error }

// visibleAutomationError marks a third-party error that may be shown in execution logs.
type visibleAutomationError struct{ err error }

func (e temporaryAutomationError) Error() string { return e.err.Error() }
func (e temporaryAutomationError) Unwrap() error { return e.err }
func (e visibleAutomationError) Error() string   { return e.err.Error() }
func (e visibleAutomationError) Unwrap() error   { return e.err }

// StartAutomationWorkers starts the PostgreSQL persistent queue executor.
func StartAutomationWorkers() {
	automationWorkers.Lock()
	defer automationWorkers.Unlock()
	if automationWorkers.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	automationWorkers.cancel = cancel
	logs.Info("register polling task automation_execution_claim, workers:%d, interval:%s", automationWorkerCount, automationPollInterval)
	for index := 0; index < automationWorkerCount; index++ {
		automationWorkers.wait.Add(1)
		go automationWorkerLoop(ctx, fmt.Sprintf("%s-%d-%d", automationHostname(), os.Getpid(), index))
	}
}

// StopAutomationWorkers stops claiming new tasks and waits for the current node to finish.
func StopAutomationWorkers() {
	automationWorkers.Lock()
	cancel := automationWorkers.cancel
	automationWorkers.cancel = nil
	automationWorkers.Unlock()
	if cancel != nil {
		cancel()
		automationWorkers.wait.Wait()
	}
}

// automationHostname returns the hostname used to distinguish lease owners.
func automationHostname() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "goteams"
	}
	return hostname
}

// automationWorkerLoop continuously claims and executes queue tasks.
func automationWorkerLoop(ctx context.Context, owner string) {
	defer automationWorkers.wait.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// 开源版：授权失效时不领取新任务，带固定退避暂停，恢复后自动继续。
		if !define.LicenseStateCanLogin(GetLicenseSnapshot().State) {
			automationWorkerSleep(ctx, automationLicenseBackoff)
			continue
		}
		execution, err := claimAutomationExecution(owner)
		if err != nil {
			logs.Error("automation claim failed: %s", err.Error())
			automationWorkerSleep(ctx, time.Second)
			continue
		}
		if execution == nil {
			automationWorkerSleep(ctx, automationPollInterval)
			continue
		}
		// 开源版：真正执行前再次检查授权；已领取但未产生副作用的任务安全退回待执行状态。
		if !define.LicenseStateCanLogin(GetLicenseSnapshot().State) {
			if releaseErr := releaseAutomationExecutionForLicense(execution); releaseErr != nil {
				logs.Error("automation execution %d license release failed: %s", execution.ID, releaseErr.Error())
			}
			automationWorkerSleep(ctx, automationLicenseBackoff)
			continue
		}
		if err = runAutomationExecution(ctx, execution); err != nil {
			logs.Error("automation execution %d failed to persist result: %s", execution.ID, err.Error())
		}
	}
}

// automationLicenseBackoff is the pause duration for claiming/execution while the license is invalid.
const automationLicenseBackoff = 30 * time.Second

// releaseAutomationExecutionForLicense returns a claimed-but-not-yet-executed task to the
// pending queue so the lease does not mask it while the license is blocked.
func releaseAutomationExecutionForLicense(execution *automationExecution) error {
	_, err := msql.RawExec(define.Postgres, `UPDATE automation_executions SET status='pending',
		lease_owner='', lease_until=0, updated_at=$2 WHERE admin_id=$3 AND workspace_id=$4 AND id=$1`, nil,
		execution.ID, time.Now().Unix(), execution.AdminID, execution.WorkspaceID)
	return err
}

// automationWorkerSleep provides a wait that can be interrupted by the shutdown signal.
func automationWorkerSleep(ctx context.Context, duration time.Duration) {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// claimAutomationExecution claims and deserializes a task in a short transaction.
func claimAutomationExecution(owner string) (*automationExecution, error) {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().Unix()
	execution := &automationExecution{}
	var configJSON, snapshotJSON, progressJSON, logsJSON string
	err = tx.QueryRow(automationClaimSQL, now, owner, now+automationLeaseSeconds).Scan(
		&execution.ID, &execution.AdminID, &execution.WorkspaceID, &execution.RuleID, &execution.RuleName,
		&execution.CreatorID, &execution.CreatorName, &execution.CreatorAvatar, &execution.Locale,
		&configJSON, &snapshotJSON, &progressJSON, &logsJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if execution.Config, err = decodeAutomationConfig([]byte(configJSON)); err != nil {
		return nil, err
	}
	if err = json.Unmarshal([]byte(snapshotJSON), &execution.Snapshot); err != nil {
		return nil, err
	}
	for _, snapshot := range []*AutomationWorkItemSnapshot{execution.Snapshot.Event.Before, execution.Snapshot.Event.After, execution.Snapshot.Parent, execution.Snapshot.Child} {
		if snapshot == nil {
			continue
		}
		fields, queryErr := msql.RawValues(define.Postgres, `SELECT key FROM workspace_custom_fields WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND field_type='user_multi'`, tx, execution.AdminID, snapshot.WorkspaceID, snapshot.Type)
		if queryErr != nil {
			logs.Error("load automation snapshot member fields failed: admin_id=%d workspace_id=%d err=%s", execution.AdminID, snapshot.WorkspaceID, queryErr.Error())
			return nil, queryErr
		}
		for _, field := range fields {
			normalizeAutomationSnapshotMemberValue(snapshot.Fields, field["key"])
		}
	}
	if err = json.Unmarshal([]byte(progressJSON), &execution.Progress); err != nil {
		return nil, err
	}
	if err = json.Unmarshal([]byte(logsJSON), &execution.Logs); err != nil {
		return nil, err
	}
	if execution.Progress.Completed == nil {
		execution.Progress.Completed = []int{}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return execution, nil
}

// runAutomationExecution executes nodes in config order and resumes from existing checkpoints.
func runAutomationExecution(ctx context.Context, execution *automationExecution) error {
	fields, err := automationFields(execution.AdminID, execution.WorkspaceID, execution.Config.Trigger.Object, execution.Locale)
	if err != nil {
		return failAutomationExecution(execution, err)
	}
	if automationHasCascaderJudgment(execution.Config, fields) {
		return failAutomationExecution(execution, fmt.Errorf("%w: cascader fields cannot be used in automation judgments", ErrAutomationConfigInvalid))
	}

	nodeIndex := 0
	for _, flow := range execution.Config.Nodes {
		if flow.Mode == "direct" {
			if !automationProgressHas(execution.Progress, nodeIndex) {
				if err := runAutomationActionWithRetry(ctx, execution, nodeIndex, *flow.Action); err != nil {
					return failAutomationExecution(execution, err)
				}
			}
			nodeIndex++
			continue
		}
		trigger := execution.Snapshot.Event.After
		if trigger == nil {
			trigger = execution.Snapshot.Event.Before
		}
		types := map[string]string{}
		if trigger != nil {
			fields, err := automationFields(execution.AdminID, execution.WorkspaceID, trigger.Type, execution.Locale)
			if err != nil {
				return failAutomationExecution(execution, err)
			}
			for _, field := range fields {
				types[field.Key] = field.Type
			}
		}
		matchedBranch, skippedFields := evaluateAutomationBranches(flow, trigger.Fields, types)
		matched := matchedBranch >= 0
		execution.Logs = append(execution.Logs, AutomationLog{Time: time.Now().Unix(), Key: map[bool]string{true: "automation_log_condition_matched", false: "automation_log_condition_unmatched"}[matched], Parameters: map[string]any{"skipped_fields": skippedFields}})
		for branchIndex, branch := range automationConditionBranches(flow) {
			for _, action := range branch.Actions {
				if automationProgressHas(execution.Progress, nodeIndex) {
					nodeIndex++
					continue
				}
				if branchIndex != matchedBranch {
					if err := checkpointAutomationNode(execution, nodeIndex, false, "automation_log_condition_action_skipped", nil); err != nil {
						return err
					}
					nodeIndex++
					continue
				}
				if err := runAutomationActionWithRetry(ctx, execution, nodeIndex, action); err != nil {
					return failAutomationExecution(execution, err)
				}
				nodeIndex++
			}
		}
	}
	status := "skipped"
	logKey := "automation_log_execution_skipped"
	if execution.Progress.AnySuccess {
		status = "success"
		logKey = "automation_log_execution_success"
	}
	execution.Logs = append(execution.Logs, AutomationLog{Time: time.Now().Unix(), Key: logKey})
	logsJSON, err := json.Marshal(execution.Logs)
	if err != nil {
		return err
	}
	_, err = msql.RawExec(define.Postgres, `UPDATE automation_executions SET status=$1, finished_at=$2,
		lease_owner='', lease_until=0, structured_logs=$3, updated_at=$2 WHERE admin_id=$4 AND workspace_id=$5 AND id=$6`, nil,
		status, time.Now().Unix(), logsJSON, execution.AdminID, execution.WorkspaceID, execution.ID)
	return err
}

// runAutomationActionWithRetry retries temporary errors with immediate, 1s and 3s delays.
func runAutomationActionWithRetry(ctx context.Context, execution *automationExecution, nodeIndex int, action AutomationAction) error {
	delays := []time.Duration{0, time.Second, 3 * time.Second}
	var lastErr error
	for attempt := 1; attempt <= automationMaxAttempts; attempt++ {
		if delays[attempt-1] > 0 {
			automationWorkerSleep(ctx, delays[attempt-1])
			if ctx.Err() != nil {
				return temporaryAutomationError{ctx.Err()}
			}
		}
		execution.Logs = append(execution.Logs, AutomationLog{Time: time.Now().Unix(), Key: "automation_log_node_attempt", Node: nodeIndex + 1, Attempt: attempt, Parameters: map[string]any{"action": action.Type}})
		_, _, _, err := executeAutomationAction(execution, nodeIndex, action)
		if err == nil {
			return nil
		}
		lastErr = err
		logs.Error("automation execution %d node %d attempt %d failed: %s", execution.ID, nodeIndex+1, attempt, err.Error())
		execution.Logs = append(execution.Logs, AutomationLog{Time: time.Now().Unix(), Key: "automation_log_node_attempt_failed", Node: nodeIndex + 1, Attempt: attempt, Raw: visibleAutomationErrorText(err)})
		if !isTemporaryAutomationError(err) || attempt == automationMaxAttempts {
			break
		}
	}
	return lastErr
}

// executeAutomationAction dispatches the six action types; local actions and checkpoints share a transaction.
func executeAutomationAction(execution *automationExecution, nodeIndex int, action AutomationAction) (succeeded, skipped bool, reason string, err error) {
	if action.Type == "webhook" {
		skipped, reason, err = executeAutomationWebhook(execution, nodeIndex, action)
		if err != nil {
			return false, false, "", err
		}
		if skipped {
			err = checkpointAutomationNode(execution, nodeIndex, false, "automation_log_node_skipped", map[string]any{"action": action.Type, "reason": reason})
			return false, true, reason, err
		}
		if err = checkpointAutomationNode(execution, nodeIndex, true, "automation_log_node_success", map[string]any{"action": action.Type}); err != nil {
			return false, false, "", err
		}
		return true, false, "", nil
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return false, false, "", err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockSLAWorkspacesTx(tx, execution.AdminID, []int{execution.WorkspaceID, action.TargetWorkspaceID}); err != nil {
		return false, false, "", err
	}
	switch action.Type {
	case "status_transition":
		succeeded, skipped, reason, err = executeAutomationStatusTx(tx, execution, action)
	case "field_update":
		succeeded, skipped, reason, err = executeAutomationFieldsTx(tx, execution, action)
	case "add_comment":
		succeeded, skipped, reason, err = executeAutomationCommentTx(tx, execution, action)
	case "create_requirement":
		succeeded, skipped, reason, err = executeAutomationCreateTx(tx, execution, action)
	case "copy_work_item":
		succeeded, skipped, reason, err = executeAutomationCopyTx(tx, execution, action)
	default:
		err = ErrAutomationConfigInvalid
	}
	if err != nil {
		return false, false, "", err
	}
	if err = checkpointAutomationNodeTx(tx, execution, nodeIndex, succeeded, map[bool]string{true: "automation_log_node_success", false: "automation_log_node_skipped"}[succeeded], map[string]any{"action": action.Type, "reason": reason}); err != nil {
		return false, false, "", err
	}
	if err = tx.Commit(); err != nil {
		return false, false, "", err
	}
	// 自动化直接写入工单，不经过普通更新入口，必须在提交后同时失效公开列表与详情。
	if succeeded && (action.Type == "status_transition" || action.Type == "field_update") {
		if cacheErr := RotateIssueInviteRenderVersion(execution.AdminID, execution.WorkspaceID); cacheErr != nil {
			logs.Error("invalidate issue invite cache after automation failed, admin_id:%d, workspace_id:%d, err:%s", execution.AdminID, execution.WorkspaceID, cacheErr.Error())
		}
	}
	return succeeded, skipped, reason, nil
}

// executeAutomationStatusTx performs status transitions; stale status references are skipped by convention.
func executeAutomationStatusTx(tx *sql.Tx, execution *automationExecution, action AutomationAction) (bool, bool, string, error) {
	changed := false
	for _, transition := range action.Transitions {
		targets, err := automationTargets(tx, execution, transition.Scope)
		if err != nil {
			return false, false, "", err
		}
		if len(targets) == 0 {
			continue
		}
		for _, target := range targets {
			statusRows, queryErr := msql.RawValues(define.Postgres, `SELECT name FROM workspace_statuses WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND id=$4`, tx,
				execution.AdminID, execution.WorkspaceID, target["type"], transition.TargetStatusID)
			if queryErr != nil {
				return false, false, "", queryErr
			}
			if len(statusRows) == 0 {
				continue
			}
			itemRows, queryErr := msql.RawValues(define.Postgres, `SELECT workspace_id,status_id,COALESCE(priority,0) AS priority,category_id FROM requirements WHERE admin_id=$1 AND workspace_id=$2 AND id=$3 FOR UPDATE`, tx,
				execution.AdminID, execution.WorkspaceID, cast.ToInt(target["id"]))
			if queryErr != nil {
				return false, false, "", queryErr
			}
			if len(itemRows) == 0 || cast.ToInt(itemRows[0]["status_id"]) == transition.TargetStatusID {
				continue
			}
			if _, queryErr = msql.RawExec(define.Postgres, `UPDATE requirements SET status_id=$1,updated_at=$2 WHERE admin_id=$3 AND workspace_id=$4 AND id=$5`, tx,
				transition.TargetStatusID, time.Now().Unix(), execution.AdminID, execution.WorkspaceID, cast.ToInt(target["id"])); queryErr != nil {
				return false, false, "", queryErr
			}
			if queryErr = createAutomationHistoryTx(tx, execution, automationHistoryModel{WorkspaceID: execution.WorkspaceID, ItemID: cast.ToInt(target["id"]), Action: define.HistoryActionStatusChanged, Field: "status", OldValue: itemRows[0]["status_id"], NewValue: statusRows[0]["name"]}); queryErr != nil {
				return false, false, "", queryErr
			}
			if target["type"] == define.TypeKeyRequirement {
				enabled, err := prepareSLAMutationTx(tx, execution.AdminID, execution.WorkspaceID)
				if err != nil {
					return false, false, "", err
				}
				if enabled {
					before, err := slaStateFromRow(itemRows[0])
					if err != nil {
						return false, false, "", err
					}
					if err = recordSLAUpdatedTx(tx, execution.AdminID, execution.WorkspaceID, cast.ToInt(target["id"]), before, "automation"); err != nil {
						return false, false, "", err
					}
				}
			}
			changed = true
		}
	}
	if !changed {
		return false, true, "target_or_status_missing", nil
	}
	return true, false, "", nil
}

// executeAutomationFieldsTx updates built-in or custom fields and skips stale references.
func executeAutomationFieldsTx(tx *sql.Tx, execution *automationExecution, action AutomationAction) (bool, bool, string, error) {
	targets, err := automationTargets(tx, execution, action.Scope)
	if err != nil {
		return false, false, "", err
	}
	if len(targets) == 0 {
		return false, true, "scope_empty", nil
	}
	changed := false
	var templateValues map[string]string
	for _, target := range targets {
		itemID := cast.ToInt(target["id"])
		for _, update := range action.Updates {
			value, found := automationActionValue(execution, update)
			if !found {
				continue
			}
			if text, ok := value.(string); ok && update.ValueSource == "fixed" && strings.Contains(text, "{{") {
				if templateValues == nil {
					if templateValues, err = automationTemplateValues(execution); err != nil {
						return false, false, "", err
					}
				}
				// An empty render would wipe the field, so it is skipped like any other empty value.
				if value = renderAutomationTemplate(text, templateValues); value == "" {
					continue
				}
			}
			value, found, err = automationMappedBuiltInValueTx(tx, execution, update, target["type"], value)
			if err != nil {
				return false, false, "", err
			}
			if !found {
				continue
			}
			column := automationBuiltInColumn(update.FieldKey)
			if column != "" {
				if oneOf(update.FieldKey, "planned_start_date", "planned_end_date") {
					if unix, ok := automationDateKeywordUnix(execution, value); ok {
						value = unix
					}
				}
				rows, queryErr := msql.RawValues(define.Postgres, `SELECT `+column+` AS old_value FROM requirements WHERE admin_id=$1 AND workspace_id=$2 AND id=$3 FOR UPDATE`, tx, execution.AdminID, execution.WorkspaceID, itemID)
				if queryErr != nil {
					return false, false, "", queryErr
				}
				if len(rows) == 0 {
					continue
				}
				oldValue := rows[0]["old_value"]
				newValue := fmt.Sprint(value)
				databaseValue := value
				if update.FieldKey == "assignee" {
					assigneeIDs := automationAssigneeIDs(value)
					oldValue = normalizeStoredAssigneeIDs(oldValue)
					newValue = assigneeIDsText(assigneeIDs)
					databaseValue = pq.Array(assigneeIDs)
				}
				if automationValuesEqual(oldValue, newValue) {
					continue
				}
				var slaBefore SLAWorkItemStateModel
				slaPriorityChanged := update.FieldKey == "priority" && target["type"] == define.TypeKeyRequirement
				if slaPriorityChanged {
					slaBefore, err = lockSLAWorkItemTx(tx, execution.AdminID, execution.WorkspaceID, itemID)
					if err != nil {
						return false, false, "", err
					}
				}
				if _, queryErr = msql.RawExec(define.Postgres, `UPDATE requirements SET `+column+`=$1,updated_at=$2 WHERE admin_id=$3 AND workspace_id=$4 AND id=$5`, tx, databaseValue, time.Now().Unix(), execution.AdminID, execution.WorkspaceID, itemID); queryErr != nil {
					return false, false, "", queryErr
				}
				if queryErr = createAutomationHistoryTx(tx, execution, automationHistoryModel{WorkspaceID: execution.WorkspaceID, ItemID: itemID, Action: define.HistoryActionFieldUpdated, Field: update.FieldKey, OldValue: oldValue, NewValue: newValue}); queryErr != nil {
					return false, false, "", queryErr
				}
				if slaPriorityChanged {
					if err = recordSLAUpdatedTx(tx, execution.AdminID, execution.WorkspaceID, itemID, slaBefore, "automation"); err != nil {
						return false, false, "", err
					}
				}
				changed = true
				continue
			}
			fieldRows, queryErr := msql.RawValues(define.Postgres, `SELECT `+customFieldQueryFields+` FROM workspace_custom_fields WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND key=$4 AND is_enabled=1`, tx, execution.AdminID, execution.WorkspaceID, target["type"], update.FieldKey)
			if queryErr != nil {
				return false, false, "", queryErr
			}
			if len(fieldRows) == 0 {
				continue
			}
			if fieldRows[0]["field_type"] == "date" && !automationValueEmpty(value) {
				if unix, ok := automationDateKeywordUnix(execution, value); ok {
					value = automationDateText(execution, unix)
				} else {
					value = time.Unix(cast.ToInt64(value), 0).UTC().Format("2006-01-02")
				}
			}
			text, queryErr := automationStoredValue(value)
			if queryErr != nil {
				return false, false, "", queryErr
			}
			field, queryErr := toCustomFieldModel(fieldRows[0])
			if queryErr != nil {
				return false, false, "", queryErr
			}
			valid, queryErr := automationCustomValueValid(execution.AdminID, execution.WorkspaceID, field, text)
			if queryErr != nil {
				return false, false, "", queryErr
			}
			if !valid {
				continue
			}
			oldRows, queryErr := msql.RawValues(define.Postgres, `SELECT value_text FROM requirement_custom_field_values WHERE admin_id=$1 AND requirement_id=$2 AND field_id=$3`, tx, execution.AdminID, itemID, cast.ToInt(fieldRows[0]["id"]))
			if queryErr != nil {
				return false, false, "", queryErr
			}
			oldValue := ""
			if len(oldRows) > 0 {
				oldValue = oldRows[0]["value_text"]
			}
			if oldValue == text {
				continue
			}
			if _, queryErr = msql.RawExec(define.Postgres, `INSERT INTO requirement_custom_field_values(admin_id,requirement_id,field_id,value_text,created_at,updated_at)
				VALUES($1,$2,$3,$4,$5,$5) ON CONFLICT(admin_id,requirement_id,field_id) DO UPDATE SET value_text=EXCLUDED.value_text,updated_at=EXCLUDED.updated_at`, tx,
				execution.AdminID, itemID, cast.ToInt(fieldRows[0]["id"]), text, time.Now().Unix()); queryErr != nil {
				return false, false, "", queryErr
			}
			if queryErr = createAutomationHistoryTx(tx, execution, automationHistoryModel{WorkspaceID: execution.WorkspaceID, ItemID: itemID, Action: define.HistoryActionFieldUpdated, Field: update.FieldKey, OldValue: oldValue, NewValue: text}); queryErr != nil {
				return false, false, "", queryErr
			}
			changed = true
		}
	}
	if !changed {
		return false, true, "value_or_reference_empty", nil
	}
	return true, false, "", nil
}

// executeAutomationCommentTx adds an automation comment signed by the rule creator.
func executeAutomationCommentTx(tx *sql.Tx, execution *automationExecution, action AutomationAction) (bool, bool, string, error) {
	values, err := automationTemplateValues(execution)
	if err != nil {
		return false, false, "", err
	}
	content := strings.TrimSpace(renderAutomationTemplate(action.Template, values))
	if content == "" {
		return false, true, "template_empty", nil
	}
	targets, err := automationTargets(tx, execution, action.Scope)
	if err != nil {
		return false, false, "", err
	}
	if len(targets) == 0 {
		return false, true, "scope_empty", nil
	}
	for _, target := range targets {
		if _, err = msql.RawValues(define.Postgres, `INSERT INTO requirement_comments
			(admin_id,workspace_id,requirement_id,user_id,content,source,automation_rule_id,automation_execution_id,
			author_name_snapshot,author_avatar_snapshot,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,'automation',$6,$7,$8,$9,$10,$10) RETURNING id`, tx,
			execution.AdminID, execution.WorkspaceID, cast.ToInt(target["id"]), execution.CreatorID, content,
			execution.RuleID, execution.ID, execution.CreatorName, execution.CreatorAvatar, time.Now().Unix()); err != nil {
			return false, false, "", err
		}
		if _, err = msql.RawExec(define.Postgres, `UPDATE requirements SET comment_count=comment_count+1,updated_at=$1 WHERE admin_id=$2 AND workspace_id=$3 AND id=$4`, tx,
			time.Now().Unix(), execution.AdminID, execution.WorkspaceID, cast.ToInt(target["id"])); err != nil {
			return false, false, "", err
		}
	}
	return true, false, "", nil
}

// executeAutomationCreateTx creates a new requirement in the current workspace.
func executeAutomationCreateTx(tx *sql.Tx, execution *automationExecution, action AutomationAction) (bool, bool, string, error) {
	if err := CheckStorageQuota(execution.AdminID, 0); err != nil {
		return false, false, "", err
	}
	values, err := automationTemplateValues(execution)
	if err != nil {
		return false, false, "", err
	}
	title := strings.TrimSpace(renderAutomationTemplate(action.Template, values))
	if title == "" {
		return false, true, "template_empty", nil
	}
	parentID := 0
	if action.Parent == "trigger" && execution.Snapshot.Event.ItemType == define.TypeKeyRequirement {
		parentID = execution.Snapshot.Event.ItemID
	}
	statusID, priorityID, err := automationDefaultsTx(tx, execution.AdminID, execution.WorkspaceID, define.TypeKeyRequirement)
	if err != nil {
		return false, false, "", err
	}
	trigger := execution.Snapshot.Event.After
	if trigger == nil {
		trigger = execution.Snapshot.Event.Before
	}
	// Configured fields resolve against the requirement type; unusable values fall back and leave an execution log.
	if action.Status != nil {
		candidate := 0
		if action.Status.Source == "fixed" {
			candidate = action.Status.Value
		} else if trigger != nil {
			candidate = cast.ToInt(trigger.Fields["status"])
		}
		resolved := false
		if candidate > 0 {
			statusRows, queryErr := msql.RawValues(define.Postgres, `SELECT id FROM workspace_statuses WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND id=$4`, tx,
				execution.AdminID, execution.WorkspaceID, define.TypeKeyRequirement, candidate)
			if queryErr != nil {
				return false, false, "", queryErr
			}
			resolved = len(statusRows) > 0
		}
		if resolved {
			statusID = candidate
		} else {
			execution.Logs = append(execution.Logs, AutomationLog{Time: time.Now().Unix(), Key: "automation_log_create_field_fallback", Parameters: map[string]any{"field": "status", "value": candidate}})
		}
	}
	versionID := any(nil)
	assigneeIDs := []int{}
	createFields := []struct {
		key   string
		field *AutomationCreateField
	}{{"priority", action.Priority}, {"version", action.Version}, {"assignee", action.Assignee}}
	for _, item := range createFields {
		if item.field == nil {
			continue
		}
		var value any
		if item.field.Source == "fixed" {
			value = any(item.field.Value)
			if item.key == "assignee" {
				value = item.field.Values
			}
		} else if trigger != nil {
			value = trigger.Fields[item.key]
			if item.key == "assignee" && len(trigger.AssigneeIDs) > 0 {
				value = trigger.AssigneeIDs
			}
		}
		// An empty source value simply leaves the field empty; that is expected and not logged.
		if item.key == "assignee" {
			if len(automationAssigneeIDs(value)) == 0 {
				continue
			}
		} else if cast.ToInt(value) <= 0 {
			continue
		}
		mapped, ok, mapErr := automationMappedBuiltInValueTx(tx, execution, AutomationFieldUpdate{FieldKey: item.key, ValueSource: item.field.Source}, define.TypeKeyRequirement, value)
		if mapErr != nil {
			return false, false, "", mapErr
		}
		if !ok {
			execution.Logs = append(execution.Logs, AutomationLog{Time: time.Now().Unix(), Key: "automation_log_create_field_fallback", Parameters: map[string]any{"field": item.key, "value": value}})
			continue
		}
		switch item.key {
		case "priority":
			priorityID = cast.ToInt(mapped)
		case "version":
			versionID = cast.ToInt(mapped)
		case "assignee":
			assigneeIDs = automationAssigneeIDs(mapped)
		}
	}
	description := strings.TrimSpace(renderAutomationTemplate(action.Description, values))
	categoryId, err := automationChildCategoryTx(tx, execution.AdminID, execution.WorkspaceID, parentID)
	if err != nil {
		return false, false, "", err
	}
	rows, err := msql.RawValues(define.Postgres, `INSERT INTO requirements(admin_id,workspace_id,parent_id,title,description,type,priority,status_id,
		assignee_ids,version_id,planned_start_date,planned_end_date,created_by,created_at,updated_at,category_id)
		VALUES($1,$2,$3,$4,$5,'requirement',$6,$7,$8,$9,$10,$11,$12,$13,$13,$14) RETURNING id`, tx,
		execution.AdminID, execution.WorkspaceID, parentID, title, description, priorityID, statusID,
		pq.Array(assigneeIDs), versionID, automationCreateDateUnix(execution, trigger, action.PlannedStart, "planned_start_date"),
		automationCreateDateUnix(execution, trigger, action.PlannedEnd, "planned_end_date"), execution.CreatorID, time.Now().Unix(), categoryId)
	if err != nil {
		return false, false, "", err
	}
	if err = createAutomationHistoryTx(tx, execution, automationHistoryModel{WorkspaceID: execution.WorkspaceID, ItemID: cast.ToInt(rows[0]["id"]), Action: define.HistoryActionCreated, Field: "", OldValue: "", NewValue: title}); err != nil {
		return false, false, "", err
	}
	if err = recordSLACreatedTx(tx, execution.AdminID, execution.WorkspaceID, cast.ToInt(rows[0]["id"]), "automation"); err != nil {
		return false, false, "", err
	}
	return true, false, "", nil
}

// automationCreateDateUnix resolves a create_requirement date keyword into a Unix timestamp; zero leaves the field empty.
func automationCreateDateUnix(execution *automationExecution, trigger *AutomationWorkItemSnapshot, keyword, fieldKey string) int64 {
	if keyword == "trigger" {
		if trigger == nil {
			return 0
		}
		return cast.ToInt64(trigger.Fields[fieldKey])
	}
	unix, ok := automationDateKeywordUnix(execution, keyword)
	if !ok {
		return 0
	}
	return unix
}

// executeAutomationCopyTx copies the trigger work item to a permitted target workspace.
func executeAutomationCopyTx(tx *sql.Tx, execution *automationExecution, action AutomationAction) (bool, bool, string, error) {
	if err := CheckStorageQuota(execution.AdminID, 0); err != nil {
		return false, false, "", err
	}
	if !HasPermission(execution.AdminID, execution.CreatorID, cast.ToString(action.TargetWorkspaceID), execution.Snapshot.Event.ItemType+":create") {
		return false, true, "creator_permission_missing", nil
	}
	trigger := execution.Snapshot.Event.After
	if trigger == nil {
		trigger = execution.Snapshot.Event.Before
	}
	if trigger == nil || !automationWorkspaceSupportsType(execution.AdminID, action.TargetWorkspaceID, trigger.Type) {
		return false, true, "target_type_missing", nil
	}
	parentID := 0
	if action.TargetWorkspaceID == execution.WorkspaceID && action.Parent == "trigger" && trigger.Type == define.TypeKeyRequirement {
		parentID = trigger.ID
	}
	statusID, priorityID, err := automationDefaultsTx(tx, execution.AdminID, action.TargetWorkspaceID, trigger.Type)
	if err != nil {
		return false, false, "", err
	}
	if sourcePriority := cast.ToInt(trigger.Fields["priority"]); sourcePriority > 0 {
		priorityRows, queryErr := msql.RawValues(define.Postgres, `SELECT target.id FROM workspace_priorities source JOIN workspace_priorities target
			ON target.admin_id=source.admin_id AND target.workspace_id=$3 AND target.type_key=source.type_key AND target.key=source.key
			WHERE source.admin_id=$1 AND source.workspace_id=$2 AND source.id=$4`, tx, execution.AdminID, execution.WorkspaceID, action.TargetWorkspaceID, sourcePriority)
		if queryErr != nil {
			return false, false, "", queryErr
		}
		if len(priorityRows) > 0 {
			priorityID = cast.ToInt(priorityRows[0]["id"])
		}
	}
	// Only members of the target workspace can stay assigned after the copy.
	assigneeIDs := trigger.AssigneeIDs
	if len(assigneeIDs) == 0 {
		assigneeIDs = automationAssigneeIDs(trigger.Fields["assignee"])
	}
	if len(assigneeIDs) > 0 {
		memberRows, queryErr := msql.RawValues(define.Postgres, `SELECT user_id FROM workspace_members WHERE admin_id=$1 AND workspace_id=$2 AND user_id=ANY($3)`, tx, execution.AdminID, action.TargetWorkspaceID, pq.Array(assigneeIDs))
		if queryErr != nil {
			return false, false, "", queryErr
		}
		assigneeIDs = automationKeepMembers(assigneeIDs, memberRows)
	}
	categoryId, err := automationChildCategoryTx(tx, execution.AdminID, action.TargetWorkspaceID, parentID)
	if err != nil {
		return false, false, "", err
	}
	rows, err := msql.RawValues(define.Postgres, `INSERT INTO requirements(admin_id,workspace_id,parent_id,title,description,type,priority,status_id,
		assignee_ids,version_id,planned_start_date,planned_end_date,created_by,created_at,updated_at,category_id)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,NULL,$10,$11,$12,$13,$13,$14) RETURNING id`, tx,
		execution.AdminID, action.TargetWorkspaceID, parentID, trigger.Title, trigger.Description, trigger.Type, priorityID, statusID,
		pq.Array(assigneeIDs), cast.ToInt64(trigger.Fields["planned_start_date"]), cast.ToInt64(trigger.Fields["planned_end_date"]), execution.CreatorID, time.Now().Unix(), categoryId)
	if err != nil {
		return false, false, "", err
	}
	newID := cast.ToInt(rows[0]["id"])
	if err = copyAutomationCustomFieldsTx(tx, execution, trigger, action.TargetWorkspaceID, newID); err != nil {
		return false, false, "", err
	}
	if err = createAutomationHistoryTx(tx, execution, automationHistoryModel{WorkspaceID: action.TargetWorkspaceID, ItemID: newID, Action: define.HistoryActionCreated, Field: "", OldValue: "", NewValue: trigger.Title}); err != nil {
		return false, false, "", err
	}
	if err = recordSLACreatedTx(tx, execution.AdminID, action.TargetWorkspaceID, newID, "automation"); err != nil {
		return false, false, "", err
	}
	return true, false, "", nil
}

func automationChildCategoryTx(tx *sql.Tx, adminId, workspaceId, parentId int) (int, error) {
	if parentId == 0 {
		return 0, nil
	}
	if err := lockRequirementHierarchyTx(tx, adminId, workspaceId); err != nil {
		return 0, err
	}
	return requirementParentCategoryTx(tx, adminId, workspaceId, parentId)
}

// copyAutomationCustomFieldsTx copies fields supported by the target workspace by stable field key.
func copyAutomationCustomFieldsTx(tx *sql.Tx, execution *automationExecution, trigger *AutomationWorkItemSnapshot, targetWorkspaceID, targetItemID int) error {
	for key, value := range trigger.Fields {
		if automationValueInvalid(value) {
			execution.Logs = append(execution.Logs, AutomationLog{Time: time.Now().Unix(), Key: "automation_log_copy_field_skipped", Parameters: map[string]any{"field": key}})
			continue
		}
		if automationBuiltInColumn(key) != "" || oneOf(key, "id", "status", "created_at", "created_by", "updated_at") {
			continue
		}
		fields, err := msql.RawValues(define.Postgres, `SELECT target.id,target.key,target.name,target.field_type,target.is_required,target.sort_order,target.is_enabled,target.config_json FROM workspace_custom_fields source JOIN workspace_custom_fields target
			ON target.admin_id=source.admin_id AND target.workspace_id=$3 AND target.type_key=source.type_key
			AND target.key=source.key AND target.field_type=source.field_type
			WHERE source.admin_id=$1 AND source.workspace_id=$2 AND source.type_key=$4 AND source.key=$5 AND target.is_enabled=1`, tx,
			execution.AdminID, execution.WorkspaceID, targetWorkspaceID, trigger.Type, key)
		if err != nil {
			return err
		}
		if len(fields) == 0 {
			execution.Logs = append(execution.Logs, AutomationLog{Time: time.Now().Unix(), Key: "automation_log_copy_field_skipped", Parameters: map[string]any{"field": key}})
			continue
		}
		if fields[0]["field_type"] == "date" && !automationValueEmpty(value) {
			value = time.Unix(cast.ToInt64(value), 0).UTC().Format("2006-01-02")
		}
		text, err := automationStoredValue(value)
		if err != nil {
			return err
		}
		field, err := toCustomFieldModel(fields[0])
		if err != nil {
			return err
		}
		valid, err := automationCustomValueValid(execution.AdminID, targetWorkspaceID, field, text)
		if err != nil {
			return err
		}
		if !valid {
			execution.Logs = append(execution.Logs, AutomationLog{Time: time.Now().Unix(), Key: "automation_log_copy_field_skipped", Parameters: map[string]any{"field": key}})
			continue
		}

		if _, err = msql.RawExec(define.Postgres, `INSERT INTO requirement_custom_field_values(admin_id,requirement_id,field_id,value_text,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$5)`, tx, execution.AdminID, targetItemID, cast.ToInt(fields[0]["id"]), text, time.Now().Unix()); err != nil {
			return err
		}
	}
	return nil
}

// executeAutomationWebhook sends a POST JSON request and records a length-limited response.
func executeAutomationWebhook(execution *automationExecution, nodeIndex int, action AutomationAction) (bool, string, error) {
	values, err := automationTemplateValues(execution)
	if err != nil {
		return false, "", err
	}
	renderedURL, err := renderWebhookURL(action.URL, values)
	if err != nil {
		return false, "", err
	}
	if renderedURL == "" {
		return true, "template_empty", nil
	}
	idempotencyKey := automationIdempotencyKey(execution.ID, nodeIndex)
	var encoded []byte
	if action.PushSystem == "" || action.PushSystem == "custom" {
		encoded, err = automationCustomWebhookBody(execution, action, values, idempotencyKey)
	} else {
		renderedURL, encoded, err = automationRobotRequest(execution, nodeIndex, action, values, renderedURL)
	}
	if err != nil {
		return false, "", err
	}
	if encoded == nil {
		return true, "template_empty", nil
	}
	logAutomationWebhookRequest(execution, nodeIndex, action, renderedURL, idempotencyKey, encoded, values)
	client := automationHTTPClient()
	request, err := http.NewRequest(http.MethodPost, renderedURL, bytes.NewReader(encoded))
	if err != nil {
		return false, "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-GoTeams-Idempotency-Key", idempotencyKey)
	response, err := client.Do(request)
	if err != nil {
		return false, "", temporaryAutomationError{visibleAutomationError{err}}
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			logs.Error("close automation webhook response failed: %s", closeErr.Error())
		}
	}()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, automationWebhookLogMaxSize+1))
	if readErr != nil {
		return false, "", temporaryAutomationError{visibleAutomationError{readErr}}
	}
	responseBodyBytes := len(responseBody)
	responseBodyTruncated := responseBodyBytes > automationWebhookLogMaxSize
	if responseBodyTruncated {
		responseBody = responseBody[:automationWebhookLogMaxSize]
	}
	execution.Logs = append(execution.Logs, AutomationLog{Time: time.Now().Unix(), Key: "automation_log_webhook_response", Node: nodeIndex + 1, Parameters: map[string]any{
		"url": renderedURL, "status": response.StatusCode, "body_bytes": responseBodyBytes, "body_truncated": responseBodyTruncated,
	}, Raw: string(responseBody)})
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return false, "", nil
	}
	err = fmt.Errorf("webhook HTTP %d", response.StatusCode)
	if response.StatusCode == http.StatusRequestTimeout || response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
		return false, "", temporaryAutomationError{visibleAutomationError{err}}
	}
	return false, "", visibleAutomationError{err}
}

// automationWebhookRule describes the rule section of the default webhook payload.
type automationWebhookRule struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

// automationWebhookExecution describes the execution section of the default webhook payload.
type automationWebhookExecution struct {
	Id int64 `json:"id"`
}

// AutomationDisplayValues describes rendered template values keyed by variable name.
type AutomationDisplayValues map[string]string

// automationWebhookPayload is the default event payload sent when no custom body is configured.
type automationWebhookPayload struct {
	IdempotencyKey string                      `json:"idempotency_key"`
	Rule           automationWebhookRule       `json:"rule"`
	Execution      automationWebhookExecution  `json:"execution"`
	EventType      string                      `json:"event_type"`
	Trigger        *AutomationWorkItemSnapshot `json:"trigger"`
	Before         *AutomationWorkItemSnapshot `json:"before"`
	After          *AutomationWorkItemSnapshot `json:"after"`
	TriggeredBy    AutomationUserSnapshot      `json:"triggered_by"`
	TriggeredAt    int64                       `json:"triggered_at"`
	DisplayValues  AutomationDisplayValues     `json:"display_values"`
}

// automationCustomWebhookBody renders the configured body, or falls back to the default event payload.
func automationCustomWebhookBody(execution *automationExecution, action AutomationAction, values map[string]string, idempotencyKey string) ([]byte, error) {
	// The body is JSON text, so variable values are escaped to preserve quotes, slashes, and newlines.
	if strings.TrimSpace(action.Body) != "" {
		escaped := make(map[string]string, len(values))
		for key, value := range values {
			quoted, err := json.Marshal(value)
			if err != nil {
				return nil, err
			}
			escaped[key] = string(quoted[1 : len(quoted)-1])
		}
		return []byte(renderAutomationTemplate(action.Body, escaped)), nil
	}
	event := execution.Snapshot.Event
	return json.Marshal(automationWebhookPayload{
		IdempotencyKey: idempotencyKey,
		Rule:           automationWebhookRule{Id: execution.RuleID, Name: execution.RuleName},
		Execution:      automationWebhookExecution{Id: execution.ID},
		EventType:      event.Operation,
		Trigger:        event.After, Before: event.Before, After: event.After,
		TriggeredBy: event.TriggeredBy, TriggeredAt: event.TriggeredAt,
		DisplayValues: values,
	})
}

func logAutomationWebhookRequest(execution *automationExecution, nodeIndex int, action AutomationAction, renderedURL, idempotencyKey string, encoded []byte, values map[string]string) {
	body := encoded
	bodyTruncated := len(body) > automationWebhookLogMaxSize
	if bodyTruncated {
		body = body[:automationWebhookLogMaxSize]
	}
	emptyVariables := map[string]bool{}
	for _, template := range []string{action.URL, action.Body, action.Template} {
		for _, match := range templateVariablePattern.FindAllStringSubmatch(template, -1) {
			if values[match[1]] == "" {
				emptyVariables[match[1]] = true
			}
		}
	}
	emptyVariableKeys := make([]string, 0, len(emptyVariables))
	for key := range emptyVariables {
		emptyVariableKeys = append(emptyVariableKeys, key)
	}
	sort.Strings(emptyVariableKeys)
	pushSystem := action.PushSystem
	if pushSystem == "" {
		pushSystem = "custom"
	}
	execution.Logs = append(execution.Logs, AutomationLog{Time: time.Now().Unix(), Key: "automation_log_webhook_request", Node: nodeIndex + 1, Parameters: map[string]any{
		"push_system": pushSystem, "method": http.MethodPost, "url": automationMaskedURL(renderedURL), "content_type": "application/json",
		"idempotency_key": idempotencyKey, "body_bytes": len(encoded), "body_truncated": bodyTruncated, "empty_variables": emptyVariableKeys,
	}, Raw: string(body)})
}

// automationMaskedURL hides the robot token carried by the webhook query string.
func automationMaskedURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	query := parsed.Query()
	for _, key := range []string{"access_token", "key", "sign"} {
		if query.Get(key) != "" {
			query.Set(key, "***")
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

// automationIdempotencyKey generates a stable webhook idempotency key for each execution node.
func automationIdempotencyKey(executionID int64, nodeIndex int) string {
	return fmt.Sprintf("goteams-%d-%d", executionID, nodeIndex+1)
}

// automationHTTPClient creates a secure client that disables proxies and validates every connection address.
func automationHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, errors.New("host has no address")
		}
		for _, ip := range ips {
			if !publicAutomationIP(ip) {
				return nil, errors.New("private address is not allowed")
			}
		}
		var lastErr error
		for _, ip := range ips {
			conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, errors.New("unable to connect to resolved host")
	}}
	client := &http.Client{Timeout: 10 * time.Second, Transport: transport}
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errors.New("too many redirects")
		}
		if request.URL.Scheme != "http" && request.URL.Scheme != "https" {
			return errors.New("redirect scheme is not allowed")
		}
		return validateAutomationHost(request.URL.Hostname(), func(host string) ([]net.IP, error) {
			return net.LookupIP(host)
		})
	}
	return client
}

// automationTargets returns the trigger item or parent requirement based on the action scope.
func automationTargets(tx *sql.Tx, execution *automationExecution, scope string) ([]msql.Params, error) {
	itemID := execution.Snapshot.Event.ItemID
	switch scope {
	case "trigger":
		return msql.RawValues(define.Postgres, `SELECT id,type FROM requirements WHERE admin_id=$1 AND workspace_id=$2 AND id=$3`, tx, execution.AdminID, execution.WorkspaceID, itemID)
	case "parent":
		trigger := execution.Snapshot.Event.After
		if trigger == nil {
			trigger = execution.Snapshot.Event.Before
		}
		if trigger == nil || trigger.ParentID == 0 {
			return []msql.Params{}, nil
		}
		return msql.RawValues(define.Postgres, `SELECT id,type FROM requirements WHERE admin_id=$1 AND workspace_id=$2 AND id=$3 AND type='requirement'`, tx, execution.AdminID, execution.WorkspaceID, trigger.ParentID)
	case "descendants":
		return msql.RawValues(define.Postgres, `WITH RECURSIVE tree AS (
			SELECT id,type,created_at,1 AS depth FROM requirements WHERE admin_id=$1 AND workspace_id=$2 AND parent_id=$3 AND type='requirement'
			UNION ALL SELECT r.id,r.type,r.created_at,t.depth+1 FROM requirements r JOIN tree t ON r.parent_id=t.id
			WHERE r.admin_id=$1 AND r.workspace_id=$2 AND r.type='requirement'
		) SELECT id,type FROM tree ORDER BY depth ASC,created_at ASC,id ASC`, tx, execution.AdminID, execution.WorkspaceID, itemID)
	default:
		return nil, ErrAutomationConfigInvalid
	}
}

// automationActionValue reads a fixed value or an event snapshot field value.
func automationActionValue(execution *automationExecution, update AutomationFieldUpdate) (any, bool) {
	if update.ValueSource == "fixed" {
		value := rawJSONValue(update.Value)
		return value, !automationValueEmpty(value)
	}
	var snapshot *AutomationWorkItemSnapshot
	switch update.ValueSource {
	case "trigger":
		snapshot = execution.Snapshot.Event.After
	case "parent":
		snapshot = execution.Snapshot.Parent
	case "child":
		snapshot = execution.Snapshot.Child
	}
	if snapshot == nil {
		return nil, false
	}
	value, ok := snapshot.Fields[update.SourceFieldKey]
	// The assignee field carries every member, so the whole group is copied instead of the first one.
	if update.SourceFieldKey == "assignee" && len(snapshot.AssigneeIDs) > 0 {
		return snapshot.AssigneeIDs, true
	}
	return value, ok && !automationValueInvalid(value) && !automationValueEmpty(value)
}

// automationKeepMembers keeps the ids that are still workspace members, preserving the original order.
func automationKeepMembers(ids []int, memberRows []msql.Params) []int {
	members := make(map[int]bool, len(memberRows))
	for _, row := range memberRows {
		members[cast.ToInt(row["user_id"])] = true
	}
	result := []int{}
	for _, id := range ids {
		if members[id] {
			result = append(result, id)
		}
	}
	return result
}

// automationDateKeywordUnix resolves a relative fixed date value into the start-of-day timestamp of the execution timezone.
// A zero timestamp clears the field; ok is false for any value that is not a relative keyword.
func automationDateKeywordUnix(execution *automationExecution, value any) (int64, bool) {
	keyword, ok := value.(string)
	if !ok {
		return 0, false
	}
	if keyword == automationDateClear {
		return 0, true
	}
	offset, exists := automationDateOffsets[keyword]
	if !exists {
		return 0, false
	}
	triggered := time.Unix(execution.Snapshot.Event.TriggeredAt, 0).In(automationTimeLocation(execution))
	return time.Date(triggered.Year(), triggered.Month(), triggered.Day()+offset, 0, 0, 0, 0, triggered.Location()).Unix(), true
}

// automationDateText formats a relative date timestamp in the execution timezone; a zero timestamp clears the field.
func automationDateText(execution *automationExecution, unix int64) string {
	if unix == 0 {
		return ""
	}
	return time.Unix(unix, 0).In(automationTimeLocation(execution)).Format("2006-01-02")
}

// automationMappedBuiltInValueTx maps a stable key across types or workspaces to a target ID.
func automationMappedBuiltInValueTx(tx *sql.Tx, execution *automationExecution, update AutomationFieldUpdate, targetType string, value any) (any, bool, error) {
	switch update.FieldKey {
	case "priority":
		rows, err := msql.RawValues(define.Postgres, `SELECT id FROM workspace_priorities WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND id=$4`, tx, execution.AdminID, execution.WorkspaceID, targetType, cast.ToInt(value))
		if err != nil {
			return nil, false, err
		}
		if len(rows) > 0 {
			return value, true, nil
		}
		sourceType := execution.Snapshot.Event.ItemType
		if update.ValueSource == "parent" || update.ValueSource == "child" {
			sourceType = define.TypeKeyRequirement
		}
		rows, err = msql.RawValues(define.Postgres, `SELECT target.id FROM workspace_priorities source JOIN workspace_priorities target
			ON target.admin_id=source.admin_id AND target.workspace_id=source.workspace_id AND target.type_key=$4 AND target.key=source.key
			WHERE source.admin_id=$1 AND source.workspace_id=$2 AND source.type_key=$3 AND source.id=$5`, tx,
			execution.AdminID, execution.WorkspaceID, sourceType, targetType, cast.ToInt(value))
		if err != nil || len(rows) == 0 {
			return nil, false, err
		}
		return cast.ToInt(rows[0]["id"]), true, nil
	case "assignee":
		assigneeIDs := automationAssigneeIDs(value)
		if len(assigneeIDs) == 0 {
			return nil, false, nil
		}
		rows, err := msql.RawValues(define.Postgres, `SELECT user_id FROM workspace_members WHERE admin_id=$1 AND workspace_id=$2 AND user_id=ANY($3)`, tx, execution.AdminID, execution.WorkspaceID, pq.Array(assigneeIDs))
		if err != nil {
			return nil, false, err
		}
		kept := automationKeepMembers(assigneeIDs, rows)
		return kept, len(kept) > 0, nil
	case "version":
		rows, err := msql.RawValues(define.Postgres, `SELECT id FROM versions WHERE admin_id=$1 AND workspace_id=$2 AND id=$3`, tx, execution.AdminID, execution.WorkspaceID, cast.ToInt(value))
		return value, err == nil && len(rows) > 0, err
	default:
		return value, true, nil
	}
}

// automationBuiltInColumn returns the column name of a built-in field that may be updated directly.
func automationBuiltInColumn(key string) string {
	columns := map[string]string{"title": "title", "description": "description", "priority": "priority", "assignee": "assignee_ids", "version": "version_id", "planned_start_date": "planned_start_date", "planned_end_date": "planned_end_date"}
	return columns[key]
}

// automationStoredValue converts a field value into its database text representation.
func automationStoredValue(value any) (string, error) {
	if automationValueInvalid(value) {
		return "", ErrAutomationConfigInvalid
	}
	if value == nil {
		return "", nil
	}
	if text, ok := value.(string); ok {
		return text, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	if _, ok := value.([]string); ok {
		return string(encoded), nil
	}
	if _, ok := value.([]any); ok {
		return string(encoded), nil
	}
	return strings.Trim(string(encoded), `"`), nil
}

// automationDefaultsTx returns the default status and priority for a work item type.
func automationDefaultsTx(tx *sql.Tx, adminID, workspaceID int, itemType string) (int, int, error) {
	statusField := "requirement_initial_status_id"
	if itemType == define.TypeKeyDefect {
		statusField = "defect_initial_status_id"
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT `+statusField+` AS status_id FROM workspaces_settings WHERE admin_id=$1 AND workspace_id=$2`, tx, adminID, workspaceID)
	if err != nil || len(rows) == 0 {
		return 0, 0, err
	}
	priorityRows, err := msql.RawValues(define.Postgres, `SELECT id FROM workspace_priorities WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 ORDER BY is_default DESC,sort_order ASC,id ASC LIMIT 1`, tx, adminID, workspaceID, itemType)
	if err != nil {
		return 0, 0, err
	}
	priorityID := 0
	if len(priorityRows) > 0 {
		priorityID = cast.ToInt(priorityRows[0]["id"])
	}
	return cast.ToInt(rows[0]["status_id"]), priorityID, nil
}

type automationHistoryModel struct {
	WorkspaceID int
	ItemID      int
	Action      string
	Field       string
	OldValue    string
	NewValue    string
}

// createAutomationHistoryTx 保存事件所在工作空间，来源规则仍保留原执行信息。
func createAutomationHistoryTx(tx *sql.Tx, execution *automationExecution, model automationHistoryModel) error {
	snapshot, err := captureProcessingSnapshotTx(tx, execution.AdminID, model.WorkspaceID, model.ItemID, model.Action, model.Field)
	if err != nil {
		return err
	}
	actor := strings.TrimSpace(execution.RuleName + " · " + execution.CreatorName)
	query := `INSERT INTO ` + define.TableRequirementHistories + `(admin_id,workspace_id,requirement_id,user_id,
 action,field_name,old_value,new_value,source,automation_rule_id,automation_execution_id,actor_name_snapshot,created_at,processing_snapshot)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,'automation',$9,$10,$11,$12,$13::jsonb)`
	_, err = msql.RawExec(define.Postgres, query, tx, execution.AdminID, model.WorkspaceID, model.ItemID, execution.CreatorID, model.Action, model.Field, model.OldValue, model.NewValue, execution.RuleID, execution.ID, actor, time.Now().Unix(), snapshot)
	if err != nil {
		logs.Error("automation history sql:%s, err:%s", query, err.Error())
	}
	return err
}

// automationWorkItemLink builds the work item detail URL from the configured site URL.
// An unset site_url yields an empty string so templates render nothing instead of a broken link.
func automationWorkItemLink(workspaceID, itemID int) string {
	base := strings.TrimRight(define.Config.WebService[`site_url`], `/`)
	if base == `` || workspaceID <= 0 || itemID <= 0 {
		return ``
	}
	return fmt.Sprintf(`%s/workspace/%d/board?detail=%d`, base, workspaceID, itemID)
}

// automationTemplateValues builds template variables for the trigger item, parent requirement and triggering user.
func automationTemplateValues(execution *automationExecution) (map[string]string, error) {
	values := map[string]string{
		"rule.name":            execution.RuleName,
		"execution.id":         strconv.FormatInt(execution.ID, 10),
		"event.time":           time.Unix(execution.Snapshot.Event.TriggeredAt, 0).In(automationTimeLocation(execution)).Format("2006-01-02 15:04:05"),
		"triggered_by.id":      strconv.Itoa(execution.Snapshot.Event.TriggeredBy.ID),
		"triggered_by.name":    execution.Snapshot.Event.TriggeredBy.Name,
		"triggered_by.account": execution.Snapshot.Event.TriggeredBy.Account,
		"triggered_by.phone":   execution.Snapshot.Event.TriggeredBy.Phone,
		"triggered_by.remark":  execution.Snapshot.Event.TriggeredBy.Remark,
	}
	fill := func(prefix string, snapshot *AutomationWorkItemSnapshot) error {
		if snapshot == nil {
			return nil
		}
		values[prefix+".id"] = strconv.Itoa(snapshot.ID)
		values[prefix+".link"] = automationWorkItemLink(snapshot.WorkspaceID, snapshot.ID)
		values[prefix+".title"] = snapshot.Title
		values[prefix+".description"] = snapshot.Description
		for key, value := range snapshot.Fields {
			if automationValueInvalid(value) || automationValueEmpty(value) {
				values[prefix+".field."+key] = ""
				continue
			}
			if user, ok, userErr := automationUserFieldDisplay(execution, snapshot, key, value); userErr != nil {
				return userErr
			} else if ok {
				values[prefix+".field."+key+".name"] = user.Name
				values[prefix+".field."+key+".account"] = user.Account
				values[prefix+".field."+key+".phone"] = user.Phone
				values[prefix+".field."+key+".remark"] = user.Remark
				values[prefix+".field."+key] = user.Name // backward compat for legacy rule templates
				continue
			}
			display, err := automationTemplateFieldDisplay(execution, snapshot, key, value)
			if err != nil {
				return err
			}
			if display == automationEmptyDisplay {
				values[prefix+".field."+key] = ""
				continue
			}
			if display != "" {
				values[prefix+".field."+key] = display
				continue
			}
			if set, ok := automationStringSet(value); ok {
				items := make([]string, 0, len(set))
				for item := range set {
					items = append(items, item)
				}
				values[prefix+".field."+key] = strings.Join(items, ",")
			} else {
				values[prefix+".field."+key] = fmt.Sprint(value)
			}
		}
		return nil
	}
	trigger := execution.Snapshot.Event.After
	if trigger == nil {
		trigger = execution.Snapshot.Event.Before
	}
	if err := fill("trigger", trigger); err != nil {
		return nil, err
	}
	if err := fill("parent", execution.Snapshot.Parent); err != nil {
		return nil, err
	}
	if err := fill("child", execution.Snapshot.Child); err != nil {
		return nil, err
	}
	return values, nil
}

// automationTimeLocation prefers the triggering user timezone, then falls back to the rule timezone and UTC.
func automationTimeLocation(execution *automationExecution) *time.Location {
	for _, timezone := range []string{execution.Snapshot.Event.Timezone, execution.Config.Timezone} {
		timezone = strings.TrimSpace(timezone)
		if timezone == "" {
			continue
		}
		if location, err := time.LoadLocation(timezone); err == nil {
			return location
		}
	}
	return time.UTC
}

type automationUserDisplayModel struct {
	Name    string
	Account string
	Phone   string
	Remark  string
}

// automationUserFieldDisplay resolves built-in user fields (assignee, created_by); every assignee is joined by comma.
func automationUserFieldDisplay(execution *automationExecution, snapshot *AutomationWorkItemSnapshot, key string, value any) (automationUserDisplayModel, bool, error) {
	if key != "assignee" && key != "created_by" {
		if snapshot == nil || !strings.HasPrefix(key, "cf_") {
			return automationUserDisplayModel{}, false, nil
		}
		fields, err := msql.RawValues(define.Postgres, `SELECT field_type FROM workspace_custom_fields WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND key=$4`, nil, execution.AdminID, snapshot.WorkspaceID, snapshot.Type, key)
		if err != nil {
			return automationUserDisplayModel{}, false, err
		}
		if len(fields) == 0 || !customFieldIsMember(fields[0]["field_type"]) {
			return automationUserDisplayModel{}, false, nil
		}
	}
	userIDs := []int{}
	if key == "assignee" {
		if snapshot != nil && len(snapshot.AssigneeIDs) > 0 {
			userIDs = snapshot.AssigneeIDs
		} else {
			userIDs = automationAssigneeIDs(value)
		}
	} else {
		userIDs = automationAssigneeIDs(value)
	}
	if len(userIDs) == 0 {
		return automationUserDisplayModel{}, true, nil
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT id,name,username,tel_number,remark FROM users WHERE admin_id=$1 AND id=ANY($2)`, nil, execution.AdminID, pq.Array(userIDs))
	if err != nil {
		return automationUserDisplayModel{}, true, err
	}
	indexed := make(map[int]msql.Params, len(rows))
	for _, row := range rows {
		indexed[cast.ToInt(row["id"])] = row
	}
	join := func(current, next string) string {
		if next == "" {
			return current
		}
		if current == "" {
			return next
		}
		return current + "," + next
	}
	display := automationUserDisplayModel{}
	for _, userID := range userIDs {
		row, exists := indexed[userID]
		if !exists {
			continue
		}
		display.Name = join(display.Name, lib_i18n.ShowStored(execution.Locale, row["name"]))
		display.Account = join(display.Account, row["username"])
		display.Phone = join(display.Phone, row["tel_number"])
		display.Remark = join(display.Remark, row["remark"])
	}
	return display, true, nil
}

// automationTemplateFieldDisplay converts a raw field value into a readable name.
func automationTemplateFieldDisplay(execution *automationExecution, snapshot *AutomationWorkItemSnapshot, key string, value any) (string, error) {
	var query string
	var args []any
	switch key {
	case "id", "title", "description":
		// Built-in text fields have no lookup table, the snapshot value is already the display value.
		return fmt.Sprint(value), nil
	case "status":
		query = `SELECT name FROM workspace_statuses WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND id=$4`
		args = []any{execution.AdminID, snapshot.WorkspaceID, snapshot.Type, cast.ToInt(value)}
	case "priority":
		query = `SELECT name FROM workspace_priorities WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND id=$4`
		args = []any{execution.AdminID, snapshot.WorkspaceID, snapshot.Type, cast.ToInt(value)}
	case "assignee", "created_by":
		query = `SELECT name FROM users WHERE admin_id=$1 AND id=$2`
		args = []any{execution.AdminID, cast.ToInt(value)}
	case "version":
		query = `SELECT name FROM versions WHERE admin_id=$1 AND workspace_id=$2 AND id=$3`
		args = []any{execution.AdminID, snapshot.WorkspaceID, cast.ToInt(value)}
	case "planned_start_date", "planned_end_date":
		if timestamp := cast.ToInt64(value); timestamp > 0 {
			return time.Unix(timestamp, 0).In(automationTimeLocation(execution)).Format("2006-01-02"), nil
		}
		return automationEmptyDisplay, nil
	case "created_at", "updated_at":
		if timestamp := cast.ToInt64(value); timestamp > 0 {
			return time.Unix(timestamp, 0).In(automationTimeLocation(execution)).Format("2006-01-02 15:04:05"), nil
		}
		return automationEmptyDisplay, nil
	default:
		fields, err := msql.RawValues(define.Postgres, `SELECT field_type,config_json FROM workspace_custom_fields WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND key=$4`, nil, execution.AdminID, snapshot.WorkspaceID, snapshot.Type, key)
		if err != nil {
			return "", err
		}
		if len(fields) == 0 {
			return automationEmptyDisplay, nil
		}
		config := CustomFieldConfig{}
		if json.Unmarshal([]byte(fields[0]["config_json"]), &config) != nil || len(config.Options) == 0 {
			return "", nil
		}
		names := map[string]string{}
		for _, option := range config.Options {
			names[option.Key] = option.Name
		}
		if fields[0]["field_type"] == "select_multi" {
			selected, ok := automationStringSet(value)
			if !ok {
				return "", nil
			}
			result := []string{}
			for key := range selected {
				if names[key] != "" {
					result = append(result, names[key])
				}
			}
			sort.Strings(result)
			if len(result) == 0 {
				return automationEmptyDisplay, nil
			}
			return strings.Join(result, ","), nil
		}
		if name := names[fmt.Sprint(value)]; name != "" {
			return name, nil
		}
		return automationEmptyDisplay, nil
	}
	if cast.ToInt(value) == 0 {
		return automationEmptyDisplay, nil
	}
	rows, err := msql.RawValues(define.Postgres, query, nil, args...)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return automationEmptyDisplay, nil
	}
	return lib_i18n.ShowStored(execution.Locale, rows[0]["name"]), nil
}

// checkpointAutomationNode persists a webhook node checkpoint on its own.
func checkpointAutomationNode(execution *automationExecution, nodeIndex int, succeeded bool, key string, parameters map[string]any) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = checkpointAutomationNodeTx(tx, execution, nodeIndex, succeeded, key, parameters); err != nil {
		return err
	}
	return tx.Commit()
}

// checkpointAutomationNodeTx persists a node checkpoint within the local side-effect transaction.
func checkpointAutomationNodeTx(tx *sql.Tx, execution *automationExecution, nodeIndex int, succeeded bool, key string, parameters map[string]any) error {
	if !automationProgressHas(execution.Progress, nodeIndex) {
		execution.Progress.Completed = append(execution.Progress.Completed, nodeIndex)
	}
	if succeeded {
		execution.Progress.AnySuccess = true
	}
	execution.Logs = append(execution.Logs, AutomationLog{Time: time.Now().Unix(), Key: key, Node: nodeIndex + 1, Parameters: parameters})
	progressJSON, err := json.Marshal(execution.Progress)
	if err != nil {
		return err
	}
	logsJSON, err := json.Marshal(execution.Logs)
	if err != nil {
		return err
	}
	_, err = msql.RawExec(define.Postgres, `UPDATE automation_executions SET node_progress=$1,structured_logs=$2,
		lease_until=$3,updated_at=$4 WHERE admin_id=$5 AND workspace_id=$6 AND id=$7`, tx,
		progressJSON, logsJSON, time.Now().Unix()+automationLeaseSeconds, time.Now().Unix(), execution.AdminID, execution.WorkspaceID, execution.ID)
	return err
}

// automationProgressHas reports whether a node is already completed to avoid re-running it after lease recovery.
func automationProgressHas(progress AutomationNodeProgress, index int) bool {
	for _, completed := range progress.Completed {
		if completed == index {
			return true
		}
	}
	return false
}

// failAutomationExecution persists the final failure status and structured logs.
func failAutomationExecution(execution *automationExecution, cause error) error {
	logs.Error("automation execution %d node failed: %s", execution.ID, cause.Error())
	execution.Logs = append(execution.Logs, AutomationLog{Time: time.Now().Unix(), Key: "automation_log_execution_failed", Raw: visibleAutomationErrorText(cause)})
	logsJSON, err := json.Marshal(execution.Logs)
	if err != nil {
		return err
	}
	_, err = msql.RawExec(define.Postgres, `UPDATE automation_executions SET status='failed',failure_reason=$1,
		finished_at=$2,lease_owner='',lease_until=0,structured_logs=$3,updated_at=$2
		WHERE admin_id=$4 AND workspace_id=$5 AND id=$6`, nil, "automation_execution_failed", time.Now().Unix(), logsJSON,
		execution.AdminID, execution.WorkspaceID, execution.ID)
	return err
}

// visibleAutomationErrorText returns only the raw third-party error that is allowed to be shown.
func visibleAutomationErrorText(err error) string {
	var visible visibleAutomationError
	if errors.As(err, &visible) {
		return visible.Error()
	}
	return ""
}

// isTemporaryAutomationError identifies network and PostgreSQL temporary errors.
func isTemporaryAutomationError(err error) bool {
	var temporary temporaryAutomationError
	if errors.As(err, &temporary) {
		return true
	}
	var networkError net.Error
	if errors.As(err, &networkError) && (networkError.Timeout() || networkError.Temporary()) {
		return true
	}
	var pqError *pq.Error
	if errors.As(err, &pqError) {
		return strings.HasPrefix(string(pqError.Code), "08") || pqError.Code == "40001" || pqError.Code == "40P01" || pqError.Code == "55P03"
	}
	return false
}

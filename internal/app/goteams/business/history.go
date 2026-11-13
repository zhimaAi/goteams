// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_storage"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

func createHistoryTx(tx *sql.Tx, adminID, workspaceID, requirementID, userID int, action, field, oldValue, newValue string) error {
	snapshot, err := captureProcessingSnapshotTx(tx, adminID, workspaceID, requirementID, action, field)
	if err != nil {
		return err
	}
	oldValue = lib_storage.StripOSSURLSignatures(oldValue)
	newValue = lib_storage.StripOSSURLSignatures(newValue)
	query := `INSERT INTO ` + define.TableRequirementHistories + `
		(admin_id, workspace_id, requirement_id, user_id, action, field_name, old_value, new_value, created_at, processing_snapshot)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb)`
	_, err = msql.RawExec(define.Postgres, query, tx,
		adminID, workspaceID, requirementID, userID, action, field, oldValue, newValue, tool.Time2Int(), snapshot)
	if err != nil {
		logs.Error("create history sql:%s, err:%s", query, err.Error())
	}
	return err
}

// CreateHistory creates a requirement change-history record.
func CreateHistory(adminID, workspaceID, requirementID, userID int, action, field, oldValue, newValue string) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = createHistoryTx(tx, adminID, workspaceID, requirementID, userID, action, field, oldValue, newValue); err != nil {
		return err
	}
	return tx.Commit()
}

// processingSnapshotRecord 只用于快照存储，不作为 HTTP 响应或跨层模型。
type processingSnapshotRecord struct {
	StatusId      int      `json:"status_id"`
	StatusName    string   `json:"status_name"`
	StatusColor   string   `json:"status_color"`
	AssigneeIds   []int    `json:"assignee_ids"`
	AssigneeNames []string `json:"assignee_names"`
}

// ProcessingSnapshotModel 保存事件发生时的状态与处理人。
type ProcessingSnapshotModel struct {
	StatusId      int
	StatusName    string
	StatusColor   string
	AssigneeIds   []int
	AssigneeNames []string
}

func isProcessingHistory(action, field string) bool {
	return action == define.HistoryActionCreated || action == define.HistoryActionStatusChanged ||
		(action == define.HistoryActionIssueRecall && (field == "status" || field == "status_id")) ||
		((action == define.HistoryActionAssignChanged || action == define.HistoryActionFieldUpdated) && field == "assignee")
}

func captureProcessingSnapshotTx(tx *sql.Tx, adminID, workspaceID, requirementID int, action, field string) (*string, error) {
	if !isProcessingHistory(action, field) {
		return nil, nil
	}
	// 调用方已完成本次写入；锁定同一行，避免快照混入其他并发事务的结果。
	query := `SELECT jsonb_build_object('status_id', r.status_id,
 'status_name', COALESCE(s.name, ''), 'status_color', COALESCE(s.color, ''),
 'assignee_ids', COALESCE(to_jsonb(r.assignee_ids), '[]'::jsonb),
 'assignee_names', COALESCE((SELECT jsonb_agg(COALESCE(NULLIF(u.name,''), NULLIF(ru.name,''), '#' || a.id::text) ORDER BY a.ord)
 FROM unnest(r.assignee_ids) WITH ORDINALITY a(id,ord)
 LEFT JOIN ` + define.TableUsers + ` u ON u.admin_id=r.admin_id AND u.id=a.id
 LEFT JOIN ` + define.TableRemovedUsers + ` ru ON ru.admin_id=r.admin_id AND ru.user_id=a.id), '[]'::jsonb))::text AS snapshot
 FROM ` + define.TableRequirements + ` r
 LEFT JOIN ` + define.TableWorkItemStatuses + ` s ON s.admin_id=r.admin_id AND s.workspace_id=r.workspace_id AND s.type_key=r.type AND s.id=r.status_id
 WHERE r.admin_id=$1 AND r.workspace_id=$2 AND r.id=$3 FOR UPDATE OF r`
	rows, err := msql.RawValues(define.Postgres, query, tx, adminID, workspaceID, requirementID)
	if err != nil {
		logs.Error("processing snapshot sql:%s, err:%s", query, err.Error())
		return nil, err
	}
	if len(rows) != 1 {
		return nil, ErrRequirementNotFound
	}
	snapshot := rows[0]["snapshot"]
	return &snapshot, nil
}

// linkProcessingCommentTx 只关联已锁定工作项在本次真实状态变更中生成的记录。
func linkProcessingCommentTx(tx *sql.Tx, model TransitionWorkItemStatusModel, commentID int) error {
	query := `UPDATE ` + define.TableRequirementHistories + ` SET processing_snapshot =
 jsonb_set(processing_snapshot, '{comment_id}', to_jsonb($5::int))
 WHERE admin_id=$1 AND workspace_id=$2 AND requirement_id=$3 AND user_id=$4
 AND id=(SELECT id FROM ` + define.TableRequirementHistories + `
 WHERE admin_id=$1 AND workspace_id=$2 AND requirement_id=$3 AND user_id=$4
 AND action=$6 AND field_name='status' ORDER BY id DESC LIMIT 1)
 AND processing_snapshot IS NOT NULL`
	result, err := msql.RawExec(define.Postgres, query, tx, model.AdminId, cast.ToInt(model.WorkspaceId), model.WorkItemId, model.UserId, commentID, define.HistoryActionStatusChanged)
	if err != nil {
		logs.Error("link processing comment sql:%s, err:%s", query, err.Error())
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("processing history missing for transition comment")
	}
	return nil
}

// historySelectClause is shared by the list and the single-record query so both render
// the same operator name for one record.
const historySelectClause = `SELECT h.id,h.admin_id,h.workspace_id,h.requirement_id,h.user_id,h.action,h.field_name,h.old_value,h.new_value,h.created_at,h.source,h.automation_rule_id,h.automation_execution_id,h.actor_name_snapshot,h.processing_snapshot,COALESCE(c.content, '') AS processing_note,
	CASE WHEN h.source='automation' THEN COALESCE(r.name || ' · ' || COALESCE(u.name, e.creator_name_snapshot), h.actor_name_snapshot)
	ELSE COALESCE(u.name, ru.name, h.actor_name_snapshot, '') END AS user_name
	FROM requirement_histories h
	LEFT JOIN requirement_comments c ON c.id=(h.processing_snapshot->>'comment_id')::int
	 AND c.admin_id=h.admin_id AND c.workspace_id=h.workspace_id AND c.requirement_id=h.requirement_id
	LEFT JOIN users u ON u.id = h.user_id AND u.admin_id=h.admin_id
	LEFT JOIN removed_users ru ON ru.user_id = h.user_id AND ru.admin_id = h.admin_id
	LEFT JOIN automation_rules r ON r.id=h.automation_rule_id AND r.admin_id=h.admin_id AND r.workspace_id=h.workspace_id
	LEFT JOIN automation_executions e ON e.id=h.automation_execution_id AND e.admin_id=h.admin_id AND e.workspace_id=h.workspace_id`

func listRequirementHistoryRows(adminID, workspaceID, requirementID int) ([]msql.Params, error) {
	query := historySelectClause + `
 WHERE h.admin_id = $1 AND h.workspace_id = $2 AND h.requirement_id = $3 ORDER BY h.id DESC`
	list, err := msql.RawValues(define.Postgres, query, nil, adminID, workspaceID, requirementID)
	if err != nil {
		logs.Error("list history sql:%s, err:%s", query, err.Error())
		return []msql.Params{}, err
	}
	return list, nil
}

// ListRequirementHistoriesModel 限定历史列表的租户与工作空间。
type ListRequirementHistoriesModel struct {
	AdminId       int
	WorkspaceId   int
	RequirementId int
}

// RequirementHistoryListItemModel 保留变更历史数据，并增加可选的处理人投影。
type RequirementHistoryListItemModel struct {
	RequirementHistoryModel
	AdminId               int
	WorkspaceId           int
	RequirementId         int
	UserId                int
	Source                string
	AutomationRuleId      int
	AutomationExecutionId int64
	ActorNameSnapshot     string
	AssigneeNames         []string
}

// ListHistoriesByRequirement 返回历史列表，旧分配记录仅解析其自身的新值。
func ListHistoriesByRequirement(model ListRequirementHistoriesModel) ([]RequirementHistoryListItemModel, error) {
	rows, err := listRequirementHistoryRows(model.AdminId, model.WorkspaceId, model.RequirementId)
	if err != nil {
		return nil, err
	}
	items := make([]RequirementHistoryListItemModel, 0, len(rows))
	names := map[int]string{}
	for _, row := range rows {
		history, err := rowToRequirementHistoryModel(row)
		if err != nil {
			return nil, err
		}
		item := RequirementHistoryListItemModel{RequirementHistoryModel: history, Source: row["source"], ActorNameSnapshot: row["actor_name_snapshot"]}
		for _, pair := range []struct {
			key    string
			target *int
		}{{"admin_id", &item.AdminId}, {"workspace_id", &item.WorkspaceId}, {"requirement_id", &item.RequirementId}, {"user_id", &item.UserId}, {"automation_rule_id", &item.AutomationRuleId}} {
			value, err := strconv.Atoi(row[pair.key])
			if err != nil {
				return nil, err
			}
			*pair.target = value
		}
		item.AutomationExecutionId, err = strconv.ParseInt(row["automation_execution_id"], 10, 64)
		if err != nil {
			return nil, err
		}
		if history.ProcessingSnapshot == nil && isAssigneeHistory(history.Action, history.FieldName) {
			for _, id := range parseStoredAssigneeIDs(history.NewValue) {
				names[id] = ""
			}
		}
		items = append(items, item)
	}
	if len(names) > 0 {
		ids := make([]int, 0, len(names))
		for id := range names {
			ids = append(ids, id)
		}
		query := `SELECT a.id,COALESCE(NULLIF(u.name,''),NULLIF(ru.name,''),'#'||a.id::text) AS name
 FROM unnest($2::int[]) a(id)
 LEFT JOIN ` + define.TableUsers + ` u ON u.admin_id=$1 AND u.id=a.id
 LEFT JOIN ` + define.TableRemovedUsers + ` ru ON ru.admin_id=$1 AND ru.user_id=a.id`
		users, err := msql.RawValues(define.Postgres, query, nil, model.AdminId, pq.Array(ids))
		if err != nil {
			logs.Error("history assignee names sql:%s, err:%s", query, err.Error())
			return nil, err
		}
		for _, user := range users {
			id, err := strconv.Atoi(user["id"])
			if err != nil {
				return nil, err
			}
			names[id] = user["name"]
		}
	}
	for i := range items {
		item := &items[i]
		if item.ProcessingSnapshot != nil {
			item.AssigneeNames = item.ProcessingSnapshot.AssigneeNames
			continue
		}
		if !isAssigneeHistory(item.Action, item.FieldName) {
			continue
		}
		item.AssigneeNames = []string{}
		for _, id := range parseStoredAssigneeIDs(item.NewValue) {
			name := names[id]
			if strings.TrimSpace(name) == "" {
				name = fmt.Sprintf("#%d", id)
			}
			item.AssigneeNames = append(item.AssigneeNames, name)
		}
	}
	return items, nil
}

func isAssigneeHistory(action, field string) bool {
	return field == "assignee" && (action == define.HistoryActionAssignChanged || action == define.HistoryActionFieldUpdated)
}

// RequirementHistoryModel is one change record of a work item.
type RequirementHistoryModel struct {
	ProcessingNote     string
	ProcessingSnapshot *ProcessingSnapshotModel
	Id                 int
	Action             string
	FieldName          string
	OldValue           string
	NewValue           string
	CreatedAt          int64
	UserName           string
}

// rowToRequirementHistoryModel converts one history row. A malformed id or timestamp fails the
// request instead of silently returning a zero value.
func rowToRequirementHistoryModel(row msql.Params) (RequirementHistoryModel, error) {
	id, err := strconv.Atoi(row[`id`])
	if err != nil {
		return RequirementHistoryModel{}, err
	}
	createdAt, err := strconv.ParseInt(row[`created_at`], 10, 64)
	if err != nil {
		return RequirementHistoryModel{}, err
	}
	var snapshot *ProcessingSnapshotModel
	if raw := row["processing_snapshot"]; raw != "" && raw != "null" {
		var record processingSnapshotRecord
		if err := json.Unmarshal([]byte(raw), &record); err != nil {
			return RequirementHistoryModel{}, err
		}
		if record.AssigneeIds == nil || record.AssigneeNames == nil || len(record.AssigneeIds) != len(record.AssigneeNames) {
			return RequirementHistoryModel{}, errors.New("invalid processing snapshot assignees")
		}
		snapshot = &ProcessingSnapshotModel{StatusId: record.StatusId, StatusName: record.StatusName, StatusColor: record.StatusColor, AssigneeIds: record.AssigneeIds, AssigneeNames: record.AssigneeNames}
	}
	return RequirementHistoryModel{
		ProcessingSnapshot: snapshot,
		ProcessingNote:     row["processing_note"],
		Id:                 id,
		Action:             row[`action`],
		FieldName:          row[`field_name`],
		OldValue:           row[`old_value`],
		NewValue:           row[`new_value`],
		CreatedAt:          createdAt,
		UserName:           row[`user_name`],
	}, nil
}

// GetHistoryById returns one change record of a work item. The lookup always carries admin_id,
// workspace_id and requirement_id, so a record outside the scope is never returned. The boolean
// reports whether the record exists.
func GetHistoryById(adminID, workspaceID, requirementID, historyID int) (RequirementHistoryModel, bool, error) {
	rows, err := msql.RawValues(define.Postgres, historySelectClause+`
		WHERE h.admin_id = $1 AND h.workspace_id = $2 AND h.requirement_id = $3 AND h.id = $4`, nil,
		adminID, workspaceID, requirementID, historyID)
	if err != nil {
		return RequirementHistoryModel{}, false, err
	}
	if len(rows) == 0 {
		return RequirementHistoryModel{}, false, nil
	}
	model, err := rowToRequirementHistoryModel(rows[0])
	if err != nil {
		return RequirementHistoryModel{}, false, err
	}
	return model, true, nil
}

// CountHistoriesByRequirement returns the total count of requirement change history.
func CountHistoriesByRequirement(adminID, workspaceID, requirementID int) (int, error) {
	m := msql.Model(define.TableRequirementHistories, define.Postgres)
	count, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminID)},
		{`workspace_id`, cast.ToString(workspaceID)},
		{`requirement_id`, cast.ToString(requirementID)},
	}).Count()
	if err != nil {
		logs.Error("CountHistoriesByRequirement sql:%s, err:%s", m.GetLastSql(), err.Error())
	}
	return count, err
}

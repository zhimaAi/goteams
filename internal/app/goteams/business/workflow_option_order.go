package business

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"

	"github.com/lib/pq"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

const (
	// WorkflowStatusOrder 表示状态选项排序。
	WorkflowStatusOrder = "status"
	// WorkflowPriorityOrder 表示优先级选项排序。
	WorkflowPriorityOrder = "priority"
	// WorkflowSeverityOrder 表示缺陷严重等级选项排序。
	WorkflowSeverityOrder = "severity"
)

var (
	// ErrWorkflowOptionOrderInvalid 表示排序请求格式或作用域无效。
	ErrWorkflowOptionOrderInvalid = errors.New("invalid workflow option order")
	// ErrWorkflowOptionOrderChanged 表示客户端使用的选项集合已失效。
	ErrWorkflowOptionOrderChanged = errors.New("workflow option set changed")
)

// ReorderWorkflowOptionsModel 保存一个配置组的完整选项顺序。
type ReorderWorkflowOptionsModel struct {
	AdminId, WorkspaceId int
	Kind, TypeKey        string
	Ids                  []int
}

// ReorderWorkflowOptions 原子保存同一配置组的顺序。
func ReorderWorkflowOptions(model ReorderWorkflowOptionsModel) error {
	if model.AdminId <= 0 || model.WorkspaceId <= 0 || !validate.WorkflowOptionOrder(model.Ids) {
		return ErrWorkflowOptionOrderInvalid
	}
	var table string
	switch model.Kind {
	case WorkflowStatusOrder:
		table = define.TableWorkItemStatuses
	case WorkflowPriorityOrder:
		table = define.TableWorkItemPriorities
	case WorkflowSeverityOrder:
		table = define.TableWorkspaceSeverities
	default:
		return ErrWorkflowOptionOrderInvalid
	}
	if model.Kind != WorkflowSeverityOrder && len(model.Ids) > math.MaxInt16 {
		return ErrWorkflowOptionOrderInvalid
	}
	args := []any{model.AdminId, model.WorkspaceId}
	scope := " WHERE admin_id=$1 AND workspace_id=$2"
	if model.Kind == WorkflowSeverityOrder {
		if model.TypeKey != define.TypeKeyDefect {
			return ErrWorkflowOptionOrderInvalid
		}
	} else {
		switch model.TypeKey {
		case define.TypeKeyRequirement, define.TypeKeyDefect, define.TypeKeyPlanning:
		default:
			return ErrWorkflowOptionOrderInvalid
		}
		scope += " AND type_key=$3"
		args = append(args, model.TypeKey)
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	workspaceType, err := lockStatusActionWorkspace(tx, model.AdminId, model.WorkspaceId, true)
	if err != nil {
		return err
	}
	if workspaceType == define.WorkspaceTypeIssue && model.TypeKey != define.TypeKeyRequirement {
		return ErrWorkflowOptionOrderInvalid
	}

	rows, err := statusActionRows(tx, "SELECT id FROM "+table+scope+" ORDER BY id FOR UPDATE", args...)
	if err != nil {
		return err
	}
	if len(rows) != len(model.Ids) {
		return ErrWorkflowOptionOrderChanged
	}
	ids := make(map[int]struct{}, len(rows))
	for _, row := range rows {
		id, err := strconv.Atoi(row["id"])
		if err != nil {
			return fmt.Errorf("parse workflow option id: %w", err)
		}
		ids[id] = struct{}{}
	}
	for _, id := range model.Ids {
		if _, exists := ids[id]; !exists {
			return ErrWorkflowOptionOrderChanged
		}
	}

	set := " SET sort_order=position"
	args = append(args, pq.Array(model.Ids))
	ordered := " FROM unnest($" + strconv.Itoa(len(args)) + "::integer[]) WITH ORDINALITY AS ordered(option_id, position)"
	if model.Kind != WorkflowSeverityOrder {
		args = append(args, tool.Time2Int())
		set += ", updated_at=$" + strconv.Itoa(len(args))
	}
	if _, err := statusActionExec(tx, "UPDATE "+table+set+ordered+scope+" AND id=option_id", args...); err != nil {
		return err
	}
	return tx.Commit()
}

func nextWorkflowOptionSortOrder(tx *sql.Tx, table string, adminId, workspaceId int, typeKey string) (int, error) {
	query := "SELECT COALESCE(MAX(sort_order), 0)::bigint + 1 AS next_order FROM " + table +
		" WHERE admin_id=$1 AND workspace_id=$2"
	args := []any{adminId, workspaceId}
	if table != define.TableWorkspaceSeverities {
		query += " AND type_key=$3"
		args = append(args, typeKey)
	}
	rows, err := statusActionRows(tx, query, args...)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(rows[0]["next_order"])
}

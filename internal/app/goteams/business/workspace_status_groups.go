package business

import (
	"database/sql"
	"errors"
	"strconv"

	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"goteams/internal/app/goteams/define"
)

// ErrStatusGroupsInvalid 表示状态配置包含越界 ID 或完成、关闭属性冲突。
var ErrStatusGroupsInvalid = errors.New("invalid workspace status groups")

type workspaceStatusPatch struct {
	typeKey                  string
	initial                  *int
	final, completed, closed *[]int
}

func statusDifference(ids, excluded []int) []int {
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		if !intSliceContains(excluded, id) {
			result = append(result, id)
		}
	}
	return result
}

func statusIntersection(ids, included []int) []int {
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		if intSliceContains(included, id) {
			result = append(result, id)
		}
	}
	return result
}

// final_status_ids 继续保存终态并集，避免待办、SLA 和工单操作各自维护另一种终态口径。
func applyWorkspaceStatusPatch(tx *sql.Tx, adminId, workspaceId int, old msql.Params, patch workspaceStatusPatch, data msql.Datas) error {
	if patch.initial == nil && patch.final == nil && patch.completed == nil && patch.closed == nil {
		return nil
	}
	finalField, completedField := patch.typeKey+"_final_status_ids", patch.typeKey+"_completed_status_ids"
	completed := toIntSlice(old[completedField])
	closed := statusDifference(toIntSlice(old[finalField]), completed)
	if patch.final != nil {
		if patch.closed != nil || patch.completed != nil {
			return ErrStatusGroupsInvalid
		}
		closed = statusDifference(*patch.final, completed)
		completed = statusIntersection(completed, *patch.final)
	}
	if patch.completed != nil {
		completed = *patch.completed
	}
	if patch.closed != nil {
		closed = *patch.closed
	}
	final, err := terminalStatusIDs(completed, closed)
	if err != nil {
		return err
	}
	query := `SELECT id FROM ` + define.TableWorkItemStatuses + ` WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 FOR SHARE`
	rows, err := msql.RawValues(define.Postgres, query, tx, adminId, workspaceId, patch.typeKey)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return err
	}
	available := make(map[int]bool, len(rows))
	for _, row := range rows {
		id, err := strconv.Atoi(row["id"])
		if err != nil {
			return err
		}
		available[id] = true
	}
	for _, id := range final {
		if !available[id] {
			return ErrStatusGroupsInvalid
		}
	}
	if patch.initial != nil && *patch.initial != 0 && !available[*patch.initial] {
		return ErrStatusGroupsInvalid
	}
	if patch.final != nil || patch.completed != nil || patch.closed != nil {
		data[finalField], data[completedField] = joinIntSlice(final), joinIntSlice(completed)
	}
	return nil
}

func terminalStatusIDs(completed, closed []int) ([]int, error) {
	ids := make([]int, 0, len(completed)+len(closed))
	seen := map[int]bool{}
	for _, group := range [][]int{completed, closed} {
		for _, id := range group {
			if id <= 0 || seen[id] {
				return nil, ErrStatusGroupsInvalid
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

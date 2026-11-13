// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"slices"
	"strings"

	"goteams/internal/app/goteams/define"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// unfinishedAssignedItemsSQL selects the unfinished work items (requirement / defect / planning)
// a user is assigned to. "Unfinished" means the status is not listed in the workspace's final
// status ids for that item type; a workspace without a settings row has no final statuses, so all
// of its items count as unfinished. Recycle-bin items (deleted_at > 0) are excluded.
// $1 = admin id, $2 = workspace id (0 = every workspace of the tenant), $3 = assignee user id.
const unfinishedAssignedItemsSQL = `SELECT r.id, r.workspace_id, r.assignee_ids
	FROM ` + define.TableRequirements + ` r
	LEFT JOIN ` + define.TableWorkspaceSettings + ` s
		ON s.admin_id = r.admin_id AND s.workspace_id = r.workspace_id
	WHERE r.admin_id = $1
		AND ($2 = 0 OR r.workspace_id = $2)
		AND r.deleted_at = 0
		AND r.assignee_ids @> ARRAY[$3]::INTEGER[]
		AND NOT COALESCE(r.status_id = ANY(string_to_array(NULLIF(
			CASE r.type
				WHEN '` + define.TypeKeyDefect + `' THEN s.defect_final_status_ids
				WHEN '` + define.TypeKeyPlanning + `' THEN s.planning_final_status_ids
				ELSE s.requirement_final_status_ids
			END, ''), ',')::INTEGER[]), FALSE)
	ORDER BY r.id`

// CountUnfinishedAssignedItems returns how many unfinished work items the user is assigned to.
// wsId 0 counts across every workspace of the tenant. Used by the removal preview so the dialog
// only offers a hand-over when there is something to hand over.
// ponytail: counts by scanning the id list; a member's unfinished items are at most a few hundred.
func CountUnfinishedAssignedItems(adminId, wsId, userId int) (int, error) {
	if adminId <= 0 || userId <= 0 {
		return 0, nil
	}
	rows, err := msql.RawValues(define.Postgres, unfinishedAssignedItemsSQL, nil, adminId, wsId, userId)
	if err != nil {
		logs.Error("count unfinished assigned items error: %s", err.Error())
		return 0, err
	}
	return len(rows), nil
}

// transferUnfinishedAssignedItems hands the user's unfinished work items over to another member,
// replacing only that one user inside assignee_ids and leaving co-assignees untouched.
// wsId 0 covers every workspace of the tenant. Returns the workspaces whose items were moved, so
// callers can make sure the receiver has access to them.
func transferUnfinishedAssignedItems(tx *sql.Tx, adminId, wsId, fromUserId, toUserId, operatorUserId int) ([]int, error) {
	if adminId <= 0 || fromUserId <= 0 || toUserId <= 0 || fromUserId == toUserId {
		return nil, nil
	}
	rows, err := msql.RawValues(define.Postgres, unfinishedAssignedItemsSQL, tx, adminId, wsId, fromUserId)
	if err != nil {
		logs.Error("list unfinished assigned items error: %s", err.Error())
		return nil, err
	}

	now := tool.Time2Int()
	workspaceIds := make([]int, 0, len(rows))
	for _, row := range rows {
		oldIds := parseStoredAssigneeIDs(row[`assignee_ids`])
		newIds := replaceAssignee(oldIds, fromUserId, toUserId)
		itemId := cast.ToInt(row[`id`])
		workspaceId := cast.ToInt(row[`workspace_id`])
		if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableRequirements+`
			SET assignee_ids = $1, updated_at = $2 WHERE id = $3 AND admin_id = $4`,
			tx, pq.Array(newIds), now, itemId, adminId); err != nil {
			logs.Error("transfer work item assignee error: item=%d, err=%s", itemId, err.Error())
			return nil, err
		}
		if err = createHistoryTx(tx, adminId, workspaceId, itemId, operatorUserId,
			define.HistoryActionAssignChanged, `assignee`,
			assigneeIDsText(oldIds), assigneeIDsText(newIds)); err != nil {
			return nil, err
		}
		if !slices.Contains(workspaceIds, workspaceId) {
			workspaceIds = append(workspaceIds, workspaceId)
		}
	}
	return workspaceIds, nil
}

// replaceAssignee swaps one assignee for another while preserving order and the other assignees.
// When the target is already assigned, the source is dropped instead of being duplicated.
func replaceAssignee(ids []int, fromUserId, toUserId int) []int {
	targetAlreadyAssigned := slices.Contains(ids, toUserId)
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		switch {
		case id != fromUserId:
			result = append(result, id)
		case targetAlreadyAssigned:
			// Target is already an assignee: drop the source rather than adding a duplicate.
		default:
			result = append(result, toUserId)
		}
	}
	return result
}

// parseStoredAssigneeIDs converts the stored postgres array text ("{1,2}") into ids.
func parseStoredAssigneeIDs(value string) []int {
	text := normalizeStoredAssigneeIDs(value)
	if text == `` {
		return nil
	}
	parts := strings.Split(text, `,`)
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		if id := cast.ToInt(part); id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

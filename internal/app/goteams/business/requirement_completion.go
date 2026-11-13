package business

import (
	"fmt"
	"goteams/internal/app/goteams/define"
	"strconv"
)

func workItemCompletedIDs(adminId, workspaceId int) ([]int, error) {
	s, err := GetWorkspaceSettings(WorkspaceSettingsModel{AdminId: adminId, WsId: strconv.Itoa(workspaceId)})
	if err != nil {
		return nil, err
	}
	return append(append([]int{}, s.RequirementCompletedStatusIds...), s.DefectCompletedStatusIds...), nil
}

// completionAssignment 仅在状态实际改变并进入完成状态时记录最新完成时间。
// 离开完成状态保留历史时间，统计仍以当前状态是否完成为准。
func completionAssignment(statusArg, timeArg, completedArg int) string {
	return fmt.Sprintf(`completed_at = CASE WHEN status_id IS DISTINCT FROM $%d::INTEGER AND $%d::INTEGER = ANY($%d::INTEGER[]) THEN $%d::BIGINT ELSE completed_at END`, statusArg, statusArg, completedArg, timeArg)
}

func initialCompletionTime(adminId, workspaceId int, typ string, statusId int, now int64) (int64, error) {
	if typ != define.TypeKeyRequirement && typ != define.TypeKeyDefect {
		return 0, nil
	}
	ids, err := workItemCompletedIDs(adminId, workspaceId)
	if err != nil {
		return 0, err
	}
	if intSliceContains(ids, statusId) {
		return now, nil
	}
	return 0, nil
}

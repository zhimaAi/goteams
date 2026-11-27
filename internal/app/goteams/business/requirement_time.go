package business

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/define"

	"github.com/zhimaAi/go_tools/msql"
)

// workItemTimeAssignments 在状态更新的同一 SQL 中比较数据库旧值，防止并发覆盖首次时间。
// 数据库时钟在写入时取值，不让等待行锁的请求沿用等待前的时间。
func workItemTimeAssignments(statusArg int) string {
	assignments := ""
	for _, field := range []string{"started_at", "ended_at"} {
		statusField, firstOnly := "requirement_end_status_ids", ""
		if field == "started_at" {
			statusField, firstOnly = "requirement_start_status_ids", " AND started_at=0"
		}
		if assignments != "" {
			assignments += ", "
		}
		assignments += fmt.Sprintf(`%s = CASE WHEN type='requirement' AND status_id IS DISTINCT FROM $%d::INTEGER%s
            AND EXISTS (SELECT 1 FROM %s s
                JOIN %s f ON f.admin_id=s.admin_id AND f.workspace_id=s.workspace_id
                    AND f.type_key='requirement' AND f.field_key='%s' AND f.is_enabled=1
                JOIN %s st ON st.admin_id=s.admin_id AND st.workspace_id=s.workspace_id
                    AND st.type_key='requirement' AND st.id=$%d::INTEGER AND st.id=ANY(string_to_array(NULLIF(s.%s,''), ',')::INTEGER[])
                WHERE s.admin_id=%s.admin_id AND s.workspace_id=%s.workspace_id
                    AND s.requirement_time_enabled=TRUE )
            THEN FLOOR(EXTRACT(EPOCH FROM clock_timestamp()))::BIGINT ELSE %s END`,
			field, statusArg, firstOnly, define.TableWorkspaceSettings, define.TableWorkItemFieldConfigs,
			field, define.TableWorkItemStatuses, statusArg, statusField, define.TableRequirements, define.TableRequirements, field)
	}
	return assignments
}

// ErrWorkItemTimeSettingsInvalid 表示自动时间配置引用无效或缺少目标状态。
var ErrWorkItemTimeSettingsInvalid = errors.New("invalid work item time settings")

// ErrWorkItemTimeStatusReferenced 表示状态仍被自动时间配置引用。
var ErrWorkItemTimeStatusReferenced = errors.New("status referenced by work item time settings")

func parseWorkItemTimeStatusIDs(value string) ([]int, error) {
	ids := []int{}
	if value == "" {
		return ids, nil
	}
	for _, raw := range strings.Split(value, ",") {
		id, err := strconv.Atoi(raw)
		if err != nil {
			return nil, err
		}
		if id <= 0 || intSliceContains(ids, id) {
			return nil, ErrWorkItemTimeSettingsInvalid
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func validateWorkItemTimeSettingsTx(tx *sql.Tx, adminId, workspaceId int, old msql.Params, patch UpdateWorkspaceSettingsModel) error {
	if patch.RequirementTimeEnabled == nil && patch.RequirementStartStatusIds == nil && patch.RequirementEndStatusIds == nil {
		return nil
	}
	enabled, startIds, endIds := false, []int{}, []int{}
	var err error
	if len(old) > 0 {
		enabled, err = strconv.ParseBool(old["requirement_time_enabled"])
		if err != nil {
			return err
		}
		startIds, err = parseWorkItemTimeStatusIDs(old["requirement_start_status_ids"])
		if err != nil {
			return err
		}
		endIds, err = parseWorkItemTimeStatusIDs(old["requirement_end_status_ids"])
		if err != nil {
			return err
		}
	}
	if patch.RequirementTimeEnabled != nil {
		enabled = *patch.RequirementTimeEnabled
	}
	if patch.RequirementStartStatusIds != nil {
		startIds = *patch.RequirementStartStatusIds
	}
	if patch.RequirementEndStatusIds != nil {
		endIds = *patch.RequirementEndStatusIds
	}
	for _, ids := range [][]int{startIds, endIds} {
		seen := map[int]bool{}
		for _, id := range ids {
			if id <= 0 || seen[id] {
				return ErrWorkItemTimeSettingsInvalid
			}
			seen[id] = true
			rows, err := statusActionRows(tx, "SELECT id FROM "+define.TableWorkItemStatuses+" WHERE admin_id=$1 AND workspace_id=$2 AND type_key='requirement' AND id=$3 FOR SHARE", adminId, workspaceId, id)
			if err != nil {
				return err
			}
			if len(rows) != 1 {
				return ErrWorkItemTimeSettingsInvalid
			}
		}
	}
	if !enabled {
		return nil
	}
	fields, err := statusActionRows(tx, "SELECT field_key FROM "+define.TableWorkItemFieldConfigs+" WHERE admin_id=$1 AND workspace_id=$2 AND type_key='requirement' AND field_key IN ('started_at','ended_at') AND is_enabled=1", adminId, workspaceId)
	if err != nil {
		return err
	}
	if len(fields) == 0 {
		return ErrWorkItemTimeSettingsInvalid
	}
	for _, field := range fields {
		if (field["field_key"] == "started_at" && len(startIds) == 0) || (field["field_key"] == "ended_at" && len(endIds) == 0) {
			return ErrWorkItemTimeSettingsInvalid
		}
	}
	return nil
}

// 跨工作空间列表按每行所属空间决定字段可见性，避免关闭字段后仍展示保留的历史值。
const workItemTimeVisibilitySQL = `
    (r.type='requirement' AND EXISTS (SELECT 1 FROM ` + define.TableWorkItemFieldConfigs + ` f WHERE f.admin_id=r.admin_id AND f.workspace_id=r.workspace_id AND f.type_key='requirement' AND f.field_key='started_at' AND f.is_enabled=1)) AS started_at_enabled,
    (r.type='requirement' AND EXISTS (SELECT 1 FROM ` + define.TableWorkItemFieldConfigs + ` f WHERE f.admin_id=r.admin_id AND f.workspace_id=r.workspace_id AND f.type_key='requirement' AND f.field_key='ended_at' AND f.is_enabled=1)) AS ended_at_enabled`

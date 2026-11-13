// Copyright © 2026 Wuhan Sesame Small Customer Service Network Co., Ltd.

package business

import (
	"fmt"
	"strings"

	"goteams/internal/app/goteams/define"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/msql"
)

// pipelineWorkItemLogFilter is the shared WHERE of the pipeline log.
// 流水线日志按需求聚合：对话轮次是该需求在筛选范围内被执行的次数，Token 是这些执行的合计。
type pipelineWorkItemLogFilter struct {
	adminID       int
	viewerUserID  int
	tenantOwner   bool
	keyword       string
	userIDs       []int
	startAt       int64
	endAt         int64
	periodStart   int64
	periodEnd     int64
	periodEnabled bool
}

func (f pipelineWorkItemLogFilter) where() (string, []interface{}) {
	args := []interface{}{f.adminID}
	clauses := []string{`t.admin_id=$1`}
	if !f.tenantOwner {
		args = append(args, f.viewerUserID)
		clauses = append(clauses, pipelineWorkItemVisibilityClause(len(args)))
	}
	if keyword := strings.TrimSpace(f.keyword); keyword != `` {
		args = append(args, `%`+keyword+`%`)
		clauses = append(clauses, fmt.Sprintf(`t.title_snapshot LIKE $%d`, len(args)))
	}
	if len(f.userIDs) > 0 {
		args = append(args, pq.Array(f.userIDs))
		clauses = append(clauses, fmt.Sprintf(`t.user_id = ANY($%d)`, len(args)))
	}
	if f.startAt > 0 {
		args = append(args, f.startAt, f.endAt)
		clauses = append(clauses,
			fmt.Sprintf(`t.started_at >= $%d`, len(args)-1),
			fmt.Sprintf(`t.started_at < $%d`, len(args)),
		)
	}
	if f.periodEnabled {
		args = append(args, f.periodStart, f.periodEnd)
		clauses = append(clauses,
			fmt.Sprintf(`t.last_uploaded_at >= $%d`, len(args)-1),
			fmt.Sprintf(`t.last_uploaded_at < $%d`, len(args)),
		)
	}
	return strings.Join(clauses, ` AND `), args
}

func pipelineWorkItemVisibilityClause(userArg int) string {
	return fmt.Sprintf(`EXISTS (
		SELECT 1
		FROM workspace_members wm
		JOIN roles wr ON wr.admin_id=wm.admin_id AND wr.workspace_id=wm.workspace_id AND wr.id=wm.role_id
		JOIN role_permissions rp ON rp.admin_id=wm.admin_id AND rp.role_id=wm.role_id
		JOIN requirements r ON r.admin_id=t.admin_id AND r.workspace_id=t.workspace_id
			AND r.id=t.work_item_id AND r.type=t.work_item_type AND r.deleted_at=0
		WHERE wm.admin_id=t.admin_id AND wm.workspace_id=t.workspace_id AND wm.user_id=$%d
			AND rp.permission_key=CASE t.work_item_type
				WHEN 'requirement' THEN 'requirement:view'
				WHEN 'defect' THEN 'defect:view'
				ELSE ''
			END
			AND (
				wr.data_view_scope='all'
				OR r.created_by=$%d
				OR r.assignee_ids @> ARRAY[$%d]::INTEGER[]
				OR (wr.data_view_scope='partial' AND EXISTS (
					SELECT 1 FROM role_data_view_members rdvm
					WHERE rdvm.admin_id=wr.admin_id AND rdvm.role_id=wr.id
						AND rdvm.user_id=ANY(r.assignee_ids)
				))
			)
	)`, userArg, userArg, userArg)
}

func countAggregatedPipelineWorkItems(filter pipelineWorkItemLogFilter) (int, error) {
	where, args := filter.where()
	rows, err := msql.RawValues(define.Postgres, fmt.Sprintf(`
		SELECT COUNT(*) AS total FROM (
			SELECT 1
			FROM %s t
			WHERE %s
			GROUP BY t.workspace_id, t.work_item_type, t.work_item_id
		) grouped`, define.TablePipelineTask, where), nil, args...)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return cast.ToInt(rows[0][`total`]), nil
}

func listAggregatedPipelineWorkItems(filter pipelineWorkItemLogFilter, limit, offset int) ([]msql.Params, error) {
	where, args := filter.where()
	args = append(args, limit, offset)
	limitArg := len(args) - 1
	offsetArg := len(args)
	query := fmt.Sprintf(`
		WITH filtered AS (
			SELECT t.id, t.user_id, u.name AS user_name, u.avatar AS user_avatar,
			       t.pipeline_id, t.pipeline_name_snapshot, t.pipeline_color_snapshot,
			       t.work_item_type, t.work_item_id, t.workspace_id, t.title_snapshot,
			       t.task_status, t.execution_status, t.current_step_key, t.execution_summary,
			       t.input_tokens, t.output_tokens, t.total_tokens,
			       t.started_at, t.finished_at, t.device_uuid, t.last_uploaded_at, t.created_at,
			       r.status_id AS requirement_status_id,
			       st.name AS requirement_status_name,
			       st.color AS requirement_status_color,
			       CASE WHEN r.type = 'defect'
			            THEN COALESCE(r.status_id = ANY(string_to_array(NULLIF(wss.defect_final_status_ids, ''), ',')::INTEGER[]), FALSE)
			            ELSE COALESCE(r.status_id = ANY(string_to_array(NULLIF(wss.requirement_final_status_ids, ''), ',')::INTEGER[]), FALSE)
			       END AS is_done
			FROM %s t
			LEFT JOIN %s u ON t.user_id=u.id AND t.admin_id=u.admin_id
			LEFT JOIN %s r ON t.work_item_id=r.id AND t.workspace_id=r.workspace_id AND t.work_item_type=r.type AND t.admin_id=r.admin_id
			LEFT JOIN %s st ON r.status_id=st.id AND r.workspace_id=st.workspace_id AND r.type=st.type_key AND r.admin_id=st.admin_id
			LEFT JOIN %s wss ON r.workspace_id=wss.workspace_id AND r.admin_id=wss.admin_id
			WHERE %s
		)
		SELECT id, user_id, user_name, user_avatar,
		       pipeline_id, pipeline_name_snapshot, pipeline_color_snapshot,
		       work_item_type, work_item_id, workspace_id, title_snapshot,
		       task_status, execution_status, current_step_key, execution_summary,
		       sum_input_tokens AS input_tokens,
		       sum_output_tokens AS output_tokens,
		       sum_total_tokens AS total_tokens,
		       execution_count AS conversation_rounds,
		       started_at, finished_at, device_uuid, last_uploaded_at, created_at,
		       requirement_status_id, requirement_status_name, requirement_status_color, is_done
		FROM (
			SELECT f.*,
			       SUM(f.input_tokens) OVER (PARTITION BY f.workspace_id, f.work_item_type, f.work_item_id) AS sum_input_tokens,
			       SUM(f.output_tokens) OVER (PARTITION BY f.workspace_id, f.work_item_type, f.work_item_id) AS sum_output_tokens,
			       SUM(f.total_tokens) OVER (PARTITION BY f.workspace_id, f.work_item_type, f.work_item_id) AS sum_total_tokens,
			       COUNT(*) OVER (PARTITION BY f.workspace_id, f.work_item_type, f.work_item_id) AS execution_count,
			       ROW_NUMBER() OVER (
			           PARTITION BY f.workspace_id, f.work_item_type, f.work_item_id
			           ORDER BY f.last_uploaded_at DESC, f.id DESC
			       ) AS rn
			FROM filtered f
		) ranked
		WHERE rn = 1
		ORDER BY last_uploaded_at DESC, id DESC
		LIMIT $%d OFFSET $%d`,
		define.TablePipelineTask, define.TableUsers, define.TableRequirements, define.TableWorkItemStatuses, define.TableWorkspaceSettings,
		where, limitArg, offsetArg)
	return msql.RawValues(define.Postgres, query, nil, args...)
}

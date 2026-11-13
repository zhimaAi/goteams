// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// DeletedPlanningModel is one row of the planning recycle bin.
type DeletedPlanningModel struct {
	Id            int
	Title         string
	GroupId       int
	GroupName     string
	DeletedAt     int64
	DeletedBy     int
	DeletedByName string
}

// ListDeletedPlanningsModel describes the recycle-bin list use case.
type ListDeletedPlanningsModel struct {
	AdminId     int
	WorkspaceId string
	Lang        string
	Items       []DeletedPlanningModel
}

// RecyclePlanningsModel describes the batch restore / purge use case.
type RecyclePlanningsModel struct {
	AdminId     int
	WorkspaceId string
	PlanningIds []int
	Affected    int
}

// ListDeletedPlannings returns the workspace's recycle-bin plannings, newest deletion first.
// The group name is read even when the group itself is deleted, so the original group is still shown.
func ListDeletedPlannings(model ListDeletedPlanningsModel) (ListDeletedPlanningsModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT r.id, r.title, r.group_id, r.deleted_at, r.deleted_by,
		COALESCE(g.name, '') AS group_name, COALESCE(u.name, ru.name, '') AS deleted_by_name
		FROM requirements r
		LEFT JOIN planning_groups g ON g.id = r.group_id AND g.admin_id = r.admin_id
		LEFT JOIN users u ON u.id = r.deleted_by AND u.admin_id = r.admin_id
		LEFT JOIN removed_users ru ON ru.user_id = r.deleted_by AND ru.admin_id = r.admin_id
		WHERE r.admin_id = $1 AND r.workspace_id = $2 AND r.type = $3 AND r.deleted_at > 0
		ORDER BY r.deleted_at DESC, r.id DESC`,
		nil, model.AdminId, cast.ToInt(model.WorkspaceId), define.TypeKeyPlanning)
	if err != nil {
		logs.Error("list deleted plannings failed, err:%s", err.Error())
		return ListDeletedPlanningsModel{}, err
	}
	items := make([]DeletedPlanningModel, 0, len(rows))
	for _, row := range rows {
		items = append(items, DeletedPlanningModel{
			Id:      cast.ToInt(row[`id`]),
			Title:   row[`title`],
			GroupId: cast.ToInt(row[`group_id`]),
			// Seeded groups store an i18n key, so translate it the same way the tab list does.
			GroupName:     lib_i18n.ShowStored(model.Lang, row[`group_name`]),
			DeletedAt:     cast.ToInt64(row[`deleted_at`]),
			DeletedBy:     cast.ToInt(row[`deleted_by`]),
			DeletedByName: row[`deleted_by_name`],
		})
	}
	model.Items = items
	return model, nil
}

// RestorePlannings restores plannings from the recycle bin.
// When the original group was deleted too, it is revived so the restored rows keep their group.
func RestorePlannings(model RecyclePlanningsModel) (RecyclePlanningsModel, error) {
	if len(model.PlanningIds) == 0 {
		return model, nil
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin restore plannings transaction failed, err:%s", err.Error())
		return RecyclePlanningsModel{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// Revive the groups the restored rows belong to; a group already alive is left untouched.
	if _, err = msql.RawExec(define.Postgres, `UPDATE planning_groups SET deleted_at = 0, updated_at = $1
		WHERE admin_id = $2 AND workspace_id = $3 AND deleted_at > 0 AND id IN (
			SELECT group_id FROM requirements
			WHERE admin_id = $2 AND workspace_id = $3 AND type = $4 AND deleted_at > 0 AND id = ANY($5::INTEGER[])
		)`, tx, tool.Time2Int(), model.AdminId, cast.ToInt(model.WorkspaceId),
		define.TypeKeyPlanning, pq.Array(model.PlanningIds)); err != nil {
		logs.Error("restore planning groups failed, err:%s", err.Error())
		return RecyclePlanningsModel{}, err
	}

	result, err := msql.RawExec(define.Postgres, `UPDATE requirements SET deleted_at = 0, deleted_by = 0, updated_at = $1
		WHERE admin_id = $2 AND workspace_id = $3 AND type = $4 AND deleted_at > 0 AND id = ANY($5::INTEGER[])`,
		tx, tool.Time2Int(), model.AdminId, cast.ToInt(model.WorkspaceId),
		define.TypeKeyPlanning, pq.Array(model.PlanningIds))
	if err != nil {
		logs.Error("restore plannings failed, err:%s", err.Error())
		return RecyclePlanningsModel{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		logs.Error("read restored plannings count failed, err:%s", err.Error())
		return RecyclePlanningsModel{}, err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit restore plannings transaction failed, err:%s", err.Error())
		return RecyclePlanningsModel{}, err
	}
	model.Affected = cast.ToInt(affected)
	return model, nil
}

// PurgePlannings permanently deletes recycle-bin plannings together with their relations and custom field values.
func PurgePlannings(model RecyclePlanningsModel) (RecyclePlanningsModel, error) {
	if len(model.PlanningIds) == 0 {
		return model, nil
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin purge plannings transaction failed, err:%s", err.Error())
		return RecyclePlanningsModel{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// Only rows already in the recycle bin can be purged; lock them so a concurrent restore cannot slip through.
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM requirements
		WHERE admin_id = $1 AND workspace_id = $2 AND type = $3 AND deleted_at > 0 AND id = ANY($4::INTEGER[])
		FOR UPDATE`,
		tx, model.AdminId, cast.ToInt(model.WorkspaceId), define.TypeKeyPlanning, pq.Array(model.PlanningIds))
	if err != nil {
		logs.Error("lock plannings for purge failed, err:%s", err.Error())
		return RecyclePlanningsModel{}, err
	}
	if len(rows) == 0 {
		return model, nil
	}
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, cast.ToInt(row[`id`]))
	}

	if err := purgePlanningRows(tx, model.AdminId, model.WorkspaceId, ids); err != nil {
		return RecyclePlanningsModel{}, err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit purge plannings transaction failed, err:%s", err.Error())
		return RecyclePlanningsModel{}, err
	}
	model.Affected = len(ids)
	return model, nil
}

// purgePlanningRows physically deletes plannings and their related data inside a transaction.
// The recycle-bin purge and the empty-title direct delete share this logic so that neither side
// misses a related table.
func purgePlanningRows(tx *sql.Tx, adminId int, workspaceId string, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	if _, err := msql.RawExec(define.Postgres, `DELETE FROM planning_relations
		WHERE admin_id = $1 AND planning_id = ANY($2::INTEGER[])`, tx, adminId, pq.Array(ids)); err != nil {
		logs.Error("purge planning relations failed, err:%s", err.Error())
		return err
	}
	if _, err := msql.RawExec(define.Postgres, `DELETE FROM requirement_custom_field_values
		WHERE admin_id = $1 AND requirement_id = ANY($2::INTEGER[])`, tx, adminId, pq.Array(ids)); err != nil {
		logs.Error("purge planning custom field values failed, err:%s", err.Error())
		return err
	}
	if _, err := msql.RawExec(define.Postgres, `DELETE FROM requirements
		WHERE admin_id = $1 AND workspace_id = $2 AND type = $3 AND id = ANY($4::INTEGER[])`,
		tx, adminId, cast.ToInt(workspaceId), define.TypeKeyPlanning, pq.Array(ids)); err != nil {
		logs.Error("purge plannings failed, err:%s", err.Error())
		return err
	}
	return nil
}

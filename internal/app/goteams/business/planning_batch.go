// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"fmt"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_storage"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// planningInsertColumnCount is the column count of one planning INSERT row, used to size the placeholder chunks.
const planningInsertColumnCount = 15

// planningInsertChunkSize caps the rows of a single multi-value INSERT.
// PostgreSQL allows at most 65535 bind parameters per statement; 500 rows x 15 columns = 7500, well within the limit.
const planningInsertChunkSize = 500

// planningDefaultColor is the title colour given to planning rows created without an explicit colour.
const planningDefaultColor = `#434343`

// PlanningDraftModel is one row of a batch create; zero values fall back to the project defaults.
type PlanningDraftModel struct {
	Title            string
	Priority         int
	StatusId         int
	AssigneeIds      []int
	PlannedStartDate int64
	PlannedEndDate   int64
	CustomFields     CustomFieldValues
}

// BatchCreatePlanningsModel describes the batch create-planning use case.
type BatchCreatePlanningsModel struct {
	AdminId     int
	UserId      int
	WorkspaceId string
	GroupId     int
	Drafts      []PlanningDraftModel
	CreatedIds  []int
	Created     int
}

// BatchCreatePlannings appends planning rows to the end of the group in one transaction.
//
// Unlike calling CreatePlanning in a loop, the per-row lookups (group check, initial status,
// default priority, member validation, sort-order probing) are hoisted out and executed once,
// and the rows themselves go in via multi-value INSERTs. This turns an O(n) round-trip count
// into a constant one, and makes the whole batch atomic: a failure rolls everything back
// instead of leaving a half-imported list behind.
func BatchCreatePlannings(model BatchCreatePlanningsModel) (BatchCreatePlanningsModel, error) {
	if len(model.Drafts) == 0 {
		return model, nil
	}
	if err := CheckStorageQuota(model.AdminId, 0); err != nil {
		return model, err
	}

	// The group check must run before every default-value query: a missing group must return
	// 404 planning group not found, matching the error semantics of single-row CreatePlanning.
	// Otherwise getInitialStatusId fails first with an internal error over the missing workspace
	// config and masks the 404 as a 500. The transaction re-checks it to guard concurrent deletes.
	if err := ensurePlanningGroupExists(nil, model.AdminId, model.WorkspaceId, model.GroupId); err != nil {
		return model, err
	}

	// Batch-wide defaults and validation data, fetched once instead of once per row.
	initialStatusId, err := getInitialStatusId(model.AdminId, model.WorkspaceId, define.TypeKeyPlanning)
	if err != nil {
		return model, err
	}
	defaultPriority := getDefaultPriorityId(model.AdminId, model.WorkspaceId, define.TypeKeyPlanning)
	memberIds, err := listActivePlanningMemberIds(model.AdminId, model.WorkspaceId)
	if err != nil {
		return model, err
	}

	// Normalise and validate in memory; the transaction below should only ever perform writes.
	drafts := make([]PlanningDraftModel, len(model.Drafts))
	copy(drafts, model.Drafts)
	hasCustomFields := false
	for index := range drafts {
		draft := &drafts[index]
		draft.Title = lib_storage.StripOSSURLSignatures(draft.Title)
		if draft.StatusId == 0 {
			draft.StatusId = initialStatusId
		}
		if draft.Priority == 0 {
			draft.Priority = defaultPriority
		}
		if err := validate.ValidatePlannedDateRange(draft.PlannedStartDate, draft.PlannedEndDate); err != nil {
			return model, err
		}
		assigneeIds := uniqueInts(draft.AssigneeIds)
		for _, id := range assigneeIds {
			if !memberIds[id] {
				return model, ErrRequirementAssigneesInvalid
			}
		}
		draft.AssigneeIds = assigneeIds
		if len(draft.CustomFields) > 0 {
			hasCustomFields = true
		}
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin batch create plannings transaction failed, err:%s", err.Error())
		return model, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := ensurePlanningGroupExists(tx, model.AdminId, model.WorkspaceId, model.GroupId); err != nil {
		return model, err
	}
	// Batch rows are always appended, so the sort values can be derived in memory
	// from a single MAX probe instead of re-reading neighbours for every row.
	baseOrder, err := planningMaxSortOrder(tx, model.AdminId, model.WorkspaceId, model.GroupId)
	if err != nil {
		return model, err
	}

	now := int64(tool.Time2Int())
	orderToIndex := make(map[int64]int, len(drafts))
	ids := make([]int, len(drafts))
	for start := 0; start < len(drafts); start += planningInsertChunkSize {
		end := start + planningInsertChunkSize
		if end > len(drafts) {
			end = len(drafts)
		}
		if err := insertPlanningChunk(tx, model, drafts[start:end], baseOrder, start, now, orderToIndex, ids); err != nil {
			return model, err
		}
	}

	if err = tx.Commit(); err != nil {
		logs.Error("commit batch create plannings transaction failed, err:%s", err.Error())
		return model, err
	}

	// Custom field values are written after the commit, mirroring CreatePlanning, so the
	// insert transaction stays short. Skipped entirely when no draft carries custom values.
	if hasCustomFields {
		if err := batchInsertPlanningCustomFields(model.AdminId, model.WorkspaceId, drafts, ids, now); err != nil {
			return model, err
		}
	}

	model.CreatedIds = ids
	model.Created = len(ids)
	return model, nil
}

// insertPlanningChunk writes one multi-value INSERT and maps the returned IDs back to their draft index.
// The mapping goes through sort_order rather than the RETURNING order, because SQL does not
// guarantee that a multi-row INSERT returns rows in VALUES order.
func insertPlanningChunk(tx *sql.Tx, model BatchCreatePlanningsModel, drafts []PlanningDraftModel,
	baseOrder int64, offset int, now int64, orderToIndex map[int64]int, ids []int) error {
	placeholders, args := buildPlanningInsertChunk(model, drafts, baseOrder, offset, now, orderToIndex)
	rows, err := msql.RawValues(define.Postgres, `INSERT INTO requirements
		(admin_id, workspace_id, group_id, title, color, type, priority, status_id, assignee_ids,
		 planned_start_date, planned_end_date, sort_order, created_by, created_at, updated_at)
		VALUES `+strings.Join(placeholders, `,`)+` RETURNING id, sort_order`, tx, args...)
	if err != nil {
		logs.Error("batch insert plannings failed, err:%s", err.Error())
		return err
	}
	if len(rows) != len(drafts) {
		return fmt.Errorf("batch insert plannings returned %d rows, want %d", len(rows), len(drafts))
	}
	for _, row := range rows {
		index, ok := orderToIndex[cast.ToInt64(row[`sort_order`])]
		if !ok {
			return fmt.Errorf("batch insert plannings returned unknown sort_order %s", row[`sort_order`])
		}
		ids[index] = cast.ToInt(row[`id`])
	}
	return nil
}

// buildPlanningInsertChunk assembles the placeholders and bind args of one multi-value INSERT,
// and records the sort_order to draft-index mapping. Kept free of DB access so the sort-order
// sequence and the parameter layout can be unit tested directly.
func buildPlanningInsertChunk(model BatchCreatePlanningsModel, drafts []PlanningDraftModel,
	baseOrder int64, offset int, now int64, orderToIndex map[int64]int) ([]string, []any) {
	placeholders := make([]string, 0, len(drafts))
	args := make([]any, 0, len(drafts)*planningInsertColumnCount)
	for index, draft := range drafts {
		position := len(args)
		cells := make([]string, planningInsertColumnCount)
		for cell := range cells {
			cells[cell] = fmt.Sprintf(`$%d`, position+cell+1)
		}
		placeholders = append(placeholders, `(`+strings.Join(cells, `,`)+`)`)

		// Rows are appended in file order, so the sort values are a plain arithmetic sequence
		// above the group's current maximum; offset keeps it continuous across chunks.
		sortOrder := baseOrder + define.PlanningSortGap*int64(offset+index+1)
		orderToIndex[sortOrder] = offset + index
		args = append(args,
			model.AdminId, cast.ToInt(model.WorkspaceId), model.GroupId,
			draft.Title, planningDefaultColor, define.TypeKeyPlanning,
			draft.Priority, draft.StatusId, pq.Array(draft.AssigneeIds),
			draft.PlannedStartDate, draft.PlannedEndDate, sortOrder,
			model.UserId, now, now)
	}
	return placeholders, args
}

// batchInsertPlanningCustomFields writes the custom field values of freshly created plannings.
// New rows cannot have previous values, so unlike SaveCustomFieldValues there is no
// read-then-update step: everything goes in through chunked multi-value INSERTs.
func batchInsertPlanningCustomFields(adminId int, wsId string, drafts []PlanningDraftModel, ids []int, now int64) error {
	fieldsModel, err := ListCustomFields(ListCustomFieldsModel{AdminId: adminId, WsId: wsId, TypeKey: define.TypeKeyPlanning})
	if err != nil {
		return err
	}
	memberScope, err := loadCustomFieldMemberScopeForFields(adminId, wsId, fieldsModel.Items)
	if err != nil {
		return err
	}
	type valueRow struct {
		requirementId int
		fieldId       int
		text          string
	}
	pending := make([]valueRow, 0, len(drafts))
	for index, draft := range drafts {
		if len(draft.CustomFields) == 0 {
			continue
		}
		for _, field := range fieldsModel.Items {
			if field.IsEnabled != 1 {
				continue
			}
			value, exists := draft.CustomFields[cast.ToString(field.Id)]
			if !exists {
				continue
			}
			text, err := customFieldValueText(value)
			if err != nil {
				return err
			}
			if len(text) == 0 || text == `[]` {
				continue
			}
			if err := validateCustomFieldValue(field, text, memberScope); err != nil {
				return err
			}
			pending = append(pending, valueRow{requirementId: ids[index], fieldId: field.Id, text: text})
		}
	}
	if len(pending) == 0 {
		return nil
	}

	const columnCount = 6
	// Same parameter-budget reasoning as the planning INSERT: 6 columns, so chunk more rows per statement.
	chunkSize := planningInsertChunkSize * 2
	for start := 0; start < len(pending); start += chunkSize {
		end := start + chunkSize
		if end > len(pending) {
			end = len(pending)
		}
		placeholders := make([]string, 0, end-start)
		args := make([]any, 0, (end-start)*columnCount)
		for _, item := range pending[start:end] {
			position := len(args)
			cells := make([]string, columnCount)
			for cell := range cells {
				cells[cell] = fmt.Sprintf(`$%d`, position+cell+1)
			}
			placeholders = append(placeholders, `(`+strings.Join(cells, `,`)+`)`)
			args = append(args, adminId, item.requirementId, item.fieldId, item.text, now, now)
		}
		if _, err := msql.RawExec(define.Postgres, `INSERT INTO requirement_custom_field_values
			(admin_id, requirement_id, field_id, value_text, created_at, updated_at)
			VALUES `+strings.Join(placeholders, `,`)+`
			ON CONFLICT (admin_id, requirement_id, field_id) DO UPDATE SET
			value_text = EXCLUDED.value_text, updated_at = EXCLUDED.updated_at`, nil, args...); err != nil {
			logs.Error("batch insert planning custom field values failed, err:%s", err.Error())
			return err
		}
	}
	return nil
}

// listActivePlanningMemberIds returns the enabled members of the workspace as a set,
// so a batch can validate assignees in memory instead of querying once per row.
func listActivePlanningMemberIds(adminId int, wsId string) (map[int]bool, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT wm.user_id FROM workspace_members wm
		JOIN users u ON u.id = wm.user_id AND u.admin_id = wm.admin_id AND COALESCE(u.status, 1) = 1
		WHERE wm.admin_id = $1 AND wm.workspace_id = $2`, nil, adminId, cast.ToInt(wsId))
	if err != nil {
		logs.Error("list active workspace members failed, err:%s", err.Error())
		return nil, err
	}
	members := make(map[int]bool, len(rows))
	for _, row := range rows {
		members[cast.ToInt(row[`user_id`])] = true
	}
	return members, nil
}

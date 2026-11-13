// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/define"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

var (
	// ErrWorkItemActionInvalid indicates malformed action input or an unsupported type transition.
	ErrWorkItemActionInvalid = errors.New("invalid work item action")
	// ErrWorkItemActionForbidden indicates that the source or target permission check failed.
	ErrWorkItemActionForbidden = errors.New("work item action forbidden")
	// ErrWorkItemMergeTargetInvalid indicates that a merge target is missing, inaccessible, or is not a defect.
	ErrWorkItemMergeTargetInvalid = errors.New("invalid work item merge target")
)

// WorkItemActionTargetModel describes a project available to a work-item action.
type WorkItemActionTargetModel struct {
	Id   int
	Name string
}

// ListWorkItemActionTargetsModel describes target-project discovery.
type ListWorkItemActionTargetsModel struct {
	AdminId           int
	UserId            int
	SourceWorkspaceId int
	Type              string
	Action            string
	Items             []WorkItemActionTargetModel
}

// WorkItemMergeTargetModel describes a defect available as a merge target.
type WorkItemMergeTargetModel struct {
	Id            int
	WorkspaceId   int
	WorkspaceName string
	Title         string
}

// ListWorkItemMergeTargetsModel describes merge-target discovery.
type ListWorkItemMergeTargetsModel struct {
	AdminId           int
	UserId            int
	SourceWorkspaceId int
	SourceId          int
	Items             []WorkItemMergeTargetModel
}

// WorkItemActionResultModel identifies the work item that remains after an action.
type WorkItemActionResultModel struct {
	Id          int
	WorkspaceId int
	Type        string
}

// TransferWorkItemModel describes a move or type-conversion action.
type TransferWorkItemModel struct {
	AdminId           int
	UserId            int
	SourceWorkspaceId int
	RequirementId     int
	TargetWorkspaceId int
	TargetType        string
	Convert           bool
}

// MergeWorkItemModel describes merging one defect into another.
type MergeWorkItemModel struct {
	AdminId           int
	UserId            int
	SourceWorkspaceId int
	SourceId          int
	TargetWorkspaceId int
	TargetId          int
}

// ListWorkItemActionTargets lists active projects where the user can create the requested type.
func ListWorkItemActionTargets(model ListWorkItemActionTargetsModel) (ListWorkItemActionTargetsModel, error) {
	if model.AdminId <= 0 || model.UserId <= 0 || model.SourceWorkspaceId <= 0 || !isTransferType(model.Type) ||
		(model.Action != `move` && model.Action != `convert`) {
		return ListWorkItemActionTargetsModel{}, ErrWorkItemActionInvalid
	}
	sourceType := model.Type
	if model.Action == `convert` {
		if model.Type == define.TypeKeyRequirement {
			sourceType = define.TypeKeyDefect
		} else {
			sourceType = define.TypeKeyRequirement
		}
	}
	if !HasPermission(model.AdminId, model.UserId, cast.ToString(model.SourceWorkspaceId), workItemActionPermissionKey(sourceType, `edit`)) {
		return ListWorkItemActionTargetsModel{}, ErrWorkItemActionForbidden
	}
	permissionKey := workItemActionPermissionKey(model.Type, `create`)
	var rows []msql.Params
	var err error
	if model.UserId == model.AdminId {
		rows, err = msql.RawValues(define.Postgres, `SELECT id, name FROM workspaces
			WHERE admin_id = $1 AND COALESCE(ended_at, 0) = 0 ORDER BY sort_order ASC, id DESC`, nil, model.AdminId)
	} else {
		rows, err = msql.RawValues(define.Postgres, `SELECT DISTINCT w.id, w.name, w.sort_order
			FROM workspace_members wm
			JOIN role_permissions rp ON rp.admin_id = wm.admin_id AND rp.role_id = wm.role_id
			JOIN workspaces w ON w.admin_id = wm.admin_id AND w.id = wm.workspace_id
			WHERE wm.admin_id = $1 AND wm.user_id = $2 AND rp.permission_key = $3
				AND COALESCE(w.ended_at, 0) = 0
			ORDER BY w.sort_order ASC, w.id DESC`, nil, model.AdminId, model.UserId, permissionKey)
	}
	if err != nil {
		logs.Error("list work item action targets failed: %s", err.Error())
		return ListWorkItemActionTargetsModel{}, err
	}
	model.Items = make([]WorkItemActionTargetModel, 0, len(rows))
	for _, row := range rows {
		id, convErr := strconv.Atoi(row[`id`])
		if convErr != nil {
			return ListWorkItemActionTargetsModel{}, fmt.Errorf("parse work item action target id: %w", convErr)
		}
		if model.Action == `move` && id == model.SourceWorkspaceId {
			continue
		}
		model.Items = append(model.Items, WorkItemActionTargetModel{Id: id, Name: row[`name`]})
	}
	return model, nil
}

// ListWorkItemMergeTargets lists visible defects in projects where the user can edit defects.
func ListWorkItemMergeTargets(model ListWorkItemMergeTargetsModel) (ListWorkItemMergeTargetsModel, error) {
	if model.AdminId <= 0 || model.UserId <= 0 || model.SourceWorkspaceId <= 0 || model.SourceId <= 0 {
		return ListWorkItemMergeTargetsModel{}, ErrWorkItemActionInvalid
	}
	visible, err := CheckRequirementVisible(model.AdminId, cast.ToString(model.SourceWorkspaceId), cast.ToString(model.SourceId), model.UserId)
	if err != nil {
		return ListWorkItemMergeTargetsModel{}, err
	}
	if !visible {
		return ListWorkItemMergeTargetsModel{}, ErrWorkItemActionForbidden
	}
	source, err := GetRequirementById(model.AdminId, cast.ToString(model.SourceWorkspaceId), cast.ToString(model.SourceId))
	if err != nil {
		return ListWorkItemMergeTargetsModel{}, err
	}
	if len(source) == 0 || source[`type`] != define.TypeKeyDefect {
		return ListWorkItemMergeTargetsModel{}, ErrWorkItemMergeTargetInvalid
	}
	if !HasPermission(model.AdminId, model.UserId, cast.ToString(model.SourceWorkspaceId), `defect:delete`) {
		return ListWorkItemMergeTargetsModel{}, ErrWorkItemActionForbidden
	}
	projects, err := listWorkItemPermissionProjects(model.AdminId, model.UserId, `defect:edit`)
	if err != nil {
		return ListWorkItemMergeTargetsModel{}, err
	}
	model.Items = []WorkItemMergeTargetModel{}
	for _, project := range projects {
		if !HasPermission(model.AdminId, model.UserId, cast.ToString(project.Id), `defect:edit`) {
			continue
		}
		rows, queryErr := msql.RawValues(define.Postgres, `SELECT id, title, created_by, assignee_ids::text AS assignee_ids
			FROM requirements WHERE admin_id = $1 AND workspace_id = $2 AND type = $3 AND deleted_at = 0
			ORDER BY id DESC`, nil, model.AdminId, project.Id, define.TypeKeyDefect)
		if queryErr != nil {
			return ListWorkItemMergeTargetsModel{}, queryErr
		}
		for _, row := range rows {
			id := cast.ToInt(row[`id`])
			if id == model.SourceId && project.Id == model.SourceWorkspaceId {
				continue
			}
			assignees, parseErr := parsePgIntArray(row[`assignee_ids`])
			if parseErr != nil {
				return ListWorkItemMergeTargetsModel{}, parseErr
			}
			allowed, scopeErr := requirementDataViewVisible(model.AdminId, cast.ToString(project.Id), model.UserId, cast.ToInt(row[`created_by`]), assignees)
			if scopeErr != nil {
				return ListWorkItemMergeTargetsModel{}, scopeErr
			}
			if allowed {
				model.Items = append(model.Items, WorkItemMergeTargetModel{Id: id, WorkspaceId: project.Id, WorkspaceName: project.Name, Title: row[`title`]})
			}
		}
	}
	sortMergeTargets(model.Items)
	return model, nil
}

type transferField struct {
	Id        int
	Name      string
	FieldType string
	Options   []CustomFieldOption
}

type transferBuiltinConfig struct {
	Enabled  int
	Editable int
}

type transferItem struct {
	Id          int
	ParentId    int
	Type        string
	StatusId    int
	PriorityId  int
	VersionId   int
	AssigneeIds []int
}

// TransferWorkItem moves a work item/tree, optionally converting its type.
func TransferWorkItem(model TransferWorkItemModel) (WorkItemActionResultModel, error) {
	if model.AdminId <= 0 || model.UserId <= 0 || model.SourceWorkspaceId <= 0 || model.RequirementId <= 0 || model.TargetWorkspaceId <= 0 || !isTransferType(model.TargetType) {
		return WorkItemActionResultModel{}, ErrWorkItemActionInvalid
	}
	source, err := GetRequirementById(model.AdminId, cast.ToString(model.SourceWorkspaceId), cast.ToString(model.RequirementId))
	if err != nil {
		return WorkItemActionResultModel{}, err
	}
	if len(source) == 0 || !isTransferType(source[`type`]) {
		return WorkItemActionResultModel{}, ErrRequirementNotFound
	}
	sourceType := source[`type`]
	if model.Convert == (sourceType == model.TargetType) {
		return WorkItemActionResultModel{}, ErrWorkItemActionInvalid
	}
	if !model.Convert && model.SourceWorkspaceId == model.TargetWorkspaceId {
		return WorkItemActionResultModel{}, ErrWorkItemActionInvalid
	}
	if !HasPermission(model.AdminId, model.UserId, cast.ToString(model.SourceWorkspaceId), workItemActionPermissionKey(sourceType, `edit`)) ||
		!HasPermission(model.AdminId, model.UserId, cast.ToString(model.TargetWorkspaceId), workItemActionPermissionKey(model.TargetType, `create`)) {
		return WorkItemActionResultModel{}, ErrWorkItemActionForbidden
	}
	targetProjects, err := msql.RawValues(define.Postgres, `SELECT id FROM workspaces
		WHERE admin_id=$1 AND id=$2 AND COALESCE(ended_at,0)=0`, nil, model.AdminId, model.TargetWorkspaceId)
	if err != nil {
		return WorkItemActionResultModel{}, err
	}
	if len(targetProjects) == 0 {
		return WorkItemActionResultModel{}, ErrWorkItemActionInvalid
	}
	visible, err := CheckRequirementVisible(model.AdminId, cast.ToString(model.SourceWorkspaceId), cast.ToString(model.RequirementId), model.UserId)
	if err != nil {
		return WorkItemActionResultModel{}, err
	}
	if !visible {
		return WorkItemActionResultModel{}, ErrWorkItemActionForbidden
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return WorkItemActionResultModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockSLAWorkspacesTx(tx, model.AdminId, []int{model.SourceWorkspaceId, model.TargetWorkspaceId}); err != nil {
		return WorkItemActionResultModel{}, err
	}
	workspaceIds := []int{model.SourceWorkspaceId, model.TargetWorkspaceId}
	sort.Ints(workspaceIds)
	for index, workspaceId := range workspaceIds {
		if index == 0 || workspaceId != workspaceIds[index-1] {
			if err = lockRequirementHierarchyTx(tx, model.AdminId, workspaceId); err != nil {
				return WorkItemActionResultModel{}, err
			}
		}
	}
	items, err := lockTransferItems(tx, model.AdminId, model.SourceWorkspaceId, model.RequirementId, sourceType)
	if err != nil {
		return WorkItemActionResultModel{}, err
	}
	if len(items) == 0 {
		return WorkItemActionResultModel{}, ErrRequirementNotFound
	}
	targetStatusId, err := mapNamedDefinition(tx, `workspace_statuses`, model.AdminId, model.SourceWorkspaceId, model.TargetWorkspaceId, sourceType, model.TargetType, items[0].StatusId, true)
	if err != nil {
		return WorkItemActionResultModel{}, err
	}
	targetPriorityId, err := mapNamedDefinition(tx, `workspace_priorities`, model.AdminId, model.SourceWorkspaceId, model.TargetWorkspaceId, sourceType, model.TargetType, items[0].PriorityId, false)
	if err != nil {
		return WorkItemActionResultModel{}, err
	}
	targetMemberIds, err := targetWorkspaceMemberSet(tx, model.AdminId, model.TargetWorkspaceId)
	if err != nil {
		return WorkItemActionResultModel{}, err
	}
	sourceFields, err := loadTransferFields(tx, model.AdminId, model.SourceWorkspaceId, sourceType)
	if err != nil {
		return WorkItemActionResultModel{}, err
	}
	targetFields, err := loadTransferFields(tx, model.AdminId, model.TargetWorkspaceId, model.TargetType)
	if err != nil {
		return WorkItemActionResultModel{}, err
	}
	sourceBuiltinConfigs, err := loadTransferBuiltinConfigs(tx, model.AdminId, model.SourceWorkspaceId, sourceType)
	if err != nil {
		return WorkItemActionResultModel{}, err
	}
	targetBuiltinConfigs, err := loadTransferBuiltinConfigs(tx, model.AdminId, model.TargetWorkspaceId, model.TargetType)
	if err != nil {
		return WorkItemActionResultModel{}, err
	}
	now := tool.Time2Int()
	itemIDs := make([]int64, 0, len(items))
	itemSet := make(map[int]bool, len(items))
	for _, item := range items {
		itemIDs = append(itemIDs, int64(item.Id))
		itemSet[item.Id] = true
	}
	for _, item := range items {
		statusId := targetStatusId
		priorityId := targetPriorityId
		if item.Id != items[0].Id {
			statusId, err = mapNamedDefinition(tx, `workspace_statuses`, model.AdminId, model.SourceWorkspaceId, model.TargetWorkspaceId, sourceType, model.TargetType, item.StatusId, true)
			if err != nil {
				return WorkItemActionResultModel{}, err
			}
			priorityId, err = mapNamedDefinition(tx, `workspace_priorities`, model.AdminId, model.SourceWorkspaceId, model.TargetWorkspaceId, sourceType, model.TargetType, item.PriorityId, false)
			if err != nil {
				return WorkItemActionResultModel{}, err
			}
		}
		parentId := transferParentID(item, items[0], itemSet, model)
		assignees := filterTransferAssignees(item.AssigneeIds, targetMemberIds)
		if !matchingBuiltinConfig(`assignee`, sourceBuiltinConfigs, targetBuiltinConfigs) {
			assignees = []int{}
		}
		plannedStartDateSQL := `planned_start_date`
		plannedEndDateSQL := `planned_end_date`
		if !matchingBuiltinConfig(`planned_start_date`, sourceBuiltinConfigs, targetBuiltinConfigs) {
			plannedStartDateSQL = `0`
		}
		if !matchingBuiltinConfig(`planned_end_date`, sourceBuiltinConfigs, targetBuiltinConfigs) {
			plannedEndDateSQL = `0`
		}
		categoryId := 0
		versionId := 0
		if model.SourceWorkspaceId == model.TargetWorkspaceId && !model.Convert {
			versionId = item.VersionId
			if sourceType == define.TypeKeyRequirement {
				categoryRows, queryErr := msql.RawValues(define.Postgres, `SELECT category_id FROM requirements WHERE admin_id=$1 AND workspace_id=$2 AND id=$3`, tx, model.AdminId, model.SourceWorkspaceId, item.Id)
				if queryErr != nil {
					return WorkItemActionResultModel{}, queryErr
				}
				if len(categoryRows) > 0 {
					categoryId = cast.ToInt(categoryRows[0][`category_id`])
				}
			}
		}
		if model.Convert && sourceType == define.TypeKeyDefect && model.TargetType == define.TypeKeyRequirement && parentId > 0 {
			parentRows, queryErr := msql.RawValues(define.Postgres, `SELECT category_id FROM requirements
				WHERE admin_id=$1 AND workspace_id=$2 AND id=$3 AND type=$4 LIMIT 1`, tx,
				model.AdminId, model.TargetWorkspaceId, parentId, define.TypeKeyRequirement)
			if queryErr != nil {
				return WorkItemActionResultModel{}, queryErr
			}
			if len(parentRows) == 0 {
				parentId = 0
			} else {
				categoryId = cast.ToInt(parentRows[0][`category_id`])
			}
		}
		completedAt, completionErr := initialCompletionTime(model.AdminId, model.TargetWorkspaceId, model.TargetType, statusId, int64(now))
		if completionErr != nil {
			return WorkItemActionResultModel{}, completionErr
		}
		if completedAt > 0 {
			sourceFinalIds, finalErr := workItemCompletedIDs(model.AdminId, model.SourceWorkspaceId)
			if finalErr != nil {
				return WorkItemActionResultModel{}, finalErr
			}
			if intSliceContains(sourceFinalIds, item.StatusId) {
				rows, readErr := msql.RawValues(define.Postgres, `SELECT completed_at FROM `+define.TableRequirements+` WHERE admin_id=$1 AND workspace_id=$2 AND id=$3`, tx, model.AdminId, model.SourceWorkspaceId, item.Id)
				if readErr != nil {
					return WorkItemActionResultModel{}, readErr
				}
				if len(rows) != 1 {
					return WorkItemActionResultModel{}, ErrRequirementNotFound
				}
				completedAt, readErr = strconv.ParseInt(rows[0]["completed_at"], 10, 64)
				if readErr != nil {
					return WorkItemActionResultModel{}, readErr
				}
			}
		}
		if item.Type == define.TypeKeyRequirement {
			if err = interruptSLAItemTx(tx, model.AdminId, model.SourceWorkspaceId, item.Id, "transferred"); err != nil {
				return WorkItemActionResultModel{}, err
			}
		}
		updateSQL := fmt.Sprintf(`UPDATE requirements SET workspace_id=$1, type=$2, parent_id=$3,
			category_id=$4, status_id=$5, priority=$6, version_id=$7, assignee_ids=$8,
			planned_start_date=%s, planned_end_date=%s, updated_at=$9, completed_at=$13, sla_match=NULL
			WHERE admin_id=$10 AND workspace_id=$11 AND id=$12`, plannedStartDateSQL, plannedEndDateSQL)
		if _, err = msql.RawExec(define.Postgres, updateSQL, tx,
			model.TargetWorkspaceId, model.TargetType, parentId, categoryId, statusId, priorityId, versionId,
			pq.Array(assignees), now, model.AdminId, model.SourceWorkspaceId, item.Id, completedAt); err != nil {
			return WorkItemActionResultModel{}, err
		}
		if model.TargetType == define.TypeKeyRequirement {
			if err = recordSLATransferredTx(tx, model.AdminId, model.TargetWorkspaceId, item.Id, int64(now)); err != nil {
				return WorkItemActionResultModel{}, err
			}
		}
		if err = remapCustomFieldValues(tx, model.AdminId, item.Id, sourceFields, targetFields, targetMemberIds, now); err != nil {
			return WorkItemActionResultModel{}, err
		}
		if model.SourceWorkspaceId != model.TargetWorkspaceId {
			if err = createHistoryTx(tx, model.AdminId, model.TargetWorkspaceId, item.Id, model.UserId,
				define.HistoryActionFieldUpdated, `workspace_id`, cast.ToString(model.SourceWorkspaceId), cast.ToString(model.TargetWorkspaceId)); err != nil {
				return WorkItemActionResultModel{}, err
			}
		}
		if model.Convert {
			if err = createHistoryTx(tx, model.AdminId, model.TargetWorkspaceId, item.Id, model.UserId,
				define.HistoryActionFieldUpdated, `type`, sourceType, model.TargetType); err != nil {
				return WorkItemActionResultModel{}, err
			}
		}
	}
	if model.SourceWorkspaceId != model.TargetWorkspaceId {
		for _, table := range []string{define.TableRequirementComments, define.TableRequirementHistories} {
			if _, err = msql.RawExec(define.Postgres, fmt.Sprintf(`UPDATE %s SET workspace_id=$1 WHERE admin_id=$2 AND requirement_id=ANY($3)`, table), tx,
				model.TargetWorkspaceId, model.AdminId, pq.Array(itemIDs)); err != nil {
				return WorkItemActionResultModel{}, err
			}
		}

		if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.GetEnterpriseFileTable(model.AdminId)+`
			SET workspace_id=$1 WHERE admin_id=$2 AND source_id=ANY($3)`, tx,
			model.TargetWorkspaceId, model.AdminId, pq.Array(itemIDs)); err != nil {
			return WorkItemActionResultModel{}, err
		}
		if _, err = msql.RawExec(define.Postgres, `UPDATE requirement_work_logs SET workspace_id=$1
			WHERE admin_id=$2 AND workspace_id=$3 AND requirement_id=ANY($4)`, tx,
			model.TargetWorkspaceId, model.AdminId, model.SourceWorkspaceId, pq.Array(itemIDs)); err != nil {
			return WorkItemActionResultModel{}, err
		}
		if _, err = msql.RawExec(define.Postgres, `DELETE FROM planning_relations WHERE admin_id=$1 AND item_id=ANY($2)`, tx, model.AdminId, pq.Array(itemIDs)); err != nil {
			return WorkItemActionResultModel{}, err
		}
	}
	if sourceType == define.TypeKeyRequirement && (model.SourceWorkspaceId != model.TargetWorkspaceId || model.Convert) {
		defects, queryErr := msql.RawValues(define.Postgres, `SELECT id,parent_id FROM requirements
			WHERE admin_id=$1 AND workspace_id=$2 AND type=$3 AND parent_id=ANY($4) FOR UPDATE`, tx,
			model.AdminId, model.SourceWorkspaceId, define.TypeKeyDefect, pq.Array(itemIDs))
		if queryErr != nil {
			return WorkItemActionResultModel{}, queryErr
		}
		if len(defects) > 0 {
			if _, err = msql.RawExec(define.Postgres, `UPDATE requirements SET parent_id=0,updated_at=$1
				WHERE admin_id=$2 AND workspace_id=$3 AND type=$4 AND parent_id=ANY($5)`, tx,
				now, model.AdminId, model.SourceWorkspaceId, define.TypeKeyDefect, pq.Array(itemIDs)); err != nil {
				return WorkItemActionResultModel{}, err
			}
			for _, defect := range defects {
				if err = createHistoryTx(tx, model.AdminId, model.SourceWorkspaceId, cast.ToInt(defect[`id`]), model.UserId,
					define.HistoryActionFieldUpdated, `parent_id`, defect[`parent_id`], `0`); err != nil {
					return WorkItemActionResultModel{}, err
				}
			}
		}
	}
	versionIds := make(map[int]bool)
	for _, item := range items {
		if item.VersionId <= 0 || versionIds[item.VersionId] {
			continue
		}
		versionIds[item.VersionId] = true
		if err = RecalcVersionCounts(tx, model.AdminId, item.VersionId); err != nil {
			return WorkItemActionResultModel{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return WorkItemActionResultModel{}, err
	}
	return WorkItemActionResultModel{Id: model.RequirementId, WorkspaceId: model.TargetWorkspaceId, Type: model.TargetType}, nil
}

// MergeWorkItem merges a source defect into a target defect and deletes the source row.
func MergeWorkItem(model MergeWorkItemModel) (WorkItemActionResultModel, error) {
	if model.AdminId <= 0 || model.UserId <= 0 || model.SourceWorkspaceId <= 0 || model.TargetWorkspaceId <= 0 || model.SourceId <= 0 || model.TargetId <= 0 ||
		(model.SourceWorkspaceId == model.TargetWorkspaceId && model.SourceId == model.TargetId) {
		return WorkItemActionResultModel{}, ErrWorkItemActionInvalid
	}
	if !HasPermission(model.AdminId, model.UserId, cast.ToString(model.SourceWorkspaceId), `defect:delete`) ||
		!HasPermission(model.AdminId, model.UserId, cast.ToString(model.TargetWorkspaceId), `defect:edit`) {
		return WorkItemActionResultModel{}, ErrWorkItemActionForbidden
	}
	for _, item := range []struct{ ws, id int }{{model.SourceWorkspaceId, model.SourceId}, {model.TargetWorkspaceId, model.TargetId}} {
		visible, err := CheckRequirementVisible(model.AdminId, cast.ToString(item.ws), cast.ToString(item.id), model.UserId)
		if err != nil {
			return WorkItemActionResultModel{}, err
		}
		if !visible {
			return WorkItemActionResultModel{}, ErrWorkItemActionForbidden
		}
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return WorkItemActionResultModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := msql.RawValues(define.Postgres, `SELECT id, workspace_id, title, description, description_mode, version_id, completed_work_hours
		FROM requirements WHERE admin_id=$1 AND ((workspace_id=$2 AND id=$3) OR (workspace_id=$4 AND id=$5))
		AND type=$6 FOR UPDATE`, tx, model.AdminId, model.SourceWorkspaceId, model.SourceId,
		model.TargetWorkspaceId, model.TargetId, define.TypeKeyDefect)
	if err != nil {
		return WorkItemActionResultModel{}, err
	}
	if len(rows) != 2 {
		return WorkItemActionResultModel{}, ErrWorkItemMergeTargetInvalid
	}
	var source, target msql.Params
	for _, row := range rows {
		if cast.ToInt(row[`id`]) == model.SourceId && cast.ToInt(row[`workspace_id`]) == model.SourceWorkspaceId {
			source = row
		} else {
			target = row
		}
	}
	if len(source) == 0 || len(target) == 0 {
		return WorkItemActionResultModel{}, ErrWorkItemMergeTargetInvalid
	}
	mergedDescription := appendMergedDescription(target, source)
	mergedCompletedHours := roundRequirementHours(cast.ToFloat64(target[`completed_work_hours`]) + cast.ToFloat64(source[`completed_work_hours`]))
	if mergedCompletedHours > 99999999.99 {
		return WorkItemActionResultModel{}, ErrRequirementWorkHoursTotalInvalid
	}
	now := tool.Time2Int()
	if _, err = msql.RawExec(define.Postgres, `UPDATE requirements SET description=$1, comment_count=comment_count+
		(SELECT COUNT(*) FROM requirement_comments WHERE admin_id=$2 AND requirement_id=$3), completed_work_hours=$4, updated_at=$5
		WHERE admin_id=$2 AND workspace_id=$6 AND id=$7`, tx, mergedDescription, model.AdminId, model.SourceId,
		mergedCompletedHours, now, model.TargetWorkspaceId, model.TargetId); err != nil {
		return WorkItemActionResultModel{}, err
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE requirement_work_logs SET workspace_id=$1, requirement_id=$2
		WHERE admin_id=$3 AND workspace_id=$4 AND requirement_id=$5`, tx,
		model.TargetWorkspaceId, model.TargetId, model.AdminId, model.SourceWorkspaceId, model.SourceId); err != nil {
		return WorkItemActionResultModel{}, err
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE requirement_comments SET workspace_id=$1, requirement_id=$2 WHERE admin_id=$3 AND requirement_id=$4`, tx,
		model.TargetWorkspaceId, model.TargetId, model.AdminId, model.SourceId); err != nil {
		return WorkItemActionResultModel{}, err
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.GetEnterpriseFileTable(model.AdminId)+`
		SET workspace_id=$1, source_id=$2, source_name=$3 WHERE admin_id=$4 AND source_id=$5`, tx,
		model.TargetWorkspaceId, model.TargetId, target[`title`], model.AdminId, model.SourceId); err != nil {
		return WorkItemActionResultModel{}, err
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE requirement_histories SET workspace_id=$1, requirement_id=$2 WHERE admin_id=$3 AND requirement_id=$4`, tx,
		model.TargetWorkspaceId, model.TargetId, model.AdminId, model.SourceId); err != nil {
		return WorkItemActionResultModel{}, err
	}
	if _, err = msql.RawExec(define.Postgres, `INSERT INTO requirement_visits(admin_id,user_id,requirement_id,visited_at)
		SELECT admin_id,user_id,$1,visited_at FROM requirement_visits WHERE admin_id=$2 AND requirement_id=$3
		ON CONFLICT(admin_id,user_id,requirement_id) DO UPDATE SET visited_at=GREATEST(requirement_visits.visited_at,EXCLUDED.visited_at)`, tx,
		model.TargetId, model.AdminId, model.SourceId); err != nil {
		return WorkItemActionResultModel{}, err
	}
	for _, table := range []string{define.TableCustomFieldValues, define.TableRequirementVisits} {
		if _, err = msql.RawExec(define.Postgres, fmt.Sprintf(`DELETE FROM %s WHERE admin_id=$1 AND requirement_id=$2`, table), tx, model.AdminId, model.SourceId); err != nil {
			return WorkItemActionResultModel{}, err
		}
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM planning_relations WHERE admin_id=$1 AND item_id=$2`, tx, model.AdminId, model.SourceId); err != nil {
		return WorkItemActionResultModel{}, err
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM requirements WHERE admin_id=$1 AND workspace_id=$2 AND id=$3`, tx, model.AdminId, model.SourceWorkspaceId, model.SourceId); err != nil {
		return WorkItemActionResultModel{}, err
	}
	if err = createHistoryTx(tx, model.AdminId, model.TargetWorkspaceId, model.TargetId, model.UserId, define.HistoryActionFieldUpdated, `merge`, cast.ToString(model.SourceId), cast.ToString(model.TargetId)); err != nil {
		return WorkItemActionResultModel{}, err
	}
	if sourceVersionId := cast.ToInt(source[`version_id`]); sourceVersionId > 0 {
		if err = RecalcVersionCounts(tx, model.AdminId, sourceVersionId); err != nil {
			return WorkItemActionResultModel{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return WorkItemActionResultModel{}, err
	}
	return WorkItemActionResultModel{Id: model.TargetId, WorkspaceId: model.TargetWorkspaceId, Type: define.TypeKeyDefect}, nil
}

func isTransferType(value string) bool {
	return value == define.TypeKeyRequirement || value == define.TypeKeyDefect
}

func workItemActionPermissionKey(itemType, action string) string {
	if itemType == define.TypeKeyDefect {
		return `defect:` + action
	}
	return `requirement:` + action
}

func lockTransferItems(tx *sql.Tx, adminId, workspaceId, itemId int, itemType string) ([]transferItem, error) {
	query := `SELECT id,parent_id,type,status_id,priority,version_id,assignee_ids::text AS assignee_ids FROM requirements
		WHERE admin_id=$1 AND workspace_id=$2 AND id=$3 FOR UPDATE`
	if itemType == define.TypeKeyRequirement {
		query = `WITH RECURSIVE tree AS (
			SELECT id FROM requirements WHERE admin_id=$1 AND workspace_id=$2 AND id=$3 AND type='requirement'
			UNION ALL SELECT r.id FROM requirements r JOIN tree t ON r.parent_id=t.id
			WHERE r.admin_id=$1 AND r.workspace_id=$2 AND r.type='requirement'
		) SELECT r.id,r.parent_id,r.type,r.status_id,r.priority,r.version_id,r.assignee_ids::text AS assignee_ids
			FROM requirements r WHERE r.id IN (SELECT id FROM tree)
			ORDER BY CASE WHEN r.id=$3 THEN 0 ELSE 1 END,r.id FOR UPDATE`
	}
	rows, err := msql.RawValues(define.Postgres, query, tx, adminId, workspaceId, itemId)
	if err != nil {
		return nil, err
	}
	items := make([]transferItem, 0, len(rows))
	for _, row := range rows {
		assignees, parseErr := parsePgIntArray(row[`assignee_ids`])
		if parseErr != nil {
			return nil, parseErr
		}
		items = append(items, transferItem{Id: cast.ToInt(row[`id`]), ParentId: cast.ToInt(row[`parent_id`]), Type: row[`type`], StatusId: cast.ToInt(row[`status_id`]), PriorityId: cast.ToInt(row[`priority`]), VersionId: cast.ToInt(row[`version_id`]), AssigneeIds: assignees})
	}
	return items, nil
}

func mapNamedDefinition(tx *sql.Tx, table string, adminId, sourceWsId, targetWsId int, sourceType, targetType string, sourceId int, required bool) (int, error) {
	if sourceId > 0 {
		rows, err := msql.RawValues(define.Postgres, fmt.Sprintf(`SELECT name,color FROM %s WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND id=$4`, table), tx, adminId, sourceWsId, sourceType, sourceId)
		if err != nil {
			return 0, err
		}
		if len(rows) > 0 {
			target, queryErr := msql.RawValues(define.Postgres, fmt.Sprintf(`SELECT id FROM %s WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND name=$4 AND color=$5 ORDER BY id LIMIT 1`, table), tx, adminId, targetWsId, targetType, rows[0][`name`], rows[0][`color`])
			if queryErr != nil {
				return 0, queryErr
			}
			if len(target) > 0 {
				return cast.ToInt(target[0][`id`]), nil
			}
		}
	}
	if !required {
		return 0, nil
	}
	settingsField := `requirement_initial_status_id`
	if targetType == define.TypeKeyDefect {
		settingsField = `defect_initial_status_id`
	}
	rows, err := msql.RawValues(define.Postgres, fmt.Sprintf(`SELECT %s AS id FROM workspaces_settings WHERE admin_id=$1 AND workspace_id=$2`, settingsField), tx, adminId, targetWsId)
	if err != nil {
		return 0, err
	}
	if len(rows) > 0 && cast.ToInt(rows[0][`id`]) > 0 {
		return cast.ToInt(rows[0][`id`]), nil
	}
	rows, err = msql.RawValues(define.Postgres, `SELECT id FROM workspace_statuses WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 ORDER BY is_default DESC,sort_order,id LIMIT 1`, tx, adminId, targetWsId, targetType)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, ErrStatusNotFound
	}
	return cast.ToInt(rows[0][`id`]), nil
}

func listWorkItemPermissionProjects(adminId, userId int, permissionKey string) ([]WorkItemActionTargetModel, error) {
	var rows []msql.Params
	var err error
	if userId == adminId {
		rows, err = msql.RawValues(define.Postgres, `SELECT id,name FROM workspaces
			WHERE admin_id=$1 AND COALESCE(ended_at,0)=0 ORDER BY sort_order,id`, nil, adminId)
	} else {
		rows, err = msql.RawValues(define.Postgres, `SELECT DISTINCT w.id,w.name,w.sort_order FROM workspace_members wm
			JOIN role_permissions rp ON rp.admin_id=wm.admin_id AND rp.role_id=wm.role_id
			JOIN workspaces w ON w.admin_id=wm.admin_id AND w.id=wm.workspace_id
			WHERE wm.admin_id=$1 AND wm.user_id=$2 AND rp.permission_key=$3 AND COALESCE(w.ended_at,0)=0
			ORDER BY w.sort_order,w.id`, nil, adminId, userId, permissionKey)
	}
	if err != nil {
		return nil, err
	}
	items := make([]WorkItemActionTargetModel, 0, len(rows))
	for _, row := range rows {
		id, convErr := strconv.Atoi(row[`id`])
		if convErr != nil {
			return nil, fmt.Errorf("parse work item permission project id: %w", convErr)
		}
		items = append(items, WorkItemActionTargetModel{Id: id, Name: row[`name`]})
	}
	return items, nil
}

func targetWorkspaceMemberSet(tx *sql.Tx, adminId, workspaceId int) (map[int]bool, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT user_id FROM workspace_members WHERE admin_id=$1 AND workspace_id=$2`, tx, adminId, workspaceId)
	if err != nil {
		return nil, err
	}
	result := make(map[int]bool, len(rows)+1)
	result[adminId] = true
	for _, row := range rows {
		result[cast.ToInt(row[`user_id`])] = true
	}
	return result, nil
}

func filterTransferAssignees(values []int, allowed map[int]bool) []int {
	result := make([]int, 0, len(values))
	for _, value := range values {
		if allowed[value] {
			result = append(result, value)
		}
	}
	return result
}

func transferParentID(item, root transferItem, itemSet map[int]bool, model TransferWorkItemModel) int {
	if model.Convert && root.Type == define.TypeKeyRequirement { // A converted requirement tree becomes independent defects.
		if item.Id == root.Id && model.SourceWorkspaceId == model.TargetWorkspaceId && !itemSet[item.ParentId] {
			return item.ParentId
		}
		return 0
	}
	if model.Convert && root.Type == define.TypeKeyDefect { // A defect can retain its related requirement only in the same project.
		if model.SourceWorkspaceId == model.TargetWorkspaceId {
			return item.ParentId
		}
		return 0
	}
	if model.SourceWorkspaceId == model.TargetWorkspaceId {
		return item.ParentId
	}
	if item.Id != root.Id && itemSet[item.ParentId] {
		return item.ParentId
	}
	return 0
}

func loadTransferFields(tx *sql.Tx, adminId, workspaceId int, itemType string) (map[int]transferField, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id,name,field_type,config_json FROM workspace_custom_fields
		WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND is_enabled=1`, tx, adminId, workspaceId, itemType)
	if err != nil {
		return nil, err
	}
	result := make(map[int]transferField, len(rows))
	for _, row := range rows {
		config := CustomFieldConfig{}
		if raw := row[`config_json`]; raw != `` {
			if err = json.Unmarshal([]byte(raw), &config); err != nil {
				return nil, err
			}
		}
		id := cast.ToInt(row[`id`])
		result[id] = transferField{Id: id, Name: strings.TrimSpace(row[`name`]), FieldType: row[`field_type`], Options: config.Options}
	}
	return result, nil
}

func loadTransferBuiltinConfigs(tx *sql.Tx, adminId, workspaceId int, itemType string) (map[string]transferBuiltinConfig, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT field_key,is_enabled,is_editable FROM workspace_field_configs
		WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3`, tx, adminId, workspaceId, itemType)
	if err != nil {
		return nil, err
	}
	result := make(map[string]transferBuiltinConfig, len(rows))
	for _, row := range rows {
		result[row[`field_key`]] = transferBuiltinConfig{Enabled: cast.ToInt(row[`is_enabled`]), Editable: cast.ToInt(row[`is_editable`])}
	}
	return result, nil
}

func matchingBuiltinConfig(key string, source, target map[string]transferBuiltinConfig) bool {
	sourceConfig, sourceExists := source[key]
	targetConfig, targetExists := target[key]
	return sourceExists && targetExists && sourceConfig == targetConfig && targetConfig.Enabled == 1
}

func remapCustomFieldValues(tx *sql.Tx, adminId, requirementId int, sourceFields, targetFields map[int]transferField, targetMemberIds map[int]bool, now int) error {
	rows, err := msql.RawValues(define.Postgres, `SELECT field_id,value_text FROM requirement_custom_field_values WHERE admin_id=$1 AND requirement_id=$2 FOR UPDATE`, tx, adminId, requirementId)
	if err != nil {
		return err
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM requirement_custom_field_values WHERE admin_id=$1 AND requirement_id=$2`, tx, adminId, requirementId); err != nil {
		return err
	}
	for _, row := range rows {
		source, ok := sourceFields[cast.ToInt(row[`field_id`])]
		if !ok {
			continue
		}
		target, optionMap, ok := matchingTransferField(source, targetFields)
		if !ok {
			continue
		}
		value, ok := remapTransferFieldValue(source, row[`value_text`], optionMap, targetMemberIds)
		if !ok || value == `` || value == `[]` {
			continue
		}
		if _, err = msql.RawExec(define.Postgres, `INSERT INTO requirement_custom_field_values(admin_id,requirement_id,field_id,value_text,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$5)`, tx, adminId, requirementId, target.Id, value, now); err != nil {
			return err
		}
	}
	return nil
}

func matchingTransferField(source transferField, targets map[int]transferField) (transferField, map[string]string, bool) {
	for _, target := range targets {
		if !strings.EqualFold(source.Name, target.Name) || source.FieldType != target.FieldType {
			continue
		}
		optionMap := map[string]string{}
		if source.FieldType == `select_single` || source.FieldType == `select_multi` {
			if len(source.Options) != len(target.Options) {
				continue
			}
			targetByName := map[string]CustomFieldOption{}
			for _, option := range target.Options {
				targetByName[strings.ToLower(strings.TrimSpace(option.Name))+`\x00`+option.Color] = option
			}
			matched := true
			for _, option := range source.Options {
				targetOption, exists := targetByName[strings.ToLower(strings.TrimSpace(option.Name))+`\x00`+option.Color]
				if !exists {
					matched = false
					break
				}
				optionMap[option.Key] = targetOption.Key
			}
			if !matched {
				continue
			}
		}
		return target, optionMap, true
	}
	return transferField{}, nil, false
}

func remapTransferFieldValue(field transferField, raw string, optionMap map[string]string, targetMemberIds map[int]bool) (string, bool) {
	if field.FieldType == `select_single` {
		value, ok := optionMap[raw]
		return value, ok
	}
	if field.FieldType == `select_multi` {
		var values []string
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			return ``, false
		}
		mapped := make([]string, 0, len(values))
		for _, value := range values {
			target, ok := optionMap[value]
			if !ok {
				return ``, false
			}
			mapped = append(mapped, target)
		}
		encoded, err := json.Marshal(mapped)
		return string(encoded), err == nil
	}
	if field.FieldType == `user` {
		userId := cast.ToInt(raw)
		return raw, userId > 0 && targetMemberIds[userId]
	}
	return raw, true
}

func appendMergedDescription(target, source msql.Params) string {
	title := source[`title`]
	description := source[`description`]
	if target[`description_mode`] == `markdown` {
		return strings.TrimSpace(target[`description`]) + "\n\n---\n\n## " + title + "\n\n" + description
	}
	return strings.TrimSpace(target[`description`]) + `<hr><p><strong>` + html.EscapeString(title) + `</strong></p>` + description
}

// Stable ordering makes action-target responses deterministic in tests and clients.
func sortMergeTargets(items []WorkItemMergeTargetModel) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].WorkspaceId == items[j].WorkspaceId {
			return items[i].Id > items[j].Id
		}
		return items[i].WorkspaceId < items[j].WorkspaceId
	})
}

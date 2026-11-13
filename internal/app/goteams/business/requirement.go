// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/permission"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_storage"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

var (
	// ErrInvalidStatusTransition indicates the status transition does not conform to the workflow configuration.
	ErrInvalidStatusTransition = errors.New("invalid status transition")
	// ErrPriorityNotFound indicates the priority does not exist or does not belong to the current workspace type.
	ErrPriorityNotFound = errors.New("priority not found")
	// ErrStatusNotFound indicates the status does not exist or does not belong to the current workspace type.
	ErrStatusNotFound = errors.New("status not found")
	// ErrRequirementNotFound indicates the requirement does not exist or does not belong to the current workspace.
	ErrRequirementNotFound = errors.New("requirement not found")
	// ErrRequirementRelationInvalid indicates the parent or associated requirement relation is invalid.
	ErrRequirementRelationInvalid = errors.New("invalid requirement relation")
	// ErrRequirementAssigneesInvalid indicates that assignees are duplicated, invalid, or outside the workspace.
	ErrRequirementAssigneesInvalid = errors.New("invalid requirement assignees")
	// ErrRequirementHistoryNotFound indicates the change record does not exist within the work item scope.
	ErrRequirementHistoryNotFound = errors.New("requirement history not found")
	// ErrRequirementHistoryNotRestorable indicates the change record carries no restorable description version.
	ErrRequirementHistoryNotRestorable = errors.New("requirement history not restorable")
)

// validateRequirementAssignees normalises assignee ids and checks them against the workspace
// member list. keptIds are the assignees already stored on the work item: any of them that is no
// longer an active project member stays valid so historical work items remain editable, while
// every other id must be an active member — a departed member can never be newly assigned.
// A member can depart in two ways, and both must keep the item editable: removed from the
// enterprise (users row soft-deleted) or dropped from this project only (users row untouched).
func validateRequirementAssignees(adminId int, wsId string, assigneeIds []int, keptIds []int) ([]int, error) {
	normalized := make([]int, 0, len(assigneeIds))
	seen := make(map[int]bool, len(assigneeIds))
	for _, id := range assigneeIds {
		if id <= 0 || seen[id] {
			return nil, ErrRequirementAssigneesInvalid
		}
		seen[id] = true
		normalized = append(normalized, id)
	}
	if len(normalized) == 0 {
		return normalized, nil
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT wm.user_id FROM workspace_members wm
		JOIN users u ON u.id = wm.user_id AND u.admin_id = wm.admin_id AND COALESCE(u.status, 1) = 1
		WHERE wm.admin_id = $1 AND wm.workspace_id = $2 AND wm.user_id = ANY($3)`, nil,
		adminId, cast.ToInt(wsId), pq.Array(normalized))
	if err != nil {
		return nil, err
	}
	if len(rows) == len(normalized) {
		return normalized, nil
	}

	active := make(map[int]bool, len(rows))
	for _, row := range rows {
		active[cast.ToInt(row[`user_id`])] = true
	}
	kept := make(map[int]bool, len(keptIds))
	for _, id := range keptIds {
		kept[id] = true
	}
	missing := make([]int, 0, len(normalized)-len(rows))
	for _, id := range normalized {
		if active[id] {
			continue
		}
		if !kept[id] {
			return nil, ErrRequirementAssigneesInvalid
		}
		missing = append(missing, id)
	}
	keptTenantUsers, err := tenantUserIds(adminId, missing)
	if err != nil {
		return nil, err
	}
	for _, id := range missing {
		if !keptTenantUsers[id] {
			return nil, ErrRequirementAssigneesInvalid
		}
	}
	return normalized, nil
}

func assigneeIDsText(ids []int) string {
	parts := make([]string, len(ids))
	for index, id := range ids {
		parts[index] = cast.ToString(id)
	}
	return strings.Join(parts, `,`)
}

func normalizeStoredAssigneeIDs(value string) string {
	value = strings.Trim(strings.TrimSpace(value), `[]{}`)
	return strings.ReplaceAll(value, ` `, ``)
}

var requirementUpdatableFields = map[string]bool{
	`title`: true, `description`: true, `description_mode`: true, `priority`: true, `status_id`: true,
	`parent_id`: true, `assignee_ids`: true, `version_id`: true,
	`planned_start_date`: true, `planned_end_date`: true,
	`work_hours`: true,
}

var requirementSortExpressions = map[string]string{
	`id`:                   `r.id`,
	`status`:               `s.sort_order`,
	`priority`:             `p.sort_order`,
	`version`:              `LOWER(NULLIF(v.name, ''))`,
	`planned_start_date`:   `NULLIF(r.planned_start_date, 0)`,
	`planned_end_date`:     `NULLIF(r.planned_end_date, 0)`,
	`assignee`:             `LOWER(NULLIF(_assignee.first_assignee_name, ''))`,
	`created_by`:           `LOWER(NULLIF(u2.name, ''))`,
	`created_at`:           `r.created_at`,
	`updated_at`:           `r.updated_at`,
	`estimated_work_hours`: `r.work_hours`,
	`completed_work_hours`: `r.completed_work_hours`,
	`remaining_work_hours`: `GREATEST(r.work_hours - r.completed_work_hours, 0)`,
	`overtime_work_hours`:  `GREATEST(r.completed_work_hours - r.work_hours, 0)`,
	`work_progress`:        `CASE WHEN r.work_hours > 0 THEN LEAST(ROUND(r.completed_work_hours * 100 / r.work_hours), 100) ELSE 0 END`,
}

// IsRequirementSortField reports whether field is supported by the requirement list API.
func IsRequirementSortField(field string) bool {
	_, ok := requirementSortExpressions[field]
	return ok
}

func requirementOrderSQL(field, direction string) string {
	expression, ok := requirementSortExpressions[field]
	if !ok {
		return `r.id DESC`
	}
	directionSQL := `ASC`
	if strings.EqualFold(direction, `desc`) {
		directionSQL = `DESC`
	}
	if field == `id` {
		return fmt.Sprintf(`r.id %s`, directionSQL)
	}
	return fmt.Sprintf(`%s %s NULLS LAST, r.id %s`, expression, directionSQL, directionSQL)
}

// RequirementModel is the business data of a requirement/defect list item.
type RequirementModel struct {
	Id                 int
	WorkspaceId        int
	ParentId           int
	CategoryId         int
	Title              string
	Description        string // Returned by list query, used for Excel export
	Type               string
	Priority           int
	StatusId           int
	TemplateId         int
	TemplateName       string
	CreatedBy          int
	VersionId          int
	PlannedStartDate   int64
	PlannedEndDate     int64
	EstimatedWorkHours float64
	CompletedWorkHours float64
	RemainingWorkHours float64
	OvertimeWorkHours  float64
	WorkProgress       int
	CreatedAt          int64
	UpdatedAt          int64
	StatusName         string // DB stores an i18n key, translated by the controller per request language
	StatusColor        string
	AgingStatus        int    // Derived overall aging conclusion: 0=normal, 1=due soon, 2=overdue; the worst span wins, maintained by the SLA worker.
	PriorityName       string // DB stores an i18n key, translated by the controller per request language
	PriorityColor      string
	CreatorName        string
	WorkspaceName      string
	VersionName        string
	ParentTitle        string
	AssigneeIds        []int
	AssigneeNames      []string
	AssigneeAvatars    []string
	AssigneeUsernames  []string
	HasChildren        bool
	CustomFields       CustomFieldValues
}

// ListRequirementsModel is the input/output model for the requirement/defect list use case.
type ListRequirementsModel struct {
	AdminId     int
	WorkspaceId int
	UserId      int // Current requester; used to filter the list by the project role's data view scope.
	// Filter conditions: a zero numeric value or an empty string means the condition is not applied.
	StatusId             int
	StatusIds            []int
	Priority             int
	PriorityIds          []int
	Type                 string
	CategoryIds          []int // nil means all categories; {0} means uncategorized.
	AssigneeIds          []int
	VersionId            int
	VersionIds           []int
	CreatedBy            int
	CreatedByIds         []int
	ParentId             *int // nil means no parent filtering; pointing to 0 means only root nodes
	Search               string
	TitleSearch          string
	SearchFilters        string // Flat search-bar filter tree, ANDed with legacy params and the selected view.
	Timezone             string // IANA timezone used to compare timestamp columns by calendar date.
	ExcludeDescendantsOf int
	PlanningId           int // Only return work items already linked to this planning
	SortField            string
	SortDirection        string
	ViewId               int    // Apply the saved view's filter tree on top of other filters
	ContextType          string // iteration 跨项目查询；gantt 限定甘特图视图，均由服务端设置。
	Page                 int
	Size                 int
	// Query results.
	Items []RequirementModel
	Total int
}

// parseRequirementInt64 reads an integer field from a list row; empty values are treated as 0 and invalid values error.
func parseRequirementInt64(row msql.Params, column string) (int64, error) {
	value := row[column]
	if len(value) == 0 {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf(`parse requirement %s: %w`, column, err)
	}
	return parsed, nil
}

func parseRequirementFloat64(row msql.Params, column string) (float64, error) {
	value := row[column]
	if len(value) == 0 {
		return 0, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf(`parse requirement %s: %w`, column, err)
	}
	return parsed, nil
}

// toRequirementModel converts a list query row into a business model; any key field parse failure fails the whole row.
func toRequirementModel(row msql.Params) (RequirementModel, error) {
	model := RequirementModel{
		Title:         row[`title`],
		Description:   row[`description`],
		Type:          row[`type`],
		TemplateName:  row[`template_name`],
		StatusName:    row[`status_name`],
		StatusColor:   row[`status_color`],
		PriorityName:  row[`priority_name`],
		PriorityColor: row[`priority_color`],
		CreatorName:   row[`creator_name`],
		WorkspaceName: row[`workspace_name`],
		VersionName:   row[`version_name`],
		ParentTitle:   row[`parent_title`],
		CustomFields:  CustomFieldValues{},
	}
	// Accumulate the first parse error to avoid repeating a three-line error branch per numeric field.
	var parseErr error
	number := func(column string) int64 {
		if parseErr != nil {
			return 0
		}
		value, err := parseRequirementInt64(row, column)
		if err != nil {
			parseErr = err
		}
		return value
	}
	model.Id = int(number(`id`))
	model.WorkspaceId = int(number(`workspace_id`))
	model.ParentId = int(number(`parent_id`))
	model.CategoryId = int(number(`category_id`))
	model.Priority = int(number(`priority`))
	model.StatusId = int(number(`status_id`))
	model.TemplateId = int(number(`template_id`))
	model.CreatedBy = int(number(`created_by`))
	model.VersionId = int(number(`version_id`))
	model.PlannedStartDate = number(`planned_start_date`)
	model.PlannedEndDate = number(`planned_end_date`)
	model.CreatedAt = number(`created_at`)
	model.UpdatedAt = number(`updated_at`)
	model.AgingStatus = int(number(`aging_status`))
	if parseErr != nil {
		return RequirementModel{}, parseErr
	}
	estimatedHours, err := parseRequirementFloat64(row, `work_hours`)
	if err != nil {
		return RequirementModel{}, err
	}
	completedHours, err := parseRequirementFloat64(row, `completed_work_hours`)
	if err != nil {
		return RequirementModel{}, err
	}
	summary := CalculateRequirementWorkHours(estimatedHours, completedHours)
	model.EstimatedWorkHours = summary.Estimated
	model.CompletedWorkHours = summary.Completed
	model.RemainingWorkHours = summary.Remaining
	model.OvertimeWorkHours = summary.Overtime
	model.WorkProgress = summary.Progress
	// The assignee field is aggregated by json_agg; the SQL already falls back to '[]' with COALESCE.
	if err := json.Unmarshal([]byte(row[`assignee_ids`]), &model.AssigneeIds); err != nil {
		return RequirementModel{}, fmt.Errorf(`parse requirement assignee_ids: %w`, err)
	}
	for _, field := range []struct {
		column string
		target *[]string
	}{
		{`assignee_names`, &model.AssigneeNames},
		{`assignee_avatars`, &model.AssigneeAvatars},
		{`assignee_usernames`, &model.AssigneeUsernames},
	} {
		if err := json.Unmarshal([]byte(row[field.column]), field.target); err != nil {
			return RequirementModel{}, fmt.Errorf(`parse requirement %s: %w`, field.column, err)
		}
	}
	return model, nil
}

func buildIterationVisibilityCondition(model ListRequirementsModel, nextArgIndex int) (string, []any, []int, error) {
	accesses, err := listIterationWorkspaceAccesses(model.AdminId, model.UserId, model.WorkspaceId, model.VersionId)
	if err != nil {
		return ``, nil, nil, err
	}
	parts := make([]string, 0, len(accesses))
	args := make([]any, 0)
	workspaceIds := make([]int, 0, len(accesses))
	for _, access := range accesses {
		types := make([]string, 0, 2)
		if access.ViewRequirement {
			types = append(types, define.TypeKeyRequirement)
		}
		if access.ViewDefect {
			types = append(types, define.TypeKeyDefect)
		}
		if len(types) == 0 {
			continue
		}
		workspaceIds = append(workspaceIds, access.Workspace.Id)
		workspaceArg := nextArgIndex + len(args)
		args = append(args, access.Workspace.Id)
		typesArg := nextArgIndex + len(args)
		args = append(args, pq.Array(types))
		conditions := []string{
			fmt.Sprintf(`r.workspace_id = $%d`, workspaceArg),
			fmt.Sprintf(`r.type = ANY($%d)`, typesArg),
		}
		if model.UserId > 0 && model.UserId != model.AdminId {
			scopeCondition, scopeArgs, scopeErr := buildDataViewScopeCondition(
				model.AdminId, cast.ToString(access.Workspace.Id), model.UserId, nextArgIndex+len(args),
			)
			if scopeErr != nil {
				return ``, nil, nil, scopeErr
			}
			if scopeCondition != `` {
				args = append(args, scopeArgs...)
				conditions = append(conditions, scopeCondition)
			}
		}
		parts = append(parts, `(`+strings.Join(conditions, ` AND `)+`)`)
	}
	if len(parts) == 0 {
		return `FALSE`, args, workspaceIds, nil
	}
	return `(` + strings.Join(parts, ` OR `) + `)`, args, workspaceIds, nil
}

func requirementListViewTypeMatches(viewType, itemType, contextType string) bool {
	if viewType == define.TypeKeyGantt {
		return itemType == `` || itemType == define.TypeKeyRequirement || itemType == define.TypeKeyDefect
	}
	return contextType != define.TypeKeyGantt && viewType == itemType
}

// ListRequirements returns the requirement/defect list (admin_id isolated, with pagination and filtering, joining status, priority, version, and assignee).
func ListRequirements(model ListRequirementsModel) (ListRequirementsModel, error) {
	if model.Page < 1 {
		model.Page = 1
	}
	if model.Size < 1 || model.Size > 100 {
		model.Size = 20
	}
	orderSQL := requirementOrderSQL(model.SortField, model.SortDirection)
	iterationContext := model.ContextType == define.TypeKeyIteration
	where := []string{`r.admin_id = $1`}
	args := []any{model.AdminId}
	filterWorkspaceIds := []int{model.WorkspaceId}
	if iterationContext {
		if model.VersionId <= 0 {
			return ListRequirementsModel{}, ErrVersionNotFound
		}
		visibilityCondition, visibilityArgs, workspaceIds, err := buildIterationVisibilityCondition(model, len(args)+1)
		if err != nil {
			return ListRequirementsModel{}, err
		}
		args = append(args, visibilityArgs...)
		where = append(where, visibilityCondition)
		filterWorkspaceIds = workspaceIds
	} else {
		args = append(args, model.WorkspaceId)
		where = append(where, `r.workspace_id = $2`)
	}
	filterTypeKey := model.Type
	if iterationContext {
		filterTypeKey = define.TypeKeyIteration
	}
	addFilter := func(column string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(`%s = $%d`, column, len(args)))
	}
	if model.StatusId > 0 {
		addFilter(`r.status_id`, model.StatusId)
	}
	if len(model.StatusIds) > 0 {
		args = append(args, pq.Array(model.StatusIds))
		where = append(where, fmt.Sprintf(`r.status_id = ANY($%d)`, len(args)))
	}
	if model.Priority > 0 {
		addFilter(`r.priority`, model.Priority)
	}
	if len(model.PriorityIds) > 0 {
		args = append(args, pq.Array(model.PriorityIds))
		where = append(where, fmt.Sprintf(`COALESCE(r.priority, 0) = ANY($%d)`, len(args)))
	}
	if len(model.Type) > 0 {
		addFilter(`r.type`, model.Type)
	} else {
		// When the type is not specified, only return requirements and defects; planning has a separate list API and must not be mixed in.
		args = append(args, define.TypeKeyRequirement, define.TypeKeyDefect)
		where = append(where, fmt.Sprintf(`r.type IN ($%d, $%d)`, len(args)-1, len(args)))
	}
	if model.CategoryIds != nil {
		args = append(args, pq.Array(model.CategoryIds))
		where = append(where, fmt.Sprintf(`r.category_id = ANY($%d)`, len(args)))
	}
	if len(model.AssigneeIds) > 0 {
		args = append(args, pq.Array(model.AssigneeIds))
		where = append(where, fmt.Sprintf(`r.assignee_ids && $%d`, len(args)))
	}
	if model.VersionId > 0 {
		addFilter(`r.version_id`, model.VersionId)
	}
	if len(model.VersionIds) > 0 {
		args = append(args, pq.Array(model.VersionIds))
		where = append(where, fmt.Sprintf(`COALESCE(r.version_id, 0) = ANY($%d)`, len(args)))
	}
	if model.CreatedBy > 0 {
		addFilter(`r.created_by`, model.CreatedBy)
	}
	if len(model.CreatedByIds) > 0 {
		args = append(args, pq.Array(model.CreatedByIds))
		where = append(where, fmt.Sprintf(`r.created_by = ANY($%d)`, len(args)))
	}
	if model.ParentId != nil {
		addFilter(`r.parent_id`, *model.ParentId)
	}
	if search := strings.TrimSpace(model.Search); len(search) > 0 {
		args = append(args, `%`+search+`%`)
		where = append(where, fmt.Sprintf(`(CAST(r.id AS TEXT) ILIKE $%d OR r.title ILIKE $%d)`, len(args), len(args)))
	}
	for _, keyword := range strings.Fields(model.TitleSearch) {
		args = append(args, `%`+keyword+`%`)
		where = append(where, fmt.Sprintf(`r.title ILIKE $%d`, len(args)))
	}
	if strings.TrimSpace(model.SearchFilters) != `` {
		searchCondition, searchArgs, searchErr := buildViewFilterCondition(model.AdminId, cast.ToString(model.WorkspaceId), filterTypeKey, model.UserId, filterWorkspaceIds, model.SearchFilters, model.Timezone, len(args)+1)
		if searchErr != nil {
			return ListRequirementsModel{}, searchErr
		}
		if searchCondition != `` {
			args = append(args, searchArgs...)
			where = append(where, searchCondition)
		}
	}
	if model.ExcludeDescendantsOf > 0 {
		args = append(args, model.ExcludeDescendantsOf)
		where = append(where, fmt.Sprintf(`r.id NOT IN (
			WITH RECURSIVE descendants(id, path) AS (
				SELECT id, ARRAY[id] FROM requirements WHERE admin_id = $1 AND workspace_id = $2 AND id = $%d
				UNION ALL
				SELECT child.id, descendants.path || child.id FROM requirements child
				JOIN descendants ON child.parent_id = descendants.id
				WHERE child.admin_id = $1 AND child.workspace_id = $2 AND NOT child.id = ANY(descendants.path)
			) SELECT id FROM descendants
		)`, len(args)))
	}
	// Data view scope filter: non-owner users only see what they created plus what their project
	// role scope covers (matched on assignees). The iteration context resolves visibility through
	// buildIterationVisibilityCondition instead, so it must not be narrowed again here.
	if !iterationContext && model.UserId > 0 && !IsTenantOwner(model.AdminId, model.UserId) {
		scopeCondition, scopeArgs, scopeErr := buildDataViewScopeCondition(model.AdminId, cast.ToString(model.WorkspaceId), model.UserId, len(args)+1)
		if scopeErr != nil {
			return ListRequirementsModel{}, scopeErr
		}
		if scopeCondition != `` {
			args = append(args, scopeArgs...)
			where = append(where, scopeCondition)
		}
	}
	// View filtering: apply the saved view's filter tree on top of the other filters.
	// System views apply to every member; personal views only work for their creator.
	if model.ViewId > 0 {
		view, viewErr := GetWorkItemViewById(model.AdminId, cast.ToString(model.WorkspaceId), model.ViewId)
		if viewErr != nil {
			return ListRequirementsModel{}, viewErr
		}
		// Gantt views span both work item types: they accept the mixed-type query (empty
		// type) as well as an explicit requirement/defect filter layered on top. Other
		// views must match the queried type exactly.
		typeMatches := requirementListViewTypeMatches(view.TypeKey, filterTypeKey, model.ContextType)
		if view.Id == 0 || !typeMatches || (view.ViewType != ViewTypeSystem && view.CreatedBy != model.UserId) {
			return ListRequirementsModel{}, ErrViewNotFound
		}
		var viewCondition string
		var viewArgs []any
		if view.TypeKey == define.TypeKeyGantt {
			viewCondition, viewArgs, viewErr = buildGanttViewFilterCondition(model.AdminId, cast.ToString(model.WorkspaceId), model.UserId, filterWorkspaceIds, view.Filters, model.Timezone, len(args)+1)
		} else {
			viewCondition, viewArgs, viewErr = buildViewFilterCondition(model.AdminId, cast.ToString(model.WorkspaceId), filterTypeKey, model.UserId, filterWorkspaceIds, view.Filters, model.Timezone, len(args)+1)
		}
		if viewErr != nil {
			return ListRequirementsModel{}, viewErr
		}
		if viewCondition != `` {
			args = append(args, viewArgs...)
			where = append(where, viewCondition)
		}
	}
	whereSQL := strings.Join(where, ` AND `)
	// Planning association filtering is expressed with JOIN to avoid subqueries; the relation table is unique on (planning_id, item_id), so the result set is not inflated.
	fromSQL := `requirements r`
	if model.PlanningId > 0 {
		args = append(args, model.PlanningId)
		fromSQL = fmt.Sprintf(`requirements r JOIN planning_relations pr ON pr.item_id = r.id AND pr.admin_id = r.admin_id AND pr.planning_id = $%d`, len(args))
	}
	countRows, err := msql.RawValues(define.Postgres, `SELECT COUNT(*) AS total FROM `+fromSQL+` WHERE `+whereSQL, nil, args...)
	if err != nil {
		logs.Error("ListRequirements count failed, err:%s", err.Error())
		return ListRequirementsModel{}, err
	}
	if len(countRows) > 0 {
		model.Total = cast.ToInt(countRows[0][`total`])
	}
	listArgs := append([]any{}, args...)
	listArgs = append(listArgs, model.Size, (model.Page-1)*model.Size)
	query := fmt.Sprintf(`SELECT r.id, r.workspace_id, r.parent_id, r.category_id, r.title, r.description, r.type, r.priority, r.status_id, r.created_by, r.template_id, r.template_name,
		COALESCE(_assignee.assignee_ids, '[]') AS assignee_ids, r.version_id, r.planned_start_date, r.planned_end_date,
		r.work_hours, r.completed_work_hours, r.created_at, r.updated_at, r.aging_status,
		s.name AS status_name, s.color AS status_color, p.name AS priority_name,
		p.color AS priority_color, COALESCE(u2.name, ru2.name, '') AS creator_name, w.name AS workspace_name,
		v.name AS version_name, parent.title AS parent_title,
		COALESCE(_assignee.assignee_names, '[]') AS assignee_names,
		COALESCE(_assignee.assignee_avatars, '[]') AS assignee_avatars,
		COALESCE(_assignee.assignee_usernames, '[]') AS assignee_usernames
		FROM `+fromSQL+`
		LEFT JOIN workspace_statuses s ON s.id = r.status_id AND s.admin_id = r.admin_id AND s.workspace_id = r.workspace_id AND s.type_key = r.type
		LEFT JOIN workspace_priorities p ON p.id = r.priority AND p.admin_id = r.admin_id AND p.workspace_id = r.workspace_id AND p.type_key = r.type
		LEFT JOIN users u2 ON u2.id = r.created_by AND u2.admin_id = r.admin_id
		LEFT JOIN removed_users ru2 ON ru2.user_id = r.created_by AND ru2.admin_id = r.admin_id
		LEFT JOIN workspaces w ON w.id = r.workspace_id AND w.admin_id = r.admin_id
		LEFT JOIN versions v ON v.id = r.version_id AND v.admin_id = r.admin_id
		LEFT JOIN requirements parent ON parent.id = r.parent_id AND parent.admin_id = r.admin_id AND parent.workspace_id = r.workspace_id
		LEFT JOIN LATERAL (
			SELECT COALESCE(json_agg(assigned.user_id ORDER BY assigned.ordinality), '[]'::json)::text AS assignee_ids,
				COALESCE(json_agg(COALESCE(u.name, ru.name, '') ORDER BY assigned.ordinality), '[]'::json)::text AS assignee_names,
				COALESCE(json_agg(COALESCE(u.avatar, ru.avatar, '') ORDER BY assigned.ordinality), '[]'::json)::text AS assignee_avatars,
				COALESCE(json_agg(COALESCE(u.username, '') ORDER BY assigned.ordinality), '[]'::json)::text AS assignee_usernames,
				MAX(NULLIF(COALESCE(u.name, ru.name, ''), '')) FILTER (WHERE assigned.ordinality = 1) AS first_assignee_name
			FROM unnest(r.assignee_ids) WITH ORDINALITY AS assigned(user_id, ordinality)
			LEFT JOIN users u ON u.id = assigned.user_id AND u.admin_id = r.admin_id
			LEFT JOIN removed_users ru ON ru.user_id = assigned.user_id AND ru.admin_id = r.admin_id
		) _assignee ON true
		WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d`, whereSQL, orderSQL, len(listArgs)-1, len(listArgs))
	rows, err := msql.RawValues(define.Postgres, query, nil, listArgs...)
	if err != nil {
		logs.Error("ListRequirements query failed, err:%s", err.Error())
		return ListRequirementsModel{}, err
	}
	items := make([]RequirementModel, 0, len(rows))
	for _, row := range rows {
		item, convErr := toRequirementModel(row)
		if convErr != nil {
			logs.Error("convert requirement row failed, err:%s", convErr.Error())
			return ListRequirementsModel{}, convErr
		}
		items = append(items, item)
	}
	if iterationContext {
		itemsByWorkspace := make(map[int][]RequirementModel)
		for _, item := range items {
			itemsByWorkspace[item.WorkspaceId] = append(itemsByWorkspace[item.WorkspaceId], item)
		}
		filledById := make(map[int]RequirementModel, len(items))
		for workspaceId, workspaceItems := range itemsByWorkspace {
			if err = fillRequirementListFields(model.AdminId, workspaceId, model.UserId, workspaceItems); err != nil {
				return ListRequirementsModel{}, err
			}
			for _, item := range workspaceItems {
				filledById[item.Id] = item
			}
		}
		for index := range items {
			items[index] = filledById[items[index].Id]
		}
	} else if err = fillRequirementListFields(model.AdminId, model.WorkspaceId, model.UserId, items); err != nil {
		return ListRequirementsModel{}, err
	}
	model.Items = items
	return model, nil
}

// fillRequirementListFields batch-fills the visible sub-requirement flag and custom field values for list items.
func fillRequirementListFields(adminId, wsId, userId int, list []RequirementModel) error {
	items := make(map[int]*RequirementModel, len(list))
	ids := make([]int, 0, len(list))
	for index := range list {
		item := &list[index]
		if item.Id <= 0 {
			continue
		}
		items[item.Id] = item
		ids = append(ids, item.Id)
	}
	if len(ids) == 0 {
		return nil
	}

	childWhere := []string{`r.admin_id = $1`, `r.workspace_id = $2`, `r.type = 'requirement'`, `r.parent_id = ANY($3)`}
	childArgs := []any{adminId, wsId, pq.Array(ids)}
	scopeCondition, scopeArgs, scopeErr := buildDataViewScopeCondition(adminId, cast.ToString(wsId), userId, len(childArgs)+1)
	if scopeErr != nil {
		return scopeErr
	}
	if scopeCondition != `` {
		childWhere = append(childWhere, scopeCondition)
		childArgs = append(childArgs, scopeArgs...)
	}
	children, err := msql.RawValues(define.Postgres, `SELECT DISTINCT r.parent_id FROM requirements r WHERE `+strings.Join(childWhere, ` AND `), nil, childArgs...)
	if err != nil {
		logs.Error("ListRequirements children query failed, err:%s", err.Error())
		return err
	}
	for _, child := range children {
		if item := items[cast.ToInt(child[`parent_id`])]; item != nil && item.Type == define.TypeKeyRequirement {
			item.HasChildren = true
		}
	}

	rows, err := msql.RawValues(define.Postgres, `SELECT cfv.requirement_id, cfv.field_id, cfv.value_text, cf.type_key
		FROM requirement_custom_field_values cfv
		JOIN workspace_custom_fields cf ON cf.id = cfv.field_id AND cf.admin_id = cfv.admin_id
		WHERE cfv.admin_id = $1 AND cf.workspace_id = $2 AND cf.is_enabled = 1
			AND cfv.requirement_id = ANY($3)`, nil, adminId, wsId, pq.Array(ids))
	if err != nil {
		logs.Error("ListRequirements custom fields query failed, err:%s", err.Error())
		return err
	}
	for _, row := range rows {
		item := items[cast.ToInt(row[`requirement_id`])]
		if item == nil || item.Type != row[`type_key`] {
			continue
		}
		item.CustomFields[row[`field_id`]] = row[`value_text`]
	}
	return nil
}

// Shared SQL fragments of the cross-workspace work-item lists.
// In every query built here $1 is the tenant id and $2 the current user id.
const (
	// workItemCanEditSQL yields 1 when the current user may edit the row's work item.
	workItemCanEditSQL = `CASE WHEN $1::INTEGER = $2::INTEGER OR EXISTS (
			SELECT 1 FROM workspace_members wm
			JOIN role_permissions rp ON rp.admin_id = wm.admin_id AND rp.role_id = wm.role_id
				AND rp.permission_key = (r.type || ':edit')
			WHERE wm.admin_id = $1 AND wm.workspace_id = r.workspace_id AND wm.user_id = $2
		) THEN 1 ELSE 0 END`
	// workItemVisibleSQL keeps only the work items the current user is still allowed to view.
	workItemVisibleSQL = `($1::INTEGER = $2::INTEGER OR EXISTS (
		SELECT 1 FROM workspace_members wm
		JOIN role_permissions rp ON rp.admin_id = wm.admin_id AND rp.role_id = wm.role_id
			AND rp.permission_key = (r.type || ':view')
		WHERE wm.admin_id = $1 AND wm.workspace_id = r.workspace_id AND wm.user_id = $2
	))`
	// recentVisitWindow is the rolling window of the recent-visit list: 7x24 hours.
	recentVisitWindow = 7 * 24 * 3600
)

// buildWorkItemListSQL assembles the cross-workspace work-item list query shared by
// "my work" and "recent visits". The four arguments carry the per-list differences:
// extra select columns, extra joins, the row filter and the ordering.
func buildWorkItemListSQL(extraColumns, extraJoins, filter, orderBy string) string {
	return fmt.Sprintf(`SELECT r.id, r.admin_id, r.workspace_id, r.parent_id, r.title, r.description, r.type,
		r.priority, r.status_id, r.version_id, r.planned_start_date, r.planned_end_date,
		r.work_hours, r.completed_work_hours, r.comment_count, r.created_by, r.created_at, r.updated_at,
		w.name AS workspace_name, w.color AS workspace_color,
		s.name AS status_name, s.color AS status_color, s.sort_order AS status_sort_order,
		p.name AS priority_name, p.color AS priority_color, p.sort_order AS priority_sort_order,
		COALESCE(_assignee.assignee_ids, '[]') AS assignee_ids,
		COALESCE(_assignee.assignee_names, '[]') AS assignee_names,
		COALESCE(_assignee.assignee_avatars, '[]') AS assignee_avatars,
		COALESCE(_assignee.assignee_usernames, '[]') AS assignee_usernames,
		parent.title AS parent_title,
		CASE WHEN r.type = 'defect' THEN COALESCE(r.status_id = ANY(
			string_to_array(NULLIF(settings.defect_final_status_ids, ''), ',')::INTEGER[]), FALSE)
		ELSE COALESCE(r.status_id = ANY(
			string_to_array(NULLIF(settings.requirement_final_status_ids, ''), ',')::INTEGER[]), FALSE)
		END AS is_done,
		CASE WHEN r.type = 'defect' THEN COALESCE(r.status_id = ANY(
			string_to_array(NULLIF(settings.defect_completed_status_ids, ''), ',')::INTEGER[]), FALSE)
		ELSE COALESCE(r.status_id = ANY(
			string_to_array(NULLIF(settings.requirement_completed_status_ids, ''), ',')::INTEGER[]), FALSE)
		END AS is_completed,
		%s AS can_edit%s
		FROM requirements r
		JOIN workspaces w ON w.id = r.workspace_id AND w.admin_id = r.admin_id
		LEFT JOIN workspaces_settings settings ON settings.workspace_id = r.workspace_id AND settings.admin_id = r.admin_id
		LEFT JOIN workspace_statuses s ON s.id = r.status_id AND s.admin_id = r.admin_id
			AND s.workspace_id = r.workspace_id AND s.type_key = r.type
		LEFT JOIN workspace_priorities p ON p.id = r.priority AND p.admin_id = r.admin_id
			AND p.workspace_id = r.workspace_id AND p.type_key = r.type
		LEFT JOIN LATERAL (
			SELECT COALESCE(json_agg(assigned.user_id ORDER BY assigned.ordinality), '[]'::json)::text AS assignee_ids,
				COALESCE(json_agg(COALESCE(u.name, ru.name, '') ORDER BY assigned.ordinality), '[]'::json)::text AS assignee_names,
				COALESCE(json_agg(COALESCE(u.avatar, ru.avatar, '') ORDER BY assigned.ordinality), '[]'::json)::text AS assignee_avatars,
				COALESCE(json_agg(COALESCE(u.username, '') ORDER BY assigned.ordinality), '[]'::json)::text AS assignee_usernames
			FROM unnest(r.assignee_ids) WITH ORDINALITY AS assigned(user_id, ordinality)
			LEFT JOIN users u ON u.id = assigned.user_id AND u.admin_id = r.admin_id
			LEFT JOIN removed_users ru ON ru.user_id = assigned.user_id AND ru.admin_id = r.admin_id
		) _assignee ON true
		LEFT JOIN requirements parent ON parent.id = r.parent_id AND parent.admin_id = r.admin_id
			AND parent.workspace_id = r.workspace_id%s
		WHERE r.admin_id = $1
			AND r.type IN ('requirement', 'defect') AND %s AND %s
		ORDER BY %s`,
		workItemCanEditSQL, extraColumns, extraJoins, workItemVisibleSQL, filter, orderBy)
}

// myWorkOrderBySQL 是「我的工作」的排序口径：计划开始日期优先，其次优先级，最后按 ID 兜底。
// 界面用例与开放 API 用例共用，避免两处各写一份 ORDER BY 而逐渐跑偏。
const myWorkOrderBySQL = `NULLIF(r.planned_start_date, 0) ASC NULLS LAST, COALESCE(p.sort_order, 2147483647) ASC, r.id DESC`

// ListMyWork returns all work items assigned to the current user across workspaces.
func ListMyWork(adminId int, userId int) (MyWorkModel, error) {
	query := buildWorkItemListSQL(``, ``, `r.assignee_ids @> ARRAY[$2]`, myWorkOrderBySQL)
	items, err := msql.RawValues(define.Postgres, query, nil, adminId, userId)
	if err != nil {
		logs.Error("ListMyWork query failed, err:%s", err.Error())
		return MyWorkModel{}, err
	}

	workspaceQuery := `SELECT w.id, w.name, w.color FROM workspaces w
		WHERE w.admin_id = $1 AND ($1::INTEGER = $2::INTEGER OR EXISTS (
			SELECT 1 FROM workspace_members wm
			JOIN role_permissions rp ON rp.admin_id = wm.admin_id AND rp.role_id = wm.role_id
				AND rp.permission_key IN ('requirement:create', 'defect:create')
			WHERE wm.admin_id = $1 AND wm.workspace_id = w.id AND wm.user_id = $2
		)) ORDER BY w.id DESC`
	workspaces, err := msql.RawValues(define.Postgres, workspaceQuery, nil, adminId, userId)
	if err != nil {
		logs.Error("ListMyWork creatable workspaces failed, err:%s", err.Error())
		return MyWorkModel{}, err
	}

	// The recent-visit tab loads its rows lazily; only its badge count ships with this response.
	visitCount, err := CountRecentVisits(adminId, userId)
	if err != nil {
		return MyWorkModel{}, err
	}

	list, err := toCrossWorkspaceWorkItems(items)
	if err != nil {
		return MyWorkModel{}, err
	}
	result := MyWorkModel{Items: list, CreatableWorkspaces: make([]MyWorkWorkspaceModel, 0, len(workspaces)), RecentVisitCount: visitCount}
	for _, row := range workspaces {
		id, err := strconv.Atoi(row["id"])
		if err != nil {
			return MyWorkModel{}, err
		}
		result.CreatableWorkspaces = append(result.CreatableWorkspaces, MyWorkWorkspaceModel{Id: id, Name: row["name"], Color: row["color"]})
	}
	return result, nil
}

// recentVisitJoinSQL joins the visit records of the current user onto the work-item list.
var recentVisitJoinSQL = fmt.Sprintf(`
		JOIN %s v ON v.requirement_id = r.id AND v.admin_id = r.admin_id`, define.TableRequirementVisits)

// ListRecentVisits returns the work items the current user opened within the last 7x24 hours,
// most recently visited first. Work items no longer visible to the
// user are excluded, so the list can never expose data the user lost access to.
func ListRecentVisits(adminId int, userId int) ([]CrossWorkspaceWorkItemModel, error) {
	query := buildWorkItemListSQL(`, v.visited_at`, recentVisitJoinSQL,
		`v.user_id = $2 AND v.visited_at >= $3`, `v.visited_at DESC, r.id DESC`)
	items, err := msql.RawValues(define.Postgres, query, nil, adminId, userId, tool.Time2Int()-recentVisitWindow)
	if err != nil {
		logs.Error("ListRecentVisits query failed, err:%s", err.Error())
		return nil, err
	}
	return toCrossWorkspaceWorkItems(items)
}

// CountRecentVisits counts the recent visits of the current user using the same
// visibility rules as ListRecentVisits, so the tab badge always matches the list.
func CountRecentVisits(adminId int, userId int) (int, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) AS total FROM %s v
		JOIN requirements r ON r.id = v.requirement_id AND r.admin_id = v.admin_id
		JOIN workspaces w ON w.id = r.workspace_id AND w.admin_id = r.admin_id
		WHERE v.admin_id = $1 AND v.user_id = $2 AND v.visited_at >= $3
			AND r.type IN ('requirement', 'defect') AND %s`,
		define.TableRequirementVisits, workItemVisibleSQL)
	rows, err := msql.RawValues(define.Postgres, query, nil, adminId, userId, tool.Time2Int()-recentVisitWindow)
	if err != nil {
		logs.Error("CountRecentVisits query failed, err:%s", err.Error())
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return cast.ToInt(rows[0][`total`]), nil
}

// RecordRequirementVisit stamps the current user's latest visit time on a work item.
// The INSERT ... SELECT doubles as an existence check: a work item that does not belong
// to this tenant and workspace simply inserts no row.
func RecordRequirementVisit(adminId int, userId int, wsId, reqId string) error {
	query := fmt.Sprintf(`INSERT INTO %s (admin_id, user_id, requirement_id, visited_at)
		SELECT $1::INTEGER, $2::INTEGER, r.id, $5::BIGINT FROM requirements r
		WHERE r.admin_id = $1 AND r.workspace_id = $3 AND r.id = $4
			AND r.type IN ('requirement', 'defect')
		ON CONFLICT (admin_id, user_id, requirement_id)
		DO UPDATE SET visited_at = EXCLUDED.visited_at`, define.TableRequirementVisits)
	if _, err := msql.RawExec(define.Postgres, query, nil,
		adminId, userId, cast.ToInt(wsId), cast.ToInt(reqId), tool.Time2Int()); err != nil {
		logs.Error("RecordRequirementVisit exec failed, err:%s", err.Error())
		return err
	}
	return nil
}

// WorkItemTypeModel identifies a work item and its permission domain.
type WorkItemTypeModel struct {
	Id   int
	Type string
}

// ListRequirementTypesByIDs returns the types of the specified work items within a workspace.
func ListRequirementTypesByIDs(adminId int, wsId string, ids []int) ([]WorkItemTypeModel, error) {
	if len(ids) == 0 {
		return []WorkItemTypeModel{}, nil
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT id, type FROM requirements
		WHERE admin_id = $1 AND workspace_id = $2 AND id = ANY($3)`, nil,
		adminId, cast.ToInt(wsId), pq.Array(ids))
	if err != nil {
		logs.Error("ListRequirementTypesByIDs query failed, err:%s", err.Error())
		return nil, err
	}
	items := make([]WorkItemTypeModel, 0, len(rows))
	for _, row := range rows {
		id, parseErr := strconv.Atoi(row[`id`])
		if parseErr != nil {
			return nil, fmt.Errorf("parse work item id: %w", parseErr)
		}
		items = append(items, WorkItemTypeModel{Id: id, Type: row[`type`]})
	}
	return items, nil
}

// GetRequirementById returns requirement details (admin_id isolated, aggregating assignee arrays).
func GetRequirementById(adminId int, wsId, reqId string) (msql.Params, error) {
	return getRequirementById(nil, adminId, wsId, reqId)
}

// getRequirementById reads through the supplied transaction and locks the work item when present.
func getRequirementById(tx *sql.Tx, adminId int, wsId, reqId string) (msql.Params, error) {
	query := fmt.Sprintf(`SELECT r.id, r.admin_id, r.workspace_id, r.parent_id, r.category_id, r.title, r.description, r.description_mode, r.type,
		r.template_id, r.template_name,
		r.priority, r.status_id, r.version_id, r.planned_start_date, r.planned_end_date,
		r.work_hours, r.completed_work_hours, r.comment_count, r.created_by, r.created_at, r.updated_at,
		s.name AS status_name, s.color AS status_color, p.name AS priority_name,
		p.color AS priority_color, COALESCE(u2.name, ru2.name, '') AS creator_name,
		v.name AS version_name, parent.title AS parent_title,
		ir.openid AS invite_openid,
		COALESCE(_assignee.assignee_ids, '[]') AS assignee_ids,
		COALESCE(_assignee.assignee_names, '[]') AS assignee_names,
		COALESCE(_assignee.assignee_avatars, '[]') AS assignee_avatars,
		COALESCE(_assignee.assignee_usernames, '[]') AS assignee_usernames
		FROM requirements r
		LEFT JOIN workspace_statuses s ON s.id = r.status_id AND s.admin_id = r.admin_id
			AND s.workspace_id = r.workspace_id AND s.type_key = r.type
		LEFT JOIN workspace_priorities p ON p.id = r.priority AND p.admin_id = r.admin_id
			AND p.workspace_id = r.workspace_id AND p.type_key = r.type
		LEFT JOIN users u2 ON u2.id = r.created_by
		LEFT JOIN removed_users ru2 ON ru2.user_id = r.created_by AND ru2.admin_id = r.admin_id
		LEFT JOIN versions v ON v.id = r.version_id
		LEFT JOIN requirements parent ON parent.id = r.parent_id AND parent.admin_id = r.admin_id
			AND parent.workspace_id = r.workspace_id
		LEFT JOIN ` + define.TableWorkspaceIssueRequirements + ` ir ON ir.requirement_id = r.id
			AND ir.admin_id = r.admin_id AND ir.workspace_id = r.workspace_id
		LEFT JOIN LATERAL (
			SELECT COALESCE(json_agg(assigned.user_id ORDER BY assigned.ordinality), '[]'::json)::text AS assignee_ids,
				COALESCE(json_agg(COALESCE(u.name, ru.name, '') ORDER BY assigned.ordinality), '[]'::json)::text AS assignee_names,
				COALESCE(json_agg(COALESCE(u.avatar, ru.avatar, '') ORDER BY assigned.ordinality), '[]'::json)::text AS assignee_avatars,
				COALESCE(json_agg(COALESCE(u.username, '') ORDER BY assigned.ordinality), '[]'::json)::text AS assignee_usernames
			FROM unnest(r.assignee_ids) WITH ORDINALITY AS assigned(user_id, ordinality)
			LEFT JOIN users u ON u.id = assigned.user_id AND u.admin_id = r.admin_id
			LEFT JOIN removed_users ru ON ru.user_id = assigned.user_id AND ru.admin_id = r.admin_id
		) _assignee ON true
		WHERE r.admin_id = $1 AND r.workspace_id = $2 AND r.id = $3 LIMIT 1`)
	if tx != nil {
		query += ` FOR UPDATE OF r`
	}
	rows, err := msql.RawValues(define.Postgres, query, tx, adminId, cast.ToInt(wsId), cast.ToInt(reqId))
	if err != nil {
		logs.Error("GetRequirementById query failed, err:%s", err.Error())
		return nil, err
	}
	if len(rows) == 0 {
		return msql.Params{}, nil
	}
	data := rows[0]
	if data[`status_name`] == `` {
		data[`status_name`] = ``
	}
	if data[`priority_name`] == `` {
		data[`priority_name`] = ``
	}
	return data, nil
}

// listRoleDataViewMemberIds returns the member user IDs configured for a role's partial data view scope.
// Shared by the project role config page (ListProjectRoles) and the list-filter scope resolution.
func listRoleDataViewMemberIds(adminId int, roleId int) ([]int, error) {
	m := msql.Model(define.TableRoleDataViewMembers, define.Postgres)
	list, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`role_id`, cast.ToString(roleId)},
	}).Field(`user_id`).Order(`user_id ASC`).Select()
	if err != nil {
		logs.Error("list role data view members error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	result := make([]int, 0, len(list))
	for _, row := range list {
		result = append(result, cast.ToInt(row[`user_id`]))
	}
	return result, nil
}

// getUserProjectDataViewScope returns the list data view scope of the user's project role
// within the workspace, together with the partial-scope member IDs.
// all / partial / self; users without a project role default to self.
func getUserProjectDataViewScope(adminId int, wsId string, userId int) (string, []int, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT r.id, r.data_view_scope
		FROM workspace_members wm
		JOIN roles r ON r.id = wm.role_id AND r.admin_id = wm.admin_id AND r.scope = $3
		WHERE wm.admin_id = $1 AND wm.workspace_id = $2 AND wm.user_id = $4`, nil,
		adminId, cast.ToInt(wsId), permission.ScopeProject, userId)
	if err != nil {
		logs.Error("get user project data view scope query failed, err:%s", err.Error())
		return DataViewScopeSelf, nil, err
	}
	if len(rows) == 0 {
		return DataViewScopeSelf, nil, nil
	}

	scope := rows[0][`data_view_scope`]
	if scope == `` {
		scope = DataViewScopeSelf
	}
	if scope != DataViewScopePartial {
		return scope, nil, nil
	}

	roleId := cast.ToInt(rows[0][`id`])
	memberIds, err := listRoleDataViewMemberIds(adminId, roleId)
	if err != nil {
		logs.Error("list project data view members query failed, err:%s", err.Error())
		return DataViewScopeSelf, nil, err
	}
	return scope, memberIds, nil
}

// buildDataViewScopeCondition builds the SQL fragment (and its args) that restricts a work-item
// query (table alias r) to the current user's list data view scope: 自己创建 + 范围内处理人。
// Admins and the "all" scope return an empty fragment (no filter); nextArgIndex is the next
// $n placeholder number so callers can concatenate the fragment into their own args.
func buildDataViewScopeCondition(adminId int, wsId string, userId int, nextArgIndex int) (string, []any, error) {
	if userId <= 0 || IsTenantOwner(adminId, userId) {
		return ``, nil, nil
	}
	viewScope, viewMemberIds, err := getUserProjectDataViewScope(adminId, wsId, userId)
	if err != nil {
		return ``, nil, err
	}
	switch viewScope {
	case DataViewScopeAll:
		// 全部成员：不追加过滤条件。
		return ``, nil, nil
	case DataViewScopePartial:
		// 部分成员：可查看自己创建、配置成员范围内处理、以及指派给自己的数据。
		// 即使本人未配置在部分成员列表中，也应能查看指派给自己的需求/缺陷。
		allowedMemberIds := make([]int, 0, len(viewMemberIds)+1)
		allowedMemberIds = append(allowedMemberIds, viewMemberIds...)
		allowedMemberIds = append(allowedMemberIds, userId)
		return fmt.Sprintf(`(r.created_by = $%d OR r.assignee_ids && $%d)`, nextArgIndex, nextArgIndex+1),
			[]any{userId, pq.Array(allowedMemberIds)}, nil
	default:
		// 仅自己：只能看到自己创建或自己处理的数据。
		return fmt.Sprintf(`(r.created_by = $%d OR r.assignee_ids @> ARRAY[$%d])`, nextArgIndex, nextArgIndex),
			[]any{userId}, nil
	}
}

// intSliceContains reports whether the integer slice contains the target value.
func intSliceContains(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// intSlicesIntersect reports whether the two integer slices share at least one element.
func intSlicesIntersect(a, b []int) bool {
	for _, value := range a {
		if intSliceContains(b, value) {
			return true
		}
	}
	return false
}

// requirementDataViewVisible reports whether the work item (created_by + assignee_ids) falls
// within the current user's data view scope. 与列表过滤语义一致，供详情/评论/附件等读接口复用。
func requirementDataViewVisible(adminId int, wsId string, userId int, createdBy int, assigneeIds []int) (bool, error) {
	if userId <= 0 || IsTenantOwner(adminId, userId) {
		return true, nil
	}
	if createdBy == userId {
		return true, nil
	}
	viewScope, viewMemberIds, err := getUserProjectDataViewScope(adminId, wsId, userId)
	if err != nil {
		return false, err
	}
	return requirementDataViewVisibleInScope(userId, createdBy, assigneeIds, viewScope, viewMemberIds), nil
}

func requirementDataViewVisibleInScope(userId, createdBy int, assigneeIds []int, viewScope string, viewMemberIds []int) bool {
	if createdBy == userId {
		return true
	}
	switch viewScope {
	case DataViewScopeAll:
		return true
	case DataViewScopePartial:
		// 指派给自己的数据始终可见，即使本人不在配置的部分成员列表中。
		if intSliceContains(assigneeIds, userId) {
			return true
		}
		return intSlicesIntersect(assigneeIds, viewMemberIds)
	default:
		return intSliceContains(assigneeIds, userId)
	}
}

// CheckRequirementVisible reports whether the work item is visible to the current user under
// the data view scope; it also returns false when the requirement does not exist.
// 评论/附件等附属读接口的前置校验，范围外与不存在统一返回不可见，避免泄露数据存在性。
func CheckRequirementVisible(adminId int, wsId, reqId string, userId int) (bool, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT created_by, assignee_ids::text AS assignee_ids
		FROM requirements WHERE admin_id = $1 AND workspace_id = $2 AND id = $3 LIMIT 1`,
		nil, adminId, cast.ToInt(wsId), cast.ToInt(reqId))
	if err != nil {
		logs.Error("CheckRequirementVisible query failed, err:%s", err.Error())
		return false, err
	}
	if len(rows) == 0 {
		return false, nil
	}
	assigneeIds, err := parsePgIntArray(rows[0][`assignee_ids`])
	if err != nil {
		return false, err
	}
	return requirementDataViewVisible(adminId, wsId, userId, cast.ToInt(rows[0][`created_by`]), assigneeIds)
}

// GetVisibleRequirementById returns requirement details only when the current user can view it
// under the data view scope; used by read-only endpoints (detail/history). 范围外返回空 Params。
// 注意：写操作（更新/删除/导入）仅受功能权限约束，应继续使用 GetRequirementById。
func GetVisibleRequirementById(adminId int, wsId, reqId string, userId int) (msql.Params, error) {
	data, err := GetRequirementById(adminId, wsId, reqId)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return data, nil
	}
	var assigneeIds []int
	if err = json.Unmarshal([]byte(data[`assignee_ids`]), &assigneeIds); err != nil {
		logs.Error("GetVisibleRequirementById parse assignee_ids failed, err:%s", err.Error())
		return nil, err
	}
	visible, err := requirementDataViewVisible(adminId, wsId, userId, cast.ToInt(data[`created_by`]), assigneeIds)
	if err != nil {
		return nil, err
	}
	if !visible {
		return msql.Params{}, nil
	}
	return data, nil
}

// CreateRequirement creates a requirement and returns the requirement ID.
// CreateRequirementModel holds the input and output of the create requirement/defect use case.
type CreateRequirementModel struct {
	AdminId            int
	UserId             int
	WorkspaceId        int
	Title              string
	Description        string
	DescriptionMode    string
	Type               string
	Priority           *int // nil means not provided; filled by the project default priority
	StatusId           int
	TemplateId         int
	ParentId           int
	CategoryId         int
	AssigneeIds        []int
	VersionId          int
	PlannedStartDate   int64
	PlannedEndDate     int64
	EstimatedWorkHours float64
	CustomFields       CustomFieldValues
	PlanningId         int    // When > 0, append the work item to this planning after creation
	Timezone           string // Timezone used when evaluating automation rules.
	Source             string // Empty means define.SourceUser; automation sources do not re-trigger rules.
	ImageFileIds       []int  // Editor-uploaded file ledger ids (image/file/video) to bind to the created work item
	// Creation result.
	Id int
}

// CreateRequirement 创建需求/缺陷（按 admin_id 隔离，写入变更历史并重算版本进度）。
// Source 为用户操作时触发自动化规则；自动化来源不再触发，避免规则循环。
func CreateRequirement(model CreateRequirementModel) (CreateRequirementModel, error) {
	if err := CheckStorageQuota(model.AdminId, 0); err != nil {
		return CreateRequirementModel{}, err
	}
	model.Title = lib_storage.StripOSSURLSignatures(model.Title)
	model.Description = lib_storage.StripOSSURLSignatures(model.Description)
	// The internal helper still takes the workspace ID as a string and converts it once at the boundary.
	wsId := cast.ToString(model.WorkspaceId)
	if model.Source == `` {
		model.Source = define.SourceUser
	}
	if model.AssigneeIds == nil {
		model.AssigneeIds = []int{}
	}
	assigneeIds, err := validateRequirementAssignees(model.AdminId, wsId, model.AssigneeIds, nil)
	if err != nil {
		return CreateRequirementModel{}, err
	}
	model.AssigneeIds = assigneeIds
	if err := validate.ValidatePlannedDateRange(model.PlannedStartDate, model.PlannedEndDate); err != nil {
		return CreateRequirementModel{}, err
	}
	if len(model.Type) == 0 {
		model.Type = define.TypeKeyRequirement
	}
	templateName := ``
	var createTemplate *WorkItemTemplateModel
	if model.TemplateId != 0 {
		if model.TemplateId < 0 {
			return CreateRequirementModel{}, ErrWorkItemTemplateNotFound
		}
		template, err := GetWorkItemTemplate(GetWorkItemTemplateModel{
			AdminId: model.AdminId, WorkspaceId: wsId, TemplateId: model.TemplateId, TypeKey: model.Type,
		})
		if err != nil {
			return CreateRequirementModel{}, err
		}
		templateName = template.Item.Name
		createTemplate = &template.Item
	}
	// 缺陷没有预计工时：直接归零，不报错，免得调用方为了建一条缺陷先去猜这条规则。
	if model.Type != define.TypeKeyRequirement {
		model.EstimatedWorkHours = 0
	}
	if err := validate.ValidateRequirementWorkHours(model.EstimatedWorkHours); err != nil {
		return CreateRequirementModel{}, err
	}
	if model.DescriptionMode == `` {
		model.DescriptionMode = define.DescriptionModeRichText
	}
	if err := validate.ValidateRequirementDescriptionMode(model.DescriptionMode); err != nil {
		return CreateRequirementModel{}, err
	}
	priorityID := 0
	if model.Priority == nil {
		priorityID = getDefaultPriorityId(model.AdminId, wsId, model.Type)
	} else {
		priorityID = *model.Priority
		if priorityID != 0 {
			if err := validatePriorityID(model.AdminId, wsId, model.Type, priorityID); err != nil {
				return CreateRequirementModel{}, err
			}
		}
	}
	if model.StatusId == 0 {
		statusId, err := getInitialStatusId(model.AdminId, wsId, model.Type)
		if err != nil {
			return CreateRequirementModel{}, err
		}
		model.StatusId = statusId
	} else {
		statusRow, err := getStatusById(model.AdminId, wsId, model.Type, model.StatusId)
		if err != nil {
			return CreateRequirementModel{}, err
		}
		if len(statusRow) == 0 {
			return CreateRequirementModel{}, fmt.Errorf("%w: %d", ErrStatusNotFound, model.StatusId)
		}
	}
	if err := validateParent(model.AdminId, wsId, 0, model.ParentId, model.Type); err != nil {
		return CreateRequirementModel{}, err
	}
	if err := validateRequirementVersion(model.AdminId, wsId, model.VersionId, false); err != nil {
		return CreateRequirementModel{}, err
	}
	customFields, err := ListCustomFields(ListCustomFieldsModel{AdminId: model.AdminId, WsId: wsId, TypeKey: model.Type})
	if err != nil {
		return CreateRequirementModel{}, err
	}
	if createTemplate != nil {
		customFields.Items, err = customFieldsForCreateTemplate(customFields.Items, createTemplate.Fields, model)
		if err != nil {
			return CreateRequirementModel{}, err
		}
	}
	memberScope, err := loadCustomFieldMemberScopeForFields(model.AdminId, wsId, customFields.Items)
	if err != nil {
		return CreateRequirementModel{}, err
	}
	if err := validateCustomFieldValuesWithScope(customFields.Items, model.CustomFields, true, memberScope); err != nil {
		return CreateRequirementModel{}, err
	}
	assignmentRecipients := newAssigneeNotificationRecipients(model.UserId, model.AssigneeIds, nil)
	actorName := ``
	if len(assignmentRecipients) > 0 {
		actorName = notificationActorName(model.AdminId, model.UserId)
	}
	completedAt, err := initialCompletionTime(model.AdminId, model.WorkspaceId, model.Type, model.StatusId, int64(tool.Time2Int()))
	if err != nil {
		return CreateRequirementModel{}, err
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin create requirement transaction failed, err:%s", err.Error())
		return CreateRequirementModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	slaEnabled, err := prepareSLAMutationTx(tx, model.AdminId, model.WorkspaceId)
	if err != nil {
		return CreateRequirementModel{}, err
	}
	if model.Type == define.TypeKeyRequirement && (model.ParentId > 0 || model.CategoryId > 0) {
		if err = lockRequirementHierarchyTx(tx, model.AdminId, model.WorkspaceId); err != nil {
			return CreateRequirementModel{}, err
		}
	}
	model.CategoryId, err = resolveCreateRequirementCategoryID(
		tx, model.AdminId, model.WorkspaceId, model.Type, model.ParentId, model.CategoryId,
	)
	if err != nil {
		return CreateRequirementModel{}, err
	}
	now := tool.Time2Int()
	rows, err := msql.RawValues(define.Postgres, `INSERT INTO requirements
		(workspace_id, parent_id, category_id, title, description, description_mode, type, priority,
		 status_id, assignee_ids, version_id, planned_start_date, planned_end_date, work_hours,
		 created_by, admin_id, created_at, updated_at, completed_at, template_id, template_name)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$17,$18,$19,$20) RETURNING id`, tx,
		model.WorkspaceId, model.ParentId, model.CategoryId, model.Title, model.Description,
		model.DescriptionMode, model.Type, priorityID, model.StatusId, pq.Array(model.AssigneeIds),
		model.VersionId, model.PlannedStartDate, model.PlannedEndDate, model.EstimatedWorkHours,
		model.UserId, model.AdminId, now, completedAt, model.TemplateId, templateName)
	if err != nil {
		logs.Error("create requirement failed, err:%s", err.Error())
		return CreateRequirementModel{}, err
	}
	if len(rows) == 0 {
		return CreateRequirementModel{}, errors.New("create requirement returned no id")
	}
	model.Id = cast.ToInt(rows[0][`id`])
	if err := writeCustomFieldValues(tx, model.AdminId, model.Id, customFields.Items, model.CustomFields); err != nil {
		return CreateRequirementModel{}, err
	}
	if model.PlanningId > 0 {
		if err := linkRequirementToPlanning(tx, model.AdminId, model.WorkspaceId, model.PlanningId, model.Id); err != nil {
			return CreateRequirementModel{}, err
		}
	}
	itemType := notificationItemType(model.Type)
	for _, recipientID := range assignmentRecipients {
		event := notificationEvent{AdminID: model.AdminId, RecipientID: recipientID, WorkspaceID: model.WorkspaceId, ItemID: model.Id, EventType: define.NotificationEventItemAssigned, ActorID: model.UserId}
		event.Content = notificationContent(event, actorName, itemType)
		if err := CreateNotificationTx(tx, event); err != nil {
			return CreateRequirementModel{}, err
		}
	}
	if slaEnabled && model.Type == define.TypeKeyRequirement {
		if err = recordSLACreatedTx(tx, model.AdminId, model.WorkspaceId, model.Id, "user"); err != nil {
			return CreateRequirementModel{}, err
		}
	}
	if err = createHistoryTx(tx, model.AdminId, model.WorkspaceId, model.Id, model.UserId, define.HistoryActionCreated, "", "", model.Title); err != nil {
		return CreateRequirementModel{}, err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit create requirement transaction failed, err:%s", err.Error())
		return CreateRequirementModel{}, err
	}
	// Maintain the redundant progress of the belonging version (requirement count / completed count).
	if model.VersionId > 0 {
		if err := RecalcVersionCounts(nil, model.AdminId, model.VersionId); err != nil {
			logs.Error("RecalcVersionCounts after create failed: %s", err.Error())
		}
	}
	// 回绑创建前就已上传的富文本图片/文件/视频（source_id = 0 -> 指向本工作项）。
	if len(model.ImageFileIds) > 0 {
		if e := BindPendingEnterpriseFiles(model.AdminId, model.Id, model.Type, model.Title, model.ImageFileIds); e != nil {
			logs.Error("CreateRequirement bind inline files failed: %s", e.Error())
		}
	}
	// 用户操作才入队自动化；自动化来源不再二次触发规则。
	if model.Source == define.SourceUser {
		enqueueWorkItemAutomation(model.AdminId, model.WorkspaceId, model.Id, model.UserId, "create", model.Timezone, nil)
	}
	return model, nil
}

// linkRequirementToPlanning appends a newly created work item to the given planning (append semantics, does not affect existing links).
// It first verifies the planning belongs to the same tenant and workspace to avoid cross-tenant writes.
func linkRequirementToPlanning(tx *sql.Tx, adminId, workspaceId, planningId, itemId int) error {
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM requirements
		WHERE admin_id = $1 AND workspace_id = $2 AND type = $3 AND id = $4 AND deleted_at = 0`,
		tx, adminId, workspaceId, define.TypeKeyPlanning, planningId)
	if err != nil {
		logs.Error("lookup planning for new work item failed, err:%s", err.Error())
		return err
	}
	if len(rows) == 0 {
		return ErrPlanningNotFound
	}
	if _, err = msql.RawExec(define.Postgres, `INSERT INTO planning_relations
		(admin_id, workspace_id, planning_id, item_id, created_at) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (admin_id, planning_id, item_id) DO NOTHING`,
		tx, adminId, workspaceId, planningId, itemId, tool.Time2Int()); err != nil {
		logs.Error("link new work item to planning failed, err:%s", err.Error())
		return err
	}
	return nil
}

// cleanupCreatedRequirement rolls back a just-inserted work item when a later step fails (the create flow uses no transaction, so it uses compensating delete).
func cleanupCreatedRequirement(adminId int, wsId string, requirementId int) {
	cleanupM := msql.Model(define.TableRequirements, define.Postgres)
	if _, cleanupErr := cleanupM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, wsId},
		{`id`, cast.ToString(requirementId)},
	}).Delete(); cleanupErr != nil {
		logs.Error("CreateRequirement cleanup failed: %s", cleanupErr.Error())
	}
}

// UpdateRequirement updates a user-sourced work item.
func UpdateRequirement(adminId int, wsId, reqId string, updates map[string]any, customFields map[string]any, userId int, imageFileIds []int) error {
	return updateRequirementWithTransaction(adminId, wsId, reqId, updates, customFields, userId, imageFileIds, define.SourceUser, ``)
}

// UpdateRequirementWithSource updates a work item with an explicit source; an automation source does not trigger rules again.
func UpdateRequirementWithSource(adminId int, wsId, reqId string, updates map[string]any, customFields map[string]any, userId int, source string) error {
	return updateRequirementWithTransaction(adminId, wsId, reqId, updates, customFields, userId, nil, source, ``)
}

// RestoreRequirementDescription restores the work item description to a version kept in one change record.
// side takes define.HistorySideOld for the value before that change and define.HistorySideNew for the value after it.
// The restore reuses the regular update chain so automation rules, cache invalidation and the change history
// behave exactly like a manual edit; only the recorded action becomes define.HistoryActionRestored.
func RestoreRequirementDescription(adminId int, wsId, reqId string, historyID int, side string, userId int) error {
	history, found, err := GetHistoryById(adminId, cast.ToInt(wsId), cast.ToInt(reqId), historyID)
	if err != nil {
		return err
	}
	if !found {
		return ErrRequirementHistoryNotFound
	}
	// Only a description change carries a restorable version; the creation record and every other
	// field keep their plain-text rendering in the list.
	if history.FieldName != `description` {
		return ErrRequirementHistoryNotRestorable
	}
	var description string
	switch side {
	case define.HistorySideOld:
		description = history.OldValue
	case define.HistorySideNew:
		description = history.NewValue
	default:
		return ErrRequirementHistoryNotRestorable
	}
	updates := map[string]any{`description`: description}
	return updateRequirementWithTransaction(adminId, wsId, reqId, updates, nil, userId, nil, define.SourceUser, define.HistoryActionRestored)
}

// updateRequirementWithTransaction runs the update in a transaction. A non-empty historyAction
// overrides the recorded change action, an empty one keeps the per-field derivation.
func updateRequirementWithTransaction(adminId int, wsId, reqId string, updates map[string]any, customFields map[string]any, userId int, imageFileIds []int, source string, historyAction string) error {
	var beforeSnapshot *AutomationWorkItemSnapshot
	if source == define.SourceUser {
		var err error
		beforeSnapshot, err = CaptureAutomationSnapshot(adminId, cast.ToInt(wsId), cast.ToInt(reqId))
		if err != nil {
			return err
		}
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = updateRequirement(tx, adminId, wsId, reqId, updates, customFields, userId, imageFileIds, source, historyAction); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if source == define.SourceUser {
		enqueueWorkItemAutomation(adminId, cast.ToInt(wsId), cast.ToInt(reqId), userId, "update", "", beforeSnapshot)
	}
	if err = InvalidateIssueInvitePublicCaches(adminId, []int{cast.ToInt(reqId)}); err != nil {
		logs.Error("invalidate issue invite cache after update failed, err:%s", err.Error())
	}
	return nil
}

// updateRequirement applies the existing update flow on the supplied transaction when present.
func updateRequirement(tx *sql.Tx, adminId int, wsId, reqId string, updates map[string]any, customFields map[string]any, userId int, imageFileIds []int, source string, historyAction string) error {
	for field, value := range updates {
		if text, ok := value.(string); ok {
			updates[field] = lib_storage.StripOSSURLSignatures(text)
		}
	}
	slaEnabled, err := prepareSLAMutationTx(tx, adminId, cast.ToInt(wsId))
	if err != nil {
		return err
	}
	if _, changingParent := updates[`parent_id`]; changingParent && tx != nil {
		if err = lockRequirementHierarchyTx(tx, adminId, cast.ToInt(wsId)); err != nil {
			return err
		}
	}
	if slaEnabled {
		if _, err = slaRows(tx, "SELECT id FROM "+define.TableRequirements+" WHERE admin_id=$1 AND workspace_id=$2 AND id=$3 FOR UPDATE", adminId, cast.ToInt(wsId), cast.ToInt(reqId)); err != nil {
			return err
		}
	}
	// Query the current record before update, used to compare changed fields.
	oldData, err := getRequirementById(tx, adminId, wsId, reqId)
	if err != nil {
		logs.Error("UpdateRequirement query old data failed: err:%s", err.Error())
		return err
	}
	if len(oldData) == 0 {
		return ErrRequirementNotFound
	}
	oldAssigneeIds := parseStoredAssigneeIDs(oldData[`assignee_ids`])
	if rawMode, ok := updates[`description_mode`]; ok {
		if err := validate.ValidateRequirementDescriptionMode(cast.ToString(rawMode)); err != nil {
			return err
		}
	}
	parentChanged := false
	if rawAssigneeIds, ok := updates[`assignee_ids`]; ok {
		assigneeIds, valid := rawAssigneeIds.([]int)
		if !valid {
			return ErrRequirementAssigneesInvalid
		}
		oldAssigneeIds := []int{}
		if err = json.Unmarshal([]byte(oldData[`assignee_ids`]), &oldAssigneeIds); err != nil {
			logs.Error("UpdateRequirement parse old assignee_ids failed, err:%s", err.Error())
			return err
		}
		assigneeIds, err = validateRequirementAssignees(adminId, wsId, assigneeIds, oldAssigneeIds)
		if err != nil {
			return err
		}
		updates[`assignee_ids`] = assigneeIds
	}
	plannedStartDate := cast.ToInt64(oldData[`planned_start_date`])
	plannedEndDate := cast.ToInt64(oldData[`planned_end_date`])
	if value, ok := updates[`planned_start_date`]; ok {
		plannedStartDate = cast.ToInt64(value)
	}
	if value, ok := updates[`planned_end_date`]; ok {
		plannedEndDate = cast.ToInt64(value)
	}
	if err := validate.ValidatePlannedDateRange(plannedStartDate, plannedEndDate); err != nil {
		return err
	}
	if _, ok := updates[`work_hours`]; ok {
		// 与新建侧同一条规则：非需求类型没有预计工时，归零而不是报错。
		if cast.ToString(oldData[`type`]) != define.TypeKeyRequirement {
			updates[`work_hours`] = 0
		}
		if err := validate.ValidateRequirementWorkHours(cast.ToFloat64(updates[`work_hours`])); err != nil {
			return err
		}
	}
	if rawParentId, ok := updates[`parent_id`]; ok {
		parentChanged = cast.ToInt(rawParentId) != cast.ToInt(oldData[`parent_id`])
		if tx != nil {
			err = updateRequirementParentTx(tx, adminId, wsId, cast.ToInt(reqId), cast.ToInt(rawParentId), userId)
		} else {
			err = updateRequirementParent(adminId, wsId, cast.ToInt(reqId), cast.ToInt(rawParentId), userId)
		}
		if err != nil {
			return err
		}
		delete(updates, `parent_id`)
	}

	var nextStatus msql.Params
	if rawStatusId, ok := updates[`status_id`]; ok {
		newStatusId := cast.ToInt(rawStatusId)
		oldStatusId := cast.ToInt(oldData[`status_id`])
		if newStatusId > 0 {
			nextStatus, err = getStatusById(adminId, cast.ToString(oldData[`workspace_id`]), cast.ToString(oldData[`type`]), newStatusId)
			if err != nil {
				return err
			}
			if len(nextStatus) == 0 {
				return fmt.Errorf("%w: %d", ErrStatusNotFound, newStatusId)
			}
			if oldStatusId > 0 && oldStatusId != newStatusId {
				if err := ensureStatusTransition(adminId, cast.ToString(oldData[`workspace_id`]), cast.ToString(oldData[`type`]), oldStatusId, newStatusId); err != nil {
					return err
				}
			}
		}
	}
	if rawPriority, ok := updates[`priority`]; ok && cast.ToInt(rawPriority) > 0 {
		if err := validatePriorityID(adminId, cast.ToString(oldData[`workspace_id`]), cast.ToString(oldData[`type`]), cast.ToInt(rawPriority)); err != nil {
			return err
		}
	}
	if rawVersionID, ok := updates[`version_id`]; ok {
		versionId := cast.ToInt(rawVersionID)
		allowEnded := versionId > 0 && versionId == cast.ToInt(oldData[`version_id`])
		if err := validateRequirementVersion(adminId, wsId, versionId, allowEnded); err != nil {
			return err
		}
	}
	if err := ValidateCustomFieldValues(adminId, wsId, oldData[`type`], customFields, false); err != nil {
		return err
	}

	// Build the datas map.
	data := make(msql.Datas, len(updates)+1)
	for k, v := range updates {
		if !requirementUpdatableFields[k] {
			continue
		}
		if k == `assignee_ids` {
			if ids, ok := v.([]int); ok {
				data[k] = pq.Array(ids)
			}
			continue
		}
		data[k] = v
	}
	if len(data) == 0 && len(customFields) == 0 && !parentChanged {
		return nil
	}
	data[`updated_at`] = tool.Time2Int()
	oldCustomValues := map[string]string{}
	if len(customFields) > 0 {
		oldCustomValues, err = ListCustomFieldValues(adminId, cast.ToInt(reqId))
		if err != nil {
			return err
		}
	}

	if err = updateRequirementRow(tx, adminId, wsId, reqId, data); err != nil {
		return err
	}
	if err = saveCustomFieldValues(tx, adminId, wsId, oldData[`type`], cast.ToInt(reqId), customFields, false); err != nil {
		return err
	}

	// Record the change history.
	reqIdInt := cast.ToInt(reqId)
	workspaceID := cast.ToInt(oldData[`workspace_id`])
	changedFields := make([]string, 0)
	for field, newVal := range data {
		if field == `updated_at` || field == `description_mode` {
			continue
		}
		oldVal := cast.ToString(oldData[field])
		newValText := cast.ToString(newVal)
		if field == `assignee_ids` {
			oldVal = normalizeStoredAssigneeIDs(oldVal)
			newValText = assigneeIDsText(updates[`assignee_ids`].([]int))
		}
		if oldVal == newValText {
			continue // Value unchanged, skip.
		}
		var action string
		historyField := field
		switch field {
		case `status_id`:
			action = define.HistoryActionStatusChanged
			historyField = `status`
			oldVal = cast.ToString(oldData[`status_name`])
			newValText = cast.ToString(nextStatus[`name`])
		case `assignee_ids`:
			action = define.HistoryActionAssignChanged
			historyField = `assignee`
		default:
			action = define.HistoryActionFieldUpdated
		}
		// A restore reuses this loop for its single description change; only the action differs so the
		// list can tell a manual edit apart from a restored version.
		if historyAction != `` {
			action = historyAction
		}
		changedFields = append(changedFields, historyField)
		var historyErr error
		if tx != nil {
			historyErr = createHistoryTx(tx, adminId, workspaceID, reqIdInt, userId, action, historyField, oldVal, newValText)
		} else {
			historyErr = CreateHistory(adminId, workspaceID, reqIdInt, userId, action, historyField, oldVal, newValText)
		}
		if historyErr != nil {
			logs.Error("CreateHistory failed: %s", historyErr.Error())
			if tx != nil {
				return historyErr
			}
		}
	}
	if parentChanged {
		changedFields = append(changedFields, `parent_id`)
	}
	if len(customFields) > 0 {
		fieldsModel, fieldErr := ListCustomFields(ListCustomFieldsModel{AdminId: adminId, WsId: wsId, TypeKey: oldData[`type`]})
		if fieldErr != nil {
			return fieldErr
		}
		fieldNames := make(map[string]string, len(fieldsModel.Items))
		for _, field := range fieldsModel.Items {
			fieldNames[cast.ToString(field.Id)] = field.Name
		}
		for key, value := range customFields {
			newText, textErr := customFieldValueText(value)
			if textErr != nil {
				return textErr
			}
			if strings.TrimSpace(oldCustomValues[key]) == strings.TrimSpace(newText) {
				continue
			}
			fieldName := fieldNames[key]
			if fieldName == `` {
				fieldName = key
			}
			changedFields = append(changedFields, fieldName)
		}
	}
	// Notify the creator about changes made by another user and notify newly-added assignees.
	if len(changedFields) > 0 {
		actorName := notificationActorName(adminId, userId)
		itemType := notificationItemType(oldData[`type`])
		creatorID := cast.ToInt(oldData[`created_by`])
		for _, field := range changedFields {
			if creatorID > 0 && creatorID != userId {
				event := notificationEvent{AdminID: adminId, RecipientID: creatorID, WorkspaceID: workspaceID, ItemID: reqIdInt, EventType: define.NotificationEventItemUpdated, ActorID: userId, FieldName: field}
				if field == `status` {
					event.StatusName = nextStatus[`name`]
				}
				if field == `assignee` {
					if raw, ok := updates[`assignee_ids`]; ok {
						if ids, valid := raw.([]int); valid {
							event.AssigneeNames = notificationAssigneeNames(adminId, ids)
						}
					}
				}
				event.Content = notificationContent(event, actorName, itemType)
				if notifyErr := CreateNotificationTx(tx, event); notifyErr != nil {
					return notifyErr
				}
			}
		}
		if raw, ok := updates[`assignee_ids`]; ok {
			newIds, _ := raw.([]int)
			for _, id := range newAssigneeNotificationRecipients(userId, newIds, oldAssigneeIds) {
				event := notificationEvent{AdminID: adminId, RecipientID: id, WorkspaceID: workspaceID, ItemID: reqIdInt, EventType: define.NotificationEventItemAssigned, ActorID: userId}
				event.Content = notificationContent(event, actorName, itemType)
				if notifyErr := CreateNotificationTx(tx, event); notifyErr != nil {
					return notifyErr
				}
			}
		}
	}

	// Maintain the redundant progress of the belonging version: recalculate on status change or belonging-version change (both old and new versions may be affected).
	oldVersionId := cast.ToInt(oldData[`version_id`])
	newVersionId := oldVersionId
	if rawVersionID, ok := updates[`version_id`]; ok {
		newVersionId = cast.ToInt(rawVersionID)
	}
	affected := map[int]bool{}
	if oldVersionId > 0 {
		affected[oldVersionId] = true
	}
	if newVersionId > 0 {
		affected[newVersionId] = true
	}
	if len(affected) > 0 {
		if _, statusChanged := updates[`status_id`]; statusChanged || oldVersionId != newVersionId {
			for vid := range affected {
				if recalcErr := RecalcVersionCounts(tx, adminId, vid); recalcErr != nil {
					logs.Error("RecalcVersionCounts after update failed, versionId:%d, err:%s", vid, recalcErr.Error())
					if tx != nil {
						return recalcErr
					}
				}
			}
		}
	}
	if slaEnabled && oldData[`type`] == define.TypeKeyRequirement {
		_, statusUpdated := updates[`status_id`]
		_, priorityUpdated := updates[`priority`]
		if statusUpdated || priorityUpdated {
			before, err := slaStateFromRow(oldData)
			if err != nil {
				return err
			}
			if err = recordSLAUpdatedTx(tx, adminId, cast.ToInt(wsId), cast.ToInt(reqId), before, source); err != nil {
				return err
			}
		}
	}
	// Bind rich-text assets uploaded before this save.
	if len(imageFileIds) > 0 {
		if e := BindPendingEnterpriseFiles(adminId, cast.ToInt(reqId), cast.ToString(oldData[`type`]), cast.ToString(oldData[`title`]), imageFileIds); e != nil {
			logs.Error("UpdateRequirement bind rich-text assets failed: %s", e.Error())
		}
	}

	// 回绑保存前就已上传的富文本图片/文件/视频（source_id = 0 -> 指向本工作项）。
	if len(imageFileIds) > 0 {
		if e := BindPendingEnterpriseFiles(adminId, cast.ToInt(reqId), cast.ToString(oldData[`type`]), cast.ToString(oldData[`title`]), imageFileIds); e != nil {
			logs.Error("UpdateRequirement bind inline files failed: %s", e.Error())
		}
	}
	return nil
}

// updateRequirementRow updates the work-item row without escaping an existing transaction.
func updateRequirementRow(tx *sql.Tx, adminId int, wsId, reqId string, data msql.Datas) error {

	fields := make([]string, 0, len(data))
	for field := range data {
		if field == `updated_at` || requirementUpdatableFields[field] {
			fields = append(fields, field)
		}
	}
	sort.Strings(fields)
	assignments := make([]string, 0, len(fields))
	args := make([]any, 0, len(fields)+3)
	for _, field := range fields {
		args = append(args, data[field])
		assignments = append(assignments, field+` = $`+strconv.Itoa(len(args)))
	}
	if _, changing := data[`status_id`]; changing {
		finalIds, err := workItemCompletedIDs(adminId, cast.ToInt(wsId))
		if err != nil {
			return err
		}
		statusArg, timeArg := 0, 0
		for i, field := range fields {
			if field == `status_id` {
				statusArg = i + 1
			}
			if field == `updated_at` {
				timeArg = i + 1
			}
		}
		args = append(args, pq.Array(finalIds))
		assignments = append(assignments, completionAssignment(statusArg, timeArg, len(args)))
	}
	adminArg := len(args) + 1
	workspaceArg := adminArg + 1
	idArg := workspaceArg + 1
	args = append(args, adminId, cast.ToInt(wsId), cast.ToInt(reqId))
	_, err := msql.RawExec(define.Postgres, `UPDATE `+define.TableRequirements+` SET `+strings.Join(assignments, `, `)+
		` WHERE admin_id = $`+strconv.Itoa(adminArg)+` AND workspace_id = $`+strconv.Itoa(workspaceArg)+
		` AND id = $`+strconv.Itoa(idArg), tx, args...)
	if err != nil {
		logs.Error("UpdateRequirement transaction update failed, err:%s", err.Error())
	}
	return err
}

// BatchUpdateRequirements updates work-item fields in batch within a single transaction.
func BatchUpdateRequirements(adminId int, wsId string, ids []int, field string, value any, userId int) error {
	if err := validate.ValidateRequirementBatchUpdate(field, value); err != nil {
		return err
	}
	valueInt := 0
	assigneeIds := []int(nil)
	if field == `assignee_ids` {
		assigneeIds = value.([]int)
		var err error
		assigneeIds, err = validateRequirementAssignees(adminId, wsId, assigneeIds, nil)
		if err != nil {
			return err
		}
	} else {
		valueInt = value.(int)
	}
	beforeSnapshots := make(map[int]*AutomationWorkItemSnapshot, len(ids))
	for _, id := range ids {
		snapshot, err := CaptureAutomationSnapshot(adminId, cast.ToInt(wsId), id)
		if err != nil {
			return err
		}
		beforeSnapshots[id] = snapshot
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("BatchUpdateRequirements begin failed, err:%s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()
	slaEnabled := false
	if field == `status_id` || field == `priority` {
		slaEnabled, err = prepareSLAMutationTx(tx, adminId, cast.ToInt(wsId))
		if err != nil {
			return err
		}
	}

	rows, err := msql.RawValues(define.Postgres, `SELECT r.id, r.workspace_id, r.category_id, r.type, r.status_id, r.priority, r.created_by,
		r.assignee_ids::text AS assignee_ids, r.version_id, s.name AS status_name
		FROM requirements r
		LEFT JOIN workspace_statuses s ON s.id = r.status_id AND s.admin_id = r.admin_id
			AND s.workspace_id = r.workspace_id AND s.type_key = r.type
		WHERE r.admin_id = $1 AND r.workspace_id = $2 AND r.id = ANY($3)
		ORDER BY r.id FOR UPDATE OF r`, tx, adminId, cast.ToInt(wsId), pq.Array(ids))
	if err != nil {
		logs.Error("BatchUpdateRequirements lock rows failed, err:%s", err.Error())
		return err
	}
	if len(rows) != len(ids) {
		return ErrRequirementNotFound
	}

	types := make(map[string]bool)
	for _, row := range rows {
		types[row[`type`]] = true
	}
	targetStatusNames := make(map[string]string)
	for itemType := range types {
		switch field {
		case `status_id`:
			statusRows, queryErr := msql.RawValues(define.Postgres, `SELECT name FROM workspace_statuses
				WHERE admin_id = $1 AND workspace_id = $2 AND type_key = $3 AND id = $4`, tx,
				adminId, cast.ToInt(wsId), itemType, valueInt)
			if queryErr != nil {
				logs.Error("BatchUpdateRequirements status query failed, err:%s", queryErr.Error())
				return queryErr
			}
			if len(statusRows) == 0 {
				return ErrStatusNotFound
			}
			targetStatusNames[itemType] = statusRows[0][`name`]
		case `priority`:
			if valueInt == 0 {
				continue
			}
			priorityRows, queryErr := msql.RawValues(define.Postgres, `SELECT id FROM workspace_priorities
				WHERE admin_id = $1 AND workspace_id = $2 AND type_key = $3 AND id = $4`, tx,
				adminId, cast.ToInt(wsId), itemType, valueInt)
			if queryErr != nil {
				logs.Error("BatchUpdateRequirements priority query failed, err:%s", queryErr.Error())
				return queryErr
			}
			if len(priorityRows) == 0 {
				return ErrPriorityNotFound
			}
		}
	}
	if field == `status_id` {
		for _, row := range rows {
			if err = ensureStatusTransition(adminId, wsId, row[`type`], cast.ToInt(row[`status_id`]), valueInt); err != nil {
				return err
			}
		}
	}
	updatedAt := tool.Time2Int()
	var updateSQL string
	switch field {
	case `status_id`:
		updateSQL = `UPDATE requirements SET status_id = $1, updated_at = $2
			WHERE admin_id = $3 AND workspace_id = $4 AND id = ANY($5)`
	case `priority`:
		updateSQL = `UPDATE requirements SET priority = $1, updated_at = $2
			WHERE admin_id = $3 AND workspace_id = $4 AND id = ANY($5)`
	case `assignee_ids`:
		updateSQL = `UPDATE requirements SET assignee_ids = $1, updated_at = $2
			WHERE admin_id = $3 AND workspace_id = $4 AND id = ANY($5)`
	default:
		return validate.ErrRequirementBatchInvalid
	}
	updateValue := any(valueInt)
	if field == `assignee_ids` {
		updateValue = pq.Array(assigneeIds)
	}
	updateArgs := []any{updateValue, updatedAt, adminId, cast.ToInt(wsId), pq.Array(ids)}
	if field == `status_id` {
		finalIds, finalErr := workItemCompletedIDs(adminId, cast.ToInt(wsId))
		if finalErr != nil {
			return finalErr
		}
		updateArgs = append(updateArgs, pq.Array(finalIds))
		updateSQL = strings.Replace(updateSQL, `SET status_id = $1,`, `SET `+completionAssignment(1, 2, 6)+`, status_id = $1,`, 1)
	}
	if _, err = msql.RawExec(define.Postgres, updateSQL, tx, updateArgs...); err != nil {
		logs.Error("BatchUpdateRequirements update failed, err:%s", err.Error())
		return err
	}

	versionIds := make(map[int]bool)
	changedIDs := make([]int, 0, len(rows))
	for _, row := range rows {
		oldValue := row[field]
		if field != `assignee_ids` && cast.ToInt(oldValue) == valueInt {
			continue
		}
		action := define.HistoryActionFieldUpdated
		historyField := field
		newValue := cast.ToString(valueInt)
		if field == `status_id` {
			action = define.HistoryActionStatusChanged
			historyField = `status`
			oldValue = row[`status_name`]
			newValue = targetStatusNames[row[`type`]]
		} else if field == `assignee_ids` {
			action = define.HistoryActionAssignChanged
			historyField = `assignee`
			oldValue = normalizeStoredAssigneeIDs(oldValue)
			newValue = assigneeIDsText(assigneeIds)
			if oldValue == newValue {
				continue
			}
		}
		changedIDs = append(changedIDs, cast.ToInt(row[`id`]))
		if err = createHistoryTx(tx, adminId, cast.ToInt(wsId), cast.ToInt(row[`id`]), userId,
			action, historyField, oldValue, newValue); err != nil {
			logs.Error("BatchUpdateRequirements history failed, err:%s", err.Error())
			return err
		}
		if slaEnabled && (field == `status_id` || field == `priority`) && row[`type`] == define.TypeKeyRequirement {
			before, err := slaStateFromRow(row)
			if err != nil {
				return err
			}
			if err = recordSLAUpdatedTx(tx, adminId, cast.ToInt(wsId), cast.ToInt(row[`id`]), before, "batch"); err != nil {
				return err
			}
		}
		actorName := notificationActorName(adminId, userId)
		itemType := notificationItemType(row[`type`])
		creatorID := cast.ToInt(row[`created_by`])
		if creatorID > 0 && creatorID != userId {
			event := notificationEvent{AdminID: adminId, RecipientID: creatorID, WorkspaceID: cast.ToInt(wsId), ItemID: cast.ToInt(row[`id`]), EventType: define.NotificationEventItemUpdated, ActorID: userId, FieldName: historyField}
			if field == `status_id` {
				event.StatusName = targetStatusNames[row[`type`]]
			}
			if field == `assignee_ids` {
				event.AssigneeNames = notificationAssigneeNames(adminId, assigneeIds)
			}
			event.Content = notificationContent(event, actorName, itemType)
			if notifyErr := CreateNotificationTx(tx, event); notifyErr != nil {
				return notifyErr
			}
		}
		if field == `assignee_ids` {
			oldIDs := parseStoredAssigneeIDs(row[`assignee_ids`])
			for _, id := range newAssigneeNotificationRecipients(userId, assigneeIds, oldIDs) {
				event := notificationEvent{AdminID: adminId, RecipientID: id, WorkspaceID: cast.ToInt(wsId), ItemID: cast.ToInt(row[`id`]), EventType: define.NotificationEventItemAssigned, ActorID: userId}
				event.Content = notificationContent(event, actorName, itemType)
				if notifyErr := CreateNotificationTx(tx, event); notifyErr != nil {
					return notifyErr
				}
			}
		}
		if field == `status_id` {
			if versionId := cast.ToInt(row[`version_id`]); versionId > 0 {
				versionIds[versionId] = true
			}
		}
	}
	for versionId := range versionIds {
		if err = RecalcVersionCounts(tx, adminId, versionId); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		logs.Error("BatchUpdateRequirements commit failed, err:%s", err.Error())
		return err
	}
	for _, id := range changedIDs {
		enqueueWorkItemAutomation(adminId, cast.ToInt(wsId), id, userId, "update", "", beforeSnapshots[id])
	}
	// Keep the public invitation caches in sync with batch edits.
	if err = InvalidateIssueInvitePublicCaches(adminId, ids); err != nil {
		logs.Error("invalidate issue invite cache after batch update failed, err:%s", err.Error())
	}
	return nil
}

func validateRequirementVersion(adminID int, workspaceID string, versionID int, allowEnded bool) error {
	if versionID == 0 {
		return nil
	}
	version, err := GetVersionByWorkspace(adminID, cast.ToInt(workspaceID), versionID)
	if err != nil {
		return err
	}
	if version.EndedAt > 0 && !allowEnded {
		return ErrVersionEnded
	}
	return nil
}

// DeleteRequirementsModel describes one atomic work-item deletion request.
type DeleteRequirementsModel struct {
	AdminId     int
	WorkspaceId int
	UserId      int
	Ids         []int
}

// DeleteRequirements deletes selected work items atomically; selected requirements include all descendants.
func DeleteRequirements(model DeleteRequirementsModel) error {
	requestedIds := make([]int, 0, len(model.Ids))
	seen := make(map[int]bool, len(model.Ids))
	for _, id := range model.Ids {
		if id <= 0 {
			return ErrRequirementNotFound
		}
		if !seen[id] {
			seen[id] = true
			requestedIds = append(requestedIds, id)
		}
	}
	if len(requestedIds) == 0 {
		return ErrRequirementNotFound
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = prepareSLAMutationTx(tx, model.AdminId, model.WorkspaceId); err != nil {
		return err
	}
	if err = lockRequirementHierarchyTx(tx, model.AdminId, model.WorkspaceId); err != nil {
		return err
	}
	targets, err := msql.RawValues(define.Postgres, `SELECT id, type FROM requirements
		WHERE admin_id = $1 AND workspace_id = $2 AND id = ANY($3) ORDER BY id FOR UPDATE`, tx,
		model.AdminId, model.WorkspaceId, pq.Array(requestedIds))
	if err != nil {
		return err
	}
	if len(targets) != len(requestedIds) {
		return ErrRequirementNotFound
	}
	requirementIds := make([]int, 0, len(targets))
	for _, target := range targets {
		if target[`type`] == define.TypeKeyRequirement {
			requirementIds = append(requirementIds, cast.ToInt(target[`id`]))
		}
	}

	rows, err := msql.RawValues(define.Postgres, `WITH RECURSIVE tree AS (
		SELECT id FROM requirements WHERE admin_id = $1 AND workspace_id = $2 AND id = ANY($3) AND type = $4
		UNION SELECT r.id FROM requirements r JOIN tree t ON r.parent_id = t.id
		WHERE r.admin_id = $1 AND r.workspace_id = $2 AND r.type = $4
	) SELECT r.id FROM requirements r
	WHERE r.admin_id = $1 AND r.workspace_id = $2
		AND (r.id = ANY($3) OR r.id IN (SELECT id FROM tree))
	ORDER BY r.id FOR UPDATE`, tx, model.AdminId, model.WorkspaceId, pq.Array(requestedIds), define.TypeKeyRequirement)
	if err != nil {
		return err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, cast.ToInt64(row[`id`]))
	}

	// Collect the versions that the requirements to be deleted belong to, and recalculate their progress redundant fields after deletion.
	verRows, verErr := msql.RawValues(define.Postgres, `SELECT DISTINCT version_id FROM requirements
		WHERE admin_id = $1 AND workspace_id = $2 AND id = ANY($3)`, tx,
		model.AdminId, model.WorkspaceId, pq.Array(ids))
	if verErr != nil {
		return verErr
	}
	versionIds := make([]int, 0, len(verRows))
	for _, vr := range verRows {
		if vid := cast.ToInt(vr[`version_id`]); vid > 0 {
			versionIds = append(versionIds, vid)
		}
	}
	if len(requirementIds) > 0 {
		defects, queryErr := msql.RawValues(define.Postgres, `SELECT id, parent_id FROM requirements
			WHERE admin_id = $1 AND workspace_id = $2 AND type = $3 AND parent_id = ANY($4)
				AND NOT (id = ANY($5)) FOR UPDATE`, tx, model.AdminId, model.WorkspaceId,
			define.TypeKeyDefect, pq.Array(ids), pq.Array(ids))
		if queryErr != nil {
			return queryErr
		}
		if len(defects) > 0 {
			if _, err = msql.RawExec(define.Postgres, `UPDATE requirements SET parent_id = 0, updated_at = $1
				WHERE admin_id = $2 AND workspace_id = $3 AND type = $4 AND parent_id = ANY($5)
					AND NOT (id = ANY($6))`, tx, tool.Time2Int(), model.AdminId, model.WorkspaceId,
				define.TypeKeyDefect, pq.Array(ids), pq.Array(ids)); err != nil {
				return err
			}
			for _, defect := range defects {
				if err = createHistoryTx(tx, model.AdminId, model.WorkspaceId, cast.ToInt(defect[`id`]), model.UserId,
					define.HistoryActionFieldUpdated, `parent_id`, defect[`parent_id`], `0`); err != nil {
					return err
				}
			}
		}
	}
	paths, err := msql.RawValues(define.Postgres, `SELECT stored_path FROM `+define.GetEnterpriseFileTable(model.AdminId)+`
		WHERE admin_id = $1 AND workspace_id = $2 AND source_id = ANY($3)`, tx,
		model.AdminId, model.WorkspaceId, pq.Array(ids))
	if err != nil {
		return err
	}
	// Collect public invitation owners before the association rows are deleted, so the
	// submitter caches can still be invalidated after commit.
	inviteOwners, err := msql.RawValues(define.Postgres, `SELECT workspace_id, openid FROM `+define.TableWorkspaceIssueRequirements+`
		WHERE admin_id = $1 AND workspace_id = $2 AND requirement_id = ANY($3)`, tx,
		model.AdminId, model.WorkspaceId, pq.Array(ids))
	if err != nil {
		return err
	}
	if err = clearSLAItemsTx(tx, model.AdminId, model.WorkspaceId, ids); err != nil {
		return err
	}
	for _, table := range []string{define.TableRequirementComments,
		define.TableRequirementHistories, define.TableWorkspaceIssueRequirements} {
		if _, err = msql.RawExec(define.Postgres, fmt.Sprintf(`DELETE FROM %s
			WHERE admin_id = $1 AND workspace_id = $2 AND requirement_id = ANY($3)`, table), tx,
			model.AdminId, model.WorkspaceId, pq.Array(ids)); err != nil {
			return err
		}
	}
	for _, table := range []string{define.TableCustomFieldValues, define.TableRequirementVisits} {
		if _, err = msql.RawExec(define.Postgres, fmt.Sprintf(`DELETE FROM %s WHERE admin_id = $1 AND requirement_id = ANY($2)`, table), tx,
			model.AdminId, pq.Array(ids)); err != nil {
			return err
		}
	}
	if err = DeleteNotificationsByItems(tx, model.AdminId, model.WorkspaceId, ids); err != nil {
		return err
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM `+define.GetEnterpriseFileTable(model.AdminId)+`
		WHERE admin_id = $1 AND workspace_id = $2 AND source_id = ANY($3)`, tx,
		model.AdminId, model.WorkspaceId, pq.Array(ids)); err != nil {
		return err
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM requirement_work_logs
		WHERE admin_id = $1 AND workspace_id = $2 AND requirement_id = ANY($3)`, tx,
		model.AdminId, model.WorkspaceId, pq.Array(ids)); err != nil {
		return err
	}
	// The foreign key column of the planning relation table is item_id, which is not covered by the unified cleanup loop above;
	// otherwise dangling relations would remain, causing the planning list's "has relation" filter to count deleted work items.
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM planning_relations
		WHERE admin_id = $1 AND workspace_id = $2 AND item_id = ANY($3)`, tx,
		model.AdminId, model.WorkspaceId, pq.Array(ids)); err != nil {
		return err
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM requirements
		WHERE admin_id = $1 AND workspace_id = $2 AND id = ANY($3)`, tx,
		model.AdminId, model.WorkspaceId, pq.Array(ids)); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	// Keep the public invitation caches in sync with internal deletes; the owners were
	// collected inside the transaction because the association rows are already removed.
	for _, owner := range inviteOwners {
		workspaceID := cast.ToInt(owner[`workspace_id`])
		for _, id := range ids {
			if err = InvalidateIssueInviteDetailCache(model.AdminId, workspaceID, owner[`openid`], cast.ToInt(id)); err != nil {
				logs.Error("invalidate issue invite detail cache after delete failed, err:%s", err.Error())
				break
			}
		}
		if err = RotateIssueInviteHistoryCache(model.AdminId, workspaceID, owner[`openid`]); err != nil {
			logs.Error("rotate issue invite history cache after delete failed, err:%s", err.Error())
		}
	}
	// After successful deletion, recalculate the related version progress (versionIds were collected before deletion).
	for _, vid := range versionIds {
		if err = RecalcVersionCounts(nil, model.AdminId, vid); err != nil {
			logs.Error("RecalcVersionCounts after delete failed, versionId:%d, err:%s", vid, err.Error())
		}
	}
	for _, item := range paths {
		if stored := item[`stored_path`]; len(stored) > 0 {
			if removeErr := lib_storage.M.Delete(stored); removeErr != nil {
				logs.Error("DeleteRequirement remove attachment failed, path:%s, err:%s", stored, removeErr.Error())
			}
		}
	}
	return nil
}

// DeleteRequirement deletes one work item using the shared atomic deletion flow.
func DeleteRequirement(adminId int, wsId, reqId string, userId int) error {
	return DeleteRequirements(DeleteRequirementsModel{
		AdminId:     adminId,
		WorkspaceId: cast.ToInt(wsId),
		UserId:      userId,
		Ids:         []int{cast.ToInt(reqId)},
	})
}

// UpdateRequirementStatus changes a requirement's status (admin_id isolated, records history).
func UpdateRequirementStatus(adminId int, wsId, reqId string, newStatusId int, userId int) error {
	return UpdateRequirement(adminId, wsId, reqId, map[string]any{`status_id`: newStatusId}, nil, userId, nil)
}

// ListAvailableStatusIDs returns all status IDs of that type under a workspace; the current version does not restrict status transitions.
func ListAvailableStatusIDs(adminId int, workspaceId, reqType string, currentStatusId int) ([]int, error) {
	m := msql.Model(define.TableWorkItemStatuses, define.Postgres)
	list, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, workspaceId},
		{`type_key`, reqType},
	}).Order(`sort_order asc, id asc`).Select()
	if err != nil {
		logs.Error("ListAvailableStatusIDs sql:%s, err:%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	ids := make([]int, 0, len(list))
	for _, row := range list {
		ids = append(ids, cast.ToInt(row[`id`]))
	}
	return ids, nil
}

func validateParent(adminId int, wsId string, requirementId, parentId int, reqType string) error {
	return validateParentTx(nil, adminId, wsId, requirementId, parentId, reqType)
}

func validateRequirementParentCategory(itemType string, categoryId, parentCategoryId int) error {
	if itemType == define.TypeKeyRequirement && categoryId != parentCategoryId {
		return ErrRequirementRelationInvalid
	}
	return nil
}

func validateParentTx(tx *sql.Tx, adminId int, wsId string, requirementId, parentId int, reqType string) error {
	if parentId == 0 {
		return nil
	}
	if parentId == requirementId {
		return ErrRequirementRelationInvalid
	}
	parent, err := msql.RawValues(define.Postgres, `SELECT id FROM requirements
		WHERE admin_id = $1 AND workspace_id = $2 AND id = $3 AND type = $4 LIMIT 1`, tx,
		adminId, cast.ToInt(wsId), parentId, define.TypeKeyRequirement)
	if err != nil {
		return err
	}
	if len(parent) == 0 {
		return ErrRequirementRelationInvalid
	}
	if requirementId == 0 || reqType == define.TypeKeyDefect {
		return nil
	}
	rows, err := msql.RawValues(define.Postgres, `WITH RECURSIVE descendants(id, path) AS (
		SELECT id, ARRAY[id] FROM requirements WHERE admin_id = $1 AND workspace_id = $2 AND parent_id = $3
		UNION ALL
		SELECT r.id, d.path || r.id FROM requirements r JOIN descendants d ON r.parent_id = d.id
		WHERE r.admin_id = $1 AND r.workspace_id = $2 AND NOT r.id = ANY(d.path)
	) SELECT id FROM descendants WHERE id = $4 LIMIT 1`, tx, adminId, cast.ToInt(wsId), requirementId, parentId)
	if err != nil {
		return err
	}
	if len(rows) > 0 {
		return ErrRequirementRelationInvalid
	}
	return nil
}

func updateRequirementParent(adminId int, wsId string, requirementId, parentId, userId int) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockRequirementHierarchyTx(tx, adminId, cast.ToInt(wsId)); err != nil {
		return err
	}
	if err = updateRequirementParentTx(tx, adminId, wsId, requirementId, parentId, userId); err != nil {
		return err
	}
	return tx.Commit()
}

func updateRequirementParentTx(tx *sql.Tx, adminId int, wsId string, requirementId, parentId, userId int) error {
	if tx == nil {
		return errors.New("requirement parent update requires transaction")
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT type, parent_id, category_id FROM requirements
		WHERE admin_id = $1 AND workspace_id = $2 AND id = $3 FOR UPDATE`, tx,
		adminId, cast.ToInt(wsId), requirementId)
	if err != nil {
		logs.Error("lock requirement before parent update failed: %s", err.Error())
		return err
	}
	if len(rows) == 0 {
		return ErrRequirementNotFound
	}
	oldParentId := cast.ToInt(rows[0][`parent_id`])
	if oldParentId == parentId {
		return nil
	}
	itemType := rows[0][`type`]
	if err = validateParentTx(tx, adminId, wsId, requirementId, parentId, itemType); err != nil {
		return err
	}
	if itemType == define.TypeKeyRequirement && parentId > 0 {
		parentCategoryId, queryErr := requirementParentCategoryTx(tx, adminId, cast.ToInt(wsId), parentId)
		if queryErr != nil {
			return queryErr
		}
		if err = validateRequirementParentCategory(itemType, cast.ToInt(rows[0][`category_id`]), parentCategoryId); err != nil {
			return err
		}
	}
	now := tool.Time2Int()
	if itemType == define.TypeKeyRequirement {
		_, err = msql.RawExec(define.Postgres, `UPDATE requirements SET parent_id = $1, updated_at = $2
			WHERE admin_id = $3 AND workspace_id = $4 AND id = $5`, tx,
			parentId, now, adminId, cast.ToInt(wsId), requirementId)
	} else {
		_, err = msql.RawExec(define.Postgres, `UPDATE requirements SET parent_id = $1, category_id = 0, updated_at = $2
			WHERE admin_id = $3 AND workspace_id = $4 AND id = $5`, tx,
			parentId, now, adminId, cast.ToInt(wsId), requirementId)
	}
	if err != nil {
		logs.Error("update requirement parent failed: %s", err.Error())
		return err
	}
	if err = createHistoryTx(tx, adminId, cast.ToInt(wsId), requirementId, userId,
		define.HistoryActionFieldUpdated, `parent_id`, cast.ToString(oldParentId), cast.ToString(parentId)); err != nil {
		return err
	}
	return nil
}

func getInitialStatusId(adminId int, wsId, reqType string) (int, error) {
	field, _ := settingsStatusFields(reqType)
	settingsM := msql.Model(define.TableWorkspaceSettings, define.Postgres)
	statusId, err := settingsM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, wsId},
	}).Value(field)
	if err != nil {
		logs.Error("getInitialStatusId sql:%s, err:%s", settingsM.GetLastSql(), err.Error())
		return 0, err
	}
	if cast.ToInt(statusId) > 0 {
		return cast.ToInt(statusId), nil
	}

	statusM := msql.Model(define.TableWorkItemStatuses, define.Postgres)
	fallbackId, err := statusM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, wsId},
		{`type_key`, reqType},
		{`is_default`, `1`},
	}).Order(`sort_order asc, id asc`).Value(`id`)
	if err != nil {
		logs.Error("getInitialStatusId fallback sql:%s, err:%s", statusM.GetLastSql(), err.Error())
		return 0, err
	}
	if cast.ToInt(fallbackId) > 0 {
		return cast.ToInt(fallbackId), nil
	}
	return 0, fmt.Errorf("initial status not found for workspace %s", wsId)
}

func getStatusById(adminId int, wsId, reqType string, statusId int) (msql.Params, error) {
	m := msql.Model(define.TableWorkItemStatuses, define.Postgres)
	row, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, wsId},
		{`type_key`, reqType},
		{`id`, cast.ToString(statusId)},
	}).Find()
	if err != nil {
		logs.Error("getStatusById sql:%s, err:%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	return row, nil
}

func validatePriorityID(adminId int, wsId, reqType string, priorityId int) error {
	m := msql.Model(define.TableWorkItemPriorities, define.Postgres)
	id, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, wsId},
		{`type_key`, reqType},
		{`id`, cast.ToString(priorityId)},
	}).Value(`id`)
	if err != nil {
		logs.Error("validatePriorityID sql:%s, err:%s", m.GetLastSql(), err.Error())
		return err
	}
	if cast.ToInt(id) == 0 {
		return ErrPriorityNotFound
	}
	return nil
}

// ensureStatusTransition validates whether a status transition is legal; the current version does not restrict transitions and always allows them.
func ensureStatusTransition(adminId int, wsId, reqType string, fromStatusId, toStatusId int) error {
	if fromStatusId == 0 || toStatusId == 0 || fromStatusId == toStatusId {
		return nil
	}
	return nil
}

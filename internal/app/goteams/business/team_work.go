// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/permission"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// ErrTeamWorkMemberSettingsInvalid The member visibility setting exceeds the current user's permission scope or there is data duplication.
var ErrTeamWorkMemberSettingsInvalid = errors.New("invalid team work member settings")

// TeamWorkMemberSetting Indicates the personal display settings of a member, where the slicing order is also the display order.
type TeamWorkMemberSetting struct {
	UserId  int  `json:"userId"`
	Visible bool `json:"visible"`
}

// Team work planned-date filter modes: they decide which items a cycle tab contains.
const (
	TeamWorkModeDay         = `day`         // planned range overlaps the given day
	TeamWorkModeUnscheduled = `unscheduled` // no planned date at all and not in a final status
	TeamWorkModeScheduled   = `scheduled`   // planned start or planned end is set
)

// ListTeamWorkModel 定义团队工作列表的查询上下文。
type ListTeamWorkModel struct {
	AdminId int
	UserId  int
	Day     time.Time
	Mode    string
}

// TeamWorkItemModel 定义团队工作聚合条目及详情访问状态。
type TeamWorkItemModel struct {
	Id                int
	WorkspaceId       int
	StatusId          int
	AssigneeId        int
	CreatedBy         int
	MemberSortOrder   int
	PlannedStartDate  int64
	PlannedEndDate    int64
	CreatedAt         int64
	UpdatedAt         int64
	Priority          *int
	StatusSortOrder   *int
	PrioritySortOrder *int
	Title             string
	Type              string
	WorkspaceName     string
	StatusName        string
	StatusColor       string
	PriorityName      string
	PriorityColor     string
	AssigneeName      string
	AssigneeUsername  string
	AssigneeAvatar    string
	AssigneeIds       []int
	AssigneeNames     []string
	AssigneeAvatars   []string
	AssigneeUsernames []string
	IsDone            bool
	IsCompleted       bool
	CanView           bool
	CanEdit           bool
	CanOpenDetail     bool
}

// ListTeamWork Return the team work items within the current role's viewing range, sorted by date.
// mode selects the planned date filter; only TeamWorkModeDay narrows the result to day.
func ListTeamWork(model ListTeamWorkModel) ([]TeamWorkItemModel, error) {
	adminId, userId, day, mode := model.AdminId, model.UserId, model.Day, model.Mode
	members, err := ListTeamWorkMembers(adminId, userId)
	if err != nil {
		return nil, err
	}
	visibleUserIds := make([]int, 0, len(members))
	for _, member := range members {
		if cast.ToInt(member[`is_visible`]) == 1 {
			visibleUserIds = append(visibleUserIds, cast.ToInt(member[`id`]))
		}
	}
	if len(visibleUserIds) == 0 {
		return []TeamWorkItemModel{}, nil
	}

	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)
	// Done-state check: match the workspace's configured final status set per work item type
	isDoneSQL := `CASE WHEN r.type = 'defect' THEN COALESCE(r.status_id = ANY(
			string_to_array(NULLIF(settings.defect_final_status_ids, ''), ',')::INTEGER[]), FALSE)
			ELSE COALESCE(r.status_id = ANY(
			string_to_array(NULLIF(settings.requirement_final_status_ids, ''), ',')::INTEGER[]), FALSE)
		END`
	// Date filter: by default select scheduled items overlapping [$2,$3); unscheduled selects
	// items whose planned start and end are both empty ($2=$3=0) and that are not done yet.
	// The scheduled tab opens the window to its full range, which reduces the same filter to
	// "planned start or planned end is set".
	// Every mode must keep $2 and $3 referenced: PostgreSQL cannot infer the type of a
	// parameter no branch uses and fails the whole query with
	// "could not determine data type of parameter $n".
	dateFilter := `AND (r.planned_start_date > 0 OR r.planned_end_date > 0)
			AND (r.planned_start_date = 0 OR r.planned_start_date < $3)
			AND (r.planned_end_date = 0 OR r.planned_end_date >= $2)`
	startParam, endParam := dayStart.Unix(), dayEnd.Unix()
	switch mode {
	case TeamWorkModeUnscheduled:
		dateFilter = `AND r.planned_start_date = $2 AND r.planned_end_date = $3 AND NOT ` + isDoneSQL
		startParam, endParam = 0, 0
	case TeamWorkModeScheduled:
		startParam, endParam = 0, math.MaxInt64
	}
	permissionSQL := func(action string) string {
		return fmt.Sprintf(`CASE WHEN $6::BOOLEAN OR EXISTS (
			SELECT 1 FROM workspace_members wm
			JOIN role_permissions rp ON rp.admin_id = wm.admin_id AND rp.role_id = wm.role_id
				AND rp.permission_key = (r.type || ':%s')
			WHERE wm.admin_id = $1 AND wm.workspace_id = r.workspace_id AND wm.user_id = $5
		) THEN 1 ELSE 0 END`, action)
	}
	query := fmt.Sprintf(`SELECT r.id, r.workspace_id, r.title, r.type, r.priority, r.status_id,
		au.user_id AS assignee_id, r.planned_start_date, r.planned_end_date, r.created_by,
		r.created_at, r.updated_at,
		w.name AS workspace_name, s.name AS status_name, s.color AS status_color,
		s.sort_order AS status_sort_order,
		p.name AS priority_name, p.color AS priority_color, p.sort_order AS priority_sort_order,
		u.name AS assignee_name, u.username AS assignee_username, u.avatar AS assignee_avatar,
		COALESCE(_assignee.assignee_ids, '[]') AS assignee_ids,
		COALESCE(_assignee.assignee_names, '[]') AS assignee_names,
		COALESCE(_assignee.assignee_avatars, '[]') AS assignee_avatars,
		COALESCE(_assignee.assignee_usernames, '[]') AS assignee_usernames,
		array_position($4::INTEGER[], au.user_id) AS member_sort_order,
		%s AS is_done,
		%s AS is_completed,
		%s AS can_view, %s AS can_edit
		FROM requirements r
		JOIN workspaces w ON w.id = r.workspace_id AND w.admin_id = r.admin_id
		JOIN unnest(r.assignee_ids) AS au(user_id) ON true
		JOIN users u ON u.id = au.user_id AND u.admin_id = r.admin_id
		LEFT JOIN LATERAL (
			SELECT COALESCE(json_agg(assigned.user_id ORDER BY assigned.ordinality), '[]'::json)::text AS assignee_ids,
				COALESCE(json_agg(COALESCE(u2.name, ru2.name, '') ORDER BY assigned.ordinality), '[]'::json)::text AS assignee_names,
				COALESCE(json_agg(COALESCE(u2.avatar, ru2.avatar, '') ORDER BY assigned.ordinality), '[]'::json)::text AS assignee_avatars,
				COALESCE(json_agg(COALESCE(u2.username, '') ORDER BY assigned.ordinality), '[]'::json)::text AS assignee_usernames
			FROM unnest(r.assignee_ids) WITH ORDINALITY AS assigned(user_id, ordinality)
			LEFT JOIN users u2 ON u2.id = assigned.user_id AND u2.admin_id = r.admin_id
			LEFT JOIN removed_users ru2 ON ru2.user_id = assigned.user_id AND ru2.admin_id = r.admin_id
		) _assignee ON true
		LEFT JOIN workspaces_settings settings ON settings.workspace_id = r.workspace_id
			AND settings.admin_id = r.admin_id
		LEFT JOIN workspace_statuses s ON s.id = r.status_id AND s.admin_id = r.admin_id
			AND s.workspace_id = r.workspace_id AND s.type_key = r.type
		LEFT JOIN workspace_priorities p ON p.id = r.priority AND p.admin_id = r.admin_id
			AND p.workspace_id = r.workspace_id AND p.type_key = r.type
		WHERE r.admin_id = $1
			AND r.type IN ('requirement', 'defect')
			%s
			AND au.user_id = ANY($4::INTEGER[])
		ORDER BY array_position($4::INTEGER[], au.user_id),
			NULLIF(r.planned_start_date, 0) ASC NULLS LAST,
			COALESCE(p.sort_order, 2147483647) ASC, r.id DESC`,
		isDoneSQL, strings.ReplaceAll(isDoneSQL, "_final_status_ids", "_completed_status_ids"), permissionSQL(`view`), permissionSQL(`edit`), dateFilter)
	isOwner := IsTenantOwner(adminId, userId)
	items, err := msql.RawValues(
		define.Postgres,
		query,
		nil,
		adminId,
		startParam,
		endParam,
		pq.Array(visibleUserIds),
		userId,
		isOwner,
	)
	if err != nil {
		logs.Error("list team work query failed, err:%s", err.Error())
		return nil, err
	}
	result := make([]TeamWorkItemModel, 0, len(items))
	for _, row := range items {
		item, err := teamWorkItemFromRow(row)
		if err != nil {
			logs.Error("parse team work item failed, err:%s", err.Error())
			return nil, err
		}
		result = append(result, item)
	}
	if err := fillTeamWorkDetailAccess(result, adminId, userId, isOwner); err != nil {
		return nil, err
	}
	return result, nil
}

func teamWorkItemFromRow(row msql.Params) (TeamWorkItemModel, error) {
	item := TeamWorkItemModel{
		Title:            row[`title`],
		Type:             row[`type`],
		WorkspaceName:    row[`workspace_name`],
		StatusName:       row[`status_name`],
		StatusColor:      row[`status_color`],
		PriorityName:     row[`priority_name`],
		PriorityColor:    row[`priority_color`],
		AssigneeName:     row[`assignee_name`],
		AssigneeUsername: row[`assignee_username`],
		AssigneeAvatar:   row[`assignee_avatar`],
	}
	rawCreator, exists := row[`created_by`]
	if !exists {
		return TeamWorkItemModel{}, errors.New("missing team work created_by")
	}
	// 外部邀填工单允许没有内部创建人，查询返回的空值按无创建人处理。
	if rawCreator != "" {
		creatorId, err := strconv.Atoi(rawCreator)
		if err != nil {
			return TeamWorkItemModel{}, fmt.Errorf("parse team work created_by: %w", err)
		}
		item.CreatedBy = creatorId
	}
	for _, field := range []struct {
		column string
		value  *int
	}{
		{`id`, &item.Id},
		{`workspace_id`, &item.WorkspaceId},
		{`status_id`, &item.StatusId},
		{`assignee_id`, &item.AssigneeId},
		{`member_sort_order`, &item.MemberSortOrder},
	} {
		value, err := strconv.Atoi(row[field.column])
		if err != nil {
			return TeamWorkItemModel{}, fmt.Errorf("parse team work %s: %w", field.column, err)
		}
		*field.value = value
	}
	for _, field := range []struct {
		column string
		value  *int64
	}{
		{`planned_start_date`, &item.PlannedStartDate},
		{`planned_end_date`, &item.PlannedEndDate},
		{`created_at`, &item.CreatedAt},
		{`updated_at`, &item.UpdatedAt},
	} {
		value, err := strconv.ParseInt(row[field.column], 10, 64)
		if err != nil {
			return TeamWorkItemModel{}, fmt.Errorf("parse team work %s: %w", field.column, err)
		}
		*field.value = value
	}
	for _, field := range []struct {
		column string
		value  **int
	}{
		{`priority`, &item.Priority},
		{`status_sort_order`, &item.StatusSortOrder},
		{`priority_sort_order`, &item.PrioritySortOrder},
	} {
		raw, exists := row[field.column]
		if !exists {
			return TeamWorkItemModel{}, fmt.Errorf("missing team work %s", field.column)
		}
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err != nil {
			return TeamWorkItemModel{}, fmt.Errorf("parse team work %s: %w", field.column, err)
		}
		*field.value = &value
	}
	for _, field := range []struct {
		column string
		value  *bool
	}{
		{`is_done`, &item.IsDone},
		{`is_completed`, &item.IsCompleted},
		{`can_view`, &item.CanView},
		{`can_edit`, &item.CanEdit},
	} {
		value, err := strconv.ParseBool(row[field.column])
		if err != nil {
			return TeamWorkItemModel{}, fmt.Errorf("parse team work %s: %w", field.column, err)
		}
		*field.value = value
	}
	if err := json.Unmarshal([]byte(row[`assignee_ids`]), &item.AssigneeIds); err != nil {
		return TeamWorkItemModel{}, fmt.Errorf("parse team work assignee_ids: %w", err)
	}
	if err := json.Unmarshal([]byte(row[`assignee_names`]), &item.AssigneeNames); err != nil {
		return TeamWorkItemModel{}, fmt.Errorf("parse team work assignee_names: %w", err)
	}
	if err := json.Unmarshal([]byte(row[`assignee_avatars`]), &item.AssigneeAvatars); err != nil {
		return TeamWorkItemModel{}, fmt.Errorf("parse team work assignee_avatars: %w", err)
	}
	if err := json.Unmarshal([]byte(row[`assignee_usernames`]), &item.AssigneeUsernames); err != nil {
		return TeamWorkItemModel{}, fmt.Errorf("parse team work assignee_usernames: %w", err)
	}
	return item, nil
}

func fillTeamWorkDetailAccess(items []TeamWorkItemModel, adminId, userId int, isOwner bool) error {
	type workspaceScope struct {
		scope     string
		memberIds []int
	}
	scopes := make(map[int]workspaceScope)
	for i := range items {
		item := &items[i]
		if !item.CanView {
			continue
		}
		if isOwner || item.CreatedBy == userId {
			item.CanOpenDetail = true
			continue
		}
		scope, loaded := scopes[item.WorkspaceId]
		if !loaded {
			value, memberIds, err := getUserProjectDataViewScope(adminId, strconv.Itoa(item.WorkspaceId), userId)
			if err != nil {
				return err
			}
			scope = workspaceScope{scope: value, memberIds: memberIds}
			scopes[item.WorkspaceId] = scope
		}
		item.CanOpenDetail = requirementDataViewVisibleInScope(userId, item.CreatedBy, item.AssigneeIds, scope.scope, scope.memberIds)
	}
	return nil
}

// ListTeamWorkMembers Return the member and individual display settings within the current user's permission scope.
func ListTeamWorkMembers(adminId int, userId int) ([]msql.Params, error) {
	visibleUserIds, canViewAll, err := teamWorkVisibleUserIds(adminId, userId)
	if err != nil {
		return nil, err
	}
	if !canViewAll && len(visibleUserIds) == 0 {
		return []msql.Params{}, nil
	}

	rows, err := msql.RawValues(define.Postgres, `SELECT u.id, u.name, u.username, u.avatar,
		COALESCE(settings.is_visible, 1) AS is_visible,
		COALESCE(settings.sort_order, 2147483647) AS sort_order
		FROM users u
		LEFT JOIN user_team_work_member_settings settings
			ON settings.admin_id = u.admin_id AND settings.user_id = $2
			AND settings.member_user_id = u.id
		WHERE u.admin_id = $1 AND COALESCE(u.status, 1) = 1 AND u.deleted_at = 0
			AND ($3::BOOLEAN OR u.id = ANY($4::INTEGER[]))
		ORDER BY COALESCE(settings.sort_order, 2147483647), u.id`, nil,
		adminId, userId, canViewAll, pq.Array(visibleUserIds))
	if err != nil {
		logs.Error("list team work members failed, err:%s", err.Error())
		return nil, err
	}
	return rows, nil
}

// ListTeamWorkMemberCandidates returns the picker source data for the team-work display list:
// the organization tree plus the members the current role may see, each flagged with whether it
// is already shown. The role's team work view scope is the upper bound, so members outside it are
// never offered (ListTeamWorkMembers already drops disabled and soft-deleted accounts).
func ListTeamWorkMemberCandidates(adminId int, userId int) (ListMemberCandidatesModel, error) {
	enterpriseName, err := GetEnterpriseName(adminId)
	if err != nil {
		return ListMemberCandidatesModel{}, err
	}
	departments, err := ListDepartments(adminId)
	if err != nil {
		return ListMemberCandidatesModel{}, err
	}
	roles, err := ListEnterpriseRoleBriefs(adminId)
	if err != nil {
		return ListMemberCandidatesModel{}, err
	}
	members, err := ListTeamWorkMembers(adminId, userId)
	if err != nil {
		return ListMemberCandidatesModel{}, err
	}
	joined := make(map[int]bool, len(members))
	allowed := make(map[int]bool, len(members))
	for _, member := range members {
		id := cast.ToInt(member[`id`])
		allowed[id] = true
		joined[id] = cast.ToInt(member[`is_visible`]) == 1
	}
	all, err := ListEnterpriseMembers(ListEnterpriseMembersModel{AdminId: adminId})
	if err != nil {
		return ListMemberCandidatesModel{}, err
	}

	model := ListMemberCandidatesModel{
		EnterpriseName:  enterpriseName,
		Departments:     departments,
		EnterpriseRoles: roles,
		Items:           make([]MemberCandidateModel, 0, len(all.Items)),
	}
	for _, member := range all.Items {
		if !allowed[member.Id] {
			continue
		}
		model.Items = append(model.Items, MemberCandidateModel{
			Id:            member.Id,
			Username:      member.Username,
			Name:          member.Name,
			Avatar:        member.Avatar,
			RoleId:        member.RoleId,
			DepartmentIds: member.DepartmentIds,
			Joined:        joined[member.Id],
		})
	}
	return model, nil
}

// UpdateTeamWorkMemberSettings Save the display order and switch settings of members within the current user's permission scope.
func UpdateTeamWorkMemberSettings(adminId int, userId int, settings []TeamWorkMemberSetting) error {
	allowedMembers, err := ListTeamWorkMembers(adminId, userId)
	if err != nil {
		return err
	}
	if len(settings) != len(allowedMembers) {
		return ErrTeamWorkMemberSettingsInvalid
	}

	allowedIds := make(map[int]bool, len(allowedMembers))
	for _, member := range allowedMembers {
		allowedIds[cast.ToInt(member[`id`])] = true
	}
	seen := make(map[int]bool, len(settings))
	for _, setting := range settings {
		if !allowedIds[setting.UserId] || seen[setting.UserId] {
			return ErrTeamWorkMemberSettingsInvalid
		}
		seen[setting.UserId] = true
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin team work member settings transaction failed, err:%s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM user_team_work_member_settings
		WHERE admin_id = $1 AND user_id = $2`, tx, adminId, userId); err != nil {
		logs.Error("delete team work member settings failed, err:%s", err.Error())
		return err
	}

	now := tool.Time2Int()
	for index, setting := range settings {
		isVisible := 0
		if setting.Visible {
			isVisible = 1
		}
		if _, err = msql.RawExec(define.Postgres, `INSERT INTO user_team_work_member_settings
			(admin_id, user_id, member_user_id, is_visible, sort_order, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $6)`, tx,
			adminId, userId, setting.UserId, isVisible, index, now); err != nil {
			logs.Error("insert team work member setting failed, err:%s", err.Error())
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit team work member settings failed, err:%s", err.Error())
	}
	return err
}

func teamWorkVisibleUserIds(adminId int, userId int) ([]int, bool, error) {
	if IsTenantOwner(adminId, userId) {
		return nil, true, nil
	}

	roles, err := msql.RawValues(define.Postgres, `SELECT r.id, r.team_work_view_scope
		FROM users u
		JOIN roles r ON r.id = u.role_id AND r.admin_id = u.admin_id
			AND r.scope = $3
		WHERE u.admin_id = $1 AND u.id = $2`, nil, adminId, userId, permission.ScopeEnterprise)
	if err != nil {
		logs.Error("get team work role scope failed, err:%s", err.Error())
		return nil, false, err
	}
	if len(roles) == 0 {
		return nil, false, nil
	}

	role := roles[0]
	switch role[`team_work_view_scope`] {
	case TeamWorkViewScopeAll:
		return nil, true, nil
	case TeamWorkViewScopePartial:
		ids, queryErr := expandTeamWorkTargets(adminId, cast.ToInt(role[`id`]))
		if queryErr != nil {
			return nil, false, queryErr
		}
		return ids, false, nil
	default:
		return []int{userId}, false, nil
	}
}

// expandTeamWorkTargets resolves a partial view scope into concrete member ids:
// user entries keep members that still exist, dept entries cover the department and
// all of its sub-departments, and role entries cover every member holding that role.
func expandTeamWorkTargets(adminId int, roleId int) ([]int, error) {
	targets, err := getRoleTeamWorkTargets(adminId, roleId)
	if err != nil {
		return nil, err
	}

	directIds := make([]int, 0, len(targets))
	deptIds := make([]int, 0, len(targets))
	roleIds := make([]int, 0, len(targets))
	for _, target := range targets {
		switch target.Type {
		case TeamWorkTargetTypeDept:
			deptIds = append(deptIds, target.Id)
		case TeamWorkTargetTypeRole:
			roleIds = append(roleIds, target.Id)
		default:
			directIds = append(directIds, target.Id)
		}
	}

	seen := make(map[int]bool)
	ids := make([]int, 0)
	collect := func(rows []msql.Params, key string) {
		for _, row := range rows {
			id := cast.ToInt(row[key])
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}

	if len(directIds) > 0 {
		rows, queryErr := msql.RawValues(define.Postgres, `SELECT id FROM users
			WHERE admin_id = $1 AND deleted_at = 0 AND id = ANY($2)`, nil, adminId, pq.Array(directIds))
		if queryErr != nil {
			logs.Error("list team work direct members failed, err:%s", queryErr.Error())
			return nil, queryErr
		}
		collect(rows, `id`)
	}
	if len(roleIds) > 0 {
		rows, queryErr := msql.RawValues(define.Postgres, `SELECT id FROM users
			WHERE admin_id = $1 AND deleted_at = 0 AND role_id = ANY($2)`, nil, adminId, pq.Array(roleIds))
		if queryErr != nil {
			logs.Error("list team work role members failed, err:%s", queryErr.Error())
			return nil, queryErr
		}
		collect(rows, `id`)
	}
	if len(deptIds) > 0 {
		expandedDeptIds, expandErr := expandDepartmentSubtreeIds(adminId, deptIds)
		if expandErr != nil {
			return nil, expandErr
		}
		rows, queryErr := msql.RawValues(define.Postgres, `SELECT user_id FROM user_departments
			WHERE admin_id = $1 AND department_id = ANY($2)`, nil, adminId, pq.Array(expandedDeptIds))
		if queryErr != nil {
			logs.Error("list team work department members failed, err:%s", queryErr.Error())
			return nil, queryErr
		}
		collect(rows, `user_id`)
	}

	sort.Ints(ids)
	return ids, nil
}

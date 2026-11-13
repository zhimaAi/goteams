// Copyright © 2026 Wuhan Sesame Small Customer Service Network Co., Ltd.

package business

import (
	"database/sql"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_storage"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// Expert group errors. They mirror the pipeline error semantics so the frontend and the
// client can reuse the same i18n / forbidden handling.
var (
	ErrExpertGroupNotFound            = errors.New(`expert group not found`)
	ErrExpertGroupManageForbidden     = errors.New(`expert group manage forbidden`)
	ErrExpertGroupUseForbidden        = errors.New(`expert group use forbidden`)
	ErrExpertGroupScopeInvalid        = errors.New(`expert group scope invalid`)
	ErrExpertGroupMemberInvalid       = errors.New(`expert group member invalid`)
	ErrExpertGroupNoLeader            = errors.New(`expert group requires exactly one leader`)
	ErrExpertGroupNoAgent             = errors.New(`expert group requires at least one agent`)
	ErrExpertGroupAvatarInvalid       = errors.New(`expert group avatar invalid`)
	ErrExpertGroupAvatarTooLarge      = errors.New(`expert group avatar too large`)
	ErrExpertGroupAgentPromptRequired = errors.New(`expert group agent prompt required`)
)

// permExpertGroupManage is the RBAC permission key for expert group management. It reuses
// the pipeline management permission so a project admin who manages pipelines also manages
// expert groups, keeping the permission matrix consistent (需求 1896 决策：沿用 CanManage).
const permExpertGroupManage = `pipeline:manage`

// normalizeExpertGroupScope normalizes the scope, defaulting to all (expert groups are
// org-facing templates: a blank scope means everyone can use, unlike pipelines that
// historically defaulted to members).
func normalizeExpertGroupScope(scope string) string {
	scope = strings.ToLower(strings.TrimSpace(scope))
	switch scope {
	case define.PipelineScopeAll, define.PipelineScopeMembers, define.PipelineScopeDepartments:
		return scope
	case ``:
		return define.PipelineScopeAll
	default:
		return scope
	}
}

// CanManageExpertGroup checks whether the user can manage expert groups (same as pipelines).
func CanManageExpertGroup(adminId, userId int) bool {
	return HasPermission(adminId, userId, ``, permExpertGroupManage)
}

// ======================== Expert group CRUD ========================

// ListExpertGroups returns the expert group list (filtered by permission).
func ListExpertGroups(adminId, userId int, keyword, statusStr string, page, size int) ([]define.ExpertGroupListItem, int, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	offset := (page - 1) * size

	conditions := [][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`deleted_at`, `0`},
	}
	if keyword != `` {
		conditions = append(conditions, []string{`name`, `like`, `%` + keyword + `%`})
	}
	if statusStr != `` {
		conditions = append(conditions, []string{`status`, statusStr})
	}

	canManage := CanManageExpertGroup(adminId, userId)
	if !canManage {
		groupIds, err := listMemberExpertGroupIds(adminId, userId)
		if err != nil {
			return nil, 0, err
		}
		if len(groupIds) == 0 {
			return []define.ExpertGroupListItem{}, 0, nil
		}
		conditions = append(conditions, []string{`id`, `in`, strings.Join(groupIds, `,`)})
	}

	m := msql.Model(define.TableExpertGroup, define.Postgres)
	total, err := m.Where2(conditions).Count()
	if err != nil {
		logs.Error("ListExpertGroups count error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return nil, 0, err
	}

	listM := msql.Model(define.TableExpertGroup, define.Postgres)
	rows, err := listM.Where2(conditions).
		Field(`id, name, description, avatar, status, scope, updated_at`).
		Order(`id desc`).
		Limit(offset, size).
		Select()
	if err != nil {
		logs.Error("ListExpertGroups select error: sql=%s, err=%s", listM.GetLastSql(), err.Error())
		return nil, 0, err
	}

	list := make([]define.ExpertGroupListItem, 0, len(rows))
	for _, row := range rows {
		item := define.ExpertGroupListItem{
			ID:          cast.ToInt(row[`id`]),
			Name:        row[`name`],
			Description: row[`description`],
			Avatar:      row[`avatar`],
			Status:      cast.ToInt(row[`status`]),
			Scope:       normalizeExpertGroupScope(row[`scope`]),
			UpdatedAt:   cast.ToInt(row[`updated_at`]),
			CanManage:   canManage,
			CanUse:      true,
		}
		agentCount, err := countExpertGroupAgents(adminId, item.ID)
		if err != nil {
			logs.Error("ListExpertGroups count agents error: admin_id=%d, group_id=%d, err=%s", adminId, item.ID, err.Error())
		}
		memberCount, err := countExpertGroupUsers(adminId, item.ID)
		if err != nil {
			logs.Error("ListExpertGroups count users error: admin_id=%d, group_id=%d, err=%s", adminId, item.ID, err.Error())
		}
		departmentCount, err := countExpertGroupDepartments(adminId, item.ID)
		if err != nil {
			logs.Error("ListExpertGroups count departments error: admin_id=%d, group_id=%d, err=%s", adminId, item.ID, err.Error())
		}
		item.AgentCount = agentCount
		agents, err := getExpertGroupAgents(adminId, item.ID)
		if err != nil {
			logs.Error("ListExpertGroups load agents error: admin_id=%d, group_id=%d, err=%s", adminId, item.ID, err.Error())
		} else {
			item.Agents = agents
		}
		item.MemberCount = memberCount
		item.DepartmentCount = departmentCount
		if item.Scope != define.PipelineScopeAll {
			item.ScopeMembers, item.ScopeTotal, err = expertGroupScopeUserPreview(adminId, item.ID, item.Scope)
			if err != nil {
				logs.Error("ListExpertGroups scope preview error: admin_id=%d, group_id=%d, err=%s", adminId, item.ID, err.Error())
			}
		}
		list = append(list, item)
	}
	return list, total, nil
}

// listMemberExpertGroupIds returns the IDs of expert groups the user is authorized to use
// under the scope rules (all / members / departments), as strings for the SQL IN condition.
func listMemberExpertGroupIds(adminId, userId int) ([]string, error) {
	rows, err := msql.RawValues(define.Postgres, `
		SELECT g.id FROM expert_groups g
		WHERE g.admin_id=$1 AND g.deleted_at=0 AND (
			g.scope='all'
			OR (g.scope='members' AND EXISTS(
				SELECT 1 FROM expert_group_users u WHERE u.admin_id=g.admin_id AND u.expert_group_id=g.id AND u.user_id=$2))
			OR (g.scope='departments' AND EXISTS(
				SELECT 1 FROM expert_group_departments d
				JOIN user_departments ud ON ud.department_id=d.department_id AND ud.admin_id=d.admin_id
				WHERE d.admin_id=g.admin_id AND d.expert_group_id=g.id AND ud.user_id=$2))
		)
		ORDER BY g.id DESC`, nil, adminId, userId)
	if err != nil {
		logs.Error("listMemberExpertGroupIds error: admin_id=%d, err=%s", adminId, err.Error())
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, cast.ToString(row[`id`]))
	}
	return ids, nil
}

// GetExpertGroup returns expert group details.
func GetExpertGroup(adminId, groupId int) (define.ExpertGroupDetail, error) {
	m := msql.Model(define.TableExpertGroup, define.Postgres)
	row, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(groupId)},
		{`deleted_at`, `0`},
	}).Find()
	if err != nil {
		return define.ExpertGroupDetail{}, err
	}
	if len(row) == 0 {
		return define.ExpertGroupDetail{}, ErrExpertGroupNotFound
	}

	detail := define.ExpertGroupDetail{
		ID:          cast.ToInt(row[`id`]),
		AdminId:     cast.ToInt(row[`admin_id`]),
		Name:        row[`name`],
		Description: row[`description`],
		Avatar:      row[`avatar`],
		Status:      cast.ToInt(row[`status`]),
		Scope:       normalizeExpertGroupScope(row[`scope`]),
		CreatedBy:   cast.ToInt(row[`created_by`]),
		UpdatedAt:   cast.ToInt(row[`updated_at`]),
	}

	members, err := getExpertGroupUsers(adminId, groupId)
	if err != nil {
		return define.ExpertGroupDetail{}, err
	}
	detail.Members = members

	departments, err := getExpertGroupDepartments(adminId, groupId)
	if err != nil {
		return define.ExpertGroupDetail{}, err
	}
	detail.Departments = departments
	detail.DepartmentIds = make([]int, 0, len(departments))
	for _, dept := range departments {
		detail.DepartmentIds = append(detail.DepartmentIds, dept.DepartmentId)
	}

	agents, err := getExpertGroupAgents(adminId, groupId)
	if err != nil {
		return define.ExpertGroupDetail{}, err
	}
	detail.Agents = agents

	return detail, nil
}

// validateExpertGroupMembers validates that member IDs belong to the current tenant.
func validateExpertGroupMembers(adminId int, memberIds []int) ([]int, error) {
	normalized := make([]int, 0, len(memberIds))
	seen := make(map[int]bool, len(memberIds))
	for _, memberId := range memberIds {
		if memberId <= 0 || seen[memberId] {
			return nil, ErrExpertGroupMemberInvalid
		}
		seen[memberId] = true
		normalized = append(normalized, memberId)
	}
	if len(normalized) == 0 {
		return normalized, nil
	}
	rows, err := msql.RawValues(define.Postgres,
		`SELECT id FROM users WHERE admin_id = $1 AND deleted_at = 0 AND id = ANY($2)`, nil,
		adminId, pq.Array(normalized))
	if err != nil {
		logs.Error("validateExpertGroupMembers query error: admin_id=%d, err=%s", adminId, err.Error())
		return nil, err
	}
	if len(rows) != len(normalized) {
		logs.Error("validateExpertGroupMembers reject foreign members: admin_id=%d, want=%d, got=%d",
			adminId, len(normalized), len(rows))
		return nil, ErrExpertGroupMemberInvalid
	}
	return normalized, nil
}

// validateExpertGroupDepartments validates that department IDs belong to the current tenant.
func validateExpertGroupDepartments(adminId int, departmentIds []int) ([]int, error) {
	normalized := make([]int, 0, len(departmentIds))
	seen := make(map[int]bool, len(departmentIds))
	for _, departmentId := range departmentIds {
		if departmentId <= 0 || seen[departmentId] {
			return nil, ErrExpertGroupMemberInvalid
		}
		seen[departmentId] = true
		normalized = append(normalized, departmentId)
	}
	if len(normalized) == 0 {
		return normalized, nil
	}
	rows, err := msql.RawValues(define.Postgres,
		`SELECT id FROM departments WHERE admin_id = $1 AND id = ANY($2)`, nil,
		adminId, pq.Array(normalized))
	if err != nil {
		logs.Error("validateExpertGroupDepartments query error: admin_id=%d, err=%s", adminId, err.Error())
		return nil, err
	}
	if len(rows) != len(normalized) {
		logs.Error("validateExpertGroupDepartments reject foreign departments: admin_id=%d, want=%d, got=%d",
			adminId, len(normalized), len(rows))
		return nil, ErrExpertGroupMemberInvalid
	}
	return normalized, nil
}

// validateExpertGroupScopeTargets normalizes and validates the scope and its targets.
func validateExpertGroupScopeTargets(adminId int, scope string, memberIds, departmentIds []int) (string, []int, []int, error) {
	scope = normalizeExpertGroupScope(scope)
	if scope != define.PipelineScopeAll && scope != define.PipelineScopeMembers && scope != define.PipelineScopeDepartments {
		return ``, nil, nil, ErrExpertGroupScopeInvalid
	}
	memberIds, err := validateExpertGroupMembers(adminId, memberIds)
	if err != nil {
		return ``, nil, nil, err
	}
	departmentIds, err = validateExpertGroupDepartments(adminId, departmentIds)
	if err != nil {
		return ``, nil, nil, err
	}
	return scope, memberIds, departmentIds, nil
}

// validateExpertGroupAgents validates the orchestrated agents: at least one expert and
// exactly one leader. Member role values other than leader are normalized to member.
func validateExpertGroupAgents(agents []define.CreateExpertGroupAgentReq) ([]define.CreateExpertGroupAgentReq, error) {
	if len(agents) == 0 {
		return nil, ErrExpertGroupNoAgent
	}
	normalized := make([]define.CreateExpertGroupAgentReq, 0, len(agents))
	leaderCount := 0
	for i, agent := range agents {
		agent.Name = strings.TrimSpace(agent.Name)
		agent.Prompt = strings.TrimSpace(agent.Prompt)
		if agent.Name == `` {
			return nil, ErrExpertGroupNoAgent
		}
		if agent.Prompt == `` {
			return nil, ErrExpertGroupAgentPromptRequired
		}
		if agent.Role == define.ExpertGroupRoleLeader {
			leaderCount++
		} else {
			agent.Role = define.ExpertGroupRoleMember
		}
		if agent.SortOrder == 0 {
			agent.SortOrder = i
		}
		normalized = append(normalized, agent)
	}
	if leaderCount != 1 {
		return nil, ErrExpertGroupNoLeader
	}
	return normalized, nil
}

// CreateExpertGroup creates an expert group (including orchestrated agents and scope).
func CreateExpertGroup(adminId, userId int, input define.CreateExpertGroupReq) (int, error) {
	if !CanManageExpertGroup(adminId, userId) {
		return 0, ErrExpertGroupManageForbidden
	}
	agents, err := validateExpertGroupAgents(input.Agents)
	if err != nil {
		return 0, err
	}
	scope, memberIds, departmentIds, err := validateExpertGroupScopeTargets(adminId, input.Scope, input.MemberIds, input.DepartmentIds)
	if err != nil {
		return 0, err
	}
	if err := CheckQuota(adminId, define.QuotaAgent); err != nil {
		return 0, err
	}

	now := tool.Time2Int()
	if input.Status == 0 {
		input.Status = 1
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := msql.RawValues(define.Postgres, `INSERT INTO expert_groups(
		admin_id,name,description,avatar,scope,status,created_by,updated_by,created_at,updated_at,deleted_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$7,$8,$8,0) RETURNING id`, tx,
		adminId, input.Name, input.Description, input.Avatar, scope, input.Status, userId, now)
	if err != nil {
		return 0, err
	}
	groupId := cast.ToInt(rows[0][`id`])

	if err = saveExpertGroupScope(tx, adminId, groupId, userId, scope, memberIds, departmentIds, now); err != nil {
		return 0, err
	}
	if err = saveExpertGroupAgents(tx, adminId, groupId, agents, now); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return groupId, nil
}

// UpdateExpertGroup updates an expert group (basic info, scope and orchestrated agents).
func UpdateExpertGroup(adminId, userId, groupId int, input define.UpdateExpertGroupReq) error {
	if !CanManageExpertGroup(adminId, userId) {
		return ErrExpertGroupManageForbidden
	}
	if err := checkExpertGroupExists(adminId, groupId); err != nil {
		return err
	}
	agents, err := validateExpertGroupAgents(input.Agents)
	if err != nil {
		return err
	}
	scope, memberIds, departmentIds, err := validateExpertGroupScopeTargets(adminId, input.Scope, input.MemberIds, input.DepartmentIds)
	if err != nil {
		return err
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := tool.Time2Int()
	if _, err = msql.RawExec(define.Postgres, `UPDATE expert_groups SET
		name=$1,description=$2,avatar=$3,status=$4,scope=$5,updated_by=$6,updated_at=$7
		WHERE admin_id=$8 AND id=$9 AND deleted_at=0`, tx, input.Name, input.Description,
		input.Avatar, input.Status, scope, userId, now, adminId, groupId); err != nil {
		return err
	}
	if err = saveExpertGroupScope(tx, adminId, groupId, userId, scope, memberIds, departmentIds, now); err != nil {
		return err
	}
	if err = saveExpertGroupAgents(tx, adminId, groupId, agents, now); err != nil {
		return err
	}
	return tx.Commit()
}

// saveExpertGroupScope replaces the scope authorization rows (users and/or departments)
// of one expert group inside the caller's transaction.
func saveExpertGroupScope(tx *sql.Tx, adminId, groupId, userId int, scope string, memberIds, departmentIds []int, now int) error {
	if _, err := msql.RawExec(define.Postgres,
		`DELETE FROM expert_group_users WHERE admin_id=$1 AND expert_group_id=$2`,
		tx, adminId, groupId); err != nil {
		return err
	}
	if _, err := msql.RawExec(define.Postgres,
		`DELETE FROM expert_group_departments WHERE admin_id=$1 AND expert_group_id=$2`,
		tx, adminId, groupId); err != nil {
		return err
	}
	if scope == define.PipelineScopeMembers {
		for _, memberId := range memberIds {
			if _, err := msql.RawExec(define.Postgres, `INSERT INTO expert_group_users(
				admin_id,expert_group_id,user_id,created_by,created_at)
				VALUES($1,$2,$3,$4,$5)`, tx, adminId, groupId, memberId, userId, now); err != nil {
				return err
			}
		}
	}
	if scope == define.PipelineScopeDepartments {
		for _, departmentId := range departmentIds {
			if _, err := msql.RawExec(define.Postgres, `INSERT INTO expert_group_departments(
				admin_id,expert_group_id,department_id,created_by,created_at)
				VALUES($1,$2,$3,$4,$5)`, tx, adminId, groupId, departmentId, userId, now); err != nil {
				return err
			}
		}
	}
	return nil
}

// saveExpertGroupAgents replaces the orchestrated agents of one expert group inside the
// caller's transaction.
func saveExpertGroupAgents(tx *sql.Tx, adminId, groupId int, agents []define.CreateExpertGroupAgentReq, now int) error {
	if _, err := msql.RawExec(define.Postgres,
		`DELETE FROM expert_group_agents WHERE admin_id=$1 AND expert_group_id=$2`,
		tx, adminId, groupId); err != nil {
		return err
	}
	for _, agent := range agents {
		if _, err := msql.RawExec(define.Postgres, `INSERT INTO expert_group_agents(
			admin_id,expert_group_id,name,role,avatar,prompt,sort_order,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$8)`, tx, adminId, groupId, agent.Name,
			agent.Role, agent.Avatar, agent.Prompt, agent.SortOrder, now); err != nil {
			return err
		}
	}
	return nil
}

// DeleteExpertGroup soft-deletes an expert group.
func DeleteExpertGroup(adminId, userId, groupId int) error {
	if !CanManageExpertGroup(adminId, userId) {
		return ErrExpertGroupManageForbidden
	}
	now := tool.Time2Int()
	m := msql.Model(define.TableExpertGroup, define.Postgres)
	_, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(groupId)},
		{`deleted_at`, `0`},
	}).Update(msql.Datas{
		`deleted_at`: now,
		`status`:     0,
		`updated_by`: userId,
		`updated_at`: now,
	})
	return err
}

// ======================== Client-only API ========================

// GetClientExpertGroups returns the expert group snapshots available to the current user
// under the scope rules (all / members / departments), for client-side pull.
func GetClientExpertGroups(adminId, userId int) ([]define.ClientExpertGroupItem, error) {
	rows, err := msql.RawValues(define.Postgres, `
		SELECT g.id, g.name, g.description, g.avatar, g.scope, g.updated_at
		FROM expert_groups g
		WHERE g.admin_id=$1 AND g.deleted_at=0 AND g.status=1 AND (
			g.scope='all'
			OR (g.scope='members' AND EXISTS(
				SELECT 1 FROM expert_group_users u WHERE u.admin_id=g.admin_id AND u.expert_group_id=g.id AND u.user_id=$2))
			OR (g.scope='departments' AND EXISTS(
				SELECT 1 FROM expert_group_departments d
				JOIN user_departments ud ON ud.department_id=d.department_id AND ud.admin_id=d.admin_id
				WHERE d.admin_id=g.admin_id AND d.expert_group_id=g.id AND ud.user_id=$2))
		)
		ORDER BY g.id DESC`, nil, adminId, userId)
	if err != nil {
		logs.Error("GetClientExpertGroups error: admin_id=%d, err=%s", adminId, err.Error())
		return nil, err
	}
	list := make([]define.ClientExpertGroupItem, 0, len(rows))
	for _, row := range rows {
		groupId := cast.ToInt(row[`id`])
		agents, err := getExpertGroupAgents(adminId, groupId)
		if err != nil {
			return nil, err
		}
		list = append(list, define.ClientExpertGroupItem{
			ID:          groupId,
			Name:        row[`name`],
			Description: row[`description`],
			Avatar:      row[`avatar`],
			Scope:       normalizeExpertGroupScope(row[`scope`]),
			Agents:      agents,
			UpdatedAt:   cast.ToInt(row[`updated_at`]),
		})
	}
	return list, nil
}

// ======================== Expert group helpers ========================

func checkExpertGroupExists(adminId, groupId int) error {
	m := msql.Model(define.TableExpertGroup, define.Postgres)
	exist, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(groupId)},
		{`deleted_at`, `0`},
	}).Value(`id`)
	if err != nil {
		return err
	}
	if exist == `` {
		return ErrExpertGroupNotFound
	}
	return nil
}

func getExpertGroupUsers(adminId, groupId int) ([]define.PipelineMemberInfo, error) {
	m := msql.Model(define.TableExpertGroupUser, define.Postgres)
	rows, err := m.Where2([][]string{
		{`expert_group_users.admin_id`, cast.ToString(adminId)},
		{`expert_group_users.expert_group_id`, cast.ToString(groupId)},
	}).Join(define.TableUsers, `expert_group_users.user_id=users.id AND users.admin_id=expert_group_users.admin_id`, `left`).
		Field(`expert_group_users.user_id, users.name as user_name`).
		Select()
	if err != nil {
		return nil, err
	}
	list := make([]define.PipelineMemberInfo, 0, len(rows))
	for _, row := range rows {
		list = append(list, define.PipelineMemberInfo{
			UserId:   cast.ToInt(row[`user_id`]),
			UserName: row[`user_name`],
		})
	}
	return list, nil
}

func getExpertGroupDepartments(adminId, groupId int) ([]define.PipelineDepartmentInfo, error) {
	rows, err := msql.RawValues(define.Postgres, `
		SELECT d.id AS department_id, d.name AS department_name
		FROM expert_group_departments gd
		JOIN departments d ON d.id=gd.department_id AND d.admin_id=gd.admin_id
		WHERE gd.admin_id=$1 AND gd.expert_group_id=$2
		ORDER BY d.id`, nil, adminId, groupId)
	if err != nil {
		return nil, err
	}
	list := make([]define.PipelineDepartmentInfo, 0, len(rows))
	for _, row := range rows {
		list = append(list, define.PipelineDepartmentInfo{
			DepartmentId:   cast.ToInt(row[`department_id`]),
			DepartmentName: row[`department_name`],
		})
	}
	return list, nil
}

func getExpertGroupAgents(adminId, groupId int) ([]define.ExpertGroupAgentInfo, error) {
	m := msql.Model(define.TableExpertGroupAgent, define.Postgres)
	rows, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`expert_group_id`, cast.ToString(groupId)},
	}).Order(`sort_order asc, id asc`).Select()
	if err != nil {
		return nil, err
	}
	list := make([]define.ExpertGroupAgentInfo, 0, len(rows))
	for _, row := range rows {
		list = append(list, define.ExpertGroupAgentInfo{
			Name:      row[`name`],
			Role:      row[`role`],
			Avatar:    row[`avatar`],
			Prompt:    row[`prompt`],
			SortOrder: cast.ToInt(row[`sort_order`]),
		})
	}
	return list, nil
}

func countExpertGroupUsers(adminId, groupId int) (int, error) {
	m := msql.Model(define.TableExpertGroupUser, define.Postgres)
	return m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`expert_group_id`, cast.ToString(groupId)},
	}).Count()
}

func countExpertGroupDepartments(adminId, groupId int) (int, error) {
	m := msql.Model(define.TableExpertGroupDepartment, define.Postgres)
	return m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`expert_group_id`, cast.ToString(groupId)},
	}).Count()
}

func countExpertGroupAgents(adminId, groupId int) (int, error) {
	m := msql.Model(define.TableExpertGroupAgent, define.Postgres)
	return m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`expert_group_id`, cast.ToString(groupId)},
	}).Count()
}

// expertGroupScopeUserPreview returns the first scope member avatars and total headcount.
func expertGroupScopeUserPreview(adminId, groupId int, scope string) ([]define.PipelineScopeMember, int, error) {
	var query string
	if scope == define.PipelineScopeDepartments {
		query = `
			SELECT t.id, t.name, t.avatar, COUNT(*) OVER() AS total FROM (
				SELECT DISTINCT u.id, u.name, u.avatar
				FROM expert_group_departments gd
				JOIN user_departments ud ON ud.department_id=gd.department_id AND ud.admin_id=gd.admin_id
				JOIN users u ON u.id=ud.user_id AND u.admin_id=ud.admin_id
				WHERE gd.admin_id=$1 AND gd.expert_group_id=$2
			) t
			ORDER BY t.id
			LIMIT 6`
	} else {
		query = `
			SELECT u.id, u.name, u.avatar, COUNT(*) OVER() AS total
			FROM expert_group_users gu
			JOIN users u ON u.id=gu.user_id AND u.admin_id=gu.admin_id
			WHERE gu.admin_id=$1 AND gu.expert_group_id=$2
			ORDER BY u.id
			LIMIT 6`
	}
	rows, err := msql.RawValues(define.Postgres, query, nil, adminId, groupId)
	if err != nil {
		return nil, 0, err
	}
	members := make([]define.PipelineScopeMember, 0, len(rows))
	total := 0
	for _, row := range rows {
		if total == 0 {
			total = cast.ToInt(row[`total`])
		}
		members = append(members, define.PipelineScopeMember{
			UserId: cast.ToInt(row[`id`]),
			Name:   row[`name`],
			Avatar: row[`avatar`],
		})
	}
	return members, total, nil
}

// SaveExpertGroupAvatar validates and saves the expert group avatar, returning the
// accessible path generated by the storage system. It mirrors SavePipelineIcon so the
// cloud management page behaves consistently for both resource kinds.
func SaveExpertGroupAvatar(adminId, userId, groupId int, file *multipart.FileHeader) (string, error) {
	if !CanManageExpertGroup(adminId, userId) {
		return ``, ErrExpertGroupManageForbidden
	}
	if file == nil || file.Size <= 0 {
		return ``, ErrExpertGroupAvatarInvalid
	}
	if file.Size > maxExpertGroupAvatarSize {
		return ``, ErrExpertGroupAvatarTooLarge
	}
	if err := checkExpertGroupExists(adminId, groupId); err != nil {
		return ``, err
	}

	opened, err := file.Open()
	if err != nil {
		return ``, err
	}
	defer func() { _ = opened.Close() }()

	header := make([]byte, 512)
	n, readErr := io.ReadFull(opened, header)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return ``, readErr
	}
	ext := ``
	switch http.DetectContentType(header[:n]) {
	case `image/jpeg`:
		ext = `.jpg`
	case `image/png`:
		ext = `.png`
	default:
		return ``, ErrExpertGroupAvatarInvalid
	}
	if _, err = opened.Seek(0, io.SeekStart); err != nil {
		return ``, err
	}

	m := msql.Model(define.TableExpertGroup, define.Postgres)
	oldAvatar, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(groupId)},
		{`deleted_at`, `0`},
	}).Value(`avatar`)
	if err != nil {
		return ``, err
	}
	storedPath, err := lib_storage.M.Upload(adminId, lib_storage.BusinessExpertGroup, ext, opened)
	if err != nil {
		return ``, err
	}
	_, err = m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(groupId)},
		{`deleted_at`, `0`},
	}).Update(msql.Datas{`avatar`: storedPath, `updated_by`: userId, `updated_at`: tool.Time2Int()})
	if err != nil {
		_ = lib_storage.M.Delete(storedPath)
		return ``, err
	}
	if oldAvatar != `` && oldAvatar != storedPath && lib_storage.M.IsOwned(adminId, lib_storage.BusinessExpertGroup, oldAvatar) {
		if err := lib_storage.M.Delete(oldAvatar); err != nil {
			logs.Error("remove expert group avatar failed, err:%s", err.Error())
		}
	}
	return storedPath, nil
}

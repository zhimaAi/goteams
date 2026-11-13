// Copyright © 2026 Wuhan Sesame Small Customer Service Network Co., Ltd.

package business

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_storage"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// Pipeline error definitions.
var (
	ErrPipelineNotFound           = errors.New(`pipeline not found`)
	ErrPipelineNameRequired       = errors.New(`pipeline name required`)
	ErrPipelineWorkflowInvalid    = errors.New(`pipeline workflow invalid`)
	ErrPipelineManageForbidden    = errors.New(`pipeline manage forbidden`)
	ErrPipelineUseForbidden       = errors.New(`pipeline use forbidden`)
	ErrPipelineVersionConflict    = errors.New(`pipeline workflow version conflict`)
	ErrPipelineIconInvalid        = errors.New(`pipeline icon invalid`)
	ErrPipelineIconTooLarge       = errors.New(`pipeline icon too large`)
	ErrPipelineIconNotFound       = errors.New(`pipeline icon not found`)
	ErrPipelineMemberInvalid      = errors.New(`pipeline member invalid`)
	ErrPipelineScopeInvalid       = errors.New(`pipeline scope invalid`)
	ErrPipelineStepPromptRequired = errors.New(`pipeline step prompt required`)
)

// normalizePipelineScope maps an empty scope to the legacy members semantics so
// callers written before the scope column keep their exact previous behavior.
func normalizePipelineScope(scope string) string {
	if scope == `` {
		return define.PipelineScopeMembers
	}
	return scope
}

// validatePipelineStepPrompts requires every workflow step to carry a prompt: the cloud
// is the source of reusable team configs and the client runs each step with exactly this
// prompt, so a missing prompt would leave the client with a step it cannot execute.
func validatePipelineStepPrompts(steps []define.CreatePipelineStepReq) error {
	for _, step := range steps {
		if strings.TrimSpace(step.PromptContent) == `` {
			return ErrPipelineStepPromptRequired
		}
	}
	return nil
}

const maxPipelineIconSize int64 = 100 * 1024

// maxExpertGroupAvatarSize mirrors the pipeline icon constraint for expert group avatars.
const maxExpertGroupAvatarSize int64 = 100 * 1024

// ======================== Management permission checks ========================

// Pipeline permission key, kept consistent with those used by the route middleware withPerm.
const (
	permPipelineManage = `pipeline:manage`
)

// CanManagePipeline checks whether the user has pipeline management permission.
// It goes through RBAC uniformly: the super admin (userId == admin_id) is always allowed; others are
// decided by whether their enterprise role is granted pipeline:manage. Once a custom role is configured with this permission it takes effect; system preset role names are no longer hardcoded.
func CanManagePipeline(adminId, userId int) bool {
	return HasPermission(adminId, userId, ``, permPipelineManage)
}

// ======================== Pipeline CRUD ========================

// ListPipelines returns the pipeline list (filtered by permission).
func ListPipelines(adminId, userId int, keyword, statusStr string, page, size int) ([]define.PipelineListItem, int, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	offset := (page - 1) * size

	m := msql.Model(define.TablePipeline, define.Postgres)
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

	// Push the visibility filter down to SQL: members without management permission only see pipelines they are authorized for.
	// It must stay consistent with the count condition; otherwise total would exceed the actually visible rows and break frontend pagination.
	canManage := CanManagePipeline(adminId, userId)
	if !canManage {
		pipelineIds, err := listMemberPipelineIds(adminId, userId)
		if err != nil {
			return nil, 0, err
		}
		if len(pipelineIds) == 0 {
			return []define.PipelineListItem{}, 0, nil
		}
		conditions = append(conditions, []string{`id`, `in`, strings.Join(pipelineIds, `,`)})
	}

	// Count total
	total, err := m.Where2(conditions).Count()
	if err != nil {
		logs.Error("ListPipelines count error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return nil, 0, err
	}

	// Query list
	listM := msql.Model(define.TablePipeline, define.Postgres)
	rows, err := listM.Where2(conditions).
		Field(`id, name, description, color, icon, cli_type, workflow_version, status, scope, updated_at`).
		Order(`id desc`).
		Limit(offset, size).
		Select()
	if err != nil {
		logs.Error("ListPipelines select error: sql=%s, err=%s", listM.GetLastSql(), err.Error())
		return nil, 0, err
	}

	list := make([]define.PipelineListItem, 0, len(rows))
	for _, row := range rows {
		item := define.PipelineListItem{
			ID:              cast.ToInt(row[`id`]),
			Name:            row[`name`],
			Description:     row[`description`],
			Color:           row[`color`],
			Icon:            row[`icon`],
			CliType:         row[`cli_type`],
			WorkflowVersion: cast.ToInt(row[`workflow_version`]),
			Status:          cast.ToInt(row[`status`]),
			Scope:           normalizePipelineScope(row[`scope`]),
			UpdatedAt:       cast.ToInt(row[`updated_at`]),
			CanManage:       canManage,
		}
		// Count members, departments and steps; log on error instead of silently returning 0
		memberCount, err := countPipelineMembers(adminId, item.ID)
		if err != nil {
			logs.Error("ListPipelines count members error: admin_id=%d, pipeline_id=%d, err=%s", adminId, item.ID, err.Error())
		}
		stepCount, err := countPipelineSteps(adminId, item.ID)
		if err != nil {
			logs.Error("ListPipelines count steps error: admin_id=%d, pipeline_id=%d, err=%s", adminId, item.ID, err.Error())
		}
		departmentCount, err := countPipelineDepartments(adminId, item.ID)
		if err != nil {
			logs.Error("ListPipelines count departments error: admin_id=%d, pipeline_id=%d, err=%s", adminId, item.ID, err.Error())
		}
		item.MemberCount = memberCount
		item.StepCount = stepCount
		item.DepartmentCount = departmentCount
		if item.Scope != define.PipelineScopeAll {
			item.ScopeMembers, item.ScopeTotal, err = pipelineScopeUserPreview(adminId, item.ID, item.Scope)
			if err != nil {
				logs.Error("ListPipelines scope preview error: admin_id=%d, pipeline_id=%d, err=%s", adminId, item.ID, err.Error())
			}
		}
		// The list is already visibility-filtered, so all returned pipelines are usable by the current user
		item.CanUse = true
		list = append(list, item)
	}
	return list, total, nil
}

// listMemberPipelineIds returns the IDs of pipelines the user is authorized to use
// under the scope rules (all / members / departments), as strings for the SQL IN condition.
func listMemberPipelineIds(adminId, userId int) ([]string, error) {
	rows, err := msql.RawValues(define.Postgres, `
		SELECT p.id FROM pipelines p
		WHERE p.admin_id=$1 AND p.deleted_at=0 AND (
			p.scope='all'
			OR (p.scope='members' AND EXISTS(
				SELECT 1 FROM pipeline_members m WHERE m.admin_id=p.admin_id AND m.pipeline_id=p.id AND m.user_id=$2))
			OR (p.scope='departments' AND EXISTS(
				SELECT 1 FROM pipeline_departments d
				JOIN user_departments ud ON ud.department_id=d.department_id AND ud.admin_id=d.admin_id
				WHERE d.admin_id=p.admin_id AND d.pipeline_id=p.id AND ud.user_id=$2))
		)
		ORDER BY p.id DESC`, nil, adminId, userId)
	if err != nil {
		logs.Error("listMemberPipelineIds error: admin_id=%d, err=%s", adminId, err.Error())
		return nil, err
	}
	pipelineIds := make([]string, 0, len(rows))
	for _, row := range rows {
		pipelineIds = append(pipelineIds, cast.ToString(row[`id`]))
	}
	return pipelineIds, nil
}

// GetPipeline returns pipeline details.
func GetPipeline(adminId, pipelineId int) (define.PipelineDetail, error) {
	m := msql.Model(define.TablePipeline, define.Postgres)
	row, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(pipelineId)},
		{`deleted_at`, `0`},
	}).Find()
	if err != nil {
		return define.PipelineDetail{}, err
	}
	if len(row) == 0 {
		return define.PipelineDetail{}, ErrPipelineNotFound
	}

	detail := define.PipelineDetail{
		ID:              cast.ToInt(row[`id`]),
		AdminId:         cast.ToInt(row[`admin_id`]),
		Name:            row[`name`],
		Description:     row[`description`],
		Color:           row[`color`],
		Icon:            row[`icon`],
		CliType:         row[`cli_type`],
		WorkflowVersion: cast.ToInt(row[`workflow_version`]),
		Status:          cast.ToInt(row[`status`]),
		Scope:           normalizePipelineScope(row[`scope`]),
		CreatedBy:       cast.ToInt(row[`created_by`]),
		UpdatedAt:       cast.ToInt(row[`updated_at`]),
	}

	// Query members
	members, err := getPipelineMembers(adminId, pipelineId)
	if err != nil {
		return define.PipelineDetail{}, err
	}
	detail.Members = members

	// Query departments (scope=departments)
	departments, err := getPipelineDepartments(adminId, pipelineId)
	if err != nil {
		return define.PipelineDetail{}, err
	}
	detail.Departments = departments
	detail.DepartmentIds = make([]int, 0, len(departments))
	for _, dept := range departments {
		detail.DepartmentIds = append(detail.DepartmentIds, dept.DepartmentId)
	}

	// Query steps
	steps, err := getPipelineSteps(adminId, pipelineId)
	if err != nil {
		return define.PipelineDetail{}, err
	}
	detail.Steps = steps

	return detail, nil
}

// validatePipelineMembers validates that member IDs are valid and all belong to the current tenant, returning a normalized list (preserving input order).
//
// Skipping validation lets a crafted request write another tenant's userId into pipeline_members, creating cross-tenant "ghost members":
// such rows inflate the member count, participate in isPipelineMember checks, and leak foreign-tenant member names via the users JOIN in getPipelineMembers.
// Invalid values (<=0) and duplicate IDs are rejected outright — duplicates also hit the UNIQUE(admin_id,pipeline_id,user_id) constraint and return 500.
// One ANY query takes the intersection in bulk; if the counts don't match, some IDs don't belong to this tenant or no longer exist, so reject the whole
// request rather than silently dropping them, to avoid masking frontend bugs or malicious requests with "saved successfully but fewer members".
//
// Here we only check tenant membership, not users.status: if a disabled member were treated as invalid, an admin editing an existing pipeline would
// never be able to save because the form pre-filled that member; whether a member can actually use the pipeline is guarded by the execution-chain auth.
func validatePipelineMembers(adminId int, memberIds []int) ([]int, error) {
	normalized := make([]int, 0, len(memberIds))
	seen := make(map[int]bool, len(memberIds))
	for _, memberId := range memberIds {
		if memberId <= 0 || seen[memberId] {
			return nil, ErrPipelineMemberInvalid
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
		logs.Error("validatePipelineMembers query error: admin_id=%d, err=%s", adminId, err.Error())
		return nil, err
	}
	if len(rows) != len(normalized) {
		logs.Error("validatePipelineMembers reject foreign members: admin_id=%d, want=%d, got=%d",
			adminId, len(normalized), len(rows))
		return nil, ErrPipelineMemberInvalid
	}
	return normalized, nil
}

// validatePipelineScopeTargets validates the member and department targets of a scope:
// a non-empty scope must be one of all/members/departments; departments must all belong
// to the current tenant (same anti-ghost-member reasoning as validatePipelineMembers).
func validatePipelineScopeTargets(adminId int, scope string, memberIds, departmentIds []int) (string, []int, []int, error) {
	scope = normalizePipelineScope(scope)
	memberIds, err := validatePipelineMembers(adminId, memberIds)
	if err != nil {
		return ``, nil, nil, err
	}
	departmentIds, err = validatePipelineDepartments(adminId, departmentIds)
	if err != nil {
		return ``, nil, nil, err
	}
	return scope, memberIds, departmentIds, nil
}

// validatePipelineDepartments validates that department IDs are valid and belong to the
// current tenant, returning a normalized deduplicated list.
func validatePipelineDepartments(adminId int, departmentIds []int) ([]int, error) {
	normalized := make([]int, 0, len(departmentIds))
	seen := make(map[int]bool, len(departmentIds))
	for _, departmentId := range departmentIds {
		if departmentId <= 0 || seen[departmentId] {
			return nil, ErrPipelineMemberInvalid
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
		logs.Error("validatePipelineDepartments query error: admin_id=%d, err=%s", adminId, err.Error())
		return nil, err
	}
	if len(rows) != len(normalized) {
		logs.Error("validatePipelineDepartments reject foreign departments: admin_id=%d, want=%d, got=%d",
			adminId, len(normalized), len(rows))
		return nil, ErrPipelineMemberInvalid
	}
	return normalized, nil
}

// CreatePipeline creates a pipeline (including members and steps), performing quota checks in the same transaction.
func CreatePipeline(adminId, userId int, input define.CreatePipelineReq) (int, error) {
	// Check management permission
	if !CanManagePipeline(adminId, userId) {
		return 0, ErrPipelineManageForbidden
	}
	// Validate steps
	if len(input.Steps) == 0 {
		return 0, ErrPipelineWorkflowInvalid
	}
	if err := validatePipelineStepPrompts(input.Steps); err != nil {
		return 0, err
	}
	// Normalize and validate the scope targets; ensure all members and departments belong
	// to this tenant, preventing cross-tenant ghost authorizations
	scope, memberIds, departmentIds, err := validatePipelineScopeTargets(adminId, input.Scope, input.MemberIds, input.DepartmentIds)
	if err != nil {
		return 0, err
	}
	// 校验配额
	if err := CheckQuota(adminId, define.QuotaAgent); err != nil {
		return 0, err
	}

	now := tool.Time2Int()
	if input.Color == `` {
		input.Color = `#3157E2`
	}
	if input.Status == 0 {
		input.Status = 1
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := msql.RawValues(define.Postgres, `INSERT INTO pipelines(
		admin_id,name,description,color,icon,cli_type,workflow_version,status,scope,
		created_by,updated_by,created_at,updated_at,deleted_at)
		VALUES($1,$2,$3,$4,$5,$6,1,$7,$8,$9,$9,$10,$10,0) RETURNING id`, tx,
		adminId, input.Name, input.Description, input.Color, input.Icon, input.CliType,
		input.Status, scope, userId, now)
	if err != nil {
		return 0, err
	}
	pipelineId := cast.ToInt(rows[0][`id`])

	if err = savePipelineScope(tx, adminId, pipelineId, userId, scope, memberIds, departmentIds, now); err != nil {
		return 0, err
	}
	for _, step := range input.Steps {
		stepKey, keyErr := generateStepKey()
		if keyErr != nil {
			return 0, keyErr
		}
		if _, err = msql.RawExec(define.Postgres, `INSERT INTO pipeline_steps(
			admin_id,pipeline_id,step_key,name,avatar,prompt_content,remark,sort_order,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)`, tx, adminId, pipelineId, stepKey,
			step.Name, step.Avatar, step.PromptContent, step.Remark, step.SortOrder, now); err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return pipelineId, nil
}

// UpdatePipeline updates pipeline basic info and members (not the CLI type or workflow).
func UpdatePipeline(adminId, userId, pipelineId int, input define.UpdatePipelineReq) error {
	// Check management permission
	if !CanManagePipeline(adminId, userId) {
		return ErrPipelineManageForbidden
	}
	// Ensure the pipeline exists
	if err := checkPipelineExists(adminId, pipelineId); err != nil {
		return err
	}
	// Normalize and validate the scope targets; ensure all members and departments belong
	// to this tenant, preventing cross-tenant ghost authorizations
	scope, memberIds, departmentIds, err := validatePipelineScopeTargets(adminId, input.Scope, input.MemberIds, input.DepartmentIds)
	if err != nil {
		return err
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := tool.Time2Int()
	if _, err = msql.RawExec(define.Postgres, `UPDATE pipelines SET
		name=$1,description=$2,color=$3,icon=$4,status=$5,scope=$6,updated_by=$7,updated_at=$8
		WHERE admin_id=$9 AND id=$10 AND deleted_at=0`, tx, input.Name, input.Description,
		input.Color, input.Icon, input.Status, scope, userId, now, adminId, pipelineId); err != nil {
		return err
	}
	if err = savePipelineScope(tx, adminId, pipelineId, userId, scope, memberIds, departmentIds, now); err != nil {
		return err
	}
	return tx.Commit()
}

// savePipelineScope replaces the scope authorization rows (members and/or departments)
// of one pipeline inside the caller's transaction. Rows of the non-active kind are
// removed so switching scope never leaves stale grants behind.
func savePipelineScope(tx *sql.Tx, adminId, pipelineId, userId int, scope string, memberIds, departmentIds []int, now int) error {
	if _, err := msql.RawExec(define.Postgres,
		`DELETE FROM pipeline_members WHERE admin_id=$1 AND pipeline_id=$2`,
		tx, adminId, pipelineId); err != nil {
		return err
	}
	if _, err := msql.RawExec(define.Postgres,
		`DELETE FROM pipeline_departments WHERE admin_id=$1 AND pipeline_id=$2`,
		tx, adminId, pipelineId); err != nil {
		return err
	}
	if scope == define.PipelineScopeMembers {
		for _, memberId := range memberIds {
			if _, err := msql.RawExec(define.Postgres, `INSERT INTO pipeline_members(
				admin_id,pipeline_id,user_id,created_by,created_at)
				VALUES($1,$2,$3,$4,$5)`, tx, adminId, pipelineId, memberId, userId, now); err != nil {
				return err
			}
		}
	}
	if scope == define.PipelineScopeDepartments {
		for _, departmentId := range departmentIds {
			if _, err := msql.RawExec(define.Postgres, `INSERT INTO pipeline_departments(
				admin_id,pipeline_id,department_id,created_by,created_at)
				VALUES($1,$2,$3,$4,$5)`, tx, adminId, pipelineId, departmentId, userId, now); err != nil {
				return err
			}
		}
	}
	return nil
}

// UpdatePipelineWorkflow updates the pipeline workflow (with optimistic locking).
func UpdatePipelineWorkflow(adminId, userId, pipelineId int, input define.UpdatePipelineWorkflowReq) error {
	// Check management permission
	if !CanManagePipeline(adminId, userId) {
		return ErrPipelineManageForbidden
	}
	// Validate steps
	if len(input.Steps) == 0 {
		return ErrPipelineWorkflowInvalid
	}
	if err := validatePipelineStepPrompts(input.Steps); err != nil {
		return err
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := tool.Time2Int()
	updated, err := msql.RawValues(define.Postgres, `UPDATE pipelines
		SET cli_type=$1,workflow_version=workflow_version+1,updated_by=$2,updated_at=$3
		WHERE admin_id=$4 AND id=$5 AND deleted_at=0 AND workflow_version=$6
		RETURNING workflow_version`, tx, input.CliType, userId, now, adminId, pipelineId,
		input.ExpectedWorkflowVersion)
	if err != nil {
		return err
	}
	if len(updated) == 0 {
		exists, queryErr := msql.RawValues(define.Postgres,
			`SELECT id FROM pipelines WHERE admin_id=$1 AND id=$2 AND deleted_at=0`,
			tx, adminId, pipelineId)
		if queryErr != nil {
			return queryErr
		}
		if len(exists) == 0 {
			return ErrPipelineNotFound
		}
		return ErrPipelineVersionConflict
	}

	existingSteps, err := msql.RawValues(define.Postgres,
		`SELECT step_key FROM pipeline_steps WHERE admin_id=$1 AND pipeline_id=$2`,
		tx, adminId, pipelineId)
	if err != nil {
		return err
	}
	existingStepKeys := make(map[string]struct{}, len(existingSteps))
	for _, row := range existingSteps {
		existingStepKeys[row[`step_key`]] = struct{}{}
	}
	stepKeys := make(map[string]struct{}, len(input.Steps))
	for index := range input.Steps {
		stepKey := strings.TrimSpace(input.Steps[index].StepKey)
		if stepKey == `` {
			if stepKey, err = generateStepKey(); err != nil {
				return err
			}
			input.Steps[index].StepKey = stepKey
		} else if _, ok := existingStepKeys[stepKey]; !ok {
			return ErrPipelineWorkflowInvalid
		}
		if _, duplicated := stepKeys[stepKey]; duplicated {
			return ErrPipelineWorkflowInvalid
		}
		stepKeys[stepKey] = struct{}{}
	}

	if _, err = msql.RawExec(define.Postgres,
		`DELETE FROM pipeline_steps WHERE admin_id=$1 AND pipeline_id=$2`,
		tx, adminId, pipelineId); err != nil {
		return err
	}
	for _, step := range input.Steps {
		if _, err = msql.RawExec(define.Postgres, `INSERT INTO pipeline_steps(
			admin_id,pipeline_id,step_key,name,avatar,prompt_content,remark,sort_order,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)`, tx, adminId, pipelineId, step.StepKey,
			step.Name, step.Avatar, step.PromptContent, step.Remark, step.SortOrder, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeletePipeline soft-deletes a pipeline.
func DeletePipeline(adminId, userId, pipelineId int) error {
	// Check management permission
	if !CanManagePipeline(adminId, userId) {
		return ErrPipelineManageForbidden
	}

	now := tool.Time2Int()
	m := msql.Model(define.TablePipeline, define.Postgres)
	_, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(pipelineId)},
		{`deleted_at`, `0`},
	}).Update(msql.Datas{
		`deleted_at`: now,
		`status`:     0,
		`updated_by`: userId,
		`updated_at`: now,
	})
	return err
}

// SavePipelineIcon validates and saves the pipeline icon, returning the accessible path generated by the storage system.
func SavePipelineIcon(adminId, userId, pipelineId int, file *multipart.FileHeader) (string, error) {
	if !CanManagePipeline(adminId, userId) {
		return ``, ErrPipelineManageForbidden
	}
	if file == nil || file.Size <= 0 {
		return ``, ErrPipelineIconInvalid
	}
	if file.Size > maxPipelineIconSize {
		return ``, ErrPipelineIconTooLarge
	}
	if err := checkPipelineExists(adminId, pipelineId); err != nil {
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
		return ``, ErrPipelineIconInvalid
	}
	if _, err = opened.Seek(0, io.SeekStart); err != nil {
		return ``, err
	}

	m := msql.Model(define.TablePipeline, define.Postgres)
	oldIcon, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(pipelineId)},
		{`deleted_at`, `0`},
	}).Value(`icon`)
	if err != nil {
		return ``, err
	}
	storedPath, err := lib_storage.M.Upload(adminId, lib_storage.BusinessPipeline, ext, opened)
	if err != nil {
		return ``, err
	}
	_, err = m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(pipelineId)},
		{`deleted_at`, `0`},
	}).Update(msql.Datas{`icon`: storedPath, `updated_by`: userId, `updated_at`: tool.Time2Int()})
	if err != nil {
		_ = lib_storage.M.Delete(storedPath)
		return ``, err
	}
	if oldIcon != `` && oldIcon != storedPath && lib_storage.M.IsOwned(adminId, lib_storage.BusinessPipeline, oldIcon) {
		if err := lib_storage.M.Delete(oldIcon); err != nil {
			logs.Error("remove pipeline icon failed, err:%s", err.Error())
		}
	}
	return storedPath, nil
}

// ======================== Client-only API ========================

// GetClientPipelines returns the pipeline list available to the current user under
// the scope rules: all = every member; members = explicit pipeline_members rows;
// departments = members of the bound departments.
func GetClientPipelines(adminId, userId int) ([]map[string]interface{}, error) {
	rows, err := msql.RawValues(define.Postgres, `
		SELECT p.id, p.name, p.description, p.color, p.icon, p.cli_type, p.workflow_version
		FROM pipelines p
		WHERE p.admin_id=$1 AND p.deleted_at=0 AND p.status=1 AND (
			p.scope='all'
			OR (p.scope='members' AND EXISTS(
				SELECT 1 FROM pipeline_members m WHERE m.admin_id=p.admin_id AND m.pipeline_id=p.id AND m.user_id=$2))
			OR (p.scope='departments' AND EXISTS(
				SELECT 1 FROM pipeline_departments d
				JOIN user_departments ud ON ud.department_id=d.department_id AND ud.admin_id=d.admin_id
				WHERE d.admin_id=p.admin_id AND d.pipeline_id=p.id AND ud.user_id=$2))
		)
		ORDER BY p.id DESC`, nil, adminId, userId)
	if err != nil {
		return nil, err
	}
	list := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		list = append(list, map[string]interface{}{
			`id`:               cast.ToInt(row[`id`]),
			`name`:             row[`name`],
			`description`:      row[`description`],
			`color`:            row[`color`],
			`icon`:             row[`icon`],
			`cli_type`:         row[`cli_type`],
			`workflow_version`: cast.ToInt(row[`workflow_version`]),
		})
	}
	return list, nil
}

// GetPipelineSnapshot returns the immutable pipeline creation snapshot.
func GetPipelineSnapshot(adminId, pipelineId, userId int) (map[string]interface{}, error) {
	// Ensure the pipeline exists and is enabled
	m := msql.Model(define.TablePipeline, define.Postgres)
	row, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(pipelineId)},
		{`deleted_at`, `0`},
		{`status`, `1`},
	}).Find()
	if err != nil {
		return nil, err
	}
	if len(row) == 0 {
		return nil, ErrPipelineNotFound
	}

	// Ensure the user is within the allowed scope (all / members / departments)
	if !CanManagePipeline(adminId, userId) && !isPipelineUsable(adminId, pipelineId, userId, cast.ToString(row[`scope`])) {
		return nil, ErrPipelineUseForbidden
	}

	pipeline := map[string]interface{}{
		`id`:               cast.ToInt(row[`id`]),
		`name`:             row[`name`],
		`description`:      row[`description`],
		`color`:            row[`color`],
		`icon`:             row[`icon`],
		`cli_type`:         row[`cli_type`],
		`workflow_version`: cast.ToInt(row[`workflow_version`]),
	}

	// Query steps
	steps, err := getPipelineSteps(adminId, pipelineId)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		`pipeline`: pipeline,
		`steps`:    steps,
	}, nil
}

// CheckExecutionAuthorization confirms permission before execution.
func CheckExecutionAuthorization(adminId, pipelineId, userId int) (map[string]interface{}, error) {
	m := msql.Model(define.TablePipeline, define.Postgres)
	row, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(pipelineId)},
		{`deleted_at`, `0`},
	}).Find()
	if err != nil {
		return nil, err
	}
	if len(row) == 0 {
		return map[string]interface{}{
			`allowed`:         false,
			`pipeline_status`: `deleted`,
			`member_active`:   false,
		}, nil
	}

	status := cast.ToInt(row[`status`])
	pipelineStatus := `enabled`
	if status == 0 {
		pipelineStatus = `disabled`
	}

	memberActive := CanManagePipeline(adminId, userId) || isPipelineUsable(adminId, pipelineId, userId, cast.ToString(row[`scope`]))
	allowed := status == 1 && memberActive

	return map[string]interface{}{
		`allowed`:         allowed,
		`pipeline_status`: pipelineStatus,
		`member_active`:   memberActive,
	}, nil
}

// ======================== Device registration ========================

// RegisterDevice registers the client device and returns the device credential (returned only once).
// Shared by all clients; the bootstrap key check is no longer required.
func RegisterDevice(adminId, userId int, input define.RegisterDeviceReq) (define.RegisterDeviceResp, error) {
	// Generate the device credential; reuse the client auth middleware's fingerprint algorithm
	credential, err := generateDeviceCredential()
	if err != nil {
		return define.RegisterDeviceResp{}, err
	}
	credentialFingerprint := common.ComputeDeviceFingerprint(common.DeviceSecret(), credential)

	now := tool.Time2Int()
	// Idempotent: if device_uuid already exists, update the credential
	m := msql.Model(define.TableClientDevice, define.Postgres)
	existId, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`device_uuid`, input.DeviceUUID},
	}).Value(`id`)
	if err != nil {
		return define.RegisterDeviceResp{}, err
	}

	if existId != `` {
		// Update existing device
		upM := msql.Model(define.TableClientDevice, define.Postgres)
		_, err = upM.Where2([][]string{
			{`admin_id`, cast.ToString(adminId)},
			{`device_uuid`, input.DeviceUUID},
		}).Update(msql.Datas{
			`device_name`:            input.DeviceName,
			`os`:                     input.OS,
			`arch`:                   input.Arch,
			`client_version`:         input.ClientVersion,
			`protocol_version`:       input.ProtocolVersion,
			`credential_version`:     1,
			`credential_fingerprint`: credentialFingerprint,
			`credential_status`:      1,
			`registered_by_user_id`:  userId,
			`last_user_id`:           userId,
			`revoked_at`:             0,
			`updated_at`:             now,
		})
	} else {
		// Create new device
		insertM := msql.Model(define.TableClientDevice, define.Postgres)
		_, err = insertM.Insert(msql.Datas{
			`admin_id`:               adminId,
			`device_uuid`:            input.DeviceUUID,
			`device_name`:            input.DeviceName,
			`os`:                     input.OS,
			`arch`:                   input.Arch,
			`client_version`:         input.ClientVersion,
			`protocol_version`:       input.ProtocolVersion,
			`local_schema_version`:   1,
			`credential_version`:     1,
			`credential_fingerprint`: credentialFingerprint,
			`credential_status`:      1,
			`registered_by_user_id`:  userId,
			`last_user_id`:           userId,
			`created_at`:             now,
			`updated_at`:             now,
		})
	}
	if err != nil {
		return define.RegisterDeviceResp{}, err
	}

	return define.RegisterDeviceResp{
		DeviceUUID:        input.DeviceUUID,
		Credential:        credential,
		CredentialVersion: 1,
		ProtocolVersion:   input.ProtocolVersion,
	}, nil
}

// ======================== Client compatibility ========================

// GetClientCompatibility returns client compatibility information.
func GetClientCompatibility(adminId int) (map[string]interface{}, error) {
	m := msql.Model(define.TableUsers, define.Postgres)
	row, err := m.Where(`id`, cast.ToString(adminId)).Find()
	if err != nil {
		return nil, err
	}
	tenantName := ``
	if len(row) > 0 {
		tenantName = row[`name`]
	}
	return map[string]interface{}{
		`tenant_id`:             adminId,
		`tenant_name`:           tenantName,
		`cloud_version`:         `1.0.0`,
		`min_client_version`:    `1.0.0`,
		`protocol_version`:      define.PipelineProtocolVersion,
		`latest_client_version`: `1.0.0`,
	}, nil
}

// GetClientMe returns the current client user information.
func GetClientMe(adminId, userId int) (map[string]interface{}, error) {
	m := msql.Model(define.TableUsers, define.Postgres)
	row, err := m.Where2([][]string{
		{`id`, cast.ToString(userId)},
		{`admin_id`, cast.ToString(adminId)},
	}).Find()
	if err != nil {
		return nil, err
	}
	if len(row) == 0 {
		return nil, errors.New(`user not found`)
	}
	return map[string]interface{}{
		`user_id`:  userId,
		`admin_id`: adminId,
		`username`: row[`username`],
		`name`:     row[`name`],
		`avatar`:   row[`avatar`],
		`role_id`:  cast.ToInt(row[`role_id`]),
	}, nil
}

// ======================== Tenant work items (client use) ========================

// GetClientWorkItems returns the work item list the current user can import.
// assignee_ids is a multi-user array; use @> ARRAY to check whether it contains the current user.
func GetClientWorkItems(adminId, userId int, keyword string, page, size int) ([]map[string]interface{}, int, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 200 {
		size = 100
	}
	offset := (page - 1) * size

	// Filter: only items assigned to the current user (assignee_ids supports multiple users)
	where := []string{`r.admin_id = $1`, `r.assignee_ids @> $2`}
	args := []any{adminId, pq.Array([]int{userId})}
	keyword = strings.TrimSpace(keyword)
	if keyword != `` {
		args = append(args, `%`+keyword+`%`)
		where = append(where, fmt.Sprintf(`r.title ILIKE $%d`, len(args)))
	}
	whereSQL := strings.Join(where, ` AND `)

	countQuery := fmt.Sprintf(`SELECT COUNT(*) AS total FROM requirements r
		JOIN workspaces w ON w.id = r.workspace_id AND w.admin_id = r.admin_id
		WHERE %s`, whereSQL)
	countRows, err := msql.RawValues(define.Postgres, countQuery, nil, args...)
	if err != nil {
		logs.Error("GetClientWorkItems count failed, err:%s", err.Error())
		return nil, 0, err
	}
	total := cast.ToInt(countRows[0][`total`])
	if total == 0 {
		return []map[string]interface{}{}, 0, nil
	}

	listArgs := append([]any{}, args...)
	listArgs = append(listArgs, size, offset)
	listQuery := fmt.Sprintf(`SELECT r.id, r.workspace_id, w.name AS workspace_name, r.title,
			r.description, r.priority, r.status_id,
			r.planned_start_date, r.planned_end_date, r.created_at, r.updated_at,
			to_json(r.assignee_ids)::text AS assignee_ids
		FROM requirements r
		JOIN workspaces w ON w.id = r.workspace_id AND w.admin_id = r.admin_id
		WHERE %s
		ORDER BY r.id DESC
		LIMIT $%d OFFSET $%d`, whereSQL, len(listArgs)-1, len(listArgs))
	rows, err := msql.RawValues(define.Postgres, listQuery, nil, listArgs...)
	if err != nil {
		logs.Error("GetClientWorkItems list failed, err:%s", err.Error())
		return nil, 0, err
	}

	assigneeIDsByItem := make(map[int][]int, len(rows))
	uniqueAssigneeIDs := make(map[int]struct{})
	for _, row := range rows {
		ids := make([]int, 0)
		if err := json.Unmarshal([]byte(cast.ToString(row[`assignee_ids`])), &ids); err != nil {
			return nil, 0, err
		}
		assigneeIDsByItem[cast.ToInt(row[`id`])] = ids
		for _, id := range ids {
			uniqueAssigneeIDs[id] = struct{}{}
		}
	}
	assigneeIDs := make([]int, 0, len(uniqueAssigneeIDs))
	for id := range uniqueAssigneeIDs {
		assigneeIDs = append(assigneeIDs, id)
	}
	usersByID := make(map[int]map[string]string, len(assigneeIDs))
	if len(assigneeIDs) > 0 {
		userRows, queryErr := msql.RawValues(define.Postgres, `SELECT id, name, avatar, username
			FROM users WHERE admin_id=$1 AND id=ANY($2)`, nil, adminId, pq.Array(assigneeIDs))
		if queryErr != nil {
			return nil, 0, queryErr
		}
		for _, user := range userRows {
			usersByID[cast.ToInt(user[`id`])] = map[string]string{
				`name`:     user[`name`],
				`avatar`:   user[`avatar`],
				`username`: user[`username`],
			}
		}
	}

	list := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		itemID := cast.ToInt(row[`id`])
		ids := assigneeIDsByItem[itemID]
		names := make([]string, 0, len(ids))
		avatars := make([]string, 0, len(ids))
		usernames := make([]string, 0, len(ids))
		for _, id := range ids {
			user := usersByID[id]
			names = append(names, user[`name`])
			avatars = append(avatars, user[`avatar`])
			usernames = append(usernames, user[`username`])
		}
		list = append(list, map[string]interface{}{
			`id`:                 itemID,
			`workspace_id`:       cast.ToInt(row[`workspace_id`]),
			`workspace_name`:     row[`workspace_name`],
			`type`:               `requirement`,
			`title`:              row[`title`],
			`description`:        row[`description`],
			`priority`:           row[`priority`],
			`status_id`:          cast.ToInt(row[`status_id`]),
			`assignee_ids`:       ids,
			`assignee_names`:     names,
			`assignee_avatars`:   avatars,
			`assignee_usernames`: usernames,
			`planned_start_date`: row[`planned_start_date`],
			`planned_end_date`:   row[`planned_end_date`],
			`created_at`:         cast.ToInt(row[`created_at`]),
			`updated_at`:         cast.ToInt(row[`updated_at`]),
		})
	}
	return list, total, nil
}

// ======================== Internal helpers ========================

func checkPipelineExists(adminId, pipelineId int) error {
	m := msql.Model(define.TablePipeline, define.Postgres)
	exist, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(pipelineId)},
		{`deleted_at`, `0`},
	}).Value(`id`)
	if err != nil {
		return err
	}
	if exist == `` {
		return ErrPipelineNotFound
	}
	return nil
}

func getPipelineMembers(adminId, pipelineId int) ([]define.PipelineMemberInfo, error) {
	m := msql.Model(define.TablePipelineMember, define.Postgres)
	rows, err := m.Where2([][]string{
		{`pipeline_members.admin_id`, cast.ToString(adminId)},
		{`pipeline_members.pipeline_id`, cast.ToString(pipelineId)},
		// The JOIN carries a users.admin_id condition: even if historical dirty data (user_id pointing to another tenant) exists,
		// only an empty user name is returned and no foreign-tenant member info leaks
	}).Join(define.TableUsers, `pipeline_members.user_id=users.id AND users.admin_id=pipeline_members.admin_id`, `left`).
		Field(`pipeline_members.user_id, users.name as user_name`).
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

func getPipelineSteps(adminId, pipelineId int) ([]define.PipelineWorkflowStep, error) {
	m := msql.Model(define.TablePipelineStep, define.Postgres)
	rows, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`pipeline_id`, cast.ToString(pipelineId)},
	}).Order(`sort_order asc, id asc`).Select()
	if err != nil {
		return nil, err
	}
	list := make([]define.PipelineWorkflowStep, 0, len(rows))
	for _, row := range rows {
		list = append(list, define.PipelineWorkflowStep{
			StepKey:       row[`step_key`],
			Name:          row[`name`],
			Avatar:        row[`avatar`],
			PromptContent: row[`prompt_content`],
			Remark:        row[`remark`],
			SortOrder:     cast.ToInt(row[`sort_order`]),
		})
	}
	return list, nil
}

func countPipelineMembers(adminId, pipelineId int) (int, error) {
	m := msql.Model(define.TablePipelineMember, define.Postgres)
	return m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`pipeline_id`, cast.ToString(pipelineId)},
	}).Count()
}

func countPipelineSteps(adminId, pipelineId int) (int, error) {
	m := msql.Model(define.TablePipelineStep, define.Postgres)
	return m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`pipeline_id`, cast.ToString(pipelineId)},
	}).Count()
}

func isPipelineMember(adminId, pipelineId, userId int) bool {
	m := msql.Model(define.TablePipelineMember, define.Postgres)
	count, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`pipeline_id`, cast.ToString(pipelineId)},
		{`user_id`, cast.ToString(userId)},
	}).Count()
	if err != nil {
		return false
	}
	return count > 0
}

// isPipelineUsable checks the user's access under the scope rules; an unknown scope
// falls back to the members check, matching the pre-scope behavior for legacy rows.
func isPipelineUsable(adminId, pipelineId, userId int, scope string) bool {
	switch normalizePipelineScope(scope) {
	case define.PipelineScopeAll:
		return true
	case define.PipelineScopeDepartments:
		rows, err := msql.RawValues(define.Postgres, `
			SELECT 1 FROM pipeline_departments d
			JOIN user_departments ud ON ud.department_id=d.department_id AND ud.admin_id=d.admin_id
			WHERE d.admin_id=$1 AND d.pipeline_id=$2 AND ud.user_id=$3 LIMIT 1`, nil,
			adminId, pipelineId, userId)
		if err != nil {
			logs.Error("isPipelineUsable departments error: admin_id=%d, pipeline_id=%d, err=%s", adminId, pipelineId, err.Error())
			return false
		}
		return len(rows) > 0
	default:
		return isPipelineMember(adminId, pipelineId, userId)
	}
}

// getPipelineDepartments returns the departments bound to a pipeline (scope=departments).
func getPipelineDepartments(adminId, pipelineId int) ([]define.PipelineDepartmentInfo, error) {
	rows, err := msql.RawValues(define.Postgres, `
		SELECT d.id AS department_id, d.name AS department_name
		FROM pipeline_departments pd
		JOIN departments d ON d.id=pd.department_id AND d.admin_id=pd.admin_id
		WHERE pd.admin_id=$1 AND pd.pipeline_id=$2
		ORDER BY d.id`, nil, adminId, pipelineId)
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

func countPipelineDepartments(adminId, pipelineId int) (int, error) {
	m := msql.Model(define.TablePipelineDepartment, define.Postgres)
	return m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`pipeline_id`, cast.ToString(pipelineId)},
	}).Count()
}

// pipelineScopeUserPreview returns the first scope member avatars (for the card avatar
// stack) and the total headcount covered by the scope. The window function keeps it to
// one roundtrip; DISTINCT must run before COUNT(*) OVER() so multi-department users are
// not double counted.
func pipelineScopeUserPreview(adminId, pipelineId int, scope string) ([]define.PipelineScopeMember, int, error) {
	var query string
	if scope == define.PipelineScopeDepartments {
		query = `
			SELECT t.id, t.name, t.avatar, COUNT(*) OVER() AS total FROM (
				SELECT DISTINCT u.id, u.name, u.avatar
				FROM pipeline_departments pd
				JOIN user_departments ud ON ud.department_id=pd.department_id AND ud.admin_id=pd.admin_id
				JOIN users u ON u.id=ud.user_id AND u.admin_id=ud.admin_id
				WHERE pd.admin_id=$1 AND pd.pipeline_id=$2
			) t
			ORDER BY t.id
			LIMIT 6`
	} else {
		query = `
			SELECT u.id, u.name, u.avatar, COUNT(*) OVER() AS total
			FROM pipeline_members pm
			JOIN users u ON u.id=pm.user_id AND u.admin_id=pm.admin_id
			WHERE pm.admin_id=$1 AND pm.pipeline_id=$2
			ORDER BY u.id
			LIMIT 6`
	}
	rows, err := msql.RawValues(define.Postgres, query, nil, adminId, pipelineId)
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

// randomBytes reads cryptographically secure random bytes of the given length.
// The entropy source must return an error on failure: silent degradation yields all-zero values, making step_key fixed and device credentials predictable.
func randomBytes(size int) ([]byte, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		logs.Error("randomBytes error: size=%d, err=%s", size, err.Error())
		return nil, err
	}
	return buf, nil
}

// generateStepKey generates a unique step identifier.
func generateStepKey() (string, error) {
	buf, err := randomBytes(4)
	if err != nil {
		return ``, err
	}
	return `step_` + hex.EncodeToString(buf), nil
}

// generateDeviceCredential generates a random 256-bit device credential.
func generateDeviceCredential() (string, error) {
	buf, err := randomBytes(32)
	if err != nil {
		return ``, err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

// ======================== 客户端发布版本（通用，不绑定租户） ========================

// clientReleaseTargets 客户端支持的平台/架构矩阵及其对应的配置文件下载地址键名。
// 文件名规则参考 goteams_client dist 产物：client-{platform}-{arch}（windows 平台带 .exe 后缀）。
// 各平台下载地址在配置文件 [client] 段以独立配置项给出（完整 URL，互不相同）。
var clientReleaseTargets = []struct {
	Platform  string
	Arch      string
	ConfigKey string // [client] 段中该平台下载地址对应的键名
}{
	{`darwin`, `amd64`, `darwin_amd64_url`},
	{`darwin`, `arm64`, `darwin_arm64_url`},
	{`windows`, `amd64`, `windows_amd64_url`},
	{`windows`, `arm64`, `windows_arm64_url`},
}

// clientReleaseFileName 根据平台/架构生成客户端文件名。
func clientReleaseFileName(platform, arch string) string {
	name := fmt.Sprintf(`client-%s-%s`, platform, arch)
	if platform == `windows` {
		name += `.exe`
	}
	return name
}

// ListClientReleases 返回可下载的客户端版本列表。
// 下载地址取自配置文件 [client] 段各平台独立配置项（darwin_amd64_url 等），不再依赖数据库。
func ListClientReleases(platform, arch string) ([]map[string]interface{}, error) {
	list := make([]map[string]interface{}, 0, len(clientReleaseTargets))
	for _, t := range clientReleaseTargets {
		if platform != `` && t.Platform != platform {
			continue
		}
		if arch != `` && t.Arch != arch {
			continue
		}
		// 各平台下载地址为独立完整 URL；配置缺失则视为该平台未发布，跳过。
		downloadURL := strings.TrimSpace(define.Config.Client[t.ConfigKey])
		if downloadURL == `` {
			continue
		}
		list = append(list, map[string]interface{}{
			`id`:           t.Platform + `-` + t.Arch,
			`platform`:     t.Platform,
			`arch`:         t.Arch,
			`file_name`:    clientReleaseFileName(t.Platform, t.Arch),
			`download_url`: downloadURL,
		})
	}
	return list, nil
}

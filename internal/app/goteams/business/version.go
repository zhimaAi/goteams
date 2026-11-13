// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"errors"
	"sort"
	"strings"

	"goteams/internal/app/goteams/define"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

const (
	// IterationTypeSingle identifies an iteration associated with one workspace.
	IterationTypeSingle = `single`
	// IterationTypeCross identifies an iteration shared by multiple workspaces.
	IterationTypeCross = `cross`
)

var (
	// ErrVersionNotFound indicates the iteration is unavailable in the current workspace.
	ErrVersionNotFound = errors.New(`iteration not found`)
	// ErrVersionNameExists indicates a duplicate iteration name in its origin workspace.
	ErrVersionNameExists = errors.New(`iteration name already exists`)
	// ErrVersionWorkspaceInvalid indicates invalid or unauthorized associated workspaces.
	ErrVersionWorkspaceInvalid = errors.New(`iteration workspace selection is invalid`)
	// ErrVersionTypeImmutable indicates an existing iteration type cannot be changed.
	ErrVersionTypeImmutable = errors.New(`iteration type is immutable`)
	// ErrVersionEnded indicates an ended iteration cannot accept new work items.
	ErrVersionEnded = errors.New(`iteration is ended`)
)

// VersionWorkspaceModel describes one workspace associated with an iteration.
type VersionWorkspaceModel struct {
	Id       int
	Name     string
	IsOrigin bool
}

// VersionModel describes a workspace iteration.
type VersionModel struct {
	Id               int
	AdminId          int
	WorkspaceId      int
	Name             string
	StartDate        int64
	EndDate          int64
	EndedAt          int64
	RequirementCount int
	CompletedCount   int
	CreatedAt        int64
	UpdatedAt        int64
	Workspaces       []VersionWorkspaceModel
}

// IterationType returns the immutable single/cross-workspace type.
func (model VersionModel) IterationType() string {
	if len(model.Workspaces) > 1 {
		return IterationTypeCross
	}
	return IterationTypeSingle
}

// VersionWorkspaceOptionModel is a workspace the user may associate with a new iteration.
type VersionWorkspaceOptionModel struct {
	Id   int
	Name string
}

// CreateVersionModel describes the create-iteration use case.
type CreateVersionModel struct {
	AdminId       int
	UserId        int
	WorkspaceId   int
	Name          string
	StartDate     int64
	EndDate       int64
	IterationType string
	WorkspaceIds  []int
	Id            int
}

// UpdateVersionModel describes the update-iteration use case.
type UpdateVersionModel struct {
	AdminId       int
	UserId        int
	WorkspaceId   int
	VersionId     int
	Name          string
	StartDate     int64
	EndDate       int64
	IterationType string
	WorkspaceIds  []int
}

// VersionActionModel identifies an iteration through the current associated workspace.
type VersionActionModel struct {
	AdminId     int
	WorkspaceId int
	VersionId   int
}

// toVersionModel converts a version row into the business model.
func toVersionModel(row msql.Params) (VersionModel, error) {
	fields := []string{`id`, `admin_id`, `workspace_id`, `start_date`, `end_date`, `ended_at`, `requirement_count`, `completed_count`, `created_at`, `updated_at`}
	values := make(map[string]int64, len(fields))
	for _, field := range fields {
		value, err := metadataRowInt64(row, field)
		if err != nil {
			return VersionModel{}, err
		}
		values[field] = value
	}
	return VersionModel{
		Id: int(values[`id`]), AdminId: int(values[`admin_id`]), WorkspaceId: int(values[`workspace_id`]),
		Name: row[`name`], StartDate: values[`start_date`], EndDate: values[`end_date`], EndedAt: values[`ended_at`],
		RequirementCount: int(values[`requirement_count`]), CompletedCount: int(values[`completed_count`]),
		CreatedAt: values[`created_at`], UpdatedAt: values[`updated_at`], Workspaces: []VersionWorkspaceModel{},
	}, nil
}

// ListVersionsByWorkspace returns every iteration associated with a workspace.
func ListVersionsByWorkspace(adminID, workspaceID int) ([]VersionModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT v.id, v.admin_id, v.workspace_id, v.name,
		v.start_date, v.end_date, v.ended_at, v.requirement_count, v.completed_count, v.created_at, v.updated_at
		FROM versions v
		JOIN version_workspaces current_vw ON current_vw.admin_id = v.admin_id AND current_vw.version_id = v.id
		WHERE v.admin_id = $1 AND current_vw.workspace_id = $2
		ORDER BY v.id DESC`, nil, adminID, workspaceID)
	if err != nil {
		logs.Error("list iterations failed, err:%s", err.Error())
		return []VersionModel{}, err
	}
	items := make([]VersionModel, 0, len(rows))
	versionIds := make([]int, 0, len(rows))
	for _, row := range rows {
		item, convErr := toVersionModel(row)
		if convErr != nil {
			logs.Error("convert iteration row failed, err:%s", convErr.Error())
			return []VersionModel{}, convErr
		}
		items = append(items, item)
		versionIds = append(versionIds, item.Id)
	}
	if err = fillVersionWorkspaces(adminID, versionIds, items); err != nil {
		return []VersionModel{}, err
	}
	return items, nil
}

func fillVersionWorkspaces(adminID int, versionIds []int, versions []VersionModel) error {
	if len(versionIds) == 0 {
		return nil
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT vw.version_id, w.id, w.name, v.workspace_id AS origin_workspace_id
		FROM version_workspaces vw
		JOIN versions v ON v.admin_id = vw.admin_id AND v.id = vw.version_id
		JOIN workspaces w ON w.admin_id = vw.admin_id AND w.id = vw.workspace_id
		WHERE vw.admin_id = $1 AND vw.version_id = ANY($2)
		ORDER BY vw.version_id, CASE WHEN w.id = v.workspace_id THEN 0 ELSE 1 END, w.id`, nil,
		adminID, pq.Array(versionIds))
	if err != nil {
		logs.Error("list iteration workspaces failed, err:%s", err.Error())
		return err
	}
	byVersion := make(map[int][]VersionWorkspaceModel, len(versionIds))
	for _, row := range rows {
		versionId := cast.ToInt(row[`version_id`])
		workspaceId := cast.ToInt(row[`id`])
		byVersion[versionId] = append(byVersion[versionId], VersionWorkspaceModel{
			Id: workspaceId, Name: row[`name`], IsOrigin: workspaceId == cast.ToInt(row[`origin_workspace_id`]),
		})
	}
	for index := range versions {
		versions[index].Workspaces = byVersion[versions[index].Id]
	}
	return nil
}

// GetVersionByWorkspace loads an iteration only when it is associated with the current workspace.
func GetVersionByWorkspace(adminID, workspaceID, versionID int) (VersionModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT v.id, v.admin_id, v.workspace_id, v.name,
		v.start_date, v.end_date, v.ended_at, v.requirement_count, v.completed_count, v.created_at, v.updated_at
		FROM versions v
		JOIN version_workspaces vw ON vw.admin_id = v.admin_id AND vw.version_id = v.id
		WHERE v.admin_id = $1 AND vw.workspace_id = $2 AND v.id = $3`, nil,
		adminID, workspaceID, versionID)
	if err != nil {
		logs.Error("get iteration failed, err:%s", err.Error())
		return VersionModel{}, err
	}
	if len(rows) == 0 {
		return VersionModel{}, ErrVersionNotFound
	}
	model, err := toVersionModel(rows[0])
	if err != nil {
		return VersionModel{}, err
	}
	items := []VersionModel{model}
	if err = fillVersionWorkspaces(adminID, []int{versionID}, items); err != nil {
		return VersionModel{}, err
	}
	return items[0], nil
}

func listVersionWorkspaceIDs(adminID, versionID int, tx *sql.Tx) ([]int, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT workspace_id FROM version_workspaces
		WHERE admin_id = $1 AND version_id = $2 ORDER BY workspace_id`, tx, adminID, versionID)
	if err != nil {
		logs.Error("list iteration workspace ids failed, err:%s", err.Error())
		return nil, err
	}
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, cast.ToInt(row[`workspace_id`]))
	}
	return ids, nil
}

// RecalcVersionCounts recounts work items across every associated workspace.
func RecalcVersionCounts(tx *sql.Tx, adminID, versionID int) error {
	if versionID == 0 {
		return nil
	}
	workspaceIds, err := listVersionWorkspaceIDs(adminID, versionID, tx)
	if err != nil {
		return err
	}
	if len(workspaceIds) == 0 {
		return ErrVersionNotFound
	}
	finalStatusIds := make([]int, 0)
	for _, workspaceId := range workspaceIds {
		settings, settingsErr := GetWorkspaceSettings(WorkspaceSettingsModel{AdminId: adminID, WsId: cast.ToString(workspaceId)})
		if settingsErr != nil {
			return settingsErr
		}
		finalStatusIds = append(finalStatusIds, settings.RequirementCompletedStatusIds...)
		finalStatusIds = append(finalStatusIds, settings.DefectCompletedStatusIds...)
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT COUNT(id) AS requirement_count,
		COUNT(id) FILTER (WHERE status_id = ANY($3)) AS completed_count
		FROM requirements WHERE admin_id = $1 AND version_id = $2`, tx,
		adminID, versionID, pq.Array(finalStatusIds))
	if err != nil {
		logs.Error("recalculate iteration counts failed, err:%s", err.Error())
		return err
	}
	var requirementCount, completedCount int
	if len(rows) > 0 {
		requirementCount = cast.ToInt(rows[0][`requirement_count`])
		completedCount = cast.ToInt(rows[0][`completed_count`])
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE versions SET requirement_count = $1,
		completed_count = $2, updated_at = $3 WHERE admin_id = $4 AND id = $5`, tx,
		requirementCount, completedCount, tool.Time2Int(), adminID, versionID); err != nil {
		logs.Error("update iteration counts failed, err:%s", err.Error())
		return err
	}
	return nil
}

// RecalcWorkspaceVersionCounts recalculates every iteration associated with a workspace.
func RecalcWorkspaceVersionCounts(adminID, workspaceID int) error {
	rows, err := msql.RawValues(define.Postgres, `SELECT version_id FROM version_workspaces
		WHERE admin_id = $1 AND workspace_id = $2`, nil, adminID, workspaceID)
	if err != nil {
		logs.Error("list workspace iterations for recount failed, err:%s", err.Error())
		return err
	}
	for _, row := range rows {
		if err = RecalcVersionCounts(nil, adminID, cast.ToInt(row[`version_id`])); err != nil {
			return err
		}
	}
	return nil
}

func normalizeVersionWorkspaceIDs(originWorkspaceID int, workspaceIds []int, iterationType string) ([]int, error) {
	seen := map[int]bool{}
	normalized := make([]int, 0, len(workspaceIds)+1)
	for _, workspaceId := range workspaceIds {
		if workspaceId <= 0 || seen[workspaceId] {
			continue
		}
		seen[workspaceId] = true
		normalized = append(normalized, workspaceId)
	}
	if !seen[originWorkspaceID] {
		normalized = append(normalized, originWorkspaceID)
	}
	sort.Ints(normalized)
	switch iterationType {
	case IterationTypeSingle:
		if len(normalized) != 1 {
			return nil, ErrVersionWorkspaceInvalid
		}
	case IterationTypeCross:
		if len(normalized) < 2 {
			return nil, ErrVersionWorkspaceInvalid
		}
	default:
		return nil, ErrVersionWorkspaceInvalid
	}
	return normalized, nil
}

func validateVersionWorkspaceTargets(adminID, userID int, workspaceIds []int) error {
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM workspaces
		WHERE admin_id = $1 AND ended_at = 0 AND id = ANY($2)`, nil, adminID, pq.Array(workspaceIds))
	if err != nil {
		logs.Error("validate iteration workspaces failed, err:%s", err.Error())
		return err
	}
	if len(rows) != len(workspaceIds) {
		return ErrVersionWorkspaceInvalid
	}
	for _, workspaceId := range workspaceIds {
		if !HasPermission(adminID, userID, cast.ToString(workspaceId), `version:create`) {
			return ErrVersionWorkspaceInvalid
		}
	}
	return nil
}

// CreateVersion creates an iteration and its workspace associations atomically.
func CreateVersion(model CreateVersionModel) (CreateVersionModel, error) {
	workspaceIds, err := normalizeVersionWorkspaceIDs(model.WorkspaceId, model.WorkspaceIds, model.IterationType)
	if err != nil {
		return CreateVersionModel{}, err
	}
	if err = validateVersionWorkspaceTargets(model.AdminId, model.UserId, workspaceIds); err != nil {
		return CreateVersionModel{}, err
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin create iteration transaction failed, err:%s", err.Error())
		return CreateVersionModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	now := tool.Time2Int()
	rows, err := msql.RawValues(define.Postgres, `INSERT INTO versions
		(admin_id, workspace_id, name, start_date, end_date, ended_at, requirement_count, completed_count, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,0,0,0,$6,$6) RETURNING id`, tx,
		model.AdminId, model.WorkspaceId, strings.TrimSpace(model.Name), model.StartDate, model.EndDate, now)
	if err != nil {
		logs.Error("create iteration failed, err:%s", err.Error())
		if isVersionNameConflict(err) {
			return CreateVersionModel{}, ErrVersionNameExists
		}
		return CreateVersionModel{}, err
	}
	model.Id = cast.ToInt(rows[0][`id`])
	for _, workspaceId := range workspaceIds {
		if _, err = msql.RawExec(define.Postgres, `INSERT INTO version_workspaces
			(admin_id, version_id, workspace_id, created_at) VALUES ($1,$2,$3,$4)`, tx,
			model.AdminId, model.Id, workspaceId, now); err != nil {
			logs.Error("create iteration workspace relation failed, err:%s", err.Error())
			return CreateVersionModel{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit create iteration failed, err:%s", err.Error())
		return CreateVersionModel{}, err
	}
	model.WorkspaceIds = workspaceIds
	return model, nil
}

func lockVersionForWorkspace(tx *sql.Tx, adminID, workspaceID, versionID int) (VersionModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT v.id, v.admin_id, v.workspace_id, v.name,
		v.start_date, v.end_date, v.ended_at, v.requirement_count, v.completed_count, v.created_at, v.updated_at
		FROM versions v JOIN version_workspaces vw ON vw.admin_id = v.admin_id AND vw.version_id = v.id
		WHERE v.admin_id = $1 AND vw.workspace_id = $2 AND v.id = $3 FOR UPDATE OF v`, tx,
		adminID, workspaceID, versionID)
	if err != nil {
		logs.Error("lock iteration failed, err:%s", err.Error())
		return VersionModel{}, err
	}
	if len(rows) == 0 {
		return VersionModel{}, ErrVersionNotFound
	}
	return toVersionModel(rows[0])
}

// UpdateVersion updates iteration details and workspace associations atomically.
func UpdateVersion(model UpdateVersionModel) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin update iteration transaction failed, err:%s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()
	current, err := lockVersionForWorkspace(tx, model.AdminId, model.WorkspaceId, model.VersionId)
	if err != nil {
		return err
	}
	existingIds, err := listVersionWorkspaceIDs(model.AdminId, model.VersionId, tx)
	if err != nil {
		return err
	}
	currentType := IterationTypeSingle
	if len(existingIds) > 1 {
		currentType = IterationTypeCross
	}
	if model.IterationType != currentType {
		return ErrVersionTypeImmutable
	}
	workspaceIds, err := normalizeVersionWorkspaceIDs(current.WorkspaceId, model.WorkspaceIds, currentType)
	if err != nil {
		return err
	}
	existingSet := make(map[int]bool, len(existingIds))
	for _, workspaceId := range existingIds {
		existingSet[workspaceId] = true
	}
	addedIds := make([]int, 0)
	for _, workspaceId := range workspaceIds {
		if !existingSet[workspaceId] {
			addedIds = append(addedIds, workspaceId)
		}
	}
	if len(addedIds) > 0 {
		if err = validateVersionWorkspaceTargets(model.AdminId, model.UserId, addedIds); err != nil {
			return err
		}
	}
	now := tool.Time2Int()
	if _, err = msql.RawExec(define.Postgres, `UPDATE versions SET name = $1, start_date = $2,
		end_date = $3, updated_at = $4 WHERE admin_id = $5 AND id = $6`, tx,
		strings.TrimSpace(model.Name), model.StartDate, model.EndDate, now, model.AdminId, model.VersionId); err != nil {
		logs.Error("update iteration failed, err:%s", err.Error())
		if isVersionNameConflict(err) {
			return ErrVersionNameExists
		}
		return err
	}
	targetSet := make(map[int]bool, len(workspaceIds))
	for _, workspaceId := range workspaceIds {
		targetSet[workspaceId] = true
		if existingSet[workspaceId] {
			continue
		}
		if _, err = msql.RawExec(define.Postgres, `INSERT INTO version_workspaces
			(admin_id, version_id, workspace_id, created_at) VALUES ($1,$2,$3,$4)`, tx,
			model.AdminId, model.VersionId, workspaceId, now); err != nil {
			logs.Error("add iteration workspace failed, err:%s", err.Error())
			return err
		}
	}
	removedIds := make([]int, 0)
	for _, workspaceId := range existingIds {
		if !targetSet[workspaceId] {
			removedIds = append(removedIds, workspaceId)
		}
	}
	if len(removedIds) > 0 {
		if _, err = msql.RawExec(define.Postgres, `UPDATE requirements SET version_id = NULL, updated_at = $1
			WHERE admin_id = $2 AND version_id = $3 AND workspace_id = ANY($4)`, tx,
			now, model.AdminId, model.VersionId, pq.Array(removedIds)); err != nil {
			logs.Error("detach iteration work items failed, err:%s", err.Error())
			return err
		}
		if _, err = msql.RawExec(define.Postgres, `UPDATE workspaces SET active_version_id = 0, updated_at = $1
			WHERE admin_id = $2 AND active_version_id = $3 AND id = ANY($4)`, tx,
			now, model.AdminId, model.VersionId, pq.Array(removedIds)); err != nil {
			logs.Error("clear detached active iteration failed, err:%s", err.Error())
			return err
		}
		if _, err = msql.RawExec(define.Postgres, `DELETE FROM version_workspaces
			WHERE admin_id = $1 AND version_id = $2 AND workspace_id = ANY($3)`, tx,
			model.AdminId, model.VersionId, pq.Array(removedIds)); err != nil {
			logs.Error("delete iteration workspace relation failed, err:%s", err.Error())
			return err
		}
	}
	if err = RecalcVersionCounts(tx, model.AdminId, model.VersionId); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit update iteration failed, err:%s", err.Error())
		return err
	}
	return nil
}

// DeleteVersion deletes an iteration and clears every work-item/workspace association.
func DeleteVersion(model VersionActionModel) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin delete iteration transaction failed, err:%s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = lockVersionForWorkspace(tx, model.AdminId, model.WorkspaceId, model.VersionId); err != nil {
		return err
	}
	now := tool.Time2Int()
	if _, err = msql.RawExec(define.Postgres, `UPDATE requirements SET version_id = NULL, updated_at = $1
		WHERE admin_id = $2 AND version_id = $3`, tx, now, model.AdminId, model.VersionId); err != nil {
		logs.Error("clear deleted iteration work items failed, err:%s", err.Error())
		return err
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE workspaces SET active_version_id = 0, updated_at = $1
		WHERE admin_id = $2 AND active_version_id = $3`, tx, now, model.AdminId, model.VersionId); err != nil {
		logs.Error("clear deleted active iteration failed, err:%s", err.Error())
		return err
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM version_workspaces
		WHERE admin_id = $1 AND version_id = $2`, tx, model.AdminId, model.VersionId); err != nil {
		logs.Error("delete iteration workspace relations failed, err:%s", err.Error())
		return err
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM versions
		WHERE admin_id = $1 AND id = $2`, tx, model.AdminId, model.VersionId); err != nil {
		logs.Error("delete iteration failed, err:%s", err.Error())
		return err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit delete iteration failed, err:%s", err.Error())
		return err
	}
	return nil
}

// UpdateVersionEnded ends or reopens an iteration.
func UpdateVersionEnded(model VersionActionModel, ended bool) error {
	endedAt := int64(0)
	if ended {
		endedAt = int64(tool.Time2Int())
	}
	result, err := msql.RawExec(define.Postgres, `UPDATE versions v SET ended_at = $1, updated_at = $2
		WHERE v.admin_id = $3 AND v.id = $4 AND EXISTS (
			SELECT 1 FROM version_workspaces vw WHERE vw.admin_id = v.admin_id
			AND vw.version_id = v.id AND vw.workspace_id = $5
		)`, nil, endedAt, tool.Time2Int(), model.AdminId, model.VersionId, model.WorkspaceId)
	if err != nil {
		logs.Error("update iteration ended state failed, err:%s", err.Error())
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrVersionNotFound
	}
	return nil
}

func isVersionNameConflict(err error) bool {
	message := err.Error()
	return strings.Contains(message, `versions_admin_id_workspace_id_name_key`) ||
		(strings.Contains(message, `duplicate key`) && strings.Contains(message, `versions`))
}

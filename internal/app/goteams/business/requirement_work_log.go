// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

var (
	// ErrRequirementWorkLogNotFound indicates a work-log row is outside the requested requirement scope or does not exist.
	ErrRequirementWorkLogNotFound = errors.New("requirement work log not found")
	// ErrRequirementWorkLogDateExists indicates that the requirement already has a log for the selected date.
	ErrRequirementWorkLogDateExists = errors.New("requirement work log date already exists")
	// ErrRequirementWorkHoursTotalInvalid indicates the completed-hours total exceeds the database field range.
	ErrRequirementWorkHoursTotalInvalid = errors.New("requirement completed work hours out of range")
)

// RequirementWorkHoursSummary contains stored and derived work-hour values for a requirement.
type RequirementWorkHoursSummary struct {
	Estimated float64
	Completed float64
	Remaining float64
	Overtime  float64
	Progress  int
}

// RequirementWorkLogModel is one persisted requirement work-log row.
type RequirementWorkLogModel struct {
	Id            int
	RequirementId int
	WorkDate      int64
	Hours         float64
	Description   string
	CreatedBy     int
	CreatorName   string
	CreatedAt     int64
	UpdatedAt     int64
}

// ListRequirementWorkLogsModel describes the visible work-log list use case.
type ListRequirementWorkLogsModel struct {
	AdminId       int
	UserId        int
	WorkspaceId   int
	RequirementId int
	Items         []RequirementWorkLogModel
	Summary       RequirementWorkHoursSummary
}

// SaveRequirementWorkLogModel describes the create/update work-log use case.
type SaveRequirementWorkLogModel struct {
	AdminId       int
	UserId        int
	WorkspaceId   int
	RequirementId int
	WorkLogId     int
	WorkDate      int64
	Hours         float64
	Description   string
	Item          RequirementWorkLogModel
	Summary       RequirementWorkHoursSummary
}

// DeleteRequirementWorkLogModel describes the delete work-log use case.
type DeleteRequirementWorkLogModel struct {
	AdminId       int
	WorkspaceId   int
	RequirementId int
	WorkLogId     int
	Summary       RequirementWorkHoursSummary
}

// CalculateRequirementWorkHours returns rounded derived values for a work-hour pair.
func CalculateRequirementWorkHours(estimated, completed float64) RequirementWorkHoursSummary {
	estimated = roundRequirementHours(estimated)
	completed = roundRequirementHours(completed)
	remaining := math.Max(estimated-completed, 0)
	overtime := math.Max(completed-estimated, 0)
	progress := 0
	if estimated > 0 {
		progress = int(math.Round(completed * 100 / estimated))
		if progress > 100 {
			progress = 100
		}
	}
	return RequirementWorkHoursSummary{
		Estimated: estimated,
		Completed: completed,
		Remaining: roundRequirementHours(remaining),
		Overtime:  roundRequirementHours(overtime),
		Progress:  progress,
	}
}

func roundRequirementHours(value float64) float64 {
	return math.Round(value*100) / 100
}

func parseRequirementHours(value, field string) (float64, error) {
	if value == `` {
		return 0, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", field, err)
	}
	return parsed, nil
}

func workHoursSummaryFromRow(row msql.Params) (RequirementWorkHoursSummary, error) {
	estimated, err := parseRequirementHours(row[`work_hours`], `work_hours`)
	if err != nil {
		return RequirementWorkHoursSummary{}, err
	}
	completed, err := parseRequirementHours(row[`completed_work_hours`], `completed_work_hours`)
	if err != nil {
		return RequirementWorkHoursSummary{}, err
	}
	return CalculateRequirementWorkHours(estimated, completed), nil
}

func toRequirementWorkLogModel(row msql.Params) (RequirementWorkLogModel, error) {
	hours, err := parseRequirementHours(row[`hours`], `hours`)
	if err != nil {
		return RequirementWorkLogModel{}, err
	}
	return RequirementWorkLogModel{
		Id:            cast.ToInt(row[`id`]),
		RequirementId: cast.ToInt(row[`requirement_id`]),
		WorkDate:      cast.ToInt64(row[`work_date`]),
		Hours:         hours,
		Description:   row[`description`],
		CreatedBy:     cast.ToInt(row[`created_by`]),
		CreatorName:   row[`creator_name`],
		CreatedAt:     cast.ToInt64(row[`created_at`]),
		UpdatedAt:     cast.ToInt64(row[`updated_at`]),
	}, nil
}

// ListRequirementWorkLogs returns visible work logs ordered by work date and id descending.
func ListRequirementWorkLogs(model ListRequirementWorkLogsModel) (ListRequirementWorkLogsModel, error) {
	requirement, err := GetVisibleRequirementById(model.AdminId, cast.ToString(model.WorkspaceId), cast.ToString(model.RequirementId), model.UserId)
	if err != nil {
		return ListRequirementWorkLogsModel{}, err
	}
	if len(requirement) == 0 || requirement[`type`] != define.TypeKeyRequirement {
		return ListRequirementWorkLogsModel{}, ErrRequirementNotFound
	}
	model.Summary, err = workHoursSummaryFromRow(requirement)
	if err != nil {
		return ListRequirementWorkLogsModel{}, err
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT l.id, l.requirement_id, l.work_date, l.hours,
		l.description, l.created_by, COALESCE(u.name, '') AS creator_name, l.created_at, l.updated_at
		FROM requirement_work_logs l
		LEFT JOIN users u ON u.id = l.created_by AND u.admin_id = l.admin_id
		WHERE l.admin_id = $1 AND l.workspace_id = $2 AND l.requirement_id = $3
		ORDER BY l.work_date DESC, l.id DESC`, nil, model.AdminId, model.WorkspaceId, model.RequirementId)
	if err != nil {
		logs.Error("list requirement work logs failed, err:%s", err.Error())
		return ListRequirementWorkLogsModel{}, err
	}
	model.Items = make([]RequirementWorkLogModel, 0, len(rows))
	for _, row := range rows {
		item, convErr := toRequirementWorkLogModel(row)
		if convErr != nil {
			return ListRequirementWorkLogsModel{}, convErr
		}
		model.Items = append(model.Items, item)
	}
	return model, nil
}

func lockRequirementForWorkLog(tx *sql.Tx, adminId, workspaceId, requirementId int) (msql.Params, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, type, work_hours, completed_work_hours
		FROM requirements WHERE admin_id = $1 AND workspace_id = $2 AND id = $3 FOR UPDATE`, tx,
		adminId, workspaceId, requirementId)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 || rows[0][`type`] != define.TypeKeyRequirement {
		return nil, ErrRequirementNotFound
	}
	return rows[0], nil
}

func updateCompletedWorkHours(tx *sql.Tx, adminId, workspaceId, requirementId int, requirement msql.Params, delta float64) (RequirementWorkHoursSummary, error) {
	current, err := parseRequirementHours(requirement[`completed_work_hours`], `completed_work_hours`)
	if err != nil {
		return RequirementWorkHoursSummary{}, err
	}
	completed := roundRequirementHours(current + delta)
	if completed < 0 && completed > -0.01 {
		completed = 0
	}
	if err = validate.ValidateRequirementWorkHours(completed); err != nil {
		return RequirementWorkHoursSummary{}, ErrRequirementWorkHoursTotalInvalid
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE requirements SET completed_work_hours = $1, updated_at = $2
		WHERE admin_id = $3 AND workspace_id = $4 AND id = $5`, tx,
		completed, tool.Time2Int(), adminId, workspaceId, requirementId); err != nil {
		return RequirementWorkHoursSummary{}, err
	}
	requirement[`completed_work_hours`] = strconv.FormatFloat(completed, 'f', 2, 64)
	return workHoursSummaryFromRow(requirement)
}

func findRequirementWorkLogTx(tx *sql.Tx, adminId, workspaceId, requirementId, workLogId int, forUpdate bool) (msql.Params, error) {
	lockSQL := ``
	if forUpdate {
		lockSQL = ` FOR UPDATE OF l`
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT l.id, l.requirement_id, l.work_date, l.hours,
		l.description, l.created_by, COALESCE(u.name, '') AS creator_name, l.created_at, l.updated_at
		FROM requirement_work_logs l
		LEFT JOIN users u ON u.id = l.created_by AND u.admin_id = l.admin_id
		WHERE l.admin_id = $1 AND l.workspace_id = $2 AND l.requirement_id = $3 AND l.id = $4`+lockSQL,
		tx, adminId, workspaceId, requirementId, workLogId)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrRequirementWorkLogNotFound
	}
	return rows[0], nil
}

func ensureRequirementWorkLogDateAvailable(tx *sql.Tx, adminId, workspaceId, requirementId int, workDate int64, excludeLogId int) error {
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM requirement_work_logs
		WHERE admin_id = $1 AND workspace_id = $2 AND requirement_id = $3 AND work_date = $4 AND id <> $5
		LIMIT 1`, tx, adminId, workspaceId, requirementId, workDate, excludeLogId)
	if err != nil {
		return err
	}
	if len(rows) > 0 {
		return ErrRequirementWorkLogDateExists
	}
	return nil
}

// CreateRequirementWorkLog creates a work log and increments the completed-hours summary atomically.
func CreateRequirementWorkLog(model SaveRequirementWorkLogModel) (SaveRequirementWorkLogModel, error) {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return SaveRequirementWorkLogModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	requirement, err := lockRequirementForWorkLog(tx, model.AdminId, model.WorkspaceId, model.RequirementId)
	if err != nil {
		return SaveRequirementWorkLogModel{}, err
	}
	if err = ensureRequirementWorkLogDateAvailable(tx, model.AdminId, model.WorkspaceId, model.RequirementId, model.WorkDate, 0); err != nil {
		return SaveRequirementWorkLogModel{}, err
	}
	now := tool.Time2Int()
	rows, err := msql.RawValues(define.Postgres, `INSERT INTO requirement_work_logs
		(admin_id, workspace_id, requirement_id, work_date, hours, description, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$8) RETURNING id`, tx,
		model.AdminId, model.WorkspaceId, model.RequirementId, model.WorkDate, model.Hours, model.Description, model.UserId, now)
	if err != nil {
		return SaveRequirementWorkLogModel{}, err
	}
	model.WorkLogId = cast.ToInt(rows[0][`id`])
	model.Summary, err = updateCompletedWorkHours(tx, model.AdminId, model.WorkspaceId, model.RequirementId, requirement, model.Hours)
	if err != nil {
		return SaveRequirementWorkLogModel{}, err
	}
	row, err := findRequirementWorkLogTx(tx, model.AdminId, model.WorkspaceId, model.RequirementId, model.WorkLogId, false)
	if err != nil {
		return SaveRequirementWorkLogModel{}, err
	}
	model.Item, err = toRequirementWorkLogModel(row)
	if err != nil {
		return SaveRequirementWorkLogModel{}, err
	}
	if err = tx.Commit(); err != nil {
		return SaveRequirementWorkLogModel{}, err
	}
	return model, nil
}

// UpdateRequirementWorkLog updates a work log and applies its hour delta to the requirement summary atomically.
func UpdateRequirementWorkLog(model SaveRequirementWorkLogModel) (SaveRequirementWorkLogModel, error) {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return SaveRequirementWorkLogModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	requirement, err := lockRequirementForWorkLog(tx, model.AdminId, model.WorkspaceId, model.RequirementId)
	if err != nil {
		return SaveRequirementWorkLogModel{}, err
	}
	oldRow, err := findRequirementWorkLogTx(tx, model.AdminId, model.WorkspaceId, model.RequirementId, model.WorkLogId, true)
	if err != nil {
		return SaveRequirementWorkLogModel{}, err
	}
	oldHours, err := parseRequirementHours(oldRow[`hours`], `hours`)
	if err != nil {
		return SaveRequirementWorkLogModel{}, err
	}
	if model.WorkDate != cast.ToInt64(oldRow[`work_date`]) {
		if err = ensureRequirementWorkLogDateAvailable(tx, model.AdminId, model.WorkspaceId, model.RequirementId, model.WorkDate, model.WorkLogId); err != nil {
			return SaveRequirementWorkLogModel{}, err
		}
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE requirement_work_logs
		SET work_date = $1, hours = $2, description = $3, updated_at = $4
		WHERE admin_id = $5 AND workspace_id = $6 AND requirement_id = $7 AND id = $8`, tx,
		model.WorkDate, model.Hours, model.Description, tool.Time2Int(), model.AdminId, model.WorkspaceId, model.RequirementId, model.WorkLogId); err != nil {
		return SaveRequirementWorkLogModel{}, err
	}
	model.Summary, err = updateCompletedWorkHours(tx, model.AdminId, model.WorkspaceId, model.RequirementId, requirement, model.Hours-oldHours)
	if err != nil {
		return SaveRequirementWorkLogModel{}, err
	}
	row, err := findRequirementWorkLogTx(tx, model.AdminId, model.WorkspaceId, model.RequirementId, model.WorkLogId, false)
	if err != nil {
		return SaveRequirementWorkLogModel{}, err
	}
	model.Item, err = toRequirementWorkLogModel(row)
	if err != nil {
		return SaveRequirementWorkLogModel{}, err
	}
	if err = tx.Commit(); err != nil {
		return SaveRequirementWorkLogModel{}, err
	}
	return model, nil
}

// DeleteRequirementWorkLog deletes a scoped work log and decrements the requirement summary atomically.
func DeleteRequirementWorkLog(model DeleteRequirementWorkLogModel) (DeleteRequirementWorkLogModel, error) {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return DeleteRequirementWorkLogModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	requirement, err := lockRequirementForWorkLog(tx, model.AdminId, model.WorkspaceId, model.RequirementId)
	if err != nil {
		return DeleteRequirementWorkLogModel{}, err
	}
	row, err := findRequirementWorkLogTx(tx, model.AdminId, model.WorkspaceId, model.RequirementId, model.WorkLogId, true)
	if err != nil {
		return DeleteRequirementWorkLogModel{}, err
	}
	hours, err := parseRequirementHours(row[`hours`], `hours`)
	if err != nil {
		return DeleteRequirementWorkLogModel{}, err
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM requirement_work_logs
		WHERE admin_id = $1 AND workspace_id = $2 AND requirement_id = $3 AND id = $4`, tx,
		model.AdminId, model.WorkspaceId, model.RequirementId, model.WorkLogId); err != nil {
		return DeleteRequirementWorkLogModel{}, err
	}
	model.Summary, err = updateCompletedWorkHours(tx, model.AdminId, model.WorkspaceId, model.RequirementId, requirement, -hours)
	if err != nil {
		return DeleteRequirementWorkLogModel{}, err
	}
	if err = tx.Commit(); err != nil {
		return DeleteRequirementWorkLogModel{}, err
	}
	return model, nil
}

// CountRequirementWorkLogs returns the scoped work-log count for a requirement.
func CountRequirementWorkLogs(adminId, workspaceId, requirementId int) (int, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT COUNT(*) AS total FROM requirement_work_logs
		WHERE admin_id = $1 AND workspace_id = $2 AND requirement_id = $3`, nil, adminId, workspaceId, requirementId)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return cast.ToInt(rows[0][`total`]), nil
}

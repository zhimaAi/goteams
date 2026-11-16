package business

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"goteams/internal/app/goteams/define"

	"github.com/lib/pq"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

const (
	statusActionType  = " AND type_key = 'requirement'"
	statusActionScope = " WHERE admin_id=$1 AND workspace_id=$2"
)

var (
	// ErrStatusActionWorkspace 表示工作区不支持工单状态操作。
	ErrStatusActionWorkspace = errors.New("status actions require an issue workspace")
	// ErrStatusActionInvalid 表示名称、数量或状态引用无效。
	ErrStatusActionInvalid = errors.New("invalid status action configuration")
	// ErrStatusActionNotFound 表示配置记录不存在。
	ErrStatusActionNotFound = errors.New("status action record not found")
	// ErrStatusActionConflict 表示配置重复、存在引用或操作已失效。
	ErrStatusActionConflict = errors.New("status action conflict")
)

// WorkspaceStatusActionModel 保存工单操作名称。
type WorkspaceStatusActionModel struct {
	Id   int
	Name string
}

// WorkspaceStatusActionMappingModel 保存工单状态与操作的关系。
type WorkspaceStatusActionMappingModel struct {
	Id             int
	SourceStatusId int
	ActionId       int
	TargetStatusId int
}

// WorkspaceStatusActionsModel 是单个工单管理工作区的状态操作配置。
type WorkspaceStatusActionsModel struct {
	AdminId     int
	WorkspaceId int
	Enabled     bool
	Actions     []WorkspaceStatusActionModel
	Mappings    []WorkspaceStatusActionMappingModel
}

// WorkspaceStatusActionChangeModel 描述开关或单条配置记录的变更。
type WorkspaceStatusActionChangeModel struct {
	AdminId     int
	WorkspaceId int
	Enabled     *bool
	Action      *WorkspaceStatusActionModel
	Mapping     *WorkspaceStatusActionMappingModel
	Delete      bool
}

// WorkItemStatusActionModel 标识请求所依据的状态操作关系。
type WorkItemStatusActionModel struct {
	MappingId      int
	SourceStatusId int
}

func statusActionRows(tx *sql.Tx, query string, args ...any) ([]msql.Params, error) {
	rows, err := msql.RawValues(define.Postgres, query, tx, args...)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
	}
	return rows, err
}

func statusActionExec(tx *sql.Tx, query string, args ...any) (sql.Result, error) {
	result, err := msql.RawExec(define.Postgres, query, tx, args...)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
	}
	return result, err
}

// lockStatusActionWorkspace 在状态操作、状态管理和工作区删除之间确定锁顺序。
func lockStatusActionWorkspace(tx *sql.Tx, adminId, workspaceId int, write bool) (string, error) {
	lock := " FOR SHARE"
	if write {
		lock = " FOR UPDATE"
	}
	rows, err := statusActionRows(tx, "SELECT workspace_type FROM "+define.TableWorkspaces+" WHERE admin_id=$1 AND id=$2"+lock, adminId, workspaceId)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", ErrStatusActionNotFound
	}
	return rows[0]["workspace_type"], nil
}

func statusActionEnabled(tx *sql.Tx, adminId, workspaceId int) (bool, error) {
	rows, err := statusActionRows(tx, "SELECT requirement_status_action_enabled FROM "+define.TableWorkspaceSettings+statusActionScope, adminId, workspaceId)
	if err != nil || len(rows) == 0 {
		return false, err
	}
	return strconv.ParseBool(rows[0]["requirement_status_action_enabled"])
}

func statusActionMapping(row msql.Params) (WorkspaceStatusActionMappingModel, error) {
	id, err := strconv.Atoi(row["id"])
	if err != nil {
		return WorkspaceStatusActionMappingModel{}, err
	}
	source, err := strconv.Atoi(row["source_status_id"])
	if err != nil {
		return WorkspaceStatusActionMappingModel{}, err
	}
	action, err := strconv.Atoi(row["action_id"])
	if err != nil {
		return WorkspaceStatusActionMappingModel{}, err
	}
	target, err := strconv.Atoi(row["target_status_id"])
	if err != nil {
		return WorkspaceStatusActionMappingModel{}, err
	}
	return WorkspaceStatusActionMappingModel{Id: id, SourceStatusId: source, ActionId: action, TargetStatusId: target}, nil
}

func readWorkspaceStatusActions(tx *sql.Tx, adminId, workspaceId int, includeDisabled bool) (WorkspaceStatusActionsModel, error) {
	model := WorkspaceStatusActionsModel{
		AdminId: adminId, WorkspaceId: workspaceId,
		Actions: []WorkspaceStatusActionModel{}, Mappings: []WorkspaceStatusActionMappingModel{},
	}
	enabled, err := statusActionEnabled(tx, adminId, workspaceId)
	if err != nil {
		return model, err
	}
	model.Enabled = enabled
	if !enabled && !includeDisabled {
		return model, nil
	}
	actions, err := statusActionRows(tx, "SELECT id,name FROM "+define.TableWorkspaceStatusActions+statusActionScope+statusActionType+" ORDER BY id", adminId, workspaceId)
	if err != nil {
		return model, err
	}
	for _, row := range actions {
		id, err := strconv.Atoi(row["id"])
		if err != nil {
			return model, err
		}
		model.Actions = append(model.Actions, WorkspaceStatusActionModel{Id: id, Name: row["name"]})
	}
	mappings, err := statusActionRows(tx, "SELECT id,source_status_id,action_id,target_status_id FROM "+define.TableWorkspaceStatusActionMappings+statusActionScope+statusActionType+" ORDER BY action_id,id", adminId, workspaceId)
	if err != nil {
		return model, err
	}
	for _, row := range mappings {
		mapping, err := statusActionMapping(row)
		if err != nil {
			return model, err
		}
		model.Mappings = append(model.Mappings, mapping)
	}
	return model, nil
}

// GetWorkspaceStatusActions 读取同一工作区锁保护下的状态操作配置。
func GetWorkspaceStatusActions(model WorkspaceStatusActionsModel, includeDisabled bool) (WorkspaceStatusActionsModel, error) {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return model, err
	}
	defer func() { _ = tx.Rollback() }()
	workspaceType, err := lockStatusActionWorkspace(tx, model.AdminId, model.WorkspaceId, false)
	if err != nil {
		return model, err
	}
	if workspaceType != define.WorkspaceTypeIssue {
		return model, ErrStatusActionWorkspace
	}
	return readWorkspaceStatusActions(tx, model.AdminId, model.WorkspaceId, includeDisabled)
}

// UpdateWorkspaceStatusActions 保存开关、操作名称或一条状态操作关系。
func UpdateWorkspaceStatusActions(change WorkspaceStatusActionChangeModel) (WorkspaceStatusActionsModel, error) {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return WorkspaceStatusActionsModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	workspaceType, err := lockStatusActionWorkspace(tx, change.AdminId, change.WorkspaceId, true)
	if err != nil {
		return WorkspaceStatusActionsModel{}, err
	}
	if workspaceType != define.WorkspaceTypeIssue {
		return WorkspaceStatusActionsModel{}, ErrStatusActionWorkspace
	}
	now := tool.Time2Int()
	if _, err = statusActionExec(tx, "INSERT INTO "+define.TableWorkspaceSettings+
		" (admin_id,workspace_id,created_at,updated_at) VALUES ($1,$2,$3,$3) ON CONFLICT (admin_id,workspace_id) DO NOTHING",
		change.AdminId, change.WorkspaceId, now); err != nil {
		return WorkspaceStatusActionsModel{}, err
	}
	switch {
	case change.Enabled != nil:
		_, err = statusActionExec(tx, "UPDATE "+define.TableWorkspaceSettings+" SET requirement_status_action_enabled=$3,updated_at=$4"+statusActionScope,
			change.AdminId, change.WorkspaceId, *change.Enabled, now)
	case change.Action != nil:
		err = writeStatusAction(tx, change, now)
	case change.Mapping != nil:
		err = writeStatusActionMapping(tx, change, now)
	default:
		err = ErrStatusActionInvalid
	}
	if err != nil {
		return WorkspaceStatusActionsModel{}, err
	}
	config, err := readWorkspaceStatusActions(tx, change.AdminId, change.WorkspaceId, true)
	if err != nil {
		return config, err
	}
	return config, tx.Commit()
}

func writeStatusAction(tx *sql.Tx, change WorkspaceStatusActionChangeModel, now int) error {
	table := define.TableWorkspaceStatusActions
	action := change.Action
	if change.Delete {
		return changeStatusActionRow(tx, "DELETE FROM "+table+statusActionScope+statusActionType+" AND id=$3",
			ErrStatusActionConflict, change.AdminId, change.WorkspaceId, action.Id)
	}
	name := strings.TrimSpace(action.Name)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 5 || strings.ContainsRune(name, 0) {
		return ErrStatusActionInvalid
	}
	if action.Id == 0 {
		if err := statusActionLimit(tx, table, 100, change); err != nil {
			return err
		}
		return changeStatusActionRow(tx, "INSERT INTO "+table+" (admin_id,workspace_id,type_key,name,created_at,updated_at) VALUES ($1,$2,'requirement',$3,$4,$4)",
			ErrStatusActionInvalid, change.AdminId, change.WorkspaceId, name, now)
	}
	return changeStatusActionRow(tx, "UPDATE "+table+" SET name=$4,updated_at=$5"+statusActionScope+statusActionType+" AND id=$3",
		ErrStatusActionInvalid, change.AdminId, change.WorkspaceId, action.Id, name, now)
}

func writeStatusActionMapping(tx *sql.Tx, change WorkspaceStatusActionChangeModel, now int) error {
	table := define.TableWorkspaceStatusActionMappings
	mapping := change.Mapping
	if change.Delete {
		return changeStatusActionRow(tx, "DELETE FROM "+table+statusActionScope+statusActionType+" AND id=$3",
			ErrStatusActionConflict, change.AdminId, change.WorkspaceId, mapping.Id)
	}
	if mapping.SourceStatusId <= 0 || mapping.TargetStatusId <= 0 || mapping.ActionId <= 0 || mapping.SourceStatusId == mapping.TargetStatusId {
		return ErrStatusActionInvalid
	}
	if mapping.Id == 0 {
		if err := statusActionLimit(tx, table, 500, change); err != nil {
			return err
		}
		return changeStatusActionRow(tx, "INSERT INTO "+table+" (admin_id,workspace_id,type_key,source_status_id,action_id,target_status_id,created_at,updated_at) VALUES ($1,$2,'requirement',$3,$4,$5,$6,$6)",
			ErrStatusActionInvalid, change.AdminId, change.WorkspaceId, mapping.SourceStatusId, mapping.ActionId, mapping.TargetStatusId, now)
	}
	return changeStatusActionRow(tx, "UPDATE "+table+" SET source_status_id=$4,action_id=$5,target_status_id=$6,updated_at=$7"+statusActionScope+statusActionType+" AND id=$3",
		ErrStatusActionInvalid, change.AdminId, change.WorkspaceId, mapping.Id, mapping.SourceStatusId, mapping.ActionId, mapping.TargetStatusId, now)
}

func statusActionLimit(tx *sql.Tx, table string, limit int, change WorkspaceStatusActionChangeModel) error {
	rows, err := statusActionRows(tx, "SELECT count(*) AS count FROM "+table+statusActionScope+statusActionType, change.AdminId, change.WorkspaceId)
	if err != nil {
		return err
	}
	count, err := strconv.Atoi(rows[0]["count"])
	if err == nil && count >= limit {
		return ErrStatusActionInvalid
	}
	return err
}

func changeStatusActionRow(tx *sql.Tx, query string, referenceErr error, args ...any) error {
	result, err := statusActionExec(tx, query, args...)
	if err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return ErrStatusActionConflict
			case "23503":
				return referenceErr
			}
		}
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return ErrStatusActionNotFound
	}
	return err
}

func checkWorkItemStatusAction(tx *sql.Tx, model TransitionWorkItemStatusModel, workItem msql.Params) error {
	action := model.StatusAction
	currentStatus, err := strconv.Atoi(workItem["status_id"])
	if err != nil {
		return err
	}
	if workItem["type"] != define.TypeKeyRequirement || currentStatus != action.SourceStatusId {
		return ErrStatusActionConflict
	}
	workspaceId, err := strconv.Atoi(model.WorkspaceId)
	if err != nil {
		return err
	}
	enabled, err := statusActionEnabled(tx, model.AdminId, workspaceId)
	if err != nil {
		return err
	}
	if !enabled {
		return ErrStatusActionConflict
	}
	rows, err := statusActionRows(tx, "SELECT id FROM "+define.TableWorkspaceStatusActionMappings+statusActionScope+statusActionType+
		" AND id=$3 AND source_status_id=$4 AND target_status_id=$5", model.AdminId, workspaceId, action.MappingId, currentStatus, model.StatusId)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return ErrStatusActionConflict
	}
	return nil
}

// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// ======================== Workflow Status / Priority CRUD ========================

// statusQueryFields are the status table's query fields, to avoid SELECT *.
const statusQueryFields = `id,name,color,sort_order,is_default`

// priorityQueryFields are the priority table's query fields, to avoid SELECT *.
const priorityQueryFields = `id,key,name,color,sort_order,is_default`

const builtinStatusFlag = 1

// ErrBuiltinWorkflowItem 以内置数据保护拒绝删除状态或优先级。
var ErrBuiltinWorkflowItem = errors.New("cannot delete a built-in workflow item")

// StatusModel is the work-item status business data.
type StatusModel struct {
	Id        int
	Name      string // Built-in status stores an i18n key; already translated to the request language when read
	Color     string
	SortOrder int
	IsDefault int
	IsBuiltin bool
}

// PriorityModel is the work-item priority business data.
type PriorityModel struct {
	Id        int
	Key       string // 跨工作空间映射使用的稳定优先级键（自动化规则引用）
	Label     string // 已按请求语言翻译后的展示名
	Color     string
	SortOrder int
	IsDefault int
	IsBuiltin bool
}

// ListStatusesModel describes the status list use case.
type ListStatusesModel struct {
	AdminId int
	WsId    string
	TypeKey string
	Lang    string
	Items   []StatusModel
}

// CreateStatusModel describes the create-status use case.
type CreateStatusModel struct {
	AdminId   int
	WsId      string
	TypeKey   string
	Name      string
	Color     string
	SortOrder *int
	Id        int
}

// UpdateStatusModel describes the update-status use case.
type UpdateStatusModel struct {
	AdminId   int
	WsId      string
	TypeKey   string
	StatusId  int
	Name      string
	Color     string
	SortOrder *int
}

// DeleteStatusModel describes the delete-status use case.
type DeleteStatusModel struct {
	AdminId  int
	WsId     string
	TypeKey  string
	StatusId int
}

// ListPrioritiesModel describes the priority list use case.
type ListPrioritiesModel struct {
	AdminId int
	WsId    string
	TypeKey string
	Lang    string
	Items   []PriorityModel
}

// CreatePriorityModel describes the create-priority use case.
type CreatePriorityModel struct {
	AdminId   int
	WsId      string
	TypeKey   string
	Name      string
	Color     string
	SortOrder *int
	Id        int
}

// UpdatePriorityModel describes the update-priority use case.
type UpdatePriorityModel struct {
	AdminId    int
	WsId       string
	TypeKey    string
	PriorityId int
	Name       string
	Color      string
	SortOrder  *int
}

// DeletePriorityModel describes the delete-priority use case.
type DeletePriorityModel struct {
	AdminId    int
	WsId       string
	TypeKey    string
	PriorityId int
}

// SetDefaultPriorityModel describes setting the default priority; PriorityId 0 clears the default.
type SetDefaultPriorityModel struct {
	AdminId    int
	WsId       string
	TypeKey    string
	PriorityId int
}

// ResetPrioritiesModel describes restoring default priorities.
type ResetPrioritiesModel struct {
	AdminId int
	WsId    string
	TypeKey string
}

// toStatusModel converts a status data row into the business model.
func toStatusModel(row msql.Params, lang string) (StatusModel, error) {
	id, err := strconv.Atoi(row[`id`])
	if err != nil {
		return StatusModel{}, fmt.Errorf(`parse status id: %w`, err)
	}
	sortOrder, err := strconv.Atoi(row[`sort_order`])
	if err != nil {
		return StatusModel{}, fmt.Errorf(`parse status sort_order: %w`, err)
	}
	isDefault, err := strconv.Atoi(row[`is_default`])
	if err != nil {
		return StatusModel{}, fmt.Errorf(`parse status is_default: %w`, err)
	}
	return StatusModel{
		Id:        id,
		Name:      lib_i18n.ShowStored(lang, row[`name`]),
		Color:     row[`color`],
		SortOrder: sortOrder,
		IsDefault: isDefault,
		IsBuiltin: isDefault == builtinStatusFlag,
	}, nil
}

// toPriorityModel converts a priority data row into the business model.
func toPriorityModel(row msql.Params, typeKey, lang string) (PriorityModel, error) {
	id, err := strconv.Atoi(row[`id`])
	if err != nil {
		return PriorityModel{}, fmt.Errorf(`parse priority id: %w`, err)
	}
	sortOrder, err := strconv.Atoi(row[`sort_order`])
	if err != nil {
		return PriorityModel{}, fmt.Errorf(`parse priority sort_order: %w`, err)
	}
	isDefault, err := strconv.Atoi(row[`is_default`])
	if err != nil {
		return PriorityModel{}, fmt.Errorf(`parse priority is_default: %w`, err)
	}
	return PriorityModel{
		Id:        id,
		Key:       row[`key`],
		Label:     lib_i18n.ShowStored(lang, row[`name`]),
		Color:     row[`color`],
		SortOrder: sortOrder,
		IsDefault: isDefault,
		IsBuiltin: builtinPriorityKey(typeKey, row[`key`], row[`name`]) != "",
	}, nil
}

func builtinPriorityKey(typeKey, key, storedName string) string {
	for _, preset := range presetPriorities {
		if preset.TypeKey != typeKey {
			continue
		}
		// 旧模板复制丢失了键，仍带原始国际化标记的预置项可以识别。
		if preset.Key == key || (key == "" && preset.Name == storedName) {
			return preset.Key
		}
	}
	return ""
}

// ListStatuses returns the status list for the given type under a workspace (ordered by sort_order).
func ListStatuses(model ListStatusesModel) (ListStatusesModel, error) {
	m := msql.Model(define.TableWorkItemStatuses, define.Postgres)
	list, err := m.Field(statusQueryFields).Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WsId},
		{`type_key`, model.TypeKey},
	}).Order(`sort_order asc, id asc`).Select()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return ListStatusesModel{}, err
	}
	items := make([]StatusModel, 0, len(list))
	for _, row := range list {
		item, convErr := toStatusModel(row, model.Lang)
		if convErr != nil {
			logs.Error("convert status row failed, err:%s", convErr.Error())
			return ListStatusesModel{}, convErr
		}
		items = append(items, item)
	}
	model.Items = items
	return model, nil
}

// CreateStatus creates a work-item status (key auto-incremented by the DB; user-specified keys are not allowed).
func CreateStatus(model CreateStatusModel) (CreateStatusModel, error) {
	if len(model.Name) == 0 {
		return CreateStatusModel{}, fmt.Errorf("status name is empty")
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return CreateStatusModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	workspaceID := cast.ToInt(model.WsId)
	if _, err := lockStatusActionWorkspace(tx, model.AdminId, workspaceID, true); err != nil {
		return CreateStatusModel{}, err
	}
	sortOrder := 0
	if model.SortOrder != nil {
		sortOrder = *model.SortOrder
	} else if sortOrder, err = nextWorkflowOptionSortOrder(tx, define.TableWorkItemStatuses, model.AdminId, workspaceID, model.TypeKey); err != nil {
		return CreateStatusModel{}, err
	}
	rows, err := statusActionRows(tx, "INSERT INTO "+define.TableWorkItemStatuses+
		" (admin_id,workspace_id,type_key,name,color,sort_order,is_default,created_at,updated_at)"+
		" VALUES ($1,$2,$3,$4,$5,$6,0,$7,$7) RETURNING id",
		model.AdminId, workspaceID, model.TypeKey, model.Name, model.Color, sortOrder, tool.Time2Int())
	if err != nil {
		return CreateStatusModel{}, err
	}
	model.Id, err = strconv.Atoi(rows[0]["id"])
	if err != nil {
		return CreateStatusModel{}, err
	}
	if err := initializeDisplaySettingStatus(tx, model.AdminId, model.WsId, model.TypeKey, model.Id); err != nil {
		return CreateStatusModel{}, err
	}
	if err := tx.Commit(); err != nil {
		return CreateStatusModel{}, err
	}
	return model, nil
}

// UpdateStatus updates a work-item status.
func UpdateStatus(model UpdateStatusModel) error {
	queryM := msql.Model(define.TableWorkItemStatuses, define.Postgres)
	row, err := queryM.Field(`id`).Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WsId},
		{`type_key`, model.TypeKey},
		{`id`, cast.ToString(model.StatusId)},
	}).Find()
	if err != nil {
		logs.Error("sql:%s, err:%s", queryM.GetLastSql(), err.Error())
		return err
	}
	if len(row) == 0 {
		return fmt.Errorf("status not found: %d", model.StatusId)
	}
	updates := msql.Datas{
		`name`:       model.Name,
		`color`:      model.Color,
		`updated_at`: tool.Time2Int(),
	}
	if model.SortOrder != nil {
		updates[`sort_order`] = *model.SortOrder
	}
	updateM := msql.Model(define.TableWorkItemStatuses, define.Postgres)
	_, err = updateM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WsId},
		{`type_key`, model.TypeKey},
		{`id`, cast.ToString(model.StatusId)},
	}).Update(updates)
	if err != nil {
		logs.Error("sql:%s, err:%s", updateM.GetLastSql(), err.Error())
		return err
	}
	return nil
}

// DeleteStatus deletes a work-item status.
func DeleteStatus(model DeleteStatusModel) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	workspaceType, err := lockStatusActionWorkspace(tx, model.AdminId, cast.ToInt(model.WsId), true)
	if err != nil {
		return err
	}
	statuses, err := statusActionRows(tx, "SELECT is_default FROM "+define.TableWorkItemStatuses+
		" WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND id=$4 FOR UPDATE",
		model.AdminId, model.WsId, model.TypeKey, model.StatusId)
	if err != nil {
		return err
	}
	if len(statuses) == 0 {
		return fmt.Errorf("status not found: %d", model.StatusId)
	}
	if cast.ToInt(statuses[0]["is_default"]) == builtinStatusFlag {
		return ErrBuiltinWorkflowItem
	}
	if workspaceType == define.WorkspaceTypeIssue && model.TypeKey == define.TypeKeyRequirement {
		if _, err = statusActionExec(tx, "DELETE FROM "+define.TableWorkspaceStatusActionMappings+statusActionScope+statusActionType+
			" AND (source_status_id=$3 OR target_status_id=$3)", model.AdminId, model.WsId, model.StatusId); err != nil {
			return err
		}
	}
	result, err := statusActionExec(tx, "DELETE FROM "+define.TableWorkItemStatuses+
		" WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND id=$4", model.AdminId, model.WsId, model.TypeKey, model.StatusId)
	if err != nil {
		return err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if deleted == 0 {
		return fmt.Errorf("status not found: %d", model.StatusId)
	}

	rows, err := statusActionRows(tx, "SELECT requirement_initial_status_id,requirement_final_status_ids,requirement_completed_status_ids,"+
		"defect_initial_status_id,defect_final_status_ids,defect_completed_status_ids,planning_initial_status_id,planning_final_status_ids FROM "+
		define.TableWorkspaceSettings+" WHERE admin_id=$1 AND workspace_id=$2 FOR UPDATE", model.AdminId, model.WsId)
	if err != nil {
		return err
	}
	var settingsData msql.Datas
	if len(rows) != 0 {
		row := rows[0]
		settingsData = deletedStatusSettingsData(WorkspaceSettingsModel{
			RequirementInitialStatusId:    cast.ToInt(row["requirement_initial_status_id"]),
			RequirementFinalStatusIds:     toIntSlice(row["requirement_final_status_ids"]),
			RequirementCompletedStatusIds: toIntSlice(row["requirement_completed_status_ids"]),
			DefectInitialStatusId:         cast.ToInt(row["defect_initial_status_id"]),
			DefectFinalStatusIds:          toIntSlice(row["defect_final_status_ids"]),
			DefectCompletedStatusIds:      toIntSlice(row["defect_completed_status_ids"]),
			PlanningInitialStatusId:       cast.ToInt(row["planning_initial_status_id"]),
			PlanningFinalStatusIds:        toIntSlice(row["planning_final_status_ids"]),
		}, model.TypeKey, model.StatusId)
	}
	if len(settingsData) != 0 {
		columns := make([]string, 0, len(settingsData))
		for column := range settingsData {
			columns = append(columns, column)
		}
		sort.Strings(columns)
		args := []any{model.AdminId, model.WsId, tool.Time2Int()}
		assignments := []string{"updated_at=$3"}
		for _, column := range columns {
			args = append(args, settingsData[column])
			assignments = append(assignments, column+"=$"+strconv.Itoa(len(args)))
		}
		if _, err := statusActionExec(tx, "UPDATE "+define.TableWorkspaceSettings+" SET "+strings.Join(assignments, ",")+
			" WHERE admin_id=$1 AND workspace_id=$2", args...); err != nil {
			return err
		}
	}
	if err = RemoveDisplaySettingStatusTx(tx, model.AdminId, model.WsId, model.TypeKey, model.StatusId); err != nil {
		logs.Error("清理状态显示设置失败: %s", err.Error())
		return err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("提交状态删除事务失败: %s", err.Error())
		return err
	}
	if len(settingsData) != 0 {
		if err := RecalcWorkspaceVersionCounts(model.AdminId, cast.ToInt(model.WsId)); err != nil {
			logs.Error("RecalcWorkspaceVersionCounts after status delete failed, err:%s", err.Error())
		}
	}
	return nil
}

// ListPriorities returns the priority list for the given type under a workspace (ordered by sort_order).
func ListPriorities(model ListPrioritiesModel) (ListPrioritiesModel, error) {
	m := msql.Model(define.TableWorkItemPriorities, define.Postgres)
	list, err := m.Field(priorityQueryFields).Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WsId},
		{`type_key`, model.TypeKey},
	}).Order(`sort_order asc, id asc`).Select()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return ListPrioritiesModel{}, err
	}
	items := make([]PriorityModel, 0, len(list))
	for _, row := range list {
		item, convErr := toPriorityModel(row, model.TypeKey, model.Lang)
		if convErr != nil {
			logs.Error("convert priority row failed, err:%s", convErr.Error())
			return ListPrioritiesModel{}, convErr
		}
		items = append(items, item)
	}
	model.Items = items
	return model, nil
}

// getDefaultPriorityId returns the priority ID marked as default for the given type under a workspace (returns 0 when none).
func getDefaultPriorityId(adminId int, wsId, typeKey string) int {
	m := msql.Model(define.TableWorkItemPriorities, define.Postgres)
	id, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, wsId},
		{`type_key`, typeKey},
		{`is_default`, `1`},
	}).Value(`id`)
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return 0
	}
	return cast.ToInt(id)
}

// SetDefaultPriority sets the default priority for a work-item type; PriorityId == 0 clears the default.
func SetDefaultPriority(model SetDefaultPriorityModel) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin set default priority transaction failed, err:%s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := lockStatusActionWorkspace(tx, model.AdminId, cast.ToInt(model.WsId), true); err != nil {
		return err
	}

	if model.PriorityId > 0 {
		rows, queryErr := msql.RawValues(define.Postgres, `SELECT id FROM workspace_priorities
			WHERE admin_id = $1 AND workspace_id = $2 AND type_key = $3 AND id = $4 FOR UPDATE`,
			tx, model.AdminId, cast.ToInt(model.WsId), model.TypeKey, model.PriorityId)
		if queryErr != nil {
			logs.Error("query default priority failed, err:%s", queryErr.Error())
			return queryErr
		}
		if len(rows) == 0 {
			return ErrPriorityNotFound
		}
	}

	now := tool.Time2Int()
	if _, err = msql.RawExec(define.Postgres, `UPDATE workspace_priorities
		SET is_default = 0, updated_at = $1
		WHERE admin_id = $2 AND workspace_id = $3 AND type_key = $4 AND is_default <> 0`,
		tx, now, model.AdminId, cast.ToInt(model.WsId), model.TypeKey); err != nil {
		logs.Error("clear default priority failed, err:%s", err.Error())
		return err
	}
	if model.PriorityId > 0 {
		if _, err = msql.RawExec(define.Postgres, `UPDATE workspace_priorities
			SET is_default = 1, updated_at = $1
			WHERE admin_id = $2 AND workspace_id = $3 AND type_key = $4 AND id = $5`,
			tx, now, model.AdminId, cast.ToInt(model.WsId), model.TypeKey, model.PriorityId); err != nil {
			logs.Error("set default priority failed, err:%s", err.Error())
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit set default priority transaction failed, err:%s", err.Error())
	}
	return err
}

// CreatePriority creates a work-item priority (ID auto-incremented by the DB; user-specified IDs are not allowed).
func CreatePriority(model CreatePriorityModel) (CreatePriorityModel, error) {
	if len(model.Name) == 0 {
		return CreatePriorityModel{}, fmt.Errorf("priority name is empty")
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return CreatePriorityModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	workspaceID := cast.ToInt(model.WsId)
	if _, err := lockStatusActionWorkspace(tx, model.AdminId, workspaceID, true); err != nil {
		return CreatePriorityModel{}, err
	}
	sortOrder := 0
	if model.SortOrder != nil {
		sortOrder = *model.SortOrder
	} else if sortOrder, err = nextWorkflowOptionSortOrder(tx, define.TableWorkItemPriorities, model.AdminId, workspaceID, model.TypeKey); err != nil {
		return CreatePriorityModel{}, err
	}
	now := tool.Time2Int()
	// 稳定键与自增 ID 无关，供自动化规则跨工作空间引用。
	key := `priority_` + tool.MD5(cast.ToString(model.AdminId) + model.WsId + model.TypeKey + model.Name + cast.ToString(now))[:12]
	rows, err := statusActionRows(tx, "INSERT INTO "+define.TableWorkItemPriorities+
		" (admin_id,workspace_id,type_key,key,name,color,sort_order,is_default,created_at,updated_at)"+
		" VALUES ($1,$2,$3,$4,$5,$6,$7,0,$8,$8) RETURNING id",
		model.AdminId, workspaceID, model.TypeKey, key, model.Name, model.Color, sortOrder, now)
	if err != nil {
		return CreatePriorityModel{}, err
	}
	model.Id, err = strconv.Atoi(rows[0]["id"])
	if err != nil {
		return CreatePriorityModel{}, err
	}
	if err := tx.Commit(); err != nil {
		return CreatePriorityModel{}, err
	}
	return model, nil
}

// UpdatePriority updates a work-item priority.
func UpdatePriority(model UpdatePriorityModel) error {
	queryM := msql.Model(define.TableWorkItemPriorities, define.Postgres)
	row, err := queryM.Field(`id,key,name`).Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WsId},
		{`type_key`, model.TypeKey},
		{`id`, cast.ToString(model.PriorityId)},
	}).Find()
	if err != nil {
		logs.Error("sql:%s, err:%s", queryM.GetLastSql(), err.Error())
		return err
	}
	if len(row) == 0 {
		return fmt.Errorf("priority not found: %d", model.PriorityId)
	}
	updates := msql.Datas{
		`name`:       model.Name,
		`color`:      model.Color,
		`updated_at`: tool.Time2Int(),
	}
	if model.SortOrder != nil {
		updates[`sort_order`] = *model.SortOrder
	}
	if row[`key`] == "" {
		// 改名前补齐旧模板丢失的键，保留预置来源。
		key := builtinPriorityKey(model.TypeKey, "", row[`name`])
		if key == "" {
			key = "priority_" + strconv.Itoa(model.PriorityId)
		}
		updates[`key`] = key
	}
	updateM := msql.Model(define.TableWorkItemPriorities, define.Postgres)
	_, err = updateM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WsId},
		{`type_key`, model.TypeKey},
		{`id`, cast.ToString(model.PriorityId)},
	}).Update(updates)
	if err != nil {
		logs.Error("sql:%s, err:%s", updateM.GetLastSql(), err.Error())
		return err
	}
	return nil
}

// DeletePriority deletes a work-item priority.
func DeletePriority(model DeletePriorityModel) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := lockStatusActionWorkspace(tx, model.AdminId, cast.ToInt(model.WsId), true); err != nil {
		return err
	}
	scope := " WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND id=$4"
	rows, err := statusActionRows(tx, "SELECT key,name FROM "+define.TableWorkItemPriorities+scope+" FOR UPDATE",
		model.AdminId, model.WsId, model.TypeKey, model.PriorityId)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("priority not found: %d", model.PriorityId)
	}
	if builtinPriorityKey(model.TypeKey, rows[0][`key`], rows[0][`name`]) != "" {
		return ErrBuiltinWorkflowItem
	}
	if _, err := statusActionExec(tx, "DELETE FROM "+define.TableWorkItemPriorities+scope,
		model.AdminId, model.WsId, model.TypeKey, model.PriorityId); err != nil {
		return err
	}
	return tx.Commit()
}

// ResetPriorities restores the priorities of the given type under a workspace to the built-in defaults.
// It clears user-created/modified priorities of that type and re-seeds the built-in priorities.
func ResetPriorities(model ResetPrioritiesModel) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := lockStatusActionWorkspace(tx, model.AdminId, cast.ToInt(model.WsId), true); err != nil {
		return err
	}
	scope := " WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3"
	if _, err := statusActionExec(tx, "DELETE FROM "+define.TableWorkItemPriorities+scope,
		model.AdminId, model.WsId, model.TypeKey); err != nil {
		return err
	}
	now := tool.Time2Int()
	for _, priority := range presetPriorities {
		if priority.TypeKey != model.TypeKey {
			continue
		}
		if _, err := statusActionExec(tx, "INSERT INTO "+define.TableWorkItemPriorities+
			" (admin_id,workspace_id,type_key,key,name,color,sort_order,is_default,created_at,updated_at)"+
			" VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)",
			model.AdminId, model.WsId, priority.TypeKey, priority.Key, priority.Name, priority.Color, priority.SortOrder, priority.IsDefault, now); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	logs.Info("workspace [%s] %s priorities reset to default", model.WsId, model.TypeKey)
	return nil
}

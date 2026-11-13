// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"fmt"
	"strconv"

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
const priorityQueryFields = `id,name,color,sort_order,is_default`

// StatusModel is the work-item status business data.
type StatusModel struct {
	Id        int
	Name      string // Built-in status stores an i18n key; already translated to the request language when read
	Color     string
	SortOrder int
	IsDefault int
}

// PriorityModel is the work-item priority business data.
type PriorityModel struct {
	Id        int
	Key       string // 跨工作空间映射使用的稳定优先级键（自动化规则引用）
	Label     string // 已按请求语言翻译后的展示名
	Color     string
	SortOrder int
	IsDefault int
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
	SortOrder int
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
	SortOrder int
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
	SortOrder int
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
	SortOrder  int
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
	}, nil
}

// toPriorityModel converts a priority data row into the business model.
func toPriorityModel(row msql.Params, lang string) (PriorityModel, error) {
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
	}, nil
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
	// Ensure the status name is unique within the same workspace and type.
	queryM := msql.Model(define.TableWorkItemStatuses, define.Postgres)
	exist, err := queryM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WsId},
		{`type_key`, model.TypeKey},
		{`name`, model.Name},
	}).Value(`id`)
	if err != nil {
		logs.Error("sql:%s, err:%s", queryM.GetLastSql(), err.Error())
		return CreateStatusModel{}, err
	}
	if len(exist) > 0 {
		return CreateStatusModel{}, fmt.Errorf("status name already exists: %s", model.Name)
	}
	now := tool.Time2Int()
	insertM := msql.Model(define.TableWorkItemStatuses, define.Postgres)
	id, err := insertM.Insert(msql.Datas{
		`admin_id`:     model.AdminId,
		`workspace_id`: model.WsId,
		`type_key`:     model.TypeKey,
		`name`:         model.Name,
		`color`:        model.Color,
		`sort_order`:   model.SortOrder,
		`is_default`:   0,
		`created_at`:   now,
		`updated_at`:   now,
	}, `id`)
	if err != nil {
		logs.Error("sql:%s, err:%s", insertM.GetLastSql(), err.Error())
		return CreateStatusModel{}, err
	}
	model.Id = cast.ToInt(id)
	if err := InitializeDisplaySettingStatus(model.AdminId, model.WsId, model.TypeKey, model.Id); err != nil {
		logs.Error("initialize display setting status failed, err:%s", err.Error())
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
	now := tool.Time2Int()
	updateM := msql.Model(define.TableWorkItemStatuses, define.Postgres)
	_, err = updateM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WsId},
		{`type_key`, model.TypeKey},
		{`id`, cast.ToString(model.StatusId)},
	}).Update(msql.Datas{
		`name`:       model.Name,
		`color`:      model.Color,
		`sort_order`: model.SortOrder,
		`updated_at`: now,
	})
	if err != nil {
		logs.Error("sql:%s, err:%s", updateM.GetLastSql(), err.Error())
		return err
	}
	return nil
}

// DeleteStatus deletes a work-item status.
func DeleteStatus(model DeleteStatusModel) error {
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
	settings, err := GetWorkspaceSettings(WorkspaceSettingsModel{AdminId: model.AdminId, WsId: model.WsId})
	if err != nil {
		return err
	}
	settingsData := deletedStatusSettingsData(settings, model.TypeKey, model.StatusId)

	txM := msql.Model(define.TableWorkItemStatuses, define.Postgres)
	if err = txM.Begin(); err != nil {
		logs.Error("begin delete status transaction failed, err:%s", err.Error())
		return err
	}
	defer func() { _ = txM.Rollback() }()
	if _, err = txM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WsId},
		{`type_key`, model.TypeKey},
		{`id`, cast.ToString(model.StatusId)},
	}).Delete(); err != nil {
		logs.Error("sql:%s, err:%s", txM.GetLastSql(), err.Error())
		return err
	}
	if len(settingsData) > 0 {
		now := tool.Time2Int()
		settingsData[`updated_at`] = now
		txM.Table(define.TableWorkspaceSettings)
		exist, queryErr := txM.Where2([][]string{
			{`admin_id`, cast.ToString(model.AdminId)},
			{`workspace_id`, model.WsId},
		}).Value(`id`)
		if queryErr != nil {
			return queryErr
		}
		if len(exist) > 0 {
			_, err = txM.Where2([][]string{
				{`admin_id`, cast.ToString(model.AdminId)},
				{`workspace_id`, model.WsId},
			}).Update(settingsData)
		} else {
			settingsData[`admin_id`] = model.AdminId
			settingsData[`workspace_id`] = model.WsId
			settingsData[`created_at`] = now
			_, err = txM.Insert(settingsData)
		}
		if err != nil {
			logs.Error("clear deleted status from settings failed, sql:%s, err:%s", txM.GetLastSql(), err.Error())
			return err
		}
	}
	if err = txM.Commit(); err != nil {
		logs.Error("commit delete status transaction failed, err:%s", err.Error())
		return err
	}
	if err := RemoveDisplaySettingStatus(model.AdminId, model.WsId, model.TypeKey, model.StatusId); err != nil {
		logs.Error("clear deleted status display setting failed, err:%s", err.Error())
		return err
	}
	if len(settingsData) > 0 {
		if recalcErr := RecalcWorkspaceVersionCounts(model.AdminId, cast.ToInt(model.WsId)); recalcErr != nil {
			logs.Error("RecalcWorkspaceVersionCounts after status delete failed, err:%s", recalcErr.Error())
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
		item, convErr := toPriorityModel(row, model.Lang)
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
	now := tool.Time2Int()
	insertM := msql.Model(define.TableWorkItemPriorities, define.Postgres)
	id, err := insertM.Insert(msql.Datas{
		`admin_id`:     model.AdminId,
		`workspace_id`: model.WsId,
		`type_key`:     model.TypeKey,
		// 稳定键与自增 id 无关，供自动化规则跨工作空间引用。
		`key`:        `priority_` + tool.MD5(cast.ToString(model.AdminId) + model.WsId + model.TypeKey + model.Name + cast.ToString(now))[:12],
		`name`:       model.Name,
		`color`:      model.Color,
		`sort_order`: model.SortOrder,
		`is_default`: 0,
		`created_at`: now,
		`updated_at`: now,
	}, `id`)
	if err != nil {
		logs.Error("sql:%s, err:%s", insertM.GetLastSql(), err.Error())
		return CreatePriorityModel{}, err
	}
	model.Id = cast.ToInt(id)
	return model, nil
}

// UpdatePriority updates a work-item priority.
func UpdatePriority(model UpdatePriorityModel) error {
	queryM := msql.Model(define.TableWorkItemPriorities, define.Postgres)
	row, err := queryM.Field(`id`).Where2([][]string{
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
	now := tool.Time2Int()
	updateM := msql.Model(define.TableWorkItemPriorities, define.Postgres)
	_, err = updateM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WsId},
		{`type_key`, model.TypeKey},
		{`id`, cast.ToString(model.PriorityId)},
	}).Update(msql.Datas{
		`name`:       model.Name,
		`color`:      model.Color,
		`sort_order`: model.SortOrder,
		`updated_at`: now,
	})
	if err != nil {
		logs.Error("sql:%s, err:%s", updateM.GetLastSql(), err.Error())
		return err
	}
	return nil
}

// DeletePriority deletes a work-item priority.
func DeletePriority(model DeletePriorityModel) error {
	queryM := msql.Model(define.TableWorkItemPriorities, define.Postgres)
	row, err := queryM.Field(`id`).Where2([][]string{
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
	delM := msql.Model(define.TableWorkItemPriorities, define.Postgres)
	if _, err := delM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WsId},
		{`type_key`, model.TypeKey},
		{`id`, cast.ToString(model.PriorityId)},
	}).Delete(); err != nil {
		logs.Error("sql:%s, err:%s", delM.GetLastSql(), err.Error())
		return err
	}
	return nil
}

// ResetPriorities restores the priorities of the given type under a workspace to the built-in defaults.
// It clears user-created/modified priorities of that type and re-seeds the built-in priorities.
func ResetPriorities(model ResetPrioritiesModel) error {
	now := tool.Time2Int()
	delM := msql.Model(define.TableWorkItemPriorities, define.Postgres)
	if _, err := delM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WsId},
		{`type_key`, model.TypeKey},
	}).Delete(); err != nil {
		logs.Error("reset priorities delete sql:%s, err:%s", delM.GetLastSql(), err.Error())
		return err
	}
	for _, priority := range presetPriorities {
		if priority.TypeKey != model.TypeKey {
			continue
		}
		if err := ensureWorkspacePriority(model.AdminId, model.WsId, priority, now); err != nil {
			return err
		}
	}
	logs.Info("workspace [%s] %s priorities reset to default", model.WsId, model.TypeKey)
	return nil
}

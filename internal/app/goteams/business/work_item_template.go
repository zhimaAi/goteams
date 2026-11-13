// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

var (
	// ErrWorkItemTemplateNotFound indicates the template is missing or not visible in this workspace.
	ErrWorkItemTemplateNotFound = errors.New("work item template not found")
	// ErrWorkItemTemplateInUse indicates the pool template is still linked to a workspace.
	ErrWorkItemTemplateInUse = errors.New("work item template still linked")
	// ErrWorkItemTemplateNotLinked indicates the workspace has not linked this template.
	ErrWorkItemTemplateNotLinked = errors.New("work item template not linked")
	// ErrWorkItemTemplateFieldInvalid indicates a field_key is unknown for this workspace type.
	ErrWorkItemTemplateFieldInvalid = errors.New("work item template field invalid")
)

// templateBuiltinReadonlyKeys are builtin fields a template cannot pre-assign: their
// values belong to the work item itself, so any submitted default is dropped.
var templateBuiltinReadonlyKeys = map[string]bool{
	`id`: true, `created_at`: true, `created_by`: true, `work_progress`: true,
	`remaining_work_hours`: true, `overtime_work_hours`: true,
}

// templateDateBuiltinKeys are the builtin fields whose default is a date. Their default
// accepts either a fixed timestamp or a relative one (see validateTemplateDynamicDate).
var templateDateBuiltinKeys = map[string]bool{
	`planned_start_date`: true, `planned_end_date`: true,
}

// TemplateLinkageModel is one option-value condition controlling related field visibility.
type TemplateLinkageModel struct {
	Value      json.RawMessage
	ValueNames []string
	FieldKeys  []string
}

// TemplateFieldModel is one field row stored on a work-item template.
type TemplateFieldModel struct {
	FieldKey     string
	FieldName    string
	FieldType    string
	DefaultValue json.RawMessage
	DefaultNames []string
	Required     bool
	Linkages     []TemplateLinkageModel
	// Display controls whether the field is shown on the public invitation form;
	// nil means unset and is treated as shown by consumers.
	Display *bool
	// LinkParam 是邀请链接里给这个字段传值用的参数名，空表示不参与链接传参；
	// 项目模板不用它。
	LinkParam string
}

type templateLinkageJSON struct {
	Value      json.RawMessage `json:"value"`
	ValueNames []string        `json:"value_names,omitempty"`
	FieldKeys  []string        `json:"field_keys"`
}

type templateFieldJSON struct {
	FieldKey     string                `json:"field_key"`
	FieldName    string                `json:"field_name,omitempty"`
	FieldType    string                `json:"field_type,omitempty"`
	DefaultValue json.RawMessage       `json:"default_value"`
	DefaultNames []string              `json:"default_names,omitempty"`
	Required     bool                  `json:"required"`
	Linkages     []templateLinkageJSON `json:"linkages,omitempty"`
	Display      *bool                 `json:"display,omitempty"`
	LinkParam    string                `json:"link_param,omitempty"`
}

// WorkItemTemplateModel is a pool template plus its field list.
type WorkItemTemplateModel struct {
	Id                int
	SourceWorkspaceId int
	TypeKey           string
	Name              string
	Title             string
	Description       string
	DescriptionMode   string
	Fields            []TemplateFieldModel
	FieldCount        int
	CreatedBy         int
	CreatedAt         int64
	UpdatedAt         int64
}

// ListWorkItemTemplatesModel lists templates linked to a workspace.
type ListWorkItemTemplatesModel struct {
	AdminId     int
	WorkspaceId string
	TypeKey     string
	Enabled     bool
	Items       []WorkItemTemplateModel
}

// GetWorkItemTemplateModel loads one linked template.
type GetWorkItemTemplateModel struct {
	AdminId     int
	WorkspaceId string
	TemplateId  int
	TypeKey     string
	Item        WorkItemTemplateModel
}

// SaveWorkItemTemplateModel creates or updates a pool template and links it to the workspace.
type SaveWorkItemTemplateModel struct {
	AdminId         int
	UserId          int
	WorkspaceId     string
	TemplateId      int
	TypeKey         string
	Name            string
	Title           string
	Description     string
	DescriptionMode string
	Fields          []TemplateFieldModel
	Id              int
}

// UpdateWorkItemTemplateSettingsModel persists the per-type template switch.
type UpdateWorkItemTemplateSettingsModel struct {
	AdminId     int
	WorkspaceId string
	TypeKey     string
	Enabled     bool
}

// LinkWorkItemTemplatesModel links existing pool templates to a workspace.
type LinkWorkItemTemplatesModel struct {
	AdminId     int
	WorkspaceId string
	TypeKey     string
	TemplateIds []int
}

// DeleteWorkItemTemplateModel removes a workspace association and deletes the orphaned template.
type DeleteWorkItemTemplateModel struct {
	AdminId     int
	WorkspaceId string
	TemplateId  int
	TypeKey     string
}

// UnlinkWorkItemTemplateModel removes a workspace association.
type UnlinkWorkItemTemplateModel struct {
	AdminId     int
	WorkspaceId string
	TemplateId  int
	TypeKey     string
}

// ListWorkItemTemplatePoolModel lists tenant pool templates not yet linked to this workspace.
type ListWorkItemTemplatePoolModel struct {
	AdminId     int
	WorkspaceId string
	TypeKey     string
	Items       []WorkItemTemplatePoolItemModel
}

// WorkItemTemplatePoolItemModel is one unlinked pool template.
type WorkItemTemplatePoolItemModel struct {
	Id        int
	Name      string
	BindCount int
	CanDelete bool
}

// DeleteWorkItemTemplatePoolModel deletes a pool template that is not linked anywhere.
type DeleteWorkItemTemplatePoolModel struct {
	AdminId     int
	WorkspaceId string
	TemplateId  int
	TypeKey     string
}

// ListWorkItemTemplates returns the workspace switch and linked templates.
func ListWorkItemTemplates(model ListWorkItemTemplatesModel) (ListWorkItemTemplatesModel, error) {
	enabled, err := loadTemplateEnabled(model.AdminId, model.WorkspaceId, model.TypeKey)
	if err != nil {
		return ListWorkItemTemplatesModel{}, err
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT t.id, t.source_workspace_id, t.type_key, t.name, t.title, t.description, t.description_mode,
		t.fields_json, t.created_by, t.created_at, t.updated_at
		FROM `+define.TableWorkItemTemplates+` t
		JOIN `+define.TableWorkspaceWorkItemTemplates+` a
			ON a.admin_id = t.admin_id AND a.template_id = t.id
		WHERE t.admin_id = $1 AND a.workspace_id = $2 AND t.type_key = $3
		ORDER BY a.sort_order ASC, a.id ASC`, nil, model.AdminId, cast.ToInt(model.WorkspaceId), model.TypeKey)
	if err != nil {
		logs.Error("list work item templates failed, err:%s", err.Error())
		return ListWorkItemTemplatesModel{}, err
	}
	items := make([]WorkItemTemplateModel, 0, len(rows))
	for _, row := range rows {
		item, convErr := toWorkItemTemplateModel(row)
		if convErr != nil {
			return ListWorkItemTemplatesModel{}, convErr
		}
		items = append(items, item)
	}
	items, err = resolveWorkItemTemplates(model.AdminId, model.WorkspaceId, model.TypeKey, items, false)
	if err != nil {
		return ListWorkItemTemplatesModel{}, err
	}
	model.Enabled = enabled
	model.Items = items
	return model, nil
}

// GetWorkItemTemplate returns one template that is linked to the workspace.
func GetWorkItemTemplate(model GetWorkItemTemplateModel) (GetWorkItemTemplateModel, error) {
	item, err := loadLinkedTemplate(model.AdminId, model.WorkspaceId, model.TemplateId, model.TypeKey)
	if err != nil {
		return GetWorkItemTemplateModel{}, err
	}
	items, err := resolveWorkItemTemplates(model.AdminId, model.WorkspaceId, item.TypeKey, []WorkItemTemplateModel{item}, false)
	if err != nil {
		return GetWorkItemTemplateModel{}, err
	}
	model.Item = items[0]
	return model, nil
}

// CreateWorkItemTemplate writes a pool template and links it to the current workspace.
func CreateWorkItemTemplate(model SaveWorkItemTemplateModel) (SaveWorkItemTemplateModel, error) {
	if err := normalizeAndValidateTemplateFields(model.AdminId, model.WorkspaceId, model.TypeKey, model.Fields); err != nil {
		return SaveWorkItemTemplateModel{}, err
	}
	fieldsJSON, err := encodeTemplateFields(model.Fields)
	if err != nil {
		return SaveWorkItemTemplateModel{}, err
	}
	mode := model.DescriptionMode
	if mode == `` {
		mode = define.DescriptionModeRichText
	}
	now := tool.Time2Int()
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return SaveWorkItemTemplateModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	inserted, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableWorkItemTemplates+`
		(admin_id, source_workspace_id, type_key, name, title, description, description_mode, fields_json, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10) RETURNING id`, tx,
		model.AdminId, cast.ToInt(model.WorkspaceId), model.TypeKey, model.Name, model.Title, model.Description, mode, fieldsJSON, model.UserId, now)
	if err != nil {
		logs.Error("create work item template failed, err:%s", err.Error())
		return SaveWorkItemTemplateModel{}, err
	}
	if len(inserted) == 0 {
		return SaveWorkItemTemplateModel{}, errors.New(`create work item template returned no id`)
	}
	templateId := cast.ToInt(inserted[0][`id`])
	if err = insertTemplateLinkTx(tx, model.AdminId, model.WorkspaceId, templateId, now); err != nil {
		return SaveWorkItemTemplateModel{}, err
	}
	if err = tx.Commit(); err != nil {
		return SaveWorkItemTemplateModel{}, err
	}
	model.Id = templateId
	return model, nil
}

// UpdateWorkItemTemplate updates a pool template that is linked to this workspace.
func UpdateWorkItemTemplate(model SaveWorkItemTemplateModel) error {
	if _, err := loadLinkedTemplate(model.AdminId, model.WorkspaceId, model.TemplateId, model.TypeKey); err != nil {
		return err
	}
	if err := normalizeAndValidateTemplateFields(model.AdminId, model.WorkspaceId, model.TypeKey, model.Fields); err != nil {
		return err
	}
	fieldsJSON, err := encodeTemplateFields(model.Fields)
	if err != nil {
		return err
	}
	mode := model.DescriptionMode
	if mode == `` {
		mode = define.DescriptionModeRichText
	}
	m := msql.Model(define.TableWorkItemTemplates, define.Postgres)
	n, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`id`, cast.ToString(model.TemplateId)},
		{`type_key`, model.TypeKey},
	}).Update(msql.Datas{
		`source_workspace_id`: cast.ToInt(model.WorkspaceId),
		`name`:                model.Name,
		`title`:               model.Title,
		`description`:         model.Description,
		`description_mode`:    mode,
		`fields_json`:         fieldsJSON,
		`updated_at`:          tool.Time2Int(),
	})
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return err
	}
	if n == 0 {
		return ErrWorkItemTemplateNotFound
	}
	return nil
}

// UpdateWorkItemTemplateSettings saves the per-type template switch.
func UpdateWorkItemTemplateSettings(model UpdateWorkItemTemplateSettingsModel) error {
	enabled := 0
	if model.Enabled {
		enabled = 1
	}
	column := `requirement_template_enabled`
	if model.TypeKey == define.TypeKeyDefect {
		column = `defect_template_enabled`
	}
	m := msql.Model(define.TableWorkspaceSettings, define.Postgres)
	_, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WorkspaceId},
	}).Update(msql.Datas{
		column:       enabled,
		`updated_at`: tool.Time2Int(),
	})
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return err
	}
	return nil
}

// LinkWorkItemTemplates associates pool templates with the workspace.
func LinkWorkItemTemplates(model LinkWorkItemTemplatesModel) error {
	if len(model.TemplateIds) == 0 {
		return nil
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	seen := make(map[int]struct{}, len(model.TemplateIds))
	now := tool.Time2Int()
	for _, templateId := range model.TemplateIds {
		if templateId <= 0 {
			return ErrWorkItemTemplateNotFound
		}
		if _, duplicate := seen[templateId]; duplicate {
			continue
		}
		ok, existsErr := poolTemplateExistsTx(tx, model.AdminId, model.TypeKey, templateId)
		if existsErr != nil {
			return existsErr
		}
		if !ok {
			return ErrWorkItemTemplateNotFound
		}
		if err = ensureTemplateCompatible(model.AdminId, model.WorkspaceId, model.TypeKey, templateId); err != nil {
			return err
		}
		if err = insertTemplateLinkTx(tx, model.AdminId, model.WorkspaceId, templateId, now); err != nil {
			return err
		}
		seen[templateId] = struct{}{}
	}
	return tx.Commit()
}

// DeleteWorkItemTemplate removes the current workspace link and deletes the template when no links remain.
func DeleteWorkItemTemplate(model DeleteWorkItemTemplateModel) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := msql.RawValues(define.Postgres, `SELECT t.id
		FROM `+define.TableWorkItemTemplates+` t
		JOIN `+define.TableWorkspaceWorkItemTemplates+` a
			ON a.admin_id = t.admin_id AND a.template_id = t.id
		WHERE t.admin_id = $1 AND a.workspace_id = $2 AND t.id = $3 AND t.type_key = $4
		FOR UPDATE OF t, a`, tx, model.AdminId, cast.ToInt(model.WorkspaceId), model.TemplateId, model.TypeKey)
	if err != nil {
		logs.Error("lock linked work item template failed, err:%s", err.Error())
		return err
	}
	if len(rows) == 0 {
		return ErrWorkItemTemplateNotFound
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM `+define.TableWorkspaceWorkItemTemplates+`
		WHERE admin_id = $1 AND workspace_id = $2 AND template_id = $3`, tx,
		model.AdminId, cast.ToInt(model.WorkspaceId), model.TemplateId); err != nil {
		logs.Error("delete workspace template link failed, err:%s", err.Error())
		return err
	}
	countRows, err := msql.RawValues(define.Postgres, `SELECT COUNT(*) AS bind_count
		FROM `+define.TableWorkspaceWorkItemTemplates+`
		WHERE admin_id = $1 AND template_id = $2`, tx, model.AdminId, model.TemplateId)
	if err != nil {
		logs.Error("count remaining work item template links failed, err:%s", err.Error())
		return err
	}
	if len(countRows) != 1 {
		return errors.New(`count remaining work item template links returned unexpected row count`)
	}
	bindCount, err := parseTemplateRowInt(countRows[0], `bind_count`)
	if err != nil {
		return err
	}
	if bindCount == 0 {
		if _, err = msql.RawExec(define.Postgres, `DELETE FROM `+define.TableWorkItemTemplates+`
			WHERE admin_id = $1 AND id = $2 AND type_key = $3`, tx,
			model.AdminId, model.TemplateId, model.TypeKey); err != nil {
			logs.Error("delete orphaned work item template failed, err:%s", err.Error())
			return err
		}
	}
	return tx.Commit()
}

// UnlinkWorkItemTemplate removes the workspace association without deleting the pool template.
func UnlinkWorkItemTemplate(model UnlinkWorkItemTemplateModel) error {
	if _, err := loadLinkedTemplate(model.AdminId, model.WorkspaceId, model.TemplateId, model.TypeKey); err != nil {
		return err
	}
	m := msql.Model(define.TableWorkspaceWorkItemTemplates, define.Postgres)
	n, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WorkspaceId},
		{`template_id`, cast.ToString(model.TemplateId)},
	}).Delete()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return err
	}
	if n == 0 {
		return ErrWorkItemTemplateNotLinked
	}
	return nil
}

// ListWorkItemTemplatePool returns tenant templates of this type that the workspace has not linked.
func ListWorkItemTemplatePool(model ListWorkItemTemplatePoolModel) (ListWorkItemTemplatePoolModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, name, source_workspace_id FROM `+define.TableWorkItemTemplates+`
		WHERE admin_id = $1 AND type_key = $2 ORDER BY id DESC`, nil, model.AdminId, model.TypeKey)
	if err != nil {
		logs.Error("list work item template pool failed, err:%s", err.Error())
		return ListWorkItemTemplatePoolModel{}, err
	}
	linkedRows, err := msql.RawValues(define.Postgres, `SELECT template_id FROM `+define.TableWorkspaceWorkItemTemplates+`
		WHERE admin_id = $1 AND workspace_id = $2`, nil, model.AdminId, cast.ToInt(model.WorkspaceId))
	if err != nil {
		logs.Error("list linked work item templates failed, err:%s", err.Error())
		return ListWorkItemTemplatePoolModel{}, err
	}
	linked := make(map[int]struct{}, len(linkedRows))
	for _, row := range linkedRows {
		templateId, convErr := parseTemplateRowInt(row, `template_id`)
		if convErr != nil {
			return ListWorkItemTemplatePoolModel{}, convErr
		}
		linked[templateId] = struct{}{}
	}
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		id, convErr := parseTemplateRowInt(row, `id`)
		if convErr != nil {
			return ListWorkItemTemplatePoolModel{}, convErr
		}
		if _, ok := linked[id]; !ok {
			ids = append(ids, id)
		}
	}
	bindCounts, err := loadTemplateBindCounts(model.AdminId, ids)
	if err != nil {
		return ListWorkItemTemplatePoolModel{}, err
	}
	items := make([]WorkItemTemplatePoolItemModel, 0, len(ids))
	for _, row := range rows {
		id, convErr := parseTemplateRowInt(row, `id`)
		if convErr != nil {
			return ListWorkItemTemplatePoolModel{}, convErr
		}
		if _, ok := linked[id]; ok {
			continue
		}
		sourceWorkspaceId, convErr := parseTemplateRowInt(row, `source_workspace_id`)
		if convErr != nil {
			return ListWorkItemTemplatePoolModel{}, convErr
		}
		items = append(items, WorkItemTemplatePoolItemModel{
			Id: id, Name: row[`name`], BindCount: bindCounts[id],
			CanDelete: bindCounts[id] == 0 && sourceWorkspaceId == cast.ToInt(model.WorkspaceId),
		})
	}
	model.Items = items
	return model, nil
}

// DeleteWorkItemTemplatePool deletes a pool template that is not linked to any workspace.
func DeleteWorkItemTemplatePool(model DeleteWorkItemTemplatePoolModel) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM `+define.TableWorkItemTemplates+`
		WHERE admin_id = $1 AND id = $2 AND type_key = $3 AND source_workspace_id = $4 FOR UPDATE`, tx,
		model.AdminId, model.TemplateId, model.TypeKey, cast.ToInt(model.WorkspaceId))
	if err != nil {
		logs.Error("lock work item template failed, err:%s", err.Error())
		return err
	}
	if len(rows) == 0 {
		return ErrWorkItemTemplateNotFound
	}
	countRows, err := msql.RawValues(define.Postgres, `SELECT COUNT(*) AS bind_count
		FROM `+define.TableWorkspaceWorkItemTemplates+`
		WHERE admin_id = $1 AND template_id = $2`, tx, model.AdminId, model.TemplateId)
	if err != nil {
		logs.Error("count work item template binds failed, err:%s", err.Error())
		return err
	}
	if len(countRows) > 0 && cast.ToInt(countRows[0][`bind_count`]) > 0 {
		return ErrWorkItemTemplateInUse
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM `+define.TableWorkItemTemplates+`
		WHERE admin_id = $1 AND id = $2 AND type_key = $3`, tx,
		model.AdminId, model.TemplateId, model.TypeKey); err != nil {
		logs.Error("delete work item template failed, err:%s", err.Error())
		return err
	}
	return tx.Commit()
}

func loadTemplateEnabled(adminId int, wsId, typeKey string) (bool, error) {
	column := `requirement_template_enabled`
	if typeKey == define.TypeKeyDefect {
		column = `defect_template_enabled`
	}
	m := msql.Model(define.TableWorkspaceSettings, define.Postgres)
	value, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, wsId},
	}).Value(column)
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return false, err
	}
	return cast.ToInt(value) == 1, nil
}

func loadLinkedTemplate(adminId int, wsId string, templateId int, typeKey string) (WorkItemTemplateModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT t.id, t.source_workspace_id, t.type_key, t.name, t.title, t.description, t.description_mode,
		t.fields_json, t.created_by, t.created_at, t.updated_at
		FROM `+define.TableWorkItemTemplates+` t
		JOIN `+define.TableWorkspaceWorkItemTemplates+` a
			ON a.admin_id = t.admin_id AND a.template_id = t.id
		WHERE t.admin_id = $1 AND a.workspace_id = $2 AND t.id = $3
			AND ($4 = '' OR t.type_key = $4)`, nil, adminId, cast.ToInt(wsId), templateId, typeKey)
	if err != nil {
		logs.Error("get work item template failed, err:%s", err.Error())
		return WorkItemTemplateModel{}, err
	}
	if len(rows) == 0 {
		return WorkItemTemplateModel{}, ErrWorkItemTemplateNotFound
	}
	return toWorkItemTemplateModel(rows[0])
}

func poolTemplateExistsTx(tx *sql.Tx, adminId int, typeKey string, templateId int) (bool, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM `+define.TableWorkItemTemplates+`
		WHERE admin_id = $1 AND id = $2 AND type_key = $3 FOR SHARE`, tx,
		adminId, templateId, typeKey)
	if err != nil {
		logs.Error("lock work item template failed, err:%s", err.Error())
		return false, err
	}
	return len(rows) > 0, nil
}

func insertTemplateLinkTx(tx *sql.Tx, adminId int, wsId string, templateId, now int) error {
	nextRows, err := msql.RawValues(define.Postgres, `SELECT COALESCE(MAX(sort_order), -1) + 1 AS next_sort
		FROM `+define.TableWorkspaceWorkItemTemplates+` WHERE admin_id = $1 AND workspace_id = $2`, tx, adminId, cast.ToInt(wsId))
	if err != nil {
		logs.Error("next template link sort failed, err:%s", err.Error())
		return err
	}
	nextSort := 0
	if len(nextRows) > 0 {
		nextSort = cast.ToInt(nextRows[0][`next_sort`])
	}
	_, err = msql.RawExec(define.Postgres, `INSERT INTO `+define.TableWorkspaceWorkItemTemplates+`
		(admin_id, workspace_id, template_id, sort_order, created_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (admin_id, workspace_id, template_id) DO NOTHING`, tx,
		adminId, cast.ToInt(wsId), templateId, nextSort, now)
	if err != nil {
		logs.Error("link work item template failed, err:%s", err.Error())
		return err
	}
	return nil
}

func loadTemplateBindCounts(adminId int, ids []int) (map[int]int, error) {
	result := make(map[int]int, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT template_id, COUNT(*) AS bind_count
		FROM `+define.TableWorkspaceWorkItemTemplates+`
		WHERE admin_id = $1 AND template_id = ANY($2::INTEGER[])
		GROUP BY template_id`, nil, adminId, pq.Array(ids))
	if err != nil {
		logs.Error("count work item template binds failed, err:%s", err.Error())
		return nil, err
	}
	for _, row := range rows {
		templateId, convErr := parseTemplateRowInt(row, `template_id`)
		if convErr != nil {
			return nil, convErr
		}
		bindCount, convErr := parseTemplateRowInt(row, `bind_count`)
		if convErr != nil {
			return nil, convErr
		}
		result[templateId] = bindCount
	}
	return result, nil
}

func toWorkItemTemplateModel(row msql.Params) (WorkItemTemplateModel, error) {
	id, err := parseTemplateRowInt(row, `id`)
	if err != nil {
		return WorkItemTemplateModel{}, err
	}
	sourceWorkspaceId, err := parseTemplateRowInt(row, `source_workspace_id`)
	if err != nil {
		return WorkItemTemplateModel{}, err
	}
	createdBy, err := parseTemplateRowInt(row, `created_by`)
	if err != nil {
		return WorkItemTemplateModel{}, err
	}
	createdAt, err := parseTemplateRowInt64(row, `created_at`)
	if err != nil {
		return WorkItemTemplateModel{}, err
	}
	updatedAt, err := parseTemplateRowInt64(row, `updated_at`)
	if err != nil {
		return WorkItemTemplateModel{}, err
	}
	fields, err := decodeTemplateFields(row[`fields_json`])
	if err != nil {
		return WorkItemTemplateModel{}, err
	}
	return WorkItemTemplateModel{
		Id:                id,
		SourceWorkspaceId: sourceWorkspaceId,
		TypeKey:           row[`type_key`],
		Name:              row[`name`],
		Title:             row[`title`],
		Description:       row[`description`],
		DescriptionMode:   row[`description_mode`],
		Fields:            fields,
		FieldCount:        len(fields),
		CreatedBy:         createdBy,
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	}, nil
}

func parseTemplateRowInt(row msql.Params, key string) (int, error) {
	value, exists := row[key]
	if !exists {
		return 0, fmt.Errorf(`work item template row missing %s`, key)
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf(`parse work item template %s: %w`, key, err)
	}
	return parsed, nil
}

func parseTemplateRowInt64(row msql.Params, key string) (int64, error) {
	value, exists := row[key]
	if !exists {
		return 0, fmt.Errorf(`work item template row missing %s`, key)
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf(`parse work item template %s: %w`, key, err)
	}
	return parsed, nil
}

func decodeTemplateFields(raw string) ([]TemplateFieldModel, error) {
	if strings.TrimSpace(raw) == `` {
		return []TemplateFieldModel{}, nil
	}
	var rows []templateFieldJSON
	if err := tool.JsonDecode(raw, &rows); err != nil {
		return nil, fmt.Errorf(`parse template fields: %w`, err)
	}
	fields := make([]TemplateFieldModel, 0, len(rows))
	for _, row := range rows {
		fields = append(fields, TemplateFieldModel{
			FieldKey:     row.FieldKey,
			FieldName:    row.FieldName,
			FieldType:    row.FieldType,
			DefaultValue: row.DefaultValue,
			DefaultNames: row.DefaultNames,
			Required:     row.Required,
			Linkages:     templateLinkagesFromJSON(row.Linkages),
			Display:      row.Display,
			LinkParam:    row.LinkParam,
		})
	}
	return fields, nil
}

func encodeTemplateFields(fields []TemplateFieldModel) (string, error) {
	rows := make([]templateFieldJSON, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, templateFieldJSON{
			FieldKey:     field.FieldKey,
			FieldName:    field.FieldName,
			FieldType:    field.FieldType,
			DefaultValue: field.DefaultValue,
			DefaultNames: field.DefaultNames,
			Required:     field.Required,
			Linkages:     templateLinkagesToJSON(field.Linkages),
			Display:      field.Display,
			LinkParam:    field.LinkParam,
		})
	}
	encoded, err := tool.JsonEncode(rows)
	if err != nil {
		return ``, err
	}
	return encoded, nil
}

func templateLinkagesFromJSON(rows []templateLinkageJSON) []TemplateLinkageModel {
	result := make([]TemplateLinkageModel, 0, len(rows))
	for _, row := range rows {
		result = append(result, TemplateLinkageModel{
			Value: row.Value, ValueNames: row.ValueNames, FieldKeys: row.FieldKeys,
		})
	}
	return result
}

func templateLinkagesToJSON(rows []TemplateLinkageModel) []templateLinkageJSON {
	result := make([]templateLinkageJSON, 0, len(rows))
	for _, row := range rows {
		result = append(result, templateLinkageJSON{
			Value: row.Value, ValueNames: row.ValueNames, FieldKeys: row.FieldKeys,
		})
	}
	return result
}

func normalizeAndValidateTemplateFields(adminId int, wsId, typeKey string, fields []TemplateFieldModel) error {
	context, err := loadTemplateFieldContext(adminId, wsId, typeKey)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(fields))
	for index := range fields {
		field := &fields[index]
		field.FieldKey = strings.TrimSpace(field.FieldKey)
		if field.FieldKey == `` {
			return ErrWorkItemTemplateFieldInvalid
		}
		if _, dup := seen[field.FieldKey]; dup {
			return ErrWorkItemTemplateFieldInvalid
		}
		seen[field.FieldKey] = struct{}{}
		if customId := templateCustomFieldID(field.FieldKey); customId > 0 {
			custom, exists := context.CustomByID[customId]
			if !exists {
				return ErrWorkItemTemplateFieldInvalid
			}
			if err := validateCustomTemplateDefault(field.DefaultValue, custom, context); err != nil {
				return err
			}
			field.FieldName = custom.Name
			field.FieldType = custom.FieldType
			field.DefaultNames = customTemplateDefaultNames(field.DefaultValue, custom)
			if custom.IsEnabled == 1 && custom.IsRequired == 1 {
				field.Required = true
			}
			continue
		}
		// Fixed invitation-form fields: title is always required and both are always
		// shown; description keeps its user-controlled required flag.
		if field.FieldKey == `title` || field.FieldKey == `description` {
			field.FieldName = ``
			field.FieldType = ``
			if field.FieldKey == `title` {
				field.Required = true
			}
			display := true
			field.Display = &display
			continue
		}
		// 属性字段里有没有这个字段，就是唯一判据；字段类型清单不在这里再抄一份
		if !context.Builtins[field.FieldKey] {
			return ErrWorkItemTemplateFieldInvalid
		}
		if templateBuiltinReadonlyKeys[field.FieldKey] {
			// 这些值由工单自己产生，模板给了也写不进去，直接丢掉
			field.DefaultValue = nil
			field.DefaultNames = nil
		} else if err := validateBuiltinTemplateDefault(field.FieldKey, field.DefaultValue, context); err != nil {
			return err
		}
		field.FieldName = ``
		field.FieldType = ``
		field.DefaultNames = builtinTemplateDefaultNames(field.FieldKey, field.DefaultValue, context)
	}
	for id, custom := range context.CustomByID {
		if custom.IsEnabled == 1 && custom.IsRequired == 1 {
			if _, exists := seen[`custom:`+strconv.Itoa(id)]; !exists {
				return ErrWorkItemTemplateFieldInvalid
			}
		}
	}
	return normalizeAndValidateTemplateLinkages(fields, context)
}

func normalizeAndValidateTemplateLinkages(fields []TemplateFieldModel, context templateFieldContext) error {
	fieldKeys := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		fieldKeys[field.FieldKey] = struct{}{}
	}
	for fieldIndex := range fields {
		field := &fields[fieldIndex]
		if err := validateTemplateDynamicDate(field); err != nil {
			return err
		}
		seenConditions := make(map[string]struct{}, len(field.Linkages))
		for linkageIndex := range field.Linkages {
			linkage := &field.Linkages[linkageIndex]
			names := templateLinkageValueNames(*field, linkage.Value, context)
			if len(names) == 0 || len(linkage.FieldKeys) == 0 {
				return ErrWorkItemTemplateFieldInvalid
			}
			if _, duplicate := seenConditions[names[0]]; duplicate {
				return ErrWorkItemTemplateFieldInvalid
			}
			seenConditions[names[0]] = struct{}{}
			seen := make(map[string]struct{}, len(linkage.FieldKeys))
			for targetIndex := range linkage.FieldKeys {
				target := strings.TrimSpace(linkage.FieldKeys[targetIndex])
				if target == `` || target == field.FieldKey {
					return ErrWorkItemTemplateFieldInvalid
				}
				if _, exists := fieldKeys[target]; !exists {
					return ErrWorkItemTemplateFieldInvalid
				}
				if _, duplicate := seen[target]; duplicate {
					return ErrWorkItemTemplateFieldInvalid
				}
				seen[target] = struct{}{}
				linkage.FieldKeys[targetIndex] = target
			}
			linkage.ValueNames = names
		}
	}
	return nil
}

func validateBuiltinTemplateDefault(fieldKey string, raw json.RawMessage, context templateFieldContext) error {
	if templateDefaultIsEmpty(raw) {
		return nil
	}
	if templateDateBuiltinKeys[fieldKey] {
		// 日期默认值有两种写法：固定时间戳，或者「相对今天」的天数
		if strings.HasPrefix(strings.TrimSpace(string(raw)), `{`) {
			field := TemplateFieldModel{FieldKey: fieldKey, DefaultValue: raw}
			return validateTemplateDynamicDate(&field)
		}
		var timestamp int64
		if json.Unmarshal(raw, &timestamp) != nil || timestamp <= 0 {
			return ErrWorkItemTemplateFieldInvalid
		}
		return nil
	}
	switch fieldKey {
	case `status`, `priority`, `version`:
		var id int
		if json.Unmarshal(raw, &id) != nil || id <= 0 {
			return ErrWorkItemTemplateFieldInvalid
		}
		valid := context.Statuses.ByID[id] != ``
		if fieldKey == `priority` {
			valid = context.Priorities.ByID[id] != ``
		} else if fieldKey == `version` {
			valid = context.Versions.ByID[id] != ``
		}
		if !valid {
			return ErrWorkItemTemplateFieldInvalid
		}
	case `category`:
		var id int
		if json.Unmarshal(raw, &id) != nil || id <= 0 || context.Categories.ByID[id] == `` {
			return ErrWorkItemTemplateFieldInvalid
		}
	case `assignee`:
		var ids []int
		if json.Unmarshal(raw, &ids) != nil {
			return ErrWorkItemTemplateFieldInvalid
		}
		for _, id := range ids {
			if id <= 0 || !context.Members[id] {
				return ErrWorkItemTemplateFieldInvalid
			}
		}
	case `estimated_work_hours`, `completed_work_hours`:
		var hours float64
		if json.Unmarshal(raw, &hours) != nil || hours < 0 {
			return ErrWorkItemTemplateFieldInvalid
		}
	}
	return nil
}

func validateCustomTemplateDefault(raw json.RawMessage, field CustomFieldModel, context templateFieldContext) error {
	if templateDefaultIsEmpty(raw) {
		return nil
	}
	switch field.FieldType {
	case `text`, `textarea`:
		var value string
		if json.Unmarshal(raw, &value) != nil {
			return ErrWorkItemTemplateFieldInvalid
		}
	case `number`, `numeric`:
		var value float64
		if json.Unmarshal(raw, &value) != nil {
			return ErrWorkItemTemplateFieldInvalid
		}
	case `date`:
		var value string
		if json.Unmarshal(raw, &value) != nil {
			return ErrWorkItemTemplateFieldInvalid
		}
		if _, err := time.Parse(`2006-01-02`, value); err != nil {
			return ErrWorkItemTemplateFieldInvalid
		}
	case `datetime`:
		var value string
		if json.Unmarshal(raw, &value) != nil {
			return ErrWorkItemTemplateFieldInvalid
		}
		if _, err := time.Parse(`2006-01-02 15:04:05`, value); err != nil {
			return ErrWorkItemTemplateFieldInvalid
		}
	case `phone`:
		var value string
		if json.Unmarshal(raw, &value) != nil || !customFieldPhonePattern.MatchString(value) {
			return ErrWorkItemTemplateFieldInvalid
		}
	case `user`:
		var id int
		if json.Unmarshal(raw, &id) != nil || id <= 0 || !context.Members[id] {
			return ErrWorkItemTemplateFieldInvalid
		}
	case `user_multi`:
		var ids []int
		if json.Unmarshal(raw, &ids) != nil {
			return ErrWorkItemTemplateFieldInvalid
		}
		for _, id := range ids {
			if id <= 0 || !context.Members[id] {
				return ErrWorkItemTemplateFieldInvalid
			}
		}
	case `cascader_single`, `cascader_multi`:
		var keys []string
		if json.Unmarshal(raw, &keys) != nil {
			return ErrWorkItemTemplateFieldInvalid
		}
		if field.FieldType == `cascader_single` {
			if !cascaderPathExists(field.Cascader, keys) {
				return ErrWorkItemTemplateFieldInvalid
			}
			break
		}
		valid := make(map[string]struct{}, len(field.Cascader))
		var collect func(nodes []CustomFieldCascaderNode)
		collect = func(nodes []CustomFieldCascaderNode) {
			for _, node := range nodes {
				valid[node.Key] = struct{}{}
				collect(node.Children)
			}
		}
		collect(field.Cascader)
		for _, key := range keys {
			if _, ok := valid[key]; !ok {
				return ErrWorkItemTemplateFieldInvalid
			}
		}
	case `select_single`:
		var key string
		if json.Unmarshal(raw, &key) != nil || templateOptionName(field.Options, key) == `` {
			return ErrWorkItemTemplateFieldInvalid
		}
	case `select_multi`:
		var keys []string
		if json.Unmarshal(raw, &keys) != nil {
			return ErrWorkItemTemplateFieldInvalid
		}
		for _, key := range keys {
			if templateOptionName(field.Options, key) == `` {
				return ErrWorkItemTemplateFieldInvalid
			}
		}
	default:
		return ErrWorkItemTemplateFieldInvalid
	}
	return nil
}

// cascaderPathExists reports whether keys form a parent→child chain from the roots.
func cascaderPathExists(nodes []CustomFieldCascaderNode, keys []string) bool {
	level := nodes
	for _, key := range keys {
		found := false
		for _, node := range level {
			if node.Key == key {
				found = true
				level = node.Children
				break
			}
		}
		if !found {
			return false
		}
	}
	return len(keys) > 0
}

func validateTemplateDynamicDate(field *TemplateFieldModel) error {
	isDate := field.FieldKey == `planned_start_date` || field.FieldKey == `planned_end_date` || field.FieldType == `date`
	raw := strings.TrimSpace(string(field.DefaultValue))
	if !isDate || raw == `` || raw == `null` || raw[0] != '{' {
		return nil
	}
	var value struct {
		DateType  string `json:"date_type"`
		OffsetDay *int   `json:"offset_days"`
	}
	if json.Unmarshal(field.DefaultValue, &value) != nil {
		return ErrWorkItemTemplateFieldInvalid
	}
	if value.DateType == `current` {
		if value.OffsetDay != nil {
			return ErrWorkItemTemplateFieldInvalid
		}
		return nil
	}
	if value.DateType != `after` && value.DateType != `before` {
		return ErrWorkItemTemplateFieldInvalid
	}
	if value.OffsetDay == nil || *value.OffsetDay < 1 || *value.OffsetDay > 36500 {
		return ErrWorkItemTemplateFieldInvalid
	}
	return nil
}

func templateDefaultIsEmpty(raw json.RawMessage) bool {
	value := strings.TrimSpace(string(raw))
	return value == `` || value == `null`
}

func templateLinkageValueNames(field TemplateFieldModel, raw json.RawMessage, context templateFieldContext) []string {
	if customId := templateCustomFieldID(field.FieldKey); customId > 0 {
		custom, exists := context.CustomByID[customId]
		if !exists || (custom.FieldType != `select_single` && custom.FieldType != `select_multi`) {
			return nil
		}
		var key string
		if json.Unmarshal(raw, &key) != nil {
			return nil
		}
		if name := templateOptionName(custom.Options, key); name != `` {
			return []string{name}
		}
		return nil
	}
	if field.FieldKey != `status` && field.FieldKey != `priority` && field.FieldKey != `version` && field.FieldKey != `category` {
		return nil
	}
	return builtinTemplateDefaultNames(field.FieldKey, raw, context)
}

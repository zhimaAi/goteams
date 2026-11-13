// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
	"goteams/internal/pkg/lib_storage"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// ErrCustomFieldValueInvalid indicates a custom field value does not conform to the field configuration.
var ErrCustomFieldValueInvalid = errors.New("invalid custom field value")

// ErrCustomFieldConfigInvalid indicates the field type or type-specific config is not acceptable.
var ErrCustomFieldConfigInvalid = errors.New("invalid custom field config")

type customFieldValidationError struct {
	cause       error
	messageKey  string
	messageArgs []any
}

func (err *customFieldValidationError) Error() string {
	return fmt.Sprintf(`%s: %s %v`, err.cause.Error(), err.messageKey, err.messageArgs)
}

func (err *customFieldValidationError) Unwrap() error {
	return err.cause
}

func newCustomFieldValidationError(cause error, messageKey string, messageArgs ...any) error {
	return &customFieldValidationError{cause: cause, messageKey: messageKey, messageArgs: messageArgs}
}

// CustomFieldValidationMessage returns a controlled localized message for a custom field validation error.
func CustomFieldValidationMessage(lang string, err error) string {
	var validationErr *customFieldValidationError
	if !errors.As(err, &validationErr) {
		return lib_i18n.Show(lang, define.MsgParamInvalid)
	}
	return lib_i18n.Show(lang, validationErr.messageKey, validationErr.messageArgs...)
}

// issueInviteFieldMessageKeys maps a custom field validation key onto the public invitation
// wording (「错误原因（字段名）」); the internal texts carry platform jargon external
// submitters should not see. An unmapped key falls back to the generic invitation message.
var issueInviteFieldMessageKeys = map[string]string{
	define.MsgCustomFieldRequired:        define.MsgIssueInviteFieldRequired,
	define.MsgCustomFieldValueInvalid:    define.MsgIssueInviteFieldValueInvalid,
	define.MsgCustomFieldOptionInvalid:   define.MsgIssueInviteFieldOptionInvalid,
	define.MsgCustomFieldPhoneInvalid:    define.MsgIssueInviteFieldPhoneInvalid,
	define.MsgCustomFieldNumberInvalid:   define.MsgIssueInviteFieldNumberInvalid,
	define.MsgCustomFieldIntegerRequired: define.MsgIssueInviteFieldIntegerRequired,
	define.MsgCustomFieldDateInvalid:     define.MsgIssueInviteFieldDateInvalid,
	define.MsgCustomFieldDateTimeInvalid: define.MsgIssueInviteFieldDateTimeInvalid,
	define.MsgCustomFieldMemberInvalid:   define.MsgIssueInviteFieldMemberInvalid,
}

// IssueInviteFieldValidationMessage renders one custom field validation error for the public
// invitation form; callers fall back to the generic message for anything unmapped.
func IssueInviteFieldValidationMessage(lang string, err error) string {
	var validationErr *customFieldValidationError
	if !errors.As(err, &validationErr) {
		return lib_i18n.Show(lang, define.MsgParamInvalid)
	}
	key, mapped := issueInviteFieldMessageKeys[validationErr.messageKey]
	if !mapped {
		return lib_i18n.Show(lang, define.MsgIssueInviteFieldInvalid)
	}
	return lib_i18n.Show(lang, key, validationErr.messageArgs...)
}

// ErrFieldOrderInvalid The submitted field sequence is incomplete or contains unknown fields.

var ErrFieldOrderInvalid = errors.New("invalid field order")

// ======================== Built-in Field Config (Attribute Field Switches) ========================

// fieldConfigQueryFields are the built-in field config query fields, to avoid SELECT *.
const fieldConfigQueryFields = `field_key,is_enabled,is_editable,is_locked,sort_order`

// customFieldQueryFields are the custom field query fields, to avoid SELECT *.
const customFieldQueryFields = `id,key,name,field_type,is_required,sort_order,is_enabled,config_json`

// FieldConfigModel is the built-in field config business data.
type FieldConfigModel struct {
	Key       string
	Label     string // Display name already translated to the request language
	Enabled   int
	Editable  int
	Locked    int
	SortOrder int
}

// ListFieldConfigsModel describes the built-in field config list use case.
type ListFieldConfigsModel struct {
	AdminId int
	WsId    string
	TypeKey string
	Lang    string
	Items   []FieldConfigModel
}

// toFieldConfigModel converts a field config data row into a business model.
func toFieldConfigModel(row msql.Params, typeKey, lang string) (FieldConfigModel, error) {
	enabled, err := strconv.Atoi(row[`is_enabled`])
	if err != nil {
		return FieldConfigModel{}, fmt.Errorf(`parse field config is_enabled: %w`, err)
	}
	editable, err := strconv.Atoi(row[`is_editable`])
	if err != nil {
		return FieldConfigModel{}, fmt.Errorf(`parse field config is_editable: %w`, err)
	}
	locked, err := strconv.Atoi(row[`is_locked`])
	if err != nil {
		return FieldConfigModel{}, fmt.Errorf(`parse field config is_locked: %w`, err)
	}
	sortOrder, err := strconv.Atoi(row[`sort_order`])
	if err != nil {
		return FieldConfigModel{}, fmt.Errorf(`parse field config sort_order: %w`, err)
	}
	return FieldConfigModel{
		Key:       row[`field_key`],
		Label:     fieldLabel(lang, typeKey, row[`field_key`]),
		Enabled:   enabled,
		Editable:  editable,
		Locked:    locked,
		SortOrder: sortOrder,
	}, nil
}

// ListFieldConfigs returns the built-in field config list of a specified type under the workspace (ordered by sort_order).
func ListFieldConfigs(model ListFieldConfigsModel) (ListFieldConfigsModel, error) {
	m := msql.Model(define.TableWorkItemFieldConfigs, define.Postgres)
	list, err := m.Field(fieldConfigQueryFields).Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WsId},
		{`type_key`, model.TypeKey},
	}).Order(`sort_order asc, id asc`).Select()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return ListFieldConfigsModel{}, err
	}
	items := make([]FieldConfigModel, 0, len(list))
	for _, row := range list {
		item, convErr := toFieldConfigModel(row, model.TypeKey, model.Lang)
		if convErr != nil {
			logs.Error("convert field config row failed, err:%s", convErr.Error())
			return ListFieldConfigsModel{}, convErr
		}
		items = append(items, item)
	}
	model.Items = items
	return model, nil
}

// fieldLabel maps a built-in field key to its translated display text.
// For planning, title is shown as "Planning Name" and related_items is a planning-specific field.
func fieldLabel(lang, typeKey, key string) string {
	i18nKey := ""
	switch key {
	case `row_index`:
		i18nKey = `workspace_field_row_index`
	case `id`:
		i18nKey = `workspace_field_id`
	case `title`:
		i18nKey = `workspace_field_title`
		if typeKey == define.TypeKeyPlanning {
			i18nKey = `workspace_field_planning_title`
		}
	case `status`:
		i18nKey = `workspace_field_status`
	case `version`:
		i18nKey = `workspace_field_iteration`
	case `category`:
		i18nKey = `workspace_field_category`
	case `priority`:
		i18nKey = `workspace_field_priority`
	case `related_items`:
		i18nKey = `workspace_field_related_items`
	case `planned_start_date`:
		i18nKey = `workspace_field_planned_start_date`
	case `planned_end_date`:
		i18nKey = `workspace_field_planned_end_date`
	case `assignee`:
		i18nKey = `workspace_field_assignee`
	case `created_at`:
		i18nKey = `workspace_field_created_time`
	case `created_by`:
		i18nKey = `workspace_field_created_by`
	case `estimated_work_hours`:
		i18nKey = `workspace_field_estimated_work_hours`
	case `work_progress`:
		i18nKey = `workspace_field_work_progress`
	case `completed_work_hours`:
		i18nKey = `workspace_field_completed_work_hours`
	case `remaining_work_hours`:
		i18nKey = `workspace_field_remaining_work_hours`
	case `overtime_work_hours`:
		i18nKey = `workspace_field_overtime_work_hours`
	default:
		return key
	}
	return lib_i18n.Show(lang, i18nKey)
}

// SaveFieldConfigItemModel is a single built-in field config save item.
type SaveFieldConfigItemModel struct {
	Key       string
	Enabled   int
	Editable  int
	SortOrder int
}

// SaveFieldConfigsModel describes the batch-save built-in field config use case.
type SaveFieldConfigsModel struct {
	AdminId int
	WsId    string
	TypeKey string
	Items   []SaveFieldConfigItemModel
}

// FieldOrderItemModel is a built-in or custom field reference within the unified field order.
type FieldOrderItemModel struct {
	Kind string
	Key  string
	Id   int
}

// SaveFieldOrderModel describes the save-field-order use case.
type SaveFieldOrderModel struct {
	AdminId int
	WsId    string
	TypeKey string
	Items   []FieldOrderItemModel
}

// SaveFieldOrder saves the full order of built-in and custom fields for a work item type.
func SaveFieldOrder(model SaveFieldOrderModel) error {
	if _, ok := presetFields[model.TypeKey]; !ok {
		return ErrFieldOrderInvalid
	}
	adminId, wsId, typeKey, items := model.AdminId, model.WsId, model.TypeKey, model.Items

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin field order transaction failed, err:%s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()

	builtinRows, err := msql.RawValues(define.Postgres, `SELECT field_key FROM workspace_field_configs
		WHERE admin_id = $1 AND workspace_id = $2 AND type_key = $3`, tx, adminId, wsId, typeKey)
	if err != nil {
		logs.Error("list built-in fields for ordering failed, err:%s", err.Error())
		return err
	}
	customRows, err := msql.RawValues(define.Postgres, `SELECT id FROM workspace_custom_fields
		WHERE admin_id = $1 AND workspace_id = $2 AND type_key = $3`, tx, adminId, wsId, typeKey)
	if err != nil {
		logs.Error("list custom fields for ordering failed, err:%s", err.Error())
		return err
	}

	builtinKeys := make(map[string]bool, len(builtinRows))
	for _, row := range builtinRows {
		builtinKeys[row[`field_key`]] = true
	}
	customIds := make(map[int]bool, len(customRows))
	for _, row := range customRows {
		customIds[cast.ToInt(row[`id`])] = true
	}
	if err = validateFieldOrderItems(items, builtinKeys, customIds); err != nil {
		return err
	}

	now := tool.Time2Int()
	for sortOrder, item := range items {
		if item.Kind == `builtin` {
			_, err = msql.RawExec(define.Postgres, `UPDATE workspace_field_configs
				SET sort_order = $1, updated_at = $2
				WHERE admin_id = $3 AND workspace_id = $4 AND type_key = $5 AND field_key = $6`,
				tx, sortOrder, now, adminId, wsId, typeKey, item.Key)
		} else {
			_, err = msql.RawExec(define.Postgres, `UPDATE workspace_custom_fields
				SET sort_order = $1, updated_at = $2
				WHERE admin_id = $3 AND workspace_id = $4 AND type_key = $5 AND id = $6`,
				tx, sortOrder, now, adminId, wsId, typeKey, item.Id)
		}
		if err != nil {
			logs.Error("update field order failed, err:%s", err.Error())
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit field order transaction failed, err:%s", err.Error())
	}
	return err
}

func validateFieldOrderItems(items []FieldOrderItemModel, builtinKeys map[string]bool, customIds map[int]bool) error {
	if len(items) != len(builtinKeys)+len(customIds) {
		return ErrFieldOrderInvalid
	}
	seenBuiltin := make(map[string]bool, len(builtinKeys))
	seenCustom := make(map[int]bool, len(customIds))
	for _, item := range items {
		switch item.Kind {
		case `builtin`:
			if !builtinKeys[item.Key] || seenBuiltin[item.Key] {
				return ErrFieldOrderInvalid
			}
			seenBuiltin[item.Key] = true
		case `custom`:
			if !customIds[item.Id] || seenCustom[item.Id] {
				return ErrFieldOrderInvalid
			}
			seenCustom[item.Id] = true
		default:
			return ErrFieldOrderInvalid
		}
	}
	return nil
}

// ResetFieldConfigsModel describes restoring built-in field default config.
type ResetFieldConfigsModel struct {
	AdminId int
	WsId    string
	TypeKey string
}

// ResetFieldConfigs restores built-in fields of a specified type to their preset enabled state and order, with custom fields moved after built-in fields.
func ResetFieldConfigs(model ResetFieldConfigsModel) error {
	presets, ok := presetFields[model.TypeKey]
	if !ok {
		return ErrFieldOrderInvalid
	}
	adminId, wsId, typeKey, now := model.AdminId, model.WsId, model.TypeKey, tool.Time2Int()
	for _, preset := range presets {
		m := msql.Model(define.TableWorkItemFieldConfigs, define.Postgres)
		_, err := m.Where2([][]string{
			{`admin_id`, cast.ToString(adminId)},
			{`workspace_id`, wsId},
			{`type_key`, typeKey},
			{`field_key`, preset.Key},
		}).Update(msql.Datas{
			`is_enabled`:  preset.Enabled,
			`is_editable`: preset.Editable,
			`is_locked`:   preset.Locked,
			`sort_order`:  preset.SortOrder,
			`updated_at`:  now,
		})
		if err != nil {
			logs.Error("reset field config sql:%s, err:%s", m.GetLastSql(), err.Error())
			return err
		}
	}
	// Custom fields keep their relative order and are placed after the built-in fields as a whole.
	customM := msql.Model(define.TableCustomFields, define.Postgres)
	customRows, err := customM.Field(`id`).Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, wsId},
		{`type_key`, typeKey},
	}).Order(`sort_order asc, id asc`).Select()
	if err != nil {
		logs.Error("list custom fields for reset sql:%s, err:%s", customM.GetLastSql(), err.Error())
		return err
	}
	for index, row := range customRows {
		m := msql.Model(define.TableCustomFields, define.Postgres)
		if _, err = m.Where2([][]string{
			{`admin_id`, cast.ToString(adminId)},
			{`workspace_id`, wsId},
			{`type_key`, typeKey},
			{`id`, row[`id`]},
		}).Update(msql.Datas{
			`sort_order`: len(presets) + index,
			`updated_at`: now,
		}); err != nil {
			logs.Error("reset custom field order sql:%s, err:%s", m.GetLastSql(), err.Error())
			return err
		}
	}
	return nil
}

// SaveFieldConfigs batch-saves built-in field configs of a specified type under the workspace (updates is_enabled/is_editable/sort_order by key).
// Locked fields (is_locked=1) cannot be disabled and are force-enabled.
func SaveFieldConfigs(model SaveFieldConfigsModel) error {
	adminId, wsId, typeKey := model.AdminId, model.WsId, model.TypeKey
	now := tool.Time2Int()
	for _, item := range model.Items {
		queryM := msql.Model(define.TableWorkItemFieldConfigs, define.Postgres)
		row, err := queryM.Field(`is_locked`).Where2([][]string{
			{`admin_id`, cast.ToString(adminId)},
			{`workspace_id`, wsId},
			{`type_key`, typeKey},
			{`field_key`, item.Key},
		}).Find()
		if err != nil {
			logs.Error("sql:%s, err:%s", queryM.GetLastSql(), err.Error())
			return err
		}
		if len(row) == 0 {
			continue // Ignore fields that are not seeded.
		}
		enabled := item.Enabled
		if cast.ToInt(row[`is_locked`]) == 1 {
			enabled = 1 // Force-enable locked fields.
		}
		editable := item.Editable
		if typeKey == define.TypeKeyRequirement {
			switch item.Key {
			case `estimated_work_hours`, `completed_work_hours`:
				editable = 1
			case `work_progress`, `remaining_work_hours`, `overtime_work_hours`:
				editable = 0
			}
		}
		updateM := msql.Model(define.TableWorkItemFieldConfigs, define.Postgres)
		_, err = updateM.Where2([][]string{
			{`admin_id`, cast.ToString(adminId)},
			{`workspace_id`, wsId},
			{`type_key`, typeKey},
			{`field_key`, item.Key},
		}).Update(msql.Datas{
			`is_enabled`:  enabled,
			`is_editable`: editable,
			`sort_order`:  item.SortOrder,
			`updated_at`:  now,
		})
		if err != nil {
			logs.Error("sql:%s, err:%s", updateM.GetLastSql(), err.Error())
			return err
		}
	}
	return nil
}

// ======================== Custom Field CRUD ========================

const customFieldCascaderMaxDepth = 4
const customFieldCascaderMaxNodes = 100
const customFieldCascaderNameMaxRunes = 20

var customFieldPhonePattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)

// CustomFieldOptionInput is one option submitted while creating or updating a field.
type CustomFieldOptionInput struct {
	Key  string
	Name string
}

// CustomFieldOption is a custom field option (select type, stored in config_json).
type CustomFieldOption struct {
	Key   string `json:"key"`   // Option key.
	Name  string `json:"name"`  // Option display name.
	Color string `json:"color"` // Color identifier (nullable).
}

// CustomFieldCascaderNode is one node in a cascader tree stored in config_json.
type CustomFieldCascaderNode struct {
	Key      string                    `json:"key"`
	Name     string                    `json:"name"`
	Children []CustomFieldCascaderNode `json:"children,omitempty"`
}

// CustomFieldCascaderInput is the cascader tree submitted on create/update.
type CustomFieldCascaderInput struct {
	Key      string
	Name     string
	Children []CustomFieldCascaderInput
}

// CustomFieldConfig is the config JSON for a custom field (different structures per field type).
// options: select_single / select_multi / radio / checkbox
// integer_only: numeric
// member_scope + role_ids: user / user_multi
// cascader: cascader_single / cascader_multi
type CustomFieldConfig struct {
	Options     []CustomFieldOption       `json:"options"`
	IntegerOnly int                       `json:"integer_only,omitempty"`
	MemberScope string                    `json:"member_scope,omitempty"`
	RoleIds     []int                     `json:"role_ids,omitempty"`
	Cascader    []CustomFieldCascaderNode `json:"cascader,omitempty"`
}

// CustomFieldModel is the custom field business data.
type CustomFieldModel struct {
	Id          int
	Key         string
	Name        string // Display name entered by the user, not an i18n key
	FieldType   string
	IsRequired  int
	SortOrder   int
	IsEnabled   int
	Options     []CustomFieldOption
	IntegerOnly int
	MemberScope string
	RoleIds     []int
	Cascader    []CustomFieldCascaderNode
}

// ListCustomFieldsModel describes the custom field list use case.
type ListCustomFieldsModel struct {
	AdminId int
	WsId    string
	TypeKey string
	Items   []CustomFieldModel
}

// ListCustomFields returns the custom field list of a specified type under the workspace (ordered by sort_order).
func ListCustomFields(model ListCustomFieldsModel) (ListCustomFieldsModel, error) {
	m := msql.Model(define.TableCustomFields, define.Postgres)
	list, err := m.Field(customFieldQueryFields).Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WsId},
		{`type_key`, model.TypeKey},
	}).Order(`sort_order asc, id asc`).Select()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return ListCustomFieldsModel{}, err
	}
	items := make([]CustomFieldModel, 0, len(list))
	for _, row := range list {
		item, convErr := toCustomFieldModel(row)
		if convErr != nil {
			logs.Error("convert custom field row failed, err:%s", convErr.Error())
			return ListCustomFieldsModel{}, convErr
		}
		items = append(items, item)
	}
	model.Items = items
	return model, nil
}

// toCustomFieldModel converts a workspace_custom_fields data row into a business model and parses options from config_json.
func toCustomFieldModel(row msql.Params) (CustomFieldModel, error) {
	id, err := strconv.Atoi(row[`id`])
	if err != nil {
		return CustomFieldModel{}, fmt.Errorf(`parse custom field id: %w`, err)
	}
	isRequired, err := strconv.Atoi(row[`is_required`])
	if err != nil {
		return CustomFieldModel{}, fmt.Errorf(`parse custom field is_required: %w`, err)
	}
	sortOrder, err := strconv.Atoi(row[`sort_order`])
	if err != nil {
		return CustomFieldModel{}, fmt.Errorf(`parse custom field sort_order: %w`, err)
	}
	isEnabled, err := strconv.Atoi(row[`is_enabled`])
	if err != nil {
		return CustomFieldModel{}, fmt.Errorf(`parse custom field is_enabled: %w`, err)
	}
	cfg := CustomFieldConfig{}
	if s := row[`config_json`]; s != `` {
		if decodeErr := tool.JsonDecode(s, &cfg); decodeErr != nil {
			return CustomFieldModel{}, fmt.Errorf(`parse custom field config_json: %w`, decodeErr)
		}
	}
	return CustomFieldModel{
		Id:          id,
		Key:         row[`key`],
		Name:        row[`name`],
		FieldType:   row[`field_type`],
		IsRequired:  isRequired,
		SortOrder:   sortOrder,
		IsEnabled:   isEnabled,
		Options:     cfg.Options,
		IntegerOnly: cfg.IntegerOnly,
		MemberScope: cfg.MemberScope,
		RoleIds:     cfg.RoleIds,
		Cascader:    cfg.Cascader,
	}, nil
}

// CreateCustomFieldModel describes the create-custom-field use case.
type CreateCustomFieldModel struct {
	AdminId     int
	WsId        string
	TypeKey     string
	Name        string
	FieldType   string
	IsRequired  int
	Options     []CustomFieldOptionInput
	IntegerOnly int
	MemberScope string
	RoleIds     []int
	Cascader    []CustomFieldCascaderInput
	Id          int
}

// CreateCustomField creates a custom field (options written to config_json).
func CreateCustomField(model CreateCustomFieldModel) (CreateCustomFieldModel, error) {
	adminId, wsId, typeKey := model.AdminId, model.WsId, model.TypeKey
	// 云版套餐配额限制，必须先于写入校验。
	if err := CheckQuota(adminId, define.QuotaCustomField, wsId); err != nil {
		return CreateCustomFieldModel{}, err
	}
	cfg, err := buildCustomFieldConfig(model.AdminId, model.Name, model.FieldType, model.Options, model.IntegerOnly, model.MemberScope, model.RoleIds, model.Cascader, nil)
	if err != nil {
		return CreateCustomFieldModel{}, err
	}
	key := `cf_` + tool.MD5(cast.ToString(adminId)+wsId+typeKey+model.Name+cast.ToString(tool.Time2Int()))
	now := tool.Time2Int()
	sortRows, err := msql.RawValues(define.Postgres, `SELECT GREATEST(
		COALESCE((SELECT MAX(sort_order) FROM workspace_field_configs
			WHERE admin_id = $1 AND workspace_id = $2 AND type_key = $3), -1),
		COALESCE((SELECT MAX(sort_order) FROM workspace_custom_fields
			WHERE admin_id = $1 AND workspace_id = $2 AND type_key = $3), -1)
	) + 1 AS next_sort_order`, nil, adminId, wsId, typeKey)
	if err != nil {
		logs.Error("get next custom field order failed, err:%s", err.Error())
		return CreateCustomFieldModel{}, err
	}
	nextSortOrder := 0
	if len(sortRows) > 0 {
		nextSortOrder = cast.ToInt(sortRows[0][`next_sort_order`])
	}
	configJson, err := encodeCustomFieldConfig(cfg)
	if err != nil {
		return CreateCustomFieldModel{}, err
	}
	m := msql.Model(define.TableCustomFields, define.Postgres)
	id, err := m.Insert(msql.Datas{
		`admin_id`:     adminId,
		`workspace_id`: wsId,
		`type_key`:     typeKey,
		`key`:          key,
		`name`:         model.Name,
		`field_type`:   model.FieldType,
		`is_required`:  model.IsRequired,
		`sort_order`:   nextSortOrder,
		`is_enabled`:   1,
		`config_json`:  configJson,
		`created_at`:   now,
		`updated_at`:   now,
	}, `id`)
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return CreateCustomFieldModel{}, err
	}
	model.Id = cast.ToInt(id)
	// 详情页缓存的是渲染好的字段快照，字段增删改后要让下一请求重算
	if err := RotateIssueInviteRenderVersion(adminId, cast.ToInt(wsId)); err != nil {
		logs.Error("rotate issue invite render version after custom field create failed, err:%s", err.Error())
	}
	return model, nil
}

// UpdateCustomFieldModel describes the edit-custom-field use case.
type UpdateCustomFieldModel struct {
	AdminId     int
	WsId        string
	TypeKey     string
	FieldId     int
	Name        string
	FieldType   string
	IsRequired  int
	IsEnabled   int
	Options     []CustomFieldOptionInput
	IntegerOnly int
	MemberScope string
	RoleIds     []int
	Cascader    []CustomFieldCascaderInput
}

// UpdateCustomField updates a custom field (options written to config_json).
func UpdateCustomField(model UpdateCustomFieldModel) error {
	adminId, wsId, typeKey, fieldId := model.AdminId, model.WsId, model.TypeKey, model.FieldId
	// Verify ownership.
	queryM := msql.Model(define.TableCustomFields, define.Postgres)
	row, err := queryM.Field(customFieldQueryFields).Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, wsId},
		{`type_key`, typeKey},
		{`id`, cast.ToString(fieldId)},
	}).Find()
	if err != nil {
		logs.Error("sql:%s, err:%s", queryM.GetLastSql(), err.Error())
		return err
	}
	if len(row) == 0 {
		return nil
	}
	now := tool.Time2Int()
	// Submitted option keys preserve identity across renames and reorders.
	previous, err := toCustomFieldModel(row)
	if err != nil {
		logs.Error("parse custom field row failed, err:%s", err.Error())
		return err
	}
	cfg, err := buildCustomFieldConfig(adminId, model.Name, model.FieldType, model.Options, model.IntegerOnly, model.MemberScope, model.RoleIds, model.Cascader, &previous)
	if err != nil {
		return err
	}
	configJson, err := encodeCustomFieldConfig(cfg)
	if err != nil {
		return err
	}
	updateM := msql.Model(define.TableCustomFields, define.Postgres)
	_, err = updateM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, wsId},
		{`type_key`, typeKey},
		{`id`, cast.ToString(fieldId)},
	}).Update(msql.Datas{
		`name`:        model.Name,
		`field_type`:  model.FieldType,
		`is_required`: model.IsRequired,
		`is_enabled`:  model.IsEnabled,
		`config_json`: configJson,
		`updated_at`:  now,
	})
	if err != nil {
		logs.Error("sql:%s, err:%s", updateM.GetLastSql(), err.Error())
		return err
	}
	if err := RotateIssueInviteRenderVersion(adminId, cast.ToInt(wsId)); err != nil {
		logs.Error("rotate issue invite render version after custom field update failed, err:%s", err.Error())
	}
	return nil
}

// DeleteCustomFieldModel describes the delete-custom-field use case.
type DeleteCustomFieldModel struct {
	AdminId int
	WsId    string
	TypeKey string
	FieldId int
}

// DeleteCustomField deletes a custom field and its filled values.
func DeleteCustomField(model DeleteCustomFieldModel) error {
	adminId, wsId, typeKey, fieldId := model.AdminId, model.WsId, model.TypeKey, model.FieldId
	delM := msql.Model(define.TableCustomFields, define.Postgres)
	_, err := delM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, wsId},
		{`type_key`, typeKey},
		{`id`, cast.ToString(fieldId)},
	}).Delete()
	if err != nil {
		logs.Error("sql:%s, err:%s", delM.GetLastSql(), err.Error())
		return err
	}
	// The value table has no workspace_id; field ownership has already been validated by workspace above.
	valM := msql.Model(define.TableCustomFieldValues, define.Postgres)
	if _, err := valM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`field_id`, cast.ToString(fieldId)},
	}).Delete(); err != nil {
		logs.Error("sql:%s, err:%s", valM.GetLastSql(), err.Error())
		return err
	}
	if err := RemoveDisplaySettingField(adminId, wsId, typeKey, "custom:"+cast.ToString(fieldId)); err != nil {
		logs.Error("clear deleted custom field display setting failed, err:%s", err.Error())
		return err
	}
	if err := RotateIssueInviteRenderVersion(adminId, cast.ToInt(wsId)); err != nil {
		logs.Error("rotate issue invite render version after custom field delete failed, err:%s", err.Error())
	}
	return nil
}

func customFieldHasOptions(fieldType string) bool {
	switch fieldType {
	case define.CustomFieldTypeSelectSingle, define.CustomFieldTypeSelectMulti,
		define.CustomFieldTypeRadio, define.CustomFieldTypeCheckbox:
		return true
	}
	return false
}

func customFieldIsMember(fieldType string) bool {
	return fieldType == define.CustomFieldTypeUser || fieldType == define.CustomFieldTypeUserMulti
}

func customFieldIsCascader(fieldType string) bool {
	return fieldType == define.CustomFieldTypeCascaderSingle || fieldType == define.CustomFieldTypeCascaderMulti
}

func encodeCustomFieldConfig(cfg CustomFieldConfig) (string, error) {
	encoded, err := tool.JsonEncode(cfg)
	if err != nil {
		return ``, err
	}
	return encoded, nil
}

func buildCustomFieldConfig(adminId int, fieldName, fieldType string, options []CustomFieldOptionInput, integerOnly int, memberScope string, roleIds []int, cascader []CustomFieldCascaderInput, previous *CustomFieldModel) (CustomFieldConfig, error) {
	allowed := define.CustomFieldTypes
	if previous != nil && previous.FieldType == define.CustomFieldTypeTextarea {
		allowed = append([]string{define.CustomFieldTypeTextarea}, define.CustomFieldTypes...)
	}
	ok := false
	for _, item := range allowed {
		if item == fieldType {
			ok = true
			break
		}
	}
	if !ok {
		return CustomFieldConfig{}, newCustomFieldValidationError(ErrCustomFieldConfigInvalid, define.MsgCustomFieldTypeInvalid, fieldName)
	}
	cfg := CustomFieldConfig{}
	switch {
	case customFieldHasOptions(fieldType):
		var previousOptions []CustomFieldOption
		if previous != nil {
			previousOptions = previous.Options
		}
		built, err := buildCustomFieldOptions(fieldName, options, previousOptions)
		if err != nil {
			return CustomFieldConfig{}, err
		}
		cfg.Options = built
	case fieldType == define.CustomFieldTypeNumeric:
		if integerOnly != 0 {
			cfg.IntegerOnly = 1
		}
	case customFieldIsMember(fieldType):
		scope := memberScope
		if scope == `` {
			scope = define.CustomFieldMemberScopeAll
		}
		switch scope {
		case define.CustomFieldMemberScopeAll, define.CustomFieldMemberScopeWorkspace:
			cfg.MemberScope = scope
		case define.CustomFieldMemberScopeRoles:
			if len(roleIds) == 0 {
				return CustomFieldConfig{}, newCustomFieldValidationError(ErrCustomFieldConfigInvalid, define.MsgCustomFieldMemberRolesRequired, fieldName)
			}
			seen := make(map[int]bool, len(roleIds))
			ids := make([]int, 0, len(roleIds))
			for _, roleId := range roleIds {
				if roleId <= 0 || seen[roleId] {
					continue
				}
				if !enterpriseRoleExists(adminId, roleId) {
					return CustomFieldConfig{}, newCustomFieldValidationError(ErrCustomFieldConfigInvalid, define.MsgCustomFieldMemberRoleInvalid, fieldName)
				}
				seen[roleId] = true
				ids = append(ids, roleId)
			}
			if len(ids) == 0 {
				return CustomFieldConfig{}, newCustomFieldValidationError(ErrCustomFieldConfigInvalid, define.MsgCustomFieldMemberRolesRequired, fieldName)
			}
			cfg.MemberScope = scope
			cfg.RoleIds = ids
		default:
			return CustomFieldConfig{}, newCustomFieldValidationError(ErrCustomFieldConfigInvalid, define.MsgCustomFieldMemberScopeInvalid, fieldName)
		}
	case customFieldIsCascader(fieldType):
		var previousNodes []CustomFieldCascaderNode
		if previous != nil {
			previousNodes = previous.Cascader
		}
		nodes, count, err := buildCascaderNodes(fieldName, cascader, previousNodes, 1)
		if err != nil {
			return CustomFieldConfig{}, err
		}
		if count == 0 {
			return CustomFieldConfig{}, newCustomFieldValidationError(ErrCustomFieldConfigInvalid, define.MsgCustomFieldCascaderRequired, fieldName)
		}
		cfg.Cascader = nodes
	}
	return cfg, nil
}

func buildCustomFieldOptions(fieldName string, options []CustomFieldOptionInput, previous []CustomFieldOption) ([]CustomFieldOption, error) {
	previousByKey := make(map[string]CustomFieldOption, len(previous))
	previousByName := make(map[string]CustomFieldOption, len(previous))
	for _, option := range previous {
		previousByKey[option.Key] = option
		previousByName[option.Name] = option
	}

	seenNames := make(map[string]bool, len(options))
	usedKeys := make(map[string]bool, len(previous))
	built := make([]CustomFieldOption, 0, len(options))
	for i, input := range options {
		name := strings.TrimSpace(input.Name)
		if name == `` || seenNames[name] {
			continue
		}
		seenNames[name] = true

		key := strings.TrimSpace(input.Key)
		if key != `` {
			old, ok := previousByKey[key]
			if !ok || usedKeys[key] {
				return nil, newCustomFieldValidationError(ErrCustomFieldConfigInvalid, define.MsgCustomFieldOptionKeyInvalid, fieldName)
			}
			old.Name = name
			usedKeys[key] = true
			built = append(built, old)
			continue
		}
		if old, ok := previousByName[name]; ok && !usedKeys[old.Key] {
			usedKeys[old.Key] = true
			built = append(built, old)
			continue
		}
		built = append(built, CustomFieldOption{
			Key:  `opt_` + tool.MD5(name + cast.ToString(tool.Time2Int()) + cast.ToString(i))[:12],
			Name: name,
		})
	}
	if len(built) == 0 {
		return nil, newCustomFieldValidationError(ErrCustomFieldConfigInvalid, define.MsgCustomFieldOptionsRequired, fieldName)
	}
	return built, nil
}

func buildCascaderNodes(fieldName string, inputs []CustomFieldCascaderInput, previous []CustomFieldCascaderNode, depth int) ([]CustomFieldCascaderNode, int, error) {
	if len(inputs) == 0 {
		return nil, 0, nil
	}
	if depth > customFieldCascaderMaxDepth {
		return nil, 0, newCustomFieldValidationError(ErrCustomFieldConfigInvalid, define.MsgCustomFieldCascaderDepthInvalid, fieldName, customFieldCascaderMaxDepth)
	}
	previousByName := make(map[string]CustomFieldCascaderNode, len(previous))
	previousByKey := make(map[string]CustomFieldCascaderNode, len(previous))
	for _, node := range previous {
		previousByName[node.Name] = node
		previousByKey[node.Key] = node
	}
	nodes := make([]CustomFieldCascaderNode, 0, len(inputs))
	count := 0
	for _, input := range inputs {
		name := strings.TrimSpace(input.Name)
		if name == `` {
			return nil, 0, newCustomFieldValidationError(ErrCustomFieldConfigInvalid, define.MsgCustomFieldCascaderNameRequired, fieldName)
		}
		if utf8.RuneCountInString(name) > customFieldCascaderNameMaxRunes {
			return nil, 0, newCustomFieldValidationError(ErrCustomFieldConfigInvalid, define.MsgCustomFieldCascaderNameTooLong, fieldName, customFieldCascaderNameMaxRunes)
		}
		node := CustomFieldCascaderNode{Name: name}
		var previousChildren []CustomFieldCascaderNode
		old, reused := previousByKey[input.Key]
		if !reused {
			old, reused = previousByName[name]
		}
		if reused {
			node.Key = old.Key
			previousChildren = old.Children
		} else {
			node.Key = `node_` + tool.Random(12)
		}
		children, childCount, err := buildCascaderNodes(fieldName, input.Children, previousChildren, depth+1)
		if err != nil {
			return nil, 0, err
		}
		node.Children = children
		nodes = append(nodes, node)
		count += 1 + childCount
		if count > customFieldCascaderMaxNodes {
			return nil, 0, newCustomFieldValidationError(ErrCustomFieldConfigInvalid, define.MsgCustomFieldCascaderSizeInvalid, fieldName, customFieldCascaderMaxNodes)
		}
	}
	return nodes, count, nil
}

// SaveCustomFieldValues validates and saves the requirement's custom field values.
func SaveCustomFieldValues(adminId int, wsId, typeKey string, requirementId int, values map[string]any, requireAll bool) error {
	return saveCustomFieldValues(nil, adminId, wsId, typeKey, requirementId, values, requireAll)
}

// saveCustomFieldValues persists custom field values on the supplied transaction when present.
func saveCustomFieldValues(tx *sql.Tx, adminId int, wsId, typeKey string, requirementId int, values map[string]any, requireAll bool) error {
	fieldsModel, err := ListCustomFields(ListCustomFieldsModel{AdminId: adminId, WsId: wsId, TypeKey: typeKey})
	if err != nil {
		return err
	}
	memberScope, err := loadCustomFieldMemberScopeForFields(adminId, wsId, fieldsModel.Items)
	if err != nil {
		return err
	}
	if err := validateCustomFieldValuesWithScope(fieldsModel.Items, values, requireAll, memberScope); err != nil {
		return err
	}
	return writeCustomFieldValues(tx, adminId, requirementId, fieldsModel.Items, values)
}

// writeCustomFieldValues 按已校验的字段集合写入，创建模板可在不修改属性开关的情况下启用字段。
func writeCustomFieldValues(tx *sql.Tx, adminId, requirementId int, fields []CustomFieldModel, values map[string]any) error {
	now := tool.Time2Int()
	for _, field := range fields {
		if field.IsEnabled != 1 {
			continue
		}
		value, exists := values[cast.ToString(field.Id)]
		text, err := customFieldValueText(value)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}

		rows, err := msql.RawValues(define.Postgres, `SELECT id FROM `+define.TableCustomFieldValues+`
			WHERE admin_id = $1 AND requirement_id = $2 AND field_id = $3 LIMIT 1`, tx,
			adminId, requirementId, field.Id)
		if err != nil {
			return err
		}
		id := 0
		if len(rows) > 0 {
			id = cast.ToInt(rows[0][`id`])
		}
		if len(text) == 0 || text == `[]` {
			if id > 0 {
				if _, err = msql.RawExec(define.Postgres, `DELETE FROM `+define.TableCustomFieldValues+`
					WHERE id = $1 AND admin_id = $2`, tx, id, adminId); err != nil {
					return err
				}
			}
			continue
		}
		if id > 0 {
			if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableCustomFieldValues+`
				SET value_text = $1, updated_at = $2 WHERE id = $3 AND admin_id = $4`, tx,
				text, now, id, adminId); err != nil {
				return err
			}
			continue
		}
		if _, err = msql.RawExec(define.Postgres, `INSERT INTO `+define.TableCustomFieldValues+`
			(admin_id, requirement_id, field_id, value_text, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$5)`, tx, adminId, requirementId, field.Id, text, now); err != nil {
			return err
		}
	}
	return nil
}

type customFieldMemberScope struct {
	enterpriseRoleIds  map[int]int
	workspaceMemberIds map[int]bool
}

func loadCustomFieldMemberScopeForFields(adminId int, wsId string, fields []CustomFieldModel) (*customFieldMemberScope, error) {
	hasMemberField := false
	for _, field := range fields {
		if field.IsEnabled == 1 && customFieldIsMember(field.FieldType) {
			hasMemberField = true
			break
		}
	}
	if !hasMemberField {
		return nil, nil
	}

	userM := msql.Model(define.TableUsers, define.Postgres)
	users, err := userM.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`deleted_at`, `0`},
	}).Field(`id, role_id`).Select()
	if err != nil {
		logs.Error("list custom field tenant members failed: sql=%s, err=%s", userM.GetLastSql(), err.Error())
		return nil, err
	}
	scope := &customFieldMemberScope{
		enterpriseRoleIds:  make(map[int]int, len(users)),
		workspaceMemberIds: map[int]bool{},
	}
	for _, user := range users {
		scope.enterpriseRoleIds[cast.ToInt(user[`id`])] = cast.ToInt(user[`role_id`])
	}

	workspaceMembers, err := msql.RawValues(define.Postgres, `SELECT wm.user_id FROM `+define.TableWorkspaceMembers+` wm
		JOIN `+define.TableUsers+` u ON u.id = wm.user_id AND u.admin_id = wm.admin_id AND u.deleted_at = 0
		WHERE wm.admin_id = $1 AND wm.workspace_id = $2`, nil, adminId, cast.ToInt(wsId))
	if err != nil {
		logs.Error("list custom field workspace members failed, err:%s", err.Error())
		return nil, err
	}
	scope.workspaceMemberIds = make(map[int]bool, len(workspaceMembers))
	for _, member := range workspaceMembers {
		scope.workspaceMemberIds[cast.ToInt(member[`user_id`])] = true
	}
	return scope, nil
}

func (scope *customFieldMemberScope) allows(field CustomFieldModel, userId int) bool {
	roleId, active := scope.enterpriseRoleIds[userId]
	if !active {
		return false
	}
	switch field.MemberScope {
	case ``, define.CustomFieldMemberScopeAll:
		return true
	case define.CustomFieldMemberScopeWorkspace:
		return scope.workspaceMemberIds[userId]
	case define.CustomFieldMemberScopeRoles:
		for _, allowedRoleId := range field.RoleIds {
			if roleId == allowedRoleId {
				return true
			}
		}
	}
	return false
}

// ValidateCustomFieldValues validates the required items and option values of a requirement's custom fields.
func ValidateCustomFieldValues(adminId int, wsId, typeKey string, values map[string]any, requireAll bool) error {
	fieldsModel, err := ListCustomFields(ListCustomFieldsModel{AdminId: adminId, WsId: wsId, TypeKey: typeKey})
	if err != nil {
		return err
	}
	memberScope, err := loadCustomFieldMemberScopeForFields(adminId, wsId, fieldsModel.Items)
	if err != nil {
		return err
	}
	return validateCustomFieldValuesWithScope(fieldsModel.Items, values, requireAll, memberScope)
}

// ValidateCustomFieldValuesWith runs structural validation against an already-loaded field definition list.
func ValidateCustomFieldValuesWith(fields []CustomFieldModel, values map[string]any, requireAll bool) error {
	return validateCustomFieldValuesWithScope(fields, values, requireAll, nil)
}

func validateCustomFieldValuesWithScope(fields []CustomFieldModel, values map[string]any, requireAll bool, memberScope *customFieldMemberScope) error {
	for _, field := range fields {
		if field.IsEnabled != 1 {
			continue
		}
		value, exists := values[cast.ToString(field.Id)]
		text, err := customFieldValueText(value)
		if err != nil {
			return err
		}
		if field.IsRequired == 1 && ((requireAll && !exists) || (exists && (len(text) == 0 || text == `[]`))) {
			return newCustomFieldValidationError(ErrCustomFieldValueInvalid, define.MsgCustomFieldRequired, field.Name)
		}
		if exists {
			if err := validateCustomFieldValue(field, text, memberScope); err != nil {
				return err
			}
		}
	}
	return nil
}

// ListCustomFieldValues returns the requirement's custom field values, keyed by field ID.
func ListCustomFieldValues(adminId, requirementId int) (map[string]string, error) {
	m := msql.Model(define.TableCustomFieldValues, define.Postgres)
	list, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`requirement_id`, cast.ToString(requirementId)},
	}).Select()
	if err != nil {
		return nil, err
	}
	values := make(map[string]string, len(list))
	for _, row := range list {
		values[row[`field_id`]] = row[`value_text`]
	}
	return values, nil
}

func customFieldValueText(value any) (string, error) {
	if value == nil {
		return ``, nil
	}
	if text, ok := value.(string); ok {
		return lib_storage.StripOSSURLSignatures(text), nil
	}
	encoded, err := tool.JsonEncode(value)
	if err != nil {
		return ``, err
	}
	return lib_storage.StripOSSURLSignatures(encoded), nil
}

func validateCustomFieldValue(field CustomFieldModel, text string, memberScope *customFieldMemberScope) error {
	if len(text) == 0 || text == `[]` || text == `null` {
		return nil
	}
	switch field.FieldType {
	case define.CustomFieldTypeSelectSingle, define.CustomFieldTypeRadio:
		return validateCustomFieldOptionKeys(field, []string{text})
	case define.CustomFieldTypeSelectMulti, define.CustomFieldTypeCheckbox:
		selected, err := decodeStringSlice(text)
		if err != nil {
			return newCustomFieldValidationError(ErrCustomFieldValueInvalid, define.MsgCustomFieldValueInvalid, field.Name)
		}
		return validateCustomFieldOptionKeys(field, selected)
	case define.CustomFieldTypePhone:
		if !customFieldPhonePattern.MatchString(text) {
			return newCustomFieldValidationError(ErrCustomFieldValueInvalid, define.MsgCustomFieldPhoneInvalid, field.Name)
		}
		return nil
	case define.CustomFieldTypeNumber, define.CustomFieldTypeNumeric:
		number, err := strconv.ParseFloat(text, 64)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return newCustomFieldValidationError(ErrCustomFieldValueInvalid, define.MsgCustomFieldNumberInvalid, field.Name)
		}
		if field.FieldType == define.CustomFieldTypeNumeric && field.IntegerOnly == 1 && number != float64(int64(number)) {
			return newCustomFieldValidationError(ErrCustomFieldValueInvalid, define.MsgCustomFieldIntegerRequired, field.Name)
		}
		return nil
	case define.CustomFieldTypeDate:
		if _, err := time.Parse(`2006-01-02`, text); err != nil {
			return newCustomFieldValidationError(ErrCustomFieldValueInvalid, define.MsgCustomFieldDateInvalid, field.Name)
		}
		return nil
	case define.CustomFieldTypeDateTime:
		if _, err := time.Parse(`2006-01-02 15:04:05`, text); err != nil {
			return newCustomFieldValidationError(ErrCustomFieldValueInvalid, define.MsgCustomFieldDateTimeInvalid, field.Name)
		}
		return nil
	case define.CustomFieldTypeUser:
		return validateCustomFieldMemberIds(field, []string{text}, memberScope)
	case define.CustomFieldTypeUserMulti:
		ids, err := decodeStringSlice(text)
		if err != nil {
			return newCustomFieldValidationError(ErrCustomFieldValueInvalid, define.MsgCustomFieldValueInvalid, field.Name)
		}
		return validateCustomFieldMemberIds(field, ids, memberScope)
	case define.CustomFieldTypeCascaderSingle:
		path, err := decodeStringSlice(text)
		if err != nil {
			return newCustomFieldValidationError(ErrCustomFieldValueInvalid, define.MsgCustomFieldValueInvalid, field.Name)
		}
		if !cascaderPathAllowed(field.Cascader, path) {
			return newCustomFieldValidationError(ErrCustomFieldValueInvalid, define.MsgCustomFieldOptionInvalid, field.Name)
		}
		return nil
	case define.CustomFieldTypeCascaderMulti:
		keys, err := decodeStringSlice(text)
		if err != nil {
			return newCustomFieldValidationError(ErrCustomFieldValueInvalid, define.MsgCustomFieldValueInvalid, field.Name)
		}
		for _, key := range keys {
			if !cascaderKeyExists(field.Cascader, key) {
				return newCustomFieldValidationError(ErrCustomFieldValueInvalid, define.MsgCustomFieldOptionInvalid, field.Name)
			}
		}
		return nil
	}
	return nil
}

func validateCustomFieldMemberIds(field CustomFieldModel, ids []string, memberScope *customFieldMemberScope) error {
	for _, value := range ids {
		userId := cast.ToInt(value)
		if userId <= 0 || (memberScope != nil && !memberScope.allows(field, userId)) {
			return newCustomFieldValidationError(ErrCustomFieldValueInvalid, define.MsgCustomFieldMemberInvalid, field.Name)
		}
	}
	return nil
}

func validateCustomFieldOptionKeys(field CustomFieldModel, keys []string) error {
	allowed := make(map[string]bool, len(field.Options))
	for _, option := range field.Options {
		allowed[option.Key] = true
	}
	for _, key := range keys {
		if !allowed[key] {
			return newCustomFieldValidationError(ErrCustomFieldValueInvalid, define.MsgCustomFieldOptionInvalid, field.Name)
		}
	}
	return nil
}

func decodeStringSlice(text string) ([]string, error) {
	selected := []string{}
	if err := tool.JsonDecode(text, &selected); err == nil {
		return selected, nil
	}
	numbers := []int{}
	if err := tool.JsonDecode(text, &numbers); err != nil {
		return nil, err
	}
	selected = make([]string, 0, len(numbers))
	for _, number := range numbers {
		selected = append(selected, cast.ToString(number))
	}
	return selected, nil
}

func cascaderKeyExists(nodes []CustomFieldCascaderNode, key string) bool {
	for _, node := range nodes {
		if node.Key == key || cascaderKeyExists(node.Children, key) {
			return true
		}
	}
	return false
}

func cascaderPathAllowed(nodes []CustomFieldCascaderNode, path []string) bool {
	if len(path) == 0 {
		return false
	}
	current := nodes
	for _, key := range path {
		found := false
		for _, node := range current {
			if node.Key == key {
				current = node.Children
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

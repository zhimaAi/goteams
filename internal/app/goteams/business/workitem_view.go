// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/permission"
	"goteams/internal/pkg/lib_i18n"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// ======================== Work Item Views ========================
//
// A view is a saved filter set for the requirement/defect list.
// System views apply to every member of the workspace; personal views are only
// visible to their creator. Five builtin system views are seeded per workspace
// and per type; the builtin "all" view can never be edited or deleted.

// View type/builtin/placeholder constants live in the define package
// (shared with validate); business-local aliases keep call sites short.
const (
	ViewTypeSystem                 = define.ViewTypeSystem
	ViewTypePersonal               = define.ViewTypePersonal
	ViewBuiltinAll                 = define.ViewBuiltinAll
	ViewValueMe                    = define.ViewValueMe
	ViewValueCurrentEnterpriseRole = define.ViewValueCurrentEnterpriseRole
)

var (
	ErrViewNotFound      = errors.New(`view not found`)
	ErrViewBuiltinLocked = errors.New(`builtin all view is locked`)
)

// ValidateWorkItemFilterFields checks project-scoped dynamic fields and typed values.
func ValidateWorkItemFilterFields(adminId int, wsId, typeKey string, root ViewFilterNode) error {
	var enabledBuiltins map[string]bool
	customFields := map[int]CustomFieldModel{}
	if typeKey == define.TypeKeyIteration {
		enabledBuiltins = iterationBuiltinFilterFields()
	} else {
		configs, err := ListFieldConfigs(ListFieldConfigsModel{AdminId: adminId, WsId: wsId, TypeKey: typeKey})
		if err != nil {
			return err
		}
		enabledBuiltins = enabledBuiltinFilterFields(typeKey, configs.Items)
		customResult, err := ListCustomFields(ListCustomFieldsModel{AdminId: adminId, WsId: wsId, TypeKey: typeKey})
		if err != nil {
			return err
		}
		customFields = make(map[int]CustomFieldModel, len(customResult.Items))
		for _, field := range customResult.Items {
			if field.IsEnabled == 1 {
				customFields[field.Id] = field
			}
		}
	}
	var validateNode func(ViewFilterNode) error
	validateNode = func(node ViewFilterNode) error {
		if node.IsGroup() {
			for _, child := range node.Conditions {
				if err := validateNode(child); err != nil {
					return err
				}
			}
			return nil
		}
		if strings.HasPrefix(node.Field, define.ViewFilterCustomPrefix) {
			field, ok := customFields[cast.ToInt(strings.TrimPrefix(node.Field, define.ViewFilterCustomPrefix))]
			if !ok {
				return fmt.Errorf(`custom field is unavailable: %s`, node.Field)
			}
			if err := validateTypedFilter(field.FieldType, node); err != nil {
				return err
			}
			switch field.FieldType {
			case define.ViewFilterCustomTypeSelectSingle, define.ViewFilterCustomTypeSelectMulti,
				define.ViewFilterCustomTypeRadio, define.ViewFilterCustomTypeCheckbox:
				allowedValues := make(map[string]bool, len(field.Options))
				for _, option := range field.Options {
					allowedValues[option.Key] = true
				}
				for _, value := range node.Values {
					if strings.TrimSpace(value) != `` && !allowedValues[value] {
						return fmt.Errorf(`custom field option is invalid: %s`, value)
					}
				}
			case define.ViewFilterCustomTypeUser, define.ViewFilterCustomTypeUserMulti:
				for _, value := range node.Values {
					if value != ViewValueMe && value != ViewValueCurrentEnterpriseRole && cast.ToInt(value) <= 0 {
						return errors.New(`custom user filter value is invalid`)
					}
				}
			case define.ViewFilterCustomTypeCascaderSingle, define.ViewFilterCustomTypeCascaderMulti:
				for _, value := range node.Values {
					if field.FieldType == define.ViewFilterCustomTypeCascaderMulti && node.Operator == define.ViewFilterOperatorIn &&
						(value == define.ViewValueAll || value == define.ViewValueEmpty || value == define.ViewValueNotEmpty) {
						continue
					}
					if strings.TrimSpace(value) != `` && !cascaderKeyExists(field.Cascader, value) {
						return fmt.Errorf(`custom field option is invalid: %s`, value)
					}
				}
			}
			return nil
		}
		if node.Field == define.ViewFilterFieldAssignmentAge || node.Field == define.ViewFilterFieldMyAssignmentAge {
			if typeKey != define.TypeKeyRequirement || !enabledBuiltins[define.ViewFilterFieldAssignee] {
				return errors.New(`assignment age requires an available assignee field`)
			}
			if !slices.Contains(define.ViewFilterFieldOperators[node.Field], node.Operator) {
				return errors.New(`unsupported assignment age operator`)
			}
			_, err := viewFilterHours(node.Values)
			return err
		}
		configKey := node.Field
		if configKey == define.ViewFilterFieldCreator {
			configKey = `created_by`
		}
		if node.Field == define.ViewFilterFieldCategory {
			if typeKey != define.TypeKeyRequirement {
				return errors.New(`category is unavailable for this type`)
			}
		}
		if !enabledBuiltins[configKey] {
			return fmt.Errorf(`builtin field is unavailable: %s`, node.Field)
		}
		if node.Field == define.ViewFilterFieldID && len(node.Values) > 0 && cast.ToInt(node.Values[0]) <= 0 {
			return errors.New(`id filter requires a positive integer`)
		}
		if node.Field == define.ViewFilterFieldCreatedAt && node.Operator == define.ViewFilterOperatorOlderThanHours {
			_, err := viewFilterHours(node.Values)
			return err
		}
		if node.Field == define.ViewFilterFieldPlannedStartDate || node.Field == define.ViewFilterFieldPlannedEndDate || node.Field == define.ViewFilterFieldCreatedAt {
			return validateDateRangeValues(node)
		}
		if node.Operator == define.ViewFilterOperatorEmpty || node.Operator == define.ViewFilterOperatorNotEmpty {
			if len(node.Values) > 0 {
				return errors.New(`empty filter operators do not accept values`)
			}
			return nil
		}
		switch node.Field {
		case define.ViewFilterFieldID:
			if len(node.Values) > 1 {
				return errors.New(`id filter accepts one value`)
			}
			for _, value := range node.Values {
				if _, err := positiveFilterInteger(value); err != nil {
					return err
				}
			}
		case define.ViewFilterFieldStatus:
			for _, value := range node.Values {
				if value == define.ViewValueAllEnd || value == define.ViewValueAllNotEnd || value == define.ViewValueAllCompleted || value == define.ViewValueAllClosed {
					continue
				}
				if _, err := positiveFilterInteger(value); err != nil {
					return err
				}
			}
		case define.ViewFilterFieldVersion, define.ViewFilterFieldPriority, define.ViewFilterFieldCategory, define.ViewFilterFieldTemplate:
			for _, value := range node.Values {
				if _, err := nonNegativeFilterInteger(value); err != nil {
					return err
				}
			}
		case define.ViewFilterFieldWorkspace:
			for _, value := range node.Values {
				if _, err := positiveFilterInteger(value); err != nil {
					return err
				}
			}
		case define.ViewFilterFieldAgingStatus:
			for _, value := range node.Values {
				if _, err := nonNegativeFilterInteger(value); err != nil {
					return err
				}
			}
		case define.ViewFilterFieldWorkItemType:
			for _, value := range node.Values {
				if value != define.TypeKeyRequirement && value != define.TypeKeyDefect {
					return errors.New(`work item type filter value is invalid`)
				}
			}
		case define.ViewFilterFieldAssignee, define.ViewFilterFieldCreator:
			for _, value := range node.Values {
				if value == define.ViewValueMe || value == define.ViewValueCurrentEnterpriseRole {
					continue
				}
				if _, err := positiveFilterInteger(value); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return validateNode(root)
}

func iterationBuiltinFilterFields() map[string]bool {
	return map[string]bool{
		define.ViewFilterFieldID:           true,
		define.ViewFilterFieldTitle:        true,
		define.ViewFilterFieldWorkspace:    true,
		define.ViewFilterFieldWorkItemType: true,
		define.ViewFilterFieldStatus:       true,
		define.ViewFilterFieldPriority:     true,
		define.ViewFilterFieldAssignee:     true,
		// Filter validation looks builtin fields up by field-config key, and the
		// creator field is stored as created_by there.
		`created_by`:                           true,
		define.ViewFilterFieldPlannedStartDate: true,
		define.ViewFilterFieldPlannedEndDate:   true,
		define.ViewFilterFieldCreatedAt:        true,
	}
}

func enabledBuiltinFilterFields(typeKey string, configs []FieldConfigModel) map[string]bool {
	result := map[string]bool{define.ViewFilterFieldTitle: true}
	if typeKey == define.TypeKeyRequirement || typeKey == define.TypeKeyDefect {
		result[define.ViewFilterFieldTemplate] = true
	}
	if typeKey == define.TypeKeyRequirement {
		// Category is managed by the requirement category tree and therefore has
		// no corresponding row in workspace_field_configs.
		result[define.ViewFilterFieldCategory] = true
		// aging_status is a derived per-requirement column, filterable only for requirements.
		result[define.ViewFilterFieldAgingStatus] = true
	}
	for _, field := range configs {
		if field.Enabled == 1 {
			result[field.Key] = true
		}
	}
	return result
}

func positiveFilterInteger(value string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 0, errors.New(`filter value must be a positive integer`)
	}
	return parsed, nil
}

func nonNegativeFilterInteger(value string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 {
		return 0, errors.New(`filter value must be a non-negative integer`)
	}
	return parsed, nil
}

func validateTypedFilter(fieldType string, node ViewFilterNode) error {
	if node.Operator == define.ViewFilterOperatorEmpty || node.Operator == define.ViewFilterOperatorNotEmpty {
		if len(node.Values) > 0 {
			return errors.New(`empty filter operators do not accept values`)
		}
		return nil
	}
	if !define.ViewFilterCustomTypeOperators[fieldType][node.Operator] {
		return fmt.Errorf(`operator %s is unavailable for custom field type %s`, node.Operator, fieldType)
	}
	if fieldType == define.ViewFilterCustomTypeDate || fieldType == define.ViewFilterCustomTypeDateTime {
		return validateDateRangeValues(node)
	}
	if fieldType == define.ViewFilterCustomTypeNumber || fieldType == define.ViewFilterCustomTypeNumeric {
		for _, value := range node.Values {
			if strings.TrimSpace(value) == `` {
				continue
			}
			number, err := strconv.ParseFloat(value, 64)
			if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
				return errors.New(`number filter value is invalid`)
			}
		}
	}
	return nil
}

func validateDateRangeValues(node ViewFilterNode) error {
	if node.Operator == define.ViewFilterOperatorEmpty || node.Operator == define.ViewFilterOperatorNotEmpty {
		if len(node.Values) > 0 {
			return errors.New(`empty filter operators do not accept values`)
		}
		return nil
	}
	valueLimit := 2
	if node.Operator == define.ViewFilterOperatorEqual {
		valueLimit = 1
	}
	if len(node.Values) > valueLimit {
		return errors.New(`date filter range is invalid`)
	}
	for _, value := range node.Values {
		value = strings.TrimSpace(value)
		if value == `` {
			continue
		}
		if node.Operator == define.ViewFilterOperatorDynamic {
			parts := strings.SplitN(value, `:`, 2)
			if len(parts) != 2 {
				return errors.New(`dynamic date filter value is invalid`)
			}
			offset, err := strconv.Atoi(parts[0])
			if err != nil || offset < -999 || offset > 999 || (parts[1] != `day` && parts[1] != `week` && parts[1] != `month`) {
				return errors.New(`dynamic date filter value is invalid`)
			}
			continue
		}
		if node.Operator != define.ViewFilterOperatorEqual && node.Operator != define.ViewFilterOperatorBetween {
			return errors.New(`date filter range is invalid`)
		}
		if _, err := time.Parse(define.ViewFilterDateLayout, value); err != nil {
			return errors.New(`date filter value is invalid`)
		}
	}
	return nil
}

// ViewFilterNode is defined in the define package; alias for readability.
type ViewFilterNode = define.ViewFilterNode

// WorkItemViewModel is one view merged with the requesting user's preferences.
type WorkItemViewModel struct {
	Id         int
	TypeKey    string
	Name       string // raw stored name; builtin views hold a wrapped i18n key
	ViewType   string
	BuiltinKey string
	Filters    string
	CreatedBy  int
	Enabled    bool
	IsDefault  bool
}

// WorkItemViewListModel is the view list plus user-level settings.
type WorkItemViewListModel struct {
	Views          []WorkItemViewModel
	RememberLast   bool
	SearchExpanded bool
	SearchFilters  string
}

// UserViewSettingsPatch describes optional per-user view setting changes.
type UserViewSettingsPatch struct {
	AdminId           int
	WorkspaceId       string
	TypeKey           string
	UserId            int
	DefaultViewId     *int
	DisabledViewIds   *[]int
	PersonalViewOrder *[]int
	RememberLast      *bool
	SearchExpanded    *bool
	SearchFilters     *string
}

// presetView describes a builtin system view seeded for every workspace type.
type presetView struct {
	BuiltinKey string
	Name       string
	Filters    string
}

// presetViews are seeded in this order; display order follows builtinViewWeight.
var presetViews = []presetView{
	{ViewBuiltinAll, lib_i18n.WrapKey(define.ViewBuiltinNameAll), define.ViewBuiltinFiltersAll},
	{define.ViewBuiltinMyAssigned, lib_i18n.WrapKey(define.ViewBuiltinNameMyAssigned), define.ViewBuiltinFiltersMyAssigned},
	{define.ViewBuiltinMyCreated, lib_i18n.WrapKey(define.ViewBuiltinNameMyCreated), define.ViewBuiltinFiltersMyCreated},
	{define.ViewBuiltinUnfinished, lib_i18n.WrapKey(define.ViewBuiltinNameUnfinished), define.ViewBuiltinFiltersUnfinished},
	{define.ViewBuiltinFinished, lib_i18n.WrapKey(define.ViewBuiltinNameFinished), define.ViewBuiltinFiltersFinished},
}

// 工作项列表按预置声明顺序展示系统视图。
var projectPresetViews = []presetView{
	{`my_processing`, lib_i18n.WrapKey(`work_item_view_my_processing`), `{"logic":"and","conditions":[{"field":"assignee","operator":"in","values":["__me__"]},{"field":"status","operator":"in","values":["__all_not_end__"]}]}`},
	{`my_unresolved_48h`, lib_i18n.WrapKey(`work_item_view_my_unresolved_48h`), `{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]},{"field":"my_assignment_age","operator":"older_than_hours","values":["48"]}]}`},
	{`my_resolved`, lib_i18n.WrapKey(`work_item_view_my_resolved`), `{"logic":"and","conditions":[{"field":"assignee","operator":"in","values":["__me__"]},{"field":"status","operator":"in","values":["__all_completed__"]}]}`},
	presetViews[1],
	presetViews[2],
	{`unresolved_48h`, lib_i18n.WrapKey(`work_item_view_unresolved_48h`), `{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]},{"field":"created_at","operator":"older_than_hours","values":["48"]}]}`},
	{`closed`, lib_i18n.WrapKey(`work_item_view_closed`), `{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_closed__"]}]}`},
	{define.ViewBuiltinUnfinished, lib_i18n.WrapKey(`work_item_view_unfinished`), define.ViewBuiltinFiltersUnfinished},
	{define.ViewBuiltinFinished, lib_i18n.WrapKey(`work_item_view_finished`), `{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_completed__"]}]}`},
	presetViews[0],
}

// ganttPresetViews mirror presetViews with three-section gantt filter payloads.
var ganttPresetViews = []presetView{
	{ViewBuiltinAll, lib_i18n.WrapKey(define.ViewBuiltinNameAll), define.ViewBuiltinFiltersAll},
	{define.ViewBuiltinMyAssigned, lib_i18n.WrapKey(define.ViewBuiltinNameMyAssigned), define.GanttViewBuiltinFiltersMyAssigned},
	{define.ViewBuiltinMyCreated, lib_i18n.WrapKey(define.ViewBuiltinNameMyCreated), define.GanttViewBuiltinFiltersMyCreated},
	{define.ViewBuiltinUnfinished, lib_i18n.WrapKey(define.ViewBuiltinNameUnfinished), define.GanttViewBuiltinFiltersUnfinished},
	{define.ViewBuiltinFinished, lib_i18n.WrapKey(define.ViewBuiltinNameFinished), define.GanttViewBuiltinFiltersFinished},
}

// builtinViewWeight fixes the display order of builtin system views regardless of row ids.
func builtinViewWeight(builtinKey string) int {
	switch builtinKey {
	case ViewBuiltinAll:
		return 0
	case define.ViewBuiltinMyAssigned:
		return 1
	case define.ViewBuiltinMyCreated:
		return 2
	case define.ViewBuiltinUnfinished:
		return 3
	case define.ViewBuiltinFinished:
		return 4
	default:
		return 100
	}
}

// seedWorkspaceViews creates the builtin system views for both work item types and the gantt module (idempotent).
func seedWorkspaceViews(adminId int, wsId string, now int) error {
	for _, typeKey := range []string{define.TypeKeyRequirement, define.TypeKeyDefect, define.TypeKeyIteration, define.TypeKeyGantt} {
		views := presetViews
		if typeKey == define.TypeKeyGantt {
			views = ganttPresetViews
		} else if typeKey == define.TypeKeyRequirement {
			views = projectPresetViews
		}
		for _, view := range views {
			queryM := msql.Model(define.TableWorkspaceViews, define.Postgres)
			exist, err := queryM.Where2([][]string{
				{`admin_id`, cast.ToString(adminId)},
				{`workspace_id`, wsId},
				{`type_key`, typeKey},
				{`builtin_key`, view.BuiltinKey},
			}).Value(`id`)
			if err != nil {
				logs.Error("sql:%s, err:%s", queryM.GetLastSql(), err.Error())
				return err
			}
			if len(exist) > 0 {
				continue
			}
			insertM := msql.Model(define.TableWorkspaceViews, define.Postgres)
			if _, err = insertM.Insert(msql.Datas{
				`admin_id`:     adminId,
				`workspace_id`: wsId,
				`type_key`:     typeKey,
				`name`:         view.Name,
				`view_type`:    ViewTypeSystem,
				`builtin_key`:  view.BuiltinKey,
				`filters`:      view.Filters,
				`created_by`:   0,
				`created_at`:   now,
				`updated_at`:   now,
			}); err != nil {
				logs.Error("sql:%s, err:%s", insertM.GetLastSql(), err.Error())
				return err
			}
		}
	}
	return nil
}

// intListToText joins ids with commas for storage (same convention as final_status_ids).
func intListToText(ids []int) string {
	if len(ids) == 0 {
		return ``
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.Itoa(id))
	}
	return strings.Join(parts, `,`)
}

// textToIntList parses a comma-separated id list, dropping invalid fragments.
func textToIntList(value string) []int {
	if strings.TrimSpace(value) == `` {
		return nil
	}
	parts := strings.Split(value, `,`)
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		if id := cast.ToInt(strings.TrimSpace(part)); id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

// userViewSettingsRow reads the requesting user's settings row (may be empty).
func userViewSettingsRow(adminId int, wsId, typeKey string, userId int) (msql.Params, error) {
	m := msql.Model(define.TableUserViewSettings, define.Postgres)
	row, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, wsId},
		{`user_id`, cast.ToString(userId)},
		{`type_key`, typeKey},
	}).Find()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	return row, nil
}

// ListWorkItemViews returns system views plus the user's personal views, merged
// with the user's preferences (enabled/default/order/remember switch).
func ListWorkItemViews(adminId int, wsId, typeKey string, userId int) (WorkItemViewListModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, type_key, name, view_type, builtin_key, filters, created_by
		FROM workspace_views
		WHERE admin_id = $1 AND workspace_id = $2 AND type_key = $3 AND (view_type = $4 OR created_by = $5)
		ORDER BY id ASC`, nil, adminId, wsId, typeKey, ViewTypeSystem, userId)
	if err != nil {
		logs.Error("ListWorkItemViews query failed, err:%s", err.Error())
		return WorkItemViewListModel{}, err
	}

	settingsRow, err := userViewSettingsRow(adminId, wsId, typeKey, userId)
	if err != nil {
		return WorkItemViewListModel{}, err
	}
	disabledSet := map[int]bool{}
	for _, id := range textToIntList(settingsRow[`disabled_view_ids`]) {
		disabledSet[id] = true
	}
	personalOrder := map[int]int{}
	for index, id := range textToIntList(settingsRow[`personal_view_order`]) {
		personalOrder[id] = index
	}
	defaultViewId := cast.ToInt(settingsRow[`default_view_id`])
	rememberLast := true
	searchExpanded := true
	searchFilters := define.ViewDefaultSearchFilters
	if typeKey == define.TypeKeyIteration {
		searchFilters = define.IterationDefaultSearchFilters
	}
	if len(settingsRow) > 0 {
		rememberLast = cast.ToInt(settingsRow[`remember_last`]) == 1
		searchExpanded = cast.ToInt(settingsRow[`search_expanded`]) == 1
		if stored := strings.TrimSpace(settingsRow[`search_filters`]); stored != `` {
			searchFilters = stored
		}
	}

	projectViews := typeKey == define.TypeKeyRequirement
	viewWeight := builtinViewWeight
	if projectViews {
		viewWeight = func(key string) int {
			for index, view := range projectPresetViews {
				if view.BuiltinKey == key {
					return index
				}
			}
			return len(projectPresetViews)
		}
	}
	views := make([]WorkItemViewModel, 0, len(rows))
	defaultExists := false
	for _, row := range rows {
		id := cast.ToInt(row[`id`])
		view := WorkItemViewModel{
			Id:         id,
			TypeKey:    row[`type_key`],
			Name:       row[`name`],
			ViewType:   row[`view_type`],
			BuiltinKey: row[`builtin_key`],
			Filters:    row[`filters`],
			CreatedBy:  cast.ToInt(row[`created_by`]),
			Enabled:    !disabledSet[id] || (projectViews && row[`builtin_key`] == ViewBuiltinAll),
		}
		if id == defaultViewId {
			view.IsDefault = true
			defaultExists = true
		}
		views = append(views, view)
	}
	// Fall back to the builtin "all" view when no valid default is configured.
	if !defaultExists {
		for i := range views {
			if views[i].BuiltinKey == ViewBuiltinAll {
				views[i].IsDefault = true
				break
			}
		}
	}

	sort.SliceStable(views, func(i, j int) bool {
		a, b := views[i], views[j]
		if a.ViewType != b.ViewType {
			return a.ViewType == ViewTypeSystem
		}
		if a.ViewType == ViewTypeSystem {
			aCustom, bCustom := a.BuiltinKey == ``, b.BuiltinKey == ``
			if aCustom != bCustom {
				return aCustom
			}
			if aCustom {
				return a.Id > b.Id
			}
			wa, wb := viewWeight(a.BuiltinKey), viewWeight(b.BuiltinKey)
			if wa != wb {
				return wa < wb
			}
			return a.Id < b.Id
		}
		oa, oka := personalOrder[a.Id]
		ob, okb := personalOrder[b.Id]
		if oka && okb {
			return oa < ob
		}
		if oka != okb {
			return !oka
		}
		return a.Id > b.Id
	})

	return WorkItemViewListModel{
		Views:          views,
		RememberLast:   rememberLast,
		SearchExpanded: searchExpanded,
		SearchFilters:  searchFilters,
	}, nil
}

// GetWorkItemViewById returns one view within the tenant/workspace; Id is 0 when missing.
func GetWorkItemViewById(adminId int, wsId string, viewId int) (WorkItemViewModel, error) {
	m := msql.Model(define.TableWorkspaceViews, define.Postgres)
	row, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, wsId},
		{`id`, cast.ToString(viewId)},
	}).Find()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return WorkItemViewModel{}, err
	}
	if len(row) == 0 {
		return WorkItemViewModel{}, nil
	}
	return WorkItemViewModel{
		Id:         cast.ToInt(row[`id`]),
		TypeKey:    row[`type_key`],
		Name:       row[`name`],
		ViewType:   row[`view_type`],
		BuiltinKey: row[`builtin_key`],
		Filters:    row[`filters`],
		CreatedBy:  cast.ToInt(row[`created_by`]),
	}, nil
}

// CreateWorkItemView inserts a user-defined view and returns its id.
func CreateWorkItemView(adminId int, wsId, typeKey, name, viewType, filters string, userId int) (int, error) {
	now := tool.Time2Int()
	m := msql.Model(define.TableWorkspaceViews, define.Postgres)
	id, err := m.Insert(msql.Datas{
		`admin_id`:     adminId,
		`workspace_id`: wsId,
		`type_key`:     typeKey,
		`name`:         name,
		`view_type`:    viewType,
		`builtin_key`:  ``,
		`filters`:      filters,
		`created_by`:   userId,
		`created_at`:   now,
		`updated_at`:   now,
	}, `id`)
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return 0, err
	}
	return cast.ToInt(id), nil
}

// UpdateWorkItemView updates name and filters; the builtin "all" view is locked.
// Permission checks (system view management / personal ownership) live in the controller.
func UpdateWorkItemView(adminId int, wsId string, view WorkItemViewModel, name, filters string) error {
	if view.BuiltinKey == ViewBuiltinAll {
		return ErrViewBuiltinLocked
	}
	m := msql.Model(define.TableWorkspaceViews, define.Postgres)
	_, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, wsId},
		{`id`, cast.ToString(view.Id)},
	}).Update(msql.Datas{
		`name`:       name,
		`filters`:    filters,
		`updated_at`: tool.Time2Int(),
	})
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return err
	}
	return nil
}

// DeleteWorkItemView removes a view; the builtin "all" view is locked.
func DeleteWorkItemView(adminId int, wsId string, view WorkItemViewModel) error {
	if view.BuiltinKey == ViewBuiltinAll {
		return ErrViewBuiltinLocked
	}
	m := msql.Model(define.TableWorkspaceViews, define.Postgres)
	_, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, wsId},
		{`id`, cast.ToString(view.Id)},
	}).Delete()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return err
	}
	return nil
}

// SaveUserViewSettings applies a partial update to one user's work item view preferences.
func SaveUserViewSettings(model UserViewSettingsPatch) error {
	if model.TypeKey == define.TypeKeyRequirement && model.DisabledViewIds != nil && len(*model.DisabledViewIds) > 0 {
		rows, err := msql.RawValues(define.Postgres, `SELECT id FROM workspace_views
			WHERE admin_id = $1 AND workspace_id = $2 AND type_key = $3
			  AND builtin_key = $4 AND id = ANY($5)`,
			nil, model.AdminId, model.WorkspaceId, model.TypeKey, ViewBuiltinAll, pq.Array(*model.DisabledViewIds))
		if err != nil {
			logs.Error("load locked view for user settings failed, err:%s", err.Error())
			return err
		}
		if len(rows) > 0 {
			lockedId := cast.ToInt(rows[0][`id`])
			disabledIds := make([]int, 0, len(*model.DisabledViewIds))
			for _, id := range *model.DisabledViewIds {
				if id != lockedId {
					disabledIds = append(disabledIds, id)
				}
			}
			model.DisabledViewIds = &disabledIds
		}
	}
	now := tool.Time2Int()
	data := userViewSettingsPatchData(model, now)
	if len(data) == 1 {
		return nil
	}
	row, err := userViewSettingsRow(model.AdminId, model.WorkspaceId, model.TypeKey, model.UserId)
	if err != nil {
		return err
	}
	if len(row) > 0 {
		m := msql.Model(define.TableUserViewSettings, define.Postgres)
		if _, err = m.Where2([][]string{{`id`, row[`id`]}, {`admin_id`, cast.ToString(model.AdminId)}}).Update(data); err != nil {
			logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
			return err
		}
		return nil
	}
	data[`admin_id`] = model.AdminId
	data[`workspace_id`] = model.WorkspaceId
	data[`user_id`] = model.UserId
	data[`type_key`] = model.TypeKey
	data[`created_at`] = now
	m := msql.Model(define.TableUserViewSettings, define.Postgres)
	if _, err = m.Insert(data); err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return err
	}
	return nil
}

func userViewSettingsPatchData(model UserViewSettingsPatch, now int) msql.Datas {
	data := msql.Datas{`updated_at`: now}
	if model.DefaultViewId != nil {
		data[`default_view_id`] = *model.DefaultViewId
	}
	if model.DisabledViewIds != nil {
		data[`disabled_view_ids`] = intListToText(*model.DisabledViewIds)
	}
	if model.PersonalViewOrder != nil {
		data[`personal_view_order`] = intListToText(*model.PersonalViewOrder)
	}
	if model.RememberLast != nil {
		data[`remember_last`] = boolToSmallInt(*model.RememberLast)
	}
	if model.SearchExpanded != nil {
		data[`search_expanded`] = boolToSmallInt(*model.SearchExpanded)
	}
	if model.SearchFilters != nil {
		data[`search_filters`] = *model.SearchFilters
	}
	return data
}

func boolToSmallInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// ======================== Filter tree → SQL ========================

// viewFilterBuilder renders a filter tree into a parameterized WHERE fragment
// against the requirement list alias `r`. Status placeholder sets are loaded lazily.
type viewFilterBuilder struct {
	adminId                     int
	wsId                        string
	typeKey                     string
	userId                      int
	workspaceIds                []int
	args                        []any
	argIndex                    int
	statusLoaded                bool
	allStatusIds                []int
	endStatusIds                []int
	completedStatusIds          []int
	timezone                    string
	customLoaded                bool
	customFields                map[int]CustomFieldModel
	enterpriseRoleMembersLoaded bool
	enterpriseRoleMemberIds     []int
}

// buildViewFilterCondition parses stored filters JSON and returns (sql, args).
// An empty result sql means the view applies no filtering.
func buildViewFilterCondition(adminId int, wsId, typeKey string, userId int, workspaceIds []int, filtersJSON, timezone string, nextArgIndex int) (string, []any, error) {
	trimmed := strings.TrimSpace(filtersJSON)
	if trimmed == `` {
		return ``, nil, nil
	}
	var root ViewFilterNode
	if err := json.Unmarshal([]byte(trimmed), &root); err != nil {
		logs.Error("parse view filters failed, err:%s", err.Error())
		return ``, nil, err
	}
	return buildViewFilterNodeCondition(adminId, wsId, typeKey, userId, workspaceIds, root, timezone, nextArgIndex)
}

// buildViewFilterNodeCondition compiles one parsed filter tree into a WHERE fragment.
func buildViewFilterNodeCondition(adminId int, wsId, typeKey string, userId int, workspaceIds []int, root ViewFilterNode, timezone string, nextArgIndex int) (string, []any, error) {
	builder := &viewFilterBuilder{
		adminId:      adminId,
		wsId:         wsId,
		typeKey:      typeKey,
		userId:       userId,
		workspaceIds: workspaceIds,
		timezone:     timezone,
		argIndex:     nextArgIndex,
	}
	if builder.timezone == `` {
		builder.timezone = define.ViewFilterDefaultTimezone
	}
	sql, err := builder.buildNode(root, 1)
	if err != nil {
		return ``, nil, err
	}
	return sql, builder.args, nil
}

// buildGanttViewFilterCondition compiles the three-section gantt filters into one
// WHERE fragment: common AND ((type='requirement' AND req) OR (type='defect' AND def)).
// Sections compile against their own type so status placeholders expand correctly;
// the common section only holds shared fields and compiles as requirement.
func buildGanttViewFilterCondition(adminId int, wsId string, userId int, workspaceIds []int, filtersJSON, timezone string, nextArgIndex int) (string, []any, error) {
	trimmed := strings.TrimSpace(filtersJSON)
	if trimmed == `` {
		return ``, nil, nil
	}
	var sections define.GanttViewFilters
	if err := json.Unmarshal([]byte(trimmed), &sections); err != nil {
		logs.Error("parse gantt view filters failed, err:%s", err.Error())
		return ``, nil, err
	}
	args := make([]any, 0)
	buildSection := func(node *ViewFilterNode, sectionType string) (string, error) {
		if node == nil {
			return ``, nil
		}
		sql, sectionArgs, err := buildViewFilterNodeCondition(adminId, wsId, sectionType, userId, workspaceIds, *node, timezone, nextArgIndex+len(args))
		if err != nil {
			return ``, err
		}
		args = append(args, sectionArgs...)
		return sql, nil
	}
	parts := make([]string, 0, 2)
	commonSQL, err := buildSection(sections.Common, define.TypeKeyRequirement)
	if err != nil {
		return ``, nil, err
	}
	if commonSQL != `` {
		parts = append(parts, commonSQL)
	}
	reqSQL, err := buildSection(sections.Requirement, define.TypeKeyRequirement)
	if err != nil {
		return ``, nil, err
	}
	defectSQL, err := buildSection(sections.Defect, define.TypeKeyDefect)
	if err != nil {
		return ``, nil, err
	}
	if reqSQL != `` || defectSQL != `` {
		reqCond := `r.type = '` + define.TypeKeyRequirement + `'`
		if reqSQL != `` {
			reqCond = `(` + reqCond + ` AND ` + reqSQL + `)`
		}
		defectCond := `r.type = '` + define.TypeKeyDefect + `'`
		if defectSQL != `` {
			defectCond = `(` + defectCond + ` AND ` + defectSQL + `)`
		}
		parts = append(parts, `(`+reqCond+` OR `+defectCond+`)`)
	}
	if len(parts) == 0 {
		return ``, nil, nil
	}
	return `(` + strings.Join(parts, ` AND `) + `)`, args, nil
}

// ValidateGanttViewFilterSections validates the three gantt sections against
// workspace field configs; the common section is checked with the shared-field
// whitelist first (in validate) and typed values compile as requirement fields.
func ValidateGanttViewFilterSections(adminId int, wsId string, sections define.GanttViewFilters) error {
	if sections.Common != nil {
		if err := ValidateWorkItemFilterFields(adminId, wsId, define.TypeKeyRequirement, *sections.Common); err != nil {
			return err
		}
	}
	if sections.Requirement != nil {
		if err := ValidateWorkItemFilterFields(adminId, wsId, define.TypeKeyRequirement, *sections.Requirement); err != nil {
			return err
		}
	}
	if sections.Defect != nil {
		if err := ValidateWorkItemFilterFields(adminId, wsId, define.TypeKeyDefect, *sections.Defect); err != nil {
			return err
		}
	}
	return nil
}

// addArg appends one query argument and returns its placeholder.
func (b *viewFilterBuilder) addArg(value any) string {
	b.args = append(b.args, value)
	placeholder := fmt.Sprintf(`$%d`, b.argIndex)
	b.argIndex++
	return placeholder
}

// buildNode renders a group or leaf node; empty string means "no constraint".
func (b *viewFilterBuilder) buildNode(node ViewFilterNode, depth int) (string, error) {
	if depth > define.ViewFilterBuildMaxDepth {
		return ``, errors.New(`view filter nested too deep`)
	}
	if node.IsGroup() {
		joiner := ` AND `
		if strings.EqualFold(node.Logic, define.ViewFilterLogicOr) {
			joiner = ` OR `
		}
		parts := make([]string, 0, len(node.Conditions))
		for _, child := range node.Conditions {
			part, err := b.buildNode(child, depth+1)
			if err != nil {
				return ``, err
			}
			if part != `` {
				parts = append(parts, part)
			}
		}
		if len(parts) == 0 {
			return ``, nil
		}
		if len(parts) == 1 {
			return parts[0], nil
		}
		return `(` + strings.Join(parts, joiner) + `)`, nil
	}
	return b.buildLeaf(node)
}

// buildLeaf renders one field condition.
func (b *viewFilterBuilder) buildLeaf(node ViewFilterNode) (string, error) {
	if (node.Operator == define.ViewFilterOperatorIn || node.Operator == define.ViewFilterOperatorNotIn) && len(node.Values) == 0 {
		return ``, nil
	}
	switch node.Field {
	case define.ViewFilterFieldID:
		id := cast.ToInt(firstTrimmed(node.Values))
		if node.Operator != define.ViewFilterOperatorEqual || id <= 0 {
			return ``, nil
		}
		return `r.id = ` + b.addArg(id), nil
	case define.ViewFilterFieldTitle:
		return b.buildTitle(node)
	case define.ViewFilterFieldStatus:
		ids, err := b.expandStatusValues(node.Values)
		if err != nil {
			return ``, err
		}
		return b.buildIntColumn(`COALESCE(r.status_id, 0)`, node.Operator, ids)
	case define.ViewFilterFieldPriority:
		return b.buildIntColumn(`COALESCE(r.priority, 0)`, node.Operator, nonNegativeIntValues(node.Values))
	case define.ViewFilterFieldTemplate:
		return b.buildIntColumn(`r.template_id`, node.Operator, nonNegativeIntValues(node.Values))
	case define.ViewFilterFieldCreator:
		ids, err := b.expandMemberValues(node.Values)
		if err != nil {
			return ``, err
		}
		return b.buildSingleMember(`COALESCE(r.created_by, 0)`, node.Operator, ids)
	case define.ViewFilterFieldVersion:
		return b.buildIntColumn(`COALESCE(r.version_id, 0)`, node.Operator, nonNegativeIntValues(node.Values))
	case define.ViewFilterFieldWorkspace:
		return b.buildIntColumn(`r.workspace_id`, node.Operator, nonNegativeIntValues(node.Values))
	case define.ViewFilterFieldWorkItemType:
		return b.buildStringColumn(`r.type`, node.Operator, trimmedValues(node.Values))
	case define.ViewFilterFieldAssignee:
		return b.buildAssignee(node)
	case define.ViewFilterFieldAssignmentAge, define.ViewFilterFieldMyAssignmentAge:
		if b.typeKey != define.TypeKeyRequirement {
			return ``, errors.New(`assignment age requires requirement type`)
		}
		return b.buildAssignmentAge(node)
	case define.ViewFilterFieldCategory:
		if b.typeKey != define.TypeKeyRequirement {
			return ``, errors.New(`category view filter requires requirement type`)
		}
		ids, err := b.expandCategoryValues(node.Values)
		if err != nil {
			return ``, err
		}
		return b.buildIntColumn(`COALESCE(r.category_id, 0)`, node.Operator, ids)
	case define.ViewFilterFieldAgingStatus:
		if b.typeKey != define.TypeKeyRequirement {
			return ``, errors.New(`aging status view filter requires requirement type`)
		}
		return b.buildIntColumn(`COALESCE(r.aging_status, 0)`, node.Operator, nonNegativeIntValues(node.Values))
	case define.ViewFilterFieldPlannedStartDate, define.ViewFilterFieldPlannedEndDate, define.ViewFilterFieldCreatedAt:
		return b.buildTimestampDate(node.Field, node)
	default:
		if strings.HasPrefix(node.Field, define.ViewFilterCustomPrefix) {
			return b.buildCustomField(node)
		}
		return ``, fmt.Errorf(`unsupported view filter field: %s`, node.Field)
	}
}

func viewFilterHours(values []string) (int, error) {
	if len(values) > 1 {
		return 0, errors.New(`hour filter accepts one value`)
	}
	value := firstTrimmed(values)
	if value == `` {
		return 0, nil
	}
	hours, err := strconv.Atoi(value)
	if err != nil || hours < 1 || hours > 999 {
		return 0, errors.New(`hour filter requires 1 to 999 hours`)
	}
	return hours, nil
}

func (b *viewFilterBuilder) buildAssignmentAge(node ViewFilterNode) (string, error) {
	if !slices.Contains(define.ViewFilterFieldOperators[node.Field], node.Operator) {
		return ``, errors.New(`unsupported assignment age operator`)
	}
	hours, err := viewFilterHours(node.Values)
	if err != nil || hours == 0 {
		return ``, err
	}
	now := time.Now()
	threshold := now.Add(-time.Duration(hours) * time.Hour).Unix()
	var condition string
	if node.Operator == define.ViewFilterOperatorOlderThanHours {
		condition = `assignment.started_at > 0 AND assignment.started_at < ` + b.addArg(threshold)
	} else {
		condition = `assignment.started_at BETWEEN ` + b.addArg(threshold) + ` AND ` + b.addArg(now.Unix())
	}
	person := ``
	if node.Field == define.ViewFilterFieldMyAssignmentAge {
		person = ` AND assigned.user_id = ` + b.addArg(b.userId)
	}
	// 仅重新加入该处理人才会重置计时。初始处理人没有分配记录，使用创建时间。
	return `EXISTS (
		SELECT 1 FROM unnest(r.assignee_ids) AS assigned(user_id)
		CROSS JOIN LATERAL (
			SELECT COALESCE(MAX(h.created_at), NULLIF(r.created_at, 0)) AS started_at
			FROM ` + define.TableRequirementHistories + ` h
			WHERE h.admin_id = r.admin_id AND h.workspace_id = r.workspace_id AND h.requirement_id = r.id
			  AND h.field_name = 'assignee' AND h.action IN ('assignee_change', 'field_update')
			  AND assigned.user_id::text = ANY(string_to_array(translate(h.new_value, '[]{} ', ''), ','))
			  AND NOT (assigned.user_id::text = ANY(string_to_array(translate(COALESCE(h.old_value, ''), '[]{} ', ''), ',')))
		) assignment
		WHERE ` + condition + person + `
	)`, nil
}

func (b *viewFilterBuilder) buildTimestampDate(column string, node ViewFilterNode) (string, error) {
	if column == define.ViewFilterFieldCreatedAt && node.Operator == define.ViewFilterOperatorOlderThanHours {
		hours, err := viewFilterHours(node.Values)
		if err != nil || hours == 0 {
			return ``, err
		}
		threshold := time.Now().Add(-time.Duration(hours) * time.Hour).Unix()
		return `r.created_at > 0 AND r.created_at < ` + b.addArg(threshold), nil
	}

	if node.Operator == define.ViewFilterOperatorEmpty {
		return fmt.Sprintf(`COALESCE(r.%s, 0) = 0`, column), nil
	}
	if node.Operator == define.ViewFilterOperatorNotEmpty {
		return fmt.Sprintf(`COALESCE(r.%s, 0) <> 0`, column), nil
	}
	startValue := firstTrimmed(node.Values)
	endValue := ``
	if len(node.Values) > 1 {
		endValue = strings.TrimSpace(node.Values[1])
	}
	switch node.Operator {
	case define.ViewFilterOperatorEqual:
		endValue = startValue
	case define.ViewFilterOperatorBetween:
	case define.ViewFilterOperatorDynamic:
		var err error
		if startValue, err = b.resolveDynamicDate(startValue); err != nil {
			return ``, err
		}
		if endValue, err = b.resolveDynamicDate(endValue); err != nil {
			return ``, err
		}
	default:
		return ``, fmt.Errorf(`unsupported date operator: %s`, node.Operator)
	}
	if startValue == `` && endValue == `` {
		return ``, nil
	}
	timezoneArg := b.addArg(b.timezone)
	expression := fmt.Sprintf(`timezone(%s, to_timestamp(NULLIF(r.%s, 0)))::date`, timezoneArg, column)
	conditions := make([]string, 0, 2)
	if startValue != `` {
		conditions = append(conditions, expression+` >= `+b.addArg(startValue)+`::date`)
	}
	if endValue != `` {
		conditions = append(conditions, expression+` <= `+b.addArg(endValue)+`::date`)
	}
	return strings.Join(conditions, ` AND `), nil
}

// resolveDynamicDate converts a relative `<offset>:<unit>` value into a date in the request timezone.
func (b *viewFilterBuilder) resolveDynamicDate(value string) (string, error) {
	if value == `` {
		return ``, nil
	}
	parts := strings.SplitN(value, `:`, 2)
	if len(parts) != 2 {
		return ``, errors.New(`invalid dynamic date value`)
	}
	offset, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return ``, errors.New(`invalid dynamic date offset`)
	}
	location, err := time.LoadLocation(b.timezone)
	if err != nil {
		return ``, err
	}
	current := time.Now().In(location)
	switch strings.TrimSpace(parts[1]) {
	case `day`:
		current = current.AddDate(0, 0, offset)
	case `week`:
		current = current.AddDate(0, 0, offset*7)
	case `month`:
		current = current.AddDate(0, offset, 0)
	default:
		return ``, errors.New(`invalid dynamic date unit`)
	}
	return current.Format(define.ViewFilterDateLayout), nil
}

func (b *viewFilterBuilder) loadCustomFields() error {
	if b.customLoaded {
		return nil
	}
	result, err := ListCustomFields(ListCustomFieldsModel{AdminId: b.adminId, WsId: b.wsId, TypeKey: b.typeKey})
	if err != nil {
		return err
	}
	b.customFields = make(map[int]CustomFieldModel, len(result.Items))
	for _, field := range result.Items {
		if field.IsEnabled == 1 {
			b.customFields[field.Id] = field
		}
	}
	b.customLoaded = true
	return nil
}

func (b *viewFilterBuilder) customExists(fieldId int, predicate string, predicateArgs ...any) string {
	adminArg := b.addArg(b.adminId)
	fieldArg := b.addArg(fieldId)
	for _, value := range predicateArgs {
		predicate = strings.Replace(predicate, `{arg}`, b.addArg(value), 1)
	}
	if predicate != `` {
		predicate = ` AND (` + predicate + `)`
	}
	return fmt.Sprintf(`EXISTS (SELECT 1 FROM requirement_custom_field_values cfv WHERE cfv.admin_id = %s AND cfv.requirement_id = r.id AND cfv.field_id = %s%s)`, adminArg, fieldArg, predicate)
}

const customJSONValueSQL = `NULLIF(BTRIM(cfv.value_text), '')::jsonb`

func (b *viewFilterBuilder) buildCustomField(node ViewFilterNode) (string, error) {
	fieldId := cast.ToInt(strings.TrimPrefix(node.Field, define.ViewFilterCustomPrefix))
	if fieldId <= 0 {
		return ``, fmt.Errorf(`invalid custom field: %s`, node.Field)
	}
	if err := b.loadCustomFields(); err != nil {
		return ``, err
	}
	field, ok := b.customFields[fieldId]
	if !ok {
		return ``, nil // Deleted or disabled historical fields no longer constrain a view.
	}
	if node.Operator == define.ViewFilterOperatorEmpty || node.Operator == define.ViewFilterOperatorNotEmpty {
		notEmpty := b.customFieldNotEmpty(fieldId, field)
		if node.Operator == define.ViewFilterOperatorEmpty {
			return `NOT (` + notEmpty + `)`, nil
		}
		return notEmpty, nil
	}
	switch field.FieldType {
	case define.ViewFilterCustomTypeText, define.ViewFilterCustomTypeTextarea, define.ViewFilterCustomTypePhone:
		value := firstTrimmed(node.Values)
		if value == `` {
			return ``, nil
		}
		switch node.Operator {
		case define.ViewFilterOperatorContains:
			return b.customExists(fieldId, `cfv.value_text ILIKE {arg}`, `%`+value+`%`), nil
		case define.ViewFilterOperatorNotContains:
			return `NOT ` + b.customExists(fieldId, `cfv.value_text ILIKE {arg}`, `%`+value+`%`), nil
		case define.ViewFilterOperatorEqual:
			return b.customExists(fieldId, `cfv.value_text = {arg}`, value), nil
		case define.ViewFilterOperatorNotEqual:
			return `NOT ` + b.customExists(fieldId, `cfv.value_text = {arg}`, value), nil
		}
	case define.ViewFilterCustomTypeNumber, define.ViewFilterCustomTypeNumeric:
		return b.buildCustomNumeric(fieldId, node)
	case define.ViewFilterCustomTypeDate, define.ViewFilterCustomTypeDateTime:
		if node.Operator == define.ViewFilterOperatorBetween {
			parts := []string{`left(cfv.value_text, 10) ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$'`}
			args := []any{}
			if len(node.Values) > 0 && strings.TrimSpace(node.Values[0]) != `` {
				parts = append(parts, `left(cfv.value_text, 10)::date >= {arg}::date`)
				args = append(args, strings.TrimSpace(node.Values[0]))
			}
			if len(node.Values) > 1 && strings.TrimSpace(node.Values[1]) != `` {
				parts = append(parts, `left(cfv.value_text, 10)::date <= {arg}::date`)
				args = append(args, strings.TrimSpace(node.Values[1]))
			}
			if len(parts) == 1 {
				return ``, nil
			}
			return b.customExists(fieldId, strings.Join(parts, ` AND `), args...), nil
		}
	case define.ViewFilterCustomTypeSelectSingle, define.ViewFilterCustomTypeRadio, define.ViewFilterCustomTypeUser:
		if node.Operator == define.ViewFilterOperatorIn || node.Operator == define.ViewFilterOperatorNotIn ||
			node.Operator == define.ViewFilterOperatorContains || node.Operator == define.ViewFilterOperatorNotContains ||
			node.Operator == define.ViewFilterOperatorEqual || node.Operator == define.ViewFilterOperatorNotEqual {
			values := trimmedValues(node.Values)
			if field.FieldType == define.ViewFilterCustomTypeUser {
				ids, err := b.expandMemberValues(values)
				if err != nil {
					return ``, err
				}
				return b.buildCustomSingleMember(fieldId, node.Operator, ids), nil
			}
			if len(values) == 0 {
				return ``, nil
			}
			condition := b.customExists(fieldId, `cfv.value_text = ANY({arg})`, pq.Array(values))
			if node.Operator == define.ViewFilterOperatorNotIn {
				condition = `NOT ` + condition
			}
			return condition, nil
		}
	case define.ViewFilterCustomTypeSelectMulti, define.ViewFilterCustomTypeCheckbox, define.ViewFilterCustomTypeUserMulti:
		if node.Operator == define.ViewFilterOperatorIn || node.Operator == define.ViewFilterOperatorNotIn ||
			node.Operator == define.ViewFilterOperatorContains || node.Operator == define.ViewFilterOperatorNotContains ||
			node.Operator == define.ViewFilterOperatorEqual || node.Operator == define.ViewFilterOperatorNotEqual {
			values := trimmedValues(node.Values)
			if field.FieldType == define.ViewFilterCustomTypeUserMulti {
				ids, err := b.expandMemberValues(values)
				if err != nil {
					return ``, err
				}
				return b.buildCustomMultiMember(fieldId, node.Operator, ids), nil
			}
			if len(values) == 0 {
				return ``, nil
			}
			condition := b.customExists(fieldId, customJSONValueSQL+` ?| {arg}`, pq.Array(values))
			if node.Operator == define.ViewFilterOperatorNotIn {
				condition = `NOT ` + condition
			}
			return condition, nil
		}
	case define.ViewFilterCustomTypeCascaderSingle, define.ViewFilterCustomTypeCascaderMulti:
		if node.Operator == define.ViewFilterOperatorIn || node.Operator == define.ViewFilterOperatorNotIn {
			selected := cascaderFilterValues(field.FieldType, node.Values)
			if field.FieldType == define.ViewFilterCustomTypeCascaderMulti && node.Operator == define.ViewFilterOperatorIn && slices.Contains(selected, define.ViewValueAll) {
				return ``, nil
			}
			includeEmpty := field.FieldType == define.ViewFilterCustomTypeCascaderMulti &&
				node.Operator == define.ViewFilterOperatorIn && slices.Contains(selected, define.ViewValueEmpty)
			includeNotEmpty := field.FieldType == define.ViewFilterCustomTypeCascaderMulti &&
				node.Operator == define.ViewFilterOperatorIn && slices.Contains(selected, define.ViewValueNotEmpty)
			if includeEmpty && includeNotEmpty {
				return ``, nil
			}
			if includeNotEmpty {
				return b.customFieldNotEmpty(fieldId, field), nil
			}
			if includeEmpty {
				selected = slices.DeleteFunc(selected, func(value string) bool {
					return value == define.ViewValueEmpty
				})
			}
			emptyCondition := ``
			if includeEmpty {
				emptyCondition = `NOT (` + b.customFieldNotEmpty(fieldId, field) + `)`
			}
			values := selected
			predicate := customJSONValueSQL + ` ?| {arg}`
			if field.FieldType == define.ViewFilterCustomTypeCascaderSingle {
				values = expandCascaderFilterKeys(field.Cascader, selected)
				values = cascaderStoredPaths(field.Cascader, nil, values)
				predicate = customJSONValueSQL + ` = ANY({arg}::JSONB[])`
			}
			if len(values) == 0 {
				return emptyCondition, nil
			}
			condition := b.customExists(fieldId, predicate, pq.Array(values))
			if node.Operator == define.ViewFilterOperatorNotIn {
				condition = `NOT ` + condition
			}
			if emptyCondition != `` {
				condition = `(` + emptyCondition + ` OR ` + condition + `)`
			}
			return condition, nil
		}
	}
	return ``, fmt.Errorf(`unsupported custom field filter: %s/%s`, field.FieldType, node.Operator)
}

func (b *viewFilterBuilder) buildCustomNumeric(fieldId int, node ViewFilterNode) (string, error) {
	numeric := `cfv.value_text ~ '^-?[0-9]+(\.[0-9]+)?$'`
	switch node.Operator {
	case define.ViewFilterOperatorBetween:
		parts := []string{numeric}
		args := []any{}
		if len(node.Values) > 0 && strings.TrimSpace(node.Values[0]) != `` {
			parts = append(parts, `cfv.value_text::numeric >= {arg}`)
			args = append(args, strings.TrimSpace(node.Values[0]))
		}
		if len(node.Values) > 1 && strings.TrimSpace(node.Values[1]) != `` {
			parts = append(parts, `cfv.value_text::numeric <= {arg}`)
			args = append(args, strings.TrimSpace(node.Values[1]))
		}
		if len(parts) == 1 {
			return ``, nil
		}
		return b.customExists(fieldId, strings.Join(parts, ` AND `), args...), nil
	case define.ViewFilterOperatorEqual, define.ViewFilterOperatorNotEqual,
		define.ViewFilterOperatorGreater, define.ViewFilterOperatorGreaterEq,
		define.ViewFilterOperatorLess, define.ViewFilterOperatorLessEq:
		value := firstTrimmed(node.Values)
		if value == `` {
			return ``, nil
		}
		if node.Operator == define.ViewFilterOperatorNotEqual {
			condition := b.customExists(fieldId, numeric+` AND cfv.value_text::numeric = {arg}`, value)
			return `NOT ` + condition, nil
		}
		op := map[string]string{
			define.ViewFilterOperatorEqual:     `=`,
			define.ViewFilterOperatorGreater:   `>`,
			define.ViewFilterOperatorGreaterEq: `>=`,
			define.ViewFilterOperatorLess:      `<`,
			define.ViewFilterOperatorLessEq:    `<=`,
		}[node.Operator]
		return b.customExists(fieldId, numeric+` AND cfv.value_text::numeric `+op+` {arg}`, value), nil
	default:
		return ``, fmt.Errorf(`unsupported custom field filter: number/%s`, node.Operator)
	}
}

func cascaderFilterValues(fieldType string, values []string) []string {
	result := trimmedValues(values)
	if fieldType == define.ViewFilterCustomTypeCascaderSingle && len(result) > 1 {
		return result[len(result)-1:]
	}
	return result
}

func (b *viewFilterBuilder) customFieldNotEmpty(fieldId int, field CustomFieldModel) string {
	if field.FieldType != define.ViewFilterCustomTypeCascaderSingle {
		return b.customExists(fieldId, ``)
	}
	paths := cascaderStoredPaths(field.Cascader, nil, nil)
	if len(paths) == 0 {
		return `FALSE`
	}
	return b.customExists(fieldId, customJSONValueSQL+` = ANY({arg}::JSONB[])`, pq.Array(paths))
}

func cascaderStoredPaths(nodes []CustomFieldCascaderNode, parent, selected []string) []string {
	want := make(map[string]bool, len(selected))
	for _, key := range selected {
		want[key] = true
	}
	paths := make([]string, 0, len(nodes))
	var walk func([]CustomFieldCascaderNode, []string)
	walk = func(current []CustomFieldCascaderNode, prefix []string) {
		for _, node := range current {
			path := append(append([]string{}, prefix...), node.Key)
			if len(want) == 0 || want[node.Key] {
				stored, _ := json.Marshal(path)
				paths = append(paths, string(stored))
				if len(path) > 1 {
					legacy, _ := json.Marshal([]string{node.Key})
					paths = append(paths, string(legacy))
				}
			}
			walk(node.Children, path)
		}
	}
	walk(nodes, parent)
	return paths
}

// expandCascaderFilterKeys expands single-cascader parent filters to current descendant paths.
// Multi-cascader filters keep each explicitly selected node independent.
func expandCascaderFilterKeys(nodes []CustomFieldCascaderNode, selected []string) []string {
	want := make(map[string]bool, len(selected))
	for _, value := range selected {
		want[value] = true
	}
	result := make([]string, 0, len(selected))
	var walk func([]CustomFieldCascaderNode, bool)
	walk = func(current []CustomFieldCascaderNode, inherited bool) {
		for _, node := range current {
			on := inherited || want[node.Key]
			if on {
				result = append(result, node.Key)
			}
			walk(node.Children, on)
		}
	}
	walk(nodes, false)
	return result
}

func trimmedValues(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != `` {
			result = append(result, trimmed)
		}
	}
	return result
}

// expandCategoryValues resolves each selected category to its complete subtree.
func (b *viewFilterBuilder) expandCategoryValues(values []string) ([]int, error) {
	ids := make([]int, 0, len(values))
	seen := map[int]bool{}
	appendId := func(id int) {
		if id >= 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == `0` {
			appendId(0)
			continue
		}
		categoryId := cast.ToInt(trimmed)
		if categoryId <= 0 {
			continue
		}
		subtreeIds, err := ListRequirementCategorySubtreeIDs(b.adminId, cast.ToInt(b.wsId), categoryId)
		if errors.Is(err, ErrRequirementCategoryNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, id := range subtreeIds {
			appendId(id)
		}
	}
	return ids, nil
}

// buildTitle renders title conditions; contains/not_contains split keywords by spaces (all must match).
func (b *viewFilterBuilder) buildTitle(node ViewFilterNode) (string, error) {
	switch node.Operator {
	case define.ViewFilterOperatorContains, define.ViewFilterOperatorNotContains:
		keywords := strings.Fields(strings.Join(node.Values, ` `))
		if len(keywords) == 0 {
			return ``, nil
		}
		parts := make([]string, 0, len(keywords))
		for _, keyword := range keywords {
			placeholder := b.addArg(`%` + keyword + `%`)
			if node.Operator == define.ViewFilterOperatorContains {
				parts = append(parts, `r.title ILIKE `+placeholder)
			} else {
				parts = append(parts, `r.title NOT ILIKE `+placeholder)
			}
		}
		if len(parts) == 1 {
			return parts[0], nil
		}
		return `(` + strings.Join(parts, ` AND `) + `)`, nil
	case define.ViewFilterOperatorEqual:
		value := firstTrimmed(node.Values)
		if value == `` {
			return ``, nil
		}
		return `r.title = ` + b.addArg(value), nil
	case define.ViewFilterOperatorNotEqual:
		value := firstTrimmed(node.Values)
		if value == `` {
			return ``, nil
		}
		return `r.title <> ` + b.addArg(value), nil
	case define.ViewFilterOperatorEmpty:
		return `r.title = ''`, nil
	case define.ViewFilterOperatorNotEmpty:
		return `r.title <> ''`, nil
	default:
		return ``, fmt.Errorf(`unsupported title operator: %s`, node.Operator)
	}
}

// buildIntColumn renders in/not_in/empty/not_empty for a single integer column expression.
func (b *viewFilterBuilder) buildIntColumn(column, operator string, ids []int) (string, error) {
	switch operator {
	case define.ViewFilterOperatorIn:
		if len(ids) == 0 {
			return `FALSE`, nil // e.g. "all finished statuses" resolves to an empty set
		}
		return column + ` = ANY(` + b.addArg(pq.Array(ids)) + `)`, nil
	case define.ViewFilterOperatorNotIn:
		if len(ids) == 0 {
			return ``, nil
		}
		return column + ` <> ALL(` + b.addArg(pq.Array(ids)) + `)`, nil
	case define.ViewFilterOperatorEmpty:
		return column + ` = 0`, nil
	case define.ViewFilterOperatorNotEmpty:
		return column + ` <> 0`, nil
	default:
		return ``, fmt.Errorf(`unsupported operator: %s`, operator)
	}
}

func (b *viewFilterBuilder) buildStringColumn(column, operator string, values []string) (string, error) {
	switch operator {
	case define.ViewFilterOperatorIn:
		if len(values) == 0 {
			return `FALSE`, nil
		}
		return column + ` = ANY(` + b.addArg(pq.Array(values)) + `)`, nil
	case define.ViewFilterOperatorNotIn:
		if len(values) == 0 {
			return ``, nil
		}
		return column + ` <> ALL(` + b.addArg(pq.Array(values)) + `)`, nil
	default:
		return ``, fmt.Errorf(`unsupported string operator: %s`, operator)
	}
}

// buildAssignee renders assignee conditions against the int-array column.
func (b *viewFilterBuilder) buildAssignee(node ViewFilterNode) (string, error) {
	switch node.Operator {
	case define.ViewFilterOperatorEmpty:
		return `COALESCE(cardinality(r.assignee_ids), 0) = 0`, nil
	case define.ViewFilterOperatorNotEmpty:
		return `COALESCE(cardinality(r.assignee_ids), 0) > 0`, nil
	}
	ids, err := b.expandMemberValues(node.Values)
	if err != nil {
		return ``, err
	}
	if len(ids) == 0 {
		return ``, nil
	}
	arrayArg := b.addArg(pq.Array(ids))
	switch node.Operator {
	case define.ViewFilterOperatorIn:
		return `COALESCE(r.assignee_ids && ` + arrayArg + `, FALSE)`, nil
	case define.ViewFilterOperatorNotIn:
		return `NOT COALESCE(r.assignee_ids && ` + arrayArg + `, FALSE)`, nil
	case define.ViewFilterOperatorContains:
		return `COALESCE(r.assignee_ids @> ` + arrayArg + `, FALSE)`, nil
	case define.ViewFilterOperatorNotContains:
		return `NOT COALESCE(r.assignee_ids @> ` + arrayArg + `, FALSE)`, nil
	case define.ViewFilterOperatorEqual:
		return `(COALESCE(r.assignee_ids, '{}'::integer[]) @> ` + arrayArg + ` AND COALESCE(r.assignee_ids, '{}'::integer[]) <@ ` + arrayArg + `)`, nil
	case define.ViewFilterOperatorNotEqual:
		return `NOT (COALESCE(r.assignee_ids, '{}'::integer[]) @> ` + arrayArg + ` AND COALESCE(r.assignee_ids, '{}'::integer[]) <@ ` + arrayArg + `)`, nil
	default:
		return ``, fmt.Errorf(`unsupported assignee operator: %s`, node.Operator)
	}
}

func (b *viewFilterBuilder) buildSingleMember(column, operator string, ids []int) (string, error) {
	if operator == define.ViewFilterOperatorEmpty || operator == define.ViewFilterOperatorNotEmpty {
		return b.buildIntColumn(column, operator, ids)
	}
	if len(ids) == 0 {
		return ``, nil
	}
	switch operator {
	case define.ViewFilterOperatorIn:
		return b.buildIntColumn(column, define.ViewFilterOperatorIn, ids)
	case define.ViewFilterOperatorNotIn:
		return b.buildIntColumn(column, define.ViewFilterOperatorNotIn, ids)
	case define.ViewFilterOperatorContains, define.ViewFilterOperatorEqual:
		if len(ids) != 1 {
			return `FALSE`, nil
		}
		return column + ` = ` + b.addArg(ids[0]), nil
	case define.ViewFilterOperatorNotContains, define.ViewFilterOperatorNotEqual:
		if len(ids) != 1 {
			return ``, nil
		}
		return column + ` <> ` + b.addArg(ids[0]), nil
	default:
		return b.buildIntColumn(column, operator, ids)
	}
}

func (b *viewFilterBuilder) buildCustomSingleMember(fieldId int, operator string, ids []int) string {
	if len(ids) == 0 {
		return ``
	}
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, strconv.Itoa(id))
	}
	switch operator {
	case define.ViewFilterOperatorIn:
		return b.customExists(fieldId, `cfv.value_text = ANY({arg})`, pq.Array(values))
	case define.ViewFilterOperatorNotIn:
		return `NOT ` + b.customExists(fieldId, `cfv.value_text = ANY({arg})`, pq.Array(values))
	case define.ViewFilterOperatorContains, define.ViewFilterOperatorEqual:
		if len(values) != 1 {
			return `FALSE`
		}
		return b.customExists(fieldId, `cfv.value_text = {arg}`, values[0])
	case define.ViewFilterOperatorNotContains, define.ViewFilterOperatorNotEqual:
		if len(values) != 1 {
			return ``
		}
		return `NOT ` + b.customExists(fieldId, `cfv.value_text = {arg}`, values[0])
	}
	return ``
}

func (b *viewFilterBuilder) buildCustomMultiMember(fieldId int, operator string, ids []int) string {
	if len(ids) == 0 {
		return ``
	}
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, strconv.Itoa(id))
	}
	switch operator {
	case define.ViewFilterOperatorIn:
		return b.customExists(fieldId, customJSONValueSQL+` ?| {arg}`, pq.Array(values))
	case define.ViewFilterOperatorNotIn:
		return `NOT ` + b.customExists(fieldId, customJSONValueSQL+` ?| {arg}`, pq.Array(values))
	case define.ViewFilterOperatorContains:
		return b.customExists(fieldId, customJSONValueSQL+` ?& {arg}::text[]`, pq.Array(values))
	case define.ViewFilterOperatorNotContains:
		return `NOT ` + b.customExists(fieldId, customJSONValueSQL+` ?& {arg}::text[]`, pq.Array(values))
	case define.ViewFilterOperatorEqual, define.ViewFilterOperatorNotEqual:
		raw, _ := json.Marshal(values)
		exact := b.customExists(fieldId, customJSONValueSQL+` @> {arg}::jsonb AND `+customJSONValueSQL+` <@ {arg}::jsonb`, string(raw), string(raw))
		if operator == define.ViewFilterOperatorNotEqual {
			return `NOT ` + exact
		}
		return exact
	}
	return ``
}

// expandMemberValues resolves dynamic people selections when the view is queried.
func (b *viewFilterBuilder) expandMemberValues(values []string) ([]int, error) {
	ids := make([]int, 0, len(values))
	seen := map[int]bool{}
	appendId := func(id int) {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, value := range values {
		switch value {
		case ViewValueMe:
			appendId(b.userId)
		case ViewValueCurrentEnterpriseRole:
			roleMemberIds, err := b.loadCurrentEnterpriseRoleMemberIds()
			if err != nil {
				return nil, err
			}
			for _, id := range roleMemberIds {
				appendId(id)
			}
		default:
			appendId(cast.ToInt(strings.TrimSpace(value)))
		}
	}
	return ids, nil
}

func (b *viewFilterBuilder) loadCurrentEnterpriseRoleMemberIds() ([]int, error) {
	if b.enterpriseRoleMembersLoaded {
		return b.enterpriseRoleMemberIds, nil
	}
	const query = `SELECT member.id FROM users requester
		JOIN roles enterprise_role ON enterprise_role.admin_id = requester.admin_id AND enterprise_role.id = requester.role_id AND enterprise_role.scope = $3
		JOIN users member ON member.admin_id = requester.admin_id AND member.role_id = requester.role_id
		WHERE requester.admin_id = $1 AND requester.id = $2 AND requester.deleted_at = 0
			AND member.deleted_at = 0
		ORDER BY member.id`
	rows, err := msql.RawValues(define.Postgres, query, nil, b.adminId, b.userId, permission.ScopeEnterprise)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return nil, err
	}
	b.enterpriseRoleMemberIds = make([]int, 0, len(rows))
	for _, row := range rows {
		b.enterpriseRoleMemberIds = append(b.enterpriseRoleMemberIds, cast.ToInt(row[`id`]))
	}
	b.enterpriseRoleMembersLoaded = true
	return b.enterpriseRoleMemberIds, nil
}

// expandStatusValues maps status values to ids, resolving final-status placeholders
// against the workspace settings of the current work item type.
func (b *viewFilterBuilder) expandStatusValues(values []string) ([]int, error) {
	ids := make([]int, 0, len(values))
	seen := map[int]bool{}
	appendId := func(id int) {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, value := range values {
		switch value {
		case define.ViewValueAllCompleted, define.ViewValueAllClosed:
			if err := b.loadStatusSets(); err != nil {
				return nil, err
			}
			selected := b.completedStatusIds
			if value == define.ViewValueAllClosed {
				selected = statusDifference(b.endStatusIds, b.completedStatusIds)
			}
			for _, id := range selected {
				appendId(id)
			}
		case define.ViewValueAllEnd:
			if err := b.loadStatusSets(); err != nil {
				return nil, err
			}
			for _, id := range b.endStatusIds {
				appendId(id)
			}
		case define.ViewValueAllNotEnd:
			if err := b.loadStatusSets(); err != nil {
				return nil, err
			}
			endSet := map[int]bool{}
			for _, id := range b.endStatusIds {
				endSet[id] = true
			}
			for _, id := range b.allStatusIds {
				if !endSet[id] {
					appendId(id)
				}
			}
		default:
			appendId(cast.ToInt(strings.TrimSpace(value)))
		}
	}
	return ids, nil
}

// loadStatusSets lazily loads all status ids of the list context plus its configured final statuses.
func (b *viewFilterBuilder) loadStatusSets() error {
	if b.statusLoaded {
		return nil
	}
	if b.typeKey == define.TypeKeyIteration {
		workspaceIds := b.workspaceIds
		if len(workspaceIds) == 0 {
			workspaceIds = []int{cast.ToInt(b.wsId)}
		}
		rows, err := msql.RawValues(define.Postgres, `SELECT id FROM workspace_statuses
			WHERE admin_id = $1 AND workspace_id = ANY($2) AND type_key IN ($3, $4)`, nil,
			b.adminId, pq.Array(workspaceIds), define.TypeKeyRequirement, define.TypeKeyDefect)
		if err != nil {
			logs.Error("load iteration statuses failed, err:%s", err.Error())
			return err
		}
		for _, row := range rows {
			b.allStatusIds = append(b.allStatusIds, cast.ToInt(row[`id`]))
		}
		for _, workspaceId := range workspaceIds {
			settings, err := GetWorkspaceSettings(WorkspaceSettingsModel{AdminId: b.adminId, WsId: cast.ToString(workspaceId)})
			if err != nil {
				return err
			}
			b.endStatusIds = append(b.endStatusIds, settings.RequirementFinalStatusIds...)
			b.endStatusIds = append(b.endStatusIds, settings.DefectFinalStatusIds...)
			b.completedStatusIds = append(b.completedStatusIds, settings.RequirementCompletedStatusIds...)
			b.completedStatusIds = append(b.completedStatusIds, settings.DefectCompletedStatusIds...)
		}
		b.statusLoaded = true
		return nil
	}

	m := msql.Model(define.TableWorkItemStatuses, define.Postgres)
	rows, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(b.adminId)},
		{`workspace_id`, b.wsId},
		{`type_key`, b.typeKey},
	}).Field(`id`).Select()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return err
	}
	b.allStatusIds = make([]int, 0, len(rows))
	for _, row := range rows {
		b.allStatusIds = append(b.allStatusIds, cast.ToInt(row[`id`]))
	}
	settings, err := GetWorkspaceSettings(WorkspaceSettingsModel{AdminId: b.adminId, WsId: b.wsId})
	if err != nil {
		return err
	}
	if b.typeKey == define.TypeKeyDefect {
		b.endStatusIds = settings.DefectFinalStatusIds
		b.completedStatusIds = settings.DefectCompletedStatusIds
	} else {
		b.endStatusIds = settings.RequirementFinalStatusIds
		b.completedStatusIds = settings.RequirementCompletedStatusIds
	}
	b.statusLoaded = true
	return nil
}

// nonNegativeIntValues converts numeric values to non-negative ints; zero represents an unset field.
func nonNegativeIntValues(values []string) []int {
	ids := make([]int, 0, len(values))
	seen := map[int]bool{}
	for _, value := range values {
		id, err := strconv.Atoi(strings.TrimSpace(value))
		if err == nil && id >= 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// firstTrimmed returns the first non-empty trimmed value.
func firstTrimmed(values []string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != `` {
			return trimmed
		}
	}
	return ``
}

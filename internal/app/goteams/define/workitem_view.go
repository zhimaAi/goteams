// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package define

// ======================== Work Item View Definitions ========================
// Shared between validate (structure checks) and business (SQL building).

// View type constants.
const (
	ViewTypeSystem   = `system`
	ViewTypePersonal = `personal`
)

// Builtin view keys.
const (
	ViewBuiltinAll        = `all`
	ViewBuiltinMyAssigned = `my_assigned`
	ViewBuiltinMyCreated  = `my_created`
	ViewBuiltinUnfinished = `unfinished`
	ViewBuiltinFinished   = `finished`
)

// Builtin view i18n keys.
const (
	ViewBuiltinNameAll        = `view_builtin_all`
	ViewBuiltinNameMyAssigned = `view_builtin_my_assigned`
	ViewBuiltinNameMyCreated  = `view_builtin_my_created`
	ViewBuiltinNameUnfinished = `view_builtin_unfinished`
	ViewBuiltinNameFinished   = `view_builtin_finished`
)

// Builtin view filter payloads.
const (
	ViewBuiltinFiltersAll        = ``
	ViewBuiltinFiltersMyAssigned = `{"logic":"and","conditions":[{"field":"assignee","operator":"in","values":["__me__"]}]}`
	ViewBuiltinFiltersMyCreated  = `{"logic":"and","conditions":[{"field":"creator","operator":"in","values":["__me__"]}]}`
	ViewBuiltinFiltersUnfinished = `{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]}]}`
	ViewBuiltinFiltersFinished   = `{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_end__"]}]}`
)

// Builtin gantt view filter payloads: the gantt filters column stores a
// three-section JSON object {common, requirement, defect}; each section is a
// regular ViewFilterNode tree compiled against its own work item type.
const (
	GanttViewBuiltinFiltersMyAssigned = `{"common":{"logic":"and","conditions":[{"field":"assignee","operator":"in","values":["__me__"]}]}}`
	GanttViewBuiltinFiltersMyCreated  = `{"common":{"logic":"and","conditions":[{"field":"creator","operator":"in","values":["__me__"]}]}}`
	GanttViewBuiltinFiltersUnfinished = `{"requirement":{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]}]},"defect":{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]}]}}`
	GanttViewBuiltinFiltersFinished   = `{"requirement":{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_end__"]}]},"defect":{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_end__"]}]}}`
)

// ViewDefaultSearchFilters is the initial requirement/defect search bar configuration.
const ViewDefaultSearchFilters = `{"logic":"and","conditions":[{"field":"title","operator":"contains","values":[]},{"field":"status","operator":"in","values":[]},{"field":"assignee","operator":"in","values":[]},{"field":"version","operator":"in","values":[]},{"field":"creator","operator":"in","values":[]}]}`

// IterationDefaultSearchFilters is the initial cross-workspace iteration search bar configuration.
const IterationDefaultSearchFilters = `{"logic":"and","conditions":[{"field":"title","operator":"contains","values":[]},{"field":"workspace","operator":"in","values":[]},{"field":"work_item_type","operator":"in","values":[]},{"field":"status","operator":"in","values":[]},{"field":"priority","operator":"in","values":[]},{"field":"assignee","operator":"in","values":[]},{"field":"creator","operator":"in","values":[]}]}`

// Dynamic placeholder values inside view filter conditions, expanded at query time.
const (
	ViewValueMe                    = `__me__`                      // current request user
	ViewValueCurrentEnterpriseRole = `__current_enterprise_role__` // members sharing the request user's enterprise role
	ViewValueAll                   = `__all__`                     // all configured values plus an empty field value
	ViewValueEmpty                 = `__empty__`                   // an empty field value combined with selected configured values
	ViewValueNotEmpty              = `__not_empty__`               // a non-empty field value combined with selected configured values
	ViewValueAllCompleted          = `__all_completed__`           // 所有完成状态
	ViewValueAllClosed             = `__all_closed__`              // 所有关闭状态
	ViewValueAllEnd                = `__all_end__`                 // all final statuses of the workspace type
	ViewValueAllNotEnd             = `__all_not_end__`             // all non-final statuses of the workspace type
)

// Filter tree constants.
const (
	ViewNameMinLength          = 1
	ViewNameMaxLength          = 50
	ViewFilterValidateMaxDepth = 5
	ViewFilterBuildMaxDepth    = 8
	ViewFilterMaxNodes         = 50
	ViewFilterMaxValues        = 50
	ViewFilterValueMaxLength   = 100
	ViewFilterCustomPrefix     = `custom:`
	ViewFilterDateLayout       = `2006-01-02`
	ViewFilterDefaultTimezone  = `UTC`
)

// Filter group logic constants.
const (
	ViewFilterLogicAnd = `and`
	ViewFilterLogicOr  = `or`
)

// Builtin filter field constants.
const (
	ViewFilterFieldID               = `id`
	ViewFilterFieldTitle            = `title`
	ViewFilterFieldTemplate         = `template`
	ViewFilterFieldStatus           = `status`
	ViewFilterFieldPriority         = `priority`
	ViewFilterFieldAssignee         = `assignee`
	ViewFilterFieldAssignmentAge    = `assignment_age`
	ViewFilterFieldMyAssignmentAge  = `my_assignment_age`
	ViewFilterFieldCreator          = `creator`
	ViewFilterFieldVersion          = `version`
	ViewFilterFieldWorkspace        = `workspace`
	ViewFilterFieldWorkItemType     = `work_item_type`
	ViewFilterFieldCategory         = `category`
	ViewFilterFieldPlannedStartDate = `planned_start_date`
	ViewFilterFieldPlannedEndDate   = `planned_end_date`
	ViewFilterFieldCreatedAt        = `created_at`
	ViewFilterFieldAgingStatus      = `aging_status`
)

// Filter operator constants.
const (
	ViewFilterOperatorContains    = `contains`
	ViewFilterOperatorNotContains = `not_contains`
	ViewFilterOperatorEqual       = `eq`
	ViewFilterOperatorNotEqual    = `ne`
	ViewFilterOperatorGreater     = `gt`
	ViewFilterOperatorGreaterEq   = `gte`
	ViewFilterOperatorLess        = `lt`
	ViewFilterOperatorLessEq      = `lte`
	ViewFilterOperatorIn          = `in`
	ViewFilterOperatorNotIn       = `not_in`
	ViewFilterOperatorBetween     = `between`
	ViewFilterOperatorDynamic     = `dynamic`
	ViewFilterOperatorWithinHours = `within_hours`
	ViewFilterOperatorEmpty       = `empty`
	ViewFilterOperatorNotEmpty    = `not_empty`

	ViewFilterOperatorOlderThanHours = `older_than_hours`
)

// Custom field type constants. Stored in workspace_custom_fields.field_type.
const (
	CustomFieldTypeText           = `text`
	CustomFieldTypeTextarea       = `textarea`
	CustomFieldTypeNumber         = `number`
	CustomFieldTypeNumeric        = `numeric`
	CustomFieldTypePhone          = `phone`
	CustomFieldTypeDate           = `date`
	CustomFieldTypeDateTime       = `datetime`
	CustomFieldTypeSelectSingle   = `select_single`
	CustomFieldTypeSelectMulti    = `select_multi`
	CustomFieldTypeRadio          = `radio`
	CustomFieldTypeCheckbox       = `checkbox`
	CustomFieldTypeUser           = `user`
	CustomFieldTypeUserMulti      = `user_multi`
	CustomFieldTypeCascaderSingle = `cascader_single`
	CustomFieldTypeCascaderMulti  = `cascader_multi`
)

// Member-scope values for user / user_multi custom fields.
const (
	CustomFieldMemberScopeAll       = `all`
	CustomFieldMemberScopeRoles     = `roles`
	CustomFieldMemberScopeWorkspace = `workspace`
)

// CustomFieldTypes is the create/update whitelist, in the order shown in the type dropdown.
// Textarea, radio, and checkbox remain constants for compatibility but cannot be created or updated.
var CustomFieldTypes = []string{
	CustomFieldTypeText,
	CustomFieldTypeNumber,
	CustomFieldTypePhone,
	CustomFieldTypeNumeric,
	CustomFieldTypeDate,
	CustomFieldTypeDateTime,
	CustomFieldTypeSelectSingle,
	CustomFieldTypeSelectMulti,
	CustomFieldTypeUser,
	CustomFieldTypeUserMulti,
	CustomFieldTypeCascaderSingle,
	CustomFieldTypeCascaderMulti,
}

// View-filter aliases keep existing call sites compiling against the stored type strings.
const (
	ViewFilterCustomTypeText           = CustomFieldTypeText
	ViewFilterCustomTypeTextarea       = CustomFieldTypeTextarea
	ViewFilterCustomTypeNumber         = CustomFieldTypeNumber
	ViewFilterCustomTypeNumeric        = CustomFieldTypeNumeric
	ViewFilterCustomTypePhone          = CustomFieldTypePhone
	ViewFilterCustomTypeDate           = CustomFieldTypeDate
	ViewFilterCustomTypeDateTime       = CustomFieldTypeDateTime
	ViewFilterCustomTypeSelectSingle   = CustomFieldTypeSelectSingle
	ViewFilterCustomTypeSelectMulti    = CustomFieldTypeSelectMulti
	ViewFilterCustomTypeRadio          = CustomFieldTypeRadio
	ViewFilterCustomTypeCheckbox       = CustomFieldTypeCheckbox
	ViewFilterCustomTypeUser           = CustomFieldTypeUser
	ViewFilterCustomTypeUserMulti      = CustomFieldTypeUserMulti
	ViewFilterCustomTypeCascaderSingle = CustomFieldTypeCascaderSingle
	ViewFilterCustomTypeCascaderMulti  = CustomFieldTypeCascaderMulti
)

// ViewFilterNode is a recursive filter tree node: a group node carries
// Logic+Conditions, a leaf node carries Field+Operator+Values.
type ViewFilterNode struct {
	Logic      string           `json:"logic,omitempty"`
	Conditions []ViewFilterNode `json:"conditions,omitempty"`
	Field      string           `json:"field,omitempty"`
	Operator   string           `json:"operator,omitempty"`
	Values     []string         `json:"values,omitempty"`
}

// IsGroup reports whether the node is a condition group.
func (n ViewFilterNode) IsGroup() bool {
	return len(n.Field) == 0
}

// GanttViewFilters is the stored filters payload of a gantt view: three
// independent sections applied as common AND (requirement OR defect).
type GanttViewFilters struct {
	Common      *ViewFilterNode `json:"common,omitempty"`
	Requirement *ViewFilterNode `json:"requirement,omitempty"`
	Defect      *ViewFilterNode `json:"defect,omitempty"`
}

// GanttCommonFilterFields whitelists builtin fields usable in the gantt common
// section: only fields whose semantics are identical for both work item types
// (status/priority/category/custom fields are configured per type and stay in
// the type-specific sections).
var GanttCommonFilterFields = map[string]bool{
	ViewFilterFieldID:               true,
	ViewFilterFieldTitle:            true,
	ViewFilterFieldAssignee:         true,
	ViewFilterFieldCreator:          true,
	ViewFilterFieldVersion:          true,
	ViewFilterFieldPlannedStartDate: true,
	ViewFilterFieldPlannedEndDate:   true,
	ViewFilterFieldCreatedAt:        true,
}

// ViewFilterFieldOperators whitelists the operators allowed per filter field.
// The first operator of each field is the frontend default after switching fields.
var ViewFilterFieldOperators = map[string][]string{
	ViewFilterFieldID: {ViewFilterOperatorEqual},
	ViewFilterFieldTemplate: {
		ViewFilterOperatorIn,
		ViewFilterOperatorNotIn,
		ViewFilterOperatorEmpty,
		ViewFilterOperatorNotEmpty,
	},
	ViewFilterFieldTitle:            {ViewFilterOperatorContains, ViewFilterOperatorNotContains, ViewFilterOperatorEmpty, ViewFilterOperatorNotEmpty, ViewFilterOperatorEqual, ViewFilterOperatorNotEqual},
	ViewFilterFieldStatus:           {ViewFilterOperatorIn, ViewFilterOperatorNotIn, ViewFilterOperatorEmpty, ViewFilterOperatorNotEmpty},
	ViewFilterFieldPriority:         {ViewFilterOperatorIn, ViewFilterOperatorNotIn, ViewFilterOperatorEmpty, ViewFilterOperatorNotEmpty},
	ViewFilterFieldAssignmentAge:    {ViewFilterOperatorWithinHours},
	ViewFilterFieldMyAssignmentAge:  {ViewFilterOperatorWithinHours, ViewFilterOperatorOlderThanHours},
	ViewFilterFieldAssignee:         {ViewFilterOperatorContains, ViewFilterOperatorNotContains, ViewFilterOperatorEqual, ViewFilterOperatorNotEqual, ViewFilterOperatorIn, ViewFilterOperatorNotIn, ViewFilterOperatorEmpty, ViewFilterOperatorNotEmpty},
	ViewFilterFieldCreator:          {ViewFilterOperatorContains, ViewFilterOperatorNotContains, ViewFilterOperatorEqual, ViewFilterOperatorNotEqual, ViewFilterOperatorIn, ViewFilterOperatorNotIn, ViewFilterOperatorEmpty, ViewFilterOperatorNotEmpty},
	ViewFilterFieldVersion:          {ViewFilterOperatorIn, ViewFilterOperatorNotIn, ViewFilterOperatorEmpty, ViewFilterOperatorNotEmpty},
	ViewFilterFieldWorkspace:        {ViewFilterOperatorIn, ViewFilterOperatorNotIn},
	ViewFilterFieldWorkItemType:     {ViewFilterOperatorIn, ViewFilterOperatorNotIn},
	ViewFilterFieldCategory:         {ViewFilterOperatorIn, ViewFilterOperatorNotIn, ViewFilterOperatorEmpty, ViewFilterOperatorNotEmpty},
	ViewFilterFieldPlannedStartDate: {ViewFilterOperatorEqual, ViewFilterOperatorBetween, ViewFilterOperatorDynamic, ViewFilterOperatorEmpty, ViewFilterOperatorNotEmpty},
	ViewFilterFieldPlannedEndDate:   {ViewFilterOperatorEqual, ViewFilterOperatorBetween, ViewFilterOperatorDynamic, ViewFilterOperatorEmpty, ViewFilterOperatorNotEmpty},
	ViewFilterFieldCreatedAt:        {ViewFilterOperatorEqual, ViewFilterOperatorBetween, ViewFilterOperatorDynamic, ViewFilterOperatorEmpty, ViewFilterOperatorNotEmpty, ViewFilterOperatorOlderThanHours},
	ViewFilterFieldAgingStatus:      {ViewFilterOperatorEqual, ViewFilterOperatorNotEqual, ViewFilterOperatorIn, ViewFilterOperatorNotIn},
}

var viewFilterTextOperators = map[string]bool{
	ViewFilterOperatorContains: true, ViewFilterOperatorNotContains: true,
	ViewFilterOperatorEqual: true, ViewFilterOperatorNotEqual: true,
}

var viewFilterNumericOperators = map[string]bool{
	ViewFilterOperatorEqual: true, ViewFilterOperatorNotEqual: true,
	ViewFilterOperatorGreater: true, ViewFilterOperatorGreaterEq: true,
	ViewFilterOperatorLess: true, ViewFilterOperatorLessEq: true,
	ViewFilterOperatorBetween: true,
}

var viewFilterDateOperators = map[string]bool{
	ViewFilterOperatorBetween: true,
}

var viewFilterSelectOperators = map[string]bool{
	ViewFilterOperatorIn: true, ViewFilterOperatorNotIn: true,
}

var viewFilterPersonOperators = map[string]bool{
	ViewFilterOperatorContains: true, ViewFilterOperatorNotContains: true,
	ViewFilterOperatorEqual: true, ViewFilterOperatorNotEqual: true,
	ViewFilterOperatorIn: true, ViewFilterOperatorNotIn: true,
}

// ViewFilterCustomTypeOperators whitelists operators for each custom field type.
// empty / not_empty are accepted for every type in validateTypedFilter before this map is consulted.
var ViewFilterCustomTypeOperators = map[string]map[string]bool{
	ViewFilterCustomTypeText:           viewFilterTextOperators,
	ViewFilterCustomTypeTextarea:       viewFilterTextOperators,
	ViewFilterCustomTypePhone:          viewFilterTextOperators,
	ViewFilterCustomTypeNumber:         viewFilterNumericOperators,
	ViewFilterCustomTypeNumeric:        viewFilterNumericOperators,
	ViewFilterCustomTypeDate:           viewFilterDateOperators,
	ViewFilterCustomTypeDateTime:       viewFilterDateOperators,
	ViewFilterCustomTypeSelectSingle:   viewFilterSelectOperators,
	ViewFilterCustomTypeSelectMulti:    viewFilterSelectOperators,
	ViewFilterCustomTypeRadio:          viewFilterSelectOperators,
	ViewFilterCustomTypeCheckbox:       viewFilterSelectOperators,
	ViewFilterCustomTypeUser:           viewFilterPersonOperators,
	ViewFilterCustomTypeUserMulti:      viewFilterPersonOperators,
	ViewFilterCustomTypeCascaderSingle: viewFilterSelectOperators,
	ViewFilterCustomTypeCascaderMulti:  viewFilterSelectOperators,
}

// ViewFilterCustomFieldOperators contains every operator accepted during
// structural validation of a custom field filter.
var ViewFilterCustomFieldOperators = []string{
	ViewFilterOperatorContains,
	ViewFilterOperatorNotContains,
	ViewFilterOperatorEqual,
	ViewFilterOperatorNotEqual,
	ViewFilterOperatorGreater,
	ViewFilterOperatorGreaterEq,
	ViewFilterOperatorLess,
	ViewFilterOperatorLessEq,
	ViewFilterOperatorIn,
	ViewFilterOperatorNotIn,
	ViewFilterOperatorBetween,
	ViewFilterOperatorEmpty,
	ViewFilterOperatorNotEmpty,
}

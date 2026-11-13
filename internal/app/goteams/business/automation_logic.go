package business

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/netip"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const automationSchemaVersion = 1

// automationNodeNameMaxLen caps the user-defined execution node name.
const automationNodeNameMaxLen = 50

// automationDateClear is the fixed date value that clears the target field.
const automationDateClear = "clear"

// automationDateOffsets maps a relative fixed date value to its day offset from the trigger date.
var automationDateOffsets = map[string]int{"today": 0, "tomorrow": 1, "day_after_tomorrow": 2}

var (
	templateVariablePattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)
	// ErrAutomationConfigInvalid means the rule config is invalid.
	ErrAutomationConfigInvalid = errors.New("invalid automation config")
)

// AutomationConfig is the strictly versioned rule config persisted to config_json.
type AutomationConfig struct {
	SchemaVersion int                  `json:"schema_version"`     // config schema version
	Timezone      string               `json:"timezone,omitempty"` // IANA timezone of the user who configured the rule
	Trigger       AutomationTrigger    `json:"trigger"`            // trigger config
	Nodes         []AutomationFlowNode `json:"nodes"`              // sequentially executed nodes
}

// AutomationTrigger defines the work item trigger conditions.
type AutomationTrigger struct {
	Object        string          `json:"object"`                   // trigger object type
	Event         string          `json:"event"`                    // trigger event type
	CommentChange string          `json:"comment_change,omitempty"` // comment change type
	FromStatusID  int             `json:"from_status_id,omitempty"` // from status id
	ToStatusID    int             `json:"to_status_id,omitempty"`   // to status id
	FieldKey      string          `json:"field_key,omitempty"`      // watched field key
	FieldChange   string          `json:"field_change,omitempty"`   // field change pattern
	FieldValue    json.RawMessage `json:"field_value,omitempty"`    // field comparison value
}

// AutomationFlowNode defines a direct or conditional execution node.
type AutomationFlowNode struct {
	Name           string                      `json:"name,omitempty"`            // node display name shown in the rule editor
	Mode           string                      `json:"mode"`                      // direct or conditional
	Action         *AutomationAction           `json:"action,omitempty"`          // direct action to execute
	ModuleRelation string                      `json:"module_relation,omitempty"` // condition module relation
	Modules        []AutomationConditionModule `json:"modules,omitempty"`         // condition modules
	Actions        []AutomationAction          `json:"actions,omitempty"`         // actions to run when conditions are met
	ElseIf         []AutomationConditionBranch `json:"else_if,omitempty"`         // ordered branches checked after the primary condition
}

// AutomationConditionBranch defines one ordered conditional branch and its actions.
type AutomationConditionBranch struct {
	ModuleRelation string                      `json:"module_relation"` // condition module relation
	Modules        []AutomationConditionModule `json:"modules"`         // condition modules
	Actions        []AutomationAction          `json:"actions"`         // actions to run when conditions are met
}

// AutomationConditionModule is a non-recursive field condition module.
type AutomationConditionModule struct {
	Relation   string                `json:"relation"`   // relation between conditions in a module
	Conditions []AutomationCondition `json:"conditions"` // condition list
}

// AutomationCondition defines a typed field condition.
type AutomationCondition struct {
	FieldKey      string          `json:"field_key"`                 // condition field key
	Operator      string          `json:"operator"`                  // comparison operator
	Value         json.RawMessage `json:"value,omitempty"`           // comparison value
	SkipWhenEmpty bool            `json:"skip_when_empty,omitempty"` // skip the condition when the value is empty
}

// AutomationAction defines one of the six action types.
type AutomationAction struct {
	Type              string                       `json:"type"`                          // action type
	Scope             string                       `json:"scope,omitempty"`               // trigger or parent
	Transitions       []AutomationStatusTransition `json:"transitions,omitempty"`         // status transition list
	Updates           []AutomationFieldUpdate      `json:"updates,omitempty"`             // field update list
	Template          string                       `json:"template,omitempty"`            // text template, also the robot text message
	Parent            string                       `json:"parent,omitempty"`              // parent requirement setting for the new work item
	Description       string                       `json:"description,omitempty"`         // description template of the created requirement
	Status            *AutomationCreateField       `json:"status,omitempty"`              // status of the created requirement
	Priority          *AutomationCreateField       `json:"priority,omitempty"`            // priority of the created requirement
	Version           *AutomationCreateField       `json:"version,omitempty"`             // version of the created requirement
	Assignee          *AutomationCreateField       `json:"assignee,omitempty"`            // assignees of the created requirement
	PlannedStart      string                       `json:"planned_start,omitempty"`       // planned start keyword: trigger, today, tomorrow or day_after_tomorrow
	PlannedEnd        string                       `json:"planned_end,omitempty"`         // planned end keyword: trigger, today, tomorrow or day_after_tomorrow
	TargetWorkspaceID int                          `json:"target_workspace_id,omitempty"` // copy target workspace id
	PushSystem        string                       `json:"push_system,omitempty"`         // custom, dingtalk, feishu or wecom
	URL               string                       `json:"url,omitempty"`                 // webhook URL template
	Body              string                       `json:"body,omitempty"`                // custom webhook request body template
	Secret            string                       `json:"secret,omitempty"`              // robot signing secret, dingtalk and feishu only
	AtUserIds         []int                        `json:"at_user_ids,omitempty"`         // explicitly mentioned member ids
	AtRoles           []string                     `json:"at_roles,omitempty"`            // mentioned trigger roles: created_by or assignee
	AtAll             bool                         `json:"at_all,omitempty"`              // mention everyone in the robot group
	AtMode            string                       `json:"at_mode,omitempty"`             // robot parameter carrying the mention, see automationAtModes
}

// AutomationCreateField configures one optional field of a created requirement.
// The value comes from the trigger work item or is a fixed reference id.
type AutomationCreateField struct {
	Source string `json:"source"`           // trigger or fixed
	Value  int    `json:"value,omitempty"`  // fixed status, priority or version id
	Values []int  `json:"values,omitempty"` // fixed assignee member ids
}

// AutomationStatusTransition defines a single status transition within a node.
type AutomationStatusTransition struct {
	Scope          string `json:"scope"`            // trigger or parent
	TargetStatusID int    `json:"target_status_id"` // to status id
}

// AutomationFieldUpdate defines a single field update within a node.
type AutomationFieldUpdate struct {
	FieldKey       string          `json:"field_key"`                  // target field key
	ValueSource    string          `json:"value_source"`               // fixed, trigger, parent or child
	SourceFieldKey string          `json:"source_field_key,omitempty"` // source field key
	Value          json.RawMessage `json:"value,omitempty"`            // fixed value
}

// AutomationFieldMeta describes a field available for rule configuration.
type AutomationFieldMeta struct {
	MemberScope string
	RoleIds     []int
	Key         string              `json:"key"`               // stable field key
	Label       string              `json:"label"`             // display name
	Type        string              `json:"type"`              // field type
	Editable    bool                `json:"editable"`          // whether automatic updates are allowed
	Deleted     bool                `json:"deleted,omitempty"` // whether the reference is stale
	Options     []CustomFieldOption `json:"options,omitempty"` // available options
}

// AutomationLog is a structured log entry in an execution.
type AutomationLog struct {
	Time       int64          `json:"time"`                 // log timestamp
	Key        string         `json:"key"`                  // i18n log key
	Node       int            `json:"node,omitempty"`       // node index
	Attempt    int            `json:"attempt,omitempty"`    // attempt count
	Parameters map[string]any `json:"parameters,omitempty"` // structured parameters
	Raw        string         `json:"raw,omitempty"`        // raw third-party response or error
}

// AutomationNodeProgress stores completed nodes so lease recovery can resume from checkpoints.
type AutomationNodeProgress struct {
	Completed  []int `json:"completed"`   // completed node indices
	AnySuccess bool  `json:"any_success"` // whether at least one node succeeded
}

// decodeAutomationConfig strictly parses the config and rejects unknown fields and trailing content.
func decodeAutomationConfig(data []byte) (AutomationConfig, error) {
	var config AutomationConfig
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return config, fmt.Errorf("%w: %v", ErrAutomationConfigInvalid, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return config, ErrAutomationConfigInvalid
	}
	if err := validateAutomationConfigShape(config); err != nil {
		return config, err
	}
	return config, nil
}

// validateAutomationConfigShape validates the overall rule structure and node count.
func validateAutomationConfigShape(config AutomationConfig) error {
	invalid := func(message string) error { return fmt.Errorf("%w: %s", ErrAutomationConfigInvalid, message) }
	if config.SchemaVersion != automationSchemaVersion {
		return invalid("unsupported schema version")
	}
	if len(config.Timezone) > 100 {
		return invalid("timezone is too long")
	}
	if config.Timezone != "" {
		if _, err := time.LoadLocation(config.Timezone); err != nil {
			return invalid("unknown timezone")
		}
	}
	if config.Trigger.Object != "requirement" && config.Trigger.Object != "defect" {
		return invalid("unknown trigger object")
	}
	switch config.Trigger.Event {
	case "create":
	case "comment":
		if !oneOf(config.Trigger.CommentChange, "any", "add", "edit", "delete") {
			return invalid("unknown comment trigger")
		}
	case "status":
		if config.Trigger.ToStatusID <= 0 {
			return invalid("target status is required")
		}
	case "field":
		if config.Trigger.FieldKey == "" || !oneOf(config.Trigger.FieldChange, "any", "from", "to") {
			return invalid("field trigger is incomplete")
		}
		if config.Trigger.FieldChange != "any" && len(config.Trigger.FieldValue) == 0 {
			return invalid("field trigger value is required")
		}
	default:
		return invalid("unknown trigger event")
	}
	if len(config.Nodes) == 0 {
		return invalid("at least one node is required")
	}
	for _, node := range config.Nodes {
		if len([]rune(node.Name)) > automationNodeNameMaxLen {
			return invalid("node name is too long")
		}
		switch node.Mode {
		case "direct":
			if node.Action == nil || node.ModuleRelation != "" || len(node.Actions) != 0 || len(node.Modules) != 0 || len(node.ElseIf) != 0 {
				return invalid("direct node is incomplete")
			}
			if err := validateAutomationAction(*node.Action); err != nil {
				return err
			}
		case "conditional":
			if node.Action != nil {
				return invalid("conditional node is incomplete")
			}
			for _, branch := range automationConditionBranches(node) {
				if err := validateAutomationConditionBranch(branch); err != nil {
					return err
				}
			}
		default:
			return invalid("unknown node mode")
		}
	}
	return nil
}

// automationConditionBranches returns the primary condition followed by ordered else-if branches.
func automationConditionBranches(node AutomationFlowNode) []AutomationConditionBranch {
	branches := make([]AutomationConditionBranch, 0, len(node.ElseIf)+1)
	branches = append(branches, AutomationConditionBranch{
		ModuleRelation: node.ModuleRelation,
		Modules:        node.Modules,
		Actions:        node.Actions,
	})
	return append(branches, node.ElseIf...)
}

// validateAutomationConditionBranch validates one condition branch and its actions.
func validateAutomationConditionBranch(branch AutomationConditionBranch) error {
	invalid := func(message string) error { return fmt.Errorf("%w: %s", ErrAutomationConfigInvalid, message) }
	if !oneOf(branch.ModuleRelation, "AND", "OR") || len(branch.Modules) == 0 || len(branch.Actions) == 0 {
		return invalid("conditional branch is incomplete")
	}
	for _, module := range branch.Modules {
		if !oneOf(module.Relation, "AND", "OR") || len(module.Conditions) == 0 {
			return invalid("condition module is incomplete")
		}
		for _, condition := range module.Conditions {
			if condition.FieldKey == "" || condition.Operator == "" {
				return invalid("condition is incomplete")
			}
			if !oneOf(condition.Operator, "contains", "not_contains", "is", "is_not", "empty", "not_empty", "eq", "neq", "gt", "gte", "lt", "lte", "before", "before_eq", "after", "after_eq", "in", "not_in", "contains_any", "contains_all", "contains_none") {
				return invalid("unknown condition operator")
			}
			if condition.Operator != "empty" && condition.Operator != "not_empty" && len(condition.Value) == 0 {
				return invalid("condition value is required")
			}
		}
	}
	for _, action := range branch.Actions {
		if err := validateAutomationAction(action); err != nil {
			return err
		}
	}
	return nil
}

// validateAutomationAction validates the parameters required by each of the six action types.
func validateAutomationAction(action AutomationAction) error {
	invalid := func(message string) error { return fmt.Errorf("%w: %s", ErrAutomationConfigInvalid, message) }
	switch action.Type {
	case "status_transition":
		if len(action.Transitions) == 0 {
			return invalid("status transition is empty")
		}
		for _, item := range action.Transitions {
			if !validAutomationScope(item.Scope) || item.TargetStatusID <= 0 {
				return invalid("status transition is incomplete")
			}
		}
	case "field_update":
		if !validAutomationScope(action.Scope) || len(action.Updates) == 0 {
			return invalid("field update is incomplete")
		}
		for _, item := range action.Updates {
			if item.FieldKey == "" || !oneOf(item.ValueSource, "fixed", "trigger", "parent", "child") {
				return invalid("field update value is incomplete")
			}
			if item.ValueSource == "fixed" && len(item.Value) == 0 {
				return invalid("fixed value is required")
			}
			if item.ValueSource != "fixed" && item.SourceFieldKey == "" {
				return invalid("source field is required")
			}
		}
	case "add_comment":
		if !validAutomationScope(action.Scope) || strings.TrimSpace(action.Template) == "" {
			return invalid("comment action is incomplete")
		}
	case "create_requirement":
		if !oneOf(action.Parent, "trigger", "empty") || strings.TrimSpace(action.Template) == "" {
			return invalid("create action is incomplete")
		}
		for _, field := range []*AutomationCreateField{action.Status, action.Priority, action.Version} {
			if field == nil {
				continue
			}
			if !oneOf(field.Source, "trigger", "fixed") || len(field.Values) != 0 || (field.Source == "fixed") != (field.Value > 0) {
				return invalid("create field is incomplete")
			}
		}
		if assignee := action.Assignee; assignee != nil {
			if !oneOf(assignee.Source, "trigger", "fixed") || assignee.Value != 0 || (assignee.Source == "fixed") != (len(assignee.Values) > 0) {
				return invalid("create assignee is incomplete")
			}
			for _, userID := range assignee.Values {
				if userID <= 0 {
					return invalid("create assignee is incomplete")
				}
			}
		}
		for _, keyword := range []string{action.PlannedStart, action.PlannedEnd} {
			if keyword != "" && !oneOf(keyword, "trigger", "today", "tomorrow", "day_after_tomorrow") {
				return invalid("create date is incomplete")
			}
		}
	case "copy_work_item":
		if action.TargetWorkspaceID <= 0 || !oneOf(action.Parent, "trigger", "empty") {
			return invalid("copy action is incomplete")
		}
	case "webhook":
		return validateAutomationWebhookAction(action)
	default:
		return invalid("unknown action type")
	}
	return nil
}

// validateAutomationWebhookAction validates the push system and its exclusive parameters.
func validateAutomationWebhookAction(action AutomationAction) error {
	invalid := func(message string) error { return fmt.Errorf("%w: %s", ErrAutomationConfigInvalid, message) }
	if !oneOf(action.PushSystem, "custom", "dingtalk", "feishu", "wecom") {
		return invalid("unknown push system")
	}
	if strings.TrimSpace(action.URL) == "" {
		return invalid("webhook action is incomplete")
	}
	if action.PushSystem == "custom" {
		if strings.TrimSpace(action.Body) == "" {
			return invalid("webhook body is required")
		}
		if action.Secret != "" || action.Template != "" || action.AtMode != "" || action.AtAll || len(action.AtUserIds) != 0 || len(action.AtRoles) != 0 {
			return invalid("custom webhook does not accept robot parameters")
		}
		return nil
	}
	if strings.TrimSpace(action.Template) == "" {
		return invalid("robot text message is required")
	}
	if action.Body != "" {
		return invalid("robot webhook does not accept a custom body")
	}
	if action.PushSystem == "wecom" && action.Secret != "" {
		return invalid("wecom robot does not support signing")
	}
	if !oneOf(action.AtMode, automationAtModes(action.PushSystem)...) {
		return invalid("unknown at mode")
	}
	for _, role := range action.AtRoles {
		if !oneOf(role, "created_by", "assignee") {
			return invalid("unknown at role")
		}
	}
	for _, userID := range action.AtUserIds {
		if userID <= 0 {
			return invalid("invalid at user")
		}
	}
	return nil
}

// automationAtModes returns the mention parameters a push system accepts.
// Each value is an original robot API parameter; mobile reads the member phone directly.
func automationAtModes(pushSystem string) []string {
	switch pushSystem {
	case "dingtalk":
		return []string{"mobile", "at_mobiles", "at_user_ids"}
	case "feishu":
		return []string{"user_id"}
	case "wecom":
		return []string{"mobile", "mentioned_list", "mentioned_mobile_list"}
	default:
		return nil
	}
}

// validAutomationScope reports whether an action scope is supported.
func validAutomationScope(scope string) bool {
	return oneOf(scope, "trigger", "parent", "descendants")
}

// oneOf reports whether a string belongs to the given set.
func oneOf(value string, values ...string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

// rawJSONValue decodes an optional JSON value into a condition comparison value.
func rawJSONValue(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	return value
}

// automationValueEmpty reports whether a field value is empty.
func automationValueEmpty(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return typed == ""
	case []string:
		return len(typed) == 0
	case []any:
		return len(typed) == 0
	default:
		return false
	}
}

// automationValuesEqual compares numbers, sets and plain JSON values.
func automationValuesEqual(left, right any) bool {
	leftNumber, leftNumberOK := automationEqualityNumber(left)
	rightNumber, rightNumberOK := automationEqualityNumber(right)
	if leftNumberOK && rightNumberOK {
		return leftNumber == rightNumber
	}
	leftSet, leftOK := automationStringSet(left)
	rightSet, rightOK := automationStringSet(right)
	if leftOK && rightOK {
		if len(leftSet) != len(rightSet) {
			return false
		}
		for key := range leftSet {
			if !rightSet[key] {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(left, right)
}

// automationStringSet converts a multi-select value into an unordered set.
func automationStringSet(value any) (map[string]bool, bool) {
	result := map[string]bool{}
	switch typed := value.(type) {
	case []string:
		for _, item := range typed {
			result[item] = true
		}
	case []any:
		for _, item := range typed {
			switch value := item.(type) {
			case string:
				result[value] = true
			case float64:
				result[strconv.FormatFloat(value, 'f', -1, 64)] = true
			default:
				return nil, false
			}
		}
	default:
		return nil, false
	}
	return result, true
}

// matchAutomationFieldChange reports whether a field change matches the trigger pattern.
func matchAutomationFieldChange(change string, expected, before, after any) bool {
	if automationValueInvalid(before) || automationValueInvalid(after) {
		return false
	}
	if automationValuesEqual(before, after) {
		return false
	}
	switch change {
	case "any":
		return true
	case "from":
		return automationTriggerValuesEqual(before, expected)
	case "to":
		return automationTriggerValuesEqual(after, expected)
	default:
		return false
	}
}

// evaluateAutomationCondition evaluates a single condition by operator.
func evaluateAutomationCondition(fieldType string, condition AutomationCondition, actual any) (matched, skipped bool) {
	if automationValueInvalid(actual) {
		return false, false
	}
	if condition.SkipWhenEmpty && automationValueEmpty(actual) {
		return false, true
	}
	expected := rawJSONValue(condition.Value)
	// 日期时间保留文本存储，仅在比较时解析；空值判断不受日期解析影响。
	if fieldType == "datetime" && !oneOf(condition.Operator, "empty", "not_empty") {
		actualText, actualOK := actual.(string)
		expectedText, expectedOK := expected.(string)
		left, leftOK := normalizeAutomationDatetime(actualText)
		right, rightOK := normalizeAutomationDatetime(expectedText)
		if !actualOK || !expectedOK || !leftOK || !rightOK {
			return false, false
		}
		leftTime, err := time.Parse("2006-01-02 15:04:05", left)
		if err != nil {
			return false, false
		}
		rightTime, err := time.Parse("2006-01-02 15:04:05", right)
		if err != nil {
			return false, false
		}
		actual, expected = leftTime.Unix(), rightTime.Unix()
	}
	switch condition.Operator {
	case "empty":
		return automationValueEmpty(actual), false
	case "not_empty":
		return !automationValueEmpty(actual), false
	case "is", "eq", "is_not", "neq":
		equal := automationValuesEqual(actual, expected)
		if fieldType == "number" || fieldType == "numeric" {
			left, leftOK := automationNumber(actual)
			right, rightOK := automationNumber(expected)
			if !leftOK || !rightOK {
				return false, false
			}
			equal = left == right
		}
		if condition.Operator == "is_not" || condition.Operator == "neq" {
			equal = !equal
		}
		return equal, false
	case "contains", "not_contains":
		contains := strings.Contains(fmt.Sprint(actual), fmt.Sprint(expected))
		if condition.Operator == "not_contains" {
			contains = !contains
		}
		return contains, false
	case "gt", "gte", "lt", "lte", "before", "before_eq", "after", "after_eq":
		left, leftOK := automationNumber(actual)
		right, rightOK := automationNumber(expected)
		if !leftOK || !rightOK {
			return false, false
		}
		switch condition.Operator {
		case "gt", "after":
			return left > right, false
		case "gte", "after_eq":
			return left >= right, false
		case "lt", "before":
			return left < right, false
		default:
			return left <= right, false
		}
	case "in", "not_in":
		values, ok := automationStringSet(expected)
		if !ok {
			return false, false
		}
		matched := values[fmt.Sprint(actual)]
		if condition.Operator == "not_in" {
			matched = !matched
		}
		return matched, false
	case "contains_any", "contains_all", "contains_none":
		actualSet, actualOK := automationStringSet(actual)
		expectedSet, expectedOK := automationStringSet(expected)
		if !actualOK || !expectedOK {
			return false, false
		}
		count := 0
		for item := range expectedSet {
			if actualSet[item] {
				count++
			}
		}
		switch condition.Operator {
		case "contains_any":
			return count > 0, false
		case "contains_all":
			return count == len(expectedSet), false
		default:
			return count == 0, false
		}
	}
	return false, false
}

// automationEqualityNumber unifies the different number types produced by JSON decoding for comparison.
func automationEqualityNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}

// automationNumber converts a number or numeric text into a float.
func automationNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	case string:
		number, err := strconv.ParseFloat(typed, 64)
		return number, err == nil
	default:
		return 0, false
	}
}

// evaluateAutomationBranches returns the first matching branch and fields skipped before that match.
func evaluateAutomationBranches(node AutomationFlowNode, fields map[string]any, types map[string]string) (int, []string) {
	skippedFields := []string{}
	for index, branch := range automationConditionBranches(node) {
		matched, skipped := evaluateAutomationModules(branch, fields, types)
		skippedFields = append(skippedFields, skipped...)
		if matched {
			return index, skippedFields
		}
	}
	return -1, skippedFields
}

// evaluateAutomationModules evaluates one branch's modules and records fields skipped due to empty values.
func evaluateAutomationModules(branch AutomationConditionBranch, fields map[string]any, types map[string]string) (bool, []string) {
	moduleResults := make([]bool, 0, len(branch.Modules))
	skippedFields := []string{}
	for _, module := range branch.Modules {
		conditionResults := []bool{}
		for _, condition := range module.Conditions {
			matched, skipped := evaluateAutomationCondition(types[condition.FieldKey], condition, fields[condition.FieldKey])
			if skipped {
				skippedFields = append(skippedFields, condition.FieldKey)
				continue
			}
			conditionResults = append(conditionResults, matched)
		}
		if len(conditionResults) == 0 {
			moduleResults = append(moduleResults, false)
			continue
		}
		moduleResults = append(moduleResults, combineAutomationResults(module.Relation, conditionResults))
	}
	return combineAutomationResults(branch.ModuleRelation, moduleResults), skippedFields
}

// combineAutomationResults combines condition results by AND or OR.
func combineAutomationResults(relation string, values []bool) bool {
	if len(values) == 0 {
		return false
	}
	if relation == "OR" {
		for _, value := range values {
			if value {
				return true
			}
		}
		return false
	}
	for _, value := range values {
		if !value {
			return false
		}
	}
	return true
}

// validateTemplateVariables validates that a template only references allowed variables.
func validateTemplateVariables(template string, allowed map[string]bool) error {
	for _, match := range templateVariablePattern.FindAllStringSubmatch(template, -1) {
		if !allowed[match[1]] {
			return fmt.Errorf("%w: unknown template variable %s", ErrAutomationConfigInvalid, match[1])
		}
	}
	withoutKnown := templateVariablePattern.ReplaceAllString(template, "")
	if strings.Contains(withoutKnown, "{{") || strings.Contains(withoutKnown, "}}") {
		return fmt.Errorf("%w: malformed template", ErrAutomationConfigInvalid)
	}
	return nil
}

// renderAutomationTemplate replaces template variables with event snapshot values.
func renderAutomationTemplate(template string, values map[string]string) string {
	return templateVariablePattern.ReplaceAllStringFunc(template, func(token string) string {
		match := templateVariablePattern.FindStringSubmatch(token)
		return values[match[1]]
	})
}

// validateWebhookURLTemplate validates the webhook template structure, scheme and static host.
func validateWebhookURLTemplate(raw string, allowed map[string]bool) error {
	if err := validateTemplateVariables(raw, allowed); err != nil {
		return err
	}
	firstVariable := strings.Index(raw, "{{")
	schemeIndex := strings.Index(raw, "://")
	if schemeIndex < 0 {
		return fmt.Errorf("%w: invalid webhook URL", ErrAutomationConfigInvalid)
	}
	authorityEnd := len(raw)
	if slash := strings.Index(raw[schemeIndex+3:], "/"); slash >= 0 {
		authorityEnd = schemeIndex + 3 + slash
	}
	if query := strings.Index(raw, "?"); query >= 0 && query < authorityEnd {
		authorityEnd = query
	}
	if firstVariable >= 0 && firstVariable < authorityEnd {
		return fmt.Errorf("%w: URL authority cannot contain variables", ErrAutomationConfigInvalid)
	}
	replaced := templateVariablePattern.ReplaceAllString(raw, "placeholder")
	parsed, err := url.Parse(replaced)
	if err != nil || !oneOf(parsed.Scheme, "http", "https") || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return fmt.Errorf("%w: invalid webhook URL", ErrAutomationConfigInvalid)
	}
	if strings.EqualFold(parsed.Hostname(), "localhost") {
		return fmt.Errorf("%w: localhost is not allowed", ErrAutomationConfigInvalid)
	}
	if ip := net.ParseIP(parsed.Hostname()); ip != nil && !publicAutomationIP(ip) {
		return fmt.Errorf("%w: private address is not allowed", ErrAutomationConfigInvalid)
	}
	queryIndex := strings.Index(raw, "?")
	if queryIndex >= 0 {
		for _, pair := range strings.Split(raw[queryIndex+1:], "&") {
			key, _, _ := strings.Cut(pair, "=")
			if strings.Contains(key, "{{") {
				return fmt.Errorf("%w: query key cannot contain variables", ErrAutomationConfigInvalid)
			}
		}
	}
	return nil
}

// renderWebhookURL encodes variable values and produces the final webhook URL.
func renderWebhookURL(raw string, values map[string]string) (string, error) {
	queryIndex := strings.Index(raw, "?")
	base := raw
	rawQuery := ""
	if queryIndex >= 0 {
		base, rawQuery = raw[:queryIndex], raw[queryIndex+1:]
	}
	schemeIndex := strings.Index(base, "://")
	if schemeIndex < 0 {
		return "", ErrAutomationConfigInvalid
	}
	pathIndex := strings.Index(base[schemeIndex+3:], "/")
	authority := base
	path := ""
	if pathIndex >= 0 {
		pathIndex += schemeIndex + 3
		authority, path = base[:pathIndex], base[pathIndex:]
	}
	segments := strings.Split(path, "/")
	for index, segment := range segments {
		segments[index] = templateVariablePattern.ReplaceAllStringFunc(segment, func(token string) string {
			match := templateVariablePattern.FindStringSubmatch(token)
			return url.PathEscape(values[match[1]])
		})
	}
	result := authority + strings.Join(segments, "/")
	if rawQuery == "" {
		return result, nil
	}
	pairs := strings.Split(rawQuery, "&")
	for index, pair := range pairs {
		key, value, hasValue := strings.Cut(pair, "=")
		value = templateVariablePattern.ReplaceAllStringFunc(value, func(token string) string {
			match := templateVariablePattern.FindStringSubmatch(token)
			return url.QueryEscape(values[match[1]])
		})
		if hasValue {
			pairs[index] = key + "=" + value
		}
	}
	return result + "?" + strings.Join(pairs, "&"), nil
}

// automationReservedPrefixes lists reserved ranges that GlobalUnicast may still contain.
var automationReservedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}

// publicAutomationIP reports whether an address is allowed for webhook outbound connections.
func publicAutomationIP(ip net.IP) bool {
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsUnspecified() {
		return false
	}
	for _, prefix := range automationReservedPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

// validateAutomationHost resolves a domain name and rejects any non-public result.
func validateAutomationHost(host string, lookup func(string) ([]net.IP, error)) error {
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return errors.New("localhost is not allowed")
	}
	ips, err := lookup(host)
	if err != nil {
		return err
	}
	if len(ips) == 0 {
		return errors.New("host has no address")
	}
	for _, ip := range ips {
		if !publicAutomationIP(ip) {
			return errors.New("private address is not allowed")
		}
	}
	return nil
}

// sortedAutomationKeys returns the template variable keys in stable sorted order.
func sortedAutomationKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// automationDatetimeValueValid 保持触发器值与自定义日期时间字段的秒级文本格式一致。
func automationDatetimeValueValid(raw json.RawMessage) bool {
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return false
	}
	const layout = "2006-01-02 15:04:05"
	parsed, err := time.Parse(layout, value)
	return err == nil && parsed.Format(layout) == value
}

// automationValueInvalid 保留解析失败标记，经过事件 JSON 序列化后仍不能当成空值或复制源。
func automationValueInvalid(value any) bool {
	marker, ok := value.(map[string]any)
	return ok && marker["automation_invalid"] == true
}

// normalizeAutomationDatetime 仅为格式明确的历史分钟值补秒，保留原始墙上时间。
func normalizeAutomationDatetime(value string) (string, bool) {
	if len(value) == 16 {
		value += ":00"
	}
	const layout = "2006-01-02 15:04:05"
	parsed, err := time.Parse(layout, value)
	return value, err == nil && parsed.Format(layout) == value
}

// automationSnapshotFieldValue 区分空值、有效值和解析失败，避免异常字段阻断普通业务。
func automationSnapshotFieldValue(fieldType, text string) (any, error) {
	if text == "" {
		switch fieldType {
		case "select_multi", "user_multi":
			return []string{}, nil
		case "number", "numeric", "date", "user":
			return nil, nil
		default:
			return text, nil
		}
	}
	switch fieldType {
	case "select_multi", "user_multi":
		return decodeStringSlice(text)
	case "number", "numeric":
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil, err
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, ErrAutomationConfigInvalid
		}
		return value, nil
	case "user":
		value, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return nil, err
		}
		if value <= 0 {
			return nil, ErrAutomationConfigInvalid
		}
		return value, nil
	case "date":
		if value, err := time.Parse("2006-01-02", text); err == nil {
			return value.Unix(), nil
		}
		return strconv.ParseInt(text, 10, 64)
	default:
		return text, nil
	}
}

// normalizeAutomationSnapshotMemberValue 将旧任务的多成员文本恢复为新版数组，异常值沿用隔离标记。
func normalizeAutomationSnapshotMemberValue(fields map[string]any, key string) {
	text, legacy := fields[key].(string)
	if !legacy {
		return
	}
	value, err := automationSnapshotFieldValue("user_multi", text)
	if err != nil {
		fields[key] = map[string]any{"automation_invalid": true}
		return
	}
	fields[key] = value
}

// automationTriggerValuesEqual 兼容历史数值触发器的文本值，文本字段仍按文本匹配。
func automationTriggerValuesEqual(actual, expected any) bool {
	if _, numeric := automationEqualityNumber(actual); numeric {
		left, leftOK := automationNumber(actual)
		right, rightOK := automationNumber(expected)
		return leftOK && rightOK && left == right
	}
	return automationValuesEqual(actual, expected)
}

// automationHasCascaderJudgment 检查触发器与所有分支，禁止历史配置落入其他分支。
func automationHasCascaderJudgment(config AutomationConfig, fields []AutomationFieldMeta) bool {
	types := make(map[string]string, len(fields))
	for _, field := range fields {
		types[field.Key] = field.Type
	}
	if config.Trigger.Event == "field" && customFieldIsCascader(types[config.Trigger.FieldKey]) {
		return true
	}
	for _, node := range config.Nodes {
		if node.Mode != "conditional" {
			continue
		}
		for _, branch := range automationConditionBranches(node) {
			for _, module := range branch.Modules {
				for _, condition := range module.Conditions {
					if customFieldIsCascader(types[condition.FieldKey]) {
						return true
					}
				}
			}
		}
	}
	return false
}

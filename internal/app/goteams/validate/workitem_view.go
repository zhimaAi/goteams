// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package validate

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"goteams/internal/app/goteams/define"
)

// ======================== Work Item View Validation ========================

var (
	ErrViewNameInvalid      = fmt.Errorf(`view name must be %d-%d characters`, define.ViewNameMinLength, define.ViewNameMaxLength)
	ErrViewTypeInvalid      = errors.New(`view type must be system or personal`)
	ErrViewFiltersInvalid   = errors.New(`view filters structure is invalid`)
	ErrSearchFiltersInvalid = errors.New(`search filters structure is invalid`)
)

// AllowedViewContextTypes are the view namespace keys: both work item types plus
// the iteration list context and the gantt module.
var AllowedViewContextTypes = []string{define.TypeKeyRequirement, define.TypeKeyDefect, define.TypeKeyIteration, define.TypeKeyGantt}

// ValidateViewContextType checks a view namespace key (requirement/defect/iteration/gantt).
func ValidateViewContextType(typeKey string) error {
	// IsInList returns *FieldError; assign and nil-check before returning so a
	// typed nil pointer never gets boxed into a non-nil error interface.
	if fe := IsInList("type", typeKey, AllowedViewContextTypes); fe != nil {
		return fe
	}
	return nil
}

// ValidateViewName checks the view name length (runes, not bytes).
func ValidateViewName(name string) error {
	length := utf8.RuneCountInString(name)
	if length < define.ViewNameMinLength || length > define.ViewNameMaxLength {
		return ErrViewNameInvalid
	}
	return nil
}

// ValidateViewType checks the view type enum.
func ValidateViewType(viewType string) error {
	if viewType != define.ViewTypeSystem && viewType != define.ViewTypePersonal {
		return ErrViewTypeInvalid
	}
	return nil
}

// ValidateGanttCommonFilterFields rejects builtin fields outside the shared
// whitelist (and any custom field) inside the gantt common filter section.
func ValidateGanttCommonFilterFields(root define.ViewFilterNode) error {
	if root.IsGroup() {
		for _, child := range root.Conditions {
			if err := ValidateGanttCommonFilterFields(child); err != nil {
				return err
			}
		}
		return nil
	}
	if !define.GanttCommonFilterFields[root.Field] {
		return ErrViewFiltersInvalid
	}
	return nil
}

// ValidateViewFilters checks a parsed filter tree: whitelisted fields/operators,
// bounded depth, node count, and value sizes. The root must be a group node.
func ValidateViewFilters(root define.ViewFilterNode) error {
	if !root.IsGroup() {
		return ErrViewFiltersInvalid
	}
	nodeCount := 0
	return validateViewFilterNode(root, 1, &nodeCount)
}

// ValidateViewFiltersForType rejects fields that are unavailable for a work item type.
func ValidateViewFiltersForType(root define.ViewFilterNode, typeKey string) error {
	if err := ValidateViewFilters(root); err != nil {
		return err
	}
	if typeKey == define.TypeKeyDefect && viewFilterContainsField(root, define.ViewFilterFieldCategory) {
		return ErrViewFiltersInvalid
	}
	if typeKey != define.TypeKeyRequirement && (viewFilterContainsField(root, define.ViewFilterFieldUrgeStatus) || viewFilterContainsField(root, define.ViewFilterFieldSLAStatus)) {
		return ErrViewFiltersInvalid
	}
	return nil
}

// ValidateSearchFilters checks the flat, ordered filter set used by the board search bar.
func ValidateSearchFilters(root define.ViewFilterNode, typeKey string) error {
	if err := ValidateViewFiltersForType(root, typeKey); err != nil {
		return ErrSearchFiltersInvalid
	}
	if root.Logic != `` && root.Logic != define.ViewFilterLogicAnd {
		return ErrSearchFiltersInvalid
	}
	if len(root.Conditions) == 0 || root.Conditions[0].Field != define.ViewFilterFieldTitle {
		return ErrSearchFiltersInvalid
	}
	seen := make(map[string]bool, len(root.Conditions))
	for _, node := range root.Conditions {
		if node.IsGroup() || seen[node.Field] {
			return ErrSearchFiltersInvalid
		}
		seen[node.Field] = true
		// Search rows use the same field-specific operators as saved views.
		// Their UI remains flat AND and keeps title as the first protected row.
	}
	return nil
}

func viewFilterContainsField(node define.ViewFilterNode, field string) bool {
	if !node.IsGroup() {
		return node.Field == field
	}
	for _, child := range node.Conditions {
		if viewFilterContainsField(child, field) {
			return true
		}
	}
	return false
}

func validateViewFilterNode(node define.ViewFilterNode, depth int, nodeCount *int) error {
	if depth > define.ViewFilterValidateMaxDepth {
		return ErrViewFiltersInvalid
	}
	*nodeCount++
	if *nodeCount > define.ViewFilterMaxNodes {
		return ErrViewFiltersInvalid
	}
	if node.IsGroup() {
		if node.Logic != `` && node.Logic != define.ViewFilterLogicAnd && node.Logic != define.ViewFilterLogicOr {
			return ErrViewFiltersInvalid
		}
		// A group node must not carry leaf fields.
		if node.Operator != `` || len(node.Values) > 0 {
			return ErrViewFiltersInvalid
		}
		for _, child := range node.Conditions {
			if err := validateViewFilterNode(child, depth+1, nodeCount); err != nil {
				return err
			}
		}
		return nil
	}
	// Leaf node: field/operator must be whitelisted and values bounded.
	operators, ok := define.ViewFilterFieldOperators[node.Field]
	if !ok && strings.HasPrefix(node.Field, define.ViewFilterCustomPrefix) {
		fieldId, parseErr := strconv.Atoi(strings.TrimPrefix(node.Field, define.ViewFilterCustomPrefix))
		if parseErr == nil && fieldId > 0 {
			operators = define.ViewFilterCustomFieldOperators
			ok = true
		}
	}
	if !ok {
		return ErrViewFiltersInvalid
	}
	if len(node.Conditions) > 0 {
		return ErrViewFiltersInvalid
	}
	if err := IsInList("operator", node.Operator, operators); err != nil {
		return ErrViewFiltersInvalid
	}
	if len(node.Values) > define.ViewFilterMaxValues {
		return ErrViewFiltersInvalid
	}
	for _, value := range node.Values {
		if utf8.RuneCountInString(value) > define.ViewFilterValueMaxLength {
			return ErrViewFiltersInvalid
		}
	}
	if node.Field == define.ViewFilterFieldUrgeStatus || node.Field == define.ViewFilterFieldSLAStatus {
		return ValidateIssueFilter(node)
	}
	return nil
}

// ParseSLAFilterValue 解析进入 SLA 或具体阶段的完成结果；阶段名称不作为持久化标识。
func ParseSLAFilterValue(value string) (int, string, error) {
	if value == "entered" {
		return 0, value, nil
	}
	parts := strings.Split(value, ":")
	if len(parts) != 2 || (parts[0] != "completed_late" && parts[0] != "completed_on_time") {
		return 0, "", ErrViewFiltersInvalid
	}
	id, err := strconv.Atoi(parts[1])
	if err != nil || id <= 0 || strconv.Itoa(id) != parts[1] {
		return 0, "", ErrViewFiltersInvalid
	}
	return id, parts[0], nil
}

// ValidateIssueFilter 校验工单专属单选筛选值，空值表示不限制。
func ValidateIssueFilter(node define.ViewFilterNode) error {
	if node.Field != define.ViewFilterFieldUrgeStatus && node.Field != define.ViewFilterFieldSLAStatus {
		return ErrViewFiltersInvalid
	}
	if node.Operator != define.ViewFilterOperatorIn || len(node.Values) > 1 {
		return ErrViewFiltersInvalid
	}
	if len(node.Values) == 0 {
		return nil
	}
	if node.Field == define.ViewFilterFieldUrgeStatus {
		if node.Values[0] != "urged" && node.Values[0] != "not_urged" {
			return ErrViewFiltersInvalid
		}
		return nil
	}
	_, _, err := ParseSLAFilterValue(node.Values[0])
	return err
}

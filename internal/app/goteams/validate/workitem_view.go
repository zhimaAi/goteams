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
	return nil
}

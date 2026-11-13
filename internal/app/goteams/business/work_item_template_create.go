package business

import (
	"encoding/json"
	"slices"
	"strconv"

	"goteams/internal/app/goteams/define"

	"github.com/spf13/cast"
)

// customFieldsForCreateTemplate 只启用当前模板可见字段，工作空间必填字段始终保留。
func customFieldsForCreateTemplate(fields []CustomFieldModel, template []TemplateFieldModel, model CreateRequirementModel) ([]CustomFieldModel, error) {
	configured := make(map[string]TemplateFieldModel, len(template))
	for _, field := range template {
		configured[field.FieldKey] = field
	}
	required := make(map[string]bool)
	for _, field := range fields {
		if field.IsEnabled == 1 && field.IsRequired == 1 {
			required[`custom:`+strconv.Itoa(field.Id)] = true
		}
	}
	result := make([]CustomFieldModel, 0, len(fields))
	for _, field := range fields {
		if field.IsEnabled == 1 && field.IsRequired == 1 {
			result = append(result, field)
			continue
		}
		key := `custom:` + strconv.Itoa(field.Id)
		spec, exists := configured[key]
		if !exists {
			continue
		}
		visible, err := createTemplateFieldVisible(template, key, model, required, nil)
		if err != nil {
			return nil, err
		}
		if !visible {
			continue
		}
		field.IsEnabled = 1
		field.IsRequired = 0
		if spec.Required {
			field.IsRequired = 1
		}
		result = append(result, field)
	}
	return result, nil
}

func createTemplateFieldVisible(fields []TemplateFieldModel, key string, model CreateRequirementModel, required, visiting map[string]bool) (bool, error) {
	if required[key] {
		return true, nil
	}
	if visiting[key] {
		return false, nil
	}
	if visiting == nil {
		visiting = make(map[string]bool)
	}
	visiting[key] = true
	defer delete(visiting, key)
	linked := false
	for _, trigger := range fields {
		var value any
		switch trigger.FieldKey {
		case `status`:
			value = model.StatusId
		case `priority`:
			if model.Priority != nil {
				value = *model.Priority
			}
		case `version`:
			value = model.VersionId
		case `category`:
			value = model.CategoryId
		default:
			value = model.CustomFields[strconv.Itoa(templateCustomFieldID(trigger.FieldKey))]
		}
		if raw, ok := value.(string); ok && trigger.FieldType == define.CustomFieldTypeSelectMulti && raw != `` {
			selected, err := decodeStringSlice(raw)
			if err != nil {
				return false, newCustomFieldValidationError(ErrCustomFieldValueInvalid, define.MsgCustomFieldValueInvalid, trigger.FieldName)
			}
			value = selected
		}
		for _, rule := range trigger.Linkages {
			if !slices.Contains(rule.FieldKeys, key) {
				continue
			}
			linked = true
			var expected any
			if err := json.Unmarshal(rule.Value, &expected); err != nil {
				return false, ErrWorkItemTemplateFieldInvalid
			}
			matches := cast.ToString(value) == cast.ToString(expected)
			for _, selected := range cast.ToStringSlice(value) {
				if selected == cast.ToString(expected) {
					matches = true
					break
				}
			}
			if matches {
				visible, err := createTemplateFieldVisible(fields, trigger.FieldKey, model, required, visiting)
				if err != nil {
					return false, err
				}
				if visible {
					return true, nil
				}
			}
		}
	}
	return !linked, nil
}

// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package validate

import (
	"errors"
	"strings"
	"unicode/utf8"

	"goteams/internal/app/goteams/define"
)

const customFieldNameMaxRunes = 20

// ErrCustomFieldNameRequired indicates that a custom field name is blank.
var ErrCustomFieldNameRequired = errors.New(`custom field name is required`)

// ErrCustomFieldNameTooLong indicates that a custom field name exceeds the supported length.
var ErrCustomFieldNameTooLong = errors.New(`custom field name is too long`)

// ValidateCustomFieldName checks a custom field display name: required, at most 20 runes.
func ValidateCustomFieldName(name string) error {
	if strings.TrimSpace(name) == `` {
		return ErrCustomFieldNameRequired
	}
	if utf8.RuneCountInString(name) > customFieldNameMaxRunes {
		return ErrCustomFieldNameTooLong
	}
	return nil
}

func inStringList(value string, allowed []string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

// ValidateNewCustomFieldType rejects legacy textarea so it cannot be newly created.
func ValidateNewCustomFieldType(fieldType string) error {
	if inStringList(fieldType, define.CustomFieldTypes) {
		return nil
	}
	return ErrCustomFieldTypeInvalid
}

// ValidateCustomFieldType checks that the field type is stored-valid, including legacy textarea.
func ValidateCustomFieldType(fieldType string) error {
	allowed := make([]string, 0, len(define.CustomFieldTypes)+1)
	allowed = append(allowed, define.CustomFieldTypeTextarea)
	allowed = append(allowed, define.CustomFieldTypes...)
	if inStringList(fieldType, allowed) {
		return nil
	}
	return ErrCustomFieldTypeInvalid
}

// ErrCustomFieldTypeInvalid indicates that a field type is not in the create whitelist.
var ErrCustomFieldTypeInvalid = errors.New(`custom field type is invalid`)

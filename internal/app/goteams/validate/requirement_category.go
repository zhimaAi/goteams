// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package validate

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// ValidateRequirementCategoryName validates a category name after trimming surrounding whitespace.
func ValidateRequirementCategoryName(name string) error {
	name = strings.TrimSpace(name)
	if err := NotBlank("name", name); err != nil {
		return err
	}
	if utf8.RuneCountInString(name) > 20 {
		return errors.New("requirement category name exceeds 20 characters")
	}
	return nil
}

// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package validate

import (
	"fmt"
	"regexp"

	"goteams/internal/app/goteams/define"
)

var customWorkspaceTypePattern = regexp.MustCompile(`^custom_[0-9a-f]{12}$`)

// ValidateWorkspaceTypeName 校验类型名称的存储边界。
func ValidateWorkspaceTypeName(name string) error {
	if err := NotBlank("name", name); err != nil {
		return err
	}
	if err := MaxLen("name", name, 100); err != nil {
		return err
	}
	return nil
}

// ValidateWorkspaceType 校验新建空间必须保存的类型。
func ValidateWorkspaceType(workspaceType string) error {
	switch workspaceType {
	case define.WorkspaceTypeSoftware, define.WorkspaceTypeAgile, define.WorkspaceTypeIssue:
		return nil
	default:
		if customWorkspaceTypePattern.MatchString(workspaceType) {
			return nil
		}
		return fmt.Errorf("invalid workspace type %q", workspaceType)
	}
}

// ======================== Workspace Parameter Validation ========================

// ValidateWorkspaceCreate validates workspace creation.
func ValidateWorkspaceCreate(name, description, color string) error {
	if fe := NotBlank("name", name); fe != nil {
		return fe
	}
	if fe := MaxLen("name", name, 200); fe != nil {
		return fe
	}
	if len(description) > 0 {
		if fe := MaxLen("description", description, 5000); fe != nil {
			return fe
		}
	}
	if len(color) > 0 {
		if fe := MaxLen("color", color, 20); fe != nil {
			return fe
		}
	}
	return nil
}

// ValidateWorkspaceUpdate validates workspace update.
func ValidateWorkspaceUpdate(name, description, color string) error {
	return ValidateWorkspaceCreate(name, description, color) // Same rules.
}

// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package validate

import (
	"unicode/utf8"

	"goteams/internal/app/goteams/define"
)

const (
	workItemTemplateNameMax  = 50
	workItemTemplateTitleMax = 50
)

// WorkItemTemplateType checks that the work-item type can own templates.
func WorkItemTemplateType(typeKey string) error {
	if err := IsInList(`type`, typeKey, []string{define.TypeKeyRequirement, define.TypeKeyDefect}); err != nil {
		return err
	}
	return nil
}

// WorkItemTemplateName checks that a template name is present and within the 50-character limit.
func WorkItemTemplateName(name string) error {
	if err := NotBlank(`name`, name); err != nil {
		return err
	}
	if utf8.RuneCountInString(name) > workItemTemplateNameMax {
		return MaxLen(`name`, name, workItemTemplateNameMax)
	}
	return nil
}

// WorkItemTemplateTitle checks the optional default title length.
func WorkItemTemplateTitle(title string) error {
	if utf8.RuneCountInString(title) > workItemTemplateTitleMax {
		return MaxLen(`title`, title, workItemTemplateTitleMax)
	}
	return nil
}

// WorkItemTemplateDescriptionMode checks the description editor mode.
func WorkItemTemplateDescriptionMode(mode string) error {
	if mode == `` {
		return nil
	}
	if err := IsInList(`description_mode`, mode, []string{define.DescriptionModeRichText, define.DescriptionModeMarkdown}); err != nil {
		return err
	}
	return nil
}

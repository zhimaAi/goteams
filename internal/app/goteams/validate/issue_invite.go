// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package validate

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"goteams/internal/app/goteams/define"
)

var (
	// ErrIssueInviteKeyInvalid indicates an invitation/template public key failed validation.
	ErrIssueInviteKeyInvalid = errors.New("issue invite key invalid")
	// ErrIssueInviteOpenIDInvalid indicates an external submitter identifier failed validation.
	ErrIssueInviteOpenIDInvalid = errors.New("issue invite openid invalid")
	// ErrIssueInvitePageInvalid indicates a public history page number failed validation.
	ErrIssueInvitePageInvalid = errors.New("issue invite page invalid")
	// ErrIssueInviteTemplateInvalid indicates a copied-template update failed validation.
	ErrIssueInviteTemplateInvalid = errors.New("issue invite template invalid")
	// ErrIssueInviteOrderInvalid indicates a copied-template order failed validation.
	ErrIssueInviteOrderInvalid = errors.New("issue invite template order invalid")
	// ErrIssueInviteSubmissionInvalid indicates a public submission failed validation.
	ErrIssueInviteSubmissionInvalid = errors.New("issue invite submission invalid")
	// ErrIssueInviteGuideTitleInvalid indicates the guide title exceeded its limit.
	ErrIssueInviteGuideTitleInvalid = errors.New("issue invite guide title invalid")
	// ErrIssueInviteGuideDescriptionInvalid indicates the guide description exceeded its limit.
	ErrIssueInviteGuideDescriptionInvalid = errors.New("issue invite guide description invalid")
	// ErrIssueInviteThemeImageInvalid indicates the theme-image URL exceeded its limit.
	ErrIssueInviteThemeImageInvalid = errors.New("issue invite theme image invalid")
)

// issueInviteSafeASCII reports whether every byte is a letter, digit, dot, underscore or hyphen.
// This is the shared whitelist for both public keys and external openids.
func issueInviteSafeASCII(value string) bool {
	if value == `` {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '.' || c == '_' || c == '-':
		default:
			return false
		}
	}
	return true
}

// IssueInviteKey validates an invitation or template public key.
func IssueInviteKey(key string) error {
	if len(key) != define.IssueInviteKeyLength || !issueInviteSafeASCII(key) {
		return ErrIssueInviteKeyInvalid
	}
	return nil
}

// IssueInviteOpenID validates a provided public user identifier.
func IssueInviteOpenID(openID string) error {
	if len(openID) < 1 || len(openID) > define.IssueInviteOpenIDMaxLength || !issueInviteSafeASCII(openID) {
		return ErrIssueInviteOpenIDInvalid
	}
	return nil
}

// IssueInvitePage normalizes and validates a public history page number.
func IssueInvitePage(page int) (int, error) {
	if page < 0 {
		return 0, ErrIssueInvitePageInvalid
	}
	if page == 0 {
		return 1, nil
	}
	return page, nil
}

// issueInviteFieldsJSON validates that the raw fields_json string is a JSON array.
func issueInviteFieldsJSON(fieldsJSON string) error {
	trimmed := strings.TrimSpace(fieldsJSON)
	if trimmed == `` {
		trimmed = `[]`
	}
	var fields []json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &fields); err != nil {
		return ErrIssueInviteTemplateInvalid
	}
	return nil
}

// issueInviteRecallConfig 校验撤回开关：开关只能是 0/1，状态 id 不能是负数。
// 开启时状态必须落到一个真实状态上，这一步要查库，由业务层在解析状态后拦。
func issueInviteRecallConfig(enabled, statusID int) error {
	if enabled != 0 && enabled != 1 {
		return ErrIssueInviteTemplateInvalid
	}
	if statusID < 0 {
		return ErrIssueInviteTemplateInvalid
	}
	return nil
}

// IssueInviteTemplateCreate validates one new independent invitation template.
func IssueInviteTemplateCreate(model define.IssueInviteTemplateCreateModel) error {
	if model.TypeKey != define.TypeKeyRequirement && model.TypeKey != define.TypeKeyDefect {
		return ErrIssueInviteTemplateInvalid
	}
	if err := WorkItemTemplateName(model.Name); err != nil {
		return err
	}
	if err := WorkItemTemplateDescriptionMode(model.DescriptionMode); err != nil {
		return err
	}
	if model.Enabled != 0 && model.Enabled != 1 {
		return ErrIssueInviteTemplateInvalid
	}
	// 模板级邀请文案开关独立于项目级配置，同样只收 0/1
	if model.GuideEnabled != 0 && model.GuideEnabled != 1 {
		return ErrIssueInviteTemplateInvalid
	}
	if err := issueInviteRecallConfig(model.RecallEnabled, model.RecallStatusID); err != nil {
		return err
	}
	if utf8.RuneCountInString(model.GuideTitle) > define.IssueInviteGuideTitleMaxLength {
		return ErrIssueInviteGuideTitleInvalid
	}
	if utf8.RuneCountInString(model.GuideDescription) > define.IssueInviteGuideDescriptionMaxLength {
		return ErrIssueInviteGuideDescriptionInvalid
	}
	if len(model.ThemeImage) > define.IssueInviteThemeImageMaxBytes {
		return ErrIssueInviteThemeImageInvalid
	}
	return issueInviteFieldsJSON(model.FieldsJSON)
}

// IssueInviteTemplateUpdate validates invitation-template editable fields.
func IssueInviteTemplateUpdate(model define.IssueInviteTemplateUpdateModel) error {
	if err := WorkItemTemplateName(model.Name); err != nil {
		return err
	}
	if err := WorkItemTemplateDescriptionMode(model.DescriptionMode); err != nil {
		return err
	}
	if model.Enabled != 0 && model.Enabled != 1 {
		return ErrIssueInviteTemplateInvalid
	}
	// 模板级邀请文案开关独立于项目级配置，同样只收 0/1
	if model.GuideEnabled != 0 && model.GuideEnabled != 1 {
		return ErrIssueInviteTemplateInvalid
	}
	if err := issueInviteRecallConfig(model.RecallEnabled, model.RecallStatusID); err != nil {
		return err
	}
	if utf8.RuneCountInString(model.GuideTitle) > define.IssueInviteGuideTitleMaxLength {
		return ErrIssueInviteGuideTitleInvalid
	}
	if utf8.RuneCountInString(model.GuideDescription) > define.IssueInviteGuideDescriptionMaxLength {
		return ErrIssueInviteGuideDescriptionInvalid
	}
	if len(model.ThemeImage) > define.IssueInviteThemeImageMaxBytes {
		return ErrIssueInviteThemeImageInvalid
	}
	return issueInviteFieldsJSON(model.FieldsJSON)
}

// IssueInviteTemplateOrder validates a deduplicated positive template ID list.
func IssueInviteTemplateOrder(model define.IssueInviteTemplateOrderModel) error {
	if len(model.OrderedTemplateID) == 0 {
		return ErrIssueInviteOrderInvalid
	}
	seen := make(map[int]struct{}, len(model.OrderedTemplateID))
	for _, id := range model.OrderedTemplateID {
		if id <= 0 {
			return ErrIssueInviteOrderInvalid
		}
		if _, dup := seen[id]; dup {
			return ErrIssueInviteOrderInvalid
		}
		seen[id] = struct{}{}
	}
	return nil
}

// IssueInviteSubmission validates the public request shape before business data lookup.
func IssueInviteSubmission(model define.IssueInviteSubmitModel) error {
	if err := IssueInviteKey(model.InviteKey); err != nil {
		return err
	}
	if err := IssueInviteKey(model.TemplateKey); err != nil {
		return err
	}
	if err := IssueInviteOpenID(model.OpenID); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(model.Fields))
	for _, field := range model.Fields {
		key := strings.TrimSpace(field.FieldKey)
		if key == `` {
			return ErrIssueInviteSubmissionInvalid
		}
		if _, dup := seen[key]; dup {
			return ErrIssueInviteSubmissionInvalid
		}
		seen[key] = struct{}{}
	}
	return nil
}

// IssueInviteRequirementAction validates one public action on an already submitted issue:
// the public key, the submitter and the issue id, plus the field list a resubmit carries.
func IssueInviteRequirementAction(model define.IssueInviteRequirementActionModel) error {
	if err := IssueInviteKey(model.InviteKey); err != nil {
		return err
	}
	if err := IssueInviteOpenID(model.OpenID); err != nil {
		return err
	}
	if model.RequirementID <= 0 {
		return ErrIssueInviteSubmissionInvalid
	}
	seen := make(map[string]struct{}, len(model.Fields))
	for _, field := range model.Fields {
		key := strings.TrimSpace(field.FieldKey)
		if key == `` {
			return ErrIssueInviteSubmissionInvalid
		}
		if _, dup := seen[key]; dup {
			return ErrIssueInviteSubmissionInvalid
		}
		seen[key] = struct{}{}
	}
	return nil
}

// IssueInviteConfigUpdate validates editable invitation guide configuration.
func IssueInviteConfigUpdate(model define.IssueInviteConfigUpdateModel) error {
	if model.Enabled != 0 && model.Enabled != 1 {
		return ErrIssueInviteTemplateInvalid
	}
	if utf8.RuneCountInString(model.GuideTitle) > define.IssueInviteGuideTitleMaxLength {
		return ErrIssueInviteGuideTitleInvalid
	}
	if utf8.RuneCountInString(model.GuideDescription) > define.IssueInviteGuideDescriptionMaxLength {
		return ErrIssueInviteGuideDescriptionInvalid
	}
	return nil
}

package validate

import (
	"net/url"
	"strings"
	"unicode/utf8"

	"goteams/internal/app/goteams/define"
)

// ValidateSkillText 按数据库字符长度限制校验技能文本。
func ValidateSkillText(field, value string, limit int) error {
	if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		return &FieldError{Field: field, Key: define.MsgParamInvalid}
	}
	if utf8.RuneCountInString(value) > limit {
		return &FieldError{Field: field, Key: "validate_param_too_long"}
	}
	return nil
}

// ValidateSkillName 校验技能名称。
func ValidateSkillName(value string) error {
	if err := NotBlank("name", value); err != nil {
		return err
	}
	return ValidateSkillText("name", value, 100)
}

// ValidateSkillSourceURL 校验来源地址，仅存储 HTTP(S) 地址，不发起远程请求。
func ValidateSkillSourceURL(value string) error {
	if err := ValidateSkillText("source_url", value, 2048); err != nil {
		return err
	}
	u, err := url.ParseRequestURI(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return &FieldError{Field: "source_url", Key: define.MsgParamInvalid}
	}
	return nil
}

// ValidateSkillSource 校验技能来源。
func ValidateSkillSource(value string) error {
	if err := IsInList("source", value, []string{define.SkillSourceGitHub, define.SkillSourceSkillHub, define.SkillSourceClawHub}); err != nil {
		return err
	}
	return nil
}

// ValidateSkillReviewStatus 校验审核状态。
func ValidateSkillReviewStatus(value string) error {
	if err := IsInList("review_status", value, []string{define.SkillReviewPending, define.SkillReviewApproved, define.SkillReviewRejected}); err != nil {
		return err
	}
	return nil
}

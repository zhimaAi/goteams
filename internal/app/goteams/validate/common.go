// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package validate

import (
	"net/mail"
	"regexp"
	"strings"
)

// ======================== Field-Level Validation Error ========================

// FieldError carries a field name and i18n key for validation errors, so controllers can return field-level hints.
type FieldError struct {
	Field string // JSON field name, e.g. "password"
	Key   string // i18n message key, e.g. "validate_password_chars"
}

// Error implements the error interface; controllers can use errors.As to check the type.
func (e *FieldError) Error() string {
	if e == nil {
		return ``
	}
	return e.Field + ": " + e.Key
}

// ======================== Common Validation Utilities ========================

// NormalizeEmail normalizes email case and trims surrounding whitespace.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// NotBlank checks that the string is non-empty. field is the JSON field name.
func NotBlank(field, s string) *FieldError {
	if len(strings.TrimSpace(s)) == 0 {
		return &FieldError{Field: field, Key: "validate_param_blank"}
	}
	return nil
}

// MaxLen checks the maximum length of a string. field is the JSON field name.
func MaxLen(field, s string, maxLen int) *FieldError {
	if len(s) > maxLen {
		return &FieldError{Field: field, Key: "validate_param_too_long"}
	}
	return nil
}

// ValidateAccount validates a login account: non-blank and within the column limit, without forcing an email format.
// 云版仅注册/找回密码要求邮箱（需收验证码）；登录、新增成员、邀请加入的账号可以是非邮箱，只校验非空与长度。
func ValidateAccount(field, account string) *FieldError {
	if fe := NotBlank(field, account); fe != nil {
		return fe
	}
	return MaxLen(field, account, 255)
}

// ValidateUsername validates that the login account is a legal email address. field is the JSON field name (e.g. "email").
// 只用于必须收验证码的邮箱场景（注册/找回密码/重置密码），其余账号入口请用 ValidateAccount。
func ValidateUsername(field, username string) error {
	username = NormalizeEmail(username)
	if fe := NotBlank(field, username); fe != nil {
		fe.Key = "validate_email_empty"
		return fe
	}
	if fe := MaxLen(field, username, 255); fe != nil {
		fe.Key = "validate_email_too_long"
		return fe
	}
	address, err := mail.ParseAddress(username)
	if err != nil || !strings.EqualFold(address.Address, username) {
		return &FieldError{Field: field, Key: "validate_email_invalid"}
	}
	return nil
}

// IsValidPassword validates password strength: 8-32 characters, letters/digits/common special characters.
func IsValidPassword(password string) error {
	if len(password) < 8 || len(password) > 32 {
		return &FieldError{Field: "password", Key: "validate_password_length"}
	}
	ok, _ := regexp.MatchString(`^[a-zA-Z0-9_\-!@#$%^&*.]+$`, password)
	if !ok {
		return &FieldError{Field: "password", Key: "validate_password_chars"}
	}
	return nil
}

// IsInList checks whether a value is in the allowed list. field is the JSON field name.
func IsInList(field, val string, allowed []string) *FieldError {
	for _, item := range allowed {
		if val == item {
			return nil
		}
	}
	return &FieldError{Field: field, Key: "common_param_invalid"}
}

package validate

import (
	"errors"
	"strings"
)

// ErrAutomationInvalid means the automation request parameters are invalid.
var ErrAutomationInvalid = errors.New("invalid automation request")

// ValidateAutomationRule validates the rule name, locale and config body size.
func ValidateAutomationRule(name, locale string, configSize int) error {
	length := len([]rune(strings.TrimSpace(name)))
	if length == 0 || length > 200 || (locale != "zh-CN" && locale != "en-US") || configSize == 0 || configSize > 1<<20 {
		return ErrAutomationInvalid
	}
	return nil
}

// ValidateAutomationPage validates the pagination parameters.
func ValidateAutomationPage(page, size int) error {
	if page < 1 || (size != 20 && size != 50 && size != 100) {
		return ErrAutomationInvalid
	}
	return nil
}

// ValidateAutomationTimeRange validates the local natural-day time range submitted by the frontend.
func ValidateAutomationTimeRange(start, end int64) error {
	if start < 0 || end < 0 || (start > 0 && end > 0 && end <= start) {
		return ErrAutomationInvalid
	}
	return nil
}

// ValidateAutomationPushIdentifier validates the length limit of one member push identifier.
func ValidateAutomationPushIdentifier(identifier string) error {
	if len([]rune(identifier)) > 100 {
		return ErrAutomationInvalid
	}
	return nil
}

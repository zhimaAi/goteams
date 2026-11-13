package validate

import (
	"net/url"
	"strings"
	"unicode/utf8"

	"goteams/internal/app/goteams/define"
)

var externalNotificationEvents = map[string]bool{
	"assign_requirement": true,
	"assign_bug":         true,
	"comment_mention":    true,
	"status_change":      true,
	"due_reminder":       true,
	"overdue":            true,
}

var externalNotificationPlatforms = []string{"feishu", "wecom", "dingtalk"}

// externalNotificationAtModes mirrors each robot API's mention parameters.
var externalNotificationAtModes = map[string]map[string]bool{
	"dingtalk": {"mobile": true, "at_mobiles": true, "at_user_ids": true},
	"feishu":   {"user_id": true},
	"wecom":    {"mobile": true, "mentioned_list": true, "mentioned_mobile_list": true},
}

var externalNotificationAtRoles = map[string]bool{
	"created_by": true,
	"assignee":   true,
}

// externalNotificationNameMax 按字符数限制策略名称，避免 Go 字节长度误伤中文。
const externalNotificationNameMax = 30

// ValidateExternalNotificationStrategy validates a notification strategy payload.
func ValidateExternalNotificationStrategy(model define.ExternalNotificationStrategyValidation) error {
	if fe := NotBlank("name", model.Name); fe != nil {
		return fe
	}
	if utf8.RuneCountInString(model.Name) > externalNotificationNameMax {
		return &FieldError{Field: "name", Key: "validate_param_too_long"}
	}
	if len(model.MemberIDs) == 0 {
		return &FieldError{Field: "member_ids", Key: "validate_param_blank"}
	}
	if len(model.Events) == 0 {
		return &FieldError{Field: "events", Key: "validate_param_blank"}
	}
	if fe := IsInList("platform", model.Platform, externalNotificationPlatforms); fe != nil {
		return fe
	}
	if fe := NotBlank("webhook_url", model.WebhookURL); fe != nil {
		return fe
	}
	u, err := url.Parse(strings.TrimSpace(model.WebhookURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return &FieldError{Field: "webhook_url", Key: "common_param_invalid"}
	}
	seen := make(map[string]bool, len(model.Events))
	for _, event := range model.Events {
		if !externalNotificationEvents[event] || seen[event] {
			return &FieldError{Field: "events", Key: "common_param_invalid"}
		}
		seen[event] = true
	}
	// at_mode 由业务层补默认值后传入，这里只拒绝平台不支持的模式。
	if !externalNotificationAtModes[model.Platform][model.AtMode] {
		return &FieldError{Field: "at_mode", Key: "common_param_invalid"}
	}
	seenRoles := make(map[string]bool, len(model.AtRoles))
	for _, role := range model.AtRoles {
		if !externalNotificationAtRoles[role] || seenRoles[role] {
			return &FieldError{Field: "at_roles", Key: "common_param_invalid"}
		}
		seenRoles[role] = true
	}
	return nil
}

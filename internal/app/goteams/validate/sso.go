// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package validate

import (
	"encoding/json"
	"net/url"
	"strings"
)

// ValidateSsoUrl 校验 SSO 配置里的地址字段：允许为空（关闭状态可以先不填），
// 非空时必须是带主机名的 http/https 绝对地址。
//
// 这里只挡格式，不做网络可达性判断：SSRF 防线在业务层的放行网段，不该让校验层去解析 DNS。
func ValidateSsoUrl(field, raw string, maxLen int) error {
	value := strings.TrimSpace(raw)
	if value == `` {
		return nil
	}
	if len(value) > maxLen {
		return &FieldError{Field: field, Key: "validate_param_too_long"}
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != `http` && parsed.Scheme != `https`) || parsed.Host == `` {
		return &FieldError{Field: field, Key: "sso_validate_url_invalid"}
	}
	return nil
}

// ValidateSsoFieldMapping 校验字段映射：允许为空，非空时必须是一个 JSON 对象。
// 键是 IdP 侧字段名、运行时才知道，所以只验形态，不枚举字段。
func ValidateSsoFieldMapping(field, raw string) error {
	value := strings.TrimSpace(raw)
	if value == `` {
		return nil
	}
	if len(value) > 2000 {
		return &FieldError{Field: field, Key: "validate_param_too_long"}
	}
	var mapping map[string]any
	if err := json.Unmarshal([]byte(value), &mapping); err != nil {
		return &FieldError{Field: field, Key: "sso_validate_mapping_invalid"}
	}
	return nil
}

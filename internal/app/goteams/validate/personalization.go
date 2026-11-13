package validate

import (
	"regexp"
	"unicode/utf8"
)

var personalizationDomainPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{5,61}[a-z0-9]$`)

// ValidatePersonalization 校验个性化字段；空值表示恢复产品默认值。
func ValidatePersonalization(appName, domainPrefix, loginSlogan string) error {
	if utf8.RuneCountInString(appName) > 100 || len(loginSlogan) > 20000 {
		return &FieldError{Key: "personalization_invalid"}
	}
	if domainPrefix != "" && !personalizationDomainPattern.MatchString(domainPrefix) {
		return &FieldError{Field: "domain_prefix", Key: "personalization_domain_invalid"}
	}
	return nil
}

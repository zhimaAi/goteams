// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package validate

import "regexp"

// Online trial and license-key input validation (commercial license subsystem).

// verifyCodeRegexp matches the 6-digit email verification code.
var verifyCodeRegexp = regexp.MustCompile(`^[0-9]{6}$`)

// phoneRegexp matches digits and an optional leading plus, 5-20 characters total.
var phoneRegexp = regexp.MustCompile(`^\+?[0-9]{5,20}$`)

// ValidateTrialCorpName validates the online-trial enterprise name: 1-100 runes.
func ValidateTrialCorpName(corpName string) error {
	runes := len([]rune(corpName))
	if runes < 1 {
		return &FieldError{Field: "corp_name", Key: "license_corp_name_blank"}
	}
	if runes > 100 {
		return &FieldError{Field: "corp_name", Key: "license_corp_name_too_long"}
	}
	return nil
}

// ValidateTrialPhone validates the online-trial phone number.
func ValidateTrialPhone(phone string) error {
	if fe := NotBlank("phone", phone); fe != nil {
		fe.Key = "license_phone_blank"
		return fe
	}
	if !phoneRegexp.MatchString(phone) {
		return &FieldError{Field: "phone", Key: "license_phone_invalid"}
	}
	return nil
}

// ValidateTrialEmail validates the online-trial email (verification-code receivers only).
func ValidateTrialEmail(email string) error {
	return ValidateUsername("email", email)
}

// ValidateVerifyCode validates the 6-digit email verification code format.
func ValidateVerifyCode(code string) error {
	if !verifyCodeRegexp.MatchString(code) {
		return &FieldError{Field: "verify_code", Key: "license_verify_code_invalid"}
	}
	return nil
}

// ValidateRequestId validates the optional idempotency request id.
func ValidateRequestId(requestId string) error {
	if fe := MaxLen("request_id", requestId, 64); fe != nil {
		return fe
	}
	return nil
}

// ValidateLicenseKeyInput validates the pasted GTLIC1 key envelope before parsing.
func ValidateLicenseKeyInput(licenseKey string) error {
	if fe := NotBlank("license_key", licenseKey); fe != nil {
		fe.Key = "license_key_blank"
		return fe
	}
	if fe := MaxLen("license_key", licenseKey, 4096); fe != nil {
		fe.Key = "license_key_too_long"
		return fe
	}
	return nil
}

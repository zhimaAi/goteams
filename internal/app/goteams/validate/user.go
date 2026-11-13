// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package validate

import "unicode/utf8"

// ======================== User Parameter Validation ========================

// LoginParams login parameters.
type LoginParams struct {
	Username string
	Password string
}

// ValidateLogin validates login parameters.
func ValidateLogin(p LoginParams) error {
	if fe := ValidateAccount("username", p.Username); fe != nil {
		return fe
	}
	if fe := NotBlank("password", p.Password); fe != nil {
		fe.Key = "validate_password_empty"
		return fe
	}
	return nil
}

// RegisterParams registration parameters.
type RegisterParams struct {
	Username string
	Password string
	Name     string
}

// ValidateRegister validates registration parameters.
func ValidateRegister(p RegisterParams) error {
	if fe := ValidateUsername("email", p.Username); fe != nil {
		return fe
	}
	if fe := NotBlank("password", p.Password); fe != nil {
		fe.Key = "validate_password_empty"
		return fe
	}
	if fe := IsValidPassword(p.Password); fe != nil {
		return fe
	}
	// Name is optional but has a length limit.
	if len(p.Name) > 0 {
		if fe := MaxLen("name", p.Name, 100); fe != nil {
			fe.Key = "validate_name_too_long"
			return fe
		}
	}
	return nil
}

// ValidateProfile validates profile update parameters.
func ValidateProfile(name string) error {
	if fe := NotBlank("name", name); fe != nil {
		return fe
	}
	if fe := MaxLen("name", name, 100); fe != nil {
		return fe
	}
	return nil
}

// ValidateEnterpriseName 校验企业名称。
func ValidateEnterpriseName(name string) error {
	if fe := NotBlank("enterprise_name", name); fe != nil {
		return fe
	}
	if utf8.RuneCountInString(name) > 100 {
		return &FieldError{Field: "enterprise_name", Key: "validate_param_too_long"}
	}
	return nil
}

// ValidatePassword validates password change parameters.
func ValidatePassword(oldPassword, newPassword string) error {
	if fe := NotBlank("old_password", oldPassword); fe != nil {
		fe.Key = "validate_password_empty"
		return fe
	}
	if fe := NotBlank("new_password", newPassword); fe != nil {
		fe.Key = "validate_password_empty"
		return fe
	}
	return IsValidPassword(newPassword)
}

// ValidateForgotPassword validates forgot password parameters.
func ValidateForgotPassword(username string) error {
	return ValidateUsername("email", username)
}

// ValidateResetPassword validates reset password parameters.
func ValidateResetPassword(username, verifyCode, newPassword string) error {
	if fe := ValidateUsername("email", username); fe != nil {
		return fe
	}
	if fe := NotBlank("verifyCode", verifyCode); fe != nil {
		fe.Key = "validate_verify_code_empty"
		return fe
	}
	if fe := NotBlank("newPassword", newPassword); fe != nil {
		fe.Key = "validate_password_empty"
		return fe
	}
	return IsValidPassword(newPassword)
}

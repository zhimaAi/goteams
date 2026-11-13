// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package validate

import (
	"errors"
	"regexp"
)

// ======================== Member Parameter Validation ========================

// Allowed member roles.
var AllowedMemberRoles = []string{`admin`, `member`}

// ValidateAddMember validates adding a member.
func ValidateAddMember(username, role string) error {
	if fe := ValidateUsername("username", username); fe != nil {
		return fe
	}
	if len(role) > 0 {
		if fe := IsInList("role", role, AllowedMemberRoles); fe != nil {
			return fe
		}
	}
	return nil
}

// ValidateMemberRole validates a role value.
func ValidateMemberRole(role string) error {
	if fe := IsInList("role", role, AllowedMemberRoles); fe != nil {
		return fe
	}
	return nil
}

// ValidateMemberPhone validates an optional member phone number.
func ValidateMemberPhone(phone string) error {
	if err := MaxLen("phone", phone, 20); err != nil {
		return err
	}
	return nil
}

// ValidateMemberEmail validates an optional member email. Empty means unbound and is allowed;
// a non-empty value must be a legal address that can receive verification codes.
func ValidateMemberEmail(email string) error {
	if len(NormalizeEmail(email)) == 0 {
		return nil
	}
	return ValidateUsername("email", email)
}

// ErrInvitePhoneInvalid indicates the invite-join phone number format is invalid.
var ErrInvitePhoneInvalid = errors.New(`invite phone number invalid`)

// invitePhoneRegexp matches a mainland China mobile number: 11 digits starting with 1.
var invitePhoneRegexp = regexp.MustCompile(`^1\d{10}$`)

// ValidateInvitePhone validates an invite-join phone number (1 + 10 digits).
func ValidateInvitePhone(phone string) error {
	if !invitePhoneRegexp.MatchString(phone) {
		return ErrInvitePhoneInvalid
	}
	return nil
}

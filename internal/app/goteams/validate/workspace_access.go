package validate

import "unicode/utf8"

// ValidateWorkspaceAccessReason validates the applicant's short explanation.
func ValidateWorkspaceAccessReason(reason string) error {
	if err := NotBlank(`reason`, reason); err != nil {
		return err
	}
	if utf8.RuneCountInString(reason) > 30 {
		return &FieldError{Field: `reason`, Key: `validate_param_too_long`}
	}
	return nil
}

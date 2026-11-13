// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package validate

import (
	"errors"

	"goteams/internal/app/goteams/define"
)

// ======================== Version Parameter Validation ========================

// ValidateVersionCreate validates the version name.
func ValidateVersionCreate(name string) error {
	if fe := NotBlank("name", name); fe != nil {
		return fe
	}
	if fe := MaxLen("name", name, 200); fe != nil {
		return fe
	}
	return nil
}

// ValidateIterationType checks the immutable single/cross-workspace type.
func ValidateIterationType(iterationType string) error {
	if iterationType != define.IterationTypeSingle && iterationType != define.IterationTypeCross {
		return errors.New(`iteration type must be single or cross`)
	}
	return nil
}

// ValidateVersionDates validates optional dates and their order.
func ValidateVersionDates(startDate, endDate int64) error {
	const maxUnixTimestamp int64 = 253402300799
	if startDate < 0 || endDate < 0 || startDate > maxUnixTimestamp || endDate > maxUnixTimestamp {
		return errors.New(`version date timestamp is invalid`)
	}
	if startDate > 0 && endDate > 0 && startDate > endDate {
		return errors.New(`version start date must not be after end date`)
	}
	return nil
}

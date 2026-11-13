// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package validate

import (
	"errors"
	"math"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
)

// ErrPlannedDateRangeInvalid indicates the planned start date is later than the planned end date.
var ErrPlannedDateRangeInvalid = errors.New("planned start date cannot be later than planned end date")

// ErrTimezoneInvalid means the client timezone is missing or is not a valid IANA timezone.
var ErrTimezoneInvalid = errors.New("invalid timezone")

// ErrRequirementBatchInvalid indicates the field or value in a batch update is invalid.
var ErrRequirementBatchInvalid = errors.New("invalid requirement batch update")

// ErrRequirementWorkHoursInvalid indicates a work-hour value is out of range or has more than two decimal places.
var ErrRequirementWorkHoursInvalid = errors.New("invalid requirement work hours")

// ErrRequirementWorkDateInvalid indicates a work-log date is missing or invalid.
var ErrRequirementWorkDateInvalid = errors.New("invalid requirement work date")

// ErrRequirementHistorySideInvalid indicates the requested restore side is neither the value before nor after a recorded change.
var ErrRequirementHistorySideInvalid = errors.New("invalid requirement history restore side")

// MaxRequirementWorkHours is the largest value supported by DECIMAL(10,2).
const MaxRequirementWorkHours = 99999999.99

// ======================== Requirement Parameter Validation ========================

// Allowed requirement types.
var AllowedRequirementTypes = []string{define.TypeKeyRequirement, define.TypeKeyDefect}

// AllowedRequirementDescriptionModes lists the persisted description editor modes.
var AllowedRequirementDescriptionModes = []string{define.DescriptionModeRichText, define.DescriptionModeMarkdown}

// AllowedRequirementHistorySides lists the sides of a recorded description change that can be restored.
var AllowedRequirementHistorySides = []string{define.HistorySideOld, define.HistorySideNew}

// ValidateRequirementHistoryRestoreSide validates which side of a recorded description change to restore.
func ValidateRequirementHistoryRestoreSide(side string) error {
	if fe := IsInList("side", side, AllowedRequirementHistorySides); fe != nil {
		return fe
	}
	return nil
}

// ValidateRequirementCreate validates requirement creation.
func ValidateRequirementCreate(title, reqType string, priority int) error {
	if fe := NotBlank("title", title); fe != nil {
		return fe
	}
	if fe := MaxLen("title", title, 500); fe != nil {
		return fe
	}
	if len(reqType) > 0 {
		if fe := IsInList("type", reqType, AllowedRequirementTypes); fe != nil {
			return fe
		}
	}
	return nil
}

// ValidateRequirementUpdate validates requirement update.
func ValidateRequirementUpdate(title, reqType string, priority int) error {
	if len(title) > 0 {
		if fe := MaxLen("title", title, 500); fe != nil {
			return fe
		}
	}
	if len(reqType) > 0 {
		if fe := IsInList("type", reqType, AllowedRequirementTypes); fe != nil {
			return fe
		}
	}
	return nil
}

// ValidateRequirementType validates a requirement type.
func ValidateRequirementType(reqType string) error {
	if fe := IsInList("type", reqType, AllowedRequirementTypes); fe != nil {
		return fe
	}
	return nil
}

// ValidateRequirementDescriptionMode validates a persisted description editor mode.
func ValidateRequirementDescriptionMode(mode string) error {
	if fe := IsInList("description_mode", mode, AllowedRequirementDescriptionModes); fe != nil {
		return fe
	}
	return nil
}

// ValidateTimezone validates the IANA timezone submitted by the browser.
func ValidateTimezone(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 100 {
		return ErrTimezoneInvalid
	}
	if _, err := time.LoadLocation(value); err != nil {
		return ErrTimezoneInvalid
	}
	return nil
}

// ValidatePlannedDateRange validates the order of the planned start and end dates.
func ValidatePlannedDateRange(start, end int64) error {
	if start > 0 && end > 0 && start > end {
		return ErrPlannedDateRangeInvalid
	}
	return nil
}

// ValidateRequirementWorkHours validates a non-negative work-hour value with at most two decimal places.
func ValidateRequirementWorkHours(value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > MaxRequirementWorkHours {
		return ErrRequirementWorkHoursInvalid
	}
	if math.Abs(value*100-math.Round(value*100)) > 0.000001 {
		return ErrRequirementWorkHoursInvalid
	}
	return nil
}

// ValidateRequirementWorkDate validates a required Unix timestamp used as a work-log calendar date.
func ValidateRequirementWorkDate(value int64) error {
	return nil
}

// ValidateRequirementBatchUpdate validates the fields and values of a batch update.
func ValidateRequirementBatchUpdate(field string, value any) error {
	switch field {
	case `status_id`:
		valueInt, ok := value.(int)
		if !ok || valueInt <= 0 {
			return ErrRequirementBatchInvalid
		}
	case `priority`:
		valueInt, ok := value.(int)
		if !ok || valueInt < 0 {
			return ErrRequirementBatchInvalid
		}
	case `assignee_ids`:
		if _, ok := value.([]int); !ok {
			return ErrRequirementBatchInvalid
		}
	default:
		return ErrRequirementBatchInvalid
	}
	return nil
}

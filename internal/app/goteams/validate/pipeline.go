// Copyright © 2026 Wuhan Sesame Small Customer Service Network Co., Ltd.

package validate

import (
	"errors"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/define"
)

// ValidatePipelineCreate validates the parameters for creating a pipeline.
func ValidatePipelineCreate(name string, steps int) error {
	if name == `` {
		return errors.New(define.MsgPipelineNameRequired)
	}
	if steps == 0 {
		return errors.New(define.MsgPipelineWorkflowInvalid)
	}
	return nil
}

// ValidatePipelineScope validates the pipeline scope enum: members requires at least
// one member and departments requires at least one department; all ignores both lists.
func ValidatePipelineScope(scope string, memberCount, departmentCount int) error {
	switch scope {
	case define.PipelineScopeAll:
		return nil
	case define.PipelineScopeMembers:
		if memberCount == 0 {
			return errors.New(define.MsgPipelineMemberInvalid)
		}
	case define.PipelineScopeDepartments:
		if departmentCount == 0 {
			return errors.New(define.MsgPipelineDepartmentInvalid)
		}
	default:
		return errors.New(define.MsgPipelineScopeInvalid)
	}
	return nil
}

// ValidatePipelineUpdate validates the parameters for updating a pipeline.
func ValidatePipelineUpdate(name string) error {
	if name == `` {
		return errors.New(define.MsgPipelineNameRequired)
	}
	return nil
}

// ValidatePipelineWorkflow validates the parameters for updating a workflow.
func ValidatePipelineWorkflow(steps int) error {
	if steps == 0 {
		return errors.New(define.MsgPipelineWorkflowInvalid)
	}
	return nil
}

// ValidateDeviceRegister validates the device registration parameters.
func ValidateDeviceRegister(deviceUUID, osType string) error {
	if deviceUUID == `` {
		return errors.New(`device_uuid is required`)
	}
	if osType == `` {
		return errors.New(`os is required`)
	}
	return nil
}

// ValidatePipelineTaskPeriod validates the pipeline task period.
func ValidatePipelineTaskPeriod(period string) error {
	if period == `` ||
		period == define.PipelineTaskPeriodAll ||
		period == define.PipelineTaskPeriodToday ||
		period == define.PipelineTaskPeriodYesterday ||
		period == define.PipelineTaskPeriodWeek {
		return nil
	}
	return errors.New(`invalid pipeline task period`)
}

// ParsePipelineTaskListFilters parses and validates the pipeline task member and time-range parameters.
func ParsePipelineTaskListFilters(userIDsValue, startAtValue, endAtValue string) ([]int, int64, int64, error) {
	userIDs := make([]int, 0)
	seenUserIDs := make(map[int]struct{})
	if strings.TrimSpace(userIDsValue) != `` {
		for _, part := range strings.Split(userIDsValue, `,`) {
			userID, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil || userID <= 0 {
				return nil, 0, 0, errors.New(`user_ids must contain positive integers`)
			}
			if _, exists := seenUserIDs[userID]; exists {
				continue
			}
			seenUserIDs[userID] = struct{}{}
			userIDs = append(userIDs, userID)
		}
	}

	if startAtValue == `` && endAtValue == `` {
		return userIDs, 0, 0, nil
	}
	if startAtValue == `` || endAtValue == `` {
		return nil, 0, 0, errors.New(`start_at and end_at must be provided together`)
	}
	startAt, startErr := strconv.ParseInt(startAtValue, 10, 64)
	endAt, endErr := strconv.ParseInt(endAtValue, 10, 64)
	if startErr != nil || endErr != nil || startAt <= 0 || endAt <= startAt {
		return nil, 0, 0, errors.New(`invalid pipeline task time range`)
	}
	return userIDs, startAt, endAt, nil
}

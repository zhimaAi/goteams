// Copyright © 2026 Wuhan Sesame Small Customer Service Network Co., Ltd.

package validate

import (
	"errors"

	"goteams/internal/app/goteams/define"
)

// ValidateExpertGroupCreate validates the parameters for creating an expert group.
func ValidateExpertGroupCreate(name string, agents int) error {
	if name == `` {
		return errors.New(define.MsgExpertGroupNameRequired)
	}
	if agents == 0 {
		return errors.New(define.MsgExpertGroupNoAgent)
	}
	return nil
}

// ValidateExpertGroupUpdate validates the parameters for updating an expert group.
func ValidateExpertGroupUpdate(name string, agents int) error {
	if name == `` {
		return errors.New(define.MsgExpertGroupNameRequired)
	}
	if agents == 0 {
		return errors.New(define.MsgExpertGroupNoAgent)
	}
	return nil
}

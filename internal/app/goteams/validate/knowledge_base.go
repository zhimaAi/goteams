// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package validate

import (
	"errors"
	"strings"
)

// Knowledge item title limits (title column is VARCHAR(200)).
const (
	KnowledgeTitleMaxLen = 200
)

// Knowledge content size limit for editable documents and autosave (10 MiB).
// It is deliberately lower than the file upload limit: content is parsed into the document body
// (and stored/autosaved as text), so a larger txt/md file is kept as a file attachment instead.
const KnowledgeContentMaxBytes = 10 * 1024 * 1024

// Knowledge file upload size limit (200 MiB).
const KnowledgeFileUploadMaxBytes = 200 * 1024 * 1024

// Knowledge version description limit (description column is VARCHAR(500)).
const KnowledgeVersionDescMaxLen = 500

var (
	// ErrKnowledgeTitleInvalid indicates an empty or over-long item title.
	ErrKnowledgeTitleInvalid = errors.New(`knowledge title invalid`)
	// ErrKnowledgeContentTooLarge indicates the content body exceeds the size limit.
	ErrKnowledgeContentTooLarge = errors.New(`knowledge content too large`)
	// ErrKnowledgeVersionDescTooLong indicates the version description exceeds the limit.
	ErrKnowledgeVersionDescTooLong = errors.New(`knowledge version description too long`)
)

// ValidateKnowledgeTitle checks the title length (trimmed, 1..200).
func ValidateKnowledgeTitle(title string) error {
	if n := len([]rune(strings.TrimSpace(title))); n < 1 || n > KnowledgeTitleMaxLen {
		return ErrKnowledgeTitleInvalid
	}
	return nil
}

// ValidateKnowledgeContentSize checks the content byte size (10 MiB limit).
func ValidateKnowledgeContentSize(content string) error {
	if len([]byte(content)) > KnowledgeContentMaxBytes {
		return ErrKnowledgeContentTooLarge
	}
	return nil
}

// ValidateKnowledgeVersionDescription checks the manual version description length.
func ValidateKnowledgeVersionDescription(description string) error {
	if n := len([]rune(description)); n > KnowledgeVersionDescMaxLen {
		return ErrKnowledgeVersionDescTooLong
	}
	return nil
}

// ValidateKnowledgeFileUploadSize checks the local file upload size (200 MiB limit).
func ValidateKnowledgeFileUploadSize(size int64) error {
	if size < 0 || size > KnowledgeFileUploadMaxBytes {
		return ErrKnowledgeContentTooLarge
	}
	return nil
}

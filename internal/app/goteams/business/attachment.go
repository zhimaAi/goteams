// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"errors"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_storage"

	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

const maxAttachmentSize int64 = 200 * 1024 * 1024

var ErrAttachmentNotFound = errors.New(`attachment not found`)

// AttachmentMaxSize returns the size limit for a single attachment.
func AttachmentMaxSize() int64 { return maxAttachmentSize }

// CreateAttachment saves the attachment (prefers oss-s3, falls back to local when unavailable) and writes the attachment record.
func CreateAttachment(adminID, workspaceID, requirementID, commentID, userID int, file *multipart.FileHeader) (msql.Params, error) {
	if file == nil || file.Size < 0 || file.Size > maxAttachmentSize {
		return nil, errors.New(`attachment too large`)
	}
	// Validate enterprise storage quota before upload. Returns QuotaExceededError if exceeded (controller converts to 403 to trigger upgrade modal).
	if err := CheckStorageQuota(adminID, file.Size); err != nil {
		return nil, err
	}
	opened, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = opened.Close() }()
	ext := filepath.Ext(file.Filename)
	storedPath, err := lib_storage.M.Upload(adminID, lib_storage.BusinessRequirement, ext, opened)
	if err != nil {
		return nil, err
	}
	_, key, _ := lib_storage.ParseStored(storedPath)
	storedName := filepath.Base(key)
	now := tool.Time2Int()
	sourceType, sourceName, uploaderName := resolveFileSource(adminID, requirementID, userID)
	tableName := define.GetEnterpriseFileTable(adminID)
	m := msql.Model(tableName, define.Postgres)
	id, err := m.Insert(msql.Datas{
		`admin_id`: adminID, `workspace_id`: workspaceID,
		`source_type`: sourceType, `source_id`: requirementID, `source_name`: sourceName,
		`comment_id`: commentID, `file_name`: file.Filename,
		`file_ext`:  strings.TrimPrefix(strings.ToLower(ext), `.`),
		`file_size`: file.Size, `stored_path`: storedPath, `content_type`: file.Header.Get(`Content-Type`),
		`uploader_id`: userID, `uploader_name`: uploaderName,
		`created_at`: now, `updated_at`: now,
		`stored_name`: storedName,
		`folder_id`:   0, `document_id`: 0,
	}, `id`)
	if err != nil {
		_ = lib_storage.M.Delete(storedPath)
		logs.Error("CreateAttachment sql:%s, err:%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	// Hook: sync-write to enterprise file ledger shard (async persistence failure does not affect the main flow).
	go HookRecordEnterpriseFile(adminID, workspaceID, requirementID, commentID, userID, file.Filename, storedPath, file.Header.Get(`Content-Type`), file.Size)

	return msql.Params{`id`: fmt.Sprintf(`%d`, id), `original_name`: file.Filename, `stored_name`: storedName,
		`content_type`: file.Header.Get(`Content-Type`), `file_size`: fmt.Sprintf(`%d`, file.Size), `comment_id`: fmt.Sprintf(`%d`, commentID),
		`url`: storedPath}, nil
}

// CreatePendingAttachment stores a file as an unbound enterprise file record. The caller
// must later call BindPendingEnterpriseFiles once the owning work-item id is known.
func CreatePendingAttachment(adminID, workspaceID, uploaderID int, file *multipart.FileHeader) (msql.Params, error) {
	if file == nil || file.Size < 0 || file.Size > maxAttachmentSize {
		return nil, errors.New(`attachment too large`)
	}
	opened, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = opened.Close() }()
	ext := filepath.Ext(file.Filename)
	storedPath, err := lib_storage.M.Upload(adminID, lib_storage.BusinessRequirement, ext, opened)
	if err != nil {
		return nil, err
	}
	id, err := RecordPendingEnterpriseFile(adminID, workspaceID, uploaderID, file.Filename, storedPath, file.Header.Get(`Content-Type`), file.Size)
	if err != nil {
		_ = lib_storage.M.Delete(storedPath)
		return nil, err
	}
	return msql.Params{
		`id`:   fmt.Sprintf(`%d`, id),
		`url`:  storedPath,
		`name`: file.Filename,
		`size`: fmt.Sprintf(`%d`, file.Size),
	}, nil
}

// ListAttachments returns the requirement attachment list.
// The file_name column is also exposed as original_name to keep the legacy API shape.
func ListAttachments(adminID, workspaceID, requirementID int) ([]msql.Params, error) {
	tableName := define.GetEnterpriseFileTable(adminID)
	list, err := msql.RawValues(define.Postgres, `SELECT a.* FROM `+tableName+` a
		WHERE a.admin_id = $1 AND a.workspace_id = $2 AND a.source_id = $3 ORDER BY a.id DESC`, nil,
		adminID, workspaceID, requirementID)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if name, ok := list[i][`file_name`]; ok {
			list[i][`original_name`] = name
		}
	}
	return list, nil
}

// GetAttachment returns a single attachment record (including stored_path, for Serve).
// The file_name column is also exposed as original_name to keep the legacy API shape.
func GetAttachment(adminID, workspaceID, requirementID, attachmentID int) (msql.Params, error) {
	tableName := define.GetEnterpriseFileTable(adminID)
	list, err := msql.RawValues(define.Postgres, `SELECT * FROM `+tableName+`
		WHERE admin_id = $1 AND workspace_id = $2 AND source_id = $3 AND id = $4 LIMIT 1`, nil,
		adminID, workspaceID, requirementID, attachmentID)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrAttachmentNotFound
	}
	row := list[0]
	if name, ok := row[`file_name`]; ok {
		row[`original_name`] = name
	}
	return row, nil
}

// DeleteAttachment deletes the attachment record and the local file, and syncs the enterprise file ledger.
func DeleteAttachment(adminID, workspaceID, requirementID, attachmentID int) error {
	attachment, err := GetAttachment(adminID, workspaceID, requirementID, attachmentID)
	if err != nil {
		return err
	}
	tableName := define.GetEnterpriseFileTable(adminID)
	_, err = msql.RawExec(define.Postgres, `DELETE FROM `+tableName+`
		WHERE admin_id = $1 AND workspace_id = $2 AND source_id = $3 AND id = $4`, nil,
		adminID, workspaceID, requirementID, attachmentID)
	if err != nil {
		return err
	}
	if path := attachment[`stored_path`]; path != `` {
		if err = lib_storage.M.Delete(path); err != nil {
			logs.Error("DeleteAttachment remove file failed, err:%s", err.Error())
		}
		// Hook: sync-delete enterprise file ledger record.
		go HookDeleteEnterpriseFile(adminID, path)
	}
	return nil
}

// DeletePendingAttachment removes an unbound attachment record and its stored file.
// Only rows still carrying source_id = 0 may be deleted, so a file already bound to a
// submitted work item can never be removed through this path.
func DeletePendingAttachment(adminID, workspaceID, attachmentID int) error {
	tableName := define.GetEnterpriseFileTable(adminID)
	rows, err := msql.RawValues(define.Postgres, `SELECT stored_path FROM `+tableName+`
		WHERE admin_id = $1 AND workspace_id = $2 AND source_id = 0 AND id = $3 LIMIT 1`, nil,
		adminID, workspaceID, attachmentID)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return ErrAttachmentNotFound
	}
	_, err = msql.RawExec(define.Postgres, `DELETE FROM `+tableName+`
		WHERE admin_id = $1 AND workspace_id = $2 AND source_id = 0 AND id = $3`, nil,
		adminID, workspaceID, attachmentID)
	if err != nil {
		logs.Error("DeletePendingAttachment sql failed, err:%s", err.Error())
		return err
	}
	if path := rows[0][`stored_path`]; path != `` {
		if err = lib_storage.M.Delete(path); err != nil {
			logs.Error("DeletePendingAttachment remove file failed, err:%s", err.Error())
		}
	}
	return nil
}

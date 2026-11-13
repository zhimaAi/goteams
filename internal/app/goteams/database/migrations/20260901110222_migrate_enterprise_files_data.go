// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

// Package migrations registers the Go-based goose migrations of the service.
package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pressly/goose/v3"
)

// enterpriseFileShardCount matches the shard tables created by 20260724100000_add_enterprise_files.sql.
const enterpriseFileShardCount = 10

// knowledgeSourceType is the source_type value of knowledge base documents (see 20260901103359_add_enterprise_files_columns.sql).
const knowledgeSourceType = `knowledge`

func init() {
	goose.AddNamedMigrationContext(`20260901110222_migrate_enterprise_files_data.go`, upMigrateEnterpriseFilesData, downMigrateEnterpriseFilesData)
}

// legacyAttachmentRow is one storage_attachments row enriched with the source work item and uploader info.
type legacyAttachmentRow struct {
	ID             int64
	AdminID        int
	WorkspaceID    int
	RequirementID  int
	CommentID      int
	OriginalName   string
	StoredName     string
	StoredPath     string
	ContentType    string
	FileSize       int64
	CreatedAt      int64
	UpdatedAt      int64
	AttachmentType string
	FolderID       int
	DocumentID     int
	UploadedBy     int
	SourceType     string
	SourceName     string
	UploaderName   string
}

// upMigrateEnterpriseFilesData migrates the legacy storage_attachments rows into the enterprise_files_xx
// shard tables (deduplicated by admin_id + stored_path) and remaps
// knowledge_document_contents.source_attachment_id to the new shard ids.
func upMigrateEnterpriseFilesData(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT s.id, s.admin_id, s.workspace_id, s.requirement_id, s.comment_id,
		       s.original_name, s.stored_name, s.stored_path, s.content_type,
		       s.file_size, s.created_at, s.updated_at, s.attachment_type,
		       s.folder_id, s.document_id, s.uploaded_by,
		       COALESCE(r.type, ''), COALESCE(r.title, ''), COALESCE(u.name, '')
		FROM storage_attachments s
		LEFT JOIN requirements r ON r.admin_id = s.admin_id AND r.id = s.requirement_id
		LEFT JOIN users u ON u.admin_id = s.admin_id AND u.id = s.uploaded_by
		ORDER BY s.id`)
	if err != nil {
		return fmt.Errorf("query storage_attachments: %w", err)
	}

	// lib/pq forbids running another query on the same connection while a result set
	// is still open, so buffer all scanned rows first and close the cursor before
	// issuing the per-row dedup/insert queries on the same tx below.
	var legacyRows []legacyAttachmentRow
	for rows.Next() {
		var row legacyAttachmentRow
		var contentType sql.NullString
		if err := rows.Scan(&row.ID, &row.AdminID, &row.WorkspaceID, &row.RequirementID, &row.CommentID,
			&row.OriginalName, &row.StoredName, &row.StoredPath, &contentType,
			&row.FileSize, &row.CreatedAt, &row.UpdatedAt, &row.AttachmentType,
			&row.FolderID, &row.DocumentID, &row.UploadedBy,
			&row.SourceType, &row.SourceName, &row.UploaderName); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan storage_attachments row: %w", err)
		}
		row.ContentType = contentType.String
		// source_type takes over the attachment_type semantics: knowledge attachments are
		// tagged knowledge, work item attachments keep requirements.type (requirement/defect).
		if row.AttachmentType == knowledgeSourceType {
			row.SourceType = knowledgeSourceType
		}
		legacyRows = append(legacyRows, row)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate storage_attachments: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close storage_attachments rows: %w", err)
	}

	// remap keeps the old storage_attachments.id -> new shard id mapping for the remap step below.
	remap := make(map[int64]int64, len(legacyRows))
	for _, row := range legacyRows {
		table := fmt.Sprintf(`enterprise_files_%d`, row.AdminID%enterpriseFileShardCount)

		// Deduplicate by admin_id + stored_path so a re-run is a no-op for already
		// migrated rows; rows already present in the ledger (older version) keep their id.
		var newID int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM `+table+` WHERE admin_id=$1 AND stored_path=$2`,
			row.AdminID, row.StoredPath).Scan(&newID)
		switch {
		case err == sql.ErrNoRows:
			if err := tx.QueryRowContext(ctx, `
				INSERT INTO `+table+`
					(admin_id, workspace_id, source_type, source_id, source_name, comment_id,
					 file_name, file_ext, file_size, stored_path, content_type,
					 uploader_id, uploader_name, created_at, updated_at,
					 stored_name, folder_id, document_id)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18) RETURNING id`,
				row.AdminID, row.WorkspaceID, row.SourceType, row.RequirementID, row.SourceName, row.CommentID,
				row.OriginalName, legacyFileExt(row.OriginalName), row.FileSize, row.StoredPath, row.ContentType,
				row.UploadedBy, row.UploaderName, row.CreatedAt, row.UpdatedAt,
				row.StoredName, row.FolderID, row.DocumentID).Scan(&newID); err != nil {
				return fmt.Errorf("insert storage_attachments row %d into %s: %w", row.ID, table, err)
			}
		case err != nil:
			return fmt.Errorf("query %s for stored_path %s: %w", table, row.StoredPath, err)
		}
		remap[row.ID] = newID
	}

	return remapKnowledgeSourceAttachments(ctx, tx, remap)
}

// remapKnowledgeSourceAttachments repoints knowledge_document_contents.source_attachment_id from the
// legacy storage_attachments.id to the shard id (matched by admin_id + stored_path, which covers both
// migration-inserted rows and pre-existing ledger rows).
func remapKnowledgeSourceAttachments(ctx context.Context, tx *sql.Tx, remap map[int64]int64) error {
	rows, err := tx.QueryContext(ctx, `SELECT id, admin_id, source_attachment_id
		FROM knowledge_document_contents WHERE source_attachment_id > 0`)
	if err != nil {
		return fmt.Errorf("query knowledge_document_contents: %w", err)
	}

	// Buffer the rows and close the cursor first: the same lib/pq restriction as above,
	// an UPDATE cannot run on the tx connection while the result set is still open.
	var docRows []knowledgeDocRow
	for rows.Next() {
		var r knowledgeDocRow
		if err := rows.Scan(&r.id, &r.adminID, &r.oldID); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan knowledge_document_contents row: %w", err)
		}
		docRows = append(docRows, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate knowledge_document_contents: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close knowledge_document_contents rows: %w", err)
	}

	for _, r := range docRows {
		newID, ok := remap[r.oldID]
		if !ok || newID == r.oldID {
			continue
		}
		// The old id condition keeps the update a no-op when the row was already remapped.
		if _, err := tx.ExecContext(ctx, `UPDATE knowledge_document_contents
			SET source_attachment_id=$1 WHERE admin_id=$2 AND id=$3 AND source_attachment_id=$4`,
			newID, r.adminID, r.id, r.oldID); err != nil {
			return fmt.Errorf("remap knowledge_document_contents %d: %w", r.id, err)
		}
	}
	return nil
}

// knowledgeDocRow is one knowledge_document_contents row pending source_attachment_id remap.
type knowledgeDocRow struct {
	id      int64
	adminID int64
	oldID   int64
}

// legacyFileExt returns the lowercase extension without the leading dot, matching the
// file_ext format written by the business code.
func legacyFileExt(name string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), `.`)
}

// downMigrateEnterpriseFilesData intentionally does not restore the migrated data:
// the legacy storage_attachments rows are kept as archive, and removing the copied rows
// here would risk deleting ledger rows that were not written by this migration.
func downMigrateEnterpriseFilesData(ctx context.Context, tx *sql.Tx) error {
	return nil
}

// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"errors"
	"path/filepath"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_storage"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// SaveKnowledgeDocumentModel is the autosave request/output model.
type SaveKnowledgeDocumentModel struct {
	AdminId       int
	WorkspaceId   int
	DocumentId    int
	UserId        int
	Revision      int64
	Title         string
	Content       string
	ContentMode   string
	EditSessionId string

	// Unchanged is true when the request content matches the current server state (no-op).
	Unchanged bool
	// NewRevision is the revision after a successful save.
	NewRevision int64
	// ContentHash is the server-computed hash after the save.
	ContentHash string
	// UpdatedAt is the server-side update time (Unix seconds).
	UpdatedAt int64
	// UpdatedBy is the last modifier (server state on success/conflict).
	UpdatedBy int
}

// isKnowledgeEditableContentMode reports whether a document mode can be edited and autosaved.
func isKnowledgeEditableContentMode(mode string) bool {
	return mode == define.KnowledgeContentModeRichText || mode == define.KnowledgeContentModeMarkdown
}

// SaveKnowledgeDocumentContent persists the current document state under a revision lock:
// SELECT FOR UPDATE → revision check → hash no-op → update items + contents, revision + 1.
// The conflict case is returned as *KnowledgeRevisionConflictError with the current server state.
func SaveKnowledgeDocumentContent(model SaveKnowledgeDocumentModel) (SaveKnowledgeDocumentModel, error) {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin save knowledge document transaction failed, err:%s", err.Error())
		return SaveKnowledgeDocumentModel{}, err
	}
	defer func() { _ = tx.Rollback() }()

	lockRows, err := msql.RawValues(define.Postgres, `SELECT ki.id, ki.title, ki.revision, ki.creator_id, ki.updated_by, ki.updated_at,
		c.content, c.content_mode, c.content_hash
		FROM knowledge_items ki
		JOIN knowledge_document_contents c ON c.admin_id = ki.admin_id AND c.workspace_id = ki.workspace_id AND c.document_id = ki.id
		WHERE ki.admin_id = $1 AND ki.workspace_id = $2 AND ki.id = $3 AND ki.deleted_at = 0
		FOR UPDATE OF ki`, tx, model.AdminId, model.WorkspaceId, model.DocumentId)
	if err != nil {
		logs.Error("lock knowledge document for save failed, err:%s", err.Error())
		return SaveKnowledgeDocumentModel{}, err
	}
	if len(lockRows) == 0 {
		return SaveKnowledgeDocumentModel{}, ErrKnowledgeItemNotFound
	}
	current := lockRows[0]
	item := toKnowledgeItemModel(msql.Params{
		`id`: current[`id`], `parent_id`: `0`, `item_type`: define.KnowledgeItemTypeDocument,
		`title`: current[`title`], `revision`: current[`revision`], `version_seq`: `0`,
		`creator_id`: current[`creator_id`], `creator_name`: ``, `updated_by`: current[`updated_by`],
		`created_at`: `0`, `updated_at`: current[`updated_at`],
	})
	if !canEditKnowledgeItem(model.AdminId, model.UserId, cast.ToString(model.WorkspaceId), item) {
		return SaveKnowledgeDocumentModel{}, ErrForbidden
	}
	if !isKnowledgeEditableContentMode(current[`content_mode`]) || !isKnowledgeEditableContentMode(model.ContentMode) {
		return SaveKnowledgeDocumentModel{}, ErrKnowledgeDocumentNotEditable
	}

	currentRevision := cast.ToInt64(current[`revision`])
	if currentRevision != model.Revision {
		return SaveKnowledgeDocumentModel{}, &KnowledgeRevisionConflictError{
			CurrentRevision: currentRevision,
			UpdatedAt:       cast.ToInt64(current[`updated_at`]),
			UpdatedBy:       cast.ToInt(current[`updated_by`]),
		}
	}

	model.Title = lib_storage.StripOSSURLSignatures(model.Title)
	model.Content = lib_storage.StripOSSURLSignatures(model.Content)
	nextHash := knowledgeContentHash(model.Title, model.ContentMode, model.Content)
	if nextHash == current[`content_hash`] {
		if err = tx.Commit(); err != nil {
			logs.Error("commit knowledge document no-op transaction failed, err:%s", err.Error())
			return SaveKnowledgeDocumentModel{}, err
		}
		model.Unchanged = true
		model.NewRevision = currentRevision
		model.ContentHash = current[`content_hash`]
		model.UpdatedAt = cast.ToInt64(current[`updated_at`])
		model.UpdatedBy = cast.ToInt(current[`updated_by`])
		return model, nil
	}

	now := int64(tool.Time2Int())
	if _, err = msql.RawExec(define.Postgres, `UPDATE knowledge_items
		SET title = $1, revision = revision + 1, updated_by = $2, updated_at = $3
		WHERE admin_id = $4 AND workspace_id = $5 AND id = $6`,
		tx, model.Title, model.UserId, now, model.AdminId, model.WorkspaceId, model.DocumentId); err != nil {
		logs.Error("update knowledge item for save failed, err:%s", err.Error())
		return SaveKnowledgeDocumentModel{}, err
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE knowledge_document_contents
		SET content = $1, content_mode = $2, content_hash = $3, updated_at = $4
		WHERE admin_id = $5 AND workspace_id = $6 AND document_id = $7`,
		tx, model.Content, model.ContentMode, nextHash, now, model.AdminId, model.WorkspaceId, model.DocumentId); err != nil {
		logs.Error("update knowledge document contents for save failed, err:%s", err.Error())
		return SaveKnowledgeDocumentModel{}, err
	}
	if err = writeKnowledgeActivity(tx, model.AdminId, model.WorkspaceId, model.DocumentId, model.UserId,
		model.Title, define.KnowledgeItemTypeDocument, define.KnowledgeActionUpdate, now); err != nil {
		return SaveKnowledgeDocumentModel{}, err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit save knowledge document transaction failed, err:%s", err.Error())
		return SaveKnowledgeDocumentModel{}, err
	}
	model.NewRevision = currentRevision + 1
	model.ContentHash = nextHash
	model.UpdatedAt = now
	model.UpdatedBy = model.UserId
	return model, nil
}

// ConvertKnowledgeUploadDocumentModel is the convert-upload-to-editable request model.
type ConvertKnowledgeUploadDocumentModel struct {
	AdminId     int
	WorkspaceId int
	DocumentId  int
	UserId      int
	Revision    int64
	Title       string
	Content     string
}

// ConvertKnowledgeUploadDocument converts an uploaded docx document (content_mode=file) into an
// editable rich-text document under a revision lock: SELECT FOR UPDATE → revision check → update
// items + contents (content_mode becomes rich-text, the source attachment is kept) → snapshot a
// convert version. The original file remains untouched as the source attachment.
func ConvertKnowledgeUploadDocument(model ConvertKnowledgeUploadDocumentModel) (KnowledgeItemModel, error) {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin convert knowledge document transaction failed, err:%s", err.Error())
		return KnowledgeItemModel{}, err
	}
	defer func() { _ = tx.Rollback() }()

	lockRows, err := msql.RawValues(define.Postgres, `SELECT ki.id, ki.title, ki.revision, ki.version_seq, ki.creator_id, ki.updated_by, ki.updated_at,
		c.content_mode, c.content_hash, c.source_type, a.file_name AS source_file_name
		FROM knowledge_items ki
		JOIN knowledge_document_contents c ON c.admin_id = ki.admin_id AND c.workspace_id = ki.workspace_id AND c.document_id = ki.id
		LEFT JOIN `+define.GetEnterpriseFileTable(model.AdminId)+` a ON a.admin_id = c.admin_id AND a.workspace_id = c.workspace_id AND a.id = c.source_attachment_id
		WHERE ki.admin_id = $1 AND ki.workspace_id = $2 AND ki.id = $3 AND ki.deleted_at = 0
		FOR UPDATE OF ki`, tx, model.AdminId, model.WorkspaceId, model.DocumentId)
	if err != nil {
		logs.Error("lock knowledge document for convert failed, err:%s", err.Error())
		return KnowledgeItemModel{}, err
	}
	if len(lockRows) == 0 {
		return KnowledgeItemModel{}, ErrKnowledgeItemNotFound
	}
	current := lockRows[0]
	item := toKnowledgeItemModel(msql.Params{
		`id`: current[`id`], `parent_id`: `0`, `item_type`: define.KnowledgeItemTypeDocument,
		`title`: current[`title`], `revision`: current[`revision`], `version_seq`: current[`version_seq`],
		`creator_id`: current[`creator_id`], `creator_name`: ``, `updated_by`: current[`updated_by`],
		`created_at`: `0`, `updated_at`: current[`updated_at`],
	})
	if !canEditKnowledgeItem(model.AdminId, model.UserId, cast.ToString(model.WorkspaceId), item) {
		return KnowledgeItemModel{}, ErrForbidden
	}
	if current[`content_mode`] != define.KnowledgeContentModeFile || current[`source_type`] != define.KnowledgeSourceTypeUpload {
		return KnowledgeItemModel{}, ErrKnowledgeDocumentNotEditable
	}
	if !strings.EqualFold(filepath.Ext(current[`source_file_name`]), `.docx`) {
		return KnowledgeItemModel{}, ErrKnowledgeDocumentNotEditable
	}
	currentRevision := cast.ToInt64(current[`revision`])
	if currentRevision != model.Revision {
		return KnowledgeItemModel{}, &KnowledgeRevisionConflictError{
			CurrentRevision: currentRevision,
			UpdatedAt:       cast.ToInt64(current[`updated_at`]),
			UpdatedBy:       cast.ToInt(current[`updated_by`]),
		}
	}

	model.Title = lib_storage.StripOSSURLSignatures(model.Title)
	model.Content = lib_storage.StripOSSURLSignatures(model.Content)
	now := int64(tool.Time2Int())
	nextHash := knowledgeContentHash(model.Title, define.KnowledgeContentModeRichText, model.Content)
	if _, err = msql.RawExec(define.Postgres, `UPDATE knowledge_items
		SET title = $1, revision = revision + 1, version_seq = $2, updated_by = $3, updated_at = $4
		WHERE admin_id = $5 AND workspace_id = $6 AND id = $7`,
		tx, model.Title, cast.ToInt64(current[`version_seq`])+1, model.UserId, now, model.AdminId, model.WorkspaceId, model.DocumentId); err != nil {
		logs.Error("update knowledge item for convert failed, err:%s", err.Error())
		return KnowledgeItemModel{}, err
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE knowledge_document_contents
		SET content = $1, content_mode = $2, content_hash = $3, updated_at = $4
		WHERE admin_id = $5 AND workspace_id = $6 AND document_id = $7`,
		tx, model.Content, define.KnowledgeContentModeRichText, nextHash, now, model.AdminId, model.WorkspaceId, model.DocumentId); err != nil {
		logs.Error("update knowledge document contents for convert failed, err:%s", err.Error())
		return KnowledgeItemModel{}, err
	}
	if _, err = insertKnowledgeVersionSnapshot(tx, model.AdminId, model.WorkspaceId, model.DocumentId, model.UserId,
		cast.ToInt64(current[`version_seq`])+1, define.KnowledgeVersionTypeConvert, ``, ``, 0,
		model.Title, model.Content, define.KnowledgeContentModeRichText, nextHash, now); err != nil {
		return KnowledgeItemModel{}, err
	}
	if err = writeKnowledgeActivity(tx, model.AdminId, model.WorkspaceId, model.DocumentId, model.UserId,
		model.Title, define.KnowledgeItemTypeDocument, define.KnowledgeActionUpdate, now); err != nil {
		return KnowledgeItemModel{}, err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit convert knowledge document transaction failed, err:%s", err.Error())
		return KnowledgeItemModel{}, err
	}
	return GetKnowledgeItemDetail(model.AdminId, model.WorkspaceId, model.DocumentId, model.UserId)
}

// KnowledgeVersionModel is one version row (metadata plus snapshot fields for the detail view).
type KnowledgeVersionModel struct {
	Id                   int
	DocumentId           int
	VersionNo            int64
	VersionType          string
	Description          string
	RestoreFromVersionId int64
	EditSessionId        string
	CreatedBy            int
	CreatedByName        string
	CreatedAt            int64
	Title                string
	Content              string
	ContentMode          string
	ContentHash          string
	// Created reports whether the manual version request inserted a new snapshot.
	Created bool
}

// GetKnowledgeVersionList returns the paginated version metadata list (no snapshot body).
func GetKnowledgeVersionList(adminId, wsId, documentId, page, size int) ([]KnowledgeVersionModel, int, error) {
	if _, err := loadKnowledgeItem(adminId, wsId, documentId); err != nil {
		return nil, 0, err
	}
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	offset := (page - 1) * size
	m := msql.Model(define.TableKnowledgeDocumentVersions, define.Postgres)
	total, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, cast.ToString(wsId)},
		{`document_id`, cast.ToString(documentId)},
	}).Count()
	if err != nil {
		logs.Error("count knowledge versions sql:%s, err:%s", m.GetLastSql(), err.Error())
		return nil, 0, err
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT v.id, v.document_id, v.version_no, v.version_type, v.description,
		v.restore_from_version_id, v.edit_session_id, v.created_by, v.created_at, u.name AS created_by_name
		FROM knowledge_document_versions v LEFT JOIN users u ON u.id = v.created_by
		WHERE v.admin_id = $1 AND v.workspace_id = $2 AND v.document_id = $3
		ORDER BY v.version_no DESC LIMIT $4 OFFSET $5`, nil, adminId, wsId, documentId, size, offset)
	if err != nil {
		logs.Error("list knowledge versions failed, err:%s", err.Error())
		return nil, 0, err
	}
	items := make([]KnowledgeVersionModel, 0, len(rows))
	for _, row := range rows {
		items = append(items, toKnowledgeVersionModel(row))
	}
	return items, total, nil
}

// GetKnowledgeVersionDetail returns one version with its full snapshot.
func GetKnowledgeVersionDetail(adminId, wsId, documentId, versionId int) (KnowledgeVersionModel, error) {
	if _, err := loadKnowledgeItem(adminId, wsId, documentId); err != nil {
		return KnowledgeVersionModel{}, err
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT v.id, v.document_id, v.version_no, v.version_type, v.description,
		v.restore_from_version_id, v.edit_session_id, v.created_by, v.created_at, u.name AS created_by_name,
		vc.title AS snapshot_title, vc.content AS snapshot_content, vc.content_mode AS snapshot_content_mode, vc.content_hash AS snapshot_content_hash
		FROM knowledge_document_versions v
		LEFT JOIN users u ON u.id = v.created_by
		JOIN knowledge_document_version_contents vc ON vc.admin_id = v.admin_id AND vc.workspace_id = v.workspace_id AND vc.version_id = v.id
		WHERE v.admin_id = $1 AND v.workspace_id = $2 AND v.document_id = $3 AND v.id = $4 LIMIT 1`,
		nil, adminId, wsId, documentId, versionId)
	if err != nil {
		logs.Error("get knowledge version detail failed, err:%s", err.Error())
		return KnowledgeVersionModel{}, err
	}
	if len(rows) == 0 {
		return KnowledgeVersionModel{}, ErrKnowledgeVersionNotFound
	}
	model := toKnowledgeVersionModel(rows[0])
	model.Title = rows[0][`snapshot_title`]
	model.Content = rows[0][`snapshot_content`]
	model.ContentMode = rows[0][`snapshot_content_mode`]
	model.ContentHash = rows[0][`snapshot_content_hash`]
	return model, nil
}

// toKnowledgeVersionModel converts a version data row into the model (metadata only).
func toKnowledgeVersionModel(row msql.Params) KnowledgeVersionModel {
	return KnowledgeVersionModel{
		Id:                   cast.ToInt(row[`id`]),
		DocumentId:           cast.ToInt(row[`document_id`]),
		VersionNo:            cast.ToInt64(row[`version_no`]),
		VersionType:          row[`version_type`],
		Description:          row[`description`],
		RestoreFromVersionId: cast.ToInt64(row[`restore_from_version_id`]),
		EditSessionId:        row[`edit_session_id`],
		CreatedBy:            cast.ToInt(row[`created_by`]),
		CreatedByName:        row[`created_by_name`],
		CreatedAt:            cast.ToInt64(row[`created_at`]),
	}
}

// CreateKnowledgeVersionModel is the manual version creation model.
type CreateKnowledgeVersionModel struct {
	AdminId       int
	WorkspaceId   int
	DocumentId    int
	UserId        int
	Revision      int64
	Description   string
	EditSessionId string
}

// shouldCreateKnowledgeVersion reports whether the current snapshot differs from the latest stored version.
func shouldCreateKnowledgeVersion(latestExists bool, latestHash, currentHash string) bool {
	return !latestExists || latestHash != currentHash
}

// CreateKnowledgeVersion creates a manual snapshot only when the current state differs from the latest version.
func CreateKnowledgeVersion(model CreateKnowledgeVersionModel) (KnowledgeVersionModel, error) {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin create knowledge version transaction failed, err:%s", err.Error())
		return KnowledgeVersionModel{}, err
	}
	defer func() { _ = tx.Rollback() }()

	lockRows, err := msql.RawValues(define.Postgres, `SELECT ki.id, ki.title, ki.revision, ki.version_seq, ki.creator_id, ki.updated_by, ki.updated_at,
		c.content, c.content_mode, c.content_hash
		FROM knowledge_items ki
		JOIN knowledge_document_contents c ON c.admin_id = ki.admin_id AND c.workspace_id = ki.workspace_id AND c.document_id = ki.id
		WHERE ki.admin_id = $1 AND ki.workspace_id = $2 AND ki.id = $3 AND ki.deleted_at = 0
		FOR UPDATE OF ki`, tx, model.AdminId, model.WorkspaceId, model.DocumentId)
	if err != nil {
		logs.Error("lock knowledge document for manual version failed, err:%s", err.Error())
		return KnowledgeVersionModel{}, err
	}
	if len(lockRows) == 0 {
		return KnowledgeVersionModel{}, ErrKnowledgeItemNotFound
	}
	current := lockRows[0]
	item := toKnowledgeItemModel(msql.Params{
		`id`: current[`id`], `parent_id`: `0`, `item_type`: define.KnowledgeItemTypeDocument,
		`title`: current[`title`], `revision`: current[`revision`], `version_seq`: current[`version_seq`],
		`creator_id`: current[`creator_id`], `creator_name`: ``, `updated_by`: current[`updated_by`],
		`created_at`: `0`, `updated_at`: current[`updated_at`],
	})
	if !canEditKnowledgeItem(model.AdminId, model.UserId, cast.ToString(model.WorkspaceId), item) {
		return KnowledgeVersionModel{}, ErrForbidden
	}
	currentRevision := cast.ToInt64(current[`revision`])
	if currentRevision != model.Revision {
		return KnowledgeVersionModel{}, &KnowledgeRevisionConflictError{
			CurrentRevision: currentRevision,
			UpdatedAt:       cast.ToInt64(current[`updated_at`]),
			UpdatedBy:       cast.ToInt(current[`updated_by`]),
		}
	}

	current[`title`] = lib_storage.StripOSSURLSignatures(current[`title`])
	current[`content`] = lib_storage.StripOSSURLSignatures(current[`content`])
	current[`content_hash`] = knowledgeContentHash(current[`title`], current[`content_mode`], current[`content`])
	latestRows, err := msql.RawValues(define.Postgres, `SELECT v.id, vc.content_hash
		FROM knowledge_document_versions v
		JOIN knowledge_document_version_contents vc
			ON vc.admin_id = v.admin_id AND vc.workspace_id = v.workspace_id AND vc.version_id = v.id
		WHERE v.admin_id = $1 AND v.workspace_id = $2 AND v.document_id = $3
		ORDER BY v.version_no DESC LIMIT 1`,
		tx, model.AdminId, model.WorkspaceId, model.DocumentId)
	if err != nil {
		logs.Error("load latest knowledge version for manual save failed, err:%s", err.Error())
		return KnowledgeVersionModel{}, err
	}
	latestExists := len(latestRows) > 0
	latestHash := ``
	if latestExists {
		latestHash = latestRows[0][`content_hash`]
	}
	if !shouldCreateKnowledgeVersion(latestExists, latestHash, current[`content_hash`]) {
		latestVersionId := cast.ToInt(latestRows[0][`id`])
		if err = tx.Commit(); err != nil {
			logs.Error("commit unchanged knowledge version transaction failed, err:%s", err.Error())
			return KnowledgeVersionModel{}, err
		}
		latestVersion, detailErr := GetKnowledgeVersionDetail(model.AdminId, model.WorkspaceId, model.DocumentId, latestVersionId)
		if detailErr != nil {
			return KnowledgeVersionModel{}, detailErr
		}
		latestVersion.Created = false
		return latestVersion, nil
	}

	now := int64(tool.Time2Int())
	nextVersionNo := cast.ToInt64(current[`version_seq`]) + 1
	versionId, err := insertKnowledgeVersionSnapshot(tx, model.AdminId, model.WorkspaceId, model.DocumentId, model.UserId,
		nextVersionNo, define.KnowledgeVersionTypeManual, model.Description, model.EditSessionId, 0,
		current[`title`], current[`content`], current[`content_mode`], current[`content_hash`], now)
	if err != nil {
		return KnowledgeVersionModel{}, err
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE knowledge_items SET version_seq = $1, updated_at = $2
		WHERE admin_id = $3 AND workspace_id = $4 AND id = $5`,
		tx, nextVersionNo, now, model.AdminId, model.WorkspaceId, model.DocumentId); err != nil {
		logs.Error("update knowledge version_seq failed, err:%s", err.Error())
		return KnowledgeVersionModel{}, err
	}
	if err = writeKnowledgeActivity(tx, model.AdminId, model.WorkspaceId, model.DocumentId, model.UserId,
		current[`title`], define.KnowledgeItemTypeDocument, define.KnowledgeActionUpdate, now); err != nil {
		return KnowledgeVersionModel{}, err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit create knowledge version transaction failed, err:%s", err.Error())
		return KnowledgeVersionModel{}, err
	}
	createdVersion, err := GetKnowledgeVersionDetail(model.AdminId, model.WorkspaceId, model.DocumentId, cast.ToInt(versionId))
	if err != nil {
		return KnowledgeVersionModel{}, err
	}
	createdVersion.Created = true
	return createdVersion, nil
}

// RestoreKnowledgeVersionModel is the restore request model.
type RestoreKnowledgeVersionModel struct {
	AdminId     int
	WorkspaceId int
	DocumentId  int
	VersionId   int
	UserId      int
	Revision    int64
}

// RestoreKnowledgeVersion restores a version snapshot into the current state and creates a new restore version.
func RestoreKnowledgeVersion(model RestoreKnowledgeVersionModel) (KnowledgeVersionModel, error) {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin restore knowledge version transaction failed, err:%s", err.Error())
		return KnowledgeVersionModel{}, err
	}
	defer func() { _ = tx.Rollback() }()

	lockRows, err := msql.RawValues(define.Postgres, `SELECT ki.id, ki.title, ki.revision, ki.version_seq, ki.creator_id, ki.updated_by, ki.updated_at
		FROM knowledge_items ki
		WHERE ki.admin_id = $1 AND ki.workspace_id = $2 AND ki.id = $3 AND ki.deleted_at = 0
		FOR UPDATE OF ki`, tx, model.AdminId, model.WorkspaceId, model.DocumentId)
	if err != nil {
		logs.Error("lock knowledge document for restore failed, err:%s", err.Error())
		return KnowledgeVersionModel{}, err
	}
	if len(lockRows) == 0 {
		return KnowledgeVersionModel{}, ErrKnowledgeItemNotFound
	}
	current := lockRows[0]
	item := toKnowledgeItemModel(msql.Params{
		`id`: current[`id`], `parent_id`: `0`, `item_type`: define.KnowledgeItemTypeDocument,
		`title`: current[`title`], `revision`: current[`revision`], `version_seq`: current[`version_seq`],
		`creator_id`: current[`creator_id`], `creator_name`: ``, `updated_by`: current[`updated_by`],
		`created_at`: `0`, `updated_at`: current[`updated_at`],
	})
	if !canEditKnowledgeItem(model.AdminId, model.UserId, cast.ToString(model.WorkspaceId), item) {
		return KnowledgeVersionModel{}, ErrForbidden
	}
	currentRevision := cast.ToInt64(current[`revision`])
	if currentRevision != model.Revision {
		return KnowledgeVersionModel{}, &KnowledgeRevisionConflictError{
			CurrentRevision: currentRevision,
			UpdatedAt:       cast.ToInt64(current[`updated_at`]),
			UpdatedBy:       cast.ToInt(current[`updated_by`]),
		}
	}

	sourceRows, err := msql.RawValues(define.Postgres, `SELECT v.id, v.version_no, vc.title, vc.content, vc.content_mode, vc.content_hash
		FROM knowledge_document_versions v
		JOIN knowledge_document_version_contents vc ON vc.admin_id = v.admin_id AND vc.workspace_id = v.workspace_id AND vc.version_id = v.id
		WHERE v.admin_id = $1 AND v.workspace_id = $2 AND v.document_id = $3 AND v.id = $4 LIMIT 1`,
		tx, model.AdminId, model.WorkspaceId, model.DocumentId, model.VersionId)
	if err != nil {
		logs.Error("load knowledge source version for restore failed, err:%s", err.Error())
		return KnowledgeVersionModel{}, err
	}
	if len(sourceRows) == 0 {
		return KnowledgeVersionModel{}, ErrKnowledgeVersionNotFound
	}
	source := sourceRows[0]
	source[`title`] = lib_storage.StripOSSURLSignatures(source[`title`])
	source[`content`] = lib_storage.StripOSSURLSignatures(source[`content`])
	source[`content_hash`] = knowledgeContentHash(source[`title`], source[`content_mode`], source[`content`])
	now := int64(tool.Time2Int())
	if _, err = msql.RawExec(define.Postgres, `UPDATE knowledge_items
		SET title = $1, revision = revision + 1, updated_by = $2, updated_at = $3
		WHERE admin_id = $4 AND workspace_id = $5 AND id = $6`,
		tx, source[`title`], model.UserId, now, model.AdminId, model.WorkspaceId, model.DocumentId); err != nil {
		logs.Error("update knowledge item for restore failed, err:%s", err.Error())
		return KnowledgeVersionModel{}, err
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE knowledge_document_contents
		SET content = $1, content_mode = $2, content_hash = $3, updated_at = $4
		WHERE admin_id = $5 AND workspace_id = $6 AND document_id = $7`,
		tx, source[`content`], source[`content_mode`], source[`content_hash`], now, model.AdminId, model.WorkspaceId, model.DocumentId); err != nil {
		logs.Error("update knowledge document contents for restore failed, err:%s", err.Error())
		return KnowledgeVersionModel{}, err
	}

	nextVersionNo := cast.ToInt64(current[`version_seq`]) + 1
	versionId, err := insertKnowledgeVersionSnapshot(tx, model.AdminId, model.WorkspaceId, model.DocumentId, model.UserId,
		nextVersionNo, define.KnowledgeVersionTypeRestore, ``, ``, cast.ToInt64(source[`id`]),
		source[`title`], source[`content`], source[`content_mode`], source[`content_hash`], now)
	if err != nil {
		return KnowledgeVersionModel{}, err
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE knowledge_items SET version_seq = $1, updated_at = $2
		WHERE admin_id = $3 AND workspace_id = $4 AND id = $5`,
		tx, nextVersionNo, now, model.AdminId, model.WorkspaceId, model.DocumentId); err != nil {
		logs.Error("update knowledge version_seq for restore failed, err:%s", err.Error())
		return KnowledgeVersionModel{}, err
	}
	if err = writeKnowledgeActivity(tx, model.AdminId, model.WorkspaceId, model.DocumentId, model.UserId,
		source[`title`], define.KnowledgeItemTypeDocument, define.KnowledgeActionRestore, now); err != nil {
		return KnowledgeVersionModel{}, err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit restore knowledge version transaction failed, err:%s", err.Error())
		return KnowledgeVersionModel{}, err
	}
	return GetKnowledgeVersionDetail(model.AdminId, model.WorkspaceId, model.DocumentId, cast.ToInt(versionId))
}

// IsKnowledgeRevisionConflict reports whether the error is a typed revision conflict.
func IsKnowledgeRevisionConflict(err error) bool {
	var conflict *KnowledgeRevisionConflictError
	return errors.As(err, &conflict)
}

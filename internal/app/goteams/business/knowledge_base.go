// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"mime/multipart"
	"path/filepath"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_storage"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// Knowledge base error sentinels.
var (
	ErrKnowledgeItemNotFound          = errors.New(`knowledge item not found`)
	ErrKnowledgeDocumentNotEditable   = errors.New(`knowledge document not editable`)
	ErrKnowledgeVersionNotFound       = errors.New(`knowledge version not found`)
	ErrKnowledgeAttachmentNotFound    = errors.New(`knowledge attachment not found`)
	ErrKnowledgeAttachmentTooLarge    = errors.New(`knowledge attachment too large`)
	ErrKnowledgeAttachmentTypeInvalid = errors.New(`knowledge attachment type invalid`)
	ErrKnowledgeParentNotFound        = errors.New(`knowledge parent not found`)
	ErrKnowledgeFolderDepthExceeded   = errors.New(`knowledge folder depth exceeded`)
	ErrKnowledgeInvalidParent         = errors.New(`knowledge parent must not be the node itself or its descendant`)
	// ErrForbidden indicates the current user lacks object-level permission.
	ErrForbidden = errors.New(`forbidden`)
)

// KnowledgeRevisionConflictError is a typed conflict carrying the current server state.
type KnowledgeRevisionConflictError struct {
	CurrentRevision int64
	UpdatedAt       int64
	UpdatedBy       int
}

// Error implements the error interface.
func (e *KnowledgeRevisionConflictError) Error() string {
	return `knowledge revision conflict`
}

// KnowledgeItemModel is the tree/metadata model of a knowledge base node.
type KnowledgeItemModel struct {
	Id       int
	ParentId int
	// SortOrder is the manual sibling position (1-based) used by the manual sort mode.
	SortOrder   int
	ItemType    string
	Title       string
	Revision    int64
	VersionSeq  int64
	CreatorId   int
	CreatorName string
	UpdatedBy   int
	CreatedAt   int64
	UpdatedAt   int64
	Favorite    bool
	// ContentMode is the current content mode of documents (empty for folders/links).
	ContentMode string
	// SourceUrl is the current URL of links (empty otherwise).
	SourceUrl string
	// Content is the current document body (detail responses only).
	Content string
	// ContentHash is the SHA-256 of title+content_mode+content (or title+source_url for links).
	ContentHash string
	// SourceType distinguishes documents created in the editor from uploaded local files.
	SourceType string
	// SourceAttachmentId is the original uploaded file attachment id (zero for created documents).
	SourceAttachmentId int
	// SourceFileName is the original uploaded file name.
	SourceFileName string
	// SourceMimeType is the original uploaded file MIME type.
	SourceMimeType string
	// SourceFileSize is the original uploaded file size in bytes.
	SourceFileSize int64
	// SourceExtension is the lowercase original file extension without a leading dot.
	SourceExtension string
	// FolderTitle is the parent folder title (search responses only).
	FolderTitle string
	// DocumentCount is the number of document/link descendants (favorite folders only).
	DocumentCount int
}

// KnowledgeActivityModel is one home feed row.
type KnowledgeActivityModel struct {
	Id        int
	ItemId    int
	ItemTitle string
	ItemType  string
	UserId    int
	UserName  string
	Avatar    string
	Action    string
	CreatedAt int64
	// SourceExtension is the lowercase original file extension of uploaded documents (empty otherwise).
	SourceExtension string
}

// knowledgeItemFields lists the item columns loaded with a user name join.
const knowledgeItemFields = `ki.id, ki.parent_id, ki.sort_order, ki.item_type, ki.title, ki.revision, ki.version_seq,
	ki.creator_id, u.name AS creator_name, ki.updated_by, ki.created_at, ki.updated_at`

// knowledgeContentHash computes SHA-256(title + "\x00" + content_mode + "\x00" + content).
func knowledgeContentHash(title, contentMode, content string) string {
	sum := sha256.Sum256([]byte(title + "\x00" + contentMode + "\x00" + content))
	return hex.EncodeToString(sum[:])
}

// knowledgeLinkHash computes SHA-256(title + source_url).
func knowledgeLinkHash(title, sourceUrl string) string {
	sum := sha256.Sum256([]byte(title + sourceUrl))
	return hex.EncodeToString(sum[:])
}

// loadKnowledgeItem loads a non-deleted item with tenant isolation and creator name.
func loadKnowledgeItem(adminId, wsId, itemId int) (KnowledgeItemModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT `+knowledgeItemFields+`
		FROM knowledge_items ki LEFT JOIN users u ON u.id = ki.creator_id
		WHERE ki.admin_id = $1 AND ki.workspace_id = $2 AND ki.id = $3 AND ki.deleted_at = 0 LIMIT 1`, nil,
		adminId, wsId, itemId)
	if err != nil {
		logs.Error("load knowledge item failed, err:%s", err.Error())
		return KnowledgeItemModel{}, err
	}
	if len(rows) == 0 {
		return KnowledgeItemModel{}, ErrKnowledgeItemNotFound
	}
	return toKnowledgeItemModel(rows[0]), nil
}

// toKnowledgeItemModel converts a joined data row into the item model.
func toKnowledgeItemModel(row msql.Params) KnowledgeItemModel {
	return KnowledgeItemModel{
		Id:          cast.ToInt(row[`id`]),
		ParentId:    cast.ToInt(row[`parent_id`]),
		SortOrder:   cast.ToInt(row[`sort_order`]),
		ItemType:    row[`item_type`],
		Title:       row[`title`],
		Revision:    cast.ToInt64(row[`revision`]),
		VersionSeq:  cast.ToInt64(row[`version_seq`]),
		CreatorId:   cast.ToInt(row[`creator_id`]),
		CreatorName: row[`creator_name`],
		UpdatedBy:   cast.ToInt(row[`updated_by`]),
		CreatedAt:   cast.ToInt64(row[`created_at`]),
		UpdatedAt:   cast.ToInt64(row[`updated_at`]),
	}
}

// hasKnowledgeManage reports whether the user holds kb:manage in the workspace.
func hasKnowledgeManage(adminId, userId int, wsId string) bool {
	return HasPermission(adminId, userId, wsId, `kb:manage`)
}

// canEditKnowledgeItem checks object-level edit permission:
// the item's creator, or any workspace member with kb:manage.
func canEditKnowledgeItem(adminId, userId int, wsId string, item KnowledgeItemModel) bool {
	if userId > 0 && item.CreatorId == userId {
		return true
	}
	return hasKnowledgeManage(adminId, userId, wsId)
}

// getKnowledgeFavoriteSet returns the user's favorite item id set in the workspace.
func getKnowledgeFavoriteSet(adminId, wsId, userId int) (map[int]bool, error) {
	m := msql.Model(define.TableKnowledgeFavorites, define.Postgres)
	rows, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, cast.ToString(wsId)},
		{`user_id`, cast.ToString(userId)},
	}).Field(`item_id`).Select()
	if err != nil {
		logs.Error("get knowledge favorites sql:%s, err:%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	set := make(map[int]bool, len(rows))
	for _, row := range rows {
		set[cast.ToInt(row[`item_id`])] = true
	}
	return set, nil
}

// GetKnowledgeTree returns the full non-deleted item list of the workspace (metadata only, no body).
func GetKnowledgeTree(adminId, wsId, userId int) ([]KnowledgeItemModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT `+knowledgeItemFields+`
		FROM knowledge_items ki LEFT JOIN users u ON u.id = ki.creator_id
		WHERE ki.admin_id = $1 AND ki.workspace_id = $2 AND ki.deleted_at = 0
		ORDER BY ki.sort_order ASC, ki.id ASC`, nil, adminId, wsId)
	if err != nil {
		logs.Error("get knowledge tree failed, err:%s", err.Error())
		return nil, err
	}
	favorites, err := getKnowledgeFavoriteSet(adminId, wsId, userId)
	if err != nil {
		return nil, err
	}
	contentModes, err := getKnowledgeDocumentContentModes(adminId, wsId)
	if err != nil {
		return nil, err
	}
	linkUrls, err := getKnowledgeLinkUrls(adminId, wsId)
	if err != nil {
		return nil, err
	}
	sourceExtensions, err := getKnowledgeSourceExtensions(adminId, wsId)
	if err != nil {
		return nil, err
	}
	items := make([]KnowledgeItemModel, 0, len(rows))
	for _, row := range rows {
		item := toKnowledgeItemModel(row)
		item.Favorite = favorites[item.Id]
		item.ContentMode = contentModes[item.Id]
		item.SourceUrl = linkUrls[item.Id]
		item.SourceExtension = sourceExtensions[item.Id]
		items = append(items, item)
	}
	return items, nil
}

// getKnowledgeSourceExtensions builds document_id → lowercase extension (no leading dot) for uploaded documents.
func getKnowledgeSourceExtensions(adminId, wsId int) (map[int]string, error) {
	m := msql.Model(define.GetEnterpriseFileTable(adminId), define.Postgres)
	rows, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, cast.ToString(wsId)},
		{`source_type`, define.SourceTypeKnowledge},
	}).Field(`document_id, file_name`).Select()
	if err != nil {
		logs.Error("get knowledge source extensions sql:%s, err:%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	exts := make(map[int]string, len(rows))
	for _, row := range rows {
		docId := cast.ToInt(row[`document_id`])
		if docId <= 0 {
			continue
		}
		exts[docId] = strings.TrimPrefix(strings.ToLower(filepath.Ext(row[`file_name`])), `.`)
	}
	return exts, nil
}

// getKnowledgeDocumentContentModes builds document_id → content_mode (no body loaded).
func getKnowledgeDocumentContentModes(adminId, wsId int) (map[int]string, error) {
	m := msql.Model(define.TableKnowledgeDocumentContents, define.Postgres)
	rows, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, cast.ToString(wsId)},
	}).Field(`document_id, content_mode`).Select()
	if err != nil {
		logs.Error("get knowledge content modes sql:%s, err:%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	result := make(map[int]string, len(rows))
	for _, row := range rows {
		result[cast.ToInt(row[`document_id`])] = row[`content_mode`]
	}
	return result, nil
}

// getKnowledgeLinkUrls builds link_id → source_url.
func getKnowledgeLinkUrls(adminId, wsId int) (map[int]string, error) {
	m := msql.Model(define.TableKnowledgeLinkContents, define.Postgres)
	rows, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, cast.ToString(wsId)},
	}).Field(`link_id, source_url`).Select()
	if err != nil {
		logs.Error("get knowledge link urls sql:%s, err:%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	result := make(map[int]string, len(rows))
	for _, row := range rows {
		result[cast.ToInt(row[`link_id`])] = row[`source_url`]
	}
	return result, nil
}

// GetKnowledgeItemDetail returns the item detail with current content (documents/links only).
func GetKnowledgeItemDetail(adminId, wsId, itemId, userId int) (KnowledgeItemModel, error) {
	item, err := loadKnowledgeItem(adminId, wsId, itemId)
	if err != nil {
		return KnowledgeItemModel{}, err
	}
	switch item.ItemType {
	case define.KnowledgeItemTypeDocument:
		m := msql.Model(define.TableKnowledgeDocumentContents, define.Postgres)
		row, err := m.Where2([][]string{
			{`admin_id`, cast.ToString(adminId)},
			{`workspace_id`, cast.ToString(wsId)},
			{`document_id`, cast.ToString(itemId)},
		}).Field(`content, content_mode, content_hash, source_type, source_attachment_id, updated_at`).Find()
		if err != nil {
			logs.Error("get knowledge document content sql:%s, err:%s", m.GetLastSql(), err.Error())
			return KnowledgeItemModel{}, err
		}
		if len(row) == 0 {
			return KnowledgeItemModel{}, ErrKnowledgeItemNotFound
		}
		item.Content = row[`content`]
		item.ContentMode = row[`content_mode`]
		item.ContentHash = row[`content_hash`]
		item.SourceType = row[`source_type`]
		item.SourceAttachmentId = cast.ToInt(row[`source_attachment_id`])
		if item.SourceType == `` {
			item.SourceType = define.KnowledgeSourceTypeCreated
		}
		if item.SourceAttachmentId > 0 {
			attachment, attachmentErr := GetKnowledgeAttachment(adminId, wsId, item.SourceAttachmentId)
			if attachmentErr != nil {
				return KnowledgeItemModel{}, attachmentErr
			}
			if attachment.DocumentId != item.Id || !lib_storage.M.IsOwned(adminId, lib_storage.BusinessKnowledgeBase, attachment.StoredPath) {
				return KnowledgeItemModel{}, ErrKnowledgeAttachmentNotFound
			}
			item.SourceFileName = attachment.OriginalName
			item.SourceMimeType = attachment.MimeType
			item.SourceFileSize = attachment.FileSize
			item.SourceExtension = strings.TrimPrefix(strings.ToLower(filepath.Ext(attachment.OriginalName)), `.`)
		}
	case define.KnowledgeItemTypeLink:
		m := msql.Model(define.TableKnowledgeLinkContents, define.Postgres)
		row, err := m.Where2([][]string{
			{`admin_id`, cast.ToString(adminId)},
			{`workspace_id`, cast.ToString(wsId)},
			{`link_id`, cast.ToString(itemId)},
		}).Field(`source_url, content_hash, updated_at`).Find()
		if err != nil {
			logs.Error("get knowledge link content sql:%s, err:%s", m.GetLastSql(), err.Error())
			return KnowledgeItemModel{}, err
		}
		if len(row) == 0 {
			return KnowledgeItemModel{}, ErrKnowledgeItemNotFound
		}
		item.SourceUrl = row[`source_url`]
		item.ContentHash = row[`content_hash`]
	}
	favorites, err := getKnowledgeFavoriteSet(adminId, wsId, userId)
	if err != nil {
		return KnowledgeItemModel{}, err
	}
	item.Favorite = favorites[itemId]
	return item, nil
}

// checkKnowledgeParent validates that a parent folder exists and belongs to the workspace.
func checkKnowledgeParent(adminId, wsId, parentId int) error {
	if parentId == 0 {
		return nil
	}
	parent, err := loadKnowledgeItem(adminId, wsId, parentId)
	if err != nil {
		if errors.Is(err, ErrKnowledgeItemNotFound) {
			return ErrKnowledgeParentNotFound
		}
		return err
	}
	if parent.ItemType != define.KnowledgeItemTypeFolder {
		return ErrKnowledgeParentNotFound
	}
	return nil
}

// checkKnowledgeFolderParent only allows folders to be created at the workspace root.
func checkKnowledgeFolderParent(parentId int) error {
	if parentId == 0 {
		return nil
	}
	return ErrKnowledgeFolderDepthExceeded
}

// insertKnowledgeItemRow inserts the items row and returns the new id.
func insertKnowledgeItemRow(adminId, wsId, parentId, userId int, itemType, title string, now int64) (int64, error) {
	sortOrder, err := nextKnowledgeSortOrder(adminId, wsId, parentId)
	if err != nil {
		return 0, err
	}
	m := msql.Model(define.TableKnowledgeItems, define.Postgres)
	return m.Insert(msql.Datas{
		`admin_id`: adminId, `workspace_id`: wsId, `parent_id`: parentId, `item_type`: itemType,
		`title`: title, `revision`: 1, `version_seq`: 1, `creator_id`: userId, `updated_by`: userId,
		`created_at`: now, `updated_at`: now, `sort_order`: sortOrder,
	}, `id`)
}

// nextKnowledgeSortOrder returns the max sibling sort_order plus one (new nodes append at the end).
func nextKnowledgeSortOrder(adminId, wsId, parentId int) (int, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT COALESCE(MAX(sort_order), 0) + 1
		FROM knowledge_items WHERE admin_id = $1 AND workspace_id = $2 AND parent_id = $3 AND deleted_at = 0`,
		nil, adminId, wsId, parentId)
	if err != nil {
		logs.Error("load knowledge max sort order failed, err:%s", err.Error())
		return 0, err
	}
	if len(rows) == 0 {
		return 1, nil
	}
	return cast.ToInt(rows[0][`coalesce`]), nil
}

// insertKnowledgeVersionSnapshot writes one version row (metadata + content snapshot) inside a transaction.
// versionNo 0 means "use version_seq + 1"; the caller must hold the items row lock.
func insertKnowledgeVersionSnapshot(tx *sql.Tx, adminId, wsId, documentId, userId int, versionNo int64,
	versionType, description, editSessionId string, restoreFromVersionId int64,
	title, content, contentMode, contentHash string, now int64) (int64, error) {
	title = lib_storage.StripOSSURLSignatures(title)
	content = lib_storage.StripOSSURLSignatures(content)
	contentHash = knowledgeContentHash(title, contentMode, content)
	if versionNo == 0 {
		itemRows, err := msql.RawValues(define.Postgres, `SELECT version_seq FROM knowledge_items
			WHERE admin_id = $1 AND workspace_id = $2 AND id = $3 FOR UPDATE`, tx, adminId, wsId, documentId)
		if err != nil {
			logs.Error("lock knowledge item for version failed, err:%s", err.Error())
			return 0, err
		}
		if len(itemRows) == 0 {
			return 0, ErrKnowledgeItemNotFound
		}
		versionNo = cast.ToInt64(itemRows[0][`version_seq`]) + 1
	}
	versionM := msql.Model(define.TableKnowledgeDocumentVersions, define.Postgres)
	versionId, err := versionM.Insert(msql.Datas{
		`admin_id`: adminId, `workspace_id`: wsId, `document_id`: documentId, `version_no`: versionNo,
		`version_type`: versionType, `description`: description, `restore_from_version_id`: restoreFromVersionId,
		`edit_session_id`: editSessionId, `created_by`: userId, `created_at`: now,
	}, `id`)
	if err != nil {
		logs.Error("insert knowledge version sql:%s, err:%s", versionM.GetLastSql(), err.Error())
		return 0, err
	}
	contentM := msql.Model(define.TableKnowledgeDocumentVersionContents, define.Postgres)
	_, err = contentM.Insert(msql.Datas{
		`admin_id`: adminId, `workspace_id`: wsId, `version_id`: versionId,
		`title`: title, `content`: content, `content_mode`: contentMode, `content_hash`: contentHash,
	})
	if err != nil {
		logs.Error("insert knowledge version content sql:%s, err:%s", contentM.GetLastSql(), err.Error())
		return 0, err
	}
	return versionId, nil
}

// writeKnowledgeActivity inserts a home feed row (tx may be nil to use the pool directly).
func writeKnowledgeActivity(tx *sql.Tx, adminId, wsId, itemId, userId int, itemTitle, itemType, action string, now int64) error {
	_, err := msql.RawExec(define.Postgres, `INSERT INTO knowledge_activities
		(admin_id, workspace_id, item_id, item_title, item_type, user_id, action, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		tx, adminId, wsId, itemId, itemTitle, itemType, userId, action, now)
	if err != nil {
		logs.Error("write knowledge activity failed, err:%s", err.Error())
		return err
	}
	return nil
}

// CreateKnowledgeFolder creates a root-level folder node.
func CreateKnowledgeFolder(adminId, wsId, userId, parentId int, title string) (KnowledgeItemModel, error) {
	if err := checkKnowledgeFolderParent(parentId); err != nil {
		return KnowledgeItemModel{}, err
	}
	now := int64(tool.Time2Int())
	id, err := insertKnowledgeItemRow(adminId, wsId, parentId, userId, define.KnowledgeItemTypeFolder, title, now)
	if err != nil {
		return KnowledgeItemModel{}, err
	}
	if err = writeKnowledgeActivity(nil, adminId, wsId, cast.ToInt(id), userId, title, define.KnowledgeItemTypeFolder, define.KnowledgeActionCreate, now); err != nil {
		return KnowledgeItemModel{}, err
	}
	return loadKnowledgeItem(adminId, wsId, cast.ToInt(id))
}

// CreateKnowledgeDocument creates a document with its current content and the initial v1 version.
func CreateKnowledgeDocument(adminId, wsId, userId, parentId int, title, content, contentMode, editSessionId string) (KnowledgeItemModel, error) {
	title = lib_storage.StripOSSURLSignatures(title)
	content = lib_storage.StripOSSURLSignatures(content)
	if err := checkKnowledgeParent(adminId, wsId, parentId); err != nil {
		return KnowledgeItemModel{}, err
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin create knowledge document transaction failed, err:%s", err.Error())
		return KnowledgeItemModel{}, err
	}
	defer func() { _ = tx.Rollback() }()

	now := int64(tool.Time2Int())
	itemId, err := insertKnowledgeItemRow(adminId, wsId, parentId, userId, define.KnowledgeItemTypeDocument, title, now)
	if err != nil {
		return KnowledgeItemModel{}, err
	}
	documentId := cast.ToInt(itemId)
	contentM := msql.Model(define.TableKnowledgeDocumentContents, define.Postgres)
	contentHash := knowledgeContentHash(title, contentMode, content)
	_, err = contentM.Insert(msql.Datas{
		`admin_id`: adminId, `workspace_id`: wsId, `document_id`: documentId,
		`content`: content, `content_mode`: contentMode, `content_hash`: contentHash, `updated_at`: now,
	})
	if err != nil {
		logs.Error("insert knowledge document content sql:%s, err:%s", contentM.GetLastSql(), err.Error())
		return KnowledgeItemModel{}, err
	}
	if _, err = insertKnowledgeVersionSnapshot(tx, adminId, wsId, documentId, userId, 1,
		define.KnowledgeVersionTypeCreate, ``, editSessionId, 0, title, content, contentMode, contentHash, now); err != nil {
		return KnowledgeItemModel{}, err
	}
	if err = writeKnowledgeActivity(tx, adminId, wsId, documentId, userId, title, define.KnowledgeItemTypeDocument, define.KnowledgeActionCreate, now); err != nil {
		return KnowledgeItemModel{}, err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit create knowledge document transaction failed, err:%s", err.Error())
		return KnowledgeItemModel{}, err
	}
	return loadKnowledgeItem(adminId, wsId, documentId)
}

// CreateKnowledgeLink creates a read-only link node (title defaults to the URL when empty).
func CreateKnowledgeLink(adminId, wsId, userId, parentId int, title, sourceUrl string) (KnowledgeItemModel, error) {
	if err := checkKnowledgeParent(adminId, wsId, parentId); err != nil {
		return KnowledgeItemModel{}, err
	}
	if strings.TrimSpace(title) == `` {
		title = sourceUrl
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin create knowledge link transaction failed, err:%s", err.Error())
		return KnowledgeItemModel{}, err
	}
	defer func() { _ = tx.Rollback() }()

	now := int64(tool.Time2Int())
	itemId, err := insertKnowledgeItemRow(adminId, wsId, parentId, userId, define.KnowledgeItemTypeLink, title, now)
	if err != nil {
		return KnowledgeItemModel{}, err
	}
	linkId := cast.ToInt(itemId)
	linkM := msql.Model(define.TableKnowledgeLinkContents, define.Postgres)
	_, err = linkM.Insert(msql.Datas{
		`admin_id`: adminId, `workspace_id`: wsId, `link_id`: linkId,
		`source_url`: sourceUrl, `content_hash`: knowledgeLinkHash(title, sourceUrl), `updated_at`: now,
	})
	if err != nil {
		logs.Error("insert knowledge link content sql:%s, err:%s", linkM.GetLastSql(), err.Error())
		return KnowledgeItemModel{}, err
	}
	if err = writeKnowledgeActivity(tx, adminId, wsId, linkId, userId, title, define.KnowledgeItemTypeLink, define.KnowledgeActionCreate, now); err != nil {
		return KnowledgeItemModel{}, err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit create knowledge link transaction failed, err:%s", err.Error())
		return KnowledgeItemModel{}, err
	}
	return loadKnowledgeItem(adminId, wsId, linkId)
}

// RenameKnowledgeItem renames a folder/document/link with object-level permission checks.
// sourceUrl is optional: when provided for a link item, the link URL is updated too.
func RenameKnowledgeItem(adminId, wsId, userId, itemId int, title, sourceUrl string) error {
	item, err := loadKnowledgeItem(adminId, wsId, itemId)
	if err != nil {
		return err
	}
	if !canEditKnowledgeItem(adminId, userId, cast.ToString(wsId), item) {
		return ErrForbidden
	}
	titleChanged := item.Title != title
	if !titleChanged && (item.ItemType != define.KnowledgeItemTypeLink || sourceUrl == ``) {
		return nil
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin rename knowledge item transaction failed, err:%s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := int64(tool.Time2Int())
	lockRows, err := msql.RawValues(define.Postgres, `SELECT id FROM knowledge_items
		WHERE admin_id = $1 AND workspace_id = $2 AND id = $3 AND deleted_at = 0 FOR UPDATE`,
		tx, adminId, wsId, itemId)
	if err != nil {
		logs.Error("lock knowledge item for rename failed, err:%s", err.Error())
		return err
	}
	if len(lockRows) == 0 {
		return ErrKnowledgeItemNotFound
	}
	if titleChanged {
		if _, err = msql.RawExec(define.Postgres, `UPDATE knowledge_items SET title = $1, updated_by = $2, updated_at = $3
			WHERE admin_id = $4 AND workspace_id = $5 AND id = $6 AND deleted_at = 0`,
			tx, title, userId, now, adminId, wsId, itemId); err != nil {
			logs.Error("update knowledge item title failed, err:%s", err.Error())
			return err
		}
	}
	switch item.ItemType {
	case define.KnowledgeItemTypeDocument:
		contentRows, err := msql.RawValues(define.Postgres, `SELECT content, content_mode FROM knowledge_document_contents
			WHERE admin_id = $1 AND workspace_id = $2 AND document_id = $3`, tx, adminId, wsId, itemId)
		if err != nil {
			logs.Error("load knowledge document content for rename failed, err:%s", err.Error())
			return err
		}
		if len(contentRows) == 0 {
			return ErrKnowledgeItemNotFound
		}
		nextHash := knowledgeContentHash(title, contentRows[0][`content_mode`], contentRows[0][`content`])
		if _, err = msql.RawExec(define.Postgres, `UPDATE knowledge_document_contents
			SET content_hash = $1, updated_at = $2 WHERE admin_id = $3 AND workspace_id = $4 AND document_id = $5`,
			tx, nextHash, now, adminId, wsId, itemId); err != nil {
			logs.Error("update knowledge document hash failed, err:%s", err.Error())
			return err
		}
		if _, err = msql.RawExec(define.Postgres, `UPDATE knowledge_items SET revision = revision + 1
			WHERE admin_id = $1 AND workspace_id = $2 AND id = $3`, tx, adminId, wsId, itemId); err != nil {
			logs.Error("bump knowledge document revision failed, err:%s", err.Error())
			return err
		}
	case define.KnowledgeItemTypeLink:
		linkRows, err := msql.RawValues(define.Postgres, `SELECT source_url FROM knowledge_link_contents
			WHERE admin_id = $1 AND workspace_id = $2 AND link_id = $3`, tx, adminId, wsId, itemId)
		if err != nil {
			logs.Error("load knowledge link content for rename failed, err:%s", err.Error())
			return err
		}
		if len(linkRows) == 0 {
			return ErrKnowledgeItemNotFound
		}
		currentUrl := linkRows[0][`source_url`]
		urlChanged := sourceUrl != `` && sourceUrl != currentUrl
		if !urlChanged && !titleChanged {
			return nil
		}
		nextUrl := currentUrl
		if urlChanged {
			nextUrl = sourceUrl
		}
		nextHash := knowledgeLinkHash(title, nextUrl)
		if _, err = msql.RawExec(define.Postgres, `UPDATE knowledge_link_contents
			SET source_url = $1, content_hash = $2, updated_at = $3
			WHERE admin_id = $4 AND workspace_id = $5 AND link_id = $6`,
			tx, nextUrl, nextHash, now, adminId, wsId, itemId); err != nil {
			logs.Error("update knowledge link content failed, err:%s", err.Error())
			return err
		}
		if _, err = msql.RawExec(define.Postgres, `UPDATE knowledge_items SET revision = revision + 1
			WHERE admin_id = $1 AND workspace_id = $2 AND id = $3`, tx, adminId, wsId, itemId); err != nil {
			logs.Error("bump knowledge link revision failed, err:%s", err.Error())
			return err
		}
	}
	if item.ItemType != define.KnowledgeItemTypeFolder {
		if err = writeKnowledgeActivity(tx, adminId, wsId, itemId, userId, title, item.ItemType, define.KnowledgeActionUpdate, now); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit rename knowledge item transaction failed, err:%s", err.Error())
		return err
	}
	return nil
}

// MoveKnowledgeItem moves a folder/document/link under another folder (or the root)
// with object-level permission checks; a folder must never become its own descendant.
// prevId is the sibling the item is inserted after: 0 = insert at the top of the
// target folder, -1 = append at the end, >0 = insert after the target sibling.
// The target (and old) sibling lists are loaded by sort_order and rewritten with a
// single batched UPDATE per affected parent.
func MoveKnowledgeItem(adminId, wsId, userId, itemId, parentId, prevId int) error {
	item, err := loadKnowledgeItem(adminId, wsId, itemId)
	if err != nil {
		return err
	}
	if !canEditKnowledgeItem(adminId, userId, cast.ToString(wsId), item) {
		return ErrForbidden
	}
	if prevId == itemId {
		return nil
	}
	if err = checkKnowledgeParent(adminId, wsId, parentId); err != nil {
		return err
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin move knowledge item transaction failed, err:%s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if item.ItemType == define.KnowledgeItemTypeFolder {
		descendants, err := collectKnowledgeDescendantIds(tx, adminId, wsId, itemId)
		if err != nil {
			return err
		}
		for _, id := range descendants {
			if id == parentId {
				return ErrKnowledgeInvalidParent
			}
		}
	}

	oldParent := item.ParentId
	targetIds, err := loadKnowledgeSiblingIds(tx, adminId, wsId, parentId)
	if err != nil {
		return err
	}
	newOrder := make([]int, 0, len(targetIds)+1)
	for _, id := range targetIds {
		if id != itemId {
			newOrder = append(newOrder, id)
		}
	}
	insertAt := len(newOrder)
	switch {
	case prevId == 0:
		insertAt = 0
	case prevId > 0:
		insertAt = -1
		for i, id := range newOrder {
			if id == prevId {
				insertAt = i + 1
				break
			}
		}
		if insertAt < 0 {
			return ErrKnowledgeInvalidParent
		}
	}
	// 同目录且插入位置与当前所在位置一致时无实际变化，跳过写入。
	if parentId == oldParent && knowledgeSiblingIndex(targetIds, itemId) == insertAt {
		return nil
	}
	newOrder = append(newOrder, 0)
	copy(newOrder[insertAt+1:], newOrder[insertAt:])
	newOrder[insertAt] = itemId

	now := int64(tool.Time2Int())
	if parentId == oldParent {
		if err = batchUpdateKnowledgeSortOrder(tx, adminId, wsId, newOrder); err != nil {
			return err
		}
	} else {
		oldIds, err := loadKnowledgeSiblingIds(tx, adminId, wsId, oldParent)
		if err != nil {
			return err
		}
		oldOrder := make([]int, 0, len(oldIds)-1)
		for _, id := range oldIds {
			if id != itemId {
				oldOrder = append(oldOrder, id)
			}
		}
		if err = batchUpdateKnowledgeSortOrder(tx, adminId, wsId, oldOrder); err != nil {
			return err
		}
		if err = batchUpdateKnowledgeSortOrder(tx, adminId, wsId, newOrder); err != nil {
			return err
		}
		if _, err = msql.RawExec(define.Postgres, `UPDATE knowledge_items
			SET parent_id = $1, updated_by = $2, updated_at = $3
			WHERE admin_id = $4 AND workspace_id = $5 AND id = $6 AND deleted_at = 0`,
			tx, parentId, userId, now, adminId, wsId, itemId); err != nil {
			logs.Error("update knowledge item parent failed, err:%s", err.Error())
			return err
		}
	}
	if item.ItemType != define.KnowledgeItemTypeFolder {
		if err = writeKnowledgeActivity(tx, adminId, wsId, itemId, userId, item.Title, item.ItemType, define.KnowledgeActionUpdate, now); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit move knowledge item transaction failed, err:%s", err.Error())
		return err
	}
	return nil
}

// loadKnowledgeSiblingIds returns the ordered child ids of a folder (or the root when parentId is 0).
func loadKnowledgeSiblingIds(tx *sql.Tx, adminId, wsId, parentId int) ([]int, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM knowledge_items
		WHERE admin_id = $1 AND workspace_id = $2 AND parent_id = $3 AND deleted_at = 0
		ORDER BY sort_order ASC, id ASC`, tx, adminId, wsId, parentId)
	if err != nil {
		logs.Error("load knowledge sibling ids failed, err:%s", err.Error())
		return nil, err
	}
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, cast.ToInt(row[`id`]))
	}
	return ids, nil
}

// knowledgeSiblingIndex returns the position of target in ids, or -1 when absent.
func knowledgeSiblingIndex(ids []int, target int) int {
	for i, id := range ids {
		if id == target {
			return i
		}
	}
	return -1
}

// batchUpdateKnowledgeSortOrder rewrites sort_order of the given ordered id list in one statement.
func batchUpdateKnowledgeSortOrder(tx *sql.Tx, adminId, wsId int, order []int) error {
	if len(order) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString(`UPDATE knowledge_items SET sort_order = CASE id `)
	args := make([]interface{}, 0, len(order)*2+3)
	for i, id := range order {
		sb.WriteString(`WHEN $`)
		sb.WriteString(cast.ToString(len(args) + 1))
		sb.WriteString(` THEN $`)
		sb.WriteString(cast.ToString(len(args) + 2))
		sb.WriteString(` `)
		args = append(args, id, i+1)
	}
	sb.WriteString(`ELSE sort_order END WHERE admin_id = $`)
	sb.WriteString(cast.ToString(len(args) + 1))
	sb.WriteString(` AND workspace_id = $`)
	sb.WriteString(cast.ToString(len(args) + 2))
	sb.WriteString(` AND id = ANY($`)
	sb.WriteString(cast.ToString(len(args) + 3))
	sb.WriteString(`::int[]) AND deleted_at = 0`)
	idList := make([]string, 0, len(order))
	for _, id := range order {
		idList = append(idList, cast.ToString(id))
	}
	args = append(args, adminId, wsId, `{`+strings.Join(idList, `,`)+`}`)
	if _, err := msql.RawExec(define.Postgres, sb.String(), tx, args...); err != nil {
		logs.Error("batch update knowledge sort order failed, err:%s", err.Error())
		return err
	}
	return nil
}

// DeleteKnowledgeItem soft-deletes a node; deleting a folder cascades to all descendants.
func DeleteKnowledgeItem(adminId, wsId, userId, itemId int) error {
	item, err := loadKnowledgeItem(adminId, wsId, itemId)
	if err != nil {
		return err
	}
	if !canEditKnowledgeItem(adminId, userId, cast.ToString(wsId), item) {
		return ErrForbidden
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin delete knowledge item transaction failed, err:%s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := int64(tool.Time2Int())
	ids := []int{itemId}
	if item.ItemType == define.KnowledgeItemTypeFolder {
		children, err := collectKnowledgeDescendantIds(tx, adminId, wsId, itemId)
		if err != nil {
			return err
		}
		ids = append(ids, children...)
	}
	idList := make([]string, 0, len(ids))
	for _, id := range ids {
		idList = append(idList, cast.ToString(id))
	}
	idArray := `{` + strings.Join(idList, `,`) + `}`
	attachmentRows, err := msql.RawValues(define.Postgres, `SELECT id, stored_path, file_size FROM `+define.GetEnterpriseFileTable(adminId)+`
		WHERE admin_id = $1 AND workspace_id = $2 AND source_type = $3 AND document_id = ANY($4::int[])`,
		tx, adminId, wsId, define.SourceTypeKnowledge, idArray)
	if err != nil {
		logs.Error("load knowledge attachments for delete failed, err:%s", err.Error())
		return err
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE knowledge_items SET deleted_at = $1, updated_at = $1
		WHERE admin_id = $2 AND workspace_id = $3 AND id = ANY($4::int[]) AND deleted_at = 0`,
		tx, now, adminId, wsId, idArray); err != nil {
		logs.Error("soft delete knowledge items failed, err:%s", err.Error())
		return err
	}
	// Current content rows are removed with the node; version snapshots keep the full history.
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM knowledge_document_contents
		WHERE admin_id = $1 AND workspace_id = $2 AND document_id = ANY($3::int[])`, tx, adminId, wsId, idArray); err != nil {
		logs.Error("delete knowledge document contents failed, err:%s", err.Error())
		return err
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM knowledge_link_contents
		WHERE admin_id = $1 AND workspace_id = $2 AND link_id = ANY($3::int[])`, tx, adminId, wsId, idArray); err != nil {
		logs.Error("delete knowledge link contents failed, err:%s", err.Error())
		return err
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM `+define.GetEnterpriseFileTable(adminId)+`
		WHERE admin_id = $1 AND workspace_id = $2 AND source_type = $3 AND document_id = ANY($4::int[])`,
		tx, adminId, wsId, define.SourceTypeKnowledge, idArray); err != nil {
		logs.Error("delete knowledge attachment rows failed, err:%s", err.Error())
		return err
	}
	if err = writeKnowledgeActivity(tx, adminId, wsId, itemId, userId, item.Title, item.ItemType, define.KnowledgeActionDelete, now); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit delete knowledge item transaction failed, err:%s", err.Error())
		return err
	}
	var removedBytes int64
	for _, attachment := range attachmentRows {
		removedBytes += cast.ToInt64(attachment[`file_size`])
		if path := attachment[`stored_path`]; path != `` {
			if removeErr := lib_storage.M.Delete(path); removeErr != nil {
				logs.Error("delete knowledge attachment file failed, attachment:%s, err:%s", attachment[`id`], removeErr.Error())
			}
		}
	}
	if removedBytes > 0 {
		adjustStorageUsedCache(adminId, -removedBytes)
	}
	return nil
}

// collectKnowledgeDescendantIds returns all descendant item ids of a folder (BFS over the workspace items).
func collectKnowledgeDescendantIds(tx *sql.Tx, adminId, wsId, folderId int) ([]int, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, parent_id FROM knowledge_items
		WHERE admin_id = $1 AND workspace_id = $2 AND deleted_at = 0`, tx, adminId, wsId)
	if err != nil {
		logs.Error("load knowledge items for cascade delete failed, err:%s", err.Error())
		return nil, err
	}
	childrenByParent := make(map[int][]int)
	for _, row := range rows {
		parentId := cast.ToInt(row[`parent_id`])
		childrenByParent[parentId] = append(childrenByParent[parentId], cast.ToInt(row[`id`]))
	}
	ids := make([]int, 0)
	queue := []int{folderId}
	visited := map[int]struct{}{folderId: {}}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, child := range childrenByParent[current] {
			if _, exists := visited[child]; exists {
				continue
			}
			visited[child] = struct{}{}
			ids = append(ids, child)
			queue = append(queue, child)
		}
	}
	return ids, nil
}

// CopyKnowledgeDocument creates a new document copy from the source's current state.
func CopyKnowledgeDocument(adminId, wsId, userId, documentId, parentId int, editSessionId string) (KnowledgeItemModel, error) {
	source, err := GetKnowledgeItemDetail(adminId, wsId, documentId, userId)
	if err != nil {
		return KnowledgeItemModel{}, err
	}
	if source.ItemType != define.KnowledgeItemTypeDocument {
		return KnowledgeItemModel{}, ErrKnowledgeDocumentNotEditable
	}
	if source.ContentMode == define.KnowledgeContentModeFile {
		return KnowledgeItemModel{}, ErrKnowledgeDocumentNotEditable
	}
	if !HasPermission(adminId, userId, cast.ToString(wsId), `kb:create`) && !hasKnowledgeManage(adminId, userId, cast.ToString(wsId)) {
		return KnowledgeItemModel{}, ErrForbidden
	}
	if parentId == 0 {
		parentId = source.ParentId
	}
	if err = checkKnowledgeParent(adminId, wsId, parentId); err != nil {
		return KnowledgeItemModel{}, err
	}
	return CreateKnowledgeDocument(adminId, wsId, userId, parentId, source.Title, source.Content, source.ContentMode, editSessionId)
}

// SearchKnowledgeItems searches by title or creator name (ILIKE, limited to 50 rows).
func SearchKnowledgeItems(adminId, wsId, userId int, keyword string, limit int) ([]KnowledgeItemModel, error) {
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	pattern := `%` + keyword + `%`
	rows, err := msql.RawValues(define.Postgres, `SELECT `+knowledgeItemFields+`
		FROM knowledge_items ki LEFT JOIN users u ON u.id = ki.creator_id
		WHERE ki.admin_id = $1 AND ki.workspace_id = $2 AND ki.deleted_at = 0
		  AND (ki.title ILIKE $3 OR u.name ILIKE $3)
		ORDER BY ki.updated_at DESC LIMIT $4`, nil, adminId, wsId, pattern, limit)
	if err != nil {
		logs.Error("search knowledge items failed, err:%s", err.Error())
		return nil, err
	}
	favorites, err := getKnowledgeFavoriteSet(adminId, wsId, userId)
	if err != nil {
		return nil, err
	}
	parentIds := make(map[int]bool)
	for _, row := range rows {
		if parentId := cast.ToInt(row[`parent_id`]); parentId > 0 {
			parentIds[parentId] = true
		}
	}
	parentTitles := make(map[int]string)
	if len(parentIds) > 0 {
		parentList := make([]string, 0, len(parentIds))
		for id := range parentIds {
			parentList = append(parentList, cast.ToString(id))
		}
		parentRows, err := msql.RawValues(define.Postgres, `SELECT id, title FROM knowledge_items
			WHERE admin_id = $1 AND workspace_id = $2 AND id = ANY($3::int[])`,
			nil, adminId, wsId, `{`+strings.Join(parentList, `,`)+`}`)
		if err != nil {
			logs.Error("load knowledge parent titles failed, err:%s", err.Error())
			return nil, err
		}
		for _, row := range parentRows {
			parentTitles[cast.ToInt(row[`id`])] = row[`title`]
		}
	}
	items := make([]KnowledgeItemModel, 0, len(rows))
	for _, row := range rows {
		item := toKnowledgeItemModel(row)
		item.Favorite = favorites[item.Id]
		item.FolderTitle = parentTitles[item.ParentId]
		items = append(items, item)
	}
	return items, nil
}

// SetKnowledgeFavorite toggles the user's favorite state on an item (idempotent).
func SetKnowledgeFavorite(adminId, wsId, userId, itemId int, favorite bool) error {
	if _, err := loadKnowledgeItem(adminId, wsId, itemId); err != nil {
		return err
	}
	if favorite {
		now := int64(tool.Time2Int())
		_, err := msql.RawExec(define.Postgres, `INSERT INTO knowledge_favorites
			(admin_id, workspace_id, user_id, item_id, created_at) VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (admin_id, workspace_id, user_id, item_id) DO NOTHING`,
			nil, adminId, wsId, userId, itemId, now)
		if err != nil {
			logs.Error("insert knowledge favorite failed, err:%s", err.Error())
			return err
		}
		return nil
	}
	_, err := msql.RawExec(define.Postgres, `DELETE FROM knowledge_favorites
		WHERE admin_id = $1 AND workspace_id = $2 AND user_id = $3 AND item_id = $4`,
		nil, adminId, wsId, userId, itemId)
	if err != nil {
		logs.Error("delete knowledge favorite failed, err:%s", err.Error())
		return err
	}
	return nil
}

// GetKnowledgeHomeActivities returns the paginated activity feed.
func GetKnowledgeHomeActivities(adminId, wsId int, page, size int) ([]KnowledgeActivityModel, int, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	offset := (page - 1) * size
	m := msql.Model(define.TableKnowledgeActivities, define.Postgres)
	total, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, cast.ToString(wsId)},
	}).Count()
	if err != nil {
		logs.Error("count knowledge activities sql:%s, err:%s", m.GetLastSql(), err.Error())
		return nil, 0, err
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT a.id, a.item_id, a.item_title, a.item_type, a.user_id, a.action, a.created_at,
		u.name AS user_name, u.avatar AS avatar
		FROM knowledge_activities a LEFT JOIN users u ON u.id = a.user_id
		WHERE a.admin_id = $1 AND a.workspace_id = $2
		ORDER BY a.created_at DESC, a.id DESC LIMIT $3 OFFSET $4`, nil, adminId, wsId, size, offset)
	if err != nil {
		logs.Error("list knowledge activities failed, err:%s", err.Error())
		return nil, 0, err
	}
	items := make([]KnowledgeActivityModel, 0, len(rows))
	sourceExtensions, err := getKnowledgeSourceExtensions(adminId, wsId)
	if err != nil {
		return nil, 0, err
	}
	for _, row := range rows {
		item := KnowledgeActivityModel{
			Id: cast.ToInt(row[`id`]), ItemId: cast.ToInt(row[`item_id`]), ItemTitle: row[`item_title`],
			ItemType: row[`item_type`], UserId: cast.ToInt(row[`user_id`]), UserName: row[`user_name`],
			Avatar: row[`avatar`], Action: row[`action`], CreatedAt: cast.ToInt64(row[`created_at`]),
		}
		if item.ItemType == define.KnowledgeItemTypeDocument {
			item.SourceExtension = sourceExtensions[item.ItemId]
		}
		items = append(items, item)
	}
	return items, total, nil
}

// GetKnowledgeHomeFavorites returns the user's favorite folders and documents separately.
func GetKnowledgeHomeFavorites(adminId, wsId, userId int) (folders, documents []KnowledgeItemModel, err error) {
	favorites, err := getKnowledgeFavoriteSet(adminId, wsId, userId)
	if err != nil {
		return nil, nil, err
	}
	if len(favorites) == 0 {
		return []KnowledgeItemModel{}, []KnowledgeItemModel{}, nil
	}
	idList := make([]string, 0, len(favorites))
	for id := range favorites {
		idList = append(idList, cast.ToString(id))
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT `+knowledgeItemFields+`
		FROM knowledge_items ki LEFT JOIN users u ON u.id = ki.creator_id
		WHERE ki.admin_id = $1 AND ki.workspace_id = $2 AND ki.deleted_at = 0
		  AND ki.id = ANY($3::int[])
		ORDER BY ki.created_at DESC, ki.id DESC`, nil, adminId, wsId, `{`+strings.Join(idList, `,`)+`}`)
	if err != nil {
		logs.Error("list knowledge favorites failed, err:%s", err.Error())
		return nil, nil, err
	}
	folders = []KnowledgeItemModel{}
	documents = []KnowledgeItemModel{}
	sourceExtensions, err := getKnowledgeSourceExtensions(adminId, wsId)
	if err != nil {
		return nil, nil, err
	}
	for _, row := range rows {
		item := toKnowledgeItemModel(row)
		item.Favorite = true
		item.SourceExtension = sourceExtensions[item.Id]
		switch item.ItemType {
		case define.KnowledgeItemTypeFolder:
			folders = append(folders, item)
		case define.KnowledgeItemTypeDocument, define.KnowledgeItemTypeLink:
			documents = append(documents, item)
		}
	}
	if len(folders) > 0 {
		counts, countErr := countKnowledgeDocumentsByFolder(adminId, wsId, folders)
		if countErr != nil {
			return nil, nil, countErr
		}
		for index := range folders {
			folders[index].DocumentCount = counts[folders[index].Id]
		}
	}
	return folders, documents, nil
}

// countKnowledgeDocumentsByFolder counts document/link descendants for the requested folders.
func countKnowledgeDocumentsByFolder(adminId, wsId int, folders []KnowledgeItemModel) (map[int]int, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, parent_id, item_type FROM knowledge_items
		WHERE admin_id = $1 AND workspace_id = $2 AND deleted_at = 0`, nil, adminId, wsId)
	if err != nil {
		logs.Error("load knowledge items for folder counts failed, err:%s", err.Error())
		return nil, err
	}
	childrenByParent := make(map[int][]KnowledgeItemModel)
	for _, row := range rows {
		parentId := cast.ToInt(row[`parent_id`])
		childrenByParent[parentId] = append(childrenByParent[parentId], KnowledgeItemModel{
			Id: cast.ToInt(row[`id`]), ParentId: parentId, ItemType: row[`item_type`],
		})
	}
	counts := make(map[int]int, len(folders))
	for _, folder := range folders {
		visited := map[int]struct{}{folder.Id: {}}
		queue := []int{folder.Id}
		for len(queue) > 0 {
			parentId := queue[0]
			queue = queue[1:]
			for _, child := range childrenByParent[parentId] {
				if _, exists := visited[child.Id]; exists {
					continue
				}
				visited[child.Id] = struct{}{}
				if child.ItemType == define.KnowledgeItemTypeFolder {
					queue = append(queue, child.Id)
				} else if child.ItemType == define.KnowledgeItemTypeDocument || child.ItemType == define.KnowledgeItemTypeLink {
					counts[folder.Id]++
				}
			}
		}
	}
	return counts, nil
}

// GetKnowledgeHomeCreatedByMe returns the paginated documents/links created by the user.
func GetKnowledgeHomeCreatedByMe(adminId, wsId, userId int, page, size int) ([]KnowledgeItemModel, int, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	offset := (page - 1) * size
	m := msql.Model(define.TableKnowledgeItems, define.Postgres)
	total, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, cast.ToString(wsId)},
		{`creator_id`, cast.ToString(userId)},
		{`deleted_at`, `0`},
	}).WhereIn(`item_type`, `document`, `link`).Count()
	if err != nil {
		logs.Error("count knowledge created-by-me sql:%s, err:%s", m.GetLastSql(), err.Error())
		return nil, 0, err
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT `+knowledgeItemFields+`
		FROM knowledge_items ki LEFT JOIN users u ON u.id = ki.creator_id
		WHERE ki.admin_id = $1 AND ki.workspace_id = $2 AND ki.deleted_at = 0
		  AND ki.creator_id = $3 AND ki.item_type IN ('document','link')
		ORDER BY ki.created_at DESC, ki.id DESC LIMIT $4 OFFSET $5`, nil, adminId, wsId, userId, size, offset)
	if err != nil {
		logs.Error("list knowledge created-by-me failed, err:%s", err.Error())
		return nil, 0, err
	}
	favorites, err := getKnowledgeFavoriteSet(adminId, wsId, userId)
	if err != nil {
		return nil, 0, err
	}
	sourceExtensions, err := getKnowledgeSourceExtensions(adminId, wsId)
	if err != nil {
		return nil, 0, err
	}
	items := make([]KnowledgeItemModel, 0, len(rows))
	for _, row := range rows {
		item := toKnowledgeItemModel(row)
		item.Favorite = favorites[item.Id]
		item.SourceExtension = sourceExtensions[item.Id]
		items = append(items, item)
	}
	return items, total, nil
}

// ======================== Knowledge Attachments ========================

// KnowledgeAttachmentModel is the attachment upload result.
type KnowledgeAttachmentModel struct {
	Id           int
	DocumentId   int
	FileId       int
	OriginalName string
	MimeType     string
	FileSize     int64
	StoredPath   string
	URL          string
}

// UploadKnowledgeAttachment stores an image, file, or video, records it in the enterprise file
// ledger bound to the document (source_type='knowledge' + document_id + folder_id), and returns
// its relative URL.
func UploadKnowledgeAttachment(adminId, wsId, userId, documentId int, file *multipart.FileHeader, isImage, isVideo bool) (KnowledgeAttachmentModel, error) {
	if file == nil || file.Size <= 0 {
		return KnowledgeAttachmentModel{}, ErrKnowledgeAttachmentTypeInvalid
	}
	maxSize := int64(MaxEditorFileSize)
	allowedExt := allowedEditorFileExt
	if isImage {
		maxSize = MaxImageSize
		allowedExt = allowedImageExt
	} else if isVideo {
		maxSize = MaxEditorVideoSize
		allowedExt = allowedEditorVideoExt
	}
	if file.Size > maxSize {
		return KnowledgeAttachmentModel{}, ErrKnowledgeAttachmentTooLarge
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(file.Filename)), `.`)
	if !allowedExt[ext] {
		return KnowledgeAttachmentModel{}, ErrKnowledgeAttachmentTypeInvalid
	}
	item, err := loadKnowledgeItem(adminId, wsId, documentId)
	if err != nil {
		return KnowledgeAttachmentModel{}, err
	}
	if item.ItemType != define.KnowledgeItemTypeDocument {
		return KnowledgeAttachmentModel{}, ErrKnowledgeDocumentNotEditable
	}
	if !canEditKnowledgeItem(adminId, userId, cast.ToString(wsId), item) {
		return KnowledgeAttachmentModel{}, ErrForbidden
	}

	opened, err := file.Open()
	if err != nil {
		return KnowledgeAttachmentModel{}, err
	}
	defer func() { _ = opened.Close() }()
	storedPath, err := lib_storage.M.Upload(adminId, lib_storage.BusinessKnowledgeBase, ext, opened)
	if err != nil {
		return KnowledgeAttachmentModel{}, err
	}
	fileID, err := RecordKnowledgeEnterpriseFile(adminId, wsId, userId, documentId, item.ParentId,
		file.Filename, storedPath, file.Header.Get(`Content-Type`), file.Size)
	if err != nil {
		return KnowledgeAttachmentModel{}, err
	}
	return KnowledgeAttachmentModel{
		DocumentId: documentId, FileId: fileID, OriginalName: file.Filename,
		MimeType: file.Header.Get(`Content-Type`), FileSize: file.Size,
		StoredPath: storedPath, URL: storedPath,
	}, nil
}

// GetKnowledgeAttachment loads a knowledge attachment with tenant isolation.
func GetKnowledgeAttachment(adminId, wsId, attachmentId int) (KnowledgeAttachmentModel, error) {
	m := msql.Model(define.GetEnterpriseFileTable(adminId), define.Postgres)
	row, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, cast.ToString(wsId)},
		{`id`, cast.ToString(attachmentId)},
		{`source_type`, define.SourceTypeKnowledge},
	}).Field(`id, document_id, file_name, content_type, file_size, stored_path`).Find()
	if err != nil {
		logs.Error("get knowledge attachment sql:%s, err:%s", m.GetLastSql(), err.Error())
		return KnowledgeAttachmentModel{}, err
	}
	if len(row) == 0 {
		return KnowledgeAttachmentModel{}, ErrKnowledgeAttachmentNotFound
	}
	return KnowledgeAttachmentModel{
		Id: cast.ToInt(row[`id`]), DocumentId: cast.ToInt(row[`document_id`]),
		OriginalName: row[`file_name`], MimeType: row[`content_type`],
		FileSize: cast.ToInt64(row[`file_size`]), StoredPath: row[`stored_path`],
	}, nil
}

// ServeKnowledgeAttachment streams the attachment file after verifying the bound document still exists.
func ServeKnowledgeAttachment(c *gin.Context, adminId, wsId, attachmentId int) error {
	attachment, err := GetKnowledgeAttachment(adminId, wsId, attachmentId)
	if err != nil {
		return err
	}
	if attachment.DocumentId > 0 {
		if _, err = loadKnowledgeItem(adminId, wsId, attachment.DocumentId); err != nil {
			return err
		}
	}
	if !lib_storage.M.IsOwned(adminId, lib_storage.BusinessKnowledgeBase, attachment.StoredPath) {
		return ErrKnowledgeAttachmentNotFound
	}
	if err = lib_storage.M.Serve(attachment.StoredPath, c); err != nil {
		logs.Error("serve knowledge attachment failed, stored:%s, err:%s", attachment.StoredPath, err.Error())
		return ErrKnowledgeAttachmentNotFound
	}
	return nil
}

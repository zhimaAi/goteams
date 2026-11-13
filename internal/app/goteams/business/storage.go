// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"context"
	"database/sql"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_redis"
	"goteams/internal/pkg/lib_storage"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// ======================== Enterprise File Ledger ========================

// EnterpriseFile represents a single row in the enterprise file ledger sharding table.
type EnterpriseFile struct {
	ID           int    `json:"id"`
	AdminID      int    `json:"admin_id"`
	WorkspaceID  int    `json:"workspace_id"`
	SourceType   string `json:"source_type"`
	SourceID     int    `json:"source_id"`
	SourceName   string `json:"source_name"`
	CommentID    int    `json:"comment_id"`
	DocumentID   int    `json:"document_id"`
	FileName     string `json:"file_name"`
	FileExt      string `json:"file_ext"`
	FileSize     int64  `json:"file_size"`
	StoredPath   string `json:"stored_path"`
	ContentType  string `json:"content_type"`
	UploaderID   int    `json:"uploader_id"`
	UploaderName string `json:"uploader_name"`
	CreatedAt    int64  `json:"created_at"`
}

// EnterpriseFileListResult is the paginated query result for the storage page.
type EnterpriseFileListResult struct {
	List      []EnterpriseFile `json:"list"`
	Total     int              `json:"total"`
	TotalSize int64            `json:"total_size"`
}

// HookRecordEnterpriseFile is called from CreateAttachment to sync-write the enterprise file ledger.
// It queries the requirement to resolve source_type and source_name before inserting.
func HookRecordEnterpriseFile(adminID, workspaceID, requirementID, commentID, uploaderID int, originalName, storedPath, contentType string, fileSize int64) {
	// Resolve source_type and source_name from the requirement.
	sourceType, sourceName, uploaderName := resolveFileSource(adminID, requirementID, uploaderID)
	if sourceType == "" {
		sourceType = define.TypeKeyRequirement
	}

	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(originalName)), ".")
	tableName := define.GetEnterpriseFileTable(adminID)
	now := tool.Time2Int()

	m := msql.Model(tableName, define.Postgres)
	_, err := m.Insert(msql.Datas{
		`admin_id`:      adminID,
		`workspace_id`:  workspaceID,
		`source_type`:   sourceType,
		`source_id`:     requirementID,
		`source_name`:   sourceName,
		`comment_id`:    commentID,
		`file_name`:     originalName,
		`file_ext`:      ext,
		`file_size`:     fileSize,
		`stored_path`:   storedPath,
		`content_type`:  contentType,
		`uploader_id`:   uploaderID,
		`uploader_name`: uploaderName,
		`created_at`:    now,
		`updated_at`:    now,
	})
	if err != nil {
		logs.Error("HookRecordEnterpriseFile insert failed: table=%s, sql=%s, err=%s", tableName, m.GetLastSql(), err.Error())
		return
	}
	adjustStorageUsedCache(adminID, fileSize)
}

// HookDeleteEnterpriseFile is called from DeleteAttachment to sync-delete the enterprise file ledger record.
func HookDeleteEnterpriseFile(adminID int, storedPath string) {
	tableName := define.GetEnterpriseFileTable(adminID)
	m := msql.Model(tableName, define.Postgres)
	sizeStr, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminID)},
		{`stored_path`, storedPath},
	}).Value(`file_size`)
	if err != nil {
		logs.Error("HookDeleteEnterpriseFile query size failed: table=%s, sql=%s, err=%s", tableName, m.GetLastSql(), err.Error())
		return
	}
	delM := msql.Model(tableName, define.Postgres)
	_, err = delM.Where2([][]string{
		{`admin_id`, cast.ToString(adminID)},
		{`stored_path`, storedPath},
	}).Delete()
	if err != nil {
		logs.Error("HookDeleteEnterpriseFile delete failed: table=%s, sql=%s, err=%s", tableName, delM.GetLastSql(), err.Error())
		return
	}
	adjustStorageUsedCache(adminID, -cast.ToInt64(sizeStr))
}

// RecordPendingEnterpriseFile writes an unbound ledger entry for a rich-text image, file, or video.
// The requirement save flow later binds the entry to its owning work item.
func RecordPendingEnterpriseFile(adminID, workspaceID, uploaderID int, originalName, storedPath, contentType string, fileSize int64) (int, error) {
	tableName := define.GetEnterpriseFileTable(adminID)
	uploaderName := ""
	if uploaderID > 0 {
		u := msql.Model(define.TableUsers, define.Postgres)
		userRow, err := u.Where2([][]string{
			{`admin_id`, cast.ToString(adminID)},
			{`id`, cast.ToString(uploaderID)},
		}).Value(`name`)
		if err == nil {
			uploaderName = userRow
		}
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(originalName)), ".")
	now := tool.Time2Int()
	id, err := msql.Model(tableName, define.Postgres).Insert(msql.Datas{
		`admin_id`:      adminID,
		`workspace_id`:  workspaceID,
		`source_type`:   "",
		`source_id`:     0,
		`source_name`:   "",
		`comment_id`:    0,
		`file_name`:     originalName,
		`file_ext`:      ext,
		`file_size`:     fileSize,
		`stored_path`:   storedPath,
		`content_type`:  contentType,
		`uploader_id`:   uploaderID,
		`uploader_name`: uploaderName,
		`created_at`:    now,
		`updated_at`:    now,
	}, `id`)
	if err != nil {
		logs.Error("RecordPendingEnterpriseFile insert failed: table=%s, err=%s", tableName, err.Error())
		return 0, err
	}
	adjustStorageUsedCache(adminID, fileSize)
	return cast.ToInt(id), nil
}

// BindPendingEnterpriseFiles associates previously uploaded rich-text assets with their owning work item.
// Only unbound files (source_id = 0) owned by the same admin are updated, preventing cross-user / cross-item hijacking.
func BindPendingEnterpriseFiles(adminID, sourceID int, sourceType, sourceName string, fileIDs []int) error {
	if len(fileIDs) == 0 || sourceID <= 0 {
		return nil
	}
	tableName := define.GetEnterpriseFileTable(adminID)
	ids := make([]string, 0, len(fileIDs))
	for _, fid := range fileIDs {
		if fid > 0 {
			ids = append(ids, cast.ToString(fid))
		}
	}
	if len(ids) == 0 {
		return nil
	}
	_, err := msql.Model(tableName, define.Postgres).Where2([][]string{
		{`admin_id`, cast.ToString(adminID)},
		{`source_id`, `0`},
		{`id`, `IN`, strings.Join(ids, ",")},
	}).Update(msql.Datas{
		`source_type`: sourceType,
		`source_id`:   sourceID,
		`source_name`: sourceName,
		`updated_at`:  tool.Time2Int(),
	})
	if err != nil {
		logs.Error("BindPendingEnterpriseFiles update failed: table=%s, err=%s", tableName, err.Error())
		return err
	}
	return nil
}

// resolveFileSource queries the requirement title/type and uploader name.
func resolveFileSource(adminID, requirementID, uploaderID int) (sourceType, sourceName, uploaderName string) {
	m := msql.Model(define.TableRequirements, define.Postgres)
	row, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminID)},
		{`id`, cast.ToString(requirementID)},
	}).Field(`type, title`).Find()
	if err != nil {
		logs.Error("resolveFileSource query requirement failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return define.TypeKeyRequirement, "", ""
	}
	if len(row) > 0 {
		sourceType = row[`type`]
		sourceName = row[`title`]
	}

	// Query uploader name.
	u := msql.Model(define.TableUsers, define.Postgres)
	userRow, err := u.Where2([][]string{
		{`admin_id`, cast.ToString(adminID)},
		{`id`, cast.ToString(uploaderID)},
	}).Value(`name`)
	if err == nil {
		uploaderName = userRow
	}
	return sourceType, sourceName, uploaderName
}

// ======================== Storage Page Query ========================

// ListEnterpriseFiles returns the paginated file list for the enterprise storage page.
// Supports optional filtering by workspace_id and source_type, plus sorting by file_size/created_at.
func ListEnterpriseFiles(adminID int, workspaceID int, sourceType string, page, pageSize int, sortField, sortOrder string) (*EnterpriseFileListResult, error) {
	tableName := define.GetEnterpriseFileTable(adminID)
	m := msql.Model(tableName, define.Postgres)

	where := [][]string{
		{`admin_id`, cast.ToString(adminID)},
	}
	if workspaceID > 0 {
		where = append(where, []string{`workspace_id`, cast.ToString(workspaceID)})
	}
	if sourceType != "" && sourceType != define.TypeKeyRequirement && sourceType != define.TypeKeyDefect {
		sourceType = ""
	}
	if sourceType != "" {
		where = append(where, []string{`source_type`, sourceType})
	}

	// Total count and total size.
	count, err := m.Where2(where).Count()
	if err != nil {
		logs.Error("ListEnterpriseFiles count failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return nil, err
	}

	// Unfiltered total equals the enterprise-wide usage: read the cached value instead of a direct SUM.
	// Filtered totals (workspace_id / source_type) stay a direct SUM since they are page-specific.
	var totalSize int64
	if workspaceID == 0 && sourceType == "" {
		if totalSize, err = GetEnterpriseStorageUsed(adminID); err != nil {
			logs.Error("ListEnterpriseFiles cached sum failed: admin_id=%d, err=%s", adminID, err.Error())
			return nil, err
		}
	} else {
		totalSizeStr, err := m.Where2(where).Value(`COALESCE(SUM(file_size), 0) AS total_size`)
		if err != nil {
			logs.Error("ListEnterpriseFiles sum failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
			return nil, err
		}
		totalSize = cast.ToInt64(totalSizeStr)
	}

	// Paginated list.
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Sort field is restricted to file_size / created_at, sort direction to asc / desc, to prevent SQL injection.
	allowedSort := map[string]bool{`file_size`: true, `created_at`: true}
	if !allowedSort[sortField] {
		sortField = `created_at`
	}
	sortDir := `DESC`
	if sortOrder == `asc` || sortOrder == `ASC` {
		sortDir = `ASC`
	}
	orderBy := sortField + ` ` + sortDir

	list, err := m.Where2(where).
		Field(`id, admin_id, workspace_id, source_type, source_id, source_name, comment_id, document_id, file_name, file_ext, file_size, stored_path, content_type, uploader_id, uploader_name, created_at`).
		Order(orderBy).
		Limit(offset, pageSize).
		Select()
	if err != nil {
		logs.Error("ListEnterpriseFiles select failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return nil, err
	}

	result := make([]EnterpriseFile, 0, len(list))
	knowledgeIDs := make([]string, 0, len(list))
	for _, row := range list {
		item := EnterpriseFile{
			ID:           cast.ToInt(row[`id`]),
			AdminID:      cast.ToInt(row[`admin_id`]),
			WorkspaceID:  cast.ToInt(row[`workspace_id`]),
			SourceType:   row[`source_type`],
			SourceID:     cast.ToInt(row[`source_id`]),
			SourceName:   row[`source_name`],
			CommentID:    cast.ToInt(row[`comment_id`]),
			DocumentID:   cast.ToInt(row[`document_id`]),
			FileName:     row[`file_name`],
			FileExt:      row[`file_ext`],
			FileSize:     cast.ToInt64(row[`file_size`]),
			StoredPath:   row[`stored_path`],
			ContentType:  row[`content_type`],
			UploaderID:   cast.ToInt(row[`uploader_id`]),
			UploaderName: row[`uploader_name`],
			CreatedAt:    cast.ToInt64(row[`created_at`]),
		}
		if item.SourceType == define.SourceTypeKnowledge && item.DocumentID > 0 {
			knowledgeIDs = append(knowledgeIDs, cast.ToString(item.DocumentID))
		}
		result = append(result, item)
	}

	// Knowledge rows have no source_name stored; batch-fill the document title so the storage page
	// can show the document name and deep-link into the knowledge base.
	if len(knowledgeIDs) > 0 {
		titles, err := queryKnowledgeItemTitles(adminID, knowledgeIDs)
		if err != nil {
			return nil, err
		}
		for i := range result {
			if result[i].SourceType == define.SourceTypeKnowledge && titles[result[i].DocumentID] != "" {
				result[i].SourceName = titles[result[i].DocumentID]
			}
		}
	}

	return &EnterpriseFileListResult{
		List:      result,
		Total:     count,
		TotalSize: totalSize,
	}, nil
}

// queryKnowledgeItemTitles returns a map of knowledge_items.id -> title for the given ids,
// scoped to the tenant and excluding soft-deleted items.
func queryKnowledgeItemTitles(adminID int, ids []string) (map[int]string, error) {
	m := msql.Model(define.TableKnowledgeItems, define.Postgres)
	rows, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminID)},
		{`id`, `IN`, strings.Join(ids, `,`)},
		{`deleted_at`, `0`},
	}).Field(`id, title`).Select()
	if err != nil {
		logs.Error("queryKnowledgeItemTitles select failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	result := make(map[int]string, len(rows))
	for _, row := range rows {
		result[cast.ToInt(row[`id`])] = row[`title`]
	}
	return result, nil
}

// GetEnterpriseFileByID returns a single enterprise file record.
func GetEnterpriseFileByID(adminID int, fileID int) (*EnterpriseFile, error) {
	tableName := define.GetEnterpriseFileTable(adminID)
	m := msql.Model(tableName, define.Postgres)
	row, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminID)},
		{`id`, cast.ToString(fileID)},
	}).Find()
	if err != nil {
		return nil, err
	}
	if len(row) == 0 {
		return nil, ErrAttachmentNotFound
	}
	return &EnterpriseFile{
		ID:           cast.ToInt(row[`id`]),
		AdminID:      cast.ToInt(row[`admin_id`]),
		WorkspaceID:  cast.ToInt(row[`workspace_id`]),
		SourceType:   row[`source_type`],
		SourceID:     cast.ToInt(row[`source_id`]),
		SourceName:   row[`source_name`],
		CommentID:    cast.ToInt(row[`comment_id`]),
		DocumentID:   cast.ToInt(row[`document_id`]),
		FileName:     row[`file_name`],
		FileExt:      row[`file_ext`],
		FileSize:     cast.ToInt64(row[`file_size`]),
		StoredPath:   row[`stored_path`],
		ContentType:  row[`content_type`],
		UploaderID:   cast.ToInt(row[`uploader_id`]),
		UploaderName: row[`uploader_name`],
		CreatedAt:    cast.ToInt64(row[`created_at`]),
	}, nil
}

// DeleteEnterpriseFile deletes a single enterprise file record and the physical file.
// Deletion order: delete the DB ledger record first, then the physical file.
// This way, even if physical file deletion fails, only orphaned files remain without causing ledger-disk inconsistency.
// The legacy requirement_attachments table is an archive and is no longer modified (see migration 20260901103359).
func DeleteEnterpriseFile(adminID int, fileID int) error {
	file, err := GetEnterpriseFileByID(adminID, fileID)
	if err != nil {
		return err
	}

	// 1. Delete the enterprise file ledger record (critical step; returns immediately on failure).
	tableName := define.GetEnterpriseFileTable(adminID)
	m := msql.Model(tableName, define.Postgres)
	if _, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminID)},
		{`id`, cast.ToString(fileID)},
	}).Delete(); err != nil {
		logs.Error("DeleteEnterpriseFile delete record failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return err
	}
	adjustStorageUsedCache(adminID, -file.FileSize)

	// 3. Finally delete the physical file (OSS or local, automatically determined by lib_storage based on stored_path).
	//    Physical deletion failure only leaves orphaned files without affecting ledger consistency, so only a warning is logged.
	if err := lib_storage.M.Delete(file.StoredPath); err != nil {
		logs.Error("DeleteEnterpriseFile remove physical file failed: path=%s, err=%s", file.StoredPath, err.Error())
	}

	return nil
}

// BatchDeleteEnterpriseFiles deletes multiple enterprise file records and their physical files.
func BatchDeleteEnterpriseFiles(adminID int, fileIDs []int) error {
	for _, fileID := range fileIDs {
		if err := DeleteEnterpriseFile(adminID, fileID); err != nil {
			return err
		}
	}
	return nil
}

// storageUsedCacheTTL is the TTL of the tenant storage usage cache. Mutations adjust the cached
// value, and the daily expiry rebuilds it from the SUM so any missed-path drift self-heals.
const storageUsedCacheTTL = 24 * time.Hour

// storageUsedCacheHandler implements lib_redis.CacheBuildHandler for the tenant storage usage cache.
type storageUsedCacheHandler struct {
	adminID int
}

// GetCacheKey returns the redis key of the tenant storage usage cache.
// v2: knowledge-base upload/delete paths now adjust the cache; bumping the version
// invalidates values cached before that fix so the next read rebuilds from the SUM.
func (h storageUsedCacheHandler) GetCacheKey() string {
	return fmt.Sprintf(`goteams:storage_used:v2:%d`, h.adminID)
}

// GetCacheData 从累计用量列重建缓存，有效期 1 天。
func (h storageUsedCacheHandler) GetCacheData() (any, time.Duration, error) {
	used, err := sumEnterpriseStorageUsed(h.adminID)
	if err != nil {
		return nil, 0, err
	}
	return used, storageUsedCacheTTL, nil
}

// GetEnterpriseStorageUsed 返回租户累计已用存储（字节）。
// 读取 1 天缓存；只有新增写入会增加缓存，删除不会减少。
func GetEnterpriseStorageUsed(adminID int) (int64, error) {
	var used int64
	if err := lib_redis.GetCacheWithBuild(define.Redis, storageUsedCacheHandler{adminID: adminID}, &used); err != nil {
		return 0, err
	}
	return used, nil
}

// sumEnterpriseStorageUsed 读取累计已用字节。这个数只增不减，不能改回当前文件合计。
func sumEnterpriseStorageUsed(adminID int) (int64, error) {
	m := msql.Model(define.TableTenantPlans, define.Postgres)
	totalStr, err := m.Where(`admin_id`, cast.ToString(adminID)).Value(`storage_used_bytes`)
	if err != nil {
		logs.Error("read cumulative storage failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return 0, err
	}
	return cast.ToInt64(totalStr), nil
}

// adjustStorageUsedCache 把新写入的字节累加到租户用量。删除不调用本函数回收：
// 缓存没有时下次读取会从累计列重建，减量如果写进缓存，重建前会把用量算少。
func adjustStorageUsedCache(adminID int, delta int64) {
	if adminID <= 0 || delta <= 0 {
		return
	}
	if _, err := msql.RawExec(define.Postgres, `UPDATE `+define.TableTenantPlans+`
		SET storage_used_bytes = storage_used_bytes + $1, updated_at = $2
		WHERE admin_id = $3`, nil, delta, tool.Time2Int(), adminID); err != nil {
		logs.Error("add cumulative storage failed: admin_id=%d, delta=%d, err=%s", adminID, delta, err.Error())
		return
	}
	key := storageUsedCacheHandler{adminID: adminID}.GetCacheKey()
	ctx := context.Background()
	exists, err := define.Redis.Exists(ctx, key).Result()
	if err != nil || exists == 0 {
		return
	}
	if err = define.Redis.IncrBy(ctx, key, delta).Err(); err != nil {
		logs.Error("adjust storage used cache incr failed: key=%s, delta=%d, err=%s", key, delta, err.Error())
	}
}

// CheckStorageQuota 按累计已用空间判断本次写入是否放行。
// addBytes 是这次要新增的字节；传 0 时只拒绝已经超过上限的租户，供创建需求等操作复用。
// 上限小于等于 0 视为不限制。比较使用严格大于，刚好用满仍允许本次不再新增字节的操作。
func CheckStorageQuota(adminID int, addBytes int64) error {
	plan, err := GetPlan(adminID)
	if err != nil {
		return err
	}
	if plan.StorageLimit <= 0 {
		return nil
	}
	// The frontend uses 1024^3 as the conversion factor for 1 GB; keep it consistent here.
	limitBytes := int64(plan.StorageLimit) * 1024 * 1024 * 1024
	usedBytes, err := GetEnterpriseStorageUsed(adminID)
	if err != nil {
		return err
	}
	if usedBytes+addBytes > limitBytes {
		return &QuotaExceededError{
			Key:   define.QuotaStorage,
			Scope: `total`,
			Used:  int(usedBytes),
			Limit: plan.StorageLimit,
		}
	}
	return nil
}

// GetWorkspaceNameMap returns a map of workspace_id → workspace name for the given tenant.
func GetWorkspaceNameMap(adminID int) (map[int]string, error) {
	m := msql.Model(define.TableWorkspaces, define.Postgres)
	list, err := m.Where(`admin_id`, cast.ToString(adminID)).
		Field(`id, name`).
		Select()
	if err != nil {
		return nil, err
	}
	result := make(map[int]string, len(list))
	for _, row := range list {
		result[cast.ToInt(row[`id`])] = row[`name`]
	}
	return result, nil
}

// RecordKnowledgeEnterpriseFile writes a knowledge document editor attachment directly bound to its
// owning document (source_type='knowledge' + document_id + folder_id), so no later rebind is needed.
// The upload endpoint already verified the document exists and the uploader may edit it.
func RecordKnowledgeEnterpriseFile(adminID, workspaceID, uploaderID, documentID, folderID int, originalName, storedPath, contentType string, fileSize int64) (int, error) {
	tableName := define.GetEnterpriseFileTable(adminID)
	uploaderName := ""
	if uploaderID > 0 {
		u := msql.Model(define.TableUsers, define.Postgres)
		userRow, err := u.Where2([][]string{
			{`admin_id`, cast.ToString(adminID)},
			{`id`, cast.ToString(uploaderID)},
		}).Value(`name`)
		if err == nil {
			uploaderName = userRow
		}
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(originalName)), ".")
	now := tool.Time2Int()
	id, err := msql.Model(tableName, define.Postgres).Insert(msql.Datas{
		`admin_id`:      adminID,
		`workspace_id`:  workspaceID,
		`source_type`:   define.SourceTypeKnowledge,
		`source_id`:     0,
		`source_name`:   "",
		`comment_id`:    0,
		`file_name`:     originalName,
		`file_ext`:      ext,
		`file_size`:     fileSize,
		`stored_path`:   storedPath,
		`content_type`:  contentType,
		`uploader_id`:   uploaderID,
		`uploader_name`: uploaderName,
		`folder_id`:     folderID,
		`document_id`:   documentID,
		`created_at`:    now,
		`updated_at`:    now,
	}, `id`)
	if err != nil {
		logs.Error("RecordKnowledgeEnterpriseFile insert failed: table=%s, err=%s", tableName, err.Error())
		return 0, err
	}
	adjustStorageUsedCache(adminID, fileSize)
	return cast.ToInt(id), nil
}

// UploadDashboardAttachment stores one pending rich-text asset after verifying that the caller
// may edit the target dashboard. It is bound only when the card or announcement save succeeds.
func UploadDashboardAttachment(adminId, workspaceId, userId, dashboardId int, file *multipart.FileHeader, isImage, isVideo bool) (DashboardAttachmentModel, error) {
	if file == nil || file.Size <= 0 {
		return DashboardAttachmentModel{}, ErrDashboardAttachmentInvalid
	}
	maxSize := int64(MaxEditorFileSize)
	allowed := IsAllowedEditorFileExt
	if isImage {
		maxSize = MaxImageSize
		allowed = IsAllowedImageExt
	} else if isVideo {
		maxSize = MaxEditorVideoSize
		allowed = IsAllowedEditorVideoExt
	}
	if file.Size > maxSize {
		return DashboardAttachmentModel{}, ErrDashboardAttachmentTooLarge
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(file.Filename)), `.`)
	if !allowed(ext) {
		return DashboardAttachmentModel{}, ErrDashboardAttachmentInvalid
	}
	dashboard, err := getDashboardForActor(adminId, workspaceId, userId, false, dashboardId)
	if err != nil {
		return DashboardAttachmentModel{}, err
	}
	if !dashboard.CanEdit {
		return DashboardAttachmentModel{}, ErrDashboardForbidden
	}
	opened, err := file.Open()
	if err != nil {
		return DashboardAttachmentModel{}, err
	}
	defer func() { _ = opened.Close() }()
	storedPath, err := lib_storage.M.Upload(adminId, lib_storage.BusinessDashboard, ext, opened)
	if err != nil {
		return DashboardAttachmentModel{}, err
	}
	fileID, err := RecordPendingEnterpriseFile(adminId, workspaceId, userId, file.Filename, storedPath, file.Header.Get(`Content-Type`), file.Size)
	if err != nil {
		if deleteErr := lib_storage.M.Delete(storedPath); deleteErr != nil {
			logs.Error("cleanup dashboard upload after ledger failure failed, err:%s", deleteErr.Error())
		}
		return DashboardAttachmentModel{}, err
	}
	return DashboardAttachmentModel{
		FileId: fileID, Url: storedPath, OriginalName: file.Filename,
		MimeType: file.Header.Get(`Content-Type`), FileSize: file.Size,
	}, nil
}

// bindDashboardPendingFilesTx claims only unbound dashboard-namespace assets uploaded by the
// current actor. Existing bound assets are deliberately untouched when a later payload omits them.
func bindDashboardPendingFilesTx(tx *sql.Tx, adminId, workspaceId, userId, sourceId int, sourceType, sourceName string, fileIds []int) error {
	if len(fileIds) == 0 {
		return nil
	}
	if tx == nil || sourceId <= 0 || sourceType == `` || validate.DashboardFileIDs(fileIds) != nil {
		return ErrDashboardAttachmentInvalid
	}
	files, err := msql.RawValues(define.Postgres, `SELECT id, stored_path FROM `+define.GetEnterpriseFileTable(adminId)+`
		WHERE admin_id = $1 AND workspace_id = $2 AND source_id = 0 AND uploader_id = $3
			AND id = ANY($4::INTEGER[]) FOR UPDATE`, tx,
		adminId, workspaceId, userId, pq.Array(fileIds))
	if err != nil {
		logs.Error("lock pending dashboard files failed, err:%s", err.Error())
		return err
	}
	if len(files) != len(fileIds) {
		return ErrDashboardAttachmentInvalid
	}
	for _, file := range files {
		if !lib_storage.M.IsOwned(adminId, lib_storage.BusinessDashboard, file[`stored_path`]) {
			return ErrDashboardAttachmentInvalid
		}
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.GetEnterpriseFileTable(adminId)+`
		SET source_type = $1, source_id = $2, source_name = $3, updated_at = $4
		WHERE admin_id = $5 AND workspace_id = $6 AND source_id = 0 AND uploader_id = $7
			AND id = ANY($8::INTEGER[])`, tx,
		sourceType, sourceId, sourceName, tool.Time2Int(), adminId, workspaceId, userId, pq.Array(fileIds)); err != nil {
		logs.Error("bind pending dashboard files failed, err:%s", err.Error())
		return err
	}
	return nil
}

// copyDashboardRichTextAttachmentsTx duplicates source rich-text ledger assets and rewrites their
// managed URLs in content. Each new object is tracked so the caller can compensate after rollback
// or commit failure; source objects are never shared with the copied dashboard.
func copyDashboardRichTextAttachmentsTx(tx *sql.Tx, adminId, sourceWorkspaceId, targetWorkspaceId int,
	sourceType string, sourceId int, targetType string, targetId, operatorId int, targetName, content string,
	createdPaths *[]string) (string, error) {
	if tx == nil || sourceId <= 0 || targetId <= 0 || sourceType == `` || targetType == `` || createdPaths == nil {
		return ``, ErrDashboardAttachmentInvalid
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT file_name, file_ext, file_size, stored_path, content_type
		FROM `+define.GetEnterpriseFileTable(adminId)+`
		WHERE admin_id = $1 AND workspace_id = $2 AND source_type = $3 AND source_id = $4
		ORDER BY id ASC FOR UPDATE`, tx, adminId, sourceWorkspaceId, sourceType, sourceId)
	if err != nil {
		logs.Error("load dashboard rich-text attachments for copy failed, err:%s", err.Error())
		return ``, err
	}
	if len(rows) == 0 {
		return content, nil
	}
	uploaderName, err := dashboardAttachmentUploaderName(adminId, operatorId)
	if err != nil {
		return ``, err
	}
	now := int64(tool.Time2Int())
	result := content
	for _, row := range rows {
		oldPath := row[`stored_path`]
		if !lib_storage.M.IsOwned(adminId, lib_storage.BusinessDashboard, oldPath) {
			return ``, ErrDashboardAttachmentInvalid
		}
		newPath, copyErr := lib_storage.M.Copy(oldPath, adminId, lib_storage.BusinessDashboard, lib_storage.BusinessDashboard, row[`file_ext`])
		if copyErr != nil {
			return ``, copyErr
		}
		*createdPaths = append(*createdPaths, newPath)
		_, key, parseErr := lib_storage.ParseStored(newPath)
		if parseErr != nil {
			return ``, parseErr
		}
		if _, copyErr = msql.RawExec(define.Postgres, `INSERT INTO `+define.GetEnterpriseFileTable(adminId)+`
			(admin_id, workspace_id, source_type, source_id, source_name, comment_id,
			 file_name, file_ext, file_size, stored_path, content_type, uploader_id, uploader_name,
			 created_at, updated_at, stored_name, folder_id, document_id)
			VALUES ($1, $2, $3, $4, $5, 0, $6, $7, $8, $9, $10, $11, $12, $13, $13, $14, 0, 0)`, tx,
			adminId, targetWorkspaceId, targetType, targetId, targetName,
			row[`file_name`], row[`file_ext`], cast.ToInt64(row[`file_size`]), newPath, row[`content_type`],
			operatorId, uploaderName, now, filepath.Base(key)); copyErr != nil {
			logs.Error("insert copied dashboard rich-text attachment failed, err:%s", copyErr.Error())
			return ``, copyErr
		}
		result = strings.ReplaceAll(result, oldPath, newPath)
	}
	return result, nil
}

func dashboardAttachmentUploaderName(adminId, userId int) (string, error) {
	if userId <= 0 {
		return ``, nil
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT name FROM `+define.TableUsers+`
		WHERE admin_id = $1 AND id = $2 LIMIT 1`, nil, adminId, userId)
	if err != nil {
		logs.Error("load dashboard attachment uploader failed, err:%s", err.Error())
		return ``, err
	}
	if len(rows) == 0 {
		return ``, fmt.Errorf("dashboard attachment uploader not found")
	}
	return rows[0][`name`], nil
}

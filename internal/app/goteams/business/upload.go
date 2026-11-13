// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"

	"goteams/internal/pkg/lib_storage"
)

// MaxImageSize is the per-image size limit (20MB), serving as a server-side safeguard; the frontend validates at the same 20MB.
const MaxImageSize = 20 * 1024 * 1024

// IssueInviteThemeImageMaxSize is the invitation guide theme-image limit (750*360, 500KB).
const IssueInviteThemeImageMaxSize = 500 * 1024

// MaxEditorFileSize is the maximum size of a document inserted into rich text.
const MaxEditorFileSize int64 = 200 * 1024 * 1024

// MaxEditorVideoSize is the maximum size of a video inserted into rich text.
const MaxEditorVideoSize int64 = 200 * 1024 * 1024

// allowedImageExt lists the allowed image extensions (svg excluded to avoid XSS).
var allowedImageExt = map[string]bool{
	"jpg":  true,
	"jpeg": true,
	"png":  true,
	"gif":  true,
	"webp": true,
}

var allowedEditorFileExt = map[string]bool{
	"pdf": true, "doc": true, "docx": true, "xls": true, "xlsx": true,
	"ppt": true, "pptx": true, "txt": true, "csv": true, "md": true,
	"zip": true, "rar": true, "7z": true, "html": true, "htm": true,
}

var allowedEditorVideoExt = map[string]bool{
	"mp4":  true,
	"webm": true,
}

// IsAllowedImageExt reports whether the extension is an allowed image type (ext has no dot, e.g. "png").
// Used by the controller layer for unified client-side validation, avoiding misjudgment caused by a spoofed Content-Type header.
func IsAllowedImageExt(ext string) bool {
	return allowedImageExt[ext]
}

// IsAllowedEditorFileExt 判断扩展名是否允许作为正文附件上传。
func IsAllowedEditorFileExt(ext string) bool {
	return allowedEditorFileExt[ext]
}

// IsAllowedEditorVideoExt reports whether an extension is supported for an inline video.
func IsAllowedEditorVideoExt(ext string) bool {
	return allowedEditorVideoExt[ext]
}

// UploadImage writes an editor-inserted image to storage and records it in the enterprise file ledger.
func UploadImage(adminID, workspaceID, uploaderID int, file *multipart.FileHeader) (url string, fileID int, err error) {
	return uploadEditorAsset(adminID, workspaceID, uploaderID, file)
}

// UploadEditorFile writes an editor-inserted document to storage and records it in the enterprise file ledger.
func UploadEditorFile(adminID, workspaceID, uploaderID int, file *multipart.FileHeader) (url string, fileID int, err error) {
	return uploadEditorAsset(adminID, workspaceID, uploaderID, file)
}

// UploadEditorVideo writes an editor-inserted video to storage and records it in the enterprise file ledger.
func UploadEditorVideo(adminID, workspaceID, uploaderID int, file *multipart.FileHeader) (url string, fileID int, err error) {
	return uploadEditorAsset(adminID, workspaceID, uploaderID, file)
}

// uploadEditorAsset stores a rich-text asset as an unbound ledger entry until the requirement is saved.
func uploadEditorAsset(adminID, workspaceID, uploaderID int, file *multipart.FileHeader) (url string, fileID int, err error) {
	if file == nil {
		return "", 0, fmt.Errorf("file is nil")
	}
	if err = CheckStorageQuota(adminID, file.Size); err != nil {
		return "", 0, err
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(file.Filename)), ".")
	opened, err := file.Open()
	if err != nil {
		return "", 0, err
	}
	defer opened.Close()
	stored, err := lib_storage.M.Upload(adminID, lib_storage.BusinessRequirement, ext, opened)
	if err != nil {
		return "", 0, err
	}
	fileID, err = RecordPendingEnterpriseFile(adminID, workspaceID, uploaderID, file.Filename, stored, file.Header.Get("Content-Type"), file.Size)
	if err != nil {
		return "", 0, err
	}
	return stored, fileID, nil
}

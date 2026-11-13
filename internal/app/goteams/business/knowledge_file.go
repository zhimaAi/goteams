// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"bytes"
	"errors"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_storage"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// CreateKnowledgeDocumentFromFile stores an uploaded local file and creates its knowledge document atomically.
func CreateKnowledgeDocumentFromFile(adminId, wsId, userId, parentId int, file *multipart.FileHeader) (KnowledgeItemModel, error) {
	if file == nil || validate.ValidateKnowledgeFileUploadSize(file.Size) != nil {
		return KnowledgeItemModel{}, ErrKnowledgeAttachmentTooLarge
	}
	if err := checkKnowledgeParent(adminId, wsId, parentId); err != nil {
		return KnowledgeItemModel{}, err
	}

	originalName := knowledgeUploadFileName(file.Filename)
	if originalName == `` {
		originalName = `untitled`
	}
	title := truncateKnowledgeRunes(originalName, validate.KnowledgeTitleMaxLen)
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(originalName)), `.`)
	content, contentMode, uploadReader, err := readKnowledgeUpload(file, ext)
	if err != nil {
		return KnowledgeItemModel{}, err
	}
	defer func() { _ = uploadReader.Close() }()

	storedExt := safeKnowledgeStoredExtension(ext)
	storedPath, err := lib_storage.M.Upload(adminId, lib_storage.BusinessKnowledgeBase, storedExt, uploadReader)
	if err != nil {
		logs.Error("upload knowledge source file failed, err:%s", err.Error())
		return KnowledgeItemModel{}, err
	}
	committed := false
	defer func() {
		if !committed {
			if deleteErr := lib_storage.M.Delete(storedPath); deleteErr != nil {
				logs.Error("cleanup knowledge source file failed, err:%s", deleteErr.Error())
			}
		}
	}()

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin create knowledge file transaction failed, err:%s", err.Error())
		return KnowledgeItemModel{}, err
	}
	defer func() { _ = tx.Rollback() }()

	now := int64(tool.Time2Int())
	itemRows, err := msql.RawValues(define.Postgres, `INSERT INTO knowledge_items
		(admin_id, workspace_id, parent_id, item_type, title, revision, version_seq, creator_id, updated_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 1, 1, $6, $6, $7, $7) RETURNING id`, tx,
		adminId, wsId, parentId, define.KnowledgeItemTypeDocument, title, userId, now)
	if err != nil || len(itemRows) == 0 {
		if err == nil {
			err = errors.New(`create knowledge file item returned no id`)
		}
		logs.Error("insert knowledge file item failed, err:%s", err.Error())
		return KnowledgeItemModel{}, err
	}
	documentId := cast.ToInt(itemRows[0][`id`])
	contentHash := knowledgeContentHash(title, contentMode, content)
	if _, err = msql.RawExec(define.Postgres, `INSERT INTO knowledge_document_contents
		(admin_id, workspace_id, document_id, content, content_mode, content_hash, source_type, source_attachment_id, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 0, $8)`, tx,
		adminId, wsId, documentId, content, contentMode, contentHash, define.KnowledgeSourceTypeUpload, now); err != nil {
		logs.Error("insert knowledge file content failed, err:%s", err.Error())
		return KnowledgeItemModel{}, err
	}

	_, storedKey, parseErr := lib_storage.ParseStored(storedPath)
	if parseErr != nil {
		logs.Error("parse knowledge stored path failed, err:%s", parseErr.Error())
		return KnowledgeItemModel{}, parseErr
	}
	mimeType := knowledgeUploadMimeType(file, ext)
	attachmentRows, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.GetEnterpriseFileTable(adminId)+`
		(admin_id, workspace_id, source_type, source_id, source_name, comment_id,
		 file_name, file_ext, file_size, stored_path, content_type,
		 uploader_id, uploader_name, created_at, updated_at,
		 stored_name, folder_id, document_id)
		VALUES ($1, $2, $3, 0, '', 0, $4, $5, $6, $7, $8, $9, '', $10, $10, $11, $12, $13) RETURNING id`, tx,
		adminId, wsId, define.SourceTypeKnowledge, originalName, ext, file.Size, storedPath, mimeType, userId, now,
		filepath.Base(storedKey), parentId, documentId)
	if err != nil || len(attachmentRows) == 0 {
		if err == nil {
			err = errors.New(`create knowledge source attachment returned no id`)
		}
		logs.Error("insert knowledge source attachment failed, err:%s", err.Error())
		return KnowledgeItemModel{}, err
	}
	attachmentId := cast.ToInt(attachmentRows[0][`id`])
	if _, err = msql.RawExec(define.Postgres, `UPDATE knowledge_document_contents SET source_attachment_id = $1
		WHERE admin_id = $2 AND workspace_id = $3 AND document_id = $4`, tx,
		attachmentId, adminId, wsId, documentId); err != nil {
		logs.Error("bind knowledge source attachment failed, err:%s", err.Error())
		return KnowledgeItemModel{}, err
	}

	versionRows, err := msql.RawValues(define.Postgres, `INSERT INTO knowledge_document_versions
		(admin_id, workspace_id, document_id, version_no, version_type, description, restore_from_version_id,
		 edit_session_id, created_by, created_at)
		VALUES ($1, $2, $3, 1, $4, '', 0, '', $5, $6) RETURNING id`, tx,
		adminId, wsId, documentId, define.KnowledgeVersionTypeCreate, userId, now)
	if err != nil || len(versionRows) == 0 {
		if err == nil {
			err = errors.New(`create knowledge file version returned no id`)
		}
		logs.Error("insert knowledge file version failed, err:%s", err.Error())
		return KnowledgeItemModel{}, err
	}
	if _, err = msql.RawExec(define.Postgres, `INSERT INTO knowledge_document_version_contents
		(admin_id, workspace_id, version_id, title, content, content_mode, content_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, tx,
		adminId, wsId, cast.ToInt(versionRows[0][`id`]), title, content, contentMode, contentHash); err != nil {
		logs.Error("insert knowledge file version content failed, err:%s", err.Error())
		return KnowledgeItemModel{}, err
	}
	if err = writeKnowledgeActivity(tx, adminId, wsId, documentId, userId, title,
		define.KnowledgeItemTypeDocument, define.KnowledgeActionCreate, now); err != nil {
		return KnowledgeItemModel{}, err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit create knowledge file transaction failed, err:%s", err.Error())
		return KnowledgeItemModel{}, err
	}
	committed = true
	adjustStorageUsedCache(adminId, file.Size)
	return GetKnowledgeItemDetail(adminId, wsId, documentId, userId)
}

// ServeKnowledgeDocumentDownload streams the original upload or a generated docx for an editor-created document.
func ServeKnowledgeDocumentDownload(c *gin.Context, adminId, wsId, userId, documentId int) error {
	item, err := GetKnowledgeItemDetail(adminId, wsId, documentId, userId)
	if err != nil {
		return err
	}
	if item.ItemType != define.KnowledgeItemTypeDocument {
		return ErrKnowledgeDocumentNotEditable
	}
	c.Header(`Cache-Control`, `no-store`)
	c.Header(`X-Content-Type-Options`, `nosniff`)
	if item.SourceType == define.KnowledgeSourceTypeUpload && item.SourceAttachmentId > 0 && item.ContentMode != define.KnowledgeContentModeFile {
		// The uploaded file has been converted into an editable rich-text document:
		// the stored original no longer reflects the content, export the current state as a new docx.
		return serveKnowledgeGeneratedDOCX(c, adminId, item)
	}
	if item.SourceType == define.KnowledgeSourceTypeUpload && item.SourceAttachmentId > 0 {
		attachment, attachmentErr := GetKnowledgeAttachment(adminId, wsId, item.SourceAttachmentId)
		if attachmentErr != nil {
			return attachmentErr
		}
		if attachment.DocumentId != documentId || !lib_storage.M.IsOwned(adminId, lib_storage.BusinessKnowledgeBase, attachment.StoredPath) {
			return ErrKnowledgeAttachmentNotFound
		}
		c.Header(`Content-Disposition`, mime.FormatMediaType(`attachment`, map[string]string{`filename`: attachment.OriginalName}))
		if item.SourceExtension == `md` && item.ContentMode == define.KnowledgeContentModeMarkdown {
			c.Data(http.StatusOK, `text/markdown; charset=utf-8`, []byte(item.Content))
			return nil
		}
		if item.SourceExtension == `txt` && item.ContentMode == define.KnowledgeContentModeRichText {
			c.Data(http.StatusOK, `text/plain; charset=utf-8`, []byte(knowledgeHTMLPlainText(item.Content)))
			return nil
		}
		if attachment.MimeType != `` {
			c.Header(`Content-Type`, attachment.MimeType)
		}
		if serveErr := lib_storage.M.Serve(attachment.StoredPath, c); serveErr != nil {
			logs.Error("serve knowledge source file failed, stored:%s, err:%s", attachment.StoredPath, serveErr.Error())
			return ErrKnowledgeAttachmentNotFound
		}
		return nil
	}

	return serveKnowledgeGeneratedDOCX(c, adminId, item)
}

// serveKnowledgeGeneratedDOCX exports the document content as a generated OOXML docx download.
func serveKnowledgeGeneratedDOCX(c *gin.Context, adminId int, item KnowledgeItemModel) error {
	data, err := buildKnowledgeDOCX(adminId, knowledgeRequestBaseURL(c), item.Title, item.Content, item.ContentMode)
	if err != nil {
		logs.Error("build knowledge docx failed, document:%d, err:%s", item.Id, err.Error())
		return err
	}
	filename := item.Title
	if !strings.EqualFold(filepath.Ext(filename), `.docx`) {
		filename += `.docx`
	}
	c.Header(`Content-Disposition`, mime.FormatMediaType(`attachment`, map[string]string{`filename`: filename}))
	c.Data(http.StatusOK, `application/vnd.openxmlformats-officedocument.wordprocessingml.document`, data)
	return nil
}

func knowledgeRequestBaseURL(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ``
	}
	if browserOrigin := knowledgeValidatedResourceOrigin(c.GetHeader(`X-GoTeams-Resource-Origin`)); browserOrigin != `` {
		return browserOrigin
	}
	scheme := `http`
	if c.Request.TLS != nil {
		scheme = `https`
	}
	host := strings.TrimSpace(c.Request.Host)
	if knowledgeRequestFromTrustedProxy(c.Request.RemoteAddr) {
		if forwardedProto := knowledgeFirstForwardedValue(c.GetHeader(`X-Forwarded-Proto`)); forwardedProto == `http` || forwardedProto == `https` {
			scheme = forwardedProto
		}
		if forwardedHost := knowledgeFirstForwardedValue(c.GetHeader(`X-Forwarded-Host`)); forwardedHost != `` {
			host = forwardedHost
		}
	}
	if host == `` || strings.ContainsAny(host, `/\\?#@ \t\r\n`) {
		return ``
	}
	baseURL := (&url.URL{Scheme: scheme, Host: host}).String()
	return strings.TrimRight(baseURL, `/`)
}

func knowledgeValidatedResourceOrigin(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed == nil || parsed.User != nil || parsed.Host == `` ||
		(parsed.Scheme != `http` && parsed.Scheme != `https`) ||
		(parsed.Path != `` && parsed.Path != `/`) || parsed.RawQuery != `` || parsed.Fragment != `` {
		return ``
	}
	return parsed.Scheme + `://` + parsed.Host
}

func knowledgeRequestFromTrustedProxy(remoteAddress string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddress))
	if err != nil {
		host = strings.Trim(strings.TrimSpace(remoteAddress), `[]`)
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate())
}

func knowledgeFirstForwardedValue(value string) string {
	if comma := strings.IndexByte(value, ','); comma >= 0 {
		value = value[:comma]
	}
	return strings.ToLower(strings.TrimSpace(value))
}

func readKnowledgeUpload(file *multipart.FileHeader, ext string) (string, string, multipart.File, error) {
	opened, err := file.Open()
	if err != nil {
		return ``, ``, nil, err
	}
	if (ext == `txt` || ext == `md`) && file.Size <= validate.KnowledgeContentMaxBytes {
		data, readErr := io.ReadAll(opened)
		_ = opened.Close()
		if readErr != nil {
			return ``, ``, nil, readErr
		}
		mode := define.KnowledgeContentModeRichText
		content := escapeKnowledgePlainText(string(data))
		if ext == `md` {
			mode = define.KnowledgeContentModeMarkdown
			content = string(data)
		}
		content = lib_storage.StripOSSURLSignatures(content)
		return content, mode, &memoryMultipartFile{Reader: bytes.NewReader(data)}, nil
	}
	return ``, define.KnowledgeContentModeFile, opened, nil
}

type memoryMultipartFile struct {
	*bytes.Reader
}

func (f *memoryMultipartFile) Close() error { return nil }

func knowledgeUploadFileName(name string) string {
	name = strings.ReplaceAll(strings.TrimSpace(name), `\`, `/`)
	parts := strings.Split(name, `/`)
	name = parts[len(parts)-1]
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	return truncateKnowledgeRunes(name, 500)
}

func truncateKnowledgeRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) > max {
		runes = runes[:max]
	}
	return string(runes)
}

func safeKnowledgeStoredExtension(ext string) string {
	if ext == `` || len(ext) > 20 {
		return `bin`
	}
	for _, r := range ext {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return `bin`
		}
	}
	return ext
}

func knowledgeUploadMimeType(file *multipart.FileHeader, ext string) string {
	value := strings.TrimSpace(file.Header.Get(`Content-Type`))
	if value == `` || value == `application/octet-stream` {
		if detected := mime.TypeByExtension(`.` + ext); detected != `` {
			return detected
		}
	}
	if value == `` {
		return `application/octet-stream`
	}
	return value
}

func escapeKnowledgePlainText(text string) string {
	text = html.EscapeString(text)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.ReplaceAll(text, "\n", `<br>`)
}

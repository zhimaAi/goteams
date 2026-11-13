// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
	"goteams/internal/pkg/lib_storage"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

const (
	// pipelineDocumentMaxFileSize 与 business 侧的上限保持一致，用于 HTTP 层提前拒绝超大请求体。
	pipelineDocumentMaxFileSize = 20 << 20
	// pipelineDocumentFormOverhead 是 multipart 头与其它字段的余量。
	pipelineDocumentFormOverhead = 1 << 20
)

// pushPipelineTaskDocumentRequest 是客户端上报产出文档的 multipart 表单字段。
type pushPipelineTaskDocumentRequest struct {
	LocalTaskUUID   string
	ExecutionIndex  int
	DocKey          string
	RelPath         string
	Title           string
	FileExt         string
	FileSize        int64
	ContentSha256   string
	ClientUpdatedAt int64
	SourceAbsPath   string
}

// pushPipelineTaskDocumentResponse 是上报结果；result 的取值域由客户端硬依赖。
type pushPipelineTaskDocumentResponse struct {
	DocumentId   int64  `json:"document_id"`
	DocKey       string `json:"doc_key"`
	Revision     int64  `json:"revision"`
	StorageState string `json:"storage_state"`
	Result       string `json:"result"`
}

// pipelineTaskDocumentsResponse 是一个任务的产出文档清单，目录树由前端按 rel_path 分段构建。
// 工作项字段供文档整页上的「任务详情」抽屉直接使用，避免前端为取归属再调一次任务接口。
type pipelineTaskDocumentsResponse struct {
	TaskId       int64                          `json:"task_id"`
	TaskTitle    string                         `json:"task_title"`
	WorkspaceId  int                            `json:"workspace_id"`
	WorkItemId   int64                          `json:"work_item_id"`
	WorkItemType string                         `json:"work_item_type"`
	RootCount    int                            `json:"root_count"`
	Documents    []pipelineTaskDocumentResponse `json:"documents"`
}

// pipelineTaskDocumentResponse 是单个产出文档的元数据与正文。
type pipelineTaskDocumentResponse struct {
	DocumentId      int64  `json:"document_id"`
	DocKey          string `json:"doc_key"`
	RelPath         string `json:"rel_path"`
	Title           string `json:"title"`
	FileExt         string `json:"file_ext"`
	MimeType        string `json:"mime_type"`
	ContentMode     string `json:"content_mode"`
	FileSize        int64  `json:"file_size"`
	StorageState    string `json:"storage_state"`
	StorageError    string `json:"storage_error"`
	Revision        int64  `json:"revision"`
	ClientUpdatedAt int64  `json:"client_updated_at"`
	UpdatedAt       int64  `json:"updated_at"`
	SourceAbsPath   string `json:"source_abs_path"`
	UploaderName    string `json:"uploader_name"`
	Content         string `json:"content,omitempty"`
}

// pipelineTaskDocumentLinkResponse 是 2 小时时效的查看/下载链接。
type pipelineTaskDocumentLinkResponse struct {
	Link        string `json:"link"`
	ExpiresIn   int    `json:"expires_in"`
	Disposition string `json:"disposition"`
}

// PushPipelineTaskDocument 接收客户端上报的一个产出文档：落盘暂存并记账入队，接口内不做对象存储 IO。
func PushPipelineTaskDocument(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	userId := cast.ToInt(c.GetInt(`userId`))

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body,
		int64(pipelineDocumentMaxFileSize+pipelineDocumentFormOverhead))
	// 只把 1MB 留在内存，超出部分由标准库落到临时目录，20MB 文件不会整块堆在堆上。
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		tooLarge := &http.MaxBytesError{}
		if errors.As(err, &tooLarge) {
			abortTooLarge(c, lib_i18n.Show(lang, define.MsgPipelineDocumentTooLarge), `pipeline.document.too_large`, err)
			return
		}
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.document.form`, err)
		return
	}
	req := pushPipelineTaskDocumentRequest{
		LocalTaskUUID:   strings.TrimSpace(c.PostForm(`local_task_uuid`)),
		ExecutionIndex:  cast.ToInt(c.PostForm(`execution_index`)),
		DocKey:          strings.TrimSpace(c.PostForm(`doc_key`)),
		RelPath:         c.PostForm(`rel_path`),
		Title:           c.PostForm(`title`),
		FileExt:         c.PostForm(`file_ext`),
		FileSize:        cast.ToInt64(c.PostForm(`file_size`)),
		ContentSha256:   strings.TrimSpace(c.PostForm(`content_sha256`)),
		ClientUpdatedAt: cast.ToInt64(c.PostForm(`client_updated_at`)),
		SourceAbsPath:   c.PostForm(`source_abs_path`),
	}
	if req.LocalTaskUUID == `` || req.DocKey == `` || req.RelPath == `` || req.ContentSha256 == `` {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.document.required`, errInvalidID)
		return
	}
	fileHeader, err := c.FormFile(`file`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.document.file`, err)
		return
	}
	if fileHeader.Size > pipelineDocumentMaxFileSize {
		abortTooLarge(c, lib_i18n.Show(lang, define.MsgPipelineDocumentTooLarge), `pipeline.document.size`, business.ErrPipelineDocumentTooLarge)
		return
	}
	// 存储配额：产出文档不进 enterprise_files 台账，必须把它自己的占用一起算进来。
	if err = business.CheckPipelineDocumentQuota(adminId, fileHeader.Size); err != nil {
		if abortQuotaExceeded(c, lib_i18n.Show(lang, define.MsgQuotaExceeded), `pipeline.document.quota`, err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `pipeline.document.quota`, err)
		return
	}
	staged, err := stagePipelineDocumentFile(fileHeader)
	if err != nil {
		if errors.Is(err, business.ErrPipelineDocumentTooLarge) {
			abortTooLarge(c, lib_i18n.Show(lang, define.MsgPipelineDocumentTooLarge), `pipeline.document.stage.size`, err)
			return
		}
		abortServiceUnavailable(c, lib_i18n.Show(lang, define.MsgPipelineDocumentBusy), `pipeline.document.stage`, err)
		return
	}

	result, err := business.SavePipelineTaskDocument(business.SavePipelineTaskDocumentModel{
		AdminId:         adminId,
		UserId:          userId,
		LocalTaskUUID:   req.LocalTaskUUID,
		ExecutionIndex:  req.ExecutionIndex,
		DocKey:          req.DocKey,
		RelPath:         req.RelPath,
		Title:           req.Title,
		FileExt:         req.FileExt,
		FileSize:        req.FileSize,
		ContentSha256:   req.ContentSha256,
		ClientUpdatedAt: req.ClientUpdatedAt,
		SourceAbsPath:   req.SourceAbsPath,
		Staged:          staged,
	})
	if err != nil {
		switch {
		case errors.Is(err, business.ErrPipelineTaskNotFound):
			abortNotFound(c, lib_i18n.Show(lang, define.MsgPipelineTaskNotFound))
		case errors.Is(err, business.ErrPipelineDocumentTooLarge):
			abortTooLarge(c, lib_i18n.Show(lang, define.MsgPipelineDocumentTooLarge), `pipeline.document.save.size`, err)
		case errors.Is(err, business.ErrPipelineDocumentLocked):
			abortTooManyRequests(c, lib_i18n.Show(lang, define.MsgPipelineDocumentBusy), `pipeline.document.save.locked`, err)
		case errors.Is(err, business.ErrPipelineDocumentTypeInvalid),
			errors.Is(err, business.ErrPipelineDocumentHashMismatch):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.document.save.validate`, err)
		default:
			abortServiceUnavailable(c, lib_i18n.Show(lang, define.MsgPipelineDocumentBusy), `pipeline.document.save`, err)
		}
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: pushPipelineTaskDocumentResponse{
			DocumentId:   result.DocumentId,
			DocKey:       result.DocKey,
			Revision:     result.Revision,
			StorageState: result.StorageState,
			Result:       result.Result,
		},
	})
}

// stagePipelineDocumentFile 把上传的文件流写入暂存盘，避免把整个文件读进内存。
func stagePipelineDocumentFile(header *multipart.FileHeader) (business.StagedDocumentFile, error) {
	file, err := header.Open()
	if err != nil {
		return business.StagedDocumentFile{}, err
	}
	defer func() { _ = file.Close() }()
	return business.StageDocumentFile(file, pipelineDocumentMaxFileSize)
}

// ListPipelineTaskDocuments 返回一个任务的产出文档清单（扁平，含目录相对路径）。
func ListPipelineTaskDocuments(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	taskId, err := positiveParam(c, `taskId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.document.list.param`, err)
		return
	}
	result, err := business.ListPipelineTaskDocuments(adminId, int64(taskId))
	if err != nil {
		if errors.Is(err, business.ErrPipelineTaskNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgPipelineTaskNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `pipeline.document.list`, err)
		return
	}
	documents := make([]pipelineTaskDocumentResponse, 0, len(result.Documents))
	roots := make(map[string]struct{})
	for _, document := range result.Documents {
		// 根层文件（rel_path 里没有 /）也要算一个根节点，否则 root_count 会漏掉它们。
		if root, _, found := strings.Cut(document.RelPath, `/`); found {
			roots[root] = struct{}{}
		} else if root != `` {
			roots[root] = struct{}{}
		}
		documents = append(documents, toPipelineTaskDocumentResponse(document))
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: pipelineTaskDocumentsResponse{
			TaskId:       result.TaskId,
			TaskTitle:    result.TaskTitle,
			WorkspaceId:  result.WorkspaceId,
			WorkItemId:   result.WorkItemId,
			WorkItemType: result.WorkItemType,
			RootCount:    len(roots),
			Documents:    documents,
		},
	})
}

// GetPipelineDocument 返回单个产出文档的元数据；文本类同时返回正文，供页面直接渲染。
func GetPipelineDocument(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	documentId, err := positiveParam(c, `documentId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.document.detail.param`, err)
		return
	}
	document, err := business.GetPipelineTaskDocument(adminId, int64(documentId))
	if err != nil {
		if errors.Is(err, business.ErrPipelineDocumentNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgPipelineDocumentNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `pipeline.document.detail`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toPipelineTaskDocumentResponse(document),
	})
}

// GetPipelineDocumentLink 返回产出文档的 2 小时签名链接，每次调用重新签，前端点击时才取。
func GetPipelineDocumentLink(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	documentId, err := positiveParam(c, `documentId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `pipeline.document.link.param`, err)
		return
	}
	disposition := c.Query(`disposition`)
	if disposition != `attachment` {
		disposition = `inline`
	}
	link, err := business.GetPipelineTaskDocumentLink(adminId, int64(documentId), disposition)
	if err != nil {
		switch {
		case errors.Is(err, business.ErrPipelineDocumentNotFound):
			abortNotFound(c, lib_i18n.Show(lang, define.MsgPipelineDocumentNotFound))
		case errors.Is(err, business.ErrPipelineDocumentNotReady):
			abortConflict(c, lib_i18n.Show(lang, define.MsgPipelineDocumentStorageFailed))
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `pipeline.document.link`, err)
		}
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: pipelineTaskDocumentLinkResponse{
			Link:        link,
			ExpiresIn:   int(lib_storage.DocumentSignedURLTTL / time.Minute),
			Disposition: disposition,
		},
	})
}

// toPipelineTaskDocumentResponse 把业务模型转成对外白名单字段。
func toPipelineTaskDocumentResponse(document business.PipelineTaskDocumentModel) pipelineTaskDocumentResponse {
	return pipelineTaskDocumentResponse{
		DocumentId:      document.DocumentId,
		DocKey:          document.DocKey,
		RelPath:         document.RelPath,
		Title:           document.Title,
		FileExt:         document.FileExt,
		MimeType:        document.MediaType,
		ContentMode:     document.ContentMode,
		FileSize:        document.FileSize,
		StorageState:    document.StorageState,
		StorageError:    document.StorageError,
		Revision:        document.Revision,
		ClientUpdatedAt: document.ClientUpdatedAt,
		UpdatedAt:       document.UpdatedAt,
		SourceAbsPath:   document.SourceAbsPath,
		UploaderName:    document.UploaderName,
		Content:         document.Content,
	}
}

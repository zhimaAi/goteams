// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_storage"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

const (
	// documentMaxFileSize 是单个产出文档的大小上限，客户端与云端一致，云端不信任客户端声明。
	documentMaxFileSize = 20 << 20
	// documentMaxRelPath 与建表长度一致，超出会在数据库报错前先被拒绝。
	documentMaxRelPath = 512
	// documentMaxTitle 与建表长度一致。
	documentMaxTitle = 255
	// documentMaxSourcePath 与建表长度一致。
	documentMaxSourcePath = 1024
	// documentInlineLimit 是服务端回吐正文的上限，超过只给签名链接，避免把大文件读进内存再转 JSON。
	documentInlineLimit = 1 << 20
	// documentPendingDir 是上报接口落盘的暂存目录，worker 消费成功后删除。
	// 刻意放在 data/uploads 之外：nginx 把 /uploads/ 整个目录对外静态暴露，暂存文件不能走那条路。
	documentPendingDir = `data/pending-documents`
	// documentStagingLowWatermark 是暂存盘剩余空间下限，低于它就拒绝新上报，避免把主业务的磁盘打满。
	documentStagingLowWatermark = 2 << 30
)

// 产出文档业务错误，controller 据此映射到稳定的对外状态。
var (
	ErrPipelineDocumentNotFound     = errors.New(`pipeline document not found`)
	ErrPipelineDocumentTooLarge     = errors.New(`pipeline document too large`)
	ErrPipelineDocumentTypeInvalid  = errors.New(`pipeline document type invalid`)
	ErrPipelineDocumentHashMismatch = errors.New(`pipeline document content hash mismatch`)
	ErrPipelineDocumentNotReady     = errors.New(`pipeline document not ready`)
	// ErrPipelineDocumentBusy 表示暂存盘压力过高，本次上报不应被接收。
	ErrPipelineDocumentBusy = errors.New(`pipeline document staging busy`)
	// ErrPipelineDocumentLocked 表示同一文档正在被另一次上报更新，调用方稍后重试即可。
	ErrPipelineDocumentLocked = errors.New(`pipeline document is being updated`)
)

// documentBlockedExts 是明确不允许上报的类型：可执行文件、安装包和压缩包。
var documentBlockedExts = map[string]bool{
	`exe`: true, `msi`: true, `com`: true, `scr`: true, `cmd`: true, `dll`: true,
	`apk`: true, `aar`: true, `ipa`: true, `jar`: true, `app`: true, `dmg`: true, `pkg`: true,
	`deb`: true, `rpm`: true, `iso`: true, `zip`: true, `rar`: true, `7z`: true, `tar`: true,
	`gz`: true, `bz2`: true, `xz`: true, `tgz`: true, `zst`: true,
}

// documentMarkdownExts 走 markdown 渲染，其余文本类型按纯文本展示。
var documentMarkdownExts = map[string]bool{`md`: true, `markdown`: true, `mdx`: true}

// documentTextExts 是可以安全抽正文按文本展示的类型。
var documentTextExts = map[string]bool{
	`txt`: true, `log`: true, `json`: true, `jsonl`: true, `yaml`: true, `yml`: true,
	`toml`: true, `ini`: true, `conf`: true, `csv`: true, `tsv`: true, `xml`: true,
	`html`: true, `htm`: true, `sql`: true, `css`: true, `scss`: true, `less`: true,
	`js`: true, `jsx`: true, `ts`: true, `tsx`: true, `vue`: true, `go`: true, `py`: true,
	`java`: true, `kt`: true, `c`: true, `h`: true, `cpp`: true, `hpp`: true, `cs`: true,
	`rb`: true, `php`: true, `rs`: true, `swift`: true, `sh`: true, `proto`: true, `diff`: true,
}

// PipelineTaskDocumentModel 是一个产出文档的业务模型。
type PipelineTaskDocumentModel struct {
	DocumentId      int64
	DocKey          string
	RelPath         string
	Title           string
	FileExt         string
	MediaType       string
	ContentMode     string
	FileSize        int64
	StorageState    string
	StorageError    string
	Revision        int64
	ClientUpdatedAt int64
	UpdatedAt       int64
	SourceAbsPath   string
	UploaderName    string
	Content         string
}

// PipelineTaskDocumentsModel 是一个任务的产出文档清单；目录树由前端按 rel_path 分段构建。
// WorkspaceId/WorkItemId/WorkItemType 让文档整页能自己打开所属工作项详情，不必再查一次任务。
type PipelineTaskDocumentsModel struct {
	TaskId       int64
	TaskTitle    string
	WorkspaceId  int
	WorkItemId   int64
	WorkItemType string
	Documents    []PipelineTaskDocumentModel
}

// StagedDocumentFile 是上报文件落盘后的暂存结果，Path 由 worker 消费后删除。
type StagedDocumentFile struct {
	Path   string
	Size   int64
	Sha256 string
}

// SavePipelineTaskDocumentModel 是客户端上报一个产出文档的业务输入。
// Staged 必须是已经落盘的暂存文件：函数返回后它的所有权转移给队列，除「入队成功」外都会被清掉。
type SavePipelineTaskDocumentModel struct {
	AdminId         int
	UserId          int
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
	Staged          StagedDocumentFile
}

// SavePipelineTaskDocumentResult 是上报结果，Result 取值域见 define.PipelineDocument*。
type SavePipelineTaskDocumentResult struct {
	DocumentId   int64
	DocKey       string
	Revision     int64
	StorageState string
	Result       string
}

// StageDocumentFile 把上报文件写入暂存盘并算出真实大小与 sha256。
// 云端不信任客户端声明的 file_size / content_sha256，落盘结果才是入库依据。
func StageDocumentFile(reader io.Reader, maxBytes int64) (StagedDocumentFile, error) {
	dir := filepath.Join(define.AppRoot, documentPendingDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return StagedDocumentFile{}, err
	}
	// 暂存盘水位：留不住新文件时宁可让客户端退避，也不要把业务主盘的剩余空间吃干净。
	if free, ok := stagedDiskFree(dir); ok && free < documentStagingLowWatermark {
		logs.Warning(`staged document dir is running out of space: dir=%s, free=%d`, dir, free)
		return StagedDocumentFile{}, ErrPipelineDocumentBusy
	}
	path := filepath.Join(dir, tool.Random(24)+`.bin`)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return StagedDocumentFile{}, err
	}
	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hasher), io.LimitReader(reader, maxBytes+1))
	syncErr := file.Sync()
	closeErr := file.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(path)
		for _, cause := range []error{copyErr, syncErr, closeErr} {
			if cause != nil {
				return StagedDocumentFile{}, cause
			}
		}
	}
	if written > maxBytes {
		_ = os.Remove(path)
		return StagedDocumentFile{}, ErrPipelineDocumentTooLarge
	}
	return StagedDocumentFile{Path: path, Size: written, Sha256: hex.EncodeToString(hasher.Sum(nil))}, nil
}

// RemoveStagedDocumentFile 删除暂存文件，供上报失败或任务删除后清理。
func RemoveStagedDocumentFile(path string) {
	if strings.TrimSpace(path) == `` {
		return
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		logs.Error(`remove staged document failed: path=%s, err=%s`, path, err.Error())
	}
}

// PurgeStagedDocumentFiles 清理超过 24 小时仍留在暂存目录的文件（进程崩溃后无人消费的残留）。
// 队列行仍引用的暂存文件一律保留，否则会把还在等重试的作业删成「暂存文件丢失」。
func PurgeStagedDocumentFiles() {
	dir := filepath.Join(define.AppRoot, documentPendingDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logs.Error(`read staged document dir failed: dir=%s, err=%s`, dir, err.Error())
		}
		return
	}
	deadline := time.Now().Add(-24 * time.Hour)
	queued, queueKnown := stagedDocumentQueue()
	if !queueKnown {
		// 队列读不出来时宁可什么都不删：误删会让还在重试的作业直接变成「暂存文件丢失」。
		return
	}
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil {
			logs.Error(`stat staged document failed: name=%s, err=%s`, entry.Name(), statErr.Error())
			continue
		}
		if info.ModTime().After(deadline) {
			continue
		}
		full := filepath.Join(dir, entry.Name())
		if _, ok := queued[filepath.Clean(full)]; ok {
			continue
		}
		if removeErr := os.Remove(full); removeErr != nil {
			logs.Error(`purge staged document failed: name=%s, err=%s`, entry.Name(), removeErr.Error())
			continue
		}
		removed++
	}
	if removed > 0 {
		logs.Info(`purge staged documents: removed=%d`, removed)
	}
}

// stagedDocumentQueue 返回上传队列引用到的暂存文件集合；第二个返回值表示查询是否成功。
func stagedDocumentQueue() (map[string]struct{}, bool) {
	rows, err := msql.RawValues(define.Postgres,
		`SELECT temp_path FROM pipeline_document_upload_jobs`, nil)
	if err != nil {
		logs.Error(`query staged document queue failed: %s`, err.Error())
		return nil, false
	}
	queued := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		queued[filepath.Clean(cast.ToString(row[`temp_path`]))] = struct{}{}
	}
	return queued, true
}

// DocumentContentMode 按扩展名决定正文形态：markdown 渲染、纯文本或只能靠链接打开。
func DocumentContentMode(ext string) string {
	ext = normalizeDocumentExt(ext)
	if documentMarkdownExts[ext] {
		return define.PipelineDocumentContentMarkdown
	}
	if documentTextExts[ext] {
		return define.PipelineDocumentContentText
	}
	return define.PipelineDocumentContentFile
}

// SavePipelineTaskDocument 接收客户端上报的一个产出文档：校验、幂等判断、入队，全部在一个短事务内。
// 除成功入队外，暂存文件在所有失败分支都会被删除。
func SavePipelineTaskDocument(model SavePipelineTaskDocumentModel) (SavePipelineTaskDocumentResult, error) {
	relPath, err := normalizeDocumentRelPath(model.RelPath)
	if err != nil {
		RemoveStagedDocumentFile(model.Staged.Path)
		return SavePipelineTaskDocumentResult{}, err
	}
	docKey := strings.ToLower(strings.TrimSpace(model.DocKey))
	if !isHexOfLength(docKey, 32) {
		RemoveStagedDocumentFile(model.Staged.Path)
		return SavePipelineTaskDocumentResult{}, ErrPipelineDocumentTypeInvalid
	}
	ext := normalizeDocumentExt(model.FileExt)
	relExt := normalizeDocumentExt(filepath.Ext(relPath))
	if ext == `` {
		ext = relExt
	}
	// 黑名单要按「客户端声明」和「真实路径后缀」同时判：只判声明的话，改个 file_ext 就能把
	// 可执行文件或压缩包传上来。
	if documentBlockedExts[ext] || documentBlockedExts[relExt] {
		RemoveStagedDocumentFile(model.Staged.Path)
		return SavePipelineTaskDocumentResult{}, ErrPipelineDocumentTypeInvalid
	}
	title := strings.TrimSpace(model.Title)
	if title == `` {
		title = strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath))
	}
	if len([]rune(title)) > documentMaxTitle {
		title = string([]rune(title)[:documentMaxTitle])
	}
	sourceAbsPath := strings.TrimSpace(model.SourceAbsPath)
	if len(sourceAbsPath) > documentMaxSourcePath {
		sourceAbsPath = sourceAbsPath[:documentMaxSourcePath]
	}
	// 落盘结果覆盖客户端声明的大小；内容 hash 必须一致，否则说明传输被截断或改过。
	if model.Staged.Size > documentMaxFileSize {
		RemoveStagedDocumentFile(model.Staged.Path)
		return SavePipelineTaskDocumentResult{}, ErrPipelineDocumentTooLarge
	}
	if declared := strings.ToLower(strings.TrimSpace(model.ContentSha256)); declared != `` && declared != model.Staged.Sha256 {
		RemoveStagedDocumentFile(model.Staged.Path)
		return SavePipelineTaskDocumentResult{}, ErrPipelineDocumentHashMismatch
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		RemoveStagedDocumentFile(model.Staged.Path)
		return SavePipelineTaskDocumentResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	taskRows, err := msql.RawValues(define.Postgres, `SELECT id, workspace_id, work_item_type, work_item_id
		FROM pipeline_tasks WHERE admin_id=$1 AND user_id=$2 AND local_task_uuid=$3 AND execution_index=$4`,
		tx, model.AdminId, model.UserId, model.LocalTaskUUID, model.ExecutionIndex)
	if err != nil {
		RemoveStagedDocumentFile(model.Staged.Path)
		return SavePipelineTaskDocumentResult{}, err
	}
	if len(taskRows) == 0 {
		RemoveStagedDocumentFile(model.Staged.Path)
		return SavePipelineTaskDocumentResult{}, ErrPipelineTaskNotFound
	}
	taskId := cast.ToInt64(taskRows[0][`id`])
	now := tool.Time2Int()

	existing, err := msql.RawValues(define.Postgres, `SELECT id, revision, content_sha256, client_updated_at, storage_state
		FROM pipeline_task_documents WHERE admin_id=$1 AND pipeline_task_id=$2 AND doc_key=$3 FOR UPDATE NOWAIT`,
		tx, model.AdminId, taskId, docKey)
	if err != nil {
		RemoveStagedDocumentFile(model.Staged.Path)
		if isDocumentLockBusy(err) {
			return SavePipelineTaskDocumentResult{}, ErrPipelineDocumentLocked
		}
		return SavePipelineTaskDocumentResult{}, err
	}
	if len(existing) > 0 {
		return finishExistingDocumentTx(tx, model, existing[0], taskId, docKey, ext, relPath, title, sourceAbsPath, now)
	}

	rows, err := msql.RawValues(define.Postgres, `INSERT INTO pipeline_task_documents(
		admin_id,workspace_id,work_item_type,work_item_id,pipeline_task_id,execution_index,doc_key,rel_path,title,
		file_ext,mime_type,file_size,content_sha256,storage_state,revision,client_updated_at,source_abs_path,
		uploader_id,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'pending',1,$14,$15,$16,$17,$17)
		ON CONFLICT(admin_id, pipeline_task_id, doc_key) DO NOTHING
		RETURNING id`, tx,
		model.AdminId, cast.ToInt(taskRows[0][`workspace_id`]), taskRows[0][`work_item_type`],
		cast.ToInt(taskRows[0][`work_item_id`]), taskId, model.ExecutionIndex, docKey, relPath, title,
		ext, documentMediaType(ext), model.Staged.Size, model.Staged.Sha256, model.ClientUpdatedAt,
		sourceAbsPath, model.UserId, now)
	if err != nil {
		RemoveStagedDocumentFile(model.Staged.Path)
		return SavePipelineTaskDocumentResult{}, err
	}
	if len(rows) == 0 {
		// 并发上报同一新文档：另一请求已建行，回到行锁分支重新判断，不再尝试插入。
		raced, queryErr := msql.RawValues(define.Postgres, `SELECT id, revision, content_sha256, client_updated_at, storage_state
			FROM pipeline_task_documents WHERE admin_id=$1 AND pipeline_task_id=$2 AND doc_key=$3 FOR UPDATE`,
			tx, model.AdminId, taskId, docKey)
		if queryErr != nil {
			RemoveStagedDocumentFile(model.Staged.Path)
			return SavePipelineTaskDocumentResult{}, queryErr
		}
		if len(raced) == 0 {
			RemoveStagedDocumentFile(model.Staged.Path)
			return SavePipelineTaskDocumentResult{}, ErrPipelineDocumentNotFound
		}
		return finishExistingDocumentTx(tx, model, raced[0], taskId, docKey, ext, relPath, title, sourceAbsPath, now)
	}
	documentId := cast.ToInt64(rows[0][`id`])
	if err = enqueueDocumentUploadTx(tx, documentId, model.AdminId, 1, model.Staged.Path, now); err != nil {
		RemoveStagedDocumentFile(model.Staged.Path)
		return SavePipelineTaskDocumentResult{}, err
	}
	if err = tx.Commit(); err != nil {
		RemoveStagedDocumentFile(model.Staged.Path)
		return SavePipelineTaskDocumentResult{}, err
	}
	return SavePipelineTaskDocumentResult{
		DocumentId: documentId, DocKey: docKey, Revision: 1,
		StorageState: define.PipelineDocumentStatePending, Result: define.PipelineDocumentAccepted,
	}, nil
}

// finishExistingDocumentTx 处理同一 doc_key 的重复上报：内容相同或旧包直接判重复，否则更新元数据并入队。
func finishExistingDocumentTx(tx *sql.Tx, model SavePipelineTaskDocumentModel, row msql.Params, taskId int64,
	docKey, ext, relPath, title, sourceAbsPath string, now int) (SavePipelineTaskDocumentResult, error) {
	documentId := cast.ToInt64(row[`id`])
	revision := cast.ToInt64(row[`revision`])
	state := cast.ToString(row[`storage_state`])
	sameContent := cast.ToString(row[`content_sha256`]) == model.Staged.Sha256
	olderPackage := model.ClientUpdatedAt <= cast.ToInt64(row[`client_updated_at`])
	// 内容相同（客户端去抖失败重报）或旧包乱序到达都不改动已存的内容。
	if sameContent || olderPackage {
		if sameContent && state != define.PipelineDocumentStateReady {
			// 内容没变但云端没有可读对象（上次上传失败，或队列行已丢）：重新入队。
			// 客户端「失败后重新上报」发的就是同一份内容，不重排的话这个文档永远修不好。
			requeued, err := requeueUnstoredDocumentTx(tx, model, documentId, revision, state, sourceAbsPath, now)
			if err != nil {
				RemoveStagedDocumentFile(model.Staged.Path)
				return SavePipelineTaskDocumentResult{}, err
			}
			if requeued {
				if err = tx.Commit(); err != nil {
					RemoveStagedDocumentFile(model.Staged.Path)
					return SavePipelineTaskDocumentResult{}, err
				}
				return SavePipelineTaskDocumentResult{
					DocumentId: documentId, DocKey: docKey, Revision: revision,
					StorageState: define.PipelineDocumentStatePending, Result: define.PipelineDocumentDuplicate,
				}, nil
			}
		}
		RemoveStagedDocumentFile(model.Staged.Path)
		if err := tx.Commit(); err != nil {
			return SavePipelineTaskDocumentResult{}, err
		}
		return SavePipelineTaskDocumentResult{
			DocumentId: documentId, DocKey: docKey, Revision: revision, StorageState: state,
			Result: define.PipelineDocumentDuplicate,
		}, nil
	}
	revision++
	// stored_path 保持不变：对象 key 与 revision 无关，worker 覆盖写同一对象，读侧不会读到半写文件。
	if _, err := msql.RawExec(define.Postgres, `UPDATE pipeline_task_documents SET
		rel_path=$1,title=$2,file_ext=$3,mime_type=$4,file_size=$5,content_sha256=$6,
		storage_state='pending',storage_error='',revision=$7,client_updated_at=$8,
		source_abs_path=$9,uploader_id=$10,execution_index=$11,deleted_at=0,updated_at=$12
		WHERE admin_id=$13 AND id=$14`, tx,
		relPath, title, ext, documentMediaType(ext), model.Staged.Size, model.Staged.Sha256,
		revision, model.ClientUpdatedAt, sourceAbsPath, model.UserId, model.ExecutionIndex, now,
		model.AdminId, documentId); err != nil {
		RemoveStagedDocumentFile(model.Staged.Path)
		return SavePipelineTaskDocumentResult{}, err
	}
	if err := enqueueDocumentUploadTx(tx, documentId, model.AdminId, revision, model.Staged.Path, now); err != nil {
		RemoveStagedDocumentFile(model.Staged.Path)
		return SavePipelineTaskDocumentResult{}, err
	}
	if err := tx.Commit(); err != nil {
		RemoveStagedDocumentFile(model.Staged.Path)
		return SavePipelineTaskDocumentResult{}, err
	}
	return SavePipelineTaskDocumentResult{
		DocumentId: documentId, DocKey: docKey, Revision: revision,
		StorageState: define.PipelineDocumentStatePending, Result: define.PipelineDocumentReplaced,
	}, nil
}

// requeueUnstoredDocumentTx 在内容未变、但云端没有可读对象时，用本次暂存文件重新排队同一版本。
// 返回 false 表示队列里已经有一个健康的待上传作业，无需重排（此时按 duplicate 处理）。
func requeueUnstoredDocumentTx(tx *sql.Tx, model SavePipelineTaskDocumentModel, documentId, revision int64,
	state, sourceAbsPath string, now int) (bool, error) {
	if state != define.PipelineDocumentStateFailed {
		// 待上传且队列里还有作业：worker 会继续处理，不必重新入队。
		jobRows, err := msql.RawValues(define.Postgres, `SELECT id FROM pipeline_document_upload_jobs
			WHERE admin_id=$1 AND document_id=$2`, tx, model.AdminId, documentId)
		if err != nil {
			return false, err
		}
		if len(jobRows) > 0 {
			return false, nil
		}
	}
	if _, err := msql.RawExec(define.Postgres, `UPDATE pipeline_task_documents SET
		storage_state='pending',storage_error='',uploader_id=$1,execution_index=$2,
		source_abs_path=$3,deleted_at=0,updated_at=$4
		WHERE admin_id=$5 AND id=$6`, tx,
		model.UserId, model.ExecutionIndex, sourceAbsPath, now,
		model.AdminId, documentId); err != nil {
		return false, err
	}
	// 重新入队会删掉该文档的旧作业（含已判定失败、等待定时重试的那条）。
	if err := enqueueDocumentUploadTx(tx, documentId, model.AdminId, revision, model.Staged.Path, now); err != nil {
		return false, err
	}
	return true, nil
}

// isDocumentLockBusy 识别 FOR UPDATE NOWAIT 抢锁失败（PostgreSQL SQLSTATE 55P03）。
// 同一文档被并发上报时用它直接回 429，而不是让请求排在行锁上把连接池占满。
func isDocumentLockBusy(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return string(pqErr.Code) == `55P03`
	}
	return false
}

// enqueueDocumentUploadTx 把暂存文件交进队列；同一文档的旧作业在这里一并作废，避免旧内容后写覆盖新内容。
// 旧作业的暂存文件不在这里删除：事务可能回滚，交给 24 小时清理兜底。
func enqueueDocumentUploadTx(tx *sql.Tx, documentId int64, adminID int, revision int64, tempPath string, now int) error {
	if _, err := msql.RawExec(define.Postgres,
		`DELETE FROM pipeline_document_upload_jobs WHERE admin_id=$1 AND document_id=$2`, tx, adminID, documentId); err != nil {
		return err
	}
	_, err := msql.RawExec(define.Postgres, `INSERT INTO pipeline_document_upload_jobs(
		document_id,admin_id,revision,temp_path,created_at) VALUES($1,$2,$3,$4,$5)`,
		tx, documentId, adminID, revision, tempPath, now)
	return err
}

// PipelineTaskDocumentIndexes 返回渲染联动用的轻量文档索引（任务详情随附），只含替换链接所需字段。
func PipelineTaskDocumentIndexes(adminId int, taskId int64) ([]define.PipelineTaskDocumentIndex, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id,doc_key,title,rel_path,source_abs_path
		FROM pipeline_task_documents WHERE admin_id=$1 AND pipeline_task_id=$2 AND deleted_at=0
		ORDER BY rel_path ASC, id ASC`, nil, adminId, taskId)
	if err != nil {
		return nil, err
	}
	indexes := make([]define.PipelineTaskDocumentIndex, 0, len(rows))
	for _, row := range rows {
		indexes = append(indexes, define.PipelineTaskDocumentIndex{
			DocumentId:    cast.ToInt64(row[`id`]),
			DocKey:        cast.ToString(row[`doc_key`]),
			Title:         cast.ToString(row[`title`]),
			RelPath:       cast.ToString(row[`rel_path`]),
			SourceAbsPath: cast.ToString(row[`source_abs_path`]),
		})
	}
	return indexes, nil
}

// PipelineTaskWorkspaceId 返回任务所属工作空间；产出文档接口按 taskId 寻址，路径里没有
// wsId，权限中间件要靠它解析工作项查看权限。
func PipelineTaskWorkspaceId(adminId int, taskId int64) (int, error) {
	rows, err := msql.RawValues(define.Postgres,
		`SELECT workspace_id FROM pipeline_tasks WHERE admin_id=$1 AND id=$2`, nil, adminId, taskId)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, ErrPipelineTaskNotFound
	}
	return cast.ToInt(rows[0][`workspace_id`]), nil
}

// PipelineDocumentWorkspaceId 按文档反查其任务所属工作空间，供文档详情与签名链接接口鉴权。
func PipelineDocumentWorkspaceId(adminId int, documentId int64) (int, error) {
	rows, err := msql.RawValues(define.Postgres,
		`SELECT t.workspace_id FROM pipeline_task_documents d
		JOIN pipeline_tasks t ON t.admin_id=d.admin_id AND t.id=d.pipeline_task_id
		WHERE d.admin_id=$1 AND d.id=$2 AND d.deleted_at=0`, nil, adminId, documentId)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, ErrPipelineDocumentNotFound
	}
	return cast.ToInt(rows[0][`workspace_id`]), nil
}

// CanViewPipelineDocuments 判定用户能否查看某个工作空间内任务的产出文档。
//
// 主口径与 Agent tab 的沟通记录一致：工作项查看权限（requirement:view / defect:view），
// 不再要求流水线菜单权限。pipeline:view 作为兜底继续放行，避免收窄既有能力——只有企业级
// pipeline:view、却不是该工作空间成员的成员，在 Agent 员工页看日志详情时原本就能看这些文档。
func CanViewPipelineDocuments(adminId, userId, workspaceId int) bool {
	wsId := cast.ToString(workspaceId)
	if HasPermission(adminId, userId, wsId, `requirement:view`) ||
		HasPermission(adminId, userId, wsId, `defect:view`) {
		return true
	}
	return HasPermission(adminId, userId, ``, `pipeline:view`)
}

// ListPipelineTaskDocuments 返回一个任务的产出文档清单（扁平，带 rel_path）。
func ListPipelineTaskDocuments(adminId int, taskId int64) (PipelineTaskDocumentsModel, error) {
	taskRow, err := msql.RawValues(define.Postgres,
		`SELECT title_snapshot, workspace_id, work_item_id, work_item_type FROM pipeline_tasks WHERE admin_id=$1 AND id=$2`, nil, adminId, taskId)
	if err != nil {
		return PipelineTaskDocumentsModel{}, err
	}
	if len(taskRow) == 0 {
		return PipelineTaskDocumentsModel{}, ErrPipelineTaskNotFound
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT d.id,d.doc_key,d.rel_path,d.title,d.file_ext,d.mime_type,d.file_size,
		d.storage_state,d.storage_error,d.revision,d.client_updated_at,d.updated_at,d.source_abs_path,
		COALESCE(NULLIF(u.name,''),u.username,'') AS uploader_name
		FROM pipeline_task_documents d
		LEFT JOIN users u ON u.admin_id=d.admin_id AND u.id=d.uploader_id
		WHERE d.admin_id=$1 AND d.pipeline_task_id=$2 AND d.deleted_at=0
		ORDER BY d.rel_path ASC, d.id ASC`, nil, adminId, taskId)
	if err != nil {
		return PipelineTaskDocumentsModel{}, err
	}
	documents := make([]PipelineTaskDocumentModel, 0, len(rows))
	for _, row := range rows {
		documents = append(documents, toPipelineTaskDocumentModel(row))
	}
	return PipelineTaskDocumentsModel{
		TaskId:       taskId,
		TaskTitle:    cast.ToString(taskRow[0][`title_snapshot`]),
		WorkspaceId:  cast.ToInt(taskRow[0][`workspace_id`]),
		WorkItemId:   cast.ToInt64(taskRow[0][`work_item_id`]),
		WorkItemType: cast.ToString(taskRow[0][`work_item_type`]),
		Documents:    documents,
	}, nil
}

// isOwnedDocumentObject 判断文档对象是否落在本租户的对象命名空间内。
// 当前布局是 task-docs/{adminID}/...；分支上早先写入的对象在 pipeline/{adminID}/task-docs/...，
// 两种都要认，否则历史文档取不回正文、签名链接也会被当成越权而返回不存在。
func isOwnedDocumentObject(adminId int, stored string) bool {
	return lib_storage.M.IsOwned(adminId, lib_storage.BusinessTaskDocs, stored) ||
		lib_storage.M.IsOwned(adminId, lib_storage.BusinessPipeline, stored)
}

// GetPipelineTaskDocument 返回单个文档的元数据；文本类会把正文一起取回来，其余类型由前端取签名链接。
func GetPipelineTaskDocument(adminId int, documentId int64) (PipelineTaskDocumentModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT d.id,d.doc_key,d.rel_path,d.title,d.file_ext,d.mime_type,d.file_size,
		d.storage_state,d.storage_error,d.revision,d.client_updated_at,d.updated_at,d.source_abs_path,d.stored_path,
		COALESCE(NULLIF(u.name,''),u.username,'') AS uploader_name
		FROM pipeline_task_documents d
		LEFT JOIN users u ON u.admin_id=d.admin_id AND u.id=d.uploader_id
		WHERE d.admin_id=$1 AND d.id=$2 AND d.deleted_at=0`, nil, adminId, documentId)
	if err != nil {
		return PipelineTaskDocumentModel{}, err
	}
	if len(rows) == 0 {
		return PipelineTaskDocumentModel{}, ErrPipelineDocumentNotFound
	}
	document := toPipelineTaskDocumentModel(rows[0])
	stored := cast.ToString(rows[0][`stored_path`])
	if document.StorageState != define.PipelineDocumentStateReady || stored == `` {
		return document, nil
	}
	if document.ContentMode == define.PipelineDocumentContentFile || document.FileSize > documentInlineLimit {
		// 超过内联上限的文本类降级成 file：前端只有拿到 file 形态才会回落到下载，
		// 否则会把「没有正文」的文档渲染成一片空白。
		document.ContentMode = define.PipelineDocumentContentFile
		return document, nil
	}
	if !isOwnedDocumentObject(adminId, stored) {
		logs.Error(`document stored path is not owned: admin=%d, document=%d, stored=%s`, adminId, documentId, stored)
		return document, nil
	}
	content, err := lib_storage.M.Read(stored)
	if err != nil {
		logs.Error(`read document object failed: admin=%d, document=%d, err=%s`, adminId, documentId, err.Error())
		return PipelineTaskDocumentModel{}, err
	}
	document.Content = string(content)
	return document, nil
}

// GetPipelineTaskDocumentLink 返回文档的 2 小时签名链接；每次调用重新签，避免前端缓存到过期链接。
func GetPipelineTaskDocumentLink(adminId int, documentId int64, disposition string) (string, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT stored_path,storage_state,title,file_ext
		FROM pipeline_task_documents WHERE admin_id=$1 AND id=$2 AND deleted_at=0`, nil, adminId, documentId)
	if err != nil {
		return ``, err
	}
	if len(rows) == 0 {
		return ``, ErrPipelineDocumentNotFound
	}
	stored := cast.ToString(rows[0][`stored_path`])
	if cast.ToString(rows[0][`storage_state`]) != define.PipelineDocumentStateReady || stored == `` {
		return ``, ErrPipelineDocumentNotReady
	}
	if !isOwnedDocumentObject(adminId, stored) {
		return ``, ErrPipelineDocumentNotFound
	}
	params := url.Values{}
	if disposition == `attachment` {
		// 对象存储支持用签名参数覆盖响应头；文件名按 ASCII 兜底 + UTF-8 双写，浏览器才认中文名。
		name := cast.ToString(rows[0][`title`])
		ext := normalizeDocumentExt(cast.ToString(rows[0][`file_ext`]))
		if name == `` {
			name = cast.ToString(documentId)
		}
		if ext != `` && !strings.HasSuffix(strings.ToLower(name), `.`+ext) {
			name += `.` + ext
		}
		params.Set(`response-content-disposition`, fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
			asciiFallbackName(name, ext), url.PathEscape(name)))
	}
	return lib_storage.SignedURLWithTTL(stored, lib_storage.DocumentSignedURLTTL, params), nil
}

// deletedDocumentObject 是随任务软删、等待回收的对象存储条目。
type deletedDocumentObject struct {
	DocumentId int64
	StoredPath string
}

// softDeleteTaskDocumentsTx 软删任务下的产出文档并清空其上传队列，
// 返回需要删除的暂存文件路径，以及需要异步回收的对象存储条目。
func softDeleteTaskDocumentsTx(tx *sql.Tx, adminID int, taskIDs []int64) ([]string, []deletedDocumentObject, error) {
	if len(taskIDs) == 0 {
		return nil, nil, nil
	}
	docRows, err := msql.RawValues(define.Postgres, `SELECT id,stored_path FROM pipeline_task_documents
		WHERE admin_id=$1 AND pipeline_task_id = ANY($2)`, tx, adminID, pq.Array(taskIDs))
	if err != nil {
		return nil, nil, err
	}
	documentIDs := make([]int64, 0, len(docRows))
	objects := make([]deletedDocumentObject, 0, len(docRows))
	for _, row := range docRows {
		documentIDs = append(documentIDs, cast.ToInt64(row[`id`]))
		if stored := cast.ToString(row[`stored_path`]); stored != `` {
			objects = append(objects, deletedDocumentObject{
				DocumentId: cast.ToInt64(row[`id`]),
				StoredPath: stored,
			})
		}
	}
	if len(documentIDs) == 0 {
		return nil, nil, nil
	}
	jobRows, err := msql.RawValues(define.Postgres, `SELECT temp_path FROM pipeline_document_upload_jobs
		WHERE admin_id=$1 AND document_id = ANY($2)`, tx, adminID, pq.Array(documentIDs))
	if err != nil {
		return nil, nil, err
	}
	tempPaths := make([]string, 0, len(jobRows))
	for _, row := range jobRows {
		tempPaths = append(tempPaths, cast.ToString(row[`temp_path`]))
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM pipeline_document_upload_jobs
		WHERE admin_id=$1 AND document_id = ANY($2)`, tx, adminID, pq.Array(documentIDs)); err != nil {
		return nil, nil, err
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE pipeline_task_documents SET deleted_at=$1,updated_at=$1
		WHERE admin_id=$2 AND id = ANY($3) AND deleted_at=0`, tx, tool.Time2Int(), adminID, pq.Array(documentIDs)); err != nil {
		return nil, nil, err
	}
	return tempPaths, objects, nil
}

// DocumentStorageUsed 返回该租户产出文档当前占用的字节数（软删行不计）。
func DocumentStorageUsed(adminID int) (int64, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT COALESCE(SUM(file_size),0) AS total
		FROM pipeline_task_documents WHERE admin_id=$1 AND deleted_at=0`, nil, adminID)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return cast.ToInt64(rows[0][`total`]), nil
}

// CheckPipelineDocumentQuota 把产出文档占用一并计入企业存储用量后再判断配额。
// enterprise_files 台账里没有产出文档的行，所以只调 CheckStorageQuota 会让文档占用永远不进用量。
func CheckPipelineDocumentQuota(adminID int, addBytes int64) error {
	plan, err := GetPlan(adminID)
	if err != nil {
		return err
	}
	if plan.StorageLimit <= 0 {
		return nil
	}
	limitBytes := int64(plan.StorageLimit) * 1024 * 1024 * 1024
	usedBytes, err := GetEnterpriseStorageUsed(adminID)
	if err != nil {
		return err
	}
	documentBytes, err := DocumentStorageUsed(adminID)
	if err != nil {
		return err
	}
	total := usedBytes + documentBytes
	if total+addBytes > limitBytes {
		return &QuotaExceededError{
			Key:   define.QuotaStorage,
			Scope: `total`,
			Used:  int(total),
			Limit: plan.StorageLimit,
		}
	}
	return nil
}

// PurgeDeletedDocumentObjects 补偿清理：把「已软删但仍指向对象」的行扫出来回收对象，
// 覆盖进程崩溃、异步清理失败留下的残局。启动时跑一次，平时由任务删除路径定向清理。
func PurgeDeletedDocumentObjects(limit int) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, stored_path FROM pipeline_task_documents
		WHERE deleted_at > 0 AND stored_path <> '' ORDER BY id LIMIT $1`, nil, limit)
	if err != nil {
		logs.Error(`query deleted document objects failed: %s`, err.Error())
		return
	}
	objects := make([]deletedDocumentObject, 0, len(rows))
	for _, row := range rows {
		objects = append(objects, deletedDocumentObject{
			DocumentId: cast.ToInt64(row[`id`]),
			StoredPath: cast.ToString(row[`stored_path`]),
		})
	}
	purgeDocumentObjects(objects)
}

// purgeDocumentObjects 删除给定对象并清空 stored_path；失败的行留给下次启动补偿。
func purgeDocumentObjects(objects []deletedDocumentObject) {
	removed := 0
	for _, item := range objects {
		if item.StoredPath == `` {
			continue
		}
		if err := lib_storage.M.Delete(item.StoredPath); err != nil {
			logs.Error(`remove deleted document object failed: id=%d, err=%s`, item.DocumentId, err.Error())
			continue
		}
		if _, err := msql.RawExec(define.Postgres,
			`UPDATE pipeline_task_documents SET stored_path='' WHERE id=$1`, nil, item.DocumentId); err != nil {
			logs.Error(`clear deleted document stored path failed: id=%d, err=%s`, item.DocumentId, err.Error())
			continue
		}
		removed++
	}
	if removed > 0 {
		logs.Info(`purge deleted document objects: removed=%d`, removed)
	}
}

// scheduleDocumentObjectPurge 在响应返回后异步回收指定文档的对象，失败由下次启动补做。
func scheduleDocumentObjectPurge(objects []deletedDocumentObject) {
	if len(objects) == 0 {
		return
	}
	go func() {
		defer func() {
			if cause := recover(); cause != nil {
				logs.Error(`purge deleted document objects panic: %v`, cause)
			}
		}()
		purgeDocumentObjects(objects)
	}()
}

// scheduleDeletedDocumentObjectPurge 在请求返回后异步补扫残留的软删对象。
func scheduleDeletedDocumentObjectPurge() {
	go func() {
		defer func() {
			if cause := recover(); cause != nil {
				logs.Error(`purge deleted document objects panic: %v`, cause)
			}
		}()
		PurgeDeletedDocumentObjects(200)
	}()
}

// toPipelineTaskDocumentModel 把查询行转成业务模型，关键数值转换失败直接报错，不静默补零。
func toPipelineTaskDocumentModel(row msql.Params) PipelineTaskDocumentModel {
	fileExt := cast.ToString(row[`file_ext`])
	if fileExt == `` {
		fileExt = normalizeDocumentExt(cast.ToString(row[`rel_path`]))
	}
	return PipelineTaskDocumentModel{
		DocumentId:      cast.ToInt64(row[`id`]),
		DocKey:          cast.ToString(row[`doc_key`]),
		RelPath:         cast.ToString(row[`rel_path`]),
		Title:           cast.ToString(row[`title`]),
		FileExt:         fileExt,
		MediaType:       cast.ToString(row[`mime_type`]),
		ContentMode:     DocumentContentMode(fileExt),
		FileSize:        cast.ToInt64(row[`file_size`]),
		StorageState:    cast.ToString(row[`storage_state`]),
		StorageError:    cast.ToString(row[`storage_error`]),
		Revision:        cast.ToInt64(row[`revision`]),
		ClientUpdatedAt: cast.ToInt64(row[`client_updated_at`]),
		UpdatedAt:       cast.ToInt64(row[`updated_at`]),
		SourceAbsPath:   cast.ToString(row[`source_abs_path`]),
		UploaderName:    cast.ToString(row[`uploader_name`]),
	}
}

// normalizeDocumentRelPath 把客户端给的相对路径规范成正斜杠形式，并挡掉越界路径。
func normalizeDocumentRelPath(raw string) (string, error) {
	relPath := strings.ReplaceAll(strings.TrimSpace(raw), `\`, `/`)
	relPath = strings.Trim(relPath, `/`)
	if relPath == `` || len(relPath) > documentMaxRelPath {
		return ``, ErrPipelineDocumentTypeInvalid
	}
	if strings.ContainsRune(relPath, '\x00') || strings.Contains(relPath, `:`) {
		return ``, ErrPipelineDocumentTypeInvalid
	}
	for _, segment := range strings.Split(relPath, `/`) {
		if segment == `` || segment == `.` || segment == `..` {
			return ``, ErrPipelineDocumentTypeInvalid
		}
	}
	return relPath, nil
}

// normalizeDocumentExt 统一扩展名形态：小写、去掉点、只保留字母数字。
func normalizeDocumentExt(raw string) string {
	ext := strings.ToLower(strings.TrimSpace(raw))
	ext = strings.TrimPrefix(ext, `.`)
	if ext == `` || len(ext) > 20 {
		return ``
	}
	for _, r := range ext {
		isDigit := r >= '0' && r <= '9'
		isLetter := r >= 'a' && r <= 'z'
		if !isDigit && !isLetter {
			return ``
		}
	}
	return ext
}

// documentMediaType 按扩展名给对象存储的内容类型。只信 lib_storage 的白名单：
// html / svg / js 这类会被浏览器内联执行的文件一律按二进制流存，避免在存储域跑脚本。
func documentMediaType(ext string) string {
	return lib_storage.MediaTypeForExt(normalizeDocumentExt(ext))
}

// isHexOfLength 判断字符串是否为指定长度的十六进制串。
func isHexOfLength(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, r := range value {
		isDigit := r >= '0' && r <= '9'
		isLower := r >= 'a' && r <= 'f'
		if !isDigit && !isLower {
			return false
		}
	}
	return true
}

// asciiFallbackName 给不认识 filename* 的客户端一个纯 ASCII 文件名，扩展名保持原样。
func asciiFallbackName(name, ext string) string {
	if ext == `` {
		return `document`
	}
	return `document.` + ext
}

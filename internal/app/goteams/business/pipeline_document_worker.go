// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_storage"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

const (
	// documentUploadWorkerCount 是并行上传的 worker 数，暂存盘写入与对象存储写入都受它约束。
	documentUploadWorkerCount = 3
	// documentUploadInterval 是队列空闲时的轮询间隔。
	documentUploadInterval = time.Second
	// documentUploadLease 是单次 claim 的租约，进程崩溃后到期即可被其他 worker 接管。
	documentUploadLease = 60
	// documentUploadMaxAttempts 是单个作业的连续重试上限，超过后作业挂起等定时重试。
	documentUploadMaxAttempts = 3
	// documentUploadRetryWindow 是失败作业再次被认领的等待时间：作业行保留，到点自动重试。
	documentUploadRetryWindow = 600
)

// documentUploadClaimSQL 是一条语句完成「选出可执行作业 + 加锁 + 续租」，多实例并发也不会重复消费。
const documentUploadClaimSQL = `WITH candidate AS (
	SELECT id FROM pipeline_document_upload_jobs
	WHERE claimed_until < $1
	ORDER BY id ASC FOR UPDATE SKIP LOCKED LIMIT 1
) UPDATE pipeline_document_upload_jobs j
	SET claimed_by=$2, claimed_until=$3
	FROM candidate WHERE j.id=candidate.id
	RETURNING j.id,j.document_id,j.admin_id,j.revision,j.temp_path,j.attempts`

// documentUploadJob 是一次待执行的上传作业。
type documentUploadJob struct {
	ID         int64
	DocumentID int64
	AdminID    int
	Revision   int64
	TempPath   string
	Attempts   int
}

// documentUploadWorkers 管理当前进程的上传 worker 生命周期。
var documentUploadWorkers struct {
	sync.Mutex
	cancel context.CancelFunc
	wait   sync.WaitGroup
}

// StartDocumentUploadWorkers 启动产出文档的异步上传 worker。
func StartDocumentUploadWorkers() {
	documentUploadWorkers.Lock()
	defer documentUploadWorkers.Unlock()
	if documentUploadWorkers.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	documentUploadWorkers.cancel = cancel
	// 启动时先清掉上次进程留下的孤儿暂存文件，再开始消费队列。
	PurgeStagedDocumentFiles()
	// 上次进程退出时没删完的软删对象，这里补一轮（异步，不挡启动）。
	scheduleDeletedDocumentObjectPurge()
	logs.Info("register polling task pipeline_document_upload_jobs, workers:%d, interval:%s", documentUploadWorkerCount, documentUploadInterval)
	for index := 0; index < documentUploadWorkerCount; index++ {
		documentUploadWorkers.wait.Add(1)
		go documentUploadWorkerLoop(ctx, fmt.Sprintf("%s-%d-%d", automationHostname(), os.Getpid(), index))
	}
}

// StopDocumentUploadWorkers 停止认领新作业，并等待当前作业结束。
func StopDocumentUploadWorkers() {
	documentUploadWorkers.Lock()
	cancel := documentUploadWorkers.cancel
	documentUploadWorkers.cancel = nil
	documentUploadWorkers.Unlock()
	if cancel != nil {
		cancel()
		documentUploadWorkers.wait.Wait()
	}
}

// documentUploadWorkerLoop 持续认领并执行上传作业。
func documentUploadWorkerLoop(ctx context.Context, owner string) {
	defer documentUploadWorkers.wait.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		job, err := claimDocumentUploadJob(owner)
		if err != nil {
			logs.Error("document upload claim failed: %s", err.Error())
			automationWorkerSleep(ctx, time.Second)
			continue
		}
		if job == nil {
			automationWorkerSleep(ctx, documentUploadInterval)
			continue
		}
		if err = runDocumentUploadJob(job); err != nil {
			logs.Error("document upload %d failed: %s", job.ID, err.Error())
			failDocumentUploadJob(job, err)
		}
	}
}

// claimDocumentUploadJob 在一个短事务内认领一个可执行作业。
func claimDocumentUploadJob(owner string) (*documentUploadJob, error) {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	now := tool.Time2Int()
	job := &documentUploadJob{}
	err = tx.QueryRow(documentUploadClaimSQL, now, owner, int64(now)+documentUploadLease).Scan(
		&job.ID, &job.DocumentID, &job.AdminID, &job.Revision, &job.TempPath, &job.Attempts,
	)
	if errors.Is(err, sql.ErrNoRows) {
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return job, nil
}

// runDocumentUploadJob 读取暂存文件、写入对象存储并落库；返回错误表示可重试的失败。
func runDocumentUploadJob(job *documentUploadJob) error {
	rows, err := msql.RawValues(define.Postgres, `SELECT pipeline_task_id,doc_key,file_ext,storage_state,deleted_at,revision
		FROM pipeline_task_documents WHERE admin_id=$1 AND id=$2`, nil, job.AdminID, job.DocumentID)
	if err != nil {
		return err
	}
	// 文档已删除、已就绪，或已被更新的 revision 接管：本作业直接作废，不再写对象存储。
	// failed 要放行：那正是等待定时重试的作业。
	if len(rows) == 0 || cast.ToInt64(rows[0][`deleted_at`]) > 0 {
		dropDocumentUploadJob(job)
		return nil
	}
	state := cast.ToString(rows[0][`storage_state`])
	if state != define.PipelineDocumentStatePending && state != define.PipelineDocumentStateFailed {
		dropDocumentUploadJob(job)
		return nil
	}
	if job.Revision > 0 && cast.ToInt64(rows[0][`revision`]) != job.Revision {
		// 队列里已有更新的版本，旧作业的内容会覆盖新对象，必须丢弃。
		dropDocumentUploadJob(job)
		return nil
	}
	content, err := os.ReadFile(job.TempPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// 暂存文件丢失是终态失败：重试也不会变好，直接把文档置为失败让前端提示重新上报。
			markDocumentStorageFailed(job, `staged file missing`)
			return nil
		}
		return err
	}
	key := lib_storage.DocumentObjectKey(job.AdminID, cast.ToInt64(rows[0][`pipeline_task_id`]),
		cast.ToString(rows[0][`doc_key`]), cast.ToString(rows[0][`file_ext`]))
	// 内容类型在上传时重算，不信任入库时的旧值：html / svg 之类必须按二进制流存。
	stored, err := lib_storage.M.UploadRaw(key, content, documentMediaType(cast.ToString(rows[0][`file_ext`])))
	if err != nil {
		return err
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := tool.Time2Int()
	// revision 必须一起判：上传期间若已有更新的上报入队，这次写的是旧内容，不能把文档标成就绪。
	result, err := msql.RawExec(define.Postgres, `UPDATE pipeline_task_documents SET
		stored_path=$1,storage_state='ready',storage_error='',storage_attempts=$2,updated_at=$3
		WHERE admin_id=$4 AND id=$5 AND revision=$6 AND storage_state IN ('pending','failed') AND deleted_at=0`, tx,
		stored, job.Attempts+1, now, job.AdminID, job.DocumentID, job.Revision)
	if err != nil {
		return err
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr == nil && affected == 0 {
		// 期间已有新的 revision 接管：本作业写的对象不能代表当前版本，删作业并让新作业覆盖。
		_ = tx.Rollback()
		dropDocumentUploadJob(job)
		return nil
	}
	if _, err = msql.RawExec(define.Postgres,
		`DELETE FROM pipeline_document_upload_jobs WHERE id=$1`, tx, job.ID); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	RemoveStagedDocumentFile(job.TempPath)
	return nil
}

// dropDocumentUploadJob 作废一个已无意义的作业：删队列行并清掉暂存文件。
func dropDocumentUploadJob(job *documentUploadJob) {
	if _, err := msql.RawExec(define.Postgres,
		`DELETE FROM pipeline_document_upload_jobs WHERE id=$1`, nil, job.ID); err != nil {
		logs.Error(`drop document upload job failed: id=%d, err=%s`, job.ID, err.Error())
		return
	}
	RemoveStagedDocumentFile(job.TempPath)
}

// failDocumentUploadJob 记录失败并按剩余次数决定退避重试还是挂起等定时重试。
func failDocumentUploadJob(job *documentUploadJob, cause error) {
	attempts := job.Attempts + 1
	message := cause.Error()
	if len(message) > 500 {
		message = message[:500]
	}
	now := tool.Time2Int()
	if attempts >= documentUploadMaxAttempts {
		// 连续失败到上限：文档先置为失败让前端提示，作业行保留并挂起一个窗口，
		// 到点会被重新认领（定时重试）；客户端重新上报也会直接覆盖这条作业。
		if _, err := msql.RawExec(define.Postgres, `UPDATE pipeline_task_documents SET
			storage_state='failed',storage_error=$1,storage_attempts=$2,updated_at=$3
			WHERE admin_id=$4 AND id=$5`, nil, message, documentUploadMaxAttempts, now, job.AdminID, job.DocumentID); err != nil {
			logs.Error(`mark document storage failed: id=%d, err=%s`, job.DocumentID, err.Error())
		}
		if _, err := msql.RawExec(define.Postgres, `UPDATE pipeline_document_upload_jobs SET
			attempts=$1,last_error=$2,claimed_by='',claimed_until=$3 WHERE id=$4`, nil,
			documentUploadMaxAttempts, message, now+documentUploadRetryWindow, job.ID); err != nil {
			logs.Error(`park document upload job: id=%d, err=%s`, job.ID, err.Error())
		}
		return
	}
	// 退避 10s、60s：租约到期前不会被其他 worker 抢到，同时保留重试机会。
	backoff := []int64{10, 60, 300}
	delay := backoff[len(backoff)-1]
	if attempts-1 < len(backoff) {
		delay = backoff[attempts-1]
	}
	if _, err := msql.RawExec(define.Postgres, `UPDATE pipeline_document_upload_jobs SET
		attempts=$1,last_error=$2,claimed_by='',claimed_until=$3 WHERE id=$4`, nil,
		attempts, message, now+int(delay), job.ID); err != nil {
		logs.Error(`reschedule document upload job: id=%d, err=%s`, job.ID, err.Error())
	}
	if _, err := msql.RawExec(define.Postgres, `UPDATE pipeline_task_documents SET
		storage_attempts=$1,storage_error=$2,updated_at=$3 WHERE admin_id=$4 AND id=$5`, nil,
		attempts, message, now, job.AdminID, job.DocumentID); err != nil {
		logs.Error(`update document storage attempts: id=%d, err=%s`, job.DocumentID, err.Error())
	}
}

// markDocumentStorageFailed 把文档直接置为失败并清掉作业与暂存文件。
func markDocumentStorageFailed(job *documentUploadJob, message string) {
	now := tool.Time2Int()
	if _, err := msql.RawExec(define.Postgres, `UPDATE pipeline_task_documents SET
		storage_state='failed',storage_error=$1,storage_attempts=$2,updated_at=$3
		WHERE admin_id=$4 AND id=$5`, nil, message, job.Attempts+1, now, job.AdminID, job.DocumentID); err != nil {
		logs.Error(`mark document storage failed: id=%d, err=%s`, job.DocumentID, err.Error())
		return
	}
	dropDocumentUploadJob(job)
}

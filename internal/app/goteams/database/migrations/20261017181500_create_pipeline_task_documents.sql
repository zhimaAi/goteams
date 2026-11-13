-- +goose Up
-- +goose StatementBegin

-- 产出文档上报：PG 只存任务关联与上传元数据，文件本体在 OSS（未配置时落本地）。
-- 同一 (admin_id, pipeline_task_id, doc_key) 只有一行；内容变化只递增 revision，
-- 对象 key 不随 revision 变化，上传采用覆盖写，因此读侧永远只会读到某个完整版本。
CREATE TABLE IF NOT EXISTS pipeline_task_documents (
    id                BIGSERIAL PRIMARY KEY,
    admin_id          INTEGER NOT NULL,
    workspace_id      INTEGER NOT NULL DEFAULT 0,
    work_item_type    VARCHAR(20) NOT NULL DEFAULT '',
    work_item_id      INTEGER NOT NULL DEFAULT 0,
    pipeline_task_id  BIGINT  NOT NULL,
    execution_index   INTEGER NOT NULL DEFAULT 0,
    doc_key           VARCHAR(64)  NOT NULL,
    rel_path          VARCHAR(512) NOT NULL,
    title             VARCHAR(255) NOT NULL DEFAULT '',
    file_ext          VARCHAR(20)  NOT NULL DEFAULT '',
    mime_type         VARCHAR(120) NOT NULL DEFAULT '',
    file_size         BIGINT  NOT NULL DEFAULT 0,
    content_sha256    VARCHAR(64)  NOT NULL DEFAULT '',
    stored_path       TEXT NOT NULL DEFAULT '',
    storage_state     VARCHAR(20) NOT NULL DEFAULT 'pending',
    storage_error     VARCHAR(500) NOT NULL DEFAULT '',
    storage_attempts  INTEGER NOT NULL DEFAULT 0,
    revision          BIGINT NOT NULL DEFAULT 1,
    client_updated_at BIGINT NOT NULL DEFAULT 0,
    source_abs_path   VARCHAR(1024) NOT NULL DEFAULT '',
    uploader_id       INTEGER NOT NULL DEFAULT 0,
    deleted_at        BIGINT NOT NULL DEFAULT 0,
    created_at        BIGINT NOT NULL DEFAULT 0,
    updated_at        BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE pipeline_task_documents IS '流水线任务产出文档：元数据在 PG，文件本体在对象存储';
COMMENT ON COLUMN pipeline_task_documents.doc_key IS '客户端按目录相对路径算出的稳定身份（sha1 前 32 位），内容变化不改 key';
COMMENT ON COLUMN pipeline_task_documents.rel_path IS '相对任务产出目录的路径，正斜杠分隔，由前端分段建树';
COMMENT ON COLUMN pipeline_task_documents.stored_path IS '对象存储地址或本地 /uploads/ 地址，异步上传成功前为空';
COMMENT ON COLUMN pipeline_task_documents.storage_state IS 'pending=待上传，ready=可读，failed=上传失败超过重试上限';
COMMENT ON COLUMN pipeline_task_documents.revision IS '内容版本号，同 key 每次内容变化 +1';
COMMENT ON COLUMN pipeline_task_documents.client_updated_at IS '客户端内容更新时间（毫秒），用于丢弃乱序旧包';
COMMENT ON COLUMN pipeline_task_documents.source_abs_path IS '客户端本机绝对路径，只用于渲染时把路径变成可点击链接';
COMMENT ON COLUMN pipeline_task_documents.deleted_at IS '软删时间戳，任务被客户端删除时写入';

CREATE UNIQUE INDEX IF NOT EXISTS uk_pipeline_task_documents
    ON pipeline_task_documents(admin_id, pipeline_task_id, doc_key);

CREATE INDEX IF NOT EXISTS idx_pipeline_task_documents_task
    ON pipeline_task_documents(admin_id, pipeline_task_id, deleted_at);

CREATE INDEX IF NOT EXISTS idx_pipeline_task_documents_storage
    ON pipeline_task_documents(storage_state, updated_at);

-- 上传作业表即持久化队列；接口侧只入队，worker claim 后写对象存储。
CREATE TABLE IF NOT EXISTS pipeline_document_upload_jobs (
    id            BIGSERIAL PRIMARY KEY,
    document_id   BIGINT  NOT NULL,
    admin_id      INTEGER NOT NULL,
    revision      BIGINT  NOT NULL DEFAULT 0,
    temp_path     TEXT    NOT NULL,
    claimed_by    VARCHAR(80) NOT NULL DEFAULT '',
    claimed_until BIGINT  NOT NULL DEFAULT 0,
    attempts      INTEGER NOT NULL DEFAULT 0,
    last_error    VARCHAR(500) NOT NULL DEFAULT '',
    created_at    BIGINT  NOT NULL DEFAULT 0
);

COMMENT ON TABLE pipeline_document_upload_jobs IS '产出文档异步上传队列，claim 走 FOR UPDATE SKIP LOCKED + 租约';
COMMENT ON COLUMN pipeline_document_upload_jobs.temp_path IS '接口侧落盘的暂存文件绝对路径，上传成功后删除';
COMMENT ON COLUMN pipeline_document_upload_jobs.claimed_until IS '租约到期时间，到期后可被其他 worker 重新 claim';

CREATE INDEX IF NOT EXISTS idx_pipeline_document_upload_jobs_claim
    ON pipeline_document_upload_jobs(claimed_until, id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS pipeline_document_upload_jobs;
DROP TABLE IF EXISTS pipeline_task_documents;
-- +goose StatementEnd

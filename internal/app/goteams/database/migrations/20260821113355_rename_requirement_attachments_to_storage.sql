-- +goose Up
-- +goose StatementBegin

-- 附件表通用化：requirement_attachments 改名 storage_attachments，
-- 增加 attachment_type 区分业务（旧数据全部为 requirement，知识库附件为 knowledge），
-- 增加 folder_id/document_id 供知识库附件关联节点（requirement 类型恒为 0）。

ALTER TABLE requirement_attachments RENAME TO storage_attachments;

ALTER TABLE storage_attachments
    ADD COLUMN attachment_type VARCHAR(20) NOT NULL DEFAULT 'requirement';

ALTER TABLE storage_attachments
    ADD COLUMN folder_id INTEGER NOT NULL DEFAULT 0;

ALTER TABLE storage_attachments
    ADD COLUMN document_id INTEGER NOT NULL DEFAULT 0;

-- 知识库附件按节点查询索引
CREATE INDEX idx_sa_document
    ON storage_attachments(admin_id, workspace_id, document_id);

CREATE INDEX idx_sa_folder
    ON storage_attachments(admin_id, workspace_id, folder_id);

COMMENT ON TABLE storage_attachments IS '通用附件表：需求/评论附件与知识库附件';
COMMENT ON COLUMN storage_attachments.attachment_type IS '业务类型：requirement=需求及评论，knowledge=知识库文档';
COMMENT ON COLUMN storage_attachments.folder_id IS '知识库附件所在文件夹节点 ID，requirement 类型恒为 0';
COMMENT ON COLUMN storage_attachments.document_id IS '知识库附件所在文档节点 ID（knowledge_items.id），requirement 类型恒为 0';
COMMENT ON COLUMN storage_attachments.requirement_id IS '需求 ID，0 表示知识库附件';
COMMENT ON COLUMN storage_attachments.comment_id IS '评论 ID，0 表示需求级附件或知识库附件';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_sa_document;
DROP INDEX IF EXISTS idx_sa_folder;

ALTER TABLE storage_attachments DROP COLUMN IF EXISTS attachment_type;
ALTER TABLE storage_attachments DROP COLUMN IF EXISTS folder_id;
ALTER TABLE storage_attachments DROP COLUMN IF EXISTS document_id;

ALTER TABLE storage_attachments RENAME TO requirement_attachments;

-- +goose StatementEnd

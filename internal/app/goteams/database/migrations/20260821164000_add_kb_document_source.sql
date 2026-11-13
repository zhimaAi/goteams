-- +goose Up
-- +goose StatementBegin

ALTER TABLE knowledge_document_contents
    ADD COLUMN source_type VARCHAR(20) NOT NULL DEFAULT 'created';

ALTER TABLE knowledge_document_contents
    ADD COLUMN source_attachment_id INTEGER NOT NULL DEFAULT 0;

ALTER TABLE knowledge_document_contents
    ADD CONSTRAINT chk_kdc_source_type CHECK (source_type IN ('created', 'upload')),
    ADD CONSTRAINT chk_kdc_source_attachment CHECK (source_attachment_id >= 0);

CREATE INDEX idx_kdc_source_attachment
    ON knowledge_document_contents(admin_id, workspace_id, source_attachment_id)
    WHERE source_attachment_id > 0;

COMMENT ON COLUMN knowledge_document_contents.source_type IS '文档来源：created=前端新建，upload=本地文件上传';
COMMENT ON COLUMN knowledge_document_contents.source_attachment_id IS '本地上传原文件对应 storage_attachments.id，新建文档为 0';
COMMENT ON COLUMN knowledge_document_contents.content_mode IS '内容模式：rich-text/markdown/file';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_kdc_source_attachment;
ALTER TABLE knowledge_document_contents DROP CONSTRAINT IF EXISTS chk_kdc_source_type;
ALTER TABLE knowledge_document_contents DROP CONSTRAINT IF EXISTS chk_kdc_source_attachment;
ALTER TABLE knowledge_document_contents DROP COLUMN IF EXISTS source_attachment_id;
ALTER TABLE knowledge_document_contents DROP COLUMN IF EXISTS source_type;
COMMENT ON COLUMN knowledge_document_contents.content_mode IS '内容模式：rich-text/markdown';

-- +goose StatementEnd

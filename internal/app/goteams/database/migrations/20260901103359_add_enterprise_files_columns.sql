-- +goose Up
-- 补全 enterprise_files_xx 分表字段（承接 storage_attachments 的 stored_name/folder_id/document_id），
-- 并扩展 source_type 语义：增加 knowledge=知识库文档（不再引入 attachment_type 列）。
-- storage_attachments 保留归档，不再被代码读写。

-- +goose StatementBegin
DO $$
DECLARE
    i INTEGER;
BEGIN
    FOR i IN 0..9 LOOP
        EXECUTE format('ALTER TABLE enterprise_files_%s ADD COLUMN IF NOT EXISTS stored_name VARCHAR(100) NOT NULL DEFAULT ''''', i);
        EXECUTE format('ALTER TABLE enterprise_files_%s ADD COLUMN IF NOT EXISTS folder_id INTEGER NOT NULL DEFAULT 0', i);
        EXECUTE format('ALTER TABLE enterprise_files_%s ADD COLUMN IF NOT EXISTS document_id INTEGER NOT NULL DEFAULT 0', i);
        EXECUTE format('CREATE INDEX IF NOT EXISTS idx_ef%s_admin_document ON enterprise_files_%s(admin_id, workspace_id, document_id)', i, i);
        EXECUTE format('CREATE INDEX IF NOT EXISTS idx_ef%s_admin_folder ON enterprise_files_%s(admin_id, workspace_id, folder_id)', i, i);
        EXECUTE format('COMMENT ON COLUMN enterprise_files_%s.source_type IS ''来源类型: requirement=需求, defect=缺陷, knowledge=知识库文档''', i);
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
-- 回滚仅撤销结构变更；已迁移的数据不恢复
-- +goose StatementBegin
DO $$
DECLARE
    i INTEGER;
BEGIN
    FOR i IN 0..9 LOOP
        EXECUTE format('DROP INDEX IF EXISTS idx_ef%s_admin_document', i);
        EXECUTE format('DROP INDEX IF EXISTS idx_ef%s_admin_folder', i);
        EXECUTE format('ALTER TABLE enterprise_files_%s DROP COLUMN IF EXISTS document_id', i);
        EXECUTE format('ALTER TABLE enterprise_files_%s DROP COLUMN IF EXISTS folder_id', i);
        EXECUTE format('ALTER TABLE enterprise_files_%s DROP COLUMN IF EXISTS stored_name', i);
    END LOOP;
END $$;
-- +goose StatementEnd

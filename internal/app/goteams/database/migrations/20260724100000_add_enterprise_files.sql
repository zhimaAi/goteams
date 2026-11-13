-- +goose Up
-- 企业文件台账分表（按 admin_id % 10 取模），存储需求/缺陷附件及图片的追踪记录
-- 通过钩子方式在 attachment 上传/删除时自动同步

-- +goose StatementBegin
DO $$ 
DECLARE
    i INTEGER;
BEGIN
    FOR i IN 0..9 LOOP
        EXECUTE format('
            CREATE TABLE IF NOT EXISTS enterprise_files_%s (
                id SERIAL PRIMARY KEY,
                admin_id INTEGER NOT NULL,
                workspace_id INTEGER NOT NULL,
                source_type VARCHAR(20) NOT NULL,
                source_id INTEGER NOT NULL,
                source_name VARCHAR(500) NOT NULL DEFAULT '''',
                comment_id INTEGER NOT NULL DEFAULT 0,
                file_name VARCHAR(500) NOT NULL,
                file_ext VARCHAR(20) NOT NULL DEFAULT '''',
                file_size BIGINT NOT NULL DEFAULT 0,
                stored_path TEXT NOT NULL,
                content_type VARCHAR(200) NOT NULL DEFAULT '''',
                uploader_id INTEGER NOT NULL DEFAULT 0,
                uploader_name VARCHAR(100) NOT NULL DEFAULT '''',
                created_at BIGINT NOT NULL DEFAULT 0,
                updated_at BIGINT NOT NULL DEFAULT 0
            )', i);

        EXECUTE format('
            COMMENT ON TABLE enterprise_files_%s IS ''企业文件台账分表（admin_id 取模 %%s）''', i, i);

        EXECUTE format('
            COMMENT ON COLUMN enterprise_files_%s.admin_id IS ''租户管理员ID（多租户隔离）''', i);
        EXECUTE format('
            COMMENT ON COLUMN enterprise_files_%s.workspace_id IS ''所属项目ID''', i);
        EXECUTE format('
            COMMENT ON COLUMN enterprise_files_%s.source_type IS ''来源类型: requirement=需求, defect=缺陷''', i);
        EXECUTE format('
            COMMENT ON COLUMN enterprise_files_%s.source_id IS ''来源工作项ID（requirements.id）''', i);
        EXECUTE format('
            COMMENT ON COLUMN enterprise_files_%s.source_name IS ''来源工作项名称（冗余存储，用于列表展示）''', i);
        EXECUTE format('
            COMMENT ON COLUMN enterprise_files_%s.comment_id IS ''评论ID，0=工作项级附件''', i);
        EXECUTE format('
            COMMENT ON COLUMN enterprise_files_%s.file_ext IS ''文件扩展名（不含点），如 png/pdf/mp4''', i);
        EXECUTE format('
            COMMENT ON COLUMN enterprise_files_%s.file_size IS ''文件大小（字节）''', i);
        EXECUTE format('
            COMMENT ON COLUMN enterprise_files_%s.stored_path IS ''OSS完整URL或本地 /uploads/... 路径''', i);
        EXECUTE format('
            COMMENT ON COLUMN enterprise_files_%s.uploader_id IS ''上传者用户ID''', i);

        EXECUTE format('
            CREATE INDEX IF NOT EXISTS idx_ef%s_admin_created ON enterprise_files_%s(admin_id, created_at DESC)', i, i);
        EXECUTE format('
            CREATE INDEX IF NOT EXISTS idx_ef%s_admin_workspace ON enterprise_files_%s(admin_id, workspace_id)', i, i);
        EXECUTE format('
            CREATE INDEX IF NOT EXISTS idx_ef%s_admin_source ON enterprise_files_%s(admin_id, source_type, source_id)', i, i);
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE
    i INTEGER;
BEGIN
    FOR i IN 0..9 LOOP
        EXECUTE format('DROP TABLE IF EXISTS enterprise_files_%s', i);
    END LOOP;
END $$;
-- +goose StatementEnd

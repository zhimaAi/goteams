-- +goose Up
ALTER TABLE workspaces_settings
    ADD COLUMN IF NOT EXISTS requirement_start_status_ids TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS requirement_end_status_ids TEXT NOT NULL DEFAULT '';
COMMENT ON COLUMN workspaces_settings.requirement_start_status_ids IS '首次开始时间的目标状态ID集合';
COMMENT ON COLUMN workspaces_settings.requirement_end_status_ids IS '最近结束时间的目标状态ID集合';

-- 原单选值迁移为单元素集合，0迁移为空集合；不改动已经记录的时间。
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema()
        AND table_name='workspaces_settings' AND column_name='requirement_start_status_id') THEN
        UPDATE workspaces_settings SET
            requirement_start_status_ids=CASE WHEN requirement_start_status_id>0 THEN requirement_start_status_id::TEXT ELSE '' END,
            requirement_end_status_ids=CASE WHEN requirement_end_status_id>0 THEN requirement_end_status_id::TEXT ELSE '' END;
        ALTER TABLE workspaces_settings DROP COLUMN requirement_start_status_id, DROP COLUMN requirement_end_status_id;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- 多选无法无损降级为单选，保留集合，避免丢弃管理员的配置。
SELECT 1;

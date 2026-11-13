-- +goose Up

CREATE TABLE IF NOT EXISTS workspace_work_item_display_settings (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    type_key VARCHAR(32) NOT NULL DEFAULT '',
    edit_display_mode VARCHAR(16) NOT NULL DEFAULT 'create',
    view_display_mode VARCHAR(16) NOT NULL DEFAULT 'create',
    status_fields_json TEXT NOT NULL DEFAULT '{}',
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, workspace_id, type_key)
);

COMMENT ON TABLE workspace_work_item_display_settings IS '工作项编辑与查看页字段显示配置';
COMMENT ON COLUMN workspace_work_item_display_settings.edit_display_mode IS '编辑页显示模式: create/view';
COMMENT ON COLUMN workspace_work_item_display_settings.view_display_mode IS '查看页显示模式: create/status';
COMMENT ON COLUMN workspace_work_item_display_settings.status_fields_json IS '按状态ID保存有序字段key数组的JSON对象';

CREATE INDEX IF NOT EXISTS idx_wids_workspace_type
    ON workspace_work_item_display_settings(admin_id, workspace_id, type_key);

-- +goose Down

DROP TABLE IF EXISTS workspace_work_item_display_settings;

-- +goose Up
CREATE TABLE IF NOT EXISTS user_workspace_order_settings (
    id BIGSERIAL PRIMARY KEY,
    admin_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    workspace_order TEXT NOT NULL DEFAULT '[]',
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE user_workspace_order_settings IS '企业项目侧边栏排序：user_id=0 为企业默认顺序，其余为账号个人顺序';
COMMENT ON COLUMN user_workspace_order_settings.admin_id IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN user_workspace_order_settings.user_id IS '设置所属用户ID，0表示企业默认顺序';
COMMENT ON COLUMN user_workspace_order_settings.workspace_order IS '按顺序排列的项目ID JSON数组';

CREATE UNIQUE INDEX IF NOT EXISTS uk_user_workspace_order_settings
    ON user_workspace_order_settings(admin_id, user_id);

-- +goose Down
DROP TABLE IF EXISTS user_workspace_order_settings;

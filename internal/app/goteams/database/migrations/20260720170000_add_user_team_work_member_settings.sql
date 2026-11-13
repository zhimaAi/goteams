-- +goose Up
CREATE TABLE IF NOT EXISTS user_team_work_member_settings (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    member_user_id INTEGER NOT NULL,
    is_visible SMALLINT NOT NULL DEFAULT 1,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, user_id, member_user_id)
);

COMMENT ON TABLE user_team_work_member_settings IS '用户个人的团队工作成员显示与排序设置';
COMMENT ON COLUMN user_team_work_member_settings.admin_id IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN user_team_work_member_settings.user_id IS '设置所属用户ID';
COMMENT ON COLUMN user_team_work_member_settings.member_user_id IS '被展示的企业成员ID';
COMMENT ON COLUMN user_team_work_member_settings.is_visible IS '是否在团队工作中显示: 1=显示, 0=隐藏';
COMMENT ON COLUMN user_team_work_member_settings.sort_order IS '当前用户自定义排序';

CREATE INDEX IF NOT EXISTS idx_user_team_work_settings_owner
    ON user_team_work_member_settings(admin_id, user_id, sort_order);

-- +goose Down
DROP TABLE IF EXISTS user_team_work_member_settings;

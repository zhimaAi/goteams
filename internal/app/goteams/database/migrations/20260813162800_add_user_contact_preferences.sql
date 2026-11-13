-- +goose Up
CREATE TABLE IF NOT EXISTS user_contact_preferences (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    contact_user_id INTEGER NOT NULL,
    is_favorite SMALLINT NOT NULL DEFAULT 0,
    last_selected_at BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, user_id, contact_user_id)
);

COMMENT ON TABLE user_contact_preferences IS '用户个人的常用联系人与最近选择记录';
COMMENT ON COLUMN user_contact_preferences.admin_id IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN user_contact_preferences.user_id IS '偏好所属用户ID';
COMMENT ON COLUMN user_contact_preferences.contact_user_id IS '联系人用户ID';
COMMENT ON COLUMN user_contact_preferences.is_favorite IS '是否为常用联系人: 1=是, 0=否';
COMMENT ON COLUMN user_contact_preferences.last_selected_at IS '最近选择时间（Unix时间戳）';
COMMENT ON COLUMN user_contact_preferences.created_at IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN user_contact_preferences.updated_at IS '更新时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_user_contact_preferences_favorite
    ON user_contact_preferences(admin_id, user_id, is_favorite, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_contact_preferences_recent
    ON user_contact_preferences(admin_id, user_id, last_selected_at DESC)
    WHERE last_selected_at > 0;

-- +goose Down
DROP TABLE IF EXISTS user_contact_preferences;

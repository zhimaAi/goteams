-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS workspace_navigation_settings (
    id BIGSERIAL PRIMARY KEY,
    admin_id BIGINT NOT NULL,
    workspace_id BIGINT NOT NULL,
    main_nav_order TEXT NOT NULL DEFAULT '[]',
    nav_style VARCHAR(16) NOT NULL DEFAULT 'simple',
    prevent_member_edit SMALLINT NOT NULL DEFAULT 0,
    updated_by BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_workspace_navigation_settings
    ON workspace_navigation_settings(admin_id, workspace_id);

CREATE TABLE IF NOT EXISTS user_workspace_navigation_settings (
    id BIGSERIAL PRIMARY KEY,
    admin_id BIGINT NOT NULL,
    workspace_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    main_nav_order TEXT NOT NULL DEFAULT '[]',
    nav_style VARCHAR(16) NOT NULL DEFAULT 'simple',
    updated_at BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_user_workspace_navigation_settings
    ON user_workspace_navigation_settings(admin_id, workspace_id, user_id);

CREATE INDEX IF NOT EXISTS idx_user_workspace_navigation_settings_workspace
    ON user_workspace_navigation_settings(admin_id, workspace_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS user_workspace_navigation_settings;
DROP TABLE IF EXISTS workspace_navigation_settings;

-- +goose StatementEnd

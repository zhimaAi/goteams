-- +goose Up
CREATE TABLE IF NOT EXISTS workspace_notification_strategies (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    name VARCHAR(30) NOT NULL,
    platform VARCHAR(20) NOT NULL,
    webhook_url TEXT NOT NULL,
    secret TEXT NOT NULL DEFAULT '',
    events JSONB NOT NULL DEFAULT '[]'::jsonb,
    enabled SMALLINT NOT NULL DEFAULT 1,
    today_trigger_count INTEGER NOT NULL DEFAULT 0,
    total_trigger_count INTEGER NOT NULL DEFAULT 0,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_workspace_notification_strategies_scope
    ON workspace_notification_strategies(admin_id, workspace_id, id);
ALTER TABLE workspace_notification_strategies
    ADD COLUMN IF NOT EXISTS enabled SMALLINT NOT NULL DEFAULT 1;

CREATE TABLE IF NOT EXISTS workspace_notification_strategy_members (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    strategy_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, workspace_id, strategy_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_workspace_notification_strategy_members_scope
    ON workspace_notification_strategy_members(admin_id, workspace_id, strategy_id);

-- +goose Down
DROP TABLE IF EXISTS workspace_notification_strategy_members;
DROP TABLE IF EXISTS workspace_notification_strategies;

-- +goose Up
-- External system notification strategies are configured once per enterprise and push for every
-- project, so the tables lose their workspace dimension and are renamed to match the feature.
ALTER TABLE workspace_notification_strategies RENAME TO external_notification_strategies;
ALTER TABLE workspace_notification_strategy_members RENAME TO external_notification_strategy_members;

DROP INDEX IF EXISTS idx_workspace_notification_strategies_scope;
DROP INDEX IF EXISTS idx_workspace_notification_strategy_members_scope;

ALTER TABLE external_notification_strategies DROP COLUMN IF EXISTS workspace_id;
ALTER TABLE external_notification_strategy_members DROP COLUMN IF EXISTS workspace_id;

CREATE INDEX IF NOT EXISTS idx_external_notification_strategies_tenant
    ON external_notification_strategies(admin_id, id);
CREATE INDEX IF NOT EXISTS idx_external_notification_strategy_members_strategy
    ON external_notification_strategy_members(admin_id, strategy_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_external_notification_strategy_member
    ON external_notification_strategy_members(admin_id, strategy_id, user_id);

-- +goose Down
DROP INDEX IF EXISTS uq_external_notification_strategy_member;
DROP INDEX IF EXISTS idx_external_notification_strategies_tenant;
DROP INDEX IF EXISTS idx_external_notification_strategy_members_strategy;

ALTER TABLE external_notification_strategies RENAME TO workspace_notification_strategies;
ALTER TABLE workspace_notification_strategies ADD COLUMN IF NOT EXISTS workspace_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE external_notification_strategy_members RENAME TO workspace_notification_strategy_members;
ALTER TABLE workspace_notification_strategy_members ADD COLUMN IF NOT EXISTS workspace_id INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_workspace_notification_strategies_scope
    ON workspace_notification_strategies(admin_id, workspace_id, id);
CREATE INDEX IF NOT EXISTS idx_workspace_notification_strategy_members_scope
    ON workspace_notification_strategy_members(admin_id, workspace_id, strategy_id);

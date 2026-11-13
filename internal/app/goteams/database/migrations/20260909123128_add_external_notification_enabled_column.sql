-- +goose Up
-- The original strategy migration was amended after it could already have run in dev1.
ALTER TABLE workspace_notification_strategies
    ADD COLUMN IF NOT EXISTS enabled SMALLINT NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE workspace_notification_strategies
    DROP COLUMN IF EXISTS enabled;

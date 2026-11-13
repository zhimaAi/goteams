-- +goose Up

UPDATE workspace_priorities
SET is_default = 0
WHERE is_default <> 0;

CREATE UNIQUE INDEX IF NOT EXISTS uk_wip_single_default
    ON workspace_priorities(admin_id, workspace_id, type_key)
    WHERE is_default = 1;

-- +goose Down

DROP INDEX IF EXISTS uk_wip_single_default;

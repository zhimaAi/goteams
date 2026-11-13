-- +goose Up
CREATE INDEX IF NOT EXISTS idx_requirements_admin_type
    ON requirements (admin_id, type);

-- +goose Down
DROP INDEX IF EXISTS idx_requirements_admin_type;

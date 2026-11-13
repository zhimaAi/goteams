-- +goose Up
CREATE TABLE IF NOT EXISTS notifications (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    recipient_user_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    requirement_id INTEGER NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    actor_user_id INTEGER NOT NULL DEFAULT 0,
    field_name VARCHAR(100) NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    read_at BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_notifications_recipient_read_created
    ON notifications(admin_id, recipient_user_id, read_at, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_item
    ON notifications(admin_id, workspace_id, requirement_id);

-- +goose Down
DROP TABLE IF EXISTS notifications;

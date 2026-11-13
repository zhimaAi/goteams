-- +goose Up
CREATE TABLE workspace_access_requests (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    reason VARCHAR(120) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    UNIQUE (admin_id, workspace_id, user_id),
    CONSTRAINT workspace_access_requests_status_check CHECK (status IN ('pending', 'joined'))
);

COMMENT ON TABLE workspace_access_requests IS '项目加入申请';

ALTER TABLE notifications ALTER COLUMN requirement_id DROP NOT NULL;

-- +goose Down
-- 已有项目通知没有可回填的工作项 ID，回滚会误删通知，因此不逆转本次结构变更。

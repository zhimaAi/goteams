-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS agent_task_sessions (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    agent_task_id BIGINT NOT NULL,
    session_uuid VARCHAR(64) NOT NULL,
    conversation_uuid VARCHAR(64) NOT NULL DEFAULT '',
    parent_session_uuid VARCHAR(64) NOT NULL DEFAULT '',
    step_key VARCHAR(64) NOT NULL,
    run_no INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'created',
    cli_type VARCHAR(32) NOT NULL DEFAULT '',
    request_summary TEXT NOT NULL DEFAULT '',
    response_summary TEXT NOT NULL DEFAULT '',
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    started_at BIGINT NOT NULL DEFAULT 0,
    finished_at BIGINT NOT NULL DEFAULT 0,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, agent_task_id, session_uuid),
    UNIQUE(admin_id, agent_task_id, step_key, run_no)
);

CREATE INDEX IF NOT EXISTS idx_agent_task_sessions_step
    ON agent_task_sessions(
        admin_id, agent_task_id, step_key, run_no, started_at, id
    );

COMMENT ON TABLE agent_task_sessions IS '客户端每一次 CLI 执行或续聊的云端索引';
COMMENT ON COLUMN agent_task_sessions.session_uuid IS '客户端本地 Session UUID';
COMMENT ON COLUMN agent_task_sessions.step_key IS '所属工作流步骤 key';
COMMENT ON COLUMN agent_task_sessions.run_no IS '该步骤下的执行轮次';
COMMENT ON COLUMN agent_task_sessions.request_summary IS '本轮请求，最大 8KiB';
COMMENT ON COLUMN agent_task_sessions.response_summary IS '本轮最终可见回复，最大 16KiB';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS agent_task_sessions;
-- +goose StatementEnd

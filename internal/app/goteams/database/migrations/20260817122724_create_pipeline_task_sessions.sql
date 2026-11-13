-- +goose Up
-- +goose StatementBegin
-- 客户端 gt_cli_sessions 记录级同步的云端落库表：
-- 客户端每创建/更新一条云端任务的 CLI 会话记录，由客户端异步同步服务按任务顺序
-- 逐条推送到云端并在此表 upsert（按 admin_id + pipeline_task_id + session_uuid 幂等）。
CREATE TABLE IF NOT EXISTS pipeline_task_sessions (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    pipeline_task_id BIGINT NOT NULL,
    session_uuid VARCHAR(64) NOT NULL,
    step_key VARCHAR(64) NOT NULL DEFAULT '',
    conversation_uuid VARCHAR(64) NOT NULL DEFAULT '',
    parent_session_uuid VARCHAR(64) NOT NULL DEFAULT '',
    run_no INTEGER NOT NULL DEFAULT 1,
    status VARCHAR(32) NOT NULL DEFAULT '',
    cli_type VARCHAR(32) NOT NULL DEFAULT '',
    model_name VARCHAR(64) NOT NULL DEFAULT '',
    external_session_id TEXT NOT NULL DEFAULT '',
    prompt_snapshot TEXT NOT NULL DEFAULT '',
    result_status VARCHAR(32) NOT NULL DEFAULT '',
    final_result TEXT NOT NULL DEFAULT '',
    exit_code INTEGER NOT NULL DEFAULT 0,
    error_code VARCHAR(64) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    stop_requested_at BIGINT NOT NULL DEFAULT 0,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    latest_event_type VARCHAR(64) NOT NULL DEFAULT '',
    latest_event_content TEXT NOT NULL DEFAULT '',
    latest_event_at BIGINT NOT NULL DEFAULT 0,
    client_started_at BIGINT NOT NULL DEFAULT 0,
    client_finished_at BIGINT NOT NULL DEFAULT 0,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    client_created_at BIGINT NOT NULL DEFAULT 0,
    client_updated_at BIGINT NOT NULL DEFAULT 0,
    last_uploaded_at BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, pipeline_task_id, session_uuid)
);

CREATE INDEX IF NOT EXISTS idx_pipeline_task_sessions_task
    ON pipeline_task_sessions(admin_id, pipeline_task_id, client_created_at, id);

COMMENT ON TABLE pipeline_task_sessions IS '客户端 CLI 会话记录（gt_cli_sessions）的云端镜像，按记录创建/更新逐条同步';
COMMENT ON COLUMN pipeline_task_sessions.admin_id IS '租户 ID';
COMMENT ON COLUMN pipeline_task_sessions.user_id IS '执行任务的用户 ID';
COMMENT ON COLUMN pipeline_task_sessions.pipeline_task_id IS '所属流水线任务投影 ID';
COMMENT ON COLUMN pipeline_task_sessions.session_uuid IS '客户端本地会话 UUID';
COMMENT ON COLUMN pipeline_task_sessions.step_key IS '所属步骤 key';
COMMENT ON COLUMN pipeline_task_sessions.conversation_uuid IS '会话对话标识（同一对话链共享）';
COMMENT ON COLUMN pipeline_task_sessions.parent_session_uuid IS '父会话 UUID（续聊时指向上一次会话）';
COMMENT ON COLUMN pipeline_task_sessions.run_no IS '该步骤内的第几次运行';
COMMENT ON COLUMN pipeline_task_sessions.status IS '会话生命周期状态：created/running/waiting_input/success/failed/stopped/interrupted 等';
COMMENT ON COLUMN pipeline_task_sessions.cli_type IS 'CLI 类型：codex/claude/codebuddy 等';
COMMENT ON COLUMN pipeline_task_sessions.model_name IS '使用的模型名';
COMMENT ON COLUMN pipeline_task_sessions.external_session_id IS 'CLI 原生会话 ID（用于恢复续聊）';
COMMENT ON COLUMN pipeline_task_sessions.prompt_snapshot IS '本次发送的提示词快照';
COMMENT ON COLUMN pipeline_task_sessions.result_status IS '结果状态';
COMMENT ON COLUMN pipeline_task_sessions.final_result IS '最终结果文本';
COMMENT ON COLUMN pipeline_task_sessions.exit_code IS 'CLI 进程退出码';
COMMENT ON COLUMN pipeline_task_sessions.error_code IS '错误码';
COMMENT ON COLUMN pipeline_task_sessions.error_message IS '错误信息';
COMMENT ON COLUMN pipeline_task_sessions.stop_requested_at IS '用户请求停止的时间（Unix 毫秒，0=未请求）';
COMMENT ON COLUMN pipeline_task_sessions.input_tokens IS '输入 Token 数';
COMMENT ON COLUMN pipeline_task_sessions.output_tokens IS '输出 Token 数';
COMMENT ON COLUMN pipeline_task_sessions.total_tokens IS '总 Token 数';
COMMENT ON COLUMN pipeline_task_sessions.latest_event_type IS '最近一次 CLI 活动事件类型';
COMMENT ON COLUMN pipeline_task_sessions.latest_event_content IS '最近一次 CLI 活动事件内容';
COMMENT ON COLUMN pipeline_task_sessions.latest_event_at IS '最近一次 CLI 活动事件时间（Unix 毫秒）';
COMMENT ON COLUMN pipeline_task_sessions.client_started_at IS '客户端会话开始时间（Unix 毫秒）';
COMMENT ON COLUMN pipeline_task_sessions.client_finished_at IS '客户端会话结束时间（Unix 毫秒）';
COMMENT ON COLUMN pipeline_task_sessions.duration_ms IS '会话耗时（毫秒）';
COMMENT ON COLUMN pipeline_task_sessions.client_created_at IS '客户端记录创建时间（Unix 毫秒）';
COMMENT ON COLUMN pipeline_task_sessions.client_updated_at IS '客户端记录更新时间（Unix 毫秒），用于乱序推送保护';
COMMENT ON COLUMN pipeline_task_sessions.last_uploaded_at IS '最后一次上报时间（Unix 秒）';
COMMENT ON COLUMN pipeline_task_sessions.created_at IS '云端创建时间（Unix 秒）';
COMMENT ON COLUMN pipeline_task_sessions.updated_at IS '云端更新时间（Unix 秒）';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS pipeline_task_sessions;
-- +goose StatementEnd

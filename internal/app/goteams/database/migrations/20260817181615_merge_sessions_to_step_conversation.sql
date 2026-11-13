-- +goose Up
-- +goose StatementBegin
-- 会话同步模型收敛：pipeline_task_conversations 废弃，会话内容合并进
-- pipeline_task_sessions（每步骤一条，唯一键 admin_id + pipeline_task_id + step_key）。
-- 客户端按步骤聚合完整对话 JSON（conversation_json）推送，云端整行 upsert 覆盖；
-- client_updated_at 保留作乱序保护（旧于已存行时拒绝覆盖）。
DROP TABLE IF EXISTS pipeline_task_conversations;

DROP TABLE IF EXISTS pipeline_task_sessions;
CREATE TABLE IF NOT EXISTS pipeline_task_sessions (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    pipeline_task_id BIGINT NOT NULL,
    step_key VARCHAR(64) NOT NULL,
    step_name VARCHAR(128) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT '',
    cli_type VARCHAR(32) NOT NULL DEFAULT '',
    model_name VARCHAR(64) NOT NULL DEFAULT '',
    prompt_snapshot TEXT NOT NULL DEFAULT '',
    result_status VARCHAR(32) NOT NULL DEFAULT '',
    final_result TEXT NOT NULL DEFAULT '',
    exit_code INTEGER NOT NULL DEFAULT 0,
    error_code VARCHAR(64) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    conversation_json JSONB NOT NULL DEFAULT '{"round_count":0,"rounds":[]}'::jsonb,
    conversation_rounds INTEGER NOT NULL DEFAULT 0,
    started_at BIGINT NOT NULL DEFAULT 0,
    finished_at BIGINT NOT NULL DEFAULT 0,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    last_uploaded_at BIGINT NOT NULL DEFAULT 0,
    client_updated_at BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, pipeline_task_id, step_key)
);

CREATE INDEX IF NOT EXISTS idx_pipeline_task_sessions_task
    ON pipeline_task_sessions(admin_id, pipeline_task_id, started_at, id);

COMMENT ON TABLE pipeline_task_sessions IS '步骤级会话（每步骤一条，含完整对话 JSON 与运行结果汇总）';
COMMENT ON COLUMN pipeline_task_sessions.admin_id IS '租户 ID';
COMMENT ON COLUMN pipeline_task_sessions.user_id IS '执行任务的用户 ID';
COMMENT ON COLUMN pipeline_task_sessions.pipeline_task_id IS '所属流水线任务投影 ID';
COMMENT ON COLUMN pipeline_task_sessions.step_key IS '所属步骤 key（步骤粒度唯一，每步骤一条）';
COMMENT ON COLUMN pipeline_task_sessions.step_name IS '步骤名快照';
COMMENT ON COLUMN pipeline_task_sessions.status IS '步骤会话状态：created/running/waiting_input/success/failed/stopped/interrupted 等';
COMMENT ON COLUMN pipeline_task_sessions.cli_type IS 'CLI 类型：codex/claude/codebuddy 等';
COMMENT ON COLUMN pipeline_task_sessions.model_name IS '该步骤最后一次运行使用的模型名';
COMMENT ON COLUMN pipeline_task_sessions.prompt_snapshot IS '该步骤最后一次运行的提示词快照';
COMMENT ON COLUMN pipeline_task_sessions.result_status IS '结果状态';
COMMENT ON COLUMN pipeline_task_sessions.final_result IS '最终结果文本';
COMMENT ON COLUMN pipeline_task_sessions.exit_code IS 'CLI 进程退出码';
COMMENT ON COLUMN pipeline_task_sessions.error_code IS '错误码';
COMMENT ON COLUMN pipeline_task_sessions.error_message IS '错误信息';
COMMENT ON COLUMN pipeline_task_sessions.input_tokens IS '步骤输入 Token 数';
COMMENT ON COLUMN pipeline_task_sessions.output_tokens IS '步骤输出 Token 数';
COMMENT ON COLUMN pipeline_task_sessions.total_tokens IS '步骤总 Token 数';
COMMENT ON COLUMN pipeline_task_sessions.conversation_json IS '步骤完整对话 JSON（round_count 与每轮用户问题及逐条 AI 消息）';
COMMENT ON COLUMN pipeline_task_sessions.conversation_rounds IS '步骤内对话轮次';
COMMENT ON COLUMN pipeline_task_sessions.started_at IS '步骤会话开始时间（Unix 毫秒）';
COMMENT ON COLUMN pipeline_task_sessions.finished_at IS '步骤会话结束时间（Unix 毫秒）';
COMMENT ON COLUMN pipeline_task_sessions.duration_ms IS '会话耗时（毫秒）';
COMMENT ON COLUMN pipeline_task_sessions.last_uploaded_at IS '最后一次上报时间（Unix 秒）';
COMMENT ON COLUMN pipeline_task_sessions.client_updated_at IS '客户端记录更新时间（Unix 毫秒），用于乱序推送保护';
COMMENT ON COLUMN pipeline_task_sessions.created_at IS '云端创建时间（Unix 秒）';
COMMENT ON COLUMN pipeline_task_sessions.updated_at IS '云端更新时间（Unix 秒）';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 反向迁移：重建会话记录粒度表与对话聚合表（数据已丢弃，仅重建结构）。
DROP TABLE IF EXISTS pipeline_task_sessions;
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

CREATE TABLE IF NOT EXISTS pipeline_task_conversations (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    pipeline_task_id BIGINT NOT NULL,
    conversation_uuid VARCHAR(64) NOT NULL,
    step_key VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'created',
    cli_type VARCHAR(32) NOT NULL DEFAULT '',
    conversation_json JSONB NOT NULL DEFAULT '{"round_count":0,"rounds":[]}'::jsonb,
    error_summary TEXT NOT NULL DEFAULT '',
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    started_at BIGINT NOT NULL DEFAULT 0,
    finished_at BIGINT NOT NULL DEFAULT 0,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, pipeline_task_id, conversation_uuid)
);
-- +goose StatementEnd
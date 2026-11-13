-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS agent_task_conversations (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    agent_task_id BIGINT NOT NULL,
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
    UNIQUE(admin_id, agent_task_id, conversation_uuid)
);

CREATE INDEX IF NOT EXISTS idx_agent_task_conversations_step
    ON agent_task_conversations(
        admin_id, agent_task_id, step_key, started_at, id
    );

-- 把旧的逐 Session 数据按 conversation_uuid 聚合成一次会话。
-- 历史 response_summary 无法可靠拆分时保留为该轮的一条 AI 消息。
INSERT INTO agent_task_conversations (
    admin_id, agent_task_id, conversation_uuid, step_key, status, cli_type,
    conversation_json, error_summary, input_tokens, output_tokens, total_tokens,
    started_at, finished_at, duration_ms, created_at, updated_at
)
SELECT
    grouped.admin_id,
    grouped.agent_task_id,
    grouped.conversation_uuid,
    (grouped.last_value->>'step_key')::VARCHAR(64),
    (grouped.last_value->>'status')::VARCHAR(32),
    (grouped.last_value->>'cli_type')::VARCHAR(32),
    jsonb_build_object(
        'round_count', grouped.round_count,
        'rounds', grouped.rounds
    ),
    '',
    grouped.input_tokens,
    grouped.output_tokens,
    grouped.total_tokens,
    grouped.started_at,
    grouped.finished_at,
    grouped.duration_ms,
    grouped.created_at,
    grouped.updated_at
FROM (
    SELECT
        admin_id,
        agent_task_id,
        normalized_conversation_uuid AS conversation_uuid,
        COUNT(*)::INTEGER AS round_count,
        jsonb_agg(
            jsonb_build_object(
                'round_no', conversation_round_no,
                'session_uuid', session_uuid,
                'status', status,
                'user_message', request_summary,
                'assistant_messages', CASE
                    WHEN response_summary <> '' THEN jsonb_build_array(
                        jsonb_build_object(
                            'sequence', 1,
                            'role', 'assistant',
                            'content', response_summary,
                            'timestamp', ''
                        )
                    )
                    ELSE '[]'::jsonb
                END,
                'error_summary', '',
                'usage', jsonb_build_object(
                    'input_tokens', input_tokens,
                    'output_tokens', output_tokens,
                    'total_tokens', total_tokens,
                    'conversation_rounds', 1
                ),
                'started_at', CASE WHEN started_at > 0
                    THEN to_char(
                        to_timestamp(started_at) AT TIME ZONE 'UTC',
                        'YYYY-MM-DD"T"HH24:MI:SS"Z"'
                    )
                    ELSE NULL
                END,
                'finished_at', CASE WHEN finished_at > 0
                    THEN to_char(
                        to_timestamp(finished_at) AT TIME ZONE 'UTC',
                        'YYYY-MM-DD"T"HH24:MI:SS"Z"'
                    )
                    ELSE NULL
                END,
                'duration_ms', duration_ms
            )
            ORDER BY run_no, started_at, id
        ) AS rounds,
        (array_agg(
            jsonb_build_object('step_key', step_key, 'status', status, 'cli_type', cli_type)
            ORDER BY run_no DESC, started_at DESC, id DESC
        ))[1] AS last_value,
        SUM(input_tokens) AS input_tokens,
        SUM(output_tokens) AS output_tokens,
        SUM(total_tokens) AS total_tokens,
        COALESCE(MIN(NULLIF(started_at, 0)), 0) AS started_at,
        MAX(finished_at) AS finished_at,
        SUM(duration_ms) AS duration_ms,
        MIN(created_at) AS created_at,
        MAX(updated_at) AS updated_at
    FROM (
        SELECT
            source_session.*,
            COALESCE(NULLIF(conversation_uuid, ''), session_uuid) AS normalized_conversation_uuid,
            ROW_NUMBER() OVER (
                PARTITION BY admin_id, agent_task_id,
                    COALESCE(NULLIF(conversation_uuid, ''), session_uuid)
                ORDER BY run_no, started_at, id
            ) AS conversation_round_no
        FROM agent_task_sessions AS source_session
    ) AS ranked_session
    GROUP BY admin_id, agent_task_id, normalized_conversation_uuid
) AS grouped
ON CONFLICT (admin_id, agent_task_id, conversation_uuid) DO NOTHING;

COMMENT ON TABLE agent_task_conversations IS '按 conversation_uuid 聚合的客户端 AI 会话';
COMMENT ON COLUMN agent_task_conversations.conversation_json IS '完整会话 JSON，包含 round_count、每轮用户问题及逐条 AI 消息';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS agent_task_conversations;
-- +goose StatementEnd

-- +goose Up

-- 「新增子任务」已从通知类型下线：清掉存量策略里遗留的事件 key，
-- 否则配置了它的旧策略会因为枚举校验拒绝而永远改不动。
UPDATE external_notification_strategies
SET events = COALESCE((
    SELECT jsonb_agg(value)
    FROM jsonb_array_elements(events) AS value
    WHERE value <> '"subtask_created"'
), '[]'::jsonb)
WHERE jsonb_typeof(events) = 'array' AND events @> '"subtask_created"'::jsonb;

-- +goose Down

-- Down 为 no-op：events 里的旧 key 已经无从还原，强行补回可能会给本就没有该事件的策略加上它。

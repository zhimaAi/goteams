-- +goose Up
-- +goose StatementBegin
-- 幂等补种，重复执行时保留调度游标。
INSERT INTO scheduled_tasks (
    task_key, scope_key, admin_id, enabled, schedule_type,
    interval_seconds, next_run_at, created_at, updated_at
)
VALUES (
    'tenant_requirement_totals', 'global', 0, 1, 'interval',
    3600, EXTRACT(EPOCH FROM NOW())::BIGINT + 3600,
    EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
)
ON CONFLICT (task_key, scope_key) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
DELETE FROM scheduled_tasks
WHERE task_key = 'tenant_requirement_totals' AND scope_key = 'global' AND admin_id = 0;

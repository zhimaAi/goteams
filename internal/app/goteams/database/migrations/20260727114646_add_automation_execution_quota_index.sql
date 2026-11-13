-- +goose Up

-- 支持按企业统计北京时间自然月内的自动化执行次数。
CREATE INDEX IF NOT EXISTS idx_automation_exec_month
    ON automation_executions(admin_id, created_at);

-- +goose Down

DROP INDEX IF EXISTS idx_automation_exec_month;

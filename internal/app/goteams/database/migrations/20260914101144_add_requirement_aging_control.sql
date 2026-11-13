-- +goose Up
-- 时效管控配置：需求类型独立的 JSON 配置列，默认关闭。
ALTER TABLE workspaces_settings
    ADD COLUMN IF NOT EXISTS requirement_aging_control TEXT NOT NULL DEFAULT '{"master_enabled":false,"total_duration":{"enabled":0,"timeout_minutes":4320,"notify_enabled":0},"status_rules":[]}';

COMMENT ON COLUMN "workspaces_settings"."requirement_aging_control" IS '需求时效管控配置 JSON：{"master_enabled":false,"total_duration":{"enabled":0,"timeout_minutes":4320,"notify_enabled":0},"status_rules":[{"status_id":0,"enabled":0,"timeout_minutes":1440,"notify_enabled":0}]}';

-- 需求整体时效结论派生列：0=未超时，1=已超时。由 SLA Worker 维护，列表与筛选直接读取。
ALTER TABLE requirements
    ADD COLUMN IF NOT EXISTS aging_status SMALLINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN "requirements"."aging_status" IS '需求整体时效结论：0=未超时，1=已超时；由 SLA Worker 维护';

-- SLA 通知去重状态表：同一工作项 + 同一规则 + 同一通知类型仅保留一条有效记录。
CREATE TABLE IF NOT EXISTS sla_notification_states (
    id               SERIAL PRIMARY KEY,
    admin_id         INTEGER      NOT NULL DEFAULT 0,
    workspace_id     INTEGER      NOT NULL DEFAULT 0,
    requirement_id   INTEGER      NOT NULL DEFAULT 0,
    rule_key         VARCHAR(64)  NOT NULL DEFAULT '',
    notify_type      VARCHAR(32)  NOT NULL DEFAULT '',
    active           SMALLINT     NOT NULL DEFAULT 0,
    last_notified_at BIGINT       NOT NULL DEFAULT 0,
    created_at       BIGINT       NOT NULL DEFAULT 0,
    updated_at       BIGINT       NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sla_notification_states_unique
    ON sla_notification_states (admin_id, workspace_id, requirement_id, rule_key, notify_type);

CREATE INDEX IF NOT EXISTS idx_sla_notification_states_active
    ON sla_notification_states (active, admin_id, workspace_id);

-- +goose Down
DROP TABLE IF EXISTS sla_notification_states;
ALTER TABLE requirements DROP COLUMN IF EXISTS aging_status;
ALTER TABLE workspaces_settings DROP COLUMN IF EXISTS requirement_aging_control;

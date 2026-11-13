-- +goose Up
-- +goose StatementBegin
ALTER TABLE requirement_sla_action_executions
    ADD COLUMN delivery_platform VARCHAR(16) NOT NULL DEFAULT '' CHECK (delivery_platform IN ('','feishu','wecom','dingtalk')),
    ADD COLUMN lease_token VARCHAR(36) NOT NULL DEFAULT '';

-- 旧唯一约束由 PostgreSQL 自动命名，通过定义定位，避免依赖截断后的名称。
DO $$
DECLARE old_constraint TEXT;
BEGIN
    SELECT conname INTO STRICT old_constraint FROM pg_constraint
    WHERE conrelid='requirement_sla_action_executions'::regclass
      AND contype='u' AND pg_get_constraintdef(oid)='UNIQUE (admin_id, workspace_id, cycle_id, action_key)';
    EXECUTE format('ALTER TABLE requirement_sla_action_executions DROP CONSTRAINT %I', old_constraint);
END $$;
ALTER TABLE requirement_sla_action_executions ADD CONSTRAINT uq_sla_action_platform
    UNIQUE(admin_id,workspace_id,cycle_id,action_key,delivery_platform);

-- 仅拆分未完成的旧 Webhook 动作；保留历史成功、跳过和取消记录，避免重放。
INSERT INTO requirement_sla_action_executions
    (admin_id,workspace_id,requirement_id,cycle_id,action_key,trigger_elapsed_seconds,due_at,clock_revision,state,reason,attempts,next_attempt_at,created_at,updated_at,delivery_platform)
SELECT a.admin_id,a.workspace_id,a.requirement_id,a.cycle_id,a.action_key,a.trigger_elapsed_seconds,a.due_at,a.clock_revision,a.state,a.reason,a.attempts,a.next_attempt_at,a.created_at,a.updated_at,hook->>'platform'
FROM requirement_sla_action_executions a
JOIN requirement_sla_cycles c ON c.admin_id=a.admin_id AND c.workspace_id=a.workspace_id AND c.requirement_id=a.requirement_id AND c.id=a.cycle_id
CROSS JOIN LATERAL jsonb_array_elements(c.snapshot->'config'->'actions') action
CROSS JOIN LATERAL jsonb_array_elements(action->'webhooks') hook
WHERE a.delivery_platform='' AND a.state IN ('pending','failed')
  AND action->>'key'=a.action_key AND action->>'channel'='webhook'
ON CONFLICT(admin_id,workspace_id,cycle_id,action_key,delivery_platform) DO NOTHING;

UPDATE requirement_sla_action_executions a SET state='skipped',reason='platform_split',next_attempt_at=0
WHERE a.delivery_platform='' AND a.state IN ('pending','failed') AND EXISTS (
    SELECT 1 FROM requirement_sla_action_executions child
    WHERE child.admin_id=a.admin_id AND child.workspace_id=a.workspace_id AND child.cycle_id=a.cycle_id
      AND child.action_key=a.action_key AND child.delivery_platform<>''
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 多平台执行已有独立投递结果，无法无损合并回单条记录，拒绝破坏性回滚。
DO $$ BEGIN RAISE EXCEPTION 'SLA platform executions cannot be safely merged; use a forward migration'; END $$;
-- +goose StatementEnd

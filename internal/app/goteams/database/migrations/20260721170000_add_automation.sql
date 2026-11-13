-- +goose Up

-- 自动化规则表使用完整建表语句，规则配置以 JSONB 保存。
CREATE TABLE automation_rules (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    name VARCHAR(200) NOT NULL,
    enabled SMALLINT NOT NULL DEFAULT 0,
    created_by INTEGER NOT NULL,
    creator_name_snapshot VARCHAR(100) NOT NULL DEFAULT '',
    creator_avatar_snapshot VARCHAR(500) NOT NULL DEFAULT '',
    locale VARCHAR(10) NOT NULL DEFAULT 'zh-CN',
    total_trigger_count BIGINT NOT NULL DEFAULT 0,
    config_json JSONB NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

COMMENT ON TABLE automation_rules IS '项目自动化规则表';
COMMENT ON COLUMN automation_rules.id IS '规则主键ID';
COMMENT ON COLUMN automation_rules.admin_id IS '租户管理员ID';
COMMENT ON COLUMN automation_rules.workspace_id IS '工作空间ID';
COMMENT ON COLUMN automation_rules.name IS '规则名称';
COMMENT ON COLUMN automation_rules.enabled IS '是否启用：0否，1是';
COMMENT ON COLUMN automation_rules.created_by IS '规则创建者用户ID';
COMMENT ON COLUMN automation_rules.creator_name_snapshot IS '创建者名称快照';
COMMENT ON COLUMN automation_rules.creator_avatar_snapshot IS '创建者头像快照';
COMMENT ON COLUMN automation_rules.locale IS '规则创建时使用的语言';
COMMENT ON COLUMN automation_rules.total_trigger_count IS '累计触发次数';
COMMENT ON COLUMN automation_rules.config_json IS '版本化规则配置';
COMMENT ON COLUMN automation_rules.created_at IS '创建时间戳';
COMMENT ON COLUMN automation_rules.updated_at IS '更新时间戳';

CREATE UNIQUE INDEX idx_automation_rule_name
    ON automation_rules(admin_id, workspace_id, lower(btrim(name)));
CREATE INDEX idx_automation_rule_list
    ON automation_rules(admin_id, workspace_id, created_at, id);

-- 自动化执行表使用完整建表语句，同时承担 PostgreSQL 持久化队列。
CREATE TABLE automation_executions (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    rule_id INTEGER NOT NULL,
    trigger_item_id INTEGER NOT NULL,
    trigger_item_type VARCHAR(20) NOT NULL,
    trigger_item_title VARCHAR(500) NOT NULL DEFAULT '',
    event_type VARCHAR(30) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    triggered_at BIGINT NOT NULL,
    started_at BIGINT NOT NULL DEFAULT 0,
    finished_at BIGINT NOT NULL DEFAULT 0,
    failure_reason TEXT NOT NULL DEFAULT '',
    rule_name_snapshot VARCHAR(200) NOT NULL,
    creator_id_snapshot INTEGER NOT NULL,
    creator_name_snapshot VARCHAR(100) NOT NULL DEFAULT '',
    creator_avatar_snapshot VARCHAR(500) NOT NULL DEFAULT '',
    locale VARCHAR(10) NOT NULL DEFAULT 'zh-CN',
    config_snapshot JSONB NOT NULL,
    trigger_snapshot JSONB NOT NULL,
    node_progress JSONB NOT NULL DEFAULT '{}',
    structured_logs JSONB NOT NULL DEFAULT '[]',
    lease_owner VARCHAR(100) NOT NULL DEFAULT '',
    lease_until BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    CONSTRAINT chk_automation_execution_status CHECK (status IN ('pending','running','success','skipped','failed'))
);

COMMENT ON TABLE automation_executions IS '项目自动化执行记录及持久化队列表';
COMMENT ON COLUMN automation_executions.id IS '执行记录主键ID';
COMMENT ON COLUMN automation_executions.admin_id IS '租户管理员ID';
COMMENT ON COLUMN automation_executions.workspace_id IS '工作空间ID';
COMMENT ON COLUMN automation_executions.rule_id IS '触发规则ID';
COMMENT ON COLUMN automation_executions.trigger_item_id IS '触发工作项ID';
COMMENT ON COLUMN automation_executions.trigger_item_type IS '触发工作项类型';
COMMENT ON COLUMN automation_executions.trigger_item_title IS '触发工作项标题快照';
COMMENT ON COLUMN automation_executions.event_type IS '触发事件类型';
COMMENT ON COLUMN automation_executions.status IS '执行状态：pending、running、success、skipped、failed';
COMMENT ON COLUMN automation_executions.triggered_at IS '事件触发时间戳';
COMMENT ON COLUMN automation_executions.started_at IS '开始执行时间戳';
COMMENT ON COLUMN automation_executions.finished_at IS '完成执行时间戳';
COMMENT ON COLUMN automation_executions.failure_reason IS '最终失败原因键';
COMMENT ON COLUMN automation_executions.rule_name_snapshot IS '规则名称快照';
COMMENT ON COLUMN automation_executions.creator_id_snapshot IS '规则创建者用户ID快照';
COMMENT ON COLUMN automation_executions.creator_name_snapshot IS '规则创建者名称快照';
COMMENT ON COLUMN automation_executions.creator_avatar_snapshot IS '规则创建者头像快照';
COMMENT ON COLUMN automation_executions.locale IS '执行记录使用的语言';
COMMENT ON COLUMN automation_executions.config_snapshot IS '触发时的规则配置快照';
COMMENT ON COLUMN automation_executions.trigger_snapshot IS '原始业务事件快照';
COMMENT ON COLUMN automation_executions.node_progress IS '已完成节点检查点';
COMMENT ON COLUMN automation_executions.structured_logs IS '结构化执行日志';
COMMENT ON COLUMN automation_executions.lease_owner IS '当前租约持有者';
COMMENT ON COLUMN automation_executions.lease_until IS '租约到期时间戳';
COMMENT ON COLUMN automation_executions.created_at IS '创建时间戳';
COMMENT ON COLUMN automation_executions.updated_at IS '更新时间戳';

CREATE INDEX idx_automation_execution_list
    ON automation_executions(admin_id, workspace_id, triggered_at DESC, id DESC);
CREATE INDEX idx_automation_execution_rule
    ON automation_executions(admin_id, workspace_id, rule_id, triggered_at DESC, id DESC);
CREATE INDEX idx_automation_execution_queue
    ON automation_executions(status, lease_until, id);
CREATE INDEX idx_automation_execution_item_fifo
    ON automation_executions(admin_id, workspace_id, trigger_item_id, id, status);

-- 以下字段属于既有表，升级迁移必须使用 ALTER TABLE，不能重复建表。
ALTER TABLE requirement_comments
    ADD COLUMN source VARCHAR(20) NOT NULL DEFAULT 'user',
    ADD COLUMN automation_rule_id INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN automation_execution_id BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN author_name_snapshot VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN author_avatar_snapshot VARCHAR(500) NOT NULL DEFAULT '';
COMMENT ON COLUMN requirement_comments.source IS '评论来源：user或automation';
COMMENT ON COLUMN requirement_comments.automation_rule_id IS '产生评论的自动化规则ID';
COMMENT ON COLUMN requirement_comments.automation_execution_id IS '产生评论的自动化执行ID';
COMMENT ON COLUMN requirement_comments.author_name_snapshot IS '评论作者名称快照';
COMMENT ON COLUMN requirement_comments.author_avatar_snapshot IS '评论作者头像快照';
CREATE INDEX idx_comments_automation
    ON requirement_comments(admin_id, workspace_id, automation_rule_id);

ALTER TABLE requirement_histories
    ADD COLUMN source VARCHAR(20) NOT NULL DEFAULT 'user',
    ADD COLUMN automation_rule_id INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN automation_execution_id BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN actor_name_snapshot VARCHAR(300) NOT NULL DEFAULT '';
COMMENT ON COLUMN requirement_histories.source IS '历史记录来源：user或automation';
COMMENT ON COLUMN requirement_histories.automation_rule_id IS '产生历史记录的自动化规则ID';
COMMENT ON COLUMN requirement_histories.automation_execution_id IS '产生历史记录的自动化执行ID';
COMMENT ON COLUMN requirement_histories.actor_name_snapshot IS '操作人名称快照';
CREATE INDEX idx_histories_automation
    ON requirement_histories(admin_id, workspace_id, automation_rule_id);

ALTER TABLE workspace_priorities
    ADD COLUMN key VARCHAR(100) NOT NULL DEFAULT '';
COMMENT ON COLUMN workspace_priorities.key IS '跨工作空间映射使用的稳定优先级键';
UPDATE workspace_priorities
SET key = CASE name
    WHEN '${workitem_priority_urgent}' THEN 'urgent'
    WHEN '${workitem_priority_high}' THEN 'high'
    WHEN '${workitem_priority_medium}' THEN 'medium'
    WHEN '${workitem_priority_low}' THEN 'low'
    ELSE 'priority_' || id::text
END
WHERE key = '';
CREATE INDEX idx_priority_stable_key
    ON workspace_priorities(admin_id, workspace_id, type_key, key);

-- 为已有系统项目管理员补齐自动化管理权限。
INSERT INTO role_permissions(admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, 'automation:manage', EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
WHERE r.scope = 'project' AND r.is_system = 1 AND r.name = '${perm_role_project_admin}'
  AND NOT EXISTS (
      SELECT 1 FROM role_permissions rp
      WHERE rp.admin_id = r.admin_id AND rp.role_id = r.id AND rp.permission_key = 'automation:manage'
  );

-- +goose Down

DELETE FROM role_permissions WHERE permission_key = 'automation:manage';
DROP INDEX IF EXISTS idx_priority_stable_key;
ALTER TABLE workspace_priorities DROP COLUMN IF EXISTS key;
DROP INDEX IF EXISTS idx_histories_automation;
ALTER TABLE requirement_histories
    DROP COLUMN IF EXISTS actor_name_snapshot,
    DROP COLUMN IF EXISTS automation_execution_id,
    DROP COLUMN IF EXISTS automation_rule_id,
    DROP COLUMN IF EXISTS source;
DROP INDEX IF EXISTS idx_comments_automation;
ALTER TABLE requirement_comments
    DROP COLUMN IF EXISTS author_avatar_snapshot,
    DROP COLUMN IF EXISTS author_name_snapshot,
    DROP COLUMN IF EXISTS automation_execution_id,
    DROP COLUMN IF EXISTS automation_rule_id,
    DROP COLUMN IF EXISTS source;
DROP TABLE IF EXISTS automation_executions;
DROP TABLE IF EXISTS automation_rules;

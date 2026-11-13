-- +goose Up
-- +goose StatementBegin
-- 阶段、目标、暂停配置归属独立设置 JSON，运行事实保存为周期和动作。
CREATE TABLE workspace_sla_settings (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    timezone VARCHAR(64) NOT NULL DEFAULT '',
    activated_at BIGINT NOT NULL DEFAULT 0,
    config_revision BIGINT NOT NULL DEFAULT 0 CHECK (config_revision >= 0),
    clock_revision BIGINT NOT NULL DEFAULT 0 CHECK (clock_revision >= 0),
    next_definition_id BIGINT NOT NULL DEFAULT 1 CHECK (next_definition_id > 0),
    stages JSONB NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(stages)='array'),
    policies JSONB NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(policies)='array'),
    pauses JSONB NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(pauses)='array'),
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    UNIQUE (admin_id,workspace_id)
);

-- 仅保留最近一次有效回调的匹配上下文，历史执行事实在周期快照中保存。
ALTER TABLE requirements ADD COLUMN sla_match JSONB;

CREATE TABLE requirement_sla_cycles (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    requirement_id INTEGER NOT NULL,
    policy_id BIGINT NOT NULL,
    policy_revision BIGINT NOT NULL,
    stage_id BIGINT NOT NULL,
    stage_revision BIGINT NOT NULL,
    start_event_key VARCHAR(100) NOT NULL DEFAULT '',
    matched_key VARCHAR(100) NOT NULL,
    start_mutation_key VARCHAR(100) NOT NULL,
    start_reason VARCHAR(24) NOT NULL CHECK (start_reason IN ('transition', 'resubmitted')),
    end_mutation_key VARCHAR(100) NOT NULL DEFAULT '',
    state VARCHAR(16) NOT NULL CHECK (state IN ('running', 'completed', 'interrupted')),
    end_reason VARCHAR(24) NOT NULL DEFAULT '',
    started_at BIGINT NOT NULL,
    ended_at BIGINT,
    limit_seconds BIGINT NOT NULL CHECK (limit_seconds > 0),
    elapsed_seconds BIGINT CHECK (elapsed_seconds >= 0),
    calculation_state VARCHAR(24) NOT NULL DEFAULT 'ready' CHECK (calculation_state IN ('ready', 'calendar_unavailable')),
    snapshot JSONB NOT NULL,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    CHECK (ended_at IS NULL OR ended_at >= started_at),
    CHECK ((state = 'running' AND ended_at IS NULL AND end_reason = '') OR
           (state = 'completed' AND ended_at IS NOT NULL AND end_reason = 'completed') OR
           (state = 'interrupted' AND ended_at IS NOT NULL AND end_reason IN ('restarted', 'rematched', 'resubmitted', 'transferred'))),
    UNIQUE (admin_id, workspace_id, requirement_id, stage_id, start_mutation_key),
    UNIQUE (admin_id, workspace_id, requirement_id, id),
    FOREIGN KEY (admin_id,workspace_id) REFERENCES workspace_sla_settings(admin_id,workspace_id)
);
CREATE UNIQUE INDEX idx_sla_cycle_running ON requirement_sla_cycles(admin_id, workspace_id, requirement_id, stage_id) WHERE state = 'running';
CREATE INDEX idx_sla_cycle_history ON requirement_sla_cycles(admin_id, workspace_id, requirement_id, stage_id, id DESC);
CREATE INDEX idx_sla_cycle_calendar ON requirement_sla_cycles(admin_id, workspace_id, id) WHERE calculation_state = 'calendar_unavailable';

CREATE TABLE requirement_sla_action_executions (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    requirement_id INTEGER NOT NULL,
    cycle_id BIGINT NOT NULL,
    action_key VARCHAR(64) NOT NULL,
    trigger_elapsed_seconds BIGINT NOT NULL CHECK (trigger_elapsed_seconds >= 0),
    due_at BIGINT,
    clock_revision BIGINT NOT NULL,
    state VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'sent', 'skipped', 'canceled', 'failed')),
    recipient_ids INTEGER[] NOT NULL DEFAULT '{}',
    sent_at BIGINT,
    reason VARCHAR(64) NOT NULL DEFAULT '',
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    CHECK ((state = 'sent' AND sent_at IS NOT NULL) OR (state <> 'sent' AND sent_at IS NULL)),
    UNIQUE (admin_id, workspace_id, cycle_id, action_key),
    FOREIGN KEY (admin_id, workspace_id, requirement_id, cycle_id) REFERENCES requirement_sla_cycles(admin_id, workspace_id, requirement_id, id)
);
CREATE INDEX idx_sla_actions_due ON requirement_sla_action_executions(due_at, id) WHERE state = 'pending';
CREATE INDEX idx_sla_actions_calendar ON requirement_sla_action_executions(next_attempt_at, id) WHERE state = 'pending' AND due_at IS NULL;

-- 在原删除事务内锁定空间设置，保护 JSON 配置和运行快照里的状态/优先级引用。
CREATE FUNCTION sla_guard_definition_delete() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    referenced BOOLEAN := FALSE;
    vars JSONB := jsonb_build_object('id', to_jsonb(OLD.id));
BEGIN
    IF OLD.type_key <> 'requirement' THEN RETURN OLD; END IF;
    PERFORM id FROM workspaces_settings WHERE admin_id=OLD.admin_id AND workspace_id=OLD.workspace_id FOR UPDATE;
    IF TG_TABLE_NAME='workspace_statuses' THEN
        SELECT EXISTS (
            SELECT 1 FROM workspace_sla_settings settings, jsonb_array_elements(settings.stages) s
            WHERE settings.admin_id=OLD.admin_id AND settings.workspace_id=OLD.workspace_id
              AND COALESCE((s->>'archived_at')::BIGINT,0)=0
              AND (
                jsonb_path_exists(s->'config', '$.**.status_ids[*] ? (@ == $id)', vars)
                OR (jsonb_path_exists(s->'config', '$.**.mode ? (@ == "workspace_initial")') AND EXISTS (
                    SELECT 1 FROM workspaces_settings w WHERE w.admin_id=OLD.admin_id AND w.workspace_id=OLD.workspace_id AND w.requirement_initial_status_id=OLD.id))
                OR (jsonb_path_exists(s->'config', '$.**.mode ? (@ == "workspace_final")') AND EXISTS (
                    SELECT 1 FROM workspaces_settings w WHERE w.admin_id=OLD.admin_id AND w.workspace_id=OLD.workspace_id
                    AND OLD.id=ANY(string_to_array(NULLIF(w.requirement_final_status_ids,''),',')::INTEGER[])))
              )
        ) OR EXISTS (
            SELECT 1 FROM requirement_sla_cycles c WHERE c.admin_id=OLD.admin_id AND c.workspace_id=OLD.workspace_id
            AND c.state='running' AND jsonb_path_exists(c.snapshot, '$.stages[*].config.**.status_ids[*] ? (@ == $id)', vars)
        ) INTO referenced;
    ELSE
        SELECT EXISTS (
            SELECT 1 FROM workspace_sla_settings settings, jsonb_array_elements(settings.policies) p
            WHERE settings.admin_id=OLD.admin_id AND settings.workspace_id=OLD.workspace_id
            AND COALESCE((p->>'archived_at')::BIGINT,0)=0
            AND (jsonb_path_exists(p, '$.priority_ids[*] ? (@ == $id)', vars)
                 OR jsonb_path_exists(p->'config', '$.targets[*].priority_id ? (@ == $id)', vars))
        ) OR EXISTS (
            SELECT 1 FROM requirement_sla_cycles c WHERE c.admin_id=OLD.admin_id AND c.workspace_id=OLD.workspace_id
            AND c.state='running' AND (c.snapshot->>'priority_id')::INTEGER=OLD.id
        ) INTO referenced;
    END IF;
    IF referenced THEN
        RAISE EXCEPTION USING ERRCODE='23503', MESSAGE='sla_reference_conflict';
    END IF;
    RETURN OLD;
END;
$$;
CREATE TRIGGER guard_sla_status_delete BEFORE DELETE ON workspace_statuses FOR EACH ROW EXECUTE FUNCTION sla_guard_definition_delete();
CREATE TRIGGER guard_sla_priority_delete BEFORE DELETE ON workspace_priorities FOR EACH ROW EXECUTE FUNCTION sla_guard_definition_delete();

-- 存量系统工作空间管理员补齐权限，不按固定角色 ID 授权。
INSERT INTO role_permissions(admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, grants.permission_key, EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r CROSS JOIN (VALUES ('sla:view'), ('sla:manage')) AS grants(permission_key)
WHERE r.scope = 'project' AND r.is_system = 1 AND r.name LIKE '%perm_role_project_admin%'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER guard_sla_status_delete ON workspace_statuses;
DROP TRIGGER guard_sla_priority_delete ON workspace_priorities;
DROP FUNCTION sla_guard_definition_delete();
DROP TABLE requirement_sla_action_executions;
DROP TABLE requirement_sla_cycles;
DROP TABLE workspace_sla_settings;
ALTER TABLE requirements DROP COLUMN sla_match;
-- 无法区分迁移补种与用户授权，保留权限行避免误删后续授权。
-- +goose StatementEnd

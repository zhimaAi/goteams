-- +goose Up
-- 历史归档项目（archived=1）改为物理清理：删除项目及其全部关联数据，随后废弃 archived 字段
-- +goose StatementBegin
DO $$
DECLARE
    ws_ids INT[];
    req_ids INT[];
    role_ids INT[];
    task_ids BIGINT[];
    tbl TEXT;
BEGIN
    SELECT array_agg(id) INTO ws_ids FROM workspaces WHERE COALESCE(archived, 0) = 1;
    IF ws_ids IS NULL THEN
        RETURN;
    END IF;

    SELECT array_agg(id) INTO req_ids FROM requirements WHERE workspace_id = ANY(ws_ids);
    SELECT array_agg(id) INTO role_ids FROM roles WHERE workspace_id = ANY(ws_ids);
    SELECT array_agg(id) INTO task_ids FROM pipeline_tasks WHERE workspace_id = ANY(ws_ids);

    -- 仅通过需求ID关联的表
    IF req_ids IS NOT NULL THEN
        FOREACH tbl IN ARRAY ARRAY['requirement_custom_field_values', 'requirement_visits'] LOOP
            IF to_regclass('public.' || tbl) IS NOT NULL THEN
                EXECUTE format('DELETE FROM %I WHERE requirement_id = ANY($1)', tbl) USING req_ids;
            END IF;
        END LOOP;
    END IF;

    -- 仅通过角色ID关联的表
    IF role_ids IS NOT NULL THEN
        FOREACH tbl IN ARRAY ARRAY['role_permissions', 'role_data_view_members', 'role_team_work_members'] LOOP
            IF to_regclass('public.' || tbl) IS NOT NULL THEN
                EXECUTE format('DELETE FROM %I WHERE role_id = ANY($1)', tbl) USING role_ids;
            END IF;
        END LOOP;
    END IF;

    -- 仅通过流水线任务ID关联的表
    IF task_ids IS NOT NULL THEN
        FOREACH tbl IN ARRAY ARRAY['pipeline_task_steps', 'pipeline_task_sessions', 'pipeline_task_conversations'] LOOP
            IF to_regclass('public.' || tbl) IS NOT NULL THEN
                EXECUTE format('DELETE FROM %I WHERE pipeline_task_id = ANY($1)', tbl) USING task_ids;
            END IF;
        END LOOP;
    END IF;

    -- 带 workspace_id 的项目数据表（历史环境可能缺少已废弃的表，逐张判断是否存在）
    FOREACH tbl IN ARRAY ARRAY['automation_executions', 'automation_rules',
        'workspace_views', 'user_view_settings', 'requirement_categories',
        'requirement_attachments', 'requirement_comments', 'requirement_histories',
        'requirements', 'planning_relations', 'planning_groups', 'versions', 'workspace_members',
        'workspace_statuses', 'workspace_priorities', 'workspace_status_transitions', 'workspace_field_configs',
        'workspace_custom_fields', 'workspaces_settings', 'roles', 'pipeline_tasks'] LOOP
        IF to_regclass('public.' || tbl) IS NOT NULL THEN
            EXECUTE format('DELETE FROM %I WHERE workspace_id = ANY($1)', tbl) USING ws_ids;
        END IF;
    END LOOP;

    DELETE FROM workspaces WHERE id = ANY(ws_ids);
END $$;
-- +goose StatementEnd

-- 项目不再有归档态，slug 租户内唯一不需要再依赖 archived 的部分索引
DROP INDEX IF EXISTS workspaces_slug_active_key;
CREATE UNIQUE INDEX IF NOT EXISTS workspaces_slug_tenant_key ON workspaces(admin_id, slug);
ALTER TABLE workspaces DROP COLUMN IF EXISTS archived;

-- +goose Down
-- 已物理删除的历史归档项目数据无法恢复，回滚仅还原字段与索引结构
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS archived SMALLINT DEFAULT 0;
COMMENT ON COLUMN "workspaces"."archived" IS '归档状态: 0=正常, 1=已归档';
DROP INDEX IF EXISTS workspaces_slug_tenant_key;
CREATE UNIQUE INDEX IF NOT EXISTS workspaces_slug_active_key ON workspaces(admin_id, slug) WHERE COALESCE(archived, 0) = 0;

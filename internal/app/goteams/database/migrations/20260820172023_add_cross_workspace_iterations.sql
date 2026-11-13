-- +goose Up
-- +goose StatementBegin

ALTER TABLE versions
    ADD COLUMN IF NOT EXISTS ended_at BIGINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN "versions"."ended_at" IS '迭代结束时间（Unix时间戳，0=未结束）';

CREATE TABLE IF NOT EXISTS version_workspaces (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    version_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE "version_workspaces" IS '迭代与工作空间关联表';
COMMENT ON COLUMN "version_workspaces"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "version_workspaces"."version_id" IS '迭代ID';
COMMENT ON COLUMN "version_workspaces"."workspace_id" IS '关联工作空间ID';
COMMENT ON COLUMN "version_workspaces"."created_at" IS '创建时间（Unix时间戳）';

CREATE UNIQUE INDEX IF NOT EXISTS uniq_version_workspaces
    ON version_workspaces(admin_id, version_id, workspace_id);
CREATE INDEX IF NOT EXISTS idx_version_workspace_lookup
    ON version_workspaces(admin_id, workspace_id, version_id);

-- 存量迭代默认关联其原始工作空间。
INSERT INTO version_workspaces (admin_id, version_id, workspace_id, created_at)
SELECT admin_id, id, workspace_id, created_at
FROM versions
ON CONFLICT (admin_id, version_id, workspace_id) DO NOTHING;

-- 为存量工作空间补齐迭代系统视图。
INSERT INTO workspace_views (admin_id, workspace_id, type_key, name, view_type, builtin_key, filters, created_by, created_at, updated_at)
SELECT w.admin_id, w.id, 'iteration', v.name, 'system', v.builtin_key, v.filters, 0,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces w
CROSS JOIN (VALUES
    ('${view_builtin_all}',         'all',         ''),
    ('${view_builtin_my_assigned}', 'my_assigned', '{"logic":"and","conditions":[{"field":"assignee","operator":"in","values":["__me__"]}]}'),
    ('${view_builtin_my_created}',  'my_created',  '{"logic":"and","conditions":[{"field":"creator","operator":"in","values":["__me__"]}]}'),
    ('${view_builtin_unfinished}',  'unfinished',  '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]}]}'),
    ('${view_builtin_finished}',    'finished',    '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_end__"]}]}')
) AS v(name, builtin_key, filters)
ON CONFLICT (admin_id, workspace_id, type_key, builtin_key) WHERE builtin_key <> '' DO NOTHING;

-- 存量工作空间管理员获得管理迭代系统视图权限。
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, 'version:manage_system_view',
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
WHERE r.scope = 'project'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_project_admin%'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DELETE FROM role_permissions rp
USING roles r
WHERE rp.role_id = r.id
  AND rp.admin_id = r.admin_id
  AND rp.permission_key = 'version:manage_system_view'
  AND r.scope = 'project'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_project_admin%';

DELETE FROM user_view_settings WHERE type_key = 'iteration';
DELETE FROM workspace_views WHERE type_key = 'iteration';

-- 回滚为单工作空间模型前，移除跨工作空间工作项的迭代绑定。
UPDATE requirements r
SET version_id = NULL
FROM versions v
WHERE r.admin_id = v.admin_id
  AND r.version_id = v.id
  AND r.workspace_id <> v.workspace_id;

DROP TABLE IF EXISTS version_workspaces;
ALTER TABLE versions DROP COLUMN IF EXISTS ended_at;

-- +goose StatementEnd

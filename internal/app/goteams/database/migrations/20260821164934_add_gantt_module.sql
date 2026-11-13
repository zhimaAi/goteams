-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- 甘特图模块：应用开关字段、内置视图补种、权限补齐
-- ============================================================

-- 工作空间设置表新增甘特图应用开关（应用市场里默认关闭）。
ALTER TABLE workspaces_settings
    ADD COLUMN IF NOT EXISTS gantt_enabled SMALLINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN "workspaces_settings"."gantt_enabled" IS '甘特图应用开关: 1=开启, 0=关闭（默认）';

-- +goose StatementEnd

-- +goose StatementBegin
-- 存量工作空间补种甘特图内置视图（新建空间由 seedWorkspaceViews 负责）。
-- 甘特图视图 filters 为三段结构 {common, requirement, defect}，
-- 未完成/已完成按类型分段存放，保证结束状态按各自类型展开。

INSERT INTO workspace_views (admin_id, workspace_id, type_key, name, view_type, builtin_key, filters, created_by, created_at, updated_at)
SELECT w.admin_id, w.id, 'gantt', v.name, 'system', v.builtin_key, v.filters, 0,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces w
CROSS JOIN (VALUES
    ('${view_builtin_all}',         'all',         ''),
    ('${view_builtin_my_assigned}', 'my_assigned', '{"common":{"logic":"and","conditions":[{"field":"assignee","operator":"in","values":["__me__"]}]}}'),
    ('${view_builtin_my_created}',  'my_created',  '{"common":{"logic":"and","conditions":[{"field":"creator","operator":"in","values":["__me__"]}]}}'),
    ('${view_builtin_unfinished}',  'unfinished',  '{"requirement":{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]}]},"defect":{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]}]}}'),
    ('${view_builtin_finished}',    'finished',    '{"requirement":{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_end__"]}]},"defect":{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_end__"]}]}}')
) AS v(name, builtin_key, filters)
ON CONFLICT (admin_id, workspace_id, type_key, builtin_key) WHERE builtin_key <> '' DO NOTHING;

-- +goose StatementEnd

-- +goose StatementBegin
-- 权限补齐：存量项目角色（含自定义角色）默认开启「查看甘特图」。

INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, 'gantt:view',
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
WHERE r.scope = 'project'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- 存量项目管理员额外获得「新增/编辑系统视图」权限。
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, 'gantt:manage_system_view',
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
  AND rp.permission_key IN ('gantt:view', 'gantt:manage_system_view')
  AND r.scope = 'project';

DELETE FROM user_view_settings WHERE type_key = 'gantt';
DELETE FROM workspace_views WHERE type_key = 'gantt';

ALTER TABLE workspaces_settings DROP COLUMN IF EXISTS gantt_enabled;

-- +goose StatementEnd

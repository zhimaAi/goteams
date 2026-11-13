-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- 内置视图与权限重补种
-- 20260812125904 已在部分环境应用后，环境切换到不含视图功能的
-- 版本系运行，期间新建的工作空间缺少内置视图与「新增/编辑系统
-- 视图」权限（种子逻辑不存在、迁移又不会重跑）。
-- 本迁移原样重跑该迁移的两段幂等补种 SQL，补齐窗口期存量。
-- ============================================================

INSERT INTO workspace_views (admin_id, workspace_id, type_key, name, view_type, builtin_key, filters, created_by, created_at, updated_at)
SELECT w.admin_id, w.id, t.type_key, v.name, 'system', v.builtin_key, v.filters, 0,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces w
CROSS JOIN (VALUES ('requirement'), ('defect')) AS t(type_key)
CROSS JOIN (VALUES
    ('${view_builtin_all}',         'all',         ''),
    ('${view_builtin_my_assigned}', 'my_assigned', '{"logic":"and","conditions":[{"field":"assignee","operator":"in","values":["__me__"]}]}'),
    ('${view_builtin_my_created}',  'my_created',  '{"logic":"and","conditions":[{"field":"creator","operator":"in","values":["__me__"]}]}'),
    ('${view_builtin_unfinished}',  'unfinished',  '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]}]}'),
    ('${view_builtin_finished}',    'finished',    '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_end__"]}]}')
) AS v(name, builtin_key, filters)
ON CONFLICT (admin_id, workspace_id, type_key, builtin_key) WHERE builtin_key <> '' DO NOTHING;

-- +goose StatementEnd

-- +goose StatementBegin

-- 项目管理员角色补齐需求/缺陷的「新增/编辑系统视图」权限。
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, v.permission_key,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
CROSS JOIN (VALUES ('requirement:manage_system_view'), ('defect:manage_system_view')) AS v(permission_key)
WHERE r.scope = 'project'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_project_admin%'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 本迁移只重跑 20260812125904 的幂等补种，无法区分本次补入与
-- 既有种子数据，回滚删除会误伤正常数据，故 Down 不做操作。
SELECT 1;

-- +goose StatementEnd

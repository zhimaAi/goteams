-- +goose Up

-- kb:* 权限在知识库一期上线时才加入 permission.PresetRoles，
-- 而 SeedProjectRoles 只在创建项目时执行一次，存量项目的系统预设项目角色缺少这些权限行。
-- 知识库接口以 RBAC 权限判定，必须回填，否则存量项目的成员访问 /api/workspaces/:wsId/kb 会被中间件直接 403。

-- 项目管理员：查看 + 新建 + 管理
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, p.permission_key,
       EXTRACT(EPOCH FROM NOW())::BIGINT,
       EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
CROSS JOIN (VALUES ('kb:view'), ('kb:create'), ('kb:manage')) AS p(permission_key)
WHERE r.scope = 'project'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_project_admin%'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- 项目成员：仅查看 + 新建
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, p.permission_key,
       EXTRACT(EPOCH FROM NOW())::BIGINT,
       EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
CROSS JOIN (VALUES ('kb:view'), ('kb:create')) AS p(permission_key)
WHERE r.scope = 'project'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_project_member%'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose Down

DELETE FROM role_permissions
WHERE permission_key IN ('kb:view', 'kb:create', 'kb:manage');

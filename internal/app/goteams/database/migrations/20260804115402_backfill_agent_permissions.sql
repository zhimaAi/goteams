-- +goose Up

-- agent:* 权限在 Agent 模块上线时才加入 permission.PresetRoles，
-- 而 SeedEnterpriseRoles 只在租户注册时执行一次，存量租户的系统预设企业角色缺少这些权限行。
-- Agent 业务判定已从硬编码角色名改为 RBAC 权限判定，必须回填，否则存量租户的
-- 企业管理员/企业成员访问 /api/agents 会被中间件直接 403。

-- 企业管理员：查看 + 管理 + 查看全部任务
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, p.permission_key,
       EXTRACT(EPOCH FROM NOW())::BIGINT,
       EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
CROSS JOIN (VALUES ('agent:view'), ('agent:manage'), ('agent:view_all_tasks')) AS p(permission_key)
WHERE r.scope = 'enterprise'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_enterprise_admin%'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- 企业成员：仅查看
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, 'agent:view',
       EXTRACT(EPOCH FROM NOW())::BIGINT,
       EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
WHERE r.scope = 'enterprise'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_enterprise_member%'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose Down

DELETE FROM role_permissions
WHERE permission_key IN ('agent:view', 'agent:manage', 'agent:view_all_tasks');

-- +goose Up

-- sso:setting 在本次上线时才加入 permission.PresetRoles 的企业管理员角色，
-- 而企业预设角色只在租户初始化时种子化一次，存量租户已落库的企业管理员角色缺少这一行。
-- SSO 配置接口以 RBAC 权限判定，必须回填，否则存量企业的管理员访问 /api/enterprise/sso 会被中间件直接 403。

-- 企业管理员：SSO 配置
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, 'sso:setting',
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
WHERE r.scope = 'enterprise' AND r.is_system = 1
  AND r.name LIKE '%perm_role_enterprise_admin%'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose Down

DELETE FROM role_permissions
WHERE permission_key IN ('sso:setting');

-- +goose Up
-- +goose StatementBegin

-- 对齐 Agent 权限默认值：企业管理员拥有全部 Agent 权限。
-- 20260804115402 的回填只覆盖当时已存在的租户，这里再做一次幂等补齐，
-- 保证存量企业管理员角色在「角色管理」中能看到并拥有 Agent 权限。
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, v.permission_key,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
CROSS JOIN (VALUES ('agent:view'), ('agent:manage')) AS v(permission_key)
WHERE r.scope = 'enterprise'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_enterprise_admin%'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose StatementEnd

-- +goose StatementBegin

-- 企业成员默认不开放 Agent：回收 20260804115402 补给系统预设企业成员角色的 agent:view。
-- 仅作用于 is_system = 1 的预设成员角色，自定义企业角色不受影响；
-- 确有需要时由企业管理员在角色管理中新建企业角色并勾选 agent:view 后分配。
DELETE FROM role_permissions rp
USING roles r
WHERE rp.role_id = r.id
  AND rp.admin_id = r.admin_id
  AND rp.permission_key IN ('agent:view', 'agent:manage')
  AND r.scope = 'enterprise'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_enterprise_member%';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 回滚：把 agent:view 重新补给系统预设企业成员角色（与 20260804115402 的回填保持一致）。
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, 'agent:view',
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
WHERE r.scope = 'enterprise'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_enterprise_member%'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose StatementEnd

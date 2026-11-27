-- +goose Up

-- 拆分权限时保留存量工作空间角色原有的自动分配访问能力。
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT rp.admin_id, rp.role_id, grants.permission_key,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM role_permissions rp
JOIN roles r ON r.admin_id = rp.admin_id AND r.id = rp.role_id AND r.scope = 'project'
JOIN (VALUES
    ('automation:view', 'auto_assignment:view'),
    ('automation:manage', 'auto_assignment:manage')
) AS grants(source_key, permission_key) ON grants.source_key = rp.permission_key
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose Down

-- 无法区分补种与后续主动授权，回滚不删除权限，避免误删。

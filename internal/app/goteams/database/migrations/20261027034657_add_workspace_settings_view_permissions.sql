-- +goose Up

-- 保留旧角色的可用能力：自动化管理员获得查看权限，原项目信息编辑者获得应用市场查看和管理权限。
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT rp.admin_id, rp.role_id, grants.permission_key,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM role_permissions rp
JOIN roles r ON r.admin_id = rp.admin_id AND r.id = rp.role_id AND r.scope = 'project'
JOIN (VALUES
    ('automation:manage', 'automation:view'),
    ('project_info:edit', 'application_market:view'),
    ('project_info:edit', 'application_market:manage')
) AS grants(source_key, permission_key) ON grants.source_key = rp.permission_key
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose Down

-- 无法区分迁移补种与用户后续主动授权，回滚不删除权限，避免误删。

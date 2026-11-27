-- +goose Up

-- 保留工单工作空间中已有管理角色的读取能力，包括自定义角色。
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT rp.admin_id, rp.role_id, 'satisfaction:view',
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM role_permissions rp
JOIN roles r ON r.admin_id = rp.admin_id AND r.id = rp.role_id AND r.scope = 'project'
JOIN workspaces w ON w.admin_id = r.admin_id AND w.id = r.workspace_id AND w.workspace_type = 'issue'
WHERE rp.permission_key = 'satisfaction:manage'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose Down

-- 无法区分迁移补种和后续主动授权，回退保留权限以防误删。
SELECT 1;

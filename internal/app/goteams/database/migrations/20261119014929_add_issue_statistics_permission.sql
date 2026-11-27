-- +goose Up

-- 仅为工单工作空间的预设管理员补齐统计权限，其他角色需显式授权。
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, 'issue_statistics:view',
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
JOIN workspaces w ON w.admin_id = r.admin_id AND w.id = r.workspace_id
WHERE r.scope = 'project'
  AND r.is_system = 1
  AND r.name IN ('perm_role_project_admin', '${perm_role_project_admin}')
  AND w.workspace_type = 'issue'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose Down

-- 无法区分补种与后续主动授权，回滚保留权限，避免误删。
SELECT 1;

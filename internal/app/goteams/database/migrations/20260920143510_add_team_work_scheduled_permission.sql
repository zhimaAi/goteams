-- +goose Up
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, 'team_work:view_scheduled', EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
WHERE r.scope = 'enterprise'
  AND r.name IN ('perm_role_enterprise_admin', '${perm_role_enterprise_admin}')
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose Down
DELETE FROM role_permissions
WHERE permission_key = 'team_work:view_scheduled';

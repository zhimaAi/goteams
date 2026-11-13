-- +goose Up

INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT
    r.admin_id,
    r.id,
    'plan:view',
    EXTRACT(EPOCH FROM NOW())::BIGINT,
    EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
WHERE r.scope = 'enterprise'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_enterprise_admin%'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose Down

DELETE FROM role_permissions
WHERE permission_key = 'plan:view';

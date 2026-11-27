-- +goose Up
INSERT INTO role_permissions
    (admin_id, role_id, permission_key, created_at, updated_at)
SELECT
    r.admin_id,
    r.id,
    p.permission_key,
    EXTRACT(EPOCH FROM NOW())::BIGINT,
    EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles AS r
JOIN workspaces AS w
    ON w.admin_id = r.admin_id AND w.id = r.workspace_id
CROSS JOIN (VALUES
    ('report:view'),
    ('report:generate'),
    ('report:export')
) AS p(permission_key)
WHERE r.scope = 'project'
  AND r.is_system = 1
  AND r.workspace_id > 0
  AND r.name IN ('perm_role_project_admin', '${perm_role_project_admin}')
  AND w.workspace_type <> 'issue'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose Down
-- 无法区分本次补种和后续主动授予的权限，回退保留记录。
SELECT 1;

-- +goose Up
-- 权限键 member:change_role 更名为 member:edit（对齐 yun 版本系，降低分支合并差异）
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT admin_id, role_id, 'member:edit', created_at, updated_at
FROM role_permissions
WHERE permission_key = 'member:change_role'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

DELETE FROM role_permissions WHERE permission_key = 'member:change_role';

-- +goose Down
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT admin_id, role_id, 'member:change_role', created_at, updated_at
FROM role_permissions
WHERE permission_key = 'member:edit'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

DELETE FROM role_permissions WHERE permission_key = 'member:edit';

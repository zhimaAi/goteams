-- +goose Up
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS remark VARCHAR(500) NOT NULL DEFAULT '';

COMMENT ON COLUMN users.remark IS '企业成员备注信息';

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

ALTER TABLE users DROP COLUMN IF EXISTS remark;

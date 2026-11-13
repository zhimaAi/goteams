-- +goose Up
ALTER TABLE roles
    ADD COLUMN IF NOT EXISTS team_work_view_scope VARCHAR(20) NOT NULL DEFAULT 'self';

COMMENT ON COLUMN roles.team_work_view_scope IS '团队工作查看范围: all=全部成员, partial=部分成员, self=仅自己';

UPDATE roles
SET team_work_view_scope = 'all'
WHERE scope = 'enterprise'
  AND name IN ('perm_role_enterprise_admin', '${perm_role_enterprise_admin}');

CREATE TABLE IF NOT EXISTS role_team_work_members (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    role_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, role_id, user_id)
);

COMMENT ON TABLE role_team_work_members IS '企业角色团队工作可查看成员范围';
COMMENT ON COLUMN role_team_work_members.admin_id IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN role_team_work_members.role_id IS '企业角色ID';
COMMENT ON COLUMN role_team_work_members.user_id IS '允许查看的企业成员ID';

CREATE INDEX IF NOT EXISTS idx_role_team_work_members_role
    ON role_team_work_members(admin_id, role_id);
CREATE INDEX IF NOT EXISTS idx_role_team_work_members_user
    ON role_team_work_members(admin_id, user_id);

INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, permission_key, EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
CROSS JOIN (
    VALUES
        ('team_work:view_today'),
        ('team_work:view_yesterday'),
        ('team_work:view_tomorrow'),
        ('team_work:view_custom_date')
) AS permissions(permission_key)
WHERE r.scope = 'enterprise'
  AND r.name IN ('perm_role_enterprise_admin', '${perm_role_enterprise_admin}')
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose Down
DELETE FROM role_permissions
WHERE permission_key IN (
    'team_work:view_today',
    'team_work:view_yesterday',
    'team_work:view_tomorrow',
    'team_work:view_custom_date'
);

DROP TABLE IF EXISTS role_team_work_members;
ALTER TABLE roles DROP COLUMN IF EXISTS team_work_view_scope;

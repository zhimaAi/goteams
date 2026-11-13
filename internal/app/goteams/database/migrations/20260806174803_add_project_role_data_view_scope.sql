-- +goose Up
ALTER TABLE roles
    ADD COLUMN IF NOT EXISTS data_view_scope VARCHAR(20) NOT NULL DEFAULT 'self';

COMMENT ON COLUMN roles.data_view_scope IS '项目列表数据查看范围: all=全部成员, partial=部分成员, self=仅自己';

-- 存量数据回填：项目管理员默认查看全部数据，其余角色（含普通成员/新角色）保持默认 self
UPDATE roles
SET data_view_scope = 'all'
WHERE scope = 'project'
  AND is_system = 1
  AND name IN ('perm_role_project_admin', '${perm_role_project_admin}');

CREATE TABLE IF NOT EXISTS role_data_view_members (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    role_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, role_id, user_id)
);

COMMENT ON TABLE role_data_view_members IS '项目角色列表数据可查看成员范围';
COMMENT ON COLUMN role_data_view_members.admin_id IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN role_data_view_members.role_id IS '项目角色ID';
COMMENT ON COLUMN role_data_view_members.user_id IS '允许查看其处理数据的项目成员ID';

CREATE INDEX IF NOT EXISTS idx_role_data_view_members_role
    ON role_data_view_members(admin_id, role_id);
CREATE INDEX IF NOT EXISTS idx_role_data_view_members_user
    ON role_data_view_members(admin_id, user_id);

-- +goose Down
DROP TABLE IF EXISTS role_data_view_members;
ALTER TABLE roles DROP COLUMN IF EXISTS data_view_scope;

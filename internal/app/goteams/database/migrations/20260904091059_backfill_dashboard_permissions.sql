-- +goose Up
-- +goose StatementBegin

-- dashboard:* 在项目仪表盘上线时加入预设项目角色。既有项目的系统角色只会在创建项目时种入，
-- 因此需要幂等回填：项目管理员默认拥有查看和新建，普通成员默认只有查看。
-- 自定义角色不在此处变更，仍由项目角色配置页显式授予。

INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, p.permission_key,
       EXTRACT(EPOCH FROM NOW())::BIGINT,
       EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
CROSS JOIN (VALUES ('dashboard:view'), ('dashboard:create')) AS p(permission_key)
WHERE r.scope = 'project'
  AND r.is_system = 1
  -- 同时兼容 i18n key 改为 ${...} 前已落库的角色名；不使用模糊匹配。
  AND r.name IN ('perm_role_project_admin', '${perm_role_project_admin}')
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, 'dashboard:view',
       EXTRACT(EPOCH FROM NOW())::BIGINT,
       EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
WHERE r.scope = 'project'
  AND r.is_system = 1
  AND r.name IN ('perm_role_project_member', '${perm_role_project_member}')
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 不执行反向删除：升级前可能已有管理员手工授予这些权限，删除会静默覆盖其有意配置。

-- +goose StatementEnd

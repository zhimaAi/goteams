-- +goose Up
-- +goose StatementBegin

-- 项目成员角色补齐规划的查看与编辑权限。
-- 规划模块上线时（20260803100000）只补了项目管理员，导致存量工作空间的项目成员
-- 连规划列表都拉不到（planning:view 是列表接口的守卫）。
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, v.permission_key,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
CROSS JOIN (VALUES ('planning:view'), ('planning:edit')) AS v(permission_key)
WHERE r.scope = 'project'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_project_member%'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 只回收本次为项目成员补的权限，项目管理员的同名权限由上一版迁移负责。
DELETE FROM role_permissions rp
USING roles r
WHERE rp.role_id = r.id
  AND rp.admin_id = r.admin_id
  AND rp.permission_key IN ('planning:view', 'planning:edit')
  AND r.scope = 'project'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_project_member%';

-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin

-- 规划默认不再开放给项目成员：回收存量系统预设项目成员角色的规划查看与编辑权限。
-- 与 20260806100000 的补齐范围完全对称，仅作用于 is_system = 1 的预设成员角色，
-- 自定义项目角色不受影响；确有需要时由项目管理员在角色管理中手动勾选。
DELETE FROM role_permissions rp
USING roles r
WHERE rp.role_id = r.id
  AND rp.admin_id = r.admin_id
  AND rp.permission_key IN ('planning:view', 'planning:edit')
  AND r.scope = 'project'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_project_member%';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 回滚：把规划权限重新补给系统预设项目成员角色。
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

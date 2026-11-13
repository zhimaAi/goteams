-- +goose Up

-- 补齐版本系切换窗口期内新建项目的系统项目管理员自动化管理权限
INSERT INTO role_permissions(admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, 'automation:manage', EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
WHERE r.scope = 'project'
  AND r.is_system = 1
  AND r.name = '${perm_role_project_admin}'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose Down

-- 不回滚补种权限，避免误删原本已由自动化迁移或工作空间创建逻辑写入的数据

-- +goose Up
-- +goose StatementBegin

-- 重跑分类管理权限回填：修复在无分类功能的版本系（如 yun）上创建的工作空间，
-- 其系统预设项目管理员缺少 requirement:manage_category 的问题。
-- 幂等插入，普通成员和自定义角色不自动授予。
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, 'requirement:manage_category',
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
WHERE r.scope = 'project'
  AND r.is_system = 1
  AND r.name IN ('perm_role_project_admin', '${perm_role_project_admin}')
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 不回滚权限：无法区分本迁移插入与原迁移插入的记录，删除会破坏既有授权。
SELECT 1;

-- +goose StatementEnd

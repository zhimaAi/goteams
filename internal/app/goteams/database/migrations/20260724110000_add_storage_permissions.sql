-- +goose Up
-- 给所有已存在的企业管理员角色补入存储空间三个权限（storage:view / storage:download / storage:delete）
-- 幂等：ON CONFLICT 忽略重复插入。

INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, 'storage:view',    EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
WHERE r.scope = 'enterprise'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_enterprise_admin%'
  AND NOT EXISTS (
      SELECT 1 FROM role_permissions rp
      WHERE rp.admin_id = r.admin_id AND rp.role_id = r.id AND rp.permission_key = 'storage:view'
  );

INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, 'storage:download', EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
WHERE r.scope = 'enterprise'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_enterprise_admin%'
  AND NOT EXISTS (
      SELECT 1 FROM role_permissions rp
      WHERE rp.admin_id = r.admin_id AND rp.role_id = r.id AND rp.permission_key = 'storage:download'
  );

INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, 'storage:delete',  EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
WHERE r.scope = 'enterprise'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_enterprise_admin%'
  AND NOT EXISTS (
      SELECT 1 FROM role_permissions rp
      WHERE rp.admin_id = r.admin_id AND rp.role_id = r.id AND rp.permission_key = 'storage:delete'
  );

-- +goose Down
-- 不需要回滚（权限定义变更无法反向推断原有状态），留空即可。

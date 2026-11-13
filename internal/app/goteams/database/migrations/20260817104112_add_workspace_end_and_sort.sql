-- +goose Up
-- 项目结束状态与排序值：结束仅隐藏项目列表入口，删除改为物理删除
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS ended_at BIGINT NOT NULL DEFAULT 0;
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS sort_order INTEGER NOT NULL DEFAULT 0;

COMMENT ON COLUMN "workspaces"."ended_at" IS '项目结束时间（Unix时间戳）: 0=进行中, >0=已结束';
COMMENT ON COLUMN "workspaces"."sort_order" IS '项目排序值: 升序排列，相同排序值按ID倒序';
COMMENT ON COLUMN "workspaces"."archived" IS '【已废弃】归档状态: 0=正常, 1=已归档；仅保留历史软删除数据的过滤能力，新逻辑结束项目用 ended_at、删除项目为物理删除';

-- 项目级"删除项目"权限改名为"结束项目"，删除项目已收归企业级 project:manage 权限
UPDATE role_permissions SET permission_key = 'workspace:end', updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE permission_key = 'workspace:delete';

-- 存量租户的企业管理员补种"管理项目"权限（幂等，重复执行不会产生重复数据）
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, 'project:manage', EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
WHERE r.scope = 'enterprise'
  AND r.is_system = 1
  AND r.name = '${perm_role_enterprise_admin}'
  AND NOT EXISTS (
    SELECT 1 FROM role_permissions rp
    WHERE rp.admin_id = r.admin_id AND rp.role_id = r.id AND rp.permission_key = 'project:manage'
  );

-- +goose Down
UPDATE role_permissions SET permission_key = 'workspace:delete' WHERE permission_key = 'workspace:end';
DELETE FROM role_permissions WHERE permission_key = 'project:manage';

COMMENT ON COLUMN "workspaces"."archived" IS '归档状态: 0=正常, 1=已归档';
ALTER TABLE workspaces DROP COLUMN IF EXISTS sort_order;
ALTER TABLE workspaces DROP COLUMN IF EXISTS ended_at;

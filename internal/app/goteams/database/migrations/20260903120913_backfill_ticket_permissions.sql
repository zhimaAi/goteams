-- +goose Up
-- +goose StatementBegin

-- ticket:* 权限（邀填工单：复制链接/管理）在邀填工单功能上线时才加入 permission.go 的 PresetRoles，
-- 而系统预设项目角色只在创建项目时按 PresetRoles 种子一次，
-- 存量项目的系统预设「项目管理员」角色缺少这两条 role_permissions，
-- 导致邀填工单相关接口（/api/workspaces/:wsId/issue-invite/*）按 RBAC 判定时项目管理员被 403，
-- 且角色权限编辑页「邀填工单 → 复制链接/管理」未勾选。此处一次性全量回填补齐。
-- （新建项目由 permission.go PresetRoles 种子负责，无需回填。）

INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, v.permission_key,
       EXTRACT(EPOCH FROM NOW())::BIGINT,
       EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
CROSS JOIN (VALUES ('ticket:copy_link'), ('ticket:manage')) AS v(permission_key)
WHERE r.scope = 'project'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_project_admin%'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 仅反向删除本迁移目标范围（各项目系统预设项目管理员角色）内的这两条权限，
-- 不影响项目成员角色、自定义角色及其他权限行。
DELETE FROM role_permissions rp
USING roles r
WHERE rp.role_id = r.id
  AND rp.admin_id = r.admin_id
  AND rp.permission_key IN ('ticket:copy_link', 'ticket:manage')
  AND r.scope = 'project'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_project_admin%';

-- +goose StatementEnd

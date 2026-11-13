-- +goose Up
-- +goose StatementBegin

-- 为所有已有项目种入 defect 应用开关，默认启用。
-- 缺陷此前是项目内置能力、workspace_applications 中没有对应记录；接入应用市场后
-- “无记录 = 未启用”，若不补种，存量项目的缺陷入口（应用市场/项目设置/顶部 Tab）会全部消失。
-- ON CONFLICT DO NOTHING 保证幂等：用户已手动配置（开启或关闭）的记录不会被覆盖。
-- 注意：仅插入 enabled=1 记录，enabled_by=0 表示系统种入而非用户操作。

INSERT INTO workspace_applications (admin_id, workspace_id, app_key, enabled, enabled_by, enabled_at, updated_at)
SELECT admin_id, id, 'defect', 1, 0,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces
ON CONFLICT (admin_id, workspace_id, app_key) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 回滚不删除任何 defect 记录：用户在 Up 之后可能手动调整过开关，
-- 直接删除会丢失这些配置。Down 留空，保证数据安全。

-- +goose StatementEnd

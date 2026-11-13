-- +goose Up
-- +goose StatementBegin

-- 为所有已有项目种入 planning / iteration 应用开关，默认启用。
-- 这两个应用按需求"都默认开启"，避免老项目因缺少记录而导致项目顶部 Tab 消失。
-- ON CONFLICT DO NOTHING 保证幂等：用户已手动配置（开启或关闭）的记录不会被覆盖。
-- 注意：仅插入 enabled=1 记录，enabled_by=0 表示系统种入而非用户操作。

INSERT INTO workspace_applications (admin_id, workspace_id, app_key, enabled, enabled_by, enabled_at, updated_at)
SELECT admin_id, id, app.app_key, 1, 0,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces w
CROSS JOIN (VALUES ('planning'), ('iteration')) AS app(app_key)
ON CONFLICT (admin_id, workspace_id, app_key) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 回滚不删除任何 planning/iteration 记录：用户在 Up 之后可能手动调整过开关，
-- 直接删除会丢失这些配置。Down 留空，保证数据安全。

-- +goose StatementEnd

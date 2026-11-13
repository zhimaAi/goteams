-- +goose Up
-- +goose StatementBegin
ALTER TABLE workspace_dashboards
    DROP CONSTRAINT IF EXISTS workspace_dashboards_visibility_type_check;
ALTER TABLE workspace_dashboards
    ADD CONSTRAINT workspace_dashboards_visibility_type_check
    CHECK (visibility_type IN ('public', 'self', 'roles', 'members'));
COMMENT ON COLUMN workspace_dashboards.visibility_type IS '可见范围: public=公开, self=仅自己, roles=部分角色, members=部分成员';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 存在仅自己可见的仪表盘时拒绝回滚，避免放宽其可见范围。
ALTER TABLE workspace_dashboards
    DROP CONSTRAINT IF EXISTS workspace_dashboards_visibility_type_check;
ALTER TABLE workspace_dashboards
    ADD CONSTRAINT workspace_dashboards_visibility_type_check
    CHECK (visibility_type IN ('public', 'roles', 'members'));
COMMENT ON COLUMN workspace_dashboards.visibility_type IS '可见范围: public=公开, roles=部分角色, members=部分成员';
-- +goose StatementEnd

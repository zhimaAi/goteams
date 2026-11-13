-- +goose Up
-- +goose StatementBegin

ALTER TABLE user_view_settings
    ADD COLUMN IF NOT EXISTS search_expanded SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS search_filters TEXT NOT NULL DEFAULT '{"logic":"and","conditions":[{"field":"title","operator":"contains","values":[]},{"field":"status","operator":"in","values":[]},{"field":"assignee","operator":"in","values":[]},{"field":"version","operator":"in","values":[]},{"field":"creator","operator":"in","values":[]}]}';

COMMENT ON COLUMN "user_view_settings"."search_expanded" IS '搜索条件条展开状态: 1=展开, 0=收起';
COMMENT ON COLUMN "user_view_settings"."search_filters" IS '用户自定义搜索字段顺序和值JSON';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE user_view_settings
    DROP COLUMN IF EXISTS search_filters,
    DROP COLUMN IF EXISTS search_expanded;

-- +goose StatementEnd

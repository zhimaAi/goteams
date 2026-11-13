-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- 规划行归档标记与标题删除线：仅 type=planning 使用
-- 需求/缺陷恒为 0，不影响既有查询
-- ============================================================

ALTER TABLE requirements ADD COLUMN IF NOT EXISTS is_archived SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE requirements ADD COLUMN IF NOT EXISTS title_strike SMALLINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN "requirements"."is_archived" IS '归档状态（仅规划使用，0=未归档，1=已归档）';
COMMENT ON COLUMN "requirements"."title_strike" IS '标题删除线（仅规划使用，0=无，1=有，展示为删除线+浅灰）';

-- 归档过滤沿用既有索引 idx_req_planning_order（admin_id, workspace_id, group_id, sort_order, id）
-- 单个分组的规划行数量有限，is_archived 作为回表过滤条件即可，不额外建索引

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE requirements DROP COLUMN IF EXISTS title_strike;
ALTER TABLE requirements DROP COLUMN IF EXISTS is_archived;

-- +goose StatementEnd

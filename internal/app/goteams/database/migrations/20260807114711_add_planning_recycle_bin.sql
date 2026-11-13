-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- 规划回收站：规划行与规划分组改为软删除
-- 需求/缺陷不走软删除，deleted_at 恒为 0，不影响既有查询
-- ============================================================

ALTER TABLE requirements ADD COLUMN IF NOT EXISTS deleted_at BIGINT NOT NULL DEFAULT 0;
ALTER TABLE requirements ADD COLUMN IF NOT EXISTS deleted_by INTEGER NOT NULL DEFAULT 0;

COMMENT ON COLUMN "requirements"."deleted_at" IS '软删除时间戳（仅规划使用，0=未删除）';
COMMENT ON COLUMN "requirements"."deleted_by" IS '删除人用户ID（仅规划使用，0=未删除）';

-- 分组同样软删除：还原规划时若原分组已删除，直接复活该分组即可，无需另存分组名
ALTER TABLE planning_groups ADD COLUMN IF NOT EXISTS deleted_at BIGINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN "planning_groups"."deleted_at" IS '软删除时间戳（0=未删除）';

-- 回收站列表按删除时间倒序，部分索引只收录已删除的规划行
CREATE INDEX IF NOT EXISTS idx_req_planning_deleted
    ON requirements(admin_id, workspace_id, deleted_at DESC, id)
    WHERE type = 'planning' AND deleted_at > 0;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_req_planning_deleted;
ALTER TABLE planning_groups DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE requirements DROP COLUMN IF EXISTS deleted_by;
ALTER TABLE requirements DROP COLUMN IF EXISTS deleted_at;

-- +goose StatementEnd

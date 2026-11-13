-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- 需求访问记录：支撑「我的工作 - 最近访问」tab
-- 每人每个工作项只保留一行，重复访问只更新 visited_at，不记流水
-- ============================================================

CREATE TABLE IF NOT EXISTS requirement_visits (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    user_id INTEGER NOT NULL,
    requirement_id INTEGER NOT NULL,
    visited_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE "requirement_visits" IS '需求访问记录表';
COMMENT ON COLUMN "requirement_visits"."id" IS '记录ID';
COMMENT ON COLUMN "requirement_visits"."admin_id" IS '租户ID';
COMMENT ON COLUMN "requirement_visits"."user_id" IS '访问人用户ID';
COMMENT ON COLUMN "requirement_visits"."requirement_id" IS '被访问的工作项ID';
COMMENT ON COLUMN "requirement_visits"."visited_at" IS '最近一次访问时间戳';

-- upsert 的冲突目标：同一人同一工作项只有一行
CREATE UNIQUE INDEX IF NOT EXISTS idx_req_visits_user_item
    ON requirement_visits(admin_id, user_id, requirement_id);

-- 列表按访问时间倒序取最近一周，走该索引避免回表排序
CREATE INDEX IF NOT EXISTS idx_req_visits_user_time
    ON requirement_visits(admin_id, user_id, visited_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_req_visits_user_time;
DROP INDEX IF EXISTS idx_req_visits_user_item;
DROP TABLE IF EXISTS requirement_visits;

-- +goose StatementEnd

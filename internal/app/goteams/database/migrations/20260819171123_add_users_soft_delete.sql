-- +goose Up
-- 企业成员移除改为软删除：保留 users 行供历史工作项展示名称与头像
ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at BIGINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN "users"."deleted_at" IS '删除时间（Unix时间戳），0=未删除；移除成员时置为当前时间并清空 username 释放账号唯一性';

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS deleted_at;

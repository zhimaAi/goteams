-- +goose Up
-- +goose StatementBegin

-- API token：个人设置里创建的凭据，供 AI Agent 通过开放 API 调用。
-- token_hash 存 sha256(明文)，明文只在创建时返回一次，永不落库。
CREATE TABLE IF NOT EXISTS api_tokens (
    id               SERIAL PRIMARY KEY,
    admin_id         INTEGER NOT NULL DEFAULT 0,
    user_id          INTEGER NOT NULL,
    name             VARCHAR(100) NOT NULL DEFAULT '',
    token_prefix     VARCHAR(16)  NOT NULL DEFAULT '',
    token_hash       CHAR(64)     NOT NULL,
    permissions      TEXT[]       NOT NULL DEFAULT '{}',
    enabled          SMALLINT     NOT NULL DEFAULT 1,
    cumulative_calls BIGINT       NOT NULL DEFAULT 0,
    last_used_at     BIGINT       NOT NULL DEFAULT 0,
    created_at       BIGINT       NOT NULL DEFAULT 0,
    updated_at       BIGINT       NOT NULL DEFAULT 0
);

COMMENT ON TABLE "api_tokens" IS 'API token 表（用户级，开放 API 调用凭据）';
COMMENT ON COLUMN "api_tokens"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "api_tokens"."user_id" IS '创建者用户ID';
COMMENT ON COLUMN "api_tokens"."name" IS '备注（界面输入上限100字符）';
COMMENT ON COLUMN "api_tokens"."token_prefix" IS '明文前缀，仅用于列表展示';
COMMENT ON COLUMN "api_tokens"."token_hash" IS 'sha256(明文 token) 的 hex，唯一';
COMMENT ON COLUMN "api_tokens"."permissions" IS '已授权权限点数组，取值见 define.AllApiTokenPermissions';
COMMENT ON COLUMN "api_tokens"."enabled" IS '启停: 1=启用, 0=停用';
COMMENT ON COLUMN "api_tokens"."cumulative_calls" IS '累计触发次数（缓存，明细见 api_token_daily_stats）';
COMMENT ON COLUMN "api_tokens"."last_used_at" IS '最近一次成功调用时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_api_tokens_admin_user ON api_tokens(admin_id, user_id);
-- 鉴权时只有 token_hash 可用（admin_id 未知），必须能按 hash 单点命中。
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_tokens_hash ON api_tokens(token_hash);

-- 租户每日 API 调用上限（权益点：体验版 100 次/天，企业版 100000 次/天）。
ALTER TABLE tenant_plans
    ADD COLUMN IF NOT EXISTS api_call_daily_limit INTEGER NOT NULL DEFAULT 100;
COMMENT ON COLUMN "tenant_plans"."api_call_daily_limit" IS '每日 API 调用上限（次）';

-- 按租户 × 天 的调用计数，用于限额硬卡；day 为 Asia/Shanghai 的 YYYYMMDD。
CREATE TABLE IF NOT EXISTS api_admin_daily_quotas (
    admin_id   INTEGER NOT NULL,
    day        INTEGER NOT NULL,
    used_calls INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (admin_id, day)
);

COMMENT ON TABLE "api_admin_daily_quotas" IS '租户每日 API 调用计数（限额硬卡）';
COMMENT ON COLUMN "api_admin_daily_quotas"."day" IS '日期，Asia/Shanghai 的 YYYYMMDD';

-- 按 token × 天 的调用明细，用于列表「今日/昨日触发」展示。
CREATE TABLE IF NOT EXISTS api_token_daily_stats (
    token_id INTEGER NOT NULL,
    day      INTEGER NOT NULL,
    calls    INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (token_id, day)
);

COMMENT ON TABLE "api_token_daily_stats" IS 'API token 每日调用明细（今日/昨日触发展示）';
COMMENT ON COLUMN "api_token_daily_stats"."day" IS '日期，Asia/Shanghai 的 YYYYMMDD';

CREATE INDEX IF NOT EXISTS idx_api_token_daily_stats_day ON api_token_daily_stats(day);

-- 存量租户按套餐补齐上限：企业版 100000 次/天，其余保持体验版默认的 100。
UPDATE tenant_plans
SET api_call_daily_limit = 100000,
    updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE plan_type = 'enterprise';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_api_token_daily_stats_day;
DROP TABLE IF EXISTS api_token_daily_stats;
DROP TABLE IF EXISTS api_admin_daily_quotas;
ALTER TABLE tenant_plans DROP COLUMN IF EXISTS api_call_daily_limit;
DROP INDEX IF EXISTS idx_api_tokens_admin_user;
DROP INDEX IF EXISTS uq_api_tokens_hash;
DROP TABLE IF EXISTS api_tokens;

-- +goose StatementEnd

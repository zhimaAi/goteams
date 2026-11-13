-- +goose Up
-- +goose StatementBegin

-- 第三方账号绑定表：记录 GoTeams 账号与钉钉/飞书个人账号的绑定关系，支撑扫码登录与个人设置里的绑定管理。
-- 只存身份标识与展示信息，不存任何第三方令牌（access_token 用完即弃，不落库）。
CREATE TABLE IF NOT EXISTS user_oauth_bindings (
    id         SERIAL PRIMARY KEY,
    admin_id   INTEGER      NOT NULL DEFAULT 0,
    user_id    INTEGER      NOT NULL,
    provider   VARCHAR(20)  NOT NULL,
    app_key    VARCHAR(100) NOT NULL DEFAULT '',
    open_id    VARCHAR(64)  NOT NULL DEFAULT '',
    union_id   VARCHAR(64)  NOT NULL,
    nickname   VARCHAR(100) NOT NULL DEFAULT '',
    avatar     VARCHAR(500) NOT NULL DEFAULT '',
    created_at BIGINT       NOT NULL DEFAULT 0,
    updated_at BIGINT       NOT NULL DEFAULT 0
);

COMMENT ON TABLE "user_oauth_bindings" IS '第三方账号绑定表（钉钉/飞书扫码登录）';
COMMENT ON COLUMN "user_oauth_bindings"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "user_oauth_bindings"."user_id" IS '绑定的 GoTeams 用户ID';
COMMENT ON COLUMN "user_oauth_bindings"."provider" IS '第三方平台: dingtalk=钉钉, feishu=飞书';
COMMENT ON COLUMN "user_oauth_bindings"."app_key" IS '绑定时所用的第三方应用标识（钉钉 AppKey / 飞书 App ID）；换应用后旧绑定自动失效';
COMMENT ON COLUMN "user_oauth_bindings"."open_id" IS '第三方应用内用户标识（钉钉 openId / 飞书 open_id），同应用内唯一';
COMMENT ON COLUMN "user_oauth_bindings"."union_id" IS '第三方开发者账号内用户唯一标识（钉钉 unionId / 飞书 union_id），扫码登录据此定位账号';
COMMENT ON COLUMN "user_oauth_bindings"."nickname" IS '绑定时取到的第三方昵称，仅用于个人设置展示';
COMMENT ON COLUMN "user_oauth_bindings"."avatar" IS '绑定时取到的第三方头像URL，仅用于个人设置展示';
COMMENT ON COLUMN "user_oauth_bindings"."created_at" IS '绑定时间（Unix时间戳）';
COMMENT ON COLUMN "user_oauth_bindings"."updated_at" IS '更新时间（Unix时间戳）';

-- 扫码登录按 (provider, app_key, union_id) 单点命中；同一应用内一个第三方账号只允许绑定一个 GoTeams 账号，靠唯一索引兜底并发。
-- 带上 app_key：换过应用后旧行不再命中，等于自动失效。
CREATE UNIQUE INDEX IF NOT EXISTS uq_user_oauth_bindings_provider_app_union ON user_oauth_bindings(provider, app_key, union_id);
-- 一个 GoTeams 账号在同一应用的同一平台只允许一个绑定（个人设置按 provider 展示绑定/解绑）；换应用后旧行不再挡路，可重新绑定。
CREATE UNIQUE INDEX IF NOT EXISTS uq_user_oauth_bindings_user_provider_app ON user_oauth_bindings(user_id, provider, app_key);
-- 个人设置读取当前用户的绑定列表。
CREATE INDEX IF NOT EXISTS idx_user_oauth_bindings_admin_user ON user_oauth_bindings(admin_id, user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_user_oauth_bindings_admin_user;
DROP INDEX IF EXISTS uq_user_oauth_bindings_user_provider_app;
DROP INDEX IF EXISTS uq_user_oauth_bindings_provider_app_union;
DROP TABLE IF EXISTS user_oauth_bindings;

-- +goose StatementEnd

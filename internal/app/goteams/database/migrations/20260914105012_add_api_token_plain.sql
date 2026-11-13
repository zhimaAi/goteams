-- +goose Up
-- +goose StatementBegin

-- 明文 Key 需在列表中再次展示并可打包进 skill 安装包，因此明文随明文列入库。
-- 鉴权仍按 token_hash 单点命中，token_plain 只服务展示与下载。
-- 存量行为空：升级前创建的 token 明文不可还原，界面退化为展示 token_prefix，下载 skill 会明确报错。
ALTER TABLE api_tokens
    ADD COLUMN IF NOT EXISTS token_plain VARCHAR(64) NOT NULL DEFAULT '';

COMMENT ON COLUMN "api_tokens"."token_plain" IS 'Token 明文（仅用于界面展示与 skill 打包，鉴权走 token_hash）';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE api_tokens DROP COLUMN IF EXISTS token_plain;

-- +goose StatementEnd

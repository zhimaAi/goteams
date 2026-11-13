-- +goose Up
-- +goose StatementBegin

-- 需求 1896 优化点3：会话日志改为 OSS 对象存储 + 信封加密。
-- pipeline_task_sessions 增加三列：
--   conversation_oss_key     会话内容（密文）在 OSS/本地的对象 key；每步一对象。
--   conversation_dek_wrapped 用租户 KEK 包裹的 DEK（AES-256-GCM，nonce||密文，base64），TEXT。
--   conversation_enc         加密算法标记：plain=未加密（未配置主密钥时降级），aes-256-gcm=信封加密。
-- conversation_json 列本期保留（历史数据），但停止写入；读取以 OSS 对象为准。

ALTER TABLE pipeline_task_sessions
    ADD COLUMN IF NOT EXISTS conversation_oss_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS conversation_dek_wrapped TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS conversation_enc VARCHAR(16) NOT NULL DEFAULT '';

COMMENT ON COLUMN pipeline_task_sessions.conversation_oss_key IS '会话内容对象 key（OSS/本地存储），每步一对象，重传覆盖同 key';
COMMENT ON COLUMN pipeline_task_sessions.conversation_dek_wrapped IS '信封加密的 DEK（被租户 KEK 包裹，base64），plain 模式为空';
COMMENT ON COLUMN pipeline_task_sessions.conversation_enc IS '会话内容加密算法：plain=未加密，aes-256-gcm=信封加密';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE pipeline_task_sessions
    DROP COLUMN IF EXISTS conversation_oss_key,
    DROP COLUMN IF EXISTS conversation_dek_wrapped,
    DROP COLUMN IF EXISTS conversation_enc;
-- +goose StatementEnd

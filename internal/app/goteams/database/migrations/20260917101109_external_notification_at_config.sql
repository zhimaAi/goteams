-- +goose Up

-- 外部系统通知策略的 @ 人配置，语义与自动化 webhook 动作的 at_* 字段保持一致。
ALTER TABLE external_notification_strategies
    ADD COLUMN IF NOT EXISTS at_mode VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS at_user_ids JSONB NOT NULL DEFAULT '[]'::JSONB,
    ADD COLUMN IF NOT EXISTS at_roles JSONB NOT NULL DEFAULT '[]'::JSONB,
    ADD COLUMN IF NOT EXISTS at_all BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN external_notification_strategies.at_mode IS '机器人 @ 参数：mobile/at_mobiles/at_user_ids/user_id/mentioned_list/mentioned_mobile_list';
COMMENT ON COLUMN external_notification_strategies.at_user_ids IS '显式 @ 的成员 id 列表';
COMMENT ON COLUMN external_notification_strategies.at_roles IS '按触发角色 @：created_by 或 assignee';
COMMENT ON COLUMN external_notification_strategies.at_all IS '是否 @ 全体';

-- +goose Down

ALTER TABLE external_notification_strategies
    DROP COLUMN IF EXISTS at_all,
    DROP COLUMN IF EXISTS at_roles,
    DROP COLUMN IF EXISTS at_user_ids,
    DROP COLUMN IF EXISTS at_mode;

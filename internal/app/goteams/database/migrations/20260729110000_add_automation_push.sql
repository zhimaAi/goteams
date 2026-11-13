-- +goose Up

-- 机器人 @ 人所需的成员参数，列名直接沿用各机器人 webhook 的字段名，由参数映射统一维护。
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS push_dingtalk_at_mobiles VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS push_dingtalk_at_user_ids VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS push_feishu_user_id VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS push_wecom_mentioned_list VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS push_wecom_mentioned_mobile_list VARCHAR(100) NOT NULL DEFAULT '';

COMMENT ON COLUMN users.push_dingtalk_at_mobiles IS '钉钉机器人 at.atMobiles 取值';
COMMENT ON COLUMN users.push_dingtalk_at_user_ids IS '钉钉机器人 at.atUserIds 取值';
COMMENT ON COLUMN users.push_feishu_user_id IS '飞书机器人 at 标签 user_id 取值';
COMMENT ON COLUMN users.push_wecom_mentioned_list IS '企微群 text.mentioned_list 取值';
COMMENT ON COLUMN users.push_wecom_mentioned_mobile_list IS '企微群 text.mentioned_mobile_list 取值';

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION pg_temp.automation_action_to_push_system(action JSONB) RETURNS JSONB AS $$
BEGIN
    IF action IS NULL OR action->>'type' <> 'webhook' THEN
        RETURN action;
    END IF;
    RETURN (action - 'system_name') || jsonb_build_object('push_system', 'custom');
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION pg_temp.automation_config_to_push_system(config JSONB) RETURNS JSONB AS $$
DECLARE
    node JSONB;
    nodes JSONB := '[]'::JSONB;
BEGIN
    IF config IS NULL OR config->'nodes' IS NULL THEN
        RETURN config;
    END IF;
    FOR node IN SELECT * FROM jsonb_array_elements(config->'nodes') LOOP
        IF node ? 'action' THEN
            node := jsonb_set(node, '{action}', pg_temp.automation_action_to_push_system(node->'action'));
        END IF;
        IF node ? 'actions' THEN
            node := jsonb_set(node, '{actions}', COALESCE(
                (SELECT jsonb_agg(pg_temp.automation_action_to_push_system(item)) FROM jsonb_array_elements(node->'actions') item),
                '[]'::JSONB));
        END IF;
        nodes := nodes || jsonb_build_array(node);
    END LOOP;
    RETURN jsonb_set(config, '{nodes}', nodes);
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- 存量 webhook 动作全部是自定义推送，去掉第三方系统名称并补齐推送渠道；配置解析拒绝未知字段，必须转换。
UPDATE automation_rules SET config_json = pg_temp.automation_config_to_push_system(config_json)
WHERE config_json::TEXT LIKE '%"system_name"%';

UPDATE automation_executions SET config_snapshot = pg_temp.automation_config_to_push_system(config_snapshot)
WHERE config_snapshot::TEXT LIKE '%"system_name"%';

-- +goose Down

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION pg_temp.automation_action_to_system_name(action JSONB) RETURNS JSONB AS $$
BEGIN
    IF action IS NULL OR action->>'type' <> 'webhook' THEN
        RETURN action;
    END IF;
    -- 原始的第三方系统名称无法恢复，回退时统一写入占位值。
    RETURN (action - 'push_system' - 'secret' - 'template' - 'at_user_ids' - 'at_roles' - 'at_all' - 'at_mode')
        || jsonb_build_object('system_name', 'webhook');
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION pg_temp.automation_config_to_system_name(config JSONB) RETURNS JSONB AS $$
DECLARE
    node JSONB;
    nodes JSONB := '[]'::JSONB;
BEGIN
    IF config IS NULL OR config->'nodes' IS NULL THEN
        RETURN config;
    END IF;
    FOR node IN SELECT * FROM jsonb_array_elements(config->'nodes') LOOP
        IF node ? 'action' THEN
            node := jsonb_set(node, '{action}', pg_temp.automation_action_to_system_name(node->'action'));
        END IF;
        IF node ? 'actions' THEN
            node := jsonb_set(node, '{actions}', COALESCE(
                (SELECT jsonb_agg(pg_temp.automation_action_to_system_name(item)) FROM jsonb_array_elements(node->'actions') item),
                '[]'::JSONB));
        END IF;
        nodes := nodes || jsonb_build_array(node);
    END LOOP;
    RETURN jsonb_set(config, '{nodes}', nodes);
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

UPDATE automation_rules SET config_json = pg_temp.automation_config_to_system_name(config_json)
WHERE config_json::TEXT LIKE '%"push_system"%';

UPDATE automation_executions SET config_snapshot = pg_temp.automation_config_to_system_name(config_snapshot)
WHERE config_snapshot::TEXT LIKE '%"push_system"%';

ALTER TABLE users
    DROP COLUMN IF EXISTS push_wecom_mentioned_mobile_list,
    DROP COLUMN IF EXISTS push_wecom_mentioned_list,
    DROP COLUMN IF EXISTS push_feishu_user_id,
    DROP COLUMN IF EXISTS push_dingtalk_at_user_ids,
    DROP COLUMN IF EXISTS push_dingtalk_at_mobiles;

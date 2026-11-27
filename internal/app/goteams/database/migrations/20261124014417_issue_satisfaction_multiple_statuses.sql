-- +goose Up

ALTER TABLE workspace_issue_satisfaction_configs
    ADD COLUMN auto_status_ids INTEGER[] NOT NULL DEFAULT ARRAY[]::INTEGER[];

UPDATE workspace_issue_satisfaction_configs
SET auto_status_ids = CASE WHEN auto_status_id > 0
    THEN ARRAY[auto_status_id] ELSE ARRAY[]::INTEGER[] END;

ALTER TABLE workspace_issue_satisfaction_configs
    DROP CONSTRAINT ck_issue_sat_creator_kinds,
    DROP CONSTRAINT ck_issue_sat_enabled_config,
    DROP COLUMN auto_status_id,
    ADD CONSTRAINT ck_issue_sat_status_ids CHECK (
        array_position(auto_status_ids, NULL) IS NULL
        AND 0 < ALL(auto_status_ids)
    ),
    ADD CONSTRAINT ck_issue_sat_creator_kinds CHECK (
        auto_creator_kinds <@ ARRAY[1, 2]::SMALLINT[]
        AND array_position(auto_creator_kinds, NULL) IS NULL
        AND (cardinality(auto_status_ids) = 0 OR cardinality(auto_creator_kinds) > 0)
    ),
    ADD CONSTRAINT ck_issue_sat_enabled_config CHECK (
        enabled = 0 OR manual_enabled = 1 OR cardinality(auto_status_ids) > 0
    );

COMMENT ON COLUMN workspace_issue_satisfaction_configs.auto_status_ids
    IS '自动开放评价的目标状态集合，命中任一状态即开放；空数组表示尚未配置';

-- +goose Down
-- 多个状态无法无损还原为单个状态，保留新配置及全部评价结果；应用与迁移需协同发布。

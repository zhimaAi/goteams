-- +goose Up
ALTER TABLE requirements
    ADD COLUMN IF NOT EXISTS started_at BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS ended_at BIGINT NOT NULL DEFAULT 0;
COMMENT ON COLUMN requirements.started_at IS '首次进入配置开始状态的时间，0为未记录';
COMMENT ON COLUMN requirements.ended_at IS '最近一次进入配置结束状态的时间，0为未记录';

ALTER TABLE workspaces_settings
    ADD COLUMN IF NOT EXISTS requirement_time_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS requirement_start_status_id INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS requirement_end_status_id INTEGER NOT NULL DEFAULT 0;
COMMENT ON COLUMN workspaces_settings.requirement_time_enabled IS '是否自动记录需求和工单开始结束时间';
COMMENT ON COLUMN workspaces_settings.requirement_start_status_id IS '首次开始时间的目标状态ID';
COMMENT ON COLUMN workspaces_settings.requirement_end_status_id IS '最近结束时间的目标状态ID';

-- 存量排序可能已被用户调整，新增字段追加到现有内置与自定义字段之后，不重排已有字段。
INSERT INTO workspace_field_configs
    (admin_id,workspace_id,type_key,field_key,is_enabled,is_editable,is_locked,sort_order,created_at,updated_at)
SELECT w.admin_id,w.id,'requirement',f.key,0,0,0,
    GREATEST(COALESCE((SELECT MAX(c.sort_order) FROM workspace_field_configs c WHERE c.admin_id=w.admin_id AND c.workspace_id=w.id AND c.type_key='requirement'),-1),
             COALESCE((SELECT MAX(c.sort_order) FROM workspace_custom_fields c WHERE c.admin_id=w.admin_id AND c.workspace_id=w.id AND c.type_key='requirement'),-1))+f.step,
    EXTRACT(EPOCH FROM NOW())::BIGINT,EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces w CROSS JOIN (VALUES ('started_at',1),('ended_at',2)) f(key,step)
ON CONFLICT (admin_id,workspace_id,type_key,field_key) DO NOTHING;

-- +goose Down
-- 时间值属于不可重建的历史事实，回退不删除字段和配置，避免误删线上数据。
SELECT 1;

-- +goose Up
ALTER TABLE requirements ADD COLUMN IF NOT EXISTS responsible_ids INTEGER[] NOT NULL DEFAULT '{}';
COMMENT ON COLUMN requirements.responsible_ids IS '缺陷责任人用户 ID 列表，空数组表示未指定';

INSERT INTO workspace_field_configs
    (admin_id, workspace_id, type_key, field_key, is_enabled, is_editable, is_locked, sort_order, created_at, updated_at)
SELECT w.admin_id, w.id, 'defect', 'responsible_ids', 1, 1, 0,
       COALESCE(current_order.max_order, -1) + 1,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces w
LEFT JOIN LATERAL (
    SELECT MAX(sort_order) AS max_order
    FROM (
        SELECT sort_order FROM workspace_field_configs
        WHERE admin_id = w.admin_id AND workspace_id = w.id AND type_key = 'defect'
        UNION ALL
        SELECT sort_order FROM workspace_custom_fields
        WHERE admin_id = w.admin_id AND workspace_id = w.id AND type_key = 'defect'
    ) ordered_fields
) current_order ON TRUE
ON CONFLICT (admin_id, workspace_id, type_key, field_key) DO NOTHING;

-- +goose Down
-- 保留已经录入的责任人和用户调整过的字段配置，以便重新升级时恢复。
SELECT 1;

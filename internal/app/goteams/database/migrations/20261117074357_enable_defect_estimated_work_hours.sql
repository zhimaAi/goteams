-- +goose Up
-- 缺陷复用工作项已有的预估工时列，仅补齐字段配置。
INSERT INTO workspace_field_configs
    (admin_id, workspace_id, type_key, field_key, is_enabled, is_editable, is_locked, sort_order, created_at, updated_at)
SELECT w.admin_id, w.id, 'defect', 'estimated_work_hours', 1, 1, 0,
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
WHERE w.workspace_type <> 'issue'
ON CONFLICT (admin_id, workspace_id, type_key, field_key) DO NOTHING;

-- +goose Down
-- 保留用户可能修改过的字段配置及已录入的预估工时。
SELECT 1;

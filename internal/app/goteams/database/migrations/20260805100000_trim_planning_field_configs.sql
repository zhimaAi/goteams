-- +goose Up
-- 规划只保留产品定义的 7 个默认字段，删除默认关闭且无处展示的 ID/创建时间/创建人。
DELETE FROM workspace_field_configs
WHERE type_key = 'planning'
  AND field_key IN ('id', 'created_at', 'created_by');

-- 规划的状态与需求/缺陷对齐，锁定不可关闭。
UPDATE workspace_field_configs
SET is_locked = 1, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE type_key = 'planning' AND field_key = 'status' AND is_locked <> 1;

-- 状态被锁定后必须保持启用。
UPDATE workspace_field_configs
SET is_enabled = 1, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE type_key = 'planning' AND field_key = 'status' AND is_enabled <> 1;

-- +goose Down
UPDATE workspace_field_configs
SET is_locked = 0, updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE type_key = 'planning' AND field_key = 'status';

INSERT INTO workspace_field_configs (admin_id, workspace_id, type_key, field_key, is_enabled, is_editable, is_locked, sort_order, created_at, updated_at)
SELECT w.admin_id, w.id, 'planning', v.field_key, 0, 0, 0, v.sort_order,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces w
CROSS JOIN (VALUES
    ('id',         7),
    ('created_at', 8),
    ('created_by', 9)
) AS v(field_key, sort_order)
ON CONFLICT (admin_id, workspace_id, type_key, field_key) DO NOTHING;

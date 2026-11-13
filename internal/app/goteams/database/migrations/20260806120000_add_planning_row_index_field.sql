-- +goose Up
-- +goose StatementBegin

-- 规划列表的「序号」列纳入项目设置的字段管理，并锁定为不可关闭。
-- 仅对尚未包含 row_index 的工作空间整体后移一位，为 sort_order = 0 的序号列让位，
-- 这样即使本迁移被重复执行也不会把排序越推越大。
UPDATE workspace_field_configs c
SET sort_order = c.sort_order + 1,
    updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE c.type_key = 'planning'
  AND NOT EXISTS (
      SELECT 1 FROM workspace_field_configs e
      WHERE e.admin_id = c.admin_id
        AND e.workspace_id = c.workspace_id
        AND e.type_key = 'planning'
        AND e.field_key = 'row_index'
  );

INSERT INTO workspace_field_configs
    (admin_id, workspace_id, type_key, field_key, is_enabled, is_editable, is_locked, sort_order, created_at, updated_at)
SELECT DISTINCT c.admin_id, c.workspace_id, 'planning', 'row_index', 1, 0, 1, 0,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspace_field_configs c
WHERE c.type_key = 'planning'
ON CONFLICT (admin_id, workspace_id, type_key, field_key) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DELETE FROM workspace_field_configs
WHERE type_key = 'planning' AND field_key = 'row_index';

UPDATE workspace_field_configs
SET sort_order = sort_order - 1,
    updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE type_key = 'planning' AND sort_order > 0;

-- +goose StatementEnd

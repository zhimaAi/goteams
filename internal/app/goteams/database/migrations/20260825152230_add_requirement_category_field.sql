-- +goose Up
-- +goose StatementBegin
UPDATE workspace_field_configs
SET sort_order = sort_order + 1
WHERE type_key = 'requirement' AND sort_order > 3;

UPDATE workspace_custom_fields
SET sort_order = sort_order + 1
WHERE type_key = 'requirement' AND sort_order > 3;

INSERT INTO workspace_field_configs
    (admin_id, workspace_id, type_key, field_key, is_enabled, is_editable, is_locked, sort_order, created_at, updated_at)
SELECT admin_id, id, 'requirement', 'category', 1, 1, 0, 4,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces
ON CONFLICT (admin_id, workspace_id, type_key, field_key) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM workspace_field_configs
WHERE type_key = 'requirement' AND field_key = 'category';

UPDATE workspace_field_configs
SET sort_order = sort_order - 1
WHERE type_key = 'requirement' AND sort_order > 4;

UPDATE workspace_custom_fields
SET sort_order = sort_order - 1
WHERE type_key = 'requirement' AND sort_order > 4;
-- +goose StatementEnd

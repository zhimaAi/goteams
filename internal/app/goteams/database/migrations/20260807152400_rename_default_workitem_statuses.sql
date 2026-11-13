-- +goose Up

-- 缺陷完成态使用独立国际化键，避免需求的“已完成”被一并改名。
UPDATE workspace_statuses
SET name = '${workitem_status_resolved}',
    updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE type_key = 'defect'
  AND is_default = 1
  AND name = '${workitem_status_done}';

COMMENT ON COLUMN workspaces_settings.requirement_final_status_ids IS
  '需求结束状态ID集合（逗号分隔的 workspace_statuses.id，默认已完成、已关闭、已拒绝ID）';
COMMENT ON COLUMN workspaces_settings.defect_final_status_ids IS
  '缺陷结束状态ID集合（逗号分隔的 workspace_statuses.id，默认已解决、已关闭、已拒绝ID）';

-- +goose Down

UPDATE workspace_statuses
SET name = '${workitem_status_done}',
    updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE type_key = 'defect'
  AND is_default = 1
  AND name = '${workitem_status_resolved}';

COMMENT ON COLUMN workspaces_settings.requirement_final_status_ids IS
  '需求结束状态ID集合（逗号分隔的 workspace_statuses.id，默认已完成、已关闭、已驳回ID）';
COMMENT ON COLUMN workspaces_settings.defect_final_status_ids IS
  '缺陷结束状态ID集合（逗号分隔的 workspace_statuses.id，默认已完成、已关闭、已驳回ID）';

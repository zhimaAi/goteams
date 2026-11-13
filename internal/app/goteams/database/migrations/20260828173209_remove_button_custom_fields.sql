-- +goose Up

-- 按钮选择类型尚未上线，直接清理测试字段和值。
DELETE FROM requirement_custom_field_values cfv
USING workspace_custom_fields f
WHERE cfv.admin_id = f.admin_id
  AND cfv.field_id = f.id
  AND f.field_type IN ('radio', 'checkbox');

DELETE FROM workspace_custom_fields
WHERE field_type IN ('radio', 'checkbox');

-- +goose Down

-- 测试字段定义和值已被删除，无法无损恢复，回滚保持现状。
SELECT 1;

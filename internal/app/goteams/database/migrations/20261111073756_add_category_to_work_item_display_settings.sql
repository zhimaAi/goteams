-- +goose Up
-- 分类此前不能加入按状态配置，升级时保留原字段顺序并补齐已启用的分类。
WITH backfill AS (
    SELECT settings.admin_id, settings.workspace_id, settings.id,
           jsonb_object_agg(status.key, CASE
               WHEN status.value ? 'category' THEN status.value
               ELSE COALESCE(NULLIF(status.value, 'null'::jsonb), '[]'::jsonb) || '["category"]'::jsonb
           END) AS fields
    FROM workspace_work_item_display_settings AS settings
    JOIN workspace_field_configs AS config
      ON config.admin_id = settings.admin_id
     AND config.workspace_id = settings.workspace_id
     AND config.type_key = settings.type_key
     AND config.field_key = 'category'
     AND config.is_enabled = 1
    CROSS JOIN LATERAL jsonb_each(COALESCE(NULLIF(btrim(settings.status_fields_json), ''), '{}')::jsonb) AS status
    WHERE settings.type_key = 'requirement'
    GROUP BY settings.admin_id, settings.workspace_id, settings.id
    HAVING bool_or(NOT (status.value ? 'category'))
)
UPDATE workspace_work_item_display_settings AS settings
SET status_fields_json = backfill.fields::text
FROM backfill
WHERE settings.admin_id = backfill.admin_id
  AND settings.workspace_id = backfill.workspace_id
  AND settings.id = backfill.id;

-- +goose Down
-- 旧版的保存接口会拒绝分类，回退时仅移除此字段。
WITH field_changes AS (
    SELECT settings.admin_id, settings.workspace_id, settings.id,
           jsonb_object_agg(status.key, CASE
               WHEN status.value ? 'category' THEN status.value - 'category'
               ELSE status.value
           END) AS fields
    FROM workspace_work_item_display_settings AS settings
    CROSS JOIN LATERAL jsonb_each(COALESCE(NULLIF(btrim(settings.status_fields_json), ''), '{}')::jsonb) AS status
    WHERE settings.type_key = 'requirement'
    GROUP BY settings.admin_id, settings.workspace_id, settings.id
    HAVING bool_or(status.value ? 'category')
)
UPDATE workspace_work_item_display_settings AS settings
SET status_fields_json = field_changes.fields::text
FROM field_changes
WHERE settings.admin_id = field_changes.admin_id
  AND settings.workspace_id = field_changes.workspace_id
  AND settings.id = field_changes.id;

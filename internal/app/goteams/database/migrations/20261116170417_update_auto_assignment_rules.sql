-- +goose Up
ALTER TABLE work_item_auto_assignment ALTER COLUMN config SET DEFAULT '{"rules":[]}';
UPDATE work_item_auto_assignment
SET config = config - 'creator_first', revision = revision + 1
WHERE config ? 'creator_first';

-- 历史规则未记录创建时间，按原追加顺序补齐；手动调整过的顺序无法还原。
WITH migrated AS (
 SELECT a.admin_id, a.workspace_id,
 jsonb_agg(rule || jsonb_build_object('created_at',
   COALESCE((rule->>'created_at')::bigint, (extract(epoch FROM statement_timestamp()) * 1000)::bigint - jsonb_array_length(a.config->'rules') + ordinal))
   ORDER BY COALESCE((rule->>'created_at')::bigint, (extract(epoch FROM statement_timestamp()) * 1000)::bigint - jsonb_array_length(a.config->'rules') + ordinal) DESC) AS rules
 FROM work_item_auto_assignment a
 CROSS JOIN LATERAL jsonb_array_elements(a.config->'rules') WITH ORDINALITY AS r(rule, ordinal)
 WHERE EXISTS (SELECT 1 FROM jsonb_array_elements(a.config->'rules') old WHERE NOT old ? 'created_at')
 GROUP BY a.admin_id, a.workspace_id
)
UPDATE work_item_auto_assignment a
SET config = jsonb_set(a.config, '{rules}', m.rules), revision = a.revision + 1
FROM migrated m
WHERE a.admin_id = m.admin_id AND a.workspace_id = m.workspace_id;

-- +goose Down
-- 已移除的配置和历史人工顺序无法安全恢复，保留规则创建时间。
SELECT 1;

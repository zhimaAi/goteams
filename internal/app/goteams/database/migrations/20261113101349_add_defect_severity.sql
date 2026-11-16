-- +goose Up
CREATE TABLE IF NOT EXISTS workspace_severities (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    preset_key VARCHAR(20),
    name VARCHAR(100) NOT NULL,
    color VARCHAR(20) NOT NULL,
    sort_order INTEGER NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE (admin_id, workspace_id, preset_key)
);

CREATE UNIQUE INDEX IF NOT EXISTS workspace_severities_default ON workspace_severities (admin_id, workspace_id)
    WHERE is_default;

-- 严重等级选项删除后，缺陷保留原 ID。
ALTER TABLE requirements ADD COLUMN IF NOT EXISTS severity_id INTEGER NOT NULL DEFAULT 0;

INSERT INTO workspace_severities (admin_id, workspace_id, preset_key, name, color, sort_order)
SELECT w.admin_id, w.id, p.key, p.name, p.color, p.sort_order
FROM workspaces w
CROSS JOIN (VALUES
    ('suggestion', '${workitem_severity_suggestion}', '#808695', 1),
    ('normal', '${workitem_severity_normal}', '#2d8cf0', 2),
    ('serious', '${workitem_severity_serious}', '#ff8c00', 3),
    ('fatal', '${workitem_severity_fatal}', '#e54545', 4)
) p(key, name, color, sort_order)
ON CONFLICT (admin_id, workspace_id, preset_key) DO NOTHING;

-- 在优先级之后腾出位置，内置字段与自定义字段的原相对顺序保持一致。
WITH positions AS (
    SELECT admin_id, workspace_id, sort_order
    FROM workspace_field_configs p
    WHERE type_key = 'defect' AND field_key = 'priority'
      AND NOT EXISTS (
          SELECT 1 FROM workspace_field_configs s
          WHERE s.admin_id = p.admin_id AND s.workspace_id = p.workspace_id
            AND s.type_key = 'defect' AND s.field_key = 'severity_id'
      )
), builtin_shift AS (
    UPDATE workspace_field_configs f SET sort_order = f.sort_order + 1
    FROM positions p
    WHERE f.admin_id = p.admin_id AND f.workspace_id = p.workspace_id
      AND f.type_key = 'defect' AND f.sort_order > p.sort_order
), custom_shift AS (
    UPDATE workspace_custom_fields f SET sort_order = f.sort_order + 1
    FROM positions p
    WHERE f.admin_id = p.admin_id AND f.workspace_id = p.workspace_id
      AND f.type_key = 'defect' AND f.sort_order >= p.sort_order
)
INSERT INTO workspace_field_configs (admin_id, workspace_id, type_key, field_key, sort_order, created_at, updated_at)
SELECT w.admin_id, w.id, 'defect', 'severity_id', COALESCE(p.sort_order + 1, 5),
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces w
LEFT JOIN positions p ON p.admin_id = w.admin_id AND p.workspace_id = w.id
ON CONFLICT (admin_id, workspace_id, type_key, field_key) DO NOTHING;

-- +goose Down
-- 等级选项和缺陷引用可能已被使用，保留数据以支持回滚应用版本。
SELECT 1;

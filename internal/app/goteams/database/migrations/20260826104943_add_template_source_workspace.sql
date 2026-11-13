-- +goose Up

ALTER TABLE work_item_templates
    ADD COLUMN IF NOT EXISTS source_workspace_id INTEGER NOT NULL DEFAULT 0;

COMMENT ON COLUMN "work_item_templates"."source_workspace_id" IS '模板字段与默认值最后一次保存时所属的来源工作空间ID';

-- 已在测试环境创建的模板以最早关联的工作空间作为来源，保证增量迁移后字段ID仍可解析。
UPDATE work_item_templates AS template
SET source_workspace_id = association.workspace_id
FROM (
    SELECT DISTINCT ON (admin_id, template_id) admin_id, template_id, workspace_id
    FROM workspace_work_item_templates
    ORDER BY admin_id, template_id, id
) AS association
WHERE template.admin_id = association.admin_id
  AND template.id = association.template_id
  AND template.source_workspace_id = 0;

-- +goose Down

ALTER TABLE work_item_templates DROP COLUMN IF EXISTS source_workspace_id;

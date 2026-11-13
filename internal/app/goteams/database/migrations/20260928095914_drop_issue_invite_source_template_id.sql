-- +goose Up

-- 邀填模板改为独立新建，不再从项目模板引入：来源模板列与其唯一索引一并删除。
-- 该索引为 (admin_id, workspace_id, source_template_id)，新模板的 source 全为 0 时同项目第二条模板会冲突。
DROP INDEX IF EXISTS uk_workspace_issue_item_templates_source;

ALTER TABLE workspace_issue_item_templates
    DROP COLUMN IF EXISTS source_template_id;

-- +goose Down

ALTER TABLE workspace_issue_item_templates
    ADD COLUMN IF NOT EXISTS source_template_id INTEGER NOT NULL DEFAULT 0;

COMMENT ON COLUMN "workspace_issue_item_templates"."source_template_id" IS '引入时的来源模板ID，仅用于项目内防重复引入，不用于数据同步';

CREATE UNIQUE INDEX IF NOT EXISTS uk_workspace_issue_item_templates_source
    ON workspace_issue_item_templates(admin_id, workspace_id, source_template_id);

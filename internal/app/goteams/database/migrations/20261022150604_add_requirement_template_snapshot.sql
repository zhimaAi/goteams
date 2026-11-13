-- +goose Up
ALTER TABLE requirements
    ADD COLUMN template_id INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN template_name VARCHAR(50) NOT NULL DEFAULT '';

COMMENT ON COLUMN requirements.template_id IS '创建工单时实际使用的模板ID';
COMMENT ON COLUMN requirements.template_name IS '创建工单时使用的模板名称快照';

CREATE INDEX idx_requirements_workspace_template
    ON requirements(admin_id, workspace_id, template_id, id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_requirements_workspace_template;
ALTER TABLE requirements
    DROP COLUMN template_name,
    DROP COLUMN template_id;

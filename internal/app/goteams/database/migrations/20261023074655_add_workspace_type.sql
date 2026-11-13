-- +goose Up
ALTER TABLE workspaces ADD COLUMN workspace_type VARCHAR(20) NOT NULL DEFAULT '';
ALTER TABLE workspaces ADD CONSTRAINT workspaces_type_check
    CHECK (workspace_type IN ('', 'software', 'agile', 'issue'));
COMMENT ON COLUMN workspaces.workspace_type IS '项目类型：software软件研发、agile敏捷研发、issue工单；空值保留历史项目行为，不回填';

-- +goose Down
-- 保留已保存的项目类型，避免回滚删除新建项目的分类信息。

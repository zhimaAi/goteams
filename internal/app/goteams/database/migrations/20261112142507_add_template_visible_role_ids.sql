-- +goose Up

ALTER TABLE workspace_work_item_templates
    ADD COLUMN IF NOT EXISTS visible_role_ids INTEGER[] NOT NULL DEFAULT '{}';
COMMENT ON COLUMN workspace_work_item_templates.visible_role_ids IS '可见的工作空间角色ID，空数组表示不限制角色';

ALTER TABLE workspace_issue_item_templates
    ADD COLUMN IF NOT EXISTS visible_role_ids INTEGER[] NOT NULL DEFAULT '{}';
COMMENT ON COLUMN workspace_issue_item_templates.visible_role_ids IS '可见的工作空间角色ID，空数组表示不限制角色';

-- +goose Down

-- 已配置的可见范围需要随应用回滚保留，避免后续恢复部署时扩大可见范围。
SELECT 1;

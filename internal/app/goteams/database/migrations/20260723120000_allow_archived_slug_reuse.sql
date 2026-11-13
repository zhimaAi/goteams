-- +goose Up
-- 移除 slug 上的全量唯一约束，改为仅对未归档工作空间生效的部分唯一索引。
-- 这样允许已归档工作空间复用相同 slug，但同一租户下仍只允许存在一个未归档的相同 slug。
ALTER TABLE workspaces DROP CONSTRAINT IF EXISTS workspaces_slug_key;
CREATE UNIQUE INDEX IF NOT EXISTS workspaces_slug_active_key ON workspaces(slug) WHERE COALESCE(archived, 0) = 0;

-- +goose Down
DROP INDEX IF EXISTS workspaces_slug_active_key;
ALTER TABLE workspaces ADD CONSTRAINT workspaces_slug_key UNIQUE (slug);

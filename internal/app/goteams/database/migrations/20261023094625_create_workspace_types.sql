-- +goose Up
CREATE TABLE workspace_types (
    type_key VARCHAR(20) PRIMARY KEY,
    name VARCHAR(100) NOT NULL CHECK (btrim(name) <> ''),
    created_admin_id INTEGER NOT NULL DEFAULT 0,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX workspace_types_name_key ON workspace_types (lower(btrim(name)));
COMMENT ON TABLE workspace_types IS '部署级模板项目类型字典，仅模板管理员维护，普通租户只读';
COMMENT ON COLUMN workspace_types.type_key IS '稳定类型标识，内置类型固定，自定义类型随机生成';
COMMENT ON COLUMN workspace_types.created_admin_id IS '创建类型的模板管理员租户，内置类型为0';
COMMENT ON COLUMN workspace_types.created_by IS '创建用户，内置类型为0';

INSERT INTO workspace_types (type_key, name) VALUES
    ('software', '软件研发'), ('agile', '敏捷研发'), ('issue', '工单管理')
ON CONFLICT DO NOTHING;

ALTER TABLE workspaces DROP CONSTRAINT workspaces_type_check;
ALTER TABLE workspaces ADD CONSTRAINT workspaces_type_check
    CHECK (workspace_type IN ('', 'software', 'agile', 'issue')
        OR workspace_type ~ '^custom_[0-9a-f]{12}$');
COMMENT ON COLUMN workspaces.workspace_type IS '项目类型稳定标识，类型字典来自workspace_types或离线模板包；空值为未分类历史项目';

-- +goose Down
-- 保留类型字典及已使用的自定义类型，避免回滚丢失模板分类。

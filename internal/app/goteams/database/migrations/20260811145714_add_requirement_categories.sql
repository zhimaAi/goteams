-- +goose Up
-- +goose StatementBegin

-- 需求分类为工作空间共享的三级树；所有的和未分类为前端虚拟节点，不写入此表。
CREATE TABLE IF NOT EXISTS requirement_categories (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    parent_id INTEGER NOT NULL DEFAULT 0,
    name VARCHAR(20) NOT NULL,
    sort_order BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE requirement_categories IS '工作空间需求分类表';
COMMENT ON COLUMN requirement_categories.admin_id IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN requirement_categories.workspace_id IS '所属工作空间ID';
COMMENT ON COLUMN requirement_categories.parent_id IS '父分类ID，0表示真实一级分类';
COMMENT ON COLUMN requirement_categories.name IS '分类名称，工作空间内不区分大小写唯一';
COMMENT ON COLUMN requirement_categories.sort_order IS '同级分类排序值';
COMMENT ON COLUMN requirement_categories.created_at IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN requirement_categories.updated_at IS '更新时间（Unix时间戳）';

CREATE UNIQUE INDEX IF NOT EXISTS uq_req_category_name
    ON requirement_categories(admin_id, workspace_id, LOWER(BTRIM(name)));
CREATE INDEX IF NOT EXISTS idx_req_category_tree
    ON requirement_categories(admin_id, workspace_id, parent_id, sort_order, id);

ALTER TABLE requirements
    ADD COLUMN IF NOT EXISTS category_id INTEGER NOT NULL DEFAULT 0;

COMMENT ON COLUMN requirements.category_id IS '需求分类ID，0表示未分类；同一需求树保持一致';

CREATE INDEX IF NOT EXISTS idx_req_category_filter
    ON requirements(admin_id, workspace_id, type, parent_id, category_id, id DESC);

-- 为存量系统预设项目管理员补齐分类管理权限，普通成员和自定义角色不自动授予。
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, 'requirement:manage_category',
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
WHERE r.scope = 'project'
  AND r.is_system = 1
  AND r.name IN ('perm_role_project_admin', '${perm_role_project_admin}')
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DELETE FROM role_permissions WHERE permission_key = 'requirement:manage_category';

DROP INDEX IF EXISTS idx_req_category_filter;
ALTER TABLE requirements DROP COLUMN IF EXISTS category_id;

DROP TABLE IF EXISTS requirement_categories;

-- +goose StatementEnd

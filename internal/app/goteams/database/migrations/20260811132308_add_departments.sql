-- +goose Up
-- 部门表：企业组织架构树节点（树顶级的"企业"为虚拟节点，不入本表）
CREATE TABLE IF NOT EXISTS departments (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    parent_id INTEGER NOT NULL DEFAULT 0,
    name VARCHAR(50) NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE departments IS '企业部门（组织架构树节点）';
COMMENT ON COLUMN departments.admin_id IS '归属租户管理员ID';
COMMENT ON COLUMN departments.parent_id IS '父部门ID，0=一级部门';

CREATE INDEX IF NOT EXISTS idx_departments_admin_parent ON departments (admin_id, parent_id);
-- 同一父级下部门名称唯一（业务层先校验给出友好报错，索引兜底并发场景）
CREATE UNIQUE INDEX IF NOT EXISTS idx_departments_admin_parent_name ON departments (admin_id, parent_id, name);

-- 成员与部门关联表：多对多，一个成员可属于多个部门
CREATE TABLE IF NOT EXISTS user_departments (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    department_id INTEGER NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE user_departments IS '企业成员与部门关联（多对多）';

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_departments_user_department ON user_departments (user_id, department_id);
CREATE INDEX IF NOT EXISTS idx_user_departments_admin_department ON user_departments (admin_id, department_id);

-- 邀请链接可指定部门：受邀人接受邀请后自动归入这些部门
ALTER TABLE member_invitations ADD COLUMN IF NOT EXISTS department_ids VARCHAR(500) NOT NULL DEFAULT '';
COMMENT ON COLUMN member_invitations.department_ids IS '受邀人加入后归属的部门ID列表，逗号分隔，空=不归属部门';

-- +goose Down
ALTER TABLE member_invitations DROP COLUMN IF EXISTS department_ids;
DROP INDEX IF EXISTS idx_user_departments_admin_department;
DROP INDEX IF EXISTS idx_user_departments_user_department;
DROP TABLE IF EXISTS user_departments;
DROP INDEX IF EXISTS idx_departments_admin_parent_name;
DROP INDEX IF EXISTS idx_departments_admin_parent;
DROP TABLE IF EXISTS departments;

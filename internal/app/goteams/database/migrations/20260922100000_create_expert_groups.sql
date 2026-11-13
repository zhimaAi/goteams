-- +goose Up
-- +goose StatementBegin

-- 云端专家团（需求 1896 三项优化之优化点1：云端配置专家团）。
-- 对齐专家流水线：
-- 1. expert_groups：专家团主体，含适用范围 scope（all=全部成员，members=指定成员，departments=指定组织）。
-- 2. expert_group_agents：专家团编排的专家（Agent）成员，最多 1 个 leader。
-- 3. expert_group_users：scope=members 时指定的适用成员。
-- 4. expert_group_departments：scope=departments 时指定的适用组织。

-- 1. expert_groups
CREATE TABLE IF NOT EXISTS expert_groups (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    name VARCHAR(50) NOT NULL DEFAULT '',
    description VARCHAR(200) NOT NULL DEFAULT '',
    avatar VARCHAR(255) NOT NULL DEFAULT '',
    scope VARCHAR(20) NOT NULL DEFAULT 'all',
    status INTEGER NOT NULL DEFAULT 1,
    created_by INTEGER NOT NULL DEFAULT 0,
    updated_by INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    deleted_at BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_expert_groups_admin
    ON expert_groups(admin_id, deleted_at, id);

COMMENT ON TABLE expert_groups IS '云端专家团';
COMMENT ON COLUMN expert_groups.admin_id IS '租户 ID';
COMMENT ON COLUMN expert_groups.name IS '专家团名称（≤20 字）';
COMMENT ON COLUMN expert_groups.description IS '专家团简介（≤200 字）';
COMMENT ON COLUMN expert_groups.avatar IS '专家团头像（存储路径）';
COMMENT ON COLUMN expert_groups.scope IS '适用范围：all=全部成员，members=指定成员，departments=指定组织';
COMMENT ON COLUMN expert_groups.status IS '状态：1=启用，0=停用';
COMMENT ON COLUMN expert_groups.created_by IS '创建者用户 ID';
COMMENT ON COLUMN expert_groups.updated_by IS '最近更新者用户 ID';
COMMENT ON COLUMN expert_groups.created_at IS '创建时间（Unix 秒）';
COMMENT ON COLUMN expert_groups.updated_at IS '更新时间（Unix 秒）';
COMMENT ON COLUMN expert_groups.deleted_at IS '软删除时间（0=未删除）';

-- 2. expert_group_agents（编排的专家成员）
CREATE TABLE IF NOT EXISTS expert_group_agents (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    expert_group_id INTEGER NOT NULL,
    name VARCHAR(50) NOT NULL DEFAULT '',
    role VARCHAR(20) NOT NULL DEFAULT 'member',
    prompt TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, expert_group_id, name)
);

CREATE INDEX IF NOT EXISTS idx_expert_group_agents_group
    ON expert_group_agents(admin_id, expert_group_id, sort_order, id);

COMMENT ON TABLE expert_group_agents IS '专家团编排的专家（Agent）成员';
COMMENT ON COLUMN expert_group_agents.admin_id IS '租户 ID';
COMMENT ON COLUMN expert_group_agents.expert_group_id IS '专家团 ID';
COMMENT ON COLUMN expert_group_agents.name IS '专家名称';
COMMENT ON COLUMN expert_group_agents.role IS '角色：leader=主专家（最多 1 个），member=普通专家';
COMMENT ON COLUMN expert_group_agents.prompt IS '专家提示词';
COMMENT ON COLUMN expert_group_agents.sort_order IS '排序（越小越靠前）';
COMMENT ON COLUMN expert_group_agents.created_at IS '创建时间（Unix 秒）';
COMMENT ON COLUMN expert_group_agents.updated_at IS '更新时间（Unix 秒）';

-- 3. expert_group_users（scope=members 时的适用成员）
CREATE TABLE IF NOT EXISTS expert_group_users (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    expert_group_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, expert_group_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_expert_group_users_admin_user
    ON expert_group_users(admin_id, user_id, expert_group_id);

COMMENT ON TABLE expert_group_users IS '专家团适用成员（scope=members 时的授权成员）';
COMMENT ON COLUMN expert_group_users.admin_id IS '租户 ID';
COMMENT ON COLUMN expert_group_users.expert_group_id IS '专家团 ID';
COMMENT ON COLUMN expert_group_users.user_id IS '成员用户 ID';
COMMENT ON COLUMN expert_group_users.created_by IS '创建者用户 ID';
COMMENT ON COLUMN expert_group_users.created_at IS '创建时间（Unix 秒）';

-- 4. expert_group_departments（scope=departments 时的适用组织）
CREATE TABLE IF NOT EXISTS expert_group_departments (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    expert_group_id INTEGER NOT NULL,
    department_id INTEGER NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, expert_group_id, department_id)
);

CREATE INDEX IF NOT EXISTS idx_expert_group_departments_admin_dept
    ON expert_group_departments(admin_id, department_id, expert_group_id);

COMMENT ON TABLE expert_group_departments IS '专家团适用组织（scope=departments 时的授权组织）';
COMMENT ON COLUMN expert_group_departments.admin_id IS '租户 ID';
COMMENT ON COLUMN expert_group_departments.expert_group_id IS '专家团 ID';
COMMENT ON COLUMN expert_group_departments.department_id IS '组织（部门）ID';
COMMENT ON COLUMN expert_group_departments.created_by IS '创建者用户 ID';
COMMENT ON COLUMN expert_group_departments.created_at IS '创建时间（Unix 秒）';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS expert_group_departments;
DROP TABLE IF EXISTS expert_group_users;
DROP TABLE IF EXISTS expert_group_agents;
DROP TABLE IF EXISTS expert_groups;
-- +goose StatementEnd

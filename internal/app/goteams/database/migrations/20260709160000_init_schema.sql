-- +goose Up
-- SQL in this section is executed when the migration is applied.

-- ============================================================
-- 用户表
-- ============================================================
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    username VARCHAR(255),
    password VARCHAR(255) NOT NULL,
    name VARCHAR(100) NOT NULL DEFAULT '',
    avatar VARCHAR(500) DEFAULT '',
    role_id INTEGER DEFAULT 0,
    first_login VARCHAR(1) DEFAULT '1',
    tel_number VARCHAR(20) DEFAULT '',
    status SMALLINT DEFAULT 1,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE "users" IS '用户表';
COMMENT ON COLUMN "users"."id" IS '用户ID';
COMMENT ON COLUMN "users"."admin_id" IS '归属租户管理员ID（多租户隔离，等于自身ID即为租户管理员）';
COMMENT ON COLUMN "users"."username" IS '用户名（登录账号）';
COMMENT ON COLUMN "users"."password" IS '密码哈希（bcrypt 存储）';
COMMENT ON COLUMN "users"."name" IS '用户昵称';
COMMENT ON COLUMN "users"."avatar" IS '头像URL';
COMMENT ON COLUMN "users"."role_id" IS '系统角色ID: 关联 roles 表，0=无特殊角色';
COMMENT ON COLUMN "users"."first_login" IS '是否首次登录: 1=是, 0=否';
COMMENT ON COLUMN "users"."tel_number" IS '手机号码';
COMMENT ON COLUMN "users"."status" IS '账号状态: 1=正常, 0=禁用';
COMMENT ON COLUMN "users"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "users"."updated_at" IS '更新时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_users_admin_id ON users(admin_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username) WHERE username <> '';
CREATE INDEX IF NOT EXISTS idx_users_role_id ON users(role_id);

-- ============================================================
-- 工作空间表
-- ============================================================
CREATE TABLE IF NOT EXISTS workspaces (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    name VARCHAR(200) NOT NULL,
    slug VARCHAR(200) NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    color VARCHAR(20) DEFAULT '',
    icon VARCHAR(500) DEFAULT '',
    archived SMALLINT DEFAULT 0,
    active_version_id INTEGER DEFAULT 0,
    created_by INTEGER NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE "workspaces" IS '工作空间表';
COMMENT ON COLUMN "workspaces"."id" IS '工作空间ID';
COMMENT ON COLUMN "workspaces"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "workspaces"."name" IS '空间名称';
COMMENT ON COLUMN "workspaces"."slug" IS '空间标识（URL友好字符串）';
COMMENT ON COLUMN "workspaces"."description" IS '空间描述';
COMMENT ON COLUMN "workspaces"."color" IS '主题色标识';
COMMENT ON COLUMN "workspaces"."icon" IS '项目头像本地存储路径';
COMMENT ON COLUMN "workspaces"."archived" IS '归档状态: 0=正常, 1=已归档';
COMMENT ON COLUMN "workspaces"."active_version_id" IS '当前活跃版本ID: 0=未设置';
COMMENT ON COLUMN "workspaces"."created_by" IS '创建者用户ID';
COMMENT ON COLUMN "workspaces"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "workspaces"."updated_at" IS '更新时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_workspaces_admin_id ON workspaces(admin_id);
CREATE INDEX IF NOT EXISTS idx_workspaces_slug ON workspaces(slug);
CREATE INDEX IF NOT EXISTS idx_workspaces_created_by ON workspaces(created_by);
CREATE INDEX IF NOT EXISTS idx_workspaces_active_version_id ON workspaces(active_version_id);

-- ============================================================
-- 工作空间成员表
-- ============================================================
CREATE TABLE IF NOT EXISTS workspace_members (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    role_id INTEGER NOT NULL DEFAULT 0,
    member_type VARCHAR(20) DEFAULT 'human',
    joined_at BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(workspace_id, user_id)
);

COMMENT ON TABLE "workspace_members" IS '工作空间成员关联表';
COMMENT ON COLUMN "workspace_members"."id" IS '关联ID';
COMMENT ON COLUMN "workspace_members"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "workspace_members"."workspace_id" IS '工作空间ID';
COMMENT ON COLUMN "workspace_members"."user_id" IS '用户ID';
COMMENT ON COLUMN "workspace_members"."role_id" IS '空间内角色: 存储 roles 表的 ID（关联 role_permissions 获取权限列表）';
COMMENT ON COLUMN "workspace_members"."member_type" IS '成员类型: human=人类用户, agent=AI代理';
COMMENT ON COLUMN "workspace_members"."joined_at" IS '加入时间（Unix时间戳）';
COMMENT ON COLUMN "workspace_members"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "workspace_members"."updated_at" IS '更新时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_wm_admin_id ON workspace_members(admin_id);
CREATE INDEX IF NOT EXISTS idx_workspace_members_ws ON workspace_members(workspace_id);
CREATE INDEX IF NOT EXISTS idx_workspace_members_user ON workspace_members(user_id);

-- ============================================================
-- 需求表
-- ============================================================
CREATE TABLE IF NOT EXISTS requirements (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL,
    parent_id INTEGER DEFAULT 0,
    title VARCHAR(500) NOT NULL,
    description TEXT DEFAULT '',
    type VARCHAR(100) DEFAULT 'requirement',
    priority INTEGER DEFAULT 0,
    status_id INTEGER DEFAULT 0,
    assignee_id INTEGER DEFAULT NULL,
    version_id INTEGER DEFAULT NULL,
    planned_start_date BIGINT NOT NULL DEFAULT 0,
    planned_end_date BIGINT NOT NULL DEFAULT 0,
    work_hours DECIMAL(10,2) DEFAULT 0,
    comment_count INTEGER DEFAULT 0,
    created_by INTEGER NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE "requirements" IS '需求/缺陷表';
COMMENT ON COLUMN "requirements"."id" IS '需求ID';
COMMENT ON COLUMN "requirements"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "requirements"."workspace_id" IS '所属工作空间ID';
COMMENT ON COLUMN "requirements"."parent_id" IS '父需求ID: 0=顶级需求，非0=子需求';
COMMENT ON COLUMN "requirements"."title" IS '需求标题';
COMMENT ON COLUMN "requirements"."description" IS '需求详细描述';
COMMENT ON COLUMN "requirements"."type" IS '需求类型: requirement=需求, defect=缺陷';
COMMENT ON COLUMN "requirements"."priority" IS '优先级ID（关联 workspace_priorities.id，0=未指定）';
COMMENT ON COLUMN "requirements"."status_id" IS '状态ID（关联 workspace_statuses.id）';
COMMENT ON COLUMN "requirements"."assignee_id" IS '负责人用户ID';
COMMENT ON COLUMN "requirements"."version_id" IS '所属版本迭代ID';
COMMENT ON COLUMN "requirements"."planned_start_date" IS '预计开始时间';
COMMENT ON COLUMN "requirements"."planned_end_date" IS '预计结束时间';
COMMENT ON COLUMN "requirements"."work_hours" IS '工时（小时）';
COMMENT ON COLUMN "requirements"."comment_count" IS '评论数量（冗余字段，用于列表展示）';
COMMENT ON COLUMN "requirements"."created_by" IS '创建者用户ID';
COMMENT ON COLUMN "requirements"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "requirements"."updated_at" IS '更新时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_requirements_admin_id ON requirements(admin_id);
CREATE INDEX IF NOT EXISTS idx_requirements_ws ON requirements(workspace_id);
CREATE INDEX IF NOT EXISTS idx_requirements_parent ON requirements(parent_id);
CREATE INDEX IF NOT EXISTS idx_requirements_status_id ON requirements(status_id);
CREATE INDEX IF NOT EXISTS idx_requirements_assignee ON requirements(assignee_id);
CREATE INDEX IF NOT EXISTS idx_requirements_version ON requirements(version_id);
CREATE INDEX IF NOT EXISTS idx_req_admin_ws_status ON requirements(admin_id, workspace_id, status_id);
CREATE INDEX IF NOT EXISTS idx_req_admin_ws_version ON requirements(admin_id, workspace_id, version_id);
CREATE INDEX IF NOT EXISTS idx_req_admin_ws_assignee ON requirements(admin_id, workspace_id, assignee_id);
CREATE INDEX IF NOT EXISTS idx_req_admin_assignee_plan_start ON requirements(admin_id, assignee_id, planned_start_date);
CREATE INDEX IF NOT EXISTS idx_requirements_admin_ws_type_parent ON requirements(admin_id, workspace_id, type, parent_id);

-- ============================================================
-- 需求评论表
-- ============================================================
CREATE TABLE IF NOT EXISTS requirement_comments (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    requirement_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    content TEXT NOT NULL,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE "requirement_comments" IS '需求评论表';
COMMENT ON COLUMN "requirement_comments"."id" IS '评论ID';
COMMENT ON COLUMN "requirement_comments"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "requirement_comments"."requirement_id" IS '所属需求ID';
COMMENT ON COLUMN "requirement_comments"."user_id" IS '评论用户ID';
COMMENT ON COLUMN "requirement_comments"."content" IS '评论内容';
COMMENT ON COLUMN "requirement_comments"."workspace_id" IS '所属工作空间ID';
COMMENT ON COLUMN "requirement_comments"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "requirement_comments"."updated_at" IS '更新时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_comments_admin_id ON requirement_comments(admin_id);
CREATE INDEX IF NOT EXISTS idx_comments_requirement ON requirement_comments(requirement_id);

-- ============================================================
-- 需求及评论附件表
-- ============================================================
CREATE TABLE IF NOT EXISTS requirement_attachments (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL,
    requirement_id INTEGER NOT NULL,
    comment_id INTEGER NOT NULL DEFAULT 0,
    uploaded_by INTEGER NOT NULL,
    original_name VARCHAR(500) NOT NULL,
    stored_name VARCHAR(100) NOT NULL,
    stored_path VARCHAR(1000) NOT NULL,
    content_type VARCHAR(200) DEFAULT '',
    file_size BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE requirement_attachments IS '需求及评论附件表';
COMMENT ON COLUMN requirement_attachments.comment_id IS '评论ID，0表示需求级附件';
COMMENT ON COLUMN requirement_attachments.stored_path IS '相对于应用数据目录的文件路径';

CREATE INDEX IF NOT EXISTS idx_ra_admin_workspace ON requirement_attachments(admin_id, workspace_id);
CREATE INDEX IF NOT EXISTS idx_ra_requirement ON requirement_attachments(requirement_id);
CREATE INDEX IF NOT EXISTS idx_ra_comment ON requirement_attachments(comment_id);

-- ============================================================
-- 需求变更历史表
-- ============================================================
CREATE TABLE IF NOT EXISTS requirement_histories (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    requirement_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    action VARCHAR(50) NOT NULL,
    field_name VARCHAR(100) DEFAULT '',
    old_value TEXT DEFAULT '',
    new_value TEXT DEFAULT '',
    created_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE "requirement_histories" IS '需求变更历史表';
COMMENT ON COLUMN "requirement_histories"."id" IS '记录ID';
COMMENT ON COLUMN "requirement_histories"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "requirement_histories"."workspace_id" IS '所属工作空间ID';
COMMENT ON COLUMN "requirement_histories"."requirement_id" IS '所属需求ID';
COMMENT ON COLUMN "requirement_histories"."user_id" IS '操作人用户ID';
COMMENT ON COLUMN "requirement_histories"."action" IS '操作类型: status_change=状态变更, field_update=字段更新, assignee_change=指派人变更';
COMMENT ON COLUMN "requirement_histories"."field_name" IS '变更字段名';
COMMENT ON COLUMN "requirement_histories"."old_value" IS '旧值';
COMMENT ON COLUMN "requirement_histories"."new_value" IS '新值';
COMMENT ON COLUMN "requirement_histories"."created_at" IS '创建时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_rh_admin_id ON requirement_histories(admin_id);
CREATE INDEX IF NOT EXISTS idx_rh_requirement ON requirement_histories(requirement_id);

-- ============================================================
-- 版本迭代表
-- ============================================================
CREATE TABLE IF NOT EXISTS versions (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL,
    name VARCHAR(200) NOT NULL,
    start_date BIGINT NOT NULL DEFAULT 0,
    end_date BIGINT NOT NULL DEFAULT 0,
    requirement_count INTEGER DEFAULT 0,
    completed_count INTEGER DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, workspace_id, name)
);

COMMENT ON TABLE "versions" IS '版本迭代表';
COMMENT ON COLUMN "versions"."id" IS '版本ID';
COMMENT ON COLUMN "versions"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "versions"."workspace_id" IS '所属工作空间ID';
COMMENT ON COLUMN "versions"."name" IS '版本名称（如 v1.0.0）';
COMMENT ON COLUMN "versions"."start_date" IS '计划开始日期（Unix 秒级时间戳，0=未设置）';
COMMENT ON COLUMN "versions"."end_date" IS '计划结束日期（Unix 秒级时间戳，0=未设置）';
COMMENT ON COLUMN "versions"."requirement_count" IS '关联需求总数（冗余字段）';
COMMENT ON COLUMN "versions"."completed_count" IS '已完成需求数（冗余字段）';
COMMENT ON COLUMN "versions"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "versions"."updated_at" IS '更新时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_versions_admin_id ON versions(admin_id);
CREATE INDEX IF NOT EXISTS idx_versions_ws ON versions(workspace_id);

-- ============================================================
-- 角色表（企业角色 + 工作空间角色）
-- ============================================================
CREATE TABLE IF NOT EXISTS roles (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    name VARCHAR(100) NOT NULL,
    is_system SMALLINT DEFAULT 0,
    scope VARCHAR(20) DEFAULT 'project',
    description VARCHAR(200) DEFAULT '',
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE "roles" IS '角色表';
COMMENT ON COLUMN "roles"."id" IS '角色ID';
COMMENT ON COLUMN "roles"."admin_id" IS '归属租户管理员ID（必须为具体租户的 admin_id，不允许为0）';
COMMENT ON COLUMN "roles"."workspace_id" IS '所属工作空间ID: 0=企业角色, >0=项目角色（绑定到具体工作空间）';
COMMENT ON COLUMN "roles"."name" IS '角色名称: 以 perm. 开头为 i18n key（后端根据语言替换为对应显示名），否则直接显示';
COMMENT ON COLUMN "roles"."description" IS '角色描述: 最多200个字符，系统预设可存储 i18n key';
COMMENT ON COLUMN "roles"."is_system" IS '是否系统预设: 1=是（不可修改/不可删除）, 0=否（用户自定义）';
COMMENT ON COLUMN "roles"."scope" IS '角色作用域: enterprise=企业角色, project=项目角色（绑定到具体 workspace_id）';
COMMENT ON COLUMN "roles"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "roles"."updated_at" IS '更新时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_roles_admin_id ON roles(admin_id);
CREATE INDEX IF NOT EXISTS idx_roles_workspace ON roles(workspace_id);

-- ============================================================
-- 角色权限关联表
-- ============================================================
CREATE TABLE IF NOT EXISTS role_permissions (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    role_id INTEGER NOT NULL,
    permission_key VARCHAR(100) NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, role_id, permission_key)
);

COMMENT ON TABLE "role_permissions" IS '角色权限关联表';
COMMENT ON COLUMN "role_permissions"."id" IS '关联ID';
COMMENT ON COLUMN "role_permissions"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "role_permissions"."role_id" IS '角色ID';
COMMENT ON COLUMN "role_permissions"."permission_key" IS '权限标识（如 requirement:create），对应代码中硬编码的权限 key';
COMMENT ON COLUMN "role_permissions"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "role_permissions"."updated_at" IS '更新时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_role_permissions_admin ON role_permissions(admin_id);
CREATE INDEX IF NOT EXISTS idx_role_permissions_role ON role_permissions(role_id);
CREATE INDEX IF NOT EXISTS idx_role_permissions_key ON role_permissions(permission_key);

-- ============================================================
-- 工作项状态定义表（工作空间级）
-- ============================================================
CREATE TABLE IF NOT EXISTS workspace_statuses (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    type_key VARCHAR(50) NOT NULL,
    name VARCHAR(100) DEFAULT '',
    color VARCHAR(20) NOT NULL DEFAULT '',
    sort_order SMALLINT DEFAULT 0,
    is_default SMALLINT DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, workspace_id, type_key, name)
);

COMMENT ON TABLE "workspace_statuses" IS '工作项状态定义表';
COMMENT ON COLUMN "workspace_statuses"."id" IS '状态ID';
COMMENT ON COLUMN "workspace_statuses"."admin_id" IS '归属租户管理员ID';
COMMENT ON COLUMN "workspace_statuses"."workspace_id" IS '所属工作空间ID';
COMMENT ON COLUMN "workspace_statuses"."type_key" IS '工作项类型: requirement=需求, defect=缺陷';
COMMENT ON COLUMN "workspace_statuses"."name" IS '状态名称（内置状态存 i18n key，自定义状态存展示名）';
COMMENT ON COLUMN "workspace_statuses"."color" IS '状态颜色标识';
COMMENT ON COLUMN "workspace_statuses"."sort_order" IS '排序序号';
COMMENT ON COLUMN "workspace_statuses"."is_default" IS '是否默认状态: 1=是（创建工作空间时种子化的状态均为默认状态）';
COMMENT ON COLUMN "workspace_statuses"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "workspace_statuses"."updated_at" IS '更新时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_wis_admin_id ON workspace_statuses(admin_id);

-- ============================================================
-- 工作项优先级定义表（工作空间级）
-- ============================================================
CREATE TABLE IF NOT EXISTS workspace_priorities (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    type_key VARCHAR(50) NOT NULL,
    name VARCHAR(100) DEFAULT '',
    color VARCHAR(20) DEFAULT '',
    sort_order SMALLINT DEFAULT 0,
    is_default SMALLINT DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, workspace_id, type_key, name)
);

COMMENT ON TABLE "workspace_priorities" IS '工作项优先级定义表';
COMMENT ON COLUMN "workspace_priorities"."id" IS '优先级ID（全局唯一标识，业务均以 ID 引用）';
COMMENT ON COLUMN "workspace_priorities"."admin_id" IS '归属租户管理员ID';
COMMENT ON COLUMN "workspace_priorities"."workspace_id" IS '所属工作空间ID';
COMMENT ON COLUMN "workspace_priorities"."type_key" IS '工作项类型: requirement=需求, defect=缺陷';
COMMENT ON COLUMN "workspace_priorities"."name" IS '优先级展示名';
COMMENT ON COLUMN "workspace_priorities"."color" IS '颜色标识';
COMMENT ON COLUMN "workspace_priorities"."sort_order" IS '排序序号';
COMMENT ON COLUMN "workspace_priorities"."is_default" IS '是否默认优先级: 1=是';
COMMENT ON COLUMN "workspace_priorities"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "workspace_priorities"."updated_at" IS '更新时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_wip_admin_id ON workspace_priorities(admin_id);

-- ============================================================
-- 状态流转规则表（工作空间级）
-- ============================================================
CREATE TABLE IF NOT EXISTS workspace_status_transitions (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    type_key VARCHAR(50) NOT NULL,
    from_status_id INTEGER NOT NULL DEFAULT 0,
    to_status_id INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, workspace_id, type_key, from_status_id, to_status_id)
);

COMMENT ON TABLE "workspace_status_transitions" IS '状态流转规则表（工作空间级）';
COMMENT ON COLUMN "workspace_status_transitions"."id" IS '规则ID';
COMMENT ON COLUMN "workspace_status_transitions"."admin_id" IS '归属租户管理员ID';
COMMENT ON COLUMN "workspace_status_transitions"."workspace_id" IS '所属工作空间ID';
COMMENT ON COLUMN "workspace_status_transitions"."type_key" IS '工作项类型: requirement=需求, defect=缺陷';
COMMENT ON COLUMN "workspace_status_transitions"."from_status_id" IS '源状态ID（关联 workspace_statuses.id）';
COMMENT ON COLUMN "workspace_status_transitions"."to_status_id" IS '目标状态ID（关联 workspace_statuses.id）';
COMMENT ON COLUMN "workspace_status_transitions"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "workspace_status_transitions"."updated_at" IS '更新时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_wst_admin_id ON workspace_status_transitions(admin_id);

-- ============================================================
-- 工作项内置字段配置表（工作空间级，按类型区分需求/缺陷）
-- 每个工作空间下，每种类型（requirement/defect）各一组字段启用配置
-- ============================================================
CREATE TABLE IF NOT EXISTS workspace_field_configs (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    type_key VARCHAR(50) NOT NULL,
    field_key VARCHAR(50) NOT NULL,
    is_enabled SMALLINT NOT NULL DEFAULT 1,
    is_editable SMALLINT NOT NULL DEFAULT 1,
    is_locked SMALLINT NOT NULL DEFAULT 0,
    sort_order SMALLINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, workspace_id, type_key, field_key)
);

COMMENT ON TABLE "workspace_field_configs" IS '工作项内置字段启用配置表';
COMMENT ON COLUMN "workspace_field_configs"."id" IS '配置ID';
COMMENT ON COLUMN "workspace_field_configs"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "workspace_field_configs"."workspace_id" IS '所属工作空间ID';
COMMENT ON COLUMN "workspace_field_configs"."type_key" IS '工作项类型: requirement=需求, defect=缺陷';
COMMENT ON COLUMN "workspace_field_configs"."field_key" IS '内置字段标识: title/type/status/version/priority/assignee/planned_start_date/planned_end_date/created_at/created_by';
COMMENT ON COLUMN "workspace_field_configs"."is_enabled" IS '是否启用显示: 1=启用, 0=禁用';
COMMENT ON COLUMN "workspace_field_configs"."is_editable" IS '是否允许编辑: 1=可编辑, 0=只读（标题/状态默认不可编辑）';
COMMENT ON COLUMN "workspace_field_configs"."is_locked" IS '是否锁定不可关闭: 1=锁定（如标题必开）, 0=可配置';
COMMENT ON COLUMN "workspace_field_configs"."sort_order" IS '列表展示排序序号';
COMMENT ON COLUMN "workspace_field_configs"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "workspace_field_configs"."updated_at" IS '更新时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_wifc_admin_ws_type ON workspace_field_configs(admin_id, workspace_id, type_key);

-- ============================================================
-- 自定义字段定义表（工作空间级）
-- ============================================================
CREATE TABLE IF NOT EXISTS workspace_custom_fields (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    type_key VARCHAR(50) NOT NULL,
    key VARCHAR(50) NOT NULL,
    name VARCHAR(100) DEFAULT '',
    field_type VARCHAR(30) DEFAULT 'text',
    is_required SMALLINT DEFAULT 0,
    sort_order SMALLINT DEFAULT 0,
    is_enabled SMALLINT DEFAULT 1,
    config_json TEXT DEFAULT '{}',
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, workspace_id, type_key, key)
);

COMMENT ON TABLE "workspace_custom_fields" IS '自定义字段定义表（工作空间级）';
COMMENT ON COLUMN "workspace_custom_fields"."id" IS '字段ID';
COMMENT ON COLUMN "workspace_custom_fields"."admin_id" IS '归属租户管理员ID';
COMMENT ON COLUMN "workspace_custom_fields"."workspace_id" IS '所属工作空间ID';
COMMENT ON COLUMN "workspace_custom_fields"."type_key" IS '工作项类型: requirement=需求, defect=缺陷';
COMMENT ON COLUMN "workspace_custom_fields"."key" IS '字段标识（如 story_points）';
COMMENT ON COLUMN "workspace_custom_fields"."name" IS '自定义字段展示名（用户填写）';
COMMENT ON COLUMN "workspace_custom_fields"."field_type" IS '字段类型: text/number/select_single/select_multi/date/user/textarea';
COMMENT ON COLUMN "workspace_custom_fields"."is_required" IS '是否必填: 1=是';
COMMENT ON COLUMN "workspace_custom_fields"."sort_order" IS '排序序号';
COMMENT ON COLUMN "workspace_custom_fields"."is_enabled" IS '是否启用: 1=启用, 0=禁用';
COMMENT ON COLUMN "workspace_custom_fields"."config_json" IS '按字段类型存储配置：select_single/select_multi 存 {"options":[{"key":"","name":"","color":""}]}；后续可扩展 number 的 min/max 等';
COMMENT ON COLUMN "workspace_custom_fields"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "workspace_custom_fields"."updated_at" IS '更新时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_wcf_admin_id ON workspace_custom_fields(admin_id);

-- ============================================================
-- 自定义字段值表（EAV 值存储，需求级）
-- ============================================================
CREATE TABLE IF NOT EXISTS requirement_custom_field_values (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    requirement_id INTEGER NOT NULL,
    field_id INTEGER NOT NULL,
    value_text TEXT DEFAULT '',
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, requirement_id, field_id)
);

COMMENT ON TABLE "requirement_custom_field_values" IS '自定义字段值表（EAV 模式，需求级）';
COMMENT ON COLUMN "requirement_custom_field_values"."id" IS '值ID';
COMMENT ON COLUMN "requirement_custom_field_values"."admin_id" IS '归属租户管理员ID';
COMMENT ON COLUMN "requirement_custom_field_values"."requirement_id" IS '所属需求/缺陷ID（关联 requirements.id）';
COMMENT ON COLUMN "requirement_custom_field_values"."field_id" IS '所属自定义字段ID（关联 workspace_custom_fields.id）';
COMMENT ON COLUMN "requirement_custom_field_values"."value_text" IS '字段值（统一存为文本）';
COMMENT ON COLUMN "requirement_custom_field_values"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "requirement_custom_field_values"."updated_at" IS '更新时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_rcfv_admin_id ON requirement_custom_field_values(admin_id);
CREATE INDEX IF NOT EXISTS idx_rcfv_requirement ON requirement_custom_field_values(requirement_id);
CREATE INDEX IF NOT EXISTS idx_rcfv_field ON requirement_custom_field_values(field_id);

-- ============================================================
-- 工作空间设置表（工作空间级，每个工作空间一行）
-- 记录需求/缺陷的初始状态与结束状态配置
-- ============================================================
CREATE TABLE IF NOT EXISTS workspaces_settings (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    requirement_initial_status_id INTEGER NOT NULL DEFAULT 0,
    defect_initial_status_id INTEGER NOT NULL DEFAULT 0,
    requirement_final_status_ids VARCHAR(255) NOT NULL DEFAULT '',
    defect_final_status_ids VARCHAR(255) NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, workspace_id)
);

COMMENT ON TABLE "workspaces_settings" IS '工作空间设置表（按工作空间存储需求/缺陷的初始状态与结束状态配置）';
COMMENT ON COLUMN "workspaces_settings"."id" IS '设置ID';
COMMENT ON COLUMN "workspaces_settings"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "workspaces_settings"."workspace_id" IS '所属工作空间ID（每个工作空间一行）';
COMMENT ON COLUMN "workspaces_settings"."requirement_initial_status_id" IS '需求初始状态ID（关联 workspace_statuses.id，创建工作空间时默认待规划状态）';
COMMENT ON COLUMN "workspaces_settings"."defect_initial_status_id" IS '缺陷初始状态ID（关联 workspace_statuses.id，创建工作空间时默认待规划状态）';
COMMENT ON COLUMN "workspaces_settings"."requirement_final_status_ids" IS '需求结束状态ID集合（逗号分隔的 workspace_statuses.id，默认已完成、已关闭、已驳回ID）';
COMMENT ON COLUMN "workspaces_settings"."defect_final_status_ids" IS '缺陷结束状态ID集合（逗号分隔的 workspace_statuses.id，默认已完成、已关闭、已驳回ID）';
COMMENT ON COLUMN "workspaces_settings"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "workspaces_settings"."updated_at" IS '更新时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_ws_admin_ws ON workspaces_settings(admin_id, workspace_id);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

DROP TABLE IF EXISTS workspaces_settings;
DROP TABLE IF EXISTS requirement_custom_field_values;
DROP TABLE IF EXISTS workspace_custom_fields;
DROP TABLE IF EXISTS workspace_field_configs;
DROP TABLE IF EXISTS workspace_status_transitions;
DROP TABLE IF EXISTS workspace_priorities;
DROP TABLE IF EXISTS workspace_statuses;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS versions;
DROP TABLE IF EXISTS requirement_attachments;
DROP TABLE IF EXISTS requirement_histories;
DROP TABLE IF EXISTS requirement_comments;
DROP TABLE IF EXISTS requirements;
DROP TABLE IF EXISTS workspace_members;
DROP TABLE IF EXISTS workspaces;
DROP TABLE IF EXISTS users;

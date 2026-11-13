-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- 需求/缺陷视图：视图定义表、用户视图配置表、内置视图补种、权限补齐
-- ============================================================

-- 视图定义（系统视图对项目全员可见，个人视图仅创建者可见）。
CREATE TABLE IF NOT EXISTS workspace_views (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    type_key VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL DEFAULT '',
    view_type VARCHAR(20) NOT NULL DEFAULT 'personal',
    builtin_key VARCHAR(30) NOT NULL DEFAULT '',
    filters TEXT NOT NULL DEFAULT '',
    created_by INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE "workspace_views" IS '需求/缺陷列表视图定义表';
COMMENT ON COLUMN "workspace_views"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "workspace_views"."workspace_id" IS '所属工作空间ID';
COMMENT ON COLUMN "workspace_views"."type_key" IS '工作项类型: requirement=需求, defect=缺陷';
COMMENT ON COLUMN "workspace_views"."name" IS '视图名称（内置视图存 i18n key，用户命名存字面值）';
COMMENT ON COLUMN "workspace_views"."view_type" IS '视图类型: system=系统视图, personal=个人视图';
COMMENT ON COLUMN "workspace_views"."builtin_key" IS '内置视图标识: all/my_assigned/my_created/unfinished/finished，空=非内置；all 不允许编辑删除';
COMMENT ON COLUMN "workspace_views"."filters" IS '数据范围条件组JSON（空串=无条件）；值支持动态占位 __me__/__all_end__/__all_not_end__';
COMMENT ON COLUMN "workspace_views"."created_by" IS '创建人用户ID（内置视图为0）';
COMMENT ON COLUMN "workspace_views"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "workspace_views"."updated_at" IS '更新时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_workspace_views_ws ON workspace_views(admin_id, workspace_id, type_key);

-- 同一空间同一类型下内置视图唯一，保证补种与建空间种子幂等。
CREATE UNIQUE INDEX IF NOT EXISTS uniq_workspace_views_builtin
    ON workspace_views(admin_id, workspace_id, type_key, builtin_key)
    WHERE builtin_key <> '';

-- 用户维度的视图配置（默认视图、启用状态、个人视图排序、自动记忆开关）。
CREATE TABLE IF NOT EXISTS user_view_settings (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    user_id INTEGER NOT NULL DEFAULT 0,
    type_key VARCHAR(50) NOT NULL,
    default_view_id INTEGER NOT NULL DEFAULT 0,
    disabled_view_ids TEXT NOT NULL DEFAULT '',
    personal_view_order TEXT NOT NULL DEFAULT '',
    remember_last SMALLINT NOT NULL DEFAULT 1,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, workspace_id, user_id, type_key)
);

COMMENT ON TABLE "user_view_settings" IS '用户视图配置表（按用户+空间+工作项类型隔离）';
COMMENT ON COLUMN "user_view_settings"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "user_view_settings"."workspace_id" IS '所属工作空间ID';
COMMENT ON COLUMN "user_view_settings"."user_id" IS '配置归属用户ID';
COMMENT ON COLUMN "user_view_settings"."type_key" IS '工作项类型: requirement=需求, defect=缺陷';
COMMENT ON COLUMN "user_view_settings"."default_view_id" IS '默认视图ID（0=未设置，回退到内置「所有的」）';
COMMENT ON COLUMN "user_view_settings"."disabled_view_ids" IS '停用的视图ID集合（英文逗号分隔；默认全部启用，新系统视图对所有用户默认开启）';
COMMENT ON COLUMN "user_view_settings"."personal_view_order" IS '个人视图拖拽排序ID列表（英文逗号分隔）';
COMMENT ON COLUMN "user_view_settings"."remember_last" IS '自动记忆上次查看的视图: 1=开启（默认）, 0=关闭';
COMMENT ON COLUMN "user_view_settings"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "user_view_settings"."updated_at" IS '更新时间（Unix时间戳）';

-- +goose StatementEnd

-- +goose StatementBegin
-- ============================================================
-- 存量数据补种：为已存在的工作空间补齐需求/缺陷各 5 个内置视图
-- ============================================================

INSERT INTO workspace_views (admin_id, workspace_id, type_key, name, view_type, builtin_key, filters, created_by, created_at, updated_at)
SELECT w.admin_id, w.id, t.type_key, v.name, 'system', v.builtin_key, v.filters, 0,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces w
CROSS JOIN (VALUES ('requirement'), ('defect')) AS t(type_key)
CROSS JOIN (VALUES
    ('${view_builtin_all}',         'all',         ''),
    ('${view_builtin_my_assigned}', 'my_assigned', '{"logic":"and","conditions":[{"field":"assignee","operator":"in","values":["__me__"]}]}'),
    ('${view_builtin_my_created}',  'my_created',  '{"logic":"and","conditions":[{"field":"creator","operator":"in","values":["__me__"]}]}'),
    ('${view_builtin_unfinished}',  'unfinished',  '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_not_end__"]}]}'),
    ('${view_builtin_finished}',    'finished',    '{"logic":"and","conditions":[{"field":"status","operator":"in","values":["__all_end__"]}]}')
) AS v(name, builtin_key, filters)
ON CONFLICT (admin_id, workspace_id, type_key, builtin_key) WHERE builtin_key <> '' DO NOTHING;

-- +goose StatementEnd

-- +goose StatementBegin
-- ============================================================
-- 权限补齐：存量项目管理员角色获得需求/缺陷的「新增/编辑系统视图」权限
-- （新建空间的项目角色由 permission.go 的 PresetRoles 种子负责）
-- ============================================================

INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, v.permission_key,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
CROSS JOIN (VALUES ('requirement:manage_system_view'), ('defect:manage_system_view')) AS v(permission_key)
WHERE r.scope = 'project'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_project_admin%'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DELETE FROM role_permissions rp
USING roles r
WHERE rp.role_id = r.id
  AND rp.admin_id = r.admin_id
  AND rp.permission_key IN ('requirement:manage_system_view', 'defect:manage_system_view')
  AND r.scope = 'project'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_project_admin%';

DROP TABLE IF EXISTS user_view_settings;
DROP TABLE IF EXISTS workspace_views;

-- +goose StatementEnd

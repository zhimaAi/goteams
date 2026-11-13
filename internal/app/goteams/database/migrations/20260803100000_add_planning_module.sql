-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- 规划模块：分组表、关联表、requirements 扩展列、初始/终态配置列
-- ============================================================

-- 规划分组（看板规划页顶部 Tab）。不设 is_default：删除规则只有“至少保留一个”。
CREATE TABLE IF NOT EXISTS planning_groups (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    name VARCHAR(100) NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE "planning_groups" IS '规划分组表（看板规划页顶部分组 Tab）';
COMMENT ON COLUMN "planning_groups"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "planning_groups"."workspace_id" IS '所属工作空间ID';
COMMENT ON COLUMN "planning_groups"."name" IS '分组名称；系统种子分组存 i18n key，用户重命名后存字面值';

CREATE INDEX IF NOT EXISTS idx_planning_groups_admin_ws ON planning_groups(admin_id, workspace_id, id);

-- 规划与需求/缺陷的关联（多对多）。不冗余存目标类型，读取时 JOIN requirements.type。
CREATE TABLE IF NOT EXISTS planning_relations (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    planning_id INTEGER NOT NULL,
    item_id INTEGER NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, planning_id, item_id)
);

COMMENT ON TABLE "planning_relations" IS '规划关联工作项表（规划 ↔ 需求/缺陷）';
COMMENT ON COLUMN "planning_relations"."planning_id" IS '规划ID（requirements.id，type=planning）';
COMMENT ON COLUMN "planning_relations"."item_id" IS '被关联的工作项ID（requirements.id，type=requirement 或 defect）';

CREATE INDEX IF NOT EXISTS idx_planning_relations_planning ON planning_relations(admin_id, workspace_id, planning_id);
CREATE INDEX IF NOT EXISTS idx_planning_relations_item ON planning_relations(admin_id, item_id);

-- requirements 扩展列：仅规划行使用。
ALTER TABLE requirements ADD COLUMN IF NOT EXISTS group_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE requirements ADD COLUMN IF NOT EXISTS sort_order BIGINT NOT NULL DEFAULT 0;
ALTER TABLE requirements ADD COLUMN IF NOT EXISTS color VARCHAR(20) NOT NULL DEFAULT '';

COMMENT ON COLUMN "requirements"."group_id" IS '所属规划分组ID（仅 type=planning 使用，0=未分组）';
COMMENT ON COLUMN "requirements"."sort_order" IS '分组内排序值（仅 type=planning 使用，稀疏整数，步长65536）';
COMMENT ON COLUMN "requirements"."color" IS '标题文字颜色（仅规划使用，存十六进制色值）';

-- 部分索引：只收录规划行，需求/缺陷不进入此索引。
CREATE INDEX IF NOT EXISTS idx_req_planning_order
    ON requirements(admin_id, workspace_id, group_id, sort_order, id)
    WHERE type = 'planning';

-- 规划的初始/终态配置列。
ALTER TABLE workspaces_settings ADD COLUMN IF NOT EXISTS planning_initial_status_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE workspaces_settings ADD COLUMN IF NOT EXISTS planning_final_status_ids VARCHAR(255) NOT NULL DEFAULT '';

COMMENT ON COLUMN "workspaces_settings"."planning_initial_status_id" IS '规划的初始状态ID';
COMMENT ON COLUMN "workspaces_settings"."planning_final_status_ids" IS '规划的终态状态ID列表（英文逗号分隔）';

-- +goose StatementEnd

-- +goose StatementBegin
-- ============================================================
-- 存量数据补种：为已存在的工作空间补齐规划的状态/优先级/字段配置/默认分组
-- ============================================================

-- 规划状态（5 个）。
INSERT INTO workspace_statuses (admin_id, workspace_id, type_key, name, color, sort_order, is_default, created_at, updated_at)
SELECT w.admin_id, w.id, 'planning', v.name, v.color, v.sort_order, 1,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces w
CROSS JOIN (VALUES
    ('${workitem_status_planned}',     '#bfbfbf', 1),
    ('${workitem_status_designed}',    '#faad14', 2),
    ('${workitem_status_implemented}', '#1677ff', 3),
    ('${workitem_status_accepted}',    '#52c41a', 4),
    ('${workitem_status_closed}',      '#8c8c8c', 5)
) AS v(name, color, sort_order)
ON CONFLICT (admin_id, workspace_id, type_key, name) DO NOTHING;

-- 规划优先级（4 个，沿用需求/缺陷的既有配色，is_default 全为 0）。
INSERT INTO workspace_priorities (admin_id, workspace_id, type_key, name, color, sort_order, is_default, created_at, updated_at)
SELECT w.admin_id, w.id, 'planning', v.name, v.color, v.sort_order, 0,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces w
CROSS JOIN (VALUES
    ('${workitem_priority_urgent}', '#e54545', 1),
    ('${workitem_priority_high}',   '#ff8c00', 2),
    ('${workitem_priority_medium}', '#2d8cf0', 3),
    ('${workitem_priority_low}',    '#808695', 4)
) AS v(name, color, sort_order)
ON CONFLICT (admin_id, workspace_id, type_key, name) DO NOTHING;

-- 规划字段配置（10 条；id/created_at/created_by 默认关闭，title 锁定）。
INSERT INTO workspace_field_configs (admin_id, workspace_id, type_key, field_key, is_enabled, is_editable, is_locked, sort_order, created_at, updated_at)
SELECT w.admin_id, w.id, 'planning', v.field_key, v.is_enabled, v.is_editable, v.is_locked, v.sort_order,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces w
CROSS JOIN (VALUES
    ('title',              1, 0, 1, 0),
    ('priority',           1, 1, 0, 1),
    ('related_items',      1, 1, 0, 2),
    ('status',             1, 0, 0, 3),
    ('assignee',           1, 1, 0, 4),
    ('planned_start_date', 1, 1, 0, 5),
    ('planned_end_date',   1, 1, 0, 6),
    ('id',                 0, 0, 0, 7),
    ('created_at',         0, 0, 0, 8),
    ('created_by',         0, 0, 0, 9)
) AS v(field_key, is_enabled, is_editable, is_locked, sort_order)
ON CONFLICT (admin_id, workspace_id, type_key, field_key) DO NOTHING;

-- 每个工作空间一个默认分组（名称存 i18n key，读取时按语言替换）。
INSERT INTO planning_groups (admin_id, workspace_id, name, created_at, updated_at)
SELECT w.admin_id, w.id, '${planning_group_default}',
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces w
WHERE NOT EXISTS (
    SELECT 1 FROM planning_groups g WHERE g.admin_id = w.admin_id AND g.workspace_id = w.id
);

-- 规划的初始状态指向“已规划”。
UPDATE workspaces_settings s
SET planning_initial_status_id = st.id
FROM workspace_statuses st
WHERE st.admin_id = s.admin_id
  AND st.workspace_id = s.workspace_id
  AND st.type_key = 'planning'
  AND st.name = '${workitem_status_planned}'
  AND s.planning_initial_status_id = 0;

-- 项目管理员角色补齐规划的查看与编辑权限。
INSERT INTO role_permissions (admin_id, role_id, permission_key, created_at, updated_at)
SELECT r.admin_id, r.id, v.permission_key,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r
CROSS JOIN (VALUES ('planning:view'), ('planning:edit')) AS v(permission_key)
WHERE r.scope = 'project'
  AND r.is_system = 1
  AND r.name LIKE '%perm_role_project_admin%'
ON CONFLICT (admin_id, role_id, permission_key) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DELETE FROM role_permissions WHERE permission_key IN ('planning:view', 'planning:edit');

DELETE FROM workspace_field_configs WHERE type_key = 'planning';
DELETE FROM workspace_priorities WHERE type_key = 'planning';
DELETE FROM workspace_statuses WHERE type_key = 'planning';
DELETE FROM requirements WHERE type = 'planning';

ALTER TABLE workspaces_settings DROP COLUMN IF EXISTS planning_final_status_ids;
ALTER TABLE workspaces_settings DROP COLUMN IF EXISTS planning_initial_status_id;

DROP INDEX IF EXISTS idx_req_planning_order;
ALTER TABLE requirements DROP COLUMN IF EXISTS color;
ALTER TABLE requirements DROP COLUMN IF EXISTS sort_order;
ALTER TABLE requirements DROP COLUMN IF EXISTS group_id;

DROP TABLE IF EXISTS planning_relations;
DROP TABLE IF EXISTS planning_groups;

-- +goose StatementEnd

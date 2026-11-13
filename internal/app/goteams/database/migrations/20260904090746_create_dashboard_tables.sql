-- +goose Up
-- +goose StatementBegin

-- 项目仪表盘：共享盘面、单盘可见范围、额外编辑者、卡片、项目公告和用户个人筛选状态。
CREATE TABLE IF NOT EXISTS workspace_dashboards (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    name VARCHAR(100) NOT NULL DEFAULT '',
    visibility_type VARCHAR(20) NOT NULL DEFAULT 'public',
    sort_order INTEGER NOT NULL DEFAULT 0,
    creator_id INTEGER NOT NULL DEFAULT 0,
    revision BIGINT NOT NULL DEFAULT 1,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    deleted_at BIGINT NOT NULL DEFAULT 0,
    CHECK (visibility_type IN ('public', 'roles', 'members'))
);

CREATE INDEX IF NOT EXISTS idx_workspace_dashboards_list
    ON workspace_dashboards(admin_id, workspace_id, deleted_at, sort_order, id);

COMMENT ON TABLE workspace_dashboards IS '项目共享仪表盘';
COMMENT ON COLUMN workspace_dashboards.admin_id IS '租户 ID';
COMMENT ON COLUMN workspace_dashboards.workspace_id IS '工作空间 ID';
COMMENT ON COLUMN workspace_dashboards.visibility_type IS '可见范围: public=公开, roles=部分角色, members=部分成员';
COMMENT ON COLUMN workspace_dashboards.sort_order IS '项目内共享排序';
COMMENT ON COLUMN workspace_dashboards.creator_id IS '仪表盘创建人，默认具有查看、编辑和管理单盘权限';
COMMENT ON COLUMN workspace_dashboards.revision IS '共享配置乐观锁版本';
COMMENT ON COLUMN workspace_dashboards.deleted_at IS '软删除时间，0 表示有效';

CREATE TABLE IF NOT EXISTS dashboard_visibility_roles (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    dashboard_id INTEGER NOT NULL,
    role_id INTEGER NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_dashboard_visibility_roles_scope
    ON dashboard_visibility_roles(admin_id, workspace_id, dashboard_id, role_id);
CREATE INDEX IF NOT EXISTS idx_dashboard_visibility_roles_role
    ON dashboard_visibility_roles(admin_id, workspace_id, role_id, dashboard_id);

COMMENT ON TABLE dashboard_visibility_roles IS '仪表盘部分角色可见范围';

CREATE TABLE IF NOT EXISTS dashboard_visibility_members (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    dashboard_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_dashboard_visibility_members_scope
    ON dashboard_visibility_members(admin_id, workspace_id, dashboard_id, user_id);
CREATE INDEX IF NOT EXISTS idx_dashboard_visibility_members_user
    ON dashboard_visibility_members(admin_id, workspace_id, user_id, dashboard_id);

COMMENT ON TABLE dashboard_visibility_members IS '仪表盘部分成员可见范围';

CREATE TABLE IF NOT EXISTS dashboard_editors (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    dashboard_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_dashboard_editors_scope
    ON dashboard_editors(admin_id, workspace_id, dashboard_id, user_id);
CREATE INDEX IF NOT EXISTS idx_dashboard_editors_user
    ON dashboard_editors(admin_id, workspace_id, user_id, dashboard_id);

COMMENT ON TABLE dashboard_editors IS '仪表盘额外编辑者；项目管理员和创建人不重复落库';

CREATE TABLE IF NOT EXISTS dashboard_cards (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    dashboard_id INTEGER NOT NULL,
    card_type VARCHAR(32) NOT NULL,
    title VARCHAR(100) NOT NULL DEFAULT '',
    config TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    content_mode VARCHAR(20) NOT NULL DEFAULT 'rich-text',
    grid_x INTEGER NOT NULL DEFAULT 0,
    grid_y INTEGER NOT NULL DEFAULT 0,
    grid_width INTEGER NOT NULL DEFAULT 1,
    grid_height INTEGER NOT NULL DEFAULT 1,
    revision BIGINT NOT NULL DEFAULT 1,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    deleted_at BIGINT NOT NULL DEFAULT 0,
    CHECK (card_type IN ('metric', 'project_info', 'announcement', 'text')),
    CHECK (content_mode = 'rich-text'),
    CHECK (grid_x >= 0 AND grid_x < 12 AND grid_y >= 0 AND grid_y < 1000000
        AND grid_width >= 1 AND grid_width <= 12 AND grid_height >= 1 AND grid_height <= 1000000
        AND grid_x + grid_width <= 12 AND grid_y::BIGINT + grid_height::BIGINT <= 1000000)
);

CREATE INDEX IF NOT EXISTS idx_dashboard_cards_list
    ON dashboard_cards(admin_id, workspace_id, dashboard_id, deleted_at, grid_y, grid_x, id);
CREATE UNIQUE INDEX IF NOT EXISTS uk_dashboard_cards_announcement
    ON dashboard_cards(admin_id, workspace_id, dashboard_id)
    WHERE card_type = 'announcement' AND deleted_at = 0;

COMMENT ON TABLE dashboard_cards IS '仪表盘卡片';
COMMENT ON COLUMN dashboard_cards.config IS '卡片配置 JSON；数字指标含指标、数据范围和卡片条件';
COMMENT ON COLUMN dashboard_cards.content IS '文本卡片富文本正文；公告正文存 dashboard_announcements';
COMMENT ON COLUMN dashboard_cards.grid_x IS '12 栅格横坐标，从 0 开始';
COMMENT ON COLUMN dashboard_cards.grid_y IS '12 栅格纵坐标，从 0 开始';
COMMENT ON COLUMN dashboard_cards.revision IS '卡片乐观锁版本';

CREATE TABLE IF NOT EXISTS dashboard_announcements (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    content_mode VARCHAR(20) NOT NULL DEFAULT 'rich-text',
    revision BIGINT NOT NULL DEFAULT 1,
    created_by INTEGER NOT NULL DEFAULT 0,
    updated_by INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    CHECK (content_mode = 'rich-text')
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_dashboard_announcements_workspace
    ON dashboard_announcements(admin_id, workspace_id);

COMMENT ON TABLE dashboard_announcements IS '项目级唯一仪表盘公告，所有公告卡共享此内容';

CREATE TABLE IF NOT EXISTS dashboard_user_states (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    dashboard_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    filters TEXT NOT NULL DEFAULT '',
    filters_collapsed SMALLINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    CHECK (filters_collapsed IN (0, 1))
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_dashboard_user_states_scope
    ON dashboard_user_states(admin_id, workspace_id, dashboard_id, user_id);
CREATE INDEX IF NOT EXISTS idx_dashboard_user_states_user
    ON dashboard_user_states(admin_id, workspace_id, user_id, dashboard_id);

COMMENT ON TABLE dashboard_user_states IS '用户在单个仪表盘上的个人筛选与折叠状态，不参与另存为和项目模板复制';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS dashboard_user_states;
DROP TABLE IF EXISTS dashboard_announcements;
DROP TABLE IF EXISTS dashboard_cards;
DROP TABLE IF EXISTS dashboard_editors;
DROP TABLE IF EXISTS dashboard_visibility_members;
DROP TABLE IF EXISTS dashboard_visibility_roles;
DROP TABLE IF EXISTS workspace_dashboards;

-- +goose StatementEnd

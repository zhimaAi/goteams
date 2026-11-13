-- +goose Up
-- +goose StatementBegin

-- 知识库一期表结构：项目应用启停、目录树元数据、文档/链接当前态、
-- 历史版本元数据与内容快照、收藏、动态。附件复用通用 storage_attachments 表。

-- 1. workspace_applications — 项目应用启停配置
CREATE TABLE workspace_applications (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    app_key VARCHAR(50) NOT NULL,
    enabled SMALLINT NOT NULL DEFAULT 0,
    enabled_by INTEGER NOT NULL DEFAULT 0,
    enabled_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX uk_workspace_applications
    ON workspace_applications(admin_id, workspace_id, app_key);

COMMENT ON TABLE workspace_applications IS '项目应用启停配置';
COMMENT ON COLUMN workspace_applications.admin_id IS '租户 ID';
COMMENT ON COLUMN workspace_applications.workspace_id IS '项目 ID';
COMMENT ON COLUMN workspace_applications.app_key IS '应用标识，一期为 knowledge_base';
COMMENT ON COLUMN workspace_applications.enabled IS '1=开启，0=关闭，无记录等价于关闭';
COMMENT ON COLUMN workspace_applications.enabled_by IS '最近执行开启/关闭的用户 ID';
COMMENT ON COLUMN workspace_applications.enabled_at IS '最近一次开启时间，关闭时保留（Unix 秒）';
COMMENT ON COLUMN workspace_applications.updated_at IS '最近配置时间（Unix 秒）';

-- 2. knowledge_items — 目录树元数据（文件夹/文档/链接共用）
CREATE TABLE knowledge_items (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    parent_id INTEGER NOT NULL DEFAULT 0,
    item_type VARCHAR(20) NOT NULL,
    title VARCHAR(200) NOT NULL DEFAULT '',
    revision BIGINT NOT NULL DEFAULT 1,
    version_seq BIGINT NOT NULL DEFAULT 1,
    creator_id INTEGER NOT NULL,
    updated_by INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    deleted_at BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX idx_knowledge_items_tree
    ON knowledge_items(admin_id, workspace_id, parent_id, deleted_at, item_type);

CREATE INDEX idx_knowledge_items_creator
    ON knowledge_items(admin_id, workspace_id, creator_id, deleted_at, created_at DESC);

CREATE INDEX idx_knowledge_items_updated
    ON knowledge_items(admin_id, workspace_id, deleted_at, updated_at DESC);

COMMENT ON TABLE knowledge_items IS '知识库目录树元数据';
COMMENT ON COLUMN knowledge_items.admin_id IS '租户 ID';
COMMENT ON COLUMN knowledge_items.workspace_id IS '项目 ID';
COMMENT ON COLUMN knowledge_items.parent_id IS '父节点 ID，根目录为 0';
COMMENT ON COLUMN knowledge_items.item_type IS '节点类型：folder=文件夹，document=文本文档，link=链接文档';
COMMENT ON COLUMN knowledge_items.title IS '节点名称';
COMMENT ON COLUMN knowledge_items.revision IS '当前态版本号，创建时为 1，自动保存/恢复时递增';
COMMENT ON COLUMN knowledge_items.version_seq IS '已创建历史版本最大序号，创建时为 1，手动版本/恢复时递增';
COMMENT ON COLUMN knowledge_items.creator_id IS '创建人用户 ID，用于自己创建权限';
COMMENT ON COLUMN knowledge_items.updated_by IS '最近修改人用户 ID';
COMMENT ON COLUMN knowledge_items.created_at IS '创建时间（Unix 秒）';
COMMENT ON COLUMN knowledge_items.updated_at IS '更新时间（Unix 秒）';
COMMENT ON COLUMN knowledge_items.deleted_at IS '软删除：0=未删除，非0=删除时间';

-- 3. knowledge_document_contents — 文本文档当前态内容
CREATE TABLE knowledge_document_contents (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    document_id INTEGER NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    content_mode VARCHAR(20) NOT NULL DEFAULT 'rich-text',
    content_hash CHAR(64) NOT NULL DEFAULT '',
    updated_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX uk_knowledge_contents_document
    ON knowledge_document_contents(admin_id, document_id);

COMMENT ON TABLE knowledge_document_contents IS '文本文档当前态内容';
COMMENT ON COLUMN knowledge_document_contents.admin_id IS '租户 ID';
COMMENT ON COLUMN knowledge_document_contents.workspace_id IS '项目 ID';
COMMENT ON COLUMN knowledge_document_contents.document_id IS '对应 knowledge_items.id，仅 item_type=document 有行';
COMMENT ON COLUMN knowledge_document_contents.content IS '文本文档当前正文（HTML 或 markdown 原文）';
COMMENT ON COLUMN knowledge_document_contents.content_mode IS '内容模式：rich-text/markdown';
COMMENT ON COLUMN knowledge_document_contents.content_hash IS 'SHA-256(title+content_mode+content)，title 读 items 表';
COMMENT ON COLUMN knowledge_document_contents.updated_at IS '更新时间（Unix 秒）';

-- 4. knowledge_link_contents — 链接文档当前态内容
CREATE TABLE knowledge_link_contents (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    link_id INTEGER NOT NULL,
    source_url TEXT NOT NULL DEFAULT '',
    content_hash CHAR(64) NOT NULL DEFAULT '',
    updated_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX uk_knowledge_link_contents
    ON knowledge_link_contents(admin_id, link_id);

COMMENT ON TABLE knowledge_link_contents IS '链接文档当前态内容';
COMMENT ON COLUMN knowledge_link_contents.admin_id IS '租户 ID';
COMMENT ON COLUMN knowledge_link_contents.workspace_id IS '项目 ID';
COMMENT ON COLUMN knowledge_link_contents.link_id IS '对应 knowledge_items.id，仅 item_type=link 有行';
COMMENT ON COLUMN knowledge_link_contents.source_url IS '链接 URL';
COMMENT ON COLUMN knowledge_link_contents.content_hash IS 'SHA-256(title+source_url)，title 读 items 表';
COMMENT ON COLUMN knowledge_link_contents.updated_at IS '更新时间（Unix 秒）';

-- 5. knowledge_document_versions — 历史版本元数据
CREATE TABLE knowledge_document_versions (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    document_id INTEGER NOT NULL,
    version_no BIGINT NOT NULL,
    version_type VARCHAR(20) NOT NULL,
    description VARCHAR(500) NOT NULL DEFAULT '',
    restore_from_version_id BIGINT NOT NULL DEFAULT 0,
    edit_session_id VARCHAR(64) NOT NULL DEFAULT '',
    created_by INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX uk_knowledge_version_no
    ON knowledge_document_versions(admin_id, document_id, version_no);

CREATE INDEX idx_knowledge_versions_list
    ON knowledge_document_versions(admin_id, workspace_id, document_id, version_no DESC);

COMMENT ON TABLE knowledge_document_versions IS '知识库文档历史版本元数据';
COMMENT ON COLUMN knowledge_document_versions.admin_id IS '租户 ID';
COMMENT ON COLUMN knowledge_document_versions.workspace_id IS '项目 ID';
COMMENT ON COLUMN knowledge_document_versions.document_id IS '对应 knowledge_items.id';
COMMENT ON COLUMN knowledge_document_versions.version_no IS '文档内单调递增版本号，展示为 v1/v2/...';
COMMENT ON COLUMN knowledge_document_versions.version_type IS '版本类型：create=创建，manual=手动保存，restore=恢复';
COMMENT ON COLUMN knowledge_document_versions.description IS '手动备注，其他类型通常为空';
COMMENT ON COLUMN knowledge_document_versions.restore_from_version_id IS '恢复来源版本 ID，非恢复版本为 0';
COMMENT ON COLUMN knowledge_document_versions.edit_session_id IS '触发该版本的前端编辑会话 ID';
COMMENT ON COLUMN knowledge_document_versions.created_by IS '操作人用户 ID';
COMMENT ON COLUMN knowledge_document_versions.created_at IS '创建时间（Unix 秒）';

-- 6. knowledge_document_version_contents — 历史版本内容快照
CREATE TABLE knowledge_document_version_contents (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    version_id INTEGER NOT NULL,
    title VARCHAR(200) NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    content_mode VARCHAR(20) NOT NULL DEFAULT '',
    content_hash CHAR(64) NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX uk_knowledge_version_contents
    ON knowledge_document_version_contents(admin_id, version_id);

COMMENT ON TABLE knowledge_document_version_contents IS '知识库文档历史版本内容快照';
COMMENT ON COLUMN knowledge_document_version_contents.admin_id IS '租户 ID';
COMMENT ON COLUMN knowledge_document_version_contents.workspace_id IS '项目 ID';
COMMENT ON COLUMN knowledge_document_version_contents.version_id IS '对应 knowledge_document_versions.id';
COMMENT ON COLUMN knowledge_document_version_contents.title IS '标题快照';
COMMENT ON COLUMN knowledge_document_version_contents.content IS '正文完整快照';
COMMENT ON COLUMN knowledge_document_version_contents.content_mode IS '内容模式快照：rich-text/markdown';
COMMENT ON COLUMN knowledge_document_version_contents.content_hash IS '快照 SHA-256';

-- 7. knowledge_favorites — 收藏
CREATE TABLE knowledge_favorites (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    item_id INTEGER NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, workspace_id, user_id, item_id)
);

COMMENT ON TABLE knowledge_favorites IS '知识库收藏，每个用户对每个节点最多一条';
COMMENT ON COLUMN knowledge_favorites.admin_id IS '租户 ID';
COMMENT ON COLUMN knowledge_favorites.workspace_id IS '项目 ID';
COMMENT ON COLUMN knowledge_favorites.user_id IS '收藏用户 ID';
COMMENT ON COLUMN knowledge_favorites.item_id IS '收藏的 knowledge_items.id';
COMMENT ON COLUMN knowledge_favorites.created_at IS '收藏时间（Unix 秒）';

-- 8. knowledge_activities — 主页动态
CREATE TABLE knowledge_activities (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    item_id INTEGER NOT NULL DEFAULT 0,
    item_title VARCHAR(200) NOT NULL DEFAULT '',
    item_type VARCHAR(20) NOT NULL DEFAULT '',
    user_id INTEGER NOT NULL,
    action VARCHAR(20) NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX idx_knowledge_activities_list
    ON knowledge_activities(admin_id, workspace_id, created_at DESC);

COMMENT ON TABLE knowledge_activities IS '知识库动态，主页动态 tab 数据源';
COMMENT ON COLUMN knowledge_activities.admin_id IS '租户 ID';
COMMENT ON COLUMN knowledge_activities.workspace_id IS '项目 ID';
COMMENT ON COLUMN knowledge_activities.item_id IS '关联的 knowledge_items.id';
COMMENT ON COLUMN knowledge_activities.item_title IS '动作时的标题快照，删除后仍可展示动态';
COMMENT ON COLUMN knowledge_activities.item_type IS '动作时的节点类型快照：folder/document/link';
COMMENT ON COLUMN knowledge_activities.user_id IS '操作人用户 ID';
COMMENT ON COLUMN knowledge_activities.action IS '动作：create/update/restore/delete';
COMMENT ON COLUMN knowledge_activities.created_at IS '动态时间（Unix 秒）';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS knowledge_activities;
DROP TABLE IF EXISTS knowledge_favorites;
DROP TABLE IF EXISTS knowledge_document_version_contents;
DROP TABLE IF EXISTS knowledge_document_versions;
DROP TABLE IF EXISTS knowledge_link_contents;
DROP TABLE IF EXISTS knowledge_document_contents;
DROP TABLE IF EXISTS knowledge_items;
DROP TABLE IF EXISTS workspace_applications;

-- +goose StatementEnd

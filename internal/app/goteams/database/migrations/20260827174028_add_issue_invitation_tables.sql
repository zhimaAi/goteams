-- +goose Up

-- 项目邀填工单的通用邀请配置。应用启停仍统一存放在 workspace_applications（app_key=issue）。
CREATE TABLE IF NOT EXISTS workspace_issue_invites (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    invite_key VARCHAR(16) NOT NULL,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE "workspace_issue_invites" IS '项目邀填工单通用邀请配置';
COMMENT ON COLUMN "workspace_issue_invites"."id" IS '邀请配置ID';
COMMENT ON COLUMN "workspace_issue_invites"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "workspace_issue_invites"."workspace_id" IS '所属工作空间ID';
COMMENT ON COLUMN "workspace_issue_invites"."invite_key" IS '公开链接使用的全局唯一16位随机Key';
COMMENT ON COLUMN "workspace_issue_invites"."created_by" IS '首次开启应用并创建邀请配置的用户ID';
COMMENT ON COLUMN "workspace_issue_invites"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "workspace_issue_invites"."updated_at" IS '更新时间（Unix时间戳）';

CREATE UNIQUE INDEX IF NOT EXISTS uk_workspace_issue_invites_scope
    ON workspace_issue_invites(admin_id, workspace_id);
CREATE UNIQUE INDEX IF NOT EXISTS uk_workspace_issue_invites_key
    ON workspace_issue_invites(invite_key);

-- 邀请模板副本使用独立公开Key；已有的普通需求/缺陷模板保持空Key，不可被公开链接访问。
ALTER TABLE work_item_templates
    ADD COLUMN IF NOT EXISTS invite_key VARCHAR(16) NOT NULL DEFAULT '';

COMMENT ON COLUMN "work_item_templates"."invite_key" IS '邀填工单模板副本的全局唯一16位公开Key，普通模板为空';

CREATE UNIQUE INDEX IF NOT EXISTS uk_work_item_templates_invite_key
    ON work_item_templates(invite_key)
    WHERE invite_key <> '';

-- 项目邀请配置与模板副本的关联。source_template_id只用于防止重复引入，不参与后续同步。
CREATE TABLE IF NOT EXISTS workspace_issue_item_templates (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    template_id INTEGER NOT NULL DEFAULT 0,
    source_template_id INTEGER NOT NULL DEFAULT 0,
    enabled SMALLINT NOT NULL DEFAULT 1,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_by INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE "workspace_issue_item_templates" IS '项目邀填工单模板副本关联';
COMMENT ON COLUMN "workspace_issue_item_templates"."id" IS '关联ID';
COMMENT ON COLUMN "workspace_issue_item_templates"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "workspace_issue_item_templates"."workspace_id" IS '所属工作空间ID';
COMMENT ON COLUMN "workspace_issue_item_templates"."template_id" IS '复制后独立的邀请模板ID';
COMMENT ON COLUMN "workspace_issue_item_templates"."source_template_id" IS '引入时的来源模板ID，仅用于项目内防重复引入，不用于数据同步';
COMMENT ON COLUMN "workspace_issue_item_templates"."enabled" IS '是否允许外部用户使用: 1=启用, 0=停用';
COMMENT ON COLUMN "workspace_issue_item_templates"."sort_order" IS '邀请模板展示顺序';
COMMENT ON COLUMN "workspace_issue_item_templates"."created_by" IS '执行模板引入的用户ID';
COMMENT ON COLUMN "workspace_issue_item_templates"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "workspace_issue_item_templates"."updated_at" IS '更新时间（Unix时间戳）';

CREATE UNIQUE INDEX IF NOT EXISTS uk_workspace_issue_item_templates_template
    ON workspace_issue_item_templates(admin_id, workspace_id, template_id);
CREATE UNIQUE INDEX IF NOT EXISTS uk_workspace_issue_item_templates_source
    ON workspace_issue_item_templates(admin_id, workspace_id, source_template_id);
CREATE UNIQUE INDEX IF NOT EXISTS uk_workspace_issue_item_templates_private_copy
    ON workspace_issue_item_templates(admin_id, template_id);
CREATE INDEX IF NOT EXISTS idx_workspace_issue_item_templates_list
    ON workspace_issue_item_templates(admin_id, workspace_id, enabled, sort_order, id);

-- 外部身份与工单关联。模板信息保存提交时快照，移除模板副本后历史记录仍可展示。
CREATE TABLE IF NOT EXISTS workspace_issue_requirements (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    openid VARCHAR(128) NOT NULL,
    requirement_id INTEGER NOT NULL DEFAULT 0,
    template_id INTEGER NOT NULL DEFAULT 0,
    template_key_snapshot VARCHAR(16) NOT NULL DEFAULT '',
    template_type_key VARCHAR(32) NOT NULL DEFAULT '',
    template_name VARCHAR(50) NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE "workspace_issue_requirements" IS '外部openid、邀填工单及提交模板快照关联';
COMMENT ON COLUMN "workspace_issue_requirements"."id" IS '关联ID';
COMMENT ON COLUMN "workspace_issue_requirements"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "workspace_issue_requirements"."workspace_id" IS '所属工作空间ID';
COMMENT ON COLUMN "workspace_issue_requirements"."openid" IS '外部用户标识，允许英文字母、数字、横杠、下划线和英文句点';
COMMENT ON COLUMN "workspace_issue_requirements"."requirement_id" IS 'requirements表中的邀填工单ID';
COMMENT ON COLUMN "workspace_issue_requirements"."template_id" IS '提交时使用的邀请模板副本ID，模板删除后作为快照标识保留';
COMMENT ON COLUMN "workspace_issue_requirements"."template_key_snapshot" IS '提交时使用的模板公开Key快照';
COMMENT ON COLUMN "workspace_issue_requirements"."template_type_key" IS '提交时的来源类型快照: requirement=需求, defect=缺陷';
COMMENT ON COLUMN "workspace_issue_requirements"."template_name" IS '提交时的模板名称快照';
COMMENT ON COLUMN "workspace_issue_requirements"."created_at" IS '提交时间（Unix时间戳）';
COMMENT ON COLUMN "workspace_issue_requirements"."updated_at" IS '关联更新时间（Unix时间戳）';

CREATE UNIQUE INDEX IF NOT EXISTS uk_workspace_issue_requirements_requirement
    ON workspace_issue_requirements(admin_id, workspace_id, requirement_id);
CREATE INDEX IF NOT EXISTS idx_workspace_issue_requirements_history
    ON workspace_issue_requirements(admin_id, workspace_id, openid, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_workspace_issue_requirements_template
    ON workspace_issue_requirements(admin_id, workspace_id, template_id);

-- 邀填工单没有项目成员创建人；项目内创建的原有数据保持不变。
ALTER TABLE requirements
    ALTER COLUMN created_by DROP NOT NULL;

COMMENT ON COLUMN "requirements"."created_by" IS '创建者用户ID，外部邀填工单允许为空';

-- +goose Down

-- 回滚前将邀填工单允许的空创建人恢复为系统占位值，满足旧结构的非空约束。
UPDATE requirements
SET created_by = 0
WHERE created_by IS NULL;

ALTER TABLE requirements
    ALTER COLUMN created_by SET NOT NULL;

COMMENT ON COLUMN "requirements"."created_by" IS '创建者用户ID';

DROP TABLE IF EXISTS workspace_issue_requirements;
DROP TABLE IF EXISTS workspace_issue_item_templates;

DROP INDEX IF EXISTS uk_work_item_templates_invite_key;
ALTER TABLE work_item_templates
    DROP COLUMN IF EXISTS invite_key;

DROP TABLE IF EXISTS workspace_issue_invites;

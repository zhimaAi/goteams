-- +goose Up

-- 租户级工作项模板池（工作空间只保存关联引用）。
CREATE TABLE IF NOT EXISTS work_item_templates (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    type_key VARCHAR(32) NOT NULL DEFAULT '',
    name VARCHAR(50) NOT NULL DEFAULT '',
    title VARCHAR(50) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    description_mode VARCHAR(32) NOT NULL DEFAULT 'rich-text',
    fields_json TEXT NOT NULL DEFAULT '[]',
    created_by INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE "work_item_templates" IS '工作项模板池（租户级，供各工作空间关联复用）';
COMMENT ON COLUMN "work_item_templates"."id" IS '模板ID';
COMMENT ON COLUMN "work_item_templates"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "work_item_templates"."type_key" IS '工作项类型: requirement=需求, defect=缺陷';
COMMENT ON COLUMN "work_item_templates"."name" IS '模板名称';
COMMENT ON COLUMN "work_item_templates"."title" IS '创建时带出的默认标题';
COMMENT ON COLUMN "work_item_templates"."description" IS '创建时带出的默认描述';
COMMENT ON COLUMN "work_item_templates"."description_mode" IS '描述编辑器模式: rich-text / markdown';
COMMENT ON COLUMN "work_item_templates"."fields_json" IS '字段配置JSON：[{field_key,default_value,required}]';
COMMENT ON COLUMN "work_item_templates"."created_by" IS '创建人用户ID';
COMMENT ON COLUMN "work_item_templates"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "work_item_templates"."updated_at" IS '更新时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_wit_admin_type ON work_item_templates(admin_id, type_key);

-- 工作空间与模板池的关联（取消关联不删除池内模板）。
CREATE TABLE IF NOT EXISTS workspace_work_item_templates (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    template_id INTEGER NOT NULL DEFAULT 0,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, workspace_id, template_id)
);

COMMENT ON TABLE "workspace_work_item_templates" IS '工作空间已关联的工作项模板';
COMMENT ON COLUMN "workspace_work_item_templates"."id" IS '关联ID';
COMMENT ON COLUMN "workspace_work_item_templates"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "workspace_work_item_templates"."workspace_id" IS '所属工作空间ID';
COMMENT ON COLUMN "workspace_work_item_templates"."template_id" IS '模板池中的模板ID';
COMMENT ON COLUMN "workspace_work_item_templates"."sort_order" IS '关联列表排序';
COMMENT ON COLUMN "workspace_work_item_templates"."created_at" IS '关联时间（Unix时间戳）';

CREATE INDEX IF NOT EXISTS idx_wwit_ws ON workspace_work_item_templates(admin_id, workspace_id);
CREATE INDEX IF NOT EXISTS idx_wwit_template ON workspace_work_item_templates(admin_id, template_id);

ALTER TABLE workspaces_settings
    ADD COLUMN IF NOT EXISTS requirement_template_enabled SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE workspaces_settings
    ADD COLUMN IF NOT EXISTS defect_template_enabled SMALLINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN "workspaces_settings"."requirement_template_enabled" IS '需求模板功能开关: 1=开启, 0=关闭';
COMMENT ON COLUMN "workspaces_settings"."defect_template_enabled" IS '缺陷模板功能开关: 1=开启, 0=关闭';

-- +goose Down

ALTER TABLE workspaces_settings DROP COLUMN IF EXISTS defect_template_enabled;
ALTER TABLE workspaces_settings DROP COLUMN IF EXISTS requirement_template_enabled;
DROP TABLE IF EXISTS workspace_work_item_templates;
DROP TABLE IF EXISTS work_item_templates;

-- +goose Up
-- +goose StatementBegin

-- 工作项属性（工作空间级，按类型区分需求/缺陷）：项目设置 > 内置应用 > 需求/缺陷设置 > 工作项属性。
-- 无记录表示未配置，名称/图标走类型内置默认（需求=蓝底“需求”、缺陷=红底“缺陷”，名称走 i18n），
-- 用户首次保存时才写入该行，因此迁移不预置种子数据。
CREATE TABLE IF NOT EXISTS workspace_work_item_attributes (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    type_key VARCHAR(50) NOT NULL DEFAULT '',
    name VARCHAR(50) NOT NULL DEFAULT '',
    icon_color VARCHAR(20) NOT NULL DEFAULT '',
    icon_text VARCHAR(10) NOT NULL DEFAULT '',
    description VARCHAR(200) NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, workspace_id, type_key)
);

COMMENT ON TABLE "workspace_work_item_attributes" IS '工作项属性表（工作空间级，需求/缺陷的名称、图标与类别说明；未配置时不落行）';
COMMENT ON COLUMN "workspace_work_item_attributes"."id" IS '属性ID';
COMMENT ON COLUMN "workspace_work_item_attributes"."admin_id" IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN "workspace_work_item_attributes"."workspace_id" IS '所属工作空间ID';
COMMENT ON COLUMN "workspace_work_item_attributes"."type_key" IS '工作项类型: requirement=需求, defect=缺陷';
COMMENT ON COLUMN "workspace_work_item_attributes"."name" IS '工作项展示名称（如需求/缺陷，界面输入上限20字符）';
COMMENT ON COLUMN "workspace_work_item_attributes"."icon_color" IS '图标底色（固定色板 hex，如 #3582fb）';
COMMENT ON COLUMN "workspace_work_item_attributes"."icon_text" IS '图标文字，界面输入上限2个字';
COMMENT ON COLUMN "workspace_work_item_attributes"."description" IS '类别说明（界面输入上限200字符）';
COMMENT ON COLUMN "workspace_work_item_attributes"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "workspace_work_item_attributes"."updated_at" IS '更新时间（Unix时间戳）';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS workspace_work_item_attributes;

-- +goose StatementEnd

-- +goose Up

-- 邀请模板副本的主题图片与引导文案改为按模板独立配置；项目级配置（workspace_issue_invites）保留，
-- 通用链接模板选择页继续使用项目级，模板级用于具体模板的填写表单页。
ALTER TABLE workspace_issue_item_templates
    ADD COLUMN IF NOT EXISTS theme_image VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS guide_title VARCHAR(20) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS guide_description VARCHAR(300) NOT NULL DEFAULT '';

COMMENT ON COLUMN "workspace_issue_item_templates"."theme_image" IS '模板邀请页主题图片相对URL，建议尺寸750*360，大小500k以内';
COMMENT ON COLUMN "workspace_issue_item_templates"."guide_title" IS '模板邀请页引导标题，最多20个字符';
COMMENT ON COLUMN "workspace_issue_item_templates"."guide_description" IS '模板邀请页引导描述，非必填，最多300个字符';

-- +goose Down

ALTER TABLE workspace_issue_item_templates
    DROP COLUMN IF EXISTS theme_image,
    DROP COLUMN IF EXISTS guide_title,
    DROP COLUMN IF EXISTS guide_description;

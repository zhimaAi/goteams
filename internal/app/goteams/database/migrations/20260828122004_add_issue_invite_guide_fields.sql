-- +goose Up

-- 邀填工单邀请页引导配置：主题图片（750*360，500k以内）、引导标题（20字）、引导描述（300字）。
ALTER TABLE workspace_issue_invites
    ADD COLUMN IF NOT EXISTS guide_title VARCHAR(20) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS guide_description VARCHAR(300) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS theme_image VARCHAR(255) NOT NULL DEFAULT '';

COMMENT ON COLUMN "workspace_issue_invites"."guide_title" IS '邀请页引导标题，最多20个字符';
COMMENT ON COLUMN "workspace_issue_invites"."guide_description" IS '邀请页引导描述，非必填，最多300个字符';
COMMENT ON COLUMN "workspace_issue_invites"."theme_image" IS '邀请页主题图片相对URL，建议尺寸750*360，大小500k以内';

-- +goose Down

ALTER TABLE workspace_issue_invites
    DROP COLUMN IF EXISTS theme_image,
    DROP COLUMN IF EXISTS guide_description,
    DROP COLUMN IF EXISTS guide_title;

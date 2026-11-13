-- +goose Up

-- 邀请文案独立启用开关：关闭后隐藏主题图片/引导标题/引导描述，仅显示文案标题和开关。
ALTER TABLE workspace_issue_invites
    ADD COLUMN IF NOT EXISTS enabled SMALLINT NOT NULL DEFAULT 1;

COMMENT ON COLUMN "workspace_issue_invites"."enabled" IS '邀请文案是否启用: 1=启用显示主题图片/引导标题/引导描述, 0=停用隐藏';

-- +goose Down

ALTER TABLE workspace_issue_invites
    DROP COLUMN IF EXISTS enabled;
-- +goose Up

-- 模板级邀请文案开关：列表页的邀请文案开关继续控制项目级配置（workspace_issue_invites.enabled），
-- 详情页的开关改由这一列独立保存，互不同步。存量模板默认开启，保持原有“配了文案就展示”的行为；
-- 新建模板由前端显式传 0，默认关闭。
ALTER TABLE workspace_issue_item_templates
    ADD COLUMN IF NOT EXISTS guide_enabled SMALLINT NOT NULL DEFAULT 1;

COMMENT ON COLUMN "workspace_issue_item_templates"."guide_enabled" IS '模板邀请文案开关: 1=开启, 0=关闭（新建默认关闭）';

-- +goose Down

ALTER TABLE workspace_issue_item_templates
    DROP COLUMN IF EXISTS guide_enabled;

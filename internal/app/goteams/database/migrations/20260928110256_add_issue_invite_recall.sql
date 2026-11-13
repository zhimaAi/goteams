-- +goose Up

-- 用户撤回与催单：模板侧存撤回开关和撤回后落到的状态，工单侧存撤回时间和最近一次催单时间。
-- 撤回状态放关联表是因为状态 id 是项目级的，关联表本身就带 workspace_id。
ALTER TABLE workspace_issue_item_templates
    ADD COLUMN IF NOT EXISTS recall_enabled SMALLINT NOT NULL DEFAULT 0;

ALTER TABLE workspace_issue_item_templates
    ADD COLUMN IF NOT EXISTS recall_status_id INTEGER NOT NULL DEFAULT 0;

COMMENT ON COLUMN "workspace_issue_item_templates"."recall_enabled" IS '用户撤回开关: 1=开启, 0=关闭（默认关闭）';
COMMENT ON COLUMN "workspace_issue_item_templates"."recall_status_id" IS '用户撤回后落到的状态ID，0=未配置';

ALTER TABLE workspace_issue_requirements
    ADD COLUMN IF NOT EXISTS recalled_at BIGINT NOT NULL DEFAULT 0;

ALTER TABLE workspace_issue_requirements
    ADD COLUMN IF NOT EXISTS last_urged_at BIGINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN "workspace_issue_requirements"."recalled_at" IS '最近一次用户撤回时间（Unix时间戳），0=未撤回或已重新提交';
COMMENT ON COLUMN "workspace_issue_requirements"."last_urged_at" IS '最近一次用户催单时间（Unix时间戳），0=从未催单';

-- +goose Down

ALTER TABLE workspace_issue_item_templates
    DROP COLUMN IF EXISTS recall_enabled;

ALTER TABLE workspace_issue_item_templates
    DROP COLUMN IF EXISTS recall_status_id;

ALTER TABLE workspace_issue_requirements
    DROP COLUMN IF EXISTS recalled_at;

ALTER TABLE workspace_issue_requirements
    DROP COLUMN IF EXISTS last_urged_at;

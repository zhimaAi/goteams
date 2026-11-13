-- +goose Up
-- +goose StatementBegin

-- 邀填提交扩展：小客服(xkf)链接携带的客户资料与填写来源。
-- extra_info 首个约定 key 为 xkf_customer_info = { 字段名: 值 }；其他来源系统按 <系统>_customer_info 追加。
ALTER TABLE workspace_issue_requirements
    ADD COLUMN IF NOT EXISTS extra_info JSONB,
    ADD COLUMN IF NOT EXISTS source VARCHAR(32) NOT NULL DEFAULT 'goteams_share';

COMMENT ON COLUMN "workspace_issue_requirements"."extra_info" IS '提交扩展信息（JSONB），约定 key：xkf_customer_info';
COMMENT ON COLUMN "workspace_issue_requirements"."source" IS '填写来源: goteams_share=goteams复制链接填写, xkf_receive=小客服链接填写';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE workspace_issue_requirements
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS extra_info;
-- +goose StatementEnd

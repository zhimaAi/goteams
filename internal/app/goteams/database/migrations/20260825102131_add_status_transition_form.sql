-- +goose Up
-- 高级状态流转表单：每个工作项类型一份 JSON，默认开启且预置处理人、评论（均非必填）
ALTER TABLE workspaces_settings
    ADD COLUMN IF NOT EXISTS requirement_status_transition TEXT NOT NULL DEFAULT '{"enabled":1,"fields":[{"key":"assignee","required":0},{"key":"comment","required":0}]}';
ALTER TABLE workspaces_settings
    ADD COLUMN IF NOT EXISTS defect_status_transition TEXT NOT NULL DEFAULT '{"enabled":1,"fields":[{"key":"assignee","required":0},{"key":"comment","required":0}]}';

COMMENT ON COLUMN "workspaces_settings"."requirement_status_transition" IS '需求高级状态流转配置 JSON：{"enabled":1,"fields":[{"key":"assignee","required":0}]}';
COMMENT ON COLUMN "workspaces_settings"."defect_status_transition" IS '缺陷高级状态流转配置 JSON：{"enabled":1,"fields":[{"key":"assignee","required":0}]}';

-- +goose Down
ALTER TABLE workspaces_settings DROP COLUMN IF EXISTS defect_status_transition;
ALTER TABLE workspaces_settings DROP COLUMN IF EXISTS requirement_status_transition;

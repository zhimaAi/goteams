-- +goose Up
-- +goose StatementBegin

-- 让云端成为可复用团队配置的唯一数据源：编排步骤与专家成员需要携带默认头像，
-- 客户端拉取后仅补充本地执行配置（CLI/模型）即可运行团队任务。
-- 1) pipeline_steps 增加 avatar：专家流水线步骤的默认头像。
-- 2) expert_group_agents 增加 avatar：专家团成员的默认头像。
-- 两者存相对路径/预设路径字符串，默认 ''，向后兼容（旧数据为空时由前端/客户端回退预设）。

ALTER TABLE pipeline_steps
    ADD COLUMN IF NOT EXISTS avatar VARCHAR(255) NOT NULL DEFAULT '';

COMMENT ON COLUMN pipeline_steps.avatar IS '步骤默认头像（相对路径或预设路径，'' 表示未设置）';

ALTER TABLE expert_group_agents
    ADD COLUMN IF NOT EXISTS avatar VARCHAR(255) NOT NULL DEFAULT '';

COMMENT ON COLUMN expert_group_agents.avatar IS '专家默认头像（相对路径或预设路径，'' 表示未设置）';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE expert_group_agents DROP COLUMN IF EXISTS avatar;
ALTER TABLE pipeline_steps DROP COLUMN IF EXISTS avatar;
-- +goose StatementEnd

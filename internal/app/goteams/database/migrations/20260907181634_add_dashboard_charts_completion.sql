-- +goose Up
ALTER TABLE dashboard_cards DROP CONSTRAINT IF EXISTS dashboard_cards_card_type_check;
ALTER TABLE requirements ADD COLUMN completed_at BIGINT NOT NULL DEFAULT 0;
COMMENT ON COLUMN requirements.completed_at IS '最近一次进入最终状态的时间，0 表示未完成或历史日期未知';
CREATE INDEX idx_requirements_completion ON requirements(admin_id, workspace_id, completed_at, type) WHERE deleted_at = 0 AND completed_at > 0;
CREATE INDEX idx_requirement_histories_scope ON requirement_histories(admin_id,workspace_id,requirement_id,id);

-- +goose Down
-- 保留已记录完成日期与卡片类型，避免回滚丢失业务数据。
SELECT 1;

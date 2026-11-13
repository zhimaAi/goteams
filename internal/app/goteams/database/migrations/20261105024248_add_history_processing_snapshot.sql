-- +goose Up
ALTER TABLE requirement_histories ADD COLUMN processing_snapshot JSONB;
COMMENT ON COLUMN requirement_histories.processing_snapshot IS '事件完成时的状态和处理人快照；NULL 表示旧记录未知';

-- +goose Down
ALTER TABLE requirement_histories DROP COLUMN processing_snapshot;

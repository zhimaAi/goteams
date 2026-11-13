-- +goose Up
-- 处理人字段由单值 assignee_id 改造为多值数组 assignee_ids，支持需求/缺陷同时指派多名处理人
-- The "handler" field has been transformed from a single value, assignee_id, to a multi-value array, assignee_ids, to support the simultaneous assignment of multiple handlers for requirements/defects
ALTER TABLE requirements
    ADD COLUMN IF NOT EXISTS assignee_ids INTEGER[] NOT NULL DEFAULT '{}';

COMMENT ON COLUMN requirements.assignee_ids IS '处理人ID数组（多名成员共同负责一个工作项）';

-- 回填历史数据：将原单值处理人写入数组首位
-- Backfill historical data: Write the original single-value processor into the first position of the array
UPDATE requirements
SET assignee_ids = ARRAY[assignee_id]
WHERE assignee_id > 0
  AND COALESCE(array_length(assignee_ids, 1), 0) = 0;

-- 删除旧的单值处理人列（含其索引/外键一并移除）
-- Delete the old single-value handler column (including its index/foreign key)
ALTER TABLE requirements DROP COLUMN IF EXISTS assignee_id;

-- 为数组包含查询（@>、&&）建立 GIN 索引
-- Create GIN index for array containment queries (@>, &&)
CREATE INDEX IF NOT EXISTS idx_requirements_assignee_ids
    ON requirements USING GIN (assignee_ids);

-- +goose Down
-- 回滚：重建单值处理人列，取数组首位回填
-- Rollback: Reconstruct the single-value handler column, and backfill with the first element of the array
ALTER TABLE requirements
    ADD COLUMN IF NOT EXISTS assignee_id INTEGER DEFAULT NULL;

COMMENT ON COLUMN requirements.assignee_id IS '处理人用户ID（负责人）';

UPDATE requirements
SET assignee_id = assignee_ids[1]
WHERE array_length(assignee_ids, 1) > 0;

DROP INDEX IF EXISTS idx_requirements_assignee_ids;
ALTER TABLE requirements DROP COLUMN IF EXISTS assignee_ids;

CREATE INDEX IF NOT EXISTS idx_requirements_assignee ON requirements(assignee_id);
CREATE INDEX IF NOT EXISTS idx_req_admin_ws_assignee ON requirements(admin_id, workspace_id, assignee_id);
CREATE INDEX IF NOT EXISTS idx_req_admin_assignee_plan_start ON requirements(admin_id, assignee_id, planned_start_date);

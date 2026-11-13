-- +goose Up
-- +goose StatementBegin
ALTER TABLE requirements
ADD COLUMN IF NOT EXISTS completed_work_hours DECIMAL(10,2) NOT NULL DEFAULT 0;

COMMENT ON COLUMN requirements.work_hours IS '预估工时（人时）';
COMMENT ON COLUMN requirements.completed_work_hours IS '完成工时汇总（人时），由花费明细事务内维护';

CREATE TABLE IF NOT EXISTS requirement_work_logs (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL,
    requirement_id INTEGER NOT NULL,
    work_date BIGINT NOT NULL,
    hours DECIMAL(10,2) NOT NULL DEFAULT 0,
    description TEXT NOT NULL DEFAULT '',
    created_by INTEGER NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE requirement_work_logs IS '需求花费工时明细表';
COMMENT ON COLUMN requirement_work_logs.admin_id IS '归属租户管理员ID（多租户隔离）';
COMMENT ON COLUMN requirement_work_logs.workspace_id IS '所属工作空间ID';
COMMENT ON COLUMN requirement_work_logs.requirement_id IS '所属需求ID';
COMMENT ON COLUMN requirement_work_logs.work_date IS '花费日期（Unix时间戳）';
COMMENT ON COLUMN requirement_work_logs.hours IS '花费工时（人时）';
COMMENT ON COLUMN requirement_work_logs.description IS '花费备注';
COMMENT ON COLUMN requirement_work_logs.created_by IS '创建人用户ID';

CREATE INDEX IF NOT EXISTS idx_requirement_work_logs_scope
ON requirement_work_logs(admin_id, workspace_id, requirement_id, work_date DESC, id DESC);

INSERT INTO workspace_field_configs
    (admin_id, workspace_id, type_key, field_key, is_enabled, is_editable, is_locked, sort_order, created_at, updated_at)
SELECT w.admin_id, w.id, 'requirement', fields.field_key, 0, fields.is_editable, 0,
       COALESCE(current_order.max_order, -1) + fields.field_offset,
       EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
FROM workspaces w
CROSS JOIN (VALUES
    ('estimated_work_hours', 1, 1),
    ('work_progress', 0, 2),
    ('completed_work_hours', 1, 3),
    ('remaining_work_hours', 0, 4),
    ('overtime_work_hours', 0, 5)
) AS fields(field_key, is_editable, field_offset)
LEFT JOIN LATERAL (
    SELECT MAX(sort_order) AS max_order
    FROM (
        SELECT sort_order FROM workspace_field_configs
        WHERE admin_id = w.admin_id AND workspace_id = w.id AND type_key = 'requirement'
        UNION ALL
        SELECT sort_order FROM workspace_custom_fields
        WHERE admin_id = w.admin_id AND workspace_id = w.id AND type_key = 'requirement'
    ) ordered_fields
) current_order ON TRUE
ON CONFLICT (admin_id, workspace_id, type_key, field_key) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM workspace_field_configs
WHERE type_key = 'requirement'
  AND field_key IN ('estimated_work_hours', 'work_progress', 'completed_work_hours', 'remaining_work_hours', 'overtime_work_hours');

DROP TABLE IF EXISTS requirement_work_logs;

ALTER TABLE requirements DROP COLUMN IF EXISTS completed_work_hours;
COMMENT ON COLUMN requirements.work_hours IS '工时（小时）';
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin
-- Collapse legacy duplicate rows before enforcing one expense row per requirement/date.
WITH duplicate_groups AS (
    SELECT admin_id, workspace_id, requirement_id, work_date,
           MIN(id) AS keep_id, SUM(hours) AS total_hours
    FROM requirement_work_logs
    GROUP BY admin_id, workspace_id, requirement_id, work_date
    HAVING COUNT(*) > 1
), updated_rows AS (
    UPDATE requirement_work_logs l
    SET hours = g.total_hours,
        updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
    FROM duplicate_groups g
    WHERE l.id = g.keep_id
    RETURNING l.id
)
DELETE FROM requirement_work_logs l
USING duplicate_groups g
WHERE l.admin_id = g.admin_id
  AND l.workspace_id = g.workspace_id
  AND l.requirement_id = g.requirement_id
  AND l.work_date = g.work_date
  AND l.id <> g.keep_id;

CREATE UNIQUE INDEX IF NOT EXISTS uq_requirement_work_logs_scope_date
ON requirement_work_logs(admin_id, workspace_id, requirement_id, work_date);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS uq_requirement_work_logs_scope_date;
-- +goose StatementEnd

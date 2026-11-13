-- +goose Up
-- +goose StatementBegin

-- 同一次客户端任务切换执行方式后再次执行，必须另存一条云端运行记录。
-- execution_index 从 0 开始，每次切换 +1；历史行保留步骤、会话和 Token。
ALTER TABLE pipeline_tasks
    ADD COLUMN IF NOT EXISTS execution_index INTEGER NOT NULL DEFAULT 0;

COMMENT ON COLUMN pipeline_tasks.execution_index IS '同一次本地任务的执行序号，0 为首次执行，切换执行方式后递增';

DO $$
DECLARE
    old_constraint text;
BEGIN
    SELECT con.conname INTO old_constraint
    FROM pg_constraint con
    JOIN pg_class rel ON rel.oid = con.conrelid
    WHERE rel.relname = 'pipeline_tasks'
      AND con.contype = 'u'
      AND pg_get_constraintdef(con.oid) LIKE '%local_task_uuid%'
      AND pg_get_constraintdef(con.oid) NOT LIKE '%execution_index%';
    IF old_constraint IS NOT NULL THEN
        EXECUTE format('ALTER TABLE pipeline_tasks DROP CONSTRAINT %I', old_constraint);
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS pipeline_tasks_admin_user_local_execution_uidx
    ON pipeline_tasks(admin_id, user_id, local_task_uuid, execution_index);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS pipeline_tasks_admin_user_local_execution_uidx;
ALTER TABLE pipeline_tasks DROP COLUMN IF EXISTS execution_index;
-- +goose StatementEnd

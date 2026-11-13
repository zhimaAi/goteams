-- +goose Up
-- +goose StatementBegin

-- 专家流水线（需求 1896 第 3 部分）：
-- 1. 流水线增加适用范围 scope：all=全部成员，members=指定成员（沿用 pipeline_members），departments=指定组织。
-- 2. 新增 pipeline_departments：scope=departments 时记录指定组织。
-- 3. pipeline_task_sessions 增加客户端上报的执行统计列：工具调用次数、变更文件数、新增/删除行数。
--    存量数据无统计（默认 0），前端展示时按 0 隐藏。

-- 1. pipelines.scope
ALTER TABLE pipelines
    ADD COLUMN IF NOT EXISTS scope VARCHAR(20) NOT NULL DEFAULT 'members';

COMMENT ON COLUMN pipelines.scope IS '适用范围：all=全部成员，members=指定成员，departments=指定组织';

-- 存量流水线此前按 pipeline_members 授权，保持 members 语义不变。

-- 2. pipeline_departments
CREATE TABLE IF NOT EXISTS pipeline_departments (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    pipeline_id INTEGER NOT NULL,
    department_id INTEGER NOT NULL,
    created_by INTEGER NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, pipeline_id, department_id)
);

CREATE INDEX IF NOT EXISTS idx_pipeline_departments_admin_dept
    ON pipeline_departments(admin_id, department_id, pipeline_id);

COMMENT ON TABLE pipeline_departments IS '流水线指定组织（scope=departments 时的授权组织）';
COMMENT ON COLUMN pipeline_departments.admin_id IS '租户 ID';
COMMENT ON COLUMN pipeline_departments.pipeline_id IS '流水线 ID';
COMMENT ON COLUMN pipeline_departments.department_id IS '组织（部门）ID';
COMMENT ON COLUMN pipeline_departments.created_by IS '创建者用户 ID';
COMMENT ON COLUMN pipeline_departments.created_at IS '创建时间（Unix 秒）';

-- 3. pipeline_task_sessions 执行统计
ALTER TABLE pipeline_task_sessions
    ADD COLUMN IF NOT EXISTS tool_call_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS files_changed INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS lines_added INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS lines_deleted INTEGER NOT NULL DEFAULT 0;

COMMENT ON COLUMN pipeline_task_sessions.tool_call_count IS '本会话工具调用次数（客户端事件计数）';
COMMENT ON COLUMN pipeline_task_sessions.files_changed IS '本会话变更文件数（按 CLI 能力尽力解析，无数据为 0）';
COMMENT ON COLUMN pipeline_task_sessions.lines_added IS '本会话新增代码行数（按 CLI 能力尽力解析，无数据为 0）';
COMMENT ON COLUMN pipeline_task_sessions.lines_deleted IS '本会话删除代码行数（按 CLI 能力尽力解析，无数据为 0）';
-- +goose StatementEnd

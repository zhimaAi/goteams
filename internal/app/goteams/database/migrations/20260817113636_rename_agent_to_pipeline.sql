-- +goose Up
-- 云端 Agent 概念整体调整为流水线（Pipeline）。
-- 旧 agent_* 表因早期迁移文件被修改过而存在 schema drift（重命名约束/索引名极易踩坑），
-- 故直接删除所有 agent_*/pipeline_* 旧表，按当前正确结构重建 pipeline_* 表（开发库可接受丢弃数据）。
-- 1. agents → pipelines
-- 2. agent_members → pipeline_members
-- 3. agent_workflow_steps → pipeline_steps（移除 step_documents）
-- 4. 删除 agent_documents（自定义文档占位符功能下线）
-- 5. agent_tasks → pipeline_tasks
-- 6. agent_task_steps → pipeline_task_steps
-- 7. agent_task_conversations → pipeline_task_conversations
-- 8. role_permissions 中 agent:* 权限 key 迁移为 pipeline:*

-- 0. 幂等删除所有旧表（old + new 两种命名，兼容历史半迁移状态）
DROP TABLE IF EXISTS pipeline_task_conversations;
DROP TABLE IF EXISTS agent_task_conversations;
DROP TABLE IF EXISTS pipeline_task_steps;
DROP TABLE IF EXISTS agent_task_steps;
DROP TABLE IF EXISTS pipeline_tasks;
DROP TABLE IF EXISTS agent_tasks;
DROP TABLE IF EXISTS pipeline_steps;
DROP TABLE IF EXISTS agent_workflow_steps;
DROP TABLE IF EXISTS pipeline_members;
DROP TABLE IF EXISTS agent_members;
DROP TABLE IF EXISTS pipelines;
DROP TABLE IF EXISTS agents;
DROP TABLE IF EXISTS agent_documents;

-- 1. pipelines
CREATE TABLE pipelines (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    name VARCHAR(100) NOT NULL,
    description VARCHAR(1000) NOT NULL DEFAULT '',
    color VARCHAR(20) NOT NULL DEFAULT '#3157E2',
    icon VARCHAR(500) NOT NULL DEFAULT '',
    cli_type VARCHAR(20) NOT NULL,
    workflow_version INTEGER NOT NULL DEFAULT 1,
    status SMALLINT NOT NULL DEFAULT 1,
    created_by INTEGER NOT NULL,
    updated_by INTEGER NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    deleted_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX idx_pipelines_admin_name_active
    ON pipelines(admin_id, name)
    WHERE deleted_at = 0;

CREATE INDEX idx_pipelines_admin_status
    ON pipelines(admin_id, status, id DESC)
    WHERE deleted_at = 0;

COMMENT ON TABLE pipelines IS '流水线主表';
COMMENT ON COLUMN pipelines.admin_id IS '租户 ID';
COMMENT ON COLUMN pipelines.name IS '流水线名称';
COMMENT ON COLUMN pipelines.description IS '流水线描述';
COMMENT ON COLUMN pipelines.color IS '标识颜色';
COMMENT ON COLUMN pipelines.icon IS '图标 URL';
COMMENT ON COLUMN pipelines.cli_type IS 'CLI 类型：codex/claude/codebuddy';
COMMENT ON COLUMN pipelines.workflow_version IS '流水线版本号，步骤或 CLI 类型变更时递增';
COMMENT ON COLUMN pipelines.status IS '1=启用，0=停用';
COMMENT ON COLUMN pipelines.created_by IS '创建者用户 ID';
COMMENT ON COLUMN pipelines.updated_by IS '最后更新者用户 ID';
COMMENT ON COLUMN pipelines.created_at IS '创建时间（Unix 秒）';
COMMENT ON COLUMN pipelines.updated_at IS '更新时间（Unix 秒）';
COMMENT ON COLUMN pipelines.deleted_at IS '软删除：0=未删除，非0=删除时间';

-- 2. pipeline_members
CREATE TABLE pipeline_members (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    pipeline_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    created_by INTEGER NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, pipeline_id, user_id)
);

CREATE INDEX idx_pipeline_members_admin_user
    ON pipeline_members(admin_id, user_id, pipeline_id);

COMMENT ON TABLE pipeline_members IS '流水线允许使用的企业成员';
COMMENT ON COLUMN pipeline_members.admin_id IS '租户 ID';
COMMENT ON COLUMN pipeline_members.pipeline_id IS '流水线 ID';
COMMENT ON COLUMN pipeline_members.user_id IS '成员用户 ID';
COMMENT ON COLUMN pipeline_members.created_by IS '创建者用户 ID';
COMMENT ON COLUMN pipeline_members.created_at IS '创建时间（Unix 秒）';

-- 3. pipeline_steps（不含 step_documents）
CREATE TABLE pipeline_steps (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    pipeline_id INTEGER NOT NULL,
    step_key VARCHAR(64) NOT NULL,
    name VARCHAR(100) NOT NULL,
    prompt_content TEXT NOT NULL DEFAULT '',
    remark VARCHAR(500) NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, pipeline_id, step_key)
);

CREATE INDEX idx_pipeline_steps_order
    ON pipeline_steps(admin_id, pipeline_id, sort_order, id);

COMMENT ON TABLE pipeline_steps IS '流水线当前编排步骤';
COMMENT ON COLUMN pipeline_steps.admin_id IS '租户 ID';
COMMENT ON COLUMN pipeline_steps.pipeline_id IS '流水线 ID';
COMMENT ON COLUMN pipeline_steps.step_key IS '步骤唯一标识，服务端生成后保持稳定';
COMMENT ON COLUMN pipeline_steps.name IS '步骤名称';
COMMENT ON COLUMN pipeline_steps.prompt_content IS '提示词内容，最大 64KiB';
COMMENT ON COLUMN pipeline_steps.remark IS '步骤备注说明';
COMMENT ON COLUMN pipeline_steps.sort_order IS '排序序号';
COMMENT ON COLUMN pipeline_steps.created_at IS '创建时间（Unix 秒）';
COMMENT ON COLUMN pipeline_steps.updated_at IS '更新时间（Unix 秒）';

-- 4. agent_documents 已下线，不重建

-- 5. pipeline_tasks
CREATE TABLE pipeline_tasks (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    work_item_id INTEGER NOT NULL,
    work_item_type VARCHAR(20) NOT NULL,
    title_snapshot VARCHAR(500) NOT NULL DEFAULT '',
    pipeline_id INTEGER NOT NULL DEFAULT 0,
    pipeline_name_snapshot VARCHAR(100) NOT NULL DEFAULT '',
    pipeline_color_snapshot VARCHAR(20) NOT NULL DEFAULT '',
    cli_type VARCHAR(20) NOT NULL,
    workflow_version INTEGER NOT NULL DEFAULT 1,
    local_task_uuid VARCHAR(64) NOT NULL,
    local_revision BIGINT NOT NULL DEFAULT 0,
    projection_hash VARCHAR(64) NOT NULL DEFAULT '',
    device_uuid VARCHAR(64) NOT NULL,
    task_status VARCHAR(20) NOT NULL,
    execution_status VARCHAR(20) NOT NULL DEFAULT 'idle',
    current_step_key VARCHAR(64) NOT NULL DEFAULT '',
    execution_summary TEXT NOT NULL DEFAULT '',
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    conversation_rounds INTEGER NOT NULL DEFAULT 0,
    started_at BIGINT NOT NULL DEFAULT 0,
    finished_at BIGINT NOT NULL DEFAULT 0,
    client_updated_at BIGINT NOT NULL DEFAULT 0,
    last_uploaded_at BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, user_id, local_task_uuid)
);

CREATE INDEX idx_pipeline_tasks_admin_uploaded
    ON pipeline_tasks(admin_id, last_uploaded_at DESC);

CREATE INDEX idx_pipeline_tasks_admin_user
    ON pipeline_tasks(admin_id, user_id, last_uploaded_at DESC);

CREATE INDEX idx_pipeline_tasks_admin_pipeline
    ON pipeline_tasks(admin_id, pipeline_id, last_uploaded_at DESC);

CREATE INDEX idx_pipeline_tasks_admin_status
    ON pipeline_tasks(admin_id, task_status, last_uploaded_at DESC);

CREATE INDEX idx_pipeline_tasks_work_item
    ON pipeline_tasks(admin_id, user_id, work_item_type, work_item_id);

COMMENT ON TABLE pipeline_tasks IS '客户端任务在云端的最新完整投影主表';
COMMENT ON COLUMN pipeline_tasks.admin_id IS '租户 ID';
COMMENT ON COLUMN pipeline_tasks.user_id IS '执行任务的用户 ID';
COMMENT ON COLUMN pipeline_tasks.workspace_id IS '所属工作空间 ID';
COMMENT ON COLUMN pipeline_tasks.work_item_id IS '关联工作项 ID';
COMMENT ON COLUMN pipeline_tasks.work_item_type IS '工作项类型：requirement/defect';
COMMENT ON COLUMN pipeline_tasks.title_snapshot IS '工作项标题快照';
COMMENT ON COLUMN pipeline_tasks.pipeline_id IS '流水线 ID';
COMMENT ON COLUMN pipeline_tasks.pipeline_name_snapshot IS '流水线名称快照';
COMMENT ON COLUMN pipeline_tasks.pipeline_color_snapshot IS '流水线颜色快照';
COMMENT ON COLUMN pipeline_tasks.cli_type IS 'CLI 类型：codex/claude/codebuddy';
COMMENT ON COLUMN pipeline_tasks.workflow_version IS '流水线版本号';
COMMENT ON COLUMN pipeline_tasks.local_task_uuid IS '客户端本地任务 UUID';
COMMENT ON COLUMN pipeline_tasks.local_revision IS '客户端本地修订号';
COMMENT ON COLUMN pipeline_tasks.projection_hash IS '投影内容哈希，用于幂等判断';
COMMENT ON COLUMN pipeline_tasks.device_uuid IS '上报设备 UUID';
COMMENT ON COLUMN pipeline_tasks.task_status IS '任务状态：active/pending/developing/developed';
COMMENT ON COLUMN pipeline_tasks.execution_status IS '执行状态：idle/running/success/failed/stopped';
COMMENT ON COLUMN pipeline_tasks.current_step_key IS '当前执行步骤 key';
COMMENT ON COLUMN pipeline_tasks.execution_summary IS '执行摘要';
COMMENT ON COLUMN pipeline_tasks.input_tokens IS '累计输入 Token 数';
COMMENT ON COLUMN pipeline_tasks.output_tokens IS '累计输出 Token 数';
COMMENT ON COLUMN pipeline_tasks.total_tokens IS '累计总 Token 数';
COMMENT ON COLUMN pipeline_tasks.conversation_rounds IS '累计对话轮次';
COMMENT ON COLUMN pipeline_tasks.started_at IS '任务开始时间（Unix 秒）';
COMMENT ON COLUMN pipeline_tasks.finished_at IS '任务结束时间（Unix 秒）';
COMMENT ON COLUMN pipeline_tasks.client_updated_at IS '客户端最后更新时间（Unix 秒）';
COMMENT ON COLUMN pipeline_tasks.last_uploaded_at IS '最后上报时间（Unix 秒）';
COMMENT ON COLUMN pipeline_tasks.created_at IS '创建时间（Unix 秒）';
COMMENT ON COLUMN pipeline_tasks.updated_at IS '更新时间（Unix 秒）';

-- 6. pipeline_task_steps
CREATE TABLE pipeline_task_steps (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    pipeline_task_id BIGINT NOT NULL,
    step_key VARCHAR(64) NOT NULL,
    step_name VARCHAR(100) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    prompt_summary TEXT NOT NULL DEFAULT '',
    output_summary TEXT NOT NULL DEFAULT '',
    error_summary TEXT NOT NULL DEFAULT '',
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    conversation_rounds INTEGER NOT NULL DEFAULT 0,
    started_at BIGINT NOT NULL DEFAULT 0,
    finished_at BIGINT NOT NULL DEFAULT 0,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, pipeline_task_id, step_key)
);

CREATE INDEX idx_pipeline_task_steps_order
    ON pipeline_task_steps(admin_id, pipeline_task_id, sort_order, id);

COMMENT ON TABLE pipeline_task_steps IS '最新任务投影中的步骤';
COMMENT ON COLUMN pipeline_task_steps.admin_id IS '租户 ID';
COMMENT ON COLUMN pipeline_task_steps.pipeline_task_id IS '所属流水线任务 ID';
COMMENT ON COLUMN pipeline_task_steps.step_key IS '步骤唯一标识';
COMMENT ON COLUMN pipeline_task_steps.step_name IS '步骤名称';
COMMENT ON COLUMN pipeline_task_steps.sort_order IS '排序序号';
COMMENT ON COLUMN pipeline_task_steps.status IS '步骤状态：pending/running/success/failed/stopped';
COMMENT ON COLUMN pipeline_task_steps.prompt_summary IS '提示词摘要';
COMMENT ON COLUMN pipeline_task_steps.output_summary IS '输出摘要';
COMMENT ON COLUMN pipeline_task_steps.error_summary IS '错误摘要';
COMMENT ON COLUMN pipeline_task_steps.input_tokens IS '步骤输入 Token 数';
COMMENT ON COLUMN pipeline_task_steps.output_tokens IS '步骤输出 Token 数';
COMMENT ON COLUMN pipeline_task_steps.total_tokens IS '步骤总 Token 数';
COMMENT ON COLUMN pipeline_task_steps.conversation_rounds IS '步骤内对话轮次';
COMMENT ON COLUMN pipeline_task_steps.started_at IS '步骤开始时间（Unix 秒）';
COMMENT ON COLUMN pipeline_task_steps.finished_at IS '步骤结束时间（Unix 秒）';
COMMENT ON COLUMN pipeline_task_steps.duration_ms IS '步骤耗时（毫秒）';
COMMENT ON COLUMN pipeline_task_steps.created_at IS '创建时间（Unix 秒）';
COMMENT ON COLUMN pipeline_task_steps.updated_at IS '更新时间（Unix 秒）';

-- 7. pipeline_task_conversations
CREATE TABLE pipeline_task_conversations (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    pipeline_task_id BIGINT NOT NULL,
    conversation_uuid VARCHAR(64) NOT NULL,
    step_key VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'created',
    cli_type VARCHAR(32) NOT NULL DEFAULT '',
    conversation_json JSONB NOT NULL DEFAULT '{"round_count":0,"rounds":[]}'::jsonb,
    error_summary TEXT NOT NULL DEFAULT '',
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    started_at BIGINT NOT NULL DEFAULT 0,
    finished_at BIGINT NOT NULL DEFAULT 0,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, pipeline_task_id, conversation_uuid)
);

CREATE INDEX idx_pipeline_task_conversations_step
    ON pipeline_task_conversations(admin_id, pipeline_task_id, step_key, started_at, id);

COMMENT ON TABLE pipeline_task_conversations IS '按 conversation_uuid 聚合的客户端 AI 会话';
COMMENT ON COLUMN pipeline_task_conversations.conversation_json IS '完整会话 JSON，包含 round_count、每轮用户问题及逐条 AI 消息';

-- 8. role_permissions 权限 key 迁移：agent:* → pipeline:*
UPDATE role_permissions SET permission_key = 'pipeline:view', updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE permission_key = 'agent:view';

UPDATE role_permissions SET permission_key = 'pipeline:manage', updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE permission_key = 'agent:manage';

-- agent:view_all_tasks 已废弃（任务可见性由 pipeline:view 统一覆盖），直接清理孤儿权限行。
DELETE FROM role_permissions WHERE permission_key = 'agent:view_all_tasks';

-- +goose Down
-- 反向迁移：删除 pipeline_* 表并重建 agent_* 表（数据已丢弃，仅重建结构）。
DROP TABLE IF EXISTS pipeline_task_conversations;
DROP TABLE IF EXISTS agent_task_conversations;
DROP TABLE IF EXISTS pipeline_task_steps;
DROP TABLE IF EXISTS agent_task_steps;
DROP TABLE IF EXISTS pipeline_tasks;
DROP TABLE IF EXISTS agent_tasks;
DROP TABLE IF EXISTS pipeline_steps;
DROP TABLE IF EXISTS agent_workflow_steps;
DROP TABLE IF EXISTS pipeline_members;
DROP TABLE IF EXISTS agent_members;
DROP TABLE IF EXISTS pipelines;
DROP TABLE IF EXISTS agents;
DROP TABLE IF EXISTS agent_documents;

CREATE TABLE agents (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    name VARCHAR(100) NOT NULL,
    description VARCHAR(1000) NOT NULL DEFAULT '',
    color VARCHAR(20) NOT NULL DEFAULT '#3157E2',
    icon VARCHAR(500) NOT NULL DEFAULT '',
    cli_type VARCHAR(20) NOT NULL,
    workflow_version INTEGER NOT NULL DEFAULT 1,
    status SMALLINT NOT NULL DEFAULT 1,
    created_by INTEGER NOT NULL,
    updated_by INTEGER NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    deleted_at BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX idx_agents_admin_name_active
    ON agents(admin_id, name)
    WHERE deleted_at = 0;

CREATE INDEX idx_agents_admin_status
    ON agents(admin_id, status, id DESC)
    WHERE deleted_at = 0;

COMMENT ON TABLE agents IS 'Agent 主表';
COMMENT ON COLUMN agents.admin_id IS '租户 ID';
COMMENT ON COLUMN agents.name IS 'Agent 名称';
COMMENT ON COLUMN agents.description IS 'Agent 描述';
COMMENT ON COLUMN agents.color IS '标识颜色';
COMMENT ON COLUMN agents.icon IS '图标 URL';
COMMENT ON COLUMN agents.cli_type IS 'CLI 类型：codex/claude/codebuddy';
COMMENT ON COLUMN agents.workflow_version IS '工作流版本号，步骤或 CLI 类型变更时递增';
COMMENT ON COLUMN agents.status IS '1=启用，0=停用';
COMMENT ON COLUMN agents.created_by IS '创建者用户 ID';
COMMENT ON COLUMN agents.updated_by IS '最后更新者用户 ID';
COMMENT ON COLUMN agents.created_at IS '创建时间（Unix 秒）';
COMMENT ON COLUMN agents.updated_at IS '更新时间（Unix 秒）';
COMMENT ON COLUMN agents.deleted_at IS '软删除：0=未删除，非0=删除时间';

CREATE TABLE agent_members (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    agent_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    created_by INTEGER NOT NULL,
    created_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, agent_id, user_id)
);

CREATE INDEX idx_agent_members_admin_user
    ON agent_members(admin_id, user_id, agent_id);

COMMENT ON TABLE agent_members IS 'Agent 允许使用的企业成员';
COMMENT ON COLUMN agent_members.admin_id IS '租户 ID';
COMMENT ON COLUMN agent_members.agent_id IS 'Agent ID';
COMMENT ON COLUMN agent_members.user_id IS '成员用户 ID';
COMMENT ON COLUMN agent_members.created_by IS '创建者用户 ID';
COMMENT ON COLUMN agent_members.created_at IS '创建时间（Unix 秒）';

CREATE TABLE agent_workflow_steps (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    agent_id INTEGER NOT NULL,
    step_key VARCHAR(64) NOT NULL,
    name VARCHAR(100) NOT NULL,
    prompt_content TEXT NOT NULL DEFAULT '',
    remark VARCHAR(500) NOT NULL DEFAULT '',
    step_documents JSONB NOT NULL DEFAULT '[]'::jsonb,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, agent_id, step_key)
);

CREATE INDEX idx_agent_workflow_steps_order
    ON agent_workflow_steps(admin_id, agent_id, sort_order, id);

COMMENT ON TABLE agent_workflow_steps IS 'Agent 当前工作流步骤';
COMMENT ON COLUMN agent_workflow_steps.admin_id IS '租户 ID';
COMMENT ON COLUMN agent_workflow_steps.agent_id IS 'Agent ID';
COMMENT ON COLUMN agent_workflow_steps.step_key IS '步骤唯一标识，服务端生成后保持稳定';
COMMENT ON COLUMN agent_workflow_steps.name IS '步骤名称';
COMMENT ON COLUMN agent_workflow_steps.prompt_content IS '提示词内容，最大 64KiB';
COMMENT ON COLUMN agent_workflow_steps.remark IS '步骤备注说明';
COMMENT ON COLUMN agent_workflow_steps.step_documents IS '步骤文档配置，JSON 数组，最大 32KiB';
COMMENT ON COLUMN agent_workflow_steps.sort_order IS '排序序号';
COMMENT ON COLUMN agent_workflow_steps.created_at IS '创建时间（Unix 秒）';
COMMENT ON COLUMN agent_workflow_steps.updated_at IS '更新时间（Unix 秒）';

CREATE TABLE agent_documents (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    agent_id INTEGER NOT NULL,
    client_document_id CHAR(32) NOT NULL,
    name VARCHAR(100) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, agent_id, client_document_id),
    UNIQUE(admin_id, agent_id, name)
);

CREATE INDEX idx_agent_documents_order
    ON agent_documents(admin_id, agent_id, sort_order, id);

COMMENT ON TABLE agent_documents IS 'Agent 自定义文档占位符';
COMMENT ON COLUMN agent_documents.admin_id IS '租户 ID';
COMMENT ON COLUMN agent_documents.agent_id IS 'Agent ID';
COMMENT ON COLUMN agent_documents.client_document_id IS '客户端文档 ID，32 位随机十六进制字符串';
COMMENT ON COLUMN agent_documents.name IS '文档名称，同时作为提示词占位符名称';
COMMENT ON COLUMN agent_documents.sort_order IS '展示顺序';
COMMENT ON COLUMN agent_documents.created_at IS '创建时间（Unix 秒）';
COMMENT ON COLUMN agent_documents.updated_at IS '更新时间（Unix 秒）';

CREATE TABLE agent_tasks (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    workspace_id INTEGER NOT NULL,
    work_item_id INTEGER NOT NULL,
    work_item_type VARCHAR(20) NOT NULL,
    title_snapshot VARCHAR(500) NOT NULL DEFAULT '',
    agent_id INTEGER NOT NULL DEFAULT 0,
    agent_name_snapshot VARCHAR(100) NOT NULL DEFAULT '',
    agent_color_snapshot VARCHAR(20) NOT NULL DEFAULT '',
    cli_type VARCHAR(20) NOT NULL,
    workflow_version INTEGER NOT NULL DEFAULT 1,
    local_task_uuid VARCHAR(64) NOT NULL,
    local_revision BIGINT NOT NULL DEFAULT 0,
    projection_hash VARCHAR(64) NOT NULL DEFAULT '',
    device_uuid VARCHAR(64) NOT NULL,
    task_status VARCHAR(20) NOT NULL,
    execution_status VARCHAR(20) NOT NULL DEFAULT 'idle',
    current_step_key VARCHAR(64) NOT NULL DEFAULT '',
    execution_summary TEXT NOT NULL DEFAULT '',
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    conversation_rounds INTEGER NOT NULL DEFAULT 0,
    started_at BIGINT NOT NULL DEFAULT 0,
    finished_at BIGINT NOT NULL DEFAULT 0,
    client_updated_at BIGINT NOT NULL DEFAULT 0,
    last_uploaded_at BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, user_id, local_task_uuid)
);

CREATE INDEX idx_agent_tasks_admin_uploaded
    ON agent_tasks(admin_id, last_uploaded_at DESC);

CREATE INDEX idx_agent_tasks_admin_user
    ON agent_tasks(admin_id, user_id, last_uploaded_at DESC);

CREATE INDEX idx_agent_tasks_admin_agent
    ON agent_tasks(admin_id, agent_id, last_uploaded_at DESC);

CREATE INDEX idx_agent_tasks_admin_status
    ON agent_tasks(admin_id, task_status, last_uploaded_at DESC);

CREATE INDEX idx_agent_tasks_work_item
    ON agent_tasks(admin_id, user_id, work_item_type, work_item_id);

COMMENT ON TABLE agent_tasks IS '客户端任务在云端的最新完整投影主表';
COMMENT ON COLUMN agent_tasks.admin_id IS '租户 ID';
COMMENT ON COLUMN agent_tasks.user_id IS '执行任务的用户 ID';
COMMENT ON COLUMN agent_tasks.workspace_id IS '所属工作空间 ID';
COMMENT ON COLUMN agent_tasks.work_item_id IS '关联工作项 ID';
COMMENT ON COLUMN agent_tasks.work_item_type IS '工作项类型：requirement/defect';
COMMENT ON COLUMN agent_tasks.title_snapshot IS '工作项标题快照';
COMMENT ON COLUMN agent_tasks.agent_id IS 'Agent ID';
COMMENT ON COLUMN agent_tasks.agent_name_snapshot IS 'Agent 名称快照';
COMMENT ON COLUMN agent_tasks.agent_color_snapshot IS 'Agent 颜色快照';
COMMENT ON COLUMN agent_tasks.cli_type IS 'CLI 类型：codex/claude/codebuddy';
COMMENT ON COLUMN agent_tasks.workflow_version IS '工作流版本号';
COMMENT ON COLUMN agent_tasks.local_task_uuid IS '客户端本地任务 UUID';
COMMENT ON COLUMN agent_tasks.local_revision IS '客户端本地修订号';
COMMENT ON COLUMN agent_tasks.projection_hash IS '投影内容哈希，用于幂等判断';
COMMENT ON COLUMN agent_tasks.device_uuid IS '上报设备 UUID';
COMMENT ON COLUMN agent_tasks.task_status IS '任务状态：active/pending/developing/developed';
COMMENT ON COLUMN agent_tasks.execution_status IS '执行状态：idle/running/success/failed/stopped';
COMMENT ON COLUMN agent_tasks.current_step_key IS '当前执行步骤 key';
COMMENT ON COLUMN agent_tasks.execution_summary IS '执行摘要';
COMMENT ON COLUMN agent_tasks.input_tokens IS '累计输入 Token 数';
COMMENT ON COLUMN agent_tasks.output_tokens IS '累计输出 Token 数';
COMMENT ON COLUMN agent_tasks.total_tokens IS '累计总 Token 数';
COMMENT ON COLUMN agent_tasks.conversation_rounds IS '累计对话轮次';
COMMENT ON COLUMN agent_tasks.started_at IS '任务开始时间（Unix 秒）';
COMMENT ON COLUMN agent_tasks.finished_at IS '任务结束时间（Unix 秒）';
COMMENT ON COLUMN agent_tasks.client_updated_at IS '客户端最后更新时间（Unix 秒）';
COMMENT ON COLUMN agent_tasks.last_uploaded_at IS '最后上报时间（Unix 秒）';
COMMENT ON COLUMN agent_tasks.created_at IS '创建时间（Unix 秒）';
COMMENT ON COLUMN agent_tasks.updated_at IS '更新时间（Unix 秒）';

CREATE TABLE agent_task_steps (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    agent_task_id BIGINT NOT NULL,
    step_key VARCHAR(64) NOT NULL,
    step_name VARCHAR(100) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    prompt_summary TEXT NOT NULL DEFAULT '',
    output_summary TEXT NOT NULL DEFAULT '',
    error_summary TEXT NOT NULL DEFAULT '',
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    conversation_rounds INTEGER NOT NULL DEFAULT 0,
    started_at BIGINT NOT NULL DEFAULT 0,
    finished_at BIGINT NOT NULL DEFAULT 0,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, agent_task_id, step_key)
);

CREATE INDEX idx_agent_task_steps_order
    ON agent_task_steps(admin_id, agent_task_id, sort_order, id);

COMMENT ON TABLE agent_task_steps IS '最新任务投影中的步骤';
COMMENT ON COLUMN agent_task_steps.admin_id IS '租户 ID';
COMMENT ON COLUMN agent_task_steps.agent_task_id IS '所属 Agent 任务 ID';
COMMENT ON COLUMN agent_task_steps.step_key IS '步骤唯一标识';
COMMENT ON COLUMN agent_task_steps.step_name IS '步骤名称';
COMMENT ON COLUMN agent_task_steps.sort_order IS '排序序号';
COMMENT ON COLUMN agent_task_steps.status IS '步骤状态：pending/running/success/failed/stopped';
COMMENT ON COLUMN agent_task_steps.prompt_summary IS '提示词摘要';
COMMENT ON COLUMN agent_task_steps.output_summary IS '输出摘要';
COMMENT ON COLUMN agent_task_steps.error_summary IS '错误摘要';
COMMENT ON COLUMN agent_task_steps.input_tokens IS '步骤输入 Token 数';
COMMENT ON COLUMN agent_task_steps.output_tokens IS '步骤输出 Token 数';
COMMENT ON COLUMN agent_task_steps.total_tokens IS '步骤总 Token 数';
COMMENT ON COLUMN agent_task_steps.conversation_rounds IS '步骤内对话轮次';
COMMENT ON COLUMN agent_task_steps.started_at IS '步骤开始时间（Unix 秒）';
COMMENT ON COLUMN agent_task_steps.finished_at IS '步骤结束时间（Unix 秒）';
COMMENT ON COLUMN agent_task_steps.duration_ms IS '步骤耗时（毫秒）';
COMMENT ON COLUMN agent_task_steps.created_at IS '创建时间（Unix 秒）';
COMMENT ON COLUMN agent_task_steps.updated_at IS '更新时间（Unix 秒）';

CREATE TABLE agent_task_conversations (
    id BIGSERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    agent_task_id BIGINT NOT NULL,
    conversation_uuid VARCHAR(64) NOT NULL,
    step_key VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'created',
    cli_type VARCHAR(32) NOT NULL DEFAULT '',
    conversation_json JSONB NOT NULL DEFAULT '{"round_count":0,"rounds":[]}'::jsonb,
    error_summary TEXT NOT NULL DEFAULT '',
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    started_at BIGINT NOT NULL DEFAULT 0,
    finished_at BIGINT NOT NULL DEFAULT 0,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, agent_task_id, conversation_uuid)
);

CREATE INDEX idx_agent_task_conversations_step
    ON agent_task_conversations(admin_id, agent_task_id, step_key, started_at, id);

COMMENT ON TABLE agent_task_conversations IS '按 conversation_uuid 聚合的客户端 AI 会话';
COMMENT ON COLUMN agent_task_conversations.conversation_json IS '完整会话 JSON，包含 round_count、每轮用户问题及逐条 AI 消息';

UPDATE role_permissions SET permission_key = 'agent:view', updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE permission_key = 'pipeline:view';

UPDATE role_permissions SET permission_key = 'agent:manage', updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE permission_key = 'pipeline:manage';

-- agent:view_all_tasks 行在 Up 中已删除，无法恢复（Down 不做处理）。

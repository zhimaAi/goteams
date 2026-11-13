-- +goose Up
-- Agent 功能相关表：Agent 主表/成员/工作流步骤、客户端设备、任务投影/步骤、发布版本

-- 1. agents — Agent 主表
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

-- 2. agent_members — Agent 允许使用的企业成员
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

-- 3. agent_workflow_steps — Agent 当前工作流步骤
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

-- 4. client_devices — 客户端设备注册
CREATE TABLE client_devices (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    device_uuid VARCHAR(64) NOT NULL,
    device_name VARCHAR(100) NOT NULL DEFAULT '',
    os VARCHAR(20) NOT NULL,
    arch VARCHAR(20) NOT NULL DEFAULT '',
    client_version VARCHAR(30) NOT NULL DEFAULT '',
    protocol_version INTEGER NOT NULL DEFAULT 2,
    local_schema_version INTEGER NOT NULL DEFAULT 1,
    credential_version INTEGER NOT NULL DEFAULT 1,
    credential_fingerprint VARCHAR(64) NOT NULL,
    credential_status SMALLINT NOT NULL DEFAULT 1,
    registered_by_user_id INTEGER NOT NULL,
    last_user_id INTEGER NOT NULL DEFAULT 0,
    last_ip VARCHAR(64) NOT NULL DEFAULT '',
    last_seen_at BIGINT NOT NULL DEFAULT 0,
    revoked_at BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, device_uuid)
);

CREATE INDEX idx_client_devices_admin_seen
    ON client_devices(admin_id, credential_status, last_seen_at DESC);

COMMENT ON TABLE client_devices IS '客户端设备注册表';
COMMENT ON COLUMN client_devices.admin_id IS '租户 ID';
COMMENT ON COLUMN client_devices.device_uuid IS '设备唯一标识';
COMMENT ON COLUMN client_devices.device_name IS '设备名称';
COMMENT ON COLUMN client_devices.os IS '操作系统';
COMMENT ON COLUMN client_devices.arch IS 'CPU 架构';
COMMENT ON COLUMN client_devices.client_version IS '客户端版本号';
COMMENT ON COLUMN client_devices.protocol_version IS '通信协议版本';
COMMENT ON COLUMN client_devices.local_schema_version IS '本地数据库 schema 版本';
COMMENT ON COLUMN client_devices.credential_version IS '设备凭据版本号';
COMMENT ON COLUMN client_devices.credential_fingerprint IS '设备凭据指纹（HMAC-SHA256），原始凭据只返回一次';
COMMENT ON COLUMN client_devices.credential_status IS '1=active，0=revoked';
COMMENT ON COLUMN client_devices.registered_by_user_id IS '注册人用户 ID';
COMMENT ON COLUMN client_devices.last_user_id IS '最后使用设备用户 ID';
COMMENT ON COLUMN client_devices.last_ip IS '最后连接 IP';
COMMENT ON COLUMN client_devices.last_seen_at IS '最后在线时间（Unix 秒）';
COMMENT ON COLUMN client_devices.revoked_at IS '吊销时间（Unix 秒），0=未吊销';
COMMENT ON COLUMN client_devices.created_at IS '创建时间（Unix 秒）';
COMMENT ON COLUMN client_devices.updated_at IS '更新时间（Unix 秒）';

-- 5. agent_tasks — 客户端任务在云端的最新完整投影主表
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

-- 6. agent_task_steps — 最新任务投影中的步骤
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

-- 7. client_releases — 客户端发布版本
CREATE TABLE client_releases (
    id SERIAL PRIMARY KEY,
    version VARCHAR(50) NOT NULL,
    platform VARCHAR(20) NOT NULL,
    arch VARCHAR(20) NOT NULL,
    download_url TEXT NOT NULL DEFAULT '',
    sha256 VARCHAR(128) NOT NULL DEFAULT '',
    ed25519_signature TEXT NOT NULL DEFAULT '',
    manifest_json TEXT NOT NULL DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    created_by INTEGER NOT NULL,
    published_at BIGINT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(version, platform, arch)
);

COMMENT ON TABLE client_releases IS '客户端发布版本';
COMMENT ON COLUMN client_releases.version IS '版本号';
COMMENT ON COLUMN client_releases.platform IS '目标平台';
COMMENT ON COLUMN client_releases.arch IS '目标 CPU 架构';
COMMENT ON COLUMN client_releases.download_url IS '下载地址';
COMMENT ON COLUMN client_releases.sha256 IS '安装包 SHA256 校验值';
COMMENT ON COLUMN client_releases.ed25519_signature IS 'Ed25519 签名';
COMMENT ON COLUMN client_releases.manifest_json IS '版本清单 JSON';
COMMENT ON COLUMN client_releases.status IS '状态：draft/published/revoked';
COMMENT ON COLUMN client_releases.created_by IS '创建者用户 ID';
COMMENT ON COLUMN client_releases.published_at IS '发布时间（Unix 秒）';
COMMENT ON COLUMN client_releases.created_at IS '创建时间（Unix 秒）';
COMMENT ON COLUMN client_releases.updated_at IS '更新时间（Unix 秒）';

-- +goose Down
-- 按依赖逆序删除所有 Agent 相关表
DROP TABLE IF EXISTS client_releases;
DROP TABLE IF EXISTS agent_task_steps;
DROP TABLE IF EXISTS agent_tasks;
DROP TABLE IF EXISTS client_devices;
DROP TABLE IF EXISTS agent_workflow_steps;
DROP TABLE IF EXISTS agent_members;
DROP TABLE IF EXISTS agents;

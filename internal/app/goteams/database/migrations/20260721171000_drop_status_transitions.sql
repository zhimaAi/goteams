-- +goose Up

DROP TABLE IF EXISTS workspace_status_transitions;

-- +goose Down

CREATE TABLE workspace_status_transitions (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL DEFAULT 0,
    workspace_id INTEGER NOT NULL DEFAULT 0,
    type_key VARCHAR(50) NOT NULL,
    from_status_id INTEGER NOT NULL DEFAULT 0,
    to_status_id INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id, workspace_id, type_key, from_status_id, to_status_id)
);

COMMENT ON TABLE workspace_status_transitions IS '状态流转规则表（工作空间级）';
COMMENT ON COLUMN workspace_status_transitions.id IS '规则ID';
COMMENT ON COLUMN workspace_status_transitions.admin_id IS '归属租户管理员ID';
COMMENT ON COLUMN workspace_status_transitions.workspace_id IS '所属工作空间ID';
COMMENT ON COLUMN workspace_status_transitions.type_key IS '工作项类型: requirement=需求, defect=缺陷';
COMMENT ON COLUMN workspace_status_transitions.from_status_id IS '源状态ID（关联 workspace_statuses.id）';
COMMENT ON COLUMN workspace_status_transitions.to_status_id IS '目标状态ID（关联 workspace_statuses.id）';
COMMENT ON COLUMN workspace_status_transitions.created_at IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN workspace_status_transitions.updated_at IS '更新时间（Unix时间戳）';

CREATE INDEX idx_wst_admin_id ON workspace_status_transitions(admin_id);

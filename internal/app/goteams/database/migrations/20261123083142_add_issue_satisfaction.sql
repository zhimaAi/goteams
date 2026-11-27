-- +goose Up

CREATE TABLE workspace_issue_satisfaction_configs (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL CHECK (admin_id > 0),
    workspace_id INTEGER NOT NULL CHECK (workspace_id > 0),
    enabled SMALLINT NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    manual_enabled SMALLINT NOT NULL DEFAULT 0 CHECK (manual_enabled IN (0, 1)),
    validity_seconds INTEGER NOT NULL DEFAULT 86400 CHECK (validity_seconds > 0),
    auto_creator_kinds SMALLINT[] NOT NULL DEFAULT ARRAY[1, 2]::SMALLINT[],
    auto_status_id INTEGER NOT NULL DEFAULT 0 CHECK (auto_status_id >= 0),
    auto_template_ids INTEGER[] NOT NULL DEFAULT ARRAY[]::INTEGER[],
    rating_mode SMALLINT NOT NULL CHECK (rating_mode IN (2, 3, 4, 5)),
    rating_options JSONB NOT NULL,
    resolved_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    resolved_required BOOLEAN NOT NULL DEFAULT FALSE,
    invitation_text TEXT NOT NULL DEFAULT '',
    completion_text TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    updated_by INTEGER NOT NULL CHECK (updated_by > 0),
    created_at BIGINT NOT NULL CHECK (created_at > 0),
    updated_at BIGINT NOT NULL CHECK (updated_at >= created_at),
    CONSTRAINT uq_issue_sat_config_scope UNIQUE (admin_id, workspace_id),
    CONSTRAINT ck_issue_sat_creator_kinds CHECK (
        auto_creator_kinds <@ ARRAY[1, 2]::SMALLINT[]
        AND array_position(auto_creator_kinds, NULL) IS NULL
        AND (auto_status_id = 0 OR cardinality(auto_creator_kinds) > 0)
    ),
    CONSTRAINT ck_issue_sat_templates CHECK (array_position(auto_template_ids, NULL) IS NULL),
    CONSTRAINT ck_issue_sat_options CHECK (
        CASE WHEN jsonb_typeof(rating_options) = 'array'
            THEN jsonb_array_length(rating_options) = rating_mode ELSE FALSE END
    ),
    CONSTRAINT ck_issue_sat_resolved_required CHECK (NOT resolved_required OR resolved_enabled),
    CONSTRAINT ck_issue_sat_enabled_config CHECK (
        enabled = 0 OR manual_enabled = 1 OR auto_status_id > 0
    )
);
COMMENT ON TABLE workspace_issue_satisfaction_configs IS '本系统工单满意度当前配置';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.auto_creator_kinds
    IS '自动范围: 1=可明确识别的客户创建,2=可明确识别的内部人员创建;不依据外部链接参数授权';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.auto_status_id
    IS '自动开放目标状态;0表示未配置自动规则;仅手动模式为待确认设计';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.auto_template_ids
    IS '自动模板范围;空数组表示全部为待确认建议';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.rating_options
    IS '固定结构数组: key,label,value,sentiment,tags[{key,label}],tags_required,comment_required';

CREATE TABLE workspace_issue_satisfactions (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL CHECK (admin_id > 0),
    workspace_id INTEGER NOT NULL CHECK (workspace_id > 0),
    requirement_id INTEGER NOT NULL CHECK (requirement_id > 0),
    openid VARCHAR(128) NOT NULL CHECK (openid <> ''),
    config_version INTEGER NOT NULL CHECK (config_version > 0),
    rule_snapshot JSONB NOT NULL CHECK (jsonb_typeof(rule_snapshot) = 'object'),
    activation_kind VARCHAR(10) NOT NULL CHECK (activation_kind IN ('auto', 'manual')),
    eligible_at BIGINT NOT NULL CHECK (eligible_at > 0),
    expires_at BIGINT NOT NULL,
    revoked_at BIGINT NOT NULL DEFAULT 0 CHECK (revoked_at >= 0),
    revoke_reason VARCHAR(32) NOT NULL DEFAULT '',
    rating_mode SMALLINT NOT NULL CHECK (rating_mode IN (2, 3, 4, 5)),
    rating_option_key VARCHAR(64) NOT NULL DEFAULT '',
    rating_value SMALLINT,
    rating_label VARCHAR(100) NOT NULL DEFAULT '',
    sentiment VARCHAR(8) CHECK (sentiment IN ('positive', 'neutral', 'negative')),
    selected_tags JSONB NOT NULL DEFAULT '[]'::JSONB CHECK (jsonb_typeof(selected_tags) = 'array'),
    content VARCHAR(200) NOT NULL DEFAULT '',
    resolved BOOLEAN,
    submitted_at BIGINT NOT NULL DEFAULT 0,
    reviewer_name VARCHAR(100) NOT NULL DEFAULT '',
    reviewer_avatar TEXT NOT NULL DEFAULT '',
    category_id INTEGER NOT NULL DEFAULT 0 CHECK (category_id >= 0),
    category_name VARCHAR(100) NOT NULL DEFAULT '',
    priority_id INTEGER NOT NULL DEFAULT 0 CHECK (priority_id >= 0),
    priority_name VARCHAR(100) NOT NULL DEFAULT '',
    handlers_snapshot JSONB NOT NULL DEFAULT '[]'::JSONB CHECK (jsonb_typeof(handlers_snapshot) = 'array'),
    created_at BIGINT NOT NULL CHECK (created_at > 0),
    updated_at BIGINT NOT NULL CHECK (updated_at >= created_at),
    CONSTRAINT uq_issue_sat_requirement UNIQUE (admin_id, workspace_id, requirement_id),
    CONSTRAINT ck_issue_sat_window CHECK (expires_at > eligible_at),
    CONSTRAINT ck_issue_sat_revocation CHECK (
        (revoked_at = 0 AND revoke_reason = '') OR
        (revoked_at >= eligible_at AND revoke_reason <> '')
    ),
    CONSTRAINT ck_issue_sat_submission CHECK (
        (submitted_at = 0 AND rating_option_key = '' AND rating_value IS NULL
            AND rating_label = '' AND sentiment IS NULL AND resolved IS NULL
            AND selected_tags = '[]'::JSONB AND content = '') OR
        (submitted_at >= eligible_at AND submitted_at < expires_at
            AND rating_option_key <> '' AND rating_value IS NOT NULL
            AND rating_value BETWEEN 1 AND rating_mode
            AND rating_label <> '' AND sentiment IS NOT NULL)
    )
);
COMMENT ON TABLE workspace_issue_satisfactions IS '工单管理空间邀填工单评价资格、规则快照及一次提交结果;客户提交后只读';
COMMENT ON CONSTRAINT uq_issue_sat_requirement ON workspace_issue_satisfactions
    IS '已确认每张邀填工单仅一次评价;不按客户或处理轮次增加提交';
COMMENT ON COLUMN workspace_issue_satisfactions.rating_value
    IS '所选模式内顺序值1至rating_mode,不是跨模式统一五星分数';
COMMENT ON COLUMN workspace_issue_satisfactions.sentiment
    IS '提交时从生效选项读取的好/中/差映射快照,不接受客户端任意指定';
COMMENT ON COLUMN workspace_issue_satisfactions.resolved
    IS 'true已解决,false未解决,NULL未收集或未回答;动态必填依据rule_snapshot';
COMMENT ON COLUMN workspace_issue_satisfactions.handlers_snapshot
    IS '首次开放时[{user_id,role_id,user_name,role_name}],成员及角色筛选须匹配同一项';
COMMENT ON COLUMN workspace_issue_satisfactions.selected_tags
    IS '所选评价项下[{key,label}]快照,只能由服务端根据规则匹配生成';

CREATE TABLE workspace_issue_satisfaction_invitations (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL CHECK (admin_id > 0),
    workspace_id INTEGER NOT NULL CHECK (workspace_id > 0),
    satisfaction_id INTEGER NOT NULL CHECK (satisfaction_id > 0),
    request_key VARCHAR(64) NOT NULL CHECK (request_key <> ''),
    kind VARCHAR(10) NOT NULL CHECK (kind IN ('manual', 'auto')),
    operator_id INTEGER NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL CHECK (created_at > 0),
    CONSTRAINT uq_issue_sat_invitation_request UNIQUE (admin_id, workspace_id, request_key),
    CONSTRAINT ck_issue_sat_invitation_operator CHECK (
        (kind = 'manual' AND operator_id > 0) OR (kind = 'auto' AND operator_id = 0)
    )
);
COMMENT ON TABLE workspace_issue_satisfaction_invitations
    IS '本系统邀评动作记录;不表示外部消息发送或送达';
COMMENT ON COLUMN workspace_issue_satisfaction_invitations.request_key
    IS 'auto:<评价ID>或手动请求UUID;业务重邀换key,原请求重试复用key';

CREATE INDEX idx_issue_sat_eligible_range
    ON workspace_issue_satisfactions (admin_id, workspace_id, eligible_at, id);
CREATE INDEX idx_issue_sat_invitation_history
    ON workspace_issue_satisfaction_invitations
    (admin_id, workspace_id, satisfaction_id, created_at, id);


COMMENT ON COLUMN workspace_issue_satisfaction_configs.id IS '满意度id字段';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.admin_id IS '租户隔离号';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.workspace_id IS '所属工单管理空间';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.enabled IS '满意度enabled字段';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.manual_enabled IS '保留手动邀评配置，本期不提供外部邀评';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.validity_seconds IS '满意度validity_seconds字段';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.auto_creator_kinds IS '满意度auto_creator_kinds字段';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.auto_status_id IS '满意度auto_status_id字段';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.auto_template_ids IS '满意度auto_template_ids字段';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.rating_mode IS '满意度rating_mode字段';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.rating_options IS '各等级选项、标签及必填配置';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.resolved_enabled IS '满意度resolved_enabled字段';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.resolved_required IS '满意度resolved_required字段';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.invitation_text IS '满意度invitation_text字段';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.completion_text IS '满意度completion_text字段';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.version IS '配置乐观并发版本';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.updated_by IS '满意度updated_by字段';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.created_at IS '创建时间Unix秒';
COMMENT ON COLUMN workspace_issue_satisfaction_configs.updated_at IS '更新时间Unix秒';
COMMENT ON COLUMN workspace_issue_satisfactions.id IS '满意度id字段';
COMMENT ON COLUMN workspace_issue_satisfactions.admin_id IS '租户隔离号';
COMMENT ON COLUMN workspace_issue_satisfactions.workspace_id IS '所属工单管理空间';
COMMENT ON COLUMN workspace_issue_satisfactions.requirement_id IS '满意度requirement_id字段';
COMMENT ON COLUMN workspace_issue_satisfactions.openid IS '邀填客户归属标识';
COMMENT ON COLUMN workspace_issue_satisfactions.config_version IS '满意度config_version字段';
COMMENT ON COLUMN workspace_issue_satisfactions.rule_snapshot IS '首次生效时的完整配置快照';
COMMENT ON COLUMN workspace_issue_satisfactions.activation_kind IS '满意度activation_kind字段';
COMMENT ON COLUMN workspace_issue_satisfactions.eligible_at IS '首次开放评价时间Unix秒';
COMMENT ON COLUMN workspace_issue_satisfactions.expires_at IS '评价截止时间Unix秒';
COMMENT ON COLUMN workspace_issue_satisfactions.revoked_at IS '满意度revoked_at字段';
COMMENT ON COLUMN workspace_issue_satisfactions.revoke_reason IS '满意度revoke_reason字段';
COMMENT ON COLUMN workspace_issue_satisfactions.rating_mode IS '满意度rating_mode字段';
COMMENT ON COLUMN workspace_issue_satisfactions.rating_option_key IS '满意度rating_option_key字段';
COMMENT ON COLUMN workspace_issue_satisfactions.rating_value IS '满意度rating_value字段';
COMMENT ON COLUMN workspace_issue_satisfactions.rating_label IS '满意度rating_label字段';
COMMENT ON COLUMN workspace_issue_satisfactions.sentiment IS '满意度sentiment字段';
COMMENT ON COLUMN workspace_issue_satisfactions.selected_tags IS '满意度selected_tags字段';
COMMENT ON COLUMN workspace_issue_satisfactions.content IS '满意度content字段';
COMMENT ON COLUMN workspace_issue_satisfactions.resolved IS '满意度resolved字段';
COMMENT ON COLUMN workspace_issue_satisfactions.submitted_at IS '提交时间Unix秒，0为未提交';
COMMENT ON COLUMN workspace_issue_satisfactions.reviewer_name IS '满意度reviewer_name字段';
COMMENT ON COLUMN workspace_issue_satisfactions.reviewer_avatar IS '满意度reviewer_avatar字段';
COMMENT ON COLUMN workspace_issue_satisfactions.category_id IS '满意度category_id字段';
COMMENT ON COLUMN workspace_issue_satisfactions.category_name IS '满意度category_name字段';
COMMENT ON COLUMN workspace_issue_satisfactions.priority_id IS '满意度priority_id字段';
COMMENT ON COLUMN workspace_issue_satisfactions.priority_name IS '满意度priority_name字段';
COMMENT ON COLUMN workspace_issue_satisfactions.handlers_snapshot IS '首次开放时处理成员与角色快照';
COMMENT ON COLUMN workspace_issue_satisfactions.created_at IS '创建时间Unix秒';
COMMENT ON COLUMN workspace_issue_satisfactions.updated_at IS '更新时间Unix秒';
COMMENT ON COLUMN workspace_issue_satisfaction_invitations.id IS '满意度id字段';
COMMENT ON COLUMN workspace_issue_satisfaction_invitations.admin_id IS '租户隔离号';
COMMENT ON COLUMN workspace_issue_satisfaction_invitations.workspace_id IS '所属工单管理空间';
COMMENT ON COLUMN workspace_issue_satisfaction_invitations.satisfaction_id IS '满意度satisfaction_id字段';
COMMENT ON COLUMN workspace_issue_satisfaction_invitations.request_key IS '满意度request_key字段';
COMMENT ON COLUMN workspace_issue_satisfaction_invitations.kind IS '满意度kind字段';
COMMENT ON COLUMN workspace_issue_satisfaction_invitations.operator_id IS '满意度operator_id字段';
COMMENT ON COLUMN workspace_issue_satisfaction_invitations.created_at IS '创建时间Unix秒';

-- 仅为现有工单空间的预设管理员补齐管理权限。
INSERT INTO role_permissions(admin_id,role_id,permission_key,created_at,updated_at)
SELECT r.admin_id,r.id,'satisfaction:manage',EXTRACT(EPOCH FROM NOW())::BIGINT,EXTRACT(EPOCH FROM NOW())::BIGINT
FROM roles r JOIN workspaces w ON w.admin_id=r.admin_id AND w.id=r.workspace_id
WHERE r.scope='project' AND r.is_system=1 AND r.name IN ('perm_role_project_admin','${perm_role_project_admin}') AND w.workspace_type='issue'
ON CONFLICT(admin_id,role_id,permission_key) DO NOTHING;

-- +goose Down
-- 保留客户评价及无法区分来源的角色授权，防止回滚误删业务数据。
SELECT 1;

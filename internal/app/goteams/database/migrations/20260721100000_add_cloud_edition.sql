-- +goose Up

CREATE TABLE IF NOT EXISTS tenant_plans (
    id SERIAL PRIMARY KEY,
    admin_id INTEGER NOT NULL,
    plan_type VARCHAR(30) NOT NULL DEFAULT 'experience',
    expire_at BIGINT NOT NULL DEFAULT 0,
    member_limit INTEGER NOT NULL DEFAULT 30,
    project_limit INTEGER NOT NULL DEFAULT 10,
    custom_field_per_project_limit INTEGER NOT NULL DEFAULT 5,
    automation_rule_per_project_limit INTEGER NOT NULL DEFAULT 5,
    automation_exec_limit INTEGER NOT NULL DEFAULT 500,
    agent_limit INTEGER NOT NULL DEFAULT 1,
    storage_limit INTEGER NOT NULL DEFAULT 10,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    UNIQUE(admin_id)
);

COMMENT ON TABLE tenant_plans IS '租户套餐与配额';
COMMENT ON COLUMN tenant_plans.admin_id IS '租户管理员ID（多租户隔离）';
COMMENT ON COLUMN tenant_plans.plan_type IS 'experience=体验版，enterprise=企业版';
COMMENT ON COLUMN tenant_plans.expire_at IS '到期时间（Unix秒），0=未设置';
COMMENT ON COLUMN tenant_plans.custom_field_per_project_limit IS '每项目自定义字段上限';
COMMENT ON COLUMN tenant_plans.automation_rule_per_project_limit IS '每项目自动化规则上限（功能未实现）';
COMMENT ON COLUMN tenant_plans.storage_limit IS '存储配额上限（GB）';

CREATE INDEX IF NOT EXISTS idx_tenant_plans_admin ON tenant_plans(admin_id);

-- +goose Down

DROP TABLE IF EXISTS tenant_plans;

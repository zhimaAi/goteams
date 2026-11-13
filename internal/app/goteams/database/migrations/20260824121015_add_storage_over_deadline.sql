-- +goose Up

ALTER TABLE tenant_plans ADD COLUMN IF NOT EXISTS storage_over_deadline BIGINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN tenant_plans.storage_over_deadline IS '容量超限截止时间（Unix秒），0=未超限或已恢复；运营后台续费后需清空该值';

-- 配额上限调整：体验版 10G→50G、企业版 100G→200G，仅更新仍为旧默认值的存量数据，避免覆盖运营手动调整过的配额
UPDATE tenant_plans SET storage_limit = 50 WHERE plan_type = 'experience' AND storage_limit = 10;
UPDATE tenant_plans SET storage_limit = 200 WHERE plan_type = 'enterprise' AND storage_limit = 100;

ALTER TABLE tenant_plans ALTER COLUMN storage_limit SET DEFAULT 50;

-- +goose Down

ALTER TABLE tenant_plans DROP COLUMN IF EXISTS storage_over_deadline;
ALTER TABLE tenant_plans ALTER COLUMN storage_limit SET DEFAULT 10;

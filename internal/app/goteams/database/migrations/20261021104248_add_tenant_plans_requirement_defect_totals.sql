-- +goose Up
ALTER TABLE tenant_plans
    ADD COLUMN IF NOT EXISTS requirement_total int4 NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS defect_total int4 NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS storage_used int4 NOT NULL DEFAULT 0;

COMMENT ON COLUMN tenant_plans.requirement_total IS '需求总数';
COMMENT ON COLUMN tenant_plans.defect_total IS '缺陷总数';
COMMENT ON COLUMN tenant_plans.storage_used IS '已用存储空间（MB，1MB=1048576字节，合计后向上取整）';

-- +goose Down
ALTER TABLE tenant_plans
    DROP COLUMN IF EXISTS requirement_total,
    DROP COLUMN IF EXISTS defect_total,
    DROP COLUMN IF EXISTS storage_used;

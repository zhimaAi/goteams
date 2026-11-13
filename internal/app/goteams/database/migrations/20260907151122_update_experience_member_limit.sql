-- +goose Up
-- +goose StatementBegin

-- 体验版默认员工数上限由 30 调整为 50；
-- 仅更新仍为体验版且当前上限为 30 的租户，避免覆盖运营后台已手动调整的阈值。
UPDATE tenant_plans
SET member_limit = 50,
    updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE plan_type = 'experience'
  AND member_limit = 30;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 反向仅回退本次迁移明确调整过的记录（体验版且当前上限为 50）。
UPDATE tenant_plans
SET member_limit = 30,
    updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE plan_type = 'experience'
  AND member_limit = 50;

-- +goose StatementEnd

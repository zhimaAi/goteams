-- +goose Up
-- +goose StatementBegin

-- 付费版（企业版）每日 API 调用上限由 100000 调整为 10000。
-- 按需求对所有 enterprise 租户统一覆盖，不做「仅默认值」限定：
-- 该额度是随套餐发布的权益点，运营后台逐租户调整过的值同样按新口径收敛。
UPDATE tenant_plans
SET api_call_daily_limit = 10000,
    updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE plan_type = 'enterprise';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 反向只回退本次迁移明确调整过的记录（企业版且当前上限为 10000），
-- 避免把迁移后又由运营后台调整成其它值的租户一并改掉。
UPDATE tenant_plans
SET api_call_daily_limit = 100000,
    updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE plan_type = 'enterprise'
  AND api_call_daily_limit = 10000;

-- +goose StatementEnd

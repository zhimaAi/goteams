-- +goose Up
-- +goose StatementBegin
-- 旧配置错误地分别保存起点和终点；沿用原来源和目标状态，周期历史不改写。
CREATE OR REPLACE FUNCTION pg_temp.sla_single_transition(items JSONB) RETURNS JSONB LANGUAGE SQL IMMUTABLE AS $$
SELECT COALESCE(jsonb_agg(
  CASE WHEN item->'config' ? 'start' AND item->'config' ? 'end'
  THEN jsonb_set(item,'{config}',jsonb_build_object('from',item->'config'->'start'->'from','to',item->'config'->'end'->'to'))
  ELSE item END ORDER BY position), '[]'::JSONB)
FROM jsonb_array_elements(items) WITH ORDINALITY AS entries(item,position);
$$;
UPDATE workspace_sla_settings
SET stages=pg_temp.sla_single_transition(stages),config_revision=config_revision+1
WHERE EXISTS (SELECT 1 FROM jsonb_array_elements(stages) item WHERE item->'config' ? 'start' AND item->'config' ? 'end');
-- 最近匹配用于未开始展示及同状态重提，采用新定义；执行周期仍保留原版本快照。
UPDATE requirements SET sla_match=jsonb_set(jsonb_set(sla_match,'{snapshot,stages}',pg_temp.sla_single_transition(sla_match->'snapshot'->'stages')),'{snapshot,schema_version}','2'::JSONB)
WHERE sla_match->'snapshot'->>'schema_version'='1';
ALTER TABLE requirement_sla_cycles DROP CONSTRAINT requirement_sla_cycles_start_reason_check;
ALTER TABLE requirement_sla_cycles ADD CONSTRAINT requirement_sla_cycles_start_reason_check CHECK (start_reason IN ('created','transition','resubmitted'));
-- +goose StatementEnd

-- +goose Down
-- 已产生按创建时刻起算的周期，不能安全回退定义或收紧枚举而不破坏历史。
SELECT 1;

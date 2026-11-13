-- +goose Up
-- 时效结论由两态扩展为三态：0=未超时，1=即将超时，2=已超时。数值越大越严重，
-- 列表「时效」列直接按这个顺序取最严重的一档。
-- 存量只有 0/1（1 表示已超时），需要先改写成新语义的 2。这一步由 goose 版本保证只跑一次，
-- 重跑会把新写入的 1（即将超时）误改成 2，所以不要手工补跑。
UPDATE requirements SET aging_status = 2 WHERE aging_status = 1;

COMMENT ON COLUMN "requirements"."aging_status" IS '需求整体时效结论：0=未超时，1=即将超时，2=已超时；由 SLA Worker 维护';

-- +goose Down
-- 无法区分历史数据里的 1（旧语义：已超时）与新写入的 1（新语义：即将超时），
-- 所以 Down 只还原列注释，不回写数据。aging_status 是派生列，
-- SLA Worker 会在下一轮（60 秒内）按旧语义重新算出并刷新所有需求。
COMMENT ON COLUMN "requirements"."aging_status" IS '需求整体时效结论：0=未超时，1=已超时；由 SLA Worker 维护';

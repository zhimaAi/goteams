-- +goose Up

-- 外部系统通知策略的今日触发计数归属日：today_trigger_count 没有调度器清零，
-- 由该列记录计数属于哪一天的零点，写入和读取都据此判断跨天。
ALTER TABLE external_notification_strategies
    ADD COLUMN IF NOT EXISTS today_trigger_date BIGINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN external_notification_strategies.today_trigger_date IS 'today_trigger_count 归属日的零点时间戳，0 表示尚无归属日';

-- 存量策略的今日计数是累计值的镜像，无法确定其归属日，统一按 0 展示直到下次触发。
UPDATE external_notification_strategies
    SET today_trigger_count = 0
    WHERE today_trigger_count <> 0;

-- +goose Down

-- 计数已被清零且无法区分历史归属，回滚不还原 today_trigger_count。
ALTER TABLE external_notification_strategies
    DROP COLUMN IF EXISTS today_trigger_date;

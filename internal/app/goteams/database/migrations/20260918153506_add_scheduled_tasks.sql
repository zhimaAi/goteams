-- +goose Up
-- +goose StatementBegin

-- 定时任务基础组件：与 automation_rules（事件驱动队列）分离，只按时间维度调度。
-- 计时层按 next_run_at 领取，执行层靠 lease_owner/lease_until 租约保证同一任务不被重复领取。
CREATE TABLE IF NOT EXISTS scheduled_tasks (
    id               SERIAL PRIMARY KEY,
    task_key         VARCHAR(50)  NOT NULL,
    scope_key        VARCHAR(100) NOT NULL,
    admin_id         INTEGER      NOT NULL DEFAULT 0,
    enabled          SMALLINT     NOT NULL DEFAULT 1,
    payload          TEXT,
    schedule_type    VARCHAR(20)  NOT NULL DEFAULT 'interval',
    interval_seconds INTEGER      NOT NULL DEFAULT 0,
    run_at_weekday   SMALLINT     NOT NULL DEFAULT 0,
    run_at_hour      SMALLINT     NOT NULL DEFAULT 0,
    run_at_minute    SMALLINT     NOT NULL DEFAULT 0,
    next_run_at      BIGINT       NOT NULL DEFAULT 0,
    last_run_at      BIGINT       NOT NULL DEFAULT 0,
    last_status      VARCHAR(20)  NOT NULL DEFAULT '',
    last_error       VARCHAR(500) NOT NULL DEFAULT '',
    lease_owner      VARCHAR(100) NOT NULL DEFAULT '',
    lease_until      BIGINT       NOT NULL DEFAULT 0,
    attempts         INTEGER      NOT NULL DEFAULT 0,
    created_at       BIGINT       NOT NULL DEFAULT 0,
    updated_at       BIGINT       NOT NULL DEFAULT 0
);

COMMENT ON TABLE "scheduled_tasks" IS '定时任务表（通用基础组件）';
COMMENT ON COLUMN "scheduled_tasks"."task_key" IS '任务类型，如 sso_member_sync；与 handler 注册表一一对应';
COMMENT ON COLUMN "scheduled_tasks"."scope_key" IS '业务唯一键，如 sso_member_sync:12；(task_key, scope_key) 唯一';
COMMENT ON COLUMN "scheduled_tasks"."admin_id" IS '归属租户，0=全局任务';
COMMENT ON COLUMN "scheduled_tasks"."enabled" IS '是否参与自动调度: 0=不参与（手动入口仍可领取）, 1=参与';
COMMENT ON COLUMN "scheduled_tasks"."payload" IS '任务参数JSON';
COMMENT ON COLUMN "scheduled_tasks"."schedule_type" IS '调度类型: interval=固定间隔, daily=每日, weekly=每周';
COMMENT ON COLUMN "scheduled_tasks"."interval_seconds" IS 'interval类型的间隔秒数';
COMMENT ON COLUMN "scheduled_tasks"."run_at_weekday" IS 'weekly执行日: 1=周一…7=周日';
COMMENT ON COLUMN "scheduled_tasks"."run_at_hour" IS 'daily/weekly执行小时，按东八区解析';
COMMENT ON COLUMN "scheduled_tasks"."run_at_minute" IS 'daily/weekly执行分钟，按东八区解析';
COMMENT ON COLUMN "scheduled_tasks"."next_run_at" IS '下次触发时间（Unix秒），只在执行完成后推进';
COMMENT ON COLUMN "scheduled_tasks"."last_run_at" IS '上次执行完成时间（Unix秒）；手动执行不写';
COMMENT ON COLUMN "scheduled_tasks"."last_status" IS '上次执行结果: success/failed';
COMMENT ON COLUMN "scheduled_tasks"."last_error" IS '上次执行失败原因，成功时清空';
COMMENT ON COLUMN "scheduled_tasks"."lease_owner" IS '当前执行者令牌；续租/释放/状态回写都带它做条件更新，行数为0即失去执行权';
COMMENT ON COLUMN "scheduled_tasks"."lease_until" IS '租约到期时间（Unix秒），过期即可被重新领取（at-least-once）';
COMMENT ON COLUMN "scheduled_tasks"."attempts" IS '累计执行次数';
COMMENT ON COLUMN "scheduled_tasks"."created_at" IS '创建时间（Unix时间戳）';
COMMENT ON COLUMN "scheduled_tasks"."updated_at" IS '更新时间（Unix时间戳）';

-- 一个业务对象在一个任务类型下只有一行，UpsertScheduledTask 据此幂等。
CREATE UNIQUE INDEX IF NOT EXISTS uq_scheduled_tasks_key_scope ON scheduled_tasks(task_key, scope_key);
-- 计时层的领取查询形状：enabled = 1 AND next_run_at <= now，按 next_run_at 升序。
CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_enabled_next ON scheduled_tasks(enabled, next_run_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS scheduled_tasks;

-- +goose StatementEnd

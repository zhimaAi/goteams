// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"goteams/internal/app/goteams/define"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

const (
	// schedulerPollInterval 是队列取空后的等待间隔。定时任务的最小粒度是分钟级，
	// 不需要 automation 的 500ms，凌晨批量也不想给数据库增加空转查询。
	schedulerPollInterval = 5 * time.Second
	// schedulerLeaseSeconds 是一次领取后租约的时长，运行中的任务由续租维持。
	schedulerLeaseSeconds = 300
	// schedulerDefaultConcurrency 是 [scheduler] concurrency 缺失或非法时的兜底。
	schedulerDefaultConcurrency = 50
	// schedulerMinIntervalSeconds 防止误配的 interval 任务把领取循环打满。
	schedulerMinIntervalSeconds = 60
	// schedulerErrorMaxRunes 与 scheduled_tasks.last_error 的 VARCHAR(500) 对齐。
	schedulerErrorMaxRunes = 500
)

// scheduledTaskColumns 是两条领取 SQL 共用的 RETURNING 列清单，必须与 claimScheduledTask 的 Scan 顺序一致。
const scheduledTaskColumns = `t.id, t.task_key, t.scope_key, t.admin_id, COALESCE(t.payload, ''), t.enabled,
	t.schedule_type, t.interval_seconds, t.run_at_weekday, t.run_at_hour, t.run_at_minute, t.next_run_at, t.attempts`

// schedulerClaimSQL 是计时层的自动领取：只取启用的、到期的、且租约已过期的行。
// FOR UPDATE SKIP LOCKED 让并发的领取者互不等待，直接跳过被锁住的行。
const schedulerClaimSQL = `WITH candidate AS (
	SELECT t.id FROM scheduled_tasks t
	WHERE t.enabled = 1 AND t.next_run_at <= $1 AND t.lease_until < $1
	ORDER BY t.next_run_at ASC
	FOR UPDATE SKIP LOCKED LIMIT 1
) UPDATE scheduled_tasks t SET lease_owner = $2, lease_until = $3, updated_at = $1
	FROM candidate WHERE t.id = candidate.id
	RETURNING ` + scheduledTaskColumns

// schedulerManualClaimSQL 是手动触发的领取。这里刻意不带 enabled = 1：租户的自动周期可以是「不自动」，
// 但手动同步必须还能跑；也不看 next_run_at，手动触发与调度游标无关。
const schedulerManualClaimSQL = `WITH candidate AS (
	SELECT t.id FROM scheduled_tasks t
	WHERE t.task_key = $2 AND t.scope_key = $3 AND t.lease_until < $1
	ORDER BY t.id ASC
	FOR UPDATE SKIP LOCKED LIMIT 1
) UPDATE scheduled_tasks t SET lease_owner = $4, lease_until = $5, updated_at = $1
	FROM candidate WHERE t.id = candidate.id
	RETURNING ` + scheduledTaskColumns

// 领取条件：租约未过期即视为已有执行者，手动触发因此会拿到 ErrSchedulerTaskBusy。
var (
	ErrSchedulerTaskBusy   = errors.New("scheduled task is already running")
	ErrSchedulerTaskMissed = errors.New("scheduled task not found")
	ErrSchedulerStopped    = errors.New("scheduler is not running")
	// ErrSchedulerTaskNotScheduled 表示这个任务还没有调度行。领取为空既可能是租约被别人持有，
	// 也可能是根本没有这一行；两者处置完全相反（一个等，一个要先建行），不能都报成 ErrSchedulerTaskBusy，
	// 否则「从未配置」会被说成「正在运行」，把调用方和用户一起引到错误方向。
	ErrSchedulerTaskNotScheduled = errors.New("scheduled task row not found")
)

// ScheduleTypeInterval 等调度类型取值，与 scheduled_tasks.schedule_type 列的注释一致。
const (
	ScheduleTypeInterval = `interval`
	ScheduleTypeDaily    = `daily`
	ScheduleTypeWeekly   = `weekly`
)

// schedulerTimezone 用于解析 daily/weekly 的墙上时间。用 FixedZone 而不是 LoadLocation，
// 是因为运行镜像可能没有 tzdata，那时 LoadLocation 会失败并把触发时间挪到 UTC。
var schedulerTimezone = time.FixedZone("Asia/Shanghai", 8*60*60)

// ScheduledTask 是 scheduled_tasks 的一行，也是传给 handler 的任务上下文。
type ScheduledTask struct {
	ID          int    // 行 id
	TaskKey     string // 任务类型，决定路由到哪个 handler
	ScopeKey    string // 业务唯一键，如 sso_member_sync:12
	AdminId     int    // 归属租户，0=全局任务
	Payload     string // 任务参数 JSON
	Enabled     int    // 1=参与自动调度, 0=不参与（与 enabled 列同形，手动触发不受它影响）
	Schedule    string // interval / daily / weekly
	IntervalSec int    // interval_seconds
	Weekday     int    // run_at_weekday，1=周一…7=周日，仅 weekly 使用
	Hour        int    // run_at_hour，按东八区解析
	Minute      int    // run_at_minute，按东八区解析
	NextRunAt   int64  // 领取时刻的游标，回写时只作为参考值
	Attempts    int    // 累计执行次数
	LeaseOwner  string // 本次执行者的令牌；所有回写都以它为条件，行数为 0 即失去执行权
}

// ScheduledTaskHandler 执行一个已领取任务的业务逻辑。
//
// ponytail: 签名里没有 context，租约丢失时无法中途取消正在跑的任务，只能靠带 lease_owner 条件的
// 回写拒绝它的写入。扩展到多实例执行前需要补上 context、在丢失租约时取消，并在业务事务内复查执行权。
type ScheduledTaskHandler func(task ScheduledTask) error

// scheduler 持有当前进程内计时层与执行层的生命周期。
var scheduler struct {
	sync.Mutex
	running   bool // 是否接受新的执行请求；StopScheduler 在锁内翻 false，Trigger 在锁内检查后 Add
	cancel    context.CancelFunc
	ctx       context.Context
	wait      sync.WaitGroup
	handlers  map[string]ScheduledTaskHandler
	semaphore chan struct{}
	owner     string
}

// RegisterScheduledTask 把 handler 绑到任务类型上。在装配阶段调用，必须先于 StartScheduler。
func RegisterScheduledTask(taskKey string, handler ScheduledTaskHandler) {
	scheduler.Lock()
	defer scheduler.Unlock()
	if scheduler.handlers == nil {
		scheduler.handlers = map[string]ScheduledTaskHandler{}
	}
	scheduler.handlers[taskKey] = handler
}

// UpsertScheduledTask 按 (task_key, scope_key) 创建或更新一个任务的调度参数。
// 计时层每次领取都重新读表，所以改周期在下一次领取就生效，不需要重启调度器。
func UpsertScheduledTask(task ScheduledTask) error {
	now := time.Now().Unix()
	_, err := msql.RawExec(define.Postgres, `INSERT INTO scheduled_tasks
		(task_key,scope_key,admin_id,payload,enabled,schedule_type,interval_seconds,
		run_at_weekday,run_at_hour,run_at_minute,next_run_at,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12)
		ON CONFLICT(task_key,scope_key) DO UPDATE SET
		admin_id=EXCLUDED.admin_id, payload=EXCLUDED.payload, enabled=EXCLUDED.enabled,
		schedule_type=EXCLUDED.schedule_type, interval_seconds=EXCLUDED.interval_seconds,
		run_at_weekday=EXCLUDED.run_at_weekday, run_at_hour=EXCLUDED.run_at_hour,
		run_at_minute=EXCLUDED.run_at_minute, next_run_at=EXCLUDED.next_run_at, updated_at=EXCLUDED.updated_at`, nil,
		task.TaskKey, task.ScopeKey, task.AdminId, task.Payload, task.Enabled, task.Schedule, task.IntervalSec,
		task.Weekday, task.Hour, task.Minute, nextRunAt(task, now), now)
	if err != nil {
		logs.Error("upsert scheduled task error: key=%s, scope=%s, err=%s", task.TaskKey, task.ScopeKey, err.Error())
	}
	return err
}

// DeleteScheduledTask 移除一个任务的调度行（同步配置被删除时调用）。
func DeleteScheduledTask(taskKey, scopeKey string) error {
	_, err := msql.RawExec(define.Postgres, `DELETE FROM scheduled_tasks WHERE task_key=$1 AND scope_key=$2`, nil, taskKey, scopeKey)
	if err != nil {
		logs.Error("delete scheduled task error: key=%s, scope=%s, err=%s", taskKey, scopeKey, err.Error())
	}
	return err
}

// StartScheduler 启动计时循环。幂等，重复调用不会派生第二个循环。
func StartScheduler() {
	scheduler.Lock()
	defer scheduler.Unlock()
	if scheduler.running {
		return
	}
	concurrency := schedulerConcurrency()
	ctx, cancel := context.WithCancel(context.Background())
	scheduler.running = true
	scheduler.ctx = ctx
	scheduler.cancel = cancel
	scheduler.semaphore = make(chan struct{}, concurrency)
	scheduler.owner = schedulerOwnerToken()
	logs.Info("start scheduler, concurrency:%d, poll:%s, lease:%ds", concurrency, schedulerPollInterval, schedulerLeaseSeconds)
	scheduler.wait.Add(1)
	go schedulerLoop(ctx)
}

// StopScheduler 停止领取并等待在跑的 handler 收尾。
func StopScheduler() {
	scheduler.Lock()
	cancel := scheduler.cancel
	scheduler.cancel = nil
	scheduler.ctx = nil
	scheduler.running = false
	scheduler.Unlock()
	if cancel != nil {
		cancel()
		scheduler.wait.Wait()
		logs.Info("scheduler stopped")
	}
}

// TriggerScheduledTask 手动触发一个任务：立刻领取并在调用方 goroutine 内执行完再返回，不推进 next_run_at。
// 任务正在执行返回 ErrSchedulerTaskBusy，任务类型未注册返回 ErrSchedulerTaskMissed，
// 任务还没有调度行返回 ErrSchedulerTaskNotScheduled，调度循环已经停下返回 ErrSchedulerStopped。
// 执行是阻塞的：调用方要等本次执行结束才拿得到结果，因此请求侧的超时按最长执行时间配置。
func TriggerScheduledTask(taskKey, scopeKey string) error {
	scheduler.Lock()
	if !scheduler.running {
		scheduler.Unlock()
		return ErrSchedulerStopped
	}
	// running 与 StopScheduler 共用同一把锁：Add 要么在 Wait 开始之前完成，要么看到 running 已经是 false。
	handler, ctx, owner := scheduler.handlers[taskKey], scheduler.ctx, scheduler.owner
	scheduler.wait.Add(1)
	defer scheduler.wait.Done()
	scheduler.Unlock()
	if handler == nil {
		return fmt.Errorf("%w: %s", ErrSchedulerTaskMissed, taskKey)
	}
	now := time.Now().Unix()
	task, err := claimScheduledTask(schedulerManualClaimSQL, owner, now, taskKey, scopeKey, owner, now+schedulerLeaseSeconds)
	if err != nil {
		return err
	}
	if task == nil {
		// 领取为空有两种原因：没有这一行，或租约未过期（确实在执行）。领取 SQL 只能看到「没领到」，
		// 分不出是哪一种，而报错方向正好相反，所以这里补一次存在性判断。
		exists, existsErr := scheduledTaskExists(taskKey, scopeKey)
		if existsErr != nil {
			return existsErr
		}
		if !exists {
			return fmt.Errorf("%w: %s/%s", ErrSchedulerTaskNotScheduled, taskKey, scopeKey)
		}
		return ErrSchedulerTaskBusy
	}
	// 手动触发不进 semaphore：同一行的租约已经保证它自身不重入，再由用户点击次数限量。
	// 需要给手动执行加并发上限时，改成先抢 semaphore、抢不到就 releaseLease。
	// 在调用方 goroutine 内跑完：手动同步接口因此阻塞到本次同步结束，由接口回读结果。
	runScheduledTask(ctx, *task, handler, true)
	return nil
}

// schedulerConcurrency 读取 [scheduler] concurrency，缺失或非正数回落到默认值。
func schedulerConcurrency() int {
	value := cast.ToInt(define.Config.Scheduler[`concurrency`])
	if value <= 0 {
		return schedulerDefaultConcurrency
	}
	return value
}

// schedulerOwnerToken 是本进程在租约上的身份。它是进程级的，正好挡住今天真正会发生的场景
// （上一次重启残留的 goroutine），而不是跨主机场景。
func schedulerOwnerToken() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "goteams"
	}
	return truncateOauthText(fmt.Sprintf("%s-%d-%d", hostname, os.Getpid(), time.Now().UnixNano()%1e6), 100)
}

// schedulerLoop 连续领取：计时层不能因为某个 handler 还在跑就放慢自己，所以只在确实领不到时才睡。
func schedulerLoop(ctx context.Context) {
	defer scheduler.wait.Done()
	for {
		if ctx.Err() != nil {
			return
		}
		now := time.Now().Unix()
		task, err := claimScheduledTask(schedulerClaimSQL, scheduler.owner, now, scheduler.owner, now+schedulerLeaseSeconds)
		if err != nil {
			logs.Error("scheduler claim error: %s", err.Error())
			schedulerSleep(ctx, time.Second)
			continue
		}
		if task == nil {
			schedulerSleep(ctx, schedulerPollInterval)
			continue
		}
		// 在 semaphore 上阻塞就是背压本身：待执行的任务留在表里而不是内存队列里，
		// 于是进程在开始执行前挂掉，也只是等租约过期后被重新领取。
		select {
		case <-ctx.Done():
			releaseLease(*task, "进程退出，本次未执行")
			return
		case scheduler.semaphore <- struct{}{}:
		}
		scheduler.wait.Add(1)
		handler := schedulerHandler(task.TaskKey)
		go func(current ScheduledTask) {
			defer func() {
				<-scheduler.semaphore
				scheduler.wait.Done()
			}()
			runScheduledTask(ctx, current, handler, false)
		}(*task)
	}
}

// schedulerHandler 在读锁保护下取任务类型对应的 handler。
func schedulerHandler(taskKey string) ScheduledTaskHandler {
	scheduler.Lock()
	defer scheduler.Unlock()
	return scheduler.handlers[taskKey]
}

func schedulerSleep(ctx context.Context, duration time.Duration) {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// claimScheduledTask 取租约并返回这一行，没有可领的就返回 nil。owner 只用于回填执行权身份，
// 它作为参数的位置由调用方按各自的 SQL 排好。
// FOR UPDATE 必须在事务里，所以这里照抄 claimAutomationExecution 的短事务形状。
func claimScheduledTask(query string, owner string, args ...any) (*ScheduledTask, error) {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	task := &ScheduledTask{}
	err = tx.QueryRow(query, args...).Scan(&task.ID, &task.TaskKey, &task.ScopeKey, &task.AdminId,
		&task.Payload, &task.Enabled, &task.Schedule, &task.IntervalSec, &task.Weekday, &task.Hour,
		&task.Minute, &task.NextRunAt, &task.Attempts)
	if errors.Is(err, sql.ErrNoRows) {
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	task.LeaseOwner = owner
	return task, nil
}

// scheduledTaskExists 判断某个任务是否已经有调度行。只在手动触发领取为空时调用：
// 它把「从未排期」与「租约被别人持有」分开，其余路径不需要这一次查询。
func scheduledTaskExists(taskKey, scopeKey string) (bool, error) {
	rows, err := msql.RawValues(define.Postgres,
		`SELECT id FROM scheduled_tasks WHERE task_key=$1 AND scope_key=$2 LIMIT 1`, nil, taskKey, scopeKey)
	if err != nil {
		logs.Error("check scheduled task exists error: key=%s, scope=%s, err=%s", taskKey, scopeKey, err.Error())
		return false, err
	}
	return len(rows) > 0, nil
}

// runScheduledTask 执行一个已领取的任务：跑的时候持续续租，跑完再回写结果。
func runScheduledTask(ctx context.Context, task ScheduledTask, handler ScheduledTaskHandler, manual bool) {
	if handler == nil {
		finishScheduledTask(task, manual, "failed", fmt.Sprintf("未注册的任务类型: %s", task.TaskKey))
		return
	}
	stopRenew := startLeaseRenew(ctx, task)
	defer stopRenew()
	if err := handler(task); err != nil {
		logs.Error("scheduled task %s/%d failed: %s", task.TaskKey, task.ID, err.Error())
		finishScheduledTask(task, manual, "failed", err.Error())
		return
	}
	finishScheduledTask(task, manual, "success", "")
}

// startLeaseRenew 在 handler 运行期间保持租约有效，这样「同一任务不重复进入」不需要额外的锁。
func startLeaseRenew(ctx context.Context, task ScheduledTask) func() {
	done := make(chan struct{})
	var once sync.Once
	go func() {
		interval := time.Duration(schedulerLeaseSeconds/3) * time.Second
		for {
			timer := time.NewTimer(interval)
			select {
			case <-done:
				timer.Stop()
				return
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			if !renewLease(task) {
				return
			}
		}
	}()
	return func() { once.Do(func() { close(done) }) }
}

// renewLease 只在仍然持有租约时续期。影响行数为 0 说明租约已过期且被别人抢走，
// 于是停止续租，后续的回写也会被同一个条件挡掉。
func renewLease(task ScheduledTask) bool {
	now := time.Now().Unix()
	return schedulerConditionalUpdate(`UPDATE scheduled_tasks SET lease_until=$1, updated_at=$2
		WHERE id=$3 AND lease_owner=$4`, now+schedulerLeaseSeconds, now, task.ID, task.LeaseOwner)
}

// 自动执行推进游标并记录完成时间；手动执行两者都不写，避免绕过调度周期污染下一次触发。
const schedulerFinishAutoSQL = `UPDATE scheduled_tasks SET next_run_at=$1, last_run_at=$2, last_status=$3,
	last_error=$4, attempts=attempts+1, lease_owner='', lease_until=0, updated_at=$2
	WHERE id=$5 AND lease_owner=$6`

const schedulerFinishManualSQL = `UPDATE scheduled_tasks SET last_status=$1, last_error=$2,
	attempts=attempts+1, lease_owner='', lease_until=0, updated_at=$3
	WHERE id=$4 AND lease_owner=$5`

// finishScheduledTask 在一次执行结束后回写结果并释放租约，返回是否仍然握有执行权。
func finishScheduledTask(task ScheduledTask, manual bool, status, reason string) bool {
	now := time.Now().Unix()
	message := truncateOauthText(reason, schedulerErrorMaxRunes)
	var owned bool
	if manual {
		owned = schedulerConditionalUpdate(schedulerFinishManualSQL, status, message, now, task.ID, task.LeaseOwner)
	} else {
		owned = schedulerConditionalUpdate(schedulerFinishAutoSQL, nextRunAt(task, now), now, status, message, task.ID, task.LeaseOwner)
	}
	if !owned {
		logs.Error("scheduler lost execution rights before finish, result dropped: id=%d, key=%s", task.ID, task.TaskKey)
	}
	return owned
}

// releaseLease 在还没开始执行就退出时归还行：游标和 last_run_at 都不动，下一次照样到期可领。
func releaseLease(task ScheduledTask, reason string) {
	now := time.Now().Unix()
	schedulerConditionalUpdate(`UPDATE scheduled_tasks SET lease_owner='', lease_until=0, last_error=$2, updated_at=$1
		WHERE id=$3 AND lease_owner=$4`, now, truncateOauthText(reason, schedulerErrorMaxRunes), task.ID, task.LeaseOwner)
}

// schedulerConditionalUpdate 执行一条必须以本执行者身份为条件的 UPDATE，并回报它当时是否还是。
// 所有回写都走这里，所以一个过期的执行者永远覆盖不了新执行者的状态。
func schedulerConditionalUpdate(query string, args ...any) bool {
	result, err := msql.RawExec(define.Postgres, query, nil, args...)
	if err != nil {
		logs.Error("scheduler update error: sql=%s, err=%s", query, err.Error())
		return false
	}
	affected, err := result.RowsAffected()
	if err != nil {
		logs.Error("scheduler rows affected error: sql=%s, err=%s", query, err.Error())
		return false
	}
	return affected > 0
}

// nextRunAt 从 from 这个时刻起算下一次触发时间，东八区墙上时间只在 daily/weekly 参与解析。
func nextRunAt(task ScheduledTask, from int64) int64 {
	switch task.Schedule {
	case ScheduleTypeDaily, ScheduleTypeWeekly:
		return nextWallClockRunAt(task, from)
	default:
		interval := task.IntervalSec
		if interval < schedulerMinIntervalSeconds {
			interval = schedulerMinIntervalSeconds
		}
		return from + int64(interval)
	}
}

// nextWallClockRunAt 返回严格晚于 from 的下一个执行时刻。基准是执行结束的时刻，
// 所以一个跑得很久的任务不会立刻又被触发一次。
func nextWallClockRunAt(task ScheduledTask, from int64) int64 {
	base := time.Unix(from, 0).In(schedulerTimezone)
	stamp := time.Date(base.Year(), base.Month(), base.Day(), task.Hour, task.Minute, 0, 0, schedulerTimezone)
	days := 0
	if task.Schedule == ScheduleTypeWeekly {
		days = int(time.Weekday(task.Weekday%7)) - int(base.Weekday())
		if days < 0 {
			days += 7
		}
	}
	stamp = stamp.AddDate(0, 0, days)
	if !stamp.After(base) {
		stamp = stamp.AddDate(0, 0, schedulerDailyStep(task.Schedule))
	}
	return stamp.Unix()
}

// schedulerDailyStep 是同一个墙上时间在 weekly 下隔 7 天、在 daily 下隔 1 天。
func schedulerDailyStep(schedule string) int {
	if schedule == ScheduleTypeWeekly {
		return 7
	}
	return 1
}

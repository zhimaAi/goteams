package business

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"goteams/internal/app/goteams/define"

	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

const tenantRequirementTotalsBatchSize = 100

var errTenantRequirementTotalsLeaseLost = errors.New("tenant requirement totals task lost its lease")

type tenantRequirementTotalsPlan struct {
	Id      int
	AdminId int
}

// TenantRequirementTotalsHandler 按租户刷新需求、缺陷总数和存储用量，单个租户失败不阻塞其他租户。
func TenantRequirementTotalsHandler(task ScheduledTask) error {
	if task.AdminId != 0 || task.TaskKey != define.TenantRequirementTotalsTaskKey || task.ScopeKey != "global" || task.ID <= 0 || task.LeaseOwner == "" {
		return errors.New("invalid tenant requirement totals task")
	}
	scheduler.Lock()
	parent := scheduler.ctx
	scheduler.Unlock()
	if parent == nil {
		return ErrSchedulerStopped
	}
	// 一轮有时限，关闭调度器也会取消本轮查询；新租户留到下一轮，避免遍历持续增长。
	ctx, cancel := context.WithTimeout(parent, 30*time.Minute)
	defer cancel()
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return err
	}
	return refreshTenantRequirementTotals(ctx, db, task)
}

func refreshTenantRequirementTotals(ctx context.Context, db *sql.DB, task ScheduledTask) (err error) {
	started := time.Now()
	succeeded, failed := 0, 0
	defer func() {
		logs.Info("tenant requirement totals finished: success=%d, failed=%d, elapsed=%s, error=%v", succeeded, failed, time.Since(started), err)
	}()
	query := `SELECT COALESCE(MAX(id), 0) FROM ` + define.TableTenantPlans
	limitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	var maxId int
	err = db.QueryRowContext(limitCtx, query).Scan(&maxId)
	cancel()
	if err != nil {
		logs.Error("tenant requirement totals limit failed: sql=%s, err=%s", query, err.Error())
		return err
	}
	for afterId := 0; afterId < maxId; {
		plans, pageErr := listTenantRequirementTotalsPlans(ctx, db, afterId, maxId)
		if pageErr != nil {
			return pageErr
		}
		if len(plans) == 0 {
			break
		}
		for _, plan := range plans {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := refreshTenantRequirementTotalsPlan(ctx, db, task, plan); err != nil {
				failed++
				logs.Error("tenant requirement totals failed: admin_id=%d, err=%s", plan.AdminId, err.Error())
				if errors.Is(err, errTenantRequirementTotalsLeaseLost) || ctx.Err() != nil {
					return err
				}
				continue
			}
			succeeded++
		}
		afterId = plans[len(plans)-1].Id
	}
	if failed > 0 {
		return fmt.Errorf("tenant requirement totals: %d tenants failed, %d succeeded", failed, succeeded)
	}
	return nil
}

func listTenantRequirementTotalsPlans(ctx context.Context, db *sql.DB, afterId, maxId int) ([]tenantRequirementTotalsPlan, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	query := `SELECT id, admin_id FROM ` + define.TableTenantPlans +
		` WHERE id > $1 AND id <= $2 ORDER BY id ASC LIMIT $3`
	rows, err := db.QueryContext(ctx, query, afterId, maxId, tenantRequirementTotalsBatchSize)
	if err != nil {
		logs.Error("list tenant requirement totals plans failed: sql=%s, err=%s", query, err.Error())
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			logs.Error("close tenant requirement totals plans failed: sql=%s, err=%s", query, err.Error())
		}
	}()
	plans := make([]tenantRequirementTotalsPlan, 0, tenantRequirementTotalsBatchSize)
	for rows.Next() {
		var plan tenantRequirementTotalsPlan
		if err := rows.Scan(&plan.Id, &plan.AdminId); err != nil {
			logs.Error("scan tenant requirement totals plan failed: sql=%s, err=%s", query, err.Error())
			return nil, err
		}
		plans = append(plans, plan)
	}
	if err := rows.Err(); err != nil {
		logs.Error("read tenant requirement totals plans failed: sql=%s, err=%s", query, err.Error())
		return nil, err
	}
	return plans, nil
}

func refreshTenantRequirementTotalsPlan(ctx context.Context, db *sql.DB, task ScheduledTask, plan tenantRequirementTotalsPlan) (err error) {
	if plan.AdminId <= 0 {
		return errors.New("invalid tenant admin_id")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			logs.Error("rollback tenant requirement totals failed: admin_id=%d, err=%s", plan.AdminId, rollbackErr.Error())
		}
	}()
	// 进程级租约令牌不能区分同进程的两次领取；计数前锁行，避免旧计数覆盖新结果。
	// 每个租户至多持锁 30 秒，短于 300 秒租约，租户之间释放供续租和领取使用。
	query := `SELECT lease_owner, lease_until FROM ` + define.TableScheduledTasks +
		` WHERE id=$1 AND task_key=$2 AND scope_key=$3 AND admin_id=0 FOR UPDATE`
	var owner string
	var leaseUntil int64
	if err = tx.QueryRowContext(ctx, query, task.ID, task.TaskKey, task.ScopeKey).Scan(&owner, &leaseUntil); err != nil {
		logs.Error("check tenant requirement totals lease failed: sql=%s, err=%s", query, err.Error())
		if errors.Is(err, sql.ErrNoRows) {
			return errTenantRequirementTotalsLeaseLost
		}
		return err
	}
	if owner != task.LeaseOwner || leaseUntil <= time.Now().Unix() {
		return errTenantRequirementTotalsLeaseLost
	}
	query = `SELECT type, COUNT(*) FROM ` + define.TableRequirements +
		` WHERE admin_id=$1 AND type IN ($2, $3) GROUP BY type`
	rows, err := tx.QueryContext(ctx, query, plan.AdminId, define.TypeKeyRequirement, define.TypeKeyDefect)
	if err != nil {
		logs.Error("count tenant work items failed: admin_id=%d, sql=%s, err=%s", plan.AdminId, query, err.Error())
		return err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			logs.Error("close tenant work item counts failed: admin_id=%d, err=%s", plan.AdminId, closeErr.Error())
		}
	}()
	// 无记录的类型不会返回分组，保持零值；只有完整读取成功才更新套餐。
	var requirementTotal, defectTotal int64
	for rows.Next() {
		var itemType string
		var total int64
		if err = rows.Scan(&itemType, &total); err != nil {
			logs.Error("scan tenant work item count failed: admin_id=%d, sql=%s, err=%s", plan.AdminId, query, err.Error())
			return err
		}
		switch itemType {
		case define.TypeKeyRequirement:
			requirementTotal = total
		case define.TypeKeyDefect:
			defectTotal = total
		}
	}
	if err = rows.Err(); err != nil {
		logs.Error("read tenant work item counts failed: admin_id=%d, sql=%s, err=%s", plan.AdminId, query, err.Error())
		return err
	}
	// 文件台账包含附件、富文本资源和知识库文件；产出文档另存，沿用其用量口径排除软删行。
	// 定时汇总直接查库，避免把存储缓存的漂移固化到套餐统计中。
	query = `SELECT (SELECT COALESCE(SUM(file_size), 0) FROM ` + define.GetEnterpriseFileTable(plan.AdminId) + ` WHERE admin_id=$1)
		+ (SELECT COALESCE(SUM(file_size), 0) FROM ` + define.TablePipelineTaskDocument + ` WHERE admin_id=$1 AND deleted_at=0)`
	var storageBytes int64
	if err = tx.QueryRowContext(ctx, query, plan.AdminId).Scan(&storageBytes); err != nil {
		logs.Error("count tenant storage used failed: admin_id=%d, sql=%s, err=%s", plan.AdminId, query, err.Error())
		return err
	}
	// 套餐以 MB 保存，先合计字节再向上取整，避免逐文件取整放大用量。
	const bytesPerMB = 1024 * 1024
	storageUsed := storageBytes / bytesPerMB
	if storageBytes%bytesPerMB > 0 {
		storageUsed++
	}
	// 直接比较写入时的值，避免依赖分页阶段的旧值；数量不变时不产生更新。
	query = `UPDATE ` + define.TableTenantPlans + ` SET requirement_total=$1, defect_total=$2, storage_used=$3 WHERE id=$4 AND admin_id=$5
		AND (requirement_total <> $1 OR defect_total <> $2 OR storage_used <> $3)`
	if _, err = tx.ExecContext(ctx, query, requirementTotal, defectTotal, storageUsed, plan.Id, plan.AdminId); err != nil {
		logs.Error("update tenant requirement totals failed: admin_id=%d, sql=%s, err=%s", plan.AdminId, query, err.Error())
		return err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit tenant requirement totals failed: admin_id=%d, err=%s", plan.AdminId, err.Error())
		return err
	}
	return nil
}

package business

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"goteams/internal/app/goteams/define"
)

var warningWorkers struct {
	sync.Mutex
	cancel context.CancelFunc
	wait   sync.WaitGroup
}

// StartWarningWorkers 独立启动检测、站内投递与外发消费者，网络等待不占用检测槽位。
func StartWarningWorkers() {
	warningWorkers.Lock()
	defer warningWorkers.Unlock()
	if warningWorkers.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	warningWorkers.cancel = cancel
	start := func(work func(context.Context) error) {
		warningWorkers.wait.Add(1)
		go func() { defer warningWorkers.wait.Done(); runWarningLoop(ctx, work) }()
	}
	for i := 0; i < 2; i++ {
		start(evaluateWarningRule)
	}
	start(func(ctx context.Context) error { return deliverWarning(ctx, true) })
	for i := 0; i < 4; i++ {
		start(func(ctx context.Context) error { return deliverWarning(ctx, false) })
	}
}

// StopWarningWorkers 取消全部预警消费者并等待在途任务退出。
func StopWarningWorkers() {
	warningWorkers.Lock()
	defer warningWorkers.Unlock()
	if warningWorkers.cancel != nil {
		warningWorkers.cancel()
		warningWorkers.wait.Wait()
		warningWorkers.cancel = nil
	}
}

func runWarningLoop(ctx context.Context, work func(context.Context) error) {
	for ctx.Err() == nil {
		err := work(ctx)
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			logs.Error("warning worker failed: %v", err)
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

type warningClaim struct {
	Rule     WarningRuleModel
	Token    string
	Revision int64
	Config   warningConfig
}

const warningClaimSQL = "WITH candidate AS (SELECT id FROM " + define.TableWarningRules + " WHERE enabled AND next_check_at<=$1 AND next_notice_at<=$1 AND lease_until<=$1 ORDER BY next_check_at,id FOR UPDATE SKIP LOCKED LIMIT 1) UPDATE " + define.TableWarningRules + " r SET lease_token=$2,lease_until=$1+120,next_check_at=$1+10 FROM candidate WHERE r.id=candidate.id RETURNING r.id,r.admin_id,r.workspace_id,r.config_json,r.config_revision"

func claimWarning(ctx context.Context, db *sql.DB) (warningClaim, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	m := warningClaim{Token: uuid.NewString()}
	err := (warningSQL{db}).QueryRowContext(ctx, warningClaimSQL, time.Now().Unix(), m.Token).Scan(&m.Rule.Id, &m.Rule.AdminId, &m.Rule.WorkspaceId, &m.Rule.Config, &m.Revision)
	if err == nil {
		m.Config, err = decodeWarningConfig(m.Rule.Config)
	}
	return m, err
}

func evaluateWarningRule(parent context.Context) error {
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return err
	}
	claim, err := claimWarning(parent, db)
	if err != nil {
		if claim.Rule.Id > 0 {
			if finishErr := failWarning(db, claim); finishErr != nil {
				return finishErr
			}
		}
		if claim.Rule.Id > 0 {
			return fmt.Errorf("warning rule admin=%d workspace=%d id=%d: %w", claim.Rule.AdminId, claim.Rule.WorkspaceId, claim.Rule.Id, err)
		}
		return err
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	count, recipients, err := scanWarningWindow(ctx, db, claim)
	if err == nil {
		err = finishWarning(ctx, db, claim, count, recipients)
	}
	if err != nil {
		if finishErr := failWarning(db, claim); finishErr != nil {
			return finishErr
		}
	}
	if err != nil {
		return fmt.Errorf("warning rule admin=%d workspace=%d id=%d: %w", claim.Rule.AdminId, claim.Rule.WorkspaceId, claim.Rule.Id, err)
	}
	return nil
}

func failWarning(db *sql.DB, c warningClaim) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// 领取已提交，检测事务失败也不会回滚退避；版本和 token 防止覆盖新配置或新执行者。
	_, err := (warningSQL{db}).ExecContext(ctx, "UPDATE "+define.TableWarningRules+" SET next_check_at=$6,lease_token='',lease_until=0 WHERE admin_id=$1 AND workspace_id=$2 AND id=$3 AND lease_token=$4 AND config_revision=$5", c.Rule.AdminId, c.Rule.WorkspaceId, c.Rule.Id, c.Token, c.Revision, time.Now().Unix()+30)
	return err
}

func finishWarning(ctx context.Context, db *sql.DB, c warningClaim, count int, recipients []int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackWarning(tx)
	now := time.Now().Unix()
	var nextNotice int64
	err = (warningSQL{tx}).QueryRowContext(ctx, "SELECT next_notice_at FROM "+define.TableWarningRules+" WHERE admin_id=$1 AND workspace_id=$2 AND id=$3 AND enabled AND lease_token=$4 AND config_revision=$5 AND lease_until>$6 FOR UPDATE", c.Rule.AdminId, c.Rule.WorkspaceId, c.Rule.Id, c.Token, c.Revision, now).Scan(&nextNotice)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	triggered := count >= c.Config.Threshold && nextNotice <= now
	if triggered {
		raw, err := json.Marshal(recipients)
		if err != nil {
			return err
		}
		for _, channel := range c.Config.deliveryChannels() {
			config := c.Config
			config.Channels = nil
			config.Channel, config.URL, config.Secret, config.Content = channel.Channel, channel.URL, channel.Secret, channel.Content
			payload, err := json.Marshal(config)
			if err != nil {
				return err
			}
			if _, err = (warningSQL{tx}).ExecContext(ctx, "INSERT INTO "+define.TableWarningDeliveries+" (admin_id,workspace_id,rule_id,config_json,recipients) VALUES($1,$2,$3,$4,$5)", c.Rule.AdminId, c.Rule.WorkspaceId, c.Rule.Id, payload, raw); err != nil {
				return err
			}
		}
		nextNotice = now + int64(c.Config.IntervalMinutes)*60
	}
	if _, err = (warningSQL{tx}).ExecContext(ctx, "UPDATE "+define.TableWarningRules+" SET next_check_at=$6,next_notice_at=$7,lease_token='',lease_until=0 WHERE admin_id=$1 AND workspace_id=$2 AND id=$3 AND lease_token=$4 AND config_revision=$5", c.Rule.AdminId, c.Rule.WorkspaceId, c.Rule.Id, c.Token, c.Revision, now+10, nextNotice); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

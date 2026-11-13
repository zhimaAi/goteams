package business

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
)

func slaActionPlatforms(action slaAction) []string {
	if action.Channel != "webhook" {
		return []string{""}
	}
	platforms := make([]string, 0, len(action.Webhooks))
	for _, hook := range action.Webhooks {
		platforms = append(platforms, hook.Platform)
	}
	return platforms
}

// 四个常驻消费者共享任务，空闲槽位无需等待同批慢请求。
func runSLAWebhookWorker(ctx context.Context) {
	jobs := make(chan slaActionCandidate)
	var workers sync.WaitGroup
	for slot := 0; slot < 4; slot++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for item := range jobs {
				executeSLAAction(ctx, item)
			}
		}()
	}
	defer workers.Wait()
	defer close(jobs)
	for ctx.Err() == nil {
		rows, err := loadSLAWebhookCandidates(ctx)
		if err != nil || len(rows) == 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}
		for i := range rows {
			select {
			case <-ctx.Done():
				return
			case jobs <- rows[i]:
			}
		}
	}
}

func executeSLAWebhook(ctx context.Context, tx *sql.Tx, item slaActionCandidate, cycle slaCycle, action slaAction, platform string, now int64) error {
	var selected *slaWebhook
	for i := range action.Webhooks {
		if action.Webhooks[i].Platform == platform {
			selected = &action.Webhooks[i]
			break
		}
	}
	stage, ok := snapshotSLAStage(cycle.Snapshot, cycle.StageId)
	if selected == nil || !ok {
		return ErrSLAInvalid
	}
	token := uuid.NewString()
	if err := slaExec(tx, "UPDATE "+define.TableSLAActions+" SET next_attempt_at=$1,lease_token=$2 WHERE admin_id=$3 AND workspace_id=$4 AND id=$5 AND state='pending'", now+120, token, item.AdminId, item.WorkspaceId, item.Id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	// 网络超时必须短于租约；结果回写带租约标识，拒绝旧执行覆盖新结果。
	sendCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	snapshot, deliveryErr := captureSLAWebhookSnapshot(sendCtx, item)
	if deliveryErr == nil && snapshot == nil {
		deliveryErr = ErrRequirementNotFound
	}
	if deliveryErr == nil {
		event := AutomationEvent{AdminID: item.AdminId, WorkspaceID: item.WorkspaceId, ItemID: item.RequirementId, After: &snapshot.Item}
		content, contentErr := buildSLAWebhookContent(*snapshot, stage.Name, action, cycle.LimitSeconds, cycle.Snapshot.Config.Calendar.Timezone)
		if contentErr != nil {
			deliveryErr = contentErr
		} else {
			deliveryErr = sendExternalNotificationWebhookContext(sendCtx, selected.Platform, selected.URL, selected.Secret, notificationAtConfig{AtMode: selected.AtMode, AtUserIds: selected.AtUserIds, AtRoles: selected.AtRoles, AtAll: selected.AtAll}, content, event)
		}
	}
	return finishSLAWebhook(item, token, deliveryErr)
}

func finishSLAWebhook(item slaActionCandidate, token string, deliveryErr error) error {
	now := time.Now().Unix()
	query := "UPDATE " + define.TableSLAActions + " SET state='sent',sent_at=$1,reason='',updated_at=$1,lease_token='' WHERE admin_id=$2 AND workspace_id=$3 AND id=$4 AND state='pending' AND lease_token=$5"
	if deliveryErr != nil {
		logs.Error("SLA webhook failed, action_id:%d, err:%s", item.Id, deliveryErr.Error())
		query = slaActionFailureSQL
	}
	// 回写不沿用已超时的发送 Context，但仍限制数据库等待。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, query, now, item.AdminId, item.WorkspaceId, item.Id, token)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
	}
	return err
}

type slaWebhookSnapshot struct {
	Item          AutomationWorkItemSnapshot
	WorkspaceName string
	PriorityName  string
	AssigneeNames string
	CreatedAt     int64
}

func buildSLAWebhookContent(snapshot slaWebhookSnapshot, stage string, action slaAction, limitSeconds int64, timezone string) (string, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return "", err
	}
	priority := lib_i18n.ShowStored(notificationMessageLang, snapshot.PriorityName)
	if priority == "" {
		priority = lib_i18n.Show(notificationMessageLang, "sla_webhook_unspecified")
	}
	line := func(value string) string {
		value = strings.Join(strings.Fields(value), " ")
		if value == "" {
			return "—"
		}
		return value
	}
	title := lib_i18n.Show(notificationMessageLang, "sla_webhook_"+action.Position, float64(action.OffsetSeconds)/60)
	return lib_i18n.Show(notificationMessageLang, "sla_webhook_notice", title, line(snapshot.WorkspaceName), line(stage), line(snapshot.Item.Title), line(snapshot.AssigneeNames), line(priority), time.Unix(snapshot.CreatedAt, 0).In(location).Format("2006-01-02 15:04:05"), float64(limitSeconds)/60, externalNotificationItemLink(snapshot.Item.WorkspaceID, snapshot.Item.ID)), nil
}

// captureSLAWebhookSnapshot 仅读取通知需要的数据，避免加载无关自定义字段。
func captureSLAWebhookSnapshot(ctx context.Context, item slaActionCandidate) (*slaWebhookSnapshot, error) {
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return nil, err
	}
	query := `SELECT r.title,r.assignee_ids,r.created_by,r.created_at,w.name,COALESCE(p.name,''),
        COALESCE((SELECT string_agg(COALESCE(NULLIF(u.name,''),u.username,ru.name,''), ', ' ORDER BY assigned.ordinality)
          FROM unnest(r.assignee_ids) WITH ORDINALITY AS assigned(user_id,ordinality)
          LEFT JOIN ` + define.TableUsers + ` u ON u.admin_id=r.admin_id AND u.id=assigned.user_id
          LEFT JOIN removed_users ru ON ru.admin_id=r.admin_id AND ru.user_id=assigned.user_id), '')
        FROM ` + define.TableRequirements + ` r
        JOIN ` + define.TableWorkspaces + ` w ON w.admin_id=r.admin_id AND w.id=r.workspace_id
        LEFT JOIN workspace_priorities p ON p.admin_id=r.admin_id AND p.workspace_id=r.workspace_id AND p.type_key=r.type AND p.id=r.priority
        WHERE r.admin_id=$1 AND r.workspace_id=$2 AND r.id=$3 AND r.type='requirement' AND r.deleted_at=0`
	snapshot := &slaWebhookSnapshot{}
	var assignees pq.Int64Array
	var creator int
	err = db.QueryRowContext(ctx, query, item.AdminId, item.WorkspaceId, item.RequirementId).Scan(&snapshot.Item.Title, &assignees, &creator, &snapshot.CreatedAt, &snapshot.WorkspaceName, &snapshot.PriorityName, &snapshot.AssigneeNames)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRequirementNotFound
	}
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return nil, err
	}
	ids := make([]int, len(assignees))
	for i, id := range assignees {
		ids[i] = int(id)
	}
	snapshot.Item.ID, snapshot.Item.WorkspaceID, snapshot.Item.Type = item.RequirementId, item.WorkspaceId, "requirement"
	snapshot.Item.AssigneeIDs = ids
	snapshot.Item.Fields = map[string]any{"created_by": creator}
	return snapshot, nil
}

func loadSLAWebhookCandidates(ctx context.Context) ([]slaActionCandidate, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return nil, err
	}
	const query = "SELECT a.id,a.cycle_id,a.admin_id,a.workspace_id,a.requirement_id FROM " + define.TableSLAActions + " a JOIN " + define.TableSLASettings + " s ON s.admin_id=a.admin_id AND s.workspace_id=a.workspace_id WHERE s.enabled AND a.delivery_platform<>'' AND a.state='pending' AND a.next_attempt_at<=$1 AND (a.due_at<=$1 OR a.due_at IS NULL) ORDER BY a.next_attempt_at,a.id LIMIT 4"
	rows, err := db.QueryContext(ctx, query, time.Now().Unix())
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return nil, err
	}
	defer rows.Close()
	items := make([]slaActionCandidate, 0, 4)
	for rows.Next() {
		var item slaActionCandidate
		if err := rows.Scan(&item.Id, &item.CycleId, &item.AdminId, &item.WorkspaceId, &item.RequirementId); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return nil, err
	}
	return items, nil
}

// 有租约的发送结果必须匹配 token；领取前失败只允许更新空租约或已过期租约。
const slaActionFailureSQL = "UPDATE " + define.TableSLAActions + " SET attempts=attempts+1,state=CASE WHEN attempts+1>=10 THEN 'failed' ELSE 'pending' END,reason='delivery_failed',next_attempt_at=$1+LEAST(600,15*POWER(2,LEAST(attempts+1,5)))::BIGINT,updated_at=$1,lease_token='' WHERE admin_id=$2 AND workspace_id=$3 AND id=$4 AND state='pending' AND (($5<>'' AND lease_token=$5) OR ($5='' AND (lease_token='' OR next_attempt_at<=$1)))"

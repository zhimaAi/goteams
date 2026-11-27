package business

import (
	"database/sql"
	"strconv"

	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"goteams/internal/app/goteams/define"
)

// IssueUrgeRecordModel 保存一次可确认的催单事实；Legacy 表示更早历史已不可恢复。
type IssueUrgeRecordModel struct {
	Id        int
	CreatedAt int64
	Legacy    bool
}

// ListIssueUrgeRecordsModel 限定催单记录的身份、工作空间及分页范围。
type ListIssueUrgeRecordsModel struct {
	AdminId, WorkspaceId, RequirementId, UserId int
	Page, Size, Total                           int
	IncompleteHistory                           bool
	Items                                       []IssueUrgeRecordModel
}

type issueUrgeWriteModel struct {
	AdminId, WorkspaceId, RequirementId int
	Previous, OccurredAt                int64
}

// recordIssueUrgeTx 的调用方已锁定邀填关联行，补存最近旧记录与追加新记录不会并发重复。
func recordIssueUrgeTx(tx *sql.Tx, model issueUrgeWriteModel) error {
	previous, occurredAt := model.Previous, model.OccurredAt
	if tx == nil || model.AdminId <= 0 || model.WorkspaceId <= 0 || model.RequirementId <= 0 || previous < 0 || occurredAt <= 0 {
		return ErrIssueInviteInvalid
	}
	if previous > 0 {
		// 同一参数用于 SELECT 与 action 比较时，显式匹配列类型，避免 PostgreSQL 推断出 text/varchar 冲突。
		query := `INSERT INTO ` + define.TableRequirementHistories + `
			(admin_id,workspace_id,requirement_id,user_id,action,new_value,created_at)
			SELECT $1,$2,$3,0,CAST($4 AS VARCHAR(50)),'legacy',$5 WHERE NOT EXISTS
			(SELECT 1 FROM ` + define.TableRequirementHistories + `
			WHERE admin_id=$1 AND workspace_id=$2 AND requirement_id=$3 AND action=$4)`
		if _, err := msql.RawExec(define.Postgres, query, tx, model.AdminId, model.WorkspaceId, model.RequirementId, define.HistoryActionIssueUrge, previous); err != nil {
			logs.Error("sql:%s, err:%s", query, err.Error())
			return err
		}
	}
	query := `INSERT INTO ` + define.TableRequirementHistories + `
		(admin_id,workspace_id,requirement_id,user_id,action,new_value,created_at)
		VALUES($1,$2,$3,0,$4,'recorded',$5)`
	_, err := msql.RawExec(define.Postgres, query, tx, model.AdminId, model.WorkspaceId, model.RequirementId, define.HistoryActionIssueUrge, occurredAt)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
	}
	return err
}

func issueUrgeRecordsSQL() string {
	// 汇总与分页在同一语句的快照中完成，越界空页仍返回正确的总数与历史完整性。
	return `WITH records AS (
		SELECT h.id,h.created_at,CASE WHEN h.new_value='legacy' THEN 1 ELSE 0 END AS legacy
		FROM ` + define.TableRequirementHistories + ` h
		WHERE h.admin_id=$1 AND h.workspace_id=$2 AND h.requirement_id=$3 AND h.action=$4
		UNION ALL
		SELECT 0,ir.last_urged_at,1 FROM ` + define.TableWorkspaceIssueRequirements + ` ir
		WHERE ir.admin_id=$1 AND ir.workspace_id=$2 AND ir.requirement_id=$3 AND ir.last_urged_at>0
		AND NOT EXISTS (SELECT 1 FROM ` + define.TableRequirementHistories + ` h
		WHERE h.admin_id=$1 AND h.workspace_id=$2 AND h.requirement_id=$3 AND h.action=$4)
	) SELECT totals.total,totals.incomplete_history,page.id,page.created_at,page.legacy
	FROM (SELECT COUNT(*) AS total,COALESCE(MAX(legacy),0) AS incomplete_history FROM records) totals
	LEFT JOIN (SELECT id,created_at,legacy FROM records ORDER BY created_at DESC,id DESC LIMIT $5 OFFSET $6) page ON TRUE
	ORDER BY page.created_at DESC,page.id DESC`
}

// ListIssueUrgeRecords 返回工单可见范围内的催单记录，不依赖满意度权限。
func ListIssueUrgeRecords(model ListIssueUrgeRecordsModel) (ListIssueUrgeRecordsModel, error) {
	if model.AdminId <= 0 || model.WorkspaceId <= 0 || model.RequirementId <= 0 || model.UserId <= 0 || model.Page < 1 || model.Size < 1 || model.Size > 100 || model.Page > int(^uint(0)>>1)/model.Size {
		return model, ErrIssueInviteInvalid
	}
	scope, args, err := buildDataViewScopeCondition(model.AdminId, strconv.Itoa(model.WorkspaceId), model.UserId, 4)
	if err != nil {
		return model, err
	}
	query := `SELECT r.id FROM ` + define.TableRequirements + ` r
		JOIN ` + define.TableWorkspaces + ` w ON w.admin_id=r.admin_id AND w.id=r.workspace_id
		JOIN ` + define.TableWorkspaceIssueRequirements + ` ir ON ir.admin_id=r.admin_id AND ir.workspace_id=r.workspace_id AND ir.requirement_id=r.id
		WHERE r.admin_id=$1 AND r.workspace_id=$2 AND r.id=$3 AND r.type='requirement' AND w.workspace_type='issue'`
	if scope != "" {
		query += ` AND (` + scope + `)`
	}
	queryArgs := append([]any{model.AdminId, model.WorkspaceId, model.RequirementId}, args...)
	visible, err := msql.RawValues(define.Postgres, query, nil, queryArgs...)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return model, err
	}
	if len(visible) == 0 {
		return model, ErrRequirementNotFound
	}
	query = issueUrgeRecordsSQL()
	rows, err := msql.RawValues(define.Postgres, query, nil, model.AdminId, model.WorkspaceId, model.RequirementId, define.HistoryActionIssueUrge, model.Size, (model.Page-1)*model.Size)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return model, err
	}
	model.Items = []IssueUrgeRecordModel{}
	for _, row := range rows {
		model.Total, err = strconv.Atoi(row["total"])
		if err != nil {
			return model, err
		}
		incomplete, err := strconv.Atoi(row["incomplete_history"])
		if err != nil {
			return model, err
		}
		model.IncompleteHistory = incomplete == 1
		if row["id"] == "" {
			continue
		}
		id, err := strconv.Atoi(row["id"])
		if err != nil {
			return model, err
		}
		createdAt, err := strconv.ParseInt(row["created_at"], 10, 64)
		if err != nil {
			return model, err
		}
		legacy, err := strconv.Atoi(row["legacy"])
		if err != nil {
			return model, err
		}
		model.Items = append(model.Items, IssueUrgeRecordModel{Id: id, CreatedAt: createdAt, Legacy: legacy == 1})
	}
	return model, nil
}

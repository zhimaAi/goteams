// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"strings"

	"goteams/internal/app/goteams/define"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// MemberLogModel is one enterprise member operation log entry.
type MemberLogModel struct {
	Id           int    `json:"id"`
	Action       string `json:"action"`
	OperatorId   int    `json:"operator_id"`
	OperatorName string `json:"operator_name"`
	TargetUserId int    `json:"target_user_id"`
	TargetName   string `json:"target_name"`
	CreatedAt    int64  `json:"created_at"`
}

// ListMemberLogsModel describes the member operation log list use case.
type ListMemberLogsModel struct {
	AdminId  int
	Keyword  string
	Page     int
	PageSize int
	Items    []MemberLogModel
	Total    int
}

// writeMemberLog records one member operation (create/remove/enable/disable).
// A logging failure never aborts the operation that triggered it: the member change has already
// been committed, so returning an error here would report a false failure to the caller.
func writeMemberLog(adminId, operatorId, targetUserId int, action string) {
	m := msql.Model(define.TableMemberLogs, define.Postgres)
	if _, err := m.Insert(msql.Datas{
		`admin_id`:       adminId,
		`operator_id`:    operatorId,
		`target_user_id`: targetUserId,
		`action`:         action,
		`created_at`:     tool.Time2Int(),
	}); err != nil {
		logs.Error("write member log error: sql=%s, err=%s", m.GetLastSql(), err.Error())
	}
}

// ListMemberLogs returns the tenant's member operation logs, newest first, filtered by the
// target member's name. Both the operator and the target fall back to the removed_users
// archive so entries stay readable after either of them is removed.
func ListMemberLogs(model ListMemberLogsModel) (ListMemberLogsModel, error) {
	if model.Page < 1 {
		model.Page = 1
	}
	if model.PageSize < 1 {
		model.PageSize = 20
	}
	model.Items = make([]MemberLogModel, 0)

	args := []any{model.AdminId}
	where := `l.admin_id = $1`
	keyword := strings.TrimSpace(model.Keyword)
	if keyword != `` {
		args = append(args, `%`+keyword+`%`)
		where += ` AND COALESCE(tu.name, tr.name, '') ILIKE $2`
	}

	// The joins must be repeated in the count query: the keyword filters on the joined name.
	joins := `
		LEFT JOIN users tu ON tu.id = l.target_user_id AND tu.admin_id = l.admin_id
		LEFT JOIN removed_users tr ON tr.user_id = l.target_user_id AND tr.admin_id = l.admin_id`

	countRows, err := msql.RawValues(define.Postgres,
		`SELECT COUNT(*) AS total FROM member_logs l`+joins+` WHERE `+where, nil, args...)
	if err != nil {
		logs.Error("count member logs error: %s", err.Error())
		return ListMemberLogsModel{}, err
	}
	if len(countRows) > 0 {
		model.Total = cast.ToInt(countRows[0][`total`])
	}
	if model.Total == 0 {
		return model, nil
	}

	listArgs := append(append([]any{}, args...), model.PageSize, (model.Page-1)*model.PageSize)
	rows, err := msql.RawValues(define.Postgres, `
		SELECT l.id, l.action, l.operator_id, l.target_user_id, l.created_at,
			COALESCE(tu.name, tr.name, '') AS target_name,
			COALESCE(ou.name, ora.name, '') AS operator_name
		FROM member_logs l`+joins+`
		LEFT JOIN users ou ON ou.id = l.operator_id AND ou.admin_id = l.admin_id
		LEFT JOIN removed_users ora ON ora.user_id = l.operator_id AND ora.admin_id = l.admin_id
		WHERE `+where+`
		ORDER BY l.created_at DESC, l.id DESC
		LIMIT $`+cast.ToString(len(args)+1)+` OFFSET $`+cast.ToString(len(args)+2),
		nil, listArgs...)
	if err != nil {
		logs.Error("list member logs error: %s", err.Error())
		return ListMemberLogsModel{}, err
	}

	for _, row := range rows {
		model.Items = append(model.Items, MemberLogModel{
			Id:           cast.ToInt(row[`id`]),
			Action:       row[`action`],
			OperatorId:   cast.ToInt(row[`operator_id`]),
			OperatorName: row[`operator_name`],
			TargetUserId: cast.ToInt(row[`target_user_id`]),
			TargetName:   row[`target_name`],
			CreatedAt:    cast.ToInt64(row[`created_at`]),
		})
	}
	return model, nil
}

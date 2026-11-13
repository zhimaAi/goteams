package business

import (
	"strconv"
	"strings"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"goteams/internal/app/goteams/define"
)

// dashboardChartRankLimit caps the ranking card at a Top 10 leaderboard.
const dashboardChartRankLimit = 10

// queryDashboardChartAssigneeRank counts work items per assignee and keeps the highest ranking ones.
// Work items without an assignee collapse into bucket id 0, which the frontend renders as "未分配".
func queryDashboardChartAssigneeRank(where []string, args []any) (DashboardChartResultModel, error) {
	result := DashboardChartResultModel{Buckets: []DashboardChartBucketModel{}}
	query := `SELECT COALESCE(au.user_id, 0) AS id, COALESCE(u.name, ru.name, '') AS name, COUNT(DISTINCT r.id) AS count` +
		` FROM ` + define.TableRequirements + ` r` +
		` LEFT JOIN LATERAL unnest(r.assignee_ids) AS au(user_id) ON true` +
		` LEFT JOIN ` + define.TableUsers + ` u ON u.id = au.user_id AND u.admin_id = r.admin_id` +
		` LEFT JOIN ` + define.TableRemovedUsers + ` ru ON ru.user_id = au.user_id AND ru.admin_id = r.admin_id` +
		` WHERE ` + strings.Join(where, ` AND `) +
		` GROUP BY 1, 2 ORDER BY count DESC, id ASC LIMIT ` + strconv.Itoa(dashboardChartRankLimit)
	rows, err := msql.RawValues(define.Postgres, query, nil, args...)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return result, err
	}
	for _, row := range rows {
		count, err := strconv.ParseInt(row["count"], 10, 64)
		if err != nil {
			return result, err
		}
		result.Buckets = append(result.Buckets, DashboardChartBucketModel{Id: cast.ToInt(row["id"]), Name: row["name"], Count: count})
	}
	return result, nil
}

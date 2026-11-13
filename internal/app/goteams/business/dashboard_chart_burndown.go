package business

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"goteams/internal/app/goteams/define"
)

// Burndown series keys stored as bucket types.
const (
	dashboardChartBurndownDone      = `done`
	dashboardChartBurndownRemaining = `remaining`
	dashboardChartBurndownBaseline  = `baseline`
)

// currentDashboardIteration returns the iteration covering today; ties pick the latest start date.
func currentDashboardIteration(adminId, workspaceId int, loc *time.Location) (VersionModel, bool, error) {
	versions, err := ListVersionsByWorkspace(adminId, workspaceId)
	if err != nil {
		return VersionModel{}, false, err
	}
	now := time.Now().In(loc)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	todayEnd := todayStart.AddDate(0, 0, 1).Add(-time.Second)
	found, current := false, VersionModel{}
	for _, version := range versions {
		if version.StartDate <= 0 || version.StartDate > todayEnd.Unix() {
			continue
		}
		// end_date 为 0 表示未设置，按仍在进行中处理。
		if version.EndDate > 0 && version.EndDate < todayStart.Unix() {
			continue
		}
		if found && version.StartDate <= current.StartDate {
			continue
		}
		current, found = version, true
	}
	return current, found, nil
}

// queryDashboardChartBurndown builds the cumulative done, remaining and linear baseline series
// for the iteration covering today, scoped to the work items bound to it through version_id.
func queryDashboardChartBurndown(model DashboardChartQueryModel, where []string, args []any, loc *time.Location) (DashboardChartResultModel, error) {
	result := DashboardChartResultModel{Buckets: []DashboardChartBucketModel{}}
	version, ok, err := currentDashboardIteration(model.AdminId, model.WorkspaceId, loc)
	if err != nil {
		return result, err
	}
	// 没有包含今天的迭代时该卡片不取数，返回空结果由前端展示空态。
	if !ok {
		return result, nil
	}
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	start := dashboardChartLocalDay(time.Unix(version.StartDate, 0).In(loc))
	last := today
	if version.EndDate > 0 {
		last = dashboardChartLocalDay(time.Unix(version.EndDate, 0).In(loc))
	}
	days := []time.Time{}
	for day := start; !day.After(last); day = day.AddDate(0, 0, 1) {
		days = append(days, day)
	}
	args = append(args, version.Id)
	where = append(where, fmt.Sprintf(`r.version_id = $%d`, len(args)))
	query := `SELECT COUNT(*) AS count FROM ` + define.TableRequirements + ` r WHERE ` + strings.Join(where, ` AND `)
	rows, err := msql.RawValues(define.Postgres, query, nil, args...)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return result, err
	}
	if len(rows) != 1 {
		return result, ErrDashboardInvalid
	}
	total, err := strconv.ParseInt(rows[0][`count`], 10, 64)
	if err != nil {
		return result, err
	}
	settings, err := GetWorkspaceSettings(WorkspaceSettingsModel{AdminId: model.AdminId, WsId: strconv.Itoa(model.WorkspaceId)})
	if err != nil {
		return result, err
	}
	// 完成序列沿用完成趋势的口径：当前状态属于最终状态且记录过完成时间。
	doneArgs := append(args, pq.Array(settings.RequirementCompletedStatusIds), pq.Array(settings.DefectCompletedStatusIds), model.Timezone)
	doneWhere := append(where, fmt.Sprintf(`((r.type='requirement' AND r.status_id=ANY($%d::INTEGER[])) OR (r.type='defect' AND r.status_id=ANY($%d::INTEGER[])))`, len(doneArgs)-2, len(doneArgs)-1))
	query = `SELECT to_char(to_timestamp(r.completed_at) AT TIME ZONE $` + strconv.Itoa(len(doneArgs)) + `, 'YYYY-MM-DD') AS day, COUNT(*) AS count FROM ` + define.TableRequirements + ` r WHERE ` + strings.Join(doneWhere, ` AND `) + ` AND r.completed_at > 0 GROUP BY day`
	rows, err = msql.RawValues(define.Postgres, query, nil, doneArgs...)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return result, err
	}
	counts := map[string]int64{}
	for _, row := range rows {
		count, err := strconv.ParseInt(row[`count`], 10, 64)
		if err != nil {
			return result, err
		}
		counts[row[`day`]] = count
	}
	done := int64(0)
	for index, day := range days {
		done += counts[day.Format(`2006-01-02`)]
		remaining := total - done
		if remaining < 0 {
			remaining = 0
		}
		// 基准线从总量按天数线性递减到 0，首尾各占一天。
		baseline := total
		if span := len(days); span > 1 {
			baseline = total * int64(span-1-index) / int64(span-1)
		}
		timestamp := day.Unix()
		result.Buckets = append(result.Buckets,
			DashboardChartBucketModel{Type: dashboardChartBurndownDone, Timestamp: timestamp, Count: done},
			DashboardChartBucketModel{Type: dashboardChartBurndownRemaining, Timestamp: timestamp, Count: remaining},
			DashboardChartBucketModel{Type: dashboardChartBurndownBaseline, Timestamp: timestamp, Count: baseline},
		)
	}
	return result, nil
}

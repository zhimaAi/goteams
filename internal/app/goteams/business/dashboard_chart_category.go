package business

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/lib/pq"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// DashboardCategoryFilterModel 是分类卡片的临时筛选，日期采用左闭右开的 Unix 时间范围。
type DashboardCategoryFilterModel struct {
	From, To   int64
	AssigneeId int
	PriorityId *int
}

// DashboardCategoryStatModel 是按工单分类汇总的统计行。
type DashboardCategoryStatModel struct {
	Id, ParentId int
	Name         string
	dashboardCategoryCounts
}

type dashboardCategoryCounts struct {
	Created, Resolved, Closed, InProgress, PendingAssign, PendingProcess int64
}

func (c *dashboardCategoryCounts) add(other dashboardCategoryCounts) {
	c.Created += other.Created
	c.Resolved += other.Resolved
	c.Closed += other.Closed
	c.InProgress += other.InProgress
	c.PendingAssign += other.PendingAssign
	c.PendingProcess += other.PendingProcess
}

func dashboardCategoryStatusCounts(initial, final bool, name string, count int64) dashboardCategoryCounts {
	workload := dashboardWorkloadStatusCounts(initial, final, name, count)
	result := dashboardCategoryCounts{
		Created: count, InProgress: workload.InProgress,
		PendingAssign: workload.PendingAssign, PendingProcess: workload.PendingProcess,
	}
	switch lib_i18n.UnwrapKey(name) {
	case "workitem_status_closed", "已关闭":
		result.Closed = count
		result.Resolved = count
	case "workitem_status_done", "workitem_status_resolved", "已完成", "已解决":
		result.Resolved = count
	}
	return result
}

func queryDashboardCategoryStats(model DashboardChartQueryModel, config DashboardChartConfig, where []string, args []any, loc *time.Location) ([]DashboardCategoryStatModel, error) {
	start, end := dashboardChartDayRange(loc, 6, 0)
	from, to := start.Unix(), end.Unix()
	if filter := model.CategoryFilter; filter != nil {
		if filter.From != 0 || filter.To != 0 {
			if filter.From <= 0 || filter.To <= filter.From || filter.To-filter.From > 367*24*60*60 {
				return nil, ErrDashboardInvalid
			}
			from, to = filter.From, filter.To
		}
		if filter.AssigneeId < 0 || (filter.PriorityId != nil && *filter.PriorityId < 0) {
			return nil, ErrDashboardInvalid
		}
		if filter.AssigneeId > 0 {
			args = append(args, filter.AssigneeId)
			where = append(where, fmt.Sprintf("$%d = ANY(r.assignee_ids)", len(args)))
		}
		if filter.PriorityId != nil {
			args = append(args, *filter.PriorityId)
			where = append(where, fmt.Sprintf("COALESCE(r.priority, 0) = $%d", len(args)))
		}
	}
	args = append(args, from, to)
	where = append(where, "r.parent_id = 0", fmt.Sprintf("r.created_at >= $%d AND r.created_at < $%d", len(args)-1, len(args)))
	switch config.AssigneeScope {
	case "self":
		args = append(args, model.UserId)
		where = append(where, fmt.Sprintf("$%d = ANY(r.assignee_ids)", len(args)))
	case "members":
		args = append(args, pq.Array(config.AssigneeScopeIds))
		where = append(where, fmt.Sprintf("r.assignee_ids && $%d::INTEGER[]", len(args)))
	case "roles":
		args = append(args, pq.Array(config.AssigneeScopeIds))
		where = append(where, fmt.Sprintf(`EXISTS (SELECT 1 FROM `+define.TableWorkspaceMembers+` wm
			WHERE wm.admin_id = r.admin_id AND wm.workspace_id = r.workspace_id
			AND wm.user_id = ANY(r.assignee_ids) AND wm.role_id = ANY($%d::INTEGER[]))`, len(args)))
	}
	categories, err := listRequirementCategoryItems(model.AdminId, model.WorkspaceId)
	if err != nil {
		return nil, err
	}
	query := `SELECT r.category_id, COALESCE(r.status_id, 0) AS status_id, COALESCE(s.name, '') AS status_name, COUNT(*) AS count
		FROM ` + define.TableRequirements + ` r LEFT JOIN ` + define.TableWorkItemStatuses + ` s
		ON s.admin_id = r.admin_id AND s.workspace_id = r.workspace_id AND s.type_key = r.type AND s.id = r.status_id
		WHERE ` + strings.Join(where, " AND ") + ` GROUP BY r.category_id, r.status_id, s.name`
	rows, err := msql.RawValues(define.Postgres, query, nil, args...)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return nil, err
	}
	settings, err := GetWorkspaceSettings(WorkspaceSettingsModel{AdminId: model.AdminId, WsId: strconv.Itoa(model.WorkspaceId)})
	if err != nil {
		return nil, err
	}
	direct := make(map[int]dashboardCategoryCounts, len(categories))
	for _, row := range rows {
		id, err := strconv.Atoi(row["category_id"])
		if err != nil {
			return nil, fmt.Errorf("parse dashboard category id: %w", err)
		}
		statusId, err := strconv.Atoi(row["status_id"])
		if err != nil {
			return nil, fmt.Errorf("parse dashboard category status ID: %w", err)
		}
		count, err := strconv.ParseInt(row["count"], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse dashboard category count: %w", err)
		}
		current := direct[id]
		current.add(dashboardCategoryStatusCounts(statusId == settings.RequirementInitialStatusId,
			intSliceContains(settings.RequirementFinalStatusIds, statusId), row["status_name"], count))
		direct[id] = current
	}
	return rollupDashboardCategoryStats(categories, direct), nil
}

func rollupDashboardCategoryStats(categories []RequirementCategoryModel, direct map[int]dashboardCategoryCounts) []DashboardCategoryStatModel {
	stats := make([]DashboardCategoryStatModel, len(categories), len(categories)+1)
	positions := make(map[int]int, len(categories))
	for i, category := range categories {
		stats[i] = DashboardCategoryStatModel{Id: category.Id, ParentId: category.ParentId, Name: category.Name}
		positions[category.Id] = i
	}
	var uncategorized dashboardCategoryCounts
	for id, count := range direct {
		if _, found := positions[id]; !found {
			uncategorized.add(count)
			continue
		}
		// 分类树最多三层。遍历祖先时每层仅累加该分类的直接工单。
		var visited [3]int
		for depth := range visited {
			pos, found := positions[id]
			if !found || id == 0 {
				break
			}
			if id == visited[0] || id == visited[1] {
				break
			}
			stats[pos].add(count)
			visited[depth] = id
			id = stats[pos].ParentId
		}
	}
	if uncategorized.Created > 0 {
		stats = append(stats, DashboardCategoryStatModel{dashboardCategoryCounts: uncategorized})
	}
	return stats
}

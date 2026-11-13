package business

import (
	"errors"
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

// DashboardWorkloadFilterModel 是工作量卡片的临时筛选，日期采用左闭右开的 Unix 时间范围。
type DashboardWorkloadFilterModel struct {
	From, To               int64
	AssigneeId             int
	PriorityId, CategoryId *int
}

// DashboardWorkloadRowModel 按成员或日期汇总，Group 为 "total" 时每张工单只计一次。
type DashboardWorkloadRowModel struct {
	Group string
	dashboardCategoryCounts
	Assigned, AssignedResolved, AssignedClosed int64
}

func dashboardWorkloadStatusCounts(initial, final bool, name string, count int64) dashboardCategoryCounts {
	counts := dashboardCategoryCounts{Created: count}
	switch {
	case dashboardClosedStatus(name):
		counts.Closed = count
		counts.Resolved = count
	case final:
		counts.Resolved = count
	case initial:
		counts.PendingAssign = count
	default:
		counts.InProgress = count
		counts.PendingProcess = count
	}
	return counts
}

func dashboardClosedStatus(name string) bool {
	switch lib_i18n.UnwrapKey(name) {
	case "workitem_status_closed", "已关闭":
		return true
	}
	return false
}

func queryDashboardWorkload(model DashboardChartQueryModel, config DashboardChartConfig, where []string, args []any, loc *time.Location) ([]DashboardWorkloadRowModel, error) {
	days := 6
	if config.WorkloadDimension == "member" {
		days = 29
	}
	start, end := dashboardChartDayRange(loc, days, 0)
	from, to := start.Unix(), end.Unix()
	var assignee int
	if filter := model.WorkloadFilter; filter != nil {
		if filter.From != 0 || filter.To != 0 {
			if filter.From <= 0 || filter.To <= filter.From || filter.To-filter.From > 367*24*60*60 {
				return nil, ErrDashboardInvalid
			}
			from, to = filter.From, filter.To
		}
		if filter.AssigneeId < 0 || (filter.PriorityId != nil && *filter.PriorityId < 0) || (filter.CategoryId != nil && *filter.CategoryId < 0) {
			return nil, ErrDashboardInvalid
		}
		assignee = filter.AssigneeId
		if filter.PriorityId != nil {
			args = append(args, *filter.PriorityId)
			where = append(where, fmt.Sprintf("COALESCE(r.priority, 0) = $%d", len(args)))
		}
		if filter.CategoryId != nil {
			ids := []int{0}
			if *filter.CategoryId > 0 {
				var err error
				ids, err = ListRequirementCategorySubtreeIDs(model.AdminId, model.WorkspaceId, *filter.CategoryId)
				if errors.Is(err, ErrRequirementCategoryNotFound) {
					return nil, ErrDashboardInvalid
				}
				if err != nil {
					return nil, err
				}
			}
			args = append(args, pq.Array(ids))
			where = append(where, fmt.Sprintf("r.category_id = ANY($%d::INTEGER[])", len(args)))
		}
	}
	args = append(args, from, to)
	where = append(where, "r.parent_id = 0", fmt.Sprintf("r.created_at >= $%d AND r.created_at < $%d", len(args)-1, len(args)))

	eligible := "r.assignee_ids"
	switch config.AssigneeScope {
	case "self", "members":
		ids := config.AssigneeScopeIds
		if config.AssigneeScope == "self" {
			ids = []int{model.UserId}
		}
		args = append(args, pq.Array(ids))
		eligible = fmt.Sprintf("ARRAY(SELECT unnest(r.assignee_ids) INTERSECT SELECT unnest($%d::INTEGER[]))", len(args))
	case "roles":
		args = append(args, pq.Array(config.AssigneeScopeIds))
		eligible = fmt.Sprintf(`ARRAY(SELECT wm.user_id FROM `+define.TableWorkspaceMembers+` wm
			WHERE wm.admin_id = r.admin_id AND wm.workspace_id = r.workspace_id
			AND wm.user_id = ANY(r.assignee_ids) AND wm.role_id = ANY($%d::INTEGER[]))`, len(args))
	}
	if assignee > 0 {
		args = append(args, assignee)
		where = append(where, fmt.Sprintf("$%d = ANY(eligible.ids)", len(args)))
	}
	if config.AssigneeScope != "" && config.AssigneeScope != "all" {
		where = append(where, "cardinality(eligible.ids) > 0")
	}

	groups := "SELECT DISTINCT unnest(eligible.ids)::text AS key UNION ALL SELECT '0' WHERE COALESCE(cardinality(eligible.ids), 0) = 0"
	if config.WorkloadDimension == "date" {
		args = append(args, loc.String())
		groups = fmt.Sprintf("SELECT to_char(to_timestamp(r.created_at) AT TIME ZONE $%d, 'YYYY-MM-DD') AS key", len(args))
	} else if assignee > 0 {
		groups = "SELECT $" + strconv.Itoa(len(args)) + "::integer::text AS key"
	}
	query := `SELECT g.key, r.status_id, COALESCE(s.name, '') AS status_name,
		COALESCE(cardinality(r.assignee_ids), 0) > 0 AS assigned, COUNT(*) AS count
		FROM ` + define.TableRequirements + ` r LEFT JOIN ` + define.TableWorkItemStatuses + ` s
		ON s.admin_id = r.admin_id AND s.workspace_id = r.workspace_id AND s.type_key = r.type AND s.id = r.status_id
		CROSS JOIN LATERAL (SELECT ` + eligible + ` AS ids) eligible
		CROSS JOIN LATERAL (` + groups + ` UNION ALL SELECT 'total') g
		WHERE ` + strings.Join(where, " AND ") + ` GROUP BY 1, 2, 3, 4`
	rows, err := msql.RawValues(define.Postgres, query, nil, args...)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return nil, err
	}
	settings, err := GetWorkspaceSettings(WorkspaceSettingsModel{AdminId: model.AdminId, WsId: strconv.Itoa(model.WorkspaceId)})
	if err != nil {
		return nil, err
	}
	grouped := map[string]DashboardWorkloadRowModel{"total": {Group: "total"}}
	for _, row := range rows {
		id, err := strconv.Atoi(row["status_id"])
		if err != nil {
			return nil, fmt.Errorf("parse workload status ID: %w", err)
		}
		count, err := strconv.ParseInt(row["count"], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse workload count: %w", err)
		}
		assigned, err := strconv.ParseBool(row["assigned"])
		if err != nil {
			return nil, fmt.Errorf("parse workload assignment: %w", err)
		}
		group := grouped[row["key"]]
		group.Group = row["key"]
		counts := dashboardWorkloadStatusCounts(id == settings.RequirementInitialStatusId,
			intSliceContains(settings.RequirementFinalStatusIds, id), row["status_name"], count)
		group.dashboardCategoryCounts.add(counts)
		if assigned {
			group.Assigned += count
			group.AssignedResolved += counts.Resolved
			group.AssignedClosed += counts.Closed
		}
		grouped[group.Group] = group
	}
	if config.WorkloadDimension == "date" {
		for day := dashboardChartLocalDay(time.Unix(from, 0).In(loc)); day.Unix() < to; day = day.AddDate(0, 0, 1) {
			key := day.Format("2006-01-02")
			if _, ok := grouped[key]; !ok {
				grouped[key] = DashboardWorkloadRowModel{Group: key}
			}
		}
	}
	result := make([]DashboardWorkloadRowModel, 0, len(grouped))
	for _, row := range grouped {
		result = append(result, row)
	}
	return result, nil
}

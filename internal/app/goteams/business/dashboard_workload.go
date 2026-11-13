package business

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/lib/pq"
)

func dashboardWorkloadTemplateCards() ([]DashboardCardModel, error) {
	cards := make([]DashboardCardModel, 0, 5)
	for i, metric := range []string{
		define.DashboardMetricIssuePendingCount,
		define.DashboardMetricIssueCreatedToday,
		define.DashboardMetricIssueCompletedToday,
	} {
		config := DashboardMetricConfig{MetricType: metric, DataScope: define.DashboardMetricDataScopeRequirement}
		raw, err := json.Marshal(config)
		if err != nil {
			return nil, err
		}
		cards = append(cards, DashboardCardModel{CardType: define.DashboardCardTypeMetric,
			ContentMode: define.DashboardContentModeRichText, X: i * 4, W: 4, H: 2, Config: string(raw)})
	}
	for i, dimension := range []string{"member", "date"} {
		config := DashboardChartConfig{ChartType: "workload_stats", DataScope: define.DashboardMetricDataScopeRequirement,
			SortOrder: "default", Palette: "status", RequirementColor: "#1664ff", DefectColor: "#f5222d",
			WorkloadDimension: dimension, AssigneeScope: "all"}
		raw, err := json.Marshal(config)
		if err != nil {
			return nil, err
		}
		cards = append(cards, DashboardCardModel{CardType: define.DashboardCardTypeChart,
			ContentMode: define.DashboardContentModeRichText, Y: 2 + i*7, W: 12, H: 7, Config: string(raw)})
	}
	return cards, nil
}

func dashboardStatusMetricConditions(model DashboardMetricQueryModel, metric string, where []string, args []any) ([]string, []any, error) {
	if metric == define.DashboardMetricIssueCreatedToday || metric == define.DashboardMetricIssueCompletedToday {
		loc, err := time.LoadLocation(model.Timezone)
		if err != nil {
			return nil, nil, ErrDashboardInvalidMetric
		}
		start, end := dashboardChartDayRange(loc, 0, 0)
		args = append(args, start.Unix(), end.Unix())
		column := "r.created_at"
		if metric == define.DashboardMetricIssueCompletedToday {
			column = "r.completed_at"
		}
		where = append(where, fmt.Sprintf("%s >= $%d AND %s < $%d", column, len(args)-1, column, len(args)))
		if metric == define.DashboardMetricIssueCreatedToday {
			return where, args, nil
		}
	}
	args = append(args, pq.Array([]string{lib_i18n.WrapKey("workitem_status_closed"), "已关闭"}))
	closed := fmt.Sprintf(`EXISTS (SELECT 1 FROM `+define.TableWorkItemStatuses+` s
		WHERE s.admin_id = r.admin_id AND s.workspace_id = r.workspace_id AND s.type_key = r.type
		AND s.id = r.status_id AND s.name = ANY($%d::text[]))`, len(args))
	if metric == define.DashboardMetricClosedCount {
		return append(where, closed), args, nil
	}
	settings, err := GetWorkspaceSettings(WorkspaceSettingsModel{AdminId: model.AdminId, WsId: strconv.Itoa(model.WorkspaceId)})
	if err != nil {
		return nil, nil, err
	}
	args = append(args, pq.Array(settings.RequirementFinalStatusIds), pq.Array(settings.DefectFinalStatusIds))
	final := fmt.Sprintf(`((r.type = 'requirement' AND r.status_id = ANY(COALESCE($%d::INTEGER[], '{}')))
		OR (r.type = 'defect' AND r.status_id = ANY(COALESCE($%d::INTEGER[], '{}'))))`, len(args)-1, len(args))
	if metric == define.DashboardMetricIssueCompletedToday {
		return append(where, "("+final+" OR "+closed+")"), args, nil
	}
	where = append(where, "NOT "+closed, "NOT ("+final+")")
	if metric == define.DashboardMetricInProgressCount {
		args = append(args, settings.RequirementInitialStatusId, settings.DefectInitialStatusId)
		where = append(where, fmt.Sprintf("r.status_id <> CASE WHEN r.type = 'defect' THEN $%d::integer ELSE $%d::integer END", len(args), len(args)-1))
	}
	return where, args, nil
}

func queryDashboardSLADueSoon(adminId, workspaceId int, where []string, args []any) (int, error) {
	settings, err := getSLASettingsTx(nil, adminId, workspaceId, false)
	if err != nil || !settings.Enabled {
		return 0, err
	}
	pauses, err := loadSLAPausesTx(nil, adminId, workspaceId)
	if err != nil {
		return 0, err
	}
	rows, err := slaRows(nil, "SELECT requirement_id,"+slaCycleColumns+" FROM "+define.TableSLACycles+
		" WHERE admin_id=$1 AND workspace_id=$2 AND state='running' AND requirement_id IN (SELECT r.id FROM "+
		define.TableRequirements+" r WHERE "+strings.Join(where, " AND ")+")", args...)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	dueSoon := map[int64]bool{}
	for _, row := range rows {
		id, err := slaNumber(row, "requirement_id")
		if err != nil {
			return 0, err
		}
		if dueSoon[id] {
			continue
		}
		cycle, err := slaCycleFromRow(row)
		if err != nil {
			return 0, err
		}
		projection, err := slaCycleProjection(cycle, pauses, now.Unix())
		if err != nil {
			return 0, err
		}
		if deadline := projection.DeadlineAt; deadline != nil && *deadline > now.Unix() && *deadline <= now.Add(24*time.Hour).Unix() {
			dueSoon[id] = true
		}
	}
	return len(dueSoon), nil
}

import { inject } from "vue";
import { useI18n } from "vue-i18n";

const issueKeys = {
  dashboard_scope_requirement: "dashboard_scope_issue",
  dashboard_metric_created_requirement_count:
    "dashboard_metric_created_issue_count",
  dashboard_metric_completed_requirement_count:
    "dashboard_metric_completed_issue_count",
  dashboard_metric_overdue_requirement_count:
    "dashboard_metric_overdue_issue_count",
  dashboard_chart_burndown: "dashboard_chart_issue_burndown",
  dashboard_chart_scope_empty: "dashboard_chart_issue_scope_empty",
  dashboard_chart_scope_hint: "dashboard_chart_issue_scope_hint",
  gantt_view_requirement_filters: "dashboard_issue_filters",
};

// Only fixed dashboard labels change; saved titles and metadata remain data.
export function useDashboardI18n() {
  const { t } = useI18n();
  const workspaceType = inject("dashboardWorkspaceType", null);
  return {
    t: (key, ...args) =>
      t(
        workspaceType?.value === "issue" ? issueKeys[key] || key : key,
        ...args,
      ),
  };
}

import { computed, inject } from "vue";
import { normalizeViewFilterTree } from "../constants/workItemView.js";

export function useDashboardWorkItems(
  workspaceType = inject("dashboardWorkspaceType", null),
) {
  const isIssueWorkspace = computed(() => workspaceType?.value === "issue");
  const workItemTypes = computed(() =>
    isIssueWorkspace.value ? ["requirement"] : ["requirement", "defect"],
  );

  function dataScope(scope) {
    if (!isIssueWorkspace.value) return scope;
    return scope === "all" ? "requirement" : scope === "defect" ? "" : scope;
  }

  function normalizeFilters(value) {
    return {
      work_item_types: (value?.work_item_types || []).filter((type) =>
        workItemTypes.value.includes(type),
      ),
      common: normalizeViewFilterTree(value?.common),
      requirement: normalizeViewFilterTree(value?.requirement),
      defect: normalizeViewFilterTree(
        isIssueWorkspace.value ? null : value?.defect,
      ),
    };
  }

  function isCardVisible(card) {
    if (
      card.card_type === "chart" &&
      [
        "category_stats",
        "workload_stats",
        "sla_trend",
        "sla_dimension",
        "sla_distribution",
        "sla_risk",
      ].includes(card.config?.chart_type)
    )
      return isIssueWorkspace.value;
    if (
      card.card_type === "metric" &&
      (card.config?.metric_type?.startsWith("issue_") ||
        card.config?.metric_type?.startsWith("sla_"))
    )
      return isIssueWorkspace.value;
    return (
      !isIssueWorkspace.value ||
      !["metric", "chart"].includes(card.card_type) ||
      card.config?.data_scope !== "defect"
    );
  }

  return {
    isIssueWorkspace,
    workItemTypes,
    dataScope,
    normalizeFilters,
    isCardVisible,
  };
}

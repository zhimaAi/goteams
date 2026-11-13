import dayjs from "dayjs";

export const chartEntries = [
  {
    key: "status",
    kind: "status_bar",
    scope: "all",
    icon: "bar",
  },
  {
    key: "priority",
    kind: "priority_bar",
    scope: "all",
    icon: "bar",
  },
  {
    key: "completion",
    kind: "completion_line",
    scope: "all",
    icon: "line",
  },
  {
    key: "status_pie",
    kind: "status_pie",
    scope: "all",
    icon: "pie",
  },
  {
    key: "daily_created",
    kind: "daily_created_line",
    scope: "all",
    icon: "step",
  },
  {
    key: "ranking",
    kind: "assignee_rank_bar",
    scope: "all",
    icon: "rank",
  },
  {
    key: "burndown",
    kind: "iteration_burndown",
    scope: "all",
    icon: "burndown",
  },
  {
    key: "workload",
    kind: "workload_stats",
    scope: "requirement",
    icon: "workload",
    issueOnly: true,
  },
  {
    key: "category",
    kind: "category_stats",
    scope: "requirement",
    icon: "category",
    issueOnly: true,
  },
  {
    key: "sla_trend",
    kind: "sla_trend",
    icon: "line",
    issueOnly: true,
    sla: true,
  },
  {
    key: "sla_dimension",
    kind: "sla_dimension",
    icon: "bar",
    issueOnly: true,
    sla: true,
  },
  {
    key: "sla_distribution",
    kind: "sla_distribution",
    icon: "sla_distribution",
    issueOnly: true,
    sla: true,
  },
  {
    key: "sla_risk",
    kind: "sla_risk",
    icon: "sla_risk",
    issueOnly: true,
    sla: true,
  },
];

export function statsChartTitleKey(config) {
  if (config?.chart_type === "sla_dimension")
    return config.sla_dimension === "category"
      ? "dashboard_sla_by_category"
      : "dashboard_sla_by_priority";
  if (
    ["sla_trend", "sla_distribution", "sla_risk"].includes(config?.chart_type)
  )
    return `dashboard_chart_${config.chart_type}`;
  if (config?.chart_type === "category_stats")
    return "dashboard_chart_category";
  if (config?.chart_type === "workload_stats")
    return config.workload_dimension === "date"
      ? "dashboard_workload_title_date"
      : "dashboard_workload_title_member";
  return "";
}

export const statusMetricTypes = ["closed_count", "in_progress_count"];

export const issueMetricTypes = [
  "issue_pending_count",
  "issue_created_today",
  "issue_completed_today",
  "issue_sla_due_soon",
];

export const chartColors = [
  "#1664ff",
  "#52c41a",
  "#fa8c16",
  "#f5222d",
  "#722ed1",
  "#13c2c2",
  "#eb2f96",
  "#faad14",
  "#2f54eb",
  "#a0d911",
  "#ff5722",
];
// Config panels offer the first ten shared swatches.
export const categorySwatches = ["#1664ff", "#1bbf88", "#fa8c16", "#785cff"];
export const chartSwatches = chartColors.slice(0, 10);

export function slaCategoryColor(config, id) {
  return (
    config.sla_category_colors?.[id] || chartSwatches[id % chartSwatches.length]
  );
}
export const chartPalettes = {
  bright: chartColors,
  soft: [
    "#69b1ff",
    "#95de64",
    "#ffc069",
    "#ff7875",
    "#b37feb",
    "#5cdbd3",
    "#ff85c0",
  ],
  cool: [
    "#1677ff",
    "#13c2c2",
    "#2f54eb",
    "#722ed1",
    "#69b1ff",
    "#5cdbd3",
    "#85a5ff",
  ],
};
export function defaultChartConfig(entry = chartEntries[0]) {
  if (entry.kind === "sla_risk" || entry.kind === "sla_distribution") {
    const config = {
      chart_type: entry.kind,
      data_scope: "requirement",
      sla_stage_ids: [],
    };
    if (entry.kind === "sla_distribution") {
      config.sla_distribution_colors = {
        first_response: chartColors[3],
        resolution: chartColors[2],
      };
    }
    return config;
  }
  if (entry.kind === "sla_dimension") {
    return {
      chart_type: entry.kind,
      data_scope: "requirement",
      sla_stage_colors: {},
      sla_dimension: "priority",
      sla_stage_ids: [],
      priority_ids: [],
      sla_category_ids: [],
      sla_category_colors: {},
    };
  }
  if (entry.kind === "sla_trend") {
    return {
      chart_type: entry.kind,
      data_scope: "requirement",
      recent_days: 7,
      sla_stage_ids: [],
      sla_target_rate: 95,
      sla_rate_color: chartColors[0],
      sla_overdue_color: chartColors[3],
    };
  }
  return {
    chart_type: entry.kind,
    ...(entry.kind === "workload_stats"
      ? { workload_dimension: "member" }
      : {}),
    ...(entry.issueOnly
      ? { assignee_scope: "all", assignee_scope_ids: [] }
      : {}),
    data_scope: entry.scope,
    status_ids: [],
    priority_ids: [],
    sort_order: "default",
    custom_order: [],
    requirement_color: chartColors[0],
    defect_color: chartColors[3],
    palette: "status",
    days_from: 29,
    days_to: 0,
    metric_type: "work_item_count",
    top3_color: chartColors[0],
    rest_color: chartColors[2],
    done_color: chartColors[1],
    remaining_color: chartColors[3],
    baseline_color: chartColors[5],
  };
}

// Daily trend charts bucket per day; the rest bucket per metadata name.
export const dailyChartTypes = ["completion_line", "daily_created_line"];
// Group translated metadata by name; IDs remain the persistence identity for custom order.
export function chartGroups(buckets, config, emptyLabel) {
  const groups = new Map();
  const byDay = dailyChartTypes.includes(config.chart_type);
  for (const bucket of buckets) {
    const key = byDay ? String(bucket.timestamp) : bucket.name || emptyLabel;
    if (!groups.has(key))
      groups.set(key, {
        key,
        name: bucket.name || emptyLabel,
        timestamp: bucket.timestamp,
        ids: [],
        requirement: 0,
        defect: 0,
      });
    const group = groups.get(key);
    group.ids.push(bucket.id);
    group[bucket.type] += bucket.count;
  }
  const result = [...groups.values()];
  const rank = (group) => {
    const positions = group.ids
      .map((id) => config.custom_order?.indexOf(id) ?? -1)
      .filter((index) => index >= 0);
    return positions.length ? Math.min(...positions) : Infinity;
  };
  if (config.sort_order === "custom") result.sort((a, b) => rank(a) - rank(b));
  if (["asc", "desc"].includes(config.sort_order))
    result.sort(
      (a, b) =>
        (a.requirement + a.defect - b.requirement - b.defect) *
        (config.sort_order === "asc" ? 1 : -1),
    );
  return result;
}

export function slaStageColor(config, id) {
  const index = config.sla_stage_ids?.indexOf(id) ?? -1;
  const legacyColors =
    config.chart_type === "sla_distribution"
      ? [
          config.sla_distribution_colors?.first_response,
          config.sla_distribution_colors?.resolution,
        ]
      : [config.sla_first_response_color, config.sla_resolution_color];
  const legacyColor = legacyColors[index];
  return (
    config.sla_stage_colors?.[id] ||
    legacyColor ||
    chartSwatches[Math.max(0, index) % chartSwatches.length]
  );
}

export function categoryDateFilter(filters) {
  const [from, to] = categoryDateRange(filters.preset, filters.range);
  return { from: from.unix(), to: to.add(1, "day").unix() };
}

export function categoryDateRange(preset, range) {
  if (preset === "custom")
    return range.map((date) => dayjs(date).startOf("day"));
  const today = dayjs().startOf("day");
  const start = {
    today,
    week: today.subtract(6, "day"),
    month30: today.subtract(29, "day"),
    month: today.startOf("month"),
  };
  return [start[preset], today];
}

export const slaStageMetricType = "sla_stage_achievement_rate";
export const slaStageCriticalMetricType = "sla_stage_critical_count";
export const slaCountMetricTypes = [
  "sla_overdue_count",
  slaStageCriticalMetricType,
];
export const legacySLAStageMetricTypes = [
  "sla_first_response_rate",
  "sla_resolution_rate",
];
export const fixedSLAMetricTypes = [
  "sla_achievement_rate",
  "sla_overdue_count",
  "sla_overdue_rate",
];
export const slaMetricTypes = [
  ...fixedSLAMetricTypes,
  slaStageMetricType,
  slaStageCriticalMetricType,
  ...legacySLAStageMetricTypes,
];

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parse, compileScript } from "@vue/compiler-sfc";
import { computed, reactive } from "vue";
import * as echarts from "echarts";
import {
  slaMultipleOverdueColor,
  slaCountMetricTypes,
  slaStageCriticalMetricType,
  slaCategoryColor,
  slaStageColor,
} from "../src/components/dashboard/dashboardCharts.js";

function setup(name, props) {
  const path = new URL(
    `../src/components/dashboard/${name}.vue`,
    import.meta.url,
  );
  const { descriptor } = parse(readFileSync(path, "utf8"));
  const script = compileScript(descriptor, { id: name })
    .content.replace(/import[\s\S]*?from\s+["'][^"']+["'];?/g, "")
    .replace("export default", "return");
  const t = (key, values) => `${key}${values ? JSON.stringify(values) : ""}`;
  return new Function(
    "computed",
    "useDashboardI18n",
    "useI18n",
    "Empty",
    "DashboardEChart",
    "slaCategoryColor",
    "slaStageColor",
    "slaMultipleOverdueColor",
    "slaCountMetricTypes",
    "slaStageCriticalMetricType",
    script,
  )(
    computed,
    () => ({ t }),
    () => ({ t }),
    {},
    {},
    slaCategoryColor,
    slaStageColor,
    slaMultipleOverdueColor,
    slaCountMetricTypes,
    slaStageCriticalMetricType,
  ).setup(props, { expose() {} });
}
function render(option, height = 320) {
  const chart = echarts.init(null, null, {
    renderer: "svg",
    ssr: true,
    width: 600,
    height,
  });
  try {
    chart.setOption({ ...option, animation: false });
    const svg = chart.renderToSVGString();
    assert.ok(svg.includes("<svg"));
    assert.ok(!svg.includes("NaN"));
  } finally {
    chart.dispose();
  }
}
const trend = setup("DashboardSLATrend", {
  config: { sla_target_rate: 95, sla_overdue_color: slaMultipleOverdueColor },
  result: {
    sla_trend: {
      enabled: true,
      days: [
        { date: "2026-11-18", started: 5, achieved: 3, overdue: 2, rate: 60 },
        { date: "2026-11-19", started: 0, achieved: 0, overdue: 0, rate: null },
      ],
    },
  },
});
assert.equal(trend.hasSamples.value, true);
assert.deepEqual(trend.option.value.series[1].data, [3, 0]);
assert.equal(trend.option.value.series[1].yAxisIndex, 1);
assert.equal(trend.option.value.series[0].data[1], null);
render(trend.option.value);
const dimension = setup("DashboardSLADimension", {
  config: { sla_dimension: "priority" },
  result: {
    sla_dimension: {
      enabled: true,
      groups: [
        {
          id: 1,
          name: "Priority",
          series: [
            { stage_id: 1, stage_name: "Short", rate: 100, eligible: 4 },
            {
              stage_id: 2,
              stage_name: "A much longer stage name",
              rate: null,
              eligible: 0,
            },
          ],
        },
      ],
    },
  },
});
const group = dimension.groups.value[0];
assert.equal(dimension.rateText(group.series[1]), "—");
for (const series of group.series) {
  const option = dimension.trackOption(group, series);
  assert.equal(option.xAxis.max, 100);
  assert.equal(option.series[0].data[0].value, series.rate ?? 0);
  render(option, 8);
}
const metric = setup("DashboardSLAMetric", {
  comparisonUnit: "pt",
  config: { metric_type: "sla_achievement_rate" },
  result: { sla: { change: 2.5 } },
});
assert.equal(metric.changeText.value, "+2.5pt");
const countMetric = setup("DashboardSLAMetric", {
  config: { metric_type: "sla_overdue_count" },
  result: { sla: { change: 2 } },
});
assert.equal(countMetric.changeText.value, "+2dashboard_sla_count_unit");
const distribution = setup("DashboardSLADistribution", {
  config: {},
  byWorkItem: true,
  result: {
    sla_distribution: {
      enabled: true,
      buckets: [
        { key: "stage", stage_id: 1, stage_name: "Stage", count: 2 },
        { key: "multiple", count: 1 },
        { key: "on_time", count: 3 },
      ],
    },
  },
});
assert.equal(distribution.total.value, 6);
assert.equal(distribution.option.value.series[0].labelLine.show, true);
assert.deepEqual(
  distribution.option.value.series[0].data.map((item) => item.value),
  [2, 1, 3],
);
render(distribution.option.value);
assert.equal(
  distribution.option.value.series[0].data[1].itemStyle.color,
  slaMultipleOverdueColor,
);
assert.equal(
  trend.option.value.series[2].itemStyle.color,
  slaMultipleOverdueColor,
);
const riskFilters = reactive({ thresholdMinutes: 60 });
const risk = setup("DashboardSLARisk", {
  filters: riskFilters,
  result: {
    sla_risk: {
      threshold_seconds: 3600,
      stages: [
        { id: 1, name: "First", critical: 2, threshold_seconds: 3600 },
        { id: 2, name: "Second", critical: 1, threshold_seconds: 7200 },
      ],
    },
  },
});
for (const minutes of [60, 30, 120]) {
  riskFilters.thresholdMinutes = minutes;
  for (const item of risk.summary.value.slice(2)) {
    assert.equal(
      item.help,
      `dashboard_sla_risk_threshold${JSON.stringify({ minutes })}`,
    );
  }
}
delete riskFilters.thresholdMinutes;
assert.equal(
  risk.summary.value[2].help,
  'dashboard_sla_risk_threshold{"minutes":60}',
);
assert.equal(
  risk.summary.value[3].help,
  'dashboard_sla_risk_threshold{"minutes":120}',
);
console.log(
  "SLA charts, ECharts rendering and reactive risk threshold labels passed",
);

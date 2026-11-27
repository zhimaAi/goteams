<template>
  <div class="dashboard-sla-metric">
    <strong :style="{ color: config.color || undefined }">
      {{ valueText }}<small v-if="result?.value != null">{{ unit }}</small>
    </strong>
    <div class="dashboard-sla-metric__footer">
      <span v-if="showComparison" class="dashboard-sla-metric__comparison">
        {{ $t("dashboard_sla_comparison", { change: changeText }) }}
      </span>
      <span v-else-if="description" class="dashboard-sla-metric__comparison">
        {{ description }}
      </span>
      <DashboardEChart
        v-if="showComparison && hasTrend"
        class="dashboard-sla-metric__sparkline"
        :option="sparklineOption"
        :label="$t('dashboard_sla_sparkline')"
      />
    </div>
  </div>
</template>
<script setup>
import DashboardEChart from "./DashboardEChart.vue";
import { computed } from "vue";
import { useDashboardI18n } from "@/composables/useDashboardI18n";
import {
  slaCountMetricTypes,
  slaStageCriticalMetricType,
} from "./dashboardCharts";
const props = defineProps({
  config: { type: Object, required: true },
  result: { type: Object, default: null },
  comparisonUnit: { type: String, default: null },
});
const { t } = useDashboardI18n();
const format = (value) =>
  Number(value).toLocaleString(undefined, { maximumFractionDigits: 1 });
const valueText = computed(() =>
  props.result?.value == null ? "—" : format(props.result.value),
);
const isCount = computed(() =>
  slaCountMetricTypes.includes(props.config.metric_type),
);
const unit = computed(
  () =>
    props.config.unit ?? (isCount.value ? t("dashboard_sla_count_unit") : ""),
);
const description = computed(() => {
  if (props.config.metric_type === slaStageCriticalMetricType) {
    if (!props.config.sla_stage_ids?.length)
      return t("dashboard_sla_select_stages");
    const threshold = props.result?.sla?.threshold_seconds;
    return threshold
      ? t("dashboard_sla_risk_threshold", { minutes: threshold / 60 })
      : "";
  }
  if (props.config.metric_type === "sla_overdue_count")
    return t("dashboard_sla_overdue_caption");
  return "";
});
const showComparison = computed(
  () => props.config.sla_recent_days > 0 && props.config.sla_show_comparison,
);
const changeText = computed(() => {
  const change = props.result?.sla?.change;
  if (change == null) return "—";
  const value = Number(change).toLocaleString(undefined, {
    maximumFractionDigits: 1,
    signDisplay: "exceptZero",
  });
  const unit = isCount.value
    ? "dashboard_sla_count_unit"
    : "dashboard_sla_percentage_points";
  return value + (props.comparisonUnit ?? t(unit));
});
const lineColor = computed(() => props.config.sla_line_color || "#1677ff");
const hasTrend = computed(() =>
  props.result?.sla?.trend?.some((point) => point.value != null),
);
const sparklineOption = computed(() => ({
  grid: { left: 3, right: 3, top: 4, bottom: 4 },
  xAxis: {
    type: "category",
    show: false,
    data: (props.result?.sla?.trend || []).map((point) => point.date),
  },
  yAxis: { type: "value", show: false, scale: true },
  series: [
    {
      type: "line",
      data: (props.result?.sla?.trend || []).map((point) => point.value),
      showSymbol: (props.result?.sla?.trend?.length || 0) === 1,
      symbolSize: 4,
      lineStyle: { width: 2 },
      itemStyle: { color: lineColor.value },
    },
  ],
}));
</script>
<style scoped lang="less">
.dashboard-sla-metric {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  width: 100%;
  min-height: 100%;
  strong {
    font-size: 34px;
    line-height: 1.25;
  }
  small {
    margin-left: 4px;
    font-size: 13px;
    font-weight: 400;
  }
}
.dashboard-sla-metric__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 40px;
  margin-top: 12px;
  .dashboard-sla-metric__sparkline {
    width: 140px;
    max-width: 48%;
    height: 38px;
    margin-left: auto;
  }
}
.dashboard-sla-metric__comparison {
  color: var(--ant-color-text-secondary, #8c8c8c);
  font-size: 12px;
}
</style>

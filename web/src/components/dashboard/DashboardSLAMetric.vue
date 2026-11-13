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
      <svg
        v-if="showComparison && trendPoints.length"
        viewBox="0 0 140 38"
        preserveAspectRatio="xMaxYMid meet"
        role="img"
        :aria-label="$t('dashboard_sla_sparkline')"
      >
        <polyline
          v-if="trendPoints.length > 1"
          :points="trendPoints.map(({ x, y }) => `${x},${y}`).join(' ')"
          fill="none"
          :stroke="lineColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
        <circle
          v-else
          :cx="trendPoints[0].x"
          :cy="trendPoints[0].y"
          r="2"
          :fill="lineColor"
        />
      </svg>
    </div>
  </div>
</template>
<script setup>
import { computed } from "vue";
import { useDashboardI18n } from "@/composables/useDashboardI18n";
import {
  slaCountMetricTypes,
  slaStageCriticalMetricType,
} from "./dashboardCharts";
const props = defineProps({
  config: { type: Object, required: true },
  result: { type: Object, default: null },
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
  return value + t(unit);
});
const lineColor = computed(() => props.config.sla_line_color || "#1677ff");
const trendPoints = computed(() => {
  const values = props.result?.sla?.trend || [];
  const known = values
    .filter((point) => point.value != null)
    .map((point) => point.value);
  if (!known.length) return [];
  const low = Math.min(...known);
  const span = Math.max(...known) - low;
  return values.flatMap(({ value }, index) =>
    value == null
      ? []
      : [
          {
            x: 3 + (index * 134) / Math.max(values.length - 1, 1),
            y: span ? 34 - ((value - low) * 29) / span : 19,
          },
        ],
  );
});
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
  svg {
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

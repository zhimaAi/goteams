<template>
  <section class="sla-trend">
    <a-empty
      v-if="!hasSamples"
      :image="Empty.PRESENTED_IMAGE_SIMPLE"
      :description="
        $t(
          trend?.enabled === false
            ? 'dashboard_sla_trend_disabled'
            : 'dashboard_chart_empty',
        )
      "
    />
    <DashboardEChart
      v-else
      :option="option"
      :label="$t('dashboard_chart_sla_trend')"
    />
  </section>
</template>
<script setup>
import { computed } from "vue";
import { Empty } from "ant-design-vue";
import { useDashboardI18n } from "@/composables/useDashboardI18n";
import DashboardEChart from "./DashboardEChart.vue";
const props = defineProps({
  config: { type: Object, required: true },
  result: { type: Object, default: null },
});
const { t } = useDashboardI18n();
const trend = computed(() => props.result?.sla_trend);
const days = computed(() => trend.value?.days || []);
const hasSamples = computed(() => days.value.some((day) => day.started > 0));
const option = computed(() => ({
  aria: { enabled: true },
  tooltip: { trigger: "axis", renderMode: "richText", confine: true },
  legend: { type: "scroll", top: 0 },
  grid: { top: 45, left: 48, right: 48, bottom: 30 },
  xAxis: {
    type: "category",
    data: days.value.map((day) => day.date),
    axisLabel: { formatter: (value) => value.slice(5) },
  },
  yAxis: [
    { type: "value", min: 0, max: 100, axisLabel: { formatter: "{value}%" } },
    { type: "value", min: 0, minInterval: 1, splitLine: { show: false } },
  ],
  series: [
    {
      name: t("dashboard_sla_trend_rate"),
      type: "line",
      data: days.value.map((day) => day.rate),
      itemStyle: { color: props.config.sla_rate_color || "#1664ff" },
      markLine:
        props.config.sla_target_rate == null
          ? undefined
          : {
              symbol: "none",
              label: {
                formatter: t("dashboard_sla_trend_target_value", {
                  value: props.config.sla_target_rate,
                }),
                position: "insideEndTop",
              },
              lineStyle: { color: "#8993a4", type: "dashed" },
              data: [{ yAxis: props.config.sla_target_rate }],
            },
    },
    {
      name: t("issue_statistics_achieved_work_items"),
      type: "bar",
      yAxisIndex: 1,
      data: days.value.map((day) => day.achieved),
      itemStyle: { color: "#36cfc9" },
      barMaxWidth: 24,
    },
    {
      name: t("dashboard_metric_sla_overdue_count"),
      type: "bar",
      yAxisIndex: 1,
      data: days.value.map((day) => day.overdue),
      itemStyle: { color: props.config.sla_overdue_color || "#f5222d" },
      barMaxWidth: 24,
    },
  ],
}));
</script>
<style scoped lang="less">
.sla-trend {
  width: 100%;
  margin-top: 10px;
}
</style>

<template>
  <a-empty
    v-if="!total"
    :image="Empty.PRESENTED_IMAGE_SIMPLE"
    :description="
      $t(
        distribution?.enabled === false
          ? 'dashboard_sla_trend_disabled'
          : 'dashboard_chart_empty',
      )
    "
  />
  <DashboardEChart
    v-else
    class="sla-distribution"
    :option="option"
    :label="$t('dashboard_chart_sla_distribution')"
  />
</template>

<script setup>
import DashboardEChart from "./DashboardEChart.vue";
import { computed } from "vue";
import { Empty } from "ant-design-vue";
import { useI18n } from "vue-i18n";
import { slaStageColor, slaMultipleOverdueColor } from "./dashboardCharts";

const props = defineProps({
  config: { type: Object, required: true },
  result: { type: Object, default: null },
  byWorkItem: Boolean,
});
const { t } = useI18n();
const distribution = computed(() => props.result?.sla_distribution);
// Older responses include aggregate buckets without a stage ID.
const stageBuckets = computed(() =>
  (distribution.value?.buckets || []).filter((bucket) =>
    props.byWorkItem ? bucket.count > 0 : bucket.stage_id > 0,
  ),
);
const total = computed(() =>
  stageBuckets.value.reduce((sum, bucket) => sum + bucket.count, 0),
);
const segments = computed(() => {
  if (!total.value) return [];
  let offset = 0;
  return stageBuckets.value.map((bucket) => {
    const percent = (100 * bucket.count) / total.value;
    const result = {
      ...bucket,
      percent,
      offset,
      color:
        bucket.stage_id > 0
          ? slaStageColor(props.config, bucket.stage_id)
          : {
              multiple: slaMultipleOverdueColor,
              on_time: "#36cfc9",
              unknown: "#bfbfbf",
            }[bucket.key],
    };
    offset += percent;
    return result;
  });
});
const option = computed(() => ({
  aria: { enabled: true },
  tooltip: { trigger: "item", renderMode: "richText", confine: true },
  legend: { type: "scroll", top: 0 },
  title: {
    text: String(total.value),
    subtext: t(
      props.byWorkItem
        ? "issue_statistics_total"
        : "dashboard_sla_distribution_total",
    ),
    left: "center",
    top: "45%",
    textStyle: { fontSize: 24 },
  },
  series: [
    {
      type: "pie",
      radius: ["40%", "60%"],
      center: ["50%", "58%"],
      label: {
        show: true,
        formatter: "{b}\n{c} ({d}%)",
        fontSize: 11,
        alignTo: "edge",
        edgeDistance: 8,
      },
      labelLine: { show: true, length: 12, length2: 8 },
      data: segments.value.map((bucket) => ({
        name: bucketLabel(bucket),
        value: bucket.count,
        itemStyle: { color: bucket.color },
      })),
    },
  ],
}));

function bucketLabel(bucket) {
  return bucket.stage_id > 0
    ? t("dashboard_sla_stage_overdue", { name: bucket.stage_name })
    : t(`issue_statistics_bucket_${bucket.key}`);
}
</script>

<style scoped lang="less">
.sla-distribution {
  height: 320px;
}
</style>

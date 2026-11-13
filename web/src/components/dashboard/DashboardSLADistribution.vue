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
  <div v-else class="sla-distribution">
    <svg
      viewBox="0 0 160 160"
      role="img"
      :aria-label="$t('dashboard_chart_sla_distribution')"
    >
      <circle
        cx="80"
        cy="80"
        r="62"
        fill="none"
        stroke="#eef0f3"
        stroke-width="18"
      />
      <circle
        v-for="bucket in segments"
        :key="bucket.stage_id"
        cx="80"
        cy="80"
        r="62"
        pathLength="100"
        fill="none"
        :stroke="bucket.color"
        stroke-width="18"
        :stroke-dasharray="`${bucket.percent} ${100 - bucket.percent}`"
        :stroke-dashoffset="-bucket.offset"
        transform="rotate(-90 80 80)"
      />
      <text x="80" y="80" text-anchor="middle" class="sla-distribution__total">
        {{ total }}
      </text>
      <text x="80" y="99" text-anchor="middle">
        {{ $t("dashboard_sla_distribution_total") }}
      </text>
    </svg>
    <ul>
      <li v-for="bucket in segments" :key="bucket.stage_id">
        <i :style="{ backgroundColor: bucket.color }" />
        <span>{{
          $t("dashboard_sla_stage_overdue", { name: bucket.stage_name })
        }}</span>
        <strong>{{ bucket.count }}</strong>
        <small>{{ Number(bucket.percent.toFixed(1)) }}%</small>
      </li>
    </ul>
  </div>
</template>

<script setup>
import { computed } from "vue";
import { Empty } from "ant-design-vue";
import { slaStageColor } from "./dashboardCharts";

const props = defineProps({
  config: { type: Object, required: true },
  result: { type: Object, default: null },
});
const distribution = computed(() => props.result?.sla_distribution);
// Older responses include aggregate buckets without a stage ID.
const stageBuckets = computed(() =>
  (distribution.value?.buckets || []).filter((bucket) => bucket.stage_id > 0),
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
      color: slaStageColor(props.config, bucket.stage_id),
    };
    offset += percent;
    return result;
  });
});
</script>

<style scoped lang="less">
.sla-distribution {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px 24px;
  color: #475569;
  padding: 8px 0;

  svg {
    width: 160px;
    max-width: 100%;
    flex-shrink: 0;
  }
  text {
    font-size: 11px;
    fill: currentColor;
  }
  text.sla-distribution__total {
    font-size: 25px;
    font-weight: 700;
  }
  ul {
    display: grid;
    gap: 8px;
    padding: 0;
    margin: 0;
    list-style: none;
    flex: 1;
  }
  li {
    display: grid;
    grid-template-columns: 8px minmax(90px, 1fr) auto auto;
    align-items: center;
    gap: 8px;
    font-size: 12px;
  }
  i {
    width: 8px;
    height: 8px;
    border-radius: 50%;
  }
}
</style>

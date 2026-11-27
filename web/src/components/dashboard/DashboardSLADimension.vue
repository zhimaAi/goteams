<template>
  <section class="sla-dimension">
    <a-empty
      v-if="!dimension?.enabled || !groups.length"
      :image="Empty.PRESENTED_IMAGE_SIMPLE"
      :description="
        $t(
          dimension?.enabled === false
            ? 'dashboard_sla_trend_disabled'
            : 'dashboard_chart_empty',
        )
      "
    />
    <template v-else>
      <p v-if="!hasStageSeries" class="sla-dimension__total">
        {{ $t("dashboard_sla_dimension_total", { count: total }) }}
      </p>
      <dl class="sla-dimension__groups">
        <div
          v-for="group in groups"
          :key="group.id"
          class="sla-dimension__group"
        >
          <dt :title="groupName(group)">{{ groupName(group) }}</dt>
          <dd>
            <div
              v-for="series in group.series"
              :key="series.stage_id || series.metric_type"
              class="sla-dimension__series"
            >
              <span
                v-if="hasStageSeries"
                class="sla-dimension__label"
                :title="seriesName(series)"
                >{{ seriesName(series) }}</span
              >
              <DashboardEChart
                class="sla-dimension__track"
                :option="trackOption(group, series)"
                :label="`${groupName(group)} · ${seriesName(series)} · ${rateText(series)}`"
                :title="$t('dashboard_sla_trend_sample', series)"
              />
              <span
                :title="$t('dashboard_sla_trend_sample', series)"
                :aria-label="
                  series.rate == null
                    ? $t('dashboard_sla_trend_no_rate')
                    : undefined
                "
                >{{ rateText(series) }} ·
                {{
                  $t("dashboard_sla_dimension_count", {
                    count: series.eligible,
                  })
                }}</span
              >
            </div>
          </dd>
        </div>
      </dl>
    </template>
  </section>
</template>

<script setup>
import DashboardEChart from "./DashboardEChart.vue";
import { computed } from "vue";
import { Empty } from "ant-design-vue";
import { useDashboardI18n } from "@/composables/useDashboardI18n";
import { slaCategoryColor, slaStageColor } from "./dashboardCharts";

const props = defineProps({
  config: { type: Object, required: true },
  result: { type: Object, default: null },
  stageSeries: Boolean,
});
const { t } = useDashboardI18n();
const byPriority = computed(() => props.config.sla_dimension === "priority");
const hasStageSeries = computed(() => byPriority.value || props.stageSeries);
const dimension = computed(() => props.result?.sla_dimension);
const groups = computed(() => dimension.value?.groups || []);
const total = computed(() =>
  groups.value.reduce(
    (count, group) => count + (group.series[0]?.eligible ?? 0),
    0,
  ),
);

function trackOption(group, series) {
  return {
    animation: false,
    grid: { left: 0, right: 0, top: 0, bottom: 0 },
    xAxis: { type: "value", min: 0, max: 100, show: false },
    yAxis: { type: "category", data: [""], show: false },
    series: [
      {
        type: "bar",
        barWidth: 8,
        showBackground: true,
        backgroundStyle: { color: "#eef0f3", borderRadius: 5 },
        data: [
          {
            value: series.rate ?? 0,
            itemStyle: { color: seriesColor(group, series), borderRadius: 5 },
          },
        ],
      },
    ],
  };
}

function groupName(group) {
  return (
    group.name ||
    t(byPriority.value ? "dashboard_chart_unset" : "sla_uncategorized")
  );
}

function seriesName(series) {
  return t("dashboard_sla_named_stage_rate", {
    name: series.stage_name || t("dashboard_sla_archived_stage"),
  });
}

function seriesColor(group, series) {
  return hasStageSeries.value
    ? slaStageColor(props.config, series.stage_id)
    : slaCategoryColor(props.config, group.id);
}

function rateText(series) {
  return series.rate == null ? "—" : `${Number(series.rate.toFixed(2))}%`;
}
</script>

<style scoped lang="less">
.sla-dimension {
  width: 100%;
  color: #485568;
  font-size: 13px;

  &__total {
    margin: 0 0 8px;
    color: #64748b;
    text-align: right;
  }

  &__groups {
    display: grid;
    gap: 12px;
    margin: 0;
  }

  &__group {
    display: grid;
    grid-template-columns: minmax(44px, 22%) minmax(0, 1fr);
    align-items: center;
    gap: 8px;

    dt {
      overflow-wrap: anywhere;
    }

    dd {
      display: grid;
      gap: 6px;
      margin: 0;
    }
  }

  &__series {
    display: flex;
    align-items: center;
    gap: 8px;
    white-space: nowrap;
  }

  &__label {
    flex: 0 0 110px;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  &__series > span:last-child {
    flex: 0 0 100px;
    text-align: right;
  }

  &__track {
    width: 0;
    flex: 1;
    min-width: 12px;
    height: 8px;
    border-radius: 5px;
    background-color: #eef0f3;

    div {
      height: 100%;
      border-radius: inherit;
    }
  }
}
</style>

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
      <p v-if="!byPriority" class="sla-dimension__total">
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
              <span v-if="byPriority">{{ seriesName(series) }}</span>
              <div class="sla-dimension__track" aria-hidden="true">
                <div
                  v-if="series.rate != null"
                  :style="{
                    width: `${series.rate}%`,
                    backgroundColor: seriesColor(group, series),
                  }"
                />
              </div>
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
import { computed } from "vue";
import { Empty } from "ant-design-vue";
import { useDashboardI18n } from "@/composables/useDashboardI18n";
import { slaCategoryColor, slaStageColor } from "./dashboardCharts";

const props = defineProps({
  config: { type: Object, required: true },
  result: { type: Object, default: null },
});
const { t } = useDashboardI18n();
const byPriority = computed(() => props.config.sla_dimension === "priority");
const dimension = computed(() => props.result?.sla_dimension);
const groups = computed(() => dimension.value?.groups || []);
const total = computed(() =>
  groups.value.reduce(
    (count, group) => count + (group.series[0]?.eligible ?? 0),
    0,
  ),
);

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
  return byPriority.value
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

  &__track {
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

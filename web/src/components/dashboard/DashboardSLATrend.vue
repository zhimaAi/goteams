<template>
  <section class="sla-trend">
    <a-empty
      v-if="!hasSamples"
      :image="Empty.PRESENTED_IMAGE_SIMPLE"
      :description="
        trend?.enabled === false
          ? $t('dashboard_sla_trend_disabled')
          : $t('dashboard_chart_empty')
      "
    />
    <template v-else>
      <ul class="sla-trend__legend">
        <li>
          <i
            class="sla-trend__rate"
            :style="{ color: config.sla_rate_color }"
          />
          {{ $t("dashboard_sla_trend_rate") }}
        </li>
        <li>
          <i
            class="sla-trend__bar"
            :style="{ background: config.sla_overdue_color }"
          />
          {{ $t("dashboard_metric_sla_overdue_count") }}
        </li>
        <li v-if="config.sla_target_rate != null">
          <i class="sla-trend__target" />
          {{
            $t("dashboard_sla_trend_target_value", {
              value: config.sla_target_rate,
            })
          }}
        </li>
      </ul>
      <svg
        viewBox="0 0 560 230"
        role="img"
        :aria-label="$t('dashboard_chart_sla_trend')"
      >
        <g v-for="percent in [0, 25, 50, 75, 100]" :key="percent">
          <line
            class="sla-trend__grid-line"
            :class="{ 'sla-trend__grid-line--baseline': percent === 0 }"
            :x1="left"
            :x2="right"
            :y1="rateY(percent)"
            :y2="rateY(percent)"
          />
          <text :x="left - 5" :y="rateY(percent)" text-anchor="end">
            {{ percent }}%
          </text>
          <text
            v-if="percent === 0 || percent === 50 || percent === 100"
            :x="right + 5"
            :y="rateY(percent)"
          >
            {{ (percent * maxOverdue) / 100 }}
          </text>
        </g>
        <rect
          v-for="(day, index) in days"
          :key="day.date"
          :x="x(index) - barWidth / 2"
          :y="rateY((100 * day.overdue) / maxOverdue)"
          :width="barWidth"
          :height="(plotHeight * day.overdue) / maxOverdue"
          :fill="config.sla_overdue_color"
          fill-opacity="0.28"
          rx="2.5"
        />
        <line
          v-if="config.sla_target_rate != null"
          class="sla-trend__target-line"
          :x1="left"
          :x2="right"
          :y1="rateY(config.sla_target_rate)"
          :y2="rateY(config.sla_target_rate)"
        />
        <path
          class="sla-trend__rate-line"
          :d="ratePath"
          fill="none"
          :stroke="config.sla_rate_color"
        />
        <template v-for="(day, index) in days" :key="day.date">
          <circle
            v-if="day.rate != null"
            class="sla-trend__point"
            :cx="x(index)"
            :cy="rateY(day.rate)"
            :r="days.length <= 45 ? 2.25 : 1"
            fill="white"
            :stroke="config.sla_rate_color"
          />
          <text
            v-if="labelIndexes.has(index)"
            :x="x(index)"
            y="214"
            text-anchor="middle"
          >
            {{ day.date.slice(5) }}
          </text>
          <rect
            :x="left + index * slotWidth"
            :y="top"
            :width="slotWidth"
            :height="plotHeight"
            fill="transparent"
          >
            <title>{{ tooltip(day) }}</title>
          </rect>
        </template>
      </svg>
    </template>
  </section>
</template>

<script setup>
import { computed } from "vue";
import { Empty } from "ant-design-vue";
import { useDashboardI18n } from "@/composables/useDashboardI18n";

const props = defineProps({
  config: { type: Object, required: true },
  result: { type: Object, default: null },
});

const { t } = useDashboardI18n();
const left = 42;
const right = 515;
const top = 16;
const base = 188;
const plotHeight = base - top;
const trend = computed(() => props.result?.sla_trend);
const days = computed(() => trend.value?.days || []);
const hasSamples = computed(() => days.value.some((day) => day.started > 0));
const slotWidth = computed(
  () => (right - left) / Math.max(1, days.value.length),
);
const barWidth = computed(() => Math.min(24, slotWidth.value * 0.55));
const maxOverdue = computed(() => {
  const peak = Math.max(1, ...days.value.map((day) => day.overdue));
  return Math.ceil(peak / 2) * 2;
});
const labelIndexes = computed(() => {
  const last = days.value.length - 1;
  const segments = Math.min(6, last);
  return new Set(
    segments <= 0
      ? [0]
      : Array.from({ length: segments + 1 }, (_, index) =>
          Math.round((index * last) / segments),
        ),
  );
});
const ratePath = computed(() =>
  days.value
    .map((day, index) => {
      if (day.rate == null) return "";
      const command = days.value[index - 1]?.rate == null ? "M" : "L";
      return `${command}${x(index)},${rateY(day.rate)}`;
    })
    .join(" "),
);

function x(index) {
  return left + (index + 0.5) * slotWidth.value;
}

function rateY(percent) {
  return base - (plotHeight * percent) / 100;
}

function tooltip(day) {
  const rate =
    day.rate == null
      ? t("dashboard_sla_trend_no_rate")
      : `${t("dashboard_sla_trend_rate")} ${day.rate.toFixed(2)}%`;
  const sample = t("dashboard_sla_trend_sample", day);
  const overdue = `${t("dashboard_metric_sla_overdue_count")} ${day.overdue}`;
  return `${day.date} · ${rate} · ${sample} · ${overdue}`;
}
</script>

<style scoped lang="less">
.sla-trend {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
  margin-top: 10px;
  color: #576275;

  svg {
    width: 100%;
    max-height: 320px;
    min-height: 160px;
    text {
      dominant-baseline: middle;
      fill: currentColor;
      font-size: 10px;
      font-variant-numeric: tabular-nums;
    }
  }

  &__legend {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 14px;
    margin: 0;
    padding: 0;
    list-style: none;
    font-size: 11px;

    li {
      display: inline-flex;
      align-items: center;
      gap: 5px;
    }
  }

  &__rate,
  &__target {
    width: 14px;
    border-top: 1.5px solid currentColor;
  }

  &__bar {
    width: 8px;
    height: 8px;
    border-radius: 2px;
    opacity: 0.4;
  }

  &__target {
    border-top: 1px dashed #8993a4;
  }

  &__grid-line,
  &__target-line,
  &__rate-line,
  &__point {
    vector-effect: non-scaling-stroke;
  }

  &__grid-line {
    stroke: #edf0f4;
  }

  &__grid-line--baseline {
    stroke: #dce2ea;
  }

  &__target-line {
    stroke: #8993a4;
    stroke-dasharray: 4 3;
  }

  &__rate-line {
    stroke-width: 1.5;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  &__point {
    stroke-width: 1.25;
  }
}
</style>

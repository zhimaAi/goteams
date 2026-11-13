<template>
  <div ref="container" class="dashboard-chart">
    <a-empty
      v-if="!hasScope"
      :description="t('dashboard_chart_scope_empty')"
      :image="Empty.PRESENTED_IMAGE_SIMPLE"
    />
    <template v-else-if="total > 0">
      <div v-if="!isPie && !isRank" class="dashboard-chart__legend">
        <span v-for="item in legendItems" :key="item.key"
          ><i :style="{ background: item.color }" />{{ item.label }}</span
        >
      </div>
      <div v-if="isRank" class="dashboard-chart__ranks">
        <div
          v-for="(item, index) in ranks"
          :key="item.id"
          class="dashboard-chart__rank"
        >
          <span
            class="dashboard-chart__badge"
            :style="
              index < 3
                ? { background: config.top3_color, color: '#fff' }
                : { background: '#f0f2f5', color: '#8c8c8c' }
            "
            >{{ index + 1 }}</span
          >
          <span class="dashboard-chart__rank-name" :title="item.name">{{
            item.name
          }}</span>
          <span class="dashboard-chart__track"
            ><i
              :style="{ width: rankWidth(item), background: rankColor(index) }"
          /></span>
          <span class="dashboard-chart__rank-value">{{ item.count }}</span>
        </div>
      </div>
      <svg
        v-else
        :viewBox="`0 0 ${width} ${height}`"
        role="img"
        :aria-label="$t('dashboard_chart_statistics')"
      >
        <template v-if="isPie">
          <g
            v-for="slice in slices"
            :key="slice.key"
            tabindex="0"
            :aria-label="slice.label"
          >
            <path
              :d="slice.path"
              :fill="slice.color"
              stroke="white"
              stroke-width="1"
            >
              <title>{{ slice.label }}</title>
            </path>
            <text
              v-if="slice.fraction >= 0.04"
              :x="slice.x"
              :y="slice.y"
              text-anchor="middle"
              class="dashboard-chart__pie-label"
            >
              {{ percent(slice.fraction) }}
            </text>
          </g>
        </template>
        <template v-else>
          <g v-for="tick in ticks" :key="tick.value">
            <line
              :x1="left"
              :x2="width - right"
              :y1="tick.y"
              :y2="tick.y"
              stroke="#e5e7eb"
            />
            <text :x="left - 10" :y="tick.y + 4" text-anchor="end">
              {{ tick.value }}
            </text>
          </g>
          <g v-for="(group, index) in groups" :key="group.key">
            <text
              v-if="index % labelStep === 0 || index === groups.length - 1"
              :transform="`translate(${x(index)},${base + 18}) rotate(${groups.length > 6 ? -35 : 0})`"
              :text-anchor="groups.length > 6 ? 'end' : 'middle'"
            >
              {{ label(group) }}
            </text>
          </g>
          <template v-if="isBurndown">
            <template v-for="series in burndownSeries" :key="series.name">
              <polyline
                :points="
                  series.points.map((p, i) => `${x(i)},${y(p.count)}`).join(' ')
                "
                fill="none"
                :stroke="series.color"
                stroke-width="2.5"
              />
              <g
                v-for="(point, index) in series.points"
                :key="index"
                tabindex="0"
                :aria-label="burndownPointLabel(series, point)"
              >
                <circle
                  :cx="x(index)"
                  :cy="y(point.count)"
                  r="4"
                  :fill="series.color"
                >
                  <title>{{ burndownPointLabel(series, point) }}</title>
                </circle>
              </g>
            </template>
            <g v-for="series in burndownSeries" :key="`${series.name}-labels`">
              <template v-for="(point, index) in series.points" :key="index">
                <text
                  v-if="series.points.length <= 35"
                  :x="x(index)"
                  :y="y(point.count) - 7"
                  text-anchor="middle"
                  class="dashboard-chart__value dashboard-chart__line-value"
                >
                  {{ point.count }}
                </text>
              </template>
            </g>
          </template>
          <template v-else>
            <template v-for="(type, typeIndex) in types" :key="type">
              <polyline
                v-if="isLine"
                :points="
                  groups.map((g, i) => `${x(i)},${y(g[type])}`).join(' ')
                "
                fill="none"
                :stroke="color(type)"
                stroke-width="2.5"
              />
              <g
                v-for="(group, index) in groups"
                :key="group.key"
                tabindex="0"
                :aria-label="pointLabel(group, type)"
              >
                <circle
                  v-if="isLine"
                  :cx="x(index)"
                  :cy="y(group[type])"
                  r="4"
                  :fill="color(type)"
                >
                  <title>{{ pointLabel(group, type) }}</title>
                </circle>
                <rect
                  v-else
                  :x="x(index) - barGroupWidth / 2 + typeIndex * barWidth"
                  :y="y(group[type])"
                  :height="base - y(group[type])"
                  :width="Math.max(1, barWidth - 4)"
                  rx="2"
                  :fill="color(type)"
                >
                  <title>{{ pointLabel(group, type) }}</title>
                </rect>
              </g>
            </template>
            <g v-for="(type, typeIndex) in types" :key="`${type}-labels`">
              <template v-for="(group, index) in groups" :key="group.key">
                <text
                  v-if="group[type] > 0 && groups.length <= 35"
                  :x="
                    isLine
                      ? x(index)
                      : x(index) -
                        barGroupWidth / 2 +
                        typeIndex * barWidth +
                        (barWidth - 4) / 2
                  "
                  :y="y(group[type]) - 7"
                  text-anchor="middle"
                  class="dashboard-chart__value"
                  :class="{ 'dashboard-chart__line-value': isLine }"
                >
                  {{ group[type] }}
                </text>
                <text
                  v-if="
                    config.chart_type === 'priority_bar' &&
                    group[type] > 0 &&
                    groups.length <= 12
                  "
                  :x="
                    x(index) -
                    barGroupWidth / 2 +
                    typeIndex * barWidth +
                    (barWidth - 4) / 2
                  "
                  :y="y(group[type]) + 17"
                  text-anchor="middle"
                  class="dashboard-chart__percentage"
                >
                  {{ percent(group[type] / totals[type]) }}
                </text>
              </template>
            </g>
          </template>
        </template>
      </svg>
      <div v-if="isPie" class="dashboard-chart__pie-legend">
        <span v-for="slice in slices" :key="slice.key"
          ><i :style="{ background: slice.color }" />{{ slice.label }}</span
        >
      </div>
    </template>
    <a-empty
      v-else
      :description="$t('dashboard_chart_empty')"
      :image="Empty.PRESENTED_IMAGE_SIMPLE"
    />
    <p v-if="result?.undated_count" class="dashboard-chart__note">
      {{ $t("dashboard_chart_undated", { count: result.undated_count }) }}
    </p>
  </div>
</template>
<script setup>
import { computed, ref, onMounted, onBeforeUnmount } from "vue";
import { useDashboardWorkItems } from "@/composables/useDashboardWorkItems";
import { useDashboardI18n } from "@/composables/useDashboardI18n";
import { Empty } from "ant-design-vue";
import { formatLocalDate } from "@/utils/date";
import { chartGroups, chartPalettes, dailyChartTypes } from "./dashboardCharts";
const props = defineProps({
  config: { type: Object, required: true },
  result: { type: Object, default: null },
});
const { t } = useDashboardI18n();
const container = ref(null),
  width = ref(500),
  height = ref(280);
let observer;
onMounted(() => {
  observer = new ResizeObserver(([entry]) => {
    width.value = Math.max(280, entry.contentRect.width);
    height.value = Math.max(
      180,
      entry.contentRect.height -
        (props.config.chart_type === "status_pie" ? 110 : 64),
    );
  });
  observer.observe(container.value);
});
onBeforeUnmount(() => observer?.disconnect());
const left = 42,
  right = 22;
const base = computed(() => height.value - 55);
const { dataScope } = useDashboardWorkItems();
const chartScopes = ["requirement", "defect"];
// 数据范围可以全部取消，此时不渲染图表并展示空态。
const types = computed(() => {
  const scope = dataScope(props.config.data_scope);
  if (scope === "all") return [...chartScopes];
  return chartScopes.includes(scope) ? [scope] : [];
});
const hasScope = computed(() => types.value.length > 0);
const isPie = computed(() => props.config.chart_type === "status_pie");
const isLine = computed(() =>
  dailyChartTypes.includes(props.config.chart_type),
);
const isRank = computed(() => props.config.chart_type === "assignee_rank_bar");
const isBurndown = computed(
  () => props.config.chart_type === "iteration_burndown",
);
const buckets = computed(() =>
  (props.result?.buckets || []).filter(
    (bucket) =>
      isBurndown.value || isRank.value || types.value.includes(bucket.type),
  ),
);
// 燃烧图按 bucket.type 拆成三条序列，不参与需求/缺陷的双色分组。
const burndownSeries = computed(() =>
  ["done", "remaining", "baseline"].map((name) => ({
    name,
    color: props.config[`${name}_color`],
    points: buckets.value.filter((b) => b.type === name),
  })),
);
const groups = computed(() => {
  if (isBurndown.value)
    return (burndownSeries.value[0]?.points || []).map((point) => ({
      key: String(point.timestamp),
      timestamp: point.timestamp,
    }));
  return chartGroups(buckets.value, props.config, t("dashboard_chart_unset"));
});
const ranks = computed(() =>
  buckets.value.map((bucket) => ({
    id: bucket.id,
    name: bucket.name || t("dashboard_chart_unassigned"),
    count: bucket.count,
  })),
);
const rankMaximum = computed(() =>
  Math.max(1, ...ranks.value.map((rank) => rank.count)),
);
const rankColor = (index) =>
  index < 3 ? props.config.top3_color : props.config.rest_color;
const rankWidth = (rank) =>
  `${Math.max(4, (rank.count / rankMaximum.value) * 100)}%`;
const totals = computed(() => ({
  requirement: buckets.value
    .filter((b) => b.type === "requirement")
    .reduce((s, b) => s + b.count, 0),
  defect: buckets.value
    .filter((b) => b.type === "defect")
    .reduce((s, b) => s + b.count, 0),
}));
const total = computed(() =>
  isRank.value || isBurndown.value
    ? buckets.value.reduce((sum, bucket) => sum + bucket.count, 0)
    : totals.value.requirement + totals.value.defect,
);
const maximum = computed(() =>
  isBurndown.value
    ? Math.max(
        1,
        ...burndownSeries.value.flatMap((series) =>
          series.points.map((point) => point.count),
        ),
      )
    : Math.max(
        1,
        ...groups.value.flatMap((g) => types.value.map((type) => g[type])),
      ),
);
const legendItems = computed(() =>
  isBurndown.value
    ? burndownSeries.value.map((series) => ({
        key: series.name,
        color: series.color,
        label: t(`dashboard_chart_burndown_${series.name}`),
      }))
    : types.value.map((type) => ({
        key: type,
        color: color(type),
        label: t(`dashboard_scope_${type}`),
      })),
);
const tickSize = computed(() => Math.max(1, Math.ceil(maximum.value / 5)));
const ceiling = computed(() => tickSize.value * 5);
const y = (value) => base.value - ((base.value - 30) * value) / ceiling.value;
const ticks = computed(() =>
  Array.from({ length: 6 }, (_, i) => ({
    value: i * tickSize.value,
    y: y(i * tickSize.value),
  })),
);
const x = (index) =>
  left +
  ((width.value - left - right) * (index + 0.5)) /
    Math.max(1, groups.value.length);
const barGroupWidth = computed(() =>
  Math.min(
    90,
    ((width.value - left - right) / Math.max(1, groups.value.length)) * 0.75,
  ),
);
const barWidth = computed(() => barGroupWidth.value / types.value.length);
const labelStep = computed(() =>
  Math.max(
    1,
    Math.ceil(groups.value.length / Math.max(3, Math.floor(width.value / 65))),
  ),
);
const color = (type) => props.config[`${type}_color`];
const label = (group) =>
  isLine.value || isBurndown.value
    ? formatLocalDate(group.timestamp)
    : group.name;
const burndownPointLabel = (series, point) =>
  `${formatLocalDate(point.timestamp)} · ${t(`dashboard_chart_burndown_${series.name}`)}: ${point.count}`;
const percent = (fraction) => `${(fraction * 100).toFixed(2)}%`;
const pointLabel = (group, type) =>
  `${label(group)} · ${t(`dashboard_scope_${type}`)}: ${group[type]}${props.config.chart_type === "priority_bar" ? ` (${percent(totals.value[type] ? group[type] / totals.value[type] : 0)})` : ""}`;
const slices = computed(() => {
  let angle = -Math.PI / 2;
  const cx = width.value / 2,
    cy = height.value / 2,
    r = Math.max(30, Math.min(width.value * 0.32, height.value * 0.4));
  return buckets.value
    .filter((b) => b.count > 0)
    .map((b, index) => {
      const fraction = b.count / total.value,
        end = angle + fraction * Math.PI * 2,
        mid = (angle + end) / 2;
      const point = (a) => `${cx + r * Math.cos(a)} ${cy + r * Math.sin(a)}`;
      const path =
        fraction === 1
          ? `M ${cx - r} ${cy} a ${r} ${r} 0 1 0 ${r * 2} 0 a ${r} ${r} 0 1 0 ${-r * 2} 0`
          : `M ${cx} ${cy} L ${point(angle)} A ${r} ${r} 0 ${fraction > 0.5 ? 1 : 0} 1 ${point(end)} Z`;
      angle = end;
      const palette = chartPalettes[props.config.palette];
      return {
        key: `${b.type}-${b.id}`,
        fraction,
        path,
        color: palette ? palette[index % palette.length] : b.color || "#8c8c8c",
        x: cx + r * 0.65 * Math.cos(mid),
        y: cy + r * 0.65 * Math.sin(mid),
        label: `${types.value.length > 1 ? t(`dashboard_scope_${b.type}`) + " · " : ""}${b.name || t("dashboard_chart_unset")}: ${b.count} (${percent(fraction)})`,
      };
    });
});
</script>
<style scoped lang="less">
.dashboard-chart {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  min-height: 0;
  overflow: auto;
  align-items: stretch;
  svg {
    width: 100%;
    min-height: 160px;
    flex: 1;
    overflow: visible;
  }
  text {
    font-size: 11px;
    fill: #68758b;
  }
  &__legend {
    display: flex;
    justify-content: center;
    gap: 18px;
    font-size: 12px;
    margin-bottom: 8px;
    span {
      display: flex;
      align-items: center;
      gap: 6px;
    }
  }
  i {
    display: inline-block;
    width: 9px;
    height: 9px;
    border-radius: 2px;
    flex-shrink: 0;
  }
  &__ranks {
    display: flex;
    flex-direction: column;
    justify-content: center;
    flex: 1;
    gap: 10px;
    padding: 4px 2px;
    overflow: auto;
  }
  &__rank {
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 13px;
    color: #15284b;
  }
  &__badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    flex-shrink: 0;
    border-radius: 50%;
    font-size: 12px;
    font-weight: 600;
  }
  &__rank-name {
    width: 84px;
    flex-shrink: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
  &__track {
    flex: 1;
    min-width: 0;
    height: 8px;
    border-radius: 4px;
    background: #f0f2f5;
    i {
      display: block;
      width: 0;
      height: 100%;
      border-radius: 4px;
    }
  }
  &__rank-value {
    width: 36px;
    flex-shrink: 0;
    text-align: right;
    font-weight: 600;
  }
  &__pie-legend {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 6px 14px;
    font-size: 11px;
    max-height: 90px;
    overflow: auto;
    span {
      display: flex;
      align-items: center;
      gap: 5px;
    }
  }
  &__note {
    font-size: 12px;
    color: #8c8c8c;
    margin: 6px 0;
  }
  text.dashboard-chart__value {
    fill: #526177;
    font-weight: 600;
  }
  text.dashboard-chart__line-value {
    paint-order: stroke;
    stroke: #fff;
    stroke-width: 3px;
    stroke-linejoin: round;
    pointer-events: none;
  }
  text.dashboard-chart__percentage,
  text.dashboard-chart__pie-label {
    fill: #fff;
    font-size: 10px;
    paint-order: stroke;
    stroke: #31486b;
    stroke-width: 1.4px;
  }
  .ant-empty {
    margin: auto;
  }
}
</style>

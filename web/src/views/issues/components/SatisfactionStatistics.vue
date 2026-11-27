<template>
  <section class="satisfaction-statistics">
    <div class="satisfaction-filters">
      <DashboardSLADateFilter
        :filters="dateFilters"
        :today="today"
        @update:filters="updateDates"
      />
      <a-select
        v-for="field in facets"
        :key="field.key"
        v-model:value="filters[field.key]"
        :options="field.options"
        :placeholder="field.label"
        :aria-label="field.label"
        allow-clear
        show-search
        option-filter-prop="label"
        size="small"
        @change="page = 1"
      />
    </div>
    <a-alert
      v-if="failed"
      type="error"
      :message="t('common_requestFailed')"
      show-icon
    >
      <template #action>
        <a-button
          size="small"
          :aria-label="t('issue_retry')"
          @click="reload++"
          >{{ t("issue_retry") }}</a-button
        >
      </template>
    </a-alert>
    <a-spin :spinning="loading">
      <div v-if="data && !failed" class="satisfaction-content">
        <div class="metric-grid">
          <article
            v-for="metric in metrics"
            :key="metric.key"
            class="satisfaction-card"
          >
            <div class="satisfaction-metric-heading">
              <a-tooltip
                v-if="metric.key === 'participation'"
                :title="t('satisfaction_statistics_hint')"
                :trigger="['hover', 'focus']"
              >
                <button
                  type="button"
                  class="satisfaction-help"
                  :aria-label="t('satisfaction_statistics_hint')"
                >
                  <QuestionCircleOutlined />
                </button>
              </a-tooltip>
              <span>{{ metric.label }}</span>
            </div>
            <strong class="satisfaction-number">{{ metric.value }}</strong>
            <div class="satisfaction-parts">
              <span v-for="part in metric.parts" :key="part.label"
                >{{ part.label }} {{ part.value }}</span
              >
            </div>
          </article>
        </div>
        <div class="distribution-grid">
          <article class="satisfaction-module">
            <header class="satisfaction-module-heading">
              <h3>{{ t("satisfaction_distribution") }}</h3>
            </header>
            <DashboardEChart
              v-if="hasEvaluations"
              :option="pieOption"
              :label="t('satisfaction_distribution')"
            />
            <a-empty v-else class="satisfaction-empty" />
          </article>
          <article class="satisfaction-module">
            <header class="satisfaction-module-heading">
              <h3>{{ t("satisfaction_rating_counts") }}</h3>
            </header>
            <DashboardEChart
              v-if="hasEvaluations"
              :option="barOption"
              :label="t('satisfaction_rating_counts')"
            />
            <a-empty v-else class="satisfaction-empty" />
          </article>
        </div>
        <article class="satisfaction-module">
          <header class="satisfaction-module-heading">
            <h3>{{ t("satisfaction_trend") }}</h3>
            <span>{{ interval }}</span>
          </header>
          <DashboardEChart
            v-if="hasEvaluations"
            :option="trendOption"
            :label="t('satisfaction_trend')"
          />
          <a-empty v-else class="satisfaction-empty" />
        </article>
        <article class="satisfaction-module">
          <header class="satisfaction-module-heading">
            <h3>{{ t("satisfaction_member_statistics") }}</h3>
          </header>
          <a-table
            :columns="memberColumns"
            :data-source="data.members"
            row-key="id"
            size="small"
            :pagination="false"
            :scroll="{ x: 1100 }"
          />
        </article>
        <article class="satisfaction-module">
          <header class="satisfaction-module-heading">
            <h3>{{ t("satisfaction_category_statistics") }}</h3>
          </header>
          <a-table
            :columns="categoryColumns"
            :data-source="data.categories"
            row-key="id"
            size="small"
            :pagination="false"
            :scroll="{ x: 1100 }"
          />
        </article>
        <footer
          v-if="Math.max(data.member_total, data.category_total) > 20"
          class="satisfaction-footer"
        >
          <a-pagination
            v-model:current="page"
            :page-size="20"
            :total="Math.max(data.member_total, data.category_total)"
            :show-size-changer="false"
            size="small"
          />
        </footer>
      </div>
      <a-empty v-else-if="!loading && !failed" />
    </a-spin>
  </section>
</template>
<script setup>
import { computed, reactive, ref, watch, onBeforeUnmount } from "vue";
import { useI18n } from "vue-i18n";
import dayjs from "dayjs";
import { QuestionCircleOutlined } from "@ant-design/icons-vue";
import DashboardSLADateFilter from "@/components/dashboard/DashboardSLADateFilter.vue";
import { categoryDateRange } from "@/components/dashboard/dashboardCharts";
import DashboardEChart from "@/components/dashboard/DashboardEChart.vue";
import { querySatisfactionStatistics } from "@/api/satisfaction";
const props = defineProps({
  wsId: { type: [String, Number], required: true },
  refresh: { type: Number, default: 0 },
});
const emit = defineEmits(["summary-change", "loading-change"]);
const { t } = useI18n();
const dateFilters = reactive({ preset: "week", range: undefined });
const today = computed(() => dayjs().format("YYYY-MM-DD"));
const page = ref(1),
  reload = ref(0),
  data = ref(null),
  loading = ref(false),
  failed = ref(false);
watch(
  [data, failed],
  ([value, requestFailed]) =>
    emit(
      "summary-change",
      value && !requestFailed ? { evaluatedAt: value.evaluated_at } : null,
    ),
  { immediate: true, flush: "sync" },
);
watch(loading, (value) => emit("loading-change", value), { immediate: true });
const filters = reactive({
  role_id: undefined,
  member_id: undefined,
  category_id: undefined,
  priority_id: undefined,
});
const kinds = ["positive", "neutral", "negative"];
let controller;
function updateDates(patch) {
  Object.assign(dateFilters, patch);
  page.value = 1;
}
const hasEvaluations = computed(() => Boolean(data.value?.count.submitted));
const interval = computed(() => {
  const [from, to] = categoryDateRange(
    dateFilters.preset,
    dateFilters.range,
    today.value,
  );
  return `${from.format("YYYY-MM-DD")} — ${to.format("YYYY-MM-DD")}`;
});
const metrics = computed(() => {
  const count = data.value?.count;
  if (!count) return [];
  return [
    {
      key: "participation",
      label: t("satisfaction_participation_rate"),
      value: rate(count.submitted, count.invited),
      parts: [
        { label: t("satisfaction_invited"), value: count.invited },
        { label: t("satisfaction_participated"), value: count.submitted },
      ],
    },
    ...kinds.map((kind) => ({
      key: kind,
      label: t(`satisfaction_stat_${kind}_rate`),
      value: rate(count[kind], count.submitted),
      parts: [{ label: t(`satisfaction_stat_${kind}`), value: count[kind] }],
    })),
  ];
});
const facets = computed(() =>
  ["role", "member", "category", "priority"].map((key) => ({
    key: `${key}_id`,
    label: t(`satisfaction_filter_${key}`),
    options: (data.value?.[`${key}_options`] || []).map((item) => ({
      value: item.id,
      label: item.name,
    })),
  })),
);
function rate(n, d) {
  return d ? `${((n * 100) / d).toFixed(2)}%` : "—";
}
const columns = computed(() => [
  ...[
    "invited",
    "submitted",
    "participation_rate",
    "positive",
    "positive_rate",
    "neutral",
    "neutral_rate",
    "negative",
    "negative_rate",
  ].map((key) => ({
    title: t(`satisfaction_stat_${key}`),
    key,
    width: 105,
    align: "right",
    customRender: ({ record }) =>
      key === "participation_rate"
        ? rate(record.count.submitted, record.count.invited)
        : key.endsWith("_rate")
          ? rate(record.count[key.replace("_rate", "")], record.count.submitted)
          : record.count[key],
  })),
]);
const memberColumns = computed(() => [
  { title: t("workspace_field_assignee"), dataIndex: "name", width: 155 },
  ...columns.value,
]);
const categoryColumns = computed(() => [
  { title: t("workspace_field_category"), dataIndex: "name", width: 155 },
  ...columns.value,
]);
const distribution = computed(() =>
  kinds.map((name) => ({
    name: t(`satisfaction_sentiment_${name}`),
    value: data.value?.count[name] || 0,
  })),
);
const pieOption = computed(() => ({
  tooltip: { trigger: "item" },
  color: ["#28bdbd", "#f5cd47", "#f56969"],
  legend: { show: false },
  series: [
    {
      type: "pie",
      radius: ["40%", "65%"],
      center: ["50%", "50%"],
      label: {
        formatter: `{b}\n${t("satisfaction_count")}: {c} ({d}%)`,
        fontSize: 12,
        edgeDistance: 8,
      },
      labelLine: { length: 12, length2: 8 },
      data: distribution.value,
    },
  ],
  media: [
    {
      query: { maxWidth: 420 },
      option: {
        legend: {
          show: true,
          bottom: 0,
          itemWidth: 10,
          itemHeight: 10,
          textStyle: { fontSize: 11 },
        },
        series: [
          {
            center: ["50%", "44%"],
            label: { show: false },
            labelLine: { show: false },
          },
        ],
      },
    },
    {
      option: {
        legend: { show: false },
        series: [
          {
            center: ["50%", "50%"],
            label: { show: true },
            labelLine: { show: true },
          },
        ],
      },
    },
  ],
}));
const barOption = computed(() => ({
  tooltip: { trigger: "axis" },
  grid: { left: 12, right: 16, top: 36, bottom: 8, containLabel: true },
  xAxis: {
    type: "category",
    data: distribution.value.map((item) => item.name),
  },
  yAxis: {
    type: "value",
    minInterval: 1,
    name: t("satisfaction_count"),
    splitLine: { lineStyle: { type: "dashed", color: "#e8edf3" } },
  },
  series: [
    {
      type: "bar",
      barMaxWidth: 48,
      data: distribution.value.map((item) => item.value),
      color: "#28bdbd",
    },
  ],
}));
const trendOption = computed(() => ({
  grid: { left: 12, right: 20, top: 20, bottom: 8, containLabel: true },
  tooltip: {
    trigger: "axis",
    valueFormatter: (value) => (value === null ? "—" : `${value.toFixed(2)}%`),
  },
  xAxis: {
    type: "category",
    data: data.value?.trend.map((item) => item.name) || [],
  },
  yAxis: {
    type: "value",
    min: 0,
    max: 100,
    axisLabel: { formatter: "{value}%" },
    splitLine: { lineStyle: { type: "dashed", color: "#e8edf3" } },
  },
  series: [
    {
      type: "line",
      connectNulls: false,
      data:
        data.value?.trend.map((item) =>
          item.count.submitted
            ? (item.count.positive * 100) / item.count.submitted
            : null,
        ) || [],
      color: "#28bdbd",
      areaStyle: { opacity: 0.1 },
    },
  ],
}));
const requestData = computed(() => {
  const [start, last] = categoryDateRange(
    dateFilters.preset,
    dateFilters.range,
    today.value,
  );
  const end = last.add(1, "day");
  return {
    from: start.unix(),
    to: end.unix(),
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    ...filters,
    page: page.value,
    size: 20,
  };
});
async function load() {
  controller?.abort();
  if (!requestData.value) return;
  const request = new AbortController();
  controller = request;
  loading.value = true;
  failed.value = false;
  try {
    const result = await querySatisfactionStatistics(
      props.wsId,
      requestData.value,
      request.signal,
    );
    if (!request.signal.aborted) data.value = result;
  } catch {
    if (!request.signal.aborted) failed.value = true;
  } finally {
    if (!request.signal.aborted) loading.value = false;
  }
}
watch(
  () => [
    props.wsId,
    JSON.stringify(requestData.value),
    props.refresh,
    reload.value,
  ],
  load,
  { immediate: true },
);
watch(
  () => props.wsId,
  () => {
    data.value = null;
    page.value = 1;
    Object.keys(filters).forEach((key) => (filters[key] = undefined));
  },
  { flush: "sync" },
);
onBeforeUnmount(() => controller?.abort());
</script>
<style scoped lang="less">
.satisfaction-statistics,
.satisfaction-content {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
  color: var(--app-text);
}
.satisfaction-filters {
  display: flex;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 8px;
  padding: 10px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface);
}
.satisfaction-filters :deep(.sla-date-filter) {
  margin: 0;
}
.satisfaction-filters :deep(.ant-select) {
  min-width: 125px;
  max-width: 100%;
}
.metric-grid,
.distribution-grid {
  display: grid;
  gap: 12px;
}
.metric-grid {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}
.distribution-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}
.satisfaction-card,
.satisfaction-module {
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 10px;
  padding: 18px;
  min-width: 0;
}
.satisfaction-card {
  text-align: center;
}
.satisfaction-metric-heading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 20px;
  font-size: 13px;
}
.satisfaction-number {
  display: block;
  margin: 16px 0;
  font-size: 30px;
  line-height: 1.2;
  font-weight: 650;
  font-variant-numeric: tabular-nums;
}
.satisfaction-parts {
  display: flex;
  justify-content: center;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 12px;
  color: var(--app-text-muted, var(--app-text-secondary));
}
.satisfaction-parts span {
  border-radius: 4px;
  padding: 3px 8px;
  background: var(--app-surface-muted);
}
.satisfaction-module-heading,
.satisfaction-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
}
.satisfaction-module-heading {
  margin-bottom: 14px;
}
.satisfaction-footer {
  justify-content: flex-end;
}
.satisfaction-module-heading h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}
.satisfaction-module-heading > span,
.satisfaction-footer {
  font-size: 12px;
  color: var(--app-text-muted, var(--app-text-secondary));
}
.satisfaction-help {
  display: inline-flex;
  align-items: center;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--app-text-muted, var(--app-text-secondary));
  font-size: 14px;
  cursor: help;
}
.satisfaction-help:focus-visible {
  outline: 2px solid var(--app-primary, #3b6dff);
  outline-offset: 3px;
}
.satisfaction-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 280px;
  margin: 0;
}
.satisfaction-module :deep(.ant-table-cell) {
  font-size: 12px;
}
@media (max-width: 1100px) {
  .metric-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 900px) {
  .distribution-grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .satisfaction-number {
    font-size: 24px;
  }
}
</style>

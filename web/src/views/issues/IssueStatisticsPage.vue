<template>
  <section
    class="issue-statistics"
    :class="{ 'issue-statistics--collapsed': sidebarCollapsed }"
  >
    <nav
      v-show="!sidebarCollapsed"
      id="statistics-navigation"
      :aria-label="t('issue_statistics_title')"
      class="statistics-navigation"
    >
      <strong class="statistics-navigation__heading"
        ><BarChartOutlined /> {{ t("issue_statistics_title") }}</strong
      >
      <RouterLink
        v-for="tab in views"
        :key="tab.key"
        :to="{
          name: 'IssueStatistics',
          params: { wsId },
          query: { view: tab.key },
        }"
        :class="{ selected: view === tab.key }"
        :aria-current="view === tab.key ? 'page' : undefined"
      >
        {{ t(tab.label) }}
      </RouterLink>
    </nav>
    <button
      type="button"
      class="statistics-sidebar-toggle"
      :aria-label="
        t(
          sidebarCollapsed
            ? 'dashboard_workload_expand_views'
            : 'dashboard_workload_collapse_views',
        )
      "
      :aria-expanded="!sidebarCollapsed"
      aria-controls="statistics-navigation"
      @click="sidebarCollapsed = !sidebarCollapsed"
    >
      <RightOutlined v-if="sidebarCollapsed" />
      <LeftOutlined v-else />
    </button>
    <main class="statistics-main">
      <header class="statistics-heading">
        <button
          type="button"
          class="statistics-heading__toggle"
          :aria-label="
            t(
              sidebarCollapsed
                ? 'dashboard_workload_expand_views'
                : 'dashboard_workload_collapse_views',
            )
          "
          :aria-expanded="!sidebarCollapsed"
          aria-controls="statistics-navigation"
          @click="sidebarCollapsed = !sidebarCollapsed"
        >
          <MenuUnfoldOutlined v-if="sidebarCollapsed" /><MenuFoldOutlined
            v-else
          />
        </button>
        <h1>{{ t(views.find((tab) => tab.key === view).label) }}</h1>
        <div class="statistics-heading__actions">
          <span
            v-if="
              view === 'overview' && overviewSummary && !metadataRequest.failed
            "
            class="statistics-heading__summary"
            >{{
              t("issue_overview_updated", {
                time: formatLocalDate(overviewSummary.evaluatedAt, true),
                count: Number(overviewSummary.total).toLocaleString(locale),
              })
            }}</span
          >
          <span
            v-if="view === 'satisfaction' && satisfactionSummary"
            class="statistics-heading__summary"
            >{{
              t("satisfaction_updated", {
                time: formatLocalDate(satisfactionSummary.evaluatedAt, true),
              })
            }}</span
          >
          <a-button
            class="statistics-heading__refresh"
            size="small"
            :loading="view === 'satisfaction' && satisfactionLoading"
            @click="refresh++"
          >
            <template #icon><ReloadOutlined /></template>
            {{ t("issue_statistics_refresh") }}
          </a-button>
        </div>
      </header>
      <SatisfactionStatistics
        v-if="view === 'satisfaction'"
        :ws-id="wsId"
        :refresh="refresh"
        @summary-change="satisfactionSummary = $event"
        @loading-change="satisfactionLoading = $event"
      />
      <a-alert
        v-else-if="metadataRequest.failed"
        type="error"
        :message="t('common_requestFailed')"
      >
        <template #action>
          <a-button size="small" @click="reloadMetadata++">{{
            t("issue_retry")
          }}</a-button>
        </template>
      </a-alert>
      <IssueOverviewPanel
        v-else-if="view === 'overview'"
        :ws-id="wsId"
        :metadata="metadata"
        :members="members"
        :today="statisticsToday"
        :refresh="refresh"
        @summary-change="overviewSummary = $event"
      />
      <a-spin
        v-else
        :spinning="
          metadataRequest.loading ||
          (view === 'risk' ? riskRequest.loading : summaryRequest.loading)
        "
        wrapper-class-name="statistics-loading"
      >
        <div class="statistics-body">
          <a-empty
            v-if="metadata && (!metadata.enabled || !metadata.stages.length)"
            :description="
              t(
                metadata.enabled
                  ? 'issue_statistics_no_stages'
                  : 'dashboard_sla_trend_disabled',
              )
            "
          />
          <div v-else-if="metadata" class="statistics-content">
            <div class="statistics-filters">
              <DashboardSLADateFilter
                :filters="filters"
                :today="statisticsToday"
                @update:filters="updateFilters"
              />
              <a-select
                v-for="field in selectFilters"
                :key="field.key"
                :value="filters[field.key]"
                :options="field.options"
                :mode="field.multiple ? 'multiple' : undefined"
                :placeholder="t(field.placeholder)"
                :aria-label="t(field.label)"
                :max-tag-count="1"
                option-filter-prop="label"
                allow-clear
                show-search
                size="small"
                @change="updateFilters({ [field.key]: $event })"
              />
              <label v-if="view === 'risk'" class="statistics-threshold">
                {{ t("dashboard_sla_risk_threshold_minutes") }}
                <a-input-number
                  :value="riskFilters.thresholdMinutes"
                  :min="1"
                  :max="525600"
                  :precision="0"
                  :aria-label="t('dashboard_sla_risk_threshold_minutes')"
                  size="small"
                  @change="updateThreshold"
                />
              </label>
            </div>

            <a-empty
              v-if="
                activeResult?.enabled === false ||
                activeResult?.stages.length === 0
              "
              :description="
                t(
                  activeResult.enabled
                    ? 'issue_statistics_no_stages'
                    : 'dashboard_sla_trend_disabled',
                )
              "
            />
            <template v-else-if="view === 'risk'">
              <DashboardSLARisk
                separate-modules
                :result="riskRequest.data"
                :filters="riskFilters"
                :show-filters="false"
                :members="members"
                :priorities="metadata.priorities"
                :statuses="metadata.statuses"
                :failed="riskRequest.failed"
                @update:filters="Object.assign(riskFilters, $event)"
                @retry="refresh++"
              >
                <template #title="{ record }">
                  <button
                    type="button"
                    class="statistics-item-title"
                    :title="record.title"
                    @click="detailId = String(record.id)"
                  >
                    {{ record.title }}
                  </button>
                </template>
              </DashboardSLARisk>
            </template>
            <template v-else>
              <a-alert
                v-if="summaryRequest.failed"
                type="error"
                :message="t('dashboard_chart_failed')"
              >
                <template #action>
                  <a-button size="small" @click="refresh++">{{
                    t("issue_retry")
                  }}</a-button>
                </template>
              </a-alert>
              <template v-else>
                <div class="statistics-metrics">
                  <a-card
                    v-for="metric in metrics"
                    :key="metric.key"
                    size="small"
                  >
                    <h2>{{ metric.label }}</h2>
                    <DashboardSLAMetric
                      :config="metric.config"
                      :result="metric.result"
                      :comparison-unit="
                        metric.key === 'overdue' ? undefined : 'pt'
                      "
                    />
                  </a-card>
                </div>
                <a-card
                  :title="t('dashboard_chart_sla_trend')"
                  size="small"
                  class="statistics-trend"
                >
                  <DashboardSLATrend
                    :config="trendStyle"
                    :result="summaryRequest.data"
                  />
                </a-card>
              </template>

              <div class="statistics-charts">
                <a-card
                  :title="t('dashboard_chart_sla_dimension')"
                  size="small"
                >
                  <template #extra>
                    <a-radio-group
                      v-model:value="chartDimension"
                      :aria-label="t('dashboard_category_dimension')"
                      button-style="solid"
                      size="small"
                    >
                      <a-radio-button value="priority">{{
                        t("view_field_priority")
                      }}</a-radio-button>
                      <a-radio-button value="category">{{
                        t("dashboard_category_name")
                      }}</a-radio-button>
                    </a-radio-group>
                  </template>
                  <a-alert
                    v-if="
                      dimensionRequest.failed ||
                      (chartDimension === 'priority' && summaryRequest.failed)
                    "
                    type="error"
                    :message="t('dashboard_chart_failed')"
                  >
                    <template #action>
                      <a-button size="small" @click="refresh++">{{
                        t("issue_retry")
                      }}</a-button>
                    </template>
                  </a-alert>
                  <a-spin
                    v-else
                    :spinning="
                      chartDimension === 'category' && dimensionRequest.loading
                    "
                  >
                    <DashboardSLADimension
                      stage-series
                      :config="{
                        ...stageStyles,
                        sla_dimension: chartDimension,
                      }"
                      :result="
                        chartDimension === 'priority'
                          ? summaryRequest.data
                          : dimensionRequest.data
                      "
                    />
                  </a-spin>
                </a-card>
                <a-card
                  :title="t('dashboard_chart_sla_distribution')"
                  size="small"
                >
                  <DashboardSLADistribution
                    by-work-item
                    :config="stageStyles"
                    :result="summaryRequest.data"
                  />
                </a-card>
              </div>

              <a-card :title="t('issue_statistics_details')" size="small">
                <a-tabs v-model:active-key="detailDimension" size="small">
                  <a-tab-pane
                    v-for="dimension in detailDimensions"
                    :key="dimension.key"
                    :tab="t(dimension.label)"
                  />
                </a-tabs>
                <a-alert
                  v-if="detailsRequest.failed"
                  type="error"
                  :message="t('dashboard_chart_failed')"
                >
                  <template #action>
                    <a-button size="small" @click="refresh++">{{
                      t("issue_retry")
                    }}</a-button>
                  </template>
                </a-alert>
                <a-table
                  v-else
                  row-key="key"
                  size="small"
                  :columns="detailColumns"
                  :data-source="detailsRequest.data?.list || []"
                  :loading="detailsRequest.loading"
                  :locale="{ emptyText: t('dashboard_chart_empty') }"
                  :pagination="detailPagination"
                  :scroll="{ x: detailScrollWidth }"
                  @change="({ current }) => (detailPage = current)"
                >
                  <template #bodyCell="{ column, text, record }">
                    <template v-if="column.key === 'group'">{{
                      detailName(record)
                    }}</template>
                    <template v-else-if="column.format === 'rate'">{{
                      rate(text)
                    }}</template>
                    <template v-else-if="column.format === 'duration'">
                      {{ text == null ? "—" : slaDuration(text, t) }}
                    </template>
                  </template>
                </a-table>
              </a-card>
            </template>
          </div>
        </div>
      </a-spin>
    </main>
    <RequirementDetailDrawer
      :open="!!detailId"
      :ws-id="wsId"
      :requirement-id="detailId"
      mode="drawer"
      :navigation-items="detailNavigationItems"
      :can-edit-by-type="{
        requirement: authStore.hasPermission('requirement:edit'),
        defect: authStore.hasPermission('defect:edit'),
      }"
      :can-create-by-type="{
        requirement: authStore.hasPermission('requirement:create'),
        defect: authStore.hasPermission('defect:create'),
      }"
      @close="detailId = ''"
      @navigate="(id) => (detailId = String(id))"
      @updated="refresh++"
    />
  </section>
</template>

<script setup>
import { computed, inject, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import {
  BarChartOutlined,
  LeftOutlined,
  RightOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  ReloadOutlined,
} from "@ant-design/icons-vue";
import {
  getIssueStatisticsMetadata,
  queryIssueStatistics,
} from "@/api/issueStatistics";
import IssueOverviewPanel from "./components/IssueOverviewPanel.vue";
import DashboardSLADateFilter from "@/components/dashboard/DashboardSLADateFilter.vue";
import DashboardSLADimension from "@/components/dashboard/DashboardSLADimension.vue";
import DashboardSLADistribution from "@/components/dashboard/DashboardSLADistribution.vue";
import SatisfactionStatistics from "./components/SatisfactionStatistics.vue";
import DashboardSLAMetric from "@/components/dashboard/DashboardSLAMetric.vue";
import DashboardSLARisk from "@/components/dashboard/DashboardSLARisk.vue";
import DashboardSLATrend from "@/components/dashboard/DashboardSLATrend.vue";
import { slaMultipleOverdueColor } from "@/components/dashboard/dashboardCharts";
import { slaDuration } from "@/utils/sla";
import { formatLocalDate } from "@/utils/date";
import { useAuthStore } from "@/stores/auth";
import { useIssueStatisticsStore } from "@/stores/issueStatistics";
import RequirementDetailDrawer from "@/components/common/RequirementDetailDrawer.vue";

const { t, locale } = useI18n();
const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const statisticsStore = useIssueStatisticsStore();
const { workspaceMetadata, visibleTopTabs } = inject("issueBoard");
const canViewStatistics = computed(() =>
  visibleTopTabs.value.includes("statistics"),
);
watch(
  canViewStatistics,
  (available) => {
    if (!available) {
      router.replace({
        name: "IssueList",
        params: { wsId: route.params.wsId },
      });
    }
  },
  { immediate: true },
);
const wsId = computed(() => String(route.params.wsId));
const view = computed(() =>
  ["sla", "risk", "satisfaction"].includes(route.query.view)
    ? route.query.view
    : "overview",
);
const detailId = ref("");
const overviewSummary = ref(null);
const satisfactionSummary = ref(null);
const satisfactionLoading = ref(false);
watch(
  [wsId, view],
  () => {
    detailId.value = "";
    overviewSummary.value = null;
    satisfactionSummary.value = null;
    satisfactionLoading.value = false;
  },
  { flush: "sync" },
);
const views = [
  { key: "overview", label: "issue_overview_navigation" },
  { key: "sla", label: "dashboard_template_sla_stats" },
  { key: "risk", label: "dashboard_template_sla_risk" },
  { key: "satisfaction", label: "satisfaction_statistics" },
];
const sidebarCollapsed = ref(false);
const summaryFilters = reactive({ preset: "week" });
const riskFilters = reactive({ preset: "week", page: 1, thresholdMinutes: 30 });
watch(
  [() => authStore.userInfo?.id, wsId],
  ([userId, workspaceId]) => {
    riskFilters.thresholdMinutes = statisticsStore.getThreshold(
      userId,
      workspaceId,
    );
    riskFilters.page = 1;
  },
  { immediate: true, flush: "sync" },
);
const filters = computed(() =>
  view.value === "risk" ? riskFilters : summaryFilters,
);
const refresh = ref(0);
const reloadMetadata = ref(0);
const chartDimension = ref("priority");
const detailDimension = ref("time");
const detailPage = ref(1);
const detailDimensions = [
  { key: "time", label: "issue_statistics_by_time" },
  { key: "rule", label: "issue_statistics_by_rule" },
  { key: "category", label: "issue_statistics_by_category" },
  { key: "member", label: "issue_statistics_by_member" },
];
const trendStyle = {
  sla_target_rate: 95,
  sla_rate_color: "#1664ff",
  sla_overdue_color: slaMultipleOverdueColor,
};
const stagePalette = ["#4096ff", "#36cfc9", "#ffc069", "#b37feb", "#ff85c0"];

const metadataRequest = reactive({ data: null, loading: false, failed: false });
const metadata = computed(() => metadataRequest.data);
const stages = computed(
  () => summaryRequest.data?.stages || metadata.value?.stages || [],
);
const stageStyles = computed(() => ({
  sla_stage_colors: Object.fromEntries(
    stages.value.map((stage, index) => [
      stage.id,
      stagePalette[index % stagePalette.length],
    ]),
  ),
}));
const members = computed(() => workspaceMetadata?.value?.members || []);
const memberOptions = computed(() =>
  members.value.map((member) => ({
    value: Number(member.user_id),
    label: member.user_name || member.nickname || String(member.user_id),
  })),
);
const labels = (items) =>
  (items || []).map(({ id, name }) => ({ value: id, label: name }));
const categoryOptions = computed(() => [
  ...labels(metadata.value?.categories),
  { value: 0, label: t("sla_uncategorized") },
]);
const selectFilters = computed(() => {
  const fields = [
    {
      key: "assignee",
      options: memberOptions.value,
      label: "view_field_assignee",
      placeholder: "dashboard_category_all_members",
    },
    {
      key: "priority",
      options: [
        ...labels(metadata.value?.priorities),
        { value: 0, label: t("dashboard_chart_unset") },
      ],
      label: "view_field_priority",
      placeholder: "dashboard_category_all_priorities",
    },
    {
      key: "category",
      options: categoryOptions.value,
      label: "dashboard_category_name",
      placeholder: "issue_statistics_all_categories",
    },
  ];
  if (view.value === "risk")
    fields.push({
      key: "roles",
      options: labels(metadata.value?.roles),
      label: "issue_statistics_roles",
      placeholder: "issue_statistics_all_roles",
      multiple: true,
    });
  return fields;
});
const statisticsToday = computed(() => {
  if (!metadata.value?.timezone) return undefined;
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone: metadata.value.timezone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts();
  return ["year", "month", "day"]
    .map((type) => parts.find((part) => part.type === type).value)
    .join("-");
});

function watchQuery(state, input, load) {
  watch(
    input,
    async (key, _, onCleanup) => {
      const controller = new AbortController();
      onCleanup(() => controller.abort());
      state.data = null;
      state.failed = false;
      state.loading = Boolean(key);
      if (!key) return;
      try {
        const data = await load(key, controller.signal);
        if (!controller.signal.aborted) state.data = data;
      } catch {
        if (!controller.signal.aborted) state.failed = true;
      } finally {
        if (!controller.signal.aborted) state.loading = false;
      }
    },
    { immediate: true },
  );
}

function queryState(kind, filterState, enabled, extra = () => ({})) {
  const state = reactive({ data: null, loading: false, failed: false });
  watchQuery(
    state,
    () => {
      if (
        !canViewStatistics.value ||
        !metadata.value?.enabled ||
        !metadata.value?.stages.length ||
        !enabled()
      )
        return null;
      return {
        workspace: wsId.value,
        refresh: refresh.value,
        data: {
          preset: filterState.preset,
          from: filterState.range?.[0],
          to: filterState.range?.[1],
          assignee_id: filterState.assignee,
          priority_id: filterState.priority,
          category_id: filterState.category,
          role_ids: filterState.roles,
          threshold_minutes: filterState.thresholdMinutes,
          page: filterState.page,
          ...extra(),
        },
      };
    },
    (key, signal) =>
      queryIssueStatistics(key.workspace, kind, key.data, signal),
  );
  return state;
}

watchQuery(
  metadataRequest,
  () =>
    canViewStatistics.value && view.value !== "satisfaction"
      ? { workspace: wsId.value, refresh: reloadMetadata.value }
      : null,
  (key, signal) => getIssueStatisticsMetadata(key.workspace, signal),
);
const summaryRequest = queryState(
  "summary",
  summaryFilters,
  () => view.value === "sla",
  () => ({ dimension: "priority" }),
);
const dimensionRequest = queryState(
  "dimension",
  summaryFilters,
  () => view.value === "sla" && chartDimension.value === "category",
  () => ({ dimension: "category" }),
);
const detailsRequest = queryState(
  "details",
  summaryFilters,
  () => view.value === "sla",
  () => ({ dimension: detailDimension.value, page: detailPage.value }),
);
const riskRequest = queryState(
  "risk",
  riskFilters,
  () => view.value === "risk",
);
const detailNavigationItems = computed(() =>
  (riskRequest.data?.sla_risk?.list || []).map((item) => ({
    id: item.id,
    workspaceId: wsId.value,
  })),
);
const activeResult = computed(() =>
  view.value === "risk" ? riskRequest.data : summaryRequest.data,
);

watch(
  () => route.query.view,
  (next) => {
    if (next != null && !views.some((tab) => tab.key === next)) {
      void router.replace({ query: { ...route.query, view: "overview" } });
    }
  },
  { immediate: true },
);
watch(detailDimension, () => (detailPage.value = 1), { flush: "sync" });

function updateThreshold(value) {
  if (Number.isInteger(value) && value >= 1 && value <= 525600) {
    statisticsStore.setThreshold(authStore.userInfo?.id, wsId.value, value);
    updateFilters({ thresholdMinutes: value });
  }
}

function updateFilters(patch) {
  if (view.value === "risk") Object.assign(riskFilters, patch, { page: 1 });
  else {
    detailPage.value = 1;
    Object.assign(summaryFilters, patch);
  }
}

const metrics = computed(() => {
  const current = summaryRequest.data?.current;
  const previous = summaryRequest.data?.previous;
  const series = summaryRequest.data?.sparklines || [];
  const items = [
    {
      key: "overall",
      label: t("dashboard_metric_sla_achievement_rate"),
      now: current?.rate,
      prior: previous?.rate,
      trend: series[0],
    },
    ...[0, 1].map((index) => ({
      key: `stage-${index}`,
      label: stages.value[index]?.name
        ? t("dashboard_sla_named_stage_rate", {
            name: stages.value[index].name,
          })
        : t("issue_statistics_no_stage"),
      now: current?.stages[index]?.rate,
      prior: previous?.stages[index]?.rate,
      trend: stages.value[index] ? series[index + 1] : [],
    })),
    {
      key: "overdue",
      label: t("dashboard_metric_sla_overdue_count"),
      now: current?.overdue,
      prior: previous?.overdue,
      trend: series.at(-1),
    },
  ];
  return items.map((item, index) => ({
    key: item.key,
    label: item.label,
    config: {
      metric_type:
        item.key === "overdue" ? "sla_overdue_count" : "sla_achievement_rate",
      unit: item.key === "overdue" ? t("dashboard_sla_count_unit") : "%",
      sla_show_comparison: true,
      sla_recent_days: 1,
      sla_line_color:
        item.key === "overdue"
          ? "#f5222d"
          : stagePalette[index % stagePalette.length],
    },
    result: {
      value: item.now,
      sla: {
        trend: item.trend,
        change:
          item.now == null || item.prior == null ? null : item.now - item.prior,
      },
    },
  }));
});

const detailStages = computed(
  () => detailsRequest.data?.stages || stages.value,
);
const detailColumns = computed(() => [
  {
    key: "group",
    title: t(
      detailDimensions.find((item) => item.key === detailDimension.value).label,
    ),
    width: 175,
  },
  {
    key: "total",
    dataIndex: "total",
    title: t("issue_statistics_total"),
    width: 110,
    align: "right",
  },
  {
    key: "achieved",
    dataIndex: "achieved",
    title: t("issue_statistics_achieved"),
    width: 90,
    align: "right",
  },
  {
    key: "rate",
    dataIndex: "rate",
    title: t("dashboard_metric_sla_achievement_rate"),
    width: 115,
    align: "right",
    format: "rate",
  },
  ...detailStages.value.flatMap((stage, index) => [
    {
      key: `achieved-${stage.id}`,
      dataIndex: ["stages", index, "achieved"],
      title: t("issue_statistics_stage_achieved", { name: stage.name }),
      width: 140,
      align: "right",
    },
    {
      key: `rate-${stage.id}`,
      dataIndex: ["stages", index, "rate"],
      title: t("issue_statistics_stage_rate", { name: stage.name }),
      width: 140,
      align: "right",
      format: "rate",
    },
    {
      key: `average-${stage.id}`,
      dataIndex: ["stages", index, "average_overdue_seconds"],
      title: t("issue_statistics_stage_average_overdue", { name: stage.name }),
      width: 220,
      className: "statistics-duration",
      align: "right",
      format: "duration",
    },
  ]),
]);
const detailScrollWidth = computed(() =>
  detailColumns.value.reduce((width, column) => width + column.width, 0),
);
const detailPagination = computed(() => ({
  current: detailPage.value,
  pageSize: detailsRequest.data?.size || 20,
  total: detailsRequest.data?.total || 0,
  showSizeChanger: false,
  hideOnSinglePage: true,
}));
const rate = (value) => (value == null ? "—" : `${Number(value.toFixed(1))}%`);

function detailName(row) {
  if (detailDimension.value === "time") return row.key;
  const id = Number(row.key);
  if (detailDimension.value === "category")
    return (
      categoryOptions.value.find((item) => item.value === id)?.label || `#${id}`
    );
  if (detailDimension.value === "member")
    return id
      ? memberOptions.value.find((item) => item.value === id)?.label || `#${id}`
      : t("issue_statistics_unassigned");
  return row.name || t("issue_statistics_unknown_rule");
}
</script>

<style scoped lang="less">
.issue-statistics {
  position: relative;
  display: grid;
  grid-template-columns: 188px minmax(0, 1fr);
  flex: 1;
  min-height: 0;
  color: var(--app-text);
  background: var(--app-surface-muted);
}
.issue-statistics--collapsed {
  grid-template-columns: minmax(0, 1fr);
}
.statistics-sidebar-toggle {
  position: absolute;
  top: min(45vh, 400px);
  left: 188px;
  z-index: 21;
  width: 20px;
  height: 42px;
  padding: 0;
  transform: translateX(-50%);
  border: 1px solid #f0f0f0;
  border-radius: 7px;
  background: #fff;
  color: #8c8c8c;
  cursor: pointer;
}
.issue-statistics--collapsed .statistics-sidebar-toggle {
  left: 0;
  transform: none;
}
.statistics-heading {
  display: flex;
  flex: none;
  flex-wrap: nowrap;
  align-items: center;
  gap: 12px;
  margin: -8px -12px 12px;
  padding: 0 12px 12px;
  background: #fff;

  h1 {
    flex: 0 1 auto;
    min-width: 0;
    max-width: 280px;
    margin: 0;
    padding: 4px 0;
    overflow: hidden;
    color: #262626;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 18px;
    font-weight: 600;
  }
}
.statistics-heading__actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 12px;
  margin-left: auto;
}
.statistics-heading__summary {
  font-size: 12px;
  color: var(--app-text-muted);
}
.statistics-heading__refresh {
  flex: none;
}
.statistics-item-title {
  max-width: 100%;
  padding: 0;
  overflow: hidden;
  border: 0;
  background: transparent;
  color: var(--ant-color-link, #1677ff);
  font: inherit;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
}
.statistics-heading__toggle {
  flex: none;
  border: 0;
  background: transparent;
  color: #262626;
  cursor: pointer;
}
.statistics-navigation__heading {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px 12px;
}
.statistics-threshold {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.statistics-navigation {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px;
  border-right: 1px solid var(--app-border);
  background: var(--app-surface);

  a {
    padding: 7px 10px;
    border-radius: 6px;
    color: var(--app-text);
  }
  a.selected {
    color: #1664ff;
    background: color-mix(in srgb, #1664ff 10%, var(--app-surface));
  }
}
.statistics-main {
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: auto;
  padding: 8px 12px 12px;

  h2 {
    font-size: 13px;
    margin: 0 0 6px;
  }
}
.statistics-main :deep(.statistics-duration) {
  white-space: nowrap;
}
.statistics-loading {
  display: flex;
  flex: 1;
  flex-direction: column;

  :deep(> .ant-spin-container) {
    flex: 1;
    min-width: 0;
  }
  :deep(> div > .ant-spin) {
    max-height: none;
  }
}
.statistics-filters {
  display: flex;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 8px;
  padding: 10px;
  margin-bottom: 12px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface);

  :deep(.sla-date-filter) {
    margin: 0;
  }
  :deep(.ant-select) {
    min-width: 125px;
  }
}
.statistics-metrics,
.statistics-charts {
  display: grid;
  gap: 12px;
  margin-bottom: 12px;
}
.statistics-metrics {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}
.statistics-charts {
  grid-template-columns: minmax(0, 3fr) minmax(0, 2fr);

  :deep(.sla-dimension__group) {
    grid-template-columns: minmax(44px, 12%) minmax(0, 1fr);
  }
}
.statistics-trend {
  margin-bottom: 12px;
}

@media (max-width: 960px) {
  .issue-statistics {
    grid-template-columns: 1fr;
    grid-template-rows: auto minmax(0, 1fr);
    &.issue-statistics--collapsed {
      grid-template-rows: minmax(0, 1fr);
    }
  }
  .statistics-navigation {
    flex-direction: row;
    padding-right: 32px;
    border-right: none;
    border-bottom: 1px solid var(--app-border);
  }
  .statistics-sidebar-toggle {
    top: 8px;
    right: 4px;
    left: auto;
    transform: none;
  }
  .issue-statistics--collapsed .statistics-sidebar-toggle {
    right: auto;
  }
  .statistics-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .statistics-charts {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>

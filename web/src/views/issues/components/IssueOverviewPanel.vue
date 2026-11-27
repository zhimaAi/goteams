<template>
  <a-spin :spinning="loading || !metadata">
    <section class="overview">
      <div class="overview-filters">
        <DashboardSLADateFilter
          :filters="filters"
          :today="today"
          @update:filters="Object.assign(filters, $event)"
        />
        <a-select
          v-for="field in filterFields"
          :key="field.key"
          v-model:value="filters[field.key]"
          :options="field.options"
          :placeholder="field.label"
          :aria-label="field.label"
          option-filter-prop="label"
          allow-clear
          show-search
          size="small"
        />
      </div>
      <a-alert
        v-if="failed"
        type="error"
        :message="t('common_requestFailed')"
        show-icon
      >
        <template #action
          ><a-button @click="reload++">{{
            t("issue_retry")
          }}</a-button></template
        >
      </a-alert>
      <template v-else-if="data">
        <div class="overview-cards">
          <article v-for="card in cards" :key="card.key" class="overview-card">
            <div class="overview-card-heading">
              <span class="overview-title">
                <a-tooltip :title="card.help" :trigger="['hover', 'focus']">
                  <button
                    type="button"
                    class="overview-help"
                    :aria-label="card.help"
                  >
                    <QuestionCircleOutlined />
                  </button>
                </a-tooltip>
                {{ card.label }}
              </span>
              <component :is="card.icon" :style="{ color: card.color }" />
            </div>
            <button
              v-if="card.clickable"
              type="button"
              class="overview-number overview-link"
              @click="openDetails(card.key)"
            >
              {{ number(card.value) }}
              <small>{{ t("issue_overview_unit") }}</small>
            </button>
            <strong v-else class="overview-number"
              >{{ number(card.value) }}
              <small>{{ t("issue_overview_unit") }}</small></strong
            >
            <div class="overview-parts">
              <template v-for="part in card.parts" :key="part.key">
                <button
                  v-if="part.clickable"
                  type="button"
                  class="overview-link"
                  @click="openDetails(part.key, part.id)"
                >
                  {{ part.name }} {{ number(part.value) }}
                </button>
                <span v-else>{{ part.name }} {{ number(part.value) }}</span>
              </template>
              <span v-if="card.previous != null" :class="changeClass(card)">{{
                comparison(card)
              }}</span>
            </div>
          </article>
        </div>
        <article class="overview-module overview-trend">
          <div class="overview-module-heading">
            <h3 class="overview-title">
              <a-tooltip
                :title="t('issue_overview_trend_help')"
                :trigger="['hover', 'focus']"
              >
                <button
                  type="button"
                  class="overview-help"
                  :aria-label="t('issue_overview_trend_help')"
                >
                  <QuestionCircleOutlined />
                </button>
              </a-tooltip>
              {{ t("issue_overview_trend") }}
            </h3>
            <span>{{ interval }}</span>
          </div>
          <DashboardEChart
            :option="trendOption"
            :label="t('issue_overview_trend')"
          />
        </article>
        <div class="overview-middle">
          <article class="overview-module">
            <div class="overview-module-heading">
              <h3>{{ t("issue_overview_categories") }}</h3>
              <a-button
                type="link"
                size="small"
                @click="categoriesOpen = true"
                >{{ t("issue_overview_view_all") }}</a-button
              >
            </div>
            <div class="overview-module-heading">
              <span>{{ interval }}</span
              ><span>{{ t("issue_overview_bar_legend") }}</span>
            </div>
            <a-empty v-if="!data.current.total" />
            <div
              v-for="group in data.categories.slice(0, 5)"
              v-else
              :key="group.id"
              class="overview-category"
            >
              <div>
                <span>{{ groupName(group, "category") }}</span
                ><span
                  >{{
                    t("issue_overview_group_counts", {
                      total: number(group.total),
                      finished: number(group.finished),
                    })
                  }}
                  · {{ rate(percent(group.total, data.current.total)) }}</span
                >
              </div>
              <a-tooltip
                :title="
                  t('issue_overview_group_help', {
                    total: group.total,
                    finished: group.finished,
                    rate: rate(percent(group.finished, group.total)),
                  })
                "
              >
                <div class="overview-bar">
                  <i
                    class="overview-bar-total"
                    :style="{
                      width: `${percent(group.total, categoryMax) || 0}%`,
                    }"
                  /><i
                    :style="{
                      width: `${percent(group.finished, categoryMax) || 0}%`,
                    }"
                  />
                </div>
              </a-tooltip>
            </div>
          </article>
          <article class="overview-module">
            <div class="overview-module-heading">
              <h3>{{ t("issue_overview_statuses") }}</h3>
            </div>
            <a-empty v-if="!data.current.total" /><DashboardEChart
              v-else
              :option="statusOption"
              :label="t('issue_overview_statuses')"
            />
          </article>
        </div>
        <div class="overview-bottom">
          <article class="overview-module">
            <div class="overview-module-heading">
              <h3>{{ t("issue_overview_priorities") }}</h3>
            </div>
            <a-empty v-if="!data.current.total" /><DashboardEChart
              v-else
              :option="priorityOption"
              :label="t('issue_overview_priorities')"
            />
          </article>
          <article class="overview-module">
            <div class="overview-module-heading">
              <h3 class="overview-title">
                <a-tooltip
                  :title="t('issue_overview_ranking_help')"
                  :trigger="['hover', 'focus']"
                >
                  <button
                    type="button"
                    class="overview-help"
                    :aria-label="t('issue_overview_ranking_help')"
                  >
                    <QuestionCircleOutlined />
                  </button>
                </a-tooltip>
                {{ t("issue_overview_ranking") }}
              </h3>
              <a-button type="link" size="small" @click="membersOpen = true">{{
                t("issue_overview_view_all")
              }}</a-button>
            </div>
            <a-empty v-if="!data.members.length" />
            <div v-if="data.members.length" class="overview-members">
              <div
                v-for="(member, index) in data.members.slice(0, 5)"
                :key="member.id"
                class="overview-member"
              >
                <span
                  class="overview-rank"
                  :class="{ 'overview-rank--leading': index < 2 }"
                  >{{ index + 1 }}</span
                >
                <span class="overview-member-name" :title="member.name">{{
                  member.name
                }}</span>
                <div class="overview-bar">
                  <i
                    :style="{
                      width: `${percent(member.total, data.members[0].total) || 0}%`,
                    }"
                  />
                </div>
                <span class="overview-member-count"
                  >{{ number(member.total) }}
                  {{ t("issue_overview_unit") }}</span
                >
              </div>
            </div>
          </article>
          <article class="overview-module">
            <div class="overview-module-heading">
              <h3>{{ t("issue_overview_sla") }}</h3>
            </div>
            <a-alert
              v-if="!data.sla_enabled"
              type="info"
              :message="t('issue_overview_sla_unavailable')"
            />
            <div
              v-for="metric in slaMetrics"
              :key="metric.key"
              class="overview-sla-row"
            >
              <div>
                <span>{{ metric.name }}</span>
                <strong>{{ metric.text }}</strong>
              </div>
              <a-progress
                v-if="metric.percent != null"
                :percent="metric.percent"
                :show-info="false"
                :stroke-color="metric.color"
                size="small"
              />
            </div>
          </article>
        </div>
      </template>
    </section>
  </a-spin>
  <a-drawer
    :open="!!drawer"
    :width="'min(1400px, 94vw)'"
    :title="drawerTitle"
    @close="closeDetails"
  >
    <template #extra
      ><a-button @click="settingsOpen = true">{{
        t("common_settings")
      }}</a-button></template
    >
    <p>{{ t("issue_overview_detail_scope") }}</p>
    <div class="overview-detail-search">
      <a-input-search
        v-model:value="searchDraft"
        :maxlength="300"
        :placeholder="t('report_item_title')"
        :aria-label="t('report_item_title')"
        allow-clear
        @search="applySearch"
      />
      <a-select
        v-model:value="detailStatuses"
        :options="statusOptions"
        mode="multiple"
        :placeholder="
          t(
            drawer?.key === 'completed'
              ? 'report_target_status'
              : 'report_all_statuses',
          )
        "
        :aria-label="
          t(
            drawer?.key === 'completed'
              ? 'report_target_status'
              : 'requirement_filterStatus',
          )
        "
        :max-tag-count="2"
        allow-clear
        @change="detailPage = 1"
      />
    </div>
    <a-alert
      v-if="detailFailed"
      type="error"
      :message="t('common_requestFailed')"
      ><template #action
        ><a-button @click="detailReload++">{{
          t("issue_retry")
        }}</a-button></template
      ></a-alert
    >
    <a-table
      :columns="columns"
      :data-source="detailData?.list || []"
      :loading="detailLoading"
      :pagination="pagination"
      :scroll="{ x: 'max-content' }"
      row-key="id"
      @change="changePage"
    >
      <template #bodyCell="{ column, record, text }">
        <a-button
          v-if="column.key === 'title'"
          type="link"
          @click="itemId = String(record.id)"
          >{{ text }}</a-button
        >
        <template v-else-if="['created_at', 'event_at'].includes(column.key)">{{
          formatLocalDate(text, true)
        }}</template>
        <template
          v-else-if="
            column.key === 'status_id' || column.key === 'event_status_id'
          "
          >{{ optionName(metadata?.statuses || [], text) }}</template
        >
        <template v-else-if="column.key === 'priority_id'">{{
          optionName(metadata?.priorities || [], text, "priority")
        }}</template>
        <template v-else-if="column.key === 'category_id'">{{
          optionName(metadata?.categories || [], text, "category")
        }}</template>
        <template v-else-if="column.key === 'assignee_ids'">{{
          assigneeNames(record.assignee_ids)
        }}</template>
      </template>
    </a-table>
  </a-drawer>
  <ListSettingsModal
    v-model:open="settingsOpen"
    :user-id="auth.userInfo?.id"
    :ws-id="wsId"
    type-key="issue-overview"
    :fields="fields"
    hide-view-mode
    hide-preview-mode
    scoped-preview-mode
    @change="preferences = $event"
  />
  <RequirementDetailDrawer
    v-if="itemId"
    :open="true"
    :ws-id="wsId"
    :requirement-id="itemId"
    mode="drawer"
    :navigation-items="navigationItems"
    :can-edit-by-type="{ requirement: auth.hasPermission('requirement:edit') }"
    :can-create-by-type="{
      requirement: auth.hasPermission('requirement:create'),
    }"
    @close="itemId = ''"
    @navigate="itemId = String($event)"
    @updated="refreshAfterEdit"
  />
  <a-drawer
    v-model:open="categoriesOpen"
    :title="t('issue_overview_categories')"
    :width="700"
  >
    <p>{{ interval }}</p>
    <a-table
      :columns="categoryColumns"
      :data-source="data?.categories || []"
      row-key="id"
      :pagination="{ pageSize: 20 }"
      ><template #bodyCell="{ column, record }"
        ><template v-if="column.key === 'name'">{{
          groupName(record, "category")
        }}</template
        ><template v-else-if="column.key === 'rate'">{{
          rate(percent(record.finished, record.total))
        }}</template></template
      ></a-table
    >
  </a-drawer>
  <a-drawer
    v-model:open="membersOpen"
    :title="t('issue_overview_ranking')"
    :width="700"
  >
    <p>{{ interval }}</p>
    <a-table
      :columns="memberColumns"
      :data-source="memberRows"
      row-key="id"
      :pagination="{ pageSize: 20 }"
    />
  </a-drawer>
</template>
<script setup>
import { computed, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { theme } from "ant-design-vue";
import {
  FileTextOutlined,
  PlusSquareOutlined,
  ClockCircleOutlined,
  SyncOutlined,
  CheckCircleOutlined,
  WarningOutlined,
  QuestionCircleOutlined,
} from "@ant-design/icons-vue";
import {
  getIssueOverview,
  listIssueOverviewItems,
} from "@/api/issueStatistics";
import DashboardSLADateFilter from "@/components/dashboard/DashboardSLADateFilter.vue";
import DashboardEChart from "@/components/dashboard/DashboardEChart.vue";
import RequirementDetailDrawer from "@/components/common/RequirementDetailDrawer.vue";
import ListSettingsModal from "@/components/work-item/ListSettingsModal.vue";
import { useAuthStore } from "@/stores/auth";
import { formatLocalDate } from "@/utils/date";
import { slaDuration } from "@/utils/sla";
const props = defineProps({
  wsId: { type: String, required: true },
  metadata: { type: Object, default: null },
  members: { type: Array, default: () => [] },
  today: { type: String, default: undefined },
  refresh: { type: Number, default: 0 },
});
const emit = defineEmits(["summary-change"]);
const { t, locale } = useI18n();
const { token } = theme.useToken();
const auth = useAuthStore();
const filters = reactive({ preset: "week" });
const data = ref(null),
  loading = ref(false),
  failed = ref(false),
  reload = ref(0);
watch(
  data,
  (value) =>
    emit(
      "summary-change",
      value
        ? { evaluatedAt: value.evaluated_at, total: value.all.total }
        : null,
    ),
  { immediate: true, flush: "sync" },
);
const drawer = ref(null),
  itemId = ref(""),
  categoriesOpen = ref(false),
  membersOpen = ref(false),
  settingsOpen = ref(false);
const detailData = ref(null),
  detailLoading = ref(false),
  detailFailed = ref(false),
  detailPage = ref(1),
  detailSize = ref(20),
  detailReload = ref(0),
  detailStatuses = ref([]),
  searchDraft = ref(""),
  search = ref("");
const preferences = ref({ fields: [] });
const number = (value) =>
  value == null ? "—" : Number(value).toLocaleString(locale.value);
const percent = (value, total) => (total ? (value * 100) / total : null);
const rate = (value) => (value == null ? "—" : `${Number(value.toFixed(1))}%`);
const interval = computed(() =>
  data.value ? `${data.value.from} — ${data.value.to}` : "",
);
const statusOptions = computed(() =>
  (props.metadata?.statuses || []).map((v) => ({ value: v.id, label: v.name })),
);
const filterFields = computed(() => [
  {
    key: "status_id",
    label: t("report_all_statuses"),
    options: statusOptions.value,
  },
  {
    key: "priority_id",
    label: t("dashboard_category_all_priorities"),
    options: [
      ...(props.metadata?.priorities || []).map((v) => ({
        value: v.id,
        label: v.name,
      })),
      { value: 0, label: t("dashboard_chart_unset") },
    ],
  },
  {
    key: "category_id",
    label: t("issue_statistics_all_categories"),
    options: [
      ...(props.metadata?.categories || []).map((v) => ({
        value: v.id,
        label: v.name,
      })),
      { value: 0, label: t("sla_uncategorized") },
    ],
  },
]);
const params = computed(() => ({
  preset: filters.preset,
  from: filters.range?.[0],
  to: filters.range?.[1],
  status_id: filters.status_id,
  priority_id: filters.priority_id,
  category_id: filters.category_id,
}));
watch(
  [
    () => props.wsId,
    () => props.refresh,
    () => props.metadata,
    () => JSON.stringify(params.value),
    reload,
  ],
  async (_, __, onCleanup) => {
    const abort = new AbortController();
    onCleanup(() => abort.abort());
    data.value = null;
    failed.value = false;
    loading.value = !!props.metadata;
    if (!props.metadata) return;
    try {
      const result = await getIssueOverview(
        props.wsId,
        params.value,
        abort.signal,
      );
      if (!abort.signal.aborted) data.value = result;
    } catch {
      if (!abort.signal.aborted) failed.value = true;
    } finally {
      if (!abort.signal.aborted) loading.value = false;
    }
  },
  { immediate: true },
);
watch([() => props.wsId, () => JSON.stringify(params.value)], () => {
  closeDetails();
  categoriesOpen.value = false;
  membersOpen.value = false;
});
function groupName(group, kind) {
  return (
    group.name ||
    (group.id
      ? `#${group.id}`
      : t(kind === "category" ? "sla_uncategorized" : "dashboard_chart_unset"))
  );
}
function optionName(options, id, kind) {
  return (
    options.find((v) => v.id === id)?.name ||
    (id
      ? `#${id}`
      : t(kind === "category" ? "sla_uncategorized" : "dashboard_chart_unset"))
  );
}
function assigneeNames(ids) {
  return ids?.length
    ? ids
        .map((id) => {
          const m = props.members.find((v) => Number(v.user_id) === id);
          return m?.user_name || m?.nickname || `#${id}`;
        })
        .join(", ")
    : t("issue_statistics_unassigned");
}
const cards = computed(() => {
  if (!data.value) return [];
  const d = data.value,
    cur = d.current;
  return [
    {
      key: "total",
      label: t("issue_overview_total"),
      value: d.all.total,
      icon: FileTextOutlined,
      color: "#3b6dff",
      help: t("issue_overview_total_help"),
      parts: [
        {
          key: "finished",
          name: t("issue_overview_finished"),
          value: d.all.finished,
        },
        {
          key: "unfinished",
          name: t("issue_overview_unfinished"),
          value: d.all.total - d.all.finished,
        },
      ],
    },
    {
      key: "created",
      label: t("issue_overview_created"),
      value: cur.total,
      icon: PlusSquareOutlined,
      color: "#ff8b00",
      clickable: true,
      help: t("issue_overview_created_help"),
      parts: [
        {
          key: "finished",
          name: t("issue_overview_finished"),
          value: cur.finished,
          clickable: true,
        },
        {
          key: "unfinished",
          name: t("issue_overview_unfinished"),
          value: cur.total - cur.finished,
          clickable: true,
        },
      ],
    },
    {
      key: "pending",
      label: t("dashboard_category_pending_assign"),
      value: cur.pending,
      icon: ClockCircleOutlined,
      color: "#ff4d4f",
      clickable: true,
      parts: d.priorities
        .filter((v) => v.id)
        .slice(0, 2)
        .map((v) => ({
          key: "pending",
          id: v.id,
          name: groupName(v, "priority"),
          value: d.pending_priorities?.find((p) => p.id === v.id)?.total || 0,
          clickable: true,
        })),
    },
    {
      key: "processing",
      label: t("issue_overview_processing"),
      value: cur.processing,
      icon: SyncOutlined,
      color: "#3b6dff",
      clickable: true,
      parts: d.statuses
        .filter((v) => v.key === "processing")
        .slice(0, 2)
        .map((v) => ({
          key: "processing",
          id: v.id,
          name: v.name,
          value: v.total,
          clickable: true,
        })),
    },
    {
      key: "completed",
      label: t("issue_overview_completed"),
      value: d.completed,
      previous: d.previous_completed,
      icon: CheckCircleOutlined,
      color: "#00b42a",
      clickable: true,
      help: t("issue_overview_completed_help"),
      parts: [],
    },
    {
      key: "overdue",
      label: t("dashboard_metric_sla_overdue_count"),
      value: d.sla_enabled ? d.overdue : null,
      previous: d.sla_enabled ? d.previous_overdue : null,
      icon: WarningOutlined,
      color: "#ff4d4f",
      help: t("issue_overview_overdue_help"),
      parts: [],
    },
  ];
});
function comparison(card) {
  const last = t(
    filters.preset === "today"
      ? "issue_overview_yesterday"
      : "issue_overview_previous",
  );
  if (!card.previous)
    return card.value
      ? t("issue_overview_no_comparison")
      : t("issue_overview_unchanged");
  const change = ((card.value - card.previous) * 100) / card.previous;
  return `${change > 0 ? "↑" : change < 0 ? "↓" : "−"} ${Math.abs(change).toFixed(1)}% ${last}`;
}
function changeClass(card) {
  if (card.value == null || !card.previous) return "";
  const delta = card.value - card.previous;
  return (card.key === "overdue" ? delta < 0 : delta > 0)
    ? "overview-positive"
    : delta
      ? "overview-negative"
      : "";
}
const categoryMax = computed(() =>
  Math.max(1, ...(data.value?.categories || []).map((v) => v.total)),
);
const chartBase = computed(() => ({
  aria: { enabled: true },
  textStyle: { color: token.value.colorText },
  tooltip: { trigger: "axis", renderMode: "richText", confine: true },
  grid: { left: 45, right: 18, top: 45, bottom: 35 },
  legend: { textStyle: { color: token.value.colorText } },
  xAxis: {
    type: "category",
    axisLabel: { color: token.value.colorTextSecondary },
  },
  yAxis: {
    type: "value",
    minInterval: 1,
    splitLine: { lineStyle: { color: token.value.colorBorderSecondary } },
    axisLabel: { color: token.value.colorTextSecondary },
  },
}));
const trendOption = computed(() => ({
  ...chartBase.value,
  grid: { left: 12, right: 12, top: 48, bottom: 24, containLabel: true },
  legend: {
    ...chartBase.value.legend,
    type: "scroll",
    top: 0,
    bottom: "auto",
    left: 0,
    right: 0,
  },
  xAxis: {
    ...chartBase.value.xAxis,
    data: data.value?.trend.map((v) => v.date) || [],
    axisLabel: {
      ...chartBase.value.xAxis.axisLabel,
      formatter: (value) => value.slice(5),
      hideOverlap: true,
      margin: 12,
    },
  },
  series: [
    ["created", "#3b6dff"],
    ["completed", "#00b42a"],
    ["closed", "#ff8b00"],
  ].map(([key, color]) => ({
    name: t(`issue_overview_trend_${key}`),
    type: "line",
    data: data.value?.trend.map((v) => v[key]) || [],
    itemStyle: { color },
    areaStyle: key === "created" ? { opacity: 0.07 } : undefined,
  })),
}));
const priorityOption = computed(() => ({
  ...chartBase.value,
  grid: { left: 12, right: 12, top: 30, bottom: 58, containLabel: true },
  legend: {
    ...chartBase.value.legend,
    bottom: 0,
    itemWidth: 10,
    itemHeight: 10,
    selectedMode: false,
    textStyle: { color: token.value.colorTextSecondary, fontSize: 11 },
  },
  tooltip: { ...chartBase.value.tooltip, axisPointer: { type: "shadow" } },
  xAxis: {
    type: "category",
    data:
      data.value?.priorities.map((v) => ({
        value: groupName(v, "priority"),
        textStyle: { color: v.color || token.value.colorTextSecondary },
      })) || [],
    axisLine: { lineStyle: { color: token.value.colorBorderSecondary } },
    axisTick: { show: false },
    axisLabel: {
      interval: 0,
      rotate: 0,
      margin: 14,
      fontSize: 12,
      hideOverlap: true,
    },
  },
  yAxis: {
    type: "value",
    show: false,
    min: 0,
    max: ({ max }) => (max > 0 ? max * 1.15 : 1),
  },
  series: [
    {
      name: t("issue_overview_all"),
      type: "bar",
      barWidth: "42%",
      barMaxWidth: 54,
      data: data.value?.priorities.map((v) => v.total) || [],
      itemStyle: {
        color: token.value.colorFillSecondary,
        borderRadius: [4, 4, 0, 0],
      },
      label: {
        show: true,
        position: "top",
        distance: 10,
        color: token.value.colorText,
        fontSize: 12,
      },
    },
    {
      name: t("issue_overview_finished"),
      type: "bar",
      barGap: "-100%",
      barWidth: "42%",
      barMaxWidth: 54,
      itemStyle: { borderRadius: [4, 4, 0, 0] },
      data:
        data.value?.priorities.map((v) => ({
          value: v.finished,
          itemStyle: { color: v.color || "#3b6dff" },
        })) || [],
    },
  ],
}));
const statusOption = computed(() => ({
  ...chartBase.value,
  tooltip: {
    trigger: "item",
    renderMode: "richText",
    confine: true,
    formatter: "{b}: {c} ({d}%)",
  },
  legend: {
    ...chartBase.value.legend,
    type: "scroll",
    top: 0,
  },
  title: {
    text: number(data.value?.current.total),
    subtext: t("issue_overview_total"),
    left: "center",
    top: "45%",
    textStyle: { color: token.value.colorText, fontSize: 24 },
  },
  xAxis: undefined,
  yAxis: undefined,
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
      data: [
        ["pending", "#ff8b00"],
        ["processing", "#3b6dff"],
        ["completed", "#00b42a"],
        ["closed", "#a6adba"],
        ["unknown", "#8254c9"],
      ]
        .filter(([key]) => key !== "unknown" || data.value?.current.unknown)
        .map(([key, color]) => ({
          name: t(`issue_overview_status_${key}`),
          value: data.value?.current[key] || 0,
          itemStyle: { color },
        })),
    },
  ],
}));
const slaMetrics = computed(() => {
  if (!data.value) return [];
  const d = data.value;
  const maxAverageSeconds = Math.max(
    0,
    ...d.stages.map((stage) => stage.average_seconds ?? 0),
  );
  const metric = (key, name, value) => ({
    key,
    name,
    text: rate(value),
    percent: value,
    color: "#00b42a",
  });
  return [
    metric("sla", t("issue_overview_sla_rate"), d.sla_rate),
    ...d.stages.map((stage) => ({
      key: `stage-${stage.id}`,
      name: t("issue_overview_stage_average", { name: stage.name }),
      text:
        stage.average_seconds == null
          ? "—"
          : slaDuration(stage.average_seconds, t),
      percent:
        stage.average_seconds == null
          ? null
          : (percent(stage.average_seconds, maxAverageSeconds) ?? 0),
      color: "#3b6dff",
    })),
    metric("first", t("issue_overview_first_rate"), d.first_resolution_rate),
    metric(
      "success",
      t("issue_overview_success_rate"),
      d.resolution_success_rate,
    ),
  ];
});
function openDetails(key, id) {
  let ids = [];
  if (key === "pending") ids = [data.value.initial_status_id];
  else if (key === "completed") ids = data.value.completed_status_ids;
  else if (key === "processing")
    ids =
      id == null
        ? data.value.statuses
            .filter((v) => v.key === "processing")
            .map((v) => v.id)
        : [id];
  detailStatuses.value = ids;
  searchDraft.value = "";
  search.value = "";
  detailPage.value = 1;
  drawer.value = { key, id, initial: ids.join(",") };
}
function closeDetails() {
  drawer.value = null;
  itemId.value = "";
}
function applySearch() {
  search.value = searchDraft.value.trim();
  detailPage.value = 1;
  detailReload.value++;
}
function changePage(value) {
  detailPage.value = value.pageSize === detailSize.value ? value.current : 1;
  detailSize.value = value.pageSize;
}
const drawerTitle = computed(() =>
  drawer.value
    ? t("issue_overview_details", {
        name: t(`issue_overview_${drawer.value.key}`),
      }) +
      (search.value || detailStatuses.value.join(",") !== drawer.value.initial
        ? ` · ${t("issue_overview_adjusted")}`
        : "")
    : "",
);
watch(
  [
    () => props.wsId,
    () => JSON.stringify(drawer.value),
    () => detailStatuses.value.join(","),
    search,
    detailPage,
    detailSize,
    detailReload,
  ],
  async (_, __, onCleanup) => {
    const abort = new AbortController();
    onCleanup(() => abort.abort());
    detailData.value = null;
    detailFailed.value = false;
    detailLoading.value = !!drawer.value;
    if (!drawer.value) return;
    try {
      const result = await listIssueOverviewItems(
        props.wsId,
        {
          ...params.value,
          metric: drawer.value.key,
          part_id: drawer.value.id,
          search: search.value,
          detail_status_ids: detailStatuses.value.join(",") || undefined,
          page: detailPage.value,
          size: detailSize.value,
        },
        abort.signal,
      );
      if (!abort.signal.aborted) detailData.value = result;
    } catch {
      if (!abort.signal.aborted) detailFailed.value = true;
    } finally {
      if (!abort.signal.aborted) detailLoading.value = false;
    }
  },
  { immediate: true },
);
const fields = computed(() =>
  [
    ["id", "report_item_id"],
    ["title", "report_item_title"],
    ["status_id", "requirement_filterStatus"],
    ["priority_id", "requirement_priority"],
    ["category_id", "report_category"],
    ["assignee_ids", "report_assignees"],
    ["created_at", "report_item_created_at"],
    ...(drawer.value?.key === "completed"
      ? [
          ["event_status_id", "report_target_status"],
          ["event_at", "report_status_changed_at"],
        ]
      : []),
  ].map(([key, label]) => ({
    key,
    preferenceKey: `builtin:${key}`,
    label: t(label),
    kind: "builtin",
    locked: key === "title",
  })),
);
const columns = computed(() => {
  const labels = new Map(fields.value.map((v) => [v.key, v.label]));
  return (
    preferences.value.fields.length
      ? [
          ...preferences.value.fields,
          ...fields.value.filter(
            (f) => !preferences.value.fields.some((p) => p.key === f.key),
          ),
        ]
      : fields.value
  )
    .filter((v) => v.visible !== false && labels.has(v.key))
    .map((v) => ({ key: v.key, dataIndex: v.key, title: labels.get(v.key) }));
});
const pagination = computed(() => ({
  current: detailPage.value,
  pageSize: detailSize.value,
  total: detailData.value?.total || 0,
  pageSizeOptions: ["20", "40", "80", "100"],
  showSizeChanger: true,
}));
const navigationItems = computed(() =>
  (detailData.value?.list || []).map((v) => ({
    id: v.id,
    workspaceId: props.wsId,
  })),
);
function refreshAfterEdit() {
  reload.value++;
  detailReload.value++;
}
const categoryColumns = computed(() => [
  { key: "name", title: t("report_category") },
  { key: "total", dataIndex: "total", title: t("issue_overview_all") },
  {
    key: "finished",
    dataIndex: "finished",
    title: t("issue_overview_finished"),
  },
  { key: "rate", title: t("issue_overview_finish_rate") },
]);
const memberRows = computed(() =>
  (data.value?.members || []).map((member, index) => ({
    ...member,
    rank: index + 1,
  })),
);
const memberColumns = computed(() => [
  { key: "rank", dataIndex: "rank", title: "#", width: 70 },
  { key: "name", dataIndex: "name", title: t("view_field_assignee") },
  {
    key: "total",
    dataIndex: "total",
    title: t("issue_overview_created"),
    customRender: ({ text }) => number(text),
  },
]);
</script>
<style scoped lang="less">
.overview {
  display: flex;
  flex-direction: column;
  gap: 12px;
  color: var(--app-text);
}
.overview-module-heading,
.overview-card-heading,
.overview-category > div,
.overview-sla-row > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
h3 {
  margin: 0;
  font-size: 15px;
}
.overview-module-heading > span,
.overview-parts {
  font-size: 12px;
  color: var(--app-text-muted);
}
.overview-filters {
  display: flex;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 8px;
  padding: 10px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface);
}
.overview-filters :deep(.sla-date-filter) {
  margin: 0;
}
.overview-filters :deep(.ant-select) {
  min-width: 125px;
}
.overview-detail-search {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  flex-wrap: wrap;
  margin: 16px 0;
}
.overview-detail-search > * {
  width: 300px;
}
.overview-cards {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 12px;
}
.overview-card,
.overview-module {
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 10px;
  padding: 18px;
  min-width: 0;
}
.overview-card-heading {
  font-size: 13px;
}
.overview-card-heading > :deep(.anticon) {
  font-size: 20px;
}
.overview-number {
  display: block;
  margin: 16px 0;
  font-size: 30px;
  line-height: 1.2;
  font-weight: 650;
}
.overview-number small {
  font-size: 12px;
  font-weight: 400;
  color: var(--app-text-muted);
}
.overview-link {
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}
.overview-link:hover {
  text-decoration: underline;
  color: var(--app-primary, #3b6dff);
}
.overview-link:focus-visible {
  outline: 2px solid var(--app-primary, #3b6dff);
  outline-offset: 3px;
}
.overview-parts {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  min-height: 18px;
}
.overview-positive {
  color: #00a870;
}
.overview-negative {
  color: #e5484d;
}
.overview-middle {
  display: grid;
  grid-template-columns: 1.15fr 1fr;
  gap: 12px;
}
.overview-bottom {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}
.overview-module-heading {
  margin-bottom: 14px;
  flex-wrap: wrap;
}
.overview-category {
  margin: 15px 0;
}
.overview-category > div {
  font-size: 12px;
  margin-bottom: 8px;
}
.overview-category > div > span:last-child {
  color: var(--app-text-muted);
}
.overview-bar {
  height: 7px;
  position: relative;
  background: var(--app-surface-muted);
  border-radius: 4px;
  flex: 1;
  min-width: 30px;
}
.overview-bar i {
  position: absolute;
  left: 0;
  top: 0;
  height: 100%;
  background: #3b6dff;
  border-radius: 4px;
}
.overview-bar .overview-bar-total {
  background: var(--app-border);
}
.overview-members {
  display: grid;
  grid-template-columns: 22px minmax(0, 5.5em) minmax(30px, 1fr) max-content;
  align-items: center;
  column-gap: 10px;
  row-gap: 20px;
  font-size: 12px;
  margin: 20px 0;
}
.overview-member {
  display: contents;
}
.overview-member-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.overview-member .overview-bar {
  width: 100%;
}
.overview-member-count {
  color: var(--app-text-muted);
  text-align: right;
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}
.overview-rank {
  width: 22px;
  height: 22px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: var(--app-surface-muted);
  color: var(--app-text-muted);
}
.overview-rank--leading {
  background: #3b6dff;
  color: #fff;
}
.overview-sla-row {
  margin: 24px 0;
}
.overview-sla-row > div {
  font-size: 12px;
}
.overview-sla-row strong {
  margin-left: auto;
}
.overview-title {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.overview-help {
  display: inline-flex;
  align-items: center;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--app-text-muted);
  font-size: 14px;
  cursor: help;
}
.overview-help:focus-visible {
  outline: 2px solid var(--app-primary, #3b6dff);
  outline-offset: 3px;
}
@media (max-width: 1600px) {
  .overview-cards {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
  .overview-bottom {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .overview-bottom > article:last-child {
    grid-column: 1/-1;
  }
}
@media (max-width: 900px) {
  .overview-middle,
  .overview-bottom {
    grid-template-columns: minmax(0, 1fr);
  }
  .overview-bottom > article:last-child {
    grid-column: auto;
  }
  .overview-cards {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .overview-number {
    font-size: 24px;
  }
}
</style>

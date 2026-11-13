<template>
  <article
    class="dashboard-card"
    :class="[
      `dashboard-card--${normalizedType}`,
      { 'dashboard-card--editing': editing },
    ]"
  >
    <header
      class="dashboard-card__head"
      :class="{
        'dashboard-card__head--info': normalizedType === 'project_info',
      }"
    >
      <a-tooltip v-if="editing && canEdit" :title="$t('dashboard_drag_card')">
        <HolderOutlined
          class="dashboard-card__drag"
          :aria-label="$t('dashboard_drag_card')"
          @pointerdown.prevent="emit('drag-start', { event: $event, card })"
        />
      </a-tooltip>
      <h3
        v-if="normalizedType !== 'project_info' || editing"
        class="dashboard-card__title"
      >
        <a-input
          v-if="editing && canEdit"
          v-model:value="editingTitle"
          class="dashboard-card__title-input"
          :maxlength="100"
          :placeholder="$t('dashboard_card_title_placeholder')"
          :aria-label="$t('dashboard_card_title')"
          @blur="commitTitle"
          @press-enter="commitTitle"
        />
        <button
          v-else-if="(isMetric || isChart) && canEdit"
          type="button"
          class="dashboard-card__title-edit"
          @click="emit('action', { key: 'edit', card })"
        >
          {{ card.title || cardTitle }}
        </button>
        <template v-else>{{ card.title || cardTitle }}</template>
      </h3>
      <div class="dashboard-card__actions">
        <a-tooltip
          v-if="isText && canEdit && !editing"
          :title="$t('common_edit')"
        >
          <a-button
            type="text"
            size="small"
            :aria-label="$t('common_edit')"
            @click="emit('action', { key: 'edit', card })"
            ><EditOutlined
          /></a-button>
        </a-tooltip>
        <template v-if="(isChart || isSLAMetric) && !editing">
          <a-button
            type="text"
            size="small"
            :aria-label="$t('common_refresh')"
            :loading="metricLoading"
            @click="loadMetric"
            ><ReloadOutlined
          /></a-button>
          <a-button
            type="text"
            size="small"
            v-if="isChart"
            :aria-label="$t('dashboard_chart_expand')"
            @click="chartExpanded = true"
            ><ExpandOutlined
          /></a-button>
        </template>
        <template v-if="isText && !editing">
          <a-button
            type="text"
            size="small"
            :aria-label="$t('dashboard_expand_text')"
            @click="emit('text-expand', card)"
            ><ExpandOutlined
          /></a-button>
          <a-button
            type="text"
            size="small"
            :aria-label="$t('common_refresh')"
            @click="emit('text-refresh', card)"
            ><ReloadOutlined
          /></a-button>
        </template>
        <a-dropdown v-if="canEdit" :trigger="['click']">
          <a-button type="text" size="small" :aria-label="$t('common_more')"
            ><MoreOutlined
          /></a-button>
          <template #overlay
            ><a-menu @click="({ key }) => emit('action', { key, card })">
              <a-menu-item v-if="normalizedType !== 'project_info'" key="edit"
                ><EditOutlined /> {{ $t("dashboard_edit_card") }}</a-menu-item
              >
              <a-menu-item v-if="isMetric || isChart" key="copy"
                ><CopyOutlined /> {{ $t("dashboard_copy_card") }}</a-menu-item
              >
              <a-menu-item key="delete" danger
                ><DeleteOutlined />
                {{ $t("dashboard_delete_card") }}</a-menu-item
              >
            </a-menu></template
          >
        </a-dropdown>
      </div>
    </header>

    <div
      v-if="isChart"
      class="dashboard-card__chart"
      role="group"
      tabindex="0"
      :aria-label="card.title || cardTitle"
    >
      <a-spin :spinning="metricLoading && !isStatsTable && !isSLARisk">
        <div
          v-if="metricError && !isStatsTable && !isSLARisk"
          class="dashboard-card__chart-message"
        >
          <span>{{ $t("dashboard_chart_failed") }}</span
          ><a-button @click="loadMetric">{{ $t("common_refresh") }}</a-button>
        </div>
        <p v-else-if="String(card.id).startsWith('tmp-')">
          {{ $t("dashboard_chart_save_first") }}
        </p>
        <DashboardSLARisk
          v-else-if="isSLARisk"
          v-model:filters="categoryFilters"
          :result="metricResult"
          :members="members"
          :priorities="priorities"
          :statuses="statuses"
          :failed="metricError"
          :loading="metricLoading"
          @retry="loadMetric"
        />
        <DashboardStatsTable
          v-else-if="isStatsTable"
          v-model:filters="categoryFilters"
          :rows="isWorkload ? metricResult?.workload : metricResult?.categories"
          :workload="isWorkload ? config.workload_dimension : ''"
          :categories="categories"
          :failed="metricError"
          :loading="metricLoading"
          :members="statsMembers"
          :priorities="priorities"
          :color="config.requirement_color"
        />
        <template v-else>
          <DashboardSLADateFilter
            v-if="hasSLADateFilter"
            v-model:filters="categoryFilters"
          />
          <component
            :is="chartComponent"
            :config="config"
            :result="metricResult"
          />
        </template>
      </a-spin>
      <a-modal
        v-model:open="chartExpanded"
        :title="card.title || cardTitle"
        width="90vw"
        :footer="null"
        :destroy-on-close="true"
      >
        <div
          class="dashboard-card__chart-expanded"
          role="group"
          tabindex="0"
          :aria-label="card.title || cardTitle"
        >
          <DashboardSLARisk
            v-if="isSLARisk"
            v-model:filters="categoryFilters"
            :result="metricResult"
            :members="members"
            :priorities="priorities"
            :statuses="statuses"
            :failed="metricError"
            :loading="metricLoading"
            @retry="loadMetric"
          />
          <DashboardStatsTable
            v-else-if="isStatsTable"
            v-model:filters="categoryFilters"
            :rows="
              isWorkload ? metricResult?.workload : metricResult?.categories
            "
            :workload="isWorkload ? config.workload_dimension : ''"
            :categories="categories"
            :members="statsMembers"
            :priorities="priorities"
            :color="config.requirement_color"
            :failed="metricError"
            :loading="metricLoading"
          />
          <template v-else>
            <DashboardSLADateFilter
              v-if="hasSLADateFilter"
              v-model:filters="categoryFilters"
            />
            <component
              :is="chartComponent"
              :config="config"
              :result="metricResult"
            />
          </template>
        </div>
      </a-modal>
    </div>
    <div
      v-else-if="isMetric"
      class="dashboard-card__metric"
      :class="{ 'dashboard-card__metric--sla': isSLAMetric }"
      role="group"
      tabindex="0"
      :aria-label="card.title || cardTitle"
    >
      <a-spin :spinning="metricLoading">
        <strong v-if="metricError" class="dashboard-card__metric-error"
          >—</strong
        >
        <DashboardSLAMetric
          v-else-if="isSLAMetric"
          :config="config"
          :result="metricResult"
        />
        <strong
          v-else
          class="dashboard-card__metric-value"
          :style="{ color: config.color || undefined }"
          >{{ metricValue
          }}<small v-if="config.unit" class="dashboard-card__metric-unit">{{
            config.unit
          }}</small></strong
        >
      </a-spin>
    </div>

    <div
      v-else-if="normalizedType === 'project_info'"
      class="dashboard-card__info"
    >
      <h3 v-if="card.title && !editing" class="dashboard-card__info-heading">
        {{ card.title }}
      </h3>
      <div class="dashboard-card__info-main">
        <span
          class="dashboard-card__workspace-icon"
          :style="{ backgroundColor: workspace?.color || '#e6f0ff' }"
          >{{ (workspace?.name || "-").slice(0, 1) }}</span
        >
        <div class="dashboard-card__info-details">
          <div class="dashboard-card__info-name">
            {{ workspace?.name || "-" }}
          </div>
          <p class="dashboard-card__info-description">
            {{
              workspace?.description ||
              $t("dashboard_project_description_empty")
            }}
          </p>
          <div class="dashboard-card__info-meta">
            <span
              ><InboxOutlined /> {{ $t("dashboard_project_id") }}
              <b>{{ workspace?.id || "-" }}</b></span
            >
            <span
              ><UserOutlined /> {{ $t("dashboard_project_member_count") }}
              <b>{{ memberCount }}</b></span
            >
          </div>
        </div>
      </div>
    </div>

    <RichTextEditor
      v-else
      :model-value="content"
      :content-mode="card.content_mode || 'rich-text'"
      :read-only="true"
      :min-height="80"
      class="dashboard-card__content"
    />

    <button
      v-if="editing && canEdit"
      type="button"
      class="dashboard-card__resize"
      :aria-label="$t('dashboard_resize_card')"
      @pointerdown.prevent.stop="emit('resize-start', { event: $event, card })"
    >
      <span class="dashboard-card__resize-corner" />
    </button>
  </article>
</template>

<script setup>
import { computed, ref, watch, onBeforeUnmount } from "vue";
import { useDashboardI18n } from "@/composables/useDashboardI18n";
import {
  CopyOutlined,
  DeleteOutlined,
  InboxOutlined,
  UserOutlined,
  EditOutlined,
  ExpandOutlined,
  MoreOutlined,
  ReloadOutlined,
  HolderOutlined,
} from "@ant-design/icons-vue";
import RichTextEditor from "@/components/common/RichTextEditor.vue";
import DashboardChart from "./DashboardChart.vue";
import DashboardSLATrend from "./DashboardSLATrend.vue";
import DashboardSLADateFilter from "./DashboardSLADateFilter.vue";
import DashboardSLADimension from "./DashboardSLADimension.vue";
import DashboardSLADistribution from "./DashboardSLADistribution.vue";
import DashboardSLARisk from "./DashboardSLARisk.vue";
import DashboardStatsTable from "./DashboardStatsTable.vue";
import {
  categoryDateRange,
  categoryDateFilter,
  issueMetricTypes,
  slaMetricTypes,
  slaStageCriticalMetricType,
  slaStageMetricType,
  legacySLAStageMetricTypes,
  statsChartTitleKey,
  statusMetricTypes,
} from "./dashboardCharts";
import DashboardSLAMetric from "./DashboardSLAMetric.vue";
import { queryDashboardMetrics, queryDashboardCharts } from "@/api/dashboard";

const props = defineProps({
  wsId: { type: [String, Number], required: true },
  dashboardId: { type: [String, Number], required: true },
  card: { type: Object, required: true },
  workspace: { type: Object, default: null },
  members: { type: Array, default: () => [] },
  categories: { type: Array, default: () => [] },
  workloadMembers: { type: Array, default: () => [] },
  priorities: { type: Array, default: () => [] },
  statuses: { type: Array, default: () => [] },
  filtersRevision: { type: Number, default: 0 },
  editing: { type: Boolean, default: false },
  canEdit: { type: Boolean, default: false },
});

const emit = defineEmits({
  action: (value) => Boolean(value),
  "text-refresh": (value) => Boolean(value),
  "text-expand": (value) => Boolean(value),
  "drag-start": (value) => Boolean(value),
  "resize-start": (value) => Boolean(value),
});

const { t } = useDashboardI18n();
const metricLoading = ref(false);
const chartExpanded = ref(false);
const categoryFilters = ref({ preset: "week" });
onBeforeUnmount(() => {
  requestSequence += 1;
});
const metricError = ref(false);
const metricResult = ref(null);
let requestSequence = 0;
const normalizedType = computed(() => {
  return String(props.card.card_type || "text");
});
const isChart = computed(() => normalizedType.value === "chart");
const isMetric = computed(() => normalizedType.value === "metric");
const isSLAMetric = computed(
  () => isMetric.value && slaMetricTypes.includes(config.value.metric_type),
);
const isText = computed(() =>
  ["text", "announcement"].includes(normalizedType.value),
);
const config = computed(() => props.card.config || {});
const isCategory = computed(
  () => isChart.value && config.value.chart_type === "category_stats",
);
const isWorkload = computed(
  () => isChart.value && config.value.chart_type === "workload_stats",
);
const hasSLADateFilter = computed(
  () =>
    isChart.value &&
    ["sla_dimension", "sla_distribution"].includes(config.value.chart_type),
);
const isSLARisk = computed(
  () => isChart.value && config.value.chart_type === "sla_risk",
);
const isStatsTable = computed(() => isCategory.value || isWorkload.value);
const statsMembers = computed(() => {
  if (!isWorkload.value) return props.members;
  const ids = config.value.assignee_scope_ids || [];
  if (config.value.assignee_scope === "roles")
    return props.members.filter((member) =>
      ids.includes(Number(member.role_id)),
    );
  if (config.value.assignee_scope === "members")
    return props.members.filter((member) =>
      ids.includes(Number(member.user_id)),
    );
  if (config.value.assignee_scope === "self") return props.workloadMembers;
  return props.members;
});
const metricLabel = computed(() => {
  const type = String(config.value.metric_type || "work_item_count");
  if (
    type === slaStageCriticalMetricType &&
    metricResult.value?.sla?.stage_name
  )
    return t("dashboard_sla_stage_critical", {
      name: metricResult.value.sla.stage_name,
    });
  if (type === slaStageMetricType || legacySLAStageMetricTypes.includes(type)) {
    const name = metricResult.value?.sla?.stage_name;
    if (name) return t("dashboard_sla_named_stage_rate", { name });
    if (type === slaStageMetricType)
      return t("dashboard_metric_sla_stage_achievement_rate");
  }
  return metricLabels[type] ? t(metricLabels[type]) : type;
});
const slaChartComponents = {
  sla_trend: DashboardSLATrend,
  sla_dimension: DashboardSLADimension,
  sla_distribution: DashboardSLADistribution,
};
const chartComponent = computed(
  () => slaChartComponents[config.value.chart_type] || DashboardChart,
);
const statsTitle = computed(() => {
  const key = isChart.value ? statsChartTitleKey(config.value) : "";
  return key ? t(key) : "";
});
const cardTitle = computed(() => {
  if (statsTitle.value) return statsTitle.value;
  if (isChart.value) return t("dashboard_chart_statistics");
  if (isMetric.value) return metricLabel.value;
  if (normalizedType.value === "project_info")
    return t("dashboard_card_project_info");
  if (normalizedType.value === "announcement")
    return t("dashboard_card_announcement");
  return t("dashboard_card_text");
});
const editableTitle = computed(() => props.card.title || statsTitle.value);
const editingTitle = ref("");
watch(
  [editableTitle, () => props.editing],
  ([value]) => {
    editingTitle.value = value;
  },
  { immediate: true },
);
function commitTitle() {
  const title = editingTitle.value.trim();
  if (title === editableTitle.value) return;
  emit("action", {
    key: "title-change",
    card: { ...props.card, title },
  });
}
const metricValue = computed(() => {
  const result = metricResult.value;
  if (result === null || result === undefined) return "—";
  if (typeof result === "number" || typeof result === "string") return result;
  return result.value ?? "—";
});
const content = computed(() => props.card.content || "");
const memberCount = computed(() => Number(props.workspace?.member_count || 0));

const metricLabels = {
  ...Object.fromEntries(
    [...statusMetricTypes, ...issueMetricTypes, ...slaMetricTypes].map(
      (type) => [type, `dashboard_metric_${type}`],
    ),
  ),
  work_item_count: "dashboard_metric_work_item_count",
  created_requirement_count: "dashboard_metric_created_requirement_count",
  created_defect_count: "dashboard_metric_created_defect_count",
  completed_requirement_count: "dashboard_metric_completed_requirement_count",
  completed_defect_count: "dashboard_metric_completed_defect_count",
  estimated_work_hours: "dashboard_metric_estimated_work_hours",
  completed_work_hours: "dashboard_metric_completed_work_hours",
  remaining_work_hours: "dashboard_metric_remaining_work_hours",
  overtime_work_hours: "dashboard_metric_overtime_work_hours",
};

watch(
  () => [
    props.wsId,
    props.dashboardId,
    props.card.id,
    config.value.workload_dimension,
  ],
  () => {
    categoryFilters.value = {
      preset:
        isWorkload.value && config.value.workload_dimension === "member"
          ? "month30"
          : "week",
    };
  },
  { immediate: true },
);

watch(
  () => props.filtersRevision,
  () => {
    if (isSLARisk.value && categoryFilters.value.page > 1) {
      categoryFilters.value = { ...categoryFilters.value, page: 1 };
    }
  },
);

watch(
  [
    () => props.wsId,
    () => props.dashboardId,
    () => props.card.id,
    () => props.card.revision,
    () => props.filtersRevision,
    isMetric,
    isChart,
    categoryFilters,
  ],
  () => loadMetric(),
  { immediate: true },
);

async function loadMetric() {
  const sequence = ++requestSequence;
  if (
    (!isMetric.value && !isChart.value) ||
    !props.card.id ||
    String(props.card.id).startsWith("tmp-")
  ) {
    metricResult.value = null;
    metricLoading.value = false;
    metricError.value = false;
    return;
  }
  metricLoading.value = true;
  metricResult.value = null;
  metricError.value = false;
  try {
    const query = isChart.value ? queryDashboardCharts : queryDashboardMetrics;
    const result = await query(
      props.wsId,
      props.dashboardId,
      {
        card_id: props.card.id,
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
        ...(isCategory.value ? { category_filter: statsQueryFilter() } : {}),
        ...(hasSLADateFilter.value
          ? { category_filter: categoryDateFilter(categoryFilters.value) }
          : {}),
        ...(isWorkload.value ? { workload_filter: statsQueryFilter() } : {}),
        ...(isSLARisk.value
          ? {
              sla_risk_filter: {
                ...statsQueryFilter(),
                page: categoryFilters.value.page || 1,
              },
            }
          : {}),
      },
      { silent: true },
    );
    if (sequence !== requestSequence) return;
    const risk = result.sla_risk;
    if (risk) {
      const lastPage = Math.max(1, Math.ceil(risk.total / risk.size));
      if (risk.page > lastPage) {
        categoryFilters.value = { ...categoryFilters.value, page: lastPage };
        return;
      }
    }
    metricResult.value = result;
  } catch {
    if (sequence === requestSequence) metricError.value = true;
  } finally {
    if (sequence === requestSequence) metricLoading.value = false;
  }
}

function statsQueryFilter() {
  const { preset, range, assignee, priority, category } = categoryFilters.value;
  const [start, end] = categoryDateRange(preset, range);
  return {
    from: start.unix(),
    to: end.add(1, "day").unix(),
    assignee_id: assignee,
    priority_id: priority,
    ...(isWorkload.value ? { category_id: category } : {}),
  };
}
</script>

<style scoped lang="less">
.dashboard-card {
  position: relative;
  display: flex;
  height: 100%;
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  background: #fff;
  color: #15284b;
}
.dashboard-card--editing {
  border-color: #1677ff;
}
.dashboard-card__head {
  min-height: 52px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 16px 20px 8px;
  flex-shrink: 0;
}
.dashboard-card__title {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  color: #15284b;
  font-size: 18px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.dashboard-card__title-input {
  width: min(260px, 100%);
  :deep(input) {
    font-size: 16px;
    font-weight: 600;
  }
}
.dashboard-card__drag {
  margin-left: 8px;
  color: #8b99af;
  cursor: grab;
}
.dashboard-card__drag:active {
  cursor: grabbing;
}
.dashboard-card__title-edit {
  display: block;
  max-width: 100%;
  overflow: hidden;
  padding: 0;
  border: 0;
  color: inherit;
  font: inherit;
  background: transparent;
  cursor: pointer;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.dashboard-card__actions {
  display: flex;
  align-items: center;
  margin-left: auto;
  color: #8b99af;
  :deep(.ant-btn) {
    color: inherit;
  }
  :deep(.ant-btn-dangerous) {
    color: #ff4d4f;
  }
}
.dashboard-card__head--info {
  position: absolute;
  right: 0;
  top: 0;
  z-index: 1;
  padding: 12px;
  opacity: 0;
}
.dashboard-card:hover .dashboard-card__head--info,
.dashboard-card:focus-within .dashboard-card__head--info,
.dashboard-card--editing .dashboard-card__head--info {
  opacity: 1;
}
/* In edit mode the project info header joins the normal flow so the title
   input lines up with other cards instead of floating in the corner. */
.dashboard-card--editing .dashboard-card__head--info {
  position: static;
  width: 100%;
}
.dashboard-card__chart,
.dashboard-card__chart-expanded,
.dashboard-card__metric {
  overflow: auto;
}
.dashboard-card__chart {
  flex: 1;
  min-height: 0;
  padding: 0 20px 18px;
  :deep(.ant-spin-nested-loading),
  :deep(.ant-spin-container) {
    height: 100%;
  }
}
.dashboard-card__chart-expanded {
  height: 70vh;
}
.dashboard-card__chart-message {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  height: 100%;
  color: #8c8c8c;
}
.dashboard-card__metric {
  min-height: 0;
  display: flex;
  flex: 1;
  align-items: safe center;
  justify-content: safe center;
  padding: 0 20px 24px;
}
.dashboard-card__metric--sla {
  align-items: stretch;
  :deep(.ant-spin-nested-loading) {
    width: 100%;
  }
  :deep(.ant-spin-container) {
    height: 100%;
  }
}
.dashboard-card__metric-value,
.dashboard-card__metric-error {
  color: #15284b;
  font-size: clamp(36px, 4vw, 64px);
  font-weight: 600;
  line-height: 1.2;
  overflow-wrap: anywhere;
}
.dashboard-card__metric-error {
  color: #bfbfbf;
}
.dashboard-card__metric-unit {
  margin-left: 8px;
  font-size: 16px;
  font-weight: 400;
}
.dashboard-card__info {
  display: flex;
  flex-direction: column;
  gap: 12px;
  overflow: auto;
  padding: 32px;
}
/* Card title set via the drawer/edit mode; shown above the workspace info.
   Single line with ellipsis so long titles cannot push the card content out. */
.dashboard-card__info-heading {
  margin: 0;
  overflow: hidden;
  color: #15284b;
  font-size: 18px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.dashboard-card__info-main {
  display: flex;
  align-items: flex-start;
  gap: 20px;
  min-height: 0;
}
.dashboard-card__workspace-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 52px;
  height: 52px;
  border-radius: 10px;
  color: #fff;
  font-size: 28px;
}
.dashboard-card__info-details {
  min-width: 0;
  flex: 1;
}
.dashboard-card__info-name {
  color: #15284b;
  font-size: 22px;
  font-weight: 600;
  overflow-wrap: anywhere;
}
.dashboard-card__info-description {
  margin: 6px 0 22px;
  color: #8b99af;
  font-size: 14px;
  line-height: 1.6;
  white-space: pre-wrap;
}
.dashboard-card__info-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 24px;
  font-size: 14px;
  span {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .anticon {
    color: #8b99af;
  }
  b {
    font-weight: 400;
  }
}
.dashboard-card__content {
  min-height: 0;
  flex: 1;
  overflow: auto;
  padding: 0 20px 20px;
}
.dashboard-card__content :deep(.editor-shell) {
  min-height: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
}
.dashboard-card__content :deep(.editor-body) {
  padding: 0;
  color: #15284b;
}
.dashboard-card__content :deep(.tiptap) {
  padding: 0;
  color: #15284b;
  font-size: 14px;
  line-height: 1.8;
}
.dashboard-card__resize {
  position: absolute;
  right: 4px;
  bottom: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  padding: 0;
  border: 0;
  background: transparent;
  cursor: nwse-resize;
}
.dashboard-card__resize-corner {
  width: 15px;
  height: 15px;
  border-right: 3px solid #bfbfbf;
  border-bottom: 3px solid #bfbfbf;
  border-radius: 0 0 3px;
}
@media (max-width: 900px) {
  .dashboard-card__head {
    padding: 12px 12px 6px;
  }
  .dashboard-card__title {
    font-size: 16px;
  }
  .dashboard-card__content {
    padding: 0 12px 12px;
  }
  .dashboard-card__info {
    padding: 22px 16px;
    gap: 12px;
  }
}
</style>

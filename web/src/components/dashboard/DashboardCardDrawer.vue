<template>
  <a-drawer
    root-class-name="dashboard-card-editor"
    :open="open"
    placement="right"
    :width="
      showMetricPreview
        ? 'min(1200px, 96vw)'
        : !form.card_type
          ? 'min(920px, 96vw)'
          : 'min(720px, 96vw)'
    "
    :body-style="showMetricPreview ? { padding: 0 } : {}"
    :destroy-on-close="true"
    @close="close"
  >
    <template #title>
      <button
        v-if="showBack"
        type="button"
        class="dashboard-card-drawer__back"
        @click="backToTypes"
      >
        <LeftOutlined />
        <span class="dashboard-card-drawer__back-label">{{
          $t("common_back")
        }}</span>
        <span class="dashboard-card-drawer__back-title">{{
          cardTypeTitle
        }}</span>
      </button>
      <span v-else class="dashboard-card-drawer__head-title">{{
        drawerTitle
      }}</span>
    </template>
    <DashboardCardTypes
      v-if="!form.card_type"
      v-model:tab="chartTab"
      :has-announcement="hasAnnouncement"
      :can-add-sla="authStore.hasPermission('requirement:view')"
      @select="selectType"
    />
    <template v-else>
      <div
        :class="{ 'dashboard-card-drawer__metric-layout': showMetricPreview }"
      >
        <aside v-if="showMetricPreview" class="dashboard-card-drawer__preview">
          <h3>{{ form.title || cardTypeTitle }}</h3>
          <p class="dashboard-card-drawer__preview-metric">
            {{ $t("dashboard_metric_type") }}
            <a-tag>{{ selectedMetric.label }}</a-tag>
          </p>
          <div class="dashboard-card-drawer__preview-card">
            <a-spin
              :spinning="previewLoading"
              :class="{ 'dashboard-card-drawer__preview-sla': isSLAMetric }"
            >
              <DashboardSLAMetric
                v-if="isSLAMetric"
                :config="form.config"
                :result="previewResult"
              />
              <div
                v-else
                class="dashboard-card-drawer__preview-value"
                :style="{ color: form.config.color || undefined }"
              >
                {{ previewResult?.value ?? "—" }}
                <small>{{ form.config.unit }}</small>
              </div>
            </a-spin>
          </div>
          <a-button :loading="previewLoading" @click="previewMetric">{{
            $t("dashboard_metric_preview")
          }}</a-button>
        </aside>
        <div class="dashboard-card-drawer__form">
          <template v-if="isSLAChart">
            <a-form layout="vertical">
              <a-form-item :label="$t('dashboard_card_title')">
                <a-input
                  v-model:value="form.title"
                  :maxlength="100"
                  :placeholder="$t('dashboard_card_title_placeholder')"
                />
              </a-form-item>
              <a-form-item :label="$t('dashboard_metric_scope')">
                <a-checkbox :checked="true" disabled>{{
                  t("dashboard_scope_requirement")
                }}</a-checkbox>
              </a-form-item>
              <DashboardSLADimensionConfig
                v-if="isSLADimension"
                :config="form.config"
                :ws-id="wsId"
                :priorities="filterSections?.requirement?.priorities || []"
                @dimension="changeSLADimension"
              />
              <a-form-item
                :label="$t('dashboard_sla_stages')"
                :extra="
                  slaStagesOptional
                    ? $t('dashboard_sla_all_stages_help')
                    : undefined
                "
              >
                <a-select
                  v-model:value="form.config.sla_stage_ids"
                  mode="multiple"
                  :options="slaStageOptions"
                  option-filter-prop="label"
                  :loading="slaLoading"
                  :disabled="slaLoading || slaError"
                  :placeholder="
                    $t(
                      slaStagesOptional
                        ? 'dashboard_sla_all_stages'
                        : 'dashboard_sla_select_stages',
                    )
                  "
                  :aria-label="$t('dashboard_sla_stages')"
                  :allow-clear="slaStagesOptional"
                />
                <a-alert
                  v-if="slaError"
                  type="error"
                  :message="$t('dashboard_chart_failed')"
                >
                  <template #action>
                    <a-button size="small" @click="loadSLAOptions">{{
                      $t("issue_retry")
                    }}</a-button>
                  </template>
                </a-alert>
                <a-alert
                  v-else-if="!slaEnabled"
                  type="info"
                  :message="$t('dashboard_sla_trend_disabled')"
                />
              </a-form-item>
              <template v-if="slaRiskThresholdStages.length">
                <p class="tip-info">
                  {{
                    $t("dashboard_sla_risk_threshold_help", {
                      minutes: defaultSLARiskThresholdMinutes,
                    })
                  }}
                </p>
                <a-form-item
                  v-for="stage in slaRiskThresholdStages"
                  :key="stage.value"
                  :label="stage.label"
                >
                  <a-input-number
                    :value="
                      form.config.sla_risk_stage_threshold_minutes?.[
                        stage.value
                      ]
                    "
                    :min="1"
                    :max="maxSLARiskThresholdMinutes"
                    :precision="0"
                    :placeholder="String(defaultSLARiskThresholdMinutes)"
                    :aria-label="`${stage.label} · ${$t('dashboard_sla_risk_threshold_minutes')}`"
                    @update:value="setSLARiskThreshold(stage.value, $event)"
                  />
                </a-form-item>
              </template>
              <a-form-item
                v-for="stage in slaStageColorOptions"
                :key="stage.id"
                :label="stage.label"
              >
                <DashboardColorPicker
                  :value="stage.color"
                  :label="stage.label"
                  @update:value="setSLAStageColor(stage.id, $event)"
                />
              </a-form-item>
              <a-form-item
                v-if="isSLATrend"
                :label="$t('dashboard_sla_trend_window')"
                :extra="$t('dashboard_sla_trend_dates')"
              >
                <a-input-number
                  v-model:value="form.config.recent_days"
                  :min="1"
                  :max="365"
                  :precision="0"
                />
              </a-form-item>
              <a-form-item
                v-if="isSLATrend"
                :label="$t('dashboard_sla_trend_target')"
                :extra="$t('dashboard_sla_trend_target_help')"
              >
                <a-input-number
                  v-model:value="form.config.sla_target_rate"
                  :min="0"
                  :max="100"
                  addon-after="%"
                />
              </a-form-item>
              <a-form-item
                v-for="series in isSLATrend ? slaTrendColors : []"
                :key="series.key"
                :label="$t(series.label)"
              >
                <div class="dashboard-card-drawer__sla-swatches">
                  <button
                    v-for="color in chartSwatches"
                    :key="color"
                    type="button"
                    :style="{ backgroundColor: color }"
                    :aria-label="`${$t(series.label)} ${color}`"
                    :aria-pressed="form.config[series.key] === color"
                    @click="form.config[series.key] = color"
                  />
                </div>
              </a-form-item>
            </a-form>
          </template>
          <DashboardChartConfig
            v-else-if="form.card_type === 'chart'"
            :config="form.config"
            v-model:title="form.title"
            :sections="filterSections"
            :members="members"
            :roles="roles"
          />
          <template v-else-if="form.card_type === 'metric'">
            <div>
              <div class="dashboard-card-drawer__section-label">
                {{ $t("dashboard_metric_scope") }}
              </div>
              <a-radio-group
                v-model:value="form.config.data_scope"
                :disabled="isSLAMetric"
              >
                <a-radio v-if="!isIssueWorkspace" value="all">{{
                  $t("dashboard_scope_all")
                }}</a-radio>
                <a-radio value="requirement">{{
                  t("dashboard_scope_requirement")
                }}</a-radio>
                <a-radio v-if="!isIssueWorkspace" value="defect">{{
                  $t("dashboard_scope_defect")
                }}</a-radio>
              </a-radio-group>
              <div class="dashboard-card-drawer__section-label">
                {{ $t("dashboard_metric_type") }}
              </div>
              <a-select
                :value="selectedMetric"
                :options="metricOptions"
                :loading="isSLAMetric && slaLoading"
                label-in-value
                @change="({ value }) => changeMetricType(value)"
                class="dashboard-card-drawer__metric-select"
              />
              <p v-if="slaMetricDescription" class="tip-info">
                {{ slaMetricDescription }}
              </p>
              <template v-if="isSLAAchievement">
                <div class="dashboard-card-drawer__section-label">
                  {{ $t("dashboard_sla_stages") }}
                </div>
                <a-select
                  v-model:value="form.config.sla_stage_ids"
                  mode="multiple"
                  :options="slaStageOptions"
                  :loading="slaLoading"
                  :disabled="slaLoading || slaError"
                  :placeholder="$t('dashboard_sla_select_stages')"
                  :aria-label="$t('dashboard_sla_stages')"
                  class="dashboard-card-drawer__metric-select"
                />
              </template>
              <template v-if="isSLAMetric">
                <a-alert
                  v-if="slaError"
                  type="error"
                  :message="$t('dashboard_chart_failed')"
                >
                  <template #action>
                    <a-button size="small" @click="loadSLAOptions">{{
                      $t("common_refresh")
                    }}</a-button>
                  </template>
                </a-alert>
                <a-alert
                  v-else-if="!slaEnabled"
                  type="warning"
                  :message="$t('dashboard_sla_disabled')"
                />
              </template>
              <div class="dashboard-card-drawer__section-label">
                {{ $t("dashboard_metric_window") }}
              </div>
              <a-select
                v-model:value="
                  form.config[isSLAMetric ? 'sla_recent_days' : 'recent_days']
                "
                :options="recentWindowOptions"
                :aria-label="$t('dashboard_metric_window')"
                class="dashboard-card-drawer__metric-select"
                @change="onMetricWindowChange"
              />
              <div
                v-if="isSLAMetric"
                class="dashboard-card-drawer__sla-comparison"
              >
                <a-checkbox
                  v-model:checked="form.config.sla_show_comparison"
                  :disabled="!form.config.sla_recent_days"
                >
                  {{ $t("dashboard_sla_show_comparison") }}
                </a-checkbox>
                <p v-if="!form.config.sla_recent_days" class="tip-info">
                  {{ $t("dashboard_sla_comparison_window") }}
                </p>
              </div>
              <div class="dashboard-card-drawer__section-label">
                {{ $t("dashboard_data_conditions") }}
              </div>
              <button
                type="button"
                class="dashboard-card-drawer__condition-summary"
                @click="metricFiltersOpen = true"
              >
                <span>{{
                  conditionCount
                    ? $t("dashboard_condition_count", { count: conditionCount })
                    : $t("dashboard_no_conditions")
                }}</span
                ><EditOutlined />
              </button>
              <a-modal
                v-model:open="metricFiltersOpen"
                :title="$t('dashboard_data_conditions')"
                width="min(1000px, 94vw)"
                :footer="null"
              >
                <DashboardFilterPanel
                  v-if="metricFiltersOpen"
                  ref="metricFilterRef"
                  :model-value="form.config.filters"
                  class="dashboard-card-drawer__filter"
                  :statuses="statuses"
                  :priorities="priorities"
                  :members="members"
                  :versions="versions"
                  :categories="categories"
                  :fields="fields"
                  :sections="filterSections"
                  :default-collapsed="false"
                  :show-types="false"
                  @apply="confirmMetricFilters"
                />
              </a-modal>
              <div class="dashboard-card-drawer__section-label">
                {{ $t("dashboard_card_style") }}
              </div>
            </div>
            <div class="dashboard-card-drawer__style-form">
              <label for="dashboard-metric-title">{{
                $t("dashboard_card_title")
              }}</label>
              <a-input
                id="dashboard-metric-title"
                v-model:value="form.title"
                :maxlength="100"
                :placeholder="$t('dashboard_card_title_placeholder')"
              />
              <label for="dashboard-metric-unit">{{
                $t("dashboard_metric_unit")
              }}</label>
              <a-input
                id="dashboard-metric-unit"
                v-model:value="form.config.unit"
                :maxlength="20"
                :placeholder="$t('dashboard_metric_unit_placeholder')"
              />
              <template v-if="isSLAMetric">
                <label for="dashboard-sla-line-color">{{
                  $t("dashboard_sla_line_color")
                }}</label>
                <input
                  id="dashboard-sla-line-color"
                  v-model="form.config.sla_line_color"
                  type="color"
                />
              </template>
              <label for="dashboard-metric-color">{{
                $t("dashboard_metric_color")
              }}</label>
              <DashboardColorPicker
                v-model:value="form.config.color"
                input-id="dashboard-metric-color"
                :label="$t('dashboard_metric_color')"
              />
            </div>
          </template>
          <template
            v-else-if="
              form.card_type === 'text' || form.card_type === 'announcement'
            "
          >
            <div class="dashboard-card-drawer__label">
              {{ $t("dashboard_card_title") }}
            </div>
            <a-input
              v-model:value="form.title"
              :maxlength="100"
              :placeholder="$t('dashboard_card_title_placeholder')"
            />
            <div class="dashboard-card-drawer__label">
              {{
                form.card_type === "announcement"
                  ? $t("dashboard_announcement_content")
                  : $t("dashboard_text_content")
              }}
            </div>
            <RichTextEditor
              v-model="form.content"
              v-model:content-mode="form.content_mode"
              v-model:image-file-ids="form.file_ids"
              :ws-id="wsId"
              :image-uploader="uploadImage"
              :file-uploader="uploadFile"
              :video-uploader="uploadVideo"
              :allow-markdown="false"
              :plain="true"
              :min-height="260"
              :placeholder="$t('dashboard_content_placeholder')"
            />
          </template>

          <template v-else-if="form.card_type === 'project_info'">
            <div class="dashboard-card-drawer__label">
              {{ $t("dashboard_card_title") }}
            </div>
            <a-input
              v-model:value="form.title"
              :maxlength="100"
              :placeholder="$t('dashboard_card_title_placeholder')"
            />
            <p class="dashboard-card-drawer__info-hint">
              {{ $t("dashboard_project_info_hint") }}
            </p>
            <div class="dashboard-card-drawer__info-preview">
              <span
                class="dashboard-card-drawer__info-icon"
                :style="{ backgroundColor: workspace.color || '#1677ff' }"
                >{{ (workspace.name || "-").slice(0, 1) }}</span
              >
              <div class="dashboard-card-drawer__info-copy">
                <strong>{{ workspace.name || "-" }}</strong>
                <p>
                  {{
                    workspace.description ||
                    $t("dashboard_project_description_empty")
                  }}
                </p>
                <div>
                  <span
                    ><InboxOutlined /> {{ $t("dashboard_project_id") }}
                    <b>{{ workspace.id || "-" }}</b></span
                  >
                  <span
                    ><UserOutlined /> {{ $t("dashboard_project_member_count") }}
                    <b>{{ workspace.member_count || 0 }}</b></span
                  >
                </div>
              </div>
            </div>
          </template>
        </div>
      </div>
    </template>

    <template #footer>
      <div class="dashboard-card-drawer__footer">
        <a-button @click="close">{{ $t("common_cancel") }}</a-button>
        <a-button type="primary" :disabled="!form.card_type" @click="save">
          {{ $t("common_save") }}
        </a-button>
      </div>
    </template>
  </a-drawer>
</template>

<script setup>
import { computed, ref, watch, onBeforeUnmount } from "vue";
import { useDashboardWorkItems } from "@/composables/useDashboardWorkItems";
import { useAuthStore } from "@/stores/auth";
import { useDashboardI18n } from "@/composables/useDashboardI18n";
import {
  EditOutlined,
  InboxOutlined,
  LeftOutlined,
  UserOutlined,
} from "@ant-design/icons-vue";
import RichTextEditor from "@/components/common/RichTextEditor.vue";
import DashboardChartConfig from "./DashboardChartConfig.vue";
import DashboardSLADimensionConfig from "./DashboardSLADimensionConfig.vue";
import DashboardSLAMetric from "./DashboardSLAMetric.vue";
import DashboardColorPicker from "./DashboardColorPicker.vue";
import {
  chartEntries,
  slaStageColor,
  chartSwatches,
  defaultChartConfig,
  issueMetricTypes,
  slaMetricTypes,
  fixedSLAMetricTypes,
  legacySLAStageMetricTypes,
  slaCountMetricTypes,
  slaStageCriticalMetricType,
  slaStageMetricType,
  statsChartTitleKey,
  statusMetricTypes,
} from "./dashboardCharts";
import { message } from "ant-design-vue";
import DashboardCardTypes from "./DashboardCardTypes.vue";
import DashboardFilterPanel from "./DashboardFilterPanel.vue";
import {
  queryDashboardMetrics,
  getDashboardSLAOptions,
  uploadDashboardFile,
  uploadDashboardImage,
  uploadDashboardVideo,
} from "@/api/dashboard";

const props = defineProps({
  open: { type: Boolean, default: false },
  wsId: { type: [String, Number], required: true },
  dashboardId: { type: [String, Number], required: true },
  mode: { type: String, default: "create" },
  card: { type: Object, default: null },
  nextLayout: { type: Object, default: null },
  hasAnnouncement: { type: Boolean, default: false },
  announcement: { type: Object, default: null },
  pendingFileIds: { type: Array, default: () => [] },
  announcementPendingFileIds: { type: Array, default: () => [] },
  workspace: { type: Object, default: () => ({}) },
  statuses: { type: Array, default: () => [] },
  priorities: { type: Array, default: () => [] },
  members: { type: Array, default: () => [] },
  roles: { type: Array, default: () => [] },
  versions: { type: Array, default: () => [] },
  categories: { type: Array, default: () => [] },
  fields: { type: Array, default: () => [] },
  filterSections: { type: Object, default: null },
});

const emit = defineEmits({
  "update:open": (value) => typeof value === "boolean",
  saved: (value) => Boolean(value),
});

const { t } = useDashboardI18n();
const authStore = useAuthStore();
const { isIssueWorkspace, dataScope, normalizeFilters } =
  useDashboardWorkItems();
const form = ref(emptyCard());
const chartTab = ref("basic");
const metricFilterRef = ref(null);
const metricFiltersOpen = ref(false);
const conditionCount = computed(() => {
  function count(group) {
    return (group?.conditions || []).reduce(
      (total, node) => total + (node.field ? 1 : count(node)),
      0,
    );
  }
  return ["common", "requirement", "defect"].reduce(
    (total, key) => total + count(form.value.config.filters?.[key]),
    0,
  );
});
function confirmMetricFilters() {
  form.value.config.filters =
    metricFilterRef.value?.getFilters() || form.value.config.filters;
  metricFiltersOpen.value = false;
}
const previewLoading = ref(false);
const previewResult = ref(null);
const isSLAMetric = computed(() =>
  slaMetricTypes.includes(form.value.config?.metric_type),
);
const showMetricPreview = computed(() => form.value.card_type === "metric");
const isSLATrend = computed(
  () => form.value.config?.chart_type === "sla_trend",
);
const isSLADimension = computed(
  () => form.value.config?.chart_type === "sla_dimension",
);
const isSLADistribution = computed(
  () => form.value.config?.chart_type === "sla_distribution",
);
const maxSLARiskThresholdMinutes = 365 * 24 * 60;
const defaultSLARiskThresholdMinutes = computed(
  () => form.value.config.sla_risk_threshold_minutes ?? 60,
);
const isSLARisk = computed(() => form.value.config?.chart_type === "sla_risk");
const slaStagesOptional = computed(() => isSLARisk.value || isSLATrend.value);
const isSLAChart = computed(
  () =>
    isSLATrend.value ||
    isSLADimension.value ||
    isSLADistribution.value ||
    isSLARisk.value,
);
const needsSLAStages = computed(() => isSLAMetric.value || isSLAChart.value);
let dimensionStageDrafts = {};
const slaTrendColors = [
  { key: "sla_rate_color", label: "dashboard_sla_trend_rate_color" },
  { key: "sla_overdue_color", label: "dashboard_sla_trend_overdue_color" },
];
const isSLAAchievement = computed(
  () => form.value.config.metric_type === "sla_achievement_rate",
);
const slaMetricDescription = computed(() => {
  if (!isSLAMetric.value) return "";
  const type = form.value.config.metric_type;
  const key = legacySLAStageMetricTypes.includes(type)
    ? slaStageMetricType
    : type;
  return t(`dashboard_metric_${key}_description`);
});
const slaOptions = ref([]);
const slaLoading = ref(false);
const slaError = ref(false);
const slaEnabled = ref(true);
let slaOptionsSequence = 0;
const slaStageOptions = computed(() => {
  const options = slaOptions.value.map((stage) => ({
    value: Number(stage.id),
    label: stage.name,
  }));
  for (const id of form.value.config.sla_stage_ids || []) {
    if (!options.some((option) => option.value === id))
      options.push({
        value: id,
        label: t("dashboard_sla_archived_stage"),
        disabled: !isSLADistribution.value && !isSLARisk.value,
      });
  }
  return options;
});
const slaRiskThresholdStages = computed(() => {
  if (!isSLARisk.value || slaLoading.value || slaError.value) return [];
  const ids = form.value.config.sla_stage_ids || [];
  return ids.length
    ? slaStageOptions.value.filter((stage) => ids.includes(stage.value))
    : slaStageOptions.value;
});

function setSLARiskThreshold(id, minutes) {
  const thresholds = { ...form.value.config.sla_risk_stage_threshold_minutes };
  if (minutes == null) delete thresholds[id];
  else thresholds[id] = minutes;
  form.value.config.sla_risk_stage_threshold_minutes = thresholds;
}

const slaStageColorOptions = computed(() => {
  const config = form.value.config;
  if (
    !isSLADistribution.value &&
    !(isSLADimension.value && config.sla_dimension === "priority")
  )
    return [];
  const labelKey = isSLADistribution.value
    ? "dashboard_sla_stage_overdue"
    : "dashboard_sla_named_stage_rate";
  return (config.sla_stage_ids || []).map((id) => ({
    id,
    label: t(labelKey, {
      name:
        slaStageOptions.value.find((option) => option.value === id)?.label ||
        t("dashboard_sla_archived_stage"),
    }),
    color: slaStageColor(config, id),
  }));
});

function setSLAStageColor(id, color) {
  form.value.config.sla_stage_colors = {
    ...form.value.config.sla_stage_colors,
    [id]: color,
  };
}

let previewSequence = 0;
// 统计图按具体图表类型显示卡片名，避免落到未定义的 dashboard_card_chart。
const chartTitleKeys = {
  status_bar: "dashboard_chart_status",
  priority_bar: "dashboard_chart_priority",
  completion_line: "dashboard_chart_completion",
  status_pie: "dashboard_chart_status_pie",
  daily_created_line: "dashboard_chart_daily_created",
  assignee_rank_bar: "dashboard_chart_ranking",
  iteration_burndown: "dashboard_chart_burndown",
  category_stats: "dashboard_chart_category",
  workload_stats: "dashboard_chart_workload",
  sla_trend: "dashboard_chart_sla_trend",
  sla_dimension: "dashboard_chart_sla_dimension",
  sla_distribution: "dashboard_chart_sla_distribution",
  sla_risk: "dashboard_chart_sla_risk",
};
const cardTypeTitle = computed(() => {
  const type = form.value.card_type;
  if (!type) return "";
  if (type === "chart")
    return t(
      chartTitleKeys[form.value.config?.chart_type] || "dashboard_card_chart",
    );
  return t(`dashboard_card_${isSLAMetric.value ? "sla_metric" : type}`);
});
const drawerTitle = computed(() =>
  form.value.card_type ? cardTypeTitle.value : t("dashboard_add_card"),
);
const showBack = computed(
  () => props.mode === "create" && Boolean(form.value.card_type),
);
function backToTypes() {
  form.value.card_type = "";
}
const metricDefinitions = [
  ...statusMetricTypes.map((value) => ({
    value,
    label: `dashboard_metric_${value}`,
    scopes: ["all", "requirement", "defect"],
  })),
  {
    value: "work_item_count",
    label: "dashboard_metric_work_item_count",
    scopes: ["all", "requirement", "defect"],
  },
  {
    value: "created_requirement_count",
    label: "dashboard_metric_created_requirement_count",
    scopes: ["requirement"],
  },
  {
    value: "created_defect_count",
    label: "dashboard_metric_created_defect_count",
    scopes: ["defect"],
  },
  {
    value: "completed_requirement_count",
    label: "dashboard_metric_completed_requirement_count",
    scopes: ["requirement"],
  },
  {
    value: "overdue_requirement_count",
    label: "dashboard_metric_overdue_requirement_count",
    scopes: ["requirement"],
  },
  {
    value: "unresolved_defect_count",
    label: "dashboard_metric_unresolved_defect_count",
    scopes: ["defect"],
  },
  {
    value: "completed_defect_count",
    label: "dashboard_metric_completed_defect_count",
    scopes: ["defect"],
  },
  {
    value: "estimated_work_hours",
    label: "dashboard_metric_estimated_work_hours",
    scopes: ["requirement"],
  },
  {
    value: "completed_work_hours",
    label: "dashboard_metric_completed_work_hours",
    scopes: ["requirement"],
  },
  {
    value: "remaining_work_hours",
    label: "dashboard_metric_remaining_work_hours",
    scopes: ["requirement"],
  },
  {
    value: "overtime_work_hours",
    label: "dashboard_metric_overtime_work_hours",
    scopes: ["requirement"],
  },
];
const metricOptions = computed(() => {
  if (isSLAMetric.value) {
    const fixedOptions = fixedSLAMetricTypes.map((value) => ({
      value,
      label: t(`dashboard_metric_${value}`),
    }));
    const stageOptions = slaOptions.value.map((stage) => ({
      value: `${slaStageMetricType}:${stage.id}`,
      label: t("dashboard_sla_named_stage_rate", { name: stage.name }),
    }));
    return [fixedOptions[0], ...stageOptions, ...fixedOptions.slice(1)];
  }
  return metricDefinitions
    .filter((item) =>
      item.scopes.includes(form.value.config?.data_scope || "all"),
    )
    .map((item) => ({ value: item.value, label: t(item.label) }));
});
const isSavedIssueMetric = computed(
  () =>
    isIssueWorkspace.value &&
    form.value.config.data_scope === "requirement" &&
    issueMetricTypes.includes(form.value.config.metric_type),
);
const selectedMetric = computed(() => {
  const type = form.value.config.metric_type;
  const stageId = form.value.config.sla_stage_ids?.[0];
  if (type === slaStageCriticalMetricType) {
    const stage = slaOptions.value.find((item) => Number(item.id) === stageId);
    return {
      value: type,
      label: stage
        ? t("dashboard_sla_stage_critical", { name: stage.name })
        : t(`dashboard_metric_${type}`),
    };
  }
  if (type === slaStageMetricType) {
    const value = `${type}:${stageId}`;
    return (
      metricOptions.value.find((item) => item.value === value) || {
        value,
        label: t(`dashboard_metric_${type}`),
      }
    );
  }
  const value = type;
  // Saved cards can keep an issue metric that is no longer offered in the picker.
  if (isSavedIssueMetric.value)
    return { value, label: t(`dashboard_metric_${value}`) };
  return metricOptions.value.find((item) => item.value === value) || { value };
});
const metricTitle = computed(() =>
  Array.from(selectedMetric.value.label || "")
    .slice(0, 100)
    .join(""),
);

// Rolling "recent N days" windows for metric cards; 0 means no time limit.
const recentWindowOptions = [0, 7, 30, 90, 180, 365].map((days) => ({
  value: days,
  label: days
    ? t(`dashboard_metric_window_${days}`)
    : t("dashboard_metric_window_off"),
}));

watch(
  () => props.open,
  (open) => {
    previewSequence += 1;
    previewResult.value = null;
    previewLoading.value = false;
    metricFiltersOpen.value = false;
    dimensionStageDrafts = {};
    if (!open) return;
    chartTab.value = "basic";
    const type = props.card?.card_type;
    if (chartEntries.some((entry) => entry.key === type)) {
      selectType(type);
      return;
    }
    const card = normalizeCard(props.card || emptyCard());
    const titleKey =
      card.card_type === "chart" && statsChartTitleKey(card.config);
    if (!card.title && titleKey) card.title = t(titleKey);
    form.value = card;
  },
);
watch(
  () => form.value.config?.data_scope,
  () => {
    if (isSLAMetric.value || selectedMetric.value.label) return;
    form.value.config.metric_type =
      metricOptions.value[0]?.value || "work_item_count";
  },
);

function emptyCard() {
  return {
    id: "",
    card_type: "",
    title: "",
    config: {
      metric_type: "work_item_count",
      data_scope: dataScope("all"),
      recent_days: 0,
      unit: "",
      color: "#1f1f1f",
      filters: emptyFilters(),
    },
    content: "",
    content_mode: "rich-text",
    file_ids: [],
    ...(props.nextLayout || { x: 0, y: 0, w: 4, h: 3 }),
  };
}

function normalizeCard(source, pendingFileIds = props.pendingFileIds) {
  const type = normalizeType(source.card_type);
  const announcement = type === "announcement" ? props.announcement || {} : {};
  const metricType = source.config?.metric_type;
  const dimensionConfig = {};
  if (
    ["sla_dimension", "sla_distribution"].includes(source.config?.chart_type)
  ) {
    const config = source.config;
    if (config.chart_type === "sla_dimension")
      dimensionConfig.recent_days = undefined;
    dimensionConfig.sla_stage_colors = Object.fromEntries(
      (config.sla_stage_ids || []).map((id) => [id, slaStageColor(config, id)]),
    );
  }
  return {
    ...emptyCard(),
    ...source,
    card_type: type,
    title: source.title || "",
    content: source.content ?? announcement.content ?? "",
    content_mode:
      source.content_mode || announcement.content_mode || "rich-text",
    file_ids: copyPendingFileIds(pendingFileIds),
    config: {
      ...(type === "chart"
        ? defaultChartConfig(
            chartEntries.find(
              (entry) => entry.kind === source.config?.chart_type,
            ),
          )
        : {
            metric_type: "work_item_count",
            data_scope: dataScope("all"),
            recent_days: 0,
            unit: "",
            color: "#1f1f1f",
            filters: emptyFilters(),
          }),
      ...(source.config || {}),
      ...dimensionConfig,
      ...(source.config?.sla_distribution_colors
        ? {
            sla_distribution_colors: {
              ...source.config.sla_distribution_colors,
            },
          }
        : {}),
      ...(source.config?.chart_type === "sla_trend"
        ? { sla_target_rate: source.config.sla_target_rate ?? null }
        : {}),
      data_scope: dataScope(source.config?.data_scope ?? "all"),
      ...(type === "chart"
        ? {
            status_ids: chartRange(source.config?.status_ids, "statuses"),
            priority_ids:
              source.config?.chart_type === "sla_dimension"
                ? [...(source.config.priority_ids || [])]
                : chartRange(source.config?.priority_ids, "priorities"),
            custom_order: [...(source.config?.custom_order || [])],
          }
        : {}),
      ...(source.config?.sla_stage_ids
        ? { sla_stage_ids: [...source.config.sla_stage_ids] }
        : {}),
      ...(type === "metric" && slaMetricTypes.includes(metricType)
        ? {
            metric_type: legacySLAStageMetricTypes.includes(metricType)
              ? slaStageMetricType
              : metricType,
            unit:
              source.config?.unit ??
              (slaCountMetricTypes.includes(metricType)
                ? t("dashboard_sla_count_unit")
                : ""),
            recent_days: 0,
            sla_recent_days: source.config.sla_recent_days ?? 0,
            sla_show_comparison:
              source.config.sla_recent_days > 0 &&
              source.config.sla_show_comparison === true,
            ...(["sla_overdue_count", "sla_overdue_rate"].includes(metricType)
              ? { sla_stage_ids: [] }
              : {}),
          }
        : {}),
      filters: normalizeFilters(source.config?.filters),
    },
  };
}

function chartRange(ids, field) {
  ids = ids || [];
  if (!isIssueWorkspace.value) return [...ids];
  const allowed = new Set(
    (props.filterSections?.requirement?.[field] || []).map((item) =>
      Number(item.id),
    ),
  );
  if (field === "priorities") allowed.add(0);
  return ids.filter((id) => allowed.has(id));
}

function normalizeType(value) {
  return ["chart", "metric", "project_info", "announcement", "text"].includes(
    value,
  )
    ? value
    : "";
}

function selectType(type) {
  if (type === "announcement" && props.hasAnnouncement) return;
  dimensionStageDrafts = {};
  const card = emptyCard();
  const chartEntry = chartEntries.find((entry) => entry.key === type);
  card.card_type = chartEntry
    ? "chart"
    : type === "sla_metric"
      ? "metric"
      : type;
  if (type === "sla_metric") {
    card.w = 3;
    Object.assign(card.config, {
      metric_type: "sla_achievement_rate",
      data_scope: "requirement",
      sla_recent_days: 0,
      sla_show_comparison: false,
      unit: "%",
      color: "#1677ff",
      sla_stage_ids: [],
      sla_line_color: "#1677ff",
    });
    card.title = t("dashboard_metric_sla_achievement_rate");
  }
  if (chartEntry) {
    card.config = defaultChartConfig(chartEntry);
    card.config.data_scope = dataScope(card.config.data_scope);
    card.title = t(`dashboard_chart_${chartEntry.key}`);
    card.w = chartEntry.issueOnly ? 12 : 6;
    card.h = type === "workload" || type === "sla_risk" ? 7 : 5;
  }
  if (type === "announcement") {
    card.content = props.announcement?.content || "";
    card.content_mode = props.announcement?.content_mode || "rich-text";
    card.file_ids = copyPendingFileIds(props.announcementPendingFileIds);
  }
  form.value = card;
}

function copyPendingFileIds(fileIds) {
  return [
    ...new Set(
      (fileIds || [])
        .map(Number)
        .filter((fileId) => Number.isInteger(fileId) && fileId > 0),
    ),
  ];
}

function changeMetricType(value) {
  const previousTitle = metricTitle.value;
  const config = form.value.config;
  const [type, stageId] = value.split(":");
  if (type === slaStageMetricType) {
    config.metric_type = type;
    config.sla_stage_ids = [Number(stageId)];
  } else {
    const previousType = config.metric_type;
    config.metric_type = value;
    if (!isSLAMetric.value) return;
    if (value !== "sla_achievement_rate") config.sla_stage_ids = [];
    else if (previousType !== value)
      config.sla_stage_ids = slaOptions.value.map((stage) => Number(stage.id));
  }
  if (
    !config.unit ||
    config.unit === "%" ||
    config.unit === t("dashboard_sla_count_unit")
  )
    config.unit = slaCountMetricTypes.includes(config.metric_type)
      ? t("dashboard_sla_count_unit")
      : "%";
  if (!form.value.title.trim() || form.value.title === previousTitle)
    form.value.title = metricTitle.value;
}
async function loadSLAOptions() {
  const sequence = ++slaOptionsSequence;
  slaLoading.value = true;
  slaError.value = false;
  try {
    const result = await getDashboardSLAOptions(props.wsId, props.dashboardId);
    if (sequence === slaOptionsSequence) {
      slaOptions.value = result.stages || [];
      slaEnabled.value = result.enabled;
      if (isSLAAchievement.value && !form.value.config.sla_stage_ids?.length)
        form.value.config.sla_stage_ids = slaOptions.value.map((stage) =>
          Number(stage.id),
        );
    }
  } catch {
    if (sequence === slaOptionsSequence) slaError.value = true;
  } finally {
    if (sequence === slaOptionsSequence) slaLoading.value = false;
  }
}
watch(
  [() => props.open, () => props.wsId, () => props.dashboardId, needsSLAStages],
  ([open, , , sla]) => {
    slaOptionsSequence++;
    slaOptions.value = [];
    slaEnabled.value = true;
    slaError.value = false;
    slaLoading.value = false;
    if (open && sla) void loadSLAOptions();
  },
);
watch(
  () => form.value.config,
  () => {
    previewSequence++;
    previewLoading.value = false;
    previewResult.value = null;
  },
  { deep: true },
);
onBeforeUnmount(() => {
  slaOptionsSequence++;
  previewSequence++;
});

function emptyFilters() {
  return {
    common: { logic: "and", conditions: [] },
    requirement: { logic: "and", conditions: [] },
    defect: { logic: "and", conditions: [] },
  };
}

function uploadImage(file) {
  return uploadDashboardImage(props.wsId, props.dashboardId, file);
}

function uploadFile(file) {
  return uploadDashboardFile(props.wsId, props.dashboardId, file);
}

function uploadVideo(file) {
  return uploadDashboardVideo(props.wsId, props.dashboardId, file);
}

function close() {
  emit("update:open", false);
}

function changeSLADimension(dimension) {
  const config = form.value.config;
  dimensionStageDrafts[config.sla_dimension] = config.sla_stage_ids;
  config.sla_dimension = dimension;
  config.sla_stage_ids = dimensionStageDrafts[dimension] || [];
}

function onMetricWindowChange(days) {
  if (isSLAMetric.value && days === 0)
    form.value.config.sla_show_comparison = false;
}

function validateSLAConfig() {
  const config = form.value.config;
  if (
    isSLARisk.value &&
    Object.values(config.sla_risk_stage_threshold_minutes || {}).some(
      (minutes) =>
        !Number.isInteger(minutes) ||
        minutes < 1 ||
        minutes > maxSLARiskThresholdMinutes,
    )
  ) {
    message.warning(
      t("dashboard_sla_risk_threshold_invalid", {
        max: maxSLARiskThresholdMinutes,
      }),
    );
    return false;
  }
  if (
    (isSLAChart.value ||
      isSLAAchievement.value ||
      config.metric_type === slaStageMetricType) &&
    ((!slaStagesOptional.value && !config.sla_stage_ids?.length) ||
      (config.metric_type !== slaStageMetricType &&
        (slaLoading.value || slaError.value)))
  ) {
    message.warning(t("dashboard_sla_select_stages"));
    return false;
  }
  if (
    isSLATrend.value &&
    (!Number.isInteger(config.recent_days) ||
      config.recent_days < 1 ||
      config.recent_days > 365 ||
      (config.sla_target_rate != null &&
        (config.sla_target_rate < 0 || config.sla_target_rate > 100)))
  ) {
    message.warning(t("dashboard_sla_trend_invalid"));
    return false;
  }
  return true;
}

function save() {
  if (!validateSLAConfig()) return;
  if (
    ["category_stats", "workload_stats"].includes(
      form.value.config.chart_type,
    ) &&
    ["members", "roles"].includes(form.value.config.assignee_scope) &&
    !form.value.config.assignee_scope_ids?.length
  ) {
    message.warning(
      t(
        form.value.config.assignee_scope === "roles"
          ? "dashboard_visibility_roles_required"
          : "dashboard_visibility_members_required",
      ),
    );
    return;
  }
  if (
    form.value.card_type === "chart" &&
    !isSLAChart.value &&
    (!Number.isInteger(form.value.config.days_from) ||
      !Number.isInteger(form.value.config.days_to) ||
      form.value.config.days_to < 0 ||
      form.value.config.days_from < form.value.config.days_to ||
      form.value.config.days_from - form.value.config.days_to > 365)
  ) {
    message.warning(t("dashboard_chart_invalid_dates"));
    return;
  }
  if (form.value.card_type === "metric") {
    form.value.config.filters =
      metricFilterRef.value?.getFilters() || form.value.config.filters;
  }
  const value = normalizeCard(form.value, form.value.file_ids);
  if (value.card_type === "metric") {
    value.config.filters = metricFilters(
      value.config.filters,
      value.config.data_scope,
    );
  }
  emit("saved", value);
  close();
}

function metricFilters(filters, dataScope) {
  const normalized = normalizeFilters(filters);
  const result = {
    work_item_types: normalized.work_item_types,
    common: filterGroupOrNull(normalized.common),
    requirement:
      dataScope === "defect" ? null : filterGroupOrNull(normalized.requirement),
    defect:
      dataScope === "requirement" ? null : filterGroupOrNull(normalized.defect),
  };
  return result.work_item_types.length ||
    result.common ||
    result.requirement ||
    result.defect
    ? result
    : null;
}

async function previewMetric() {
  if (!validateSLAConfig()) return;
  const sequence = ++previewSequence;
  previewLoading.value = true;
  previewResult.value = null;
  const filters =
    metricFilterRef.value?.getFilters() || form.value.config.filters;
  try {
    const result = await queryDashboardMetrics(props.wsId, props.dashboardId, {
      config: {
        ...form.value.config,
        filters: metricFilters(filters, form.value.config.data_scope),
      },
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
    });
    if (sequence === previewSequence) previewResult.value = result;
  } catch {
    // The shared request interceptor displays the failure.
  } finally {
    if (sequence === previewSequence) previewLoading.value = false;
  }
}

function filterGroupOrNull(group) {
  return group?.conditions?.length ? group : null;
}
</script>

<style scoped lang="less">
:global(.dashboard-card-editor .ant-drawer-header-title) {
  flex-direction: row-reverse;
}
:global(.dashboard-card-editor .ant-drawer-close) {
  margin-right: 0;
}
.dashboard-card-drawer__sla-swatches {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;

  button {
    width: 22px;
    height: 22px;
    border: 1px solid transparent;
    border-radius: 50%;
    cursor: pointer;
  }

  button[aria-pressed="true"] {
    border: 2px solid #1f2937;
  }
}
.dashboard-card-drawer__condition-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  padding: 20px 16px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #f7f8fa;
  color: #15284b;
  text-align: left;
  cursor: pointer;
  .anticon {
    color: #8b99af;
  }
}

.dashboard-card-drawer__metric-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  min-height: 100%;
}
.dashboard-card-drawer__preview {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  padding: 30px;
  background: #f5f8ff;
  color: #15284b;
}
.dashboard-card-drawer__preview h3 {
  font-size: 18px;
  color: #15284b;
  margin: 0 0 20px;
}
.dashboard-card-drawer__preview-metric {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}
.dashboard-card-drawer__preview-card {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  min-height: 220px;
  margin: 100px 0 24px;
  background: #fff;
  border: 1px solid #e0e7ff;
  border-radius: 14px;
}
.dashboard-card-drawer__preview-value {
  padding: 20px;
  font-size: 64px;
  font-weight: 600;
  overflow-wrap: anywhere;
}
.dashboard-card-drawer__preview-sla {
  width: 100%;
  padding: 20px;
}
.dashboard-card-drawer__preview-value small {
  font-size: 16px;
  font-weight: 400;
}
.dashboard-card-drawer__metric-layout > .dashboard-card-drawer__form {
  padding: 8px 24px 24px;
  min-width: 0;
}
.dashboard-card-drawer__back {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin-left: -6px;
  padding: 4px 8px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: #15284b;
  font-size: 16px;
  font-weight: 600;
  line-height: 1.4;
  cursor: pointer;
  .anticon {
    color: #8c8c8c;
    font-size: 13px;
  }
  &:hover,
  &:focus-visible {
    background: #f5f7fa;
    outline: none;
    .anticon {
      color: #1677ff;
    }
  }
  &-label {
    color: #8c8c8c;
    font-weight: 400;
  }
  &-title {
    color: #15284b;
  }
}
.dashboard-card-drawer__head-title {
  color: #15284b;
  font-size: 16px;
  font-weight: 600;
}
.dashboard-card-drawer__label {
  margin: 20px 0 8px;
  color: #15284b;
  font-weight: 500;
}
.dashboard-card-drawer__section-label {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 28px 0 18px;
  color: #15284b;
  font-size: 16px;
  font-weight: 600;
  &::before {
    content: "";
    width: 3px;
    height: 15px;
    border-radius: 2px;
    background: #1677ff;
  }
}
.dashboard-card-drawer__metric-select {
  min-width: 190px;
}
.dashboard-card-drawer__sla-comparison {
  margin-top: 12px;
}
.dashboard-card-drawer__filter {
  margin-top: 16px;
}
.dashboard-card-drawer__style-form {
  display: grid;
  grid-template-columns: 60px minmax(0, 1fr);
  gap: 20px 12px;
  align-items: start;
  padding-top: 12px;
  > label {
    padding-top: 5px;
    color: #8c8c8c;
  }
}
.dashboard-card-drawer__info-hint {
  margin: 22px 0 0;
  padding: 12px;
  border-radius: 6px;
  color: #595959;
  background: #fafafa;
  font-size: 13px;
  line-height: 1.6;
}
.dashboard-card-drawer__info-preview {
  display: flex;
  gap: 16px;
  margin-top: 16px;
  padding: 32px;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  background: #fff;
}
.dashboard-card-drawer__info-icon {
  display: grid;
  flex: 0 0 56px;
  place-items: center;
  width: 56px;
  height: 56px;
  border-radius: 10px;
  color: #fff;
  font-size: 28px;
}
.dashboard-card-drawer__info-copy {
  min-width: 0;
  flex: 1;
  strong {
    display: block;
    color: #15284b;
    font-size: 18px;
  }
  p {
    margin: 6px 0 18px;
    color: #8c8c8c;
  }
  > div {
    display: flex;
    flex-wrap: wrap;
    gap: 20px;
    color: #596780;
    font-size: 13px;
  }
  b {
    color: #15284b;
    font-weight: 500;
  }
}
.dashboard-card-drawer__footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
.tip-info {
  margin-top: 6px;
  color: #8c8c8c;
}
@media (max-width: 800px) {
  .dashboard-card-drawer__metric-layout {
    grid-template-columns: 1fr;
  }
  .dashboard-card-drawer__preview {
    padding: 20px;
  }
  .dashboard-card-drawer__preview-card {
    margin-top: 12px;
    min-height: 140px;
  }
}
</style>

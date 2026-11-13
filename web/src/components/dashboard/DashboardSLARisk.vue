<template>
  <section class="sla-risk">
    <div v-if="filters" class="sla-risk__filters">
      <a-radio-group
        :value="filters.preset"
        :aria-label="t('dashboard_chart_time_range')"
        size="small"
        button-style="solid"
        @update:value="selectPreset"
      >
        <a-radio-button
          v-for="preset in presets"
          :key="preset.value"
          :value="preset.value"
        >
          {{ t(preset.label) }}
        </a-radio-button>
      </a-radio-group>
      <a-range-picker
        v-if="filters.preset === 'custom'"
        :value="filters.range"
        :allow-clear="false"
        value-format="YYYY-MM-DD"
        size="small"
        @change="selectRange"
      />
      <a-select
        :value="filters.assignee"
        :options="memberOptions"
        :placeholder="t('dashboard_category_all_members')"
        :aria-label="t('view_field_assignee')"
        option-filter-prop="label"
        allow-clear
        show-search
        size="small"
        @change="changeFilters({ assignee: $event })"
      />
      <a-select
        :value="filters.priority"
        :options="priorityOptions"
        :placeholder="t('dashboard_category_all_priorities')"
        :aria-label="t('view_field_priority')"
        option-filter-prop="label"
        allow-clear
        show-search
        size="small"
        @change="changeFilters({ priority: $event })"
      />
    </div>
    <a-alert v-if="failed" type="error" :message="t('dashboard_chart_failed')">
      <template #action>
        <a-button size="small" @click="emit('retry')">{{
          t("issue_retry")
        }}</a-button>
      </template>
    </a-alert>
    <a-spin v-else :spinning="loading" class="sla-risk__body">
      <a-empty
        v-if="risk?.enabled === false"
        :image="Empty.PRESENTED_IMAGE_SIMPLE"
        :description="t('dashboard_sla_trend_disabled')"
      />
      <template v-else-if="risk">
        <div class="sla-risk__summary">
          <div v-for="item in summary" :key="item.key">
            <span>{{ item.label || t(item.key) }}</span>
            <strong>{{ item.value }}</strong>
            <small>{{ item.help }}</small>
          </div>
        </div>
        <a-table
          row-key="id"
          size="small"
          :columns="columns"
          :data-source="risk.list"
          :pagination="pagination"
          :scroll="{ x: 565 + stageColumnWidth * (risk.stages?.length || 0) }"
          :locale="{ emptyText: t('dashboard_chart_empty') }"
          @change="changePage"
        >
          <template #bodyCell="{ column, record, text }">
            <template v-if="column.key === 'id'">#{{ record.id }}</template>
            <template v-else-if="column.key === 'assignees'">{{
              assignees(record)
            }}</template>
            <template v-else-if="column.key === 'priority'">
              <a-tag
                v-if="priorityById.get(record.priority_id)"
                :color="priorityById.get(record.priority_id).color"
              >
                {{ metadataLabel(priorityById.get(record.priority_id)) }}
              </a-tag>
              <span v-else>—</span>
            </template>
            <template v-else-if="column.key === 'status'">
              <a-tag
                v-if="statusById.get(record.status_id)"
                :color="statusById.get(record.status_id).color"
              >
                {{ metadataLabel(statusById.get(record.status_id)) }}
              </a-tag>
              <span v-else>—</span>
            </template>
            <template v-else-if="column.stageIndex != null">
              <a-tag
                v-if="text"
                class="sla-risk__balance-tag"
                :color="stageRiskColors[text.risk]"
              >
                {{ stageStatusText(text) }}
              </a-tag>
              <template v-else>{{
                duration(record.stage_times?.[column.stageIndex])
              }}</template>
            </template>
          </template>
        </a-table>
      </template>
    </a-spin>
  </section>
</template>

<script setup>
import { computed } from "vue";
import { Empty, message } from "ant-design-vue";
import { useDashboardI18n } from "@/composables/useDashboardI18n";
import { slaDuration, slaStageText } from "@/utils/sla";
import { categoryDateRange } from "./dashboardCharts";

const props = defineProps({
  result: { type: Object, default: null },
  filters: { type: Object, default: null },
  members: { type: Array, default: () => [] },
  priorities: { type: Array, default: () => [] },
  statuses: { type: Array, default: () => [] },
  loading: Boolean,
  failed: Boolean,
});
const emit = defineEmits(["update:filters", "retry"]);
const { t } = useDashboardI18n();
const risk = computed(() => props.result?.sla_risk);
const presets = [
  { value: "today", label: "common_today" },
  { value: "week", label: "dashboard_category_week" },
  { value: "month30", label: "dashboard_category_month30" },
  { value: "month", label: "dashboard_category_month" },
  { value: "custom", label: "dashboard_category_custom" },
];
const metadataLabel = (item) => item?.label || item?.name || "—";
const memberOptions = computed(() =>
  props.members.map((member) => ({
    value: Number(member.user_id),
    label: member.user_name || member.nickname || String(member.user_id),
  })),
);
const memberNames = computed(
  () =>
    new Map(memberOptions.value.map((member) => [member.value, member.label])),
);
const priorityOptions = computed(() => [
  ...props.priorities.map((item) => ({
    value: Number(item.id),
    label: metadataLabel(item),
  })),
  { value: 0, label: t("dashboard_chart_unset") },
]);
const priorityById = computed(
  () => new Map(props.priorities.map((item) => [Number(item.id), item])),
);
const statusById = computed(
  () => new Map(props.statuses.map((item) => [Number(item.id), item])),
);
const duration = (seconds) => (seconds == null ? "—" : slaDuration(seconds, t));
const stageRiskColors = { overdue: "error", critical: "warning" };
const stageColumnWidth = 240;

function stageStatusText(stage) {
  if (
    stage.calculation_state === "calendar_unavailable" ||
    stage.state === "interrupted" ||
    stage.state === "not_started"
  ) {
    return slaStageText(stage, t);
  }
  if (stage.risk === "overdue")
    return t("sla_overdueBy", { time: duration(stage.overdue_seconds) });
  if (stage.state === "completed")
    return t("dashboard_sla_risk_elapsed", {
      time: duration(stage.elapsed_seconds),
    });
  return t(
    stage.risk === "critical"
      ? "dashboard_sla_risk_critical_by"
      : "dashboard_sla_risk_remaining",
    { time: duration(stage.remaining_seconds) },
  );
}

const count = (value) => t("dashboard_sla_dimension_count", { count: value });
const summary = computed(() => {
  const data = risk.value;
  if (!data) return [];
  return [
    {
      key: "dashboard_metric_sla_overdue_count",
      value: count(data.overdue),
      help: t("dashboard_sla_selected_stage_overdue_help"),
    },
    {
      key: "dashboard_metric_sla_overdue_rate",
      value:
        data.overdue_rate == null
          ? "—"
          : `${Number(data.overdue_rate.toFixed(1))}%`,
      help:
        t("dashboard_sla_dimension_count", { count: data.started }) +
        " · " +
        t("dashboard_sla_risk_calculable"),
    },
    ...(data.stages || []).map((stage) => ({
      key: stage.id,
      label: t("dashboard_sla_stage_critical", { name: stage.name }),
      value: count(stage.critical),
      help: t("dashboard_sla_risk_threshold", {
        minutes: (stage.threshold_seconds ?? data.threshold_seconds) / 60,
      }),
    })),
  ];
});
const columns = computed(() => [
  { key: "id", title: t("dashboard_sla_risk_item_id"), width: 100 },
  {
    key: "title",
    dataIndex: "title",
    title: t("view_field_title"),
    ellipsis: true,
  },
  { key: "assignees", title: t("view_field_assignee"), width: 130 },
  { key: "priority", title: t("view_field_priority"), width: 90 },
  { key: "status", title: t("view_field_status"), width: 100 },
  ...(risk.value?.stages || []).map((stage, index) => ({
    key: `stage-${stage.id}`,
    dataIndex: ["stage_results", index],
    stageIndex: index,
    title: stage.name,
    width: stageColumnWidth,
    ellipsis: true,
  })),
]);
const pagination = computed(() =>
  props.filters && risk.value
    ? {
        current: risk.value.page,
        pageSize: risk.value.size,
        total: risk.value.total,
        showSizeChanger: false,
        hideOnSinglePage: true,
      }
    : false,
);

function assignees(row) {
  return (
    (row.assignee_ids || [])
      .map((id) => memberNames.value.get(id) || `#${id}`)
      .join(", ") || "—"
  );
}

function changeFilters(patch) {
  emit("update:filters", { ...props.filters, ...patch, page: 1 });
}

function selectPreset(preset) {
  if (preset === "custom") {
    const [from, to] = categoryDateRange(
      props.filters.preset,
      props.filters.range,
    );
    changeFilters({
      preset,
      range: [from.format("YYYY-MM-DD"), to.format("YYYY-MM-DD")],
    });
    return;
  }
  changeFilters({ preset, range: undefined });
}

function selectRange(range) {
  if (!range?.length) return;
  const [from, to] = categoryDateRange("custom", range);
  if (to.diff(from, "day") >= 366) {
    message.warning(t("dashboard_sla_risk_invalid_range"));
    return;
  }
  changeFilters({ range });
}

function changePage({ current }) {
  if (props.filters && current !== props.filters.page)
    emit("update:filters", { ...props.filters, page: current });
}
</script>

<style scoped lang="less">
.sla-risk {
  width: 100%;
  &__body {
    min-height: 90px;
  }
  &__balance-tag {
    max-width: 100%;
    margin-inline-end: 0;
    white-space: normal;
    overflow-wrap: anywhere;
  }
  &__filters {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin-bottom: 10px;
  }
  &__summary {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 10px;
    margin-bottom: 10px;

    div {
      display: grid;
      gap: 5px;
      padding: 12px;
      border: 1px solid #e9edf2;
      border-radius: 10px;
    }
    strong {
      color: #172236;
      font-size: 22px;
    }
    small {
      color: #63738a;
    }
  }
}
@media (max-width: 750px) {
  .sla-risk__summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>

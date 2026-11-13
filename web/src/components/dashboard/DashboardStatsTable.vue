<template>
  <section
    class="dashboard-stats-table"
    :style="{ '--category-accent': color }"
  >
    <div class="dashboard-stats-table__filters">
      <a-config-provider :theme="{ token: { colorPrimary: color } }">
        <a-radio-group
          :value="filters.preset"
          button-style="solid"
          :aria-label="t('dashboard_chart_time_range')"
          role="radiogroup"
          size="small"
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
      </a-config-provider>
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
        show-search
        allow-clear
        size="small"
        @change="updateFilters({ assignee: $event })"
      />
      <a-select
        v-if="workload"
        :value="filters.category"
        :options="categoryOptions"
        :placeholder="t('dashboard_workload_all_categories')"
        :aria-label="t('dashboard_category_name')"
        option-filter-prop="label"
        show-search
        allow-clear
        size="small"
        @change="updateFilters({ category: $event })"
      />
      <a-select
        :value="filters.priority"
        :options="priorityOptions"
        :placeholder="t('dashboard_category_all_priorities')"
        :aria-label="t('view_field_priority')"
        option-filter-prop="label"
        show-search
        allow-clear
        size="small"
        @change="updateFilters({ priority: $event })"
      />
    </div>
    <a-table
      size="small"
      row-key="id"
      :columns="columns"
      :data-source="workload ? workloadRows : tree"
      :pagination="false"
      :scroll="{ x: 920 }"
      :loading="loading"
      :locale="{
        emptyText: t(
          failed ? 'dashboard_chart_failed' : 'dashboard_chart_empty',
        ),
      }"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="workload && column.dataIndex === 'employee'">
          <a-avatar
            v-if="record.member"
            size="small"
            :src="record.member.user_avatar"
          >
            {{ record.employee?.slice(0, 1) }}
          </a-avatar>
          {{ record.employee }}
        </template>
        <a-tag v-if="workload && column.dataIndex === 'role' && record.role">{{
          record.role
        }}</a-tag>
      </template>
      <template v-if="workload && total" #summary>
        <a-table-summary-row class="dashboard-stats-table__total">
          <a-table-summary-cell
            v-for="(column, index) in columns"
            :key="column.dataIndex"
            :index="index"
            :align="column.align"
          >
            {{
              index === 0
                ? t("dashboard_workload_total")
                : (column.customRender?.({ record: total }) ??
                  total[column.dataIndex] ??
                  "—")
            }}
          </a-table-summary-cell>
        </a-table-summary-row>
      </template>
    </a-table>
  </section>
</template>

<script setup>
import { computed } from "vue";
import { formatLocalDate } from "@/utils/date";
import dayjs from "dayjs";
import { message } from "ant-design-vue";
import { useI18n } from "vue-i18n";
import { categoryDateRange } from "./dashboardCharts";

const props = defineProps({
  filters: { type: Object, required: true },
  rows: { type: Array, default: () => [] },
  members: { type: Array, default: () => [] },
  priorities: { type: Array, default: () => [] },
  color: { type: String, default: "#1664ff" },
  categories: { type: Array, default: () => [] },
  workload: { type: String, default: "" },
  failed: Boolean,
  loading: Boolean,
});
const emit = defineEmits(["update:filters"]);
const { t, locale } = useI18n();
const presets = computed(() => [
  ...(props.workload === "date"
    ? []
    : [{ value: "today", label: "common_today" }]),
  { value: "week", label: "dashboard_category_week" },
  { value: "month30", label: "dashboard_category_month30" },
  { value: "month", label: "dashboard_category_month" },
  { value: "custom", label: "dashboard_category_custom" },
]);
const memberOptions = computed(() =>
  props.members.map((member) => ({
    value: Number(member.user_id),
    label: member.user_name,
  })),
);
const categoryOptions = computed(() => [
  ...props.categories.map((category) => ({
    value: Number(category.id),
    label: category.name,
  })),
  { value: 0, label: t("sla_uncategorized") },
]);
const total = computed(() => props.rows.find((row) => row.group === "total"));
const workloadRows = computed(() => {
  const members = new Map(
    props.members.map((member) => [String(member.user_id), member]),
  );
  const rows = props.rows.filter((row) => row.group !== "total");
  if (props.workload === "date")
    return rows
      .sort((a, b) => b.group.localeCompare(a.group))
      .map((row) => ({
        ...row,
        id: row.group,
        date: formatLocalDate(dayjs(row.group).unix()),
      }));
  return rows
    .sort((a, b) => b.created - a.created || Number(a.group) - Number(b.group))
    .map((row) => {
      const member = members.get(row.group);
      return {
        ...row,
        id: row.group,
        member,
        employee:
          member?.user_name ||
          t(
            row.group === "0"
              ? "dashboard_workload_unassigned"
              : "dashboard_workload_unknown_member",
          ),
        role: member?.role_name,
      };
    });
});

const priorityOptions = computed(() => [
  ...props.priorities.map((priority) => ({
    value: Number(priority.id),
    label: priority.label,
  })),
  { value: 0, label: t("dashboard_chart_unset") },
]);

function updateFilters(changes) {
  emit("update:filters", { ...props.filters, ...changes });
}

function selectPreset(preset) {
  if (props.filters.preset === preset) return;
  const range =
    preset === "custom"
      ? categoryDateRange(props.filters.preset).map((date) =>
          date.format("YYYY-MM-DD"),
        )
      : undefined;
  updateFilters({ preset, range });
}

function selectRange(range) {
  if (range?.length !== 2) return;
  const [start, end] = categoryDateRange("custom", range);
  if (end.diff(start, "day") > 365) {
    message.warning(t("dashboard_category_invalid_range"));
    return;
  }
  updateFilters({ range });
}

const tree = computed(() => {
  const byParent = new Map();
  for (const row of props.rows) {
    if (!byParent.has(row.parent_id)) byParent.set(row.parent_id, []);
    byParent.get(row.parent_id).push(row);
  }
  function children(parent, level) {
    return (byParent.get(parent) || []).map((row) => {
      const nodes = row.id > 0 && level < 3 ? children(row.id, level + 1) : [];
      return {
        ...row,
        name: row.id === 0 ? t("sla_uncategorized") : row.name,
        level:
          row.id === 0 ? "—" : t("dashboard_category_level_number", { level }),
        ...(nodes.length ? { children: nodes } : {}),
      };
    });
  }
  return children(0, 1);
});

const percent = computed(
  () =>
    new Intl.NumberFormat(locale.value, {
      style: "percent",
      minimumFractionDigits: 1,
      maximumFractionDigits: 1,
    }),
);
const columns = computed(() => [
  ...(props.workload === "member"
    ? [
        {
          dataIndex: "employee",
          title: t("dashboard_workload_member"),
          width: 170,
        },
        { dataIndex: "role", title: t("dashboard_workload_role"), width: 110 },
      ]
    : props.workload === "date"
      ? [{ dataIndex: "date", title: t("dashboard_workload_date"), width: 115 }]
      : [
          {
            dataIndex: "name",
            title: t("dashboard_category_name"),
            width: 180,
          },
          {
            dataIndex: "level",
            title: t("dashboard_category_level"),
            width: 70,
          },
        ]),
  ...[
    "created",
    "resolved",
    "resolution_rate",
    "closed",
    "closure_rate",
    "in_progress",
    "pending_assign",
    "pending_process",
  ].map((field) => {
    const column = {
      dataIndex: field,
      title: t(`dashboard_category_${field}`),
      align: "right",
    };
    if (field === "resolution_rate" || field === "closure_rate") {
      const count = field === "resolution_rate" ? "resolved" : "closed";
      column.customRender = ({ record }) => {
        const denominator = props.workload ? record.assigned : record.created;
        const numerator = props.workload
          ? record[`assigned_${count}`]
          : record[count];
        return denominator
          ? percent.value.format(numerator / denominator)
          : "—";
      };
    }
    return column;
  }),
]);
</script>

<style scoped lang="less">
.dashboard-stats-table {
  height: 100%;
  overflow: auto;

  &__filters {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-bottom: 10px;

    :deep(.ant-select) {
      min-width: 112px;
    }
  }

  &__total {
    background: #f5f7fa;
    font-weight: 600;
  }

  :deep(.ant-table-row-expand-icon) {
    color: var(--category-accent);
  }
}
</style>

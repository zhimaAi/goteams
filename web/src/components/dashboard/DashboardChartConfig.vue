<template>
  <div class="dashboard-chart-config">
    <label>{{ $t("dashboard_card_title") }}</label>
    <a-input
      :value="title"
      :maxlength="100"
      @update:value="emit('update:title', $event)"
    />
    <h4 class="dashboard-chart-config__section">
      {{ $t("dashboard_metric_scope") }}
    </h4>
    <a-select
      v-if="hasAssigneeScope"
      value="requirement"
      :options="[
        { value: 'requirement', label: t('dashboard_scope_requirement') },
      ]"
      :aria-label="$t('dashboard_metric_scope')"
    />
    <a-radio-group
      v-else-if="isRank"
      :value="dataScope(config.data_scope)"
      class="dashboard-chart-config__scopes"
      @change="changeScope"
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
    <a-checkbox-group
      v-else
      :value="types"
      class="dashboard-chart-config__scopes"
      @change="changeTypes"
    >
      <a-checkbox value="requirement">{{
        t("dashboard_scope_requirement")
      }}</a-checkbox>
      <a-checkbox v-if="!isIssueWorkspace" value="defect">{{
        $t("dashboard_scope_defect")
      }}</a-checkbox>
    </a-checkbox-group>
    <p v-if="!hasScope" class="dashboard-chart-config__hint">
      {{ t("dashboard_chart_scope_hint") }}
    </p>
    <template v-if="hasScope">
      <template v-if="hasAssigneeScope">
        <h4 class="dashboard-chart-config__section">
          {{ $t("dashboard_category_dimension") }}
        </h4>
        <a-radio-group
          v-if="config.chart_type === 'workload_stats'"
          v-model:value="config.workload_dimension"
          :aria-label="$t('dashboard_category_dimension')"
        >
          <a-radio-button value="member">{{
            $t("dashboard_workload_by_member")
          }}</a-radio-button>
          <a-radio-button value="date">{{
            $t("dashboard_workload_by_date")
          }}</a-radio-button>
        </a-radio-group>
        <div v-else>
          <a-tag color="blue">{{ $t("dashboard_category_by_category") }}</a-tag>
        </div>
        <h4 class="dashboard-chart-config__section">
          {{ $t("dashboard_category_assignee_scope") }}
        </h4>
        <a-radio-group
          :value="config.assignee_scope || 'all'"
          class="dashboard-chart-config__category-scopes"
          @change="changeAssigneeScope"
        >
          <a-radio
            v-for="scope in assigneeScopes"
            :key="scope.value"
            :value="scope.value"
            >{{ $t(scope.label) }}</a-radio
          >
        </a-radio-group>
        <a-select
          v-if="['members', 'roles'].includes(config.assignee_scope)"
          v-model:value="config.assignee_scope_ids"
          mode="multiple"
          show-search
          option-filter-prop="label"
          :options="assigneeScopeOptions"
          :aria-label="$t('dashboard_category_assignee_scope')"
        />
      </template>
      <template v-else-if="isDaily">
        <h4 class="dashboard-chart-config__section">
          {{ $t("dashboard_chart_time_range") }}
        </h4>
        <div class="dashboard-chart-config__dates">
          <a-input-number
            v-model:value="config.days_from"
            :min="config.days_to"
            :max="Math.min(3650, config.days_to + 365)"
            :precision="0"
            :aria-label="$t('dashboard_chart_from_days')"
          />
          <span>{{ $t("dashboard_chart_days_ago") }} —</span>
          <a-input-number
            v-model:value="config.days_to"
            :min="Math.max(0, config.days_from - 365)"
            :max="config.days_from"
            :precision="0"
            :aria-label="$t('dashboard_chart_to_days')"
          />
          <span>{{ $t("dashboard_chart_days_ago") }}</span>
        </div>
        <p class="dashboard-chart-config__hint">
          {{ $t("dashboard_chart_time_hint") }}
        </p>
      </template>
      <template v-else-if="isRank">
        <h4 class="dashboard-chart-config__section">
          {{ $t("dashboard_chart_metric_type") }}
        </h4>
        <a-select v-model:value="config.metric_type" :options="metricOptions" />
        <h4 class="dashboard-chart-config__section">
          {{ $t("dashboard_chart_conditions") }}
        </h4>
        <!-- ponytail: 数据条件 v1 仅占位，待接入真实筛选器后替换为筛选组件 -->
        <a-input
          disabled
          :placeholder="$t('dashboard_chart_add_condition')"
          :aria-label="$t('dashboard_chart_conditions')"
        />
      </template>
      <template v-else-if="isBurndown">
        <h4 class="dashboard-chart-config__section">
          {{ $t("dashboard_chart_current_iteration") }}
        </h4>
        <p class="dashboard-chart-config__hint">
          {{ $t("dashboard_chart_current_iteration_hint") }}
        </p>
      </template>
      <template v-else-if="config.chart_type !== 'status_pie'">
        <h4 class="dashboard-chart-config__section">
          {{ $t("dashboard_chart_axis") }}
        </h4>
        <label>{{ $t("dashboard_chart_display_range") }}</label>
        <a-select
          v-model:value="config[rangeField]"
          mode="multiple"
          allow-clear
          :max-tag-count="4"
          :placeholder="$t('dashboard_scope_all')"
          :options="options"
          option-filter-prop="label"
        />
        <label>{{ $t("dashboard_chart_sort") }}</label>
        <a-select v-model:value="config.sort_order" :options="sortOptions" />
        <template v-if="config.sort_order === 'custom'">
          <p class="dashboard-chart-config__hint">
            {{ $t("dashboard_chart_order_hint") }}
          </p>
          <div
            v-for="(item, index) in orderedOptions"
            :key="item.value"
            class="dashboard-chart-config__order"
          >
            <span>{{ item.label }}</span>
            <a-button
              size="small"
              :disabled="index === 0"
              :aria-label="$t('dashboard_chart_move_up')"
              @click="move(index, -1)"
              ><UpOutlined
            /></a-button>
            <a-button
              size="small"
              :disabled="index === orderedOptions.length - 1"
              :aria-label="$t('dashboard_chart_move_down')"
              @click="move(index, 1)"
              ><DownOutlined
            /></a-button>
          </div>
        </template>
      </template>
      <h4 class="dashboard-chart-config__section">
        {{ $t(colorSection) }}
      </h4>
      <template v-if="config.chart_type === 'status_pie'">
        <a-radio-group
          v-model:value="config.palette"
          class="dashboard-chart-config__palettes"
        >
          <a-radio value="status">{{
            $t("dashboard_chart_status_colors")
          }}</a-radio>
          <a-radio
            v-for="(colors, key) in chartPalettes"
            :key="key"
            :value="key"
          >
            <span class="dashboard-chart-config__palette-name">{{
              $t(`dashboard_chart_palette_${key}`)
            }}</span>
            <span class="dashboard-chart-config__swatches"
              ><i
                v-for="color in colors"
                :key="color"
                :style="{ backgroundColor: color }"
            /></span>
          </a-radio>
        </a-radio-group>
      </template>
      <template v-else>
        <div
          v-for="row in colorRows"
          :key="row.field"
          class="dashboard-chart-config__color-field"
        >
          <label v-if="!hasAssigneeScope">{{ t(row.label) }}</label>
          <div class="dashboard-chart-config__colors">
            <button
              v-for="color in hasAssigneeScope
                ? categorySwatches
                : chartSwatches"
              :key="color"
              type="button"
              :style="{ backgroundColor: color }"
              :aria-label="`${t(row.label)} ${color}`"
              :aria-pressed="config[row.field] === color"
              @click="config[row.field] = color"
            >
              <CheckOutlined v-if="config[row.field] === color" />
            </button>
          </div>
        </div>
      </template>
    </template>
  </div>
</template>
<script setup>
import { computed } from "vue";
import { useDashboardWorkItems } from "@/composables/useDashboardWorkItems";
import { useDashboardI18n } from "@/composables/useDashboardI18n";
import { CheckOutlined, UpOutlined, DownOutlined } from "@ant-design/icons-vue";
import {
  chartSwatches,
  categorySwatches,
  chartPalettes,
  dailyChartTypes,
} from "./dashboardCharts";
const props = defineProps({
  config: { type: Object, required: true },
  title: { type: String, default: "" },
  sections: { type: Object, default: () => ({}) },
  members: { type: Array, default: () => [] },
  roles: { type: Array, default: () => [] },
});
const emit = defineEmits(["update:title"]);
const { t } = useDashboardI18n();
const { isIssueWorkspace, dataScope } = useDashboardWorkItems();
const chartScopes = ["requirement", "defect"];
// 数据范围可自由勾选，全部取消时 types 为空，卡片不取数并展示空态。
const types = computed(() => {
  const scope = dataScope(props.config.data_scope);
  if (scope === "all") return [...chartScopes];
  return chartScopes.includes(scope) ? [scope] : [];
});
const hasScope = computed(() => types.value.length > 0);
const isDaily = computed(() =>
  dailyChartTypes.includes(props.config.chart_type),
);
const hasAssigneeScope = computed(() =>
  ["category_stats", "workload_stats"].includes(props.config.chart_type),
);
const assigneeScopes = [
  { value: "all", label: "dashboard_category_all_members" },
  { value: "roles", label: "dashboard_visibility_roles" },
  { value: "members", label: "dashboard_visibility_members" },
  { value: "self", label: "dashboard_visibility_self" },
];
const assigneeScopeOptions = computed(() =>
  props.config.assignee_scope === "roles"
    ? props.roles.map((role) => ({ value: Number(role.id), label: role.name }))
    : props.members.map((member) => ({
        value: Number(member.user_id),
        label: member.user_name,
      })),
);
function changeAssigneeScope(event) {
  props.config.assignee_scope = event.target.value;
  props.config.assignee_scope_ids = [];
}
const isRank = computed(() => props.config.chart_type === "assignee_rank_bar");
const isBurndown = computed(
  () => props.config.chart_type === "iteration_burndown",
);
const colorSection = computed(() =>
  isRank.value ? "dashboard_chart_card_style" : "dashboard_chart_colors",
);
const colorRows = computed(() => {
  if (isBurndown.value)
    return [
      { label: "dashboard_chart_burndown_done", field: "done_color" },
      { label: "dashboard_chart_burndown_remaining", field: "remaining_color" },
      { label: "dashboard_chart_burndown_baseline", field: "baseline_color" },
    ];
  if (isRank.value)
    return [
      { label: "dashboard_chart_rank_top3", field: "top3_color" },
      { label: "dashboard_chart_rank_rest", field: "rest_color" },
    ];
  return types.value.map((type) => ({
    label: `dashboard_scope_${type}`,
    field: `${type}_color`,
  }));
});
const metricOptions = computed(() => [
  { value: "work_item_count", label: t("dashboard_metric_work_item_count") },
]);
const rangeField = computed(() =>
  props.config.chart_type === "priority_bar" ? "priority_ids" : "status_ids",
);
const sortOptions = computed(() =>
  ["default", "desc", "asc", "custom"].map((value) => ({
    value,
    label: t(`dashboard_chart_sort_${value}`),
  })),
);
const options = computed(() => {
  const result = types.value.flatMap((type) =>
    (
      props.sections?.[type]?.[
        rangeField.value === "status_ids" ? "statuses" : "priorities"
      ] || []
    ).map((item) => ({
      value: Number(item.id),
      label: `${t(`dashboard_scope_${type}`)} · ${item.name || item.label}`,
    })),
  );
  if (rangeField.value === "priority_ids")
    result.push({ value: 0, label: t("dashboard_chart_unset") });
  return result;
});
const orderedOptions = computed(() => {
  const selected = props.config[rangeField.value] || [];
  const result = options.value.filter(
    (item) => !selected.length || selected.includes(item.value),
  );
  const rank = (id) => {
    const index = props.config.custom_order.indexOf(id);
    return index < 0 ? Infinity : index;
  };
  return result.sort((a, b) => rank(a.value) - rank(b.value));
});
function changeScope(event) {
  props.config.data_scope = event.target.value;
}
function changeTypes(value) {
  const selected = Array.isArray(value) ? value : [];
  props.config.data_scope =
    selected.length === chartScopes.length ? "all" : selected[0] || "";
  props.config.status_ids = [];
  props.config.priority_ids = [];
  props.config.custom_order = [];
}
function move(index, delta) {
  const ids = orderedOptions.value.map((item) => item.value);
  [ids[index], ids[index + delta]] = [ids[index + delta], ids[index]];
  props.config.custom_order = ids;
}
</script>
<style scoped lang="less">
.dashboard-chart-config {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 4px 0 24px;
  color: #15284b;
  &__section {
    display: flex;
    align-items: center;
    gap: 10px;
    margin: 24px 0 2px;
    color: #15284b;
    font-size: 15px;
    font-weight: 600;
    line-height: 1.2;
    &::before {
      content: "";
      flex: 0 0 3px;
      width: 3px;
      height: 15px;
      border-radius: 2px;
      background: #1677ff;
    }
  }
  // 仅作用于本组件的字段标签，避免命中 antd 复选框自身的 label 根元素。
  > label {
    display: block;
    margin: 2px 0;
    font-size: 14px;
  }
  &__scopes {
    // 复选框组默认 align-self:center 会收缩到内容宽度并居中，显式撑满左对齐。
    display: flex;
    align-self: stretch;
    width: 100%;
    align-items: center;
    gap: 12px 64px;
    flex-wrap: wrap;
    :deep(.ant-checkbox-wrapper) {
      margin-inline-start: 0;
      color: #15284b;
      font-size: 14px;
    }
  }
  &__category-scopes {
    display: grid;
    gap: 8px;
  }
  &__dates {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }
  &__hint {
    color: #8c8c8c;
    font-size: 12px;
    margin: 0;
  }
  &__color-field {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  &__colors {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
    button {
      width: 28px;
      height: 28px;
      border: 0;
      border-radius: 6px;
      color: #fff;
      cursor: pointer;
    }
    button[aria-pressed="true"] {
      outline: 2px solid #15284b;
      outline-offset: 2px;
    }
  }
  &__palettes {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  &__palette-name {
    display: inline-block;
    min-width: 55px;
  }
  &__swatches {
    display: inline-flex;
    gap: 5px;
    vertical-align: middle;
    i {
      width: 22px;
      height: 22px;
      border-radius: 5px;
    }
  }
  &__order {
    display: flex;
    align-items: center;
    gap: 6px;
    span {
      flex: 1;
    }
  }
}
</style>

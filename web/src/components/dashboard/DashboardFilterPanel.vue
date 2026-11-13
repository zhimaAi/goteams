<template>
  <section
    class="dashboard-filter-panel"
    :class="{ 'dashboard-filter-panel--compact': compact }"
    v-show="!compact || !collapsed"
  >
    <div v-if="compact" class="dashboard-quick-filters">
      <a-range-picker
        :value="quickValues('common', 'created_at')"
        value-format="YYYY-MM-DD"
        :placeholder="[
          $t('view_field_created_at'),
          $t('dashboard_filter_end_date'),
        ]"
        @change="
          (value) => setQuickValues('common', 'created_at', value, 'between')
        "
      />
      <a-select
        v-model:value="draft.work_item_types"
        mode="multiple"
        allow-clear
        :max-tag-count="1"
        :placeholder="$t('dashboard_filter_work_item_type')"
        :options="typeOptions"
        @change="apply"
      />
      <a-popover trigger="click" placement="bottomLeft">
        <a-button class="dashboard-quick-filters__status"
          >{{ $t("view_field_status") }} <DownOutlined
        /></a-button>
        <template #content
          ><div class="dashboard-quick-filters__statuses">
            <label v-for="key in workItemTypes" :key="key"
              >{{ t(`dashboard_scope_${key}`) }}
              <a-select
                :value="quickValues(key, 'status')"
                mode="multiple"
                allow-clear
                :placeholder="$t('view_field_status')"
                :options="
                  section(key).statuses.map((status) => ({
                    value: String(status.id),
                    label: status.name,
                  }))
                "
                @change="(value) => setQuickValues(key, 'status', value)"
              />
            </label></div
        ></template>
      </a-popover>
      <a-select
        v-for="field in ['creator', 'assignee']"
        :key="field"
        :value="quickValues('common', field)"
        mode="multiple"
        allow-clear
        show-search
        option-filter-prop="label"
        :max-tag-count="1"
        :placeholder="$t(`view_field_${field}`)"
        :options="
          members.map((member) => ({
            value: String(member.user_id),
            label: member.user_name,
          }))
        "
        @change="(value) => setQuickValues('common', field, value)"
      />
      <a-popover trigger="click" placement="bottomRight">
        <a-button type="text" :aria-label="$t('dashboard_more_filters')"
          ><MoreOutlined
        /></a-button>
        <template #content
          ><div class="dashboard-quick-filters__advanced">
            <DashboardFilterPanel
              :model-value="draft"
              :sections="sections"
              :members="members"
              :versions="versions"
              :default-collapsed="false"
              @apply="
                (value) => {
                  draft = normalize(value);
                  apply();
                }
              "
            /></div
        ></template>
      </a-popover>
    </div>
    <button
      v-if="!compact"
      type="button"
      class="dashboard-filter-panel__head"
      :aria-expanded="!collapsed"
      @click="collapsed = !collapsed"
    >
      <span class="dashboard-filter-panel__title">
        <FilterOutlined /> {{ $t("dashboard_filter_title") }}
        <a-badge
          v-if="activeCount"
          class="dashboard-filter-panel__badge"
          :count="activeCount"
          :overflow-count="99"
        />
      </span>
      <span class="dashboard-filter-panel__toggle">
        {{ collapsed ? $t("common_expand") : $t("common_collapse") }}
        <DownOutlined v-if="collapsed" />
        <UpOutlined v-else />
      </span>
    </button>
    <div
      v-if="!compact"
      v-show="!collapsed"
      class="dashboard-filter-panel__body"
    >
      <p class="dashboard-filter-panel__hint">
        {{
          $t(showTypes ? "dashboard_filter_hint" : "dashboard_card_filter_hint")
        }}
      </p>
      <a-select
        v-if="showTypes"
        v-model:value="draft.work_item_types"
        mode="multiple"
        allow-clear
        class="dashboard-filter-panel__types"
        :placeholder="$t('dashboard_filter_work_item_type')"
        :options="typeOptions"
      />
      <GanttFilterSection
        :title="$t('gantt_view_common_filters')"
        :group="draft.common"
        type-key="requirement"
        :statuses="section('common').statuses"
        :priorities="section('common').priorities"
        :members="members"
        :versions="versions"
        :categories="section('common').categories"
        :fields="section('common').fields"
      />
      <GanttFilterSection
        :title="t('gantt_view_requirement_filters')"
        :group="draft.requirement"
        type-key="requirement"
        :statuses="section('requirement').statuses"
        :priorities="section('requirement').priorities"
        :members="members"
        :versions="versions"
        :categories="section('requirement').categories"
        :fields="section('requirement').fields"
      />
      <GanttFilterSection
        v-if="!isIssueWorkspace"
        :title="$t('gantt_view_defect_filters')"
        :group="draft.defect"
        type-key="defect"
        :statuses="section('defect').statuses"
        :priorities="section('defect').priorities"
        :members="members"
        :versions="versions"
        :categories="section('defect').categories"
        :fields="section('defect').fields"
      />
      <div class="dashboard-filter-panel__actions">
        <a-button @click="reset">{{ $t("common_clearFilter") }}</a-button>
        <a-button type="primary" @click="apply">
          {{ $t("dashboard_filter_apply") }}
        </a-button>
      </div>
    </div>
  </section>
</template>

<script setup>
import { useDashboardWorkItems } from "@/composables/useDashboardWorkItems";
import { useDashboardI18n } from "@/composables/useDashboardI18n";
import { computed, ref, watch } from "vue";
import {
  MoreOutlined,
  FilterOutlined,
  DownOutlined,
  UpOutlined,
} from "@ant-design/icons-vue";
import GanttFilterSection from "@/components/work-item/GanttFilterSection.vue";

const props = defineProps({
  modelValue: { type: Object, default: null },
  statuses: { type: Array, default: () => [] },
  priorities: { type: Array, default: () => [] },
  members: { type: Array, default: () => [] },
  versions: { type: Array, default: () => [] },
  categories: { type: Array, default: () => [] },
  fields: { type: Array, default: () => [] },
  // Canonical dashboard filter shape mirrors the existing Gantt view:
  // { common, requirement, defect }. Each section has its own status/priority ids.
  sections: { type: Object, default: null },
  defaultCollapsed: { type: Boolean, default: true },
  compact: { type: Boolean, default: false },
  showTypes: { type: Boolean, default: true },
});

const emit = defineEmits({
  "update:modelValue": (value) => Boolean(value),
  apply: (value) => Boolean(value),
  "update:collapsed": (value) => typeof value === "boolean",
});

const { t } = useDashboardI18n();
const {
  isIssueWorkspace,
  workItemTypes,
  normalizeFilters: normalize,
} = useDashboardWorkItems();
const typeOptions = computed(() =>
  workItemTypes.value.map((type) => ({
    value: type,
    label: t(`dashboard_scope_${type}`),
  })),
);
const collapsed = ref(props.defaultCollapsed);
const draft = ref(normalize(props.modelValue));

const activeCount = computed(() => countActiveConditions(draft.value));

watch(
  () => props.modelValue,
  (value) => {
    draft.value = normalize(value);
  },
  { deep: true },
);
watch(
  () => props.defaultCollapsed,
  (value) => {
    collapsed.value = value;
  },
);
watch(collapsed, (value) => {
  if (value !== props.defaultCollapsed) emit("update:collapsed", value);
});

function quickValues(sectionKey, field) {
  const group = draft.value[sectionKey];
  if (group.logic !== "and") return [];
  return group.conditions.find((node) => node.field === field)?.values || [];
}

function setQuickValues(sectionKey, field, values, operator = "in") {
  let group = draft.value[sectionKey];
  // Preserve existing OR/nested filters while adding the quick condition as an intersection.
  if (group.logic !== "and") {
    group = {
      logic: "and",
      conditions: group.conditions.length ? [group] : [],
    };
    draft.value[sectionKey] = group;
  }
  group.conditions = group.conditions.filter((node) => node.field !== field);
  if (values?.length)
    group.conditions.push({ field, operator, values: values.map(String) });
  apply();
}

function section(key) {
  const source = props.sections?.[key];
  if (source) return source;
  return {
    statuses: props.statuses,
    priorities: props.priorities,
    categories: props.categories,
    fields: props.fields,
  };
}

function countActiveConditions(group) {
  if (group?.common || group?.requirement || group?.defect) {
    return ["common", "requirement", "defect"].reduce(
      (count, key) => count + countActiveConditions(group[key]),
      (group.work_item_types || []).length,
    );
  }
  return (group?.conditions || []).reduce((count, node) => {
    if (!node?.field) return count + countActiveConditions(node);
    if (["empty", "not_empty"].includes(node.operator)) return count + 1;
    return (
      count + (node.values || []).filter((value) => String(value).trim()).length
    );
  }, 0);
}

function reset() {
  draft.value = normalize(null);
}

function getFilters() {
  return normalize(draft.value);
}
defineExpose({ getFilters });

function apply() {
  const value = normalize(draft.value);
  emit("update:modelValue", value);
  emit("apply", value);
}
</script>

<style scoped lang="less">
.dashboard-filter-panel--compact {
  flex: 1 1 auto;
  min-width: 0;
  overflow-x: auto;
  overflow-y: hidden;
  border: 0 !important;
  scrollbar-width: none;
  &::-webkit-scrollbar {
    display: none;
  }
}
.dashboard-quick-filters {
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  gap: 8px;
  min-width: max-content;
  > * {
    flex-shrink: 0;
  }
  > .ant-select {
    flex: 1 1 110px;
    min-width: 100px;
    max-width: 180px;
  }
  > .ant-picker {
    flex: 0 1 250px;
    min-width: 200px;
    width: auto;
  }
}
.dashboard-quick-filters__status {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 30px;
  color: #15284b;
}
.dashboard-quick-filters__statuses {
  width: 300px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  label {
    display: flex;
    flex-direction: column;
    gap: 8px;
    color: #15284b;
  }
}
.dashboard-quick-filters__advanced {
  width: min(760px, calc(100vw - 64px));
  max-height: 65vh;
  overflow: auto;
}

.dashboard-filter-panel__types {
  width: 100%;
  margin-bottom: 12px;
}

.dashboard-filter-panel {
  border: 1px solid #f0f0f0;
  border-radius: 8px;
  background: #fff;
}

.dashboard-filter-panel__head {
  width: 100%;
  min-height: 42px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 14px;
  border: 0;
  border-radius: 8px;
  color: #262626;
  background: transparent;
  cursor: pointer;
  text-align: left;

  &:hover {
    background: #fafafa;
  }
}

.dashboard-filter-panel__title,
.dashboard-filter-panel__toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.dashboard-filter-panel__title {
  font-weight: 500;
}

.dashboard-filter-panel__badge :deep(.ant-badge-count) {
  box-shadow: none;
}

.dashboard-filter-panel__toggle {
  color: #8c8c8c;
  font-size: 12px;
}

.dashboard-filter-panel__body {
  padding: 4px 14px 14px;
  border-top: 1px solid #f0f0f0;
}

.dashboard-filter-panel__hint {
  margin: 10px 0 12px;
  color: #8c8c8c;
  font-size: 12px;
}

.dashboard-filter-panel__actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 14px;
}
</style>

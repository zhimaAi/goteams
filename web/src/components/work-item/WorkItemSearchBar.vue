<!--
 * WorkItemSearchBar.vue
 * Customizable search conditions for requirement and defect lists.
 -->
<template>
  <div class="work-item-search-bar">
    <div
      v-for="condition in displayConditions"
      :key="condition.field"
      class="search-condition"
      :class="{
        'search-condition--title': condition.field === 'title',
        'search-condition--template': condition.field === 'template',
        'search-condition--member':
          fieldMeta(condition.field).control === 'person',
        'search-condition--date': fieldMeta(condition.field).control === 'date',
        'search-condition--number':
          fieldMeta(condition.field).control === 'number',
        'search-condition--cascader':
          fieldMeta(condition.field).control === 'cascader',
        'search-condition--text': fieldMeta(condition.field).control === 'text',
        'search-condition--active': conditionHasValue(condition),
      }"
    >
      <a-input-search
        v-if="condition.field === 'title'"
        :value="condition.values[0] || ''"
        class="search-title-input"
        :placeholder="$t('workspace_searchTitle')"
        allow-clear
        @update:value="
          (value) =>
            updateCondition(condition.field, value ? [value] : [], !value)
        "
        @search="$emit('search')"
      />
      <template v-else>
        <span class="search-condition__label">{{
          fieldLabel(condition.field)
        }}</span>
        <AssigneeSelect
          v-if="fieldMeta(condition.field).control === 'person'"
          :value="condition.values"
          :members="membersForCondition(condition.field)"
          :placeholder="$t('common_all')"
          :search-placeholder="
            fieldMeta(condition.field).kind === 'custom'
              ? $t('member_search')
              : ''
          "
          :default-open="false"
          :popup-width="320"
          compact
          @save="(value) => updateSelectCondition(condition.field, value)"
        />
        <CheckboxMultiSelectPanel
          v-else-if="fieldMeta(condition.field).control === 'multi'"
          :value="panelValues(condition)"
          :options="multiPanelOptions(condition.field)"
          :leading-options="multiPanelLeadingOptions(condition.field)"
          :show-closed-toggle="condition.field === 'version'"
          :panel-width="condition.field === 'version' ? 340 : 304"
          @update:value="
            (value) =>
              updateSelectCondition(condition.field, expandOptionValues(value))
          "
        />
        <DateRangeFilter
          v-else-if="fieldMeta(condition.field).control === 'date'"
          :operator="condition.operator"
          :values="condition.values"
          @change="
            ({ operator, values }) =>
              updateCondition(condition.field, values, true, operator)
          "
        />
        <NumberCompareFilter
          v-else-if="fieldMeta(condition.field).control === 'number'"
          :operator="condition.operator"
          :values="condition.values"
          @change="
            ({ operator, values }) =>
              updateCondition(condition.field, values, true, operator)
          "
        />
        <CascaderSearchPanel
          v-else-if="fieldMeta(condition.field).control === 'cascader'"
          :value="condition.values"
          :operator="condition.operator"
          :nodes="fieldMeta(condition.field).cascader || []"
          :aria-label="fieldLabel(condition.field)"
          :multiple="
            fieldMeta(condition.field).fieldType ===
            CUSTOM_FIELD_TYPE.CASCADER_MULTI
          "
          @update:value="
            (value) => updateCascaderCondition(condition.field, value)
          "
        />
        <a-input-search
          v-else-if="fieldMeta(condition.field).control === 'text'"
          :value="condition.values[0] || ''"
          class="search-condition__text-input"
          :placeholder="
            fieldMeta(condition.field).placeholder || $t('common_all')
          "
          allow-clear
          @update:value="
            (value) =>
              updateCondition(
                condition.field,
                value ? [value] : [],
                !value,
                fieldMeta(condition.field).operator,
              )
          "
          @search="$emit('search')"
        />
        <a-select
          v-else
          :value="condition.values[0]"
          class="search-condition__select"
          popup-class-name="work-item-search-dropdown"
          :options="fieldMeta(condition.field).options || []"
          :placeholder="$t('common_all')"
          :bordered="false"
          :show-search="condition.field === 'template'"
          :aria-label="fieldLabel(condition.field)"
          allow-clear
          :filter-option="filterOption"
          @change="(value) => updateSelectCondition(condition.field, value)"
        >
          <template v-if="condition.field === 'template'" #option="option">
            <a-tag
              v-if="option.sourceLabel"
              class="template-source-tag"
              :color="option.sourceColor"
              >{{ option.sourceLabel }}</a-tag
            >
            {{ option.label }}
          </template>
          <template v-if="condition.field === 'template'" #optionLabel="option">
            <a-tag
              v-if="option?.sourceLabel"
              class="template-source-tag"
              :color="option.sourceColor"
              >{{ option.sourceLabel }}</a-tag
            >
            {{ option?.label ?? condition.values[0] }}
          </template>
        </a-select>
      </template>
      <button
        v-if="!['title', 'template'].includes(condition.field)"
        type="button"
        class="search-condition__remove"
        :aria-label="
          $t('search_remove_condition', { field: fieldLabel(condition.field) })
        "
        @click="removeCondition(condition.field)"
      >
        <CloseOutlined />
      </button>
    </div>

    <a-popover
      v-model:open="addOpen"
      trigger="click"
      placement="bottomLeft"
      overlay-class-name="search-condition-popover"
    >
      <a-button
        class="add-condition-button"
        :disabled="!remainingFields.length"
      >
        <PlusOutlined /> {{ $t("view_add_condition") }}
      </a-button>
      <template #content>
        <div class="add-condition-panel">
          <a-input-search
            v-model:value="conditionKeyword"
            :placeholder="$t('search_condition_placeholder')"
            allow-clear
          />
          <template v-for="group in remainingFieldGroups" :key="group.key">
            <div v-if="group.fields.length" class="add-condition-panel__group">
              {{ group.label }}
            </div>
            <button
              v-for="field in group.fields"
              :key="field.field"
              type="button"
              class="add-condition-panel__item"
              @click="addCondition(field.field)"
            >
              <span>{{ field.label }}</span>
              <span v-if="field.typeLabel" class="add-condition-panel__type">
                {{ field.typeLabel }}
              </span>
            </button>
          </template>
          <a-empty
            v-if="!filteredRemainingFields.length"
            :description="$t('search_no_conditions')"
            :image="simpleImage"
          />
        </div>
      </template>
    </a-popover>

    <a-button class="save-view-button" @click="$emit('save-as')">
      <SaveOutlined /> {{ $t("search_save_as_view") }}
    </a-button>
    <a-button class="search-clear-button" @click="clearValues">
      <DeleteOutlined /> {{ $t("common_clearFilter") }}
    </a-button>
  </div>
</template>

<script setup>
import dayjs from "dayjs";
import { computed, ref, watch } from "vue";
import { Empty } from "ant-design-vue";
import { useI18n } from "vue-i18n";
import AssigneeSelect from "@/components/common/AssigneeSelect.vue";
import CheckboxMultiSelectPanel from "@/components/work-item/CheckboxMultiSelectPanel.vue";
import DateRangeFilter from "@/components/work-item/DateRangeFilter.vue";
import NumberCompareFilter from "@/components/work-item/NumberCompareFilter.vue";
import CascaderSearchPanel from "@/components/work-item/CascaderSearchPanel.vue";
import {
  DeleteOutlined,
  CloseOutlined,
  PlusOutlined,
  SaveOutlined,
} from "@ant-design/icons-vue";
import { filterMembersForField } from "@/utils/customField";
import { CUSTOM_FIELD_TYPE } from "@/constants/customField";
import {
  VIEW_VALUE_ALL_COMPLETED,
  VIEW_VALUE_ALL_CLOSED,
  VIEW_VALUE_ALL_END,
  VIEW_VALUE_ALL_NOT_END,
  VIEW_VALUE_EMPTY,
  VIEW_VALUE_NOT_EMPTY,
  collapseOptionValues,
  expandOptionValues,
  newSearchFilterCondition,
  normalizeSearchFilterTree,
} from "@/constants/workItemView";

const props = defineProps({
  filters: { type: Object, required: true },
  typeKey: { type: String, default: "requirement" },
  fields: { type: Array, default: () => [] },
  statuses: { type: Array, default: () => [] },
  priorities: { type: Array, default: () => [] },
  members: { type: Array, default: () => [] },
  customFieldMembers: { type: Array, default: () => [] },
  versions: { type: Array, default: () => [] },
  categories: { type: Array, default: () => [] },
});

const emit = defineEmits({
  change: (filters, detail) => true,
  search: () => true,
  "save-as": () => true,
});

const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;
const { t } = useI18n();
const addOpen = ref(false);
const conditionKeyword = ref("");

// The stored tree keeps title first to satisfy the search preference contract.
const displayConditions = computed(() => [
  ...(props.filters.conditions || []).filter(
    (condition) => condition.field === "template",
  ),
  ...(props.filters.conditions || []).filter(
    (condition) => condition.field !== "template",
  ),
]);

watch(addOpen, (open) => {
  if (!open) conditionKeyword.value = "";
});

const selectedFields = computed(
  () =>
    new Set(
      (props.filters.conditions || []).map((condition) => condition.field),
    ),
);

const remainingFields = computed(() =>
  props.fields.filter(
    (field) =>
      field.field !== "title" && !selectedFields.value.has(field.field),
  ),
);

const filteredRemainingFields = computed(() => {
  const keyword = conditionKeyword.value.trim().toLocaleLowerCase();
  if (!keyword) return remainingFields.value;
  return remainingFields.value.filter((field) =>
    field.label.toLocaleLowerCase().includes(keyword),
  );
});

const remainingFieldGroups = computed(() => [
  {
    key: "builtin",
    label: t("search_basic_fields"),
    fields: filteredRemainingFields.value.filter(
      (field) => field.kind !== "custom",
    ),
  },
  {
    key: "custom",
    label: t("search_custom_fields"),
    fields: filteredRemainingFields.value.filter(
      (field) => field.kind === "custom",
    ),
  },
]);

const fieldMap = computed(
  () => new Map(props.fields.map((field) => [field.field, field])),
);

function fieldLabel(field) {
  return fieldMeta(field).label || t(`view_field_${field}`);
}

function membersForCondition(field) {
  const descriptor = fieldMeta(field);
  return descriptor.kind === "custom"
    ? filterMembersForField(descriptor, props.customFieldMembers)
    : props.members;
}

function fieldMeta(field) {
  return (
    fieldMap.value.get(field) || {
      field,
      label: t(`view_field_${field}`),
      control: "select",
      operator: field === "title" ? "contains" : "in",
      options: [],
    }
  );
}

function conditionHasValue(condition) {
  return (
    ["empty", "not_empty"].includes(condition.operator) ||
    (condition.values || []).some((value) => String(value).trim().length > 0)
  );
}

function cloneFilters() {
  return normalizeSearchFilterTree(props.filters, props.typeKey);
}

function emitFilters(filters, detail) {
  emit("change", normalizeSearchFilterTree(filters, props.typeKey), detail);
}

function updateCondition(field, values, immediateQuery, operator) {
  const filters = cloneFilters();
  const condition = filters.conditions.find((item) => item.field === field);
  if (!condition) return;
  condition.values = values.map(String);
  condition.operator =
    operator || condition.operator || fieldMeta(field).operator;
  emitFilters(filters, { action: "value", field, immediateQuery });
}

function updateCascaderCondition(field, value) {
  const values = value || [];
  if (values.length === 1 && values[0] === VIEW_VALUE_EMPTY) {
    updateCondition(field, [], true, "empty");
    return;
  }
  if (values.length === 1 && values[0] === VIEW_VALUE_NOT_EMPTY) {
    updateCondition(field, [], true, "not_empty");
    return;
  }
  updateCondition(field, values, true, "in");
}

function updateSelectCondition(field, value) {
  const values = ["person", "multi"].includes(fieldMeta(field).control)
    ? value || []
    : value
      ? [value]
      : [];
  updateCondition(field, values, true);
}

// A merged option (iteration status/priority) covers several ids, so the picker selection is
// derived from the flat condition values instead of being used directly.
function panelValues(condition) {
  return collapseOptionValues(condition.values, [
    ...multiPanelLeadingOptions(condition.field),
    ...multiPanelOptions(condition.field),
  ]);
}

function multiPanelLeadingOptions(field) {
  if (field === "status") {
    return [
      {
        value: VIEW_VALUE_ALL_COMPLETED,
        label: `-${t("view_all_completed_statuses")}-`,
      },
      {
        value: VIEW_VALUE_ALL_CLOSED,
        label: `-${t("view_all_closed_statuses")}-`,
      },
      {
        value: VIEW_VALUE_ALL_END,
        label: `-${t("view_all_end_statuses")}-`,
      },
      {
        value: VIEW_VALUE_ALL_NOT_END,
        label: `-${t("view_all_not_end_statuses")}-`,
      },
    ];
  }
  if (field === "version" || field === "priority") {
    return [{ value: "0", label: t("search_empty_value") }];
  }
  return [];
}

function multiPanelOptions(field) {
  if (field === "status") {
    return props.statuses.map((status) => ({
      value: String(status.id),
      label: status.name,
    }));
  }
  if (field === "version") {
    return props.versions.map((version) => ({
      value: String(version.id),
      label: version.name,
      closed: versionClosed(version),
    }));
  }
  if (field === "priority") {
    return props.priorities.map((priority) => ({
      value: String(priority.id),
      label: priority.label,
    }));
  }
  return fieldMeta(field).options || [];
}

function versionClosed(version) {
  if (version?.is_ended !== undefined) {
    const ended = String(version.is_ended).toLocaleLowerCase();
    return ended === "1" || ended === "true";
  }
  const status = String(version?.status || "").toLocaleLowerCase();
  if (["ended", "completed", "closed"].includes(status)) return true;
  const endDate = Number(version?.end_date);
  return endDate > 0 && dayjs.unix(endDate).endOf("day").isBefore(dayjs());
}

function addCondition(field) {
  const filters = cloneFilters();
  filters.conditions.push(
    newSearchFilterCondition(field, fieldMeta(field).operator),
  );
  addOpen.value = false;
  emitFilters(filters, { action: "add", field, immediateQuery: false });
}

function removeCondition(field) {
  const filters = cloneFilters();
  filters.conditions = filters.conditions.filter(
    (condition) => condition.field !== field,
  );
  emitFilters(filters, { action: "remove", field, immediateQuery: true });
}

function clearValues() {
  const filters = cloneFilters();
  filters.conditions.forEach((condition) => {
    condition.values = [];
    condition.operator = fieldMeta(condition.field).operator;
  });
  emitFilters(filters, { action: "clear", field: "", immediateQuery: true });
}

function filterOption(input, option) {
  return `${option.sourceLabel || ""} ${option.label}`
    .toLocaleLowerCase()
    .includes(input.trim().toLocaleLowerCase());
}
</script>

<style scoped lang="less">
.work-item-search-bar {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 8px;
  flex-wrap: wrap;
}

.search-condition {
  position: relative;
  display: inline-flex;
  align-items: center;
  width: 130px;
  height: 30px;
  overflow: visible;
  flex-shrink: 0;
  border: 1px solid #d9d9d9;
  border-radius: 4px;
  background: var(--app-surface);

  &:hover {
    border-color: #d0d5dd;
    background: #f0f1f3;
  }

  &:focus-within {
    border-color: #3157e2;
    box-shadow: 0 0 0 2px rgb(49 87 226 / 10%);
  }
}

.search-condition--title {
  width: 212px;
  height: 30px;
  border: 0;
}

.search-condition--template {
  width: 176px;
}

.template-source-tag {
  margin-inline-end: 4px;
  font-size: 11px;
  line-height: 18px;
}

.search-condition--active:not(.search-condition--title) {
  border-color: #73a5ff;
  background: #f0f5ff;
}

.search-condition--title.search-condition--active {
  :deep(.ant-input-affix-wrapper),
  :deep(.ant-input-group-addon .ant-btn) {
    border-color: #73a5ff;
    background: #f0f5ff;
  }
}

.search-condition--member {
  width: 122px;

  :deep(.assignee-select) {
    position: absolute;
    inset: 0;
    height: 28px;
    min-height: 28px;
    padding: 0 8px 0 64px;
    border: 0;
    background: transparent;
  }

  :deep(.assignee-select--open) {
    box-shadow: none;
  }

  :deep(.assignee-select__input) {
    height: 28px;
    padding: 0;
    line-height: 28px;
  }

  :deep(.assignee-select__compact-value) {
    height: 28px;
  }

  :deep(.assignee-select__arrow) {
    margin-left: auto;
  }
}

.search-condition--date {
  width: 280px;
}

.search-condition--number,
.search-condition--text {
  width: 210px;
}

.search-condition--number :deep(.number-compare-filter__trigger),
.search-condition--cascader :deep(.cascader-search-panel__trigger) {
  width: 100%;
  padding-left: 64px;
}

.search-condition__text-input {
  position: absolute;
  inset: 0;

  :deep(.ant-input-affix-wrapper) {
    height: 28px;
    padding-left: 64px;
    border: 0;
    background: transparent;
    box-shadow: none;
  }

  :deep(.ant-input-group-addon .ant-btn) {
    height: 28px;
    border: 0;
    background: transparent;
    box-shadow: none;
  }
}

.search-title-input {
  width: 100%;
  :deep(.ant-input-affix-wrapper) {
    height: 30px;
    padding: 4px 8px 4px 10px;
    border-right: 0;
    border-radius: 4px 0 0 4px;
    box-shadow: none;
    font-size: 12px;
  }

  :deep(.ant-input) {
    color: var(--app-text);
    font-size: 12px;
  }

  :deep(.ant-input::placeholder) {
    color: #9aa5b5;
  }

  :deep(.ant-input-group-addon) {
    background: transparent;
  }

  :deep(.ant-input-group-addon .ant-btn) {
    width: 34px;
    height: 30px;
    padding: 0;
    border-color: #d9dfe8;
    border-left: 0;
    border-radius: 0 4px 4px 0;
    color: #657188;
    background: var(--app-surface);
    box-shadow: none;
  }

  &:hover {
    :deep(.ant-input-affix-wrapper),
    :deep(.ant-input-group-addon .ant-btn) {
      border-color: #b8c2d1;
    }
  }

  &:focus-within {
    :deep(.ant-input-affix-wrapper),
    :deep(.ant-input-group-addon .ant-btn) {
      border-color: #3157e2;
    }
  }
}

.search-condition__label {
  position: absolute;
  top: 0;
  left: 8px;
  z-index: 2;
  height: 28px;
  max-width: 52px;
  overflow: hidden;
  color: var(--app-text);
  font-size: 12px;
  line-height: 28px;
  text-overflow: ellipsis;
  white-space: nowrap;
  pointer-events: none;
}

.search-condition__select {
  position: absolute;
  inset: 0;
  width: 100%;

  :deep(.ant-select-selector) {
    height: 28px !important;
    padding: 0 20px 0 64px !important;
    background: transparent !important;
  }

  :deep(.ant-select-selector .ant-select-selection-search) {
    inset-inline-start: 64px;
    inset-inline-end: 24px;
  }

  :deep(.ant-select-selector input.ant-select-selection-search-input) {
    height: 26px;
    font-size: 12px;
    line-height: 26px;
  }

  :deep(.ant-select-arrow) {
    right: 8px;
  }

  :deep(.ant-select-selection-item),
  :deep(.ant-select-selection-placeholder) {
    color: var(--app-text-secondary);
    font-size: 12px;
    line-height: 26px !important;
  }
}

.search-condition__remove {
  position: absolute;
  z-index: 3;
  top: -7px;
  right: -7px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 14px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  color: #fff;
  background: #aeb7c5;
  box-shadow: 0 1px 2px rgb(31 41 55 / 18%);
  cursor: pointer;
  font-size: 9px;
  opacity: 0;
  pointer-events: none;
  transform: scale(0.8);
  transition:
    opacity 0.15s ease,
    transform 0.15s ease;
}

.search-condition__remove:hover,
.search-condition__remove:focus-visible {
  background: #8e99aa;
}

.search-condition:hover .search-condition__remove,
.search-condition:focus-within .search-condition__remove,
.search-condition__remove:focus-visible {
  opacity: 1;
  pointer-events: auto;
  transform: scale(1);
}

.search-clear-button {
  color: #7c8597;
}

.save-view-button,
.search-clear-button {
  border-color: transparent;
  box-shadow: none;
  font-size: 12px;
}

.save-view-button {
  color: #7c8597;
}

.save-view-button:hover,
.save-view-button:focus,
.search-clear-button:hover,
.search-clear-button:focus {
  border-color: transparent;
  box-shadow: none;
  background: #f0f1f3;
  color: var(--app-text);
}

.add-condition-panel {
  width: 280px;
}

.add-condition-panel__group {
  margin-top: 8px;
  padding: 4px 8px;
  color: var(--app-text-secondary);
  font-size: 12px;
}

.add-condition-panel__item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
  padding: 7px 8px;
  border: 0;
  border-radius: 4px;
  color: var(--app-text);
  background: transparent;
  text-align: left;
  cursor: pointer;

  &:hover,
  &:focus-visible {
    background: var(--app-surface-muted);
    outline: none;
  }
}

.add-condition-panel__type {
  flex: 0 0 auto;
  color: var(--app-text-secondary);
  font-size: 11px;
}

:deep(.ant-empty) {
  margin: 16px 0 8px;
}

.add-condition-button {
  color: #7c8597;
}
</style>

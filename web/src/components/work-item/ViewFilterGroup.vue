<!--
 * ViewFilterGroup.vue
 * Recursive AND/OR condition group for the view data-scope editor. Renders
 * condition rows (field / operator / value) and nested sub-groups; the root
 * group also renders the fixed "add condition / add group" footer.
 * Nodes are edited in place: the parent modal owns the reactive tree and this
 * component mutates node objects shared by reference.
 -->
<template>
  <div
    class="view-filter-group"
    :class="{ 'view-filter-group--nested': !root }"
  >
    <a-button
      v-if="!root"
      type="text"
      size="small"
      class="view-filter-group__remove"
      @click="$emit('remove')"
    >
      <CloseOutlined />
    </a-button>
    <div class="view-filter-group__body">
      <div v-if="group.conditions.length > 1" class="view-filter-group__logic">
        <a-select
          v-model:value="group.logic"
          size="small"
          class="view-filter-group__logic-select"
          :options="logicOptions"
        />
      </div>
      <div class="view-filter-group__conditions">
        <template v-for="(node, index) in group.conditions" :key="index">
          <ViewFilterGroup
            v-if="!node.field"
            :group="node"
            :type-key="typeKey"
            @remove="removeAt(index)"
          />
          <div v-else class="view-filter-row">
            <a-select
              v-model:value="node.field"
              class="view-filter-row__field"
              :options="fieldOptions"
              @change="handleFieldChange(node)"
            />
            <a-select
              v-model:value="node.operator"
              class="view-filter-row__operator"
              :options="operatorOptions(node.field)"
              @change="handleOperatorChange(node)"
            />
            <template v-if="showValueInput(node)">
              <a-input
                v-if="valueControl(node) === 'text'"
                class="view-filter-row__value"
                :value="node.values[0]"
                :placeholder="titlePlaceholder(node)"
                @update:value="(value) => (node.values = value ? [value] : [])"
              />
              <a-input-number
                v-else-if="valueControl(node) === 'number-single'"
                class="view-filter-row__value"
                :value="
                  node.values[0] === undefined || node.values[0] === ''
                    ? null
                    : Number(node.values[0])
                "
                :controls="false"
                :min="isHourOperator(node.operator) ? 1 : undefined"
                :max="isHourOperator(node.operator) ? 999 : undefined"
                :precision="isHourOperator(node.operator) ? 0 : undefined"
                @update:value="
                  (value) =>
                    (node.values =
                      value === null || value === undefined
                        ? []
                        : [String(value)])
                "
              />
              <a-date-picker
                v-else-if="isDateField(node.field) && node.operator === 'eq'"
                class="view-filter-row__value"
                :value="node.values[0] || undefined"
                value-format="YYYY-MM-DD"
                @update:value="(value) => (node.values = value ? [value] : [])"
              />
              <DateRangeFilter
                v-else-if="
                  isDateField(node.field) && node.operator === 'between'
                "
                class="view-filter-row__value"
                :operator="node.operator"
                :values="node.values"
                @change="
                  ({ operator, values }) => {
                    node.operator = operator;
                    node.values = values;
                  }
                "
              />
              <ViewDynamicDateInput
                v-else-if="
                  isDateField(node.field) && node.operator === 'dynamic'
                "
                class="view-filter-row__value"
                :values="node.values"
                @update:values="(values) => (node.values = values)"
              />
              <StatusMultiPanel
                v-else-if="node.field === 'status'"
                class="view-filter-row__value"
                :value="
                  collapseOptionValues(node.values, options.statuses.value)
                "
                :statuses="options.statuses.value"
                @update:value="
                  (values) => (node.values = expandOptionValues(values))
                "
              />
              <DateRangeFilter
                v-else-if="valueControl(node) === 'date'"
                class="view-filter-row__value"
                :operator="node.operator"
                :values="node.values"
                @change="
                  ({ operator, values }) => {
                    node.operator = operator;
                    node.values = values;
                  }
                "
              />
              <NumberRangeFilter
                v-else-if="valueControl(node) === 'number'"
                class="view-filter-row__value"
                :values="node.values"
                @change="(values) => (node.values = values)"
              />
              <CustomFieldCascader
                v-else-if="valueControl(node) === 'cascader'"
                class="view-filter-row__value"
                :nodes="fieldDescriptor(node.field)?.cascader || []"
                :model-value="node.values"
                :multiple="
                  fieldDescriptor(node.field)?.fieldType ===
                  CUSTOM_FIELD_TYPE.CASCADER_MULTI
                "
                @update:model-value="
                  (values) => updateCascaderValue(node, values)
                "
              />
              <a-select
                v-else
                class="view-filter-row__value"
                :mode="selectMode(node.field)"
                :value="selectValue(node)"
                :options="valueOptions(node.field)"
                :placeholder="$t('view_value_placeholder')"
                :max-tag-count="
                  selectMode(node.field) === 'multiple' ? 2 : undefined
                "
                show-search
                :filter-option="filterOption"
                @update:value="(value) => updateSelectValue(node, value)"
              />
            </template>
            <div
              v-else
              class="view-filter-row__value view-filter-row__value--hidden"
            />
            <span class="view-filter-row__ops">
              <a-button type="text" size="small" @click="insertAfter(index)">
                <PlusOutlined />
              </a-button>
              <a-button type="text" size="small" @click="removeAt(index)">
                <MinusOutlined />
              </a-button>
            </span>
          </div>
        </template>
        <a-button
          v-if="!group.conditions.length && !root"
          type="dashed"
          size="small"
          class="view-filter-group__fallback"
          @click="appendCondition"
        >
          <PlusOutlined /> {{ $t("view_add_condition") }}
        </a-button>
      </div>
    </div>
    <div v-if="root" class="view-filter-group__footer">
      <a-button @click="appendCondition"
        ><PlusOutlined /> {{ $t("view_add_condition") }}</a-button
      >
      <a-button @click="appendGroup"
        ><PlusOutlined /> {{ $t("view_add_group") }}</a-button
      >
    </div>
  </div>
</template>

<script setup>
import { computed, inject } from "vue";
import { useI18n } from "vue-i18n";
import {
  PlusOutlined,
  MinusOutlined,
  CloseOutlined,
} from "@ant-design/icons-vue";
import StatusMultiPanel from "@/components/work-item/StatusMultiPanel.vue";
import DateRangeFilter from "@/components/work-item/DateRangeFilter.vue";
import NumberRangeFilter from "@/components/work-item/NumberRangeFilter.vue";
import ViewDynamicDateInput from "@/components/work-item/ViewDynamicDateInput.vue";
import CustomFieldCascader from "@/components/common/CustomFieldCascader.vue";
import {
  VIEW_DATE_FIELDS,
  VIEW_FILTER_FIELD_OPERATORS,
  VIEW_VALUE_CURRENT_ENTERPRISE_ROLE,
  collapseOptionValues,
  expandOptionValues,
  newViewFilterCondition,
  newViewFilterGroup,
} from "@/constants/workItemView";
import {
  CUSTOM_FIELD_TYPE,
  customFieldViewOperators,
  isCascaderFieldType,
  isMemberFieldType,
  isNumericFieldType,
} from "@/constants/customField";
import { filterMembersForField } from "@/utils/customField";

const props = defineProps({
  // Group node ({ logic, conditions }) edited in place.
  group: { type: Object, required: true },
  // The root group renders the footer buttons and cannot be removed.
  root: { type: Boolean, default: false },
  typeKey: { type: String, default: "requirement" },
});

defineEmits({
  // Ask the parent group to remove this sub-group.
  remove: () => true,
});

const { t } = useI18n();
// Option sources (statuses/priorities/members/versions) provided once by ViewEditModal.
const options = inject("viewFilterOptions");

const logicOptions = [
  { value: "and", label: "AND" },
  { value: "or", label: "OR" },
];

const fieldOptions = computed(() =>
  (options.fields?.value || [])
    .filter(
      (field) => field.field !== "category" || props.typeKey === "requirement",
    )
    .map((field) => ({ value: field.field, label: field.label })),
);

function operatorOptions(field) {
  const descriptor = fieldDescriptor(field);
  const customOperators = customFieldViewOperators(descriptor?.fieldType);
  const operators = customOperators.length
    ? customOperators
    : VIEW_FILTER_FIELD_OPERATORS[field] || [];
  return operators.map((operator) => ({
    value: operator,
    label: t(
      isPersonField(field) &&
        ["contains", "not_contains", "eq", "ne"].includes(operator)
        ? `view_op_person_${operator}`
        : `view_op_${operator}`,
    ),
  }));
}

function isPersonField(field) {
  const descriptor = fieldDescriptor(field);
  return (
    descriptor?.control === "person" || isMemberFieldType(descriptor?.fieldType)
  );
}

function fieldDescriptor(field) {
  return (options.fields?.value || []).find((item) => item.field === field);
}

function selectMode(field) {
  const fieldType = fieldDescriptor(field)?.fieldType;
  return fieldType === CUSTOM_FIELD_TYPE.SELECT_SINGLE ||
    fieldType === CUSTOM_FIELD_TYPE.RADIO
    ? undefined
    : "multiple";
}

function selectValue(node) {
  // Iteration statuses/priorities merge several ids into one option value.
  return selectMode(node.field) === "multiple"
    ? collapseOptionValues(node.values, valueOptions(node.field))
    : node.values[0];
}

function updateSelectValue(node, value) {
  node.values =
    selectMode(node.field) === "multiple"
      ? Array.isArray(value)
        ? expandOptionValues(value)
        : []
      : value === undefined || value === null || value === ""
        ? []
        : [String(value)];
}

function updateCascaderValue(node, values) {
  const normalized = (Array.isArray(values) ? values : []).map(String);
  node.values =
    fieldDescriptor(node.field)?.fieldType ===
      CUSTOM_FIELD_TYPE.CASCADER_SINGLE && normalized.length
      ? [normalized.at(-1)]
      : normalized;
}

function valueOptions(field) {
  if (field === "priority") {
    return [
      { value: "0", label: t("search_empty_value") },
      ...options.priorities.value.map((priority) => ({
        value: String(priority.id),
        label: priority.label,
      })),
    ];
  }
  if (field === "version") {
    return [
      { value: "0", label: t("search_empty_value") },
      ...options.versions.value.map((version) => ({
        value: String(version.id),
        label: version.name,
      })),
    ];
  }
  if (field === "assignee" || field === "creator") {
    return [
      { value: "__me__", label: t("view_current_user") },
      {
        value: VIEW_VALUE_CURRENT_ENTERPRISE_ROLE,
        label: t("view_current_enterprise_role"),
      },
      ...options.members.value.map((member) => ({
        value: String(member.user_id),
        label: member.user_name,
      })),
    ];
  }
  if (field === "category") {
    return [
      { value: "0", label: t("requirement_categoryUncategorized") },
      ...options.categories.value.map((category) => ({
        value: String(category.id),
        label: category.name,
      })),
    ];
  }
  const descriptor = fieldDescriptor(field);
  if (descriptor?.options?.length) return descriptor.options;
  if (
    descriptor?.control === "person" ||
    isMemberFieldType(descriptor?.fieldType)
  ) {
    const members =
      descriptor?.kind === "custom"
        ? filterMembersForField(
            descriptor,
            options.customFieldMembers?.value || [],
          )
        : options.members.value;
    return [
      {
        value: VIEW_VALUE_CURRENT_ENTERPRISE_ROLE,
        label: t("view_current_enterprise_role"),
      },
      ...members.map((member) => ({
        value: String(member.user_id),
        label: member.user_name,
      })),
    ];
  }
  return [];
}

function filterOption(input, option) {
  return String(option.label)
    .toLowerCase()
    .includes(input.trim().toLowerCase());
}

function showValueInput(node) {
  return node.operator !== "empty" && node.operator !== "not_empty";
}

function isDateField(field) {
  return VIEW_DATE_FIELDS.includes(field);
}

function isHourOperator(operator) {
  return operator === "within_hours" || operator === "older_than_hours";
}

function valueControl(node) {
  if (isHourOperator(node.operator)) return "number-single";
  const descriptor = fieldDescriptor(node.field);
  if (isCascaderFieldType(descriptor?.fieldType)) return "cascader";
  if (node.field === "title" || node.field === "id") return "text";
  if (descriptor?.control === "text") return "text";
  if (
    isNumericFieldType(descriptor?.fieldType) &&
    node.operator !== "between"
  ) {
    return "number-single";
  }
  if (
    node.operator === "between" &&
    (descriptor?.control === "date" ||
      descriptor?.control === "number" ||
      ["planned_start_date", "planned_end_date", "created_at"].includes(
        node.field,
      ))
  ) {
    return descriptor?.control === "number" ? "number" : "date";
  }
  return "select";
}

function titlePlaceholder(node) {
  return node.operator === "contains" || node.operator === "not_contains"
    ? t("view_title_placeholder")
    : t("view_title_input_placeholder");
}

// Switching the field resets the operator to the field's first rule and clears values.
function handleFieldChange(node) {
  node.operator =
    fieldDescriptor(node.field)?.operator ||
    VIEW_FILTER_FIELD_OPERATORS[node.field]?.[0] ||
    "in";
  node.values = [];
}

// Date operators use different value formats, so switching between them resets the input.
function handleOperatorChange(node) {
  if (!showValueInput(node) || isDateField(node.field)) node.values = [];
}

function insertAfter(index) {
  props.group.conditions.splice(index + 1, 0, newViewFilterCondition());
}

function removeAt(index) {
  props.group.conditions.splice(index, 1);
}

function appendCondition() {
  props.group.conditions.push(newViewFilterCondition());
}

function appendGroup() {
  props.group.conditions.push(newViewFilterGroup());
}
</script>

<style scoped lang="less">
.view-filter-group {
  position: relative;
}
.view-filter-group--nested {
  border: 1px solid #f0f0f0;
  border-radius: 6px;
  padding: 8px 32px 8px 8px;
  margin: 4px 0;
  background: rgba(0, 0, 0, 0.02);
}
.view-filter-group__remove {
  position: absolute;
  top: 4px;
  right: 4px;
  color: rgba(0, 0, 0, 0.45);
}
.view-filter-group__body {
  display: flex;
  align-items: stretch;
  gap: 8px;
}
.view-filter-group__logic {
  display: flex;
  align-items: center;
}
.view-filter-group__logic-select {
  width: 72px;
}
.view-filter-group__conditions {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  border-left: 1px solid #f0f0f0;
  padding-left: 8px;
}
.view-filter-row {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 8px;
}
.view-filter-row__field {
  width: 132px;
  flex-shrink: 0;
}
.view-filter-row__operator {
  width: 132px;
  flex-shrink: 0;
}
.view-filter-row__value {
  flex: 1;
  /* Shrinkable so deep nesting never pushes the +/- buttons out of the group box. */
  min-width: 0;
  position: relative;
}
.view-filter-row__value--hidden {
  border: none;
}
.view-filter-row__ops {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.view-filter-group__fallback {
  align-self: flex-start;
}
.view-filter-group__footer {
  display: flex;
  gap: 8px;
  margin-top: 16px;
}
</style>

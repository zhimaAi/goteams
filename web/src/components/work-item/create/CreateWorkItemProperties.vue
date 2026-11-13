<template>
  <div class="create-right">
    <h5 class="properties-title">{{ $t("requirement_workItemProperties") }}</h5>
    <div class="prop-grid">
      <span class="prop-label" :title="relationLabel">{{ relationLabel }}</span>
      <RequirementRelationPicker
        :model-value="parentId"
        :ws-id="wsId"
        :category-id="type === 'requirement' ? categoryId : undefined"
        :mode="type === 'defect' ? 'related' : 'parent'"
        @update:model-value="$emit('update:parentId', $event)"
      />

      <template v-for="item in items" :key="item.key">
        <span
          class="prop-label"
          :class="{ required: item.required }"
          :title="item.label"
        >
          {{ item.label }}
        </span>
        <a-select
          v-if="item.key === 'status'"
          :value="status"
          :disabled="!item.editable"
          size="small"
          @update:value="$emit('update:status', $event)"
        >
          <a-select-option
            v-for="option in statuses"
            :key="option.id"
            :value="option.id"
          >
            {{ displayStatusName(option.name) }}
          </a-select-option>
        </a-select>
        <a-select
          v-else-if="item.key === 'priority'"
          :value="priority"
          :disabled="!item.editable"
          allow-clear
          size="small"
          @update:value="$emit('update:priority', $event)"
        >
          <a-select-option
            v-for="option in priorities"
            :key="option.id"
            :value="option.id"
          >
            {{ option.label }}
          </a-select-option>
        </a-select>
        <a-select
          v-else-if="item.key === 'version'"
          :value="versionId"
          :disabled="!item.editable"
          allow-clear
          size="small"
          :placeholder="$t('requirement_selectVersion')"
          @update:value="$emit('update:versionId', $event)"
        >
          <a-select-option
            v-for="option in versions"
            :key="option.id"
            :value="option.id"
          >
            <IterationLabel
              :name="option.name"
              :cross-workspace="option.iteration_type === 'cross'"
            />
          </a-select-option>
        </a-select>
        <a-select
          v-else-if="item.key === 'category'"
          :value="categoryId"
          :disabled="!item.editable"
          size="small"
          @update:value="$emit('update:categoryId', $event)"
        >
          <a-select-option
            v-for="option in categories"
            :key="option.value"
            :value="option.value"
          >
            {{ option.label }}
          </a-select-option>
        </a-select>
        <div v-else-if="item.key === 'assignee'" class="create-assignee-field">
          <AssigneeSelect
            :value="assigneeIds"
            :members="members"
            :disabled="!item.editable"
            size="small"
            :default-open="false"
            @save="$emit('update:assigneeIds', $event)"
          />
        </div>
        <PlannedDatePicker
          v-else-if="item.key === 'planned_start_date'"
          :value="plannedStartDate"
          :disabled="!item.editable"
          :default-picker-value="dayjs()"
          size="small"
          :placeholder="$t('requirement_selectDate')"
          @change="$emit('update:plannedStartDate', $event)"
        />
        <a-input-number
          v-else-if="item.key === 'estimated_work_hours'"
          :value="estimatedWorkHours"
          :disabled="!item.editable"
          :min="0"
          :max="99999999.99"
          :precision="2"
          size="small"
          @update:value="$emit('update:estimatedWorkHours', $event ?? 0)"
        >
          <template #addonAfter>{{
            $t("requirement_workHoursSuffix")
          }}</template>
        </a-input-number>
        <span v-else-if="item.key === 'work_progress'" class="readonly-value">
          0%
        </span>
        <span
          v-else-if="
            [
              'completed_work_hours',
              'remaining_work_hours',
              'overtime_work_hours',
            ].includes(item.key)
          "
          class="readonly-value"
        >
          0{{ $t("requirement_workHoursUnit") }}
        </span>
        <PlannedDatePicker
          v-else-if="item.key === 'planned_end_date'"
          :value="plannedEndDate"
          :disabled="!item.editable"
          :default-picker-value="dayjs()"
          size="small"
          :placeholder="$t('requirement_selectDate')"
          @change="$emit('update:plannedEndDate', $event)"
        />
        <CustomFieldControl
          v-else
          :model-value="customValues[item.field.id]"
          :field="item.field"
          :members="customFieldMembers"
          @update:model-value="updateCustom(item.field.id, $event)"
        />
      </template>
    </div>
  </div>
</template>

<script setup>
import { computed } from "vue";
import dayjs from "dayjs";
import { useI18n } from "vue-i18n";
import AssigneeSelect from "@/components/common/AssigneeSelect.vue";
import IterationLabel from "@/components/iteration/IterationLabel.vue";
import PlannedDatePicker from "@/components/common/PlannedDatePicker.vue";
import RequirementRelationPicker from "@/components/common/RequirementRelationPicker.vue";
import CustomFieldControl from "@/components/common/CustomFieldControl.vue";

const props = defineProps({
  type: { type: String, default: "requirement" },
  wsId: { type: String, default: "" },
  categoryId: { type: Number, default: 0 },
  parentId: { type: Number, default: undefined },
  items: { type: Array, default: () => [] },
  statuses: { type: Array, default: () => [] },
  priorities: { type: Array, default: () => [] },
  versions: { type: Array, default: () => [] },
  categories: { type: Array, default: () => [] },
  members: { type: Array, default: () => [] },
  customFieldMembers: { type: Array, default: () => [] },
  status: { type: [Number, String], default: undefined },
  priority: { type: [Number, String], default: undefined },
  versionId: { type: [Number, String], default: undefined },
  assigneeIds: { type: Array, default: () => [] },
  plannedStartDate: { type: Object, default: null },
  plannedEndDate: { type: Object, default: null },
  estimatedWorkHours: { type: Number, default: 0 },
  customValues: { type: Object, default: () => ({}) },
});
const emit = defineEmits([
  "update:parentId",
  "update:status",
  "update:priority",
  "update:versionId",
  "update:assigneeIds",
  "update:plannedStartDate",
  "update:plannedEndDate",
  "update:estimatedWorkHours",
  "update:categoryId",
  "update:customValues",
]);
const { t } = useI18n();
const relationLabel = computed(() =>
  t(
    props.type === "defect"
      ? "requirement_relatedRequirement"
      : "requirement_parentReq",
  ),
);

function displayStatusName(name) {
  return name?.startsWith("workitem.")
    ? t(name.replaceAll(".", "_"))
    : name || "-";
}
function updateCustom(id, value) {
  emit("update:customValues", { ...props.customValues, [id]: value });
}
</script>

<style scoped lang="less">
.create-right {
  width: 340px;
  overflow: auto;
  flex-shrink: 0;
  background: var(--app-surface);
}
.properties-title {
  height: 45px;
  margin: 0;
  padding: 17px 14px 12px;
  border-bottom: 1px solid #e6eaf0;
  color: #30394a;
  font-size: 14px;
  line-height: 16px;
}
.prop-grid {
  display: grid;
  grid-template-columns: minmax(72px, 108px) minmax(0, 1fr);
  gap: 12px 10px;
  padding: 12px 14px 24px;

  > :not(.prop-label) {
    width: 100%;
    min-width: 0;
  }
}
.prop-label {
  position: relative;
  min-width: 0;
  padding-left: 8px;
  overflow: hidden;
  color: #98a2b3;
  font-size: 12px;
  line-height: 28px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.prop-label.required::before {
  position: absolute;
  left: 0;
  color: #ff4d4f;
  content: "*";
}
.prop-grid :deep(.relation-button) {
  justify-content: flex-start;
  border-color: #dce2ea;
  color: var(--app-text-secondary);
}
.prop-grid :deep(.relation-button .anticon) {
  display: none;
}
.create-assignee-field {
  min-width: 0;
}
.readonly-value {
  color: var(--app-text-secondary);
  font-size: 13px;
  line-height: 28px;
}
.create-assignee-field :deep(.assignee-select) {
  box-sizing: border-box;
  min-height: 26px;
  padding: 1px 7px;
  border-color: #d9d9d9;
  border-radius: 4px;
  box-shadow: none;
}
@media (max-width: 900px) {
  .create-right {
    width: 100%;
    overflow: visible;
  }
}
</style>

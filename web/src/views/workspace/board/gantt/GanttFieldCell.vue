<!-- GanttFieldCell.vue：甘特图左侧内置字段单元格。 -->
<template>
  <div class="gantt-field-cell" :title="textTitle">
    <template v-if="field.cellType === cellTypes.TITLE">
      <WorkItemTypePill :type="item.type" :ws-id="wsId" />
      <span class="gantt-field-cell__text">{{ item.title || "-" }}</span>
    </template>
    <StatusPill
      v-else-if="field.cellType === cellTypes.STATUS"
      :text="item.status_name"
      :color="item.status_color"
    />
    <PriorityPill
      v-else-if="field.cellType === cellTypes.PRIORITY"
      :priority="item.priority"
      :label="item.priority_name"
      :color="item.priority_color"
    />
    <AssigneeCell
      v-else-if="field.cellType === cellTypes.ASSIGNEE"
      :ids="item.assignee_ids"
      :names="item.assignee_names"
      :avatars="item.assignee_avatars"
      :usernames="item.assignee_usernames"
    />
    <span v-else class="gantt-field-cell__text">{{ displayValue }}</span>
  </div>
</template>

<script setup>
import { computed } from "vue";
import AssigneeCell from "@/components/common/AssigneeCell.vue";
import WorkItemTypePill from "@/components/common/WorkItemTypePill.vue";
import PriorityPill from "@/components/work-item/PriorityPill.vue";
import StatusPill from "@/components/work-item/StatusPill.vue";
import { formatLocalDate } from "@/utils/date";
import { GANTT_FIELD_CELL_TYPES } from "@/constants/gantt";

const cellTypes = GANTT_FIELD_CELL_TYPES;
const props = defineProps({
  item: { type: Object, required: true },
  field: { type: Object, required: true },
  wsId: { type: [String, Number], default: "" },
});

const textFieldValue = {
  id: (item) => `#${item.id}`,
  version: (item) => item.version_name || "-",
  planned_start_date: (item) => formatLocalDate(item.planned_start_date),
  planned_end_date: (item) => formatLocalDate(item.planned_end_date),
  created_at: (item) => formatLocalDate(item.created_at, true),
  created_by: (item) => item.creator_name || "-",
};
const displayValue = computed(
  () => textFieldValue[props.field.configKey]?.(props.item) || "-",
);

const textTitle = computed(() => {
  if (props.field.cellType === cellTypes.TITLE) return props.item.title || "";
  return props.field.cellType === cellTypes.TEXT && displayValue.value !== "-"
    ? displayValue.value
    : "";
});
</script>

<style scoped lang="less">
.gantt-field-cell {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 8px;
  overflow: hidden;
}
.gantt-field-cell__text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>

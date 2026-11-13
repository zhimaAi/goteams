<!--
 * GanttFilterSection.vue
 * One filter section of the gantt view editor (common / requirement / defect).
 * Provides its own option sources so the nested ViewFilterGroup tree reads
 * this section's fields instead of the modal-level ones.
 -->
<template>
  <div class="gantt-filter-section">
    <div class="gantt-filter-section__head">
      <strong>{{ title }}</strong>
    </div>
    <ViewFilterGroup root :group="group" :type-key="typeKey" />
  </div>
</template>

<script setup>
import { computed, provide } from "vue";
import ViewFilterGroup from "@/components/work-item/ViewFilterGroup.vue";

const props = defineProps({
  title: { type: String, default: "" },
  // Group node ({ logic, conditions }) edited in place by ViewFilterGroup.
  group: { type: Object, required: true },
  // Effective work item type for field gating inside ViewFilterGroup.
  typeKey: { type: String, default: "requirement" },
  statuses: { type: Array, default: () => [] },
  priorities: { type: Array, default: () => [] },
  members: { type: Array, default: () => [] },
  versions: { type: Array, default: () => [] },
  categories: { type: Array, default: () => [] },
  fields: { type: Array, default: () => [] },
});

// Shadows the ViewEditModal-level provide for this subtree.
provide("viewFilterOptions", {
  statuses: computed(() => props.statuses),
  priorities: computed(() => props.priorities),
  members: computed(() => props.members),
  versions: computed(() => props.versions),
  categories: computed(() => props.categories),
  fields: computed(() => props.fields),
});
</script>

<style scoped lang="less">
.gantt-filter-section {
  border: 1px solid #f0f0f0;
  border-radius: 8px;
  padding: 12px 16px;
  & + & {
    margin-top: 12px;
  }
}
.gantt-filter-section__head {
  margin-bottom: 12px;
}
</style>

<!--
 * GanttFilterBar.vue
 * Filter row above the gantt chart. Reuses the requirement list's controls
 * (title search, checkbox panels, assignee picker) and mirrors its
 * .search-condition box styling so both pages read the same.
 -->
<template>
  <div class="gantt-filterbar">
    <div
      class="gantt-filter-condition gantt-filter-condition--title"
      :class="{ 'gantt-filter-condition--active': !!filters.keyword.trim() }"
    >
      <a-input-search
        :value="filters.keyword"
        class="gantt-filter-title-input"
        :placeholder="$t('gantt_search_placeholder')"
        allow-clear
        @update:value="$emit('update:keyword', $event)"
        @search="$emit('search')"
        @change="$emit('keyword-change')"
      />
    </div>
    <div
      class="gantt-filter-condition"
      :class="{
        'gantt-filter-condition--active': filters.statusKeys.length > 0,
      }"
    >
      <span class="gantt-filter-condition__label">{{
        $t("requirement_filterStatus")
      }}</span>
      <CheckboxMultiSelectPanel
        :value="filters.statusKeys"
        :options="statusOptions"
        @update:value="(value) => $emit('apply', 'statusKeys', value)"
      />
    </div>
    <div
      class="gantt-filter-condition"
      :class="{
        'gantt-filter-condition--active': filters.priorityKeys.length > 0,
      }"
    >
      <span class="gantt-filter-condition__label">{{
        $t("requirement_priority")
      }}</span>
      <CheckboxMultiSelectPanel
        :value="filters.priorityKeys"
        :options="priorityOptions"
        @update:value="(value) => $emit('apply', 'priorityKeys', value)"
      />
    </div>
    <div
      class="gantt-filter-condition gantt-filter-condition--member"
      :class="{
        'gantt-filter-condition--active': filters.assigneeIds.length > 0,
      }"
    >
      <span class="gantt-filter-condition__label">{{
        $t("requirement_filterAssignee")
      }}</span>
      <AssigneeSelect
        :value="filters.assigneeIds"
        :members="members"
        :placeholder="$t('common_all')"
        :default-open="false"
        :popup-width="320"
        compact
        @save="(value) => $emit('apply', 'assigneeIds', value)"
      />
    </div>
    <div
      class="gantt-filter-condition"
      :class="{ 'gantt-filter-condition--active': !!filters.type }"
    >
      <span class="gantt-filter-condition__label">{{
        $t("requirement_type")
      }}</span>
      <a-select
        :value="filters.type"
        class="gantt-filter-condition__select"
        :options="typeOptions"
        :bordered="false"
        @update:value="$emit('update:type', $event)"
        @change="$emit('search')"
      />
    </div>
    <a-button type="link" :disabled="!hasActiveValues" @click="$emit('clear')">
      {{ $t("gantt_clear_filters") }}
    </a-button>
  </div>
</template>

<script setup>
import AssigneeSelect from "@/components/common/AssigneeSelect.vue";
import CheckboxMultiSelectPanel from "@/components/work-item/CheckboxMultiSelectPanel.vue";

defineProps({
  // Reactive filter state owned by GanttPanel.
  filters: { type: Object, required: true },
  statusOptions: { type: Array, default: () => [] },
  priorityOptions: { type: Array, default: () => [] },
  typeOptions: { type: Array, default: () => [] },
  members: { type: Array, default: () => [] },
  hasActiveValues: { type: Boolean, default: false },
});

defineEmits({
  // Keyword box is bound one-way so the panel keeps ownership of the state.
  "update:keyword": (value) => true,
  "update:type": (value) => true,
  // Debounced refresh after typing.
  "keyword-change": () => true,
  // Immediate refresh (enter key / search button / type change).
  search: () => true,
  // Multi-value control changed, payload: (filterKey, values).
  apply: (key, values) => true,
  clear: () => true,
});
</script>

<style scoped lang="less">
.gantt-filterbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 0 8px;
  flex-wrap: wrap;
  flex-shrink: 0;
}
// Condition boxes mirror WorkItemSearchBar's .search-condition so the gantt
// filter row reads exactly like the requirement/defect one.
.gantt-filter-condition {
  position: relative;
  display: inline-flex;
  align-items: center;
  width: 122px;
  height: 30px;
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

  &--active:not(.gantt-filter-condition--title) {
    border-color: #73a5ff;
    background: #f0f5ff;
  }

  &--title {
    width: 212px;
    border: 0;
    background: transparent;
    box-shadow: none;
  }
}
.gantt-filter-condition__label {
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
.gantt-filter-condition__select {
  position: absolute;
  inset: 0;
  width: 100%;

  :deep(.ant-select-selector) {
    height: 28px !important;
    padding: 0 20px 0 64px !important;
    background: transparent !important;
  }

  :deep(.ant-select-arrow) {
    right: 8px;
  }

  :deep(.ant-select-selection-item),
  :deep(.ant-select-selection-placeholder) {
    // antd adds its own right padding for the arrow on top of the selector
    // padding, which would clip the value inside a 122px condition box.
    padding-right: 0 !important;
    color: var(--app-text-secondary);
    font-size: 12px;
    line-height: 28px !important;
  }
}
.gantt-filter-condition--member {
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
.gantt-filter-title-input {
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
.gantt-filter-condition--title.gantt-filter-condition--active {
  :deep(.ant-input-affix-wrapper),
  :deep(.ant-input-group-addon .ant-btn) {
    border-color: #73a5ff;
    background: #f0f5ff;
  }
}
</style>

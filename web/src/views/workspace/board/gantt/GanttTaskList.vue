<!-- GanttTaskList.vue：甘特图左侧可配置字段列表。 -->
<template>
  <div class="gantt-list" :style="{ width: `${paneWidth}px` }">
    <div ref="headEl" class="gantt-list-head">
      <div class="gantt-list-grid" :style="listGridStyle">
        <div class="gantt-list-head-cell gantt-list-action-cell" />
        <div
          v-for="field in fields"
          :key="field.preferenceKey"
          class="gantt-list-head-cell"
          :title="field.label"
        >
          {{ field.label }}
        </div>
      </div>
    </div>
    <div ref="bodyEl" class="gantt-list-body" @scroll="handleScroll">
      <div class="gantt-list-content" :style="{ width: `${contentWidth}px` }">
        <div
          v-for="(item, index) in items"
          :key="item.id"
          class="gantt-row gantt-list-grid"
          :class="{ 'gantt-row--hover': hoverIndex === index }"
          :style="listGridStyle"
          @mouseenter="$emit('update:hoverIndex', index)"
          @mouseleave="$emit('update:hoverIndex', -1)"
          @click="$emit('open-detail', item)"
        >
          <div class="gantt-list-action-cell">
            <WorkItemRowActionMenu
              :item="item"
              :can-create-child="canCreateChild(item)"
              :can-duplicate="canDuplicate(item)"
              :can-delete="canDelete(item)"
              :can-copy="canCopy(item)"
              @action="(key, record) => $emit('row-action', key, record)"
            >
              <button
                type="button"
                class="gantt-row-menu-trigger"
                :title="$t('common_more')"
                :aria-label="$t('common_more')"
                @click.stop
              >
                <EllipsisOutlined />
              </button>
            </WorkItemRowActionMenu>
          </div>
          <GanttFieldCell
            v-for="field in fields"
            :key="field.preferenceKey"
            :item="item"
            :field="field"
            :ws-id="wsId"
          />
        </div>
        <div v-if="!items.length" class="gantt-empty">
          <a-empty :image="simpleImage" :description="$t('gantt_empty')" />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref } from "vue";
import { Empty } from "ant-design-vue";
import { EllipsisOutlined } from "@ant-design/icons-vue";
import WorkItemRowActionMenu from "@/components/common/WorkItemRowActionMenu.vue";
import GanttFieldCell from "./GanttFieldCell.vue";

const props = defineProps({
  items: { type: Array, default: () => [] },
  fields: { type: Array, default: () => [] },
  width: { type: Number, default: 680 },
  hoverIndex: { type: Number, default: -1 },
  wsId: { type: [String, Number], default: "" },
  canCreateByType: { type: Object, default: () => ({}) },
  canDeleteByType: { type: Object, default: () => ({}) },
  canCopyByType: { type: Object, default: () => ({}) },
});

const emit = defineEmits({
  "update:hoverIndex": (index) => Number.isInteger(index),
  scroll: () => true,
  "open-detail": (item) => Boolean(item?.id),
  "row-action": (key, item) => Boolean(key && item?.id),
});

const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;
const bodyEl = ref(null);
const headEl = ref(null);
const actionColumnWidth = 36;
const contentWidth = computed(
  () =>
    actionColumnWidth +
    props.fields.reduce(
      (total, field) => total + Number(field.width || 120),
      0,
    ),
);
const paneWidth = computed(() => Math.min(props.width, contentWidth.value));
const listGridStyle = computed(() => ({
  width: `${contentWidth.value}px`,
  gridTemplateColumns: [
    `${actionColumnWidth}px`,
    ...props.fields.map((field) => `${Number(field.width || 120)}px`),
  ].join(" "),
}));

function canCreateChild(item) {
  return item.type === "requirement" && !!props.canCreateByType.requirement;
}
function canCopy(item) {
  return !!props.canCopyByType[item.type];
}
function canDuplicate(item) {
  return canCopy(item) && !!props.canCreateByType[item.type];
}
function canDelete(item) {
  return !!props.canDeleteByType[item.type];
}

function handleScroll() {
  if (headEl.value && bodyEl.value) {
    headEl.value.scrollLeft = bodyEl.value.scrollLeft;
  }
  emit("scroll");
}

defineExpose({ bodyEl });
</script>

<style scoped lang="less">
@import "./gantt-vars.less";

.gantt-list {
  display: flex;
  min-width: 0;
  flex-shrink: 0;
  flex-direction: column;
  border-right: 1px solid @border-color;
}
.gantt-list-head {
  height: @head-height;
  flex-shrink: 0;
  overflow: hidden;
  border-bottom: 1px solid @border-color;
  background: @surface;
}
.gantt-list-grid {
  display: grid;
  box-sizing: border-box;
}
.gantt-list-head .gantt-list-grid {
  height: 100%;
  padding-top: @head-group-height;
}
.gantt-list-head-cell {
  min-width: 0;
  overflow: hidden;
  border-top: 1px solid @border-color;
  border-right: 1px solid @border-color;
  padding: 0 12px;
  font-weight: 600;
  line-height: @head-height - @head-group-height - 1px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.gantt-list-action-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
}
.gantt-list-body {
  flex: 1;
  overflow: auto;
  padding-bottom: 14px;
  scrollbar-width: thin;

  &::-webkit-scrollbar {
    width: 0;
    height: 8px;
  }
  &::-webkit-scrollbar-thumb {
    border-radius: 4px;
    background: var(--app-border);
  }
}
.gantt-list-content {
  min-height: 100%;
}
.gantt-row {
  height: @row-height;
  align-items: center;
  border-bottom: 1px solid @border-color;
  cursor: pointer;

  > :not(.gantt-list-action-cell) {
    padding: 0 12px;
  }
  &--hover {
    background: @row-hover-bg;
  }
  &--hover :deep(.gantt-row-menu-trigger) {
    opacity: 1;
  }
}
.gantt-row-menu-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  padding: 0;
  border: 0;
  border-radius: 4px;
  color: var(--app-text-secondary);
  background: transparent;
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.15s;

  &:hover,
  &:focus-visible {
    background: var(--app-surface-muted);
    opacity: 1;
  }
}
.gantt-empty {
  padding: 48px 0;
}
</style>

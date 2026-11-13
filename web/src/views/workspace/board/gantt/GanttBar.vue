<!--
 * GanttBar.vue
 * One work item's time bar on the timeline. Colours come from the item's status
 * (border + opaque light fill + text), the caption is the day count, and the
 * two edge handles drive rescheduling. An "open" side (missing date) is marked
 * with a dashed border and cannot be resized.
 -->
<template>
  <a-tooltip placement="top">
    <template #title>
      <div>{{ item.title }}</div>
      <div>{{ tooltip }}</div>
    </template>
    <div
      class="gantt-bar"
      :class="{
        'gantt-bar--dragging': dragging,
        'gantt-bar--open-start': bar.openStart,
        'gantt-bar--open-end': bar.openEnd,
      }"
      :style="barStyle"
      @mousedown.left.prevent="$emit('drag-start', 'move', $event)"
      @click.stop="$emit('open-detail')"
    >
      <span class="gantt-bar-label">{{ label }}</span>
      <span
        v-if="canResize"
        class="gantt-bar-handle gantt-bar-handle--left"
        @mousedown.left.stop.prevent="$emit('drag-start', 'start', $event)"
      />
      <span
        v-if="canResize"
        class="gantt-bar-handle gantt-bar-handle--right"
        @mousedown.left.stop.prevent="$emit('drag-start', 'end', $event)"
      />
    </div>
  </a-tooltip>
</template>

<script setup>
defineProps({
  item: { type: Object, required: true },
  // Resolved bar range: { start, end, openStart, openEnd }.
  bar: { type: Object, required: true },
  // Inline position/colour styles computed by useGanttBars.
  barStyle: { type: Object, default: () => ({}) },
  label: { type: String, default: "" },
  tooltip: { type: String, default: "" },
  dragging: { type: Boolean, default: false },
  canResize: { type: Boolean, default: false },
});

defineEmits({
  // Start an edge/move drag, payload: (mode, mouseEvent).
  "drag-start": (mode, event) => true,
  // Plain click on the bar.
  "open-detail": () => true,
});
</script>

<style scoped lang="less">
.gantt-bar {
  position: absolute;
  top: 8px;
  height: 24px;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding: 0 8px;
  border: 1px solid transparent;
  border-radius: 8px;
  font-size: 12px;
  line-height: 22px;
  white-space: nowrap;
  cursor: pointer;
  &:hover {
    filter: brightness(0.97);
  }
  &--dragging {
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
  }
  // An open side has no known date: mark that edge with a dashed border.
  &--open-start {
    border-left-style: dashed;
  }
  &--open-end {
    border-right-style: dashed;
  }
}
.gantt-bar-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.gantt-bar-handle {
  position: absolute;
  top: 0;
  width: 8px;
  height: 100%;
  cursor: ew-resize;
  &--left {
    left: -2px;
  }
  &--right {
    right: -2px;
  }
}
</style>

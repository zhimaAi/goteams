<!--
 * GanttTimelineHeader.vue
 * Two-tier timeline header: an upper strip grouping columns by month (or year
 * at the year scale) and a lower row of date cells. Group labels stay pinned to
 * the scroller's visible left edge while their span scrolls past.
 -->
<template>
  <div class="gantt-timeline-head">
    <div class="gantt-head-groups">
      <div
        v-for="g in headerGroups"
        :key="g.key"
        class="gantt-head-group"
        :style="{ width: g.days * dayWidth + 'px' }"
      >
        <span class="gantt-head-group-label">{{ g.label }}</span>
      </div>
    </div>
    <div class="gantt-head-cells">
      <div
        v-for="col in columns"
        :key="col.key"
        class="gantt-col-head"
        :class="{
          'gantt-col-head--today': col.isToday,
          'gantt-col-head--weekend': col.isWeekend,
        }"
        :style="{ width: col.days * dayWidth + 'px' }"
      >
        <span class="gantt-col-label">{{ col.label }}</span>
        <span v-if="col.sublabel" class="gantt-col-sublabel">{{
          col.sublabel
        }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
defineProps({
  columns: { type: Array, default: () => [] },
  headerGroups: { type: Array, default: () => [] },
  dayWidth: { type: Number, default: 44 },
});
</script>

<style scoped lang="less">
@import "./gantt-vars.less";

.gantt-timeline-head {
  position: sticky;
  top: 0;
  z-index: 3;
  height: @head-height;
  flex-shrink: 0;
  border-bottom: 1px solid @border-color;
  background: @surface;
}
.gantt-head-groups {
  display: flex;
  height: @head-group-height;
  border-bottom: 1px solid @border-color;
}
.gantt-head-group {
  flex-shrink: 0;
  box-sizing: border-box;
  border-right: 1px solid @border-color;
  // No overflow:hidden here: it would re-anchor the sticky label to this cell
  // and stop it from pinning to the scroller's visible left edge.
  white-space: nowrap;
  font-size: 11px;
  line-height: @head-group-height - 1px;
  color: #999;
}
.gantt-head-group-label {
  // Keep the month/year label visible while its span is partially scrolled out.
  position: sticky;
  left: 8px;
  display: inline-block;
  padding: 0 8px;
}
.gantt-head-cells {
  display: flex;
  height: @head-height - @head-group-height - 1px;
}
.gantt-col-head {
  flex-shrink: 0;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  color: #666;
  border-right: 1px solid @border-color;
  overflow: hidden;
  white-space: nowrap;
  &--weekend {
    background: @weekend-bg;
  }
  &--today {
    background: @today-bg;
    color: @today-color;
    font-weight: 600;
  }
}
.gantt-col-sublabel {
  font-size: 11px;
  color: #999;
}
.gantt-col-head--today .gantt-col-sublabel {
  color: @today-color;
}
</style>

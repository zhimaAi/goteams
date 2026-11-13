<!--
 * GanttChart.vue
 * Layout shell for the gantt view: left task list, draggable splitter and the
 * scrollable timeline. Row geometry and rescheduling live in useGanttBars.js;
 * the list pane is GanttTaskList.vue. This file only wires them together and
 * keeps the two panes scrolled in sync.
 -->
<template>
  <div class="gantt-chart">
    <GanttTaskList
      v-show="fields.length > 0 && !collapsed"
      ref="taskListRef"
      :items="items"
      :fields="fields"
      :width="listWidth"
      :hover-index="hoverIndex"
      :ws-id="wsId"
      :can-create-by-type="canCreateByType"
      :can-delete-by-type="canDeleteByType"
      :can-copy-by-type="canCopyByType"
      @update:hover-index="hoverIndex = $event"
      @scroll="syncFromList"
      @open-detail="$emit('open-detail', $event)"
      @row-action="(key, item) => $emit('row-action', key, item)"
    />

    <!-- Splitter -->
    <div
      v-show="fields.length > 0 && !collapsed"
      class="gantt-splitter"
      @mousedown.prevent="startListResize"
    />

    <!-- Right timeline -->
    <div class="gantt-timeline-wrap">
      <button
        v-if="fields.length > 0"
        type="button"
        class="gantt-collapse-btn"
        :title="collapsed ? $t('common_expand') : $t('common_collapse')"
        @click="collapsed = !collapsed"
      >
        <RightOutlined v-if="collapsed" />
        <LeftOutlined v-else />
      </button>
      <button
        v-if="todayOutside"
        type="button"
        class="gantt-today-float"
        :class="
          todayOutside === 'left'
            ? 'gantt-today-float--left'
            : 'gantt-today-float--right'
        "
        @click="scrollToToday"
      >
        <template v-if="todayOutside === 'left'"
          ><DoubleLeftOutlined /> {{ $t("gantt_back_to_today") }}</template
        >
        <template v-else
          >{{ $t("gantt_back_to_today") }} <DoubleRightOutlined
        /></template>
      </button>
      <div
        ref="timelineEl"
        class="gantt-timeline"
        @scroll="handleTimelineScroll"
      >
        <div class="gantt-timeline-inner" :style="{ width: totalWidth + 'px' }">
          <GanttTimelineHeader
            :columns="columns"
            :header-groups="headerGroups"
            :day-width="dayWidth"
          />
          <div class="gantt-timeline-body">
            <!-- Column backgrounds (today / weekend) -->
            <div class="gantt-col-bg-layer">
              <div
                v-for="col in columns"
                :key="col.key"
                class="gantt-col-bg"
                :class="{
                  'gantt-col-bg--today': col.isToday,
                  'gantt-col-bg--weekend': col.isWeekend,
                }"
                :style="{ width: col.days * dayWidth + 'px' }"
              />
            </div>
            <div
              v-for="(item, index) in items"
              :key="item.id"
              class="gantt-row gantt-timeline-row"
              :class="{ 'gantt-row--hover': hoverIndex === index }"
              @mouseenter="hoverIndex = index"
              @mouseleave="onRowLeave"
              @mousemove="onRowMouseMove($event, item, index)"
            >
              <button
                v-if="hoverIndex === index && barOutside(item)"
                type="button"
                class="gantt-locate-item"
                :style="locateButtonStyle(barOutside(item))"
                :title="$t('gantt_locate_start')"
                :aria-label="$t('gantt_locate_start')"
                @click.stop="scrollToItem(item)"
              >
                <DoubleLeftOutlined v-if="barOutside(item) === 'left'" />
                <DoubleRightOutlined v-else />
              </button>
              <GanttBar
                v-if="barOf(item)"
                :item="item"
                :bar="barOf(item)"
                :bar-style="barStyle(item)"
                :label="barLabel(item)"
                :tooltip="barTooltip(item)"
                :dragging="!!dragState && dragState.item.id === item.id"
                :can-resize="canResizeItem(item)"
                @drag-start="(mode, event) => startBarDrag(event, item, mode)"
                @open-detail="handleBarClick(item)"
              />
              <button
                v-else-if="
                  canEditItem(item) &&
                  hoverIndex === index &&
                  hoverDayIndex >= 0
                "
                type="button"
                class="gantt-make-time"
                :style="{ left: hoverDayIndex * dayWidth + 'px' }"
                @click.stop="makeTime(item)"
              >
                <PlusOutlined /> {{ $t("gantt_click_to_schedule") }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import {
  DoubleLeftOutlined,
  DoubleRightOutlined,
  LeftOutlined,
  PlusOutlined,
  RightOutlined,
} from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import { useGanttTimeline } from "./useGanttTimeline";
import { useGanttBars } from "./useGanttBars";
import { useGanttNavigation } from "./useGanttNavigation";
import GanttTaskList from "./GanttTaskList.vue";
import GanttTimelineHeader from "./GanttTimelineHeader.vue";
import GanttBar from "./GanttBar.vue";

const props = defineProps({
  items: { type: Array, default: () => [] },
  fields: { type: Array, default: () => [] },
  dateScale: { type: String, default: "week" },
  wsId: { type: [String, Number], default: "" },
  canEditByType: { type: Object, default: () => ({}) },
  canCreateByType: { type: Object, default: () => ({}) },
  canDeleteByType: { type: Object, default: () => ({}) },
  canCopyByType: { type: Object, default: () => ({}) },
});

const emit = defineEmits({
  // Open the work item detail drawer.
  "open-detail": (item) => true,
  // Persist new planned dates, payload: (item, startUnix, endUnix).
  "update-dates": (item, start, end) => true,
  // Row menu action key and work item.
  "row-action": (key, item) => Boolean(key && item?.id),
});

const { t } = useI18n();

const itemsRef = computed(() => props.items);
const scaleRef = computed(() => props.dateScale);
// Visible width of the timeline scroller; the timeline range stretches to fill it.
const timelineViewportWidth = ref(0);
let viewportObserver = null;
const {
  dayWidth,
  totalDays,
  totalWidth,
  columns,
  headerGroups,
  todayX,
  dayIndexOf,
  dayToUnix,
  xToDayIndex,
} = useGanttTimeline(itemsRef, scaleRef, timelineViewportWidth);

function canEditItem(item) {
  return !!props.canEditByType[item.type];
}

const {
  dragState,
  barOf,
  barStyle,
  barLabel,
  barTooltip,
  canResizeItem,
  startBarDrag,
  handleBarClick,
  teardownDrag,
} = useGanttBars({
  dayWidth,
  totalDays,
  dayIndexOf,
  canEditItem,
  t,
  onUpdateDates: (item, start, end) => emit("update-dates", item, start, end),
  onOpenDetail: (item) => emit("open-detail", item),
});

// ======================== List collapse & resize ========================
const collapsed = ref(false);
const listWidth = ref(680);
let resizing = null;

function startListResize(event) {
  resizing = {
    startX: event.clientX,
    startWidth: taskListRef.value?.bodyEl?.offsetWidth ?? listWidth.value,
  };
  document.addEventListener("mousemove", onListResize);
  document.addEventListener("mouseup", stopListResize);
}
function onListResize(event) {
  if (!resizing) return;
  const next = resizing.startWidth + (event.clientX - resizing.startX);
  listWidth.value = Math.max(280, Math.min(1000, next));
}
function stopListResize() {
  resizing = null;
  document.removeEventListener("mousemove", onListResize);
  document.removeEventListener("mouseup", stopListResize);
}

// ======================== Row hover & scroll sync ========================
const hoverIndex = ref(-1);
const hoverDayIndex = ref(-1);
const taskListRef = ref(null);
const timelineEl = ref(null);

const listBodyEl = computed(() => taskListRef.value?.bodyEl || null);

// Mirror scrollTop between the list pane and the timeline scroller; assigning an
// equal value does not re-fire the scroll event, so the mirroring terminates.
function syncFromList() {
  const listEl = listBodyEl.value;
  if (!timelineEl.value || !listEl) return;
  if (timelineEl.value.scrollTop !== listEl.scrollTop) {
    timelineEl.value.scrollTop = listEl.scrollTop;
  }
}
function handleTimelineScroll() {
  const listEl = listBodyEl.value;
  if (listEl && timelineEl.value) {
    if (listEl.scrollTop !== timelineEl.value.scrollTop) {
      listEl.scrollTop = timelineEl.value.scrollTop;
    }
  }
  updateVisibility();
}
function onRowLeave() {
  hoverIndex.value = -1;
  hoverDayIndex.value = -1;
}
function onRowMouseMove(event, item) {
  if (barOf(item)) return;
  const rect = event.currentTarget.getBoundingClientRect();
  hoverDayIndex.value = xToDayIndex(event.clientX - rect.left);
}

function makeTime(item) {
  const unix = dayToUnix(hoverDayIndex.value);
  emit("update-dates", item, unix, unix);
}

const {
  todayOutside,
  updateVisibility,
  scrollToToday,
  barOutside,
  locateButtonStyle,
  scrollToItem,
} = useGanttNavigation({
  timelineEl,
  dayWidth,
  totalDays,
  todayX,
  dayIndexOf,
  barOf,
  dateScale: scaleRef,
  itemsLength: computed(() => props.items.length),
});

onMounted(() => {
  // Track the scroller's visible width (window resize, splitter drag, list
  // collapse) so the timeline range can always fill it.
  if (timelineEl.value) {
    timelineViewportWidth.value = timelineEl.value.clientWidth;
    viewportObserver = new ResizeObserver((entries) => {
      const width = entries[0]?.contentRect?.width;
      if (width) {
        timelineViewportWidth.value = width;
        updateVisibility();
      }
    });
    viewportObserver.observe(timelineEl.value);
  }
});

onBeforeUnmount(() => {
  viewportObserver?.disconnect();
  stopListResize();
  teardownDrag();
});
</script>

<style scoped lang="less">
@import "./gantt-vars.less";

.gantt-chart {
  position: relative;
  display: flex;
  flex: 1;
  min-height: 0;
  border: 1px solid @border-color;
  border-radius: 8px;
  overflow: hidden;
}

// ---------- Splitter ----------
.gantt-splitter {
  flex-shrink: 0;
  width: 4px;
  cursor: col-resize;
  background: transparent;
  transition: background 0.15s;
  &:hover {
    background: #d9d9d9;
  }
}

// ---------- Timeline ----------
.gantt-timeline-wrap {
  position: relative;
  flex: 1;
  min-width: 0;
  display: flex;
}
.gantt-timeline {
  flex: 1;
  min-width: 0;
  // One scroller owns both axes: the vertical scrollbar stays glued to the
  // visible right edge instead of living inside the horizontally scrolled inner.
  overflow: auto;
}
.gantt-timeline-inner {
  display: flex;
  flex-direction: column;
  min-height: 100%;
}
.gantt-timeline-body {
  position: relative;
  flex: 1;
}
.gantt-col-bg-layer {
  position: absolute;
  inset: 0;
  display: flex;
  pointer-events: none;
}
.gantt-col-bg {
  flex-shrink: 0;
  box-sizing: border-box;
  border-right: 1px solid @border-color;
  height: 100%;
  &--weekend {
    background: @weekend-bg;
  }
  &--today {
    background: @today-bg;
  }
}
.gantt-row {
  height: @row-height;
  display: flex;
  align-items: center;
  box-sizing: border-box;
  border-bottom: 1px solid @border-color;
  cursor: pointer;
  &--hover {
    background: @row-hover-bg;
  }
}
.gantt-timeline-row {
  position: relative;
}

// ---------- Bars ----------
.gantt-locate-item {
  position: absolute;
  z-index: 4;
  top: 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 24px;
  padding: 0;
  border: 1px solid var(--app-border);
  border-radius: 12px;
  color: @primary-color;
  background: var(--app-surface);
  box-shadow: 0 1px 4px color-mix(in srgb, var(--app-text) 15%, transparent);
  cursor: pointer;
}
.gantt-locate-item:hover {
  border-color: @primary-color;
}
.gantt-make-time {
  position: absolute;
  top: 8px;
  height: 24px;
  padding: 0 8px;
  border: 1px dashed #91caff;
  border-radius: 4px;
  background: #e6f4ff;
  color: #1677ff;
  font-size: 12px;
  cursor: pointer;
  white-space: nowrap;
}

// ---------- Floating buttons ----------
// Matches the requirement category sidebar's collapse handle.
.gantt-collapse-btn {
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  z-index: 5;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 44px;
  padding: 0;
  border: 1px solid var(--app-border);
  border-left: 0;
  border-radius: 0 7px 7px 0;
  color: var(--app-text-secondary);
  background: var(--app-surface);
  box-shadow: 2px 0 5px rgb(31 45 61 / 7%);
  font-size: 10px;
  cursor: pointer;
  &:hover {
    color: #1677ff;
  }
}
.gantt-today-float {
  position: absolute;
  top: 8px;
  z-index: 6;
  display: flex;
  align-items: center;
  gap: 4px;
  height: 28px;
  padding: 0 10px;
  border: 1px solid #1677ff;
  border-radius: 6px;
  background: #1677ff;
  color: #fff;
  font-size: 12px;
  cursor: pointer;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  &--left {
    left: 28px;
  }
  &--right {
    right: 16px;
  }
}
</style>

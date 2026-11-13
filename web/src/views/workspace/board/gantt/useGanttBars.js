// useGanttBars.js
// Bar geometry, captions and edge-drag rescheduling for the gantt timeline.
// Kept out of GanttChart.vue so the component stays a layout shell: everything
// here is pure date/pixel math plus the transient drag state.
import { ref } from "vue";
import dayjs from "dayjs";

// A plain click always jitters a pixel or two. Without a threshold the year
// scale (3px per day) would turn that jitter into a silent date change, so a
// drag only starts once the pointer clears this distance.
const DRAG_THRESHOLD_PX = 4;
// Fill ratio of the status colour blended over white for the bar background.
const BAR_FILL_RATIO = 0.22;
const DEFAULT_BAR_COLOR = "#3582fb";
const FALLBACK_BAR_FILL = "#f6f7f9";

/**
 * A bar exists as long as one planned date is set. A missing side is "open":
 * it stretches to the timeline edge and its duration is unbounded.
 * @param {object} item work item row
 * @returns {{start:number,end:number,openStart:boolean,openEnd:boolean}|null}
 */
export function barOf(item) {
  const start = Number(item.planned_start_date) || 0;
  const end = Number(item.planned_end_date) || 0;
  if (start <= 0 && end <= 0) return null;
  return { start, end, openStart: start <= 0, openEnd: end <= 0 };
}

/** Whole days covered by the bar, or 0 when either side is open. */
export function barDays(dates) {
  if (!dates || dates.start <= 0 || dates.end <= 0) return 0;
  return (
    dayjs
      .unix(dates.end)
      .startOf("day")
      .diff(dayjs.unix(dates.start).startOf("day"), "day") + 1
  );
}

/**
 * Opaque light fill derived from the status colour. Blending against white up
 * front (instead of using alpha) keeps grid lines, weekend shading and the
 * today column from showing through the bar.
 */
export function barFill(color) {
  const hex = String(color || "").replace("#", "");
  if (!/^[0-9a-f]{6}$/i.test(hex)) return FALLBACK_BAR_FILL;
  const blend = (channel) =>
    Math.round(255 - (255 - parseInt(channel, 16)) * BAR_FILL_RATIO);
  return `rgb(${blend(hex.slice(0, 2))}, ${blend(hex.slice(2, 4))}, ${blend(
    hex.slice(4, 6),
  )})`;
}

/**
 * Build the bar helpers bound to one timeline instance.
 * @param {object} deps timeline refs/helpers and the host callbacks
 * @param {import('vue').Ref<number>} deps.dayWidth pixels per day
 * @param {import('vue').Ref<number>} deps.totalDays days covered by the timeline
 * @param {(unix:number)=>number} deps.dayIndexOf day offset of a timestamp
 * @param {(item:object)=>boolean} deps.canEditItem edit permission probe
 * @param {(key:string, ...args:any[])=>string} deps.t i18n translator
 * @param {(item:object,start:number,end:number)=>void} deps.onUpdateDates persist handler
 * @param {(item:object)=>void} deps.onOpenDetail click handler
 */
export function useGanttBars(deps) {
  const { dayWidth, totalDays, dayIndexOf, canEditItem, t } = deps;
  // During a drag, preview dates override the item's stored dates.
  const dragState = ref(null);
  let dragMoved = false;

  function previewDates(item) {
    if (dragState.value && dragState.value.item.id === item.id) {
      return { start: dragState.value.start, end: dragState.value.end };
    }
    return barOf(item);
  }

  function barStyle(item) {
    const dates = previewDates(item);
    if (!dates) return {};
    const startIndex = dates.start > 0 ? dayIndexOf(dates.start) : 0;
    const endIndex =
      dates.end > 0 ? dayIndexOf(dates.end) : totalDays.value - 1;
    const color = item.status_color || DEFAULT_BAR_COLOR;
    return {
      left: startIndex * dayWidth.value + "px",
      width: Math.max(1, endIndex - startIndex + 1) * dayWidth.value - 2 + "px",
      borderColor: color,
      background: barFill(color),
      color,
    };
  }

  // Bar caption: day count only; an open-ended bar has no countable duration.
  function barLabel(item) {
    const days = barDays(previewDates(item));
    return days > 0 ? t("gantt_duration_days", { days }) : "";
  }

  function barTooltip(item) {
    const dates = previewDates(item);
    if (!dates) return "";
    const start =
      dates.start > 0 ? dayjs.unix(dates.start).format("YYYY-MM-DD") : "-";
    const end =
      dates.end > 0 ? dayjs.unix(dates.end).format("YYYY-MM-DD") : "-";
    const days = barDays(dates);
    const range = `${start} ~ ${end}`;
    return days > 0
      ? `${range} · ${t("gantt_duration_days", { days })}`
      : range;
  }

  // Only a bar bounded on both sides can be resized by dragging its edges.
  function canResizeItem(item) {
    const dates = barOf(item);
    return !!dates && dates.start > 0 && dates.end > 0 && canEditItem(item);
  }

  function onBarDrag(event) {
    const state = dragState.value;
    if (!state) return;
    const deltaX = event.clientX - state.startX;
    if (Math.abs(deltaX) < DRAG_THRESHOLD_PX) return;
    const deltaDays = Math.round(deltaX / dayWidth.value);
    if (deltaDays !== 0) dragMoved = true;
    const shift = (unix, days) =>
      dayjs.unix(unix).startOf("day").add(days, "day").unix();
    if (state.mode === "start") {
      state.start = Math.min(shift(state.origStart, deltaDays), state.origEnd);
    } else if (state.mode === "end") {
      state.end = Math.max(shift(state.origEnd, deltaDays), state.origStart);
    } else {
      state.start = shift(state.origStart, deltaDays);
      state.end = shift(state.origEnd, deltaDays);
    }
  }

  function stopBarDrag() {
    document.removeEventListener("mousemove", onBarDrag);
    document.removeEventListener("mouseup", stopBarDrag);
    const state = dragState.value;
    if (!state) return;
    dragState.value = null;
    if (
      dragMoved &&
      (state.start !== state.origStart || state.end !== state.origEnd)
    ) {
      deps.onUpdateDates(state.item, state.start, state.end);
    }
  }

  function startBarDrag(event, item, mode) {
    if (!canResizeItem(item)) return;
    const dates = barOf(item);
    if (!dates) return;
    dragMoved = false;
    dragState.value = {
      item,
      mode,
      startX: event.clientX,
      origStart: dates.start,
      origEnd: dates.end,
      start: dates.start,
      end: dates.end,
    };
    document.addEventListener("mousemove", onBarDrag);
    document.addEventListener("mouseup", stopBarDrag);
  }

  // A drag that moved must not trigger opening the detail.
  function handleBarClick(item) {
    if (dragMoved) {
      dragMoved = false;
      return;
    }
    deps.onOpenDetail(item);
  }

  function teardownDrag() {
    document.removeEventListener("mousemove", onBarDrag);
    document.removeEventListener("mouseup", stopBarDrag);
  }

  return {
    dragState,
    barOf,
    barStyle,
    barLabel,
    barTooltip,
    canResizeItem,
    startBarDrag,
    handleBarClick,
    teardownDrag,
  };
}

// 甘特图今天与工作项时间条的横向定位。
import { nextTick, onMounted, ref, watch } from "vue";

export function useGanttNavigation({
  timelineEl,
  dayWidth,
  totalDays,
  todayX,
  dayIndexOf,
  barOf,
  dateScale,
  itemsLength,
}) {
  const timelineScrollLeft = ref(0);
  const todayOutside = ref("");

  function updateVisibility() {
    const el = timelineEl.value;
    if (!el) return;
    timelineScrollLeft.value = el.scrollLeft;
    const x = todayX.value;
    if (x < el.scrollLeft) todayOutside.value = "left";
    else if (x + dayWidth.value > el.scrollLeft + el.clientWidth) {
      todayOutside.value = "right";
    } else {
      todayOutside.value = "";
    }
  }

  function scrollToToday() {
    const el = timelineEl.value;
    if (!el) return;
    el.scrollTo({
      left: Math.max(0, todayX.value - el.clientWidth / 3),
      behavior: "smooth",
    });
  }

  function barOutside(item) {
    const dates = barOf(item);
    const viewportWidth = timelineEl.value?.clientWidth || 0;
    if (!dates || !viewportWidth) return "";
    const startIndex = dates.start > 0 ? dayIndexOf(dates.start) : 0;
    const endIndex =
      dates.end > 0 ? dayIndexOf(dates.end) : totalDays.value - 1;
    const left = startIndex * dayWidth.value;
    const right = (endIndex + 1) * dayWidth.value;
    const visibleLeft = timelineScrollLeft.value;
    const visibleRight = visibleLeft + viewportWidth;
    if (right <= visibleLeft) return "left";
    if (left >= visibleRight) return "right";
    return "";
  }

  function locateButtonStyle(side) {
    const viewportWidth = timelineEl.value?.clientWidth || 0;
    const offset = side === "left" ? 8 : Math.max(8, viewportWidth - 36);
    return { left: `${timelineScrollLeft.value + offset}px` };
  }

  function scrollToItem(item) {
    const dates = barOf(item);
    const el = timelineEl.value;
    if (!dates || !el) return;
    const target = dates.start > 0 ? dates.start : dates.end;
    el.scrollTo({
      left: Math.max(
        0,
        dayIndexOf(target) * dayWidth.value - el.clientWidth / 3,
      ),
      behavior: "smooth",
    });
  }

  watch(dateScale, () =>
    nextTick(() => {
      scrollToToday();
      updateVisibility();
    }),
  );

  let anchoredToToday = false;
  watch(itemsLength, (length) => {
    if (anchoredToToday || !length) return;
    anchoredToToday = true;
    nextTick(() => {
      const el = timelineEl.value;
      if (el) el.scrollLeft = Math.max(0, todayX.value - el.clientWidth / 3);
      updateVisibility();
    });
  });

  onMounted(() => {
    nextTick(() => {
      const el = timelineEl.value;
      if (el) {
        el.scrollLeft = Math.max(0, todayX.value - el.clientWidth / 3);
      }
      updateVisibility();
    });
  });

  return {
    todayOutside,
    updateVisibility,
    scrollToToday,
    barOutside,
    locateButtonStyle,
    scrollToItem,
  };
}

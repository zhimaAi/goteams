// useGanttTimeline.js
// Pure timeline math for the gantt chart: builds header columns for the four
// date scales and converts between days and pixel offsets. All positioning is
// day-based; a column may span 1 day (week/month), 7 days (quarter) or a whole
// month (year).
import { computed } from "vue";
import dayjs from "dayjs";

// Per-scale day width in pixels (a column's width = its day span * dayWidth).
const DAY_WIDTH = { week: 44, month: 32, quarter: 8, year: 3 };

/**
 * Build the reactive timeline model.
 * @param {import('vue').Ref<Array>} items - work items with planned_start_date / planned_end_date (unix seconds)
 * @param {import('vue').Ref<string>} scale - week | month | quarter | year
 * @param {import('vue').Ref<number>} viewportWidth - visible width of the timeline scroller in px
 * @returns timeline computed refs and coordinate helpers
 */
export function useGanttTimeline(items, scale, viewportWidth) {
  const today = () => dayjs().startOf("day");

  const dayWidth = computed(() => DAY_WIDTH[scale.value] || DAY_WIDTH.week);

  // Visible range: covers all item dates plus today, padded and aligned to
  // whole columns so the first/last header cells are complete.
  const range = computed(() => {
    let min = today().subtract(14, "day");
    let max = today().add(30, "day");
    for (const item of items.value) {
      const start = Number(item.planned_start_date) || 0;
      const end = Number(item.planned_end_date) || 0;
      if (start > 0) {
        const day = dayjs.unix(start).startOf("day");
        if (day.isBefore(min)) min = day;
      }
      if (end > 0) {
        const day = dayjs.unix(end).startOf("day");
        if (day.isAfter(max)) max = day;
      }
    }
    min = min.subtract(7, "day");
    max = max.add(14, "day");
    // Extend the tail so columns always fill the visible viewport: quarter/year
    // scales pack many days per pixel and would otherwise leave a large blank
    // area on the right of the last column.
    const width = Number(viewportWidth?.value) || 0;
    if (width > 0) {
      const perDay = DAY_WIDTH[scale.value] || DAY_WIDTH.week;
      const missingDays =
        Math.ceil(width / perDay) - (max.diff(min, "day") + 1);
      if (missingDays > 0) max = max.add(missingDays, "day");
    }
    if (scale.value === "quarter") {
      // Align to Monday..Sunday weeks.
      min = min.subtract((min.day() + 6) % 7, "day");
      max = max.add((7 - max.day()) % 7, "day");
    } else if (scale.value === "year") {
      min = min.startOf("month");
      max = max.endOf("month").startOf("day");
    }
    return { start: min, end: max };
  });

  const totalDays = computed(
    () => range.value.end.diff(range.value.start, "day") + 1,
  );
  const totalWidth = computed(() => totalDays.value * dayWidth.value);

  // Header columns for the current scale. Each column carries a groupKey/groupLabel
  // used to aggregate the upper header row (month for week/month/quarter, year for year).
  const columns = computed(() => {
    const result = [];
    const { start, end } = range.value;
    const todayDay = today();
    if (scale.value === "week" || scale.value === "month") {
      for (let day = start; !day.isAfter(end); day = day.add(1, "day")) {
        result.push({
          key: day.format("YYYY-MM-DD"),
          label: day.format("D"),
          sublabel: day.format("dd"),
          groupKey: day.format("YYYY-MM"),
          groupLabel: day.format("YYYY-MM"),
          days: 1,
          isToday: day.isSame(todayDay, "day"),
          isWeekend: day.day() === 0 || day.day() === 6,
        });
      }
    } else if (scale.value === "quarter") {
      for (let day = start; !day.isAfter(end); day = day.add(7, "day")) {
        result.push({
          key: day.format("YYYY-MM-DD"),
          label: day.format("MM-DD"),
          sublabel: "",
          groupKey: day.format("YYYY-MM"),
          groupLabel: day.format("YYYY-MM"),
          days: 7,
          isToday:
            !todayDay.isBefore(day) && todayDay.isBefore(day.add(7, "day")),
          isWeekend: false,
        });
      }
    } else {
      for (let day = start; !day.isAfter(end); day = day.add(1, "month")) {
        result.push({
          key: day.format("YYYY-MM"),
          label: day.format("YYYY-MM"),
          sublabel: "",
          groupKey: day.format("YYYY"),
          groupLabel: day.format("YYYY"),
          days: day.daysInMonth(),
          isToday: todayDay.isSame(day, "month"),
          isWeekend: false,
        });
      }
    }
    return result;
  });

  // Upper header row: adjacent columns sharing a groupKey merge into one span.
  const headerGroups = computed(() => {
    const groups = [];
    for (const col of columns.value) {
      const last = groups[groups.length - 1];
      if (last && last.key === col.groupKey) {
        last.days += col.days;
      } else {
        groups.push({
          key: col.groupKey,
          label: col.groupLabel,
          days: col.days,
        });
      }
    }
    return groups;
  });

  /** Day offset (0-based) of a unix-second timestamp from the range start. */
  function dayIndexOf(unixSeconds) {
    return dayjs
      .unix(Number(unixSeconds))
      .startOf("day")
      .diff(range.value.start, "day");
  }

  /** Left pixel offset of a unix-second timestamp. */
  function xOf(unixSeconds) {
    return dayIndexOf(unixSeconds) * dayWidth.value;
  }

  /** Unix seconds (local midnight) for a 0-based day index. */
  function dayToUnix(dayIndex) {
    return range.value.start.add(dayIndex, "day").unix();
  }

  /** Clamp helper converting a pixel offset into a 0-based day index. */
  function xToDayIndex(x) {
    const index = Math.floor(x / dayWidth.value);
    return Math.max(0, Math.min(totalDays.value - 1, index));
  }

  const todayX = computed(
    () => today().diff(range.value.start, "day") * dayWidth.value,
  );

  return {
    dayWidth,
    range,
    totalDays,
    totalWidth,
    columns,
    headerGroups,
    todayX,
    dayIndexOf,
    xOf,
    dayToUnix,
    xToDayIndex,
  };
}

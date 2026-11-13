import { onBeforeUnmount, reactive, ref } from "vue";

// Column layouts of the cross-project lists (team work / my work). Both pages
// have no server-side sort and receive the whole list at once, so sorting runs
// in memory and the layout lives in the page's existing per-user preferences.
const LIST_SORT_FIELDS = new Set([
  "status",
  "priority",
  "planned_start_date",
  "planned_end_date",
]);

// Priority sort_order is COALESCEd to this value by the list SQL; sorting must
// treat a missing row the same way so the order matches the backend one.
const MISSING_PRIORITY_ORDER = 2147483647;
const MIN_TABLE_WIDTH = 960;

function toNumber(value, fallback) {
  const number = Number(value);
  return Number.isFinite(number) ? number : fallback;
}

function numericCompare(left, right, missing, direction) {
  const a = toNumber(left, missing);
  const b = toNumber(right, missing);
  if (a === b) return 0;
  return (a - b) * (direction === "desc" ? -1 : 1);
}

// Mirrors the backend `NULLS LAST`: an unset planned date stays last in both
// directions, otherwise flipping to descending would float empty rows to the top.
function plannedDateCompare(left, right, direction) {
  const a = toNumber(left, 0);
  const b = toNumber(right, 0);
  if (!a && !b) return 0;
  if (!a) return 1;
  if (!b) return -1;
  return (a - b) * (direction === "desc" ? -1 : 1);
}

const SORT_COMPARATORS = {
  status: (a, b, direction) =>
    numericCompare(a.status_sort_order, b.status_sort_order, 0, direction),
  priority: (a, b, direction) =>
    numericCompare(
      a.priority_sort_order,
      b.priority_sort_order,
      MISSING_PRIORITY_ORDER,
      direction,
    ),
  planned_start_date: (a, b, direction) =>
    plannedDateCompare(a.planned_start_date, b.planned_start_date, direction),
  planned_end_date: (a, b, direction) =>
    plannedDateCompare(a.planned_end_date, b.planned_end_date, direction),
};

function parseColumnWidths(value) {
  if (!value || typeof value !== "object" || Array.isArray(value)) return {};
  return Object.fromEntries(
    Object.entries(value)
      .map(([key, width]) => [key, Number(width)])
      .filter(([, width]) => Number.isFinite(width) && width > 0),
  );
}

function parseColumnOrder(value) {
  if (!Array.isArray(value)) return [];
  return value.filter((key) => typeof key === "string");
}

function normalizeSort(value) {
  const field = LIST_SORT_FIELDS.has(value?.field) ? value.field : "";
  const direction = ["asc", "desc"].includes(value?.direction)
    ? value.direction
    : "";
  return field && direction
    ? { field, direction }
    : { field: "", direction: "" };
}

function clamp(value, min, max) {
  return Math.min(max, Math.max(min, value));
}

export function useListColumnLayout({
  userId,
  readPreferences,
  writePreferences,
}) {
  const stored = readPreferences(userId) || {};
  const columnWidths = reactive(parseColumnWidths(stored.columnWidths));
  const columnOrder = ref(parseColumnOrder(stored.columnOrder));
  const listSort = reactive(normalizeSort(stored.listSort));

  function persist(patch) {
    writePreferences(userId, patch);
  }

  // Resizing fires on every move; coalescing to one frame keeps a drag from
  // writing storage dozens of times.
  let persistFrame;
  function persistColumnWidths() {
    persistFrame = undefined;
    persist({
      columnWidths: parseColumnWidths(columnWidths),
      columnOrder: columnOrder.value,
      listSort: { ...listSort },
    });
  }
  function scheduleColumnWidthsPersist() {
    if (persistFrame === undefined) {
      persistFrame = requestAnimationFrame(persistColumnWidths);
    }
  }

  function resolveColumns(definitions) {
    const byKey = new Map(definitions.map((column) => [column.key, column]));
    const order = columnOrder.value;
    const keys = [
      ...order.filter((key) => byKey.has(key)),
      ...definitions
        .map((column) => column.key)
        .filter((key) => !order.includes(key)),
    ];
    return keys.map((key) => {
      const column = byKey.get(key);
      const minWidth = column.minWidth || 80;
      const maxWidth = column.maxWidth || 600;
      const storedWidth = Number(columnWidths[key]);
      const width = clamp(
        Number.isFinite(storedWidth) ? storedWidth : column.width || minWidth,
        minWidth,
        maxWidth,
      );
      return { ...column, width, minWidth, maxWidth, resizable: true };
    });
  }

  function handleResizeColumn(width, column) {
    const minWidth = column.minWidth || 80;
    const maxWidth = column.maxWidth || 600;
    columnWidths[column.key] = clamp(Number(width), minWidth, maxWidth);
    scheduleColumnWidthsPersist();
  }

  // currentKeys carries the columns of the active set; keys of the other set
  // (my work renders two different column sets) keep their stored position.
  function moveColumn(sourceKey, targetKey, side, currentKeys) {
    const keys = [...(currentKeys || [])];
    const sourceIndex = keys.indexOf(sourceKey);
    const targetIndex = keys.indexOf(targetKey);
    if (sourceIndex < 0 || targetIndex < 0) return;
    const [moved] = keys.splice(sourceIndex, 1);
    keys.splice(keys.indexOf(targetKey) + (side === "after" ? 1 : 0), 0, moved);
    columnOrder.value = [
      ...keys,
      ...columnOrder.value.filter((key) => !keys.includes(key)),
    ];
    persist({
      columnWidths: parseColumnWidths(columnWidths),
      columnOrder: columnOrder.value,
      listSort: { ...listSort },
    });
  }

  // Three-state cycle like the requirement list: asc -> desc -> unsorted.
  function cycleColumnSort(column) {
    if (!column?.sortField) return;
    let sort = { field: column.sortField, direction: "asc" };
    if (listSort.field === column.sortField) {
      sort =
        listSort.direction === "asc"
          ? { field: column.sortField, direction: "desc" }
          : { field: "", direction: "" };
    }
    Object.assign(listSort, sort);
    persist({
      columnWidths: parseColumnWidths(columnWidths),
      columnOrder: columnOrder.value,
      listSort: { ...listSort },
    });
  }

  // Sorting only applies when the rendered columns carry the field, so a sort
  // picked on another tab never reorders a list that cannot show it.
  function sortItems(items, columns) {
    const { field, direction } = listSort;
    if (!field || !direction) return items;
    const sortable = (columns || []).some(
      (column) => column.sortField === field,
    );
    const compare = SORT_COMPARATORS[field];
    if (!sortable || !compare) return items;
    // Array#sort is stable, so rows with equal keys keep the backend order.
    return [...items].sort((a, b) => compare(a, b, direction));
  }

  function tableWidth(columns) {
    return Math.max(
      MIN_TABLE_WIDTH,
      (columns || []).reduce(
        (total, column) => total + (Number(column.width) || 0),
        0,
      ),
    );
  }

  onBeforeUnmount(() => {
    if (persistFrame !== undefined) {
      cancelAnimationFrame(persistFrame);
      persistColumnWidths();
    }
  });

  return {
    columnWidths,
    columnOrder,
    listSort,
    resolveColumns,
    handleResizeColumn,
    moveColumn,
    cycleColumnSort,
    sortItems,
    tableWidth,
  };
}

<template>
  <!-- Column header of the resizable/reorderable work-item tables: label plus the
       sort toggle, and the drag source/target used to reorder columns. -->
  <div
    class="work-item-column-header"
    :class="{
      'is-dragging': drag.sourceKey === column.key,
      'is-drop-before': drag.targetKey === column.key && drag.side === 'before',
      'is-drop-after': drag.targetKey === column.key && drag.side === 'after',
      'is-sorted': isSorted,
    }"
    draggable="true"
    @dragstart="handleDragStart"
    @dragover.prevent="emitDrag('dragover', $event)"
    @drop.prevent="emitDrag('drop', $event)"
    @dragend="handleDragEnd"
  >
    <span class="work-item-column-main">
      <span class="work-item-column-label">{{ title }}</span>
      <a-tooltip
        v-if="column.sortField"
        :title="$t('workspace_cycleColumnSort')"
      >
        <button
          type="button"
          class="column-header-action column-sort-action"
          :class="{ active: isSorted }"
          :aria-label="$t('workspace_cycleColumnSort')"
          draggable="false"
          @mousedown.stop
          @click.stop="$emit('sort', column)"
        >
          <img
            class="column-sort-icon"
            :src="sortIcon"
            alt=""
            aria-hidden="true"
          />
        </button>
      </a-tooltip>
    </span>
  </div>
</template>

<script setup>
defineOptions({ name: "WorkItemColumnHeader" });

import { computed, onBeforeUnmount } from "vue";
import defaultSortIcon from "@/image/order/排序.svg";
import ascendingSortIcon from "@/image/order/升序.svg";
import descendingSortIcon from "@/image/order/降序.svg";

const props = defineProps({
  // Column title as rendered by the table.
  title: { type: String, default: "" },
  // Column definition; `key` identifies the column, `sortField` enables sorting.
  column: { type: Object, required: true },
  // Current list sort: { field, direction }.
  sort: {
    type: Object,
    default: () => ({ field: "", direction: "" }),
  },
  // Shared drag state: { sourceKey, targetKey, side }.
  drag: {
    type: Object,
    default: () => ({ sourceKey: "", targetKey: "", side: "" }),
  },
});

const emit = defineEmits(["sort", "dragstart", "dragover", "drop", "dragend"]);

const isSorted = computed(
  () =>
    Boolean(props.column.sortField) &&
    props.sort?.field === props.column.sortField,
);
const sortIcon = computed(() => {
  if (!isSorted.value) return defaultSortIcon;
  return props.sort?.direction === "desc"
    ? descendingSortIcon
    : ascendingSortIcon;
});

// The preview is appended to <body>, where scoped styles never match, so its
// look is set inline (same reason the class is only a debugging hook).
let dragPreview = null;

function createDragPreview(event) {
  const source = event.currentTarget;
  const preview = document.createElement("div");
  preview.className = "column-drag-preview";
  preview.textContent =
    source.querySelector(".work-item-column-label")?.textContent || props.title;
  Object.assign(preview.style, {
    position: "fixed",
    top: "-1000px",
    left: "-1000px",
    zIndex: "10000",
    display: "flex",
    alignItems: "center",
    height: "40px",
    padding: "0 10px",
    overflow: "hidden",
    border: "1px solid #edf0f4",
    borderRadius: "6px",
    background: "#fff",
    boxShadow: "0 4px 14px rgb(31 35 41 / 12%)",
    color: "#7c8597",
    fontSize: "14px",
    whiteSpace: "nowrap",
    pointerEvents: "none",
  });
  const headerWidth = source.getBoundingClientRect().width + 56;
  preview.style.width = `${Math.max(160, Math.min(220, headerWidth))}px`;
  document.body.appendChild(preview);
  return preview;
}

function handleDragStart(event) {
  // Sort buttons live inside the draggable header but must not start a drag.
  if (event.target.closest?.(".column-header-action")) {
    event.preventDefault();
    return;
  }
  event.dataTransfer.effectAllowed = "move";
  event.dataTransfer.setData("text/plain", props.column.key);
  dragPreview?.remove();
  dragPreview = createDragPreview(event);
  emit("dragstart", event, props.column);
}

function handleDragEnd(event) {
  dragPreview?.remove();
  dragPreview = null;
  emit("dragend", event, props.column);
}

function emitDrag(name, event) {
  emit(name, event, props.column);
}

onBeforeUnmount(() => {
  dragPreview?.remove();
  dragPreview = null;
});
</script>

<style scoped lang="less">
.work-item-column-header {
  position: relative;
  display: flex;
  align-items: center;
  width: 100%;
  min-width: 0;
  height: 24px;
  padding: 0 4px;
  cursor: grab;
  user-select: none;
  border-radius: 4px;

  &:active {
    cursor: grabbing;
  }

  &.is-dragging {
    opacity: 0.45;
  }

  &.is-drop-before::before,
  &.is-drop-after::after {
    position: absolute;
    top: -8px;
    bottom: -8px;
    width: 2px;
    background: #1677ff;
    content: "";
  }

  &.is-drop-before::before {
    left: -8px;
  }

  &.is-drop-after::after {
    right: -8px;
  }
}

.work-item-column-main {
  display: inline-flex;
  align-items: center;
  flex: 0 1 auto;
  min-width: 0;
  max-width: 100%;
  padding: 0 4px;
  border-radius: 4px;
  transition: background-color 0.12s ease;
}

.work-item-column-header:hover .work-item-column-main,
.work-item-column-header:focus-within .work-item-column-main {
  max-width: calc(100% - 28px);
  background: #f0f2f5;
}

.work-item-column-header.is-sorted .work-item-column-main {
  background: #edf4ff;
  color: #1677ff;
}

.work-item-column-header.is-sorted:hover .work-item-column-main,
.work-item-column-header.is-sorted:focus-within .work-item-column-main {
  max-width: calc(100% - 28px);
  background: #edf4ff;
}

.work-item-column-label {
  flex: 0 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.column-header-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: #7c8597;
  cursor: pointer;

  &:hover,
  &:focus-visible,
  &.active {
    background: #e9edf3;
    color: #1677ff;
  }

  &.column-sort-action:hover,
  &.column-sort-action:focus-visible,
  &.column-sort-action.active {
    background: transparent;
    color: #1677ff;
  }

  &:focus-visible {
    outline: 2px solid #91caff;
    outline-offset: 1px;
  }
}

.column-sort-action {
  width: 0;
  min-width: 0;
  overflow: hidden;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.15s ease;
}

.work-item-column-header:hover .column-sort-action,
.work-item-column-header:focus-within .column-sort-action,
.work-item-column-header.is-sorted .column-sort-action {
  width: 22px;
  opacity: 1;
  pointer-events: auto;
}

.column-sort-icon {
  display: block;
  width: 14px;
  height: 14px;
  object-fit: contain;
}

.column-sort-action.active .column-sort-icon {
  filter: brightness(0) saturate(100%) invert(37%) sepia(93%) saturate(1948%)
    hue-rotate(204deg) brightness(99%) contrast(102%);
}
</style>

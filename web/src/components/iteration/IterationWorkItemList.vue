<!-- IterationWorkItemList.vue: saved views, search, list settings, and cross-workspace rows. -->
<template>
  <section class="iteration-work-items">
    <div v-if="showToolbar" class="iteration-work-items__top">
      <ViewDropdown
        :views="views"
        :current-view-id="currentViewId"
        @select="handleViewSelect"
        @create="openCreateView"
        @config="viewConfigOpen = true"
      />
      <a-button
        type="text"
        :class="{ 'search-button--active': searchExpanded || searchHasValues }"
        @click="toggleSearch"
      >
        <FilterOutlined /> {{ $t("search_button") }}
      </a-button>
      <a-button type="text" @click="listSettingsOpen = true">
        <SettingOutlined /> {{ $t("workspace_settingsShort") }}
      </a-button>
    </div>

    <div v-if="searchExpanded" class="iteration-work-items__search">
      <WorkItemSearchBar
        :filters="searchFilters"
        type-key="iteration"
        :fields="fieldDescriptors"
        :statuses="statusOptions"
        :priorities="priorityOptions"
        :members="memberOptions"
        :versions="[]"
        :categories="[]"
        @change="handleSearchFiltersChange"
        @search="fetchItems"
        @save-as="openSaveCurrentSearch"
      />
    </div>

    <a-empty
      v-if="!iterationId"
      :description="$t('workspace_selectVersionFirst')"
      :image="simpleImage"
      class="iteration-work-items__empty"
    />
    <a-table
      ref="tableRef"
      v-else
      class="iteration-work-items__table"
      size="small"
      row-key="id"
      :loading="loading"
      :data-source="treeRows"
      :columns="columns"
      :pagination="false"
      table-layout="fixed"
      :scroll="{ x: tableWidth, y: tableScrollY }"
      :locale="{ emptyText: $t('workspace_noItemsInVersion') }"
      :custom-row="(record) => ({ onClick: () => $emit('open', record) })"
      @resizeColumn="handleResizeColumn"
    >
      <template #headerCell="{ title, column }">
        <div
          v-if="column.preferenceKey"
          class="work-item-column-header"
          :class="{
            'is-menu-open': openColumnMenuKey === column.preferenceKey,
            'is-dragging': columnDragState.sourceKey === column.preferenceKey,
            'is-drop-before':
              columnDragState.targetKey === column.preferenceKey &&
              columnDragState.side === 'before',
            'is-drop-after':
              columnDragState.targetKey === column.preferenceKey &&
              columnDragState.side === 'after',
            'is-sorted': tablePreferences.sort.field === column.sortField,
          }"
          draggable="true"
          @dragstart="startColumnDrag($event, column)"
          @dragover.prevent="updateColumnDropTarget($event, column)"
          @drop.prevent="finishColumnDrop($event, column)"
          @dragend="clearColumnDrag"
        >
          <span class="work-item-column-main">
            <button
              v-if="column.key === 'title' && requirementChildrenByParent.size"
              type="button"
              class="tree-header-toggle"
              :title="
                allTreeExpanded ? $t('common_collapse') : $t('common_expand')
              "
              :aria-label="
                allTreeExpanded ? $t('common_collapse') : $t('common_expand')
              "
              draggable="false"
              @mousedown.stop
              @click.stop="toggleAllTreeRows"
            >
              <CaretDownOutlined v-if="allTreeExpanded" />
              <CaretRightOutlined v-else />
            </button>
            <span class="work-item-column-label">{{ title }}</span>
            <a-tooltip
              v-if="column.sortField"
              :title="$t('workspace_cycleColumnSort')"
            >
              <button
                type="button"
                class="column-header-action column-sort-action"
                :class="{
                  active: tablePreferences.sort.field === column.sortField,
                }"
                :aria-label="$t('workspace_cycleColumnSort')"
                draggable="false"
                @mousedown.stop
                @click.stop="cycleColumnSort(column)"
              >
                <img
                  class="column-sort-icon"
                  :src="columnSortIcon(column)"
                  alt=""
                  aria-hidden="true"
                />
              </button>
            </a-tooltip>
          </span>
          <span class="work-item-column-actions">
            <a-dropdown
              v-if="
                canFreezeColumn(column) ||
                hasFrozenColumns ||
                canHideColumn(column)
              "
              trigger="click"
              placement="bottomRight"
              @openChange="(open) => handleColumnMenuOpenChange(open, column)"
            >
              <button
                type="button"
                class="column-header-action column-more-action"
                :aria-label="$t('common_more')"
                draggable="false"
                @mousedown.stop
                @click.stop
              >
                <MoreOutlined />
              </button>
              <template #overlay>
                <a-menu @click="({ key }) => handleColumnMenu(key, column)">
                  <a-menu-item v-if="canFreezeColumn(column)" key="freeze">
                    <LockOutlined />
                    {{ $t("workspace_freezeToColumn") }}
                  </a-menu-item>
                  <a-menu-item v-if="hasFrozenColumns" key="unfreeze">
                    <UnlockOutlined />
                    {{ $t("workspace_cancelColumnFreeze") }}
                  </a-menu-item>
                  <a-menu-divider
                    v-if="
                      (canFreezeColumn(column) || hasFrozenColumns) &&
                      canHideColumn(column)
                    "
                  />
                  <a-menu-item v-if="canHideColumn(column)" key="hide">
                    <EyeInvisibleOutlined />
                    {{ $t("workspace_hideColumn") }}
                  </a-menu-item>
                </a-menu>
              </template>
            </a-dropdown>
          </span>
        </div>
        <template v-else>{{ title }}</template>
      </template>
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'title'">
          <div class="iteration-title-cell">
            <span
              class="tree-indent"
              :style="{ width: `${14 + (record.__depth || 0) * 16}px` }"
            >
              <button
                v-if="hasTreeChildren(record)"
                type="button"
                class="tree-toggle"
                :title="
                  expandedRowKeys.includes(record.id)
                    ? $t('common_collapse')
                    : $t('common_expand')
                "
                @click.stop="toggleTreeRow(record)"
              >
                <DownOutlined v-if="expandedRowKeys.includes(record.id)" />
                <RightOutlined v-else />
              </button>
            </span>
            <WorkItemTypePill
              :type="record.type"
              :ws-id="record.workspace_id || wsId"
            />
            <span :title="record.title">{{ record.title }}</span>
          </div>
        </template>
        <WorkItemTypePill
          v-else-if="column.key === 'type'"
          :type="record.type"
          :ws-id="record.workspace_id || wsId"
        />
        <StatusPill
          v-else-if="column.key === 'status'"
          :text="record.status_name"
          :color="record.status_color"
        />
        <PriorityPill
          v-else-if="column.key === 'priority'"
          :priority="record.priority"
          :label="record.priority_name"
          :color="record.priority_color"
        />
        <AssigneeCell
          v-else-if="column.key === 'assignee'"
          :ids="record.assignee_ids"
          :names="record.assignee_names"
          :avatars="record.assignee_avatars"
          :usernames="record.assignee_usernames"
        />
        <template v-else-if="column.key === 'plannedStart'">
          {{ formatLocalDate(record.planned_start_date) }}
        </template>
        <template v-else-if="column.key === 'plannedEnd'">
          {{ formatLocalDate(record.planned_end_date) }}
        </template>
        <template v-else-if="column.key === 'createdAt'">
          {{ formatLocalDate(record.created_at, true) }}
        </template>
      </template>
    </a-table>

    <Teleport to="body">
      <div
        v-if="tableResizeGuide.visible"
        class="table-resize-guide"
        :style="{
          left: `${tableResizeGuide.left}px`,
          top: `${tableResizeGuide.top}px`,
          height: `${tableResizeGuide.height}px`,
          '--header-height': `${tableResizeGuide.headerHeight}px`,
        }"
      >
        <span class="table-resize-guide-header" />
        <span class="table-resize-guide-body" />
      </div>
    </Teleport>
    <!-- Rendered off-screen and reused as the HTML5 drag image: setDragImage needs a
         laid-out element, and keeping it in the template keeps the scoped styles on it. -->
    <Teleport to="body">
      <div
        ref="columnDragPreviewRef"
        class="column-drag-preview"
        aria-hidden="true"
      >
        {{ columnDragPreviewLabel }}
      </div>
    </Teleport>

    <a-pagination
      v-if="iterationId"
      class="iteration-work-items__pagination"
      :current="page"
      :page-size="pageSize"
      :page-size-options="['20', '40', '80', '100']"
      :total="total"
      show-size-changer
      :show-total="(count) => `${$t('common_all')} ${count}`"
      @change="handlePageChange"
    />

    <ListSettingsModal
      v-model:open="listSettingsOpen"
      :user-id="authStore.userInfo?.id"
      :ws-id="wsId"
      type-key="iteration"
      :fields="listFields"
      :current-preferences="listPreferences"
      hide-view-mode
      @change="applyListPreferences"
    />
    <ViewEditModal
      v-model:open="viewEditOpen"
      :ws-id="wsId"
      type-key="iteration"
      :mode="viewEditMode"
      :view="viewEditTarget"
      :can-manage-system-view="canManageSystemView"
      :status-options="statusOptions"
      :priority-options="priorityOptions"
      :members="memberOptions"
      :versions="[]"
      :categories="[]"
      :field-descriptors="fieldDescriptors"
      :final-status-ids="filterOptions.final_status_ids || []"
      @saved="handleViewSaved"
    />
    <ViewConfigModal
      v-model:open="viewConfigOpen"
      :ws-id="wsId"
      type-key="iteration"
      :views="views"
      :remember-last="rememberLast"
      :can-manage-system-view="canManageSystemView"
      @edit="openEditView"
      @save-as="openSaveAsView"
      @deleted="handleViewSaved"
      @saved="handleViewSaved"
    />
  </section>
</template>

<script setup>
import {
  computed,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch,
} from "vue";
import { Empty } from "ant-design-vue";
import {
  CaretDownOutlined,
  CaretRightOutlined,
  DownOutlined,
  EyeInvisibleOutlined,
  FilterOutlined,
  LockOutlined,
  MoreOutlined,
  RightOutlined,
  SettingOutlined,
  UnlockOutlined,
} from "@ant-design/icons-vue";
import { formatLocalDate } from "@/utils/date";
import defaultSortIcon from "@/image/order/排序.svg";
import ascendingSortIcon from "@/image/order/升序.svg";
import descendingSortIcon from "@/image/order/降序.svg";
import { useIterationWorkItems } from "@/composables/useIterationWorkItems";
import WorkItemTypePill from "@/components/common/WorkItemTypePill.vue";
import AssigneeCell from "@/components/common/AssigneeCell.vue";
import StatusPill from "@/components/work-item/StatusPill.vue";
import PriorityPill from "@/components/work-item/PriorityPill.vue";
import WorkItemSearchBar from "@/components/work-item/WorkItemSearchBar.vue";
import ViewDropdown from "@/components/work-item/ViewDropdown.vue";
import ViewEditModal from "@/components/work-item/ViewEditModal.vue";
import ViewConfigModal from "@/components/work-item/ViewConfigModal.vue";
import ListSettingsModal from "@/components/work-item/ListSettingsModal.vue";

const props = defineProps({
  wsId: { type: [String, Number], required: true },
  iterationId: { type: [String, Number], default: null },
  filterOptions: { type: Object, default: () => ({}) },
  refreshKey: { type: Number, default: 0 },
  showToolbar: { type: Boolean, default: true },
});
const emit = defineEmits({
  open: (record) => Boolean(record?.id && record?.workspace_id),
  navigationChange: (items) => Array.isArray(items),
  previewModeChange: (mode) => ["drawer", "modal", "window"].includes(mode),
});
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;

// The table wrapper is flex-stretched, so its measured height is the real available
// space. scroll.y is derived from it to keep the body filling that space when the
// search bar is toggled. The value is only a first-paint fallback.
const tableRef = ref(null);
const tableScrollY = ref("calc(100vh - 260px)");
let tableResizeObserver;

function updateTableScrollY() {
  const tableEl = tableRef.value?.$el;
  if (!tableEl) return;
  const headerEl =
    tableEl.querySelector(".ant-table-header") ||
    tableEl.querySelector(".ant-table-thead");
  const bodyHeight = tableEl.clientHeight - (headerEl?.offsetHeight || 0);
  if (bodyHeight > 0) tableScrollY.value = `${Math.round(bodyHeight)}px`;
}

function observeTable(instance) {
  tableResizeObserver?.disconnect();
  tableResizeObserver = undefined;
  const tableEl = instance?.$el;
  if (!tableEl || typeof ResizeObserver === "undefined") return;
  tableResizeObserver = new ResizeObserver(updateTableScrollY);
  tableResizeObserver.observe(tableEl);
  updateTableScrollY();
}

watch(tableRef, observeTable, { flush: "post" });
onMounted(() => {
  document.addEventListener("mousedown", startTableResizeGuide, true);
});
onBeforeUnmount(() => {
  tableResizeObserver?.disconnect();
  document.removeEventListener("mousedown", startTableResizeGuide, true);
  tableResizeGuideCleanup?.();
  if (closeColumnMenuTimer) {
    clearTimeout(closeColumnMenuTimer);
    closeColumnMenuTimer = null;
  }
});
const {
  authStore,
  items,
  loading,
  page,
  pageSize,
  total,
  listSettingsOpen,
  viewEditOpen,
  viewEditMode,
  viewEditTarget,
  viewConfigOpen,
  listPreferences,
  views,
  currentViewId,
  rememberLast,
  searchExpanded,
  searchFilters,
  searchHasValues,
  canManageSystemView,
  statusOptions,
  priorityOptions,
  memberOptions,
  fieldDescriptors,
  listFields,
  tablePreferences,
  hasFrozenColumns,
  columns,
  tableWidth,
  handleResizeColumn,
  persistColumnWidths,
  moveColumn,
  cycleColumnSort,
  handleColumnMenu,
  canFreezeColumn,
  canHideColumn,
  fetchItems,
  handlePageChange,
  toggleSearch,
  handleSearchFiltersChange,
  handleViewSelect,
  openCreateView,
  openEditView,
  openSaveAsView,
  openSaveCurrentSearch,
  handleViewSaved,
  applyListPreferences,
} = useIterationWorkItems(props, emit);

// Column header interactions mirror the requirement list: drag the header to reorder,
// drag the handle to resize, freeze/hide from the "..." menu, cycle the sort from the icon.
const columnDragState = reactive({ sourceKey: "", targetKey: "", side: "" });
const openColumnMenuKey = ref("");
const columnDragPreviewRef = ref(null);
const columnDragPreviewLabel = ref("");
const tableResizeGuide = reactive({
  visible: false,
  left: 0,
  top: 0,
  height: 0,
  headerHeight: 0,
});
let closeColumnMenuTimer = null;
let tableResizeGuideCleanup;

function startColumnDrag(event, column) {
  if (event.target.closest?.(".column-header-action")) {
    event.preventDefault();
    return;
  }
  columnDragState.sourceKey = column.preferenceKey;
  event.dataTransfer.effectAllowed = "move";
  event.dataTransfer.setData("text/plain", column.preferenceKey);
  const source = event.currentTarget;
  const label =
    source.querySelector(".work-item-column-label")?.textContent ||
    column.title;
  columnDragPreviewLabel.value = label;
  const preview = columnDragPreviewRef.value;
  if (!preview) return;
  // setDragImage snapshots the element immediately, so the label goes straight to the DOM.
  preview.textContent = label;
  preview.style.width = `${Math.max(160, Math.min(220, source.getBoundingClientRect().width + 56))}px`;
  event.dataTransfer.setDragImage(preview, 18, 20);
}

function updateColumnDropTarget(event, column) {
  if (!columnDragState.sourceKey) return;
  const rect = event.currentTarget.getBoundingClientRect();
  columnDragState.targetKey = column.preferenceKey;
  columnDragState.side =
    event.clientX < rect.left + rect.width / 2 ? "before" : "after";
  event.dataTransfer.dropEffect = "move";
}

function clearColumnDrag() {
  columnDragState.sourceKey = "";
  columnDragState.targetKey = "";
  columnDragState.side = "";
}

function finishColumnDrop(event, column) {
  const sourceKey = columnDragState.sourceKey;
  const targetKey = column.preferenceKey;
  const side = columnDragState.side || "before";
  clearColumnDrag();
  moveColumn(sourceKey, targetKey, side);
}

// Keep the actions container expanded while the "..." dropdown is open; on close, delay
// clearing the key so the overlay finishes its exit animation before the trigger collapses
// — otherwise vc-trigger repositions the menu and it flashes.
function handleColumnMenuOpenChange(open, column) {
  if (closeColumnMenuTimer) {
    clearTimeout(closeColumnMenuTimer);
    closeColumnMenuTimer = null;
  }
  if (open) {
    openColumnMenuKey.value = column.preferenceKey;
    return;
  }
  const key = column.preferenceKey;
  closeColumnMenuTimer = setTimeout(() => {
    if (openColumnMenuKey.value === key) openColumnMenuKey.value = "";
    closeColumnMenuTimer = null;
  }, 250);
}

function columnSortIcon(column) {
  if (tablePreferences.sort.field !== column.sortField) return defaultSortIcon;
  return tablePreferences.sort.direction === "desc"
    ? descendingSortIcon
    : ascendingSortIcon;
}

// Requirement rows whose parent is in the same iteration render as a collapsible
// tree, mirroring the requirement list; all other rows stay flat top-level rows.
const expandedRowKeys = ref([]);
const requirementChildrenByParent = computed(() => {
  const requirementIds = new Set(
    items.value
      .filter((item) => item.type === "requirement")
      .map((item) => String(item.id)),
  );
  const byParent = new Map();
  for (const item of items.value) {
    if (item.type !== "requirement") continue;
    const parentId = String(item.parent_id || "");
    if (!requirementIds.has(parentId)) continue;
    if (!byParent.has(parentId)) byParent.set(parentId, []);
    byParent.get(parentId).push(item);
  }
  return byParent;
});
const treeRows = computed(() => {
  const childrenByParent = requirementChildrenByParent.value;
  if (!childrenByParent.size) return items.value;
  const expandedIds = new Set(expandedRowKeys.value.map(String));
  const rows = [];
  const appendRow = (item, depth, ancestors) => {
    rows.push({ ...item, __depth: depth });
    const key = String(item.id);
    if (!expandedIds.has(key)) return;
    for (const child of childrenByParent.get(key) || []) {
      const childId = String(child.id);
      if (ancestors.has(childId) || childId === key) continue;
      appendRow(child, depth + 1, new Set(ancestors).add(key));
    }
  };
  for (const item of items.value) {
    const parentId = String(item.parent_id || "");
    if (item.type === "requirement" && childrenByParent.has(parentId)) continue;
    appendRow(item, 0, new Set());
  }
  return rows;
});
const allTreeExpanded = computed(() => {
  const parents = requirementChildrenByParent.value;
  if (!parents.size) return false;
  const expandedIds = new Set(expandedRowKeys.value.map(String));
  for (const parentId of parents.keys()) {
    if (!expandedIds.has(parentId)) return false;
  }
  return true;
});

function hasTreeChildren(record) {
  return requirementChildrenByParent.value.has(String(record.id));
}

function toggleTreeRow(record) {
  const id = record.id;
  expandedRowKeys.value = expandedRowKeys.value.includes(id)
    ? expandedRowKeys.value.filter((key) => key !== id)
    : [...expandedRowKeys.value, id];
}

function toggleAllTreeRows() {
  expandedRowKeys.value = allTreeExpanded.value
    ? []
    : [...requirementChildrenByParent.value.keys()];
}

// Data comes back collapsed after every fetch, like the requirement list.
watch(items, () => {
  expandedRowKeys.value = [];
});

function startTableResizeGuide(event) {
  const resizeHandle = event.target.closest?.(
    ".iteration-work-items__table .ant-table-resize-handle",
  );
  if (!resizeHandle) return;
  const table = resizeHandle
    .closest(".iteration-work-items__table")
    ?.querySelector(".ant-table-container");
  if (!table) return;
  tableResizeGuideCleanup?.();
  const rect = table.getBoundingClientRect();
  const headerRect = table
    .querySelector(".ant-table-thead")
    ?.getBoundingClientRect();
  tableResizeGuide.visible = true;
  tableResizeGuide.left = event.clientX;
  tableResizeGuide.top = rect.top;
  tableResizeGuide.height = rect.height;
  tableResizeGuide.headerHeight = Math.min(
    rect.height,
    Math.max(0, (headerRect?.bottom || rect.top) - rect.top),
  );

  const moveGuide = (moveEvent) => {
    tableResizeGuide.left = Math.min(
      rect.right,
      Math.max(rect.left, moveEvent.clientX),
    );
  };
  const stopGuide = () => {
    tableResizeGuide.visible = false;
    document.removeEventListener("mousemove", moveGuide);
    document.removeEventListener("mouseup", stopGuide);
    tableResizeGuideCleanup = undefined;
    // Written once on drop; dragging must not touch storage on every frame.
    window.requestAnimationFrame(persistColumnWidths);
  };
  tableResizeGuideCleanup = stopGuide;
  document.addEventListener("mousemove", moveGuide);
  document.addEventListener("mouseup", stopGuide);
}

function openListSettings() {
  listSettingsOpen.value = true;
}
function openViewConfig() {
  viewConfigOpen.value = true;
}

defineExpose({
  views,
  currentViewId,
  searchExpanded,
  searchHasValues,
  selectView: handleViewSelect,
  openCreateView,
  openViewConfig,
  toggleSearch,
  openListSettings,
});
</script>

<style scoped lang="less">
.iteration-work-items {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  padding: 8px 0;
  padding-bottom: 0;
}
.iteration-work-items__top {
  display: flex;
  min-height: 40px;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}
.search-button--active {
  color: #1677ff;
  background: #eaf3ff;
}
.iteration-work-items__search {
  margin-bottom: 8px;
}
.iteration-work-items__empty {
  margin-top: 96px;
}
.iteration-work-items__table {
  min-height: 0;
  flex: 1;
}
.iteration-title-cell {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 8px;

  span:last-child {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: 500;
    cursor: pointer;
  }
}
.tree-indent {
  display: inline-flex;
  flex-shrink: 0;
  justify-content: flex-end;
  width: 14px;
  margin-right: 2px;
  vertical-align: middle;
}
.tree-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 20px;
  padding: 0;
  border: 0;
  background: transparent;
  color: #7c8597;
  cursor: pointer;
  font-size: 10px;
}
.tree-toggle:hover {
  color: #1677ff;
}
.tree-header-toggle {
  display: inline-flex;
  flex: 0 0 18px;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 22px;
  margin-right: 2px;
  padding: 0;
  border: 0;
  border-radius: 3px;
  background: transparent;
  color: #7c8597;
  cursor: pointer;
  font-size: 11px;
}
.tree-header-toggle:hover,
.tree-header-toggle:focus-visible {
  background: #e9edf3;
  color: #1677ff;
}
.tree-header-toggle:focus-visible {
  outline: 2px solid #91caff;
  outline-offset: 1px;
}
.iteration-work-items__pagination {
  padding: 0;
  margin-top: 12px;
  text-align: right;
}
// antd paints its own resize line; the guide below replaces it, so the built-in one
// is hidden. `!important` is required to beat antd's inline-ish dragging state rule.
.iteration-work-items__table
  :deep(.ant-table-resize-handle.dragging .ant-table-resize-handle-line) {
  opacity: 0 !important;
}
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
.work-item-column-label {
  flex: 0 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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
.column-sort-action {
  width: 0;
  min-width: 0;
  overflow: hidden;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.15s ease;
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
.work-item-column-header:hover .column-sort-action,
.work-item-column-header:focus-within .column-sort-action,
.work-item-column-header.is-sorted .column-sort-action {
  width: 22px;
  opacity: 1;
  pointer-events: auto;
}
.work-item-column-actions {
  display: inline-flex;
  align-items: center;
  flex: 0 0 auto;
  width: 0;
  gap: 0;
  margin-left: 0;
  overflow: hidden;
  opacity: 0;
  pointer-events: none;
  transition:
    width 0.15s ease,
    gap 0.15s ease,
    margin-left 0.15s ease,
    opacity 0.15s ease;
}
.work-item-column-header:hover .work-item-column-actions,
.work-item-column-header:focus-within .work-item-column-actions,
.work-item-column-header.is-menu-open .work-item-column-actions {
  width: auto;
  gap: 2px;
  margin-left: 4px;
  opacity: 1;
  pointer-events: auto;
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

  &.column-more-action:hover,
  &.column-more-action:focus-visible {
    color: #7c8597;
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
.column-drag-preview {
  position: fixed;
  top: -1000px;
  left: -1000px;
  z-index: 10000;
  display: flex;
  align-items: center;
  height: 40px;
  padding: 0 10px;
  overflow: hidden;
  border: 1px solid #edf0f4;
  border-radius: 6px;
  background: #fff;
  box-shadow: 0 4px 14px rgb(31 35 41 / 12%);
  color: #7c8597;
  font-size: 14px;
  white-space: nowrap;
  pointer-events: none;
}
.table-resize-guide {
  position: fixed;
  z-index: 9999;
  width: 1px;
  pointer-events: none;
}
.table-resize-guide-header,
.table-resize-guide-body {
  position: absolute;
  display: block;
  background: #1677ff;
}
.table-resize-guide-header {
  top: 0;
  left: -1px;
  width: 3px;
  height: var(--header-height);
}
.table-resize-guide-body {
  top: var(--header-height);
  bottom: 0;
  left: 0;
  width: 1px;
}
</style>

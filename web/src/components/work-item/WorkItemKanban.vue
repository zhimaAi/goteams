<template>
  <div
    class="work-item-kanban"
    :class="{ 'is-dragging': Boolean(draggingItemId) }"
  >
    <div
      v-if="visibleStatuses.length"
      ref="kanbanColumnsRef"
      class="kanban-columns"
      @wheel.capture="handleColumnsWheel"
    >
      <section
        v-for="status in visibleStatuses"
        :key="status.id"
        class="kanban-column"
        :class="backgroundClass(status)"
        :data-tab-scroll-root="statusRoot(status)"
        :style="customColumnStyle(status)"
      >
        <header class="kanban-column-header">
          <span
            class="kanban-status-dot"
            :style="{
              backgroundColor:
                columnState(status.id).total > 0
                  ? status.color || '#98a2b3'
                  : '#b7bdc7',
            }"
          />
          <strong :title="status.name">{{ status.name }}</strong>
          <span class="kanban-count">{{ columnState(status.id).total }}</span>
          <a-dropdown
            :trigger="['hover']"
            placement="bottomRight"
            :open="openStatusMenuId === Number(status.id)"
            @openChange="(open) => handleStatusMenuOpen(status.id, open)"
          >
            <a-button
              type="text"
              class="kanban-more-button"
              :aria-label="$t('common_more')"
              @click.stop
            >
              <EllipsisOutlined />
            </a-button>
            <template #overlay>
              <div class="kanban-column-menu" @click.stop>
                <button
                  type="button"
                  class="kanban-column-menu-item"
                  @click="hideStatus(status.id)"
                >
                  <EyeInvisibleOutlined />
                  {{ $t("workspace_hideColumn") }}
                </button>
                <div class="kanban-color-heading">
                  {{ $t("workspace_columnBackground") }}
                </div>
                <div class="kanban-color-options">
                  <button
                    v-for="color in colorOptions"
                    :key="color"
                    type="button"
                    class="kanban-color-swatch"
                    :class="{ active: customColor(status) === color }"
                    :style="{ backgroundColor: color }"
                    :aria-label="color"
                    @click="setStatusColor(status.id, color)"
                  />
                  <button
                    type="button"
                    class="kanban-color-reset"
                    :class="{ active: !customColor(status) }"
                    :style="{ backgroundColor: defaultColumnColor(status) }"
                    :aria-label="$t('workspace_restoreDefault')"
                    @click="setStatusColor(status.id, '')"
                  >
                    <CloseOutlined />
                  </button>
                </div>
                <label class="kanban-color-picker">
                  <span>{{ $t("workspace_customColor") }}</span>
                  <input
                    type="color"
                    :value="customColor(status) || '#f8f9fb'"
                    @mousedown.stop="startColorPicker(status.id)"
                    @focus="startColorPicker(status.id)"
                    @change="handleCustomColorChange(status.id, $event)"
                    @blur="finishColorPicker(status.id)"
                  />
                </label>
              </div>
            </template>
          </a-dropdown>
        </header>

        <div
          class="kanban-card-list"
          :class="{ scrolling: scrollingColumns.has(status.id) }"
          :data-status-id="status.id"
          @scroll="handleColumnScroll($event, status.id)"
        >
          <a-spin
            v-if="
              columnState(status.id).loading &&
              !columnState(status.id).items.length
            "
            class="kanban-initial-loading"
          />
          <a-empty
            v-else-if="!columnState(status.id).items.length"
            :image="simpleImage"
            :description="$t('common_noData')"
            class="kanban-empty"
          />

          <article
            v-for="item in columnState(status.id).items"
            :key="item.id"
            class="kanban-card"
            :class="{
              draggable: canEdit,
              'detail-selected-card':
                String(item.id) === String(selectedItemId),
            }"
            :data-work-item-id="item.id"
            tabindex="0"
            @click="$emit('open', item.id)"
            @keydown.enter="$emit('open', item.id)"
          >
            <div class="kanban-card-title">
              <span class="kanban-title-text" :title="item.title">
                <span
                  v-if="item.priority_name"
                  class="kanban-priority-pill"
                  :style="{ color: item.priority_color }"
                >
                  {{ item.priority_name.slice(0, 1) }}
                </span>
                {{ item.title }}
              </span>
            </div>

            <div class="kanban-card-meta">
              <span
                v-if="assigneeNames(item)"
                class="kanban-assignee-display"
                :class="{
                  editable: canEdit && fieldEditable.assignee,
                }"
              >
                <div class="kanban-assignee-avatar">
                  <img src="@/image/avatar.svg" alt="" />
                </div>
                <span class="kanban-assignee-name">
                  {{ assigneeNames(item) || $t("workspace_unassigned") }}
                </span>
              </span>
              <span v-else></span>
              <span class="kanban-updated-at">
                {{ relativeUpdatedAt(item.updated_at) }}
              </span>
            </div>
          </article>

          <div v-if="columnState(status.id).loading" class="kanban-load-more">
            <LoadingOutlined /> {{ $t("common_loading") }}
          </div>
          <button
            v-if="columnState(status.id).error"
            type="button"
            class="kanban-retry"
            @click="loadColumn(status.id, false)"
          >
            {{ $t("workspace_loadFailedRetry") }}
          </button>
        </div>
      </section>
    </div>
    <a-empty
      v-else
      :image="simpleImage"
      :description="$t('workspace_noVisibleStatuses')"
      class="kanban-no-columns"
    />
  </div>
</template>

<script setup>
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch,
} from "vue";
import dayjs from "dayjs";
import Sortable from "sortablejs";
import { Empty, message } from "ant-design-vue";
import {
  CloseOutlined,
  EllipsisOutlined,
  EyeInvisibleOutlined,
  LoadingOutlined,
} from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import { listRequirements, updateRequirement } from "@/api/workspace";
import { formatLocalDate } from "@/utils/date";
import { showWorkItemUpdateSuccess } from "@/utils/workItemActionToast";
import { mergeWorkItem, workItemMatchesFilters } from "@/utils/workItemList";
import PlannedDatePicker from "@/components/common/PlannedDatePicker.vue";
import PriorityPill from "@/components/work-item/PriorityPill.vue";

const props = defineProps({
  wsId: { type: [String, Number], required: true },
  typeKey: { type: String, required: true },
  statuses: { type: Array, default: () => [] },
  statusPreferences: { type: Array, default: () => [] },
  boardSort: {
    type: Object,
    default: () => ({ field: "updated_at", direction: "asc" }),
  },
  boardColors: { type: Object, default: () => ({}) },
  priorities: { type: Array, default: () => [] },
  members: { type: Array, default: () => [] },
  filters: { type: Object, default: () => ({}) },
  canCreate: { type: Boolean, default: false },
  canEdit: { type: Boolean, default: false },
  editable: {
    type: Object,
    default: () => ({
      priority: true,
      assignee: true,
      plannedStartDate: true,
      plannedEndDate: true,
    }),
  },
  fieldEditable: {
    type: Object,
    default: () => ({
      priority: true,
      assignee: true,
      plannedStartDate: true,
      plannedEndDate: true,
    }),
  },
  selectedItemId: { type: [String, Number], default: null },
});

const emit = defineEmits([
  "open",
  "create",
  "updated",
  "navigation-change",
  "status-visibility-change",
  "status-color-change",
]);
const { t } = useI18n();
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;
const columns = reactive({});
const kanbanColumnsRef = ref(null);
const scrollingColumns = ref(new Set());
const draggingItemId = ref("");
const editingPriorityId = ref("");
const editingAssignee = reactive({ id: "", draft: [] });
const editingDate = reactive({ id: "", field: "" });
const openStatusMenuId = ref(0);
const colorPickerStatusId = ref(0);
const scrollTimers = new Map();
let sortableInstances = [];
let sortableSetupToken = 0;
let statusUpdatePending = false;
let loadSequence = 0;
const colorOptions = ["#fff7cc", "#e8f0ff", "#e9f8ef", "#f1f2f4", "#ffffff"];

const orderedStatuses = computed(() => {
  const systemById = new Map(
    props.statuses.map((status) => [Number(status.id), status]),
  );
  const result = [];
  for (const preference of props.statusPreferences) {
    const id = Number(preference.id);
    const status = systemById.get(id);
    if (!status) continue;
    result.push({ ...status, id, visible: preference.visible !== false });
    systemById.delete(id);
  }
  result.push(
    ...[...systemById.values()].map((status) => ({
      ...status,
      id: Number(status.id),
      visible: true,
    })),
  );
  return result;
});

const visibleStatuses = computed(() =>
  orderedStatuses.value.filter(
    (status) =>
      status.visible &&
      (!props.filters.statusActive ||
        (props.filters.statusIds || [])
          .map(Number)
          .includes(Number(status.id))),
  ),
);

function statusRoot(status) {
  const key = String(status?.key || "").toLowerCase();
  if (key.includes("in_progress")) return "board:status:in_progress";
  if (key.includes("in_review")) return "board:status:in_review";
  if (key.includes("done") || key.includes("resolved"))
    return "board:status:done";
  if (
    key.includes("pending_assign") ||
    key.includes("pending_plan") ||
    key.includes("closed") ||
    key.includes("rejected")
  ) {
    return key.includes("pending_assign")
      ? "board:status:todo"
      : "board:status:backlog";
  }
  const name = String(status?.name || "").toLowerCase();
  if (name.includes("进行中") || name.includes("in progress")) {
    return "board:status:in_progress";
  }
  if (name.includes("审核") || name.includes("review")) {
    return "board:status:in_review";
  }
  if (
    name.includes("完成") ||
    name.includes("done") ||
    name.includes("resolved")
  ) {
    return "board:status:done";
  }
  return "board:status:custom";
}

function backgroundClass(status) {
  return `kanban-background-${statusRoot(status).split(":").pop()}`;
}

function defaultColumnColor(status) {
  const root = statusRoot(status);
  if (root.endsWith(":todo")) return "#faeeef";
  if (root.endsWith(":in_progress")) return "#e9f8ef";
  if (root.endsWith(":in_review")) return "rgba(47, 163, 93, 0.05)";
  if (root.endsWith(":done")) return "rgba(45, 110, 222, 0.05)";
  if (root.endsWith(":custom")) return "#f8f9fb";
  return "rgba(246, 246, 247, 0.4)";
}

function customColor(status) {
  return props.boardColors[String(status.id)] || "";
}

function customColumnStyle(status) {
  return {
    backgroundColor: customColor(status) || defaultColumnColor(status),
  };
}

function hideStatus(id) {
  emit("status-visibility-change", { id: Number(id), visible: false });
}

function setStatusColor(id, color) {
  emit("status-color-change", { id: Number(id), color });
}

function handleStatusMenuOpen(id, open) {
  const statusId = Number(id);
  if (open) {
    openStatusMenuId.value = statusId;
    return;
  }
  if (colorPickerStatusId.value !== statusId) openStatusMenuId.value = 0;
}

function startColorPicker(id) {
  const statusId = Number(id);
  colorPickerStatusId.value = statusId;
  openStatusMenuId.value = statusId;
}

function finishColorPicker(id) {
  const statusId = Number(id);
  window.setTimeout(() => {
    if (colorPickerStatusId.value !== statusId) return;
    colorPickerStatusId.value = 0;
    openStatusMenuId.value = 0;
  }, 100);
}

function handleCustomColorChange(id, event) {
  setStatusColor(id, event.target.value);
  finishColorPicker(id);
}

function columnState(id) {
  const key = String(id);
  if (!columns[key]) {
    columns[key] = {
      items: [],
      total: 0,
      page: 0,
      loading: false,
      done: false,
      error: false,
      token: 0,
    };
  }
  return columns[key];
}

function requestParams(statusId, page) {
  const keyword = String(props.filters.search || "").trim();
  const hasExtraFilters = Boolean(
    keyword ||
    props.filters.statusActive ||
    props.filters.assigneeIds?.length ||
    props.filters.versionIds?.length ||
    props.filters.categoryId !== undefined ||
    props.filters.createdByIds?.length ||
    props.filters.priorityIds?.length ||
    props.filters.searchActive ||
    props.filters.viewHasActiveFilters,
  );
  const params = {
    type: props.typeKey,
    status_id: statusId,
    page,
    size: 20,
    root_only: props.typeKey === "requirement" && !hasExtraFilters ? 1 : 0,
    sort_field: props.boardSort?.field || "updated_at",
    sort_direction: props.boardSort?.direction || "asc",
  };
  if (keyword) params.title_search = keyword;
  if (props.filters.assigneeIds?.length) {
    params.assignee_ids = props.filters.assigneeIds.join(",");
  }
  if (props.filters.versionIds?.length) {
    params.version_ids = props.filters.versionIds.join(",");
  }
  if (props.filters.categoryId !== undefined)
    params.category_id = props.filters.categoryId;
  if (props.filters.createdByIds?.length) {
    params.created_by_ids = props.filters.createdByIds.join(",");
  }
  if (props.filters.priorityIds?.length) {
    params.priority_ids = props.filters.priorityIds.join(",");
  }
  if (props.filters.searchFilters) {
    params.search_filters = props.filters.searchFilters;
    params.timezone = props.filters.timezone || "UTC";
  }
  if (props.filters.viewId) params.view_id = props.filters.viewId;
  return params;
}

async function loadColumn(statusId, append = true) {
  const state = columnState(statusId);
  if (state.loading || (append && state.done)) return;
  const nextPage = append ? state.page + 1 : 1;
  const token = ++loadSequence;
  state.token = token;
  state.loading = true;
  state.error = false;
  try {
    const response = await listRequirements(
      props.wsId,
      requestParams(statusId, nextPage),
    );
    if (state.token !== token) return;
    const incoming = response?.list || [];
    if (append) {
      const seen = new Set(state.items.map((item) => String(item.id)));
      state.items.push(
        ...incoming.filter((item) => !seen.has(String(item.id))),
      );
    } else {
      state.items = incoming;
    }
    state.page = nextPage;
    state.total = Number(response?.total) || 0;
    state.done = state.items.length >= state.total || incoming.length < 20;
    notifyNavigationChange();
  } catch {
    if (state.token === token) state.error = true;
  } finally {
    if (state.token === token) state.loading = false;
  }
}

function reload() {
  loadSequence += 1;
  Object.values(columns).forEach((state) => {
    state.token = loadSequence;
    state.items = [];
    state.total = 0;
    state.page = 0;
    state.loading = false;
    state.done = false;
    state.error = false;
  });
  visibleStatuses.value.forEach((status) => loadColumn(status.id, false));
  notifyNavigationChange();
}

function applyItemUpdate(change) {
  if (change?.action !== "update" || !change.item) return;
  let sourceState;
  let sourceIndex = -1;
  Object.values(columns).some((state) => {
    const index = state.items.findIndex(
      (item) => String(item.id) === String(change.id),
    );
    if (index < 0) return false;
    sourceState = state;
    sourceIndex = index;
    return true;
  });
  if (!sourceState) return;

  const item = sourceState.items[sourceIndex];
  const previous = { ...item };
  const oldStatusId = Number(item.status_id);
  mergeWorkItem(item, change.item);

  const targetStatusId = Number(item.status_id);
  const targetVisible = visibleStatuses.value.some(
    (status) => Number(status.id) === targetStatusId,
  );
  const rootMatches =
    props.typeKey !== "requirement" ||
    hasFilterValues() ||
    !Number(item.parent_id);
  const matches =
    item.type === props.typeKey &&
    targetVisible &&
    rootMatches &&
    workItemMatchesFilters(item, props.filters);

  if (matches && oldStatusId === targetStatusId) {
    notifyNavigationChange();
    return previous;
  }
  sourceState.items.splice(sourceIndex, 1);

  if (oldStatusId !== targetStatusId || !matches) {
    sourceState.total = Math.max(0, sourceState.total - 1);
  }
  if (matches) {
    const targetState = columnState(targetStatusId);
    targetState.items.unshift(item);
    if (oldStatusId !== targetStatusId) targetState.total += 1;
    targetState.done = targetState.items.length >= targetState.total;
  }
  sourceState.done = sourceState.items.length >= sourceState.total;
  notifyNavigationChange();
  return previous;
}

function hasFilterValues() {
  return Boolean(
    String(props.filters.search || "").trim() ||
    props.filters.statusActive ||
    props.filters.assigneeIds?.length ||
    props.filters.versionIds?.length ||
    props.filters.categoryId !== undefined ||
    props.filters.createdByIds?.length ||
    props.filters.priorityIds?.length ||
    props.filters.searchActive ||
    props.filters.viewHasActiveFilters,
  );
}

function handleColumnScroll(event, statusId) {
  const element = event.currentTarget;
  const next = new Set(scrollingColumns.value);
  next.add(statusId);
  scrollingColumns.value = next;
  clearTimeout(scrollTimers.get(statusId));
  scrollTimers.set(
    statusId,
    setTimeout(() => {
      const updated = new Set(scrollingColumns.value);
      updated.delete(statusId);
      scrollingColumns.value = updated;
    }, 800),
  );
  if (element.scrollHeight - element.scrollTop - element.clientHeight < 100) {
    loadColumn(statusId, true);
  }
}

function handleColumnsWheel(event) {
  const horizontalDelta = event.deltaX;
  const delta = event.shiftKey ? event.deltaY || event.deltaX : horizontalDelta;
  if (!delta) return;
  if (
    !event.shiftKey &&
    Math.abs(horizontalDelta) < Math.abs(event.deltaY || 0) * 0.5
  ) {
    return;
  }
  event.preventDefault();
  event.stopPropagation();
  event.currentTarget.scrollLeft += delta;
}

function destroySortables() {
  sortableInstances.forEach((instance) => instance.destroy());
  sortableInstances = [];
}

function setSortablesDisabled(disabled) {
  sortableInstances.forEach((instance) => {
    instance.option("disabled", disabled);
  });
}

function restoreDraggedElement(event) {
  const source = event.from;
  const item = event.item;
  if (!source || !item || item.parentElement === source) return;
  // Sortable moves the node first; restore it so Vue patches from the DOM tree represented by the current arrays.
  const cards = Array.from(source.children).filter((element) =>
    element.classList.contains("kanban-card"),
  );
  const oldIndex = Number(event.oldDraggableIndex);
  const nextCard = Number.isInteger(oldIndex) ? cards[oldIndex] : null;
  if (nextCard) {
    source.insertBefore(item, nextCard);
    return;
  }
  const lastCard = cards[cards.length - 1];
  source.insertBefore(item, lastCard?.nextSibling || source.firstChild);
}

async function handleCardDragEnd(event) {
  draggingItemId.value = "";
  restoreDraggedElement(event);

  const sourceStatusId = Number(event.from?.dataset.statusId);
  const targetStatusId = Number(event.to?.dataset.statusId);
  const itemId = String(event.item?.dataset.workItemId || "");
  if (
    !itemId ||
    !sourceStatusId ||
    !targetStatusId ||
    sourceStatusId === targetStatusId
  ) {
    return;
  }

  const sourceState = columnState(sourceStatusId);
  const targetState = columnState(targetStatusId);
  const sourceIndex = sourceState.items.findIndex(
    (item) => String(item.id) === itemId,
  );
  if (sourceIndex < 0) return;

  const [item] = sourceState.items.splice(sourceIndex, 1);
  const requestedIndex = Number(event.newDraggableIndex);
  const targetIndex = Number.isInteger(requestedIndex)
    ? Math.min(Math.max(requestedIndex, 0), targetState.items.length)
    : targetState.items.length;
  targetState.items.splice(targetIndex, 0, item);
  sourceState.total = Math.max(0, sourceState.total - 1);
  targetState.total += 1;
  sourceState.done = sourceState.items.length >= sourceState.total;
  targetState.done = targetState.items.length >= targetState.total;
  item.status_id = targetStatusId;
  item.updated_at = Math.floor(Date.now() / 1000);
  notifyNavigationChange();

  statusUpdatePending = true;
  setSortablesDisabled(true);
  try {
    await updateRequirement(props.wsId, item.id, {
      status_id: targetStatusId,
    });
    showWorkItemUpdateSuccess(t("workspace_updateSuccess"));
    emit("updated", {
      id: item.id,
      updates: { status_id: targetStatusId },
    });
  } catch {
    reload();
  } finally {
    statusUpdatePending = false;
    setSortablesDisabled(!props.canEdit);
  }
}

async function setupSortables() {
  const setupToken = ++sortableSetupToken;
  await nextTick();
  if (setupToken !== sortableSetupToken) return;
  destroySortables();
  if (!props.canEdit || !kanbanColumnsRef.value) return;

  const groupName = `work-item-kanban-${props.wsId}-${props.typeKey}`;
  sortableInstances = Array.from(
    kanbanColumnsRef.value.querySelectorAll(".kanban-card-list"),
  ).map((element) =>
    Sortable.create(element, {
      group: groupName,
      sort: false,
      draggable: ".kanban-card",
      filter: "button, input, textarea, select, a, .ant-select, .ant-picker",
      preventOnFilter: false,
      animation: 150,
      scroll: true,
      bubbleScroll: true,
      scrollSensitivity: 80,
      scrollSpeed: 12,
      emptyInsertThreshold: 80,
      ghostClass: "kanban-card-ghost",
      chosenClass: "kanban-card-chosen",
      dragClass: "kanban-card-drag",
      disabled: statusUpdatePending,
      onMove: ({ to }) => {
        const targetState = columnState(Number(to.dataset.statusId));
        return targetState.page > 0 || !targetState.loading;
      },
      onStart: ({ item }) => {
        draggingItemId.value = String(item.dataset.workItemId || "");
      },
      onEnd: handleCardDragEnd,
    }),
  );
}

function normalizedIds(value) {
  return (Array.isArray(value) ? value : [])
    .map(Number)
    .filter((id) => Number.isInteger(id) && id > 0);
}

function filterMember(input, option) {
  return String(option?.label || "")
    .toLowerCase()
    .includes(String(input || "").toLowerCase());
}

function assigneeNames(item) {
  const names = Array.isArray(item.assignee_names)
    ? item.assignee_names
    : String(item.assignee_names || "").split(",");
  const usernames = Array.isArray(item.assignee_usernames)
    ? item.assignee_usernames
    : String(item.assignee_usernames || "").split(",");
  return normalizedIds(item.assignee_ids)
    .map((_, index) => names[index] || usernames[index] || "")
    .filter(Boolean)
    .join(";");
}

function assigneeAvatar(item) {
  return Array.isArray(item.assignee_avatars)
    ? item.assignee_avatars[0] || undefined
    : undefined;
}

function assigneeInitial(item) {
  const name = assigneeNames(item).split(";")[0] || "";
  return name.trim().charAt(0).toLocaleUpperCase();
}

function relativeUpdatedAt(value) {
  const timestamp = Number(value);
  if (!timestamp) return t("workspace_updatedJustNow");
  const elapsed = Math.max(0, Math.floor(Date.now() / 1000) - timestamp);
  if (elapsed < 60) return t("workspace_updatedJustNow");
  if (elapsed < 3600) {
    return t("workspace_updatedMinutesAgo", { n: Math.floor(elapsed / 60) });
  }
  if (elapsed < 86400) {
    return t("workspace_updatedHoursAgo", { n: Math.floor(elapsed / 3600) });
  }
  if (elapsed <= 60 * 86400) {
    return t("workspace_updatedDaysAgo", { n: Math.floor(elapsed / 86400) });
  }
  return dayjs.unix(timestamp).format("MM-DD");
}

function isPriorityEditing(item) {
  return String(editingPriorityId.value) === String(item.id);
}

function startPriorityEditing(item) {
  if (!props.canEdit || !props.fieldEditable.priority) return;
  editingPriorityId.value = String(item.id);
}

function stopPriorityEditing() {
  editingPriorityId.value = "";
}

function handlePriorityChange(item, value) {
  stopPriorityEditing();
  updateItem(item, "priority", value);
}

function isAssigneeEditing(item) {
  return String(editingAssignee.id) === String(item.id);
}

function startAssigneeEditing(item) {
  if (!props.canEdit || !props.fieldEditable.assignee) return;
  editingAssignee.id = String(item.id);
  editingAssignee.draft = normalizedIds(item.assignee_ids);
}

function saveAssigneeEditing(item) {
  if (!isAssigneeEditing(item)) return;
  const value = [...editingAssignee.draft];
  editingAssignee.id = "";
  editingAssignee.draft = [];
  const current = normalizedIds(item.assignee_ids);
  if (current.join(",") === value.join(",")) return;
  updateItem(item, "assignee_ids", value);
}

function isDateEditing(item, field) {
  return (
    String(editingDate.id) === String(item.id) && editingDate.field === field
  );
}

function startDateEditing(item, field) {
  const editableKey =
    field === "planned_start_date" ? "plannedStartDate" : "plannedEndDate";
  if (!props.canEdit || !props.fieldEditable[editableKey]) return;
  editingDate.id = String(item.id);
  editingDate.field = field;
}

function stopDateEditing() {
  editingDate.id = "";
  editingDate.field = "";
}

function handleDateChange(item, field, value) {
  stopDateEditing();
  updateItem(item, field, value);
}

async function updateItem(item, field, value) {
  const updates = { [field]: value };
  try {
    await updateRequirement(props.wsId, item.id, updates);
    Object.assign(item, updates);
    item.updated_at = Math.floor(Date.now() / 1000);
    if (field === "priority") {
      const priority = props.priorities.find(
        (option) => Number(option.id) === Number(value),
      );
      item.priority_name = priority?.label || "";
      item.priority_color = priority?.color || "";
    }
    if (field === "assignee_ids") {
      const ids = normalizedIds(value);
      item.assignee_ids = ids;
      item.assignee_names = [];
      item.assignee_avatars = [];
      item.assignee_usernames = [];
      ids.forEach((id) => {
        const member =
          props.members.find(
            (candidate) => Number(candidate.user_id) === Number(id),
          ) || {};
        item.assignee_names.push(member.user_name || "");
        item.assignee_avatars.push(member.user_avatar || "");
        item.assignee_usernames.push(member.user_username || "");
      });
    }
    showWorkItemUpdateSuccess(t("workspace_updateSuccess"));
    emit("updated", { id: item.id, updates: { ...updates } });
  } catch {
    // The response interceptor reports the failure; the card keeps its old value.
  }
}

function notifyNavigationChange() {
  emit(
    "navigation-change",
    visibleStatuses.value.flatMap((status) =>
      columnState(status.id).items.map((item) => ({
        id: Number(item.id),
        workspaceId: Number(props.wsId),
      })),
    ),
  );
}

function translucent(color) {
  const hex = String(color || "").replace("#", "");
  if (!/^[0-9a-f]{6}$/i.test(hex)) return "#f6f7f9";
  const red = parseInt(hex.slice(0, 2), 16);
  const green = parseInt(hex.slice(2, 4), 16);
  const blue = parseInt(hex.slice(4, 6), 16);
  return `rgba(${red}, ${green}, ${blue}, 0.065)`;
}

function dateValue(value) {
  const timestamp = Number(value);
  return timestamp > 0 ? dayjs.unix(timestamp) : null;
}

function shortDate(value) {
  const formatted = formatLocalDate(value);
  return formatted === "-" ? "" : formatted.slice(5);
}

watch(
  () => [
    props.wsId,
    props.typeKey,
    props.filters,
    props.boardSort?.field,
    props.boardSort?.direction,
    visibleStatuses.value.map((status) => status.id).join(","),
  ],
  reload,
  { deep: true, immediate: true },
);

watch(
  () => [
    props.canEdit,
    props.wsId,
    props.typeKey,
    visibleStatuses.value.map((status) => status.id).join(","),
  ],
  setupSortables,
  { flush: "post" },
);

onMounted(setupSortables);

onBeforeUnmount(() => {
  sortableSetupToken += 1;
  destroySortables();
  scrollTimers.forEach(clearTimeout);
});

defineExpose({ reload, applyItemUpdate });
</script>

<style scoped lang="less">
.work-item-kanban {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
.kanban-columns {
  display: flex;
  height: 100%;
  min-height: 360px;
  padding-bottom: 8px;
  overflow-x: auto;
  gap: 14px;
  overscroll-behavior-x: none;
  touch-action: pan-y;
}
.kanban-column {
  display: flex;
  flex: 0 0 280px;
  width: 280px;
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
  border-radius: 16px;
  background: #f8f9fb;
}
.kanban-background-backlog {
  background: rgba(246, 246, 247, 0.4);
}
.kanban-background-todo {
  background: #faeeef;
}
.kanban-background-in_progress {
  background: #e9f8ef;
}
.kanban-background-in_review {
  background: rgba(47, 163, 93, 0.05);
}
.kanban-background-done {
  background: rgba(45, 110, 222, 0.05);
}
.kanban-background-custom {
  background: #f8f9fb;
}
.kanban-column-header {
  display: flex;
  flex: 0 0 24px;
  align-items: center;
  min-width: 0;
  padding: 14px 12px 0 16px;
  gap: 8px;

  strong {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--02, #262626);
    font-size: 12px;
    font-style: normal;
    font-weight: 600;
  }
}
.kanban-status-dot {
  width: 9px;
  height: 9px;
  flex: none;
  border-radius: 50%;
}
.kanban-count {
  color: var(--app-text-secondary);
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 22px;
  width: 22px;
  border-radius: 50%;
  background: #edeff2;
}
.kanban-more-button {
  display: inline-flex;
  width: 28px;
  min-width: 28px;
  height: 28px;
  margin-left: auto;
  align-items: center;
  justify-content: center;
  padding: 0;
  color: #595959;
}
.kanban-column-menu {
  min-width: 190px;
  padding: 6px;
  border: 1px solid var(--app-border);
  border-radius: 6px;
  background: var(--app-surface);
  box-shadow: 0 6px 18px rgba(16, 24, 40, 0.14);
}
.kanban-column-menu-item,
.kanban-color-picker {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 8px;
  min-height: 30px;
  padding: 4px 8px;
  border: 0;
  background: transparent;
  color: var(--app-text);
  cursor: pointer;
  font-size: 12px;
  text-align: left;
}
.kanban-column-menu-item:hover,
.kanban-color-picker:hover {
  background: var(--app-surface-muted);
}
.kanban-color-heading {
  padding: 7px 8px 4px;
  color: var(--app-text-secondary);
  font-size: 11px;
}
.kanban-color-options {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px 8px;
}
.kanban-color-swatch,
.kanban-color-reset {
  width: 20px;
  height: 20px;
  padding: 0;
  border: 1px solid #d9dde5;
  border-radius: 3px;
  cursor: pointer;
}
.kanban-color-swatch.active,
.kanban-color-reset.active {
  outline: 2px solid #1677ff;
  outline-offset: 1px;
}
.kanban-color-reset {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: #fff;
  color: #8c8c8c;
  font-size: 11px;
}
.kanban-color-picker {
  justify-content: space-between;
}
.kanban-color-picker input {
  width: 26px;
  height: 22px;
  padding: 0;
  border: 0;
  background: transparent;
  cursor: pointer;
}
.kanban-card-list {
  flex: 1;
  min-height: 0;
  padding: 12px;
  overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: transparent transparent;
  overscroll-behavior-y: contain;
  overscroll-behavior-x: none;
  transition: box-shadow 0.15s ease;

  &::-webkit-scrollbar {
    width: 4px;
  }
  &::-webkit-scrollbar-thumb {
    border-radius: 4px;
    background: transparent;
  }
  &.scrolling {
    scrollbar-color: rgba(75, 85, 99, 0.45) transparent;
  }
  &.scrolling::-webkit-scrollbar-thumb {
    background: rgba(75, 85, 99, 0.45);
  }
}
.kanban-card {
  box-sizing: border-box;
  height: 115px;
  padding: 16px;
  overflow: hidden;
  border: 1px solid #dfe5ee;
  border-radius: 13px;
  background: #fff;
  box-shadow: 0 1px 2px rgba(16, 24, 40, 0.03);
  cursor: pointer;
  display: flex;
  flex-direction: column;
  justify-content: space-between;

  & + & {
    margin-top: 10px;
  }
  &:hover,
  &:focus-visible {
    border-color: #9abdf8;
    box-shadow: 0 2px 6px rgba(22, 119, 255, 0.09);
    outline: none;
  }
}
.kanban-card.detail-selected-card {
  border-color: #7da9f5;
  background: #e9f2ff;
  box-shadow: 0 3px 10px rgba(22, 119, 255, 0.14);
}
.kanban-card.draggable {
  cursor: grab;
}
.kanban-card.draggable:active,
.kanban-card-chosen,
.kanban-card-drag {
  cursor: grabbing;
}
.kanban-card-ghost {
  opacity: 0.35;
  border-style: dashed;
}
.kanban-card-chosen {
  border-color: #7da9f5;
  box-shadow: 0 5px 14px rgba(22, 119, 255, 0.16);
}
.work-item-kanban.is-dragging .kanban-card-list {
  box-shadow: inset 0 0 0 1px rgba(22, 119, 255, 0.2);
}
.kanban-card-title {
  display: block;
  width: 100%;
  min-width: 0;
  color: var(--02, #262626);
  font-size: 16px;
  font-style: normal;
  font-weight: 600;
  line-height: 24px; /* 150% */

  .kanban-title-text {
    display: -webkit-box;
    min-width: 0;
    overflow: hidden;
    color: var(--app-text);
    font-size: 14px;
    font-weight: 600;
    line-height: 22px;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
  }
}
.kanban-card-title .kanban-priority-pill {
  display: inline-block;
  flex: none;
  overflow: hidden;
  text-overflow: ellipsis;
  vertical-align: text-bottom;
  white-space: nowrap;
}
.kanban-card-meta {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 6px;
  padding-top: 8px;
  justify-content: space-between;
}
.kanban-priority-select {
  width: 76px;
  flex: 0 0 76px;

  :deep(.ant-select-selector) {
    height: 22px !important;
    padding: 0 4px !important;
    border: 1px solid var(--app-border) !important;
    border-radius: 10px !important;
  }
  :deep(.ant-select-selection-item) {
    font-size: 12px;
    line-height: 16px !important;
  }
}
.kanban-priority-pill {
  height: 20px;
  width: 22px;
  font-size: 14px;
  line-height: 22px;
  padding: 0 4px;
  background: #fbf6e6;
  border-radius: 6px;
}
.kanban-priority-pill.editable {
  cursor: pointer;
}
.kanban-assignee-select {
  min-width: 0;
  flex: 1 1 96px;

  :deep(.ant-select-selector) {
    height: 22px !important;
    min-height: 22px !important;
    padding: 0 4px !important;
    overflow: hidden;
  }
  :deep(.ant-select-selection-overflow) {
    flex-wrap: nowrap;
  }
  :deep(.ant-select-selection-item),
  :deep(.ant-select-selection-placeholder) {
    font-size: 12px;
    line-height: 16px !important;
  }
}
.kanban-assignee-display {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  width: 0;
  min-width: 0;
  flex: 1 1 0;
  overflow: hidden;
  color: var(--app-text-secondary);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.kanban-assignee-name {
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
}
.kanban-assignee-avatar {
  background: #f4f4f5;
  border-radius: 50%;
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  img {
    width: 15px;
    height: 15px;
  }
}
.kanban-assignee-display.editable {
  cursor: pointer;
}
.kanban-meta-spacer {
  flex: 1;
}
.kanban-updated-at {
  flex: none;
  color: var(--app-text-secondary);
  font-size: 12px;
  white-space: nowrap;
}
.kanban-date-field {
  display: inline-flex;
  flex: 0 0 54px;
  min-width: 0;
  align-items: center;
  justify-content: flex-end;
  margin-left: auto;
}
.kanban-date-picker {
  width: 80px;
  position: relative;
  z-index: 1;

  :deep(.ant-picker-input > input) {
    font-size: 12px;
  }
  :deep(.ant-picker-suffix) {
    display: none;
  }
  :deep(.ant-picker-clear) {
    right: 1px;
  }
  &.ant-picker {
    height: 24px;
    padding: 0 5px;
    border: 1px solid #d9dde5;
    border-radius: 4px;
    background: var(--app-surface);
  }
}
.kanban-date-display {
  width: 54px;
  height: 24px;
  padding: 0;
  overflow: hidden;
  border: 0;
  background: transparent;
  color: var(--app-text-secondary);
  cursor: default;
  font: inherit;
  font-size: 13px;
  line-height: 22px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.kanban-date-display.editable {
  cursor: pointer;
}
.kanban-initial-loading,
.kanban-empty,
.kanban-no-columns {
  display: block;
  margin: 60px auto 0;
  text-align: center;
}
.kanban-load-more {
  padding: 12px 0;
  color: var(--app-text-secondary);
  font-size: 12px;
  text-align: center;
}
.kanban-retry {
  display: block;
  margin: 10px auto;
  border: 0;
  background: transparent;
  color: #1677ff;
  cursor: pointer;
  font-size: 12px;
}

@media (max-width: 768px) {
  .kanban-column {
    flex-basis: min(360px, calc(100vw - 48px));
    width: min(360px, calc(100vw - 48px));
  }
}
</style>

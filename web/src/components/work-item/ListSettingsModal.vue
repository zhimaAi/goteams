<template>
  <a-modal
    :open="open"
    :title="$t('workspace_listSettings')"
    :footer="null"
    width="570px"
    class="list-settings-modal"
    @update:open="$emit('update:open', $event)"
  >
    <section
      v-if="!hidePreviewMode"
      class="list-settings-section mode-settings-section"
    >
      <h3>{{ $t("workspace_requirementPreviewMode") }}</h3>
      <a-radio-group
        v-model:value="preferences.previewMode"
        class="mode-group"
        button-style="solid"
        @change="savePreferences"
      >
        <a-radio-button value="drawer">
          <MenuUnfoldOutlined /> {{ $t("workspace_previewDrawer") }}
        </a-radio-button>
        <a-radio-button value="modal">
          <AppstoreOutlined /> {{ $t("workspace_previewModal") }}
        </a-radio-button>
        <a-radio-button value="window">
          <ExportOutlined /> {{ $t("workspace_previewWindow") }}
        </a-radio-button>
      </a-radio-group>
    </section>

    <slot name="extra-settings" />

    <section
      v-if="!hidePreviewMode && !hideViewMode"
      class="list-settings-section mode-settings-section"
    >
      <h3>{{ $t("workspace_viewMode") }}</h3>
      <a-radio-group
        v-model:value="preferences.viewMode"
        class="mode-group"
        button-style="solid"
        @change="savePreferences"
      >
        <a-radio-button value="list">
          <UnorderedListOutlined /> {{ $t("workspace_listView") }}
        </a-radio-button>
        <a-radio-button value="board">
          <AppstoreOutlined /> {{ $t("workspace_boardView") }}
        </a-radio-button>
      </a-radio-group>
    </section>

    <section
      v-if="hidePreviewMode || preferences.viewMode === 'list'"
      class="list-settings-section field-settings-section"
    >
      <div class="list-settings-heading">
        <h3>{{ $t("workspace_listFieldDisplay") }}</h3>
        <a-button type="link" size="small" @click="restoreFieldDefaults">
          {{ $t("workspace_resetMyDisplay") }}
        </a-button>
      </div>
      <div class="user-field-list">
        <div v-if="showRowIndex" class="user-field-row">
          <div class="user-field-main">
            <span class="user-field-drag-handle disabled"
              ><HolderOutlined
            /></span>
            <span class="user-field-name">{{ $t("planning_colIndex") }}</span>
            <a-tag color="blue">{{ $t("settings_builtin") }}</a-tag>
          </div>
          <a-switch :checked="true" size="small" disabled />
        </div>
        <div ref="fieldListRef">
          <div
            v-for="field in displayFields"
            :key="field.preferenceKey"
            class="user-field-row"
          >
            <div class="user-field-main">
              <span
                class="user-field-drag-handle"
                :class="{ disabled: !canDragOrder || fieldOrderSaving }"
                :title="
                  canDragOrder && !fieldOrderSaving
                    ? $t('settings_dragFieldToSort')
                    : ''
                "
              >
                <HolderOutlined />
              </span>
              <span class="user-field-name">{{ field.label }}</span>
              <a-tag v-if="field.kind === 'builtin'" color="blue">
                {{ $t("settings_builtin") }}
              </a-tag>
            </div>
            <a-switch
              :checked="field.visible"
              :disabled="field.locked"
              size="small"
              @change="
                (visible) => setFieldVisible(field.preferenceKey, visible)
              "
            />
          </div>
        </div>
      </div>
    </section>

    <section v-else class="list-settings-section field-settings-section">
      <div class="list-settings-heading">
        <h3>{{ $t("workspace_boardCardDisplay") }}</h3>
        <a-button type="link" size="small" @click="restoreBoardDefaults">
          {{ $t("workspace_restoreDefault") }}
        </a-button>
      </div>
      <div class="board-sort-setting">
        <span class="board-sort-label">{{ $t("workspace_boardSort") }}</span>
        <a-select
          :value="boardSortValue"
          size="small"
          class="board-sort-select"
          @change="setBoardSort"
        >
          <a-select-option
            v-for="option in boardSortOptions"
            :key="option.value"
            :value="option.value"
          >
            {{ $t(option.label) }}
          </a-select-option>
        </a-select>
      </div>
      <div class="user-field-list">
        <div
          v-for="status in effectiveBoardStatuses"
          :key="status.id"
          class="user-field-row"
          :class="{ dragging: draggingStatusId === status.id }"
          @dragover.prevent
          @drop="handleStatusDrop(status.id)"
        >
          <div class="user-field-main">
            <span
              class="user-field-drag-handle"
              draggable="true"
              :title="$t('workspace_dragStatusToSort')"
              @dragstart="handleStatusDragStart($event, status.id)"
              @dragend="draggingStatusId = 0"
            >
              <HolderOutlined />
            </span>
            <span
              class="board-status-dot"
              :style="{ backgroundColor: status.color || '#98a2b3' }"
            />
            <span class="user-field-name">{{ status.name }}</span>
          </div>
          <a-switch
            :checked="status.visible"
            size="small"
            @change="(visible) => setStatusVisible(status.id, visible)"
          />
        </div>
      </div>
    </section>
  </a-modal>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, reactive, ref, watch } from "vue";
import {
  AppstoreOutlined,
  ExportOutlined,
  HolderOutlined,
  MenuUnfoldOutlined,
  UnorderedListOutlined,
} from "@ant-design/icons-vue";
import Sortable from "sortablejs";
import { saveFieldOrder } from "@/api/workspace";
import {
  BOARD_SORT_OPTIONS,
  boardSortValue as getBoardSortValue,
  globalListPreferencesKey,
  normalizeBoardSort,
  readBoardColors,
  readListPreferences,
  scopedListPreferencesKey,
  writeGlobalListPreferences,
  writeScopedListPreferences,
} from "@/utils/boardPreferences";

const props = defineProps({
  open: { type: Boolean, default: false },
  userId: { type: [String, Number], default: "" },
  wsId: { type: [String, Number], default: "" },
  typeKey: { type: String, default: "requirement" },
  fields: { type: Array, default: () => [] },
  statuses: { type: Array, default: () => [] },
  currentPreferences: { type: Object, default: null },
  // Order scope: local = browser only; project = written to project-level field config (used by planning).
  orderScope: { type: String, default: "local" },
  // Project-level order needs edit permission; dragging is disabled without it.
  canManageOrder: { type: Boolean, default: true },
  // Planning has no detail page, so the preview mode is hidden; the locked index column is shown first.
  hidePreviewMode: { type: Boolean, default: false },
  hideViewMode: { type: Boolean, default: false },
  showRowIndex: { type: Boolean, default: false },
});
const emit = defineEmits(["update:open", "change", "order-changed"]);

const canDragOrder = computed(
  () => props.orderScope !== "project" || props.canManageOrder,
);

const preferences = reactive({
  previewMode: "drawer",
  viewMode: "list",
  fields: [],
  boardStatuses: [],
  boardSort: { field: "updated_at", direction: "asc" },
  boardColors: {},
});
const displayFields = ref([]);
const fieldListRef = ref(null);
const draggingStatusId = ref(0);
const fieldOrderSaving = ref(false);
let fieldSortable = null;
const boardSortOptions = computed(() => [
  { value: "updated_at:asc", label: "workspace_sortUpdatedAsc" },
  { value: "updated_at:desc", label: "workspace_sortUpdatedDesc" },
  { value: "created_at:asc", label: "workspace_sortCreatedAsc" },
  { value: "created_at:desc", label: "workspace_sortCreatedDesc" },
]);
const boardSortValue = computed(() => getBoardSortValue(preferences.boardSort));

// Fields disabled in project settings do not exist in this project and are excluded from personal show/hide.
const enabledFields = computed(() =>
  props.fields.filter((field) => field.enabled !== false),
);

const effectiveFields = computed(() => {
  const visibleByKey = new Map(
    preferences.fields.map((item) => [item.key, item.visible !== false]),
  );
  const withVisible = (field) => ({
    ...field,
    visible: field.locked
      ? true
      : (visibleByKey.get(field.preferenceKey) ?? field.visible !== false),
  });
  // Project-level order always follows the backend; only show/hide is stored locally.
  if (props.orderScope === "project") {
    return enabledFields.value.map(withVisible);
  }
  const systemByKey = new Map(
    enabledFields.value.map((field) => [field.preferenceKey, field]),
  );
  const result = [];
  for (const preference of preferences.fields) {
    const systemField = systemByKey.get(preference.key);
    if (!systemField) continue;
    result.push(withVisible(systemField));
    systemByKey.delete(preference.key);
  }
  result.push(...[...systemByKey.values()].map(withVisible));
  return result;
});

const effectiveBoardStatuses = computed(() => {
  const systemById = new Map(
    props.statuses.map((status) => [Number(status.id), status]),
  );
  const result = [];
  for (const preference of preferences.boardStatuses) {
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

function scopedStorageKey() {
  return scopedListPreferencesKey(props.userId, props.wsId, props.typeKey);
}

function globalStorageKey() {
  return globalListPreferencesKey(props.userId);
}

function notifyChange() {
  emit("change", {
    previewMode: preferences.previewMode,
    viewMode: preferences.viewMode,
    fields: effectiveFields.value.map((field) => ({ ...field })),
    boardStatuses: effectiveBoardStatuses.value.map((status) => ({
      id: status.id,
      visible: status.visible,
    })),
    boardSort: { ...preferences.boardSort },
    boardColors: { ...preferences.boardColors },
  });
}

function loadPreferences() {
  preferences.previewMode = "drawer";
  preferences.viewMode = "list";
  preferences.fields = [];
  preferences.boardStatuses = [];
  preferences.boardSort = { field: "updated_at", direction: "asc" };
  preferences.boardColors = {};
  const scopedStored = readListPreferences(scopedStorageKey());
  const globalStored = readListPreferences(globalStorageKey());
  const storedPreviewMode =
    globalStored.previewMode ?? scopedStored.previewMode;
  const storedViewMode = globalStored.viewMode ?? scopedStored.viewMode;
  if (["drawer", "modal", "window"].includes(storedPreviewMode)) {
    preferences.previewMode = storedPreviewMode;
  }
  if (["list", "board"].includes(storedViewMode)) {
    preferences.viewMode = props.hideViewMode ? "list" : storedViewMode;
  }
  if (Array.isArray(scopedStored.fields)) {
    preferences.fields = scopedStored.fields.filter(
      (field) => typeof field?.key === "string",
    );
  }
  if (Array.isArray(scopedStored.boardStatuses)) {
    preferences.boardStatuses = scopedStored.boardStatuses.filter(
      (status) => Number.isInteger(Number(status?.id)) && Number(status.id) > 0,
    );
  }
  preferences.boardSort = normalizeBoardSort(scopedStored.boardSort);
  preferences.boardColors = readBoardColors(scopedStored.boardColors);
  // Migrate the current list's legacy modes once, then share them across all list settings.
  if (
    globalStorageKey() &&
    (globalStored.previewMode === undefined ||
      (!props.hideViewMode && globalStored.viewMode === undefined))
  ) {
    saveGlobalPreferences();
  }
  notifyChange();
}

function syncCurrentPreferences() {
  const source = props.currentPreferences;
  if (!source) return;
  preferences.previewMode = ["drawer", "modal", "window"].includes(
    source.previewMode,
  )
    ? source.previewMode
    : "drawer";
  preferences.viewMode = ["list", "board"].includes(source.viewMode)
    ? source.viewMode
    : "list";
  preferences.fields = Array.isArray(source.fields)
    ? source.fields
        .filter((field) => typeof field?.preferenceKey === "string")
        .map((field) => ({
          key: field.preferenceKey,
          visible: field.visible !== false,
        }))
    : [];
  preferences.boardStatuses = Array.isArray(source.boardStatuses)
    ? source.boardStatuses
        .filter(
          (status) =>
            Number.isInteger(Number(status?.id)) && Number(status.id) > 0,
        )
        .map((status) => ({
          id: Number(status.id),
          visible: status.visible !== false,
        }))
    : [];
  preferences.boardSort = normalizeBoardSort(source.boardSort);
  preferences.boardColors = readBoardColors(source.boardColors);
}

function savePreferences() {
  saveGlobalPreferences();
  writeScopedListPreferences(props.userId, props.wsId, props.typeKey, {
    fields: preferences.fields,
    boardStatuses: preferences.boardStatuses,
    boardSort: preferences.boardSort,
    boardColors: preferences.boardColors,
  });
  notifyChange();
}

function saveGlobalPreferences() {
  const patch = { previewMode: preferences.previewMode };
  if (!props.hideViewMode) patch.viewMode = preferences.viewMode;
  writeGlobalListPreferences(props.userId, patch);
}

function persistBoardStatuses(statuses) {
  preferences.boardStatuses = statuses.map((status) => ({
    id: Number(status.id),
    visible: status.visible !== false,
  }));
  savePreferences();
}

function setStatusVisible(id, visible) {
  persistBoardStatuses(
    effectiveBoardStatuses.value.map((status) =>
      Number(status.id) === Number(id) ? { ...status, visible } : status,
    ),
  );
}

function handleStatusDragStart(event, id) {
  draggingStatusId.value = Number(id);
  event.dataTransfer.effectAllowed = "move";
  event.dataTransfer.setData("text/plain", String(id));
}

function handleStatusDrop(targetId) {
  const sourceId = draggingStatusId.value;
  draggingStatusId.value = 0;
  if (!sourceId || Number(sourceId) === Number(targetId)) return;
  const statuses = [...effectiveBoardStatuses.value];
  const sourceIndex = statuses.findIndex(
    (status) => Number(status.id) === Number(sourceId),
  );
  const targetIndex = statuses.findIndex(
    (status) => Number(status.id) === Number(targetId),
  );
  if (sourceIndex < 0 || targetIndex < 0) return;
  const [moved] = statuses.splice(sourceIndex, 1);
  statuses.splice(targetIndex, 0, moved);
  persistBoardStatuses(statuses);
}

function restoreBoardDefaults() {
  preferences.boardStatuses = [];
  preferences.boardSort = { field: "updated_at", direction: "asc" };
  preferences.boardColors = {};
  savePreferences();
}

function setBoardSort(value) {
  const option = BOARD_SORT_OPTIONS.find((item) => item.value === value);
  preferences.boardSort = normalizeBoardSort(option);
  savePreferences();
}

function persistEffectiveFields(fields) {
  preferences.fields = fields.map((field) => ({
    key: field.preferenceKey,
    visible: field.visible,
  }));
  savePreferences();
}

function setFieldVisible(preferenceKey, visible) {
  persistEffectiveFields(
    displayFields.value.map((field) =>
      field.preferenceKey === preferenceKey ? { ...field, visible } : field,
    ),
  );
}

function destroyFieldSortable() {
  fieldSortable?.destroy();
  fieldSortable = null;
}

function setupFieldSortable() {
  destroyFieldSortable();
  const showFields =
    props.open && (props.hidePreviewMode || preferences.viewMode === "list");
  if (
    !fieldListRef.value ||
    !canDragOrder.value ||
    fieldOrderSaving.value ||
    !showFields
  )
    return;
  fieldSortable = Sortable.create(fieldListRef.value, {
    handle: ".user-field-drag-handle",
    draggable: ".user-field-row",
    animation: 150,
    // Native HTML5 drag inside the overflow modal does not move rows; fallback does.
    forceFallback: true,
    fallbackOnBody: true,
    ghostClass: "user-field-row-ghost",
    onEnd({ oldIndex, newIndex }) {
      if (oldIndex === newIndex) return;
      const previous = displayFields.value.map((field) => ({ ...field }));
      const next = [...displayFields.value];
      const [moved] = next.splice(oldIndex, 1);
      next.splice(newIndex, 0, moved);
      displayFields.value = next;
      persistFieldOrder(next, previous);
    },
  });
}

async function persistFieldOrder(fields, previous) {
  if (!canDragOrder.value) return;
  // Project-level order is written to the backend; personal order stays in the browser.
  if (props.orderScope === "project") {
    if (fieldOrderSaving.value) return;
    fieldOrderSaving.value = true;
    fieldSortable?.option("disabled", true);
    try {
      await saveFieldOrder(
        props.wsId,
        props.typeKey,
        buildProjectOrder(fields),
      );
      emit("order-changed");
    } catch {
      displayFields.value = previous;
    } finally {
      fieldOrderSaving.value = false;
      nextTick(setupFieldSortable);
    }
    return;
  }
  persistEffectiveFields(fields);
}

// The backend requires the full field order; re-insert hidden/disabled fields at their original positions.
function buildProjectOrder(visibleFields) {
  const merged = [...visibleFields];
  props.fields.forEach((field, index) => {
    if (field.enabled === false) merged.splice(index, 0, field);
  });
  const items = merged.map((field) =>
    field.kind === "custom"
      ? { kind: "custom", key: "", id: field.customField.id }
      : { kind: "builtin", key: field.configKey, id: 0 },
  );
  if (props.showRowIndex) {
    items.unshift({ kind: "builtin", key: "row_index", id: 0 });
  }
  return items;
}

function restoreFieldDefaults() {
  preferences.fields = [];
  savePreferences();
}

watch(() => [props.userId, props.wsId, props.typeKey], loadPreferences, {
  immediate: true,
});
watch(
  () => props.open,
  (open) => {
    if (open) syncCurrentPreferences();
  },
);
watch(
  () => props.currentPreferences,
  () => {
    if (props.open) syncCurrentPreferences();
  },
  { deep: true },
);
watch(() => props.fields, notifyChange, { deep: true });
watch(() => props.statuses, notifyChange, { deep: true });
watch(
  () => [props.fields, preferences.fields],
  () => {
    displayFields.value = effectiveFields.value.map((field) => ({ ...field }));
  },
  { immediate: true, deep: true },
);
watch(
  () => [
    props.open,
    canDragOrder.value,
    displayFields.value.length,
    preferences.viewMode,
    props.hidePreviewMode,
  ],
  ([open]) => {
    destroyFieldSortable();
    const showFields =
      open && (props.hidePreviewMode || preferences.viewMode === "list");
    if (showFields && canDragOrder.value) nextTick(setupFieldSortable);
  },
);
onBeforeUnmount(destroyFieldSortable);
</script>

<style scoped lang="less">
:global(.list-settings-modal .ant-modal-content) {
  padding: 24px 30px 28px;
  border-radius: 8px;
}
:global(.list-settings-modal .ant-modal-title) {
  font-size: 18px;
}
.list-settings-section {
  margin-top: 14px;
}
.list-settings-section h3 {
  margin: 0 0 12px;
  color: var(--app-text);
  font-size: 14px;
  font-weight: 600;
}
.mode-settings-section {
  margin-top: 0;
  padding: 16px 6px 20px;

  & + & {
    border-top: 1px solid var(--app-border);
  }
}
.mode-group {
  display: flex;
  width: 100%;

  :deep(.ant-radio-button-wrapper) {
    display: inline-flex;
    min-width: 0;
    flex: 1;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding-inline: 8px;
  }
}
.field-settings-section {
  margin-top: 28px;
}
.list-settings-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.list-settings-heading :deep(.ant-btn) {
  height: 28px;
  padding: 0;
}
.board-sort-setting {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 12px 0;
  color: var(--app-text-secondary);
}
.board-sort-select {
  width: 190px;
}
.user-field-list {
  max-height: min(380px, calc(100vh - 360px));
  overflow-y: auto;
  border: 1px solid var(--app-border);
  border-radius: 6px;
}
.user-field-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 52px;
  padding: 0 16px;
  border-bottom: 1px solid var(--app-border);
  background: var(--app-surface);
  transition:
    opacity 0.15s,
    background 0.15s;

  &:last-child {
    border-bottom: 0;
  }
  &.dragging {
    background: #f5f8ff;
    opacity: 0.55;
  }
}
.user-field-main {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10px;
}
.user-field-drag-handle {
  display: inline-flex;
  flex: none;
  align-items: center;
  color: var(--app-text-secondary);
  cursor: grab;
  font-size: 17px;

  &:active {
    cursor: grabbing;
  }
  &.disabled {
    cursor: not-allowed;
    opacity: 0.4;
  }
}
:deep(.user-field-row-ghost) {
  background: #f5f8ff;
  opacity: 0.55;
}
.user-field-name {
  overflow: hidden;
  color: var(--app-text);
  text-overflow: ellipsis;
  white-space: nowrap;
}
.board-status-dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  flex: none;
}

@media (max-width: 620px) {
  .mode-group :deep(.ant-radio-button-wrapper) {
    font-size: 12px;
  }

  .mode-settings-section {
    padding-inline: 0;
  }
}
</style>

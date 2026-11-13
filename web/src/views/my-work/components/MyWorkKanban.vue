<template>
  <div ref="boardRef" class="my-work-kanban">
    <div v-if="columns.length" class="kanban-columns">
      <section
        v-for="column in columns"
        :key="column.id"
        class="kanban-column"
        :data-project-id="column.id"
        :style="{
          backgroundColor: customColor(column) || defaultColumnColor(),
        }"
      >
        <header class="kanban-column-header">
          <img
            v-if="columnIconUrl(column.id)"
            :src="columnIconUrl(column.id)"
            alt=""
            class="column-icon-image"
          />
          <a-avatar
            v-else
            :size="24"
            class="column-avatar"
            :style="{ backgroundColor: column.color || '#315bea' }"
          >
            {{ projectInitial(column) }}
          </a-avatar>
          <strong :title="column.name">{{ column.name }}</strong>
          <span class="kanban-count">{{ column.items.length }}</span>
          <a-dropdown
            :trigger="['hover']"
            placement="bottomRight"
            :open="openProjectMenuId === Number(column.id)"
            @openChange="(open) => handleProjectMenuOpen(column.id, open)"
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
                  @click="hideProject(column.id)"
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
                    :class="{ active: customColor(column) === color }"
                    :style="{ backgroundColor: color }"
                    :aria-label="color"
                    @click="setProjectColor(column.id, color)"
                  />
                  <button
                    type="button"
                    class="kanban-color-reset"
                    :class="{ active: !customColor(column) }"
                    :style="{ backgroundColor: defaultColumnColor() }"
                    :aria-label="$t('workspace_restoreDefault')"
                    @click="setProjectColor(column.id, '')"
                  >
                    <CloseOutlined />
                  </button>
                </div>
                <label class="kanban-color-picker">
                  <span>{{ $t("workspace_customColor") }}</span>
                  <input
                    type="color"
                    :value="customColor(column) || '#f7f8fa'"
                    @mousedown.stop="startColorPicker(column.id)"
                    @focus="startColorPicker(column.id)"
                    @change="handleCustomColorChange(column.id, $event)"
                    @blur="finishColorPicker(column.id)"
                  />
                </label>
              </div>
            </template>
          </a-dropdown>
        </header>

        <div class="kanban-card-list">
          <a-empty
            v-if="!column.items.length"
            :image="simpleImage"
            :description="$t('common_noData')"
            class="kanban-empty"
          />
          <article
            v-for="item in column.items"
            :key="`${item.workspace_id}:${item.id}`"
            class="my-work-kanban-card"
            :class="{ done: isDone(item), selected: isSelected(item) }"
            tabindex="0"
            @click="openItem(item)"
            @keydown.enter="openItem(item)"
          >
            <div class="card-title" :title="item.title">
              <span
                v-if="item.priority_name"
                class="card-priority"
                :style="{ color: item.priority_color || undefined }"
              >
                {{ item.priority_name.slice(0, 1) }}
              </span>
              {{ item.title }}
            </div>
            <div class="card-meta">
              <StatusPill :text="item.status_name" :color="item.status_color" />
              <span class="card-updated-at">
                {{ relativeUpdatedAt(item.updated_at) }}
              </span>
            </div>
          </article>
        </div>
      </section>
    </div>
    <a-empty
      v-else
      :image="simpleImage"
      :description="$t('myWork_noVisibleProjects')"
      class="kanban-no-columns"
    />
  </div>
</template>

<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import dayjs from "dayjs";
import { Empty } from "ant-design-vue";
import Sortable from "sortablejs";
import { useI18n } from "vue-i18n";
import {
  CloseOutlined,
  EllipsisOutlined,
  EyeInvisibleOutlined,
} from "@ant-design/icons-vue";
import StatusPill from "@/components/work-item/StatusPill.vue";
import { useWorkspaceStore } from "@/stores/workspace";

const workspaceStore = useWorkspaceStore();
// Reuse the sidebar's project icon: a custom image when set, otherwise a colored
// initial tile. Keeps "My Work" consistent with the sidebar project list.
function columnIconUrl(id) {
  return workspaceStore.iconUrls[String(id)] || "";
}

const props = defineProps({
  columns: { type: Array, default: () => [] },
  selectedItemKey: { type: String, default: "" },
  projectColors: { type: Object, default: () => ({}) },
});
const emit = defineEmits([
  "open",
  "project-visibility-change",
  "project-color-change",
  "project-order-change",
]);
const { t } = useI18n();
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;
const boardRef = ref(null);
const openProjectMenuId = ref(0);
const colorPickerProjectId = ref(0);
const colorOptions = ["#fff7cc", "#e8f0ff", "#e9f8ef", "#f1f2f4", "#ffffff"];
let sortable;

function projectInitial(project) {
  return (project.name || "?").trim().charAt(0);
}

function customColor(project) {
  return props.projectColors[String(project.id)] || "";
}

function defaultColumnColor() {
  return "#f7f8fa";
}

function isSelected(item) {
  return props.selectedItemKey === `${item.workspace_id}:${item.id}`;
}

function hideProject(id) {
  openProjectMenuId.value = 0;
  emit("project-visibility-change", { id: Number(id), visible: false });
}

function setProjectColor(id, color) {
  emit("project-color-change", { id: Number(id), color });
  if (!color) openProjectMenuId.value = 0;
}

function handleProjectMenuOpen(id, open) {
  const projectId = Number(id);
  if (open) openProjectMenuId.value = projectId;
  else if (colorPickerProjectId.value !== projectId)
    openProjectMenuId.value = 0;
}

function startColorPicker(id) {
  colorPickerProjectId.value = Number(id);
  openProjectMenuId.value = Number(id);
}

function finishColorPicker(id) {
  const projectId = Number(id);
  window.setTimeout(() => {
    if (colorPickerProjectId.value !== projectId) return;
    colorPickerProjectId.value = 0;
    openProjectMenuId.value = 0;
  }, 100);
}

function handleCustomColorChange(id, event) {
  setProjectColor(id, event.target.value);
  finishColorPicker(id);
}

function initializeSortable() {
  sortable?.destroy();
  const container = boardRef.value?.querySelector(".kanban-columns");
  if (!container) return;
  sortable = new Sortable(container, {
    animation: 160,
    handle: ".kanban-column-header",
    filter: ".kanban-more-button, .kanban-column-menu, input",
    preventOnFilter: true,
    draggable: ".kanban-column",
    ghostClass: "kanban-column-ghost",
    onEnd: () => {
      const order = [...container.querySelectorAll(".kanban-column")].map(
        (element) => element.dataset.projectId,
      );
      emit("project-order-change", order.filter(Boolean));
    },
  });
}

watch(
  () => props.columns.map((column) => column.id).join(","),
  () => nextTick(initializeSortable),
);
onMounted(initializeSortable);
onBeforeUnmount(() => sortable?.destroy());

function isDone(item) {
  return (
    item?.is_done === true ||
    item?.is_done === 1 ||
    item?.is_done === "1" ||
    item?.is_done === "true"
  );
}

function relativeUpdatedAt(value) {
  const timestamp = Number(value);
  if (!timestamp) return "";
  const elapsed = Math.max(0, dayjs().unix() - timestamp);
  if (elapsed < 60) return t("workspace_updatedJustNow");
  if (elapsed < 3600)
    return t("workspace_updatedMinutesAgo", { n: Math.floor(elapsed / 60) });
  if (elapsed < 86400)
    return t("workspace_updatedHoursAgo", { n: Math.floor(elapsed / 3600) });
  if (elapsed <= 60 * 86400)
    return t("workspace_updatedDaysAgo", {
      n: Math.floor(elapsed / 86400),
    });
  return dayjs.unix(timestamp).format("MM-DD");
}

function openItem(item) {
  emit("open", item);
}
</script>

<style scoped lang="less">
.my-work-kanban,
.kanban-columns {
  min-width: 0;
  height: 100%;
}

.my-work-kanban {
  min-height: 0;
  flex: 1;
  overflow: hidden;
}

.kanban-columns {
  display: flex;
  align-items: stretch;
  gap: 16px;
  overflow-x: auto;
  padding: 16px 0 8px;
}

.kanban-column {
  display: flex;
  width: 288px;
  min-width: 288px;
  min-height: 320px;
  flex-direction: column;
  overflow: hidden;
  border-radius: 8px;
  background: #f7f8fa;
}

.kanban-column-header {
  display: flex;
  height: 54px;
  flex: none;
  align-items: center;
  gap: 8px;
  padding: 0 14px;
  color: var(--app-text);
  cursor: grab;

  strong {
    min-width: 0;
    max-width: 180px;
    flex: 0 1 auto;
    overflow: hidden;
    font-size: 14px;
    font-weight: 600;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

.column-avatar {
  flex: none;
  color: #fff;
  background: #315bea;
  font-size: 14px !important;
}
.column-avatar :deep(.ant-avatar-string) {
  line-height: 20px !important;
}

.column-icon-image {
  flex: none;
  width: 24px;
  height: 24px;
  border-radius: 6px;
  object-fit: cover;
}

.kanban-more-button {
  display: inline-flex;
  width: 28px;
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
  box-shadow: 0 6px 18px rgb(16 24 40 / 14%);
}

.kanban-column-menu-item,
.kanban-color-picker {
  display: flex;
  width: 100%;
  min-height: 30px;
  align-items: center;
  gap: 8px;
  padding: 4px 8px;
  border: 0;
  color: var(--app-text);
  background: transparent;
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
  color: #8c8c8c;
  background: #fff;
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

.kanban-column-ghost {
  border: 1px dashed #91caff;
  opacity: 0.45;
}

.kanban-column-header:active {
  cursor: grabbing;
}

.column-color {
  width: 12px;
  height: 12px;
  flex: none;
  border-radius: 50%;
}

.kanban-count {
  display: inline-flex;
  min-width: 22px;
  height: 22px;
  align-items: center;
  justify-content: center;
  padding: 0 6px;
  border-radius: 11px;
  color: #68758c;
  background: #e9edf3;
  font-size: 12px;
}

.kanban-card-list {
  min-height: 0;
  flex: 1;
  overflow-y: auto;
  padding: 0 12px 16px;
}

.kanban-empty {
  margin-top: 72px;
}

.my-work-kanban-card {
  display: flex;
  min-height: 115px;
  flex-direction: column;
  padding: 16px;
  border: 1px solid #dfe4eb;
  border-radius: 8px;
  background: #fff;
  box-shadow: 0 1px 2px rgb(16 24 40 / 4%);
  cursor: pointer;
  outline: none;
  transition:
    border-color 0.15s,
    box-shadow 0.15s,
    opacity 0.15s;

  & + & {
    margin-top: 10px;
  }

  &:hover,
  &:focus-visible {
    border-color: #91caff;
    box-shadow: 0 3px 8px rgb(22 119 255 / 10%);
  }

  &.done {
    opacity: 0.58;
  }

  &.selected {
    border-color: #1677ff;
    background: #f0f7ff;
    opacity: 1;
  }
}

.card-title {
  min-height: 42px;
  overflow: hidden;
  color: var(--app-text);
  font-size: 14px;
  font-weight: 600;
  line-height: 21px;
  overflow-wrap: anywhere;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.card-priority {
  display: inline-flex;
  width: 20px;
  height: 20px;
  align-items: center;
  justify-content: center;
  margin-right: 4px;
  border-radius: 4px;
  background: #f2f4f7;
  font-size: 12px;
  font-weight: 600;
  vertical-align: top;
}

.card-meta {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: auto;
  padding-top: 10px;
}

.card-updated-at {
  min-width: 0;
  flex: none;
  color: #98a2b3;
  font-size: 12px;
  white-space: nowrap;
}

.kanban-no-columns {
  margin-top: 120px;
}

@media (max-width: 860px) {
  .kanban-column {
    width: min(288px, calc(100vw - 64px));
    min-width: min(288px, calc(100vw - 64px));
  }
}
</style>

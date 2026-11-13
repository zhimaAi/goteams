<template>
  <div ref="boardRef" class="team-work-kanban">
    <div v-if="columns.length" class="kanban-columns">
      <section
        v-for="column in columns"
        :key="column.id"
        class="kanban-column"
        :style="{
          backgroundColor: customColor(column) || defaultColumnColor(column),
        }"
      >
        <header class="kanban-column-header">
          <a-avatar
            :size="24"
            :src="column.avatar || undefined"
            class="column-avatar"
          >
            {{ memberInitial(column) }}
          </a-avatar>
          <strong :title="column.name">{{ column.name }}</strong>
          <span class="kanban-count">{{ column.items.length }}</span>
          <a-dropdown
            :trigger="['hover']"
            placement="bottomRight"
            :open="openMemberMenuId === Number(column.id)"
            @openChange="(open) => handleMemberMenuOpen(column.id, open)"
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
                  @click="hideMember(column.id)"
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
                    @click="setMemberColor(column.id, color)"
                  />
                  <button
                    type="button"
                    class="kanban-color-reset"
                    :class="{ active: !customColor(column) }"
                    :style="{ backgroundColor: defaultColumnColor(column) }"
                    :aria-label="$t('workspace_restoreDefault')"
                    @click="setMemberColor(column.id, '')"
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

        <div class="kanban-card-list" :data-member-id="column.id">
          <a-empty
            v-if="!column.items.length"
            :image="simpleImage"
            :description="$t('common_noData')"
            class="kanban-empty"
          />

          <article
            v-for="item in column.items"
            :key="`${item.workspace_id}:${item.id}`"
            class="team-work-kanban-card"
            :class="{
              draggable: canEdit(item),
              viewable: canClickDetail(item),
              done: isDone(item),
              selected: isSelected(item),
            }"
            :data-workspace-id="item.workspace_id"
            :data-work-item-id="item.id"
            :tabindex="canClickDetail(item) ? 0 : -1"
            @click="openItem(item)"
            @keydown.enter="openItem(item)"
            @keydown.space.prevent="openItem(item)"
          >
            <div class="card-title">
              <span class="card-title-text" :title="item.title">
                <span
                  v-if="item.priority_name"
                  class="card-priority"
                  :style="{ color: item.priority_color || undefined }"
                >
                  {{ item.priority_name.slice(0, 1) }}
                </span>
                {{ item.title }}
              </span>
            </div>
            <div class="card-project" :title="item.workspace_name" v-if="false">
              {{ item.workspace_name || "-" }}
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
      :description="$t('performance_noVisibleMembers')"
      class="kanban-no-columns"
    />
  </div>
</template>

<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Empty } from "ant-design-vue";
import dayjs from "dayjs";
import Sortable from "sortablejs";
import { useI18n } from "vue-i18n";
import {
  CloseOutlined,
  EllipsisOutlined,
  EyeInvisibleOutlined,
} from "@ant-design/icons-vue";
import StatusPill from "@/components/work-item/StatusPill.vue";

const props = defineProps({
  columns: { type: Array, default: () => [] },
  selectedItemKey: { type: String, default: "" },
  memberColors: { type: Object, default: () => ({}) },
});

const emit = defineEmits([
  "open",
  "move",
  "member-visibility-change",
  "member-color-change",
]);
const { t } = useI18n();
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;
const boardRef = ref(null);
const openMemberMenuId = ref(0);
const colorPickerMemberId = ref(0);
const colorOptions = ["#fff7cc", "#e8f0ff", "#e9f8ef", "#f1f2f4", "#ffffff"];
let sortables = [];

function boolValue(value) {
  return value === true || value === 1 || value === "1" || value === "true";
}

function canView(item) {
  return boolValue(item?.can_view);
}

function canClickDetail(item) {
  return !!item && (!canView(item) || boolValue(item.can_open_detail));
}

function canEdit(item) {
  return boolValue(item?.can_edit);
}

function isDone(item) {
  return boolValue(item?.is_done);
}

function isSelected(item) {
  return (
    props.selectedItemKey === `${item.workspace_id}:${item.id}` &&
    boolValue(item.can_open_detail)
  );
}

function memberInitial(member) {
  return (member.name || member.username || "?").trim().charAt(0).toUpperCase();
}

function customColor(member) {
  return props.memberColors[String(member.id)] || "";
}

function defaultColumnColor() {
  return "#f7f8fa";
}

function hideMember(id) {
  openMemberMenuId.value = 0;
  emit("member-visibility-change", { id: Number(id), visible: false });
}

function setMemberColor(id, color) {
  emit("member-color-change", { id: Number(id), color });
  if (!color) openMemberMenuId.value = 0;
}

function handleMemberMenuOpen(id, open) {
  const memberId = Number(id);
  if (open) {
    openMemberMenuId.value = memberId;
    return;
  }
  if (colorPickerMemberId.value !== memberId) openMemberMenuId.value = 0;
}

function startColorPicker(id) {
  const memberId = Number(id);
  colorPickerMemberId.value = memberId;
  openMemberMenuId.value = memberId;
}

function finishColorPicker(id) {
  const memberId = Number(id);
  window.setTimeout(() => {
    if (colorPickerMemberId.value !== memberId) return;
    colorPickerMemberId.value = 0;
    openMemberMenuId.value = 0;
  }, 100);
}

function handleCustomColorChange(id, event) {
  setMemberColor(id, event.target.value);
  finishColorPicker(id);
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
  if (!canClickDetail(item)) return;
  emit("open", item);
}

function restoreCardPosition(event) {
  const source = event.from;
  const card = event.item;
  const originalIndex = Number(event.oldIndex);
  if (!source || !card || !Number.isInteger(originalIndex)) return;
  if (card.parentElement === source) source.removeChild(card);
  const reference = source.children[originalIndex];
  if (reference) source.insertBefore(card, reference);
  else source.appendChild(card);
}

function findItem(memberId, workspaceId, itemId) {
  const column = props.columns.find(
    (candidate) => Number(candidate.id) === Number(memberId),
  );
  return column?.items.find(
    (item) =>
      Number(item.workspace_id) === Number(workspaceId) &&
      Number(item.id) === Number(itemId),
  );
}

function handleCardMove(event) {
  const sourceMemberId = Number(event.from?.dataset.memberId);
  const targetMemberId = Number(event.to?.dataset.memberId);
  const workspaceId = Number(event.item?.dataset.workspaceId);
  const itemId = Number(event.item?.dataset.workItemId);
  restoreCardPosition(event);
  if (!sourceMemberId || !targetMemberId || sourceMemberId === targetMemberId)
    return;
  const item = findItem(sourceMemberId, workspaceId, itemId);
  if (!item || !canEdit(item)) return;
  emit("move", { item, sourceMemberId, targetMemberId });
}

function destroySortables() {
  sortables.forEach((sortable) => sortable.destroy());
  sortables = [];
}

function initializeSortables() {
  destroySortables();
  const lists = boardRef.value?.querySelectorAll(".kanban-card-list") || [];
  sortables = [...lists].map(
    (element) =>
      new Sortable(element, {
        group: "team-work-members",
        animation: 160,
        draggable: ".team-work-kanban-card.draggable",
        ghostClass: "kanban-card-ghost",
        chosenClass: "kanban-card-chosen",
        dragClass: "kanban-card-drag",
        onEnd: handleCardMove,
      }),
  );
}

watch(
  () => props.columns.map((column) => column.id).join(","),
  () => nextTick(initializeSortables),
);

onMounted(initializeSortables);
onBeforeUnmount(destroySortables);
</script>

<style scoped lang="less">
.team-work-kanban,
.kanban-columns {
  min-width: 0;
  height: 100%;
}

.team-work-kanban {
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
  font-size: 11px;
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

.kanban-card-list {
  min-height: 0;
  flex: 1;
  overflow-y: auto;
  padding: 0 12px 16px;
}

.kanban-empty {
  margin-top: 72px;
}

.team-work-kanban-card {
  display: flex;
  min-height: 115px;
  flex-direction: column;
  padding: 16px;
  border: 1px solid #dfe4eb;
  border-radius: 8px;
  background: #fff;
  box-shadow: 0 1px 2px rgb(16 24 40 / 4%);
  outline: none;
  transition:
    border-color 0.15s,
    box-shadow 0.15s,
    opacity 0.15s;

  & + & {
    margin-top: 10px;
  }

  &.viewable {
    cursor: pointer;
  }

  &.viewable:hover,
  &.viewable:focus-visible {
    border-color: #91caff;
    box-shadow: 0 3px 8px rgb(22 119 255 / 10%);
  }

  &.draggable {
    cursor: grab;
  }

  &.draggable:active {
    cursor: grabbing;
  }

  &.done:not(.selected) {
    opacity: 0.58;
  }

  &.selected {
    border-color: #1677ff;
    background: #f0f7ff;
  }
}

.card-title {
  min-height: 42px;
  color: var(--app-text);
  font-size: 14px;
  font-weight: 600;
  line-height: 21px;
}

.card-title-text {
  display: -webkit-box;
  overflow: hidden;
  overflow-wrap: anywhere;
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

.card-project {
  margin-top: 10px;
  overflow: hidden;
  color: #7c8799;
  font-size: 12px;
  line-height: 20px;
  text-overflow: ellipsis;
  white-space: nowrap;
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

.kanban-card-ghost {
  border-style: dashed;
  opacity: 0.38;
}

.kanban-card-chosen,
.kanban-card-drag {
  box-shadow: 0 8px 20px rgb(16 24 40 / 16%);
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

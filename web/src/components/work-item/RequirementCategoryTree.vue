<template>
  <div
    ref="listRef"
    class="category-sort-list"
    :class="{ nested: depth > 1, empty: !nodes.length }"
    :data-parent-id="parentId"
    :data-depth="depth"
  >
    <div
      v-for="node in nodes"
      :key="node.id"
      class="category-tree-item"
      :data-category-id="node.id"
      :data-subtree-height="subtreeHeight(node)"
    >
      <div
        class="category-node-row"
        :class="{
          selected: selectedKey === categoryKey(node.id),
          'menu-open': openMenuId === node.id,
        }"
        :style="{ '--category-depth': depth }"
        role="treeitem"
        tabindex="0"
        :aria-selected="selectedKey === categoryKey(node.id)"
        :aria-expanded="node.children?.length ? isExpanded(node.id) : undefined"
        @click="$emit('select', categoryKey(node.id))"
        @keydown.enter.prevent="$emit('select', categoryKey(node.id))"
        @keydown.space.prevent="$emit('select', categoryKey(node.id))"
      >
        <span
          v-if="canDrag"
          class="category-drag-handle"
          :title="$t('requirement_categoryDrag')"
          @click.stop
        >
          <HolderOutlined />
        </span>
        <span v-else class="category-drag-placeholder" />
        <button
          v-if="node.children?.length"
          type="button"
          class="category-switcher"
          :aria-label="
            isExpanded(node.id) ? $t('common_collapse') : $t('common_expand')
          "
          :disabled="forceExpanded"
          @click.stop="$emit('toggle', node.id)"
        >
          <CaretDownOutlined v-if="isExpanded(node.id)" />
          <CaretRightOutlined v-else />
        </button>
        <span v-else class="category-switcher-spacer" />
        <span class="category-node-name" :title="node.name">{{
          node.name
        }}</span>
        <span class="category-node-count">{{ node.count }}</span>
        <a-dropdown
          v-if="canCreate || canManage"
          trigger="click"
          placement="bottomRight"
          @openChange="(open) => setMenuOpen(node.id, open)"
        >
          <button
            type="button"
            class="category-more-button"
            :class="{ active: openMenuId === node.id }"
            :aria-label="$t('common_more')"
            @click.stop
          >
            <MoreOutlined />
          </button>
          <template #overlay>
            <a-menu @click="({ key }) => handleAction(key, node)">
              <a-menu-item v-if="canCreate" key="create-work-item">
                <PlusOutlined /> {{ $t("requirement_categoryCreateWorkItem") }}
              </a-menu-item>
              <a-menu-item v-if="canManage && depth < 3" key="create-child">
                <ApartmentOutlined />
                {{ $t("requirement_categoryCreateChild") }}
              </a-menu-item>
              <a-menu-divider v-if="(canCreate || canManage) && canManage" />
              <a-menu-item v-if="canManage" key="edit">
                <EditOutlined /> {{ $t("common_edit") }}
              </a-menu-item>
              <a-menu-item v-if="canManage" key="delete" danger>
                <DeleteOutlined /> {{ $t("common_delete") }}
              </a-menu-item>
            </a-menu>
          </template>
        </a-dropdown>
      </div>

      <RequirementCategoryTree
        v-show="
          isExpanded(node.id) ||
          (canDrag && depth < 3 && !node.children?.length)
        "
        :nodes="node.children || []"
        :parent-id="node.id"
        :depth="depth + 1"
        :ws-id="wsId"
        :selected-key="selectedKey"
        :expanded-keys="expandedKeys"
        :force-expanded="forceExpanded"
        :can-manage="canManage"
        :can-drag="canDrag"
        :can-create="canCreate"
        @select="$emit('select', $event)"
        @toggle="$emit('toggle', $event)"
        @action="$emit('action', $event)"
        @move="$emit('move', $event)"
        @drag-state="$emit('drag-state', $event)"
      />
    </div>
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import Sortable from "sortablejs";
import {
  DeleteOutlined,
  CaretDownOutlined,
  CaretRightOutlined,
  ApartmentOutlined,
  EditOutlined,
  HolderOutlined,
  MoreOutlined,
  PlusOutlined,
} from "@ant-design/icons-vue";

const props = defineProps({
  nodes: { type: Array, default: () => [] },
  parentId: { type: Number, required: true },
  depth: { type: Number, required: true },
  wsId: { type: [String, Number], required: true },
  selectedKey: { type: String, default: "all" },
  expandedKeys: { type: Array, default: () => [] },
  forceExpanded: { type: Boolean, default: false },
  canManage: { type: Boolean, default: false },
  canDrag: { type: Boolean, default: false },
  canCreate: { type: Boolean, default: false },
});

const emit = defineEmits(["select", "toggle", "action", "move", "drag-state"]);
const listRef = ref(null);
const openMenuId = ref(null);
let sortable;

function categoryKey(id) {
  return `category:${id}`;
}

function isExpanded(id) {
  return props.forceExpanded || props.expandedKeys.includes(Number(id));
}

function setMenuOpen(nodeId, open) {
  openMenuId.value = open ? nodeId : null;
}

function handleAction(action, node) {
  openMenuId.value = null;
  emit("action", { action, node });
}

function subtreeHeight(node) {
  return 1 + Math.max(0, ...(node.children || []).map(subtreeHeight));
}

function directCategoryIds(element) {
  return [...element.children]
    .filter((child) => child.classList.contains("category-tree-item"))
    .map((child) => Number(child.dataset.categoryId))
    .filter((id) => Number.isInteger(id) && id > 0);
}

function setupSortable() {
  sortable?.destroy();
  sortable = undefined;
  if (!props.canDrag || !listRef.value) return;
  sortable = Sortable.create(listRef.value, {
    group: `requirement-categories-${props.wsId}`,
    animation: 140,
    handle: ".category-drag-handle",
    draggable: ".category-tree-item",
    fallbackOnBody: true,
    fallbackTolerance: 4,
    emptyInsertThreshold: 16,
    scroll: true,
    scrollSensitivity: 48,
    scrollSpeed: 12,
    swapThreshold: 0.65,
    ghostClass: "category-sort-ghost",
    chosenClass: "category-sort-chosen",
    dragClass: "category-sort-drag",
    fallbackClass: "category-sort-fallback",
    onMove: ({ dragged, to }) => {
      const targetDepth = Number(to.dataset.depth || 0);
      const height = Number(dragged.dataset.subtreeHeight || 1);
      return !dragged.contains(to) && targetDepth + height - 1 <= 3;
    },
    onStart: () => {
      openMenuId.value = null;
      emit("drag-state", true);
    },
    onEnd: (event) => {
      emit("drag-state", false);
      if (
        event.from === event.to &&
        event.oldDraggableIndex === event.newDraggableIndex
      ) {
        return;
      }
      const ids = directCategoryIds(event.to);
      const categoryId = Number(event.item.dataset.categoryId);
      const index = ids.indexOf(categoryId);
      if (!Number.isInteger(categoryId) || categoryId <= 0 || index < 0) {
        emit("move", {});
        return;
      }
      emit("move", {
        categoryId,
        parentId: Number(event.to.dataset.parentId) || 0,
        prevId: index > 0 ? ids[index - 1] : 0,
        nextId: index >= 0 && index < ids.length - 1 ? ids[index + 1] : 0,
      });
    },
  });
}

onMounted(setupSortable);
onBeforeUnmount(() => sortable?.destroy());
watch(() => [props.canDrag, props.wsId], setupSortable);
</script>

<style scoped lang="less">
.category-sort-list {
  min-height: 3px;
}
.category-sort-list.nested {
  position: relative;
  margin-left: 9px;
}
.category-sort-list.empty {
  min-height: 0;
  border-left: 0;
}
.category-tree-item {
  position: relative;
}
.category-node-row {
  display: flex;
  align-items: center;
  width: 100%;
  height: 36px;
  padding: 0 6px 0 10px;
  border-radius: 4px;
  color: var(--app-text);
  cursor: pointer;
  user-select: none;
  transition:
    color 0.15s ease,
    background 0.15s ease;
}
.category-node-row:hover {
  background: var(--app-surface-muted);
}
.category-node-row:focus-visible {
  outline: 2px solid #91caff;
  outline-offset: -2px;
}
.category-node-row.selected {
  color: #1677ff;
  background: #edf5ff;
}
.category-node-row.selected .category-node-name {
  font-weight: 500;
}
.category-drag-handle,
.category-drag-placeholder {
  width: 20px;
  flex: 0 0 20px;
}
.category-drag-handle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--app-text-secondary);
  font-size: 14px;
  cursor: grab;
  opacity: 0;
  transition:
    color 0.15s ease,
    opacity 0.15s ease;
}
.category-node-row:hover .category-drag-handle,
.category-node-row:focus-within .category-drag-handle,
.category-node-row.menu-open .category-drag-handle,
:global(.requirement-category-sidebar.is-dragging) .category-drag-handle {
  opacity: 1;
}
.category-node-row.selected .category-drag-handle,
:global(.requirement-category-sidebar.is-dragging) .category-drag-handle {
  color: #1677ff;
}
.category-drag-handle:active {
  cursor: grabbing;
}
.category-switcher,
.category-switcher-spacer {
  width: 18px;
  height: 18px;
  flex: 0 0 18px;
}
.category-switcher {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 0;
  color: #8b95a7;
  background: transparent;
  font-size: 10px;
  cursor: pointer;
}
.category-node-name {
  min-width: 0;
  margin-left: 2px;
  overflow: hidden;
  font-size: 14px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.category-node-count {
  margin-left: 8px;
  color: #8796b2;
  font-size: 12px;
}
.category-node-row.selected .category-node-count {
  color: #8796b2;
}
.category-more-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  margin-left: auto;
  padding: 0;
  border: 0;
  border-radius: 5px;
  color: var(--app-text-secondary);
  background: transparent;
  cursor: pointer;
  opacity: 0;
  transition:
    color 0.15s ease,
    background 0.15s ease,
    opacity 0.15s ease;
}
.category-node-row:hover .category-more-button,
.category-node-row:focus-within .category-more-button,
.category-node-row.menu-open .category-more-button,
.category-more-button:focus-visible,
.category-more-button.active {
  opacity: 1;
}
.category-node-row.selected .category-more-button {
  color: #1677ff;
}
.category-more-button:hover,
.category-more-button:focus-visible,
.category-more-button.active {
  color: var(--app-text);
  background: var(--app-surface-muted);
}
:global(.requirement-category-sidebar.is-dragging .category-sort-list.empty) {
  min-height: 6px;
  border-radius: 2px;
  background: transparent;
  box-shadow: inset 0 1px 0 #b9d5ff;
}
:global(.requirement-category-sidebar .category-sort-ghost) {
  opacity: 0.22;
}
:global(
  .requirement-category-sidebar .category-sort-ghost > .category-sort-list
),
:global(
  .requirement-category-sidebar .category-sort-chosen > .category-sort-list
),
:global(
  .requirement-category-sidebar .category-sort-drag > .category-sort-list
),
:global(
  .requirement-category-sidebar .category-sort-fallback > .category-sort-list
) {
  display: none;
}
:global(
  .requirement-category-sidebar .category-sort-chosen > .category-node-row
) {
  color: #1677ff;
  background: color-mix(in srgb, #1677ff 10%, var(--app-surface));
}
:global(.category-sort-drag .category-node-row),
:global(.category-sort-fallback .category-node-row) {
  background: var(--app-surface);
  box-shadow: 0 2px 6px rgb(31 45 61 / 12%);
}
</style>

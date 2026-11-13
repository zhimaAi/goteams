<template>
  <aside
    class="requirement-category-sidebar"
    :class="{
      collapsed,
      'is-dragging': dragging,
      'is-ordering': ordering,
      'is-resizing': resizing,
    }"
  >
    <div v-show="!collapsed" class="category-sidebar-content">
      <div v-if="$slots.views" class="sidebar-tabs">
        <button
          v-for="tab in sidebarTabs"
          :key="tab.key"
          type="button"
          :aria-pressed="sidebarTab === tab.key"
          @click="sidebarTab = tab.key"
        >
          {{ $t(tab.label) }}
        </button>
      </div>
      <div
        v-if="$slots.views"
        v-show="sidebarTab === 'views'"
        class="sidebar-views"
      >
        <slot name="views" />
      </div>
      <div
        v-show="!$slots.views || sidebarTab === 'categories'"
        class="category-sidebar-toolbar"
      >
        <a-input
          v-model:value="searchText"
          allow-clear
          class="category-search"
          :placeholder="$t('requirement_categorySearch')"
        >
          <template #prefix><SearchOutlined /></template>
        </a-input>
        <a-tooltip v-if="canManage" :title="$t('requirement_categoryCreate')">
          <button
            type="button"
            class="category-add-button"
            :aria-label="$t('requirement_categoryCreate')"
            :disabled="ordering"
            @click="openCreate()"
          >
            <PlusOutlined />
          </button>
        </a-tooltip>
      </div>

      <div
        v-show="!$slots.views || sidebarTab === 'categories'"
        class="category-tree-scroll"
        role="tree"
      >
        <div
          class="category-virtual-row category-all-row"
          :class="{ selected: selectedKey === 'all' }"
          role="treeitem"
          tabindex="0"
          :aria-selected="selectedKey === 'all'"
          :aria-expanded="rootExpanded || hasSearch"
          @click="$emit('select', 'all')"
          @keydown.enter.prevent="$emit('select', 'all')"
          @keydown.space.prevent="$emit('select', 'all')"
        >
          <button
            type="button"
            class="category-root-switcher"
            :aria-label="
              rootExpanded || hasSearch
                ? $t('common_collapse')
                : $t('common_expand')
            "
            :disabled="hasSearch"
            @click.stop="rootExpanded = !rootExpanded"
          >
            <CaretDownOutlined v-if="rootExpanded || hasSearch" />
            <CaretRightOutlined v-else />
          </button>
          <span class="category-virtual-name">{{
            $t("requirement_categoryAll")
          }}</span>
          <span class="category-virtual-count">{{ allCount }}</span>
        </div>

        <div v-show="rootExpanded || hasSearch" class="category-root-children">
          <div
            class="category-virtual-row category-uncategorized-row"
            :class="{ selected: selectedKey === 'uncategorized' }"
            role="treeitem"
            tabindex="0"
            :aria-selected="selectedKey === 'uncategorized'"
            @click="$emit('select', 'uncategorized')"
            @keydown.enter.prevent="$emit('select', 'uncategorized')"
            @keydown.space.prevent="$emit('select', 'uncategorized')"
          >
            <span class="category-virtual-indent" />
            <span class="category-virtual-name">{{
              $t("requirement_categoryUncategorized")
            }}</span>
            <span class="category-virtual-count">{{ uncategorizedCount }}</span>
          </div>

          <RequirementCategoryTree
            :key="treeVersion"
            :nodes="filteredTree"
            :parent-id="0"
            :depth="1"
            :ws-id="wsId"
            :selected-key="selectedKey"
            :expanded-keys="expandedKeys"
            :force-expanded="hasSearch"
            :can-manage="canManage"
            :can-drag="canManage && !hasSearch && !ordering"
            :can-create="canCreate"
            @select="$emit('select', $event)"
            @toggle="toggleCategory"
            @action="handleCategoryAction"
            @move="handleMove"
            @drag-state="dragging = $event"
          />

          <div
            v-if="hasSearch && !filteredTree.length"
            class="category-search-empty"
          >
            {{ $t("requirement_categoryNoMatch") }}
          </div>
        </div>
      </div>
    </div>

    <span
      v-if="!collapsed"
      class="category-sidebar-resizer"
      role="separator"
      aria-orientation="vertical"
      :aria-valuenow="width"
      aria-valuemin="220"
      aria-valuemax="420"
      @pointerdown.prevent="startResize"
    />

    <button
      type="button"
      class="category-collapse-handle"
      :class="{ collapsed }"
      :aria-label="collapsed ? $t('common_expand') : $t('common_collapse')"
      @click="$emit('update:collapsed', !collapsed)"
    >
      <RightOutlined v-if="collapsed" />
      <LeftOutlined v-else />
    </button>

    <a-modal
      v-model:open="editorOpen"
      :title="
        editingCategory
          ? $t('requirement_categoryEdit')
          : creatingParent
            ? $t('requirement_categoryCreateChild')
            : $t('requirement_categoryCreate')
      "
      :confirm-loading="saving"
      :ok-text="editingCategory ? $t('common_save') : $t('common_create')"
      :cancel-text="$t('common_cancel')"
      @ok="saveCategory"
      @cancel="closeEditor"
    >
      <a-form layout="vertical">
        <a-alert
          v-if="creatingParent"
          type="info"
          show-icon
          class="category-parent-hint"
          :message="
            $t('requirement_categoryCreateChildHint', {
              name: creatingParent.name,
            })
          "
        />
        <a-form-item :label="$t('requirement_categoryName')" required>
          <a-input
            v-model:value="categoryName"
            :maxlength="20"
            show-count
            autofocus
            :placeholder="$t('requirement_categoryNamePlaceholder')"
            @press-enter="saveCategory"
          />
        </a-form-item>
      </a-form>
    </a-modal>
  </aside>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import {
  CaretDownOutlined,
  CaretRightOutlined,
  LeftOutlined,
  PlusOutlined,
  RightOutlined,
  SearchOutlined,
} from "@ant-design/icons-vue";
import RequirementCategoryTree from "@/components/work-item/RequirementCategoryTree.vue";
import {
  createRequirementCategory,
  deleteRequirementCategory,
  listRequirementCategories,
  moveRequirementCategory,
  updateRequirementCategory,
} from "@/api/workspace";

const props = defineProps({
  wsId: { type: [String, Number], required: true },
  selectedKey: { type: String, default: "all" },
  collapsed: { type: Boolean, default: false },
  initialWidth: { type: Number, default: 280 },
  canManage: { type: Boolean, default: false },
  canCreate: { type: Boolean, default: false },
});

const emit = defineEmits([
  "select",
  "update:collapsed",
  "categories-loaded",
  "create-work-item",
  "changed",
  "resize",
  "resize-end",
]);
const { t: $t } = useI18n();

const sidebarTab = ref("views");
const sidebarTabs = [
  { key: "views", label: "view_button" },
  { key: "categories", label: "requirement_category_tab" },
];

const categories = ref([]);
const loaded = ref(false);
const treeVersion = ref(0);
const allCount = ref(0);
const uncategorizedCount = ref(0);
const searchText = ref("");
const rootExpanded = ref(true);
const expandedKeys = ref([]);
const dragging = ref(false);
const ordering = ref(false);
const resizing = ref(false);
const editorOpen = ref(false);
const editingCategory = ref(null);
const creatingParent = ref(null);
const categoryName = ref("");
const saving = ref(false);
const width = ref(Math.max(220, Math.min(420, props.initialWidth)));
let resizeStartX = 0;
let resizeStartWidth = 280;
let initializedWorkspaceId = "";

watch(
  () => props.initialWidth,
  (value) => setWidth(value),
);

const hasSearch = computed(() => !!searchText.value.trim());
const widthCss = computed(() => `${width.value}px`);

function buildCategoryTree(items) {
  const children = new Map();
  for (const item of items) {
    const parentId = Number(item.parent_id) || 0;
    if (!children.has(parentId)) children.set(parentId, []);
    children
      .get(parentId)
      .push({ ...item, id: Number(item.id), parent_id: parentId });
  }
  for (const siblings of children.values()) {
    siblings.sort(
      (left, right) =>
        Number(left.sort_order) - Number(right.sort_order) ||
        left.id - right.id,
    );
  }
  const append = (parentId, path) =>
    (children.get(parentId) || [])
      .filter((item) => !path.has(item.id))
      .map((item) => ({
        ...item,
        children: append(item.id, new Set(path).add(item.id)),
      }));
  return append(0, new Set());
}

const categoryTree = computed(() => buildCategoryTree(categories.value));

function filterCategoryTree(nodes, keyword) {
  return nodes.flatMap((node) => {
    const children = filterCategoryTree(node.children || [], keyword);
    if (node.name.toLocaleLowerCase().includes(keyword) || children.length) {
      return [{ ...node, children }];
    }
    return [];
  });
}

const filteredTree = computed(() => {
  const keyword = searchText.value.trim().toLocaleLowerCase();
  return keyword
    ? filterCategoryTree(categoryTree.value, keyword)
    : categoryTree.value;
});

async function refresh() {
  const workspaceId = String(props.wsId);
  try {
    const result = await listRequirementCategories(workspaceId);
    if (workspaceId !== String(props.wsId)) return [];
    categories.value = result?.list || [];
    loaded.value = true;
    treeVersion.value += 1;
    allCount.value = Number(result?.all_count) || 0;
    uncategorizedCount.value = Number(result?.uncategorized_count) || 0;
    if (initializedWorkspaceId !== workspaceId) {
      expandedKeys.value = categories.value.map((item) => Number(item.id));
      initializedWorkspaceId = workspaceId;
    }
    emit("categories-loaded", categories.value);
    if (
      props.selectedKey !== "all" &&
      props.selectedKey !== "uncategorized" &&
      (!props.selectedKey.startsWith("category:") ||
        !categories.value.some(
          (item) => `category:${item.id}` === props.selectedKey,
        ))
    ) {
      emit("select", "uncategorized");
    }
    return categories.value;
  } catch {
    return [];
  }
}

defineExpose({ refresh, setWidth });

function toggleCategory(id) {
  const target = Number(id);
  expandedKeys.value = expandedKeys.value.includes(target)
    ? expandedKeys.value.filter((key) => key !== target)
    : [...expandedKeys.value, target];
}

function openCreate(parent = null) {
  editingCategory.value = null;
  creatingParent.value = parent;
  categoryName.value = "";
  editorOpen.value = true;
}

function openEdit(node) {
  editingCategory.value = node;
  creatingParent.value = null;
  categoryName.value = node.name;
  editorOpen.value = true;
}

function closeEditor() {
  if (saving.value) return;
  editorOpen.value = false;
  editingCategory.value = null;
  creatingParent.value = null;
  categoryName.value = "";
}

function validCategoryName() {
  const value = categoryName.value.trim();
  if (!value) {
    message.warning($t("requirement_categoryNameRequired"));
    return false;
  }
  if ([...value].length > 20) {
    message.warning($t("requirement_categoryNameTooLong"));
    return false;
  }
  return true;
}

async function saveCategory() {
  if (saving.value || !validCategoryName()) return;
  saving.value = true;
  try {
    const name = categoryName.value.trim();
    if (editingCategory.value) {
      await updateRequirementCategory(props.wsId, editingCategory.value.id, {
        name,
      });
      message.success($t("requirement_categoryUpdated"));
    } else {
      await createRequirementCategory(props.wsId, {
        name,
        parent_id: Number(creatingParent.value?.id) || 0,
      });
      const parentId = Number(creatingParent.value?.id) || 0;
      if (parentId && !expandedKeys.value.includes(parentId)) {
        expandedKeys.value = [...expandedKeys.value, parentId];
      }
      message.success($t("requirement_categoryCreated"));
    }
    editorOpen.value = false;
    editingCategory.value = null;
    creatingParent.value = null;
    categoryName.value = "";
    await refresh();
  } catch {
    /* handled by interceptor */
  } finally {
    saving.value = false;
  }
}

async function removeCategory(node) {
  try {
    await deleteRequirementCategory(props.wsId, node.id);
    if (props.selectedKey === `category:${node.id}`) {
      emit("select", "uncategorized");
    }
    await refresh();
    emit("changed");
    message.success($t("requirement_categoryDeleted"));
  } catch {
    /* handled by interceptor */
  }
}

function handleCategoryAction({ action, node }) {
  if (action === "create-work-item") emit("create-work-item", node.id);
  if (action === "create-child") openCreate(node);
  if (action === "edit") openEdit(node);
  if (action === "delete") removeCategory(node);
}

function startResize(event) {
  if (props.collapsed) return;
  resizeStartX = event.clientX;
  resizeStartWidth = width.value;
  resizing.value = true;
  document.body.classList.add("category-sidebar-resizing");
  window.addEventListener("pointermove", handleResize);
  window.addEventListener("pointerup", stopResize, { once: true });
}

function handleResize(event) {
  width.value = Math.max(
    220,
    Math.min(420, resizeStartWidth + event.clientX - resizeStartX),
  );
  emit("resize", width.value);
}

function stopResize() {
  resizing.value = false;
  document.body.classList.remove("category-sidebar-resizing");
  window.removeEventListener("pointermove", handleResize);
  emit("resize-end", width.value);
}

onBeforeUnmount(() => {
  document.body.classList.remove("category-sidebar-resizing");
  window.removeEventListener("pointermove", handleResize);
  window.removeEventListener("pointerup", stopResize);
});

function setWidth(value) {
  width.value = Math.max(220, Math.min(420, Number(value) || 280));
}

async function handleMove({ categoryId, parentId, prevId, nextId }) {
  if (!props.canManage || ordering.value || !categoryId) {
    await refresh();
    return;
  }
  ordering.value = true;
  try {
    await moveRequirementCategory(props.wsId, categoryId, {
      parent_id: parentId,
      prev_id: prevId,
      next_id: nextId,
    });
    const destinationParentId = Number(parentId) || 0;
    if (
      destinationParentId > 0 &&
      !expandedKeys.value.includes(destinationParentId)
    ) {
      expandedKeys.value = [...expandedKeys.value, destinationParentId];
    }
    await refresh();
    emit("changed");
  } catch {
    await refresh();
  } finally {
    dragging.value = false;
    ordering.value = false;
  }
}

watch(
  () => props.wsId,
  () => {
    initializedWorkspaceId = "";
    categories.value = [];
    loaded.value = false;
    searchText.value = "";
    rootExpanded.value = true;
    refresh();
  },
  { immediate: true },
);

watch(
  () => props.selectedKey,
  (key) => {
    const malformed =
      key !== "all" && key !== "uncategorized" && !key.startsWith("category:");
    const missing =
      key.startsWith("category:") &&
      loaded.value &&
      !categories.value.some((item) => `category:${item.id}` === key);
    if (malformed || missing) {
      emit("select", "uncategorized");
    }
  },
);
</script>

<style scoped lang="less">
.requirement-category-sidebar {
  position: relative;
  width: v-bind(widthCss);
  min-width: v-bind(widthCss);
  min-height: 0;
  border-right: 1px solid var(--app-border);
  background: var(--app-surface);
  transition:
    width 0.2s ease,
    min-width 0.2s ease;
}
.requirement-category-sidebar.collapsed {
  width: 0;
  min-width: 0;
  border-right-color: transparent;
}
.requirement-category-sidebar.is-resizing {
  transition: none;
}
.category-sidebar-content {
  display: flex;
  width: v-bind(widthCss);
  height: 100%;
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
}
.sidebar-tabs {
  display: flex;
  gap: 16px;
  padding: 0 12px;
  border-bottom: 1px solid var(--app-border);

  button {
    padding: 10px 0;
    border: 0;
    border-bottom: 2px solid transparent;
    background: none;
    color: inherit;
    cursor: pointer;
  }

  button[aria-pressed="true"] {
    color: var(--ant-color-primary, #1677ff);
    border-bottom-color: currentColor;
  }
}
.sidebar-views {
  display: flex;
  min-height: 0;
  flex: 1;
  flex-direction: column;
}
.category-sidebar-resizer {
  position: absolute;
  z-index: 8;
  top: 0;
  right: -3px;
  width: 6px;
  height: 100%;
  cursor: col-resize;
}
.category-sidebar-resizer::after {
  position: absolute;
  top: 0;
  right: 2px;
  width: 2px;
  height: 100%;
  background: #1677ff;
  content: "";
  opacity: 0;
  transition: opacity 0.15s ease;
}
.category-sidebar-resizer:hover::after,
:global(body.category-sidebar-resizing) .category-sidebar-resizer::after {
  opacity: 1;
}
.category-parent-hint {
  margin-bottom: 12px;
}
.category-sidebar-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 14px 10px;
}
.category-search {
  flex: 1;
}
:deep(.category-search.ant-input-affix-wrapper) {
  height: 32px;
  border-color: var(--app-border);
  border-radius: 6px;
  background: var(--app-surface);
}
:deep(.category-search.ant-input-affix-wrapper:hover),
:deep(.category-search.ant-input-affix-wrapper-focused) {
  border-color: #1677ff;
}
:deep(.category-search .anticon) {
  color: var(--app-text-secondary);
}
.category-add-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  flex: 0 0 30px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  color: var(--app-text-secondary);
  background: transparent;
  font-size: 17px;
  cursor: pointer;
  transition:
    color 0.15s ease,
    background 0.15s ease;
}
.category-add-button:hover,
.category-add-button:focus-visible {
  color: #1677ff;
  background: color-mix(in srgb, #1677ff 10%, var(--app-surface));
  outline: none;
}
.category-add-button:disabled {
  cursor: wait;
  opacity: 0.45;
}
.category-tree-scroll {
  flex: 1;
  min-height: 0;
  padding: 2px 6px 20px;
  overflow: auto;
}
.category-virtual-row {
  display: flex;
  align-items: center;
  height: 36px;
  padding: 0 10px;
  border-radius: 4px;
  color: var(--app-text);
  cursor: pointer;
  user-select: none;
}
.category-virtual-row:hover {
  background: var(--app-surface-muted);
}
.category-virtual-row:focus-visible {
  outline: 2px solid #91caff;
  outline-offset: -2px;
}
.category-virtual-row.selected {
  color: #1677ff;
  background: #edf5ff;
}
.category-virtual-row.selected .category-virtual-name {
  font-weight: 500;
}
.category-root-switcher {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  flex: 0 0 18px;
  margin-right: 2px;
  padding: 0;
  border: 0;
  color: #172b4d;
  background: transparent;
  font-size: 9px;
  cursor: pointer;
}
.category-root-switcher:disabled {
  cursor: default;
}
.category-root-switcher:focus-visible {
  border-radius: 3px;
  outline: 2px solid #91caff;
}
.category-virtual-indent {
  width: 38px;
  flex: 0 0 38px;
}
.category-virtual-name {
  min-width: 0;
  overflow: hidden;
  font-size: 14px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.category-virtual-count {
  margin-left: 8px;
  color: #8796b2;
  font-size: 12px;
}
.category-virtual-row.selected .category-virtual-count {
  color: #8796b2;
}
.category-root-children {
  position: relative;
  margin-left: 0;
  padding-left: 0;
}
.category-uncategorized-row {
  padding-left: 10px;
}
.category-search-empty {
  padding: 22px 12px;
  color: #9aa3b2;
  text-align: center;
  font-size: 12px;
}
.category-collapse-handle {
  position: absolute;
  z-index: 12;
  top: 50%;
  right: -11px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 44px;
  padding: 0;
  border: 1px solid var(--app-border);
  border-left: 0;
  border-radius: 0 7px 7px 0;
  color: var(--app-text-secondary);
  background: var(--app-surface);
  box-shadow: 2px 0 5px rgb(31 45 61 / 7%);
  font-size: 10px;
  cursor: pointer;
  transform: translateY(-50%);
}
/* Keep the collapsed handle inside the page padding so it never covers the table checkbox column. */
.category-collapse-handle.collapsed {
  right: 0;
  border-left: 1px solid var(--app-border);
}
.category-collapse-handle:hover,
.category-collapse-handle:focus-visible {
  color: #1677ff;
  background: color-mix(in srgb, #1677ff 6%, var(--app-surface));
  outline: none;
}
.requirement-category-sidebar.is-ordering {
  cursor: progress;
}
</style>

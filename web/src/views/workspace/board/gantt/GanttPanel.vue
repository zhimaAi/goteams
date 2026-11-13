<!--
 * GanttPanel.vue
 * Gantt tab container: owns the view/filter state, loads the work items and
 * hosts the toolbar (teleported onto the board tab row), the filter bar, the
 * chart and the view/settings modals.
 -->
<template>
  <div class="gantt-panel">
    <!-- Toolbar teleported onto the board tab row (target rendered by BoardPage). -->
    <Teleport to="#gantt-top-actions">
      <ViewDropdown
        :views="ganttViews"
        :current-view-id="currentViewId"
        @select="handleViewSelect"
        @create="openCreateView"
        @config="viewConfigOpen = true"
      />
      <a-button
        type="text"
        class="gantt-search-toggle"
        :class="{
          'gantt-search-toggle--expanded': searchExpanded,
          'gantt-search-toggle--active': hasActiveFilterValues,
        }"
        @click="toggleSearchBar"
      >
        <FilterOutlined /> {{ $t("search_button") }}
      </a-button>
      <a-button
        type="text"
        class="gantt-settings-toggle"
        @click="settingsOpen = true"
      >
        <SettingOutlined /> {{ $t("workspace_settingsShort") }}
      </a-button>
    </Teleport>

    <GanttFilterBar
      v-if="searchExpanded"
      :filters="filters"
      :status-options="statusOptions"
      :priority-options="priorityOptions"
      :type-options="typeOptions"
      :members="viewFilterMembers"
      :has-active-values="hasActiveFilterValues"
      @update:keyword="filters.keyword = $event"
      @update:type="filters.type = $event"
      @keyword-change="onKeywordChange"
      @search="fetchItems"
      @apply="applyFilter"
      @clear="clearFilters"
    />

    <GanttChart
      :items="items"
      :fields="visibleListFields"
      :date-scale="dateScale"
      :ws-id="wsId"
      :can-edit-by-type="canEditByType"
      :can-create-by-type="canCreateByType"
      :can-delete-by-type="canDeleteByType"
      :can-copy-by-type="canCopyByType"
      @open-detail="openDetail"
      @update-dates="handleUpdateDates"
      @row-action="handleRowAction"
    />

    <ListSettingsModal
      v-model:open="settingsOpen"
      :user-id="authStore.userInfo?.id"
      :ws-id="wsId"
      type-key="gantt"
      :fields="ganttListFields"
      :current-preferences="listPreferences"
      hide-view-mode
      @change="applyListPreferences"
    >
      <template #extra-settings>
        <GanttDateScaleSetting :value="dateScale" @change="applyDateScale" />
      </template>
    </ListSettingsModal>

    <ViewConfigModal
      v-model:open="viewConfigOpen"
      :ws-id="wsId"
      type-key="gantt"
      :views="ganttViews"
      :remember-last="rememberLast"
      :can-manage-system-view="canManageSystemView"
      @edit="openEditView"
      @save-as="openSaveAsView"
      @deleted="handleViewDeleted"
      @saved="reloadViews"
    />
    <ViewEditModal
      v-model:open="viewEditOpen"
      :ws-id="wsId"
      type-key="gantt"
      :mode="viewEditMode"
      :view="viewEditTarget"
      :can-manage-system-view="canManageSystemView"
      :members="viewFilterMembers"
      :versions="versionOptions"
      :gantt-sections="ganttSections"
      :show-defect-section="defectAppEnabled"
      @saved="handleViewSaved"
    />

    <RequirementDetailDrawer
      :open="!!detailId"
      :ws-id="wsId"
      :requirement-id="detailId"
      :mode="previewMode"
      :navigation-items="ganttNavigationItems"
      :can-edit-by-type="canEditByType"
      :can-create-by-type="canCreateByType"
      @close="detailId = null"
      @navigate="openDetail"
      @updated="fetchItems"
    />
  </div>
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
import { useI18n } from "vue-i18n";
import { FilterOutlined, SettingOutlined } from "@ant-design/icons-vue";
import { getWorkspaceMetadata, listGanttRequirements } from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";
import { useWorkItemViews } from "@/composables/useWorkItemViews";
import { GANTT_DATE_SCALES } from "@/constants/gantt";
import RequirementDetailDrawer from "@/components/common/RequirementDetailDrawer.vue";
import GanttDateScaleSetting from "./GanttDateScaleSetting.vue";
import GanttFilterBar from "./GanttFilterBar.vue";
import { useGanttListSettings } from "./useGanttListSettings";
import { useGanttRowActions } from "./useGanttRowActions";
import { useGanttViewSections } from "@/composables/useGanttViewSections";
import { useWorkspaceApplications } from "@/composables/useWorkspaceApplications";
import ListSettingsModal from "@/components/work-item/ListSettingsModal.vue";
import ViewDropdown from "@/components/work-item/ViewDropdown.vue";
import ViewConfigModal from "@/components/work-item/ViewConfigModal.vue";
import ViewEditModal from "@/components/work-item/ViewEditModal.vue";
import GanttChart from "./GanttChart.vue";

const props = defineProps({
  wsId: { type: [String, Number], required: true },
});

const { t } = useI18n();
const authStore = useAuthStore();

// ======================== Permissions ========================
const canEditByType = computed(() => ({
  requirement: authStore.hasPermission("requirement:edit"),
  defect: authStore.hasPermission("defect:edit"),
}));
const canCreateByType = computed(() => ({
  requirement: authStore.hasPermission("requirement:create"),
  defect: authStore.hasPermission("defect:create"),
}));
const canDeleteByType = computed(() => ({
  requirement: authStore.hasPermission("requirement:delete"),
  defect: authStore.hasPermission("defect:delete"),
}));
const canCopyByType = computed(() => ({
  requirement: authStore.hasPermission("requirement:copy"),
  defect: authStore.hasPermission("defect:copy"),
}));
const canManageSystemView = computed(() =>
  authStore.hasPermission("gantt:manage_system_view"),
);

// ======================== Metadata ========================
const metadata = ref(null);
const userIdRef = computed(() => authStore.userInfo?.id);
const {
  previewMode,
  listPreferences,
  ganttListFields,
  visibleListFields,
  applyListPreferences,
} = useGanttListSettings(userIdRef, metadata, t);

// Raw member rows for the view editor (ViewFilterGroup reads user_id/user_name).
const viewFilterMembers = computed(() =>
  (metadata.value?.members || []).filter((member) => !member.removed),
);
const versionOptions = computed(() => metadata.value?.versions || []);

// Merge requirement/defect options that share a display name so one choice
// filters both types (ids of both types are sent together).
function mergedOptions(reqList, defectList, labelKey) {
  const byLabel = new Map();
  for (const item of [...(reqList || []), ...(defectList || [])]) {
    const label = item[labelKey];
    if (!byLabel.has(label)) byLabel.set(label, []);
    byLabel.get(label).push(Number(item.id));
  }
  return [...byLabel.entries()].map(([label, ids]) => ({
    label,
    value: ids.join(","),
  }));
}
const statusOptions = computed(() =>
  mergedOptions(
    metadata.value?.requirement?.statuses,
    metadata.value?.defect?.statuses,
    "name",
  ),
);
const priorityOptions = computed(() =>
  mergedOptions(
    metadata.value?.requirement?.priorities,
    metadata.value?.defect?.priorities,
    "label",
  ),
);
// The defect type follows the defect app switch: with the app off there is no defect
// entry to reach, so the filter must not offer it either.
const { defectAppEnabled } = useWorkspaceApplications(
  computed(() => props.wsId),
);
const typeOptions = computed(() => [
  { value: "", label: t("common_all") },
  { value: "requirement", label: t("requirement_feature") },
  ...(defectAppEnabled.value
    ? [{ value: "defect", label: t("requirement_bug") }]
    : []),
]);

// ======================== Views ========================
const wsIdRef = computed(() => props.wsId);
const {
  rememberByType,
  currentViewIdByType,
  searchExpandedByType,
  viewsOf,
  loadViews,
  ensureViewsLoaded,
  selectView,
  saveSearchSettings,
} = useWorkItemViews(wsIdRef);

const ganttViews = computed(() => viewsOf("gantt"));
const currentViewId = computed(() => currentViewIdByType.value.gantt ?? null);
const rememberLast = computed(() => rememberByType.value.gantt !== false);
const searchExpanded = computed(() => searchExpandedByType.value.gantt);

const viewEditOpen = ref(false);
const viewEditMode = ref("create");
const viewEditTarget = ref(null);
const viewConfigOpen = ref(false);

function handleViewSelect(viewId) {
  selectView("gantt", viewId);
  fetchItems();
}
function openCreateView() {
  viewEditMode.value = "create";
  viewEditTarget.value = null;
  viewEditOpen.value = true;
}
function openEditView(view) {
  viewEditMode.value = "edit";
  viewEditTarget.value = view;
  viewEditOpen.value = true;
}
function openSaveAsView(view) {
  viewEditMode.value = "saveAs";
  viewEditTarget.value = view;
  viewEditOpen.value = true;
}
async function reloadViews() {
  await loadViews("gantt");
  fetchItems();
}
async function handleViewSaved(payload) {
  await loadViews("gantt");
  if (payload?.id && payload.mode !== "edit") {
    selectView("gantt", String(payload.id));
  }
  fetchItems();
}
async function handleViewDeleted(viewId) {
  await loadViews("gantt");
  if (String(currentViewId.value) === String(viewId)) {
    const fallback = ganttViews.value.find((view) => view.enabled);
    if (fallback) selectView("gantt", fallback.id);
  }
  fetchItems();
}

const ganttSections = useGanttViewSections(metadata, t);

// ======================== Filters & data ========================
const filters = reactive({
  keyword: "",
  statusKeys: [],
  priorityKeys: [],
  assigneeIds: [],
  type: "",
});
const items = ref([]);
const settingsOpen = ref(false);
const {
  detailId,
  navigationItems: ganttNavigationItems,
  openDetail,
  updateDates: handleUpdateDates,
  handleRowAction,
} = useGanttRowActions({
  wsId: wsIdRef,
  items,
  previewMode,
  t,
  refresh: fetchItems,
});

const hasActiveFilterValues = computed(
  () =>
    !!filters.keyword.trim() ||
    filters.statusKeys.length > 0 ||
    filters.priorityKeys.length > 0 ||
    filters.assigneeIds.length > 0 ||
    !!filters.type,
);

// A remembered "defect" filter must not stay selected once the app is off: the option is
// gone from the dropdown, so the user could neither see nor undo the condition. An
// undefined previous value marks the immediate first run, before any fetch exists.
watch(
  [defectAppEnabled, () => filters.type],
  ([enabled, type], previous) => {
    if (enabled || type !== "defect") return;
    filters.type = "";
    if (previous) fetchItems();
  },
  { immediate: true },
);

function expandKeys(keys) {
  return keys.flatMap((key) => String(key).split(",")).join(",");
}

// Page size is capped at 100 server-side; the gantt needs every matching row,
// so it pages through with a hard ceiling as a runaway guard.
const PAGE_SIZE = 100;
const MAX_PAGES = 50;
// Debounce for search-as-you-type in the keyword box.
const SEARCH_DEBOUNCE_MS = 300;

// Monotonic fetch id: a stale paged response must not overwrite a newer one.
let fetchSeq = 0;

async function fetchItems() {
  const seq = ++fetchSeq;
  const params = {
    size: PAGE_SIZE,
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
  };
  // Guard the query too: a hidden type must never narrow the chart to an empty result.
  if (filters.type && (filters.type !== "defect" || defectAppEnabled.value))
    params.type = filters.type;
  const keyword = filters.keyword.trim();
  if (keyword) params.search = keyword;
  if (filters.statusKeys.length)
    params.status_ids = expandKeys(filters.statusKeys);
  if (filters.priorityKeys.length)
    params.priority_ids = expandKeys(filters.priorityKeys);
  if (filters.assigneeIds.length)
    params.assignee_ids = filters.assigneeIds.join(",");
  if (currentViewId.value) params.view_id = currentViewId.value;
  try {
    // Page through the shared list API (its page size is capped at 100 server-side)
    // so the gantt renders every matching work item.
    const all = [];
    for (let page = 1; page <= MAX_PAGES; page++) {
      const res = await listGanttRequirements(props.wsId, { ...params, page });
      const list = res?.list || [];
      all.push(...list);
      const total = Number(res?.total || 0);
      if (list.length < params.size || all.length >= total) break;
    }
    if (seq === fetchSeq) items.value = all;
  } catch {
    /* handled by interceptor */
  }
}

let keywordTimer = null;
function onKeywordChange() {
  // Debounced search-as-you-type; clearing via the clear icon lands here too.
  clearTimeout(keywordTimer);
  keywordTimer = setTimeout(fetchItems, SEARCH_DEBOUNCE_MS);
}
onBeforeUnmount(() => clearTimeout(keywordTimer));

function clearFilters() {
  filters.keyword = "";
  filters.statusKeys = [];
  filters.priorityKeys = [];
  filters.assigneeIds = [];
  filters.type = "";
  fetchItems();
}

// Filter controls emit their whole value set; store it and refetch.
function applyFilter(key, value) {
  filters[key] = value || [];
  fetchItems();
}

function toggleSearchBar() {
  saveSearchSettings("gantt", { expanded: !searchExpanded.value });
}

// ======================== Date scale ========================
function scaleStorageKey() {
  return `goteams-gantt-scale-${props.wsId}`;
}
const dateScale = ref("week");

function applyDateScale(value) {
  if (!GANTT_DATE_SCALES.includes(value)) return;
  dateScale.value = value;
  try {
    localStorage.setItem(scaleStorageKey(), value);
  } catch {
    /* storage failures only lose the preference */
  }
}

onMounted(async () => {
  try {
    const storedScale = localStorage.getItem(scaleStorageKey());
    dateScale.value = GANTT_DATE_SCALES.includes(storedScale)
      ? storedScale
      : "week";
  } catch {
    dateScale.value = "week";
  }
  const [meta] = await Promise.all([
    getWorkspaceMetadata(props.wsId),
    ensureViewsLoaded("gantt"),
  ]);
  metadata.value = meta;
  fetchItems();
});
</script>

<style scoped lang="less">
.gantt-panel {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  padding-top: 8px;
}
// Teleported toolbar buttons live under BoardPage's .board-top-actions container;
// their own styles stay here because the teleport keeps this component's scope id.
.gantt-search-toggle,
.gantt-settings-toggle {
  display: inline-flex;
  align-items: center;
  flex-shrink: 0;
}
.gantt-search-toggle {
  &--expanded,
  &--active {
    color: #1677ff !important;
    background: #f0f6ff !important;
  }
}
</style>

import {
  h,
  inject,
  ref,
  computed,
  reactive,
  watch,
  onMounted,
  onBeforeUnmount,
  nextTick,
} from "vue";
import dayjs from "dayjs";
import { getSLASummaries } from "@/api/sla";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { Button, Checkbox, Empty, message } from "ant-design-vue";
import { MoreOutlined } from "@ant-design/icons-vue";
import WorkItemRowActionMenu from "@/components/common/WorkItemRowActionMenu.vue";

import { listIssueInviteTemplates } from "@/api/issueInvite";
import {
  assignRequirementCategory,
  listRequirements,
  listWorkItemTemplates,
  getWorkspaceMetadata,
  deleteRequirement,
  batchDeleteRequirements,
  getRequirement,
  updateRequirement,
  batchUpdateRequirements,
  exportWorkItems,
} from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";
import { useLocaleStore } from "@/stores/locale";
import { useContactPreferencesStore } from "@/stores/contactPreferences";
import { setSecondaryKey } from "@/composables/usePageTitle";
import { useWorkItemClipboardActions } from "@/composables/useWorkItemClipboardActions";
import { useWorkItemTypeAttributes } from "@/composables/useWorkItemTypeAttributes";
import { syncWorkspaceApplications } from "@/composables/useWorkspaceApplications";
import { buildCategoryOptions } from "@/utils/requirementCategory";
import { formatLocalDate } from "@/utils/date";
import { showWorkItemUpdateSuccess } from "@/utils/workItemActionToast";
import {
  customFieldValues,
  flattenCascaderNodes,
  isValidPhoneFieldValue,
  parseCustomFieldValue,
  serializeCustomFieldValue,
  storedCustomFieldValue,
  customFieldSearchControl,
  filterMembersForField,
  normalizeCascaderFilterValues,
} from "@/utils/customField";
import {
  CUSTOM_FIELD_TYPE_LABELS,
  NUMERIC_SEARCH_OPERATORS,
} from "@/constants/customField";
import { isStatusTransitionEnabled } from "@/utils/statusTransition";
import { sortFieldsByOrder } from "@/utils/fieldOrder";
import {
  useWorkItemViews,
  hasActiveFilters,
} from "@/composables/useWorkItemViews";
import {
  VIEW_FILTER_FIELD_OPERATORS,
  VIEW_VALUE_ALL_COMPLETED,
  VIEW_VALUE_ALL_CLOSED,
  VIEW_VALUE_ALL_END,
  VIEW_VALUE_ALL_NOT_END,
  hasSearchFilterValues,
  newSearchFilterCondition,
  normalizeSearchFilterTree,
} from "@/constants/workItemView";
import {
  mergeWorkItem,
  patchLoadedWorkItem,
  workItemMatchesFilters,
} from "@/utils/workItemList";
import { saveWorkItemNavigationContext } from "@/utils/workItemNavigation";
import { getRememberedTab, setRememberedTab } from "@/utils/rememberedTab";
import {
  readGlobalListPreferences,
  GLOBAL_LIST_PREFERENCES_CHANGE_EVENT,
  readScopedListPreferences,
  writeScopedListPreferences,
  normalizeBoardSort,
  readBoardColors,
} from "@/utils/boardPreferences";
import defaultSortIcon from "@/image/order/排序.svg";
import ascendingSortIcon from "@/image/order/升序.svg";
import descendingSortIcon from "@/image/order/降序.svg";
import { workItemRoute } from "@/utils/workItemRoutes";
import { usesIssuePages } from "@/utils/workspaceType";
import { buildWorkItemPageUrl } from "@/utils/workItemClipboard";

export function useWorkItemBoard(options = {}) {
  const { t } = useI18n();
  const route = useRoute();
  const router = useRouter();
  const surface = route.meta.workItemSurface;
  const issueMetadata = inject("issueWorkspaceMetadata", null);
  const authStore = useAuthStore();
  const localeStore = useLocaleStore();
  const contactPreferences = useContactPreferencesStore();
  const wsId = ref(route.params.wsId);
  const workItemAttributes = useWorkItemTypeAttributes(wsId);
  const { copyWorkItemInfo, copyWorkItemsInfo } = useWorkItemClipboardActions(
    wsId,
    t,
    (id) => buildWorkItemPageUrl(wsId.value, id, surface),
  );
  const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;

  function categoryKeyFromQuery(value) {
    if (value === undefined || value === null || value === "") return "all";
    if (String(value) === "uncategorized" || String(value) === "0") {
      return "uncategorized";
    }
    const id = Number(value);
    return Number.isInteger(id) && id > 0 ? `category:${id}` : "invalid";
  }

  function categoryQueryFromKey(key) {
    if (key === "uncategorized") return "0";
    if (key.startsWith("category:")) return key.slice("category:".length);
    return undefined;
  }

  // Category sidebar collapse state is remembered per user + workspace, collapsed by default.
  function readStoredCategorySidebarCollapsed() {
    const stored = readScopedListPreferences(
      authStore.userInfo?.id,
      wsId.value,
      "requirement",
    ).categorySidebarCollapsed;
    return stored !== false;
  }

  function readStoredCategorySidebarWidth() {
    const width = Number(
      readScopedListPreferences(
        authStore.userInfo?.id,
        wsId.value,
        "requirement",
      ).categorySidebarWidth,
    );
    return Number.isFinite(width) ? Math.max(220, Math.min(420, width)) : 280;
  }

  const selectedCategoryKey = ref(categoryKeyFromQuery(route.query.categoryId));
  const categorySidebarCollapsed = ref(readStoredCategorySidebarCollapsed());
  const categorySidebarWidth = ref(readStoredCategorySidebarWidth());
  const requirementCategories = ref([]);
  const categorySidebarRef = ref(null);

  const categoryEditTarget = ref(null);
  const categoryDraft = ref(0);
  const categorySaving = ref(false);
  const categoryOptions = computed(() =>
    buildCategoryOptions(
      requirementCategories.value,
      t("requirement_categoryUncategorized"),
    ),
  );

  function canAssignCategory(record) {
    return (
      record.type === "requirement" &&
      (Number(record.parent_id) || 0) === 0 &&
      fieldEnabled(record.type, "category") &&
      fieldEditable(record.type, "category")
    );
  }

  function openCategoryEditor(record) {
    if (categorySaving.value || !canAssignCategory(record)) return;
    categoryDraft.value = Number(record.category_id) || 0;
    categoryEditTarget.value = {
      id: record.id,
      wsId: wsId.value,
      original: categoryDraft.value,
    };
  }

  function closeCategoryEditor() {
    if (!categorySaving.value) categoryEditTarget.value = null;
  }

  async function refreshAfterCategoryAssignment() {
    if (!isWorkItemTab.value) return;
    const workspaceId = wsId.value;
    const tab = activeTab.value;
    if (listPreferences.viewMode === "board") {
      kanbanRef.value?.reload();
      await refreshRequirementCategories();
      return;
    }
    await Promise.all([fetchRequirements(), refreshRequirementCategories()]);
    if (wsId.value !== workspaceId || activeTab.value !== tab) return;
    const lastPage = Math.max(1, Math.ceil(total.value / pageSize.value));
    if (page.value > lastPage) {
      page.value = lastPage;
      await fetchRequirements();
    }
  }

  async function confirmCategoryAssignment() {
    const target = categoryEditTarget.value;
    if (!target || categorySaving.value || target.wsId !== wsId.value) return;
    if (categoryDraft.value === target.original) {
      closeCategoryEditor();
      return;
    }
    categorySaving.value = true;
    try {
      await assignRequirementCategory(
        target.wsId,
        target.id,
        categoryDraft.value,
      );
      if (categoryEditTarget.value === target) {
        categoryEditTarget.value = null;
        await refreshAfterCategoryAssignment();
        showWorkItemUpdateSuccess(t("workspace_updateSuccess"));
      }
    } catch {
      // The request interceptor shows the failure. Keep the selected category for retry.
    } finally {
      categorySaving.value = false;
    }
  }

  function requirementCategoryName(categoryId) {
    const id = Number(categoryId);
    if (!id) return t("requirement_categoryUncategorized");
    return (
      requirementCategories.value.find((item) => Number(item.id) === id)
        ?.name || "-"
    );
  }

  // Status.
  // Enabled app keys from the app market (from workspace metadata.enabled_applications).
  // Declared before boardTabs because availableBoardTabs runs during setup.
  const enabledApplications = ref(
    issueMetadata?.value?.enabled_applications || [],
  );
  const boardTabs = [
    { key: "planning", permission: "planning:view", app: "planning" },
    { key: "requirement", permission: "requirement:view" },
    { key: "defect", permission: "defect:view", app: "defect" },
    { key: "iteration", permission: "version:view", app: "iteration" },
    { key: "gantt", permission: "gantt:view", app: "gantt" },
    { key: "dashboard", permission: "dashboard:view", app: "dashboard" },
    { key: "knowledge", permission: "kb:view", app: "knowledge_base" },
    { key: "issue-invite", permission: "ticket:copy_link", app: "issue" },
  ];

  // The knowledge base tab additionally requires the workspace application to be enabled.
  const canViewKnowledge = computed(
    () =>
      enabledApplications.value.includes("knowledge_base") &&
      (authStore.hasPermission("kb:view") ||
        authStore.hasPermission("kb:manage")),
  );
  const canViewIssueInvite = computed(() =>
    enabledApplications.value.includes("issue"),
  );
  const canViewDashboard = computed(
    () =>
      enabledApplications.value.includes("dashboard") &&
      authStore.hasPermission("dashboard:view"),
  );

  function canAccessBoardTab(tab) {
    if (tab.key === "dashboard") return canViewDashboard.value;
    if (tab.key === "knowledge") {
      return (
        authStore.hasPermission("kb:view") ||
        authStore.hasPermission("kb:manage")
      );
    }
    if (tab.key === "issue-invite") {
      return true;
    }
    return authStore.hasPermission(tab.permission);
  }

  function availableBoardTabs() {
    return boardTabs
      .filter(canAccessBoardTab)
      .filter((tab) => !tab.app || enabledApplications.value.includes(tab.app))
      .map((tab) => tab.key);
  }

  function requestedBoardTab() {
    if (options.activeTab) return options.activeTab;
    if (surface === "issue") {
      if (route.meta.boardTab) return route.meta.boardTab;
      if (route.name === "IssueDashboard") return "dashboard";
      if (route.name === "IssueInviteManage") return "issue-invite";
    }
    return typeof route.query.tab === "string" ? route.query.tab : "";
  }

  function boardTabStorageScope() {
    return `project:${wsId.value}`;
  }

  function resolveBoardTab() {
    const availableTabs = availableBoardTabs().filter(
      (key) =>
        surface !== "issue" ||
        route.name !== "IssueList" ||
        ["requirement", "defect"].includes(key),
    );
    const requestedTab = requestedBoardTab();
    if (availableTabs.includes(requestedTab)) {
      setRememberedTab(
        authStore.userInfo?.id,
        boardTabStorageScope(),
        requestedTab,
      );
      return requestedTab;
    }
    return getRememberedTab(
      authStore.userInfo?.id,
      boardTabStorageScope(),
      availableTabs,
      availableTabs.includes("requirement") ? "requirement" : "",
    );
  }

  const activeTab = ref(resolveBoardTab());
  // The board tab's secondary-menu title (i18n key).
  const boardTabTitle = {
    planning: "planning_tabTitle",
    requirement: "requirement_feature",
    defect: "requirement_bug",
    iteration: "requirement_filterVersion",
    gantt: "gantt_tab",
    dashboard: "app_dashboard",
    knowledge: "app_knowledge_base",
    "issue-invite": "app_issue_invite",
  };
  watch(
    activeTab,
    (k) =>
      setSecondaryKey(
        surface === "issue" && k === "issue-invite"
          ? ""
          : boardTabTitle[k] || "",
      ),
    {
      immediate: true,
    },
  );
  const loading = ref(false);
  const boardContextReady = ref(false);
  const searchText = ref("");
  const appliedSearchText = ref("");
  const appliedSearchFiltersByType = ref({});
  const filterStatusActive = ref(false);
  const filterStatusIds = ref([]);
  const filterAssignee = ref([]);
  const filterVersionIds = ref([]);
  const filterCreatorIds = ref([]);
  const filterPriorityIds = ref([]);
  const page = ref(1);
  const pageSize = ref(40);
  const total = ref(0);
  const selectedRowKeys = ref([]);
  const editingCell = ref(null);
  const expandedRowKeys = ref([]);
  const allTreeExpanded = ref(false);
  const childrenMap = ref({});
  const childLoadingKeys = ref(new Set());
  const sharedDetailId =
    route.query.detail && route.query.tab === undefined
      ? String(route.query.detail)
      : "";
  const detailReqId = ref(sharedDetailId ? null : route.query.detail || null);
  const workLogEditorToken = ref(0);
  const workLogEditorRequirementId = ref("");
  const directWorkLogRequirementId = ref("");
  const directWorkLogsRef = ref(null);
  const showImportModal = ref(false);
  const batchAssigneeOpen = ref(false);
  const batchAssigneeIds = ref([]);
  const batchAssigneeSaving = ref(false);
  const deleteConfirmOpen = ref(false);
  const deleteTarget = ref(null);
  const deletingTarget = ref(false);
  const listSettingsOpen = ref(false);
  const showListSettingsNotice = ref(false);
  const storedGlobalListPreferences = readGlobalListPreferences(
    authStore.userInfo?.id,
  );
  const listPreferences = reactive({
    previewMode: ["drawer", "modal", "window"].includes(
      storedGlobalListPreferences.previewMode,
    )
      ? storedGlobalListPreferences.previewMode
      : "drawer",
    viewMode: ["list", "board"].includes(storedGlobalListPreferences.viewMode)
      ? storedGlobalListPreferences.viewMode
      : "list",
    fields: [],
    boardStatuses: [],
    boardSort: { field: "updated_at", direction: "asc" },
    boardColors: {},
  });
  const LIST_SORT_FIELDS = new Set([
    "id",
    "status",
    "priority",
    "version",
    "planned_start_date",
    "planned_end_date",
    "assignee",
    "created_by",
    "created_at",
    "estimated_work_hours",
    "completed_work_hours",
    "remaining_work_hours",
    "overtime_work_hours",
    "work_progress",
  ]);
  const listTablePreferences = reactive({
    requirement: { sort: { field: "", direction: "" }, frozenThrough: "" },
    defect: { sort: { field: "", direction: "" }, frozenThrough: "" },
  });
  const columnDragState = reactive({
    sourceKey: "",
    targetKey: "",
    side: "",
  });
  // Tracks which column's "..." dropdown is open so the actions container stays
  // expanded during the overlay's close animation (otherwise the trigger collapses
  // and vc-trigger repositions the overlay, causing a visible "flash").
  const openColumnMenuKey = ref("");
  let closeColumnMenuTimer = null;
  let columnDragPreview = null;
  const currentListSort = computed(
    () =>
      listTablePreferences[activeTab.value]?.sort || {
        field: "",
        direction: "",
      },
  );
  const kanbanRef = ref(null);
  const kanbanNavigationItems = ref([]);

  function listSettingsNoticeKey() {
    const userId = authStore.userInfo?.id;
    return userId ? `goteams:list-settings-seen:${userId}` : "";
  }

  function loadListSettingsNotice() {
    const key = listSettingsNoticeKey();
    if (!key) {
      showListSettingsNotice.value = false;
      return;
    }
    try {
      showListSettingsNotice.value = window.localStorage.getItem(key) !== "1";
    } catch {
      showListSettingsNotice.value = true;
    }
  }

  function openListSettings() {
    showListSettingsNotice.value = false;
    const key = listSettingsNoticeKey();
    if (key) {
      try {
        window.localStorage.setItem(key, "1");
      } catch {
        // Browser storage failure should not block opening list settings.
      }
    }
    listSettingsOpen.value = true;
  }

  watch(() => authStore.userInfo?.id, loadListSettingsNotice, {
    immediate: true,
  });
  // Work item view state (kept per tab by the composable).
  const {
    rememberByType: viewRememberByType,
    currentViewIdByType,
    searchExpandedByType,
    searchFiltersByType,
    viewsOf,
    findView,
    loadViews,
    ensureViewsLoaded,
    selectView,
    saveSearchSettings,
    resetForWorkspace: resetViewsForWorkspace,
  } = useWorkItemViews(wsId);
  const viewEditOpen = ref(false);
  const viewEditMode = ref("create"); // create | edit | saveAs
  const viewEditTarget = ref(null);
  const viewConfigOpen = ref(false);
  const deleteConfirmTitle = computed(() => t("workspace_confirmDelete"));
  const deleteConfirmContent = computed(() =>
    deleteTarget.value?.kind === "batch"
      ? t("workspace_confirmBatchDeleteContent", {
          n: deleteTarget.value.ids.length,
        })
      : deleteTarget.value
        ? t("workspace_confirmDeleteContent", {
            title: deleteTarget.value.record.title,
          })
        : "",
  );
  const deleteConfirmOkText = computed(() => t("common_delete"));
  let listRequestSequence = 0;
  let listContextSequence = 0;
  let childLoadSequence = 0;
  let workspaceLoadSequence = 0;
  const childLoadTokens = new Map();
  const childLoadPromises = new Map();

  const columnWidths = reactive({ requirement: {}, defect: {} });
  const selectionColumnWidth = 48;
  const versionTitleWidth = ref(null);
  const boardPageRef = ref(null);
  const tableAreaWidth = ref(0);
  const tableResizeGuide = reactive({
    visible: false,
    left: 0,
    top: 0,
    height: 0,
    headerHeight: 0,
  });
  let tableResizeObserver;
  let tableResizeGuideCleanup;
  let detailRowScrollFrame;

  // Data.
  const requirements = ref([]);
  const members = ref([]);
  const customFieldMembers = ref([]);
  // Selectors exclude disabled and removed members. Filters keep the full `members` list so
  // historical items can still be filtered by a former or disabled colleague.
  const assignableMembers = computed(() =>
    members.value.filter((m) => String(m.user_status) !== "0" && !m.removed),
  );
  const selectableMembers = computed(() => assignableMembers.value);
  const versions = ref([]);
  const statusesByType = ref({ requirement: [], defect: [] });
  const templatesByType = ref({
    requirement: [],
    defect: [],
  });
  let templateRequestSequence = 0;
  const prioritiesByType = ref({ requirement: [], defect: [] });
  const finalStatusIds = ref({ requirement: [], defect: [] });
  const completedStatusIds = ref({ requirement: [], defect: [] });
  const fieldConfigsByType = ref({ requirement: [], defect: [] });
  const customFieldsByType = ref({ requirement: [], defect: [] });
  const workspaceMetadata = ref(issueMetadata?.value || null);
  const transitionOpen = ref(false);
  const transitionItem = ref(null);

  const selectedRequirementCategory = computed(() => {
    if (!selectedCategoryKey.value.startsWith("category:")) return null;
    const id = selectedCategoryKey.value.slice("category:".length);
    return requirementCategories.value.find(
      (item) => String(item.id) === String(id),
    );
  });
  const requirementCategoryChipTitle = computed(() =>
    selectedRequirementCategory.value
      ? t("requirement_categoryChip", {
          name: selectedRequirementCategory.value.name,
        })
      : "",
  );
  const requirementCategoryChipLabel = computed(() => {
    const name = selectedRequirementCategory.value?.name || "";
    const characters = [...name];
    const shortName =
      characters.length > 5 ? `${characters.slice(0, 5).join("")}…` : name;
    return t("requirement_categoryChip", { name: shortName });
  });

  const workspaceName = ref(issueMetadata?.value?.workspace?.name || "");
  const projectEnded = ref(
    Number(issueMetadata?.value?.workspace?.ended_at || 0) > 0,
  );
  // Mirrors the sidebar tile: uploaded icon when set, otherwise the initial letter
  // on the project's own colour.
  const projectIcon = ref(issueMetadata?.value?.workspace?.icon || "");
  const projectColor = ref(issueMetadata?.value?.workspace?.color || "#0d68ff");
  const projectChar = computed(() => (workspaceName.value || "?").trim()[0]);
  const iconFgColor = "#fff";

  const canViewRequirement = computed(() =>
    authStore.hasPermission("requirement:view"),
  );
  const canViewDefect = computed(() => authStore.hasPermission("defect:view"));
  const canViewProjectSettings = computed(() =>
    [
      "project_info:edit",
      "project_member:add",
      "project_member:remove",
      "project_member:change_role",
      "project_role:add",
      "project_role:edit",
      "project_role:delete",
      "requirement_setting:field",
      "requirement_setting:status",
      "requirement_setting:priority",
      "defect_setting:field",
      "defect_setting:status",
      "defect_setting:priority",
      "automation:view",
      "automation:manage",
      "application_market:view",
      "application_market:manage",
      "planning:edit",
      "workspace:end",
    ].some((permission) => authStore.hasPermission(permission)),
  );
  const canViewVersion = computed(
    () =>
      enabledApplications.value.includes("iteration") &&
      authStore.hasPermission("version:view"),
  );
  const canViewGantt = computed(
    () =>
      enabledApplications.value.includes("gantt") &&
      authStore.hasPermission("gantt:view"),
  );
  const canViewPlanning = computed(
    () =>
      enabledApplications.value.includes("planning") &&
      authStore.hasPermission("planning:view"),
  );
  const canManageRequirementCategories = computed(() =>
    authStore.hasPermission("requirement:manage_category"),
  );
  // Only requirements/defects use the requirements list API; iterations have a dedicated view,
  // and planning fetches its own data via PlanningPanel, must not be sent as a work-item type to the backend.
  const isWorkItemTab = computed(() =>
    ["requirement", "defect"].includes(activeTab.value),
  );
  const visibleTopTabs = computed(availableBoardTabs);
  const canCreateActiveWorkItem = computed(() =>
    canCreateType(activeTab.value),
  );
  watch([wsId, activeTab], () => {
    categoryEditTarget.value = null;
  });

  const canEditActiveWorkItem = computed(() => canEditType(activeTab.value));
  const canDeleteActiveWorkItem = computed(() =>
    hasTypePermission(activeTab.value, "delete"),
  );
  const canCopyActiveWorkItem = computed(() =>
    hasTypePermission(activeTab.value, "copy"),
  );
  const canImportActiveWorkItem = computed(() =>
    hasTypePermission(activeTab.value, "import"),
  );
  const canExportActiveWorkItem = computed(() =>
    hasTypePermission(activeTab.value, "export"),
  );
  const canUseBatchActions = computed(
    () =>
      canEditActiveWorkItem.value ||
      canExportActiveWorkItem.value ||
      canDeleteActiveWorkItem.value ||
      canCopyActiveWorkItem.value,
  );
  const statusOptions = computed(() => statusesForType(activeTab.value));
  const priorityOptions = computed(() =>
    priorityOptionsForType(activeTab.value),
  );
  const currentTabViews = computed(() => viewsOf(activeTab.value));
  const currentViewId = computed(
    () => currentViewIdByType.value[activeTab.value] ?? null,
  );
  const currentTabRememberLast = computed(
    () => viewRememberByType.value[activeTab.value] !== false,
  );
  const currentSearchExpanded = computed(
    () => searchExpandedByType.value[activeTab.value] === true,
  );
  const currentSearchFilters = computed(
    () =>
      searchFiltersByType.value[activeTab.value] ||
      normalizeSearchFilterTree(null, activeTab.value),
  );
  const currentAppliedSearchFilters = computed(
    () =>
      appliedSearchFiltersByType.value[activeTab.value] ||
      normalizeSearchFilterTree(null, activeTab.value),
  );
  const currentSearchHasValues = computed(() =>
    hasSearchFilterValues(currentSearchFilters.value),
  );
  const searchFieldDescriptors = computed(() =>
    searchFieldsForType(activeTab.value),
  );
  const canManageSystemView = computed(() =>
    authStore.hasPermission(
      `${activeTab.value === "defect" ? "defect" : "requirement"}:manage_system_view`,
    ),
  );
  // Whether the current view carries real filter conditions (drives flat list mode for requirements).
  const viewHasActiveFilters = computed(() =>
    hasActiveFilters(findView(activeTab.value, currentViewId.value)?.filters),
  );
  const kanbanFilters = computed(() => ({
    search: appliedSearchText.value,
    statusActive: filterStatusActive.value,
    statusIds: filterStatusIds.value,
    assigneeIds: filterAssignee.value,
    versionIds: filterVersionIds.value,
    createdByIds: filterCreatorIds.value,
    priorityIds: filterPriorityIds.value,
    searchFilters: JSON.stringify(currentAppliedSearchFilters.value),
    searchActive: hasSearchFilterValues(currentAppliedSearchFilters.value),
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
    viewId: currentViewId.value,
    viewHasActiveFilters: viewHasActiveFilters.value,
    // Requirement tab only: keep the kanban in sync with the category sidebar (0 = uncategorized).
    categoryId:
      activeTab.value === "requirement" ? categoryFilterParam() : undefined,
  }));
  const kanbanEditableFields = computed(() => ({
    priority: fieldEnabled(activeTab.value, "priority"),
    assignee: fieldEnabled(activeTab.value, "assignee"),
    plannedStartDate: fieldEnabled(activeTab.value, "planned_start_date"),
    plannedEndDate: fieldEnabled(activeTab.value, "planned_end_date"),
  }));
  const kanbanFieldEditability = computed(() => ({
    priority: fieldEditable(activeTab.value, "priority"),
    assignee: fieldEditable(activeTab.value, "assignee"),
    plannedStartDate: fieldEditable(activeTab.value, "planned_start_date"),
    plannedEndDate: fieldEditable(activeTab.value, "planned_end_date"),
  }));

  function statusesForType(type) {
    return statusesByType.value[type] || [];
  }

  function priorityOptionsForType(type) {
    return prioritiesByType.value[type] || [];
  }

  function statusesForRecord(record) {
    const all = statusesForType(record.type);
    if (!Array.isArray(record.available_status_ids)) return all;
    const ids = new Set(record.available_status_ids.map(Number));
    ids.add(Number(record.status_id));
    return all.filter((item) => ids.has(Number(item.id)));
  }

  function isFinalStatus(record) {
    return finalStatusIds.value[record.type]?.includes(
      Number(record.status_id),
    );
  }

  function fieldConfig(type, key) {
    return (fieldConfigsByType.value[type] || []).find(
      (item) => item.key === key,
    );
  }

  function fieldEnabled(type, key) {
    const config = fieldConfig(type, key);
    return !config || Number(config.enabled) === 1;
  }

  function searchFieldsForType(type) {
    const builtinDefinitions = {
      id: { field: "id", control: "text", operator: "eq" },
      title: { field: "title", control: "text", operator: "contains" },
      status: { field: "status", control: "multi", operator: "in" },
      version: { field: "version", control: "multi", operator: "in" },
      priority: { field: "priority", control: "multi", operator: "in" },
      planned_start_date: {
        field: "planned_start_date",
        control: "date",
        operator: "between",
      },
      planned_end_date: {
        field: "planned_end_date",
        control: "date",
        operator: "between",
      },
      assignee: { field: "assignee", control: "person", operator: "in" },
      created_at: { field: "created_at", control: "date", operator: "between" },
      created_by: { field: "creator", control: "person", operator: "in" },
    };
    const storedConfigs = fieldConfigsByType.value[type] || [];
    const configs = storedConfigs.length
      ? storedConfigs
      : Object.keys(builtinDefinitions).map((key, index) => ({
          key,
          enabled: 1,
          sort_order: index,
        }));
    const builtin = configs
      .filter(
        (config) =>
          Number(config.enabled) === 1 && builtinDefinitions[config.key],
      )
      .map((config) => ({
        ...builtinDefinitions[config.key],
        kind: "builtin",
        label: builtinListFieldMeta.value[config.key]?.label || config.key,
        sort_order: Number(config.sort_order) || 0,
      }))
      .sort((left, right) => left.sort_order - right.sort_order);
    if (!builtin.some((field) => field.field === "title")) {
      builtin.unshift({
        ...builtinDefinitions.title,
        kind: "builtin",
        label: builtinListFieldMeta.value.title.label,
        sort_order: -1,
      });
    }
    if (["requirement", "defect"].includes(type)) {
      builtin.push({
        field: "template",
        kind: "builtin",
        label: t("settings_template"),
        control: "select",
        operator: "in",
        options: [
          {
            value: "0",
            label: t("search_empty_value"),
          },
          ...(templatesByType.value[type] || []).map((template) => ({
            value: String(template.id),
            label: template.name,
            sourceLabel: t(
              template.isInvite
                ? "work_item_template_source_invite"
                : "work_item_template_source_internal",
            ),
            sourceColor: template.isInvite ? "purple" : "blue",
          })),
        ],
      });
    }
    if (type === "requirement") {
      builtin.push({
        field: "category",
        kind: "builtin",
        label: t("view_field_category"),
        control: "select",
        operator: "in",
        options: searchCategoryOptions(),
        sort_order: Number.MAX_SAFE_INTEGER,
      });
      if (agingEnabled.value) {
        builtin.push({
          field: "aging_status",
          kind: "builtin",
          label: t("aging_column"),
          control: "select",
          operator: "in",
          options: [
            { value: "0", label: t("aging_normal") },
            { value: "1", label: t("aging_soon") },
            { value: "2", label: t("aging_overdue") },
          ],
          sort_order: Number.MAX_SAFE_INTEGER - 1,
        });
      }
    }
    const custom = (customFieldsByType.value[type] || [])
      .map(customSearchFieldDescriptor)
      .filter(Boolean)
      .sort((left, right) => left.sort_order - right.sort_order);
    return [...builtin, ...custom];
  }

  function customSearchFieldDescriptor(field) {
    const [control, operator] = customFieldSearchControl(field.field_type);
    if (!control) return null;
    const cascaderOptions = flattenCascaderNodes(field.cascader || []).map(
      (node) => ({
        value: String(node.key),
        label: node.name,
        children: node.children,
      }),
    );
    return {
      field: `custom:${field.id}`,
      kind: "custom",
      label: field.name,
      typeLabel: t(
        CUSTOM_FIELD_TYPE_LABELS[field.field_type] || field.field_type,
      ),
      fieldType: field.field_type,
      control,
      operator,
      options: (field.options || []).map((option) => ({
        value: String(option.key),
        label: option.name,
      })),
      cascader: field.cascader || [],
      cascaderOptions,
      integerOnly: Number(field.integer_only) === 1,
      member_scope: field.member_scope,
      role_ids: field.role_ids || [],
      sort_order: Number(field.sort_order) || 0,
    };
  }

  function searchCategoryOptions() {
    const result = [
      { value: "0", label: t("requirement_categoryUncategorized") },
    ];
    const byParent = new Map();
    requirementCategories.value.forEach((category) => {
      const parentId = Number(category.parent_id) || 0;
      if (!byParent.has(parentId)) byParent.set(parentId, []);
      byParent.get(parentId).push(category);
    });
    const visited = new Set();
    function append(parentId, depth) {
      for (const category of byParent.get(parentId) || []) {
        if (visited.has(Number(category.id))) continue;
        visited.add(Number(category.id));
        result.push({
          value: String(category.id),
          label: `${"  ".repeat(depth)}${category.name}`,
        });
        append(Number(category.id), depth + 1);
      }
    }
    append(0, 0);
    return result;
  }

  function fieldEditable(type, key) {
    const config = fieldConfig(type, key);
    return (
      canEditType(type) &&
      (["title", "status"].includes(key) ||
        !config ||
        Number(config.editable) === 1)
    );
  }

  function permissionType(type) {
    return type === "defect" ? "defect" : "requirement";
  }

  function hasTypePermission(type, action) {
    if (type === "iteration") return false;
    return authStore.hasPermission(`${permissionType(type)}:${action}`);
  }

  function canCreateType(type) {
    return hasTypePermission(type, "create");
  }

  function canEditType(type) {
    return hasTypePermission(type, "edit");
  }

  function canDeleteRecord(record) {
    return hasTypePermission(record?.type, "delete");
  }

  function canEditRecord(record) {
    return canEditType(record?.type);
  }

  // "复制需求" jumps to the create page prefilled from the source work item; the user can
  // review the copy and change the type or project. The `create` permission is required too:
  // without it the create-page route guard sends the user to /403 instead of showing the form.
  function canDuplicateRecord(record) {
    return (
      hasTypePermission(record?.type, "copy") &&
      hasTypePermission(record?.type, "create")
    );
  }

  // The clipboard copy actions only write the work item info to the clipboard,
  // matching the requirement detail drawer. The `copy` permission alone gates the menu.
  function canCopyRecord(record) {
    return hasTypePermission(record?.type, "copy");
  }

  function categoryFilterParam() {
    if (selectedCategoryKey.value === "uncategorized") return 0;
    if (selectedCategoryKey.value.startsWith("category:")) {
      return Number(selectedCategoryKey.value.slice("category:".length));
    }
    return undefined;
  }

  function conditionOf(filters, field) {
    return (filters?.conditions || []).find(
      (condition) => condition.field === field,
    );
  }

  function searchCategoryKey(filters) {
    const value = conditionOf(filters, "category")?.values?.[0];
    if (String(value) === "0") return "uncategorized";
    const id = Number(value);
    return Number.isInteger(id) && id > 0 ? `category:${id}` : "all";
  }

  function withSearchCategory(filters, key) {
    const normalized = normalizeSearchFilterTree(filters, "requirement");
    let condition = conditionOf(normalized, "category");
    if (key !== "all" && !condition) {
      condition = newSearchFilterCondition("category");
      normalized.conditions.push(condition);
    }
    if (condition) {
      const value = categoryQueryFromKey(key);
      condition.values = value === undefined ? [] : [String(value)];
    }
    return normalized;
  }

  function setCategorySelection(key) {
    const nextKey = key || "all";
    const changed = selectedCategoryKey.value !== nextKey;
    selectedCategoryKey.value = nextKey;
    const query = { ...route.query };
    const categoryId = categoryQueryFromKey(nextKey);
    const currentCategoryId = route.query.categoryId;
    if (categoryId && String(currentCategoryId) !== String(categoryId)) {
      query.categoryId = categoryId;
      router.replace({ query });
    } else if (!categoryId && currentCategoryId !== undefined) {
      delete query.categoryId;
      router.replace({ query });
    }
    return changed;
  }

  function currentCategoryCreateQuery() {
    const query = {};
    const categoryId = categoryQueryFromKey(selectedCategoryKey.value);
    if (categoryId) query.categoryId = categoryId;
    return query;
  }

  function selectRequirementCategory(key) {
    const nextKey = key || "all";
    const changed = setCategorySelection(nextKey);
    const filters = withSearchCategory(currentSearchFilters.value, nextKey);
    applySearchFilters(activeTab.value, filters, false);
    setAppliedSearchFilters(activeTab.value, filters);
    persistSearchFilters(activeTab.value, filters, false);
    if (changed && activeTab.value === "requirement") {
      page.value = 1;
      selectedRowKeys.value = [];
      fetchRequirements();
    }
  }

  function setCategorySidebarCollapsed(collapsed) {
    categorySidebarCollapsed.value = collapsed;
    writeScopedListPreferences(
      authStore.userInfo?.id,
      wsId.value,
      "requirement",
      { categorySidebarCollapsed: collapsed },
    );
  }

  function setCategorySidebarWidth(width) {
    categorySidebarWidth.value = Math.max(
      220,
      Math.min(420, Number(width) || 280),
    );
  }

  function persistCategorySidebarWidth(width) {
    setCategorySidebarWidth(width);
    writeScopedListPreferences(
      authStore.userInfo?.id,
      wsId.value,
      "requirement",
      { categorySidebarWidth: categorySidebarWidth.value },
    );
  }

  function goCreateActiveWorkItem() {
    const query = { type: activeTab.value };
    if (activeTab.value === "requirement") {
      Object.assign(query, currentCategoryCreateQuery());
    }
    router.push({ ...workItemRoute(surface, "create", wsId.value), query });
  }

  function goCreateInCategory(categoryId) {
    router.push({
      ...workItemRoute(surface, "create", wsId.value),
      query: {
        type: "requirement",
        categoryId: String(categoryId),
      },
    });
  }

  async function refreshRequirementCategories() {
    await categorySidebarRef.value?.refresh?.();
  }

  async function handleRequirementCategoryChanged() {
    page.value = 1;
    await fetchRequirements();
  }

  async function handleWorkItemsImported() {
    await Promise.all([fetchRequirements(), refreshRequirementCategories()]);
  }

  function renderSelectionCell(_, record, __, checkbox) {
    const menu = record.__loadMore
      ? null
      : h(
          WorkItemRowActionMenu,
          {
            item: record,
            canCreateChild:
              record.type === "requirement" && canCreateType("requirement"),
            canDuplicate: canDuplicateRecord(record),
            canDelete: canDeleteRecord(record),
            canCopy: canCopyRecord(record),
            onAction: handleRowAction,
          },
          {
            default: () =>
              h(
                Button,
                {
                  type: "text",
                  size: "small",
                  class: "row-more-button",
                  title: t("common_more"),
                  "aria-label": t("common_more"),
                },
                () => h(MoreOutlined),
              ),
          },
        );

    return h(
      "div",
      {
        class: "selection-actions",
        onClick: (event) => event.stopPropagation(),
      },
      [checkbox, menu],
    );
  }

  function uniqueWorkItemIds(ids) {
    return [
      ...new Set(
        ids.map(Number).filter((id) => Number.isInteger(id) && id > 0),
      ),
    ];
  }

  const rowSelection = computed(() => ({
    selectedRowKeys: selectedRowKeys.value,
    onChange: (keys) => (selectedRowKeys.value = keys),
    columnWidth: selectionColumnWidth,
    fixed: hasFrozenColumns.value,
    hideSelectAll: true,
    columnTitle: h(Checkbox, {
      checked: allVisibleRowsSelected.value,
      indeterminate: someVisibleRowsSelected.value,
      "aria-label": t("common_selectAll"),
      onChange: (event) => toggleVisibleRowSelection(event.target.checked),
      onClick: (event) => event.stopPropagation(),
    }),
    getCheckboxProps: (record) => ({ disabled: record.__loadMore }),
    renderCell: renderSelectionCell,
  }));

  const visibleSelectableRowKeys = computed(() =>
    treeList.value
      .filter((record) => !record.__loadMore)
      .map((record) => record.id),
  );
  const allVisibleRowsSelected = computed(() => {
    if (!visibleSelectableRowKeys.value.length) return false;
    const selected = new Set(selectedRowKeys.value.map(String));
    return visibleSelectableRowKeys.value.every((key) =>
      selected.has(String(key)),
    );
  });
  const someVisibleRowsSelected = computed(() => {
    if (allVisibleRowsSelected.value) return false;
    const selected = new Set(selectedRowKeys.value.map(String));
    return visibleSelectableRowKeys.value.some((key) =>
      selected.has(String(key)),
    );
  });

  function toggleVisibleRowSelection(checked) {
    const visibleKeys = new Set(visibleSelectableRowKeys.value.map(String));
    if (checked) {
      const selected = new Map(
        selectedRowKeys.value.map((key) => [String(key), key]),
      );
      visibleSelectableRowKeys.value.forEach((key) =>
        selected.set(String(key), key),
      );
      selectedRowKeys.value = [...selected.values()];
      return;
    }
    selectedRowKeys.value = selectedRowKeys.value.filter(
      (key) => !visibleKeys.has(String(key)),
    );
  }

  const builtinListFieldMeta = computed(() => ({
    id: { label: t("workspace_id"), columnKey: "id" },
    title: { label: t("workspace_workItemTitle"), columnKey: "title" },
    status: { label: t("workspace_status"), columnKey: "status" },
    version: {
      label: t("requirement_filterVersion"),
      columnKey: "version",
    },
    category: {
      label: t("workspace_field_category"),
      columnKey: "category",
    },
    priority: { label: t("workspace_priority"), columnKey: "priority" },
    planned_start_date: {
      label: t("workspace_plannedStartDateCol"),
      columnKey: "dueDate",
    },
    planned_end_date: {
      label: t("workspace_plannedEndDateCol"),
      columnKey: "endDate",
    },
    assignee: { label: t("workspace_assignee"), columnKey: "assignee" },
    created_at: {
      label: t("workspace_createdTime"),
      columnKey: "createdAt",
    },
    created_by: {
      label: t("workspace_createdBy"),
      columnKey: "createdBy",
    },
    estimated_work_hours: {
      label: t("requirement_estimatedWorkHours"),
      columnKey: "estimatedHours",
    },
    work_progress: {
      label: t("requirement_workProgress"),
      columnKey: "workProgress",
    },
    completed_work_hours: {
      label: t("requirement_completedWorkHours"),
      columnKey: "completedHours",
    },
    remaining_work_hours: {
      label: t("requirement_remainingWorkHours"),
      columnKey: "remainingHours",
    },
    overtime_work_hours: {
      label: t("requirement_overtimeWorkHours"),
      columnKey: "overtimeHours",
    },
    aging: {
      label: t("aging_column"),
      columnKey: "aging",
    },
  }));

  const agingEnabled = computed(
    () =>
      activeTab.value === "requirement" &&
      !workspaceMetadata.value?.settings?.sla_activated_at &&
      !!workspaceMetadata.value?.settings?.requirement_aging_control
        ?.master_enabled,
  );

  const slaEnabled = computed(
    () =>
      activeTab.value === "requirement" &&
      workspaceMetadata.value?.workspace?.workspace_type === "issue" &&
      !!workspaceMetadata.value?.settings?.sla_enabled,
  );
  const slaSummaries = ref({});
  const slaFailed = ref(false);
  let slaSequence = 0;
  async function loadSLASummaries() {
    const sequence = ++slaSequence;
    const workspace = wsId.value;
    if (!slaEnabled.value) {
      slaSummaries.value = {};
      slaFailed.value = false;
      return;
    }
    const ids = [
      ...new Set(
        treeList.value
          .filter((row) => !row.__loadMore)
          .map((row) => Number(row.id)),
      ),
    ];
    try {
      const results = [];
      for (let index = 0; index < ids.length; index += 100) {
        if (sequence !== slaSequence) return;
        results.push(
          ...(await getSLASummaries(workspace, ids.slice(index, index + 100)))
            .list,
        );
      }
      if (sequence === slaSequence) {
        if (results.some((row) => !row.sla.enabled))
          workspaceMetadata.value.settings.sla_enabled = false;
        slaSummaries.value = Object.fromEntries(
          results.map((row) => [row.requirement_id, row.sla]),
        );
        slaFailed.value = false;
      }
    } catch {
      if (sequence === slaSequence) {
        slaSummaries.value = {};
        slaFailed.value = true;
      }
    }
  }
  const slaTimer = setInterval(loadSLASummaries, 60000);
  onBeforeUnmount(() => {
    ++slaSequence;
    clearInterval(slaTimer);
  });

  const systemListFields = computed(() => {
    const type = activeTab.value === "defect" ? "defect" : "requirement";
    const configs = fieldConfigsByType.value[type] || [];
    const builtinFields = (
      configs.length
        ? configs
        : Object.keys(builtinListFieldMeta.value)
            .filter((key) => key !== "aging")
            .map((key, index) => ({
              key,
              enabled: 1,
              sort_order: index,
            }))
    )
      .map((config) => ({
        kind: "builtin",
        preferenceKey: `builtin:${config.key}`,
        configKey: config.key,
        columnKey: builtinListFieldMeta.value[config.key]?.columnKey,
        label: builtinListFieldMeta.value[config.key]?.label,
        enabled: Number(config.enabled) === 1,
        visible: Number(config.enabled) === 1,
        sort_order: config.sort_order,
      }))
      .filter((field) => field.columnKey);
    const customFields = (customFieldsByType.value[type] || []).map(
      (field) => ({
        kind: "custom",
        preferenceKey: `custom:${field.id}`,
        customField: field,
        columnKey: `custom_${field.id}`,
        label: field.name,
        enabled: true,
        visible: true,
        sort_order: field.sort_order,
      }),
    );
    const fields = [...builtinFields, ...customFields];
    if (type === "requirement" && agingEnabled.value) {
      fields.push({
        kind: "builtin",
        preferenceKey: "builtin:aging",
        configKey: "aging",
        columnKey: "aging",
        label: t("aging_column"),
        enabled: true,
        visible: true,
        sort_order: Number.MAX_SAFE_INTEGER,
      });
    }
    if (slaEnabled.value)
      fields.push({
        kind: "builtin",
        preferenceKey: "builtin:sla",
        configKey: "sla",
        columnKey: "sla",
        label: t("sla_title"),
        enabled: true,
        visible: true,
        sort_order: Number.MAX_SAFE_INTEGER,
      });
    const orderedFields = sortFieldsByOrder(fields);
    const statusIndex = orderedFields.findIndex(
      (field) => field.columnKey === "status",
    );
    const insertAfter =
      statusIndex >= 0
        ? statusIndex
        : orderedFields.findIndex((field) => field.columnKey === "title");
    orderedFields.splice(insertAfter + 1, 0, {
      kind: "builtin",
      preferenceKey: "builtin:template",
      configKey: "template",
      columnKey: "template",
      label: t("settings_template"),
      enabled: true,
      visible: true,
    });
    return orderedFields;
  });

  const effectiveListFields = computed(() => {
    return listPreferences.fields.length
      ? listPreferences.fields
      : systemListFields.value;
  });

  function normalizeListSort(value) {
    const field = LIST_SORT_FIELDS.has(value?.field) ? value.field : "";
    const direction = ["asc", "desc"].includes(value?.direction)
      ? value.direction
      : "";
    return field && direction
      ? { field, direction }
      : { field: "", direction: "" };
  }

  function loadListTablePreferences() {
    const userId = authStore.userInfo?.id;
    for (const type of ["requirement", "defect"]) {
      const stored = readScopedListPreferences(userId, wsId.value, type);
      listTablePreferences[type] = {
        sort: normalizeListSort(stored.listSort),
        frozenThrough:
          typeof stored.frozenThrough === "string" ? stored.frozenThrough : "",
      };
    }
  }

  function persistCurrentListTablePreferences(patch) {
    const type = activeTab.value;
    if (!["requirement", "defect"].includes(type)) return;
    Object.assign(listTablePreferences[type], patch);
    writeScopedListPreferences(authStore.userInfo?.id, wsId.value, type, {
      listSort: { ...listTablePreferences[type].sort },
      frozenThrough: listTablePreferences[type].frozenThrough,
    });
  }

  const visibleListPreferenceKeys = computed(() =>
    effectiveListFields.value
      .filter((field) => field.visible)
      .map((field) => field.preferenceKey),
  );

  const frozenColumnIndex = computed(() =>
    visibleListPreferenceKeys.value.indexOf(
      listTablePreferences[activeTab.value]?.frozenThrough || "",
    ),
  );

  const hasFrozenColumns = computed(() => frozenColumnIndex.value >= 0);

  function persistListFields(fields) {
    listPreferences.fields = fields.map((field) => ({ ...field }));
    writeScopedListPreferences(
      authStore.userInfo?.id,
      wsId.value,
      activeTab.value,
      {
        fields: fields.map((field) => ({
          key: field.preferenceKey,
          visible: field.visible !== false,
        })),
      },
    );
  }

  function startColumnDrag(event, column) {
    if (event.target.closest?.(".column-header-action")) {
      event.preventDefault();
      return;
    }
    columnDragState.sourceKey = column.preferenceKey;
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData("text/plain", column.preferenceKey);
    const source = event.currentTarget;
    const preview = document.createElement("div");
    preview.className = "column-drag-preview";
    preview.textContent =
      source.querySelector(".work-item-column-label")?.textContent ||
      column.title;
    preview.style.width = `${Math.max(160, Math.min(220, source.getBoundingClientRect().width + 56))}px`;
    document.body.appendChild(preview);
    event.dataTransfer.setDragImage(preview, 18, 20);
    columnDragPreview = preview;
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
    columnDragPreview?.remove();
    columnDragPreview = null;
    columnDragState.sourceKey = "";
    columnDragState.targetKey = "";
    columnDragState.side = "";
  }

  function finishColumnDrop(event, column) {
    const sourceKey = columnDragState.sourceKey;
    const targetKey = column.preferenceKey;
    const side = columnDragState.side || "before";
    clearColumnDrag();
    if (!sourceKey || sourceKey === targetKey) return;
    const fields = effectiveListFields.value.map((field) => ({ ...field }));
    const sourceIndex = fields.findIndex(
      (field) => field.preferenceKey === sourceKey,
    );
    const targetIndex = fields.findIndex(
      (field) => field.preferenceKey === targetKey,
    );
    if (sourceIndex < 0 || targetIndex < 0) return;
    const [moved] = fields.splice(sourceIndex, 1);
    const nextTargetIndex = fields.findIndex(
      (field) => field.preferenceKey === targetKey,
    );
    fields.splice(nextTargetIndex + (side === "after" ? 1 : 0), 0, moved);
    persistListFields(fields);
  }

  function cycleColumnSort(column) {
    if (!column.sortField) return;
    const current = currentListSort.value;
    let sort = { field: column.sortField, direction: "asc" };
    if (current.field === column.sortField && current.direction === "asc") {
      sort = { field: column.sortField, direction: "desc" };
    } else if (
      current.field === column.sortField &&
      current.direction === "desc"
    ) {
      sort = { field: "", direction: "" };
    }
    persistCurrentListTablePreferences({ sort });
    page.value = 1;
    selectedRowKeys.value = [];
    fetchRequirements();
  }

  function columnSortIcon(column) {
    if (currentListSort.value.field !== column.sortField)
      return defaultSortIcon;
    return currentListSort.value.direction === "desc"
      ? descendingSortIcon
      : ascendingSortIcon;
  }

  function handleColumnMenu(key, column) {
    if (key === "freeze") {
      if (!canFreezeColumn(column)) return;
      persistCurrentListTablePreferences({
        frozenThrough: column.preferenceKey,
      });
    } else if (key === "unfreeze") {
      persistCurrentListTablePreferences({ frozenThrough: "" });
    } else if (key === "hide") {
      if (!canHideColumn(column)) return;
      const fields = effectiveListFields.value.map((field) => ({ ...field }));
      const target = fields.find(
        (field) => field.preferenceKey === column.preferenceKey,
      );
      if (!target || !target.visible) return;
      target.visible = false;
      persistListFields(fields);
      message.success(t("workspace_columnHiddenSuccess"));
    }
  }

  function canFreezeColumn(column) {
    const index = visibleListPreferenceKeys.value.indexOf(column.preferenceKey);
    return index >= 0 && index < visibleListPreferenceKeys.value.length - 1;
  }

  // The title column carries the tree expand/collapse toggle and the detail open
  // entry; hiding it from this quick action keeps the list usable. Reopen hidden
  // columns via the list settings modal.
  function canHideColumn(column) {
    return column.key !== "title" && Boolean(column.preferenceKey);
  }

  // Keep the column header actions container expanded while the "..." dropdown is
  // open. On close, delay clearing the key so the overlay's exit animation finishes
  // before the trigger collapses — otherwise the menu "flashes" as vc-trigger
  // repositions the overlay to the shrinking trigger element.
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
      if (openColumnMenuKey.value === key) {
        openColumnMenuKey.value = "";
      }
      closeColumnMenuTimer = null;
    }, 250);
  }

  function applyListPreferences(preferences) {
    const previousViewMode = listPreferences.viewMode;
    listPreferences.previewMode = preferences.previewMode;
    listPreferences.viewMode = preferences.viewMode;
    listPreferences.fields = preferences.fields;
    listPreferences.boardStatuses = preferences.boardStatuses;
    listPreferences.boardSort = normalizeBoardSort(preferences.boardSort);
    if (preferences.boardColors) {
      listPreferences.boardColors = readBoardColors(preferences.boardColors);
    }
    if (previousViewMode !== preferences.viewMode) {
      selectedRowKeys.value = [];
      if (preferences.viewMode === "list") fetchRequirements();
    }
  }

  function handleSharedListPreferencesChange(event) {
    if (String(event.detail?.userId) !== String(authStore.userInfo?.id)) return;
    const patch = event.detail?.patch || {};
    if (["drawer", "modal", "window"].includes(patch.previewMode)) {
      listPreferences.previewMode = patch.previewMode;
    }
    if (["list", "board"].includes(patch.viewMode)) {
      const previousViewMode = listPreferences.viewMode;
      listPreferences.viewMode = patch.viewMode;
      if (previousViewMode !== patch.viewMode) {
        selectedRowKeys.value = [];
        if (patch.viewMode === "list") fetchRequirements();
      }
    }
  }

  function persistKanbanPreferences(patch) {
    const userId = authStore.userInfo?.id;
    const typeKey = activeTab.value;
    writeScopedListPreferences(userId, wsId.value, typeKey, patch);
  }

  function handleKanbanStatusVisibility({ id, visible }) {
    const statuses = listPreferences.boardStatuses.length
      ? listPreferences.boardStatuses.map((status) => ({ ...status }))
      : statusOptions.value.map((status) => ({ id: status.id, visible: true }));
    const current = statuses.find((status) => Number(status.id) === Number(id));
    if (current) current.visible = visible !== false;
    else statuses.push({ id: Number(id), visible: visible !== false });
    listPreferences.boardStatuses = statuses;
    persistKanbanPreferences({ boardStatuses: statuses });
  }

  function handleKanbanStatusColor({ id, color }) {
    const colors = { ...listPreferences.boardColors };
    if (color) colors[String(id)] = color;
    else delete colors[String(id)];
    listPreferences.boardColors = colors;
    persistKanbanPreferences({ boardColors: colors });
  }

  async function loadTemplateOptions() {
    const workspaceId = wsId.value;
    const sequence = ++templateRequestSequence;
    const types = ["requirement", "defect"].filter((type) =>
      authStore.hasPermission(`${type}:view`),
    );
    const inviteTemplates =
      enabledApplications.value.includes("issue") &&
      authStore.hasPermission("workspace:view") &&
      types.length
        ? listIssueInviteTemplates(workspaceId).catch(() => {
            console.error("Failed to load invitation templates");
            return [];
          })
        : Promise.resolve([]);
    const entries = await Promise.all(
      types.map(async (type) => {
        try {
          const result = await listWorkItemTemplates(workspaceId, type);
          return [type, result?.items || []];
        } catch {
          console.error("Failed to load work item templates", type);
          return [type, []];
        }
      }),
    );
    const invites = await inviteTemplates;
    if (sequence !== templateRequestSequence || workspaceId !== wsId.value)
      return;
    templatesByType.value = Object.fromEntries(
      entries.map(([type, templates]) => [
        type,
        [
          ...templates,
          ...invites
            .filter((template) => template.type_key === type)
            .map((template) => ({
              ...template,
              isInvite: true,
            })),
        ],
      ]),
    );
  }

  async function loadWorkflowOptions() {
    const types = ["requirement", "defect"];
    const configs =
      authStore.consumeProjectMetadata(wsId.value) ||
      issueMetadata?.value ||
      (await getWorkspaceMetadata(wsId.value));
    statusesByType.value = Object.fromEntries(
      types.map((type) => [
        type,
        (configs?.[type]?.statuses || []).map((item) => ({
          ...item,
          id: Number(item.id),
        })),
      ]),
    );
    prioritiesByType.value = Object.fromEntries(
      types.map((type) => [
        type,
        (configs?.[type]?.priorities || []).map((item) => ({
          ...item,
          id: Number(item.id),
        })),
      ]),
    );
    fieldConfigsByType.value = Object.fromEntries(
      types.map((type) => [type, configs?.[type]?.field_configs || []]),
    );
    customFieldsByType.value = Object.fromEntries(
      types.map((type) => [
        type,
        (configs?.[type]?.custom_fields || []).filter(
          (field) => Number(field.is_enabled) === 1,
        ),
      ]),
    );
    members.value = (configs?.members || []).map((member) => ({
      ...member,
      user_id: Number(member.user_id),
    }));
    customFieldMembers.value = (configs?.custom_field_members || []).map(
      (member) => ({
        ...member,
        user_id: Number(member.user_id),
      }),
    );
    workspaceName.value = configs?.workspace?.name || "";
    projectIcon.value = configs?.workspace?.icon || "";
    projectColor.value = configs?.workspace?.color || "#0d68ff";
    projectEnded.value = Number(configs?.workspace?.ended_at || 0) > 0;
    enabledApplications.value = configs?.enabled_applications || [];
    // Publish the resolved switches so the surfaces gated by an app read this answer
    // instead of requesting their own copy.
    if (configs?.enabled_applications)
      syncWorkspaceApplications(
        wsId.value,
        enabledApplications.value,
        configs?.workspace?.workspace_type || "",
      );
    // Re-resolve the tab once app availability is known: a remembered or URL-requested
    // tab of an app-driven module (gantt, issue-invite, defect) is unavailable before the
    // metadata loads; leave the content empty if no permitted tab is ready yet.
    for (const appTab of ["gantt", "issue-invite", "defect"]) {
      if (!visibleTopTabs.value.includes(appTab) || activeTab.value === appTab)
        continue;
      if (resolveBoardTab() === appTab) activeTab.value = appTab;
    }
    completedStatusIds.value = {
      requirement: (
        configs?.settings?.requirement_completed_status_ids || []
      ).map(Number),
      defect: (configs?.settings?.defect_completed_status_ids || []).map(
        Number,
      ),
    };
    finalStatusIds.value = {
      requirement: (configs?.settings?.requirement_final_status_ids || []).map(
        Number,
      ),
      defect: (configs?.settings?.defect_final_status_ids || []).map(Number),
    };
    applyVersions(configs?.versions);
    workspaceMetadata.value = configs;
    applyRequestedBoardTab();
  }

  // Fetch the requirement list.
  async function fetchRequirements() {
    // Planning, iteration, gantt and knowledge panels fetch their own data inside
    // themselves; the requirement list API rejects those tab keys as work item types.
    if (!isWorkItemTab.value) return;
    if (isWorkItemTab.value && !boardContextReady.value) return;
    if (isWorkItemTab.value && listPreferences.viewMode === "board") {
      requirements.value = [];
      total.value = 0;
      loading.value = false;
      return;
    }
    const requestSequence = ++listRequestSequence;
    const isWorkItemList = isWorkItemTab.value;
    if (isWorkItemList) resetTreeContext();
    loading.value = true;
    try {
      if (
        isWorkItemList &&
        filterStatusActive.value &&
        !filterStatusIds.value.length
      ) {
        requirements.value = [];
        total.value = 0;
        return;
      }
      const keyword = appliedSearchText.value.trim();
      const params = {
        page: page.value,
        size: pageSize.value,
        // A view with active filters may match child requirements whose parents do not match,
        // so the list falls back to flat mode (same as searching).
        root_only:
          activeTab.value === "requirement" &&
          !hasAppliedSearchValues() &&
          !viewHasActiveFilters.value
            ? 1
            : 0,
      };
      if (isWorkItemList && currentListSort.value.field) {
        params.sort_field = currentListSort.value.field;
        params.sort_direction = currentListSort.value.direction;
      }
      params.type = activeTab.value;
      if (currentViewId.value) params.view_id = currentViewId.value;
      if (activeTab.value === "requirement") {
        const categoryId = categoryFilterParam();
        if (categoryId !== undefined) params.category_id = categoryId;
      }
      if (filterStatusIds.value.length)
        params.status_ids = filterStatusIds.value.join(",");
      if (filterAssignee.value.length)
        params.assignee_ids = filterAssignee.value.join(",");
      if (filterVersionIds.value.length)
        params.version_ids = filterVersionIds.value.join(",");
      if (filterCreatorIds.value.length)
        params.created_by_ids = filterCreatorIds.value.join(",");
      if (filterPriorityIds.value.length)
        params.priority_ids = filterPriorityIds.value.join(",");
      if (keyword) params.title_search = keyword;
      params.search_filters = JSON.stringify(currentAppliedSearchFilters.value);
      params.timezone =
        Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
      const res = await listRequirements(wsId.value, params);
      if (requestSequence !== listRequestSequence) return;
      const list = res?.list || [];
      requirements.value = list;
      total.value = Number(res?.total) || 0;
      selectedRowKeys.value = selectedRowKeys.value.filter((id) =>
        list.some((item) => String(item.id) === String(id)),
      );
      if (params.root_only === 1) void prefetchChildRequirements(list);
    } catch {
      /* handled by interceptor */
    } finally {
      if (requestSequence === listRequestSequence) loading.value = false;
    }
  }

  function resetTreeContext() {
    listContextSequence += 1;
    const visibleIds = new Set(
      requirements.value.map((item) => String(item.id)),
    );
    selectedRowKeys.value = selectedRowKeys.value.filter((id) =>
      visibleIds.has(String(id)),
    );
    expandedRowKeys.value = [];
    allTreeExpanded.value = false;
    childrenMap.value = {};
    childLoadingKeys.value = new Set();
    childLoadTokens.clear();
    childLoadPromises.clear();
  }

  // Fetch the version list.
  async function fetchVersions() {
    try {
      const configs = await getWorkspaceMetadata(wsId.value);
      applyVersions(configs?.versions);
    } catch {
      /* handled */
    }
  }

  function applyVersions(list) {
    versions.value = (list || []).map((version) => ({
      ...version,
      id: Number(version.id),
      is_ended: version.is_ended === true || Number(version.ended_at) > 0,
      requirement_count: Number(version.requirement_count) || 0,
      completed_count: Number(version.completed_count) || 0,
    }));
  }

  function versionsForRecord(record) {
    const selectedId = Number(record?.version_id) || 0;
    return versions.value.filter(
      (version) => !version.is_ended || Number(version.id) === selectedId,
    );
  }

  function isCrossVersion(versionId) {
    return versions.value.some(
      (version) =>
        Number(version.id) === Number(versionId) &&
        version.iteration_type === "cross",
    );
  }

  function resetWorkItemRuntimeState() {
    templateRequestSequence += 1;
    templatesByType.value = {
      requirement: [],
      defect: [],
    };
    listRequestSequence += 1;
    listContextSequence += 1;
    requirements.value = [];
    total.value = 0;
    loading.value = false;
    searchText.value = "";
    appliedSearchText.value = "";
    appliedSearchFiltersByType.value = {};
    filterStatusActive.value = false;
    filterStatusIds.value = [];
    filterAssignee.value = [];
    filterVersionIds.value = [];
    filterCreatorIds.value = [];
    filterPriorityIds.value = [];
    selectedRowKeys.value = [];
    expandedRowKeys.value = [];
    childrenMap.value = {};
    childLoadingKeys.value = new Set();
    childLoadTokens.clear();
    childLoadPromises.clear();
    kanbanNavigationItems.value = [];
  }

  onMounted(async () => {
    const workspaceSequence = ++workspaceLoadSequence;
    window.addEventListener(
      GLOBAL_LIST_PREFERENCES_CHANGE_EVENT,
      handleSharedListPreferencesChange,
    );
    document.addEventListener("mousedown", startTableResizeGuide, true);
    tableResizeObserver = new ResizeObserver(([entry]) => {
      tableAreaWidth.value = entry.contentRect.width;
    });
    if (boardPageRef.value) tableResizeObserver.observe(boardPageRef.value);
    try {
      await loadWorkflowOptions();
    } catch {
      /* handled */
    }
    if (workspaceSequence !== workspaceLoadSequence) return;
    await loadTemplateOptions();
    if (workspaceSequence !== workspaceLoadSequence) return;

    if (sharedDetailId && (await openSharedDetail(sharedDetailId))) return;
    if (workspaceSequence !== workspaceLoadSequence) return;
    normalizeBoardTabQuery();
    if (isWorkItemTab.value) {
      await ensureViewsLoaded(activeTab.value);
      if (workspaceSequence !== workspaceLoadSequence) return;
      hydrateSearchState(activeTab.value);
    }
    boardContextReady.value = true;
    fetchRequirements();
  });
  onBeforeUnmount(() => {
    workspaceLoadSequence += 1;
    templateRequestSequence += 1;
    window.removeEventListener(
      GLOBAL_LIST_PREFERENCES_CHANGE_EVENT,
      handleSharedListPreferencesChange,
    );
    flushPendingSearchSave();
    document.removeEventListener("mousedown", startTableResizeGuide, true);
    tableResizeObserver?.disconnect();
    tableResizeGuideCleanup?.();
    if (detailRowScrollFrame) cancelAnimationFrame(detailRowScrollFrame);
    if (closeColumnMenuTimer) {
      clearTimeout(closeColumnMenuTimer);
      closeColumnMenuTimer = null;
    }
  });

  watch(() => [authStore.userInfo?.id, wsId.value], loadColumnWidths, {
    immediate: true,
  });

  watch(() => [authStore.userInfo?.id, wsId.value], loadListTablePreferences, {
    immediate: true,
  });

  watch(
    [activeTab, visibleListPreferenceKeys],
    async ([type]) => {
      if (!["requirement", "defect"].includes(type)) return;
      await nextTick();
      if (type !== activeTab.value || !systemListFields.value.length) return;
      const frozenThrough = listTablePreferences[type].frozenThrough;
      if (
        frozenThrough &&
        (!visibleListPreferenceKeys.value.includes(frozenThrough) ||
          visibleListPreferenceKeys.value.indexOf(frozenThrough) ===
            visibleListPreferenceKeys.value.length - 1)
      ) {
        const previousType = activeTab.value;
        if (previousType === type) {
          persistCurrentListTablePreferences({ frozenThrough: "" });
        } else {
          listTablePreferences[type].frozenThrough = "";
          writeScopedListPreferences(authStore.userInfo?.id, wsId.value, type, {
            frozenThrough: "",
          });
        }
      }
    },
    { deep: true },
  );

  // Re-fetch data when the workspace changes.
  watch(
    () => route.params.wsId,
    async (newId) => {
      if (newId && newId !== wsId.value) {
        const workspaceSequence = ++workspaceLoadSequence;
        boardContextReady.value = false;
        flushPendingSearchSave();
        resetWorkItemRuntimeState();
        resetViewsForWorkspace();
        wsId.value = newId;
        activeTab.value = resolveBoardTab();
        selectedCategoryKey.value = categoryKeyFromQuery(
          route.query.categoryId,
        );
        categorySidebarCollapsed.value = readStoredCategorySidebarCollapsed();
        categorySidebarWidth.value = readStoredCategorySidebarWidth();
        requirementCategories.value = [];
        detailReqId.value = null;
        page.value = 1;
        try {
          await loadWorkflowOptions();
        } catch {
          /* handled */
        }
        if (workspaceSequence !== workspaceLoadSequence) return;
        await loadTemplateOptions();

        if (workspaceSequence !== workspaceLoadSequence) return;
        normalizeBoardTabQuery();
        if (isWorkItemTab.value) {
          await ensureViewsLoaded(activeTab.value);
          if (workspaceSequence !== workspaceLoadSequence) return;
          hydrateSearchState(activeTab.value);
        }
        boardContextReady.value = true;
        fetchRequirements();
      }
    },
  );

  watch(
    () => route.query.categoryId,
    (value) => {
      const nextKey = categoryKeyFromQuery(value);
      if (nextKey === selectedCategoryKey.value) return;
      selectedCategoryKey.value = nextKey;
      if (!boardContextReady.value) return;
      if (activeTab.value === "requirement") {
        const filters = withSearchCategory(currentSearchFilters.value, nextKey);
        applySearchFilters("requirement", filters, false);
        setAppliedSearchFilters("requirement", filters);
        persistSearchFilters("requirement", filters, false);
        page.value = 1;
        selectedRowKeys.value = [];
        fetchRequirements();
      }
    },
  );

  watch(
    visibleTopTabs,
    (tabs) => {
      if (tabs.length && !tabs.includes(activeTab.value)) {
        // Closing the defect app while its tab is open must land on the requirement list,
        // not on whichever tab happens to be first (planning usually is).
        const fallback =
          activeTab.value === "defect" && tabs.includes("requirement")
            ? "requirement"
            : tabs[0];
        activeTab.value = fallback;
        normalizeBoardTabQuery();
        if (boardContextReady.value) fetchRequirements();
      }
    },
    { immediate: true },
  );

  // Search hits are always visible; expanded items only supplement the direct child requirements of the current node.
  const treeList = computed(() => {
    if (activeTab.value !== "requirement") return requirements.value;

    const baseItems = requirements.value;
    const baseById = new Map(baseItems.map((item) => [String(item.id), item]));
    const baseChildren = new Map();
    for (const item of baseItems) {
      const parentId = String(item.parent_id || "");
      if (!parentId || parentId === "0") continue;
      if (!baseChildren.has(parentId)) baseChildren.set(parentId, []);
      baseChildren.get(parentId).push(item);
    }
    const expandedIds = new Set(expandedRowKeys.value.map(String));

    function mergedChildren(parentId) {
      const key = String(parentId);
      const expanded = expandedIds.has(key);
      const loaded = expanded ? childrenMap.value[parentId]?.list || [] : [];
      // Flat mode (search/view filters) returns both parents and children in one page;
      // children are only shown when their parent is expanded, otherwise the arrow has no effect.
      const matched = expanded ? baseChildren.get(key) || [] : [];
      const result = [];
      const seen = new Set();
      for (const child of [...loaded, ...matched]) {
        const childId = String(child.id);
        if (seen.has(childId)) continue;
        seen.add(childId);
        result.push(baseById.get(childId) || child);
      }
      return result;
    }

    const nestedIds = new Set();
    function markNested(parentId, ancestors) {
      const key = String(parentId);
      if (ancestors.has(key)) return;
      const nextAncestors = new Set(ancestors).add(key);
      for (const child of mergedChildren(parentId)) {
        const childId = String(child.id);
        if (nextAncestors.has(childId)) continue;
        nestedIds.add(childId);
        markNested(child.id, nextAncestors);
      }
    }
    for (const item of baseItems) {
      if (!baseById.has(String(item.parent_id || ""))) {
        markNested(item.id, new Set());
      }
    }

    const result = [];
    const renderedIds = new Set();
    function append(item, depth, parentId, ancestors) {
      const itemId = String(item.id);
      if (renderedIds.has(itemId) || ancestors.has(itemId)) return;
      renderedIds.add(itemId);
      result.push({
        ...item,
        __isChild: depth > 0,
        __parentId: parentId,
        __depth: depth,
      });
      const nextAncestors = new Set(ancestors).add(itemId);
      for (const child of mergedChildren(item.id)) {
        append(child, depth + 1, item.id, nextAncestors);
      }
      const state = childrenMap.value[item.id];
      if (expandedIds.has(itemId) && state && state.list.length < state.total) {
        result.push({
          id: `load-more-${item.id}-${state.page + 1}`,
          __loadMore: true,
          __isChild: true,
          __parentId: item.id,
          __depth: depth + 1,
        });
      }
    }
    for (const item of baseItems) {
      if (!nestedIds.has(String(item.id))) append(item, 0, null, new Set());
    }
    for (const item of baseItems) {
      if (!renderedIds.has(String(item.id))) append(item, 0, null, new Set());
    }
    return result;
  });
  const boardNavigationItems = computed(() =>
    listPreferences.viewMode === "board" && isWorkItemTab.value
      ? kanbanNavigationItems.value
      : treeList.value
          .filter((item) => !item.__loadMore)
          .map((item) => ({ id: item.id, workspaceId: wsId.value })),
  );

  function scrollSelectedDetailRowIntoView() {
    if (listPreferences.previewMode !== "drawer" || !detailReqId.value) return;
    if (detailRowScrollFrame) cancelAnimationFrame(detailRowScrollFrame);
    nextTick(() => {
      detailRowScrollFrame = requestAnimationFrame(() => {
        detailRowScrollFrame = undefined;
        const selectedId = String(detailReqId.value);
        const rows = boardPageRef.value?.querySelectorAll(
          ".requirement-table .ant-table-tbody > tr[data-row-key]",
        );
        const selectedRow = [...(rows || [])].find(
          (row) => row.getAttribute("data-row-key") === selectedId,
        );
        selectedRow?.scrollIntoView({ block: "nearest", inline: "nearest" });
      });
    });
  }

  watch(
    [
      detailReqId,
      () => listPreferences.previewMode,
      () => treeList.value.length,
    ],
    scrollSelectedDetailRowIntoView,
    { flush: "post", immediate: true },
  );

  watch([wsId, slaEnabled, treeList], () => {
    slaSummaries.value = {};
    loadSLASummaries();
  });

  const tableCols = computed(() => {
    const type = activeTab.value;
    const dataColumn = (column, minWidth = 80) => {
      const maxWidth = column.maxWidth || 600;
      const width = Number(columnWidth(type, column.key, column.width));
      return {
        ...column,
        width: Math.min(
          maxWidth,
          Math.max(minWidth, Number.isFinite(width) ? width : minWidth),
        ),
        minWidth,
        maxWidth,
        resizable: true,
      };
    };
    const editableDataColumn = (column, field, minWidth = 80) => ({
      ...dataColumn(column, minWidth),
      customCell: (record) =>
        editableCellProps(record, field, column.customField),
    });
    const builtinColumns = {
      id: dataColumn(
        {
          title: t("workspace_id"),
          dataIndex: "id",
          key: "id",
          width: 90,
          sortField: "id",
        },
        80,
      ),
      title: dataColumn(
        {
          title: t("workspace_workItemTitle"),
          key: "title",
          width: 320,
          maxWidth: 1000,
          ellipsis: true,
        },
        240,
      ),
      status: editableDataColumn(
        {
          title: t("workspace_status"),
          key: "status",
          width: 90,
          sortField: "status",
        },
        "status_id",
        96,
      ),
      template: dataColumn(
        {
          title: t("settings_template"),
          dataIndex: "template_name",
          key: "template",
          width: 160,
          ellipsis: true,
        },
        120,
      ),
      version: editableDataColumn(
        {
          title: t("requirement_filterVersion"),
          key: "version",
          width: 120,
          sortField: "version",
        },
        "version_id",
        120,
      ),
      category: dataColumn(
        {
          title: t("workspace_field_category"),
          dataIndex: "category_id",
          key: "category",
          width: 140,
          ellipsis: true,
        },
        100,
      ),
      priority: editableDataColumn(
        {
          title: t("workspace_priority"),
          key: "priority",
          width: 80,
          sortField: "priority",
        },
        "priority",
        96,
      ),
      planned_start_date: editableDataColumn(
        {
          title: t("workspace_plannedStartDateCol"),
          dataIndex: "planned_start_date",
          key: "dueDate",
          width: 124,
          sortField: "planned_start_date",
        },
        "planned_start_date",
        124,
      ),
      planned_end_date: editableDataColumn(
        {
          title: t("workspace_plannedEndDateCol"),
          dataIndex: "planned_end_date",
          key: "endDate",
          width: 124,
          sortField: "planned_end_date",
        },
        "planned_end_date",
        124,
      ),
      assignee: editableDataColumn(
        {
          title: t("workspace_assignee"),
          key: "assignee",
          width: 100,
          sortField: "assignee",
        },
        "assignee_ids",
        110,
      ),
      created_at: dataColumn(
        {
          title: t("workspace_createdTime"),
          dataIndex: "created_at",
          key: "createdAt",
          width: 160,
          sortField: "created_at",
        },
        160,
      ),
      created_by: dataColumn(
        {
          title: t("workspace_createdBy"),
          key: "createdBy",
          width: 100,
          sortField: "created_by",
        },
        100,
      ),
      estimated_work_hours: editableDataColumn(
        {
          title: t("requirement_estimatedWorkHours"),
          dataIndex: "estimated_work_hours",
          key: "estimatedHours",
          width: 150,
          sortField: "estimated_work_hours",
        },
        "estimated_work_hours",
        140,
      ),
      work_progress: dataColumn(
        {
          title: t("requirement_workProgress"),
          dataIndex: "work_progress",
          key: "workProgress",
          width: 90,
          sortField: "work_progress",
        },
        90,
      ),
      completed_work_hours: dataColumn(
        {
          title: t("requirement_completedWorkHours"),
          dataIndex: "completed_work_hours",
          key: "completedHours",
          width: 120,
          sortField: "completed_work_hours",
        },
        110,
      ),
      remaining_work_hours: dataColumn(
        {
          title: t("requirement_remainingWorkHours"),
          dataIndex: "remaining_work_hours",
          key: "remainingHours",
          width: 120,
          sortField: "remaining_work_hours",
        },
        110,
      ),
      overtime_work_hours: dataColumn(
        {
          title: t("requirement_overtimeWorkHours"),
          dataIndex: "overtime_work_hours",
          key: "overtimeHours",
          width: 120,
          sortField: "overtime_work_hours",
        },
        110,
      ),
      sla: dataColumn({ title: t("sla_title"), key: "sla", width: 260 }, 180),
      aging: dataColumn(
        {
          title: t("aging_column"),
          dataIndex: "aging_status",
          key: "aging",
          width: columnWidths.requirement?.aging || 120,
        },
        96,
      ),
    };
    const builtinFieldColumns = systemListFields.value
      .filter((field) => field.kind === "builtin")
      .map((field) => ({
        preferenceKey: field.preferenceKey,
        column: builtinColumns[field.configKey],
      }))
      .filter((field) => field.column);
    const customFieldColumns = (
      customFieldsByType.value[activeTab.value] || []
    ).map((field) => ({
      preferenceKey: `custom:${field.id}`,
      column: editableDataColumn(
        {
          title: field.name,
          key: `custom_${field.id}`,
          width: 140,
          ellipsis: true,
          customField: field,
        },
        `custom_${field.id}`,
        120,
      ),
    }));

    const columnsByPreferenceKey = new Map(
      [...builtinFieldColumns, ...customFieldColumns].map((field) => [
        field.preferenceKey,
        field.column,
      ]),
    );
    const columns = [
      ...effectiveListFields.value
        .filter((field) => field.visible)
        .map((field, index) => {
          const column = columnsByPreferenceKey.get(field.preferenceKey);
          if (!column) return null;
          return {
            ...column,
            preferenceKey: field.preferenceKey,
            fixed: frozenColumnIndex.value >= index ? "left" : undefined,
          };
        })
        .filter(Boolean),
    ];
    const titleColumn = columns.find((column) => column.key === "title");
    if (titleColumn && !columnWidths[type]?.title) {
      const fixedWidth = columns.reduce(
        (total, column) =>
          total + (column.key === "title" ? 0 : Number(column.width) || 0),
        selectionColumnWidth,
      );
      titleColumn.width = Math.min(
        1000,
        Math.max(320, tableAreaWidth.value - fixedWidth),
      );
    }
    return columns;
  });

  const tableScrollWidth = computed(() =>
    Math.max(
      960,
      tableCols.value.reduce(
        (total, column) =>
          total + (Number(column.width) || column.minWidth || 80),
        selectionColumnWidth,
      ),
    ),
  );

  function columnWidth(type, key, fallback) {
    return columnWidths[type]?.[key] ?? fallback;
  }

  function columnWidthStorageKey(type) {
    const userId = authStore.userInfo?.id;
    if (!userId || !wsId.value || !["requirement", "defect"].includes(type)) {
      return "";
    }
    return `goteams:list-column-widths:${userId}:${wsId.value}:${type}`;
  }

  function parseStoredColumnWidths(value) {
    if (!value || typeof value !== "object" || Array.isArray(value)) return {};
    return Object.fromEntries(
      Object.entries(value)
        .map(([key, width]) => [key, Number(width)])
        .filter(([, width]) => Number.isFinite(width) && width > 0),
    );
  }

  function loadColumnWidths() {
    for (const type of ["requirement", "defect"]) {
      columnWidths[type] = {};
      const key = columnWidthStorageKey(type);
      if (!key) continue;
      try {
        const stored = JSON.parse(window.localStorage.getItem(key) || "{}");
        columnWidths[type] = parseStoredColumnWidths(stored);
      } catch {
        // When browser cache is unavailable or corrupted, keep using the default column width.
      }
    }
  }

  function persistColumnWidths(type) {
    const key = columnWidthStorageKey(type);
    if (!key) return;
    try {
      window.localStorage.setItem(
        key,
        JSON.stringify(parseStoredColumnWidths(columnWidths[type])),
      );
    } catch {
      // Limited browser cache should not block the current page's column-width adjustment.
    }
  }

  function handleResizeColumn(width, column) {
    const minWidth = column.minWidth || 80;
    const maxWidth = column.maxWidth || 600;
    columnWidths[activeTab.value][column.key] = Math.min(
      maxWidth,
      Math.max(minWidth, width),
    );
  }

  function startTableResizeGuide(event) {
    const resizeHandle = event.target.closest?.(
      ".requirement-table .ant-table-resize-handle",
    );
    if (!resizeHandle) return;
    const table = resizeHandle
      .closest(".requirement-table")
      ?.querySelector(".ant-table-container");
    if (!table) return;
    tableResizeGuideCleanup?.();
    const rect = table.getBoundingClientRect();
    const headerRect = table
      .querySelector(".ant-table-thead")
      ?.getBoundingClientRect();
    const resizeType = activeTab.value;
    const resizeStorageKey = columnWidthStorageKey(resizeType);
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
      window.requestAnimationFrame(() => {
        if (columnWidthStorageKey(resizeType) === resizeStorageKey) {
          persistColumnWidths(resizeType);
        }
      });
    };
    tableResizeGuideCleanup = stopGuide;
    document.addEventListener("mousemove", moveGuide);
    document.addEventListener("mouseup", stopGuide);
  }

  function fmtDate(d) {
    return formatLocalDate(d);
  }

  let pendingSearchSave = null;
  let searchSaveTimer = null;

  function sanitizeSearchFilters(type, filters) {
    const normalized = normalizeSearchFilterTree(filters, type);
    const descriptors = searchFieldsForType(type);
    const descriptorMap = new Map(
      descriptors.map((field) => [field.field, field]),
    );
    const allowed = new Set(descriptorMap.keys());
    normalized.conditions = normalized.conditions.filter(
      (condition) =>
        condition.field === "title" || allowed.has(condition.field),
    );
    normalized.conditions.forEach((condition) => {
      condition.operator = normalizeSearchConditionOperator(
        condition,
        descriptorMap.get(condition.field),
      );
      condition.values = sanitizeSearchConditionValues(
        type,
        condition,
        descriptorMap.get(condition.field),
      );
    });
    return normalizeSearchFilterTree(normalized, type);
  }

  function normalizeSearchConditionOperator(condition, descriptor) {
    const operator = condition?.operator;
    const defaultOperator =
      descriptor?.operator ||
      VIEW_FILTER_FIELD_OPERATORS[condition?.field]?.[0] ||
      "in";
    const availableOperators = operatorsForSearchField(
      condition?.field,
      descriptor,
    );
    return typeof operator === "string" && availableOperators.includes(operator)
      ? operator
      : defaultOperator;
  }

  function operatorsForSearchField(field, descriptor) {
    if (descriptor?.control === "date") {
      return ["between", "empty", "not_empty"];
    }
    if (descriptor?.control === "number") {
      return NUMERIC_SEARCH_OPERATORS;
    }
    if (descriptor?.control === "cascader") {
      return ["in", "empty", "not_empty"];
    }
    if (descriptor?.control === "text") {
      return ["contains", "not_contains", "empty", "not_empty", "eq", "ne"];
    }
    if (["multi", "person", "select"].includes(descriptor?.control)) {
      return ["in", "not_in", "empty", "not_empty"];
    }
    return VIEW_FILTER_FIELD_OPERATORS[field] || ["in"];
  }

  function uniqueStringValues(values) {
    return [
      ...new Set(
        (Array.isArray(values) ? values : [])
          .map((value) => String(value).trim())
          .filter(Boolean),
      ),
    ];
  }

  function sanitizeSearchConditionValues(type, condition, descriptor) {
    if (["empty", "not_empty"].includes(condition.operator)) return [];
    const rawValues = (
      Array.isArray(condition.values) ? condition.values : []
    ).map((value) => String(value).trim());
    if (condition.operator === "between") {
      return rawValues.slice(0, 2);
    }
    const values = uniqueStringValues(rawValues);
    if (!values.length) return [];
    if (condition.field === "id") {
      const id = Number(values[0]);
      return Number.isInteger(id) && id > 0 ? [String(id)] : [];
    }
    if (condition.field === "template") {
      return values.filter(
        (value) => /^\d+$/.test(value) && Number.isSafeInteger(Number(value)),
      );
    }
    if (descriptor?.control === "person") {
      const candidates =
        descriptor.kind === "custom"
          ? filterMembersForField(descriptor, customFieldMembers.value)
          : members.value;
      const allowed = new Set(
        candidates.map((member) => String(member.user_id)),
      );
      return values.filter((value) => allowed.has(value));
    }
    const allowed = allowedSearchValues(type, condition.field, descriptor);
    const sanitized = allowed
      ? values.filter((value) => allowed.has(value))
      : values;
    return normalizeCascaderFilterValues(descriptor?.fieldType, sanitized);
  }

  function allowedSearchValues(type, field, descriptor) {
    if (field === "status") {
      return new Set([
        VIEW_VALUE_ALL_COMPLETED,
        VIEW_VALUE_ALL_CLOSED,
        VIEW_VALUE_ALL_END,
        VIEW_VALUE_ALL_NOT_END,
        ...statusesForType(type).map((status) => String(status.id)),
      ]);
    }
    if (field === "priority") {
      return new Set([
        "0",
        ...priorityOptionsForType(type).map((priority) => String(priority.id)),
      ]);
    }
    if (field === "version") {
      return new Set([
        "0",
        ...versions.value.map((version) => String(version.id)),
      ]);
    }
    if (field === "category") {
      return new Set(
        searchCategoryOptions().map((option) => String(option.value)),
      );
    }
    if (["select", "multi"].includes(descriptor?.control)) {
      return new Set(
        (descriptor.options || []).map((option) => String(option.value)),
      );
    }
    return null;
  }

  function numericConditionValues(filters, field, allowZero = false) {
    const minimum = allowZero ? 0 : 1;
    return [
      ...new Set(
        (conditionOf(filters, field)?.values || [])
          .map(Number)
          .filter((value) => Number.isInteger(value) && value >= minimum),
      ),
    ];
  }

  function resolvedStatusConditionValues(type, filters) {
    const values = conditionOf(filters, "status")?.values || [];
    const resolved = new Set(
      values
        .map(Number)
        .filter((value) => Number.isInteger(value) && value > 0),
    );
    const finalIds = new Set(
      (finalStatusIds.value[type] || []).map(Number).filter((id) => id > 0),
    );
    if (values.includes(VIEW_VALUE_ALL_COMPLETED)) {
      (completedStatusIds.value[type] || []).forEach((id) => resolved.add(id));
    }
    if (values.includes(VIEW_VALUE_ALL_CLOSED)) {
      const completed = new Set(completedStatusIds.value[type] || []);
      finalIds.forEach((id) => {
        if (!completed.has(id)) resolved.add(id);
      });
    }
    if (values.includes(VIEW_VALUE_ALL_END)) {
      finalIds.forEach((id) => resolved.add(id));
    }
    if (values.includes(VIEW_VALUE_ALL_NOT_END)) {
      statusesForType(type).forEach((status) => {
        const id = Number(status.id);
        if (id > 0 && !finalIds.has(id)) resolved.add(id);
      });
    }
    return [...resolved];
  }

  function applySearchFilters(type, filters, applyTitle) {
    const normalized = sanitizeSearchFilters(type, filters);
    searchFiltersByType.value[type] = normalized;
    if (type !== activeTab.value) return normalized;
    searchText.value = conditionOf(normalized, "title")?.values?.[0] || "";
    if (applyTitle) {
      appliedSearchText.value = searchText.value.trim();
      setAppliedSearchFilters(type, normalized);
    }
    filterStatusActive.value = Boolean(
      conditionOf(normalized, "status")?.values?.length,
    );
    filterStatusIds.value = resolvedStatusConditionValues(type, normalized);
    filterAssignee.value = (conditionOf(normalized, "assignee")?.values || [])
      .map(Number)
      .filter((id) => Number.isInteger(id) && id > 0);
    filterVersionIds.value = numericConditionValues(
      normalized,
      "version",
      true,
    );
    filterCreatorIds.value = numericConditionValues(normalized, "creator");
    filterPriorityIds.value = numericConditionValues(
      normalized,
      "priority",
      true,
    );
    if (type === "requirement")
      setCategorySelection(searchCategoryKey(normalized));
    return normalized;
  }

  function setAppliedSearchFilters(type, filters) {
    appliedSearchFiltersByType.value = {
      ...appliedSearchFiltersByType.value,
      [type]: normalizeSearchFilterTree(filters, type),
    };
  }

  function hydrateSearchState(type) {
    const stored = normalizeSearchFilterTree(
      searchFiltersByType.value[type],
      type,
    );
    let filters = sanitizeSearchFilters(type, stored);
    if (type === "requirement" && route.query.categoryId !== undefined) {
      filters = withSearchCategory(
        filters,
        categoryKeyFromQuery(route.query.categoryId),
      );
    }
    filters = applySearchFilters(type, filters, true);
    if (JSON.stringify(filters) !== JSON.stringify(stored)) {
      saveSearchSettings(type, { filters }).catch(() => undefined);
    }
  }

  function persistSearchFilters(type, filters, deferred) {
    const payload = {
      type,
      filters: sanitizeSearchFilters(type, filters),
    };
    if (deferred) {
      pendingSearchSave = payload;
      clearTimeout(searchSaveTimer);
      searchSaveTimer = setTimeout(flushPendingSearchSave, 400);
      return;
    }
    clearTimeout(searchSaveTimer);
    searchSaveTimer = null;
    pendingSearchSave = null;
    saveSearchSettings(type, { filters: payload.filters }).catch(
      () => undefined,
    );
  }

  function flushPendingSearchSave() {
    clearTimeout(searchSaveTimer);
    searchSaveTimer = null;
    const pending = pendingSearchSave;
    pendingSearchSave = null;
    if (pending) {
      saveSearchSettings(pending.type, { filters: pending.filters }).catch(
        () => undefined,
      );
    }
  }

  function handleSearchFiltersChange(filters, detail) {
    const type = activeTab.value;
    if (
      type === "requirement" &&
      detail.field === "category" &&
      detail.action === "add" &&
      selectedCategoryKey.value !== "all"
    ) {
      filters = withSearchCategory(filters, selectedCategoryKey.value);
    }
    const normalized = applySearchFilters(type, filters, false);
    if (type === "requirement" && detail.field === "category") {
      setCategorySelection(
        detail.action === "remove" ? "all" : searchCategoryKey(normalized),
      );
    }
    const descriptor = searchFieldsForType(type).find(
      (field) => field.field === detail.field,
    );
    const deferSave =
      detail.action === "value" &&
      ["text", "number"].includes(descriptor?.control);
    persistSearchFilters(type, normalized, deferSave);
    if (detail.immediateQuery) {
      if (detail.field === "title" || detail.action === "clear") {
        appliedSearchText.value = searchText.value.trim();
      }
      setAppliedSearchFilters(type, normalized);
      handleFilterChange();
    }
  }

  function toggleSearchBar() {
    saveSearchSettings(activeTab.value, {
      expanded: !currentSearchExpanded.value,
    }).catch(() => undefined);
  }

  function hasAppliedSearchValues() {
    return hasSearchFilterValues(currentAppliedSearchFilters.value);
  }

  async function handleTabChange(key) {
    if (
      surface === "issue" &&
      key !== "issue-invite" &&
      !usesIssuePages(
        workspaceMetadata.value?.workspace?.workspace_type,
        enabledApplications.value,
      )
    ) {
      router.push({
        name: "RequirementBoard",
        params: { wsId: wsId.value },
        query: { tab: key },
      });
      return;
    }
    if (surface === "issue" && !["requirement", "defect"].includes(key)) {
      const name = {
        dashboard: "IssueDashboard",
        knowledge: "IssueKnowledge",
        planning: "IssuePlanning",
        iteration: "IssueIteration",
        gantt: "IssueGantt",
        "issue-invite": "IssueInviteManage",
      }[key];
      if (name) router.push({ name, params: { wsId: wsId.value } });
      return;
    }
    if (
      surface === "issue" &&
      route.name !== "IssueList" &&
      ["requirement", "defect"].includes(key)
    ) {
      router.push({
        name: "IssueList",
        params: { wsId: wsId.value },
        query: { tab: key },
      });
      return;
    }
    const query = { ...route.query, tab: key };
    if (key !== "dashboard") delete query.dashboardId;
    const switching = activateBoardTab(key);
    router.replace({ query });
    await switching;
  }

  async function activateBoardTab(key) {
    const workspaceSequence = workspaceLoadSequence;
    flushPendingSearchSave();
    activeTab.value = key;
    setRememberedTab(authStore.userInfo?.id, boardTabStorageScope(), key);
    page.value = 1;
    selectedRowKeys.value = [];
    kanbanNavigationItems.value = [];
    listRequestSequence += 1;
    requirements.value = [];
    total.value = 0;
    if (!boardContextReady.value) return;
    if (["requirement", "defect"].includes(key)) {
      await Promise.all([ensureViewsLoaded(key), fetchVersions()]);
      if (
        workspaceSequence !== workspaceLoadSequence ||
        activeTab.value !== key
      )
        return;
      hydrateSearchState(key);
    }
    fetchRequirements();
  }

  // The issue header outlives its child routes, including browser back/forward navigation.
  watch(
    () => [route.name, route.query.tab],
    () => {
      if (route.name !== "RequirementBoard" && surface !== "issue") return;
      const next = resolveBoardTab();
      if (next !== activeTab.value) void activateBoardTab(next);
    },
  );

  // ======================== View handlers ========================

  function handleViewSelect(viewId) {
    selectView(activeTab.value, viewId);
    page.value = 1;
    selectedRowKeys.value = [];
    fetchRequirements();
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

  function openSaveCurrentSearch() {
    viewEditMode.value = "saveAs";
    viewEditTarget.value = {
      name: "",
      view_type: "personal",
      filters: normalizeSearchFilterTree(
        currentSearchFilters.value,
        activeTab.value,
      ),
    };
    viewEditOpen.value = true;
  }

  // Reload views and the list after any view create/update, delete, or settings save;
  // the current view's filters may have changed and deletions may retarget the fallback view.
  async function handleViewSaved(savedView) {
    flushPendingSearchSave();
    await saveSearchSettings(activeTab.value, {
      filters: currentSearchFilters.value,
    }).catch(() => undefined);
    await loadViews(activeTab.value);
    if (
      savedView?.id != null &&
      ["create", "saveAs"].includes(savedView.mode) &&
      findView(activeTab.value, savedView.id)?.enabled
    ) {
      selectView(activeTab.value, String(savedView.id));
    }
    hydrateSearchState(activeTab.value);
    page.value = 1;
    fetchRequirements();
  }

  // Deletion emits the removed view id, which must NOT be selected; settings save emits none.
  const handleViewDeleted = () => handleViewSaved();
  const handleViewSettingsSaved = () => handleViewSaved();

  function normalizeBoardTabQuery() {
    if (options.activeTab) return;
    if (surface === "issue" && route.name !== "IssueList") return;
    const shouldRemoveDashboardId =
      activeTab.value !== "dashboard" && route.query.dashboardId !== undefined;
    if (
      (route.query.tab === undefined ||
        requestedBoardTab() === activeTab.value) &&
      !shouldRemoveDashboardId
    ) {
      return;
    }
    const query = { ...route.query, tab: activeTab.value };
    if (shouldRemoveDashboardId) delete query.dashboardId;
    router.replace({ query });
  }

  // App tabs depend on workspace metadata. Dashboard additionally needs its visible list, so this
  // runs only after both are ready and allows deep links / remembered tabs to resolve once.
  function applyRequestedBoardTab() {
    const next = resolveBoardTab();
    if (!availableBoardTabs().includes(next) || activeTab.value === next)
      return;
    activeTab.value = next;
  }

  function handleFilterChange() {
    page.value = 1;
    selectedRowKeys.value = [];
    fetchRequirements();
  }

  function handleSearch() {
    appliedSearchText.value = searchText.value.trim();
    setAppliedSearchFilters(activeTab.value, currentSearchFilters.value);
    page.value = 1;
    selectedRowKeys.value = [];
    fetchRequirements();
  }

  function handleListPageChange(nextPage, nextPageSize) {
    const sizeChanged = Number(nextPageSize) !== Number(pageSize.value);
    pageSize.value = nextPageSize;
    page.value = sizeChanged ? 1 : nextPage;
    selectedRowKeys.value = [];
    fetchRequirements();
  }

  function createInStatus(statusId) {
    router.push({
      ...workItemRoute(surface, "create", wsId.value),
      query: { type: activeTab.value, statusId: String(statusId) },
    });
  }

  function handleKanbanUpdated() {
    fetchVersions();
  }

  // Inline edit helper functions.
  function isEditing(record, field) {
    return (
      editingCell.value &&
      editingCell.value.id === record.id &&
      editingCell.value.field === field
    );
  }
  function editableCellProps(record, field, customField) {
    if (record.__loadMore) return {};
    const configKey =
      { status_id: "status", version_id: "version", assignee_ids: "assignee" }[
        field
      ] || field;
    const editable =
      canEditRecord(record) && fieldEditable(record.type, configKey);
    if (!editable) return {};
    return {
      class: "editable-table-cell",
      onClick: () => startCellEdit(record, field, customField),
    };
  }

  function startCellEdit(record, field, customField) {
    if (isEditing(record, field)) return;
    if (field === "status_id") {
      startStatusEdit(record);
      return;
    }
    startEdit(record, field, customField);
  }

  function startEdit(record, field, customField) {
    if (!canEditRecord(record)) return false;
    const configKey =
      { status_id: "status", version_id: "version", assignee_ids: "assignee" }[
        field
      ] || field;
    if (!fieldEditable(record.type, configKey)) return false;
    let original = record[field];
    if (customField) original = customFieldValue(record, customField);
    editingCell.value = {
      id: record.id,
      field,
      customField,
      original,
      draft: cloneEditValue(original, customField),
    };
    return true;
  }

  function customFieldValue(record, field) {
    return parseCustomFieldValue(
      field,
      customFieldValues(record.custom_fields)[String(field.id)],
    );
  }

  function cloneEditValue(value, field) {
    if (Array.isArray(value))
      return value.map((item) => (Array.isArray(item) ? [...item] : item));
    return value;
  }

  async function startStatusEdit(record) {
    if (!fieldEditable(record.type, "status")) return;
    if (
      isStatusTransitionEnabled(workspaceMetadata.value?.settings, record.type)
    ) {
      transitionItem.value = record;
      transitionOpen.value = true;
      return;
    }
    try {
      const latest = await getRequirement(wsId.value, record.id);
      record.available_status_ids = latest.available_status_ids;
      startEdit(record, "status_id");
    } catch {
      /* handled by interceptor */
    }
  }

  function handleStatusTransitionSuccess(patch) {
    if (!transitionItem.value) return;
    patchLoadedWorkItem(
      requirements.value,
      childrenMap.value,
      transitionItem.value.id,
      patch,
    );
  }
  function clearEditing() {
    editingCell.value = null;
  }

  function cancelEditing() {
    clearEditing();
  }

  function getRowClassName(record) {
    if (record.__loadMore) return "load-more-row";
    return [
      record.__isChild ? "child-row" : "",
      getFinalStatusRowClassName(record),
      getDetailSelectedRowClassName(record),
    ]
      .filter(Boolean)
      .join(" ");
  }

  function getFinalStatusRowClassName(record) {
    return isFinalStatus(record) ? "final-status-row" : "";
  }

  function getDetailSelectedRowClassName(record) {
    return listPreferences.previewMode === "drawer" &&
      String(record.id) === String(detailReqId.value)
      ? "detail-selected-row"
      : "";
  }

  function getVersionRowClassName(record) {
    return [
      getFinalStatusRowClassName(record),
      getDetailSelectedRowClassName(record),
    ]
      .filter(Boolean)
      .join(" ");
  }

  // Expand/collapse child requirements (TAPD style).
  function hasChildren(record) {
    const loaded = childrenMap.value[record.id];
    if (loaded) return loaded.total > 0;
    return [true, 1, "1", "true", "t"].includes(record.has_children);
  }

  function isChildLoading(id) {
    return childLoadingKeys.value.has(String(id));
  }

  function unloadedChildParentIds(items) {
    return [
      ...new Set(
        (items || [])
          .filter((item) => hasChildren(item) && !childrenMap.value[item.id])
          .map((item) => item.id),
      ),
    ];
  }

  // ponytail: one request per parent; batch parent_ids if a page has many trees.
  async function prefetchChildRequirements(items) {
    const parentIds = unloadedChildParentIds(items);
    if (!parentIds.length) return;
    await Promise.all(parentIds.map((id) => loadChildren(id)));
  }

  async function loadChildren(parentId, childPage = 1) {
    const promiseKey = `${parentId}:${childPage}`;
    const inflight = childLoadPromises.get(promiseKey);
    if (inflight) return inflight;

    const promise = (async () => {
      const key = String(parentId);
      const contextSequence = listContextSequence;
      const loadToken = ++childLoadSequence;
      childLoadTokens.set(key, loadToken);
      childLoadingKeys.value = new Set(childLoadingKeys.value).add(key);
      try {
        const params = {
          page: childPage,
          size: 100,
          parent_id: parentId,
          type: "requirement",
        };
        if (currentViewId.value) params.view_id = currentViewId.value;
        if (currentListSort.value.field) {
          params.sort_field = currentListSort.value.field;
          params.sort_direction = currentListSort.value.direction;
        }
        const categoryId = categoryFilterParam();
        if (categoryId !== undefined) params.category_id = categoryId;
        const res = await listRequirements(wsId.value, params);
        if (contextSequence !== listContextSequence) return false;
        const current = childrenMap.value[parentId];
        const nextList = childPage > 1 ? [...(current?.list || [])] : [];
        const seen = new Set(nextList.map((item) => String(item.id)));
        for (const item of res?.list || []) {
          if (!seen.has(String(item.id))) nextList.push(item);
        }
        childrenMap.value = {
          ...childrenMap.value,
          [parentId]: {
            list: nextList,
            total: Number(res?.total) || 0,
            page: Number(res?.page) || childPage,
          },
        };
        return true;
      } catch {
        return false;
      } finally {
        if (childLoadTokens.get(key) === loadToken) {
          childLoadTokens.delete(key);
          const nextLoading = new Set(childLoadingKeys.value);
          nextLoading.delete(key);
          childLoadingKeys.value = nextLoading;
        }
      }
    })();
    childLoadPromises.set(promiseKey, promise);
    promise.finally(() => {
      if (childLoadPromises.get(promiseKey) === promise) {
        childLoadPromises.delete(promiseKey);
      }
    });
    return promise;
  }

  async function toggleExpand(record) {
    allTreeExpanded.value = false;
    const idx = expandedRowKeys.value.indexOf(record.id);
    if (idx >= 0) {
      expandedRowKeys.value = expandedRowKeys.value.filter(
        (id) => id !== record.id,
      );
      const hiddenIds = collectLoadedDescendantIds(record.id);
      const matchedIds = new Set(
        requirements.value.map((item) => String(item.id)),
      );
      selectedRowKeys.value = selectedRowKeys.value.filter(
        (id) => matchedIds.has(String(id)) || !hiddenIds.has(String(id)),
      );
      return;
    }
    if (!childrenMap.value[record.id]) {
      const loaded = await loadChildren(record.id);
      if (!loaded) return;
    }
    expandedRowKeys.value = [...expandedRowKeys.value, record.id];
    void prefetchChildRequirements(childrenMap.value[record.id]?.list || []);
  }

  async function expandAllTreeRows() {
    if (activeTab.value !== "requirement" || loading.value) return;
    const contextSequence = listContextSequence;
    const expandedIds = new Set();
    const queuedIds = new Set();
    const queue = requirements.value
      .filter((item) => hasChildren(item))
      .map((item) => item.id);

    while (queue.length) {
      const parentId = queue.shift();
      const key = String(parentId);
      if (queuedIds.has(key)) continue;
      queuedIds.add(key);
      const loaded = await loadChildren(parentId);
      if (contextSequence !== listContextSequence || !loaded) continue;
      let state = childrenMap.value[parentId];
      while (state && state.list.length < state.total) {
        await loadChildren(parentId, state.page + 1);
        if (contextSequence !== listContextSequence) return;
        state = childrenMap.value[parentId];
      }
      expandedIds.add(parentId);
      for (const child of state?.list || []) {
        if (hasChildren(child)) queue.push(child.id);
      }
    }
    if (contextSequence !== listContextSequence) return;
    expandedRowKeys.value = [...expandedIds];
    allTreeExpanded.value = true;
  }

  function collapseAllTreeRows() {
    const hiddenIds = new Set();
    for (const parentId of expandedRowKeys.value) {
      collectLoadedDescendantIds(parentId, hiddenIds);
    }
    const matchedIds = new Set(
      requirements.value.map((item) => String(item.id)),
    );
    selectedRowKeys.value = selectedRowKeys.value.filter(
      (id) => matchedIds.has(String(id)) || !hiddenIds.has(String(id)),
    );
    expandedRowKeys.value = [];
    allTreeExpanded.value = false;
  }

  function toggleAllTreeRows() {
    if (allTreeExpanded.value) {
      collapseAllTreeRows();
      return;
    }
    void expandAllTreeRows();
  }

  async function loadMoreChildren(parentId) {
    const state = childrenMap.value[parentId];
    if (!state || state.list.length >= state.total) return;
    await loadChildren(parentId, state.page + 1);
    void prefetchChildRequirements(childrenMap.value[parentId]?.list || []);
  }

  function collectLoadedDescendantIds(parentId, result = new Set()) {
    for (const child of childrenMap.value[parentId]?.list || []) {
      const childId = String(child.id);
      if (result.has(childId)) continue;
      result.add(childId);
      collectLoadedDescendantIds(child.id, result);
    }
    return result;
  }
  const titleDraft = ref("");
  const titleInputRefs = new Map();
  function setTitleInputRef(recordId, element) {
    const key = String(recordId);
    if (element) titleInputRefs.set(key, element);
    else titleInputRefs.delete(key);
  }
  function startEditTitle(record) {
    titleDraft.value = String(record.title || "");
    if (!startEdit(record, "title")) return;
    nextTick(() => {
      const input = titleInputRefs.get(String(record.id));
      input?.focus();
      input?.select();
    });
  }
  async function saveInlineTitle(record) {
    if (!isEditing(record, "title")) return;
    const v = titleDraft.value.trim();
    if (!v || v === record.title) {
      clearEditing();
      return;
    }
    await saveInline(record, "title", v);
  }
  async function saveInline(record, field, value) {
    if (!isEditing(record, field)) return;
    clearEditing();
    if (String(record[field]) === String(value)) return;
    try {
      const updatedValues = { [field]: value };
      await updateRequirement(wsId.value, record.id, updatedValues);
      if (field === "status_id") {
        const status = statusesForType(record.type).find(
          (item) => item.id === Number(value),
        );
        updatedValues.status_name = status?.name || "";
        updatedValues.status_color = status?.color || "";
      }
      if (field === "priority") {
        const priority = priorityOptionsForType(record.type).find(
          (item) => item.id === Number(value),
        );
        updatedValues.priority_name = priority?.label || "";
        updatedValues.priority_color = priority?.color || "";
      }
      if (field === "assignee_ids") {
        const ids = Array.isArray(value) ? value : [];
        updatedValues.assignee_ids = ids;
        const names = [];
        const avatars = [];
        const usernames = [];
        ids.forEach((id) => {
          const member =
            members.value.find((m) => Number(m.user_id) === Number(id)) || {};
          names.push(member.user_name || "");
          avatars.push(member.user_avatar || "");
          usernames.push(member.user_username || "");
        });
        updatedValues.assignee_names = names;
        updatedValues.assignee_avatars = avatars;
        updatedValues.assignee_usernames = usernames;
      }
      if (field === "version_id") {
        updatedValues.version_name =
          (versions.value.find((v) => v.id === Number(value)) || {}).name || "";
      }
      patchLoadedWorkItem(
        requirements.value,
        childrenMap.value,
        record.id,
        updatedValues,
      );
      if (field === "priority" && slaEnabled.value) await loadSLASummaries();
      if (field === "estimated_work_hours") await fetchRequirements();
      if (field === "version_id" || field === "status_id")
        await fetchVersions();
      if (field === "assignee_ids")
        await contactPreferences.recordRecent(value);
      showWorkItemUpdateSuccess(t("workspace_updateSuccess"));
    } catch {
      /* handled by interceptor; record still contains the original value */
    }
  }

  // Save assignees on every change (AssigneeSelect "change-to-save" mode).
  // Do NOT clearEditing here, otherwise the component unmounts and the
  // dropdown disappears while saving. The edit session ends when the user
  // clicks outside (handled by AssigneeSelect @cancel -> cancelEditing).
  async function saveAssigneesInline(record, value) {
    if (!isEditing(record, "assignee_ids")) return;
    if (String(record.assignee_ids) === String(value)) return;
    const ids = Array.isArray(value) ? value : [];
    try {
      const updatedValues = { assignee_ids: ids };
      const names = [];
      const avatars = [];
      const usernames = [];
      ids.forEach((id) => {
        const member =
          members.value.find((m) => Number(m.user_id) === Number(id)) || {};
        names.push(member.user_name || "");
        avatars.push(member.user_avatar || "");
        usernames.push(member.user_username || "");
      });
      updatedValues.assignee_names = names;
      updatedValues.assignee_avatars = avatars;
      updatedValues.assignee_usernames = usernames;
      await updateRequirement(wsId.value, record.id, updatedValues);
      patchLoadedWorkItem(
        requirements.value,
        childrenMap.value,
        record.id,
        updatedValues,
      );
      // Recent-contact ordering is deferred to dropdown close inside
      // AssigneeSelect, so the list order does not shift during the
      // current open session.
      showWorkItemUpdateSuccess(t("workspace_updateSuccess"));
    } catch {
      /* handled by interceptor; record still contains the original value */
    }
  }

  async function saveInlineCustomField(record, field) {
    const fieldKey = "custom_" + field.id;
    if (!isEditing(record, fieldKey)) return;
    const original = editingCell.value.original;
    if (!isValidPhoneFieldValue(field, editingCell.value.draft)) {
      message.warning(t("settings_fieldPhoneInvalid"));
      return;
    }
    const value = normalizeCustomFieldDraft(field, editingCell.value.draft);
    clearEditing();
    if (
      JSON.stringify(normalizeCustomFieldDraft(field, original)) ===
      JSON.stringify(value)
    )
      return;
    try {
      await updateRequirement(wsId.value, record.id, {
        custom_fields: { [field.id]: value },
      });
      patchLoadedWorkItem(requirements.value, childrenMap.value, record.id, {
        custom_fields: {
          ...customFieldValues(record.custom_fields),
          [field.id]: storedCustomFieldValue(field, value),
        },
      });
      showWorkItemUpdateSuccess(t("workspace_updateSuccess"));
    } catch {
      /* handled by interceptor; record still contains the original value */
    }
  }

  function normalizeCustomFieldDraft(field, value) {
    return serializeCustomFieldValue(field, value);
  }

  function handleRowAction(key, record) {
    if (key === "child")
      router.push({
        ...workItemRoute(surface, "create", wsId.value),
        query: {
          parentId: record.id,
          type: "requirement",
          ...currentCategoryCreateQuery(),
        },
      });
    if (key === "duplicate") handleCopyItem(record);
    if (
      key === "copyTitleLink" ||
      key === "copyTitle" ||
      key === "copyLink" ||
      key === "copyDescription" ||
      key === "copyTitleLinkDescription"
    ) {
      handleListCopyAction(record, key);
    }
    if (key === "delete") {
      deleteTarget.value = { kind: "workItem", record };
      deleteConfirmOpen.value = true;
    }
  }

  // "复制需求" opens the create page prefilled from the source work item so the user
  // can review the new copy and change the type or project before saving it.
  function handleCopyItem(record) {
    if (!canDuplicateRecord(record)) return;
    router.push({
      ...workItemRoute(surface, "create", wsId.value),
      query: { copyFrom: record.id, type: record.type },
    });
  }

  function handleListCopyAction(record, key) {
    if (!record) return;
    return copyWorkItemInfo(record, key);
  }

  function handleDeleteConfirmOpen(open) {
    if (deletingTarget.value) return;
    deleteConfirmOpen.value = open;
    if (!open) deleteTarget.value = null;
  }

  async function confirmDeleteTarget() {
    const target = deleteTarget.value;
    if (!target) return;
    deletingTarget.value = true;
    try {
      if (target.kind === "batch") {
        const result = await batchDeleteRequirements(wsId.value, {
          ids: target.ids,
        });
        message.success(
          t("workspace_batchDeleted", {
            n: Number(result?.deleted_count) || target.ids.length,
          }),
        );
        selectedRowKeys.value = [];
        await Promise.all([
          fetchRequirements(),
          refreshRequirementCategories(),
        ]);
      } else {
        await deleteRequirement(wsId.value, target.record.id);
        message.success(t("workspace_deleteSuccess"));
        fetchRequirements();
        refreshRequirementCategories();
      }
      deleteConfirmOpen.value = false;
      deleteTarget.value = null;
    } catch {
      deleteConfirmOpen.value = false;
      deleteTarget.value = null;
    } finally {
      deletingTarget.value = false;
    }
  }

  // Copying writes the title, link, and/or description to the clipboard — see handleListCopyAction.

  async function handleBatchExport(
    selectedOnly = selectedRowKeys.value.length > 0,
  ) {
    const params = {
      type: activeTab.value,
      root_only:
        !selectedOnly &&
        activeTab.value === "requirement" &&
        !hasAppliedSearchValues()
          ? 1
          : 0,
    };
    if (selectedOnly)
      params.ids = uniqueWorkItemIds(selectedRowKeys.value).join(",");
    if (filterStatusIds.value.length)
      params.status_ids = filterStatusIds.value.join(",");
    if (filterAssignee.value.length)
      params.assignee_ids = filterAssignee.value.join(",");
    if (filterVersionIds.value.length)
      params.version_ids = filterVersionIds.value.join(",");
    if (filterCreatorIds.value.length)
      params.created_by_ids = filterCreatorIds.value.join(",");
    if (filterPriorityIds.value.length)
      params.priority_ids = filterPriorityIds.value.join(",");
    if (appliedSearchText.value) params.title_search = appliedSearchText.value;
    params.search_filters = JSON.stringify(currentAppliedSearchFilters.value);
    params.timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
    try {
      const response = await exportWorkItems(wsId.value, params);
      const disposition = response.headers?.["content-disposition"] || "";
      const match = disposition.match(/filename\*=UTF-8''([^;]+)/i);
      const filename = match
        ? decodeURIComponent(match[1])
        : `${t("requirement_title")}.xlsx`;
      const url = URL.createObjectURL(response.data);
      const link = document.createElement("a");
      link.href = url;
      link.download = filename;
      link.click();
      URL.revokeObjectURL(url);
      message.success(t("workspace_exported"));
      if (selectedOnly) selectedRowKeys.value = [];
    } catch {
      /* handled by interceptor */
    }
  }

  function handleMoreMenu(key) {
    if (key === "import") showImportModal.value = true;
    if (key === "export") handleBatchExport(false);
  }

  async function handleBatchUpdate(field, value) {
    if (!canEditActiveWorkItem.value) return;
    const ids = uniqueWorkItemIds(selectedRowKeys.value);
    if (!ids.length) return;
    if (ids.length > 500) {
      message.error(t("workspace_batchLimit"));
      return;
    }
    const hide = message.loading(t("common_processing"), 0);
    try {
      await batchUpdateRequirements(wsId.value, {
        ids,
        field,
        value: Number(value),
      });
      hide();
      message.success(t("workspace_batchModified", { n: ids.length }));
      selectedRowKeys.value = [];
      fetchRequirements();
    } catch {
      hide();
    }
  }

  function handleBatchMenu(key) {
    if (key === "export") {
      handleBatchExport(true);
      return;
    }
    if (key === "delete") {
      if (!canDeleteActiveWorkItem.value) return;
      const ids = uniqueWorkItemIds(selectedRowKeys.value);
      if (!ids.length) return;
      if (ids.length > 500) {
        message.error(t("workspace_batchLimit"));
        return;
      }
      deleteTarget.value = { kind: "batch", ids };
      deleteConfirmOpen.value = true;
      return;
    }
    if (["copyTitleLink", "copyTitle", "copyLink"].includes(key)) {
      if (!canCopyActiveWorkItem.value) return;
      const selected = new Set(selectedRowKeys.value.map(String));
      copyWorkItemsInfo(
        treeList.value.filter(
          (record) => !record.__loadMore && selected.has(String(record.id)),
        ),
        key,
      );
      return;
    }
    const [field, value] = key.split(":");
    if (field === "assignee_ids") {
      batchAssigneeIds.value = [];
      batchAssigneeOpen.value = true;
      return;
    }
    handleBatchUpdate(field, value);
  }

  async function submitBatchAssignees() {
    if (!canEditActiveWorkItem.value) return;
    const ids = uniqueWorkItemIds(selectedRowKeys.value);
    if (!ids.length) return;
    if (ids.length > 500) {
      message.error(t("workspace_batchLimit"));
      return;
    }
    batchAssigneeSaving.value = true;
    try {
      await batchUpdateRequirements(wsId.value, {
        ids,
        field: "assignee_ids",
        value: batchAssigneeIds.value,
      });
      message.success(t("workspace_batchModified", { n: ids.length }));
      selectedRowKeys.value = [];
      batchAssigneeOpen.value = false;
      batchAssigneeIds.value = [];
      await Promise.all([fetchRequirements(), refreshRequirementCategories()]);
    } finally {
      batchAssigneeSaving.value = false;
    }
  }

  function dateValue(value) {
    const timestamp = Number(value);
    return timestamp > 0 ? dayjs.unix(timestamp) : null;
  }

  function toPlannedDateTimestamp(dateString) {
    if (!dateString) return 0;
    const date = dayjs(dateString);
    return date.isValid() ? Number(date.startOf("day").unix()) : 0;
  }

  // Detail drawer.
  function openDetailInNewWindow(
    id,
    navigationItems = [],
    openWorkLogEditor = false,
    tab,
  ) {
    const navigationContext = saveWorkItemNavigationContext(navigationItems);
    const target = router.resolve({
      name: surface === "issue" ? "IssueDetail" : "WorkItemDetail",
      params: { wsId: wsId.value, id },
      query: {
        fromTab: activeTab.value,
        ...(surface === "issue" ? { returnTo: route.fullPath } : {}),
        ...(navigationContext ? { navContext: navigationContext } : {}),
        ...(openWorkLogEditor ? { workLogEditor: "1" } : {}),
        ...(tab ? { detailTab: tab } : {}),
      },
    });
    window.open(target.href, "_blank", "noopener,noreferrer");
  }

  async function openSharedDetail(id) {
    if (
      String(route.query.detail || "") !== String(id) ||
      (surface === "issue" && route.name !== "IssueList")
    )
      return;
    const workspaceSequence = workspaceLoadSequence;
    const sourceRoute = route.fullPath;
    let workItem;
    try {
      workItem = await getRequirement(wsId.value, id);
    } catch {
      return;
    }
    if (
      workspaceSequence !== workspaceLoadSequence ||
      route.fullPath !== sourceRoute
    )
      return;
    const type = workItem?.type;
    if (!["requirement", "defect"].includes(type)) return;
    activeTab.value = type;
    setRememberedTab(authStore.userInfo?.id, boardTabStorageScope(), type);
    if (listPreferences.previewMode === "window") {
      await router.replace({
        name: surface === "issue" ? "IssueDetail" : "WorkItemDetail",
        params: { wsId: wsId.value, id },
        query: { fromTab: type },
      });
      return true;
    }
    detailReqId.value = id;
    return false;
  }

  // The drawer emits navigate(id, workspaceId); workspaceId is never a tab key.
  // Forwarding it into openDetail used to write the workspace id into detailTab,
  // leaving the detail tabs with no active pane after prev/next navigation.
  function handleDrawerNavigate(id) {
    openDetail(id);
  }

  // Derived conclusion written by the SLA worker: 0=not overdue, 1=due soon, 2=overdue, so the
  // larger value always wins when the cell is rendered.
  function agingTagClass(status) {
    if (Number(status) === 2) return "aging-tag-overdue";
    if (Number(status) === 1) return "aging-tag-warning";
    return "aging-tag-normal";
  }

  function agingTagText(status) {
    if (Number(status) === 2) return t("aging_overdue");
    if (Number(status) === 1) return t("aging_soon");
    return t("aging_normal");
  }

  function openDetail(id, tab) {
    const query = { ...route.query, detail: String(id) };
    if (tab) {
      query.detailTab = tab;
    } else {
      delete query.detailTab;
    }
    if (listPreferences.previewMode === "window") {
      openDetailInNewWindow(id, boardNavigationItems.value, false, tab);
      return;
    }
    detailReqId.value = id;
    router.replace({ query });
  }

  function openWorkLogEditor(record) {
    if (
      record.type !== "requirement" ||
      !fieldEditable(record.type, "completed_work_hours")
    )
      return;
    directWorkLogRequirementId.value = String(record.id);
    nextTick(() => {
      directWorkLogsRef.value?.openCreate();
    });
  }

  function handleDirectWorkLogSummaryChange(summary) {
    const requirementId = String(directWorkLogRequirementId.value || "");
    if (!requirementId || !summary) return;
    const apply = (item) => {
      if (String(item.id) !== requirementId) return;
      Object.assign(item, summary);
    };
    requirements.value.forEach(apply);
    Object.values(childrenMap.value).forEach((state) =>
      state.list.forEach(apply),
    );
  }

  function formatWorkHours(value) {
    const numeric = Number(value) || 0;
    const text = numeric
      .toFixed(2)
      .replace(/\.00$/, "")
      .replace(/(\.\d)0$/, "$1");
    return `${text}${t("requirement_workHoursUnit")}`;
  }

  function formatWorkHoursValue(value) {
    const numeric = Number(value) || 0;
    return numeric
      .toFixed(2)
      .replace(/\.00$/, "")
      .replace(/(\.\d)0$/, "$1");
  }

  function closeDetail() {
    detailReqId.value = null;
    if (route.query.detail) {
      const query = { ...route.query };
      delete query.detail;
      delete query.detailTab;
      router.replace({ query });
    }
  }

  function removeLoadedWorkItem(id) {
    let removedFromRoot = false;
    requirements.value = requirements.value.filter((item) => {
      if (String(item.id) !== String(id)) return true;
      removedFromRoot = true;
      return false;
    });
    Object.values(childrenMap.value).forEach((state) => {
      const previousLength = state.list.length;
      state.list = state.list.filter((item) => String(item.id) !== String(id));
      if (state.list.length < previousLength) {
        state.total = Math.max(0, state.total - 1);
      }
    });
    return removedFromRoot;
  }

  function patchRequirementList(change) {
    let current;
    let currentChildState;
    const find = (item) => {
      if (String(item.id) === String(change.id)) current = item;
    };
    requirements.value.forEach(find);
    Object.values(childrenMap.value).forEach((state) => {
      if (state.list.some((item) => String(item.id) === String(change.id))) {
        currentChildState = state;
      }
      state.list.forEach(find);
    });
    if (!current) return;

    const previous = { ...current };
    const wasInRoot = requirements.value.some(
      (item) => String(item.id) === String(change.id),
    );
    mergeWorkItem(current, change.item);

    const filters = {
      search: isWorkItemTab.value ? appliedSearchText.value : "",
      statusActive: isWorkItemTab.value && filterStatusActive.value,
      statusIds: isWorkItemTab.value ? filterStatusIds.value : [],
      assigneeIds: isWorkItemTab.value ? filterAssignee.value : [],
      versionIds: filterVersionIds.value,
      createdByIds: isWorkItemTab.value ? filterCreatorIds.value : [],
      priorityIds: isWorkItemTab.value ? filterPriorityIds.value : [],
      searchFilters: isWorkItemTab.value
        ? JSON.stringify(currentAppliedSearchFilters.value)
        : "",
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
    };
    const matches =
      current.type === activeTab.value &&
      workItemMatchesFilters(current, filters);
    const usesRootRows =
      activeTab.value === "requirement" &&
      !hasAppliedSearchValues() &&
      !viewHasActiveFilters.value;
    const shouldBeRoot =
      matches && (!usesRootRows || !Number(current.parent_id));
    const parentState = childrenMap.value[current.parent_id];

    if (wasInRoot && shouldBeRoot) return previous;
    if (
      !wasInRoot &&
      matches &&
      usesRootRows &&
      currentChildState === parentState
    ) {
      return previous;
    }
    removeLoadedWorkItem(change.id);

    if (shouldBeRoot) requirements.value.unshift(current);
    else if (matches && usesRootRows && parentState) {
      parentState.list.unshift(current);
      parentState.total += 1;
    }
    if (wasInRoot && !shouldBeRoot) total.value = Math.max(0, total.value - 1);
    if (!wasInRoot && shouldBeRoot) total.value += 1;
    selectedRowKeys.value = selectedRowKeys.value.filter(
      (id) => String(id) !== String(change.id) || shouldBeRoot || !!parentState,
    );
    return previous;
  }

  function adjustVersionStats(change, previous) {
    if (!previous || !change.item) return;
    const oldVersionId = Number(previous.version_id);
    const newVersionId = Number(change.item.version_id);
    const oldDone = completedStatusIds.value[previous.type]?.includes(
      Number(previous.status_id),
    );
    const newDone =
      completedStatusIds.value[change.item.type]?.includes(
        Number(change.item.status_id),
      ) || false;
    const oldVersion = versions.value.find(
      (version) => Number(version.id) === oldVersionId,
    );
    const newVersion = versions.value.find(
      (version) => Number(version.id) === newVersionId,
    );

    if (oldVersionId !== newVersionId) {
      if (oldVersion) {
        oldVersion.requirement_count = Math.max(
          0,
          oldVersion.requirement_count - 1,
        );
        if (oldDone) {
          oldVersion.completed_count = Math.max(
            0,
            oldVersion.completed_count - 1,
          );
        }
      }
      if (newVersion) {
        newVersion.requirement_count += 1;
        if (newDone) newVersion.completed_count += 1;
      }
      return;
    }
    if (oldVersion && oldDone !== newDone) {
      oldVersion.completed_count = Math.max(
        0,
        oldVersion.completed_count + (newDone ? 1 : -1),
      );
    }
  }

  async function handleDetailUpdated(change) {
    if (change?.changedFields?.includes("category_id")) {
      if (Number(change.workspaceId) === Number(wsId.value)) {
        await refreshAfterCategoryAssignment();
      }
      return;
    }
    if (!change || change.action === "refresh") {
      if (listPreferences.viewMode === "board" && isWorkItemTab.value) {
        kanbanRef.value?.reload();
        await Promise.all([fetchVersions(), refreshRequirementCategories()]);
        return;
      }
      await Promise.all([
        fetchRequirements(),
        fetchVersions(),
        refreshRequirementCategories(),
      ]);
      return;
    }
    let previous;
    if (listPreferences.viewMode === "board" && isWorkItemTab.value) {
      previous = kanbanRef.value?.applyItemUpdate(change);
    } else {
      previous = patchRequirementList(change);
    }
    if (
      change.changedFields?.some((field) =>
        ["status_id", "version_id"].includes(field),
      )
    ) {
      adjustVersionStats(change, previous);
    }
    if (change.changedFields?.includes("assignee_ids")) {
      await refreshRequirementCategories();
    }
  }

  return {
    canViewRequirement,
    canViewDefect,
    t,
    route,
    router,
    authStore,
    wsId,
    workItemAttributes,
    selectedCategoryKey,
    categorySidebarCollapsed,
    categorySidebarWidth,
    requirementCategories,
    categorySidebarRef,
    requirementCategoryName,
    categoryEditTarget,
    categoryDraft,
    categorySaving,
    categoryOptions,
    canAssignCategory,
    openCategoryEditor,
    closeCategoryEditor,
    confirmCategoryAssignment,
    enabledApplications,
    boardTabs,
    activeTab,
    boardTabTitle,
    loading,
    boardContextReady,
    page,
    pageSize,
    total,
    selectedRowKeys,
    editingCell,
    expandedRowKeys,
    allTreeExpanded,
    detailReqId,
    workLogEditorToken,
    workLogEditorRequirementId,
    directWorkLogRequirementId,
    directWorkLogsRef,
    showImportModal,
    batchAssigneeOpen,
    batchAssigneeIds,
    batchAssigneeSaving,
    deleteConfirmOpen,
    deletingTarget,
    listSettingsOpen,
    showListSettingsNotice,
    listPreferences,
    columnDragState,
    openColumnMenuKey,
    currentListSort,
    kanbanRef,
    kanbanNavigationItems,
    openListSettings,
    selectView,
    viewEditOpen,
    viewEditMode,
    viewEditTarget,
    viewConfigOpen,
    deleteConfirmTitle,
    deleteConfirmContent,
    deleteConfirmOkText,
    boardPageRef,
    tableResizeGuide,
    members,
    customFieldMembers,
    assignableMembers,
    versions,
    finalStatusIds,
    workspaceMetadata,
    transitionOpen,
    transitionItem,
    workspaceName,
    projectEnded,
    projectIcon,
    projectColor,
    projectChar,
    iconFgColor,
    canViewProjectSettings,
    canManageRequirementCategories,
    isWorkItemTab,
    visibleTopTabs,
    canCreateActiveWorkItem,
    canEditActiveWorkItem,
    canDeleteActiveWorkItem,
    canCopyActiveWorkItem,
    canImportActiveWorkItem,
    canExportActiveWorkItem,
    canUseBatchActions,
    statusOptions,
    priorityOptions,
    currentTabViews,
    currentViewId,
    currentTabRememberLast,
    currentSearchExpanded,
    currentSearchFilters,
    currentSearchHasValues,
    searchFieldDescriptors,
    canManageSystemView,
    kanbanFilters,
    kanbanEditableFields,
    kanbanFieldEditability,
    priorityOptionsForType,
    statusesForRecord,
    searchFieldsForType,
    fieldEditable,
    canCreateType,
    selectRequirementCategory,
    setCategorySidebarCollapsed,
    setCategorySidebarWidth,
    persistCategorySidebarWidth,
    goCreateActiveWorkItem,
    goCreateInCategory,
    handleRequirementCategoryChanged,
    handleWorkItemsImported,
    rowSelection,
    systemListFields,
    hasFrozenColumns,
    startColumnDrag,
    updateColumnDropTarget,
    clearColumnDrag,
    finishColumnDrop,
    cycleColumnSort,
    columnSortIcon,
    handleColumnMenu,
    canFreezeColumn,
    canHideColumn,
    handleColumnMenuOpenChange,
    applyListPreferences,
    handleKanbanStatusVisibility,
    handleKanbanStatusColor,
    versionsForRecord,
    isCrossVersion,
    treeList,
    boardNavigationItems,
    tableCols,
    tableScrollWidth,
    handleResizeColumn,
    fmtDate,
    handleSearchFiltersChange,
    toggleSearchBar,
    handleTabChange,
    handleViewSelect,
    openCreateView,
    openEditView,
    openSaveAsView,
    openSaveCurrentSearch,
    handleViewSaved,
    handleViewDeleted,
    handleViewSettingsSaved,
    handleSearch,
    handleListPageChange,
    createInStatus,
    handleKanbanUpdated,
    isEditing,
    handleStatusTransitionSuccess,
    clearEditing,
    cancelEditing,
    getRowClassName,
    hasChildren,
    isChildLoading,
    toggleExpand,
    toggleAllTreeRows,
    loadMoreChildren,
    titleDraft,
    setTitleInputRef,
    startEditTitle,
    saveInlineTitle,
    saveInline,
    saveAssigneesInline,
    saveInlineCustomField,
    handleDeleteConfirmOpen,
    confirmDeleteTarget,
    handleMoreMenu,
    handleBatchMenu,
    submitBatchAssignees,
    dateValue,
    toPlannedDateTimestamp,
    handleDrawerNavigate,
    slaSummaries,
    slaFailed,
    agingTagClass,
    agingTagText,
    openDetail,
    openWorkLogEditor,
    handleDirectWorkLogSummaryChange,
    formatWorkHours,
    formatWorkHoursValue,
    closeDetail,
    handleDetailUpdated,
  };
}

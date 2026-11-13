import { computed, onMounted, reactive, ref, toRef, watch } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import { listVersionWorkItems } from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";
import { useWorkItemViews } from "@/composables/useWorkItemViews";
import { useWorkspaceApplications } from "@/composables/useWorkspaceApplications";
import {
  hasSearchFilterValues,
  normalizeSearchFilterTree,
} from "@/constants/workItemView";
import {
  readGlobalListPreferences,
  readListColumnWidths,
  readScopedListPreferences,
  writeListColumnWidths,
  writeScopedListPreferences,
} from "@/utils/boardPreferences";
import {
  effectiveIterationFields,
  iterationColumns,
  iterationFieldDescriptors,
  iterationFilterSources,
  iterationListFields,
  sanitizeIterationSearchFilters,
} from "@/components/iteration/iterationListConfig";

// Sortable through the iteration work-item list endpoint; mirrors the backend
// whitelist so an unsupported column never shows a header sort button.
const ITERATION_SORT_FIELDS = new Set([
  "status",
  "priority",
  "assignee",
  "planned_start_date",
  "planned_end_date",
  "created_by",
  "created_at",
]);

function normalizeListSort(value) {
  const field = ITERATION_SORT_FIELDS.has(value?.field) ? value.field : "";
  const direction = ["asc", "desc"].includes(value?.direction)
    ? value.direction
    : "";
  return field && direction
    ? { field, direction }
    : { field: "", direction: "" };
}

export function useIterationWorkItems(props, emit) {
  const { t } = useI18n();
  const authStore = useAuthStore();
  const viewsState = useWorkItemViews(toRef(props, "wsId"));
  const items = ref([]);
  const loading = ref(false);
  const page = ref(1);
  const pageSize = ref(40);
  const total = ref(0);
  const listSettingsOpen = ref(false);
  const viewEditOpen = ref(false);
  const viewEditMode = ref("create");
  const viewEditTarget = ref(null);
  const viewConfigOpen = ref(false);
  let requestSequence = 0;

  const globalPreferences = readGlobalListPreferences(authStore.userInfo?.id);
  const scopedPreferences = readScopedListPreferences(
    authStore.userInfo?.id,
    props.wsId,
    "iteration",
  );
  const listPreferences = reactive({
    previewMode: ["drawer", "modal", "window"].includes(
      globalPreferences.previewMode,
    )
      ? globalPreferences.previewMode
      : "drawer",
    fields: Array.isArray(scopedPreferences.fields)
      ? scopedPreferences.fields
      : [],
  });
  // Sort and freeze live in the same scoped key as the fields; writeScopedListPreferences
  // merges patches, so neither the settings modal nor the other key loses data.
  const tablePreferences = reactive({
    sort: normalizeListSort(scopedPreferences.listSort),
    frozenThrough:
      typeof scopedPreferences.frozenThrough === "string"
        ? scopedPreferences.frozenThrough
        : "",
  });
  const columnWidths = reactive(
    readListColumnWidths(authStore.userInfo?.id, props.wsId, "iteration"),
  );

  const views = computed(() => viewsState.viewsOf("iteration"));
  const currentViewId = computed(
    () => viewsState.currentViewIdByType.value.iteration ?? null,
  );
  const rememberLast = computed(
    () => viewsState.rememberByType.value.iteration !== false,
  );
  const searchExpanded = computed(
    () => viewsState.searchExpandedByType.value.iteration === true,
  );
  const searchFilters = computed(() =>
    normalizeSearchFilterTree(
      viewsState.searchFiltersByType.value.iteration,
      "iteration",
    ),
  );
  const searchHasValues = computed(() =>
    hasSearchFilterValues(searchFilters.value),
  );
  const canManageSystemView = computed(() =>
    authStore.hasPermission("version:manage_system_view"),
  );
  const filterSources = computed(() =>
    iterationFilterSources(props.filterOptions),
  );
  const statusOptions = computed(() => filterSources.value.statuses);
  const priorityOptions = computed(() => filterSources.value.priorities);
  const memberOptions = computed(() => filterSources.value.members);
  const { defectAppEnabled } = useWorkspaceApplications(
    computed(() => props.wsId),
  );
  const fieldDescriptors = computed(() =>
    iterationFieldDescriptors(props.filterOptions, t, defectAppEnabled.value),
  );
  const listFields = computed(() => iterationListFields(t));
  const effectiveFields = computed(() =>
    effectiveIterationFields(listFields.value, listPreferences.fields),
  );
  const columnDefinitions = computed(() => iterationColumns(t));
  const visiblePreferenceKeys = computed(() =>
    effectiveFields.value
      .filter((field) => field.visible !== false)
      .map((field) => field.preferenceKey),
  );
  const frozenColumnIndex = computed(() =>
    visiblePreferenceKeys.value.indexOf(tablePreferences.frozenThrough),
  );
  const hasFrozenColumns = computed(() => frozenColumnIndex.value >= 0);
  const columns = computed(() =>
    effectiveFields.value
      .filter((field) => field.visible !== false)
      .map((field, index) => {
        const definition = columnDefinitions.value[field.preferenceKey];
        if (!definition) return null;
        const minWidth = definition.minWidth || 80;
        const maxWidth = definition.maxWidth || 600;
        const width = Number(
          columnWidths[field.preferenceKey] ?? definition.width,
        );
        return {
          ...definition,
          preferenceKey: field.preferenceKey,
          width: Math.min(
            maxWidth,
            Math.max(minWidth, Number.isFinite(width) ? width : minWidth),
          ),
          minWidth,
          maxWidth,
          resizable: true,
          fixed: frozenColumnIndex.value >= index ? "left" : undefined,
        };
      })
      .filter(Boolean),
  );
  const tableWidth = computed(() =>
    Math.max(
      960,
      columns.value.reduce(
        (sum, column) => sum + (Number(column.width) || column.minWidth || 80),
        0,
      ),
    ),
  );

  // Filter sources are iteration-scoped, so values kept from a previously opened
  // iteration are dropped (and persisted) before they reach the query.
  function querySearchFilters() {
    const filters = searchFilters.value;
    if (!props.filterOptions?.workspaces?.length) return filters;
    const sanitized = sanitizeIterationSearchFilters(
      filters,
      filterSources.value,
      fieldDescriptors.value,
    );
    if (JSON.stringify(sanitized) === JSON.stringify(filters)) return filters;
    viewsState
      .saveSearchSettings("iteration", { filters: sanitized })
      .catch(() => undefined);
    return sanitized;
  }

  async function fetchItems() {
    const sequence = ++requestSequence;
    if (!props.iterationId) {
      items.value = [];
      total.value = 0;
      emit("navigationChange", []);
      return;
    }
    loading.value = true;
    try {
      const result = await listVersionWorkItems(props.wsId, props.iterationId, {
        page: page.value,
        size: pageSize.value,
        view_id: currentViewId.value || undefined,
        search_filters: JSON.stringify(querySearchFilters()),
        sort_field: tablePreferences.sort.field || undefined,
        sort_direction: tablePreferences.sort.direction || undefined,
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
      });
      if (sequence !== requestSequence) return;
      items.value = result?.list || [];
      total.value = Number(result?.total) || 0;
      emit(
        "navigationChange",
        items.value.map((item) => ({
          id: item.id,
          workspaceId: item.workspace_id,
        })),
      );
    } catch {
      // The shared interceptor reports request errors.
    } finally {
      if (sequence === requestSequence) loading.value = false;
    }
  }

  function handlePageChange(nextPage, nextSize) {
    page.value = Number(nextSize) === Number(pageSize.value) ? nextPage : 1;
    pageSize.value = nextSize;
    fetchItems();
  }
  function toggleSearch() {
    viewsState
      .saveSearchSettings("iteration", {
        expanded: !searchExpanded.value,
      })
      .catch(() => undefined);
  }
  function handleSearchFiltersChange(filters, detail) {
    viewsState
      .saveSearchSettings("iteration", { filters })
      .catch(() => undefined);
    if (detail?.immediateQuery) {
      page.value = 1;
      fetchItems();
    }
  }
  function handleViewSelect(viewId) {
    viewsState.selectView("iteration", viewId);
    page.value = 1;
    fetchItems();
  }
  function setViewEditor(mode, view = null) {
    viewEditMode.value = mode;
    viewEditTarget.value = view;
    viewEditOpen.value = true;
  }
  function openCreateView() {
    setViewEditor("create");
  }
  function openEditView(view) {
    setViewEditor("edit", view);
  }
  function openSaveAsView(view) {
    setViewEditor("saveAs", view);
  }
  function openSaveCurrentSearch() {
    setViewEditor("saveAs", {
      name: "",
      view_type: "personal",
      filters: searchFilters.value,
    });
  }
  async function handleViewSaved(savedView) {
    await viewsState.loadViews("iteration");
    if (savedView?.id && ["create", "saveAs"].includes(savedView.mode)) {
      viewsState.selectView("iteration", String(savedView.id));
    }
    page.value = 1;
    fetchItems();
  }
  function applyListPreferences(preferences) {
    listPreferences.previewMode = preferences.previewMode;
    listPreferences.fields = preferences.fields;
    emit("previewModeChange", preferences.previewMode);
  }

  // Column order and visibility share the scoped storage key with ListSettingsModal,
  // so a column dropped here can be switched back on in the settings modal.
  function persistListFields(fields) {
    listPreferences.fields = fields.map((field) => ({ ...field }));
    writeScopedListPreferences(
      authStore.userInfo?.id,
      props.wsId,
      "iteration",
      {
        fields: fields.map((field) => ({
          key: field.preferenceKey,
          visible: field.visible !== false,
        })),
      },
    );
  }

  function persistTablePreferences(patch) {
    Object.assign(tablePreferences, patch);
    writeScopedListPreferences(
      authStore.userInfo?.id,
      props.wsId,
      "iteration",
      {
        listSort: { ...tablePreferences.sort },
        frozenThrough: tablePreferences.frozenThrough,
      },
    );
  }

  function persistColumnWidths() {
    writeListColumnWidths(
      authStore.userInfo?.id,
      props.wsId,
      "iteration",
      columnWidths,
    );
  }

  function handleResizeColumn(width, column) {
    const minWidth = column.minWidth || 80;
    const maxWidth = column.maxWidth || 600;
    columnWidths[column.key] = Math.min(maxWidth, Math.max(minWidth, width));
  }

  // Reorder the full field list, hidden columns included, so hiding a column never
  // shifts the stored position of the ones left behind.
  function moveColumn(sourceKey, targetKey, side) {
    if (!sourceKey || !targetKey || sourceKey === targetKey) return;
    const fields = effectiveFields.value.map((field) => ({ ...field }));
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
    const current = tablePreferences.sort;
    let sort = { field: column.sortField, direction: "asc" };
    if (current.field === column.sortField && current.direction === "asc") {
      sort = { field: column.sortField, direction: "desc" };
    } else if (
      current.field === column.sortField &&
      current.direction === "desc"
    ) {
      sort = { field: "", direction: "" };
    }
    persistTablePreferences({ sort });
    page.value = 1;
    fetchItems();
  }

  function handleColumnMenu(key, column) {
    if (key === "freeze") {
      if (!canFreezeColumn(column)) return;
      persistTablePreferences({ frozenThrough: column.preferenceKey });
    } else if (key === "unfreeze") {
      persistTablePreferences({ frozenThrough: "" });
    } else if (key === "hide") {
      if (!canHideColumn(column)) return;
      const fields = effectiveFields.value.map((field) => ({ ...field }));
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
    const index = visiblePreferenceKeys.value.indexOf(column.preferenceKey);
    return index >= 0 && index < visiblePreferenceKeys.value.length - 1;
  }

  // The title column carries the type pill and the detail open entry; keeping it out
  // of this quick action leaves the list usable. Hidden columns reopen in the settings.
  function canHideColumn(column) {
    return column.key !== "title" && Boolean(column.preferenceKey);
  }

  onMounted(async () => {
    await viewsState.ensureViewsLoaded("iteration");
    fetchItems();
  });
  watch(
    () => [props.wsId, props.iterationId, props.refreshKey],
    () => {
      page.value = 1;
      fetchItems();
    },
  );
  // Filter sources arrive after the iteration switch, so re-query once stale values are dropped.
  watch(
    () => props.filterOptions,
    () => {
      const before = JSON.stringify(searchFilters.value);
      querySearchFilters();
      if (JSON.stringify(searchFilters.value) === before) return;
      page.value = 1;
      fetchItems();
    },
  );

  return {
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
    effectiveFields,
    tablePreferences,
    columnWidths,
    hasFrozenColumns,
    visiblePreferenceKeys,
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
  };
}

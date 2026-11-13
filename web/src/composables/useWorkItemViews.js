// useWorkItemViews.js
// View state for the requirement/defect board: loads the view list per work
// item type, tracks the current view, and remembers the last viewed one in
// localStorage when the user's remember switch is on.
import { ref } from "vue";
import { listWorkItemViews, saveUserViewSettings } from "@/api/workItemView";
import {
  createDefaultSearchFilters,
  normalizeSearchFilterTree,
} from "@/constants/workItemView";

// localStorage key of the last selected view for one workspace + type.
function lastViewStorageKey(wsId, type) {
  return `goteams-last-view-${wsId}-${type}`;
}

// hasActiveFilters reports whether a filter tree contains at least one leaf condition.
export function hasActiveFilters(node) {
  if (!node) return false;
  if (node.field) {
    if (node.operator === "empty" || node.operator === "not_empty") return true;
    return (node.values || []).some((value) => String(value).trim().length > 0);
  }
  return (node.conditions || []).some((child) => hasActiveFilters(child));
}

export function useWorkItemViews(wsId) {
  // State is kept per view namespace so requirement/defect/iteration/gantt tabs stay independent.
  const viewsByType = ref({
    requirement: [],
    defect: [],
    iteration: [],
    gantt: [],
  });
  const rememberByType = ref({
    requirement: true,
    defect: true,
    iteration: true,
    gantt: true,
  });
  const currentViewIdByType = ref({
    requirement: null,
    defect: null,
    iteration: null,
    gantt: null,
  });
  const searchExpandedByType = ref({
    requirement: true,
    defect: true,
    iteration: true,
    gantt: true,
  });
  const searchFiltersByType = ref({
    requirement: createDefaultSearchFilters("requirement"),
    defect: createDefaultSearchFilters("defect"),
    iteration: createDefaultSearchFilters("iteration"),
    gantt: createDefaultSearchFilters("gantt"),
  });
  const loadedTypes = ref(new Set());
  let saveQueues = {
    requirement: Promise.resolve(),
    defect: Promise.resolve(),
    iteration: Promise.resolve(),
    gantt: Promise.resolve(),
  };

  function viewsOf(type) {
    return viewsByType.value[type] || [];
  }

  function findView(type, id) {
    return viewsOf(type).find((view) => String(view.id) === String(id)) || null;
  }

  // resolveInitialView picks the view to show on load: remembered → default → builtin all → first enabled.
  function resolveInitialView(type) {
    const enabled = viewsOf(type).filter((view) => view.enabled);
    if (!enabled.length) {
      currentViewIdByType.value[type] = null;
      return;
    }
    if (rememberByType.value[type]) {
      const remembered = localStorage.getItem(
        lastViewStorageKey(wsId.value, type),
      );
      if (
        remembered &&
        enabled.some((view) => String(view.id) === String(remembered))
      ) {
        currentViewIdByType.value[type] = remembered;
        return;
      }
    }
    const fallback =
      enabled.find((view) => view.is_default) ||
      enabled.find((view) => view.builtin_key === "all") ||
      enabled[0];
    currentViewIdByType.value[type] = fallback.id;
  }

  // loadViews fetches the list for one type and re-resolves the current view.
  async function loadViews(type) {
    try {
      const res = await listWorkItemViews(wsId.value, { type });
      viewsByType.value[type] = res?.list || [];
      rememberByType.value[type] = res?.remember_last !== false;
      searchExpandedByType.value[type] = res?.search_expanded === true;
      searchFiltersByType.value[type] = normalizeSearchFilterTree(
        res?.search_filters,
        type,
      );
      loadedTypes.value.add(type);
      const current = currentViewIdByType.value[type];
      if (!current || !findView(type, current)?.enabled) {
        resolveInitialView(type);
      }
    } catch {
      /* handled by interceptor */
    }
  }

  // ensureViewsLoaded loads a type once; later refreshes go through loadViews.
  async function ensureViewsLoaded(type) {
    if (!loadedTypes.value.has(type)) {
      await loadViews(type);
    }
  }

  // selectView switches the current view and persists it when remembering is on.
  function selectView(type, viewId) {
    currentViewIdByType.value[type] = viewId;
    if (rememberByType.value[type]) {
      localStorage.setItem(
        lastViewStorageKey(wsId.value, type),
        String(viewId),
      );
    }
  }

  function saveSearchSettings(type, patch) {
    const workspaceId = wsId.value;
    if (Object.hasOwn(patch, "expanded")) {
      searchExpandedByType.value[type] = Boolean(patch.expanded);
    }
    if (patch.filters) {
      searchFiltersByType.value[type] = normalizeSearchFilterTree(
        patch.filters,
        type,
      );
    }
    const payload = { type };
    if (Object.hasOwn(patch, "expanded")) {
      payload.search_expanded = Boolean(patch.expanded);
    }
    if (patch.filters) {
      payload.search_filters = normalizeSearchFilterTree(patch.filters, type);
    }
    saveQueues[type] = (saveQueues[type] || Promise.resolve())
      .catch(() => undefined)
      .then(() => saveUserViewSettings(workspaceId, payload));
    return saveQueues[type];
  }

  // resetForWorkspace clears cached state when switching workspaces.
  function resetForWorkspace() {
    viewsByType.value = {
      requirement: [],
      defect: [],
      iteration: [],
      gantt: [],
    };
    rememberByType.value = {
      requirement: true,
      defect: true,
      iteration: true,
      gantt: true,
    };
    currentViewIdByType.value = {
      requirement: null,
      defect: null,
      iteration: null,
      gantt: null,
    };
    searchExpandedByType.value = {
      requirement: true,
      defect: true,
      iteration: true,
      gantt: true,
    };
    searchFiltersByType.value = {
      requirement: createDefaultSearchFilters("requirement"),
      defect: createDefaultSearchFilters("defect"),
      iteration: createDefaultSearchFilters("iteration"),
      gantt: createDefaultSearchFilters("gantt"),
    };
    loadedTypes.value = new Set();
    saveQueues = {
      requirement: Promise.resolve(),
      defect: Promise.resolve(),
      iteration: Promise.resolve(),
      gantt: Promise.resolve(),
    };
  }

  return {
    viewsByType,
    rememberByType,
    currentViewIdByType,
    searchExpandedByType,
    searchFiltersByType,
    viewsOf,
    findView,
    loadViews,
    ensureViewsLoaded,
    selectView,
    saveSearchSettings,
    resolveInitialView,
    resetForWorkspace,
  };
}

import { reactive, watch } from "vue";
import { useRoute } from "vue-router";
import { i18n } from "@/locales";
import { useWorkspaceStore } from "@/stores/workspace";
import { useLocaleStore } from "@/stores/locale";

// Fixed suffix for the tab title.
const SUFFIX = "GoTeams";

// Module-level state: primaryText is the raw text (e.g. project name); primaryKey/secondaryKey are i18n keys.
const state = reactive({
  primaryText: "",
  primaryKey: "",
  secondaryKey: "",
  fixedKey: "",
  detailId: "",
  detailTitle: "",
});

// 渲染 document.title：固定标题优先，否则按二级-一级-后缀拼接
function renderTitle() {
  const t = i18n?.global?.t;
  if (state.fixedKey && t) {
    document.title = t(state.fixedKey);
    return;
  }
  const primary =
    state.primaryText || (state.primaryKey && t ? t(state.primaryKey) : "");
  const secondary = state.secondaryKey && t ? t(state.secondaryKey) : "";
  const parts = [primary, secondary].filter((v) => !!v);
  document.title = parts.length ? `${parts.join(" - ")} - ${SUFFIX}` : SUFFIX;
}

// Called by tab pages: report the secondary-menu i18n key and re-render immediately.
export function setSecondaryKey(key) {
  state.secondaryKey = key || "";
  renderTitle();
}

// 工作项详情页（新窗口打开的需求/缺陷）用需求/缺陷名称做标题，不再拼接项目名和「需求详情」。
function isWorkItemDetailPath(path) {
  return /^\/(workspace\/[^/]+\/work-items|issues\/[^/]+\/items)\/[^/]+/.test(
    path,
  );
}

// 已上报的标题只在仍属于当前工作项时有效；切页或切换工作项后失效。
function applyDetailTitle() {
  if (!state.detailTitle) return;
  state.primaryText = state.detailTitle;
  state.primaryKey = "";
  state.secondaryKey = "";
}

// Called by the work-item detail page: report the requirement/defect name as the tab title.
export function setDetailTitle(id, title) {
  state.detailId = String(id || "");
  state.detailTitle = title || "";
  applyDetailTitle();
  renderTitle();
}

// Whether this is a "tab-type" page: the secondary menu is decided by the in-page tab; globally we only hold a placeholder.
function isTabPage(path) {
  return (
    path.startsWith("/my-work") ||
    path.startsWith("/team-work") ||
    /^\/pipelines\/?$/.test(path) ||
    /^\/workspace\/[^/]+\/board(\/|$)/.test(path)
  );
}

// Resolve the tab-type page's secondary menu (i18n key).
// Returning null means "let the page report via setSecondaryKey; globally do not override";
// The board page triggers query changes when switching tabs (component reuse, no remount), so it must be derived directly from the route.
function resolveTabSecondary(route) {
  if (/^\/workspace\/[^/]+\/board(\/|$)/.test(route.path)) {
    // Keep in sync with BoardPage.vue's boardTabTitle; new tabs need updates in both places.
    const map = {
      planning: "planning_tabTitle",
      requirement: "requirement_feature",
      defect: "requirement_bug",
      iteration: "requirement_filterVersion",
      dashboard: "app_dashboard",
    };
    return route.query.tab in map
      ? map[route.query.tab]
      : "requirement_feature";
  }
  // The "my work" / "team work" / Pipeline secondary menus are reported by the page's active-tab watcher.
  return null;
}

// Resolve the primary menu: returns { key } (i18n) or { text } (raw text, e.g. project name).
function resolvePrimary(route, workspaceStore) {
  const { path, params, meta } = route;
  if (meta?.workItemSurface === "issue")
    return { key: meta.titleKey || "issue_management" };
  if (path.startsWith("/my-work")) return { key: "menu_myWork" };
  if (path.startsWith("/team-work")) return { key: "menu_teamWork" };
  if (path.startsWith("/pipelines")) return { key: "menu_pipeline" };
  if (path.startsWith("/members")) return { key: "workspace_members" };
  if (path.startsWith("/profile")) return { key: "menu_profile" };
  if (path.startsWith("/workspace/")) {
    const wsId = String(params.wsId || "");
    const ws = workspaceStore.workspaces.find((w) => String(w.id) === wsId);
    return { text: ws?.name || "" };
  }
  // Fall back to the route's meta.title (e.g. login / workspace list / 404 / 403).
  return { text: meta?.title || "" };
}

// Resolve the route-type secondary menu (global only for non-tab pages).
function resolveSecondary(route) {
  const { path } = route;
  if (path.startsWith("/workspace/")) {
    if (/\/versions(\/|$)/.test(path)) return "workspace_versions";
    if (/\/settings(\/|$)/.test(path)) return "workspace_settings";
    if (/\/create(\/|$)/.test(path)) return "requirement_create";
    if (/\/work-items\//.test(path)) return "requirement_detail";
  }
  return "";
}

// Called once in App.vue's setup: set up global listeners and render the title.
export function usePageTitle() {
  const route = useRoute();
  const workspaceStore = useWorkspaceStore();
  const localeStore = useLocaleStore();

  // Resolve and render the primary/secondary menus from the current route.
  function sync() {
    if (!route.matched.length) return;
    state.fixedKey = route.meta?.fixedTitleKey || "";
    // 离开工作项详情、或切换到另一个工作项后，旧标题不再适用。
    if (
      !isWorkItemDetailPath(route.path) ||
      String(route.params.id || "") !== state.detailId
    ) {
      state.detailId = "";
      state.detailTitle = "";
    }
    const { key, text } = resolvePrimary(route, workspaceStore);
    if (text !== undefined) {
      state.primaryText = text;
      state.primaryKey = "";
    } else {
      state.primaryKey = key || "";
      state.primaryText = "";
    }
    // Tab page secondary menus: the board page is derived directly from query.tab; others are reported by the page (no override, to avoid clearing).
    if (isTabPage(route.path)) {
      const tabSecondary = resolveTabSecondary(route);
      if (tabSecondary !== null) state.secondaryKey = tabSecondary;
    } else {
      state.secondaryKey = resolveSecondary(route);
    }
    applyDetailTitle();
    renderTitle();

    // The project name may not be loaded yet; the sidebar's fetch fills it in and the
    // workspaces watcher below re-syncs. Never chain sync() onto this call: while a request
    // is in flight fetchWorkspaces resolves immediately with the still-empty list, so the
    // chained sync() re-enters and spins the main thread forever.
    if (route.path.startsWith("/workspace/")) {
      const wsId = String(route.params.wsId || "");
      if (!workspaceStore.workspaces.some((w) => String(w.id) === wsId)) {
        workspaceStore.fetchWorkspaces().catch(() => {});
      }
    }
  }

  // 首次打开根路径时，占位路由和主页的 fullPath 都是 “/”，需同时监听匹配结果。
  watch(() => [route.fullPath, route.matched.length], sync, {
    immediate: true,
  });
  // 语言切换时重新翻译（secondary/primary 存的是 i18n key）
  watch(() => localeStore.currentLang, renderTitle);
  // Refresh the project name after the sidebar finishes fetching the workspace list.
  watch(
    () => workspaceStore.workspaces.length,
    () => {
      if (route.path.startsWith("/workspace/")) sync();
    },
  );
}

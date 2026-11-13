import { issueWorkItemRoute, legacyIssueRoute } from "@/utils/workItemRoutes";
import router from "./index";
import { isIssueWorkspace } from "@/composables/useWorkspaceApplications";
import NProgress from "nprogress";
import { useAuthStore } from "@/stores/auth";
import { readGlobalListPreferences } from "@/utils/boardPreferences";
import { getWorkspaceAccess } from "@/api/workspace";
import { safeInternalPath } from "@/utils/internalRedirect";
import { useLicenseStore } from "@/stores/license";
import { LICENSE_ROUTE_PATH } from "@/constants/license";
import "@/assets/nprogress.css";

NProgress.configure({ showSpinner: false, minimum: 0.12 });

// Allowlist routes (accessible without login).
const WHITE_LIST = ["/login", "/license", "/404", "/403"];

// Module-level flag, only valid within the current page lifecycle (reset to false on refresh or re-entry).
// It does not rely on store.sessionInitialized, which may be restored as true by legacy persisted state.
// This guarantees the latest permissions are fetched on every refresh, so permission changes take effect without re-login.
let sessionInitializedOnce = false;

router.beforeEach(async (to, _from, next) => {
  NProgress.start();
  const authStore = useAuthStore();
  // 0. Commercial license gate (highest priority, before login and page permissions).
  // A status fetch failure never blocks navigation: the backend gate still rejects
  // every business API, and the request interceptor then sends the user to /license.
  const licenseStore = useLicenseStore();
  await licenseStore.fetchStatus();
  if (licenseStore.isCommercial && !licenseStore.fetchFailed) {
    if (!licenseStore.canLogin) {
      if (to.path !== LICENSE_ROUTE_PATH) {
        authStore.clearSession();
        next({
          path: LICENSE_ROUTE_PATH,
          query: to.path === "/login" ? {} : { redirect: to.fullPath },
        });
        return;
      }
    } else if (licenseStore.managementOnly) {
      const managementTarget = {
        path: "/members",
        query: { tab: "version" },
        replace: true,
      };
      if (!authStore.token) {
        if (to.path !== "/login") {
          next({
            path: "/login",
            query: { redirect: "/members?tab=version" },
            replace: true,
          });
          return;
        }
      } else if (to.path !== "/members" || to.query.tab !== "version") {
        next(managementTarget);
        return;
      }
    } else if (to.path === LICENSE_ROUTE_PATH) {
      // A valid license never shows the license page; route back into the system.
      next(
        authStore.token
          ? { path: to.query.redirect || "/my-work", replace: true }
          : { path: "/login" },
      );
      return;
    }
  }

  // Operations login link (/login?token=xxx): exchange the token here so the login view never
  // renders. An invalid or expired token falls back to the manual login form.
  if (
    to.path === "/login" &&
    typeof to.query.token === "string" &&
    to.query.token
  ) {
    const redirectQuery =
      typeof to.query.redirect === "string" ? to.query.redirect : "";
    const safeRedirect =
      redirectQuery.startsWith("/") && !redirectQuery.startsWith("//")
        ? redirectQuery
        : "";
    try {
      await authStore.loginWithToken(to.query.token);
      sessionInitializedOnce = true;
      next({ path: safeRedirect || "/my-work", replace: true });
    } catch {
      // loginWithToken already logs out on failure; drop the token from the URL and stay on /login.
      next({ path: "/login", replace: true });
    }
    return;
  }

  const legacyIssue = legacyIssueRoute(to);
  if (legacyIssue) {
    next(legacyIssue);
    return;
  }

  // 1. Allowlist routes pass through directly (no login required).
  if (
    WHITE_LIST.includes(to.path) ||
    to.meta.public === true ||
    to.path.startsWith("/invite/")
  ) {
    // A signed-in user opening /login from TeamsBoard should skip the form and confirm immediately.
    if (to.path === "/login" && authStore.token) {
      const dest = safeInternalPath(
        typeof to.query.redirect === "string" ? to.query.redirect : "",
      );
      if (dest && dest !== "/login") {
        next(dest);
        return;
      }
    }
    next();
    return;
  }

  // 2. Check login status.
  if (!authStore.token) {
    next({ path: "/login", query: { redirect: to.fullPath } });
    return;
  }

  // 3. After a page refresh, initialize user info + enterprise permissions only once.
  const needsSessionInitialization = !sessionInitializedOnce;
  try {
    if (needsSessionInitialization) {
      await authStore.initializeSession();
      sessionInitializedOnce = true;
    } else if (to.path === "/profile") {
      // Refresh user info once each time the profile page is entered.
      await authStore.fetchUserInfo();
    }
  } catch {
    authStore.logout();
    next({ path: "/login", query: { redirect: to.fullPath } });
    return;
  }

  // Redirect a shared work-item URL before loading board metadata. The detail page resolves
  // the item type from its own response, so no extra blocking detail request is needed here.
  if (!needsSessionInitialization) {
    authStore.fetchEnterprisePlanInfo();
  }
  if (
    to.name === "RequirementBoard" &&
    to.query.detail &&
    to.query.tab === undefined
  ) {
    const preferences = readGlobalListPreferences(authStore.userInfo?.id);
    if (preferences.previewMode === "window") {
      next({
        name: "WorkItemDetail",
        params: { wsId: to.params.wsId, id: String(to.query.detail) },
        replace: true,
      });
      return;
    }
  }

  // 4. Fetch the current workspace's project permissions.
  // Preloaded metadata may only be consumed by the current target page and cannot be reused across routes as an old snapshot.
  authStore.clearProjectMetadata();
  // 不在 wsId 路由下的页面（如产出文档整页）用 query 带空间 id，否则拿不到项目权限快照，
  // 新标签打开或刷新时会把有工作项权限的成员误判成无权限。
  const wsId = String(to.params.wsId || to.query.wsId || "");
  if (to.name === "WorkspaceAccessDenied") {
    next();
    return;
  }
  if (wsId && authStore.permissionWorkspaceId !== wsId) {
    try {
      await authStore.fetchProjectPermissions(wsId);
    } catch {
      // A permission fetch failure does not block the route; the backend middleware provides a fallback.
    }
  }
  if (wsId && authStore.permissionWorkspaceId !== wsId) {
    try {
      const access = await getWorkspaceAccess(wsId);
      if (!access.is_member) {
        next({
          name: "WorkspaceAccessDenied",
          params: { wsId },
          query: { from: to.fullPath },
        });
        return;
      }
    } catch (error) {
      if (error.response?.status === 404) {
        next({ path: "/404" });
        return;
      }
    }
  }

  const issueWorkItem = issueWorkItemRoute(to);
  if (wsId && (issueWorkItem || to.name === "IssueInviteManage")) {
    const issueWorkspace = await isIssueWorkspace(wsId);
    if (issueWorkItem && issueWorkspace) {
      next(issueWorkItem);
      return;
    }
    if (to.name === "IssueInviteManage" && !issueWorkspace) {
      next({
        name: "RequirementBoard",
        params: { wsId },
        query: { ...to.query, tab: "issue-invite" },
        hash: to.hash,
        replace: true,
      });
      return;
    }
  }

  if (
    to.name === "IssueList" &&
    !["requirement:view", "defect:view"].some(authStore.hasPermission) &&
    ["workspace:view", "ticket:copy_link", "ticket:manage"].some(
      authStore.hasPermission,
    )
  ) {
    next({ name: "IssueInviteManage", params: { wsId }, replace: true });
    return;
  }

  // 5. Page-level permission interception; the API layer is still backed by backend middleware.
  const invitationBoard =
    to.name === "RequirementBoard" && to.query.tab === "issue-invite";
  const required = invitationBoard ? null : to.meta.permission;
  const anyRequired = invitationBoard
    ? ["workspace:view", "ticket:copy_link", "ticket:manage"]
    : to.meta.anyPermission;
  if (required && !authStore.hasPermission(required)) {
    next(await deniedRoute(wsId, to.fullPath));
    return;
  }
  if (
    Array.isArray(anyRequired) &&
    anyRequired.length &&
    !anyRequired.some((key) => authStore.hasPermission(key))
  ) {
    next(await deniedRoute(wsId, to.fullPath));
    return;
  }

  next();
});

async function deniedRoute(wsId, target) {
  if (!wsId) return { path: "/403" };
  try {
    const access = await getWorkspaceAccess(wsId);
    if (!access.is_member) {
      return {
        name: "WorkspaceAccessDenied",
        params: { wsId },
        query: { from: target },
      };
    }
    return { path: "/403" };
  } catch (error) {
    return { path: error.response?.status === 404 ? "/404" : "/403" };
  }
}

router.afterEach(() => {
  NProgress.done();
});

router.onError(() => {
  NProgress.done();
});

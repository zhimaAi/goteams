import { defineStore } from "pinia";
import { computed, ref } from "vue";
import {
  getAvatarBlob,
  getProfile,
  login as loginApi,
  logout as logoutApi,
  register as registerApi,
  updateProfile as updateProfileApi,
} from "@/api/auth";
import { getEnterprisePlanInfo } from "@/api/plan";
import { request } from "@/api/request";
import { getWorkspaceMetadata } from "@/api/workspace";
import { useWorkspaceStore } from "@/stores/workspace";
import { useContactPreferencesStore } from "@/stores/contactPreferences";
import { openCapacityOverLimitModal } from "@/utils/capacityOverLimitModal";

export const useAuthStore = defineStore(
  "auth",
  () => {
    // ---- State ----
    const token = ref("");
    const userInfo = ref(null);
    const avatarUrl = ref("");
    const enterprisePermissions = ref([]); // Enterprise permissions (independent of wsId, fetched once after login).
    const projectPermissions = ref([]); // Project permissions (depend on wsId).
    const permissionWorkspaceId = ref(""); // The wsId whose project permissions have been fetched.
    const projectMetadata = ref(null); // Project metadata preloaded by the route guard for the target page to consume.
    const projectMetadataWorkspaceId = ref("");
    const sessionInitialized = ref(false); // Whether user and enterprise permissions have been initialized within the current page lifecycle.
    const enterprisePlan = ref(null); // Current tenant plan info, used to detect enterprise edition expiration.

    // ---- Getters ----
    const isLoggedIn = () => !!token.value;

    /** Whether the enterprise edition service has expired. */
    const isEnterpriseExpired = computed(() => {
      const plan = enterprisePlan.value;
      return (
        !!plan && plan.plan_type === "enterprise" && plan.is_expired === true
      );
    });

    /** Whether storage capacity is over the limit and the system must be blocked (1=first time, 2=already extended and expired). */
    const capacityBlocked = computed(() => {
      const status = enterprisePlan.value?.capacity_status;
      return status === 1 || status === 2;
    });

    /**
     * Whether the user is the tenant's default system administrator.
     * Ownership is transferable, so it comes from the backend is_owner flag rather than
     * comparing the user id with admin_id (admin_id stays on the original creator).
     */
    function isSuperAdmin() {
      const u = userInfo.value;
      if (!u) return false;
      return u.is_owner === true || u.is_owner === 1 || u.is_owner === "1";
    }

    /** Check whether a permission exists (super admin has all permissions; merges enterprise + project permissions). */
    function hasPermission(permKey) {
      if (isSuperAdmin()) return true;
      return (
        enterprisePermissions.value.includes(permKey) ||
        projectPermissions.value.includes(permKey)
      );
    }

    /** Check enterprise-level permission without merging project permissions. */
    function hasEnterprisePermission(permKey) {
      if (isSuperAdmin()) return true;
      return enterprisePermissions.value.includes(permKey);
    }

    // ---- Actions ----

    let avatarObjectUrl = "";
    let avatarLoadingPromise = null;
    let avatarLoadVersion = 0;
    let userInfoLoadingPromise = null;
    const projectPermissionLoadingPromises = new Map();
    let latestPermissionWorkspaceId = "";

    /** Release the temporary URL of the current avatar. */
    function clearAvatar() {
      avatarLoadVersion += 1;
      avatarLoadingPromise = null;
      if (avatarObjectUrl) URL.revokeObjectURL(avatarObjectUrl);
      avatarObjectUrl = "";
      avatarUrl.value = "";
    }

    /** Update the global preview using the avatar URL returned by the backend (directly accessible, no extra request). */
    function setAvatarUrl(url) {
      clearAvatar();
      avatarUrl.value = url || "";
      if (userInfo.value) userInfo.value.avatar = url || "";
    }

    /** Load the current user's avatar. The backend returns a directly-accessible public URL, so no extra request is needed. */
    async function loadAvatar(force = false) {
      if (avatarUrl.value && !force) return avatarUrl.value;
      const url = userInfo.value?.avatar || "";
      if (!url) {
        clearAvatar();
        return "";
      }
      avatarUrl.value = url;
      return url;
    }

    /** Login: call the real API. */
    async function login(params) {
      const res = await loginApi(params);
      if (res) {
        clearAvatar();
        clearSessionScopedStores();
        token.value = res.token || "";
        const { token: _, password: __, ...info } = res;
        userInfo.value = info;
        await fetchEnterprisePermissions();
        await fetchEnterprisePlanInfo();
        sessionInitialized.value = true;
      }
      return res;
    }

    /** Accept a standard JWT from an operations login link and initialize the session. */
    async function loginWithToken(loginToken) {
      clearAvatar();
      useWorkspaceStore().clear();
      token.value = loginToken || "";
      userInfo.value = null;
      enterprisePermissions.value = [];
      sessionInitialized.value = false;
      try {
        await initializeSession();
        return userInfo.value;
      } catch (error) {
        logout();
        throw error;
      }
    }

    /** Register. */
    async function register(params) {
      const res = await registerApi(params);
      if (res) {
        clearAvatar();
        clearSessionScopedStores();
        token.value = res.token || "";
        const { token: _, password: __, ...info } = res;
        userInfo.value = info;
        await fetchEnterprisePermissions();
        await fetchEnterprisePlanInfo();
        sessionInitialized.value = true;
      }
      return res;
    }

    /** Fetch the current user's info, reusing the same request when triggered from multiple places at once. */
    function fetchUserInfo() {
      if (userInfoLoadingPromise) return userInfoLoadingPromise;

      const request = getProfile()
        .then((res) => {
          if (res) {
            userInfo.value = res;
            if (!res.avatar) clearAvatar();
          }
          return res;
        })
        .finally(() => {
          if (userInfoLoadingPromise === request) userInfoLoadingPromise = null;
        });
      userInfoLoadingPromise = request;
      return request;
    }

    /** Initialize user info and enterprise permissions once after a page refresh. */
    async function initializeSession() {
      const [res] = await Promise.all([
        fetchUserInfo(),
        fetchEnterprisePermissions(),
        fetchEnterprisePlanInfo(),
      ]);
      sessionInitialized.value = true;
      return res;
    }

    /** Fetch the current tenant's plan info for enterprise expiration detection. */
    async function fetchEnterprisePlanInfo() {
      try {
        const res = await getEnterprisePlanInfo();
        enterprisePlan.value = res || null;
        if (capacityBlocked.value) openCapacityOverLimitModal();
      } catch {
        enterprisePlan.value = null;
      }
    }

    /** Fetch enterprise permissions (independent of wsId, called only once). */
    async function fetchEnterprisePermissions() {
      try {
        const res = await request({
          url: "/api/user/permissions",
          method: "GET",
          params: { wsId: "" },
        });
        const keys = [];
        if (res) {
          for (const group of res) {
            for (const p of group.Permissions || []) {
              keys.push(p.Key);
            }
          }
        }
        enterprisePermissions.value = keys;
      } catch {
        enterprisePermissions.value = [];
      }
    }

    /** Fetch the user's project permission in a given workspace. */
    function fetchProjectPermissions(wsId) {
      if (!wsId) {
        projectPermissions.value = [];
        permissionWorkspaceId.value = "";
        clearProjectMetadata();
        latestPermissionWorkspaceId = "";
        return Promise.resolve([]);
      }

      const targetWorkspaceId = String(wsId);
      latestPermissionWorkspaceId = targetWorkspaceId;
      if (permissionWorkspaceId.value === targetWorkspaceId) {
        return Promise.resolve(projectPermissions.value);
      }
      if (projectPermissionLoadingPromises.has(targetWorkspaceId)) {
        return projectPermissionLoadingPromises.get(targetWorkspaceId);
      }

      const loadingPromise = getWorkspaceMetadata(targetWorkspaceId)
        .then((res) => {
          const keys = Array.isArray(res?.permissions) ? res.permissions : [];
          // When switching projects quickly, an earlier-returning request must not overwrite the current project's permissions.
          if (latestPermissionWorkspaceId === targetWorkspaceId) {
            projectPermissions.value = keys;
            permissionWorkspaceId.value = targetWorkspaceId;
            projectMetadata.value = res;
            projectMetadataWorkspaceId.value = targetWorkspaceId;
          }
          return keys;
        })
        .catch(() => {
          if (latestPermissionWorkspaceId === targetWorkspaceId) {
            projectPermissions.value = [];
            permissionWorkspaceId.value = "";
            clearProjectMetadata();
          }
          return [];
        })
        .finally(() => {
          projectPermissionLoadingPromises.delete(targetWorkspaceId);
        });
      projectPermissionLoadingPromises.set(targetWorkspaceId, loadingPromise);
      return loadingPromise;
    }

    /** Force-refresh project permissions after an application changes permission visibility. */
    function refreshProjectPermissions(wsId) {
      // TODO(issue-invite): keep this helper generic for other application-scoped permissions.
      permissionWorkspaceId.value = "";
      clearProjectMetadata();
      return fetchProjectPermissions(wsId);
    }

    /** Clear project metadata preloaded by the route guard. */
    function clearProjectMetadata() {
      projectMetadata.value = null;
      projectMetadataWorkspaceId.value = "";
    }

    /** Read project metadata preloaded by the route guard without consuming it, so
     * every early reader can reuse it and the page that owns the data still consumes it once. */
    function peekProjectMetadata(wsId) {
      if (
        projectMetadataWorkspaceId.value !== String(wsId) ||
        !projectMetadata.value
      ) {
        return null;
      }
      return projectMetadata.value;
    }

    /** Consume project metadata preloaded by the route guard to avoid duplicate requests on the target page. */
    function consumeProjectMetadata(wsId) {
      if (
        projectMetadataWorkspaceId.value !== String(wsId) ||
        !projectMetadata.value
      ) {
        return null;
      }
      const metadata = projectMetadata.value;
      clearProjectMetadata();
      return metadata;
    }

    /** Update the profile and sync the current user's information. */
    async function updateProfile(params) {
      await updateProfileApi(params);
      return fetchUserInfo();
    }

    /** Refresh the token. */
    async function refreshToken() {
      const res = await request({
        url: "/api/user/refreshToken",
        method: "GET",
      });
      if (res && res.token) {
        token.value = res.token;
        const { token: _, password: __, ...info } = res;
        userInfo.value = info;
      }
      return res;
    }

    /**
     * Drop every store that caches data belonging to one signed-in user.
     * Called from both ends of a session switch: logout, and login itself because signing in
     * as someone else straight from /login never goes through logout. The workspace list is
     * the visible symptom — it is fetched once and guarded by a `loaded` flag, so a stale list
     * would show the previous user's projects in the new user's sidebar.
     */
    function clearSessionScopedStores() {
      useWorkspaceStore().clear();
      useContactPreferencesStore().clear();
    }

    /** Log out. */
    function logout() {
      // Notify the backend to delete the session first (must be sent before clearing the token); failure does not block local logout.
      const logoutRequest = token.value
        ? logoutApi(token.value).catch(() => null)
        : Promise.resolve(null);
      clearAvatar();
      userInfoLoadingPromise = null;
      projectPermissionLoadingPromises.clear();
      latestPermissionWorkspaceId = "";
      clearProjectMetadata();
      token.value = "";
      userInfo.value = null;
      enterprisePermissions.value = [];
      projectPermissions.value = [];
      permissionWorkspaceId.value = "";
      sessionInitialized.value = false;
      enterprisePlan.value = null;
      clearSessionScopedStores();
      clearSession();
      return logoutRequest;
    }

    /**
     * Clear the local session without notifying the backend. Used by the commercial
     * license gate: the logout API itself would be rejected by the license gate, so
     * callers that already know the license is invalid must skip that call.
     */
    function clearSession() {
      clearAvatar();
      userInfoLoadingPromise = null;
      projectPermissionLoadingPromises.clear();
      latestPermissionWorkspaceId = "";
      clearProjectMetadata();
      token.value = "";
      userInfo.value = null;
      enterprisePermissions.value = [];
      projectPermissions.value = [];
      permissionWorkspaceId.value = "";
      sessionInitialized.value = false;
      enterprisePlan.value = null;
      clearSessionScopedStores();
    }

    return {
      token,
      userInfo,
      avatarUrl,
      enterprisePermissions,
      projectPermissions,
      permissionWorkspaceId,
      sessionInitialized,
      enterprisePlan,
      isLoggedIn,
      isSuperAdmin,
      hasPermission,
      hasEnterprisePermission,
      isEnterpriseExpired,
      capacityBlocked,
      login,
      loginWithToken,
      register,
      loadAvatar,
      setAvatarUrl,
      fetchUserInfo,
      initializeSession,
      updateProfile,
      fetchEnterprisePermissions,
      fetchEnterprisePlanInfo,
      fetchProjectPermissions,
      refreshProjectPermissions,
      clearProjectMetadata,
      peekProjectMetadata,
      consumeProjectMetadata,
      refreshToken,
      logout,
      clearSession,
    };
  },
  {
    persist: {
      key: "goteams-auth",
      // v3 uses the "paths" option; "pick" is the v4 API and is silently ignored here,
      // which would otherwise persist the whole state.
      // Only the token is persisted: permissions and user info must be refetched on every refresh
      // and must never be cached.
      paths: ["token"],
      // Backward-compatible cleanup: older versions persisted the whole state
      // (including sessionInitialized and enterprisePermissions) to local storage.
      // Restoring it would cache permissions and skip initializeSession, so keep only the token.
      beforeRestore: () => {
        try {
          const raw = localStorage.getItem("goteams-auth");
          if (raw) {
            const data = JSON.parse(raw);
            const clean = {
              token: data && typeof data.token === "string" ? data.token : "",
            };
            localStorage.setItem("goteams-auth", JSON.stringify(clean));
          }
        } catch {
          /* A cleanup failure must not block the main flow. */
        }
      },
    },
  },
);

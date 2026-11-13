// Shared per-workspace application market state (which optional modules are enabled).
// Application switches gate entries on several surfaces at once (app market cards, project
// settings sidebar, board top tabs, create page, detail drawer, filters), so the answer is
// cached per wsId here instead of every surface requesting its own copy.
import { computed, reactive, watch } from "vue";
import { getWorkspaceMetadata } from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";
import { usesIssuePages } from "@/utils/workspaceType";

// wsId (string) -> { keys, promise, loaded }.
const applicationState = reactive(new Map());

function recordOf(key) {
  let record = applicationState.get(key);
  if (!record) {
    record = { keys: [], workspaceType: "", promise: null, loaded: false };
    applicationState.set(key, record);
  }
  return record;
}

// The app market endpoint (GET /api/workspaces/:wsId/applications) is gated by
// `project_info:edit`, so ordinary members get a 403 for it. Workspace metadata is
// readable by any member and carries the same `enabled_applications` answer, so the
// switches are read from there.
function enabledKeysOf(metadata) {
  return Array.isArray(metadata?.enabled_applications)
    ? [...metadata.enabled_applications]
    : [];
}

function resolveKeys(record, metadata) {
  record.workspaceType = metadata?.workspace?.workspace_type || "";
  record.keys = enabledKeysOf(metadata);
  record.loaded = true;
  return record.keys;
}

// ensureWorkspaceApplications loads the enabled application keys of a workspace once and
// resolves with them; concurrent callers share the same in-flight request.
export function ensureWorkspaceApplications(wsId) {
  const key = String(wsId ?? "");
  if (!key) return Promise.resolve([]);
  const record = recordOf(key);
  if (record.promise) return record.promise;
  if (record.loaded) return Promise.resolve(record.keys);
  // Reuse the metadata the route guard already preloaded (without consuming it) so the
  // page that owns it still gets its copy and no duplicate request is sent.
  const preloaded = useAuthStore().peekProjectMetadata(key);
  if (preloaded) return Promise.resolve(resolveKeys(record, preloaded));
  record.promise = getWorkspaceMetadata(key)
    .then((metadata) => resolveKeys(record, metadata))
    .catch(() => {
      // Drop the failed entry so the next caller retries; the unknown state keeps
      // default-on modules (defect) visible until a response proves otherwise.
      applicationState.delete(key);
      return [];
    })
    .finally(() => {
      const current = applicationState.get(key);
      if (current) current.promise = null;
    });
  return record.promise;
}

// syncWorkspaceApplications feeds keys a caller already has (workspace metadata
// `enabled_applications`) into the cache, so dependent surfaces skip their own request.
export function syncWorkspaceApplications(wsId, keys, workspaceType) {
  const key = String(wsId ?? "");
  if (!key) return;
  const record = recordOf(key);
  if (workspaceType !== undefined) record.workspaceType = workspaceType;
  record.keys = Array.isArray(keys) ? [...keys] : [];
  record.loaded = true;
}

// setWorkspaceApplicationEnabled updates one switch in place after the market toggle
// succeeds, so every surface reacts without refetching.
export function setWorkspaceApplicationEnabled(wsId, appKey, enabled) {
  const key = String(wsId ?? "");
  if (!key || !appKey) return;
  const record = recordOf(key);
  const has = record.keys.includes(appKey);
  if (enabled && !has) record.keys = [...record.keys, appKey];
  if (!enabled && has)
    record.keys = record.keys.filter((item) => item !== appKey);
  record.loaded = true;
}

// Sidebar links use the same application state as the pages; unknown workspaces
// enter through the existing workspace route, whose guard resolves the destination.
export function workspaceEntryPath(wsId, workspaceType) {
  const key = String(wsId ?? "");
  const record = applicationState.get(key);
  return usesIssuePages(workspaceType || record?.workspaceType, record?.keys)
    ? `/issues/${key}`
    : `/workspace/${key}/board`;
}

export async function isIssueWorkspace(wsId) {
  const keys = await ensureWorkspaceApplications(wsId);
  return usesIssuePages(
    applicationState.get(String(wsId))?.workspaceType,
    keys,
  );
}

// useWorkspaceApplications exposes the cached switches of a workspace.
// wsId may be a ref (number/string) or a plain string; loading starts immediately.
export function useWorkspaceApplications(wsId) {
  const key = computed(() => {
    const raw =
      wsId && typeof wsId === "object" && "value" in wsId ? wsId.value : wsId;
    return String(raw ?? "");
  });
  watch(
    key,
    (k) => {
      if (k) ensureWorkspaceApplications(k);
    },
    { immediate: true },
  );

  const record = computed(() => applicationState.get(key.value));
  const applications = computed(() => record.value?.keys || []);
  const workspaceType = computed(() => record.value?.workspaceType || "");
  const loading = computed(() => !!record.value?.promise);
  // An unresolved workspace must not hide default-on modules (defect): report them as
  // enabled until a response says otherwise, and let callers check `ready` when the
  // distinction matters.
  const ready = computed(() => !!record.value?.loaded);
  const isAppEnabled = (appKey) =>
    !ready.value || applications.value.includes(appKey);
  const defectAppEnabled = computed(() => isAppEnabled("defect"));

  return {
    applications,
    workspaceType,
    loading,
    ready,
    isAppEnabled,
    defectAppEnabled,
  };
}

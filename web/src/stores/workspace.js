import { defineStore } from "pinia";
import { ref } from "vue";
import {
  listWorkspaces,
  createWorkspace as createWorkspaceApi,
  updateWorkspace as updateWorkspaceApi,
  deleteWorkspace as deleteWorkspaceApi,
} from "@/api/workspace";

function normalizeWorkspace(workspace = {}) {
  return {
    ...workspace,
    id: String(workspace.id || ""),
    name: workspace.name || "",
    slug: workspace.slug || "",
    description: workspace.description || "",
    color: workspace.color || "#0d68ff",
    icon: workspace.icon || "",
  };
}

export const useWorkspaceStore = defineStore("workspace", () => {
  const workspaces = ref([]);
  const loaded = ref(false);
  const loading = ref(false);
  const iconUrls = ref({});
  const iconObjectUrls = new Map();
  const iconLoadingPromises = new Map();

  function syncWorkspace(payload) {
    const id = String(payload?.id || "");
    if (!id) return null;
    const current = workspaces.value.find((item) => item.id === id);
    const workspace = normalizeWorkspace({ ...current, ...payload, id });
    if (current?.icon && current.icon !== workspace.icon)
      removeWorkspaceIcon(id);
    if (current) {
      workspaces.value = workspaces.value.map((item) =>
        item.id === id ? workspace : item,
      );
    } else {
      workspaces.value = [workspace, ...workspaces.value];
    }
    return workspace;
  }

  function setWorkspaceIconUrl(id, url) {
    const key = String(id);
    removeWorkspaceIcon(key);
    iconUrls.value = { ...iconUrls.value, [key]: url || "" };
    workspaces.value = workspaces.value.map((item) =>
      item.id === key ? { ...item, icon: url || "" } : item,
    );
    return url;
  }

  function removeWorkspaceIcon(id) {
    const key = String(id);
    const url = iconObjectUrls.get(key);
    if (url) URL.revokeObjectURL(url);
    iconObjectUrls.delete(key);
    iconLoadingPromises.delete(key);
    const nextUrls = { ...iconUrls.value };
    delete nextUrls[key];
    iconUrls.value = nextUrls;
    workspaces.value = workspaces.value.map((item) =>
      item.id === key ? { ...item, icon: "" } : item,
    );
  }

  async function loadWorkspaceIcon(id, force = false) {
    const key = String(id);
    const workspace = workspaces.value.find((item) => item.id === key);
    if (!workspace?.icon) {
      removeWorkspaceIcon(key);
      return "";
    }
    if (iconUrls.value[key] && !force) return iconUrls.value[key];
    // The backend already returns a directly accessible public URL; no extra request is needed.
    iconUrls.value = { ...iconUrls.value, [key]: workspace.icon };
    return workspace.icon;
  }

  async function fetchWorkspaces(force = false) {
    if (loading.value) {
      return workspaces.value;
    }
    if (loaded.value && !force) {
      return workspaces.value;
    }

    loading.value = true;
    try {
      const list = await listWorkspaces();
      workspaces.value = Array.isArray(list)
        ? list.map(normalizeWorkspace)
        : [];
      loaded.value = true;
      return workspaces.value;
    } finally {
      loading.value = false;
    }
  }

  async function createWorkspace(payload) {
    const res = await createWorkspaceApi(payload);
    const workspace = normalizeWorkspace({
      id: res?.id,
      workspace_type: payload.workspace_type || "software",
      name: payload.name,
      description: payload.description,
      color: payload.color,
    });

    workspaces.value = [
      workspace,
      ...workspaces.value.filter((item) => item.id !== workspace.id),
    ];
    loaded.value = true;
    return workspace;
  }

  async function updateWorkspace(id, payload) {
    await updateWorkspaceApi(id, payload);
    const workspace = normalizeWorkspace({
      ...workspaces.value.find((item) => item.id === String(id)),
      ...payload,
      id,
    });
    workspaces.value = workspaces.value.map((item) =>
      item.id === workspace.id ? workspace : item,
    );
    return workspace;
  }

  async function deleteWorkspace(id) {
    await deleteWorkspaceApi(id);
    removeWorkspaceIcon(id);
    workspaces.value = workspaces.value.filter(
      (item) => item.id !== String(id),
    );
  }

  function clear() {
    for (const url of iconObjectUrls.values()) URL.revokeObjectURL(url);
    iconObjectUrls.clear();
    iconLoadingPromises.clear();
    iconUrls.value = {};
    workspaces.value = [];
    loaded.value = false;
    loading.value = false;
  }

  return {
    workspaces,
    loaded,
    loading,
    iconUrls,
    syncWorkspace,
    loadWorkspaceIcon,
    setWorkspaceIconUrl,
    removeWorkspaceIcon,
    fetchWorkspaces,
    createWorkspace,
    updateWorkspace,
    deleteWorkspace,
    clear,
  };
});

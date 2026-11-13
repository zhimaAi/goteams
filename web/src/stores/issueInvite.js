import { defineStore } from "pinia";
import { ref } from "vue";
import {
  createIssueInviteTemplate as createIssueInviteTemplateApi,
  getIssueInviteConfig,
  listIssueInviteTemplates as listIssueInviteTemplatesApi,
  removeIssueInviteTemplate as removeIssueInviteTemplateApi,
  reorderIssueInviteTemplates as reorderIssueInviteTemplatesApi,
  updateIssueInviteConfig as updateIssueInviteConfigApi,
  updateIssueInviteTemplate as updateIssueInviteTemplateApi,
  uploadIssueInviteThemeImage as uploadIssueInviteThemeImageApi,
} from "@/api/issueInvite";

// useIssueInviteStore owns authenticated invitation-configuration state.
export const useIssueInviteStore = defineStore("issueInvite", () => {
  const appEnabled = ref(false);
  const loading = ref(false);
  const inviteKey = ref("");
  const config = ref({
    invite_key: "",
    enabled: 0,
    guide_title: "",
    guide_description: "",
    theme_image: "",
  });
  const templates = ref([]);

  // Per-workspace request sequence marker; stale async responses are ignored.
  let requestSeq = 0;

  async function loadConfiguration(wsId) {
    if (!wsId) return;
    const seq = ++requestSeq;
    loading.value = true;
    try {
      const [configData, list] = await Promise.all([
        getIssueInviteConfig(wsId),
        listIssueInviteTemplatesApi(wsId),
      ]);
      if (seq !== requestSeq) return;
      appEnabled.value = true;
      inviteKey.value = configData?.invite_key || "";
      config.value = {
        invite_key: configData?.invite_key || "",
        enabled: Number(configData?.enabled ?? 1),
        guide_title: configData?.guide_title || "",
        guide_description: configData?.guide_description || "",
        theme_image: configData?.theme_image || "",
      };
      templates.value = list || [];
    } catch (error) {
      if (seq !== requestSeq) return;
      throw error;
    } finally {
      if (seq === requestSeq) loading.value = false;
    }
  }

  async function saveConfig(wsId, payload) {
    const seq = requestSeq;
    // Partial payloads (e.g. a switch toggle that only carries `enabled`) must
    // keep the current guide fields instead of sending empty strings.
    const item = await updateIssueInviteConfigApi(wsId, {
      guide_title: payload.guide_title ?? config.value.guide_title ?? "",
      guide_description:
        payload.guide_description ?? config.value.guide_description ?? "",
      theme_image: payload.theme_image ?? config.value.theme_image ?? "",
      enabled:
        payload.enabled !== undefined
          ? payload.enabled
          : (config.value.enabled ?? 1),
    });
    if (seq !== requestSeq) return item;
    config.value = {
      invite_key: item?.invite_key ?? inviteKey.value,
      enabled: Number(item?.enabled ?? 1),
      guide_title: item?.guide_title || "",
      guide_description: item?.guide_description || "",
      theme_image: item?.theme_image || "",
    };
    return config.value;
  }

  async function uploadThemeImage(wsId, file) {
    const seq = requestSeq;
    const result = await uploadIssueInviteThemeImageApi(wsId, file);
    const url = result?.url || "";
    if (seq === requestSeq)
      config.value = { ...config.value, theme_image: url };
    return url;
  }

  async function createTemplate(wsId, payload) {
    const seq = requestSeq;
    const item = await createIssueInviteTemplateApi(wsId, payload);
    if (seq === requestSeq) templates.value = [...templates.value, item];
    return item;
  }

  async function saveTemplate(wsId, templateId, payload) {
    const seq = requestSeq;
    const item = await updateIssueInviteTemplateApi(wsId, templateId, payload);
    if (seq !== requestSeq) return item;
    templates.value = templates.value.map((existing) =>
      existing.id === item.id ? item : existing,
    );
    return item;
  }

  async function removeTemplate(wsId, templateId) {
    const seq = requestSeq;
    await removeIssueInviteTemplateApi(wsId, templateId);
    if (seq !== requestSeq) return;
    templates.value = templates.value.filter((item) => item.id !== templateId);
  }

  async function reorderTemplates(wsId, templateIds) {
    const seq = requestSeq;
    const previous = templates.value;
    await reorderIssueInviteTemplatesApi(wsId, templateIds);
    if (seq !== requestSeq) return;
    templates.value = templateIds
      .map((id) => previous.find((item) => item.id === id))
      .filter(Boolean)
      .concat(previous.filter((item) => !templateIds.includes(item.id)));
  }

  function reset() {
    requestSeq += 1;
    appEnabled.value = false;
    loading.value = false;
    inviteKey.value = "";
    config.value = {
      invite_key: "",
      enabled: 0,
      guide_title: "",
      guide_description: "",
      theme_image: "",
    };
    templates.value = [];
  }

  return {
    appEnabled,
    loading,
    inviteKey,
    config,
    templates,
    loadConfiguration,
    createTemplate,
    saveTemplate,
    removeTemplate,
    reorderTemplates,
    saveConfig,
    uploadThemeImage,
    reset,
  };
});

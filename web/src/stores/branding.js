import { computed, ref } from "vue";
import { defineStore } from "pinia";
import { getPublicBranding, getTenantBranding } from "@/api/personalization";

export const useBrandingStore = defineStore("branding", () => {
  const publicBranding = ref({});
  const tenantBranding = ref(null);
  const current = computed(() => tenantBranding.value || publicBranding.value);
  let revision = 0;

  async function loadPublic() {
    try {
      publicBranding.value = (await getPublicBranding()) || {};
    } catch {
      publicBranding.value = {};
    }
  }

  async function loadTenant() {
    const requestRevision = ++revision;
    try {
      const data = await getTenantBranding();
      if (requestRevision === revision) tenantBranding.value = data || {};
    } catch {
      if (requestRevision === revision) tenantBranding.value = {};
    }
  }

  function clearTenant() {
    revision += 1;
    tenantBranding.value = null;
  }

  return { publicBranding, current, loadPublic, loadTenant, clearTenant };
});

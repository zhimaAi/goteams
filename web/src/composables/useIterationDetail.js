// Detail drawer state for cross-workspace iteration rows.
import { ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { readGlobalListPreferences } from "@/utils/boardPreferences";
import { saveWorkItemNavigationContext } from "@/utils/workItemNavigation";

export function useIterationDetail() {
  const authStore = useAuthStore();
  const router = useRouter();
  const navigationItems = ref([]);
  const detailId = ref(null);
  const detailWorkspaceId = ref(null);
  const stored = readGlobalListPreferences(authStore.userInfo?.id);
  const previewMode = ref(
    ["drawer", "modal", "window"].includes(stored.previewMode)
      ? stored.previewMode
      : "drawer",
  );

  function openDetail(record) {
    // Window mode leaves the current page untouched, so the row opens the standalone
    // detail page of its own workspace instead of the shared drawer.
    if (previewMode.value === "window") {
      const token = saveWorkItemNavigationContext(navigationItems.value);
      const target = router.resolve({
        name: "WorkItemDetail",
        params: { wsId: record.workspace_id, id: record.id },
        query: token ? { navContext: token } : {},
      });
      window.open(target.href, "_blank", "noopener,noreferrer");
      return;
    }
    detailId.value = record.id;
    detailWorkspaceId.value = record.workspace_id;
  }
  function closeDetail() {
    detailId.value = null;
    detailWorkspaceId.value = null;
  }
  function navigateDetail(id, workspaceId) {
    detailId.value = id;
    detailWorkspaceId.value = workspaceId;
  }

  return {
    authStore,
    navigationItems,
    detailId,
    detailWorkspaceId,
    previewMode,
    openDetail,
    closeDetail,
    navigateDetail,
  };
}

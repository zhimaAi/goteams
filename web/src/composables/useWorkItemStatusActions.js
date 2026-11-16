import { computed } from "vue";
import { getWorkspaceMetadata } from "@/api/workspace";

export function useWorkItemStatusActions({
  wsId,
  metadata,
  canEditStatus,
  openTransition,
  onRefresh,
}) {
  const config = computed(() => {
    if (
      String(metadata.value?.workspace?.id) !== String(wsId.value) ||
      metadata.value?.workspace?.workspace_type !== "issue"
    )
      return null;
    return metadata.value.requirement?.status_actions;
  });
  const actionsByStatus = computed(() => {
    const grouped = new Map();
    for (const item of config.value?.items || []) {
      const list = grouped.get(item.source_status_id) || [];
      list.push(item);
      grouped.set(item.source_status_id, list);
    }
    return grouped;
  });
  const statusActionsEnabled = computed(() => !!config.value?.enabled);

  function statusActionsFor(item) {
    if (
      !statusActionsEnabled.value ||
      item?.type !== "requirement" ||
      !canEditStatus(item.type)
    )
      return [];
    return actionsByStatus.value.get(Number(item.status_id)) || [];
  }

  async function refreshStatusActions(workspaceId, itemId) {
    const latest = await getWorkspaceMetadata(workspaceId).catch(() => null);
    if (String(workspaceId) !== String(wsId.value)) return;
    if (latest) metadata.value = latest;
    try {
      await onRefresh(itemId);
    } catch {
      // Request interceptor reports refresh errors.
    }
  }

  function runStatusAction(item, action) {
    if (statusActionsFor(item).includes(action)) openTransition(item, action);
  }

  return {
    statusActionsEnabled,
    statusActionsFor,
    runStatusAction,
    refreshStatusActions,
  };
}

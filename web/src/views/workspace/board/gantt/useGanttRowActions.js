// 甘特图工作项详情、编辑与行菜单动作。
import { computed, ref, unref, watch } from "vue";
import { useRouter } from "vue-router";
import { Modal, message } from "ant-design-vue";
import { deleteRequirement, updateRequirement } from "@/api/workspace";
import { useWorkItemClipboardActions } from "@/composables/useWorkItemClipboardActions";
import { saveWorkItemNavigationContext } from "@/utils/workItemNavigation";

export function useGanttRowActions({ wsId, items, previewMode, t, refresh }) {
  const router = useRouter();
  const detailId = ref(null);
  const currentWsId = () => unref(wsId);
  const { copyWorkItemInfo } = useWorkItemClipboardActions(wsId, t);
  const navigationItems = computed(() =>
    items.value.map((item) => ({
      id: item.id,
      workspaceId: currentWsId(),
    })),
  );

  function openDetail(itemOrId) {
    const id = itemOrId?.id ?? itemOrId;
    if (!id) return;
    if (previewMode.value === "window") {
      const navigationContext = saveWorkItemNavigationContext(
        navigationItems.value,
      );
      const target = router.resolve({
        name: "WorkItemDetail",
        params: { wsId: currentWsId(), id },
        query: {
          fromTab: "gantt",
          ...(navigationContext ? { navContext: navigationContext } : {}),
        },
      });
      window.open(target.href, "_blank", "noopener,noreferrer");
      return;
    }
    detailId.value = id;
  }

  async function updateDates(item, start, end) {
    try {
      await updateRequirement(currentWsId(), item.id, {
        planned_start_date: start,
        planned_end_date: end,
      });
      item.planned_start_date = String(start);
      item.planned_end_date = String(end);
    } catch {
      /* 由请求拦截器统一提示 */
    }
  }

  function createChild(item) {
    router.push({
      path: `/workspace/${currentWsId()}/create`,
      query: { type: "requirement", parentId: item.id },
    });
  }

  function duplicate(item) {
    router.push({
      path: `/workspace/${currentWsId()}/create`,
      query: { copyFrom: item.id, type: item.type },
    });
  }

  function handleRowAction(key, item) {
    if (key === "child") createChild(item);
    else if (key === "duplicate") duplicate(item);
    else if (key === "delete") deleteWorkItem(item);
    else copyWorkItemInfo(item, key);
  }

  function deleteWorkItem(item) {
    Modal.confirm({
      title: t("workspace_confirmDelete"),
      content: item.title,
      okText: t("common_confirm"),
      okType: "danger",
      cancelText: t("common_cancel"),
      onOk: async () => {
        await deleteRequirement(currentWsId(), item.id);
        message.success(t("workspace_deleteSuccess"));
        refresh();
      },
    });
  }

  watch(previewMode, (mode) => {
    if (mode === "window") detailId.value = null;
  });

  return {
    detailId,
    navigationItems,
    openDetail,
    updateDates,
    handleRowAction,
  };
}

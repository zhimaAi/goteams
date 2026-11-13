<template>
  <div class="work-item-detail-route-page">
    <RequirementDetailDrawer
      :open="true"
      :initial-tab="String(route.query.detailTab || '')"
      :ws-id="wsId"
      :requirement-id="workItemId"
      mode="page"
      :work-log-editor-token="workLogEditorToken"
      :work-log-editor-requirement-id="workItemId"
      :navigation-items="navigationItems"
      :can-edit-by-type="{
        requirement: authStore.hasPermission('requirement:edit'),
        defect: authStore.hasPermission('defect:edit'),
      }"
      :can-create-by-type="{
        requirement: authStore.hasPermission('requirement:create'),
        defect: authStore.hasPermission('defect:create'),
      }"
      @close="returnToBoard"
      @loaded="handleDetailLoaded"
      @navigate="openWorkItem"
    />
  </div>
</template>

<script setup>
import { computed, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import RequirementDetailDrawer from "@/components/common/RequirementDetailDrawer.vue";
import { useAuthStore } from "@/stores/auth";
import {
  loadWorkItemNavigationContext,
  normalizeWorkItemNavigationItems,
} from "@/utils/workItemNavigation";
import { setDetailTitle } from "@/composables/usePageTitle";
import { setRememberedTab } from "@/utils/rememberedTab";

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const wsId = computed(() => route.params.wsId);
const workItemId = computed(() => route.params.id);
const workLogEditorToken = computed(() =>
  route.query.workLogEditor === "1" ? 1 : 0,
);
const workItemType = ref("");
const navigationItems = computed(() => {
  const contextItems = loadWorkItemNavigationContext(route.query.navContext);
  if (contextItems.length) return contextItems;
  return normalizeWorkItemNavigationItems([
    { id: workItemId.value, workspaceId: wsId.value },
  ]);
});

function handleDetailLoaded(workItem) {
  setDetailTitle(workItem?.id, workItem?.title);
  const type = workItem?.type;
  if (!["requirement", "defect"].includes(type)) return;
  workItemType.value = type;
  setRememberedTab(authStore.userInfo?.id, `project:${wsId.value}`, type);
}

function returnToBoard() {
  router.push({
    name: "RequirementBoard",
    params: { wsId: wsId.value },
    query: { tab: route.query.fromTab || workItemType.value || "requirement" },
  });
}

function openWorkItem(id, workspaceId = wsId.value) {
  router.replace({
    name: "WorkItemDetail",
    params: { wsId: workspaceId, id },
    query: route.query,
  });
}
</script>

<style scoped lang="less">
@import "@/components/work-item/workItemDetailPage.less";
</style>

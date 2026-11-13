<template>
  <div class="work-item-detail-route-page">
    <component
      :is="detailContainer"
      v-bind="detailContainerProps"
      @close="requestClose"
      @cancel="requestClose"
      ><template #title><WorkItemDetailHeader :model="model" /></template
      ><WorkItemDetailBody :model="model"
    /></component>
  </div>
</template>
<script setup>
import { computed, reactive } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useWorkItemDetail } from "@/composables/useWorkItemDetail";
import {
  loadWorkItemNavigationContext,
  normalizeWorkItemNavigationItems,
} from "@/utils/workItemNavigation";
import { safeInternalPath } from "@/utils/internalRedirect";
import { setDetailTitle } from "@/composables/usePageTitle";
import WorkItemDetailHeader from "@/components/work-item/WorkItemDetailHeader.vue";
import WorkItemDetailBody from "@/components/work-item/WorkItemDetailBody.vue";
const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const wsId = computed(() => route.params.wsId);
const id = computed(() => route.params.id);
const detailProps = reactive({
  open: true,
  wsId,
  requirementId: id,
  mode: "page",
  canEditByType: computed(() => ({
    requirement: authStore.hasPermission("requirement:edit"),
    defect: authStore.hasPermission("defect:edit"),
  })),
  canCreateByType: computed(() => ({
    requirement: authStore.hasPermission("requirement:create"),
    defect: authStore.hasPermission("defect:create"),
  })),
  navigationItems: computed(() => {
    const items = loadWorkItemNavigationContext(route.query.navContext);
    return items.length
      ? items
      : normalizeWorkItemNavigationItems([
          { id: id.value, workspaceId: wsId.value },
        ]);
  }),
  initialTab: computed(() => String(route.query.detailTab || "")),
  workLogEditorToken: computed(() =>
    route.query.workLogEditor === "1" ? 1 : 0,
  ),
  workLogEditorRequirementId: id,
});
function handleEvent(event, ...args) {
  if (event === "loaded") setDetailTitle(args[0]?.id, args[0]?.title);
  if (event === "navigate")
    router.replace({
      name: "IssueDetail",
      params: { wsId: args[1] || wsId.value, id: args[0] },
      query: route.query,
    });
  if (event === "close") {
    const back = safeInternalPath(String(route.query.returnTo || ""));
    const target = back && router.resolve(back);
    router.push(
      target?.name === "IssueList" &&
        String(target.params.wsId) === String(wsId.value)
        ? back
        : {
            name: "IssueList",
            params: { wsId: wsId.value },
            query: {
              tab:
                model.detail.value?.type ||
                route.query.fromTab ||
                "requirement",
            },
          },
    );
  }
}
const model = useWorkItemDetail(detailProps, handleEvent);
const { detailContainer, detailContainerProps, requestClose } = model;
</script>
<style scoped lang="less">
@import "@/components/work-item/workItemDetailPage.less";
</style>

<style lang="less">
@import "@/components/work-item/workItemDetailGlobal.less";
</style>

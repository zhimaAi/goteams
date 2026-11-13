<template>
  <DashboardPanel
    v-if="activeTab === 'dashboard'"
    ref="dashboardPanelRef"
    :key="`dashboard-${wsId}`"
    :ws-id="wsId"
    :metadata="workspaceMetadata"
    :filter-fields="{
      requirement: searchFieldsForType('requirement'),
      defect: searchFieldsForType('defect'),
    }"
  />
</template>
<script setup>
import { inject, watch } from "vue";
import { useRouter } from "vue-router";
import DashboardPanel from "@/components/dashboard/DashboardPanel.vue";

const router = useRouter();
const {
  activeTab,
  wsId,
  workspaceMetadata,
  searchFieldsForType,
  dashboardPanelRef,
} = inject("issueBoard");
watch(
  workspaceMetadata,
  (metadata) => {
    if (metadata && !metadata.enabled_applications?.includes("dashboard")) {
      router.replace({ name: "IssueList", params: { wsId: wsId.value } });
    }
  },
  { immediate: true },
);
</script>

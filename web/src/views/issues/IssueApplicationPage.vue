<template>
  <KnowledgeBaseView
    v-if="activeTab === 'knowledge'"
    :key="wsId"
    :ws-id="wsId"
  />
  <PlanningPanel
    v-else-if="activeTab === 'planning'"
    :key="wsId"
    :ws-id="wsId"
  />
  <GanttPanel
    v-else-if="activeTab === 'gantt'"
    :key="`gantt-${wsId}`"
    :ws-id="wsId"
  />
  <IterationPanel
    v-else-if="activeTab === 'iteration'"
    ref="iterationPanelRef"
    :key="wsId"
    :ws-id="wsId"
    :show-toolbar="false"
  />
</template>

<script setup>
import { inject, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import KnowledgeBaseView from "@/views/knowledgebase/KnowledgeBaseView.vue";
import PlanningPanel from "@/views/workspace/board/planning/PlanningPanel.vue";
import GanttPanel from "@/views/workspace/board/gantt/GanttPanel.vue";
import IterationPanel from "@/components/iteration/IterationPanel.vue";

const route = useRoute();
const router = useRouter();
const { wsId, activeTab, visibleTopTabs, iterationPanelRef } =
  inject("issueBoard");
watch(
  [() => route.meta.boardTab, visibleTopTabs],
  ([tab, tabs]) => {
    if (tab && !tabs.includes(tab)) {
      router.replace({ name: "IssueList", params: { wsId: wsId.value } });
    }
  },
  { immediate: true },
);
</script>

<template>
  <div ref="boardPageRef" class="board-page">
    <!-- Ended projects keep a corner ribbon; they stay editable but are hidden from project lists. -->
    <WorkItemBoardHeader
      :model="{ ...model, dashboardPanelRef, iterationPanelRef }"
    />
    <!-- Top: project icon + name + Tab -->

    <!-- ============ Planning View ============ -->
    <!-- Keyed by wsId so switching projects remounts the panel and reloads groups/metadata. -->
    <PlanningPanel v-if="activeTab === 'planning'" :key="wsId" :ws-id="wsId" />

    <!-- ============ Gantt View ============ -->
    <GanttPanel
      v-else-if="activeTab === 'gantt'"
      :key="`gantt-${wsId}`"
      :ws-id="wsId"
    />

    <!-- ============ Project Dashboard ============ -->
    <DashboardPanel
      ref="dashboardPanelRef"
      v-else-if="activeTab === 'dashboard'"
      :key="`dashboard-${wsId}`"
      :ws-id="wsId"
      :metadata="workspaceMetadata"
      :filter-fields="{
        requirement: searchFieldsForType('requirement'),
        defect: searchFieldsForType('defect'),
      }"
    />

    <!-- ============ Knowledge Base View ============ -->
    <KnowledgeBaseView
      v-else-if="activeTab === 'knowledge'"
      :key="wsId"
      :ws-id="wsId"
    />

    <!-- ============ Issue Invitation View ============ -->
    <IssueInviteManagePage
      v-else-if="activeTab === 'issue-invite'"
      :key="wsId"
    />

    <!-- ============ Iteration View ============ -->
    <IterationPanel
      ref="iterationPanelRef"
      v-else-if="activeTab === 'iteration'"
      :key="wsId"
      :ws-id="wsId"
      :show-toolbar="false"
    />

    <!-- ============ Requirement/Defect View ============ -->
    <WorkItemList v-else-if="isWorkItemTab" :model="model" />
  </div>
</template>
<script setup>
import { ref } from "vue";
import { useWorkItemBoard } from "@/composables/useWorkItemBoard";
import WorkItemBoardHeader from "@/components/work-item/WorkItemBoardHeader.vue";
import WorkItemList from "@/components/work-item/WorkItemList.vue";
import PlanningPanel from "./board/planning/PlanningPanel.vue";
import GanttPanel from "./board/gantt/GanttPanel.vue";
import DashboardPanel from "@/components/dashboard/DashboardPanel.vue";
import KnowledgeBaseView from "@/views/knowledgebase/KnowledgeBaseView.vue";
import IterationPanel from "@/components/iteration/IterationPanel.vue";
import IssueInviteManagePage from "@/views/issues/IssueInviteManagePage.vue";
const model = useWorkItemBoard();
const {
  boardPageRef,
  activeTab,
  isWorkItemTab,
  wsId,
  workspaceMetadata,
  searchFieldsForType,
} = model;
const dashboardPanelRef = ref(null);
const iterationPanelRef = ref(null);
</script>
<style scoped lang="less">
@import "@/components/work-item/workItemBoard.less";
</style>

<template>
  <div ref="boardPageRef" class="board-page">
    <WorkItemBoardHeader
      :model="{ ...model, dashboardPanelRef, iterationPanelRef }"
    />
    <router-view />
  </div>
</template>

<script setup>
import { provide, ref } from "vue";
import { useWorkItemBoard } from "@/composables/useWorkItemBoard";
import WorkItemBoardHeader from "@/components/work-item/WorkItemBoardHeader.vue";

const model = useWorkItemBoard();
const { boardPageRef } = model;
const dashboardPanelRef = ref(null);
const iterationPanelRef = ref(null);
provide("issueBoard", { ...model, dashboardPanelRef, iterationPanelRef });
</script>

<style scoped lang="less">
@import "@/components/work-item/workItemBoard.less";
</style>

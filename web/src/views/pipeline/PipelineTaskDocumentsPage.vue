<template>
  <div class="pipeline-documents-page">
    <TaskDocumentsPanel
      v-if="taskId"
      :task-id="taskId"
      :document-id="initialDocumentId"
      mode="page"
      @open-detail="openDetail"
      @update:document-id="syncDocumentId"
    />
    <!-- 文档页里的「任务详情」：从任务日志进来，关注点是执行过程，所以默认落在 Agent 页签；
         收起只隐藏（keepAlive），再点同一个按钮直接显示，不重新加载也不跳走。 -->
    <RequirementDetailDrawer
      v-if="detailTarget"
      :open="detailOpen"
      :ws-id="detailTarget.workspaceId"
      :requirement-id="detailTarget.workItemId"
      mode="drawer"
      initial-tab="agent"
      keep-alive
      @close="detailOpen = false"
    />
  </div>
</template>

<script setup>
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import RequirementDetailDrawer from "@/components/common/RequirementDetailDrawer.vue";
import TaskDocumentsPanel from "@/components/pipeline/TaskDocumentsPanel.vue";
import { setSecondaryKey } from "@/composables/usePageTitle";

const route = useRoute();
const router = useRouter();

const taskId = Number(route.params.taskId) || 0;
// The URL wins over the remembered document so a shared link opens the right file.
const initialDocumentId = ref(Number(route.query.doc) || null);
// 文档页自带的详情抽屉：不离开当前文档就能核对任务详情。
const detailTarget = ref(null);
const detailOpen = ref(false);

onMounted(() => {
  setSecondaryKey("pipeline_documents_title");
});

// Keep the address bar in sync without pushing a history entry per document.
function syncDocumentId(documentId) {
  if (!documentId) return;
  router.replace({
    name: "pipeline-task-documents",
    params: { taskId },
    query: { ...route.query, doc: String(documentId) },
  });
}

function openDetail(meta) {
  if (!meta?.workItemId) return;
  detailTarget.value = meta;
  detailOpen.value = true;
}
</script>

<style scoped lang="less">
@import "@/styles/variables.less";

.pipeline-documents-page {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
  background: @neutral-white-10;
  border-radius: @border-radius-md;
}
</style>

<template>
  <a-drawer
    :open="open"
    width="min(1097px, 96vw)"
    :closable="false"
    :body-style="{ padding: 0 }"
    root-class-name="task-documents-drawer"
    @close="close"
  >
    <TaskDocumentsPanel
      v-if="open && taskId"
      :task-id="taskId"
      :document-id="documentId"
      mode="drawer"
      @open-detail="onOpenDetail"
      @update:document-id="$emit('update:documentId', $event)"
    />
  </a-drawer>
  <!-- 文档抽屉从沟通记录点开，任务详情同样默认落在 Agent 页签；收起只隐藏，再点直接显示。 -->
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
</template>

<script setup>
import { inject, ref } from "vue";
import RequirementDetailDrawer from "@/components/common/RequirementDetailDrawer.vue";
import TaskDocumentsPanel from "./TaskDocumentsPanel.vue";

defineProps({
  open: { type: Boolean, default: false },
  taskId: { type: [Number, String], default: null },
  documentId: { type: [Number, String], default: null },
});

const emit = defineEmits(["update:open", "update:documentId"]);

// 文档抽屉是从详情里点开的（沟通记录里的文档引用）：详情就在下面，收起自己就回到详情。
const detailMode = inject("workItemDetailMode", null);
const detailTarget = ref(null);
const detailOpen = ref(false);

function onOpenDetail(meta) {
  if (!detailMode?.value && meta?.workItemId) {
    detailTarget.value = meta;
    detailOpen.value = true;
  }
  close();
}

function close() {
  emit("update:open", false);
}
</script>

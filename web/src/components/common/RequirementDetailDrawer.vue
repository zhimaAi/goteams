<template>
  <component
    :is="detailContainer"
    v-bind="detailContainerProps"
    @close="requestClose"
    @cancel="requestClose"
  >
    <template #title><WorkItemDetailHeader :model="model" /></template>
    <WorkItemDetailBody :model="model" />
  </component>
</template>
<script setup>
import { useWorkItemDetail } from "@/composables/useWorkItemDetail";
import WorkItemDetailHeader from "@/components/work-item/WorkItemDetailHeader.vue";
import WorkItemDetailBody from "@/components/work-item/WorkItemDetailBody.vue";
const props = defineProps({
  open: { type: Boolean, default: false },
  wsId: { type: [String, Number], default: "" },
  requirementId: { type: [String, Number], default: "" },
  canEdit: { type: Boolean, default: false },
  canEditByType: { type: Object, default: null },
  canCreateByType: { type: Object, default: null },
  navigationItems: { type: Array, default: () => [] },
  mode: { type: String, default: "drawer" },
  workLogEditorToken: { type: Number, default: 0 },
  workLogEditorRequirementId: { type: [String, Number], default: "" },
  initialTab: { type: String, default: "" },
  // keepAlive：收起抽屉只隐藏，保留已加载内容与当前页签；再次展开直接显示，
  // 不重新请求也不跳回默认页签。产出文档页的「任务详情」按钮用它承载。
  keepAlive: { type: Boolean, default: false },
});
const emit = defineEmits(["close", "navigate", "updated", "loaded"]);
const model = useWorkItemDetail(props, emit);
const { detailContainer, detailContainerProps, requestClose } = model;
</script>
<style scoped lang="less">
@import "@/components/work-item/workItemDetail.less";
</style>

<style lang="less">
@import "@/components/work-item/workItemDetailGlobal.less";
</style>

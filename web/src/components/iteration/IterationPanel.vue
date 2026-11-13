<!-- IterationPanel.vue: shared iteration page used by the board tab and legacy route. -->
<template>
  <div class="iteration-panel">
    <IterationSidebar
      :ws-id="wsId"
      :iterations="iterations"
      :active-id="activeIterationId"
      :can-create="canCreate"
      :can-edit="canEdit"
      :can-delete="canDelete"
      @create="openCreate"
      @select="selectIteration"
      @action="handleAction"
    />
    <IterationWorkItemList
      ref="listRef"
      :ws-id="wsId"
      :iteration-id="activeIterationId"
      :filter-options="filterOptions"
      :refresh-key="listRefreshKey"
      :show-toolbar="showToolbar"
      @open="openDetail"
      @navigation-change="navigationItems = $event"
      @preview-mode-change="previewMode = $event"
    />

    <VersionModal
      v-model:open="createOpen"
      :form="createForm"
      :confirm-loading="saving"
      :workspace-options="workspaceOptions"
      :origin-workspace-id="wsId"
      @ok="createIteration"
    />
    <VersionModal
      v-model:open="editOpen"
      mode="edit"
      :form="editForm"
      :confirm-loading="saving"
      :workspace-options="workspaceOptions"
      :origin-workspace-id="editingIteration?.workspace_id || wsId"
      @ok="updateIteration"
    />
    <RequirementDetailDrawer
      :open="Boolean(detailId)"
      :ws-id="detailWorkspaceId || wsId"
      :requirement-id="detailId"
      :mode="previewMode"
      :navigation-items="navigationItems"
      :can-edit-by-type="{
        requirement: authStore.hasPermission('requirement:edit'),
        defect: authStore.hasPermission('defect:edit'),
      }"
      :can-create-by-type="{
        requirement: authStore.hasPermission('requirement:create'),
        defect: authStore.hasPermission('defect:create'),
      }"
      @close="closeDetail"
      @navigate="navigateDetail"
      @updated="refreshAfterWorkItemChange"
    />
  </div>
</template>

<script setup>
import { computed, ref } from "vue";
import { useIterationManagement } from "@/composables/useIterationManagement";
import RequirementDetailDrawer from "@/components/common/RequirementDetailDrawer.vue";
import VersionModal from "@/views/workspace/board/components/VersionModal.vue";
import IterationSidebar from "./IterationSidebar.vue";
import IterationWorkItemList from "./IterationWorkItemList.vue";

const props = defineProps({
  wsId: { type: [String, Number], required: true },
  showToolbar: { type: Boolean, default: true },
});
const listRef = ref(null);
const {
  authStore,
  iterations,
  activeIterationId,
  filterOptions,
  workspaceOptions,
  createOpen,
  editOpen,
  saving,
  editingIteration,
  listRefreshKey,
  navigationItems,
  detailId,
  detailWorkspaceId,
  previewMode,
  createForm,
  editForm,
  canCreate,
  canEdit,
  canDelete,
  selectIteration,
  openCreate,
  createIteration,
  updateIteration,
  handleAction,
  openDetail,
  closeDetail,
  navigateDetail,
  refreshAfterWorkItemChange,
} = useIterationManagement(props);

defineExpose({
  views: computed(() => listRef.value?.views ?? []),
  currentViewId: computed(() => listRef.value?.currentViewId ?? null),
  searchExpanded: computed(() => listRef.value?.searchExpanded ?? false),
  searchHasValues: computed(() => listRef.value?.searchHasValues ?? false),
  selectView: (viewId) => listRef.value?.selectView(viewId),
  openCreateView: () => listRef.value?.openCreateView(),
  openViewConfig: () => listRef.value?.openViewConfig(),
  toggleSearch: () => listRef.value?.toggleSearch(),
  openListSettings: () => listRef.value?.openListSettings(),
});
</script>

<style scoped lang="less">
.iteration-panel {
  display: flex;
  min-height: 0;
  flex: 1;
  // overflow: hidden;
  border-top: 1px solid #f0f0f0;
}
</style>

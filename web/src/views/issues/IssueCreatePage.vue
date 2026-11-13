<template>
  <div class="create-page">
    <CreateWorkItemHeader
      v-model:type="type"
      v-model:selected-ws="selectedWs"
      v-model:template-id="templateId"
      :workspaces="workspaces"
      :can-create-requirement="canCreateRequirement"
      :can-create-defect="canCreateDefect"
      :show-templates="showTemplates"
      :templates="templates"
      @back="goBoard"
    />

    <div class="create-body">
      <CreateWorkItemEditor
        v-model:title="title"
        v-model:description="description"
        v-model:image-file-ids="imageFileIds"
        v-model:description-mode="descriptionMode"
        :title-invalid="titleInvalid"
        :content-revision="draftRestoreRevision"
        :ws-id="selectedWs"
        @touch-title="titleTouched = true"
      />
      <CreateWorkItemProperties
        v-model:parent-id="parentId"
        v-model:status="status"
        v-model:priority="priority"
        v-model:version-id="versionId"
        v-model:assignee-ids="assigneeIds"
        v-model:planned-start-date="plannedStartDate"
        v-model:planned-end-date="plannedEndDate"
        v-model:estimated-work-hours="estimatedWorkHours"
        v-model:category-id="categoryId"
        v-model:custom-values="customValues"
        :type="type"
        :ws-id="selectedWs"
        :items="propertyItems"
        :statuses="statuses"
        :priorities="priorityOptions"
        :versions="selectableVersionOptions"
        :categories="categoryOptions"
        :members="memberOpts"
        :custom-field-members="customFieldMembers"
      />
    </div>

    <CreateWorkItemActions
      :can-create="canCreateCurrentType"
      :creating="creating"
      :draft="draft"
      :draft-modal-open="draftModalOpen"
      :draft-records="draftRecords"
      @create="submitCreate(false)"
      @continue="submitCreate(true)"
      @cancel="goBoard"
      @open-drafts="openDraftModal"
      @close-drafts="draftModalOpen = false"
      @restore-draft="applyDraftRestore"
    />
  </div>
</template>
<script setup>
import CreateWorkItemHeader from "@/components/work-item/create/CreateWorkItemHeader.vue";
import CreateWorkItemEditor from "@/components/work-item/create/CreateWorkItemEditor.vue";
import CreateWorkItemProperties from "@/components/work-item/create/CreateWorkItemProperties.vue";
import CreateWorkItemActions from "@/components/work-item/create/CreateWorkItemActions.vue";
import { useWorkItemCreate } from "@/composables/useWorkItemCreate";
const {
  selectedWs,
  type,
  draftModalOpen,
  draftRecords,
  draftRestoreRevision,
  draft,
  categoryId,
  title,
  titleTouched,
  titleInvalid,
  description,
  imageFileIds,
  descriptionMode,
  status,
  priority,
  versionId,
  assigneeIds,
  plannedStartDate,
  plannedEndDate,
  estimatedWorkHours,
  parentId,
  creating,
  openDraftModal,
  applyDraftRestore,
  canCreateRequirement,
  canCreateDefect,
  canCreateCurrentType,
  workspaces,
  selectableVersionOptions,
  memberOpts,
  statuses,
  priorityOptions,
  categoryOptions,
  customFieldMembers,
  customValues,
  templates,
  templateId,
  showTemplates,
  propertyItems,
  submitCreate,
  goBoard,
} = useWorkItemCreate();
</script>
<style scoped lang="less">
@import "@/components/work-item/create/createWorkItemPage.less";
</style>

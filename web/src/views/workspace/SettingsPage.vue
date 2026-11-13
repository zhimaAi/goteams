<template>
  <WorkspaceSettingsFrame
    v-slot="{
      section: activeSection,
      wsId,
      selectSection,
      onApplicationState: handleApplicationState,
      loadEnabledApplications,
    }"
  >
    <ProjectInfoTab v-if="activeSection === 'info'" />
    <ProjectMembersTab v-else-if="activeSection === 'members'" :ws-id="wsId" />
    <ProjectRolesTab v-else-if="activeSection === 'roles'" :ws-id="wsId" />
    <ApplicationMarketPanel
      v-else-if="activeSection === 'applications'"
      :ws-id="wsId"
      @application-state="handleApplicationState"
      @configure="selectSection('issue-invite')"
      @changed="loadEnabledApplications"
    />
    <TypeSubTabs
      v-else-if="activeSection === 'requirement'"
      type-key="requirement"
    />
    <TypeSubTabs v-else-if="activeSection === 'defect'" type-key="defect" />
    <TypeSubTabs v-else-if="activeSection === 'planning'" type-key="planning" />
    <SLATab v-else-if="activeSection === 'sla'" :ws-id="wsId" />
    <AutomationTab v-else-if="activeSection === 'automation'" :ws-id="wsId" />
  </WorkspaceSettingsFrame>
</template>
<script setup>
import WorkspaceSettingsFrame from "@/components/work-item/WorkspaceSettingsFrame.vue";
import ProjectInfoTab from "./settings/ProjectInfoTab.vue";
import ProjectMembersTab from "./settings/ProjectMembersTab.vue";
import ProjectRolesTab from "./settings/ProjectRolesTab.vue";
import TypeSubTabs from "./settings/TypeSubTabs.vue";
import SLATab from "./settings/SLATab.vue";
import AutomationTab from "./settings/AutomationTab.vue";
import ApplicationMarketPanel from "./settings/ApplicationMarketPanel.vue";
</script>

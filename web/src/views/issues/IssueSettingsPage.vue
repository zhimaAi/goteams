<template>
  <WorkspaceSettingsFrame v-slot="{ wsId }"
    ><IssueInviteSettings :ws-id="wsId" :app-enabled="true"
  /></WorkspaceSettingsFrame>
</template>
<script setup>
import WorkspaceSettingsFrame from "@/components/work-item/WorkspaceSettingsFrame.vue";
import IssueInviteSettings from "./settings/IssueInviteSettings.vue";
</script>

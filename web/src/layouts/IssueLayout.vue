<template>
  <component :is="standalone ? 'div' : AppLayout">
    <a-spin v-if="loading" />
    <a-result
      v-else-if="failed"
      status="error"
      :title="$t('common_requestFailed')"
      ><template #extra
        ><a-button @click="loadWorkspace">{{
          $t("issue_retry")
        }}</a-button></template
      ></a-result
    >
    <a-empty
      v-else-if="!pageAvailable"
      :description="$t('issue_invite_disabled')"
    />
    <router-view v-else :key="wsId" />
  </component>
</template>
<script setup>
import AppLayout from "./AppLayout.vue";

import { computed, onBeforeUnmount, provide, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { getWorkspaceMetadata } from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";
import { useIssueInviteStore } from "@/stores/issueInvite";
import { usesIssuePages } from "@/utils/workspaceType";
import {
  syncWorkspaceApplications,
  useWorkspaceApplications,
} from "@/composables/useWorkspaceApplications";

defineProps({ standalone: { type: Boolean, default: false } });

const route = useRoute();
const authStore = useAuthStore();
const inviteStore = useIssueInviteStore();
const wsId = computed(() => String(route.params.wsId || ""));

const metadata = ref(null);
const loading = ref(true);
const failed = ref(false);
const { applications } = useWorkspaceApplications(wsId);
const pageAvailable = computed(() => {
  const internalPages = [
    "IssueList",
    "IssueCreate",
    "IssueDetail",
    "IssueHistoryDetail",
    "IssueDashboard",
    "IssueKnowledge",
    "IssuePlanning",
    "IssueIteration",
    "IssueGantt",
  ];
  return internalPages.includes(route.name)
    ? usesIssuePages(
        metadata.value?.workspace?.workspace_type,
        applications.value,
      )
    : applications.value.includes("issue");
});
provide("issueWorkspaceMetadata", metadata);
let sequence = 0;
async function loadWorkspace() {
  const current = ++sequence;
  loading.value = true;
  failed.value = false;
  metadata.value = null;
  inviteStore.reset();
  try {
    const result =
      authStore.peekProjectMetadata(wsId.value) ||
      (await getWorkspaceMetadata(wsId.value));
    if (current !== sequence) return;
    metadata.value = result;
    syncWorkspaceApplications(
      wsId.value,
      result?.enabled_applications || [],
      result?.workspace?.workspace_type || "",
    );
  } catch {
    if (current === sequence) failed.value = true;
  } finally {
    if (current === sequence) loading.value = false;
  }
}
watch(wsId, loadWorkspace, { immediate: true });
onBeforeUnmount(() => {
  sequence += 1;
  inviteStore.reset();
});
</script>

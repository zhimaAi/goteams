<!--
 * EnterpriseMemberPickerModal.vue
 * Enterprise-scoped member picker for settings that are not tied to a workspace, e.g. external
 * notification strategies. Unlike the project picker it never adds anyone anywhere — it only
 * reports the picked member ids, and it reads departments and roles from the enterprise APIs so
 * it does not need the project_member:add permission the workspace candidate endpoint requires.
 -->
<template>
  <a-modal
    :open="open"
    :title="$t('external_notification_select_members')"
    width="800px"
    wrap-class-name="member-picker-modal"
    :destroy-on-close="true"
    @update:open="handleOpenChange"
  >
    <template #footer>
      <a-button @click="handleOpenChange(false)">{{
        $t("common_cancel")
      }}</a-button>
      <a-button type="primary" @click="handleConfirm">{{
        $t("common_confirm")
      }}</a-button>
    </template>
    <MemberPickerPanel
      v-model:selected-ids="draftIds"
      :loading="loading"
      :enterprise-name="enterpriseName"
      :departments="departments"
      :enterprise-roles="enterpriseRoles"
      :members="members"
    />
  </a-modal>
</template>

<script setup>
import { ref, watch } from "vue";
import {
  listEnterpriseDepartments,
  listEnterpriseRoles,
} from "@/api/workspace";
import MemberPickerPanel from "@/components/common/MemberPickerPanel.vue";

const props = defineProps({
  open: { type: Boolean, default: false },
  // Every valid enterprise member; the caller already loads them for its own member labels.
  members: { type: Array, default: () => [] },
  // Ids already picked; reopening resumes from them so members can be added incrementally.
  selectedIds: { type: Array, default: () => [] },
});

const emit = defineEmits({
  "update:open": (value) => typeof value === "boolean",
  confirm: (ids) => Array.isArray(ids),
});

const loading = ref(false);
const draftIds = ref([]);
const enterpriseName = ref("");
const departments = ref([]);
const enterpriseRoles = ref([]);

async function loadSourceData() {
  loading.value = true;
  try {
    const [departmentData, roleList] = await Promise.all([
      listEnterpriseDepartments(),
      listEnterpriseRoles(),
    ]);
    enterpriseName.value = departmentData?.enterprise_name || "";
    departments.value = departmentData?.items || [];
    enterpriseRoles.value = roleList || [];
  } catch {
    enterpriseName.value = "";
    departments.value = [];
    enterpriseRoles.value = [];
  } finally {
    loading.value = false;
  }
}

function handleConfirm() {
  emit("confirm", [...draftIds.value]);
  handleOpenChange(false);
}

function handleOpenChange(value) {
  emit("update:open", value);
}

watch(
  () => props.open,
  async (open) => {
    if (!open) return;
    draftIds.value = props.selectedIds.map((id) => String(id));
    await loadSourceData();
  },
);
</script>

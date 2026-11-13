<!--
 * AddProjectMembersModal.vue
 * Project member entry point: the shared enterprise picker plus the project-specific
 * add (every picked member joins with the caller's project role).
-->
<template>
  <EnterpriseMemberPickerModal
    :open="open"
    :candidates="loadCandidates"
    :saving="saving"
    @update:open="(value) => emit('update:open', value)"
    @confirm="handleConfirm"
  />
</template>

<script setup>
import { ref } from "vue";
import { message } from "ant-design-vue";
import { useI18n } from "vue-i18n";
import EnterpriseMemberPickerModal from "@/components/member/EnterpriseMemberPickerModal.vue";
import { addMember, listMemberCandidates } from "@/api/workspace";

const props = defineProps({
  open: { type: Boolean, default: false },
  wsId: { type: [String, Number], default: "" },
  // Project role every picked member is added with; the entry point decides it.
  roleId: { type: [String, Number], default: "" },
});

const emit = defineEmits({
  "update:open": (value) => typeof value === "boolean",
  // Members were added; the payload is how many joined.
  added: (count) => typeof count === "number",
});

const { t } = useI18n();
const saving = ref(false);

function loadCandidates() {
  return listMemberCandidates(props.wsId);
}

async function handleConfirm(userIds) {
  if (!userIds.length) return;
  saving.value = true;
  try {
    const res = await addMember(props.wsId, {
      userIds,
      roleId: Number(props.roleId),
    });
    const added = res?.added ?? userIds.length;
    message.success(t("workspace_addedMembers", { n: added }));
    emit("added", Number(added));
    emit("update:open", false);
  } finally {
    saving.value = false;
  }
}
</script>

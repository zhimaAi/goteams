<template>
  <a-modal
    :open="open"
    :title="$t('requirement_batchAssignee')"
    :confirm-loading="confirmLoading"
    @ok="$emit('ok')"
    @cancel="handleCancel"
  >
    <a-select
      v-model:value="selectedIds"
      mode="multiple"
      allow-clear
      show-search
      style="width: 100%"
      :placeholder="$t('workspace_selectMembers')"
      :filter-option="filterMemberOption"
    >
      <a-select-option
        v-for="member in members"
        :key="member.user_id"
        :value="Number(member.user_id)"
        :label="member.user_name"
      >
        {{ member.user_name }}
      </a-select-option>
    </a-select>
  </a-modal>
</template>

<script setup>
import { computed } from "vue";

const props = defineProps({
  open: { type: Boolean, default: false },
  members: { type: Array, default: () => [] },
  assigneeIds: { type: Array, default: () => [] },
  confirmLoading: { type: Boolean, default: false },
});
const emit = defineEmits(["update:open", "update:assigneeIds", "ok", "cancel"]);

const selectedIds = computed({
  get: () => props.assigneeIds,
  set: (value) => emit("update:assigneeIds", value),
});

function filterMemberOption(input, option) {
  return String(option?.label || "")
    .toLowerCase()
    .includes(String(input || "").toLowerCase());
}

function handleCancel() {
  emit("update:open", false);
  emit("cancel");
}
</script>

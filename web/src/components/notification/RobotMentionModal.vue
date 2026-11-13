<template>
  <a-modal
    :open="open"
    :title="$t('automation_selectAtMembers')"
    @cancel="emit('update:open', false)"
    @ok="confirm"
  >
    <a-radio-group v-model:value="tab" button-style="solid">
      <a-radio-button value="users">{{
        $t("automation_atSpecified")
      }}</a-radio-button>
      <a-radio-button value="roles">{{
        $t("automation_atRelated")
      }}</a-radio-button>
    </a-radio-group>
    <a-select
      v-if="tab === 'users'"
      v-model:value="users"
      class="at-select"
      mode="multiple"
      option-filter-prop="label"
      :options="[{ value: 'all', label: $t('automation_atAll') }, ...members]"
      :placeholder="$t('automation_atSpecified')"
    />
    <a-select
      v-else
      v-model:value="roles"
      class="at-select"
      mode="multiple"
      option-filter-prop="label"
      :options="[
        { value: 'created_by', label: $t('automation_atRoleCreatedBy') },
        { value: 'assignee', label: $t('automation_atRoleAssignee') },
      ]"
      :placeholder="$t('automation_atRelated')"
    />
  </a-modal>
</template>
<script setup>
import { ref, watch } from "vue";
const props = defineProps({
  open: Boolean,
  value: { type: Object, default: () => ({}) },
  members: { type: Array, default: () => [] },
});
const emit = defineEmits(["update:open", "confirm"]);
const tab = ref("users"),
  users = ref([]),
  roles = ref([]);
watch(
  () => props.open,
  (open) => {
    if (!open) return;
    tab.value = "users";
    users.value = [
      ...(props.value.at_all ? ["all"] : []),
      ...(props.value.at_user_ids || []),
    ];
    roles.value = [...(props.value.at_roles || [])];
  },
  { immediate: true },
);
function confirm() {
  emit("confirm", {
    at_all: users.value.includes("all"),
    at_user_ids: users.value.filter((id) => id !== "all").map(Number),
    at_roles: [...roles.value],
  });
  emit("update:open", false);
}
</script>
<style scoped lang="less">
.at-select {
  display: block;
  width: 100%;
  margin-top: 16px;
}
</style>

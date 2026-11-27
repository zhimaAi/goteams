<template>
  <a-select
    class="role-select"
    :value="selectedValues"
    mode="multiple"
    show-search
    show-arrow
    :options="options"
    :filter-option="filterOption"
    :max-tag-count="2"
    :placeholder="$t('settings_editRolesAll')"
    :disabled="disabled || saving"
    :loading="saving"
    :title="selectedNames"
    :aria-label="$t('settings_editRoles')"
    @change="onChange"
    @blur="saveSelection"
  />
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import { saveFieldEditRoles } from "@/api/workspace";

const props = defineProps({
  wsId: { type: [String, Number], required: true },
  typeKey: { type: String, required: true },
  field: { type: Object, required: true },
  roles: { type: Array, default: () => [] },
  disabled: Boolean,
});
const emit = defineEmits(["saved"]);
const { t, locale } = useI18n();
const selectedValues = ref([]);
const saving = ref(false);
const roleNames = computed(
  () => new Map(props.roles.map((role) => [Number(role.id), role.name])),
);
const options = computed(() => [
  ...props.roles.map((role) => ({ value: Number(role.id), label: role.name })),
  ...(props.field.edit_role_ids || [])
    .filter((id) => !roleNames.value.has(Number(id)))
    .map((id) => ({
      value: Number(id),
      label: t("settings_editRoleDeleted"),
      disabled: true,
    })),
]);
const selectedNames = computed(() =>
  !selectedValues.value.length
    ? t("settings_editRolesAll")
    : selectedValues.value
        .map(
          (id) =>
            roleNames.value.get(Number(id)) || t("settings_editRoleDeleted"),
        )
        .join(locale.value.startsWith("zh") ? "、" : ", "),
);

watch(
  () => [props.field.edit_scope, props.field.edit_role_ids],
  () => {
    selectedValues.value =
      props.field.edit_scope === "all"
        ? []
        : (props.field.edit_role_ids || []).map(Number);
  },
  { immediate: true, deep: true },
);

function filterOption(input, option) {
  return String(option.label).toLowerCase().includes(input.toLowerCase());
}

function onChange(values) {
  if (props.disabled || saving.value) return;
  selectedValues.value = values.filter((id) => roleNames.value.has(Number(id)));
}

async function saveSelection() {
  if (props.disabled || saving.value) return;
  const previous =
    props.field.edit_scope === "all"
      ? []
      : (props.field.edit_role_ids || []).map(Number);
  const next = [...selectedValues.value];
  if (
    next.length === previous.length &&
    next.every((id) => previous.includes(id))
  ) {
    return;
  }

  const policy = {
    edit_scope: next.length ? "roles" : "all",
    edit_role_ids: next.map(Number),
  };
  saving.value = true;
  try {
    await saveFieldEditRoles(props.wsId, props.typeKey, {
      kind: props.field.kind,
      key: props.field.key,
      id: props.field.id || 0,
      ...policy,
    });
    emit("saved", policy);
    message.success(t("common_saveSuccess"));
  } catch {
    // The request interceptor displays failures; restore the last saved selection.
    selectedValues.value = previous;
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped lang="less">
.role-select {
  width: 188px;
  max-width: 100%;
  text-align: left;
  :deep(.ant-select-selector) {
    min-height: 36px;
    border-radius: 6px;
    box-shadow: none;
  }
}
</style>

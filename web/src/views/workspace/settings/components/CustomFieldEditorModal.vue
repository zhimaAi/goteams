<!--
 * CustomFieldEditorModal.vue
 * Create / edit dialog for a workspace custom field.
 -->
<template>
  <a-modal
    :open="open"
    :title="editingId ? $t('settings_editField') : $t('settings_addField')"
    :ok-text="$t('common_save')"
    :cancel-text="$t('common_cancel')"
    width="560px"
    :confirm-loading="saving"
    destroy-on-close
    @ok="handleSave"
    @cancel="emit('update:open', false)"
  >
    <a-form layout="vertical" style="margin-top: 16px">
      <a-form-item
        :label="$t('settings_fieldName')"
        required
        :validate-status="errors.name ? 'error' : undefined"
        :help="errors.name"
      >
        <a-input
          ref="nameInputRef"
          v-model:value="name"
          :placeholder="$t('settings_fieldNamePlaceholder')"
          :maxlength="20"
        />
      </a-form-item>
      <a-form-item :label="$t('settings_fieldType')" required>
        <a-select v-model:value="fieldType" :options="typeOptions" />
      </a-form-item>
      <a-form-item
        v-if="showOptions"
        :label="$t('settings_fieldOptions')"
        :extra="$t('settings_fieldOptionsHint')"
        required
        :validate-status="errors.options ? 'error' : undefined"
        :help="errors.options"
      >
        <a-textarea
          ref="optionsInputRef"
          :value="optionsText"
          :rows="4"
          :placeholder="$t('settings_fieldOptionsPlaceholder')"
          @update:value="handleOptionsTextChange"
        />
      </a-form-item>
      <a-form-item
        v-if="fieldType === CUSTOM_FIELD_TYPE.NUMERIC"
        :label="$t('settings_fieldIntegerOnly')"
      >
        <a-switch v-model:checked="integerOnly" />
      </a-form-item>
      <template v-if="showMemberScope">
        <a-form-item :label="$t('settings_fieldMemberScope')" required>
          <a-radio-group v-model:value="memberScope">
            <a-radio :value="CUSTOM_FIELD_MEMBER_SCOPE.ALL">{{
              $t("settings_fieldMemberScopeAll")
            }}</a-radio>
            <a-radio :value="CUSTOM_FIELD_MEMBER_SCOPE.ROLES">{{
              $t("settings_fieldMemberScopeRoles")
            }}</a-radio>
            <a-radio :value="CUSTOM_FIELD_MEMBER_SCOPE.WORKSPACE">{{
              $t("settings_fieldMemberScopeWorkspace")
            }}</a-radio>
          </a-radio-group>
        </a-form-item>
        <a-form-item
          v-if="memberScope === CUSTOM_FIELD_MEMBER_SCOPE.ROLES"
          :label="$t('settings_fieldEnterpriseRoles')"
          required
          :validate-status="errors.roles ? 'error' : undefined"
          :help="errors.roles"
        >
          <a-select
            ref="roleSelectRef"
            v-model:value="roleIds"
            mode="multiple"
            show-search
            option-filter-prop="label"
            :placeholder="$t('settings_fieldEnterpriseRolesPlaceholder')"
            :options="roleOptions"
            :loading="rolesLoading"
          />
        </a-form-item>
      </template>
      <CascaderTreeEditor
        v-if="showCascader"
        ref="cascaderEditorRef"
        v-model="cascader"
        :error="errors.cascader"
      />
    </a-form>
  </a-modal>
</template>

<script setup>
import { computed, nextTick, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import {
  createCustomField,
  listCustomFieldEnterpriseRoles,
  updateCustomField,
} from "@/api/workspace";
import {
  CUSTOM_FIELD_CASCADER_NAME_MAX_LENGTH,
  CUSTOM_FIELD_MEMBER_SCOPE,
  CUSTOM_FIELD_TYPE,
  CUSTOM_FIELD_TYPE_GROUPS,
  CUSTOM_FIELD_TYPE_LABELS,
  isCascaderFieldType,
  isMemberFieldType,
  isOptionFieldType,
} from "@/constants/customField";
import CascaderTreeEditor from "./CascaderTreeEditor.vue";

const props = defineProps({
  open: { type: Boolean, default: false },
  wsId: { type: [String, Number], default: "" },
  typeKey: { type: String, default: "requirement" },
  field: { type: Object, default: null },
});

const emit = defineEmits({
  "update:open": (open) => true,
  saved: () => true,
});

const { t } = useI18n();
const saving = ref(false);
const name = ref("");
const fieldType = ref(CUSTOM_FIELD_TYPE.TEXT);
const optionsText = ref("");
const optionDrafts = ref([]);
const integerOnly = ref(false);
const memberScope = ref(CUSTOM_FIELD_MEMBER_SCOPE.ALL);
const roleIds = ref([]);
const cascader = ref([]);
const errors = ref({});
const roleOptions = ref([]);
const rolesLoading = ref(false);
const nameInputRef = ref(null);
const optionsInputRef = ref(null);
const roleSelectRef = ref(null);
const cascaderEditorRef = ref(null);

const editingId = computed(() => props.field?.id || null);
const showOptions = computed(() => isOptionFieldType(fieldType.value));
const showMemberScope = computed(() => isMemberFieldType(fieldType.value));
const showCascader = computed(() => isCascaderFieldType(fieldType.value));
const typeOptions = computed(() =>
  CUSTOM_FIELD_TYPE_GROUPS.map((group) => ({
    label: t(group.labelKey),
    options: group.options.map((value) => ({
      value,
      label: t(CUSTOM_FIELD_TYPE_LABELS[value]),
    })),
  })),
);

watch([fieldType, memberScope], ([type, scope]) => {
  if (
    props.open &&
    isMemberFieldType(type) &&
    scope === CUSTOM_FIELD_MEMBER_SCOPE.ROLES
  ) {
    void loadRoles();
  }
});

watch(
  () => props.open,
  async (open) => {
    if (!open) return;
    errors.value = {};
    const field = props.field;
    name.value = field?.name || "";
    fieldType.value = field?.field_type || CUSTOM_FIELD_TYPE.TEXT;
    optionDrafts.value = (field?.options || []).map((option) => ({
      key: option.key || "",
      name: option.name || "",
    }));
    optionsText.value = optionDrafts.value
      .map((option) => option.name)
      .join("\n");
    integerOnly.value = Number(field?.integer_only) === 1;
    memberScope.value = field?.member_scope || CUSTOM_FIELD_MEMBER_SCOPE.ALL;
    roleIds.value = (field?.role_ids || []).map(Number);
    cascader.value = cloneCascader(field?.cascader || []);
    if (
      isMemberFieldType(fieldType.value) &&
      memberScope.value === CUSTOM_FIELD_MEMBER_SCOPE.ROLES
    ) {
      await loadRoles();
    }
  },
);

async function loadRoles() {
  if (roleOptions.value.length || rolesLoading.value) return;
  rolesLoading.value = true;
  try {
    const roles = await listCustomFieldEnterpriseRoles(
      props.wsId,
      props.typeKey,
    );
    roleOptions.value = (roles || []).map((role) => ({
      value: Number(role.id),
      label: role.name,
    }));
  } catch {
    roleOptions.value = [];
  } finally {
    rolesLoading.value = false;
  }
}

function cloneCascader(nodes) {
  return (nodes || []).map((node) => ({
    key: node.key || "",
    name: node.name || "",
    children: cloneCascader(node.children || []),
  }));
}

function optionNames(value) {
  return String(value || "")
    .split("\n")
    .map((option) => option.trim())
    .filter(Boolean);
}

function reconcileOptionDrafts(value) {
  const previous = optionDrafts.value;
  const used = new Set();
  const matched = optionNames(value).map((name) => {
    const previousIndex = previous.findIndex(
      (option, candidate) => !used.has(candidate) && option.name === name,
    );
    if (previousIndex < 0) return { key: null, name };
    used.add(previousIndex);
    return { key: previous[previousIndex].key || "", name };
  });
  const renamed = previous.filter((_, index) => !used.has(index));
  let renamedIndex = 0;
  return matched.map((option) => {
    if (option.key !== null) return option;
    const old = renamed[renamedIndex++];
    return { key: old?.key || "", name: option.name };
  });
}

function handleOptionsTextChange(value) {
  optionDrafts.value = reconcileOptionDrafts(value);
  optionsText.value = value;
}

function parsedOptions() {
  return optionDrafts.value;
}

function cascaderHasEmptyName(nodes) {
  return (nodes || []).some(
    (node) =>
      !String(node.name || "").trim() || cascaderHasEmptyName(node.children),
  );
}

function cascaderHasLongName(nodes) {
  return (nodes || []).some(
    (node) =>
      Array.from(String(node.name || "").trim()).length >
        CUSTOM_FIELD_CASCADER_NAME_MAX_LENGTH ||
      cascaderHasLongName(node.children),
  );
}

function validate() {
  const next = {};
  if (!name.value.trim()) next.name = t("settings_fieldNameRequired");
  if (showOptions.value && !parsedOptions().length) {
    next.options = t("settings_fieldOptionsRequired");
  }
  if (
    showMemberScope.value &&
    memberScope.value === CUSTOM_FIELD_MEMBER_SCOPE.ROLES &&
    !roleIds.value.length
  ) {
    next.roles = t("settings_fieldRolesRequired");
  }
  if (showCascader.value) {
    if (!cascader.value.length)
      next.cascader = t("settings_fieldCascaderRequired");
    else if (cascaderHasEmptyName(cascader.value)) {
      next.cascader = t("settings_fieldCascaderNameRequired");
    } else if (cascaderHasLongName(cascader.value)) {
      next.cascader = t("settings_fieldCascaderNameTooLong", {
        max: CUSTOM_FIELD_CASCADER_NAME_MAX_LENGTH,
      });
    }
  }
  errors.value = next;
  if (Object.keys(next).length) {
    void focusFirstError(next);
    return false;
  }
  return true;
}

async function focusFirstError(currentErrors) {
  await nextTick();
  if (currentErrors.name) nameInputRef.value?.focus?.();
  else if (currentErrors.options) optionsInputRef.value?.focus?.();
  else if (currentErrors.roles) roleSelectRef.value?.focus?.();
  else if (currentErrors.cascader) cascaderEditorRef.value?.focus?.();
}

function buildPayload() {
  return {
    name: name.value.trim(),
    field_type: fieldType.value,
    is_required: props.field?.is_required ?? 0,
    is_enabled: props.field?.is_enabled ?? 1,
    options: showOptions.value ? parsedOptions() : [],
    integer_only:
      fieldType.value === CUSTOM_FIELD_TYPE.NUMERIC && integerOnly.value
        ? 1
        : 0,
    member_scope: showMemberScope.value
      ? memberScope.value
      : CUSTOM_FIELD_MEMBER_SCOPE.ALL,
    role_ids:
      showMemberScope.value &&
      memberScope.value === CUSTOM_FIELD_MEMBER_SCOPE.ROLES
        ? roleIds.value
        : [],
    cascader: showCascader.value ? cascader.value : [],
  };
}

async function handleSave() {
  if (!validate()) return;
  saving.value = true;
  try {
    const payload = buildPayload();
    if (editingId.value) {
      await updateCustomField(
        props.wsId,
        props.typeKey,
        editingId.value,
        payload,
      );
      message.success(t("settings_fieldUpdated"));
    } else {
      await createCustomField(props.wsId, props.typeKey, payload);
      message.success(t("settings_fieldCreated"));
    }
    emit("update:open", false);
    emit("saved");
  } catch {
    // The request interceptor displays the failure message.
  } finally {
    saving.value = false;
  }
}
</script>

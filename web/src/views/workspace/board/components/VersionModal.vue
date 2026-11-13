<!-- VersionModal.vue: creates or edits a single/cross-workspace iteration. -->
<template>
  <a-modal
    :open="open"
    :title="title"
    :confirm-loading="confirmLoading"
    :ok-text="okText"
    :cancel-text="$t('common_cancel')"
    @ok="$emit('ok')"
    @cancel="handleCancel"
  >
    <a-form layout="vertical">
      <a-form-item :label="$t('workspace_versionName')" required>
        <a-input
          v-model:value="form.name"
          :placeholder="$t('workspace_versionNamePlaceholder')"
        />
      </a-form-item>
      <a-form-item :label="$t('iteration_type')" required>
        <a-radio-group
          v-model:value="form.iterationType"
          button-style="solid"
          :disabled="mode === 'edit'"
        >
          <a-radio-button value="single">
            {{ $t("iteration_typeSingle") }}
          </a-radio-button>
          <a-radio-button value="cross">
            {{ $t("iteration_typeCross") }}
          </a-radio-button>
        </a-radio-group>
      </a-form-item>
      <a-form-item
        v-if="form.iterationType === 'cross'"
        :label="$t('iteration_associatedWorkspaces')"
        required
      >
        <a-select
          v-model:value="form.workspaceIds"
          mode="multiple"
          :placeholder="$t('iteration_selectWorkspaces')"
          :max-tag-count="3"
          show-search
          option-filter-prop="label"
        >
          <a-select-option
            v-for="workspace in workspaceOptions"
            :key="workspace.id"
            :value="Number(workspace.id)"
            :label="workspace.name"
            :disabled="Number(workspace.id) === Number(originWorkspaceId)"
          >
            {{ workspace.name }}
            <span
              v-if="Number(workspace.id) === Number(originWorkspaceId)"
              class="origin-hint"
            >
              {{ originHint }}
            </span>
          </a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item :label="$t('workspace_startDate')">
        <a-date-picker v-model:value="form.startDate" style="width: 100%" />
      </a-form-item>
      <a-form-item :label="$t('workspace_endDate')">
        <a-date-picker v-model:value="form.endDate" style="width: 100%" />
      </a-form-item>
    </a-form>
  </a-modal>
</template>

<script setup>
import { computed } from "vue";
import { useI18n } from "vue-i18n";

const props = defineProps({
  open: { type: Boolean, default: false },
  mode: { type: String, default: "create" },
  form: { type: Object, required: true },
  confirmLoading: { type: Boolean, default: false },
  workspaceOptions: { type: Array, default: () => [] },
  originWorkspaceId: { type: [String, Number], required: true },
});
const emit = defineEmits({
  "update:open": (open) => typeof open === "boolean",
  ok: () => true,
  cancel: () => true,
});
const { t } = useI18n();

const title = computed(() =>
  t(props.mode === "edit" ? "workspace_editVersion" : "workspace_newVersion"),
);
const okText = computed(() =>
  t(props.mode === "edit" ? "common_save" : "common_create"),
);
// Creating always starts from the current project, while editing can be opened from
// another project sharing the same cross-project iteration.
const originHint = computed(() =>
  t(
    props.mode === "edit"
      ? "iteration_originWorkspace"
      : "iteration_currentWorkspace",
  ),
);

function handleCancel() {
  emit("update:open", false);
  emit("cancel");
}
</script>

<style scoped lang="less">
.origin-hint {
  margin-left: 8px;
  color: rgba(0, 0, 0, 0.45);
}
</style>

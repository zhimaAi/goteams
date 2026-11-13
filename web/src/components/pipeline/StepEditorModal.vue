<template>
  <a-modal
    :open="open"
    :title="modalTitle"
    width="730px"
    centered
    class="step-editor-modal"
    @update:open="handleOpenChange"
  >
    <div class="agent-avatar-preview">
      <img :src="form.avatar" alt="" />
    </div>

    <a-form class="step-editor-form" layout="vertical" :model="form">
      <a-form-item class="step-editor-form-item" :label="$t('pipeline_step_avatar_label')">
        <AvatarPicker v-model="form.avatar" :options="PIPELINE_STEP_AVATARS" />
      </a-form-item>

      <a-form-item class="step-editor-form-item">
        <template #label>
          <span class="step-modal-label">{{ $t("pipeline_flow_name") }}</span>
          <span class="step-modal-label-hint">{{ $t("agent_editor_name_limit") }}</span>
        </template>
        <a-input
          v-model:value="form.name"
          :maxlength="nameMaxLength"
          :show-count="false"
          :placeholder="$t('pipeline_step_name_placeholder')"
        />
      </a-form-item>

      <a-form-item class="step-editor-form-item step-prompt-form-item">
        <template #label>
          <span class="step-modal-label">{{ $t("pipeline_step_prompt_label") }}</span>
          <span class="step-modal-label-hint step-modal-label-hint--flush">{{
            $t("agent_editor_prompt_hint")
          }}</span>
        </template>
        <a-textarea
          v-model:value="form.prompt_content"
          :maxlength="2000"
          :show-count="false"
          :placeholder="$t('pipeline_step_prompt_placeholder')"
        />
      </a-form-item>
    </a-form>

    <template #footer>
      <div class="step-modal-footer-actions">
        <a-button @click="handleOpenChange(false)">{{ $t("common_cancel") }}</a-button>
        <a-button
          type="primary"
          :loading="loading"
          @click="handleSubmit"
        >
          {{ $t("common_ok") }}
        </a-button>
      </div>
    </template>
  </a-modal>
</template>

<script setup>
import { computed, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import AvatarPicker from "@/components/pipeline/AvatarPicker.vue";
import { PIPELINE_STEP_AVATARS } from "@/components/pipeline/avatars";

function defaultStepAvatar(index) {
  const i = Math.max(Number(index) || 0, 0);
  return PIPELINE_STEP_AVATARS[i % PIPELINE_STEP_AVATARS.length];
}

const props = defineProps({
  open: { type: Boolean, default: false },
  step: { type: Object, default: null },
  index: { type: Number, default: 0 },
  loading: { type: Boolean, default: false },
});

const emit = defineEmits(["update:open", "save"]);

const { t: $t } = useI18n();

const NAME_LIMIT = 20;

const isEdit = computed(() => !!props.step);
const modalTitle = computed(() =>
  isEdit.value ? $t("pipeline_step_edit_title") : $t("pipeline_step_add_title"),
);
const nameMaxLength = ref(NAME_LIMIT);

const form = reactive({
  name: "",
  avatar: "",
  prompt_content: "",
});

watch(
  () => props.open,
  (visible) => {
    if (!visible) return;
    const name = (props.step?.name || "").trim();
    form.name = name;
    form.avatar = props.step?.avatar || defaultStepAvatar(props.index);
    form.prompt_content = props.step?.prompt_content || "";
    // 已有名称长于 20 字时不截断，新建仍限制 20 字。
    nameMaxLength.value = Math.max(NAME_LIMIT, name.length);
  },
  { immediate: true },
);

function handleOpenChange(open) {
  emit("update:open", open);
}

function handleSubmit() {
  if (!form.name.trim()) return;
  if (!form.prompt_content.trim()) {
    message.warning($t("pipeline_step_prompt_required"));
    return;
  }
  emit("save", {
    name: form.name.trim(),
    avatar: form.avatar,
    prompt_content: form.prompt_content.trim(),
    remark: (props.step?.remark || "").trim(),
  });
}
</script>

<style scoped>
.step-modal-footer-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.agent-avatar-preview {
  display: flex;
  width: 60px;
  height: 60px;
  align-items: center;
  justify-content: center;
  margin: 0 auto;
  border-radius: 50%;
  background: #f2f3f5;
}

.agent-avatar-preview img {
  display: block;
  width: 60px;
  height: 60px;
  border-radius: 50%;
  object-fit: cover;
}

.step-editor-form {
  margin-top: 24px;
}

.step-editor-form-item,
.step-prompt-form-item {
  margin-bottom: 20px;
}

.step-prompt-form-item {
  margin-bottom: 0;
}

.step-modal-label {
  color: #262626;
  font-size: 14px;
  font-weight: 600;
}

.step-modal-label-hint {
  margin-left: 4px;
  color: #8c8c8c;
  font-weight: 400;
}

.step-modal-label-hint--flush {
  margin-left: 0;
}

.step-modal-required {
  margin-left: 4px;
  color: #ff4d4f;
}

:deep(.step-editor-modal) {
  max-width: calc(100vw - 32px);
  padding-bottom: 0;
}

:deep(.step-editor-modal .ant-modal-content) {
  overflow: hidden;
  padding: 0;
  border-radius: 24px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.16);
}

:deep(.step-editor-modal .ant-modal-header) {
  margin-bottom: 0;
  padding: 24px;
  border-radius: 24px 24px 0 0;
  background: #fff;
}

:deep(.step-editor-modal .ant-modal-title) {
  color: #262626;
  font-size: 24px;
  font-weight: 600;
  line-height: 32px;
}

:deep(.step-editor-modal .ant-modal-body) {
  max-height: calc(100vh - 184px);
  overflow-y: auto;
  padding: 16px 48px 24px;
}

:deep(.step-editor-modal .ant-modal-footer) {
  margin-top: 0;
  padding: 16px 48px;
  border-top: 1px solid #d9d9d9;
}

:deep(.step-editor-modal .ant-modal-footer .ant-btn) {
  min-width: 65px;
  height: 32px;
  padding: 5px 16px;
  border-radius: 6px;
  font-size: 14px;
  line-height: 22px;
}

:deep(.step-editor-form .ant-form-item-label) {
  padding-bottom: 8px;
}

:deep(.step-editor-form .ant-form-item-label > label) {
  height: 22px;
  color: #262626;
  font-size: 14px;
  font-weight: 600;
  line-height: 22px;
}

:deep(.step-editor-form .ant-input),
:deep(.step-editor-form .ant-input-affix-wrapper) {
  min-height: 40px;
  border-color: #d9d9d9 !important;
  border-radius: 12px !important;
  color: #262626;
  font-size: 14px;
  line-height: 22px;
}

:deep(.step-editor-form .ant-input) {
  padding: 8px 12px;
}

:deep(.step-editor-form textarea.ant-input) {
  height: 128px;
  min-height: 128px;
  resize: none;
}

:deep(.step-editor-form .ant-input:hover) {
  border-color: #3157e2 !important;
}

:deep(.step-editor-form .ant-input:focus) {
  border-color: #3157e2 !important;
  box-shadow: 0 0 0 2px rgba(49, 87, 226, 0.15) !important;
}

@media (max-width: 760px) {
  :deep(.step-editor-modal .ant-modal-header) {
    padding: 20px 24px;
  }

  :deep(.step-editor-modal .ant-modal-title) {
    font-size: 20px;
    line-height: 28px;
  }

  :deep(.step-editor-modal .ant-modal-body) {
    padding: 16px 24px 24px;
  }

  :deep(.step-editor-modal .ant-modal-footer) {
    padding: 16px 24px;
  }
}
</style>

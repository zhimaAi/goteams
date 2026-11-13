<template>
  <a-form
    class="agent-basic-form"
    layout="vertical"
    :model="modelValue"
    @finish="handleSubmit"
  >
    <div class="agent-avatar-section">
      <button
        type="button"
        class="agent-avatar-button"
        :aria-label="$t('pipeline_uploadIcon')"
        @click="triggerUpload"
      >
        <img :src="shownIcon" alt="" class="agent-avatar-image" />
        <span class="agent-camera-badge"><CameraOutlined /></span>
      </button>
      <p class="agent-avatar-hint">{{ $t('pipeline_iconHint') }}</p>
      <input
        ref="fileInput"
        type="file"
        accept="image/jpeg,image/png"
        hidden
        @change="handleFileChange"
      />
    </div>

    <a-form-item class="agent-basic-form-item" :label="$t('pipeline_default_avatar')">
      <AvatarPicker
        :model-value="selectedPreset"
        :options="PIPELINE_AVATARS"
        @update:model-value="selectPreset"
      />
    </a-form-item>

    <a-form-item
      class="agent-basic-form-item"
      :label="$t('pipeline_name_label')"
      name="name"
      :rules="[{ required: true, message: $t('pipeline_name_required') }]"
    >
      <a-input
        :value="modelValue.name"
        :maxlength="100"
        :show-count="false"
        :placeholder="$t('pipeline_name_placeholder')"
        @update:value="updateField('name', $event)"
      />
    </a-form-item>

    <a-form-item class="agent-basic-form-item"
        :label="$t('pipeline_description_label')"
    >
      <a-textarea
        :value="modelValue.description"
        :rows="4"
        :maxlength="1000"
        :show-count="false"
        :placeholder="$t('pipeline_description_placeholder')"
        @update:value="updateField('description', $event)"
      />
    </a-form-item>

    <a-form-item class="agent-basic-form-item"
        :label="$t('pipeline_member_scope')"
    >
      <a-radio-group
        :value="modelValue.scope || 'members'"
        class="scope-radio-group"
        @update:value="updateField('scope', $event)"
      >
        <a-radio value="all">{{ $t("pipeline_scope_all") }}</a-radio>
        <a-radio value="members">{{ $t("pipeline_scope_members") }}</a-radio>
        <a-radio value="departments">{{
          $t("pipeline_scope_departments")
        }}</a-radio>
      </a-radio-group>
    </a-form-item>

    <a-form-item
      class="agent-basic-form-item"
      v-if="(modelValue.scope || 'members') === 'members'"
      :label="$t('pipeline_scope_members_label')"
    >
      <a-select
        :value="modelValue.memberIds"
        mode="multiple"
        allow-clear
        :placeholder="$t('pipeline_member_scope_placeholder')"
        :options="memberOptions"
          :field-names="{ label: 'name', value: 'id' }"
          @update:value="updateField('memberIds', $event)"
      />
    </a-form-item>

    <a-form-item
      class="agent-basic-form-item"
      v-if="modelValue.scope === 'departments'"
      :label="$t('pipeline_scope_departments_label')"
    >
      <a-tree-select
        :value="modelValue.departmentIds"
        multiple
        allow-clear
        tree-default-expand-all
        :placeholder="$t('pipeline_scope_departments_placeholder')"
        :tree-data="departmentTree"
          :field-names="{ label: 'title', value: 'value' }"
          @update:value="updateField('departmentIds', $event)"
      />
    </a-form-item>

    <div v-if="showActions" class="agent-form-actions">
      <a-button @click="$emit('cancel')">{{ $t("common_cancel") }}</a-button>
      <a-button type="primary" html-type="submit" :loading="loading">
        {{ submitText || $t("common_save") }}
      </a-button>
    </div>
  </a-form>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import { CameraOutlined } from "@ant-design/icons-vue";
import AvatarPicker from "@/components/pipeline/AvatarPicker.vue";
import { PIPELINE_AVATARS } from "@/components/pipeline/avatars";

const { t: $t } = useI18n();

const props = defineProps({
  modelValue: { type: Object, required: true },
  memberOptions: { type: Array, default: () => [] },
  departmentOptions: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
  showActions: { type: Boolean, default: false },
  submitText: { type: String, default: "" },
});

const emit = defineEmits([
  "update:modelValue",
  "submit",
  "icon-change",
  "cancel",
]);

const fileInput = ref(null);
const preview = ref("");
const selectedFile = ref(null);

const shownIcon = computed(
  () => preview.value || props.modelValue.icon || PIPELINE_AVATARS[0],
);
const selectedPreset = computed(() =>
  PIPELINE_AVATARS.includes(props.modelValue.icon)
    ? props.modelValue.icon
    : "",
);

const departmentTree = computed(() => {
  const nodes = new Map();
  for (const dept of props.departmentOptions) {
    nodes.set(String(dept.id), {
      value: Number(dept.id),
      title: dept.name,
      children: [],
    });
  }
  const roots = [];
  for (const dept of props.departmentOptions) {
    const node = nodes.get(String(dept.id));
    const parent = nodes.get(String(dept.parentId));
    if (parent && parent !== node) parent.children.push(node);
    else roots.push(node);
  }
  return roots;
});

watch(
  () => props.modelValue.icon,
  (icon) => {
    selectedFile.value = null;
    preview.value = icon || "";
  },
  { immediate: true },
);

function updateField(field, value) {
  emit("update:modelValue", { ...props.modelValue, [field]: value });
}

function triggerUpload() {
  fileInput.value?.click();
}

function selectPreset(avatar) {
  selectedFile.value = null;
  preview.value = "";
  if (fileInput.value) fileInput.value.value = "";
  updateField("icon", avatar);
}

function handleFileChange(event) {
  const file = event.target.files?.[0];
  if (!file) return;

  if (!["image/jpeg", "image/png"].includes(file.type)) {
    message.error($t("pipeline_icon_invalid"));
    event.target.value = "";
    return;
  }
  if (file.size > 100 * 1024) {
    message.error($t("pipeline_icon_too_large"));
    event.target.value = "";
    return;
  }

  selectedFile.value = file;
  emit("icon-change", file);
  const reader = new FileReader();
  reader.onload = (loadEvent) => {
    preview.value = loadEvent.target?.result || "";
  };
  reader.readAsDataURL(file);
}

function handleSubmit() {
  emit("submit", { ...props.modelValue, iconFile: selectedFile.value });
}
</script>

<style scoped lang="less">
.agent-basic-form {
  width: 100%;
}

.agent-avatar-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  margin-bottom: 20px;
}

.agent-avatar-button {
  position: relative;
  display: flex;
  width: 60px;
  height: 60px;
  align-items: center;
  justify-content: center;
  padding: 0;
  overflow: visible;
  border: 0;
  border-radius: 50%;
  background: #f2f3f5;
  cursor: pointer;
}

.agent-avatar-image {
  width: 60px;
  height: 60px;
  border-radius: 50%;
  object-fit: cover;
}

.agent-camera-badge {
  position: absolute;
  right: -4px;
  bottom: -4px;
  display: inline-flex;
  width: 24px;
  height: 24px;
  align-items: center;
  justify-content: center;
  border: 2px solid #fff;
  border-radius: 50%;
  color: #fff;
  background: #262626;
  font-size: 12px;
}

.agent-avatar-hint {
  margin: 12px 0 0;
  color: rgba(0, 0, 0, 0.45);
  font-size: 14px;
  line-height: 22px;
  text-align: center;
}

.agent-basic-form-item {
  margin-bottom: 20px;

  &:last-child {
    margin-bottom: 0;
  }
}

.pipeline-modal-label {
  color: #262626;
  font-size: 14px;
  font-weight: 600;
}

.agent-color-picker {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.agent-color-swatch {
  display: inline-flex;
  width: 32px;
  height: 32px;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 2px solid transparent;
  border-radius: 6px;
  color: #fff;
  cursor: pointer;
  transition: transform 0.15s;

  &:hover {
    transform: scale(1.08);
  }
}

.agent-color-swatch.selected {
  border-color: #1f1f1f;
  box-shadow: 0 0 0 1px #fff inset;
}

.scope-radio-group {
  display: flex;
  flex-wrap: wrap;
  gap: 24px;
}

// ── 输入框/下拉框样式（对齐客户端 PipelineEditorModal） ────────

:deep(.ant-form-item-label) {
  padding-bottom: 8px;
}

:deep(.ant-form-item-label > label) {
  height: 22px;
  color: #262626;
  font-size: 14px;
  font-weight: 600;
  line-height: 22px;
}

:deep(.ant-input),
:deep(.ant-input-affix-wrapper) {
  min-height: 40px;
  border-color: #d9d9d9 !important;
  border-radius: 12px !important;
  color: #262626;
  font-size: 14px;
  line-height: 22px;
}

:deep(.ant-input) {
  padding: 8px 12px;
}

:deep(textarea.ant-input) {
  height: 128px;
  min-height: 128px;
  resize: none;
}

:deep(.ant-select .ant-select-selector) {
  min-height: 40px;
  border-color: #d9d9d9 !important;
  border-radius: 12px !important;
  padding: 4px 12px !important;
}

:deep(.ant-radio-wrapper) {
  font-size: 14px;
}

:deep(.ant-input:hover),
:deep(.ant-select:not(.ant-select-disabled):hover .ant-select-selector) {
  border-color: #3157e2 !important;
}

:deep(.ant-input:focus),
:deep(.ant-input-focused),
:deep(.ant-select-focused .ant-select-selector) {
  border-color: #3157e2 !important;
  box-shadow: 0 0 0 2px rgba(49, 87, 226, 0.15) !important;
}

.agent-form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 24px;
}
</style>

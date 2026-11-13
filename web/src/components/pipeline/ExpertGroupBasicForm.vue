<template>
  <a-form
    class="expert-group-basic-form"
    layout="vertical"
    :model="modelValue"
    @finish="handleSubmit"
  >
    <a-form-item
      class="expert-group-form-item"
      :label="$t('expert_group_name_label')"
      name="name"
      :rules="[{ required: true, message: $t('expert_group_name_required') }]"
    >
      <a-input
        :value="modelValue.name"
        :maxlength="20"
        :show-count="false"
        :placeholder="$t('expert_group_name_placeholder')"
        @update:value="updateField('name', $event)"
      />
    </a-form-item>

    <a-form-item class="expert-group-form-item"
        :label="$t('expert_group_description_label')"
    >
      <a-textarea
        :value="modelValue.description"
        :rows="4"
        :maxlength="200"
        :show-count="false"
        :placeholder="$t('expert_group_description_placeholder')"
        @update:value="updateField('description', $event)"
      />
    </a-form-item>

    <a-form-item class="expert-group-form-item"
        :label="$t('expert_group_avatar_label')"
    >
      <div class="group-avatar-row">
        <img :src="groupAvatarSrc" alt="" class="group-avatar-preview" />
        <AvatarPicker
          :modelValue="modelValue.avatar"
          :options="EXPERT_GROUP_AVATARS"
          @update:modelValue="updateField('avatar', $event)"
        />
      </div>
    </a-form-item>

    <a-form-item class="expert-group-form-item"
        :label="$t('pipeline_member_scope')"
    >
      <a-radio-group
        :value="modelValue.scope || 'all'"
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
      class="expert-group-form-item"
      v-if="(modelValue.scope || 'all') === 'members'"
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
      class="expert-group-form-item"
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

    <div v-if="showActions" class="expert-group-form-actions">
      <a-button @click="$emit('cancel')">{{ $t("common_cancel") }}</a-button>
      <a-button type="primary" html-type="submit" :loading="loading">
        {{ submitText || $t("common_save") }}
      </a-button>
    </div>
  </a-form>
</template>

<script setup>
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import AvatarPicker from "@/components/pipeline/AvatarPicker.vue";
import { EXPERT_GROUP_AVATARS, resolveAvatar } from "@/components/pipeline/avatars";

const { t: $t } = useI18n();

const groupAvatarSrc = computed(() =>
  resolveAvatar(props.modelValue.avatar, EXPERT_GROUP_AVATARS, 0),
);

const props = defineProps({
  modelValue: { type: Object, required: true },
  memberOptions: { type: Array, default: () => [] },
  departmentOptions: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
  showActions: { type: Boolean, default: false },
  submitText: { type: String, default: "" },
});

const emit = defineEmits(["update:modelValue", "submit", "cancel"]);

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

function updateField(field, value) {
  emit("update:modelValue", { ...props.modelValue, [field]: value });
}

function handleSubmit() {
  emit("submit", { ...props.modelValue });
}
</script>

<style scoped lang="less">
.expert-group-basic-form {
  width: 100%;
}

.expert-group-form-item {
  margin-bottom: 20px;

  &:last-child {
    margin-bottom: 0;
  }
}

.scope-radio-group {
  display: flex;
  flex-wrap: wrap;
  gap: 24px;
}

// ── 输入框/下拉框样式（对齐客户端） ──────────────────────────

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

.expert-group-form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 24px;
}

.group-avatar-row {
  display: flex;
  align-items: flex-start;
  gap: 16px;
}

.group-avatar-preview {
  width: 40px;
  height: 40px;
  flex: 0 0 40px;
  border: 1px solid #f0f0f0;
  border-radius: 50%;
  background: #fafafa;
  object-fit: cover;
}
</style>

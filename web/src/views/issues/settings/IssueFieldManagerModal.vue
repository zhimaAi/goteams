<!--
 * IssueFieldManagerModal.vue
 * 管理字段弹窗：勾选公开表单要显示的字段。
 * 只列出属性字段设置里已经开启的基础字段与自定义字段。
-->
<template>
  <a-modal
    :open="props.open"
    :title="$t('settings_template_manage_fields')"
    :ok-text="$t('common_confirm')"
    :cancel-text="$t('common_cancel')"
    width="560px"
    @ok="handleConfirm"
    @cancel="close"
  >
    <div class="issue-field-manager">
      <h4 class="issue-field-manager__title">
        {{ $t("settings_template_builtin_fields") }}
      </h4>
      <div class="issue-field-manager__options">
        <a-checkbox
          v-for="item in builtinOptions"
          :key="item.key"
          :title="item.label"
          :checked="checked.includes(item.key)"
          @change="toggle(item.key, $event.target.checked)"
        >
          {{ item.label }}
        </a-checkbox>
      </div>

      <div class="issue-field-manager__group-head">
        <h4 class="issue-field-manager__title">
          {{ $t("settings_template_custom_fields") }}
        </h4>
        <a-button type="link" size="small" @click="customFieldOpen = true">
          <PlusOutlined />
          {{ $t("settings_newField") }}
        </a-button>
      </div>
      <div v-if="customOptions.length" class="issue-field-manager__options">
        <a-checkbox
          v-for="item in customOptions"
          :key="item.key"
          :title="item.label"
          :checked="checked.includes(item.key)"
          @change="toggle(item.key, $event.target.checked)"
        >
          {{ item.label }}
        </a-checkbox>
      </div>
      <p v-else class="issue-field-manager__empty">
        {{ $t("settings_noCustomFields") }}
      </p>
    </div>
  </a-modal>

  <CustomFieldEditorModal
    v-model:open="customFieldOpen"
    :ws-id="props.wsId"
    :type-key="props.typeKey"
    @saved="onCustomFieldSaved"
  />
</template>

<script setup>
import { ref, watch } from "vue";
import { PlusOutlined } from "@ant-design/icons-vue";
import { listCustomFields, listFieldConfigs } from "@/api/workspace";
import { TEMPLATE_BUILTIN_KEYS } from "@/constants/workItemTemplate";
import { customTemplateFieldKey } from "@/utils/workItemTemplate";
import CustomFieldEditorModal from "@/views/workspace/settings/components/CustomFieldEditorModal.vue";

const props = defineProps({
  open: { type: Boolean, default: false },
  wsId: { type: [String, Number], default: "" },
  typeKey: { type: String, default: "requirement" },
  // 模板当前字段，用来回填勾选：display 为 false 的就是被移除的
  fields: { type: Array, default: () => [] },
});

const emit = defineEmits(["update:open", "confirm", "field-created"]);

// 公开表单的固定项（标题/内容）和永不展示的字段不在管理范围内：标题和内容是公开表单
// 的固定必填项，其余几个后端渲染不到，勾了也没用。
const MANAGED_EXCLUDED_KEYS = new Set([
  "title",
  "description",
  "id",
  "created_at",
  "created_by",
  "work_progress",
  "remaining_work_hours",
  "overtime_work_hours",
]);

const builtinOptions = ref([]);
const customOptions = ref([]);
const checked = ref([]);
const customFieldOpen = ref(false);

// 打开时才拉一次：属性字段列表不随详情页渲染变化
watch(
  () => props.open,
  async (open) => {
    if (!open) return;
    await loadOptions();
    syncChecked();
  },
);

async function loadOptions() {
  const [configs, customs] = await Promise.all([
    listFieldConfigs(props.wsId, props.typeKey),
    listCustomFields(props.wsId, props.typeKey),
  ]);
  // 只收属性字段里已开启的：关掉的字段公开表单本来就不该出现
  const order = new Map(
    TEMPLATE_BUILTIN_KEYS.map((key, index) => [key, index]),
  );
  builtinOptions.value = (configs || [])
    .filter(
      (config) =>
        Number(config.enabled) === 1 &&
        !MANAGED_EXCLUDED_KEYS.has(config.key) &&
        !String(config.key || "").startsWith("custom:"),
    )
    .sort(
      (left, right) =>
        (order.get(left.key) ?? order.size) -
        (order.get(right.key) ?? order.size),
    )
    .map((config) => ({ key: config.key, label: config.label || config.key }));
  customOptions.value = (customs || [])
    .filter((field) => Number(field.is_enabled) === 1)
    .map((field) => ({
      key: customTemplateFieldKey(field.id),
      label: field.name,
    }));
}

function syncChecked() {
  checked.value = props.fields
    .filter((field) => field.display !== false)
    .map((field) => field.field_key);
}

function toggle(key, selected) {
  checked.value = selected
    ? [...new Set([...checked.value, key])]
    : checked.value.filter((item) => item !== key);
}

// 新建自定义字段是项目级的：重拉候选列表，新字段默认勾上，同时让外层刷新字段列表
async function onCustomFieldSaved() {
  const before = new Set(customOptions.value.map((item) => item.key));
  await loadOptions();
  const added = customOptions.value
    .filter((item) => !before.has(item.key))
    .map((item) => item.key);
  if (added.length) {
    checked.value = [...new Set([...checked.value, ...added])];
  }
  emit("field-created");
}

function handleConfirm() {
  emit("confirm", [...checked.value]);
  close();
}

function close() {
  emit("update:open", false);
}
</script>

<style scoped lang="less">
.issue-field-manager {
  &__title {
    margin: 16px 0 12px;
    color: #262626;
    font-size: 14px;
  }

  &__title:first-child {
    margin-top: 0;
  }

  &__group-head {
    display: flex;
    align-items: center;
    gap: 16px;
    margin: 12px 0;

    .issue-field-manager__title {
      margin: 0;
    }
  }

  &__options {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 12px 20px;
    width: 100%;

    // 字段名过长时缩成一行省略，完整名称走复选框上的 title
    :deep(.ant-checkbox-wrapper) {
      display: flex;
      align-items: center;
      max-width: 100%;
      min-width: 0;
      margin-inline-start: 0;

      > span:not(.ant-checkbox) {
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
      }
    }
  }

  &__empty {
    margin: 0;
    color: #8b8b8b;
    font-size: 13px;
  }
}
</style>

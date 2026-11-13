<template>
  <div class="settings-panel fields-panel">
    <a-card
      :title="$t('settings_attributeFields')"
      size="small"
      class="settings-card"
    >
      <template #extra>
        <a-space v-if="canManage" :size="10">
          <a-button type="primary" size="small" @click="openCustomEditor(null)">
            <PlusOutlined />
            {{ $t("settings_newField") }}
          </a-button>
          <a-button size="small" :loading="restoring" @click="restoreDefault">
            {{ $t("settings_restoreDefault") }}
          </a-button>
        </a-space>
      </template>
      <div v-if="loading" class="empty-fields">...</div>
      <div v-else ref="fieldListRef" class="field-list" :class="{ ordering }">
        <div v-for="field in fields" :key="field.orderKey" class="field-row">
          <div class="field-main">
            <a-tooltip :title="canManage ? $t('settings_dragFieldToSort') : ''">
              <span
                class="drag-handle"
                :class="{ disabled: !canManage || ordering }"
              >
                <HolderOutlined />
              </span>
            </a-tooltip>

            <div class="field-info">
              <template v-if="field.kind === 'builtin'">
                <span class="field-name">{{ field.label }}</span>
              </template>
              <template v-else>
                <strong class="field-name">{{ field.name }}</strong>
                <span v-if="fieldSummary(field)" class="field-options-count">
                  {{ fieldSummary(field) }}
                </span>
              </template>
            </div>

            <a-tag
              v-if="field.kind === 'builtin'"
              color="blue"
              class="field-tag"
            >
              {{ $t("settings_builtin") }}
            </a-tag>
            <a-tag v-else class="field-tag">
              {{ $t(typeLabelKey(field.field_type)) }}
            </a-tag>
          </div>

          <div class="field-side">
            <a-switch
              v-if="field.kind === 'builtin'"
              :checked="field.visible"
              :disabled="!canManage || field.locked || field.saving || ordering"
              size="small"
              :loading="field.saving"
              @change="(value) => onBuiltinChange(field, value)"
            />
            <a-space v-else :size="4">
              <a-switch
                :checked="!!field.is_enabled"
                :disabled="!canManage || ordering"
                size="small"
                @change="(value) => onCustomEnabledChange(field, value)"
              />
              <a-button
                v-if="canManage"
                type="link"
                size="small"
                class="field-action"
                @click="openCustomEditor(field.id)"
              >
                <EditOutlined />
              </a-button>
              <a-popconfirm
                v-if="canManage"
                :title="$t('settings_deleteFieldConfirm')"
                @confirm="deleteCustom(field.id)"
              >
                <a-button type="link" size="small" danger class="field-action">
                  <DeleteOutlined />
                </a-button>
              </a-popconfirm>
            </a-space>
          </div>
        </div>
      </div>
    </a-card>

    <CustomFieldEditorModal
      v-model:open="customModalOpen"
      :ws-id="wsId"
      :type-key="typeKey"
      :field="editingField"
      @saved="loadAll"
    />
  </div>
</template>

<script setup>
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
} from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import Sortable from "sortablejs";
import {
  DeleteOutlined,
  EditOutlined,
  HolderOutlined,
  PlusOutlined,
} from "@ant-design/icons-vue";
import {
  deleteCustomField,
  listCustomFields,
  listFieldConfigs,
  resetFieldConfigs,
  saveFieldConfigs,
  saveFieldOrder,
  updateCustomField,
} from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";
import {
  CUSTOM_FIELD_MEMBER_SCOPE,
  CUSTOM_FIELD_TYPE,
  CUSTOM_FIELD_TYPE_LABELS,
  isCascaderFieldType,
  isMemberFieldType,
  isOptionFieldType,
} from "@/constants/customField";
import { cascaderDepth } from "@/utils/customField";
import { buildFieldOrderItems, sortFieldsByOrder } from "@/utils/fieldOrder";
import { typeSettingPermission } from "@/utils/typePermission";
import CustomFieldEditorModal from "./components/CustomFieldEditorModal.vue";

const props = defineProps({
  wsId: { type: [String, Number], default: "" },
  typeKey: { type: String, default: "requirement" },
});

const { t: $t } = useI18n();
const authStore = useAuthStore();
function typeLabelKey(type) {
  return CUSTOM_FIELD_TYPE_LABELS[type] || type;
}

function fieldSummary(field) {
  if (isOptionFieldType(field.field_type)) {
    return `(${$t("settings_fieldOptionsCount", {
      n: field.options?.length || 0,
    })})`;
  }
  if (isMemberFieldType(field.field_type)) {
    if (field.member_scope === CUSTOM_FIELD_MEMBER_SCOPE.ROLES) {
      return $t("settings_fieldMemberScopeRolesCount", {
        n: field.role_ids?.length || 0,
      });
    }
    if (field.member_scope === CUSTOM_FIELD_MEMBER_SCOPE.WORKSPACE) {
      return $t("settings_fieldMemberScopeWorkspace");
    }
    return $t("settings_fieldMemberScopeAll");
  }
  if (
    field.field_type === CUSTOM_FIELD_TYPE.NUMERIC &&
    Number(field.integer_only) === 1
  ) {
    return $t("settings_fieldIntegerOnly");
  }
  if (isCascaderFieldType(field.field_type)) {
    return `(${$t("settings_fieldCascaderCount", {
      n: cascaderDepth(field.cascader || []),
    })})`;
  }
  return "";
}

const fields = ref([]);
const loading = ref(false);
const restoring = ref(false);
const ordering = ref(false);
const fieldListRef = ref(null);
let fieldSortable = null;
const customModalOpen = ref(false);
const editingCustomId = ref(null);
const canManage = computed(() =>
  authStore.hasPermission(typeSettingPermission(props.typeKey, "field")),
);
const editingField = computed(
  () =>
    fields.value.find(
      (field) => field.kind === "custom" && field.id === editingCustomId.value,
    ) || null,
);

async function loadAll() {
  loading.value = true;
  try {
    const [builtinFields, customFields] = await Promise.all([
      listFieldConfigs(props.wsId, props.typeKey),
      listCustomFields(props.wsId, props.typeKey),
    ]);
    fields.value = sortFieldsByOrder([
      ...(builtinFields || []).map((field) => ({
        kind: "builtin",
        orderKey: `builtin:${field.key}`,
        key: field.key,
        label: field.label,
        visible: !!field.enabled,
        locked: !!field.locked,
        editable: field.editable,
        sort_order: field.sort_order,
        saving: false,
      })),
      ...(customFields || []).map((field) => ({
        ...field,
        kind: "custom",
        orderKey: `custom:${field.id}`,
      })),
    ]);
  } catch {
    fields.value = [];
  } finally {
    loading.value = false;
  }
}

async function onBuiltinChange(field, value) {
  if (!canManage.value || field.locked) return;
  field.saving = true;
  try {
    await saveFieldConfigs(props.wsId, props.typeKey, [
      {
        key: field.key,
        enabled: value ? 1 : 0,
        editable: field.editable,
        sort_order: field.sort_order,
      },
    ]);
    field.visible = value;
    message.success($t("common_saveSuccess"));
  } catch {
    // The request interceptor displays the failure message.
  } finally {
    field.saving = false;
  }
}

// Restore defaults: the backend resets enabled state and order by preset (custom fields moved to the end).
async function restoreDefault() {
  if (!canManage.value) return;
  restoring.value = true;
  try {
    await resetFieldConfigs(props.wsId, props.typeKey);
    message.success($t("common_saveSuccess"));
    await loadAll();
  } catch {
    // The request interceptor displays the failure message.
  } finally {
    restoring.value = false;
  }
}

function destroyFieldSortable() {
  fieldSortable?.destroy();
  fieldSortable = null;
}

function setupFieldSortable() {
  destroyFieldSortable();
  const element = fieldListRef.value;
  if (!element || !canManage.value || ordering.value || !fields.value.length) {
    return;
  }
  fieldSortable = Sortable.create(element, {
    handle: ".drag-handle",
    draggable: ".field-row",
    animation: 180,
    easing: "cubic-bezier(0.22, 0.61, 0.36, 1)",
    // The fallback drag renders the row itself, so the movement stays smooth.
    forceFallback: true,
    fallbackOnBody: true,
    fallbackTolerance: 3,
    ghostClass: "field-row-ghost",
    chosenClass: "field-row-chosen",
    dragClass: "field-row-drag",
    onEnd: handleFieldSortEnd,
  });
}

function handleFieldSortEnd({ oldIndex, newIndex, item }) {
  if (
    !canManage.value ||
    ordering.value ||
    oldIndex == null ||
    newIndex == null ||
    oldIndex === newIndex
  ) {
    return;
  }
  // Sortable already moved the DOM node; restore it so Vue keeps owning the order.
  const container = fieldListRef.value;
  if (container && item) {
    const others = [...container.children].filter((node) => node !== item);
    container.insertBefore(item, others[oldIndex] || null);
  }
  const previousFields = [...fields.value];
  const nextFields = [...fields.value];
  const [movedField] = nextFields.splice(oldIndex, 1);
  nextFields.splice(newIndex, 0, movedField);
  fields.value = nextFields;
  persistFieldOrder(nextFields, previousFields);
}

async function persistFieldOrder(nextFields, previousFields) {
  if (!canManage.value) return;
  ordering.value = true;
  fieldSortable?.option("disabled", true);
  try {
    await saveFieldOrder(
      props.wsId,
      props.typeKey,
      buildFieldOrderItems(nextFields),
    );
    nextFields.forEach((field, index) => {
      field.sort_order = index;
    });
    message.success($t("common_saveSuccess"));
  } catch {
    fields.value = previousFields;
    await loadAll();
  } finally {
    ordering.value = false;
    nextTick(setupFieldSortable);
  }
}

function openCustomEditor(id) {
  if (!canManage.value) return;
  editingCustomId.value = id || null;
  customModalOpen.value = true;
}

function fieldUpdatePayload(field, extra = {}) {
  return {
    name: field.name,
    field_type: field.field_type,
    is_required: field.is_required,
    is_enabled: field.is_enabled,
    options: (field.options || []).map((option) => ({
      key: option.key || "",
      name: option.name || "",
    })),
    integer_only: field.integer_only || 0,
    member_scope: field.member_scope || CUSTOM_FIELD_MEMBER_SCOPE.ALL,
    role_ids: field.role_ids || [],
    cascader: cloneCascader(field.cascader || []),
    ...extra,
  };
}

function cloneCascader(nodes) {
  return (nodes || []).map((node) => ({
    key: node.key || "",
    name: node.name || "",
    children: cloneCascader(node.children || []),
  }));
}

async function onCustomEnabledChange(field, value) {
  if (!canManage.value) return;
  const previousValue = field.is_enabled;
  field.is_enabled = value ? 1 : 0;
  try {
    await updateCustomField(
      props.wsId,
      props.typeKey,
      field.id,
      fieldUpdatePayload(field),
    );
    message.success($t("common_saveSuccess"));
  } catch {
    field.is_enabled = previousValue;
  }
}

async function deleteCustom(id) {
  if (!canManage.value) return;
  try {
    await deleteCustomField(props.wsId, props.typeKey, id);
    message.success($t("settings_fieldDeleted"));
    await loadAll();
  } catch {
    // The request interceptor displays the failure message.
  }
}

onMounted(loadAll);
onBeforeUnmount(destroyFieldSortable);
watch(() => [props.wsId, props.typeKey], loadAll);
// The list node is re-created while loading, so the instance is rebuilt after render.
watch(
  () => [
    fieldListRef.value,
    loading.value,
    fields.value.length,
    canManage.value,
  ],
  () => nextTick(setupFieldSortable),
);
</script>

<style scoped lang="less">
.settings-panel {
  width: min(920px, 100%);
  margin: 0 auto;
}

.settings-card {
  overflow: hidden;
  border-color: #eef1f6;
  border-radius: 12px;
  box-shadow: 0 2px 8px rgba(24, 43, 80, 0.04);

  :deep(.ant-card-head) {
    min-height: 54px;
    padding: 0 16px;
    background: #f5f7fa;
    border-color: #eef1f6;
    font-weight: 600;
  }

  :deep(.ant-card-head-title) {
    padding: 15px 0;
    font-size: 14px;
  }

  :deep(.ant-card-body) {
    padding: 16px;
  }
}

.field-list {
  display: flex;
  flex-direction: column;

  &.ordering {
    pointer-events: none;
    opacity: 0.72;
  }
}

.field-row {
  display: flex;
  min-height: 47px;
  align-items: center;
  justify-content: space-between;
  padding: 0 4px;
  border-bottom: 1px solid #eef1f6;
  background: #fff;
  transition:
    opacity 0.15s ease,
    background-color 0.15s ease,
    box-shadow 0.2s ease;

  &:last-child {
    border-bottom: 0;
  }
}

.field-row-ghost {
  background: #f0f7ff;
  opacity: 0.6;
}

.field-row-chosen {
  box-shadow: 0 3px 10px rgb(16 24 40 / 12%);
}

.field-row-drag {
  opacity: 0.92;
}

.field-main {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10px;
}

.drag-handle {
  display: inline-flex;
  flex: none;
  align-items: center;
  color: rgba(0, 0, 0, 0.35);
  cursor: grab;
  font-size: 17px;

  &:active {
    cursor: grabbing;
  }

  &:hover:not(.disabled) {
    color: rgba(0, 0, 0, 0.65);
  }

  &.disabled {
    cursor: default;
    opacity: 0.5;
  }
}

.field-name {
  min-width: 0;
  overflow: hidden;
  color: rgba(0, 0, 0, 0.88);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.field-side {
  display: flex;
  flex: none;
  align-items: center;
  gap: 10px;
  margin-left: auto;
}

/* Fixed-width name column keeps the type tag on the same column for all rows. */
.field-info {
  display: flex;
  flex: none;
  width: 350px;
  min-width: 0;
  align-items: center;
  gap: 6px;
}

.field-tag {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  min-width: 44px;
  margin-inline-end: 0;
  font-size: 11px;
}

.field-options-count {
  flex: none;
  color: #999;
  font-size: 11px;
}

.field-action {
  padding: 0 4px;
}

.empty-fields {
  display: flex;
  min-height: 52px;
  align-items: center;
  justify-content: center;
  color: rgba(0, 0, 0, 0.45);
  text-align: center;
}
</style>

<template>
  <div
    class="settings-panel fields-panel"
    :style="{
      '--field-bg': themeToken.colorBgContainer,
      '--field-border': themeToken.colorBorderSecondary,
      '--field-header-bg': themeToken.colorFillQuaternary,
      '--field-text': themeToken.colorText,
      '--field-secondary': themeToken.colorTextSecondary,
      '--field-tertiary': themeToken.colorTextTertiary,
      '--field-quaternary': themeToken.colorTextQuaternary,
      '--field-primary-bg': themeToken.colorPrimaryBg,
    }"
  >
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
          <a-button
            size="small"
            :loading="restoring"
            @click="confirmRestoreDefault"
          >
            {{ $t("settings_restoreDefault") }}
          </a-button>
        </a-space>
      </template>
      <p v-if="typeKey !== 'planning'" class="edit-role-help">
        {{ $t("settings_editRolesPrototypeHelp") }}
      </p>
      <div class="field-table-scroll">
        <div
          class="field-table"
          :class="{ 'field-table--without-roles': typeKey === 'planning' }"
          role="table"
          :aria-label="$t('settings_attributeFields')"
        >
          <div class="field-heading" role="row">
            <span role="columnheader">{{ $t("settings_fieldName") }}</span>
            <span role="columnheader">{{ $t("settings_fieldType") }}</span>
            <span v-if="typeKey !== 'planning'" role="columnheader">{{
              $t("settings_editRoles")
            }}</span>
            <span class="field-controls" role="columnheader">{{
              $t("settings_action")
            }}</span>
          </div>
          <div v-if="loading" class="empty-fields">...</div>
          <div
            v-else
            ref="fieldListRef"
            class="field-list"
            :class="{ ordering }"
            role="rowgroup"
          >
            <div
              v-for="field in fields"
              :key="field.orderKey"
              class="field-row"
              role="row"
            >
              <div class="field-title-cell" role="cell">
                <a-tooltip
                  :title="canManage ? $t('settings_dragFieldToSort') : ''"
                >
                  <span
                    class="drag-handle"
                    :class="{ disabled: !canManage || ordering }"
                    ><HolderOutlined
                  /></span>
                </a-tooltip>
                <div class="field-info">
                  <span
                    class="field-name"
                    :title="field.kind === 'builtin' ? field.label : field.name"
                    >{{
                      field.kind === "builtin" ? field.label : field.name
                    }}</span
                  >
                  <span
                    v-if="field.kind !== 'builtin' && fieldSummary(field)"
                    class="field-options-count"
                    >{{ fieldSummary(field) }}</span
                  >
                </div>
              </div>
              <div class="field-type-cell" role="cell">
                <a-tag
                  :color="field.kind === 'builtin' ? 'blue' : undefined"
                  class="field-tag"
                  >{{
                    $t(
                      field.kind === "builtin"
                        ? "settings_builtin"
                        : "settings_custom",
                    )
                  }}</a-tag
                >
                <span class="field-type">{{
                  field.kind === "builtin"
                    ? builtinTypeLabels[field.key]
                      ? $t(builtinTypeLabels[field.key])
                      : "—"
                    : $t(typeLabelKey(field.field_type))
                }}</span>
              </div>
              <div
                v-if="typeKey !== 'planning'"
                class="edit-role-cell"
                role="cell"
              >
                <FieldEditRoleSelect
                  v-if="field.supports_edit_roles"
                  :key="`${wsId}:${typeKey}:${field.orderKey}`"
                  :ws-id="wsId"
                  :type-key="typeKey"
                  :field="field"
                  :roles="roles"
                  :disabled="
                    !canManage || rolesLoading || rolesFailed || ordering
                  "
                  @saved="(policy) => onFieldRolesSaved(field, policy)"
                />
                <span v-else class="field-readonly">{{
                  field.supports_edit_roles === false
                    ? $t("settings_editRolesNotRequired")
                    : "—"
                }}</span>
              </div>
              <div class="field-controls" role="cell">
                <a-switch
                  v-if="field.kind === 'builtin'"
                  :checked="field.visible"
                  :disabled="
                    !canManage || field.locked || field.saving || ordering
                  "
                  :loading="field.saving"
                  :aria-label="field.label"
                  @change="(value) => onBuiltinChange(field, value)"
                />
                <a-space v-else :size="4">
                  <a-switch
                    :checked="!!field.is_enabled"
                    :disabled="!canManage || ordering"
                    :aria-label="field.name"
                    @change="(value) => onCustomEnabledChange(field, value)"
                  />
                  <a-button
                    v-if="canManage"
                    type="link"
                    size="small"
                    class="field-action"
                    :aria-label="$t('common_edit')"
                    @click="openCustomEditor(field.id)"
                    ><EditOutlined
                  /></a-button>
                  <a-popconfirm
                    v-if="canManage"
                    :title="$t('settings_deleteFieldConfirm')"
                    @confirm="deleteCustom(field.id)"
                  >
                    <a-button
                      type="link"
                      size="small"
                      danger
                      class="field-action"
                      :aria-label="$t('common_delete')"
                      ><DeleteOutlined
                    /></a-button>
                  </a-popconfirm>
                </a-space>
              </div>
            </div>
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
import { Modal, message, theme } from "ant-design-vue";
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
  listTemplateRoleOptions,
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
import FieldEditRoleSelect from "./components/FieldEditRoleSelect.vue";
import CustomFieldEditorModal from "./components/CustomFieldEditorModal.vue";

const props = defineProps({
  wsId: { type: [String, Number], default: "" },
  typeKey: { type: String, default: "requirement" },
});

const { t: $t } = useI18n();
const { token: themeToken } = theme.useToken();
const authStore = useAuthStore();
const builtinTypeLabels = {
  id: "settings_fieldTypeText",
  row_index: "settings_fieldTypeText",
  title: "settings_fieldTypeText",
  description: "settings_fieldTypeTextarea",
  status: "settings_fieldTypeSingleChoice",
  version: "settings_fieldTypeSingleChoice",
  category: "settings_fieldTypeSingleChoice",
  priority: "settings_fieldTypeSingleChoice",
  severity_id: "settings_fieldTypeSingleChoice",
  assignee: "settings_fieldTypeMember",
  responsible_ids: "settings_fieldTypeMember",
  created_by: "settings_fieldTypeMember",
  planned_start_date: "settings_fieldTypeDate",
  planned_end_date: "settings_fieldTypeDate",
  started_at: "settings_fieldTypeDate",
  ended_at: "settings_fieldTypeDate",
  created_at: "settings_fieldTypeDate",
  estimated_work_hours: "settings_fieldTypeNumber",
  completed_work_hours: "settings_fieldTypeNumber",
  remaining_work_hours: "settings_fieldTypeNumber",
  overtime_work_hours: "settings_fieldTypeNumber",
  work_progress: "settings_fieldTypeNumber",
  related_items: "settings_fieldTypeRelatedItems",
};

function onFieldRolesSaved(field, policy) {
  Object.assign(field, policy);
  authStore.clearProjectMetadata();
}

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
const roles = ref([]);
const rolesLoading = ref(false);
const rolesFailed = ref(false);
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
    rolesLoading.value = true;
    rolesFailed.value = false;
    const [builtinFields, customFields, roleOptions] = await Promise.all([
      listFieldConfigs(props.wsId, props.typeKey),
      listCustomFields(props.wsId, props.typeKey),
      props.typeKey !== "planning" && canManage.value
        ? listTemplateRoleOptions(props.wsId).catch(() => {
            rolesFailed.value = true;
            return [];
          })
        : Promise.resolve([]),
    ]);
    roles.value = roleOptions || [];
    fields.value = sortFieldsByOrder([
      ...(builtinFields || []).map((field) => ({
        ...field,
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
    rolesLoading.value = false;
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
function confirmRestoreDefault() {
  if (props.typeKey === "planning") return restoreDefault();
  Modal.confirm({
    title: $t("settings_restoreDefault"),
    content: $t("settings_editRolesResetConfirm"),
    onOk: restoreDefault,
  });
}
async function restoreDefault() {
  if (!canManage.value) return;
  restoring.value = true;
  try {
    await resetFieldConfigs(props.wsId, props.typeKey);
    authStore.clearProjectMetadata();
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
  width: min(1090px, 100%);
  margin: 0 auto;
}

.settings-card {
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;

  :deep(.ant-card-head) {
    padding: 0;
    border-bottom: 0;
    background: transparent;
  }

  :deep(.ant-card-body) {
    padding: 0;
  }
}

.edit-role-help {
  margin: 0;
  padding: 10px 0 16px;
  color: var(--field-secondary);
  font-size: 13px;
  line-height: 22px;
}

.field-table-scroll {
  overflow-x: auto;
  border: 1px solid var(--field-border);
  border-radius: 10px;
  background: var(--field-bg);
}

.field-table {
  --field-columns: minmax(180px, 1.1fr) minmax(190px, 1fr) minmax(188px, 2.4fr)
    112px;
  min-width: 760px;

  &--without-roles {
    --field-columns: minmax(180px, 1.1fr) minmax(190px, 1fr) 112px;
    min-width: 560px;
  }
}

.field-heading,
.field-row {
  display: grid;
  grid-template-columns: var(--field-columns);
  align-items: center;
  column-gap: 16px;
  padding: 0 24px;
}

.field-heading {
  min-height: 54px;
  border-bottom: 1px solid var(--field-border);
  background: var(--field-header-bg);
  color: var(--field-text);
  font-size: 14px;
  font-weight: 500;
}

.field-list.ordering {
  pointer-events: none;
  opacity: 0.72;
}

.field-row {
  min-height: 72px;
  border-bottom: 1px solid var(--field-border);
  background: var(--field-bg);
  transition:
    background-color 0.15s ease,
    opacity 0.15s ease;
  &:last-child {
    border-bottom: 0;
  }
}

.field-title-cell,
.field-type-cell,
.field-info {
  display: flex;
  min-width: 0;
  align-items: center;
}
.field-title-cell {
  gap: 12px;
}
.field-type-cell {
  gap: 16px;
}
.field-info {
  gap: 6px;
}

.drag-handle {
  display: inline-flex;
  flex: none;
  color: var(--field-quaternary);
  font-size: 16px;
  cursor: grab;
  &:active {
    cursor: grabbing;
  }
  &:hover:not(.disabled) {
    color: var(--field-secondary);
  }
  &.disabled {
    cursor: default;
  }
}

.field-name,
.field-type {
  min-width: 0;
  color: var(--field-text);
  font-size: 14px;
  font-weight: 400;
  line-height: 22px;
}
.field-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.field-type {
  color: var(--field-secondary);
  overflow-wrap: anywhere;
}
.field-tag {
  flex: none;
  margin-inline-end: 0;
  border-radius: 3px;
  font-size: 12px;
  line-height: 20px;
}
.field-options-count {
  overflow: hidden;
  color: var(--field-tertiary);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.edit-role-cell {
  min-width: 0;
}
.field-readonly {
  color: var(--field-tertiary);
  font-size: 14px;
}
.field-controls {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  :deep(.ant-switch-checked.ant-switch-disabled) {
    opacity: 1;
  }
}
.field-action {
  padding: 0 4px;
}
.field-row-ghost {
  background: var(--field-primary-bg);
  opacity: 0.6;
}
.field-row-chosen {
  box-shadow: 0 3px 10px rgb(16 24 40 / 12%);
}
.field-row-drag {
  opacity: 0.92;
}
.empty-fields {
  padding: 24px;
  color: var(--field-secondary);
  text-align: center;
}
</style>

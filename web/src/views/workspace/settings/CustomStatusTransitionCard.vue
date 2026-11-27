<template>
  <a-card size="small" class="settings-card">
    <template #title>{{ $t("settings_custom_transition") }}</template>
    <template #extra>
      <a-switch
        :checked="form.custom_enabled === 1"
        :disabled="!editable"
        @change="(checked) => save({ custom_enabled: checked ? 1 : 0 })"
      />
    </template>
    <template v-if="form.custom_enabled === 1">
      <div class="rule-row rule-head">
        <span>{{ $t("workspace_field_status") }}</span>
        <span>{{ $t("settings_transition_field_name") }}</span>
        <span class="rule-actions">{{ $t("settings_action") }}</span>
      </div>
      <div
        v-for="rule in form.status_rules"
        :key="rule.status_id"
        class="rule-row"
      >
        <span>{{ statusName(rule.status_id) }}</span>
        <a-tooltip :title="summary(rule)">
          <span class="field-summary">
            <template v-if="rule.fields.length">
              <span v-for="(field, index) in rule.fields" :key="field.key">
                <template v-if="index">、</template>
                {{ fieldSummary(field) }}
              </span>
            </template>
            <template v-else>{{
              $t("settings_transition_no_fields")
            }}</template>
          </span>
        </a-tooltip>
        <a-space class="rule-actions">
          <a-button
            type="text"
            size="small"
            :disabled="!editable"
            :aria-label="$t('common_edit')"
            @click="edit(rule)"
            ><EditOutlined
          /></a-button>
          <a-popconfirm
            :title="$t('settings_delete_transition_rule')"
            :disabled="!editable"
            @confirm="remove(rule.status_id)"
          >
            <a-button
              type="text"
              danger
              size="small"
              :disabled="!editable"
              :aria-label="$t('common_delete')"
              ><DeleteOutlined
            /></a-button>
          </a-popconfirm>
        </a-space>
      </div>
      <div v-if="adding" class="rule-row">
        <a-select
          v-model:value="newStatusId"
          :options="availableStatuses"
          :placeholder="$t('settings_select_transition_status')"
          :disabled="!editable"
        />
        <div>
          <a-tooltip :title="summary({ fields: newFields })">
            <span class="field-summary">
              <template v-if="newFields.length">
                <span v-for="(field, index) in newFields" :key="field.key">
                  <template v-if="index">、</template>
                  {{ fieldSummary(field) }}
                </span>
              </template>
              <template v-else>{{
                $t("settings_transition_no_fields")
              }}</template>
            </span>
          </a-tooltip>
          <a-button
            type="link"
            size="small"
            :disabled="!editable || !newStatusId"
            @click="editNewFields"
          >
            <EditOutlined />{{ $t("settings_select_transition_field") }}
          </a-button>
        </div>
        <a-space class="rule-actions">
          <a-button
            type="text"
            size="small"
            :disabled="!editable || !newStatusId"
            @click="add"
            >{{ $t("common_save") }}</a-button
          >
          <a-button
            type="text"
            size="small"
            :disabled="saving"
            @click="adding = false"
            >{{ $t("common_cancel") }}</a-button
          >
        </a-space>
      </div>
      <a-button
        v-if="canManage"
        type="link"
        style="padding: 0 7px"
        :disabled="!editable || adding || !availableStatuses.length"
        @click="startAdd"
      >
        <PlusOutlined />{{ $t("settings_add_transition_status") }}
      </a-button>
    </template>
  </a-card>
  <a-modal
    :open="editingStatusId !== null || editingNewFields"
    :title="$t('settings_select_transition_field')"
    :confirm-loading="saving"
    :ok-button-props="{ disabled: !editable }"
    :cancel-button-props="{ disabled: saving }"
    @ok="saveFields"
    @cancel="cancelEdit"
  >
    <div class="field-section-title">
      {{ $t("settings_transition_selected_fields") }}
    </div>
    <div ref="fieldListRef">
      <div
        v-for="field in fields"
        :key="field.key"
        class="field-option transition-sortable-field"
      >
        <span class="field-option-name">
          <WorkflowOptionHandle
            v-if="canManage"
            :label="fieldLabel(field.key)"
            :disabled="!editable || fields.length < 2"
            @move="moveField(field.key, $event)"
          />
          <a-checkbox
            :checked="true"
            :disabled="!editable"
            @change="(event) => select(field.key, event.target.checked)"
            >{{ fieldLabel(field.key) }}</a-checkbox
          >
        </span>
        <a-space>
          <a-tag v-if="isBuiltin(field.key)" color="blue">{{
            $t("settings_builtin")
          }}</a-tag>
          <a-checkbox
            :checked="field.required === 1"
            :disabled="!editable"
            @change="(event) => setRequired(field.key, event.target.checked)"
            >{{ $t("settings_transition_required") }}</a-checkbox
          >
        </a-space>
      </div>
    </div>
    <div v-if="!fields.length" class="field-empty">
      {{ $t("settings_transition_no_fields") }}
    </div>
    <template v-if="availableFieldOptions.length">
      <div class="field-section-title">
        {{ $t("settings_transition_available_fields") }}
      </div>
      <div
        v-for="option in availableFieldOptions"
        :key="option.key"
        class="field-option"
      >
        <a-checkbox
          :checked="false"
          :disabled="!editable"
          @change="(event) => select(option.key, event.target.checked)"
          >{{ option.label }}</a-checkbox
        >
        <a-tag v-if="option.builtin" color="blue">{{
          $t("settings_builtin")
        }}</a-tag>
      </div>
    </template>
  </a-modal>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
} from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import WorkflowOptionHandle from "./WorkflowOptionHandle.vue";
import { useStatusTransitionFieldOrder } from "@/composables/useStatusTransitionFieldOrder";

const props = defineProps({
  form: { type: Object, required: true },
  statuses: { type: Array, default: () => [] },
  options: { type: Array, default: () => [] },
  canManage: { type: Boolean, default: false },
  saving: { type: Boolean, default: false },
});
const emit = defineEmits(["save"]);
const { t: $t } = useI18n();
const adding = ref(false);
const newStatusId = ref(undefined);
const newFields = ref([]);
const editingNewFields = ref(false);
const editingStatusId = ref(null);
const fields = ref([]);
const fieldListRef = ref(null);
const editable = computed(
  () => props.canManage && !props.saving && props.form.enabled === 1,
);
const orderDisabled = computed(
  () =>
    !editable.value ||
    (editingStatusId.value === null && !editingNewFields.value),
);
const { moveField } = useStatusTransitionFieldOrder(
  fields,
  () => fieldListRef.value,
  orderDisabled,
);
const availableFieldOptions = computed(() =>
  props.options.filter((option) => !selected(option.key)),
);

const availableStatuses = computed(() => {
  const used = new Set(props.form.status_rules.map((rule) => rule.status_id));
  return props.statuses
    .filter((status) => !used.has(Number(status.id)))
    .map((status) => ({ value: Number(status.id), label: status.name }));
});
function statusName(id) {
  return (
    props.statuses.find((status) => Number(status.id) === id)?.name ||
    String(id)
  );
}
function fieldLabel(key) {
  return props.options.find((option) => option.key === key)?.label || key;
}
function isBuiltin(key) {
  return props.options.some((option) => option.key === key && option.builtin);
}
function fieldSummary(field) {
  return `${fieldLabel(field.key)}${field.required === 1 ? $t("settings_transition_required_suffix") : ""}`;
}
function summary(rule) {
  return (
    rule.fields.map(fieldSummary).join("、") ||
    $t("settings_transition_no_fields")
  );
}
function save(patch) {
  if (editable.value) emit("save", { ...props.form, ...patch });
}
function startAdd() {
  newStatusId.value = undefined;
  newFields.value = [];
  adding.value = true;
}
function add() {
  if (
    !availableStatuses.value.some(
      (status) => status.value === newStatusId.value,
    )
  )
    return;
  save({
    status_rules: [
      ...props.form.status_rules,
      {
        status_id: newStatusId.value,
        fields: newFields.value.map((field) => ({ ...field })),
      },
    ],
  });
}
function remove(id) {
  save({
    status_rules: props.form.status_rules.filter(
      (rule) => rule.status_id !== id,
    ),
  });
}
function edit(rule) {
  fields.value = rule.fields.map((field) => ({ ...field }));
  editingStatusId.value = rule.status_id;
}
function editNewFields() {
  if (!editable.value || !newStatusId.value) return;
  fields.value = newFields.value.map((field) => ({ ...field }));
  editingNewFields.value = true;
}
function selected(key) {
  return fields.value.some((field) => field.key === key);
}
function required(key) {
  return fields.value.some(
    (field) => field.key === key && field.required === 1,
  );
}
function select(key, checked) {
  fields.value = fields.value.filter((field) => field.key !== key);
  if (checked) fields.value.push({ key, required: 0 });
}
function setRequired(key, checked) {
  fields.value = fields.value.map((field) =>
    field.key === key ? { ...field, required: checked ? 1 : 0 } : field,
  );
}
function saveFields() {
  if (!editable.value) return;
  if (editingNewFields.value) {
    // Keep new fields in the row draft until the status and fields are saved together.
    newFields.value = fields.value.map((field) => ({ ...field }));
    editingNewFields.value = false;
    return;
  }
  save({
    status_rules: props.form.status_rules.map((rule) =>
      rule.status_id === editingStatusId.value
        ? { ...rule, fields: fields.value.map((field) => ({ ...field })) }
        : rule,
    ),
  });
}
function cancelEdit() {
  if (!props.saving) {
    editingStatusId.value = null;
    editingNewFields.value = false;
  }
}
// The parent replaces the form only after a successful save or reload.
watch(
  () => props.form,
  () => {
    adding.value = false;
    newFields.value = [];
    editingNewFields.value = false;
    editingStatusId.value = null;
  },
);
</script>

<style scoped lang="less">
.rule-row {
  display: grid;
  grid-template-columns: minmax(120px, 1fr) minmax(0, 3fr) 140px;
  align-items: center;
  gap: 12px;
  min-height: 48px;
  padding: 8px 4px;
  border-bottom: 1px solid var(--ant-color-border-secondary, #f0f0f0);
}
.rule-head {
  color: var(--ant-color-text-secondary, #888);
}
.rule-actions {
  justify-self: end;
  text-align: right;
}
.field-summary {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  overflow-wrap: anywhere;
}
.field-option-name {
  display: flex;
  align-items: center;
  min-width: 0;
}
.field-section-title {
  padding-top: 16px;
  font-weight: 500;
}
.field-empty {
  padding: 12px 0;
  color: var(--ant-color-text-secondary, #888);
}
.field-option {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 0;
  border-bottom: 1px solid var(--ant-color-border-secondary, #f0f0f0);
}

.transition-field-placeholder {
  background: var(--ant-color-primary-bg, #f0f7ff);
}
.transition-field-placeholder > * {
  visibility: hidden;
}
.transition-field-drag-preview {
  background: var(--ant-color-bg-container, #fff);
  border-radius: 6px;
  box-shadow: 0 3px 10px rgb(16 24 40 / 12%);
  /* Sortable sets the floating preview opacity inline. */
  opacity: 1 !important;
}
</style>

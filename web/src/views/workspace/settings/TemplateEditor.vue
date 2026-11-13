<template>
  <div class="template-editor">
    <div class="editor-top">
      <a-button @click="$emit('cancel')">
        <ArrowLeftOutlined />
        {{ $t("common_back") }}
      </a-button>
      <h3 class="editor-title">{{ $t("settings_template_edit_title") }}</h3>
      <a-space>
        <a-button @click="$emit('cancel')">{{ $t("common_cancel") }}</a-button>
        <a-button type="primary" :loading="saving" @click="save">{{
          $t("common_save")
        }}</a-button>
      </a-space>
    </div>

    <div class="editor-body">
      <div class="form-item">
        <label class="required">{{ $t("settings_template_name") }}</label>
        <a-input
          v-model:value="name"
          :maxlength="50"
          :placeholder="$t('settings_template_name_placeholder')"
        />
      </div>
      <h4 class="section-title">{{ $t("settings_template_content") }}</h4>
      <div class="form-item">
        <label>{{ $t("settings_template_title") }}</label>
        <a-input v-model:value="title" :maxlength="50" />
      </div>
      <div class="form-item">
        <label>{{ $t("settings_template_description") }}</label>
        <RichTextEditor
          v-model="description"
          v-model:content-mode="descriptionMode"
          :ws-id="wsId"
          :allow-uploads="false"
          :min-height="220"
          :placeholder="$t('requirement_descriptionPlaceholder')"
          plain
        />
      </div>
      <div class="section-heading">
        <h4 class="section-title">{{ $t("settings_template_fields") }}</h4>
        <a-button type="link" @click="openFieldManager">
          <SettingOutlined />
          {{ $t("settings_template_manage_fields") }}
        </a-button>
      </div>
      <div class="field-table">
        <div class="field-head">
          <span></span>
          <span>{{ $t("settings_template_field_name") }}</span>
          <span></span>
          <span>{{ $t("settings_template_default_value") }}</span>
          <span></span>
          <span>{{ $t("settings_template_required") }}</span>
          <span></span>
        </div>
        <TemplateFieldRow
          v-for="field in fields"
          :key="field.field_key"
          :field="field"
          :meta="
            metaMap[field.field_key] || {
              label: field.field_key,
              widget: 'text',
            }
          "
          :dragging="draggingKey === field.field_key"
          :linkage-enabled="linkageEnabled(metaMap[field.field_key])"
          :linkage-targets="linkageTargets(field.field_key)"
          @update="replaceField"
          @remove="removeField(field.field_key)"
          @dragstart="onDragStart($event, field.field_key)"
          @dragend="draggingKey = ''"
          @drop="onDrop(field.field_key)"
        />
      </div>
    </div>

    <a-modal
      v-model:open="fieldManagerOpen"
      :title="$t('settings_template_manage_fields')"
      :ok-text="$t('common_confirm')"
      :cancel-text="$t('common_cancel')"
      @ok="applyManagedFields"
    >
      <div class="field-manager">
        <h4>{{ $t("settings_template_builtin_fields") }}</h4>
        <div class="field-options">
          <a-checkbox
            v-for="meta in managedBuiltinMetas"
            :key="meta.value"
            :title="meta.label"
            :checked="managedFieldKeys.includes(meta.value)"
            @change="toggleManagedField(meta.value, $event.target.checked)"
          >
            {{ meta.label }}
          </a-checkbox>
        </div>
        <div class="custom-fields-heading">
          <h4>{{ $t("settings_template_custom_fields") }}</h4>
          <a-button
            type="link"
            size="small"
            @click="customFieldEditorOpen = true"
          >
            <PlusOutlined />
            {{ $t("settings_newField") }}
          </a-button>
        </div>
        <div class="field-options">
          <a-checkbox
            v-for="meta in managedCustomMetas"
            :key="meta.value"
            :title="meta.label"
            :checked="managedFieldKeys.includes(meta.value)"
            @change="toggleManagedField(meta.value, $event.target.checked)"
          >
            {{ meta.label }}
          </a-checkbox>
        </div>
      </div>
    </a-modal>

    <CustomFieldEditorModal
      v-model:open="customFieldEditorOpen"
      :ws-id="wsId"
      :type-key="typeKey"
      @saved="onCustomFieldSaved"
    />
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from "vue";
import { message } from "ant-design-vue";
import {
  ArrowLeftOutlined,
  PlusOutlined,
  SettingOutlined,
} from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import RichTextEditor from "@/components/common/RichTextEditor.vue";
import CustomFieldEditorModal from "./components/CustomFieldEditorModal.vue";
import TemplateFieldRow from "./TemplateFieldRow.vue";
import {
  createWorkItemTemplate,
  getWorkspaceMetadata,
  listCustomFields,
  listFieldConfigs,
  listRequirementCategories,
  updateWorkItemTemplate,
} from "@/api/workspace";
import { TEMPLATE_BUILTIN_KEYS } from "@/constants/workItemTemplate";
import { isValidPhoneNumber } from "@/utils/customField";
import {
  currentProjectMembers,
  customTemplateFieldKey,
  emptyTemplateField,
  presetTemplateFields,
  templateAssigneeIds,
} from "@/utils/workItemTemplate";

const props = defineProps({
  wsId: { type: [String, Number], default: "" },
  typeKey: { type: String, default: "requirement" },
  template: { type: Object, default: () => ({}) },
});
const emit = defineEmits(["cancel", "saved"]);
const { t } = useI18n();

const name = ref("");
const title = ref("");
const description = ref("");
const descriptionMode = ref("rich-text");
const fields = ref([]);
const projectMembers = ref([]);
const metas = ref([]);
const saving = ref(false);
const draggingKey = ref("");
const fieldManagerOpen = ref(false);
const managedFieldKeys = ref([]);
const customFieldEditorOpen = ref(false);

// The field manager modal lists every field of the type, regardless of the
// enable switches in the attribute-fields tab; `metas` only filters the
// preset fields of a new template.
const managedMetas = ref([]);
const metaMap = computed(() =>
  Object.fromEntries(managedMetas.value.map((item) => [item.value, item])),
);
const managedBuiltinMetas = computed(() =>
  managedMetas.value.filter((item) => !item.value.startsWith("custom:")),
);
const managedCustomMetas = computed(() =>
  managedMetas.value.filter((item) => item.value.startsWith("custom:")),
);

onMounted(async () => {
  await loadMetas();
  name.value = props.template.name || "";
  title.value = props.template.title || "";
  description.value = props.template.description || "";
  descriptionMode.value = props.template.description_mode || "rich-text";
  const rows = props.template.id
    ? (props.template.fields || []).map((item) => ({ ...item }))
    : presetTemplateFields(metas.value.map((item) => item.value));
  fields.value = sanitizeAssigneeDefaults(rows);
});

async function loadMetas() {
  const [configs, customs, metadata, categoryResult] = await Promise.all([
    listFieldConfigs(props.wsId, props.typeKey),
    listCustomFields(props.wsId, props.typeKey),
    getWorkspaceMetadata(props.wsId),
    props.typeKey === "requirement"
      ? listRequirementCategories(props.wsId)
      : Promise.resolve({ list: [] }),
  ]);
  const categories = buildCategoryOptions(categoryResult?.list || []);
  projectMembers.value = currentProjectMembers(metadata?.members);
  const typeMeta = metadata?.[props.typeKey] || {};
  const next = [];
  const managed = [];
  for (const key of TEMPLATE_BUILTIN_KEYS) {
    if (
      (key === "estimated_work_hours" || key === "category") &&
      props.typeKey !== "requirement"
    )
      continue;
    const config = (configs || []).find((item) => item.key === key);
    const meta = {
      value: key,
      label: config?.label || key,
      widget: builtinWidget(key),
      fieldType: key,
      options: builtinOptions(key, typeMeta, metadata, categories),
    };
    managed.push(meta);
    if (config && Number(config.enabled) !== 1) continue;
    next.push(meta);
  }
  for (const field of customs || []) {
    const meta = {
      value: customTemplateFieldKey(field.id),
      label: field.name,
      widget: customWidget(field.field_type),
      fieldType: field.field_type,
      workspaceRequired:
        Number(field.is_enabled) === 1 && Number(field.is_required) === 1,
      options: customOptions(field, metadata),
      cascader: field.cascader || [],
    };
    managed.push(meta);
    if (Number(field.is_enabled) !== 1) continue;
    next.push(meta);
  }
  metas.value = next;
  managedMetas.value = managed;
}

function builtinWidget(key) {
  if (key === "planned_start_date" || key === "planned_end_date")
    return "unix-date";
  if (key === "assignee") return "multi";
  if (key === "estimated_work_hours") return "number";
  return "select";
}

function builtinOptions(key, typeMeta, metadata, categories) {
  if (key === "status") {
    return (typeMeta.statuses || []).map((item) => ({
      value: Number(item.id),
      label: item.name,
    }));
  }
  if (key === "priority") {
    return (typeMeta.priorities || []).map((item) => ({
      value: Number(item.id),
      label: item.label || item.name,
    }));
  }
  if (key === "version") {
    return (metadata.versions || []).map((item) => ({
      value: Number(item.id),
      label: item.name,
    }));
  }
  if (key === "category") return categories;
  if (key === "assignee") {
    return activeMembers(metadata).map((item) => ({
      value: Number(item.user_id),
      label: item.user_name,
    }));
  }
  return [];
}

function customWidget(fieldType) {
  if (fieldType === "date") return "date";
  if (fieldType === "datetime") return "datetime";
  if (fieldType === "phone") return "phone";
  if (fieldType === "number" || fieldType === "numeric") return "number";
  if (fieldType === "textarea") return "textarea";
  if (fieldType === "cascader_single") return "cascader";
  if (fieldType === "cascader_multi") return "cascader-multi";
  if (fieldType === "select_multi" || fieldType === "user_multi")
    return "multi";
  if (fieldType === "select_single" || fieldType === "user") return "select";
  return "text";
}

function activeMembers() {
  return projectMembers.value;
}

function sanitizeAssigneeDefaults(rows) {
  return rows.map((field) => {
    if (field.field_key !== "assignee" || !Array.isArray(field.default_value)) {
      return field;
    }
    return {
      ...field,
      default_value: templateAssigneeIds(
        field.default_value,
        projectMembers.value,
      ),
    };
  });
}

function customOptions(field, metadata) {
  if (field.field_type === "user" || field.field_type === "user_multi") {
    return activeMembers(metadata).map((item) => ({
      value: Number(item.user_id),
      label: item.user_name,
    }));
  }
  return (field.options || []).map((item) => ({
    value: item.key,
    label: item.name,
  }));
}

function replaceField(next) {
  fields.value = fields.value.map((item) =>
    item.field_key === next.field_key ? next : item,
  );
}
function removeField(key) {
  fields.value = cleanLinkages(
    fields.value.filter((item) => item.field_key !== key),
  );
}
function openFieldManager() {
  managedFieldKeys.value = fields.value.map((item) => item.field_key);
  fieldManagerOpen.value = true;
}
// A field created from the manager modal is type-level (it also shows up in the
// attribute-fields tab); reload metas and auto-check the newly created ones.
async function onCustomFieldSaved() {
  const previousCustomKeys = new Set(
    managedCustomMetas.value.map((item) => item.value),
  );
  await loadMetas();
  const addedKeys = managedCustomMetas.value
    .filter((item) => !previousCustomKeys.has(item.value))
    .map((item) => item.value);
  if (addedKeys.length) {
    managedFieldKeys.value = [
      ...new Set([...managedFieldKeys.value, ...addedKeys]),
    ];
  }
}
function toggleManagedField(key, checked) {
  managedFieldKeys.value = checked
    ? [...managedFieldKeys.value, key]
    : managedFieldKeys.value.filter((item) => item !== key);
}
function applyManagedFields() {
  const selected = new Set(managedFieldKeys.value);
  const retained = fields.value.filter((item) => selected.has(item.field_key));
  const retainedKeys = new Set(retained.map((item) => item.field_key));
  for (const meta of managedMetas.value) {
    if (selected.has(meta.value) && !retainedKeys.has(meta.value)) {
      retained.push(emptyTemplateField(meta.value));
    }
  }
  fields.value = cleanLinkages(retained);
  fieldManagerOpen.value = false;
}
function cleanLinkages(items) {
  const keys = new Set(items.map((item) => item.field_key));
  return items.map((item) => ({
    ...item,
    linkages: (item.linkages || [])
      .map((rule) => ({
        ...rule,
        field_keys: (rule.field_keys || []).filter((key) => keys.has(key)),
      }))
      .filter((rule) => rule.field_keys.length),
  }));
}
function linkageEnabled(meta) {
  if (!meta || !["select", "multi"].includes(meta.widget)) return false;
  return !["assignee", "user", "user_multi"].includes(meta.fieldType);
}
function linkageTargets(fieldKey) {
  return fields.value
    .filter(
      (item) =>
        item.field_key !== fieldKey &&
        !metaMap.value[item.field_key]?.workspaceRequired,
    )
    .map((item) => ({
      value: item.field_key,
      label: metaMap.value[item.field_key]?.label || item.field_key,
    }));
}
function buildCategoryOptions(items) {
  const result = [];
  const children = new Map();
  for (const item of items) {
    const parentId = Number(item.parent_id) || 0;
    if (!children.has(parentId)) children.set(parentId, []);
    children.get(parentId).push(item);
  }
  const visited = new Set();
  const append = (parentId, depth) => {
    for (const item of children.get(parentId) || []) {
      const id = Number(item.id);
      if (!id || visited.has(id)) continue;
      visited.add(id);
      result.push({ value: id, label: `${"　".repeat(depth)}${item.name}` });
      append(id, depth + 1);
    }
  };
  append(0, 0);
  return result;
}
function onDragStart(event, key) {
  draggingKey.value = key;
  event.dataTransfer.effectAllowed = "move";
}
function onDrop(targetKey) {
  const from = draggingKey.value;
  draggingKey.value = "";
  if (!from || from === targetKey) return;
  const list = [...fields.value];
  const fromIndex = list.findIndex((item) => item.field_key === from);
  const toIndex = list.findIndex((item) => item.field_key === targetKey);
  if (fromIndex < 0 || toIndex < 0) return;
  const [moved] = list.splice(fromIndex, 1);
  list.splice(toIndex, 0, moved);
  fields.value = list;
}

async function save() {
  if (!name.value.trim()) {
    message.warning(t("settings_template_name_required"));
    return;
  }
  const missingRequired = managedCustomMetas.value.find(
    (meta) =>
      meta.workspaceRequired &&
      !fields.value.some((field) => field.field_key === meta.value),
  );
  if (missingRequired) {
    message.warning(
      t("settings_template_workspace_required", {
        name: missingRequired.label,
      }),
    );
    return;
  }
  if (
    fields.value.some((field) => {
      const meta = metaMap.value[field.field_key];
      return (
        meta?.fieldType === "phone" && !isValidPhoneNumber(field.default_value)
      );
    })
  ) {
    message.warning(t("settings_fieldPhoneInvalid"));
    return;
  }
  saving.value = true;
  try {
    const payload = {
      name: name.value.trim(),
      title: title.value,
      description: description.value,
      description_mode: descriptionMode.value || "rich-text",
      fields: fields.value.map((field) => ({
        ...field,
        required: !!(
          field.required || metaMap.value[field.field_key]?.workspaceRequired
        ),
        linkages: (field.linkages || []).filter(
          (rule) => rule.value != null && rule.field_keys?.length,
        ),
      })),
    };
    if (props.template.id) {
      await updateWorkItemTemplate(
        props.wsId,
        props.typeKey,
        props.template.id,
        payload,
      );
    } else {
      await createWorkItemTemplate(props.wsId, props.typeKey, payload);
    }
    message.success(t("common_saveSuccess"));
    emit("saved");
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped lang="less">
.template-editor {
  min-height: 100%;
}
.editor-top {
  // Pinned to the top of .settings-content-body (the nearest scrollport) so the
  // back/cancel/save actions stay reachable while the form scrolls.
  position: sticky;
  top: 0;
  z-index: 2;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
  margin-bottom: 12px;
  background: #fff;
}
.editor-title {
  flex: 1;
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}
.editor-body {
  max-width: 960px;
}
.section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 20px;
}
.section-title {
  margin: 0 0 12px;
  color: #1677ff;
  font-size: 14px;
}
.section-heading .section-title {
  margin: 0;
}
.form-item {
  margin-bottom: 16px;
  label {
    display: block;
    margin-bottom: 6px;
    color: #475467;
  }
  .required::before {
    margin-right: 4px;
    color: #ff4d4f;
    content: "*";
  }
}
.field-table {
  overflow: hidden;
  border: 1px solid #eef1f6;
  border-radius: 8px;
}
.field-head {
  display: grid;
  grid-template-columns:
    24px minmax(80px, 140px) 72px minmax(160px, 1fr)
    auto auto 32px;
  gap: 8px;
  padding: 10px 12px;
  background: #f5f7fa;
  color: #667085;
  font-size: 12px;
}
.field-manager h4 {
  margin: 16px 0 12px;
}
.field-manager h4:first-child {
  margin-top: 0;
}
.custom-fields-heading {
  display: flex;
  align-items: center;
  gap: 16px;
  margin: 12px 0;
  h4 {
    margin: 0;
  }
}
.field-options {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px 20px;
  width: 100%;
  // Long field names collapse to one line with an ellipsis; the native
  // title on the checkbox shows the full label on hover.
  :deep(.ant-checkbox-wrapper) {
    display: flex;
    align-items: center;
    max-width: 100%;
    min-width: 0;
    margin-inline-start: 0;
    > span:not(.ant-checkbox) {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }
}
</style>

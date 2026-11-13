<template>
  <div class="issue-template-detail">
    <div class="issue-template-detail__main">
      <section v-if="!create" class="issue-template-detail__banner">
        <div class="issue-template-detail__banner-text">
          <strong>{{ $t("issue_invite_link_title") }}</strong>
          <p>{{ $t("issue_invite_openid_hint", { code: "openid=XX" }) }}</p>
        </div>
        <a-button
          v-if="canCopyLink"
          type="primary"
          class="issue-template-detail__copy"
          @click="copyLink"
        >
          {{ $t("issue_invite_copy_link") }}
        </a-button>
      </section>

      <section class="issue-template-detail__card">
        <header class="issue-template-detail__card-head">
          <h3 class="issue-template-detail__section-title">
            {{ $t("issue_invite_copy_text") }}
          </h3>
          <span class="issue-template-detail__enabled">
            <a-button
              type="link"
              size="small"
              :disabled="!guideEnabled"
              @click="copyTextModalOpen = true"
            >
              {{ $t("issue_invite_copy_text_edit") }}
            </a-button>
            <a-switch
              :checked="guideEnabled"
              :checked-children="$t('common_on')"
              :un-checked-children="$t('common_off')"
              @change="onGuideEnabledChange"
            />
          </span>
        </header>
      </section>

      <section class="issue-template-detail__card" style="min-height: 58px">
        <header class="issue-template-detail__card-head">
          <a-flex align="center" :gap="8">
            <h3 class="issue-template-detail__section-title">
              {{ $t("issue_invite_recall") }}
            </h3>
            <span
              v-if="!recallEnabled"
              class="issue-template-detail__recall-hint"
            >
              {{ $t("issue_invite_recall_hint") }}
            </span>
            <template v-if="recallEnabled">
              <span class="issue-template-detail__recall-label">
                {{ $t("issue_invite_recall_status_label") }}
              </span>
              <a-select
                v-model:value="recallStatusValue"
                size="small"
                class="issue-template-detail__recall-status"
                :placeholder="$t('issue_invite_select_placeholder')"
                :options="recallStatusOptions"
                @change="onRecallStatusChange"
              />
            </template>
          </a-flex>

          <a-switch
            :checked="recallEnabled"
            :checked-children="$t('common_on')"
            :un-checked-children="$t('common_off')"
            @change="onRecallEnabledChange"
          />
        </header>
      </section>

      <section class="issue-template-detail__card">
        <header class="issue-template-detail__card-head">
          <h3 class="issue-template-detail__section-title">
            {{ $t("issue_invite_field_list") }}
          </h3>
          <div class="issue-template-detail__field-actions">
            <a-tooltip :title="$t('issue_invite_field_list_hint')">
              <QuestionCircleOutlined class="issue-template-detail__help" />
            </a-tooltip>
            <a-button type="link" size="small" @click="fieldManagerOpen = true">
              <SettingOutlined />
              {{ $t("settings_template_manage_fields") }}
            </a-button>
          </div>
        </header>
        <div class="issue-template-detail__field-table">
          <div
            class="issue-template-detail__field-row issue-template-detail__field-row--head"
          >
            <span></span>
            <span>{{ $t("issue_invite_field_name") }}</span>
            <span class="issue-template-detail__toggle-col">
              {{ $t("issue_invite_field_link_extraction") }}
              <a-tooltip :overlay-inner-style="{ maxWidth: '420px' }">
                <template #title>
                  <div>{{ $t("issue_invite_field_link_extraction_hint") }}</div>
                  <div>{{ $t("issue_invite_link_example_text") }}</div>
                  <div>{{ $t("issue_invite_link_example_textarea") }}</div>
                  <div>{{ $t("issue_invite_link_example_number") }}</div>
                  <div>{{ $t("issue_invite_link_example_date") }}</div>
                  <div>{{ $t("issue_invite_link_example_select") }}</div>
                  <div>{{ $t("issue_invite_link_example_multi") }}</div>
                </template>
                <QuestionCircleOutlined class="issue-template-detail__help" />
              </a-tooltip>
            </span>
            <span class="issue-template-detail__toggle-col">{{
              $t("issue_invite_field_source")
            }}</span>
            <span class="issue-template-detail__required-col">{{
              $t("settings_template_required")
            }}</span>
          </div>
          <div ref="fieldListRef" class="issue-template-detail__field-list">
            <div
              v-for="field in listFields"
              :key="field.field_key"
              :data-field-key="field.field_key"
              class="issue-template-detail__field-row"
            >
              <!-- 排序图标：拖这一列调顺序，填写页和详情页都按这个顺序展示 -->
              <span
                class="issue-template-detail__handle"
                :title="$t('issue_invite_field_sort_hint')"
              >
                <HolderOutlined />
              </span>
              <span
                class="issue-template-detail__field-name"
                :title="fieldLabel(field)"
              >
                {{ fieldLabel(field) }}
              </span>
              <!-- 参数名留空表示这个字段不从链接取值；ID 和状态由服务端定，不吃链接传参 -->
              <span class="issue-template-detail__param-col">
                <a-input
                  v-model:value="field.link_param"
                  size="small"
                  :placeholder="$t('issue_invite_enter_placeholder')"
                  :maxlength="64"
                  :disabled="LOCKED_LINK_KEYS.has(field.field_key)"
                  allow-clear
                  @blur="scheduleSave"
                />
              </span>
              <span class="issue-template-detail__source-col">
                <a-tag
                  v-if="isBuiltinField(field)"
                  class="issue-template-detail__source-tag"
                >
                  {{ fieldSource(field) }}
                </a-tag>
                <!-- 自定义字段只显示类型名，不加标签底色 -->
                <span v-else class="issue-template-detail__source-text">
                  {{ fieldSource(field) }}
                </span>
              </span>
              <!-- 必填和移除放同一格并整体左对齐：无论行内有没有删除图标，勾选框都在同一条竖线上 -->
              <span class="issue-template-detail__required-col">
                <a-checkbox
                  :checked="isRequired(field)"
                  :disabled="LOCKED_ON_KEYS.has(field.field_key)"
                  @change="
                    (event) =>
                      patchField(field, { required: event.target.checked })
                  "
                />
                <!-- 移除只是把这个字段从公开表单里撤掉，配置留在模板里，管理字段弹窗能再勾回来 -->
                <DeleteOutlined
                  v-if="!LOCKED_ON_KEYS.has(field.field_key)"
                  class="issue-template-detail__remove"
                  @click="removeField(field)"
                />
              </span>
            </div>
          </div>
        </div>
      </section>
    </div>

    <IssueFieldManagerModal
      v-model:open="fieldManagerOpen"
      :ws-id="wsId"
      :type-key="typeKey"
      :fields="fields"
      @confirm="applyManagedFields"
      @field-created="emit('field-created')"
    />

    <IssueInviteCopyTextModal
      v-model:open="copyTextModalOpen"
      v-model:guide-title="guideTitle"
      v-model:guide-description="guideDescription"
      v-model:theme-image="themeImage"
      :ws-id="wsId"
      @confirm="flushSave"
      @upload="handleCopyTextUpload"
    />

    <IssuePhonePreview
      mode="form"
      :fields="previewFields"
      :theme-image="themeImage"
      :guide-title="guideTitle"
      :guide-description="guideDescription"
      :config-enabled="guideEnabled"
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
import { message } from "ant-design-vue";
import Sortable from "sortablejs";
import {
  DeleteOutlined,
  HolderOutlined,
  QuestionCircleOutlined,
  SettingOutlined,
} from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import { CUSTOM_FIELD_TYPE_LABELS } from "@/constants/customField";
import IssueFieldManagerModal from "./IssueFieldManagerModal.vue";
import IssueInviteCopyTextModal from "./IssueInviteCopyTextModal.vue";
import IssuePhonePreview from "./IssuePhonePreview.vue";

const props = defineProps({
  template: { type: Object, default: null },
  wsId: { type: [String, Number], default: "" },
  inviteKey: { type: String, default: "" },
  canCopyLink: { type: Boolean, default: false },
  // Create mode: nothing is saved until the operator hits save, and the field list comes
  // from the project instead of the template.
  create: { type: Boolean, default: false },
  // Project fields of the selected type, used to build the list in create mode.
  projectFields: { type: Array, default: () => [] },
  // Statuses of the selected type, for the recall-status select.
  statusOptions: { type: Array, default: () => [] },
  save: { type: Function, required: true },
});

const emit = defineEmits(["field-created"]);

// 模板名称由页头的行内输入框改，这里只跟着存进 payload
const name = defineModel("name", { type: String, default: "" });

const { t } = useI18n();

// 这六个字段在公开表单里永远不画，保存时 display 一律压成关：ID、进度、剩余与超期工时
// 在列表里照旧列出但开关置灰，创建人/创建时间连列表都不进（见 LIST_HIDDEN_KEYS）；
// 其中 ID 和状态还不吃链接传参
const LOCKED_DISPLAY_KEYS = new Set([
  "id",
  "created_at",
  "created_by",
  "work_progress",
  "remaining_work_hours",
  "overtime_work_hours",
]);
const LOCKED_LINK_KEYS = new Set(["id", "status"]);
// 工单标题和工单内容是公开表单固定的必填项：默认显示、开关置灰，但照样能拖、能链接传参
const LOCKED_ON_KEYS = new Set(["title", "description"]);

const enabled = ref(true);
const themeImage = ref("");
const guideTitle = ref("");
const guideDescription = ref("");
// 邀请文案开关归模板自己，与列表页的项目级开关互不同步；新建模板进来默认关着
const guideEnabled = ref(false);
const typeKey = ref("requirement");
const fields = ref([]);
const fieldListRef = ref(null);
const fieldManagerOpen = ref(false);
const copyTextModalOpen = ref(false);
// 用户撤回开关与撤回后落到的状态；状态 id 为 0 时由后端补「已关闭」默认值
const recallEnabled = ref(false);
const recallStatusId = ref(0);
const dirty = ref(false);
const saving = ref(false);
let saveTimer = null;
let syncedId = null;
let lastSavedToastAt = 0;
// 新建页首次拿到项目字段时全部默认显示，之后再新增的字段默认不显示
let projectFieldsInitialized = false;

// 创建人/创建时间公开表单永远不画，列表里也不再列；工单内容画出来（见 LOCKED_ON_KEYS）
const LIST_HIDDEN_KEYS = new Set(["created_at", "created_by"]);
// 列表只画还留在公开表单里的字段：被移除的（display 关掉）归管理字段弹管，不占列表行
const listFields = computed(() =>
  fields.value.filter(
    (field) =>
      !LIST_HIDDEN_KEYS.has(field.field_key) && field.display !== false,
  ),
);

// 撤回状态下拉的值：0 表示还没选，留空走占位符，别把 0 当成选中项画出来
const recallStatusValue = computed({
  get: () =>
    Number(recallStatusId.value) > 0 ? Number(recallStatusId.value) : undefined,
  set: (value) => {
    recallStatusId.value = Number(value) || 0;
  },
});

// 预览只画公开表单真的会渲染的字段：可填的内置字段和自定义字段
const PREVIEW_BUILTIN_KEYS = new Set([
  "title",
  "description",
  "status",
  "version",
  "category",
  "priority",
  "planned_start_date",
  "planned_end_date",
  "assignee",
  "estimated_work_hours",
  "completed_work_hours",
]);

const previewFields = computed(() =>
  fields.value
    .filter(
      (field) =>
        field.field_key.startsWith("custom:") ||
        PREVIEW_BUILTIN_KEYS.has(field.field_key),
    )
    .map((field) => ({
      label: fieldLabel(field),
      required: field.required,
      display: field.display,
      widget: previewWidget(field),
    })),
);

const recallStatusOptions = computed(() =>
  props.statusOptions.map((status) => ({
    value: Number(status.id),
    label: status.name,
  })),
);

// Builtin keys keep no name in fields_json; map them to i18n labels like the public form does,
// with the same keys the attribute-field settings page uses.
const builtinLabelKeys = {
  id: "workspace_field_id",
  title: "issue_invite_field_title",
  description: "issue_invite_field_description",
  status: "workspace_field_status",
  version: "workspace_field_iteration",
  category: "workspace_field_category",
  priority: "workspace_field_priority",
  planned_start_date: "workspace_field_plannedStartDate",
  planned_end_date: "workspace_field_plannedEndDate",
  assignee: "workspace_field_assignee",
  created_at: "workspace_field_createdTime",
  created_by: "workspace_field_createdBy",
  estimated_work_hours: "requirement_estimatedWorkHours",
  work_progress: "workspace_field_work_progress",
  completed_work_hours: "requirement_completedWorkHours",
  remaining_work_hours: "workspace_field_remaining_work_hours",
  overtime_work_hours: "workspace_field_overtime_work_hours",
};

// 标题和内容永远开、永远关不掉的几个字段永远关，其余按存下来的值
function resolveDisplay(fieldKey, stored) {
  if (LOCKED_ON_KEYS.has(fieldKey)) return true;
  if (LOCKED_DISPLAY_KEYS.has(fieldKey)) return false;
  return stored !== false;
}

function parseFields(raw) {
  let rows = [];
  try {
    rows = JSON.parse(raw || "[]");
  } catch (error) {
    return [];
  }
  return rows.map((row) => ({
    field_key: row.field_key,
    field_name: row.field_name || "",
    field_type: row.field_type || "",
    required: !!row.required,
    display: resolveDisplay(row.field_key, row.display),
    link_param: row.link_param || "",
  }));
}

// 新建时字段列表直接取项目的属性字段：首次进来全部默认显示，之后新增的字段默认不显示
function fieldsFromProject() {
  const previous = new Map(
    fields.value.map((field) => [field.field_key, field]),
  );
  const firstBuild =
    !projectFieldsInitialized && props.projectFields.length > 0;
  const rows = props.projectFields.map((field) => {
    const existing = previous.get(field.field_key);
    const display = resolveDisplay(
      field.field_key,
      existing ? existing.display : firstBuild,
    );
    return {
      field_key: field.field_key,
      field_name: field.field_name || "",
      field_type: field.field_type || "",
      required: !!field.required,
      display,
      link_param: existing ? existing.link_param : "",
    };
  });
  if (rows.length) projectFieldsInitialized = true;
  return rows;
}

function fieldLabel(field) {
  if (field.field_key.startsWith("custom:"))
    return field.field_name || field.field_key;
  const key = builtinLabelKeys[field.field_key];
  return key ? t(key) : field.field_name || field.field_key;
}

// 来源列：内置字段统一显示「内置」，自定义字段显示它自己的类型
function fieldSource(field) {
  if (!field.field_key.startsWith("custom:"))
    return t("issue_invite_field_source_builtin");
  const key = CUSTOM_FIELD_TYPE_LABELS[field.field_type];
  return key ? t(key) : t("issue_invite_field_source_custom");
}

// 内置字段用带底色的标签，自定义字段只显示类型名
function isBuiltinField(field) {
  return !field.field_key.startsWith("custom:");
}

function previewWidget(field) {
  if (field.field_key === "description") return "textarea";
  if (
    field.field_key === "planned_start_date" ||
    field.field_key === "planned_end_date"
  )
    return "date";
  if (
    field.field_key === "assignee" ||
    field.field_key === "version" ||
    field.field_key === "category" ||
    field.field_key === "priority" ||
    field.field_key === "status" ||
    field.field_type === "user"
  )
    return "select";
  if (
    field.field_type === "select_single" ||
    field.field_type === "select_multi"
  )
    return "select";
  if (field.field_type === "date") return "date";
  if (field.field_type === "textarea") return "textarea";
  return "text";
}

// 邀请模板不存字段数据，只记录显示、必填与链接参数名
function buildFieldsJson(rows) {
  return JSON.stringify(
    rows.map((row) => ({
      field_key: row.field_key,
      display: !!row.display,
      // 标题和内容是公开表单的固定必填项，服务端也会这样钉死
      required: isRequired(row),
      link_param: (row.link_param || "").trim(),
    })),
  );
}

function buildPayload() {
  return {
    type_key: typeKey.value,
    name: name.value,
    description_mode: props.template?.description_mode || "rich-text",
    fields_json: buildFieldsJson(fields.value),
    theme_image: themeImage.value,
    guide_title: guideTitle.value,
    guide_description: guideDescription.value,
    guide_enabled: guideEnabled.value ? 1 : 0,
    enabled: enabled.value ? 1 : 0,
    recall_enabled: recallEnabled.value ? 1 : 0,
    recall_status_id: Number(recallStatusId.value) || 0,
  };
}

// 开启撤回时先把默认状态选上（已关闭），选不到就留给后端补
function onRecallEnabledChange(value) {
  recallEnabled.value = !!value;
  if (recallEnabled.value && !(Number(recallStatusId.value) > 0)) {
    const fallback = recallStatusOptions.value.find(
      (option) => option.label === t("requirement_closed"),
    );
    if (fallback) recallStatusId.value = Number(fallback.value);
  }
  scheduleSave();
}

function onRecallStatusChange() {
  scheduleSave();
}

function scheduleSave() {
  dirty.value = true;
  // 新建要先在页头填名称，保存由页面上的按钮触发，不自动提交
  if (props.create) return;
  clearTimeout(saveTimer);
  saveTimer = setTimeout(saveNow, 800);
}

async function saveNow() {
  if (saving.value) return;
  // 新建时还没有模板 ID，任何自动保存（含失焦）都不能发请求
  if (props.create) return;
  if (!dirty.value) return;
  saving.value = true;
  try {
    const item = await props.save(buildPayload());
    // 后端会给撤回状态补默认值，本地还没选就回填，免得下拉框空着
    if (
      !(Number(recallStatusId.value) > 0) &&
      Number(item?.recall_status_id) > 0
    ) {
      recallStatusId.value = Number(item.recall_status_id);
    }
    dirty.value = false;
    // A light toast, throttled so rapid autosaves don't spam.
    const now = Date.now();
    if (now - lastSavedToastAt > 3000) {
      lastSavedToastAt = now;
      message.success(t("issue_invite_saved"));
    }
  } catch (error) {
    // The request interceptor already reports the error; keep the form dirty to retry later.
  } finally {
    saving.value = false;
  }
}

async function flushSave() {
  clearTimeout(saveTimer);
  await saveNow();
}

function patchField(field, partial) {
  Object.assign(field, partial);
  scheduleSave();
}

// 标题和内容是公开表单固定的必填项，其余字段看模板里勾的必填
function isRequired(field) {
  return LOCKED_ON_KEYS.has(field.field_key) ? true : !!field.required;
}

// 移除只是把字段从公开表单里撤掉，行留在 fields 里，管理字段弹窗能再勾回来
function removeField(field) {
  field.display = false;
  scheduleSave();
}

// 管理字段弹窗确认：勾上的显示，没勾的撤下；标题和内容永远显示，永不显示的字段照旧关掉
function applyManagedFields(keys) {
  const selected = new Set(keys);
  fields.value = fields.value.map((field) => ({
    ...field,
    display: resolveDisplay(field.field_key, selected.has(field.field_key)),
  }));
  scheduleSave();
}

// 拖动排序只动 DOM，落点按 data-field-key 读回来重排 fields，存的就是这个顺序；
// 填写页和详情页都按 fields_json 的顺序渲染
let sortable = null;

function reorderFromDom() {
  const rows = fieldListRef.value?.querySelectorAll("[data-field-key]");
  if (!rows?.length) return;
  const byKey = new Map(fields.value.map((field) => [field.field_key, field]));
  const ordered = [...rows]
    .map((node) => byKey.get(node.dataset.fieldKey))
    .filter(Boolean);
  const listed = new Set(ordered.map((field) => field.field_key));
  // 列表里不画的字段（创建人、创建时间）没有顺序可言，跟在最后
  fields.value = [
    ...ordered,
    ...fields.value.filter((field) => !listed.has(field.field_key)),
  ];
  scheduleSave();
}

function setupSortable() {
  sortable?.destroy();
  sortable = null;
  if (!fieldListRef.value) return;
  sortable = Sortable.create(fieldListRef.value, {
    animation: 160,
    handle: ".issue-template-detail__handle",
    draggable: "[data-field-key]",
    ghostClass: "issue-template-detail__field-row--ghost",
    onEnd: reorderFromDom,
  });
}

// 邀请文案开关归模板自己：切换即随自动保存落库，开启时顺带打开文案编辑弹窗。
// 列表页的项目级开关走另一条链路，互不同步。
function onGuideEnabledChange(value) {
  guideEnabled.value = !!value;
  if (value) copyTextModalOpen.value = true;
  scheduleSave();
}

// Uploading a theme image is an immediate action: persist it right away with the
// current form values so it survives a quick close or refresh. In create mode
// saveNow is a no-op and the image goes out with the page-level save.
function handleCopyTextUpload(url) {
  themeImage.value = url;
  dirty.value = true;
  clearTimeout(saveTimer);
  saveNow();
}

function buildTemplateUrl() {
  const key = props.template?.template_key;
  if (!props.inviteKey || !key) return "";
  const origin = `${window.location.origin}${import.meta.env?.BASE_URL || "/"}`;
  return `${origin}invite/i/${encodeURIComponent(props.inviteKey)}/${encodeURIComponent(key)}?open_mode=template`;
}

async function copyLink() {
  const text = buildTemplateUrl();
  if (!text) return;
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
    } else {
      const textarea = document.createElement("textarea");
      textarea.value = text;
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      document.body.removeChild(textarea);
    }
    message.success(t("issue_invite_copy_success"));
  } catch (error) {
    message.error(t("issue_invite_copy_failed"));
  }
}

// Re-sync the draft only when the loaded template changes identity; the server
// response of every autosave mirrors the draft, so it must never clobber typing.
let syncing = false;
watch(
  () => props.template,
  (template) => {
    const id = template?.id;
    if (id === undefined || id === syncedId) return;
    syncedId = id;
    syncing = true;
    enabled.value = Number(template.enabled) === 1;
    themeImage.value = template.theme_image || "";
    guideTitle.value = template.guide_title || "";
    guideDescription.value = template.guide_description || "";
    guideEnabled.value = Number(template.guide_enabled ?? 1) === 1;
    typeKey.value = template.type_key || "requirement";
    name.value = template.name || "";
    recallEnabled.value = Number(template.recall_enabled) === 1;
    recallStatusId.value = Number(template.recall_status_id) || 0;
    fields.value = props.create
      ? fieldsFromProject()
      : parseFields(template.fields_json);
    syncing = false;
  },
  { immediate: true },
);

// 新建页的项目字段是异步取回来的，到了就填进列表
watch(
  () => props.projectFields,
  () => {
    if (!props.create || syncing) return;
    fields.value = fieldsFromProject();
  },
);

// 行数变了要重建拖拽：容器一直是同一个，但 Sortable 记的是初始化时的子节点
watch(
  () => listFields.value.length,
  () => nextTick(setupSortable),
);
onMounted(() => nextTick(setupSortable));

// Typing marks the form dirty; the debounced save then fires on pause, and the
// blur handlers flush immediately so focus loss never leaves edits unsaved.
// Sync flush lets the syncing flag guard the initial template load.
watch(
  [name, guideTitle, guideDescription],
  () => {
    if (syncing) return;
    scheduleSave();
  },
  { flush: "sync" },
);

onBeforeUnmount(() => {
  sortable?.destroy();
  clearTimeout(saveTimer);
  if (dirty.value && !saving.value && !props.create) {
    saveNow();
  }
});

// 新增自定义字段后外部会重拉一次，用它把字段列表换成最新的
function refreshFields() {
  fields.value = props.create
    ? fieldsFromProject()
    : parseFields(props.template?.fields_json);
}

defineExpose({ flushSave, buildPayload, refreshFields });
</script>

<style scoped lang="less">
.issue-template-detail {
  display: flex;
  gap: 24px;
  align-items: flex-start;

  &__main {
    flex: 1;
    min-width: 0;
  }

  &__banner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 16px 20px;
    background: #e6f4ff;
    border-radius: 8px;
  }

  &__banner-text {
    color: #1f1f1f;
    font-size: 14px;

    strong {
      font-weight: 600;
    }

    p {
      margin: 4px 0 0;
      color: #5f5f5f;
      font-size: 12px;
    }
  }

  &__copy {
    flex: 0 0 auto;
    background: #0d68ff;
    border-color: #0d68ff;
    border-radius: 6px;
  }

  &__card {
    margin-top: 16px;
    padding: 16px 20px;
    background: #fff;
    border: 1px solid #f0f0f0;
    border-radius: 8px;
  }

  &__card-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  &__section-title {
    margin: 0;
    color: #1f1f1f;
    font-size: 15px;
    font-weight: 600;
  }

  &__enabled {
    display: flex;
    align-items: center;
    gap: 8px;
    color: #5f5f5f;
    font-size: 13px;
  }

  &__recall {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 12px;
    min-height: 32px;
  }

  &__recall-hint {
    color: #8b8b8b;
    font-size: 13px;
  }

  &__recall-label {
    color: #262626;
    font-size: 13px;
  }

  &__recall-status {
    width: 140px;
  }

  &__help {
    color: #8b8b8b;
  }

  // 字段列表标题右侧：帮助图标 + 新增字段按钮
  &__field-actions {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  &__field-table {
    margin-top: 12px;
    overflow-x: auto;
    border: 1px solid #f0f0f0;
    border-radius: 8px;
  }

  &__field-list {
    display: block;
  }

  &__field-row {
    display: grid;
    // 表头和数据行是两个独立栅格，只有全部用固定列宽（弹性列必须同为 1fr）才能保证
    // 每行解析出一样的列宽，上下对齐；末列若用 auto 会按内容各自解析导致错位
    grid-template-columns:
      20px minmax(96px, 1fr) minmax(130px, 1fr)
      72px 48px;
    align-items: center;
    gap: 12px;
    min-width: 440px;
    padding: 10px 14px;

    & + & {
      border-top: 1px solid #f0f0f0;
    }

    &--head {
      background: #fafafa;
      color: #8b8b8b;
      font-size: 12px;

      & + & {
        border-top: none;
      }
    }

    // 拖拽时留在原位的那个影子
    &--ghost {
      opacity: 0.4;
    }
  }

  &__handle {
    display: flex;
    align-items: center;
    justify-content: center;
    color: #8b8b8b;
    cursor: grab;
  }

  &__field-name {
    overflow: hidden;
    color: #30394a;
    font-size: 13px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  &__toggle-col {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 4px;
    white-space: nowrap;
  }

  // 必填列统一左对齐：勾选框固定在列首，带不带删除图标都不会把勾选框挤得左右漂移
  &__required-col {
    display: flex;
    align-items: center;
    justify-content: flex-start;
    gap: 10px;
    white-space: nowrap;
  }

  &__param-col {
    display: flex;
    align-items: center;
  }

  &__source-col {
    display: flex;
    align-items: center;
    justify-content: center;
  }

  &__remove {
    color: #8b8b8b;
    cursor: pointer;

    &:hover {
      color: #fb363f;
    }
  }

  &__source-tag {
    margin: 0;
    color: #0d68ff;
    background: #e6f4ff;
    border-color: #e6f4ff;
    font-size: 12px;
    line-height: 20px;
  }

  // 自定义字段的类型名按普通文本走，跟字段名一列同样的字号
  &__source-text {
    color: #30394a;
    font-size: 13px;
  }
}
</style>

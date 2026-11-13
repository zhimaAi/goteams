<template>
  <section class="issue-form">
    <a-form :model="formState" layout="vertical" @finish="submit">
      <div class="issue-form__card">
        <template v-for="field in fields" :key="field.field_key">
          <a-form-item
            :name="field.field_key"
            :rules="field.rules"
            class="issue-form__row"
          >
            <!-- antd renders its own required asterisk from the rules; do not add a second one. -->
            <template #label>
              <span class="issue-form__label">{{ field.label }}</span>
            </template>
            <a-input
              v-if="field.widget === 'text'"
              v-model:value="formState[field.field_key]"
              :placeholder="t('issue_invite_enter_placeholder')"
              :maxlength="field.maxlength"
              allow-clear
              size="large"
            />
            <a-textarea
              v-else-if="field.widget === 'textarea'"
              v-model:value="formState[field.field_key]"
              :placeholder="t('issue_invite_enter_placeholder')"
              :rows="4"
              :maxlength="field.maxlength"
            />
            <a-select
              v-else-if="field.widget === 'select_single'"
              v-model:value="formState[field.field_key]"
              :options="field.options"
              :placeholder="t('issue_invite_select_placeholder')"
              :allow-clear="!field.required"
              size="large"
            />
            <a-select
              v-else-if="field.widget === 'select_multi'"
              v-model:value="formState[field.field_key]"
              :options="field.options"
              mode="multiple"
              :placeholder="t('issue_invite_select_placeholder')"
              size="large"
            />
            <a-input-number
              v-else-if="field.widget === 'number'"
              v-model:value="formState[field.field_key]"
              :placeholder="t('issue_invite_enter_placeholder')"
              style="width: 100%"
              size="large"
            />
            <a-date-picker
              v-else-if="field.widget === 'date'"
              v-model:value="formState[field.field_key]"
              :placeholder="t('issue_invite_select_placeholder')"
              style="width: 100%"
              size="large"
            />
            <a-input
              v-else
              v-model:value="formState[field.field_key]"
              :placeholder="t('issue_invite_enter_placeholder')"
              :maxlength="field.maxlength"
              allow-clear
              size="large"
            />
          </a-form-item>
          <!-- Attachments belong to the issue content, so the control sits right under
             the description row; templates without a fillable description keep the
             disabled fallback under the title row instead. -->
          <div
            v-if="
              field.field_key === 'description' ||
              (field.field_key === 'title' && !canWriteContent)
            "
            class="issue-form__attach"
          >
            <a-upload
              :show-upload-list="false"
              :before-upload="handleAttachmentUpload"
              :disabled="
                !canWriteContent ||
                submitting ||
                attachmentUploading ||
                uploadedFiles.length >= ATTACHMENT_MAX_COUNT
              "
            >
              <a-button
                class="issue-form__attach-btn"
                :loading="attachmentUploading"
                :disabled="
                  !canWriteContent ||
                  submitting ||
                  uploadedFiles.length >= ATTACHMENT_MAX_COUNT
                "
                :title="
                  uploadedFiles.length >= ATTACHMENT_MAX_COUNT
                    ? t('issue_invite_attachment_max_limit')
                    : ''
                "
              >
                <PlusOutlined />
                {{ t("issue_invite_upload_attachment") }}
              </a-button>
            </a-upload>
            <div v-if="uploadedFiles.length" class="issue-form__attach-list">
              <div
                v-for="file in uploadedFiles"
                :key="file.id"
                class="issue-form__attach-chip"
              >
                <a
                  class="issue-form__attach-link"
                  :href="file.url"
                  target="_blank"
                  rel="noopener"
                >
                  <PaperClipOutlined />
                  <span class="issue-form__attach-name">{{ file.name }}</span>
                  <small class="issue-form__attach-size">{{
                    formatFileSize(file.size)
                  }}</small>
                </a>
                <DeleteOutlined
                  class="issue-form__attach-delete"
                  @click="handleAttachmentDelete(file)"
                />
              </div>
            </div>
          </div>
        </template>
      </div>

      <div class="issue-form__submit-bar">
        <a-button
          v-if="editMode"
          size="large"
          class="issue-form__cancel"
          :disabled="submitting"
          @click="emit('cancel')"
        >
          {{ t("common_cancel") }}
        </a-button>
        <a-button
          type="primary"
          html-type="submit"
          :loading="submitting"
          :disabled="attachmentUploading"
          size="large"
          class="issue-form__submit"
        >
          {{ t(editMode ? "issue_invite_resubmit_action" : "common_submit") }}
        </a-button>
      </div>
    </a-form>
  </section>
</template>

<script setup>
import { computed, reactive, toRef, watch } from "vue";
import { message } from "ant-design-vue";
import dayjs from "dayjs";
import {
  PlusOutlined,
  PaperClipOutlined,
  DeleteOutlined,
} from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import {
  deletePublicIssueAttachment,
  uploadPublicIssueAttachment,
} from "@/api/issueInvite";

const props = defineProps({
  template: { type: Object, default: null },
  submitting: { type: Boolean, default: false },
  inviteKey: { type: String, default: "" },
  openid: { type: String, default: "" },
  lang: { type: String, default: "" },
  // 编辑已提交的工单时传当前值（详情接口的 edit_fields），没有就是新建
  initialValues: { type: Object, default: null },
  // 从邀请链接上取到的参数值（按 field_key 归位），优先于模板默认值
  linkValues: { type: Object, default: null },
  draft: { type: Object, default: null },
  // 编辑已提交工单：底部多一个取消，提交按钮换成再次提交
  editMode: { type: Boolean, default: false },
});
const emit = defineEmits(["submit", "cancel"]);

const { t } = useI18n();

const formDraft =
  props.draft || reactive({ values: {}, files: [], uploading: false });
const formState = formDraft.values;
const attachmentUploading = toRef(formDraft, "uploading");
// Uploaded files stay as chips in the form; only their ids are submitted so the
// backend can bind them to the created issue.
const uploadedFiles = toRef(formDraft, "files");

const ATTACHMENT_MAX_SIZE = 200 * 1024 * 1024;
const ATTACHMENT_MAX_COUNT = 5;

// Builtin keys carry no name in fields_json; map them to i18n labels.
const BUILTIN_LABEL_KEYS = {
  title: "issue_invite_field_title",
  description: "issue_invite_field_description",
  status: "workspace_field_status",
  version: "workspace_field_iteration",
  category: "workspace_field_category",
  priority: "workspace_field_priority",
  planned_start_date: "workspace_field_plannedStartDate",
  planned_end_date: "workspace_field_plannedEndDate",
  assignee: "workspace_field_assignee",
  estimated_work_hours: "requirement_estimatedWorkHours",
  completed_work_hours: "requirement_completedWorkHours",
};

// Builtin keys the submitter may set. This set mirrors the backend's
// issueInviteWritableBuiltinColumns: the template's display switch decides whether a field
// shows, and a builtin outside this set is rejected server-side, so it must not render.
const WRITABLE_BUILTIN_KEYS = new Set([
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

// Builtin selects whose option list comes from project data (the public endpoint injects it).
const BUILTIN_SELECT_KEYS = new Set([
  "status",
  "version",
  "category",
  "priority",
  "assignee",
]);

const TEXTAREA_MAX_LENGTH = 2000;
const INPUT_MAX_LENGTH = 200;

function readSchema() {
  try {
    return JSON.parse(props.template?.fields_json || "[]");
  } catch (error) {
    return [];
  }
}

// Widgets for the renderable set. Non-writable builtins are filtered out before this runs.
function widgetOf(spec) {
  if (spec.field_key === "description") return "textarea";
  if (BUILTIN_SELECT_KEYS.has(spec.field_key)) return "select_single";
  if (
    spec.field_key === "planned_start_date" ||
    spec.field_key === "planned_end_date"
  )
    return "date";
  if (
    spec.field_key === "estimated_work_hours" ||
    spec.field_key === "completed_work_hours"
  )
    return "number";
  if (spec.field_type === "select_single" || spec.field_type === "select_multi")
    return spec.field_type;
  if (spec.field_type === "date") return "date";
  if (spec.field_type === "textarea") return "textarea";
  if (spec.field_type === "number") return "number";
  return "text";
}

function labelOf(spec) {
  if (spec.field_key.startsWith("custom:"))
    return spec.field_name || spec.field_key;
  const key = BUILTIN_LABEL_KEYS[spec.field_key];
  return key ? t(key) : spec.field_name || spec.field_key;
}

function rulesOf(spec) {
  const rules = [];
  if (spec.required) {
    rules.push({
      required: true,
      message: t("issue_invite_enter_placeholder"),
    });
  }
  if (spec.field_type === "email") {
    rules.push({ type: "email", message: t("issue_invite_email_invalid") });
  }
  return rules;
}

// A builtin key is only renderable when the submitter is allowed to write it.
function isRenderable(spec) {
  if (spec.display === false) return false;
  if (spec.field_key.startsWith("custom:")) return true;
  return WRITABLE_BUILTIN_KEYS.has(spec.field_key);
}

// Render exactly what the template enables: writable builtins and custom fields. Builtins the
// public form never draws (id, created_at/by, progress and the hour-derived fields) stay out
// even when a template turns their switch on; `required` comes from the template config
// rather than being hardcoded.
const fields = computed(() =>
  readSchema()
    .filter(isRenderable)
    .map((spec) => {
      const widget = widgetOf(spec);
      return {
        field_key: spec.field_key,
        label: labelOf(spec),
        widget,
        required: !!spec.required,
        maxlength:
          widget === "textarea" ? TEXTAREA_MAX_LENGTH : INPUT_MAX_LENGTH,
        // Builtin options carry numeric ids while custom ones use string keys; normalize
        // to strings so a-select matching and submission stay consistent.
        options: (spec.options || []).map((option) => ({
          value:
            option.key === undefined || option.key === null
              ? ""
              : String(option.key),
          label: option.name ?? "",
        })),
        rules: rulesOf(spec),
      };
    }),
);

function defaultFor(spec) {
  const raw = spec.default_value;
  if (spec.field_type === "select_multi") {
    if (Array.isArray(raw)) return raw;
    if (typeof raw === "string" && raw.startsWith("[")) {
      try {
        return JSON.parse(raw);
      } catch (error) {
        return [];
      }
    }
    return [];
  }
  // 内置下拉存数字 id，转成字符串才能和选项对上，没填过就留空
  if (BUILTIN_SELECT_KEYS.has(spec.field_key)) {
    if (raw === undefined || raw === null || raw === "") return undefined;
    return String(raw);
  }
  return raw ?? "";
}

// 编辑态预填：详情接口给的是原始值，按控件类型转成表单认的格式
function normalizeInitialValue(spec, value) {
  const widget = widgetOf(spec);
  if (value === undefined || value === null || value === "") {
    if (widget === "select_multi") return [];
    if (widget === "date") return undefined;
    return "";
  }
  if (widget === "date") return toDayjsValue(value);
  if (widget === "select_multi") return toValueList(value);
  // 内置下拉是单选，库里可能存成数组，取第一个
  if (widget === "select_single") {
    if (Array.isArray(value))
      return value.length ? String(value[0]) : undefined;
    return String(value);
  }
  if (widget === "number") {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : "";
  }
  return String(value);
}

// 日期预填：内置字段存的是秒级时间戳，自定义字段存的是 UTC 字符串
function toDayjsValue(value) {
  if (typeof value === "number" || /^\d+$/.test(String(value))) {
    return dayjs.unix(Number(value));
  }
  const parsed = dayjs(String(value));
  return parsed.isValid() ? parsed : undefined;
}

// 多选预填：自定义字段存的是选项 key 的 JSON 数组字符串
function toValueList(value) {
  if (Array.isArray(value)) return value.map((item) => String(item));
  const text = String(value);
  if (text.startsWith("[")) {
    try {
      const parsed = JSON.parse(text);
      return Array.isArray(parsed) ? parsed.map((item) => String(item)) : [];
    } catch (error) {
      return [];
    }
  }
  return text ? [text] : [];
}

// 链接传参：下拉类字段链接里传的是选项名称，先换成选项 key，再按控件格式归一化。
// 名称对不上任何选项就当没传，免得填一个提交时必然被拒的值。
function normalizeLinkValue(spec, value) {
  const widget = widgetOf(spec);
  const options = spec.options || [];
  if (widget === "select_multi") {
    const keys = toLinkList(value)
      .map((name) => optionKeyByName(options, name))
      .filter(Boolean);
    return keys.length ? keys : defaultFor(spec);
  }
  if (widget === "select_single") {
    const key = optionKeyByName(options, String(value).trim());
    return key || defaultFor(spec);
  }
  return normalizeInitialValue(spec, value);
}

// 多选两种写法都收：?tags=A&tags=B（重复参数名）和 ?tags=A,B（逗号分隔）
function toLinkList(value) {
  const items = Array.isArray(value) ? value : [value];
  return items
    .flatMap((item) => String(item).split(","))
    .map((item) => item.trim())
    .filter(Boolean);
}

// 按选项名称找选项 key；链接里直接给 key 也认
function optionKeyByName(options, name) {
  const matched = (options || []).find(
    (option) =>
      String(option.name ?? "") === name || String(option.key) === name,
  );
  return matched ? String(matched.key) : "";
}

watch(
  () => [props.template, props.initialValues, props.linkValues],
  () => {
    const next = {};
    const initial = props.initialValues || {};
    const link = props.linkValues || {};
    for (const spec of readSchema()) {
      const key = spec.field_key;
      if (props.draft && Object.prototype.hasOwnProperty.call(formState, key)) {
        next[key] = formState[key];
      } else if (Object.prototype.hasOwnProperty.call(initial, key)) {
        next[key] = normalizeInitialValue(spec, initial[key]);
      } else if (Object.prototype.hasOwnProperty.call(link, key)) {
        next[key] = normalizeLinkValue(spec, link[key]);
      } else {
        next[key] = defaultFor(spec);
      }
    }
    Object.keys(formState).forEach((key) => delete formState[key]);
    Object.assign(formState, next);
  },
  { immediate: true },
);

// The upload button is only usable when the template lets the submitter fill the
// issue content (the writable description field is present in the rendered fields).
const canWriteContent = computed(() =>
  fields.value.some((field) => field.field_key === "description"),
);

async function handleAttachmentUpload(file) {
  if (!canWriteContent.value || props.submitting) return false;
  if (file.size > ATTACHMENT_MAX_SIZE) {
    message.error(t("issue_invite_attachment_too_large"));
    return false;
  }
  if (uploadedFiles.value.length >= ATTACHMENT_MAX_COUNT) {
    message.warning(t("issue_invite_attachment_max_limit"));
    return false;
  }
  attachmentUploading.value = true;
  try {
    const data = await uploadPublicIssueAttachment(
      props.inviteKey,
      props.openid,
      props.lang,
      file,
    );
    const url = String(data?.url || "");
    const fid = Number(data?.id);
    if (!url || !(fid > 0)) {
      message.error(t("issue_invite_attachment_upload_failed"));
      return false;
    }
    uploadedFiles.value.push({
      id: fid,
      name: file.name,
      size: file.size,
      url,
    });
    message.success(t("issue_invite_attachment_uploaded"));
  } catch (error) {
    // The request layer already toasts the server error message.
  } finally {
    attachmentUploading.value = false;
  }
  return false;
}

async function handleAttachmentDelete(file) {
  if (props.submitting) return;
  try {
    await deletePublicIssueAttachment(
      props.inviteKey,
      props.openid,
      file.id,
      props.lang,
    );
    uploadedFiles.value = uploadedFiles.value.filter(
      (item) => item.id !== file.id,
    );
  } catch (error) {
    // The request layer already toasts the server error message.
  }
}

function formatFileSize(size) {
  const bytes = Number(size) || 0;
  if (!bytes) return "";
  if (bytes < 1024 * 1024) return `${Math.ceil(bytes / 1024)}KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)}MB`;
}

function submit() {
  if (props.submitting || attachmentUploading.value) return;
  const payload = [];
  for (const field of fields.value) {
    let value = formState[field.field_key];
    if (field.widget === "select_multi")
      value = Array.isArray(value) ? value : [];
    // Skip untouched optional selects: an empty string would be rejected by the backend
    // id parser, and a blank builtin simply falls back to the project default.
    if (
      field.widget === "select_single" &&
      (value === undefined || value === null || value === "")
    ) {
      continue;
    }
    // 日期控件给的是 dayjs 对象，后端认 YYYY-MM-DD
    if (field.widget === "date") {
      if (!value) continue;
      value =
        typeof value.format === "function"
          ? value.format("YYYY-MM-DD")
          : String(value);
    }
    // The backend stores assignees as an int array, so submit a list even for one pick.
    if (field.field_key === "assignee") {
      value = Array.isArray(value) ? value : [value];
      value = value
        .map((id) => Number(id))
        .filter((id) => Number.isInteger(id) && id > 0);
      if (!value.length) continue;
    }
    // 内置下拉是数字 id，发数字后端才好解析
    if (
      BUILTIN_SELECT_KEYS.has(field.field_key) &&
      field.field_key !== "assignee"
    ) {
      const id = Number(value);
      if (!Number.isInteger(id) || id <= 0) continue;
      value = id;
    }
    payload.push({ field_key: field.field_key, value: value ?? "" });
  }
  emit("submit", {
    fields: payload,
    fileIds: uploadedFiles.value.map((file) => file.id),
  });
}
</script>

<style scoped lang="less">
.issue-form {
  // Outer wrapper. The fields live in their own white card while the submit button is
  // pinned to the viewport bottom; the 120px bottom padding is clearance for it so the
  // last field stays visible.
  padding: 0 16px 120px;
  margin-top: 24px;

  // Fields card: white card (radius 12, soft shadow). Holds all form fields and the
  // upload control. The submit button lives in a separate card below.
  &__card {
    padding: 24px 16px;
    background: #fff;
    border-radius: 12px;
    box-shadow: 0 18px 46px rgba(25, 43, 76, 0.08);
  }

  // 24px between fields; the design keeps the upload row closer to the description
  // (16px) and 24px away from the next field.
  &__row {
    margin-bottom: 24px;
  }

  &__attach {
    margin: -8px 0 24px;
  }

  // Outlined button on the white card: solid #D9D9D9 border, no brand color.
  &__attach-btn {
    height: auto;
    padding: 8px 16px;
    background: #fff;
    border: 1px solid #d9d9d9;
    border-style: solid;
    border-radius: 6px;
    color: #595959;
    font-size: 16px;
    line-height: 24px;
  }

  &__attach-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-top: 10px;
  }

  &__attach-chip {
    display: flex;
    align-items: center;
    gap: 4px;
    max-width: 100%;
    padding: 6px 10px;
    background: #f7f8fa;
    border-radius: 6px;
    color: #1d2129;
    font-size: 13px;
    line-height: 18px;
  }

  &__attach-link {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    color: #1d2129;
    text-decoration: none;

    &:hover {
      color: #0d68ff;
    }
  }

  &__attach-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  &__attach-size {
    flex-shrink: 0;
    color: #86909c;
  }

  &__attach-delete {
    flex-shrink: 0;
    color: #86909c;
    cursor: pointer;

    &:hover {
      color: #fb363f;
    }
  }

  &__label {
    color: #262626;
    font-size: 14px;
    line-height: 22px;
  }

  // Submit bar: pinned to the viewport bottom, kept inside the 998px page column
  // (centered). Per the design it is a plain white bar — no radius, no shadow —
  // 72px tall overall: 16px padding + 40px button + 16px padding.
  &__submit-bar {
    position: fixed;
    left: 50%;
    bottom: 0;
    transform: translateX(-50%);
    z-index: 20;
    display: flex;
    gap: 16px;
    width: 100%;
    max-width: 966px;
    // 32px = the section gutter (16px) + the fields card padding (16px), so the
    // button lines up with the form fields above it.
    padding: 16px 32px calc(16px + env(safe-area-inset-bottom, 0px));
    background: #fff;
  }

  // antd injects its own required asterisk; only recolor it (never add a second one).
  // Color matches the project convention used by the member invite form.
  :deep(.ant-form-item-required)::before {
    color: #fb363f !important;
  }

  :deep(.ant-input),
  :deep(.ant-input-affix-wrapper),
  :deep(.ant-input-number),
  :deep(.ant-select),
  :deep(.ant-picker) {
    width: 100%;
  }

  // Design fills every control (input / textarea / select / picker) with a light blue
  // background instead of white, keeping the 1px #D9D9D9 border and 6px radius.
  :deep(.ant-input),
  :deep(.ant-input-affix-wrapper),
  :deep(.ant-input-number),
  :deep(.ant-picker),
  :deep(.ant-select:not(.ant-select-customize-input) .ant-select-selector) {
    background: #f5f9ff;
    border-color: #d9d9d9;
    border-radius: 6px;
  }

  // The select's placeholder is slightly darker than the plain input placeholder.
  :deep(.ant-select-selection-placeholder) {
    color: #8c8c8c;
  }

  // 40px tall (8px + 24px line + 8px) with a 6px radius, per the design's
  // 大小=大 filled button.
  &__submit {
    flex: 1;
    height: 40px;
    border-radius: 6px;
    font-size: 16px;
    line-height: 24px;
  }

  // 编辑态的取消：跟再次提交同宽，走 antd 默认描边按钮
  &__cancel {
    flex: 1;
    height: 40px;
    border-color: #d9d9d9;
    border-radius: 6px;
    font-size: 16px;
    line-height: 24px;
  }

  // The submit button uses the brand blue (matches the tab underline in the picker
  // design) instead of antd's default primary.
  :deep(.issue-form__submit.ant-btn-primary) {
    background-color: #2475fc;
    border-color: #2475fc;

    &:not([disabled]):hover,
    &:not([disabled]):focus {
      background-color: #4f8aff;
      border-color: #4f8aff;
    }
  }
}

@media (max-width: 959px) {
  // iOS zooms focused form controls when their text is smaller than 16px.
  .issue-form :deep(input),
  .issue-form :deep(textarea) {
    font-size: 16px;
  }
}
</style>

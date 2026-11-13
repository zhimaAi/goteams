<template>
  <a-modal
    :open="open"
    :title="$t('workitem_importTitle')"
    :width="486"
    :mask-closable="!importing && !parsing"
    :closable="!importing && !parsing"
    :destroy-on-close="true"
    wrap-class-name="workitem-import-modal"
    @cancel="close"
  >
    <div class="import-body">
      <div class="import-tip">
        <span>{{ $t("workitem_importTipPrefix") }}</span>
        <a class="template-link" @click="downloadTemplate">{{
          $t("workitem_excelTemplate")
        }}</a>
        <span>{{ tipSuffix || $t("workitem_importTipSuffix") }}</span>
      </div>

      <template v-if="!result">
        <a-upload-dragger
          :file-list="[]"
          :before-upload="beforeUpload"
          accept=".xlsx"
          :multiple="false"
          :show-upload-list="false"
          :disabled="importing || parsing"
        >
          <p class="upload-icon"><InboxOutlined /></p>
          <p class="upload-title">
            <span class="required">*</span>{{ $t("workitem_uploadTitle") }}
          </p>
          <p class="upload-hint">{{ $t("workitem_uploadLimit") }}</p>
          <p class="upload-hint">{{ $t("workitem_uploadType") }}</p>
        </a-upload-dragger>

        <div v-if="selectedFile" class="file-row">
          <PaperClipOutlined />
          <span class="file-name">{{ selectedFile.name }}</span>
          <span class="file-size">{{ formatSize(selectedFile.size) }}</span>
          <span class="importable-count">
            <LoadingOutlined v-if="parsing" />
            <template v-else>{{
              $t("workitem_importableCount", { n: preview?.importable || 0 })
            }}</template>
          </span>
          <span class="divider" />
          <a class="delete-file" @click="removeFile">{{
            $t("common_delete")
          }}</a>
        </div>
      </template>

      <div v-else class="import-result">
        <div class="result-grid">
          <div>
            <strong>{{ result.created }}</strong
            ><span>{{ $t("workitem_importCreated") }}</span>
          </div>
          <div>
            <strong>{{ result.updated }}</strong
            ><span>{{ $t("workitem_importUpdated") }}</span>
          </div>
          <div>
            <strong>{{ result.skipped }}</strong
            ><span>{{ $t("workitem_importSkipped") }}</span>
          </div>
          <div>
            <strong :class="{ danger: result.failed }">{{
              result.failed
            }}</strong
            ><span>{{ $t("workitem_importFailed") }}</span>
          </div>
        </div>
        <div v-if="result.errors?.length" class="error-list">
          <div v-for="item in result.errors" :key="`${item.row}-${item.id}`">
            {{
              $t("workitem_importRowError", {
                row: item.row,
                reason: item.reason,
              })
            }}
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <a-button :disabled="importing || parsing" @click="close">{{
        result ? $t("common_close") : $t("common_cancel")
      }}</a-button>
      <a-button
        v-if="!result"
        type="primary"
        :loading="importing"
        :disabled="!selectedFile || parsing || !preview?.importable"
        @click="submit"
      >
        {{ $t("common_confirm") }}
      </a-button>
    </template>
  </a-modal>
</template>

<script setup>
import { ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import {
  InboxOutlined,
  LoadingOutlined,
  PaperClipOutlined,
} from "@ant-design/icons-vue";
import {
  downloadWorkItemImportTemplate,
  importWorkItems,
  previewWorkItemImport,
} from "@/api/workspace";

const props = defineProps({
  open: { type: Boolean, default: false },
  wsId: { type: [String, Number], required: true },
  typeKey: { type: String, required: true },
  // Planning reuses this dialog with its own endpoints; when omitted the work-item APIs are used.
  templateApi: { type: Function, default: null },
  previewApi: { type: Function, default: null },
  importApi: { type: Function, default: null },
  // Optional override for the tip text after the template link (e.g. member import matches by account, not ID).
  tipSuffix: { type: String, default: "" },
});
const emit = defineEmits(["update:open", "imported"]);
const { t } = useI18n();
const selectedFile = ref(null);
const importing = ref(false);
const parsing = ref(false);
const preview = ref(null);
const result = ref(null);

watch(
  () => props.open,
  (value) => {
    if (value) {
      selectedFile.value = null;
      parsing.value = false;
      preview.value = null;
      result.value = null;
    }
  },
);

function timezone() {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
}

function beforeUpload(file) {
  if (!file.name.toLowerCase().endsWith(".xlsx")) {
    message.error(t("workitem_uploadInvalidType"));
    return false;
  }
  if (file.size > 100 * 1024 * 1024) {
    message.error(t("workitem_uploadTooLarge"));
    return false;
  }
  selectedFile.value = file;
  preview.value = null;
  result.value = null;
  parseSelectedFile(file);
  return false;
}

function removeFile() {
  if (!importing.value && !parsing.value) {
    selectedFile.value = null;
    preview.value = null;
  }
}

async function parseSelectedFile(file) {
  parsing.value = true;
  try {
    const parsed = props.previewApi
      ? await props.previewApi(file)
      : await previewWorkItemImport(
          props.wsId,
          props.typeKey,
          file,
          timezone(),
        );
    if (selectedFile.value === file) preview.value = parsed;
  } catch {
    if (selectedFile.value === file) {
      selectedFile.value = null;
      preview.value = null;
    }
  } finally {
    if (selectedFile.value === file || !selectedFile.value) {
      parsing.value = false;
    }
  }
}

function close() {
  if (!importing.value && !parsing.value) emit("update:open", false);
}

function formatSize(size) {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
}

function saveResponseFile(response, fallbackName) {
  const disposition = response.headers?.["content-disposition"] || "";
  const match = disposition.match(/filename\*=UTF-8''([^;]+)/i);
  const filename = match ? decodeURIComponent(match[1]) : fallbackName;
  const url = URL.createObjectURL(response.data);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
}

async function downloadTemplate() {
  try {
    const response = props.templateApi
      ? await props.templateApi()
      : await downloadWorkItemImportTemplate(props.wsId, {
          type: props.typeKey,
          timezone: timezone(),
        });
    saveResponseFile(response, `${t("workitem_excelTemplate")}.xlsx`);
  } catch {
    // The unified interceptor has already shown a prompt.
  }
}

async function submit() {
  if (
    !selectedFile.value ||
    importing.value ||
    parsing.value ||
    !preview.value?.importable
  )
    return;
  importing.value = true;
  try {
    result.value = props.importApi
      ? await props.importApi(selectedFile.value)
      : await importWorkItems(
          props.wsId,
          props.typeKey,
          selectedFile.value,
          timezone(),
        );
    message.success(t("workitem_importCompleted"));
    emit("imported", result.value);
  } catch {
    // The unified interceptor has already shown a prompt.
  } finally {
    importing.value = false;
  }
}
</script>

<style scoped lang="less">
.import-body {
  padding: 0 8px 12px;
}
.import-tip {
  padding: 9px 16px;
  margin-bottom: 24px;
  color: #3a4559;
  font-size: 14px;
  line-height: 22px;
  background: #e9f1fe;
  border: 1px solid #99bffd;
  border-radius: 6px;
}
.template-link {
  margin: 0 3px;
  color: #2475fc;
}
:deep(.ant-upload-wrapper .ant-upload-drag) {
  padding: 16px;
  background: #fafafa;
  border: 1px solid #d9d9d9;
  border-radius: 6px;
}
:deep(.ant-upload-wrapper .ant-upload-drag .ant-upload) {
  padding: 0;
}
.upload-icon {
  margin: 0 0 20px;
  color: #2475fc;
  font-size: 48px;
  line-height: 48px;
}
.upload-title {
  margin: 0 0 4px;
  color: rgba(0, 0, 0, 0.85);
  font-size: 14px;
  line-height: 22px;
}
.required {
  color: #fb363f;
}
.upload-hint {
  margin: 0;
  color: rgba(0, 0, 0, 0.45);
  font-size: 12px;
  line-height: 20px;
}
.file-row {
  display: flex;
  gap: 8px;
  align-items: center;
  min-height: 32px;
  padding: 4px 8px;
  margin-top: 8px;
  color: #595959;
  border: 1px solid #f0f0f0;
  border-radius: 6px;
}
.file-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.file-size {
  color: #8c8c8c;
  font-size: 12px;
}
.importable-count {
  min-width: 42px;
  color: #8c8c8c;
  font-size: 14px;
  text-align: right;
}
.divider {
  width: 1px;
  height: 12px;
  background: #d9d9d9;
}
.delete-file {
  color: #2475fc;
}
.result-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
}
.result-grid > div {
  padding: 14px 6px;
  text-align: center;
  background: #fafafa;
  border-radius: 6px;
}
.result-grid strong {
  display: block;
  color: #2475fc;
  font-size: 22px;
}
.result-grid strong.danger {
  color: #fb363f;
}
.result-grid span {
  color: #8c8c8c;
  font-size: 12px;
}
.error-list {
  max-height: 180px;
  padding: 10px 12px;
  margin-top: 12px;
  overflow-y: auto;
  color: #fb363f;
  background: #fff2f0;
  border-radius: 6px;
}
</style>

<style lang="less">
.workitem-import-modal .ant-modal-content {
  padding: 0;
  overflow: hidden;
  border-radius: 12px;
}
.workitem-import-modal .ant-modal-header {
  padding: 16px 24px;
  margin: 0;
}
.workitem-import-modal .ant-modal-body {
  padding: 12px 24px;
}
.workitem-import-modal .ant-modal-footer {
  padding: 10px 24px;
  margin: 0;
  border-top: 0;
}
</style>

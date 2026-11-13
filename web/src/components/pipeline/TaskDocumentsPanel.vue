<template>
  <div class="task-documents-panel" :class="`mode-${mode}`">
    <header class="documents-header">
      <div class="documents-heading">
        <a-avatar :size="32" class="documents-avatar">
          {{ (taskTitle || "A").trim().charAt(0) }}
        </a-avatar>
        <div class="documents-heading-text">
          <p class="documents-task-title">{{ taskTitle || "-" }}</p>
          <p class="documents-task-subtitle">
            {{ $t("pipeline_documents_title") }}
          </p>
        </div>
      </div>
      <div class="documents-heading-actions">
        <a-button type="text" :loading="listLoading" @click="loadDocuments()">
          {{ $t("common_refresh") }}
        </a-button>
        <!-- 关闭改成「任务详情」：文档页自己能把所属工作项以抽屉打开，不必退回列表重找 -->
        <a-button
          type="text"
          :disabled="!taskMeta.workItemId"
          @click="$emit('open-detail', taskMeta)"
        >
          {{ $t("pipeline_task_detail") }}
        </a-button>
      </div>
    </header>

    <div class="documents-body">
      <aside v-show="treeVisible" class="documents-tree">
        <div v-if="!rows.length && !listLoading" class="documents-tree-empty">
          {{ $t("pipeline_documents_empty") }}
        </div>
        <button
          v-for="row in rows"
          :key="row.key"
          type="button"
          class="tree-row"
          :class="{
            'tree-row-folder': row.type === 'folder',
            'tree-row-active':
              row.type === 'doc' && row.document.document_id === activeId,
          }"
          :style="{ paddingLeft: `${12 + row.depth * 14}px` }"
          @click="onRowClick(row)"
        >
          <span class="tree-row-icon">
            <template v-if="row.type === 'folder'">
              <FolderOpenOutlined v-if="!collapsedPaths.has(row.path)" />
              <FolderOutlined v-else />
            </template>
            <component v-else :is="fileIcon(row.document)" />
          </span>
          <span class="tree-row-name">{{ row.name }}</span>
          <span v-if="row.type === 'folder'" class="tree-row-count">{{
            row.count
          }}</span>
          <span
            v-else-if="row.document.storage_state === 'pending'"
            class="tree-row-state"
            >{{ $t("pipeline_documents_uploading_short") }}</span
          >
          <span
            v-else-if="row.document.storage_state === 'failed'"
            class="tree-row-state failed"
            >!</span
          >
        </button>
      </aside>

      <section class="documents-content">
        <!-- 目录开关跟着文件标题走：收起后标题行仍在，随时能展开回来 -->
        <div v-if="documents.length" class="document-toolbar">
          <a-tooltip
            :title="
              treeVisible
                ? $t('pipeline_documents_collapse_tree')
                : $t('pipeline_documents_expand_tree')
            "
          >
            <button
              type="button"
              class="document-toolbar-toggle"
              :aria-expanded="treeVisible"
              :aria-label="
                treeVisible
                  ? $t('pipeline_documents_collapse_tree')
                  : $t('pipeline_documents_expand_tree')
              "
              @click="toggleTree"
            >
              <svg
                class="document-toolbar-toggle-icon"
                :class="{ 'is-collapsed': !treeVisible }"
                width="16"
                height="16"
                viewBox="0 0 20 20"
                fill="none"
                aria-hidden="true"
              >
                <rect
                  x="3.334"
                  y="4.166"
                  width="13.332"
                  height="11.668"
                  rx="2.083"
                  stroke="currentColor"
                  stroke-width="1.458"
                />
                <path
                  d="M8.333 4.166V15.834"
                  stroke="currentColor"
                  stroke-width="1.458"
                  stroke-linecap="round"
                />
                <path
                  d="M12.5 7.5L10 10L12.5 12.5"
                  stroke="currentColor"
                  stroke-width="1.458"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
            </button>
          </a-tooltip>
          <template v-if="activeDocument">
            <component
              :is="fileIcon(activeDocument)"
              class="document-toolbar-icon"
            />
            <span class="document-toolbar-title">{{
              activeDocument.title
            }}</span>
            <span class="document-toolbar-meta">{{
              formatSize(activeDocument.file_size)
            }}</span>
            <span v-if="uploadedAtText" class="document-toolbar-meta">{{
              uploadedAtText
            }}</span>
            <span v-if="uploaderText" class="document-toolbar-meta">{{
              uploaderText
            }}</span>
            <a-button
              v-if="activeDocument.storage_state === 'ready'"
              type="text"
              class="document-toolbar-download"
              :loading="downloadLoading"
              @click="download(activeDocument)"
            >
              <DownloadOutlined />
              {{ $t("pipeline_documents_download") }}
            </a-button>
          </template>
        </div>

        <a-spin v-if="listLoading || documentLoading" class="documents-spin" />
        <a-empty
          v-else-if="!documents.length"
          :image="simpleImage"
          :description="$t('pipeline_documents_empty')"
        />
        <a-empty
          v-else-if="!activeDocument"
          :image="simpleImage"
          :description="$t('pipeline_documents_select')"
        />
        <template v-else>
          <a-alert
            v-if="activeDocument.storage_state === 'pending'"
            class="document-alert"
            type="info"
            show-icon
            :message="$t('pipeline_documents_uploading')"
          />
          <a-alert
            v-else-if="activeDocument.storage_state === 'failed'"
            class="document-alert"
            type="error"
            show-icon
            :message="$t('pipeline_documents_storage_failed')"
            :description="
              activeDocument.storage_error ||
              $t('pipeline_documents_retry_hint')
            "
          />
          <a-alert
            v-else-if="documentError"
            class="document-alert"
            type="error"
            show-icon
            :message="$t('pipeline_documents_open_failed')"
          />

          <div
            v-if="activeDocument.storage_state === 'ready'"
            class="document-view"
          >
            <div
              v-if="isMarkdown"
              class="document-markdown message-markdown"
              v-html="renderedMarkdown"
              @click="onMarkdownClick"
              @keydown.enter="onMarkdownClick"
            ></div>
            <pre v-else-if="isText" class="document-text">{{
              activeDocument.content || ""
            }}</pre>
            <img
              v-else-if="isImage && fileLink"
              class="document-image"
              :src="fileLink"
              :alt="activeDocument.title"
            />
            <iframe
              v-else-if="isPdf && fileLink"
              class="document-pdf"
              :src="fileLink"
              :title="activeDocument.title"
            ></iframe>
            <div v-else class="document-download-hint">
              <FileOutlined class="document-download-icon" />
              <p>{{ $t("pipeline_documents_file_hint") }}</p>
              <a-button type="primary" @click="download(activeDocument)">
                {{ $t("pipeline_documents_download") }}
              </a-button>
            </div>
          </div>

          <p v-if="activeDocument.source_abs_path" class="document-source-path">
            {{ activeDocument.source_abs_path }}
          </p>
        </template>
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { Empty, message } from "ant-design-vue";
import {
  DownloadOutlined,
  FileImageOutlined,
  FileMarkdownOutlined,
  FileOutlined,
  FilePdfOutlined,
  FileTextOutlined,
  FolderOpenOutlined,
  FolderOutlined,
} from "@ant-design/icons-vue";
import {
  getPipelineDocument,
  getPipelineDocumentLink,
  listPipelineTaskDocuments,
} from "@/api/pipelineDocuments";
import {
  renderConversationMarkdown,
  documentIdFromEvent,
} from "@/components/requirement/conversationMarkdown";
import { useAuthStore } from "@/stores/auth";
import { formatLocalDate } from "@/utils/date";

const props = defineProps({
  taskId: { type: [Number, String], required: true },
  documentId: { type: [Number, String], default: null },
  mode: { type: String, default: "page" },
});

const emit = defineEmits(["open-detail", "update:documentId"]);

const { t } = useI18n();
const authStore = useAuthStore();
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;

const listLoading = ref(false);
const documentLoading = ref(false);
const downloadLoading = ref(false);
const documents = ref([]);
const taskTitle = ref("");
// 任务所属工作项，供「任务详情」按钮打开详情抽屉。
const taskMeta = ref({ workspaceId: 0, workItemId: 0, workItemType: "" });
const activeId = ref(null);
const activeDocument = ref(null);
const documentError = ref(false);
const fileLink = ref("");
const treeVisible = ref(true);
const collapsedPaths = ref(new Set());

const MEMORY_PREFIX = "goteams:pipeline-docs";
const IMAGE_EXTS = ["png", "jpg", "jpeg", "gif", "webp", "bmp"];
// 文档上传是异步的：还在 pending 时静默轮询几次，没人愿意为了看到正文手动点刷新。
const POLL_INTERVAL_MS = 5000;
const POLL_MAX_ROUNDS = 24;

function memoryKey() {
  const userId = authStore.userInfo?.id;
  if (!userId) return "";
  return `${MEMORY_PREFIX}:${userId}:${props.taskId}`;
}

function rememberedId() {
  const key = memoryKey();
  if (!key || typeof window === "undefined") return null;
  try {
    const raw = window.localStorage.getItem(key);
    return raw ? Number(JSON.parse(raw)?.documentId) || null : null;
  } catch {
    return null;
  }
}

function remember(documentId) {
  const key = memoryKey();
  if (!key || typeof window === "undefined") return;
  try {
    window.localStorage.setItem(
      key,
      JSON.stringify({ documentId, updatedAt: Date.now() }),
    );
  } catch {
    // Storage pressure must not break browsing.
  }
}

// Folder counts include every descendant document so the tree can show "26" on a branch.
function buildRows(list, collapsed) {
  const root = { name: "", path: "", children: [], docs: [] };
  list.forEach((document) => {
    const segments = String(document.rel_path || "")
      .split("/")
      .filter(Boolean);
    if (!segments.length) return;
    let node = root;
    segments.slice(0, -1).forEach((segment, index) => {
      const path = segments.slice(0, index + 1).join("/");
      let child = node.children.find((item) => item.name === segment);
      if (!child) {
        child = { name: segment, path, children: [], docs: [] };
        node.children.push(child);
      }
      node = child;
    });
    node.docs.push(document);
  });
  const rows = [];
  const countOf = (node) =>
    node.docs.length +
    node.children.reduce((total, child) => total + countOf(child), 0);
  const walk = (node, depth) => {
    node.children.forEach((child) => {
      rows.push({
        key: `folder:${child.path}`,
        type: "folder",
        name: child.name,
        path: child.path,
        depth,
        count: countOf(child),
      });
      if (!collapsed.has(child.path)) walk(child, depth + 1);
    });
    node.docs.forEach((document) => {
      rows.push({
        key: `doc:${document.document_id}`,
        type: "doc",
        name: baseName(document.rel_path),
        path: document.rel_path,
        depth,
        document,
      });
    });
  };
  walk(root, 0);
  return rows;
}

const rows = computed(() => buildRows(documents.value, collapsedPaths.value));

// 还有文档在等上传时，用静默轮询把 pending 推平；超过上限就交给手动刷新。
const hasPendingDocuments = computed(() =>
  documents.value.some((item) => item.storage_state === "pending"),
);

let pollTimer = null;
let pollRounds = 0;

function stopWaitingRefresh() {
  if (pollTimer) {
    clearTimeout(pollTimer);
    pollTimer = null;
  }
  pollRounds = 0;
}

function scheduleWaitingRefresh() {
  if (pollTimer || typeof window === "undefined") return;
  if (pollRounds >= POLL_MAX_ROUNDS) return;
  pollTimer = window.setTimeout(async () => {
    pollTimer = null;
    pollRounds += 1;
    if (!hasPendingDocuments.value) {
      pollRounds = 0;
      return;
    }
    await loadDocuments(true, true);
    if (hasPendingDocuments.value) scheduleWaitingRefresh();
    else pollRounds = 0;
  }, POLL_INTERVAL_MS);
}

watch(hasPendingDocuments, (pending) => {
  if (pending) {
    scheduleWaitingRefresh();
    return;
  }
  stopWaitingRefresh();
});

const isMarkdown = computed(
  () => activeDocument.value?.content_mode === "markdown",
);
const isText = computed(() => activeDocument.value?.content_mode === "text");
const isImage = computed(() =>
  IMAGE_EXTS.includes(
    String(activeDocument.value?.file_ext || "").toLowerCase(),
  ),
);
const isPdf = computed(
  () => String(activeDocument.value?.file_ext || "").toLowerCase() === "pdf",
);

const renderedMarkdown = computed(() =>
  renderConversationMarkdown(
    activeDocument.value?.content || "",
    documents.value,
  ),
);

// 上报时间用服务端入库的 updated_at：文档再次上报会刷新它，线上看到的就是最近一次上报时间。
const uploadedAtText = computed(() => {
  const seconds = Number(activeDocument.value?.updated_at);
  if (!Number.isFinite(seconds) || seconds <= 0) return "";
  const time = formatLocalDate(seconds, true).slice(0, 16);
  return t("pipeline_documents_uploaded_at", { time });
});

const uploaderText = computed(() => {
  const name = activeDocument.value?.uploader_name;
  return name ? t("pipeline_documents_uploader", { name }) : "";
});

function baseName(path) {
  const segments = String(path || "").split("/");
  return segments[segments.length - 1] || "";
}

function fileIcon(document) {
  const ext = String(document?.file_ext || "").toLowerCase();
  if (IMAGE_EXTS.includes(ext)) return FileImageOutlined;
  if (ext === "pdf") return FilePdfOutlined;
  if (["md", "markdown", "mdx"].includes(ext)) return FileMarkdownOutlined;
  if (["txt", "log", "json", "yaml", "yml", "csv", "sql"].includes(ext)) {
    return FileTextOutlined;
  }
  return FileOutlined;
}

function formatSize(size) {
  const bytes = Number(size) || 0;
  if (bytes <= 0) return "-";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

function toggleTree() {
  treeVisible.value = !treeVisible.value;
}

function onRowClick(row) {
  if (row.type === "folder") {
    const next = new Set(collapsedPaths.value);
    if (next.has(row.path)) next.delete(row.path);
    else next.add(row.path);
    collapsedPaths.value = next;
    return;
  }
  selectDocument(row.document.document_id);
}

function pickInitialId(list) {
  const candidates = [props.documentId, rememberedId()];
  for (const candidate of candidates) {
    const id = Number(candidate);
    if (id && list.some((item) => item.document_id === id)) return id;
  }
  const firstMarkdown = list.find(
    (item) =>
      item.storage_state === "ready" && item.content_mode === "markdown",
  );
  return (firstMarkdown || list[0])?.document_id || null;
}

async function loadDocuments(preserveSelection = true, silent = false) {
  if (!silent) listLoading.value = true;
  try {
    const result = await listPipelineTaskDocuments(props.taskId);
    const list = Array.isArray(result?.documents) ? result.documents : [];
    taskTitle.value = result?.task_title || "";
    taskMeta.value = {
      workspaceId: Number(result?.workspace_id) || 0,
      workItemId: Number(result?.work_item_id) || 0,
      workItemType: result?.work_item_type || "",
    };
    documents.value = list;
    // Keep the current document while it still exists; a deleted one falls back to
    // the remembered/URL document, then to the first readable markdown file.
    const stillExists =
      preserveSelection &&
      activeId.value &&
      list.some((item) => item.document_id === Number(activeId.value));
    const nextId = stillExists ? activeId.value : pickInitialId(list);
    if (nextId && list.some((item) => item.document_id === Number(nextId))) {
      await selectDocument(nextId, silent);
    } else {
      activeId.value = null;
      activeDocument.value = null;
    }
  } catch {
    documents.value = [];
    taskTitle.value = "";
    taskMeta.value = { workspaceId: 0, workItemId: 0, workItemType: "" };
  } finally {
    if (!silent) listLoading.value = false;
  }
}

let requestToken = 0;

async function selectDocument(documentId, silent = false) {
  const id = Number(documentId);
  if (!id) return;
  activeId.value = id;
  emit("update:documentId", id);
  remember(id);
  documentError.value = false;
  fileLink.value = "";
  const token = ++requestToken;
  if (!silent) documentLoading.value = true;
  try {
    const detail = await getPipelineDocument(id);
    if (token !== requestToken) return;
    activeDocument.value = detail;
    if (detail?.storage_state === "ready" && (isImage.value || isPdf.value)) {
      const link = await getPipelineDocumentLink(id);
      if (token === requestToken) fileLink.value = link?.link || "";
    }
  } catch {
    if (token === requestToken) {
      activeDocument.value = null;
      documentError.value = true;
    }
  } finally {
    if (token === requestToken && !silent) documentLoading.value = false;
  }
}

async function download(document) {
  if (!document?.document_id) return;
  downloadLoading.value = true;
  try {
    const link = await getPipelineDocumentLink(
      document.document_id,
      "attachment",
    );
    if (link?.link) window.open(link.link, "_blank", "noopener");
  } catch {
    message.error(t("pipeline_documents_open_failed"));
  } finally {
    downloadLoading.value = false;
  }
}

// Documents referenced from the body open in place: same task, same list, so the
// reader stays in the file they were reading.
function onMarkdownClick(event) {
  const documentId = documentIdFromEvent(event);
  if (!documentId) return;
  event.preventDefault();
  selectDocument(documentId);
}

watch(
  () => props.taskId,
  () => {
    activeId.value = null;
    activeDocument.value = null;
    loadDocuments(false);
  },
  { immediate: true },
);

onBeforeUnmount(() => {
  requestToken += 1;
  stopWaitingRefresh();
});

defineExpose({ refresh: () => loadDocuments() });
</script>

<style scoped lang="less">
@import "@/styles/variables.less";

.task-documents-panel {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
  height: 100%;
  min-height: 0;
  background: @neutral-white-10;
}

.documents-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: @spacing-16;
  padding: @spacing-16 @spacing-24;
  border-bottom: 1px solid @neutral-white-7;
}

.documents-heading {
  display: flex;
  align-items: center;
  gap: @spacing-12;
  min-width: 0;
}

.documents-avatar {
  background: @primary-bg-light;
  color: @primary-color;
  flex: none;
}

.documents-heading-text {
  min-width: 0;
}

.documents-task-title {
  margin: 0;
  font-size: @font-size-3;
  font-weight: @font-weight-medium;
  color: @neutral-white-2;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.documents-task-subtitle {
  margin: 0;
  font-size: @font-size-5;
  color: @neutral-blue-3;
}

.documents-heading-actions {
  display: flex;
  align-items: center;
  gap: @spacing-4;
  flex: none;
}

.documents-body {
  display: flex;
  flex: 1;
  min-height: 0;
}

.documents-tree {
  width: 248px;
  flex: none;
  padding: @spacing-8 0;
  border-right: 1px solid @neutral-white-7;
  overflow-y: auto;
}

.documents-tree-empty {
  padding: @spacing-16;
  font-size: @font-size-5;
  color: @neutral-blue-3;
}

.tree-row {
  display: flex;
  align-items: center;
  gap: @spacing-6;
  width: 100%;
  padding: @spacing-6 @spacing-12;
  border: none;
  background: transparent;
  font-size: @font-size-4;
  color: @neutral-white-2;
  text-align: left;
  cursor: pointer;
}

.tree-row:hover {
  background: @primary-selected;
}

.tree-row-active,
.tree-row-active:hover {
  background: @primary-bg-light;
  color: @primary-color;
}

.tree-row-icon {
  display: inline-flex;
  color: @neutral-blue-3;
  flex: none;
}

.tree-row-active .tree-row-icon {
  color: @primary-color;
}

.tree-row-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tree-row-count {
  flex: none;
  font-size: @font-size-5;
  color: @neutral-blue-3;
}

.tree-row-state {
  flex: none;
  font-size: @font-size-6;
  color: @warning-color;
}

.tree-row-state.failed {
  color: @danger-color;
}

.documents-content {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
  padding: 0 @spacing-24 @spacing-24;
  overflow-y: auto;
}

.documents-spin {
  margin: @spacing-48 auto;
}

.document-toolbar {
  display: flex;
  align-items: center;
  gap: @spacing-8;
  padding: @spacing-16 0 @spacing-12;
  border-bottom: 1px solid @neutral-white-7;
}

.document-toolbar-icon {
  color: @primary-color;
}

/* 目录开关：与客户端一致，16px 图标贴在文件标题最前面，折叠后再点一次即可展开 */
.document-toolbar-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  flex: none;
  padding: 0;
  border: none;
  border-radius: @border-radius-sm;
  background: transparent;
  color: @neutral-white-3;
  cursor: pointer;
}

.document-toolbar-toggle:hover,
.document-toolbar-toggle:focus-visible {
  background: @primary-selected;
  color: @primary-color;
}

.document-toolbar-toggle:focus-visible {
  outline: 2px solid @primary-color;
  outline-offset: 2px;
}

.document-toolbar-toggle-icon {
  display: block;
  width: 16px;
  height: 16px;
}

.document-toolbar-toggle-icon.is-collapsed {
  transform: rotate(180deg);
}

.document-toolbar-title {
  font-size: @font-size-3;
  font-weight: @font-weight-medium;
  color: @neutral-white-2;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.document-toolbar-meta {
  font-size: @font-size-5;
  color: @neutral-blue-3;
}

.document-toolbar-download {
  margin-left: auto;
}

.document-alert {
  margin-top: @spacing-16;
}

.document-view {
  padding-top: @spacing-16;
}

.document-markdown {
  font-size: @font-size-4;
  line-height: @line-height-4;
  color: @neutral-white-2;
}

.document-text {
  margin: 0;
  padding: @spacing-12;
  border-radius: @border-radius-sm;
  background: @neutral-blue-5;
  font-family: @font-family-mono;
  font-size: @font-size-5;
  color: @neutral-white-2;
  white-space: pre-wrap;
  word-break: break-all;
}

.document-image {
  max-width: 100%;
  border-radius: @border-radius-sm;
}

.document-pdf {
  width: 100%;
  height: 70vh;
  border: 1px solid @neutral-white-7;
  border-radius: @border-radius-sm;
}

.document-download-hint {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: @spacing-12;
  padding: @spacing-48 0;
  color: @neutral-blue-3;
}

.document-download-icon {
  font-size: 32px;
}

.document-source-path {
  margin: @spacing-16 0 0;
  font-size: @font-size-6;
  color: @neutral-blue-3;
  word-break: break-all;
}
</style>

<style lang="less">
@import "@/styles/variables.less";

// v-html content is outside the scoped attribute, so markdown body rules live here
// namespaced under .document-markdown to keep them from leaking to other pages.
.document-markdown {
  white-space: normal;
  overflow-wrap: anywhere;
  color: @neutral-white-2;
  font-size: @font-size-4;
  line-height: @line-height-4;

  h1,
  h2,
  h3,
  h4,
  h5,
  h6 {
    margin: 0.9em 0 0.4em;
    color: @neutral-blue-8;
    font-weight: @font-weight-semibold;
    line-height: 1.35;
  }

  h1 {
    font-size: 1.35em;
  }

  h2 {
    font-size: 1.2em;
  }

  h3 {
    font-size: 1.08em;
  }

  h4,
  h5,
  h6 {
    font-size: 1em;
  }

  p,
  ul,
  ol,
  blockquote,
  pre,
  table {
    margin: 0.7em 0;
  }

  ul,
  ol {
    padding-left: 1.4em;
  }

  ul {
    list-style: disc;
  }

  ol {
    list-style: decimal;
  }

  a {
    color: @primary-color;
    text-decoration: none;
  }

  a:hover {
    text-decoration: underline;
  }

  code {
    padding: 0.15em 0.35em;
    border-radius: @border-radius-xs;
    background: @neutral-blue-5;
    font-family: @font-family-mono;
    font-size: 0.92em;
  }

  pre {
    padding: @spacing-12;
    border-radius: @border-radius-sm;
    background: @neutral-blue-5;
    overflow-x: auto;
  }

  pre code {
    padding: 0;
    background: transparent;
  }

  blockquote {
    padding-left: @spacing-12;
    border-left: 3px solid @info-border;
    color: @neutral-blue-1;
  }

  table {
    width: 100%;
    border-collapse: collapse;
  }

  th,
  td {
    padding: @spacing-6 @spacing-10;
    border: 1px solid @neutral-blue-4;
    text-align: left;
  }

  img {
    max-width: 100%;
  }
}
</style>

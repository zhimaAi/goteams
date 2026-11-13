<template>
  <div
    class="markdown-editor-body"
    :class="{
      'preview-hidden': !previewVisible,
      'is-markdown-fullscreen': fullscreen,
      'is-markdown-fill': fill,
    }"
  >
    <section class="markdown-pane markdown-source-pane">
      <div class="markdown-pane-title">{{ $t("editor_source") }}</div>
      <div ref="sourceShell" class="markdown-source-shell">
        <div class="markdown-line-numbers" aria-hidden="true">
          <div :style="lineNumberStyle">
            <span v-for="line in lineCount" :key="line">{{ line }}</span>
          </div>
        </div>
        <textarea
          ref="input"
          :value="source"
          class="markdown-source"
          :aria-label="$t('editor_markdownSource')"
          :placeholder="placeholder"
          spellcheck="true"
          @input="handleInput"
          @keydown="handleKeydown"
          @keyup="rememberSelection"
          @click="rememberSelection"
          @select="rememberSelection"
          @focus="handleFocus"
          @blur="sourceFocused = false"
          @scroll="handleScroll"
          @paste="handlePaste"
          @dragover.prevent
          @drop="handleDrop"
        />
        <div
          v-if="showInlineTools"
          class="markdown-inline-tools"
          :style="inlineToolStyle"
          @mousedown.prevent
        >
          <button
            type="button"
            :title="$t('editor_heading1')"
            @mousedown.prevent="executeCommand('heading1')"
          >
            H1
          </button>
          <button
            type="button"
            :title="$t('editor_heading2')"
            @mousedown.prevent="executeCommand('heading2')"
          >
            H2
          </button>
          <button
            type="button"
            :title="$t('editor_heading3')"
            @mousedown.prevent="executeCommand('heading3')"
          >
            H3
          </button>
          <button
            type="button"
            :title="$t('editor_bulletList')"
            @mousedown.prevent="executeCommand('bullet')"
          >
            <UnorderedListOutlined />
          </button>
          <button
            type="button"
            :title="$t('editor_quote')"
            @mousedown.prevent="executeCommand('quote')"
          >
            <MessageOutlined />
          </button>
          <button
            type="button"
            :title="$t('editor_table')"
            @mousedown.prevent="executeCommand('table')"
          >
            <TableOutlined />
          </button>
          <button
            type="button"
            :title="$t('editor_link')"
            @mousedown.prevent="executeCommand('link')"
          >
            <LinkOutlined />
          </button>
        </div>
      </div>
    </section>
    <section v-if="previewVisible" class="markdown-pane markdown-preview-pane">
      <div class="markdown-pane-title">{{ $t("editor_preview") }}</div>
      <div
        v-if="previewHtml"
        class="rich-text-content markdown-preview-content"
        v-html="previewHtml"
      />
      <div v-else class="markdown-empty-preview">{{ $t("common_noData") }}</div>
    </section>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import {
  LinkOutlined,
  MessageOutlined,
  TableOutlined,
  UnorderedListOutlined,
} from "@ant-design/icons-vue";
import { htmlToMarkdown, markdownToHtml } from "./markdownContent";

const props = defineProps({
  modelValue: { type: String, default: "" },
  placeholder: { type: String, default: "" },
  previewVisible: { type: Boolean, required: true },
  fullscreen: { type: Boolean, default: false },
  fill: { type: Boolean, default: false },
});
const emit = defineEmits(["update:modelValue", "files"]);
const { t: $t } = useI18n();
const source = ref("");
const input = ref(null);
const sourceShell = ref(null);
const selectionStart = ref(0);
const selectionEnd = ref(0);
const scrollTop = ref(0);
const sourceFocused = ref(false);
let lastRenderedHtml = "";

const previewHtml = computed(() => markdownToHtml(source.value));
const lineCount = computed(() => source.value.split("\n").length);
const cursorContext = computed(() => {
  const cursor = Math.min(selectionStart.value, source.value.length);
  const beforeCursor = source.value.slice(0, cursor);
  const lineIndex = beforeCursor.split("\n").length - 1;
  const lineStart = beforeCursor.lastIndexOf("\n") + 1;
  const lineEndIndex = source.value.indexOf("\n", cursor);
  const lineEnd = lineEndIndex < 0 ? source.value.length : lineEndIndex;
  return {
    lineIndex,
    column: cursor - lineStart,
    text: source.value.slice(lineStart, lineEnd),
  };
});
const cursorLineTop = computed(
  () => 14 + cursorContext.value.lineIndex * 23.8 - scrollTop.value,
);
const showInlineTools = computed(
  () =>
    sourceFocused.value &&
    selectionStart.value === selectionEnd.value &&
    cursorContext.value.text.trim() === "" &&
    cursorLineTop.value > -23.8 &&
    cursorLineTop.value < (sourceShell.value?.clientHeight || 180),
);
const lineNumberStyle = computed(() => ({
  transform: `translateY(${-scrollTop.value}px)`,
}));
const inlineToolStyle = computed(() => {
  const shellWidth = sourceShell.value?.clientWidth || 460;
  const shellHeight = sourceShell.value?.clientHeight || 180;
  const belowLine = cursorLineTop.value + 23.8;
  const top =
    belowLine + 34 <= shellHeight
      ? belowLine
      : Math.max(6, cursorLineTop.value - 34);
  const left = Math.min(
    52 + cursorContext.value.column * 8.4,
    Math.max(52, shellWidth - 246),
  );
  return { top: `${top}px`, left: `${left}px` };
});

function resetFromHtml(value) {
  source.value = htmlToMarkdown(value);
  lastRenderedHtml = markdownToHtml(source.value);
}

function getHtml() {
  return previewHtml.value;
}

function handleInput(event) {
  if (event) source.value = event.target.value;
  lastRenderedHtml = previewHtml.value;
  emit("update:modelValue", lastRenderedHtml);
  rememberSelection();
}

function rememberSelection() {
  if (!input.value) return;
  selectionStart.value = input.value.selectionStart ?? 0;
  selectionEnd.value = input.value.selectionEnd ?? selectionStart.value;
}

function handleFocus() {
  sourceFocused.value = true;
  rememberSelection();
}

function handleScroll(event) {
  scrollTop.value = event.target.scrollTop;
}

function restoreSelection(start, end = start) {
  nextTick(() => {
    if (!input.value) return;
    input.value.focus();
    input.value.setSelectionRange(start, end);
    rememberSelection();
  });
}

function replaceRange(start, end, replacement, nextStart, nextEnd) {
  source.value =
    source.value.slice(0, start) + replacement + source.value.slice(end);
  handleInput();
  restoreSelection(nextStart, nextEnd);
}

function currentSelection() {
  rememberSelection();
  return { start: selectionStart.value, end: selectionEnd.value };
}

function applyInline(prefix, suffix, placeholder) {
  const { start, end } = currentSelection();
  const selected = source.value.slice(start, end);
  const content = selected || placeholder;
  const contentStart = start + prefix.length;
  replaceRange(
    start,
    end,
    `${prefix}${content}${suffix}`,
    contentStart,
    contentStart + content.length,
  );
}

function selectedLines() {
  const { start, end } = currentSelection();
  const lineStart = source.value.lastIndexOf("\n", Math.max(0, start - 1)) + 1;
  const normalizedEnd =
    end > start && source.value[end - 1] === "\n" ? end - 1 : end;
  const lineEndIndex = source.value.indexOf("\n", normalizedEnd);
  const lineEnd = lineEndIndex < 0 ? source.value.length : lineEndIndex;
  return {
    start,
    end,
    lineStart,
    lineEnd,
    lines: source.value.slice(lineStart, lineEnd).split("\n"),
  };
}

function stripBlockPrefix(line) {
  return line.replace(
    /^(\s*)(?:#{1,6}\s+|>\s?|[-+*]\s+|\d+\.\s+|-\s+\[[ xX]\]\s+)/,
    "$1",
  );
}

function blockPrefix(type, index) {
  if (type.startsWith("heading")) return `${"#".repeat(Number(type.at(-1)))} `;
  if (type === "ordered") return `${index + 1}. `;
  if (type === "task") return "- [ ] ";
  if (type === "quote") return "> ";
  return "- ";
}

function lineHasBlock(line, type) {
  if (type.startsWith("heading")) {
    return new RegExp(`^\\s*${"#".repeat(Number(type.at(-1)))}\\s+`).test(line);
  }
  if (type === "ordered") return /^\s*\d+\.\s+/.test(line);
  if (type === "task") return /^\s*-\s+\[[ xX]\]\s+/.test(line);
  if (type === "quote") return /^\s*>\s?/.test(line);
  return /^\s*[-+*]\s+/.test(line);
}

function applyBlock(type) {
  const selection = selectedLines();
  const removePrefix = selection.lines.every((line) =>
    lineHasBlock(line, type),
  );
  const transformed = selection.lines
    .map((line, index) => {
      const indent = line.match(/^\s*/)?.[0] || "";
      const content = stripBlockPrefix(line).slice(indent.length);
      return removePrefix
        ? `${indent}${content}`
        : `${indent}${blockPrefix(type, index)}${content}`;
    })
    .join("\n");
  const collapsed = selection.start === selection.end;
  const cursor = collapsed
    ? selection.lineStart + transformed.length
    : selection.lineStart;
  replaceRange(
    selection.lineStart,
    selection.lineEnd,
    transformed,
    cursor,
    collapsed ? cursor : selection.lineStart + transformed.length,
  );
}

function insertCodeBlock() {
  const { start, end } = currentSelection();
  const selected = source.value.slice(start, end);
  const content = selected || $t("editor_codePlaceholder");
  const prefix = "```\n";
  const contentStart = start + prefix.length;
  replaceRange(
    start,
    end,
    `${prefix}${content}\n\`\`\``,
    contentStart,
    contentStart + content.length,
  );
}

function insertTable() {
  const { start, end } = currentSelection();
  const column = $t("editor_tableColumn");
  const content = $t("editor_tableContent");
  const table = `| ${column} 1 | ${column} 2 |\n| --- | --- |\n| ${content} | ${content} |`;
  const leadingBreak = start > 0 && source.value[start - 1] !== "\n";
  const trailingBreak = end < source.value.length && source.value[end] !== "\n";
  const replacement = `${leadingBreak ? "\n" : ""}${table}${trailingBreak ? "\n" : ""}`;
  const firstCell = replacement.indexOf(`${column} 1`);
  replaceRange(
    start,
    end,
    replacement,
    start + firstCell,
    start + firstCell + `${column} 1`.length,
  );
}

function insertLink() {
  const { start, end } = currentSelection();
  const selected = source.value.slice(start, end);
  const text = selected || $t("editor_linkTextPlaceholder");
  const url = $t("editor_linkUrlPlaceholder");
  const urlStart = start + text.length + 3;
  replaceRange(
    start,
    end,
    `[${text}](${url})`,
    urlStart,
    urlStart + url.length,
  );
}

function executeCommand(command) {
  if (command === "bold") applyInline("**", "**", $t("editor_boldPlaceholder"));
  else if (command === "italic")
    applyInline("*", "*", $t("editor_italicPlaceholder"));
  else if (command === "strikethrough")
    applyInline("~~", "~~", $t("editor_strikethroughPlaceholder"));
  else if (command === "code") insertCodeBlock();
  else if (command === "table") insertTable();
  else if (command === "link") insertLink();
  else applyBlock(command);
}

function updateIndentation(event) {
  const start = event.target.selectionStart;
  const end = event.target.selectionEnd;
  const lineStart = source.value.lastIndexOf("\n", Math.max(0, start - 1)) + 1;
  if (start === end) {
    if (event.shiftKey) {
      const removable =
        source.value.slice(lineStart, start).match(/^ {1,2}/)?.[0] || "";
      source.value =
        source.value.slice(0, lineStart) +
        source.value.slice(lineStart + removable.length);
      handleInput();
      restoreSelection(Math.max(lineStart, start - removable.length));
    } else {
      source.value =
        source.value.slice(0, start) + "  " + source.value.slice(end);
      handleInput();
      restoreSelection(start + 2);
    }
    return;
  }
  const normalizedEnd =
    end > start && source.value[end - 1] === "\n" ? end - 1 : end;
  const lineEndIndex = source.value.indexOf("\n", normalizedEnd);
  const lineEnd = lineEndIndex < 0 ? source.value.length : lineEndIndex;
  const lines = source.value.slice(lineStart, lineEnd).split("\n");
  const transformed = (
    event.shiftKey
      ? lines.map((line) =>
          line.startsWith("  ")
            ? line.slice(2)
            : line.startsWith(" ")
              ? line.slice(1)
              : line,
        )
      : lines.map((line) => `  ${line}`)
  ).join("\n");
  source.value =
    source.value.slice(0, lineStart) +
    transformed +
    source.value.slice(lineEnd);
  handleInput();
  restoreSelection(lineStart, lineStart + transformed.length);
}

function handleKeydown(event) {
  if (event.key !== "Tab") return;
  event.preventDefault();
  updateIndentation(event);
}

function extractFiles(dataTransfer) {
  if (!dataTransfer) return [];
  const files = [...dataTransfer.files];
  if (files.length) return files;
  return [...(dataTransfer.items || [])]
    .filter((item) => item.kind === "file")
    .map((item) => item.getAsFile())
    .filter(Boolean);
}

function handlePaste(event) {
  const files = extractFiles(event.clipboardData);
  if (!files.length) return;
  event.preventDefault();
  emit("files", files);
}

function handleDrop(event) {
  const files = extractFiles(event.dataTransfer);
  if (!files.length) return;
  event.preventDefault();
  emit("files", files);
}

function insertBlock(content) {
  const { start, end } = currentSelection();
  const leadingBreak = start > 0 && source.value[start - 1] !== "\n";
  const trailingBreak = end < source.value.length && source.value[end] !== "\n";
  const insertion = `${leadingBreak ? "\n" : ""}${content}${trailingBreak ? "\n" : ""}`;
  replaceRange(
    start,
    end,
    insertion,
    start + insertion.length,
    start + insertion.length,
  );
}

function insertImage(url, name) {
  const alt = name.replace(/([\\[\]])/g, "\\$1");
  insertBlock(`![${alt}](<${url}>)`);
}

function insertFile(url, name) {
  insertBlock(`[${name.replace(/[\[\]]/g, "")}](<${url}>)`);
}

function insertVideo(url) {
  const escapedUrl = String(url).replace(/&/g, "&amp;").replace(/"/g, "&quot;");
  insertBlock(
    `<div data-editor-video="true" data-video-original-url="${escapedUrl}"></div>`,
  );
}

function focus(position) {
  nextTick(() => {
    if (!input.value) return;
    input.value.focus();
    if (position === "end") {
      input.value.setSelectionRange(source.value.length, source.value.length);
    }
    rememberSelection();
  });
}

watch(
  () => props.modelValue,
  (value) => {
    if (value !== lastRenderedHtml) resetFromHtml(value);
  },
);
onMounted(() => resetFromHtml(props.modelValue));

defineExpose({
  focus,
  resetFromHtml,
  getHtml,
  rememberSelection,
  executeCommand,
  insertImage,
  insertFile,
  insertVideo,
});
</script>

<style scoped lang="less">
.markdown-editor-body {
  display: grid;
  flex: 1;
  width: 100%;
  max-width: 100%;
  min-width: 0;
  min-height: var(--editor-min-height);
  grid-template-columns: repeat(2, minmax(0, 1fr));
  grid-template-rows: minmax(0, 1fr);
  overflow: hidden;
}
.markdown-editor-body.preview-hidden {
  grid-template-columns: minmax(0, 1fr);
}
.markdown-editor-body.is-markdown-fill,
.markdown-editor-body.is-markdown-fullscreen {
  flex: 1;
  min-height: 0;
}
.markdown-pane {
  display: flex;
  width: 100%;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  flex-direction: column;
}
.markdown-source-pane {
  border-right: 1px solid #e8e8e8;
}
.preview-hidden .markdown-source-pane {
  border-right: 0;
}
.markdown-source-shell {
  position: relative;
  display: grid;
  flex: 1;
  grid-template-columns: 42px minmax(0, 1fr);
  min-height: calc(var(--editor-min-height) - 32px);
  overflow: hidden;
}
.markdown-line-numbers {
  overflow: hidden;
  border-right: 1px solid #f0f0f0;
  background: #fafafa;
  color: #b5b5b5;
  font:
    14px/1.7 Consolas,
    "Courier New",
    monospace;
  user-select: none;
}
.markdown-line-numbers > div {
  padding-top: 14px;
  text-align: right;
  transition: transform 0.04s linear;
}
.markdown-line-numbers span {
  display: block;
  height: 23.8px;
  padding-right: 9px;
}
.markdown-pane-title {
  flex-shrink: 0;
  height: 32px;
  padding: 7px 12px;
  border-bottom: 1px solid #f0f0f0;
  background: #fafafa;
  color: #8c8c8c;
  font-size: 12px;
  line-height: 18px;
}
.markdown-source {
  display: block;
  height: 100%;
  width: 100%;
  min-width: 0;
  min-height: 0;
  padding: 12px 12px 12px 12px;
  resize: none;
  border: 0;
  outline: none;
  background: #fff;
  color: #333;
  overflow-wrap: anywhere;
  tab-size: 2;
}
.markdown-source::placeholder {
  color: #8c8c8c;
}
.markdown-inline-tools {
  position: absolute;
  z-index: 2;
  display: flex;
  align-items: center;
  gap: 1px;
  min-height: 30px;
  padding: 2px;
  border: 1px solid #dbe3ee;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.98);
  box-shadow: 0 4px 12px rgba(23, 35, 60, 0.12);
}
.markdown-inline-tools button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 26px;
  padding: 0;
  border: 0;
  background: transparent;
  color: #7f8da3;
  cursor: pointer;
  font-size: 12px;
}
.markdown-inline-tools button:hover {
  border-radius: 3px;
  background: #eef4ff;
  color: #2475fc;
}
.markdown-preview-pane {
  width: 100%;
  min-width: 0;
  overflow: hidden;
  background: #fff;
}
.markdown-preview-content,
.markdown-empty-preview {
  flex: 1;
  min-height: calc(var(--editor-min-height) - 32px);
  overflow: auto;
}
.markdown-empty-preview {
  padding: 16px;
  color: #bfbfbf;
  background: #fafafa;
}
.rich-text-content {
  min-height: var(--editor-min-height);
  padding: 16px;
  outline: none;
  color: #333;
  font-size: 14px;
  line-height: 1.7;
  overflow-wrap: anywhere;
}
.rich-text-content :deep(p) {
  margin: 0 0 8px;
}
.rich-text-content :deep(p:last-child) {
  margin-bottom: 0;
}
.rich-text-content :deep(ol),
.rich-text-content :deep(ul) {
  margin: 8px 0;
  padding-left: 24px;
}
.rich-text-content :deep(a) {
  color: #1677ff;
  text-decoration: underline;
}
.rich-text-content :deep(img) {
  display: block;
  max-width: 100%;
  height: auto;
  margin: 12px 0;
}
.markdown-preview-content :deep(h1),
.markdown-preview-content :deep(h2),
.markdown-preview-content :deep(h3),
.markdown-preview-content :deep(h4),
.markdown-preview-content :deep(h5),
.markdown-preview-content :deep(h6) {
  margin: 16px 0 8px;
  color: #262626;
  font-weight: 600;
  line-height: 1.35;
}
.markdown-preview-content :deep(h1) {
  font-size: 24px;
}
.markdown-preview-content :deep(h2) {
  font-size: 20px;
}
.markdown-preview-content :deep(h3) {
  font-size: 17px;
}
.markdown-preview-content :deep(blockquote) {
  margin: 12px 0;
  padding: 4px 12px;
  border-left: 3px solid #d9d9d9;
  color: #666;
  background: #fafafa;
}
.markdown-preview-content :deep(pre) {
  margin: 12px 0;
  padding: 12px;
  overflow: auto;
  border-radius: 4px;
  background: #f5f5f5;
  font-family: Consolas, "Courier New", monospace;
  white-space: pre;
}
.markdown-preview-content :deep(code) {
  padding: 2px 4px;
  border-radius: 3px;
  background: #f5f5f5;
  font-family: Consolas, "Courier New", monospace;
  font-size: 0.92em;
}
.markdown-preview-content :deep(pre code) {
  padding: 0;
  background: transparent;
}
.markdown-preview-content :deep(table) {
  width: 100%;
  margin: 12px 0;
  border-collapse: collapse;
}
.markdown-preview-content :deep(th),
.markdown-preview-content :deep(td) {
  padding: 8px 10px;
  border: 1px solid #d9d9d9;
  text-align: left;
}
.markdown-preview-content :deep(th) {
  background: #fafafa;
  font-weight: 600;
}
.markdown-preview-content :deep(hr) {
  margin: 16px 0;
  border: 0;
  border-top: 1px solid #e8e8e8;
}
.markdown-preview-content :deep(.task-list-item) {
  list-style: none;
}
.markdown-preview-content :deep(input[type="checkbox"]) {
  margin-right: 6px;
  pointer-events: none;
}
.is-markdown-fullscreen .markdown-source,
.is-markdown-fullscreen .markdown-preview-content,
.is-markdown-fullscreen .markdown-empty-preview {
  min-height: 0;
}
@media (max-width: 640px) {
  .markdown-editor-body {
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: minmax(180px, 1fr) minmax(180px, 1fr);
  }
  .markdown-source-pane {
    border-right: 0;
    border-bottom: 1px solid #e8e8e8;
  }
  .markdown-source-shell,
  .markdown-preview-content,
  .markdown-empty-preview {
    min-height: 180px;
  }
  .markdown-inline-tools {
    left: 52px !important;
  }
}
</style>

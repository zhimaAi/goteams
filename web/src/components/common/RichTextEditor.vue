<template>
  <div
    ref="editorRoot"
    class="rich-text-editor"
    :class="{
      'is-fullscreen': isFullscreen,
      'is-fullscreen-preview': fullscreenPreview,
      'is-fill': fill,
      'is-editable': canEdit,
    }"
    :style="{ '--editor-min-height': `${minHeight}px` }"
  >
    <div v-if="fullscreenPreview" class="fullscreen-preview-actions">
      <a-button
        v-if="!readOnly"
        type="text"
        size="small"
        class="fullscreen-preview-edit-button"
        @click="enterFullscreenEdit"
      >
        <EditOutlined /> {{ $t("common_edit") }}
      </a-button>
      <a-tooltip
        :title="$t('requirement_cancelFullscreen')"
        placement="bottomRight"
        :get-popup-container="tooltipContainer"
      >
        <a-button
          type="text"
          size="small"
          class="fullscreen-preview-exit-button"
          :aria-label="$t('requirement_cancelFullscreen')"
          @click="exitFullscreen"
        >
          <FullscreenExitOutlined />
        </a-button>
      </a-tooltip>
    </div>

    <div
      v-if="
        !fullscreenPreview &&
        ((!readOnly && !plain) || compactActions) &&
        (!externalCompactActions || editing)
      "
      class="editor-actions"
      :class="{ compact: compactActions && !editing }"
      :style="compactActions && !editing ? compactActionStyle : undefined"
    >
      <RichTextPreviewActions
        v-if="compactActions && !editing"
        :read-only="readOnly"
        @edit="enterEdit"
        @preview="enterFullscreenPreview"
      />
      <div v-else-if="editing" class="editor-edit-actions">
        <div class="editor-edit-actions-main">
          <a-button type="primary" @click="handleSave">{{
            $t("common_ok")
          }}</a-button>
          <a-button @click="handleCancel">{{ $t("common_cancel") }}</a-button>
          <span class="editor-save-hint">{{ saveHotkeyHint }}</span>
        </div>
        <div class="editor-edit-actions-extra">
          <slot name="editing-extra" />
        </div>
      </div>
      <a-button v-else size="small" @click="enterEdit">
        <EditOutlined /> {{ $t("common_edit") }}
      </a-button>
    </div>

    <div class="editor-shell" @click="handlePreviewImageClick">
      <div v-if="canEdit" class="editor-toolbar">
        <RichTextToolbar
          v-if="editorMode === richTextMode"
          :editor="editor"
          :selected-font-size="selectedFontSize"
          :selected-font-color="selectedFontColor"
          :selected-paragraph="selectedParagraph"
          :format-painter-active="formatPainterActive"
          :allow-uploads="allowUploads"
          :tooltip-placement="tooltipPlacement"
          :get-popup-container="tooltipContainer"
          :get-editor-popup-container="editorPopupContainer"
          @font-size-change="applyFontSize"
          @font-color-change="applyFontColor"
          @format-painter="toggleFormatPainter"
          @clear-format="clearFormat"
          @insert-table="insertTable"
          @open-link="openLinkDialog"
          @choose-image="chooseImage"
          @choose-network-image="openNetworkImageDialog"
          @choose-file="chooseFile"
          @choose-video="chooseVideo"
          @insert-video="openVideoDialog"
          @insert-code-block="openCodeBlockDialog"
          @paragraph-change="applyParagraph"
        />
        <MarkdownToolbar
          v-else
          :preview-visible="markdownPreviewVisible"
          :allow-uploads="allowUploads"
          :tooltip-placement="tooltipPlacement"
          :get-popup-container="tooltipContainer"
          @command="executeMarkdownCommand"
          @remember-selection="markdownEditor?.rememberSelection()"
          @choose-image="chooseImage"
          @toggle-preview="toggleMarkdownPreview"
        />
        <input
          v-if="allowUploads"
          ref="imageInput"
          class="image-input"
          type="file"
          accept="image/png,image/jpeg,image/gif,image/webp"
          @change="insertImage"
        />
        <input
          v-if="allowUploads"
          ref="fileInput"
          class="image-input"
          type="file"
          accept=".pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.txt,.csv,.md,.zip,.rar,.7z,.html,.htm"
          @change="insertFile"
        />
        <input
          v-if="allowUploads"
          ref="videoInput"
          class="image-input"
          type="file"
          accept="video/mp4,video/webm"
          @change="insertLocalVideo"
        />

        <span v-if="editorMode !== richTextMode" class="toolbar-spacer" />

        <a-tooltip
          v-if="allowMarkdown"
          :title="
            editorMode === markdownMode
              ? $t('editor_switchToRichText')
              : $t('editor_switchToMarkdown')
          "
          :placement="tooltipPlacement"
          :get-popup-container="tooltipContainer"
        >
          <a-button
            type="text"
            size="small"
            class="mode-switch-button"
            @click="requestEditorModeSwitch"
          >
            <SwapOutlined />
            <span>{{
              editorMode === markdownMode
                ? $t("editor_richText")
                : $t("editor_markdown")
            }}</span>
          </a-button>
        </a-tooltip>

        <a-tooltip
          :title="
            isFullscreen
              ? $t('requirement_exitFullscreen')
              : $t('requirement_fullscreenEdit')
          "
          :placement="tooltipPlacement"
          :get-popup-container="tooltipContainer"
        >
          <a-button
            type="text"
            size="small"
            class="fullscreen-button"
            @click="toggleFullscreen"
          >
            <FullscreenExitOutlined v-if="isFullscreen" />
            <FullscreenOutlined v-else />
            <span>{{
              isFullscreen
                ? $t("requirement_exitFullscreen")
                : $t("requirement_fullscreenEdit")
            }}</span>
          </a-button>
        </a-tooltip>
      </div>

      <div v-if="!canEdit && contentIsEmpty" class="empty-preview">
        {{ $t("common_noData") }}
      </div>
      <div v-else-if="!canEdit" class="editor-body">
        <div
          class="rich-text-content rendered-rich-text-content"
          v-html="readOnlyHtml"
        />
      </div>
      <MarkdownEditor
        v-else-if="isMarkdownEditing"
        ref="markdownEditor"
        :model-value="modelValue"
        :placeholder="placeholder"
        :preview-visible="markdownPreviewVisible"
        :fullscreen="isFullscreen"
        :fill="fill"
        @update:model-value="handleMarkdownUpdate"
        @files="uploadFiles"
      />
      <EditorContent v-else :editor="editor" class="editor-body" />
    </div>

    <LinkDialog
      :open="linkDialogOpen"
      :value="linkDialogValue"
      @update:open="linkDialogOpen = $event"
      @confirm="applyLink"
    />

    <a-modal
      v-model:open="networkImageDialogOpen"
      :title="$t('editor_networkImage')"
      :ok-text="$t('common_confirm')"
      :cancel-text="$t('common_cancel')"
      :z-index="1300"
      @ok="applyNetworkImage"
    >
      <a-input
        v-model:value="networkImageUrl"
        :placeholder="$t('editor_imageUrlPlaceholder')"
        @press-enter="applyNetworkImage"
      />
      <a-input
        v-model:value="networkImageAlt"
        class="dialog-input-gap"
        :placeholder="$t('editor_imageAltPlaceholder')"
        @press-enter="applyNetworkImage"
      />
    </a-modal>

    <a-modal
      v-model:open="videoDialogOpen"
      :title="$t('editor_videoLink')"
      :ok-text="$t('common_confirm')"
      :cancel-text="$t('common_cancel')"
      :z-index="1300"
      @ok="applyVideo"
    >
      <a-input
        v-model:value="videoUrl"
        :placeholder="$t('editor_videoPlaceholder')"
        @press-enter="applyVideo"
      />
    </a-modal>

    <CodeBlockDialog
      :open="codeBlockDialogOpen"
      :editing="codeBlockEditPosition !== null"
      :value="codeBlockDialogValue"
      @update:open="setCodeBlockDialogOpen"
      @confirm="applyCodeBlock"
    />

    <a-modal
      v-model:open="modeSwitchConfirmOpen"
      :width="500"
      centered
      :z-index="1400"
      :ok-text="$t('common_confirm')"
      :cancel-text="$t('common_cancel')"
      @ok="confirmEditorModeSwitch"
    >
      <template #title>
        <span class="mode-confirm-title">
          <ExclamationCircleFilled />
          {{ $t("editor_switchModeTitle") }}
        </span>
      </template>
      <p class="mode-confirm-message">
        {{
          $t(
            pendingEditorMode === markdownMode
              ? "editor_switchToMarkdownConfirm"
              : "editor_switchToRichTextConfirm",
          )
        }}
      </p>
    </a-modal>

    <div
      v-if="mentionState.open"
      class="mention-suggest"
      :style="{ left: `${mentionState.left}px`, top: `${mentionState.top}px` }"
    >
      <template v-if="mentionCandidates.length">
        <button
          v-for="(member, index) in mentionCandidates"
          :key="member.user_id ?? member.name"
          type="button"
          class="mention-suggest__item"
          :class="{ active: index === mentionState.activeIndex }"
          @mousedown.prevent="selectMention(member)"
        >
          {{ member.name || member.username }}
        </button>
      </template>
      <div v-else class="mention-suggest__empty">
        {{ $t("editor_noMentionMember") }}
      </div>
    </div>
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
import { Extension } from "@tiptap/core";
import { GapCursor } from "@tiptap/pm/gapcursor";
import { NodeSelection, Plugin, TextSelection } from "@tiptap/pm/state";
import { Decoration, DecorationSet } from "@tiptap/pm/view";
import { EditorContent, useEditor } from "@tiptap/vue-3";
import { TableKit } from "@tiptap/extension-table";
import StarterKit from "@tiptap/starter-kit";
import Link from "@tiptap/extension-link";
import { Color, FontSize, TextStyle } from "@tiptap/extension-text-style";
import Underline from "@tiptap/extension-underline";
import Placeholder from "@tiptap/extension-placeholder";
import {
  EditOutlined,
  ExclamationCircleFilled,
  FullscreenExitOutlined,
  FullscreenOutlined,
  SwapOutlined,
} from "@ant-design/icons-vue";
import {
  uploadRequirementFile,
  uploadRequirementImage,
  uploadRequirementVideo,
} from "@/api/upload";
import { closeImageViewer, openImageViewer } from "@/utils/imageViewer";
import MarkdownEditor from "./rich-text-editor/MarkdownEditor.vue";
import MarkdownToolbar from "./rich-text-editor/MarkdownToolbar.vue";
import RichTextToolbar from "./rich-text-editor/RichTextToolbar.vue";
import RichTextPreviewActions from "./rich-text-editor/RichTextPreviewActions.vue";
import CodeBlockDialog from "./rich-text-editor/CodeBlockDialog.vue";
import LinkDialog from "./rich-text-editor/LinkDialog.vue";
import {
  sanitizeStoredHtml,
  toEditorContent,
} from "./rich-text-editor/markdownContent";
import { TableInsertControls } from "./rich-text-editor/tableInsertControls";
import {
  EditorCodeBlock,
  EditorFile,
  EditorImage,
  EditorVideo,
  MentionTag,
  normalizeVideoUrl,
} from "./rich-text-editor/richTextNodes";

// Text entered at the right boundary must not be absorbed into the link mark.
const EditorLink = Link.extend({ inclusive: false });
const blockMediaNodeNames = new Set(["editorFile", "editorVideo"]);

function isMediaBoundary($position) {
  return (
    blockMediaNodeNames.has($position.nodeBefore?.type.name) ||
    blockMediaNodeNames.has($position.nodeAfter?.type.name)
  );
}

// Media gap positions are not text insertion points; only real paragraphs show a caret.
const MediaGapCursorGuard = Extension.create({
  name: "mediaGapCursorGuard",
  addProseMirrorPlugins() {
    return [
      new Plugin({
        filterTransaction(transaction) {
          if (transaction.docChanged) return true;
          const { selection } = transaction;
          if (!(selection instanceof GapCursor)) return true;
          return !isMediaBoundary(selection.$from);
        },
      }),
    ];
  },
});

// Chrome draws a caret sitting right before a large inline image at the bottom
// of the image's line box instead of the text line, so the caret seems to
// vanish onto the next row (ueberdosis/tiptap#4985). A zero-width anchor
// element between the caret and the image keeps the caret on the text line.
// Drop this once the upstream issue is fixed.
const InlineImageCaretAnchor = Extension.create({
  name: "inlineImageCaretAnchor",
  addProseMirrorPlugins() {
    return [
      new Plugin({
        props: {
          decorations(state) {
            const { selection } = state;
            if (
              !(selection instanceof TextSelection) ||
              !selection.empty ||
              selection.$head.nodeAfter?.type.name !== "image"
            )
              return DecorationSet.empty;
            return DecorationSet.create(state.doc, [
              Decoration.widget(
                selection.$head.pos,
                () => {
                  const anchor = document.createElement("span");
                  anchor.style.display = "inline-block";
                  anchor.style.width = "0";
                  return anchor;
                },
                { side: -1 },
              ),
            ]);
          },
        },
      }),
    ];
  },
});

const props = defineProps({
  modelValue: { type: String, default: "" },
  placeholder: { type: String, default: "" },
  readOnly: { type: Boolean, default: false },
  defaultEditing: { type: Boolean, default: false },
  plain: { type: Boolean, default: false },
  fill: { type: Boolean, default: false },
  minHeight: { type: Number, default: 180 },
  compactActions: { type: Boolean, default: false },
  externalCompactActions: { type: Boolean, default: false },
  wsId: { type: [Number, String], default: 0 },
  imageFileIds: { type: Array, default: () => [] },
  allowUploads: { type: Boolean, default: true },
  contentMode: { type: String, default: "" },
  allowMarkdown: { type: Boolean, default: true },
  // 父组件显式要求将外部 HTML 回填到 markdown 编辑器时递增。
  // 日常 modelValue 更新仍不打断 markdown 输入和光标位置。
  contentRevision: { type: Number, default: 0 },
  // 可替换的上传器（默认使用需求上传器），每个接收 (file)，
  // 解析为 { url, name?, size? }；videoUploader 可抛出 { skipToast: true } 展示自身提示。
  imageUploader: { type: Function, default: null },
  fileUploader: { type: Function, default: null },
  videoUploader: { type: Function, default: null },
  // 是否开启「输入 @ 触发成员搜索」。评论框与回复框使用。
  mentionEnabled: { type: Boolean, default: false },
  // @人 候选成员，每项 { user_id, name, username }；邀填人项由父组件追加。
  mentionMembers: { type: Array, default: () => [] },
});
const emit = defineEmits([
  "update:modelValue",
  "update:contentMode",
  "save",
  "cancel",
  "editing-change",
  "update:imageFileIds",
]);
const { t: $t } = useI18n();
const imageUpload =
  props.imageUploader || ((file) => uploadRequirementImage(props.wsId, file));
const fileUpload =
  props.fileUploader || ((file) => uploadRequirementFile(props.wsId, file));
const videoUpload =
  props.videoUploader || ((file) => uploadRequirementVideo(props.wsId, file));
const richTextMode = "rich-text";
const markdownMode = "markdown";
const editorModeStorageKey = "goteams-rich-text-mode";
const defaultFontSize = "14px";
const defaultFontColor = "#333333";
const editing = ref(props.defaultEditing);
const originalValue = ref("");
const originalMode = ref("");
const isFullscreen = ref(false);
const fullscreenPreview = ref(false);
const editorMode = ref(
  props.allowMarkdown
    ? normalizeEditorMode(props.contentMode) || readStoredEditorMode()
    : richTextMode,
);
const pendingEditorMode = ref("");
const modeSwitchConfirmOpen = ref(false);
const markdownPreviewVisible = ref(true);
const markdownEditor = ref(null);
const linkDialogOpen = ref(false);
const linkDialogValue = ref({ href: "", text: "", target: "_blank" });
const linkSelection = ref({ from: 0, to: 0, text: "" });
const imageInput = ref(null);
const fileInput = ref(null);
const videoInput = ref(null);
const networkImageDialogOpen = ref(false);
const networkImageUrl = ref("");
const networkImageAlt = ref("");
const videoDialogOpen = ref(false);
const videoUrl = ref("");
const codeBlockDialogOpen = ref(false);
const codeBlockEditPosition = ref(null);
const codeBlockDialogValue = ref({
  theme: "default",
  language: "javascript",
  code: "",
});
const editorRoot = ref(null);
const compactActionOffset = ref(0);
const selectedFontSize = ref(defaultFontSize);
const selectedFontColor = ref(defaultFontColor);
const selectedParagraph = ref("paragraph");
const formatPainterActive = ref(false);
const formatPainterSnapshot = ref(null);
let previousBodyOverflow = "";
let editorInitialTop = 0;
let colorParser;
let caretScrollSnapshot = null;

const canEdit = computed(
  () =>
    !fullscreenPreview.value &&
    !props.readOnly &&
    (props.plain || editing.value),
);
const tooltipPlacement = computed(() =>
  isFullscreen.value ? "bottom" : "top",
);
const isMarkdownEditing = computed(
  () => canEdit.value && editorMode.value === markdownMode,
);
const readOnlyHtml = computed(() => sanitizeStoredHtml(props.modelValue));
const contentIsEmpty = computed(() => isHtmlContentEmpty(props.modelValue));
const compactActionStyle = computed(() => ({
  transform: `translateY(${compactActionOffset.value}px)`,
}));

function isHtmlContentEmpty(value) {
  if (!value) return true;
  if (
    /<(?:img|video|iframe|hr|table|input)\b|data-editor-(?:file|video|code-block)=/i.test(
      value,
    )
  )
    return false;
  return (
    value
      .replace(/<[^>]*>/g, "")
      .replace(/&nbsp;/gi, "")
      .trim() === ""
  );
}

function readStoredEditorMode() {
  if (typeof window === "undefined") return richTextMode;
  try {
    return window.localStorage.getItem(editorModeStorageKey) === markdownMode
      ? markdownMode
      : richTextMode;
  } catch {
    return richTextMode;
  }
}

function normalizeEditorMode(value) {
  return value === richTextMode || value === markdownMode ? value : "";
}

function persistEditorMode() {
  try {
    window.localStorage.setItem(editorModeStorageKey, editorMode.value);
  } catch {
    // Switching still works when the browser blocks local storage; only the preference is lost.
  }
}

function normalizeColorInput(value) {
  if (!value || typeof document === "undefined") return defaultFontColor;
  colorParser ||= document.createElement("canvas").getContext("2d");
  colorParser.fillStyle = defaultFontColor;
  colorParser.fillStyle = value;
  return /^#[0-9a-f]{6}$/i.test(colorParser.fillStyle)
    ? colorParser.fillStyle
    : defaultFontColor;
}

function syncCurrentTextStyle(currentEditor) {
  const attributes = currentEditor.getAttributes("textStyle");
  selectedFontSize.value = attributes.fontSize || defaultFontSize;
  selectedFontColor.value = normalizeColorInput(attributes.color);
  const parent = currentEditor.state.selection.$from.parent;
  selectedParagraph.value =
    parent.type.name === "heading"
      ? `heading-${parent.attrs.level}`
      : "paragraph";
}

function syncEditorFromHtml(value) {
  if (!editor.value) return;
  const currentContent = editor.value.getHTML();
  if (value === currentContent) return;
  const content = toEditorContent(value);
  if (content === currentContent) return;
  editor.value.commands.setContent(content, { emitUpdate: false });
}

function handleTableBoundaryDelete(view, event) {
  if (
    (event.key !== "Backspace" && event.key !== "Delete") ||
    event.altKey ||
    event.ctrlKey ||
    event.metaKey ||
    event.shiftKey
  ) {
    return false;
  }

  const { selection } = view.state;
  const { $from } = selection;
  if (!selection.empty || $from.parent.type.name !== "paragraph") return false;

  const deleteBefore = event.key === "Backspace";
  const atBoundary = deleteBefore
    ? $from.parentOffset === 0
    : $from.parentOffset === $from.parent.content.size;
  if (!atBoundary) return false;

  const boundaryPosition = deleteBefore
    ? $from.before($from.depth)
    : $from.after($from.depth);
  const $boundary = view.state.doc.resolve(boundaryPosition);
  const table = deleteBefore ? $boundary.nodeBefore : $boundary.nodeAfter;
  if (table?.type.name !== "table") return false;

  const from = deleteBefore
    ? boundaryPosition - table.nodeSize
    : boundaryPosition;
  view.dispatch(
    view.state.tr.delete(from, from + table.nodeSize).scrollIntoView(),
  );
  return true;
}

function flushEditorValue() {
  if (editorMode.value === markdownMode) {
    const html = markdownEditor.value?.getHtml() || "";
    if (html !== props.modelValue) emit("update:modelValue", html);
    return;
  }
  if (!editor.value) return;
  const html = editor.value.isEmpty ? "" : editor.value.getHTML();
  if (html !== props.modelValue) emit("update:modelValue", html);
}

// A large inline image that wraps onto its own line makes both ProseMirror and
// the browser measure the caret as the image instead of the text line, so every
// keystroke beside the image scrolls the viewport down (ueberdosis/tiptap#4985).
// ProseMirror's own scroll is cancelled by handleScrollToSelection; the browser
// cannot be intercepted, so the scroll position is restored right after the
// input instead. Drop this once the upstream issue is fixed.
function isCaretBesideInlineImage(selection) {
  if (!(selection instanceof TextSelection) || !selection.empty) return false;
  const { $head } = selection;
  return (
    Boolean($head.parent.inlineContent) &&
    ($head.nodeAfter?.type.name === "image" ||
      $head.nodeBefore?.type.name === "image")
  );
}

function captureScrollPositions(dom) {
  const elements = [];
  for (
    let element = dom.parentElement;
    element && element !== document.body;
    element = element.parentElement
  ) {
    if (element.scrollHeight > element.clientHeight + 1)
      elements.push([element, element.scrollTop]);
  }
  return { elements, window: [window.scrollX, window.scrollY] };
}

// Only single-character keys produce text; modifier combinations and keys pressed while
// an input method is composing are left to their own handlers.
function isTextInputKey(event) {
  return (
    event.key.length === 1 &&
    !event.ctrlKey &&
    !event.metaKey &&
    !event.altKey &&
    !event.isComposing
  );
}

// An input method reports the keys that start composing as keyCode 229 / "Process" and
// the composed text arrives without it, so those keys cannot insert the text here.
function isCompositionKey(event) {
  return event.isComposing || event.keyCode === 229 || event.key === "Process";
}

// The browser would type over the selected image and take it with it, so the text is
// written beside the node: the selection collapses, the image stays.
function insertTextAfterSelectedImage(view, text) {
  const position = view.state.selection.to;
  const transaction = view.state.tr.insertText(text, position);
  transaction.setSelection(
    TextSelection.create(transaction.doc, position + text.length),
  );
  view.dispatch(transaction);
}

function handleEditorKeyDown(view, event) {
  if (handleMentionKeyDown(event)) return true;
  const { selection } = view.state;
  if (
    selection instanceof NodeSelection &&
    selection.node.type.name === "image"
  ) {
    if (event.key === "Delete" || event.key === "Backspace") {
      event.preventDefault();
      view.dispatch(view.state.tr.deleteSelection().scrollIntoView());
      return true;
    }
    if (isTextInputKey(event)) {
      event.preventDefault();
      insertTextAfterSelectedImage(view, event.key);
      return true;
    }
    // A composition inserts its own text later, only the caret can be moved out of the
    // selected image before it starts.
    if (isCompositionKey(event)) {
      view.dispatch(
        view.state.tr.setSelection(
          TextSelection.create(view.state.doc, selection.to),
        ),
      );
    }
  }
  return handleTableBoundaryDelete(view, event);
}

// ---------------- 输入 @ 触发成员搜索 ----------------

const mentionMaxCandidates = 8;
const mentionState = ref({
  open: false,
  query: "",
  from: 0,
  to: 0,
  activeIndex: 0,
  left: 0,
  top: 0,
});

function filterMentionMembers(query) {
  const keyword = String(query || "")
    .trim()
    .toLowerCase();
  const members = props.mentionMembers || [];
  const matched = keyword
    ? members.filter((member) =>
        `${member.name || ""} ${member.username || ""}`
          .toLowerCase()
          .includes(keyword),
      )
    : members;
  // 固定候选（如邀填人）不参与 8 条上限竞争，否则成员多的项目里必然被截掉。
  const pinned = matched.filter((member) => member.invite === true);
  if (!pinned.length) return matched.slice(0, mentionMaxCandidates);
  const others = matched.filter((member) => member.invite !== true);
  return [
    ...others.slice(0, Math.max(0, mentionMaxCandidates - pinned.length)),
    ...pinned,
  ];
}

const mentionCandidates = computed(() =>
  filterMentionMembers(mentionState.value.query),
);

function closeMention() {
  if (!mentionState.value.open) return;
  mentionState.value.open = false;
  mentionState.value.query = "";
  mentionState.value.activeIndex = 0;
}

// 光标前出现未完成的「@查询」时打开候选列表；光标落在已插入的标签内不重复触发。
function syncMentionPopup() {
  if (!props.mentionEnabled || !editor.value || props.readOnly)
    return closeMention();
  const { state } = editor.value;
  const { $from, empty } = state.selection;
  if (!empty || !$from.parent.isTextblock) return closeMention();
  const textBefore = $from.parent.textBetween(
    0,
    $from.parentOffset,
    null,
    "\ufffc",
  );
  const matched = /@([^@\s\u00a0]*)$/.exec(textBefore);
  if (!matched) return closeMention();
  const from = $from.pos - matched[1].length - 1;
  if (state.doc.rangeHasMark(from, $from.pos, state.schema.marks.mentionTag))
    return closeMention();
  // 候选为空时保留弹窗，由模板渲染「没有可@成员」空态，避免用户看不到任何反馈。
  const candidates = filterMentionMembers(matched[1]);
  const coords = editor.value.view.coordsAtPos(from);
  const rootRect = editorRoot.value?.getBoundingClientRect?.();
  const previousIndex = mentionState.value.activeIndex;
  mentionState.value = {
    open: true,
    query: matched[1],
    from,
    to: $from.pos,
    activeIndex: candidates.length
      ? Math.min(previousIndex, candidates.length - 1)
      : 0,
    left: Math.max(0, coords.left - (rootRect?.left || 0)),
    top: coords.bottom - (rootRect?.top || 0) + 6,
  };
}

// 候选列表打开时接管方向键、回车与 Tab；返回 true 表示事件已被处理。
function handleMentionKeyDown(event) {
  if (!mentionState.value.open) return false;
  const candidates = mentionCandidates.value;
  if (event.key === "ArrowDown" || event.key === "ArrowUp") {
    if (!candidates.length) return false;
    const delta = event.key === "ArrowDown" ? 1 : -1;
    mentionState.value.activeIndex =
      (mentionState.value.activeIndex + delta + candidates.length) %
      candidates.length;
    event.preventDefault();
    return true;
  }
  if (event.key === "Enter" || event.key === "Tab") {
    if (!candidates.length) return false;
    event.preventDefault();
    insertMention(candidates[mentionState.value.activeIndex] || candidates[0]);
    return true;
  }
  if (event.key === "Escape") closeMention();
  return false;
}

// 把「@查询」替换为带标签样式的 @显示名；邀填人额外打上 invite 标记，
// 供提交时置 at_invite（后端不解析「邀填人」这个多语言固定词）。
function insertMention(member) {
  if (!editor.value || !member) return;
  const { from, to } = mentionState.value;
  const label = `@${member.name || member.username || ""}`;
  if (label === "@") return;
  const { state, view } = editor.value;
  const mark = state.schema.marks.mentionTag.create({
    invite: member.invite === true,
  });
  const transaction = state.tr.delete(from, to).insertText(label, from);
  transaction.addMark(from, from + label.length, mark);
  transaction.setSelection(
    TextSelection.create(transaction.doc, from + label.length),
  );
  transaction.removeStoredMark(state.schema.marks.mentionTag);
  view.dispatch(transaction);
  view.focus();
  closeMention();
}

function selectMention(member) {
  insertMention(member);
}

const editor = useEditor({
  content: toEditorContent(props.modelValue),
  editable: !props.readOnly && (props.plain || props.defaultEditing),
  extensions: [
    StarterKit.configure({
      link: false,
      trailingNode: false,
      underline: false,
    }),
    Underline,
    TextStyle,
    FontSize,
    Color,
    EditorLink.configure({
      openOnClick: false,
      HTMLAttributes: { rel: "noopener noreferrer", target: "_blank" },
    }),
    EditorImage.configure({
      inline: true,
      allowBase64: true,
      HTMLAttributes: { loading: "lazy" },
      resize: {
        enabled: true,
        directions: ["top-left", "top-right", "bottom-left", "bottom-right"],
        minWidth: 80,
        minHeight: 60,
      },
    }),
    TableKit.configure({
      table: { resizable: true, cellMinWidth: 60 },
    }),
    TableInsertControls,
    EditorFile,
    EditorCodeBlock,
    EditorVideo,
    MentionTag,
    MediaGapCursorGuard,
    InlineImageCaretAnchor,
    Placeholder.configure({ placeholder: () => props.placeholder }),
  ],
  editorProps: {
    attributes: { class: "rich-text-content" },
    handleClickOn(view, _position, node, nodePosition, event, direct) {
      if (
        !direct ||
        (node.type.name !== "image" && node.type.name !== "editorFile")
      )
        return false;
      event.preventDefault();
      view.dispatch(
        view.state.tr
          .setSelection(NodeSelection.create(view.state.doc, nodePosition))
          .scrollIntoView(),
      );
      view.focus();
      return true;
    },
    handleClick(view, position) {
      return isMediaBoundary(view.state.doc.resolve(position));
    },
    // prosemirror-view passes only the view here, the selection must be read
    // from its state.
    handleScrollToSelection(view) {
      return isCaretBesideInlineImage(view.state.selection);
    },
    handleKeyDown: handleEditorKeyDown,
    handlePaste(view, event) {
      const files = extractFiles(event.clipboardData);
      if (!files.length) return false;
      event.preventDefault();
      void uploadFiles(files);
      return true;
    },
    handleDrop(view, event) {
      const files = extractFiles(event.dataTransfer);
      if (!files.length) return false;
      event.preventDefault();
      void uploadFiles(files);
      return true;
    },
    handleDoubleClickOn(_view, _pos, node, nodePos, _event, direct) {
      if (!direct || node.type.name !== "editorCodeBlock") return false;
      openCodeBlockDialog(nodePos, node.attrs);
      return true;
    },
    handleDOMEvents: {
      beforeinput: (view) => {
        caretScrollSnapshot = isCaretBesideInlineImage(view.state.selection)
          ? captureScrollPositions(view.dom)
          : null;
        return false;
      },
      input: () => {
        const snapshot = caretScrollSnapshot;
        caretScrollSnapshot = null;
        if (!snapshot) return false;
        window.requestAnimationFrame(() => {
          snapshot.elements.forEach(([element, scrollTop]) => {
            element.scrollTop = scrollTop;
          });
          window.scrollTo(snapshot.window[0], snapshot.window[1]);
        });
        return false;
      },
      click: (view, event) => {
        if (event.button !== 0 || (!event.ctrlKey && !event.metaKey))
          return false;
        const link = event.target?.closest?.("a[href]");
        if (!link || !view.dom.contains(link)) return false;
        event.preventDefault();
        window.open(
          link.href,
          link.getAttribute("target") || "_blank",
          "noopener,noreferrer",
        );
        return true;
      },
      mouseup: () => {
        if (formatPainterActive.value)
          window.requestAnimationFrame(applyFormatPainter);
        return false;
      },
      touchend: () => {
        if (formatPainterActive.value)
          window.requestAnimationFrame(applyFormatPainter);
        return false;
      },
      keyup: (_view, event) => {
        if (event.key === "Escape") {
          cancelFormatPainter();
        } else if (formatPainterActive.value) {
          window.requestAnimationFrame(applyFormatPainter);
        }
        return false;
      },
    },
  },
  onBlur: () => {
    // 编辑器失焦后浮层不该继续留在页面上（候选为空、只剩空态文案时尤其明显）。
    closeMention();
  },
  onCreate: ({ editor: currentEditor }) => {
    syncCurrentTextStyle(currentEditor);
    syncMentionPopup();
  },
  onSelectionUpdate: ({ editor: currentEditor }) => {
    syncCurrentTextStyle(currentEditor);
    syncMentionPopup();
  },
  onUpdate: ({ editor: currentEditor }) => {
    syncCurrentTextStyle(currentEditor);
    syncMentionPopup();
    emit(
      "update:modelValue",
      currentEditor.isEmpty ? "" : currentEditor.getHTML(),
    );
  },
});

function handleMarkdownUpdate(value) {
  emit("update:modelValue", value);
}

function executeMarkdownCommand(command) {
  markdownEditor.value?.executeCommand(command);
}

function toggleMarkdownPreview() {
  markdownPreviewVisible.value = !markdownPreviewVisible.value;
  markdownEditor.value?.focus();
}

function focusActiveEditor(position) {
  nextTick(() => {
    if (editorMode.value === markdownMode) {
      markdownEditor.value?.focus(position);
      return;
    }
    editor.value?.commands.focus(position);
  });
}

function requestEditorModeSwitch() {
  if (!props.allowMarkdown) return;
  pendingEditorMode.value =
    editorMode.value === richTextMode ? markdownMode : richTextMode;
  if (isCurrentEditorContentEmpty()) {
    toggleEditorMode();
    return;
  }
  modeSwitchConfirmOpen.value = true;
}

function isCurrentEditorContentEmpty() {
  if (editorMode.value === richTextMode) {
    return editor.value ? editor.value.isEmpty : contentIsEmpty.value;
  }
  return isHtmlContentEmpty(
    markdownEditor.value?.getHtml() || props.modelValue,
  );
}

function confirmEditorModeSwitch() {
  modeSwitchConfirmOpen.value = false;
  toggleEditorMode();
}

function toggleEditorMode() {
  if (editorMode.value === richTextMode) {
    const html = editor.value?.isEmpty ? "" : editor.value?.getHTML() || "";
    editorMode.value = markdownMode;
    nextTick(() => {
      markdownEditor.value?.resetFromHtml(html);
      markdownEditor.value?.focus();
    });
  } else {
    const html = markdownEditor.value?.getHtml() || "";
    syncEditorFromHtml(html);
    if (props.modelValue !== html) emit("update:modelValue", html);
    editorMode.value = richTextMode;
    focusActiveEditor();
  }
  emit("update:contentMode", editorMode.value);
  persistEditorMode();
}

function applyFontSize(value) {
  const chain = editor.value?.chain().focus();
  if (!chain) return;
  if (value === defaultFontSize) chain.unsetFontSize().run();
  else chain.setFontSize(value).run();
  selectedFontSize.value = value;
}

function applyFontColor(value) {
  const color = normalizeColorInput(value);
  const chain = editor.value?.chain().focus();
  if (!chain) return;
  if (color === defaultFontColor) chain.unsetColor().run();
  else chain.setColor(color).run();
  selectedFontColor.value = color;
}

function getPainterMarks(currentEditor) {
  const selection = currentEditor.state.selection;
  const marks = currentEditor.state.storedMarks || selection.$from.marks();
  return marks
    .filter((mark) =>
      ["bold", "italic", "underline", "strike", "textStyle"].includes(
        mark.type.name,
      ),
    )
    .map((mark) => ({ name: mark.type.name, attrs: { ...mark.attrs } }));
}

function toggleFormatPainter() {
  if (formatPainterActive.value) {
    cancelFormatPainter();
    return;
  }
  if (!editor.value) return;
  const parent = editor.value.state.selection.$from.parent;
  formatPainterSnapshot.value = {
    marks: getPainterMarks(editor.value),
    block:
      parent.type.name === "heading"
        ? { name: "heading", level: parent.attrs.level }
        : { name: "paragraph" },
  };
  formatPainterActive.value = true;
}

function cancelFormatPainter() {
  formatPainterActive.value = false;
  formatPainterSnapshot.value = null;
}

function applyFormatPainter() {
  if (
    !formatPainterActive.value ||
    !formatPainterSnapshot.value ||
    !editor.value
  )
    return;
  const { from, to } = editor.value.state.selection;
  if (from === to) return;
  const snapshot = formatPainterSnapshot.value;
  let chain = editor.value.chain().focus().unsetAllMarks();
  if (snapshot.block.name === "heading")
    chain = chain.setHeading({ level: snapshot.block.level });
  else chain = chain.setParagraph();
  snapshot.marks.forEach((mark) => {
    chain = chain.setMark(mark.name, mark.attrs);
  });
  chain.run();
  cancelFormatPainter();
}

function clearFormat() {
  cancelFormatPainter();
  editor.value?.chain().focus().unsetAllMarks().clearNodes().run();
}

function applyParagraph(value) {
  if (!editor.value) return;
  const chain = editor.value.chain().focus();
  if (value === "paragraph") {
    chain.setParagraph().run();
    selectedParagraph.value = "paragraph";
    return;
  }
  const level = Number(value.replace("heading-", ""));
  if (level >= 1 && level <= 6) {
    chain.setHeading({ level }).run();
    selectedParagraph.value = `heading-${level}`;
  }
}

function insertTable(size = {}) {
  const rows = Math.min(50, Math.max(1, Number(size.rows) || 3));
  const cols = Math.min(50, Math.max(1, Number(size.columns) || 3));
  editor.value
    ?.chain()
    .focus()
    .insertTable({ rows, cols, withHeaderRow: true })
    .command(({ tr, state }) => {
      const { $from } = tr.selection;
      let tableDepth = -1;
      for (let depth = $from.depth; depth > 0; depth -= 1) {
        if ($from.node(depth).type.name === "table") {
          tableDepth = depth;
          break;
        }
      }
      if (tableDepth < 0) return true;

      const table = $from.node(tableDepth);
      const tablePosition = $from.before(tableDepth);
      const tableEndPosition = tablePosition + table.nodeSize;
      const paragraph = state.schema.nodes.paragraph;
      if (tr.doc.nodeAt(tableEndPosition)?.type !== paragraph) {
        tr.insert(tableEndPosition, paragraph.create());
      }
      if (tr.doc.resolve(tablePosition).nodeBefore?.type !== paragraph) {
        tr.insert(tablePosition, paragraph.create());
      }
      return true;
    })
    .run();
}

function openCodeBlockDialog(position = null, attrs = {}) {
  codeBlockEditPosition.value = Number.isInteger(position) ? position : null;
  codeBlockDialogValue.value = {
    theme: attrs.theme || "default",
    language: attrs.language || "javascript",
    code: attrs.code || "",
  };
  codeBlockDialogOpen.value = true;
}

function setCodeBlockDialogOpen(open) {
  codeBlockDialogOpen.value = open;
  if (!open) codeBlockEditPosition.value = null;
}

function applyCodeBlock(attrs) {
  if (codeBlockEditPosition.value === null) {
    editor.value
      ?.chain()
      .focus()
      .insertContent([
        { type: "editorCodeBlock", attrs },
        { type: "paragraph" },
      ])
      .run();
  } else {
    const position = codeBlockEditPosition.value;
    const currentNode = editor.value?.state.doc.nodeAt(position);
    if (!currentNode || currentNode.type.name !== "editorCodeBlock") {
      message.warning($t("editor_codeBlockNotFound"));
      setCodeBlockDialogOpen(false);
      return;
    }
    editor.value
      ?.chain()
      .focus()
      .command(({ tr }) => {
        tr.setNodeMarkup(position, undefined, attrs);
        return true;
      })
      .run();
  }
  setCodeBlockDialogOpen(false);
}

function openNetworkImageDialog() {
  networkImageUrl.value = "";
  networkImageAlt.value = "";
  networkImageDialogOpen.value = true;
}

function applyNetworkImage() {
  const value = networkImageUrl.value.trim();
  let url;
  try {
    url = new URL(value);
  } catch {
    url = null;
  }
  if (
    !url ||
    !["http:", "https:"].includes(url.protocol) ||
    url.username ||
    url.password
  ) {
    message.warning($t("editor_imageUrlInvalid"));
    return;
  }
  insertImageContent({
    src: url.href,
    alt: networkImageAlt.value.trim(),
  });
  networkImageDialogOpen.value = false;
}

function insertImageContent(attrs) {
  editor.value?.chain().focus().insertContent({ type: "image", attrs }).run();
}

function openVideoDialog() {
  videoUrl.value = "";
  videoDialogOpen.value = true;
}

function applyVideo() {
  const normalized = normalizeVideoUrl(videoUrl.value);
  if (!normalized) {
    message.warning($t("editor_videoUrlInvalid"));
    return;
  }
  insertVideoContent(normalized);
  videoDialogOpen.value = false;
}

function insertVideoContent(attrs) {
  editor.value
    ?.chain()
    .focus()
    .insertContent([{ type: "editorVideo", attrs }, { type: "paragraph" }])
    .run();
}

function openLinkDialog() {
  if (!editor.value) return;
  const { from, to } = editor.value.state.selection;
  const text = editor.value.state.doc.textBetween(from, to, " ");
  linkSelection.value = { from, to, text };
  linkDialogValue.value = {
    href: "",
    text: "",
    target: "_blank",
  };
  linkDialogOpen.value = true;
}

function applyLink({ href, text, target }) {
  if (!editor.value) return;
  const { from, to, text: selectedText } = linkSelection.value;
  const displayText = text || selectedText || href;
  let selectionEnd = to;
  let chain = editor.value.chain().focus().setTextSelection({ from, to });

  if (displayText && displayText !== selectedText) {
    selectionEnd = from + displayText.length;
    chain = chain
      .insertContent(displayText)
      .setTextSelection({ from, to: selectionEnd });
  }

  if (href) {
    chain = chain
      .setLink({ href, target })
      .setTextSelection(selectionEnd)
      // Keep subsequent input outside the link that was just inserted.
      .unsetMark("link");
  } else {
    chain = chain.unsetLink().setTextSelection(selectionEnd);
  }
  chain.run();
  linkDialogOpen.value = false;
}

function chooseImage() {
  if (props.allowUploads) imageInput.value?.click();
}

function chooseFile() {
  if (props.allowUploads) fileInput.value?.click();
}

function chooseVideo() {
  if (props.allowUploads) videoInput.value?.click();
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

async function uploadFiles(files) {
  if (!props.allowUploads) return;
  for (const file of files) {
    if (
      file.type.startsWith("image/") ||
      /\.(gif|jpe?g|png|webp)$/i.test(file.name)
    )
      await uploadAndInsert(file);
    else if (file.type.startsWith("video/") || /\.(mp4|webm)$/i.test(file.name))
      await uploadAndInsertVideo(file);
    else await uploadAndInsertFile(file);
  }
}

function trackUploadedFile(res) {
  if (!res.file_id) return;
  emit("update:imageFileIds", [
    ...new Set([...props.imageFileIds, res.file_id]),
  ]);
}

async function uploadAndInsert(file) {
  if (
    !["image/png", "image/jpeg", "image/gif", "image/webp"].includes(
      file.type,
    ) &&
    !/\.(gif|jpe?g|png|webp)$/i.test(file.name)
  ) {
    message.warning($t("editor_imageTypeInvalid"));
    return;
  }
  if (file.size > 20 * 1024 * 1024) {
    message.warning($t("editor_imageTooLarge"));
    return;
  }
  // duration 0: keep the notice open until the request settles (the default 3s hid it mid-upload).
  message.loading({
    content: $t("editor_imageUploading"),
    key: "image-upload",
    duration: 0,
  });
  try {
    const res = await imageUpload(file);
    if (editorMode.value === markdownMode)
      markdownEditor.value?.insertImage(res.url, file.name);
    else insertImageContent({ src: res.url, alt: file.name });
    trackUploadedFile(res);
    message.success({
      content: $t("editor_imageInserted"),
      key: "image-upload",
    });
  } catch {
    message.error({
      content: $t("editor_imageUploadFailed"),
      key: "image-upload",
    });
  }
}

function insertImage(event) {
  const file = event.target.files?.[0];
  event.target.value = "";
  if (file) uploadAndInsert(file);
}

function insertFile(event) {
  const file = event.target.files?.[0];
  event.target.value = "";
  if (file) void uploadAndInsertFile(file);
}

async function uploadAndInsertFile(file) {
  const extension = file.name.includes(".")
    ? file.name.split(".").pop().toLowerCase()
    : "";
  const allowed = [
    "pdf",
    "doc",
    "docx",
    "xls",
    "xlsx",
    "ppt",
    "pptx",
    "txt",
    "csv",
    "md",
    "zip",
    "rar",
    "7z",
    "html",
    "htm",
  ];
  if (!allowed.includes(extension)) {
    message.warning($t("editor_fileTypeInvalid"));
    return;
  }
  if (file.size > 200 * 1024 * 1024) {
    message.warning($t("editor_fileTooLarge"));
    return;
  }
  // duration 0: keep the notice open until the request settles (the default 3s hid it mid-upload).
  message.loading({
    content: $t("editor_fileUploading"),
    key: "editor-file-upload",
    duration: 0,
  });
  try {
    const res = await fileUpload(file);
    const name = res.name || file.name;
    const size = res.size || file.size;
    if (editorMode.value === markdownMode) {
      markdownEditor.value?.insertFile(res.url, name);
    } else {
      editor.value
        ?.chain()
        .focus()
        .insertContent([
          {
            type: "editorFile",
            attrs: { href: res.url, fileName: name, fileSize: size },
          },
          { type: "paragraph" },
        ])
        .run();
    }
    trackUploadedFile(res);
    message.success({
      content: $t("editor_fileInserted"),
      key: "editor-file-upload",
    });
  } catch {
    message.error({
      content: $t("editor_fileUploadFailed"),
      key: "editor-file-upload",
    });
  }
}

function insertLocalVideo(event) {
  const file = event.target.files?.[0];
  event.target.value = "";
  if (file) void uploadAndInsertVideo(file);
}

async function uploadAndInsertVideo(file) {
  const extension = file.name.split(".").pop()?.toLowerCase() || "";
  if (!["mp4", "webm"].includes(extension)) {
    message.warning($t("editor_videoTypeInvalid"));
    return;
  }
  if (file.size > 200 * 1024 * 1024) {
    message.warning($t("editor_videoTooLarge"));
    return;
  }
  // duration 0: keep the notice open until the request settles (the default 3s hid it mid-upload).
  message.loading({
    content: $t("editor_videoUploading"),
    key: "editor-video-upload",
    duration: 0,
  });
  try {
    const res = await videoUpload(file);
    const normalized = normalizeVideoUrl(res.url);
    if (!normalized) throw new Error("invalid uploaded video url");
    if (editorMode.value === markdownMode)
      markdownEditor.value?.insertVideo(normalized.originalUrl);
    else insertVideoContent(normalized);
    trackUploadedFile(res);
    message.success({
      content: $t("editor_videoInserted"),
      key: "editor-video-upload",
    });
  } catch (error) {
    // A custom uploader that reports the failure itself still has to dismiss the loading notice,
    // which no longer closes on its own.
    if (error?.skipToast) {
      message.destroy("editor-video-upload");
      return;
    }
    message.error({
      content: $t("editor_videoUploadFailed"),
      key: "editor-video-upload",
    });
  }
}

function handlePreviewImageClick(event) {
  if (canEdit.value) return;
  const target = event.target?.closest?.(".rich-text-content img");
  if (!target || !editorRoot.value?.contains(target)) return;
  const images = [
    ...editorRoot.value.querySelectorAll(".rich-text-content img"),
  ]
    .map((image) => ({
      element: image,
      src: image.getAttribute("src") || image.currentSrc || image.src,
    }))
    .filter((image) => image.src);
  const current = images.findIndex((image) => image.element === target);
  if (current < 0) return;
  event.preventDefault();
  openImageViewer({
    sources: images.map((image) => image.src),
    index: current,
  });
}

function syncCompactActionPosition() {
  if (
    !editorRoot.value ||
    !props.compactActions ||
    props.externalCompactActions
  )
    return;
  compactActionOffset.value =
    editorInitialTop - editorRoot.value.getBoundingClientRect().top;
}

function enterEdit() {
  originalValue.value = props.modelValue;
  originalMode.value = editorMode.value;
  editing.value = true;
  emit("editing-change", true);
  focusActiveEditor("end");
}

function handleCancel() {
  markdownEditor.value?.resetFromHtml(originalValue.value);
  syncEditorFromHtml(originalValue.value);
  editorMode.value = originalMode.value || richTextMode;
  emit("update:modelValue", originalValue.value);
  emit("update:contentMode", editorMode.value);
  emit("cancel", originalValue.value);
  editing.value = false;
  emit("editing-change", false);
  exitFullscreen();
}

function handleSave() {
  flushEditorValue();
  emit("save");
  editing.value = false;
  emit("editing-change", false);
  exitFullscreen();
}

function markSaved() {
  originalValue.value = props.modelValue;
  originalMode.value = editorMode.value;
}

function toggleFullscreen() {
  if (isFullscreen.value) {
    exitFullscreen();
    return;
  }
  fullscreenPreview.value = false;
  isFullscreen.value = true;
  focusActiveEditor();
}

function enterFullscreenPreview() {
  fullscreenPreview.value = true;
  isFullscreen.value = true;
}

function exitFullscreen() {
  isFullscreen.value = false;
  fullscreenPreview.value = false;
}

function enterFullscreenEdit() {
  fullscreenPreview.value = false;
  if (!editing.value) enterEdit();
  isFullscreen.value = true;
  nextTick(() => focusActiveEditor("end"));
}

function flushAndSaveStay() {
  if (!editing.value) return;
  flushEditorValue();
  emit("save");
}

defineExpose({
  enterEdit,
  enterFullscreenPreview,
  enterFullscreenEdit,
  markSaved,
  cancelEdit: handleCancel,
  flushAndSaveStay,
});

const saveHotkeyHint = computed(() =>
  /Mac|iPhone|iPad|iPod/.test(navigator.platform || navigator.userAgent)
    ? $t("editor_cmd_s_save")
    : $t("editor_ctrl_s_save"),
);

function handleKeydown(event) {
  if (event.key === "Escape") {
    if (formatPainterActive.value) cancelFormatPainter();
    if (isFullscreen.value) exitFullscreen();
  }
}

function tooltipContainer(trigger) {
  return trigger.parentElement;
}

function editorPopupContainer() {
  return editorRoot.value || document.body;
}

watch(canEdit, (value) => {
  editor.value?.setEditable(value);
  if (!value) syncEditorFromHtml(props.modelValue);
});
watch(
  () => props.modelValue,
  (value) => {
    // 正文被替换时关闭仍在展示的预览，避免残留旧需求/旧评论的图片。
    closeImageViewer();
    if (!isMarkdownEditing.value) syncEditorFromHtml(value);
  },
);
watch(
  () => props.contentRevision,
  () => {
    if (!isMarkdownEditing.value) return;
    nextTick(() => markdownEditor.value?.resetFromHtml(props.modelValue));
  },
);
watch(
  () => props.contentMode,
  (value) => {
    const mode = normalizeEditorMode(value);
    if (!props.allowMarkdown && mode !== richTextMode) {
      emit("update:contentMode", richTextMode);
      return;
    }
    if (!mode || mode === editorMode.value) return;
    editorMode.value = mode;
    if (mode === richTextMode) {
      syncEditorFromHtml(props.modelValue);
      return;
    }
    if (isMarkdownEditing.value) {
      nextTick(() => markdownEditor.value?.resetFromHtml(props.modelValue));
    }
  },
);
watch(isFullscreen, (value) => {
  if (value) {
    previousBodyOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
  } else document.body.style.overflow = previousBodyOverflow;
});

onMounted(() => {
  if (
    !normalizeEditorMode(props.contentMode) ||
    (!props.allowMarkdown && props.contentMode !== richTextMode)
  ) {
    emit("update:contentMode", editorMode.value);
  }
  editorInitialTop = editorRoot.value?.getBoundingClientRect().top || 0;
  document.addEventListener("scroll", syncCompactActionPosition, true);
  window.addEventListener("scroll", syncCompactActionPosition);
  window.addEventListener("keydown", handleKeydown, true);
});
onBeforeUnmount(() => {
  document.removeEventListener("scroll", syncCompactActionPosition, true);
  window.removeEventListener("scroll", syncCompactActionPosition);
  window.removeEventListener("keydown", handleKeydown, true);
  if (isFullscreen.value) document.body.style.overflow = previousBodyOverflow;
});
</script>

<style scoped lang="less">
.rich-text-editor {
  position: relative;
}
.mention-suggest {
  position: absolute;
  z-index: 1200;
  min-width: 140px;
  max-width: 240px;
  max-height: 220px;
  padding: 4px;
  overflow-y: auto;
  border: 1px solid #e5e8ef;
  border-radius: 6px;
  background: #fff;
  box-shadow: 0 4px 14px rgba(23, 35, 60, 0.12);
}
.mention-suggest__item {
  display: block;
  width: 100%;
  padding: 5px 8px;
  overflow: hidden;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: #262626;
  cursor: pointer;
  font-size: 13px;
  line-height: 20px;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.mention-suggest__item.active,
.mention-suggest__item:hover {
  background: #f0f5ff;
  color: #2475fc;
}
.mention-suggest__empty {
  padding: 5px 8px;
  color: #8c8c8c;
  font-size: 13px;
  line-height: 20px;
  white-space: nowrap;
}
.rich-text-editor :deep(.mention-tag) {
  padding: 0 4px;
  border-radius: 4px;
  background: #e8f1ff;
  color: #2475fc;
  white-space: nowrap;
}
.editor-actions {
  display: flex;
  margin-bottom: 8px;
}
.editor-edit-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  width: 100%;
}
.editor-edit-actions-main {
  display: flex;
  align-items: center;
  gap: 8px;
}
.editor-save-hint {
  color: #8c95a8;
  font-size: 12px;
  line-height: 32px;
  white-space: nowrap;
}
.editor-edit-actions-extra {
  min-width: 0;
}
.fullscreen-preview-actions {
  position: absolute;
  top: 18px;
  right: 18px;
  z-index: 4;
  display: flex;
  align-items: center;
  gap: 2px;
  min-height: 36px;
  padding: 2px;
  border: 1px solid #e6eaf1;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 4px 12px rgba(23, 35, 60, 0.12);
}
.fullscreen-preview-edit-button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 30px;
  padding: 0 10px;
  color: #17233c;
}
.fullscreen-preview-exit-button {
  width: 30px;
  min-width: 30px;
  height: 30px;
  padding: 0;
  color: #657188;
}
.editor-actions.compact {
  position: absolute;
  top: 4px;
  right: 0;
  z-index: 2;
  margin: 0;
}
.editor-shell {
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  border: 1px solid #d9d9d9;
  border-radius: 6px;
  background: #fff;
}
.editor-toolbar {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 2px;
  min-height: 40px;
  padding: 4px 8px;
  border-bottom: 1px solid #f0f0f0;
  background: #fafafa;
}
.toolbar-spacer {
  flex: 1;
}
.mode-switch-button,
.fullscreen-button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex: 0 0 auto;
  color: #595959;
}
.mode-switch-button {
  padding: 0 8px;
}
.mode-confirm-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.mode-confirm-title :deep(.anticon) {
  color: #faad14;
}
.mode-confirm-message {
  margin: 10px 0 4px 28px;
  color: #334155;
  line-height: 1.7;
}
.image-input {
  display: none;
}
.dialog-input-gap {
  margin-top: 8px;
}
.rich-text-editor:not(.is-editable) :deep(.rich-text-content img) {
  cursor: zoom-in;
}
.editor-body {
  min-height: var(--editor-min-height);
  overflow: auto;
}
.empty-preview {
  min-height: var(--editor-min-height);
  padding: 16px;
  color: #bfbfbf;
  background: #fafafa;
}
.is-fill:not(.is-fullscreen) {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
}
.is-fill:not(.is-fullscreen) .editor-shell,
.is-fill:not(.is-fullscreen) .editor-body {
  flex: 1;
}
:deep(.rich-text-content) {
  min-height: var(--editor-min-height);
  padding: 16px;
  outline: none;
  color: #333;
  font-size: 14px;
  line-height: 1.7;
  overflow-wrap: anywhere;
}
:deep(.rich-text-content p) {
  margin: 0 0 8px;
}
:deep(.rich-text-content p:last-child) {
  margin-bottom: 0;
}
:deep(.rich-text-content ol),
:deep(.rich-text-content ul) {
  margin: 8px 0;
  padding-left: 24px;
}
:deep(.rich-text-content h1),
:deep(.rich-text-content h2),
:deep(.rich-text-content h3),
:deep(.rich-text-content h4),
:deep(.rich-text-content h5),
:deep(.rich-text-content h6) {
  margin: 16px 0 8px;
  color: #262626;
  font-weight: 600;
  line-height: 1.35;
}
:deep(.rich-text-content h1) {
  font-size: 24px;
}
:deep(.rich-text-content h2) {
  font-size: 20px;
}
:deep(.rich-text-content h3) {
  font-size: 17px;
}
:deep(.rich-text-content h4) {
  font-size: 16px;
}
:deep(.rich-text-content h5) {
  font-size: 14px;
}
:deep(.rich-text-content h6) {
  font-size: 13px;
}
:deep(.rich-text-content .tableWrapper) {
  margin: -4px 0 4px -16px;
  padding: 16px 0 8px 16px;
  overflow-x: auto;
}
:deep(.rich-text-content table) {
  width: 100%;
  margin: 0;
  overflow: visible;
  border-collapse: collapse;
  table-layout: fixed;
}
:deep(.rich-text-content th),
:deep(.rich-text-content td) {
  position: relative;
  box-sizing: border-box;
  min-width: 60px;
  padding: 8px 10px;
  border: 1px solid #d9d9d9;
  text-align: left;
  vertical-align: top;
}
:deep(.rich-text-content th) {
  background: #fafafa;
  font-weight: 600;
}
:deep(.rich-text-content th > p:last-of-type),
:deep(.rich-text-content td > p:last-of-type) {
  margin-bottom: 0;
}
:deep(.rich-text-content .column-resize-handle) {
  position: absolute;
  z-index: 20;
  top: 0;
  right: -2px;
  bottom: 0;
  width: 4px;
  background: #91caff;
  pointer-events: none;
}
:deep(.rich-text-content.resize-cursor) {
  cursor: col-resize;
}
:deep(.rich-text-content .table-insert-btn) {
  position: absolute;
  z-index: 25;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  padding: 0;
  border: 1px solid #91caff;
  border-radius: 4px;
  background: #fff;
  color: #2475fc;
  cursor: pointer;
  font-size: 14px;
  font-weight: 600;
  line-height: 1;
  user-select: none;
}
:deep(.rich-text-content .table-insert-btn:hover) {
  background: #e8f1ff;
}
/* Above a hovered first-row cell, on its right cell boundary. */
:deep(.rich-text-content .table-insert-btn--column) {
  top: -16px;
  left: 100%;
  transform: translateX(-50%);
}
/* Left of a hovered first-column cell, on its bottom cell boundary. */
:deep(.rich-text-content .table-insert-btn--row) {
  left: -8px;
  top: 100%;
  transform: translateY(-50%);
}
.is-editable :deep(.rich-text-content a) {
  cursor: text;
}
:deep(.rich-text-content a) {
  display: inline-block;
  max-width: 100%;
  color: #1677ff;
  text-decoration: underline;
  overflow-wrap: anywhere;
}
:deep(.rich-text-content img) {
  display: inline-block;
  max-width: 100%;
  height: auto;
  margin: 4px 0;
  vertical-align: bottom;
}
:deep(.rich-text-content .editor-file-node) {
  display: flex;
  align-items: center;
  width: min(100%, 450px);
  height: 60px;
  margin: 8px 0;
  overflow: hidden;
  border: 1px solid #d7dee9;
  border-radius: 4px;
  background: #fff;
  color: #17233c;
  user-select: none;
}
.is-editable :deep(.rich-text-content .editor-file-node) {
  cursor: pointer;
}
:deep(.rich-text-content .editor-file-node.ProseMirror-selectednode) {
  border-color: #91caff;
  outline: 2px solid rgba(22, 119, 255, 0.12);
}
:deep(.rich-text-content .editor-file-type-icon) {
  width: 28px;
  height: 32px;
  margin-left: 12px;
  flex: 0 0 28px;
  background: url("../../image/file-icon/file.png") center / contain no-repeat;
}
:deep(
  .rich-text-content
    .editor-file-node[data-file-kind="pdf"]
    .editor-file-type-icon
) {
  background-image: url("../../image/file-icon/pdf.png");
}
:deep(
  .rich-text-content
    .editor-file-node[data-file-kind="image"]
    .editor-file-type-icon
) {
  background-image: url("../../image/file-icon/png.png");
}
:deep(
  .rich-text-content
    .editor-file-node[data-file-kind="presentation"]
    .editor-file-type-icon
) {
  background-image: url("../../image/file-icon/ppt.png");
}
:deep(
  .rich-text-content
    .editor-file-node[data-file-kind="video"]
    .editor-file-type-icon
) {
  background-image: url("../../image/file-icon/mp4.png");
}
:deep(
  .rich-text-content
    .editor-file-node[data-file-kind="archive"]
    .editor-file-type-icon
) {
  background-image: url("../../image/file-icon/zip.png");
}
:deep(.rich-text-content .editor-file-meta) {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  justify-content: center;
  margin: 0 12px;
  line-height: 1.35;
}
:deep(.rich-text-content .editor-file-name) {
  overflow: hidden;
  color: #17233c;
  font-size: 14px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
:deep(.rich-text-content .editor-file-size) {
  margin-top: 2px;
  color: #a3afc2;
  font-size: 12px;
  white-space: nowrap;
}
:deep(.rich-text-content .editor-file-download) {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 52px;
  height: 36px;
  flex: 0 0 52px;
  border-left: 1px solid #e5eaf1;
  color: inherit;
  text-decoration: none;
}
:deep(.rich-text-content .editor-file-download-icon) {
  width: 18px;
  height: 18px;
  background: url("../../image/file-icon/uploader.png") center / contain
    no-repeat;
}
:deep(.rich-text-content .editor-video-node) {
  width: min(100%, 720px);
  margin: 16px 0;
  overflow: hidden;
  border: 1px solid #e6eaf1;
  border-radius: 4px;
  background: #101828;
  aspect-ratio: 16 / 9;
}
:deep(.rich-text-content .editor-video-node video),
:deep(.rich-text-content .editor-video-node iframe) {
  display: block;
  width: 100%;
  height: 100%;
  border: 0;
}
.rendered-rich-text-content :deep(h1),
.rendered-rich-text-content :deep(h2),
.rendered-rich-text-content :deep(h3),
.rendered-rich-text-content :deep(h4),
.rendered-rich-text-content :deep(h5),
.rendered-rich-text-content :deep(h6) {
  margin: 16px 0 8px;
  color: #262626;
  font-weight: 600;
  line-height: 1.35;
}
.rendered-rich-text-content :deep(h1) {
  font-size: 24px;
}
.rendered-rich-text-content :deep(h2) {
  font-size: 20px;
}
.rendered-rich-text-content :deep(h3) {
  font-size: 17px;
}
.rendered-rich-text-content :deep(h4) {
  font-size: 16px;
}
.rendered-rich-text-content :deep(h5) {
  font-size: 14px;
}
.rendered-rich-text-content :deep(h6) {
  font-size: 13px;
}
.rendered-rich-text-content :deep(blockquote) {
  margin: 12px 0;
  padding: 4px 12px;
  border-left: 3px solid #d9d9d9;
  color: #666;
  background: #fafafa;
}
.rendered-rich-text-content :deep(pre) {
  margin: 12px 0;
  padding: 12px;
  overflow: auto;
  border-radius: 4px;
  background: #f5f5f5;
  font-family: Consolas, "Courier New", monospace;
  white-space: pre;
}
.rendered-rich-text-content :deep(code) {
  padding: 2px 4px;
  border-radius: 3px;
  background: #f5f5f5;
  font-family: Consolas, "Courier New", monospace;
  font-size: 0.92em;
}
.rendered-rich-text-content :deep(pre code) {
  padding: 0;
  background: transparent;
}
:deep(.rich-text-content .editor-code-block-node) {
  box-sizing: border-box;
  width: 100%;
  margin: 12px 0;
  overflow: hidden;
  border: 1px solid #d7dee9;
  border-radius: 4px;
  background: #fbfcfe;
  color: #334155;
  cursor: default;
  user-select: text;
}
:deep(.rich-text-content .editor-code-block-node.ProseMirror-selectednode) {
  border-color: #69b1ff;
  outline: 2px solid rgba(22, 119, 255, 0.18);
}
:deep(.rich-text-content .editor-code-block-node[data-code-theme="light"]) {
  background: #fff;
  color: #17233c;
}
:deep(.rich-text-content .editor-code-block-node[data-code-theme="dark"]) {
  border-color: #424242;
  background: #292929;
  color: #e6e6e6;
}
:deep(.rich-text-content .editor-code-block-node pre) {
  min-height: 88px;
  margin: 0;
  padding: 16px 20px;
  overflow: auto;
  border-radius: 0;
  background: transparent;
  color: inherit;
  font-family: Consolas, "Courier New", monospace;
  font-size: 13px;
  line-height: 1.65;
  tab-size: 2;
  white-space: pre;
}
:deep(.rich-text-content .editor-code-block-node code) {
  display: block;
  padding: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  white-space: inherit;
}
:deep(.rich-text-content .editor-code-block-node .hljs-comment),
:deep(.rich-text-content .editor-code-block-node .hljs-quote) {
  color: #8a9199;
}
:deep(.rich-text-content .editor-code-block-node .hljs-keyword),
:deep(.rich-text-content .editor-code-block-node .hljs-selector-tag),
:deep(.rich-text-content .editor-code-block-node .hljs-meta) {
  color: #a626a4;
}
:deep(.rich-text-content .editor-code-block-node .hljs-string),
:deep(.rich-text-content .editor-code-block-node .hljs-regexp),
:deep(.rich-text-content .editor-code-block-node .hljs-addition) {
  color: #50a14f;
}
:deep(.rich-text-content .editor-code-block-node .hljs-title),
:deep(.rich-text-content .editor-code-block-node .hljs-section),
:deep(.rich-text-content .editor-code-block-node .hljs-function) {
  color: #c18401;
}
:deep(.rich-text-content .editor-code-block-node .hljs-number),
:deep(.rich-text-content .editor-code-block-node .hljs-literal) {
  color: #986801;
}
:deep(
  .rich-text-content
    .editor-code-block-node[data-code-theme="dark"]
    .hljs-comment
),
:deep(
  .rich-text-content .editor-code-block-node[data-code-theme="dark"] .hljs-quote
) {
  color: #7f8c98;
}
:deep(
  .rich-text-content
    .editor-code-block-node[data-code-theme="dark"]
    .hljs-keyword
),
:deep(
  .rich-text-content
    .editor-code-block-node[data-code-theme="dark"]
    .hljs-selector-tag
),
:deep(
  .rich-text-content .editor-code-block-node[data-code-theme="dark"] .hljs-meta
) {
  color: #ff7ab2;
}
:deep(
  .rich-text-content
    .editor-code-block-node[data-code-theme="dark"]
    .hljs-string
),
:deep(
  .rich-text-content
    .editor-code-block-node[data-code-theme="dark"]
    .hljs-regexp
),
:deep(
  .rich-text-content
    .editor-code-block-node[data-code-theme="dark"]
    .hljs-addition
) {
  color: #a8cc8c;
}
:deep(
  .rich-text-content .editor-code-block-node[data-code-theme="dark"] .hljs-title
),
:deep(
  .rich-text-content
    .editor-code-block-node[data-code-theme="dark"]
    .hljs-section
),
:deep(
  .rich-text-content
    .editor-code-block-node[data-code-theme="dark"]
    .hljs-function
) {
  color: #f2cf66;
}
:deep(
  .rich-text-content
    .editor-code-block-node[data-code-theme="dark"]
    .hljs-number
),
:deep(
  .rich-text-content
    .editor-code-block-node[data-code-theme="dark"]
    .hljs-literal
) {
  color: #d4bfff;
}
:deep(
  .rich-text-content
    .editor-code-block-node[data-code-theme="dark"]
    .hljs-built_in
),
:deep(
  .rich-text-content .editor-code-block-node[data-code-theme="dark"] .hljs-type
),
:deep(
  .rich-text-content .editor-code-block-node[data-code-theme="dark"] .hljs-attr
) {
  color: #82aaff;
}
.rendered-rich-text-content :deep(table) {
  width: 100%;
  margin: 12px 0;
  border-collapse: collapse;
}
.rendered-rich-text-content :deep(th),
.rendered-rich-text-content :deep(td) {
  padding: 8px 10px;
  border: 1px solid #d9d9d9;
  text-align: left;
}
.rendered-rich-text-content :deep(th) {
  background: #fafafa;
  font-weight: 600;
}
:deep([data-resize-container]) {
  max-width: 100%;
  margin: 4px 0;
  padding-right: 1px;
  vertical-align: bottom;
}
:deep([data-resize-wrapper] > img) {
  margin: 0;
}
:deep([data-resize-container].ProseMirror-selectednode) {
  outline: 2px solid #91caff;
  outline-offset: 0;
}
:deep([data-resize-handle]) {
  z-index: 1;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.15s ease;
}
.is-editable
  :deep([data-resize-container].ProseMirror-selectednode [data-resize-handle]),
.is-editable :deep([data-resize-state="true"] [data-resize-handle]) {
  opacity: 1;
  pointer-events: auto;
}
:deep([data-resize-handle="top-left"]),
:deep([data-resize-handle="top-right"]),
:deep([data-resize-handle="bottom-left"]),
:deep([data-resize-handle="bottom-right"]) {
  width: 10px;
  height: 10px;
  background: #4096ff;
}
:deep([data-resize-handle="top-left"]),
:deep([data-resize-handle="bottom-right"]) {
  cursor: nwse-resize;
}
:deep([data-resize-handle="top-right"]),
:deep([data-resize-handle="bottom-left"]) {
  cursor: nesw-resize;
}
:deep([data-resize-handle="top-left"]) {
  transform: translate(-50%, -50%);
}
:deep([data-resize-handle="top-right"]) {
  transform: translate(50%, -50%);
}
:deep([data-resize-handle="bottom-left"]) {
  transform: translate(-50%, 50%);
}
:deep([data-resize-handle="bottom-right"]) {
  transform: translate(50%, 50%);
}
.is-fullscreen {
  position: fixed;
  z-index: 1200;
  inset: 0;
  display: flex;
  flex-direction: column;
  padding: 16px 16px 80px;
  background: #fff;
  padding-top: 0;
}
.is-fullscreen.is-fullscreen-preview {
  padding: 0;
  background: #fff;
}
.is-fullscreen-preview .editor-shell {
  border: 0;
  border-radius: 0;
}
.is-fullscreen-preview :deep(.rich-text-content) {
  box-sizing: border-box;
  min-height: 100%;
  padding: 64px 48px 32px;
}
.is-fullscreen-preview .empty-preview {
  box-sizing: border-box;
  flex: 1;
  min-height: 0;
  padding: 64px 48px 32px;
}
.is-fullscreen .editor-actions {
  position: absolute;
  bottom: 0;
  left: 0;
  z-index: 2;
  box-sizing: border-box;
  align-items: center;
  width: 100%;
  min-height: 64px;
  margin: 0;
  padding: 12px 16px;
  border-top: 1px solid #f0f0f0;
  background: #fff;
  box-shadow: 0 -10px 12px rgba(23, 35, 60, 0.04);
}
.is-fullscreen .editor-shell,
.is-fullscreen .editor-body {
  flex: 1;
  min-height: 0;
}
.is-fullscreen :deep(.rich-text-content) {
  min-height: calc(100vh - 90px);
}
.is-fullscreen.is-fullscreen-preview :deep(.rich-text-content) {
  min-height: 100%;
}
@media (max-width: 640px) {
  .editor-toolbar {
    flex-wrap: wrap;
  }
  .toolbar-spacer {
    display: none;
  }
  .mode-switch-button span,
  .fullscreen-button span {
    display: none;
  }
  .is-fullscreen {
    padding: 8px 8px 72px;
  }
  .is-fullscreen.is-fullscreen-preview {
    padding: 0;
  }
  .fullscreen-preview-actions {
    top: 10px;
    right: 10px;
  }
  .is-fullscreen-preview :deep(.rich-text-content),
  .is-fullscreen-preview .empty-preview {
    padding: 56px 16px 24px;
  }
  .is-fullscreen .editor-actions {
    padding-inline: 8px;
  }
}
</style>

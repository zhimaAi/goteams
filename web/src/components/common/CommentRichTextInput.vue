<template>
  <div
    class="comment-rich-text-input"
    :class="{
      expanded: isExpanded,
      focused,
      'full-border': fullBorder,
    }"
    @pointerdown.capture="handlePointerDown"
    @focusin="handleFocusIn"
    @focusout="handleFocusOut"
    @keydown="handleKeydown"
  >
    <RichTextEditor
      v-model="localValue"
      v-model:image-file-ids="localImageFileIds"
      class="comment-editor"
      :ws-id="wsId"
      content-mode="rich-text"
      plain
      :min-height="isExpanded ? minHeight : collapsedMinHeight"
      :placeholder="placeholder"
      :mention-enabled="mentionEnabled"
      :mention-members="mentionMembers"
    />
  </div>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import RichTextEditor from "@/components/common/RichTextEditor.vue";

const props = defineProps({
  modelValue: { type: String, default: "" },
  wsId: { type: [String, Number], default: 0 },
  placeholder: { type: String, default: "" },
  imageFileIds: { type: Array, default: () => [] },
  minHeight: { type: Number, default: 108 },
  collapsedMinHeight: { type: Number, default: 30 },
  alwaysExpanded: { type: Boolean, default: false },
  expanded: { type: Boolean, default: undefined },
  fullBorder: { type: Boolean, default: false },
  // 输入 @ 触发成员搜索；候选由父组件给出（项目成员 + 邀填人）。
  mentionEnabled: { type: Boolean, default: false },
  mentionMembers: { type: Array, default: () => [] },
});
const emit = defineEmits([
  "update:modelValue",
  "update:imageFileIds",
  "focus-change",
  "submit",
]);

const localValue = ref(props.modelValue);
const localImageFileIds = ref([...props.imageFileIds]);
const focused = ref(false);
let pointerDownAt = 0;

const isExpanded = computed(
  () =>
    props.alwaysExpanded ||
    (props.expanded === undefined ? focused.value : props.expanded),
);

watch(
  () => props.modelValue,
  (value) => {
    if (value !== localValue.value) localValue.value = value;
  },
);
watch(
  () => props.imageFileIds,
  (value) => {
    if (value !== localImageFileIds.value)
      localImageFileIds.value = [...(value || [])];
  },
  { deep: true },
);
watch(
  () => props.expanded,
  (value) => {
    if (value !== undefined && value !== focused.value) focused.value = value;
  },
);
watch(localValue, (value) => emit("update:modelValue", value));
watch(localImageFileIds, (value) => emit("update:imageFileIds", value), {
  deep: true,
});

function handlePointerDown() {
  pointerDownAt = performance.now();
}
function handleFocusIn() {
  if (!focused.value) {
    focused.value = true;
    emit("focus-change", true);
  }
}
function handleFocusOut(event) {
  const container = event.currentTarget;
  if (
    container.contains(event.relatedTarget) ||
    event.relatedTarget?.closest?.(".comment-input") ||
    performance.now() - pointerDownAt < 250
  )
    return;
  window.requestAnimationFrame(() => {
    if (container.contains(document.activeElement)) return;
    focused.value = false;
    emit("focus-change", false);
  });
}
function handleKeydown(event) {
  if (
    event.key !== "Enter" ||
    (!event.ctrlKey && !event.metaKey) ||
    event.altKey ||
    event.shiftKey
  )
    return;
  event.preventDefault();
  emit("submit");
}
</script>

<style scoped lang="less">
.comment-rich-text-input {
  width: 100%;
}

.comment-editor :deep(.editor-shell) {
  box-sizing: border-box;
  height: 32px;
  border: 1px solid #dfe4ec;
  border-radius: 4px;
  box-shadow: none;
}

.comment-editor :deep(.editor-toolbar) {
  display: none;
  min-height: 40px;
  padding: 4px 8px;
  background: #fff;
}

.comment-editor :deep(.mode-switch-button),
.comment-editor :deep(.fullscreen-button) {
  display: none;
}

.comment-editor :deep(.editor-body) {
  height: 30px;
  overflow: hidden;
}

.comment-editor :deep(.rich-text-content) {
  box-sizing: border-box;
  height: 30px;
  max-height: 30px;
  padding: 4px 10px;
  overflow: hidden;
  font-size: 12px;
  line-height: 22px;
}

.comment-editor
  :deep(.rich-text-content p.is-editor-empty:first-child::before) {
  float: left;
  height: 0;
  color: #b7c0cf;
  content: attr(data-placeholder);
  pointer-events: none;
}

.comment-rich-text-input.expanded .comment-editor :deep(.editor-shell) {
  height: auto;
  overflow: visible;
  border-radius: 8px 8px 0 0;
}

.comment-rich-text-input.expanded:not(.full-border)
  .comment-editor
  :deep(.editor-shell) {
  border-bottom: 0;
  border-radius: 8px 8px 0 0;
}

.comment-rich-text-input.expanded.full-border
  .comment-editor
  :deep(.editor-shell) {
  border-bottom: 1px solid #dfe4ec;
  border-radius: 6px;
}

.comment-rich-text-input.expanded .comment-editor :deep(.editor-toolbar) {
  display: flex;
  overflow: visible;
  border-radius: 7px 7px 0 0;
}

.comment-rich-text-input.expanded .comment-editor :deep(.ant-dropdown),
.comment-rich-text-input.expanded
  .comment-editor
  :deep(.ant-menu-submenu-popup) {
  z-index: 1100;
}

.comment-rich-text-input.expanded .comment-editor :deep(.editor-body) {
  height: auto;
  max-height: 120px;
  overflow: auto;
}

.comment-rich-text-input.expanded .comment-editor :deep(.rich-text-content) {
  height: auto;
  max-height: none;
  padding: 10px 14px;
  font-size: 14px;
  line-height: 1.7;
}

.comment-rich-text-input.expanded.focused .comment-editor :deep(.editor-shell) {
  border-color: #2475fc;
}

.comment-rich-text-input.expanded .comment-editor :deep(.editor-toolbar) {
  display: flex;
}
</style>

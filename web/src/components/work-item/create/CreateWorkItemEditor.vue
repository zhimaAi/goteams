<template>
  <div class="create-left">
    <div class="title-field">
      <input
        :value="title"
        class="big-title-input"
        :class="{ invalid: titleInvalid }"
        :placeholder="$t('requirement_titlePlaceholder')"
        maxlength="200"
        @input="$emit('update:title', $event.target.value)"
        @blur="$emit('touch-title')"
      />
      <span class="title-count">{{ title.length }} / 200</span>
    </div>
    <RichTextEditor
      :model-value="description"
      :image-file-ids="imageFileIds"
      :content-mode="descriptionMode"
      :content-revision="contentRevision"
      :ws-id="wsId"
      class="create-editor"
      :placeholder="$t('requirement_descriptionPlaceholder')"
      :min-height="420"
      fill
      plain
      @update:model-value="$emit('update:description', $event)"
      @update:content-mode="$emit('update:descriptionMode', $event)"
      @update:image-file-ids="$emit('update:imageFileIds', $event)"
    />
  </div>
</template>

<script setup>
import RichTextEditor from "@/components/common/RichTextEditor.vue";

defineProps({
  title: { type: String, default: "" },
  titleInvalid: { type: Boolean, default: false },
  description: { type: String, default: "" },
  imageFileIds: { type: Array, default: () => [] },
  descriptionMode: { type: String, default: "" },
  contentRevision: { type: Number, default: 0 },
  wsId: { type: String, default: "" },
  imageFileIds: { type: Array, default: () => [] },
});
defineEmits([
  "update:title",
  "update:description",
  "update:imageFileIds",
  "update:descriptionMode",
  "update:imageFileIds",
  "touch-title",
]);
</script>

<style scoped lang="less">
.create-left {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  padding: 0 24px 36px;
  overflow: auto;
  border-right: 1px solid #e6eaf0;
}
.title-field {
  position: relative;
  flex-shrink: 0;
  padding-top: 16px;
}
.big-title-input {
  width: 100%;
  height: 51px;
  padding: 4px 48px 10px 2px;
  border: 0;
  border-bottom: 2px solid #2475fc;
  border-radius: 0;
  outline: none;
  color: var(--app-text);
  background: transparent;
  font-size: 18px;
  font-weight: 500;
}
.big-title-input::placeholder {
  color: #b6becb;
}
.big-title-input.invalid {
  border-bottom-color: #ff4d4f;
}
.title-count {
  position: absolute;
  right: 2px;
  bottom: 10px;
  color: #a5adba;
  font-size: 10px;
  white-space: nowrap;
}
.create-editor {
  flex: 1;
  min-height: 0;
  padding-top: 8px;

  :deep(.editor-shell) {
    border: 0;
    border-radius: 0;
  }
  :deep(.editor-toolbar) {
    min-height: 38px;
    padding: 4px;
    border-bottom-color: #e4e6eb;
    background: var(--app-surface);
  }
  :deep(.rich-text-content) {
    padding: 12px;
    font-size: 14px;
  }
}
@media (max-width: 900px) {
  .create-left {
    min-height: 520px;
    padding: 16px;
    border-right: 0;
    border-bottom: 1px solid #e6eaf0;
  }
}
</style>

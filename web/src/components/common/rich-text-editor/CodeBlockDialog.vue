<template>
  <a-modal
    :open="open"
    :title="$t(editing ? 'editor_editCodeBlock' : 'editor_insertCodeBlock')"
    :width="900"
    centered
    :ok-text="$t('common_confirm')"
    :cancel-text="$t('common_cancel')"
    :z-index="1400"
    @update:open="$emit('update:open', $event)"
    @ok="confirm"
    @cancel="close"
  >
    <div class="code-block-dialog-form">
      <label>
        <span>{{ $t("editor_codeBlockTheme") }}</span>
        <a-select
          v-model:value="theme"
          :options="themeOptions"
          :get-popup-container="getSelectPopupContainer"
        />
      </label>

      <!-- <label>
        <span>{{ $t("editor_programmingLanguage") }}</span>
        <a-select
          :get-popup-container="getSelectPopupContainer"
          v-model:value="language"
          :options="languageOptions"
          show-search
        />
      </label> -->

      <label>
        <span>{{ $t("editor_codeInput") }}</span>
        <a-textarea
          v-model:value="source"
          class="code-block-source-input"
          :rows="16"
          :placeholder="$t('editor_codePlaceholder')"
        />
      </label>
    </div>
  </a-modal>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { message } from "ant-design-vue";
import { useI18n } from "vue-i18n";
import {
  normalizeCodeBlockLanguage,
  normalizeCodeBlockTheme,
} from "./richTextNodes";

const props = defineProps({
  open: { type: Boolean, required: true },
  editing: { type: Boolean, default: false },
  value: {
    type: Object,
    default: () => ({ theme: "default", language: "javascript", code: "" }),
  },
});
const emit = defineEmits(["update:open", "confirm"]);
const { t: $t } = useI18n();
const theme = ref("default");
const language = ref("javascript");
const source = ref("");
const themeOptions = computed(() => [
  { value: "default", label: $t("editor_codeThemeDefault") },
  { value: "light", label: $t("editor_codeThemeLight") },
  { value: "dark", label: $t("editor_codeThemeDark") },
]);

function resetForm() {
  theme.value = normalizeCodeBlockTheme(props.value.theme || "default");
  language.value = normalizeCodeBlockLanguage(
    props.value.language || "javascript",
  );
  source.value = props.value.code || "";
}

function getSelectPopupContainer(trigger) {
  return trigger.parentElement || document.body;
}

function close() {
  emit("update:open", false);
}

function confirm() {
  if (!source.value.trim()) {
    message.warning($t("editor_codeRequired"));
    return;
  }
  emit("confirm", {
    theme: normalizeCodeBlockTheme(theme.value),
    language: normalizeCodeBlockLanguage(language.value),
    code: source.value,
  });
}

watch(
  () => props.open,
  (open) => {
    if (open) resetForm();
  },
);
</script>

<style scoped lang="less">
.code-block-dialog-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.code-block-dialog-form label {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 6px;
  color: #334155;
}
.code-block-source-input {
  min-height: 320px;
  font-family: Consolas, "Courier New", monospace;
  line-height: 1.6;
  resize: vertical;
  tab-size: 2;
}
</style>

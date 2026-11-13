<template>
  <a-modal
    :open="open"
    :title="$t('editor_link')"
    :width="500"
    centered
    :ok-text="$t('common_confirm')"
    :cancel-text="$t('common_cancel')"
    :z-index="1400"
    @update:open="$emit('update:open', $event)"
    @ok="confirm"
    @cancel="close"
  >
    <div class="link-dialog-form">
      <label>
        <span>{{ $t("editor_linkAddress") }}</span>
        <a-input
          ref="addressInput"
          v-model:value="address"
          :placeholder="$t('editor_linkAddressPlaceholder')"
          @press-enter="confirm"
        />
      </label>

      <label>
        <span>{{ $t("editor_linkText") }}</span>
        <a-input
          v-model:value="text"
          :placeholder="$t('editor_linkTextPlaceholder')"
          @press-enter="confirm"
        />
      </label>

      <label>
        <span>{{ $t("editor_linkOpenMode") }}</span>
        <a-select
          v-model:value="target"
          :options="targetOptions"
          :get-popup-container="getSelectPopupContainer"
        />
      </label>
    </div>
  </a-modal>
</template>

<script setup>
import { computed, nextTick, ref, watch } from "vue";
import { message } from "ant-design-vue";
import { useI18n } from "vue-i18n";

const props = defineProps({
  open: { type: Boolean, required: true },
  value: {
    type: Object,
    default: () => ({ href: "", text: "", target: "_blank" }),
  },
});
const emit = defineEmits(["update:open", "confirm"]);
const { t: $t } = useI18n();
const addressInput = ref(null);
const address = ref("");
const text = ref("");
const target = ref("_blank");
const targetOptions = computed(() => [
  { value: "_blank", label: $t("editor_linkTargetNewWindow") },
  { value: "_self", label: $t("editor_linkTargetCurrentWindow") },
]);

function normalizeLink(value) {
  const trimmed = value.trim();
  if (!trimmed) return "";
  if (/^[\w.-]+\.[a-z]{2,}(?:\/|$)/i.test(trimmed)) return `https://${trimmed}`;
  if (/^(https?:\/\/|mailto:|tel:|\/|#)/i.test(trimmed)) return trimmed;
  return null;
}

function resetForm() {
  address.value = props.value.href || "";
  text.value = props.value.text || "";
  target.value = props.value.target === "_self" ? "_self" : "_blank";
  nextTick(() => addressInput.value?.focus());
}

function getSelectPopupContainer(trigger) {
  return trigger.parentElement || document.body;
}

function close() {
  emit("update:open", false);
}

function confirm() {
  const href = normalizeLink(address.value);
  if (href === null || (!href && !props.value.href)) {
    message.warning($t("editor_linkInvalid"));
    return;
  }
  emit("confirm", {
    href,
    text: text.value.trim(),
    target: target.value,
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
.link-dialog-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 4px 18px 8px;
}
.link-dialog-form label {
  display: flex;
  flex-direction: column;
  gap: 7px;
  color: #334155;
}
</style>

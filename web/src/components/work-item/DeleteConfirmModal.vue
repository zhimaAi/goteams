<template>
  <a-modal
    :open="open"
    :title="title"
    :confirm-loading="confirmLoading"
    :ok-text="okText"
    :ok-button-props="{ danger: true }"
    :cancel-text="$t('common_cancel')"
    @ok="$emit('ok')"
    @cancel="$emit('update:open', false)"
  >
    {{ content }}
  </a-modal>
</template>

<script setup>
defineProps({
  open: { type: Boolean, default: false },
  title: { type: String, default: "" },
  content: { type: String, default: "" },
  okText: { type: String, default: "" },
  confirmLoading: { type: Boolean, default: false },
});
defineEmits(["update:open", "ok"]);
</script>

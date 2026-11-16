<template>
  <button
    type="button"
    class="workflow-option-handle"
    :title="$t('settings_reorderOptions')"
    :aria-label="`${label} · ${$t('settings_reorderOptions')}`"
    :aria-disabled="disabled"
    @keydown.up.prevent="$emit('move', -1)"
    @keydown.down.prevent="$emit('move', 1)"
  >
    <HolderOutlined />
  </button>
</template>

<script setup>
import { HolderOutlined } from "@ant-design/icons-vue";

defineProps({ label: String, disabled: Boolean });
defineEmits(["move"]);
</script>

<style scoped lang="less">
.workflow-option-handle {
  margin-inline-end: 8px;
  padding: 4px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: inherit;
  cursor: grab;
  touch-action: none;
}
.workflow-option-handle:active {
  cursor: grabbing;
}
.workflow-option-handle[aria-disabled="true"] {
  opacity: 0.4;
  cursor: default;
}
</style>

<!-- IterationLabel.vue: displays an iteration name with its cross-workspace badge. -->
<template>
  <span class="iteration-label">
    <a-tag v-if="crossWorkspace" color="purple" class="iteration-label__badge">
      {{ $t("iteration_crossBadge") }}
    </a-tag>
    <span class="iteration-label__name">{{ name }}</span>
  </span>
</template>

<script setup>
defineProps({
  name: { type: String, default: "" },
  crossWorkspace: { type: Boolean, default: false },
});
</script>

<style scoped lang="less">
.iteration-label {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 4px;
}

.iteration-label__badge {
  flex: none;
  margin-inline-end: 0;
}

.iteration-label__name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>

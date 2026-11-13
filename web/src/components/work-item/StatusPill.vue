<template>
  <span class="status-pill" :style="pillStyle">{{ displayText }}</span>
</template>

<script setup>
import { computed } from "vue";
const props = defineProps({
  text: { type: String, default: "" },
  color: { type: String, default: "" },
});

const pillStyle = computed(() => ({
  color: props.color || "#8c8c9c",
  border: `1px solid ${props.color || "#8c8c9c"}`,
}));

const displayText = computed(() => props.text || "-");
</script>

<style scoped>
.status-pill {
  display: inline-flex;
  align-items: center;
  height: 19.6px;
  padding: 0 10px;
  border-radius: 12px;
  font-size: 12px;
  line-height: 19.6px;
  white-space: nowrap;
}
</style>

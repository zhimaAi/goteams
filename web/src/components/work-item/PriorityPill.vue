<template>
  <span class="priority-pill" :style="pillStyle">{{ displayText }}</span>
</template>

<script setup>
import { computed } from "vue";
import { useI18n } from "vue-i18n";

const props = defineProps({
  // Priority ID (from backend workspace_priorities.id).
  priority: { type: [String, Number], default: "" },
  // Priority color (from backend workspace_priorities.color); used directly when passed in.
  color: { type: String, default: "" },
  // Display text (from backend workspace_priorities.name); preferred when passed in.
  label: { type: String, default: "" },
});
const { t } = useI18n();

const colorMap = {
  "#ff4d4f": "#ff4d4f",
  "#fa8c16": "#fa8c16",
  "#0d68ff": "#0d68ff",
  "#8c8c8c": "#8c8c8c",
  "#52c41a": "#52c41a",
  "#722ed1": "#722ed1",
  "#eb2f96": "#eb2f96",
  "#faad14": "#faad14",
  "#13c2c2": "#13c2c2",
  "#a0d911": "#a0d911",
  "#fa541c": "#fa541c",
};

const effectiveColor = computed(() => {
  if (props.color) return colorMap[props.color] || props.color;
  return "#8c8c9c";
});

const pillStyle = computed(() => ({
  color: effectiveColor.value,
  border: `1px solid ${effectiveColor.value}`,
}));

const displayText = computed(() => {
  if (props.priority === 0 || props.priority === "0") return "-";
  if (props.label) return props.label;
  return props.priority || "-";
});
</script>

<style scoped>
.priority-pill {
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

<template>
  <div class="dashboard-color-picker">
    <button
      v-for="color in colors"
      :key="color"
      type="button"
      :style="{ backgroundColor: color }"
      :aria-label="`${label} ${color}`"
      :aria-pressed="value === color"
      @click="emit('update:value', color)"
    >
      <CheckOutlined v-if="value === color" />
    </button>
    <input
      :id="inputId"
      :value="value || '#000000'"
      :aria-label="label"
      type="color"
      @input="emit('update:value', $event.target.value)"
    />
  </div>
</template>

<script setup>
import { CheckOutlined } from "@ant-design/icons-vue";

defineProps({
  value: { type: String, default: "" },
  label: { type: String, required: true },
  inputId: { type: String, default: undefined },
});
const emit = defineEmits(["update:value"]);
const colors = [
  "#ff5722",
  "#a0d911",
  "#2f54eb",
  "#faad14",
  "#eb2f96",
  "#13c2c2",
  "#722ed1",
  "#f5222d",
  "#fa8c16",
  "#52c41a",
  "#1677ff",
];
</script>

<style scoped lang="less">
.dashboard-color-picker {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  button,
  input {
    width: 28px;
    height: 28px;
    padding: 0;
    border: 0;
    border-radius: 6px;
    cursor: pointer;
    color: #fff;
  }
  button[aria-pressed="true"] {
    outline: 2px solid #15284b;
    outline-offset: 2px;
  }
  input {
    background: transparent;
  }
}
</style>

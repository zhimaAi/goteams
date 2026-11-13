<template>
  <div class="avatar-picker">
    <p v-if="label" class="avatar-picker-label">{{ label }}</p>
    <div class="avatar-picker-options">
      <button
        v-for="item in options"
        :key="item"
        type="button"
        class="avatar-picker-option"
        :class="{ active: modelValue === item }"
        :aria-pressed="modelValue === item"
        @click="select(item)"
      >
        <img :src="item" alt="" />
      </button>
    </div>
  </div>
</template>

<script setup>
const props = defineProps({
  modelValue: { type: String, default: "" },
  options: { type: Array, default: () => [] },
  label: { type: String, default: "" },
});

const emit = defineEmits(["update:modelValue"]);

function select(item) {
  emit("update:modelValue", item);
}
</script>

<style scoped>
.avatar-picker-label {
  margin: 0 0 8px;
  color: #262626;
  font-size: 14px;
  font-weight: 600;
}

.avatar-picker-options {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.avatar-picker-option {
  display: inline-flex;
  width: 34px;
  height: 34px;
  align-items: center;
  justify-content: center;
  padding: 1px;
  overflow: hidden;
  border: 2px solid transparent;
  border-radius: 50%;
  background: #fff;
  cursor: pointer;
}

.avatar-picker-option img {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  object-fit: cover;
}

.avatar-picker-option:hover,
.avatar-picker-option.active {
  border-color: #3157e2;
}

.avatar-picker-option:focus-visible {
  outline: 2px solid #3157e2;
  outline-offset: 2px;
}
</style>

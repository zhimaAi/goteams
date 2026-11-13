<!-- NumberRangeFilter.vue - compact open-ended numeric range. -->
<template>
  <div class="number-range-filter">
    <a-input-number
      :value="numberValue(values[0])"
      :placeholder="$t('search_number_min')"
      :controls="false"
      @change="(value) => updateAt(0, value)"
      @press-enter="$emit('search')"
    />
    <span>→</span>
    <a-input-number
      :value="numberValue(values[1])"
      :placeholder="$t('search_number_max')"
      :controls="false"
      @change="(value) => updateAt(1, value)"
      @press-enter="$emit('search')"
    />
  </div>
</template>

<script setup>
const props = defineProps({
  values: { type: Array, default: () => [] },
});

const emit = defineEmits({
  change: (values) => true,
  search: () => true,
});

function numberValue(value) {
  if (value === undefined || value === null || value === "") return null;
  const number = Number(value);
  return Number.isFinite(number) ? number : null;
}

function updateAt(index, value) {
  const next = [props.values[0] || "", props.values[1] || ""];
  next[index] = value === null || value === undefined ? "" : String(value);
  emit("change", next.some((item) => item !== "") ? next : []);
}
</script>

<style scoped lang="less">
.number-range-filter {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 4px;
  color: var(--app-text-secondary);
}

.number-range-filter :deep(.ant-input-number) {
  min-width: 0;
  flex: 1;
  border: 0;
  background: transparent;
  box-shadow: none;
}

.number-range-filter :deep(.ant-input-number-input) {
  height: 28px;
  padding: 0;
  font-size: 12px;
  text-align: center;
}
</style>

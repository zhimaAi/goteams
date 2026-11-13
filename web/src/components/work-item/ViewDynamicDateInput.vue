<!--
 * ViewDynamicDateInput.vue
 * Relative date range input for view filters: two "<amount> <unit>" sides
 * (e.g. 3 months ago ~ 1 month later). Either side may stay empty, which the
 * backend reads as an open range.
 -->
<template>
  <span class="view-dynamic-date">
    <span
      v-for="(side, index) in sides"
      :key="index ? 'end' : 'start'"
      class="view-dynamic-date__side"
    >
      <span v-if="index" class="view-dynamic-date__sep">-</span>
      <a-input-number
        class="view-dynamic-date__amount"
        :value="side.amount"
        :min="1"
        :max="999"
        :precision="0"
        :disabled="isToday(side.key)"
        @update:value="(amount) => update(index, { amount })"
      />
      <a-select
        class="view-dynamic-date__unit"
        :value="side.key || undefined"
        :options="unitOptions"
        :placeholder="$t('view_dynamic_placeholder')"
        allow-clear
        @update:value="(key) => update(index, { key: key || '' })"
      />
    </span>
  </span>
</template>

<script setup>
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import {
  VIEW_DYNAMIC_UNITS,
  parseDynamicDate,
  formatDynamicDate,
} from "@/constants/workItemView";

const props = defineProps({
  // Stored condition values: [startValue, endValue], each `<offset>:<unit>` or "".
  values: { type: Array, default: () => [] },
});

const emit = defineEmits({
  // Emit the rewritten values array.
  "update:values": (values) => true,
});

const { t } = useI18n();

const unitOptions = computed(() =>
  VIEW_DYNAMIC_UNITS.map((item) => ({
    value: item.key,
    label: t(`view_dynamic_${item.key}`),
  })),
);

const sides = computed(() => [
  parseDynamicDate(props.values[0]),
  parseDynamicDate(props.values[1]),
]);

function isToday(key) {
  return key === "today";
}

function update(index, patch) {
  const next = sides.value.map((side) => ({ ...side }));
  Object.assign(next[index], patch);
  emit("update:values", next.map(formatDynamicDate));
}
</script>

<style scoped lang="less">
.view-dynamic-date {
  display: inline-flex;
  width: 100%;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.view-dynamic-date__side {
  display: inline-flex;
  min-width: 0;
  flex: 1 1 136px;
  align-items: center;
  gap: 8px;
}
.view-dynamic-date__amount {
  width: 72px;
  min-width: 48px;
  flex: 0 1 72px;
}
.view-dynamic-date__unit {
  width: 104px;
  min-width: 80px;
  flex: 1 1 104px;
}
.view-dynamic-date__sep {
  color: rgba(0, 0, 0, 0.45);
}
</style>

<template>
  <a-date-picker
    v-bind="$attrs"
    :value="value"
    :show-today="false"
    @update:value="$emit('update:value', $event)"
    @change="(date, dateString) => $emit('change', date, dateString)"
    @open-change="$emit('openChange', $event)"
  >
    <template #renderExtraFooter>
      <div class="quick-date-actions">
        <button type="button" @click.stop="selectDate(today)">
          {{ $t("common_today") }}
        </button>
        <button type="button" @click.stop="selectDate(tomorrow)">
          {{ $t("common_tomorrow") }}
        </button>
        <button type="button" @click.stop="selectDate(nextMonday)">
          {{ $t("common_nextMonday") }}
        </button>
      </div>
    </template>
  </a-date-picker>
</template>

<script setup>
import { computed } from "vue";
import dayjs from "dayjs";

defineOptions({ inheritAttrs: false });

defineProps({
  value: {
    type: Object,
    default: null,
  },
});

const emit = defineEmits(["update:value", "change", "openChange"]);

const today = computed(() => dayjs().startOf("day"));
const tomorrow = computed(() => today.value.add(1, "day"));
const nextMonday = computed(() => {
  const daysUntilMonday = (8 - today.value.day()) % 7 || 7;
  return today.value.add(daysUntilMonday, "day");
});

function selectDate(date) {
  emit("update:value", date);
  emit("change", date, date.format("YYYY-MM-DD"));
}
</script>

<style scoped lang="less">
.quick-date-actions {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 0 8px;

  button {
    height: 30px;
    padding: 0 6px;
    color: #1677ff;
    font: inherit;
    background: transparent;
    border: 0;
    cursor: pointer;
    line-height: 30px;
    &:hover,
    &:focus-visible {
      color: #4096ff;
    }

    &:focus-visible {
      outline: 2px solid #91caff;
      outline-offset: -2px;
    }
  }
}
</style>

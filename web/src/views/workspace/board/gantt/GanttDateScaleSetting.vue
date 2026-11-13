<!-- GanttDateScaleSetting.vue：甘特图列表设置中的日期维度。 -->
<template>
  <section class="gantt-date-scale-setting">
    <h3>{{ $t("gantt_date_scale") }}</h3>
    <a-radio-group
      :value="value"
      class="gantt-date-scale-setting__group"
      button-style="solid"
      @change="$emit('change', $event.target.value)"
    >
      <a-radio-button
        v-for="option in dateScaleOptions"
        :key="option.value"
        :value="option.value"
      >
        {{ $t(option.labelKey) }}
      </a-radio-button>
    </a-radio-group>
  </section>
</template>

<script setup>
import { GANTT_DATE_SCALE_OPTIONS, GANTT_DATE_SCALES } from "@/constants/gantt";

const dateScaleOptions = GANTT_DATE_SCALE_OPTIONS;

defineProps({
  value: { type: String, default: "week" },
});
defineEmits({
  change: (value) => GANTT_DATE_SCALES.includes(value),
});
</script>

<style scoped lang="less">
.gantt-date-scale-setting {
  margin-top: 0;
  padding: 16px 6px 20px;
  border-top: 1px solid var(--app-border);

  h3 {
    margin: 0 0 12px;
    color: var(--app-text);
    font-size: 14px;
    font-weight: 600;
  }
}
.gantt-date-scale-setting__group {
  display: flex;
  width: 100%;

  :deep(.ant-radio-button-wrapper) {
    display: inline-flex;
    min-width: 0;
    flex: 1;
    align-items: center;
    justify-content: center;
    padding-inline: 8px;
  }
}
</style>

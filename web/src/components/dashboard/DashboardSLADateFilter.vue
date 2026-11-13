<template>
  <div class="sla-date-filter">
    <a-radio-group
      :value="filters.preset"
      :aria-label="t('dashboard_chart_time_range')"
      size="small"
      button-style="solid"
      @update:value="selectPreset"
    >
      <a-radio-button
        v-for="preset in presets"
        :key="preset.value"
        :value="preset.value"
        >{{ t(preset.label) }}</a-radio-button
      >
    </a-radio-group>
    <a-range-picker
      v-if="filters.preset === 'custom'"
      :value="filters.range"
      value-format="YYYY-MM-DD"
      :allow-clear="false"
      size="small"
      @change="selectRange"
    />
  </div>
</template>

<script setup>
import { message } from "ant-design-vue";
import { useDashboardI18n } from "@/composables/useDashboardI18n";
import { categoryDateRange } from "./dashboardCharts";

const props = defineProps({
  filters: { type: Object, required: true },
});
const emit = defineEmits(["update:filters"]);
const presets = [
  { value: "today", label: "common_today" },
  { value: "week", label: "dashboard_category_week" },
  { value: "month30", label: "dashboard_category_month30" },
  { value: "month", label: "dashboard_category_month" },
  { value: "custom", label: "dashboard_category_custom" },
];
const { t } = useDashboardI18n();

function selectPreset(preset) {
  const range =
    preset === "custom"
      ? categoryDateRange(props.filters.preset, props.filters.range).map(
          (date) => date.format("YYYY-MM-DD"),
        )
      : undefined;
  emit("update:filters", { preset, range });
}

function selectRange(range) {
  if (!range?.length) return;
  const [from, to] = categoryDateRange("custom", range);
  if (to.diff(from, "day") >= 366) {
    message.warning(t("dashboard_sla_risk_invalid_range"));
    return;
  }
  emit("update:filters", { preset: "custom", range });
}
</script>

<style scoped lang="less">
.sla-date-filter {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 10px;
}
</style>

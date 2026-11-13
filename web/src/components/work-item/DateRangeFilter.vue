<!-- DateRangeFilter.vue - compact calendar-range search condition. -->
<template>
  <div class="date-range-filter">
    <a-range-picker
      v-model:open="open"
      :value="pickerValue"
      :allow-empty="[true, true]"
      :placeholder="[$t('search_date_start'), $t('search_date_end')]"
      format="YYYY-MM-DD"
      :bordered="false"
      popup-class-name="date-range-filter-popup"
      :panel-render="renderPanel"
      @change="handleChange"
    />
    <span v-if="modeText" class="date-range-filter__mode" :title="modeText">
      {{ modeText }}
    </span>
  </div>
</template>

<script setup>
import dayjs from "dayjs";
import { computed, h, ref } from "vue";
import { useI18n } from "vue-i18n";

const props = defineProps({
  operator: { type: String, default: "between" },
  values: { type: Array, default: () => [] },
});

const emit = defineEmits({
  change: (condition) => true,
});

const { t } = useI18n();
const open = ref(false);

const pickerValue = computed(() => {
  if (props.operator !== "between") return [null, null];
  return [dateValue(props.values[0]), dateValue(props.values[1])];
});
const modeText = computed(() => {
  if (props.operator === "empty") return t("view_op_empty");
  if (props.operator === "not_empty") return t("view_op_not_empty");
  return "";
});
const presets = computed(() => {
  const today = dayjs().startOf("day");
  return [
    {
      key: "week",
      label: t("search_date_last_week"),
      values: [today.subtract(6, "day"), today],
    },
    {
      key: "month",
      label: t("search_date_last_month"),
      values: [today.subtract(1, "month"), today],
    },
    {
      key: "three_months",
      label: t("search_date_last_three_months"),
      values: [today.subtract(3, "month"), today],
    },
    {
      key: "from_today",
      label: t("search_date_from_today"),
      values: [today, null],
    },
    {
      key: "through_today",
      label: t("search_date_through_today"),
      values: [null, today],
    },
  ];
});
const activePresetKey = computed(() => {
  if (props.operator === "empty" || props.operator === "not_empty") {
    return props.operator;
  }
  if (props.operator !== "between") return "";
  const current = formattedValues(props.values || []);
  const currentKey = current.join("|");
  return (
    presets.value.find(
      (preset) => formattedValues(preset.values).join("|") === currentKey,
    )?.key || ""
  );
});

function renderPanel(panelNode) {
  const presetNodes = presets.value.map((preset) => {
    const active = activePresetKey.value === preset.key;
    return h(
      "button",
      {
        key: preset.key,
        type: "button",
        class: ["date-range-panel__preset", { active }],
        "aria-pressed": active,
        onClick: (event) => {
          event.stopPropagation();
          selectPreset(preset);
        },
      },
      preset.label,
    );
  });
  [
    { key: "empty", label: t("view_op_empty"), operator: "empty" },
    { key: "not_empty", label: t("view_op_not_empty"), operator: "not_empty" },
  ].forEach((preset) => {
    const active = activePresetKey.value === preset.key;
    presetNodes.push(
      h(
        "button",
        {
          key: preset.key,
          type: "button",
          class: ["date-range-panel__preset", { active }],
          "aria-pressed": active,
          onClick: (event) => {
            event.stopPropagation();
            selectMode(preset.operator);
          },
        },
        preset.label,
      ),
    );
  });

  return h("div", { class: "date-range-panel" }, [
    h("div", { class: "date-range-panel__presets" }, presetNodes),
    panelNode,
  ]);
}

function dateValue(value) {
  if (!value) return null;
  const parsed = dayjs(value, "YYYY-MM-DD", true);
  return parsed.isValid() ? parsed : null;
}

function formattedValues(values) {
  return values.map((value) => {
    if (typeof value === "string") return value;
    return value?.format?.("YYYY-MM-DD") || "";
  });
}

function handleChange(values) {
  const next = formattedValues(values || [null, null]);
  emit("change", {
    operator: "between",
    values: next.some(Boolean) ? next : [],
  });
}

function selectPreset(preset) {
  emit("change", {
    operator: "between",
    values: formattedValues(preset.values),
  });
  open.value = false;
}

function selectMode(operator) {
  emit("change", { operator, values: [] });
  open.value = false;
}
</script>

<style scoped lang="less">
.date-range-filter {
  position: absolute;
  inset: 0;
  min-width: 0;
}

.date-range-filter :deep(.ant-picker) {
  width: 100%;
  height: 28px;
  padding: 0 24px 0 64px;
  background: transparent;
  font-size: 12px;
}

.date-range-filter :deep(.ant-picker-input > input) {
  font-size: 12px;
}

.date-range-filter__mode {
  position: absolute;
  top: 0;
  right: 24px;
  left: 64px;
  overflow: hidden;
  color: var(--app-text);
  font-size: 12px;
  line-height: 28px;
  text-overflow: ellipsis;
  white-space: nowrap;
  pointer-events: none;
}

:global(.date-range-filter-popup .date-range-panel) {
  display: flex;
  min-height: 310px;
}

:global(.date-range-filter-popup .date-range-panel__presets) {
  display: flex;
  width: 112px;
  flex-direction: column;
  gap: 8px;
  padding: 12px 10px 12px 12px;
  border-right: 1px solid var(--app-border);
  background: var(--app-surface);
}

:global(.date-range-filter-popup .date-range-panel__preset) {
  width: 100%;
  min-height: 28px;
  padding: 0 10px;
  border: 1px solid transparent;
  border-radius: 999px;
  color: var(--app-text-secondary);
  background: var(--app-surface-muted);
  cursor: pointer;
  font-size: 12px;
  text-align: center;
  transition:
    color 0.15s ease,
    background 0.15s ease,
    border-color 0.15s ease;
}

:global(.date-range-filter-popup .date-range-panel__preset:hover),
:global(.date-range-filter-popup .date-range-panel__preset:focus-visible) {
  color: #3157e2;
  background: #edf3ff;
  border-color: #c9dcff;
  outline: none;
}

:global(.date-range-filter-popup .date-range-panel__preset.active) {
  color: #fff;
  background: #3157e2;
  border-color: #3157e2;
}

:global(.date-range-filter-popup .date-range-panel__preset.active:hover),
:global(
  .date-range-filter-popup .date-range-panel__preset.active:focus-visible
) {
  color: #fff;
  background: #2757d6;
  border-color: #2757d6;
}
</style>

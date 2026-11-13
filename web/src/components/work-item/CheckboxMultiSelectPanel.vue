<!--
 * CheckboxMultiSelectPanel.vue
 * Compact checkbox filter used by board search conditions.
 -->
<template>
  <a-popover
    v-model:open="panelOpen"
    trigger="click"
    placement="bottomLeft"
    overlay-class-name="checkbox-multi-select-popover"
  >
    <template #content>
      <div class="checkbox-multi-panel" :style="{ width: `${panelWidth}px` }">
        <a-input
          v-model:value="searchText"
          :placeholder="$t('common_search')"
          allow-clear
        >
          <template #suffix><SearchOutlined /></template>
        </a-input>

        <div class="checkbox-multi-panel__options">
          <div v-if="leadingOptions.length" class="checkbox-multi-panel__group">
            <label
              v-for="option in leadingOptions"
              :key="option.value"
              class="checkbox-multi-panel__item"
            >
              <a-checkbox
                :checked="selectedSet.has(String(option.value))"
                @change="toggleValue(option.value)"
              />
              <span>{{ option.label }}</span>
            </label>
          </div>

          <div
            v-if="leadingOptions.length && filteredOptions.length"
            class="checkbox-multi-panel__divider"
          />

          <div class="checkbox-multi-panel__group">
            <label
              v-for="option in filteredOptions"
              :key="option.value"
              class="checkbox-multi-panel__item"
            >
              <a-checkbox
                :checked="selectedSet.has(String(option.value))"
                @change="toggleValue(option.value)"
              />
              <span>{{ option.label }}</span>
            </label>
            <div
              v-if="!filteredOptions.length"
              class="checkbox-multi-panel__empty"
            >
              {{ $t("search_no_options") }}
            </div>
          </div>
        </div>

        <div class="checkbox-multi-panel__footer">
          <label class="checkbox-multi-panel__select-all">
            <a-checkbox
              :checked="allOptionsChecked"
              :indeterminate="someOptionsChecked"
              :disabled="!selectableOptions.length"
              @change="toggleAllOptions"
            />
            <span>
              {{ $t("common_selectAll") }} ({{ selectableOptions.length }})
            </span>
          </label>
          <label
            v-if="showClosedToggle"
            class="checkbox-multi-panel__closed-toggle"
          >
            <a-switch v-model:checked="showClosed" size="small" />
            <span>{{ $t("search_show_closed_versions") }}</span>
          </label>
        </div>
      </div>
    </template>

    <div
      class="checkbox-multi-trigger"
      :class="{
        'checkbox-multi-trigger--empty': !selectedLabels.length,
        'checkbox-multi-trigger--open': panelOpen,
      }"
      role="button"
      tabindex="0"
      :aria-expanded="panelOpen"
      @keydown.enter.prevent="panelOpen = !panelOpen"
      @keydown.space.prevent="panelOpen = !panelOpen"
    >
      <span
        class="checkbox-multi-trigger__value"
        :title="selectedLabels.length ? displayText : undefined"
      >
        {{ displayText }}
      </span>
      <DownOutlined class="checkbox-multi-trigger__arrow" />
    </div>
  </a-popover>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { DownOutlined, SearchOutlined } from "@ant-design/icons-vue";

const props = defineProps({
  value: { type: Array, default: () => [] },
  options: { type: Array, default: () => [] },
  leadingOptions: { type: Array, default: () => [] },
  showClosedToggle: { type: Boolean, default: false },
  panelWidth: { type: Number, default: 304 },
});

const emit = defineEmits({
  "update:value": (values) => true,
});

const { t } = useI18n();
const panelOpen = ref(false);
const searchText = ref("");
const showClosed = ref(false);

watch(panelOpen, (open) => {
  if (!open) searchText.value = "";
});

const selectedSet = computed(() => new Set(props.value.map(String)));
const selectableOptions = computed(() =>
  props.options.filter(
    (option) =>
      !option.closed ||
      showClosed.value ||
      selectedSet.value.has(String(option.value)),
  ),
);
const filteredOptions = computed(() => {
  const keyword = searchText.value.trim().toLocaleLowerCase();
  if (!keyword) return selectableOptions.value;
  return selectableOptions.value.filter((option) =>
    String(option.label).toLocaleLowerCase().includes(keyword),
  );
});
const allOptions = computed(() => [...props.leadingOptions, ...props.options]);
const allOptionsChecked = computed(
  () =>
    selectableOptions.value.length > 0 &&
    selectableOptions.value.every((option) =>
      selectedSet.value.has(String(option.value)),
    ),
);
const someOptionsChecked = computed(
  () =>
    !allOptionsChecked.value &&
    selectableOptions.value.some((option) =>
      selectedSet.value.has(String(option.value)),
    ),
);
const selectedLabels = computed(() => {
  const labels = new Map(
    allOptions.value.map((option) => [String(option.value), option.label]),
  );
  return props.value.map(String).map((value) => labels.get(value) || value);
});
const displayText = computed(() => {
  if (!selectedLabels.value.length) return t("common_all");
  return selectedLabels.value.join(" | ");
});

function orderedValues(values) {
  const result = allOptions.value
    .map((option) => String(option.value))
    .filter((value) => values.has(value));
  for (const value of values) {
    if (!result.includes(value)) result.push(value);
  }
  return result;
}

function toggleValue(value) {
  const next = new Set(selectedSet.value);
  const normalized = String(value);
  if (next.has(normalized)) next.delete(normalized);
  else next.add(normalized);
  emit("update:value", orderedValues(next));
}

function toggleAllOptions() {
  if (allOptionsChecked.value) {
    emit("update:value", []);
    return;
  }
  const next = new Set(selectedSet.value);
  selectableOptions.value.forEach((option) => next.add(String(option.value)));
  emit("update:value", orderedValues(next));
}
</script>

<style scoped lang="less">
.checkbox-multi-trigger {
  position: absolute;
  inset: 0;
  display: flex;
  min-width: 0;
  align-items: center;
  padding: 0 20px 0 64px;
  cursor: pointer;
}

.checkbox-multi-trigger__value {
  min-width: 0;
  overflow: hidden;
  color: var(--app-text);
  font-size: 12px;
  line-height: 28px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.checkbox-multi-trigger--empty .checkbox-multi-trigger__value {
  color: var(--app-text-secondary);
}

.checkbox-multi-trigger__arrow {
  position: absolute;
  right: 8px;
  color: var(--app-text-secondary);
  font-size: 10px;
  transition: transform 0.15s ease;
}

.checkbox-multi-trigger--open .checkbox-multi-trigger__arrow {
  transform: rotate(180deg);
}

.checkbox-multi-trigger:focus-visible {
  outline: none;
}

.checkbox-multi-panel {
  color: var(--app-text);
}

.checkbox-multi-panel :deep(.ant-input-affix-wrapper) {
  height: 38px;
  border-radius: 4px;
}

.checkbox-multi-panel :deep(.ant-input-suffix) {
  color: var(--app-text-secondary);
}

.checkbox-multi-panel__options {
  max-height: 320px;
  margin: 12px -12px 0;
  overflow-y: auto;
}

.checkbox-multi-panel__group {
  padding: 0 12px;
}

.checkbox-multi-panel__item {
  display: flex;
  min-height: 38px;
  align-items: center;
  gap: 10px;
  padding: 6px 4px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 13px;
}

.checkbox-multi-panel__item:hover {
  background: var(--app-surface-muted);
}

.checkbox-multi-panel__divider {
  height: 1px;
  margin: 6px 0;
  background: var(--app-border);
}

.checkbox-multi-panel__empty {
  padding: 20px 4px;
  color: var(--app-text-secondary);
  font-size: 12px;
  text-align: center;
}

.checkbox-multi-panel__footer {
  display: flex;
  min-height: 48px;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin: 8px -12px -12px;
  padding: 8px 16px;
  border-top: 1px solid var(--app-border);
}

.checkbox-multi-panel__select-all,
.checkbox-multi-panel__closed-toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  white-space: nowrap;
}

.checkbox-multi-panel__closed-toggle {
  color: var(--app-text-secondary);
  font-size: 12px;
}
</style>

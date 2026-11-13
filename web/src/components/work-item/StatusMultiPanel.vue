<!--
 * StatusMultiPanel.vue
 * Custom status multi-select panel for view filters: search, the special
 * "-all final statuses-" / "-all unfinished statuses-" options, per-status
 * checkboxes, and a bottom select-all with a count.
 -->
<template>
  <a-popover
    v-model:open="panelOpen"
    trigger="click"
    placement="bottomLeft"
    overlay-class-name="status-multi-pop"
  >
    <template #content>
      <div class="status-multi-panel">
        <a-input
          v-model:value="searchText"
          :placeholder="$t('common_search')"
          size="small"
          allow-clear
        >
          <template #prefix><SearchOutlined /></template>
        </a-input>
        <div class="status-multi-panel__list">
          <label
            v-for="option in specialOptions"
            :key="option.value"
            class="status-multi-panel__item"
          >
            <a-checkbox
              :checked="selectedSet.has(option.value)"
              @change="toggleValue(option.value)"
            />
            <span>-{{ option.label }}-</span>
          </label>
          <a-divider class="status-multi-panel__divider" />
          <label
            v-for="status in filteredStatuses"
            :key="status.id"
            class="status-multi-panel__item"
          >
            <a-checkbox
              :checked="selectedSet.has(String(status.id))"
              @change="toggleValue(String(status.id))"
            />
            <span>{{ status.name }}</span>
          </label>
        </div>
        <a-divider class="status-multi-panel__divider" />
        <label class="status-multi-panel__item">
          <a-checkbox
            :checked="allStatusesChecked"
            :indeterminate="someStatusesChecked"
            @change="toggleAllStatuses"
          />
          <span>{{ $t("view_select_all") }} ({{ statuses.length }})</span>
        </label>
      </div>
    </template>
    <div
      class="status-multi-trigger"
      :class="{ 'status-multi-trigger--empty': !displayText }"
    >
      {{ displayText || $t("view_value_placeholder") }}
    </div>
  </a-popover>
</template>

<script setup>
import { ref, computed } from "vue";
import { useI18n } from "vue-i18n";
import { SearchOutlined } from "@ant-design/icons-vue";

const props = defineProps({
  // Selected values: status ids as strings plus the special placeholder values.
  value: { type: Array, default: () => [] },
  // Status options of the current work item type: [{ id, name }].
  statuses: { type: Array, default: () => [] },
});

const emit = defineEmits({
  "update:value": (values) => true,
});

const { t } = useI18n();
const panelOpen = ref(false);
const searchText = ref("");

const specialOptions = computed(() => [
  { value: "__all_completed__", label: t("view_all_completed_statuses") },
  { value: "__all_closed__", label: t("view_all_closed_statuses") },
  { value: "__all_end__", label: t("view_all_end_statuses") },
  { value: "__all_not_end__", label: t("view_all_not_end_statuses") },
]);

const selectedSet = computed(() => new Set(props.value.map(String)));

const filteredStatuses = computed(() => {
  const keyword = searchText.value.trim().toLowerCase();
  if (!keyword) return props.statuses;
  return props.statuses.filter((status) =>
    String(status.name).toLowerCase().includes(keyword),
  );
});

const allStatusesChecked = computed(
  () =>
    props.statuses.length > 0 &&
    props.statuses.every((status) => selectedSet.value.has(String(status.id))),
);
const someStatusesChecked = computed(
  () =>
    !allStatusesChecked.value &&
    props.statuses.some((status) => selectedSet.value.has(String(status.id))),
);

const displayText = computed(() => {
  const labels = [];
  for (const option of specialOptions.value) {
    if (selectedSet.value.has(option.value)) labels.push(option.label);
  }
  for (const status of props.statuses) {
    if (selectedSet.value.has(String(status.id))) labels.push(status.name);
  }
  return labels.join("、");
});

function toggleValue(value) {
  const next = new Set(selectedSet.value);
  if (next.has(value)) {
    next.delete(value);
  } else {
    next.add(value);
  }
  emit("update:value", [...next]);
}

function toggleAllStatuses() {
  const next = new Set(selectedSet.value);
  if (allStatusesChecked.value) {
    for (const status of props.statuses) next.delete(String(status.id));
  } else {
    for (const status of props.statuses) next.add(String(status.id));
  }
  emit("update:value", [...next]);
}
</script>

<style scoped lang="less">
.status-multi-trigger {
  /* Shrinkable inside deeply nested condition rows; the popover holds the full list. */
  min-width: 96px;
  min-height: 32px;
  padding: 4px 11px;
  border: 1px solid #d9d9d9;
  border-radius: 6px;
  cursor: pointer;
  text-align: center;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  &:hover {
    border-color: #1677ff;
  }
}
.status-multi-trigger--empty {
  color: rgba(0, 0, 0, 0.25);
}
.status-multi-panel {
  width: 240px;
}
.status-multi-panel__list {
  max-height: 260px;
  overflow-y: auto;
  margin-top: 8px;
}
.status-multi-panel__item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 4px;
  cursor: pointer;
  border-radius: 4px;
  &:hover {
    background: rgba(0, 0, 0, 0.04);
  }
}
.status-multi-panel__divider {
  margin: 4px 0;
}
</style>

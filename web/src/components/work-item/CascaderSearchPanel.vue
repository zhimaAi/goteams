<!--
 * CascaderSearchPanel.vue
 * Hierarchical single/multiple picker for cascader custom-field search.
 -->
<template>
  <a-popover
    v-model:open="open"
    trigger="click"
    placement="bottomLeft"
    overlay-class-name="cascader-search-popover"
  >
    <span class="cascader-search-panel__trigger">
      <span class="cascader-search-panel__trigger-text">{{ triggerText }}</span>
      <button
        v-if="selected.size"
        type="button"
        class="cascader-search-panel__clear"
        :aria-label="$t('common_clearFilter')"
        @click.stop.prevent="emitValues([])"
      >
        <CloseCircleFilled />
      </button>
    </span>
    <template #content>
      <div class="cascader-search-panel">
        <a-input
          v-model:value="keyword"
          :placeholder="$t('common_search')"
          allow-clear
        />
        <component
          :is="selectionContainer"
          :value="multiple ? undefined : selectedValue"
          :name="multiple ? undefined : radioGroupName"
          :aria-label="multiple ? undefined : ariaLabel"
          class="cascader-search-panel__selection"
          @update:value="selectSingle"
        >
          <div class="cascader-search-panel__body">
            <div
              v-for="(column, columnIndex) in columns"
              :key="columnIndex"
              class="cascader-search-panel__col"
            >
              <template v-if="columnIndex === 0">
                <div
                  v-for="option in filteredLeadingOptions"
                  :key="option.value"
                  class="cascader-search-panel__item"
                >
                  <a-checkbox
                    v-if="multiple"
                    :checked="optionChecked(option.value)"
                    @change="toggle(option.value)"
                  >
                    {{ option.label }}
                  </a-checkbox>
                  <a-radio v-else :value="option.value">
                    {{ option.label }}
                  </a-radio>
                </div>
              </template>
              <div
                v-for="node in column"
                :key="node.key"
                class="cascader-search-panel__item"
                :class="{ active: activePath[columnIndex] === node.key }"
                @click="activate(columnIndex, node)"
              >
                <a-checkbox
                  v-if="multiple"
                  :checked="optionChecked(node.key)"
                  @change.stop="toggle(node.key)"
                >
                  {{ node.name }}
                </a-checkbox>
                <a-radio v-else :value="String(node.key)">
                  {{ node.name }}
                </a-radio>
                <span v-if="node.children?.length">›</span>
              </div>
            </div>
          </div>
        </component>
        <div class="cascader-search-panel__footer">
          <label v-if="multiple">
            <a-checkbox :checked="allChecked" @change="toggleAll" />
            {{ $t("common_selectAll") }}
          </label>
          <div>
            <a-button type="primary" size="small" @click="open = false">
              {{ $t("common_confirm") }}
            </a-button>
            <a-button size="small" @click="open = false">
              {{ $t("common_cancel") }}
            </a-button>
          </div>
        </div>
      </div>
    </template>
  </a-popover>
</template>

<script setup>
import { computed, getCurrentInstance, ref } from "vue";
import { useI18n } from "vue-i18n";
import { RadioGroup as ARadioGroup } from "ant-design-vue";
import { CloseCircleFilled } from "@ant-design/icons-vue";
import { flattenCascaderNodes } from "@/utils/customField";
import {
  VIEW_VALUE_ALL,
  VIEW_VALUE_EMPTY,
  VIEW_VALUE_NOT_EMPTY,
} from "@/constants/workItemView";

const props = defineProps({
  value: { type: Array, default: () => [] },
  operator: { type: String, default: "in" },
  nodes: { type: Array, default: () => [] },
  multiple: { type: Boolean, default: false },
  ariaLabel: { type: String, default: "" },
});

const emit = defineEmits({
  "update:value": (values) => true,
});

const { t } = useI18n();
const open = ref(false);
const keyword = ref("");
const activePath = ref([]);
const radioGroupName = `cascader-search-${getCurrentInstance()?.uid}`;
const selectionContainer = computed(() =>
  props.multiple ? "div" : ARadioGroup,
);

const leadingOptions = computed(() => [
  { value: VIEW_VALUE_EMPTY, label: t("view_op_empty") },
  { value: VIEW_VALUE_NOT_EMPTY, label: t("view_op_not_empty") },
]);
const normalizedKeyword = computed(() =>
  keyword.value.trim().toLocaleLowerCase(),
);
const filteredLeadingOptions = computed(() => {
  if (!normalizedKeyword.value) return leadingOptions.value;
  return leadingOptions.value.filter((option) =>
    option.label.toLocaleLowerCase().includes(normalizedKeyword.value),
  );
});

const selected = computed(() => {
  if (props.operator === "empty") return new Set([VIEW_VALUE_EMPTY]);
  if (props.operator === "not_empty") return new Set([VIEW_VALUE_NOT_EMPTY]);
  const values = (props.value || []).map(String);
  return new Set(props.multiple ? values : values.slice(-1));
});
const selectedValue = computed(() => [...selected.value][0]);

const filteredRoots = computed(() => {
  if (!normalizedKeyword.value) return props.nodes || [];
  return flattenCascaderNodes(props.nodes)
    .filter((node) =>
      String(node.name || "")
        .toLocaleLowerCase()
        .includes(normalizedKeyword.value),
    )
    .map((node) => ({ ...node, children: undefined }));
});

const columns = computed(() => {
  const cols = [filteredRoots.value];
  if (normalizedKeyword.value) return cols;
  let current = props.nodes || [];
  for (const key of activePath.value) {
    const node = current.find((item) => item.key === key);
    if (!node?.children?.length) break;
    cols.push(node.children);
    current = node.children;
  }
  return cols;
});

const allKeys = computed(() =>
  flattenCascaderNodes(props.nodes).map((node) => String(node.key)),
);
const allRecordsSelected = computed(
  () => props.multiple && selected.value.has(VIEW_VALUE_ALL),
);
const allChecked = computed(() => allRecordsSelected.value);
const triggerText = computed(() => {
  if (allRecordsSelected.value) return t("common_all");
  const names = [];
  if (selected.value.has(VIEW_VALUE_EMPTY)) names.push(t("view_op_empty"));
  if (selected.value.has(VIEW_VALUE_NOT_EMPTY))
    names.push(t("view_op_not_empty"));
  names.push(
    ...flattenCascaderNodes(props.nodes)
      .filter((node) => selected.value.has(String(node.key)))
      .map((node) => node.name),
  );
  if (!names.length) return t("common_all");
  return names.length === 1
    ? names[0]
    : t("search_selected_count", { n: names.length });
});

function optionChecked(key) {
  return allRecordsSelected.value || selected.value.has(String(key));
}

function activate(columnIndex, node) {
  activePath.value = [...activePath.value.slice(0, columnIndex), node.key];
}

function emitValues(values) {
  emit("update:value", values);
}

function selectSingle(key) {
  if (!props.multiple) emitValues([String(key)]);
}

function normalizedMultiValues(values) {
  const next = new Set(values.map(String));
  next.delete(VIEW_VALUE_ALL);
  return [
    ...[VIEW_VALUE_EMPTY, VIEW_VALUE_NOT_EMPTY].filter((value) =>
      next.has(value),
    ),
    ...allKeys.value.filter((value) => next.has(value)),
  ];
}

function toggle(key) {
  key = String(key);
  if (!props.multiple) {
    emitValues([key]);
    return;
  }
  if (allRecordsSelected.value) {
    if (key === VIEW_VALUE_EMPTY) {
      emitValues([VIEW_VALUE_NOT_EMPTY, ...allKeys.value]);
    } else if (key === VIEW_VALUE_NOT_EMPTY) {
      emitValues([VIEW_VALUE_EMPTY, ...allKeys.value]);
    } else {
      emitValues([
        VIEW_VALUE_EMPTY,
        VIEW_VALUE_NOT_EMPTY,
        ...allKeys.value.filter((value) => value !== key),
      ]);
    }
    return;
  }
  const next = new Set(selected.value);
  if (next.has(key)) next.delete(key);
  else next.add(key);
  emitValues(normalizedMultiValues([...next]));
}

function toggleAll() {
  if (!props.multiple) return;
  emitValues(allRecordsSelected.value ? [] : [VIEW_VALUE_ALL]);
}
</script>

<style scoped lang="less">
.cascader-search-panel__trigger {
  display: inline-flex;
  min-width: 0;
  flex: 1;
  align-items: center;
  padding: 0 8px;
  color: var(--app-text-secondary);
  font-size: 12px;
  cursor: pointer;
}

.cascader-search-panel__trigger-text {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cascader-search-panel__clear {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  padding: 0;
  border: 0;
  color: var(--app-text-tertiary, var(--app-text-secondary));
  background: transparent;
  cursor: pointer;
}

.cascader-search-panel {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 280px;
  max-width: 720px;
}

.cascader-search-panel__selection {
  display: block;
}

.cascader-search-panel__body {
  display: flex;
  min-height: 160px;
  overflow: auto;
  border: 1px solid var(--app-border);
  border-radius: 6px;
}

.cascader-search-panel__col {
  flex: 1 0 160px;
  overflow: auto;
  padding: 8px;
}

.cascader-search-panel__col + .cascader-search-panel__col {
  border-left: 1px solid var(--app-border);
}

.cascader-search-panel__item {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 8px;
  cursor: pointer;
}

.cascader-search-panel__item.active {
  background: var(--app-surface-muted);
}

.cascader-search-panel__footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
}

.cascader-search-panel__footer > label {
  margin-right: auto;
}
</style>

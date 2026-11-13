<template>
  <a-tree-select
    class="custom-field-cascader"
    :value="treeValue"
    :size="size"
    :disabled="disabled"
    :open="forceOpen || undefined"
    :placeholder="placeholder"
    :tree-data="treeData"
    :tree-checkable="multiple"
    :tree-check-strictly="multiple"
    :max-tag-count="multiple ? 0 : undefined"
    :max-tag-placeholder="maxTagPlaceholder"
    :dropdown-match-select-width="false"
    label-in-value
    tree-default-expand-all
    tree-node-filter-prop="title"
    placement="bottomRight"
    show-search
    allow-clear
    style="width: 100%"
    @update:value="onChange"
    @dropdownVisibleChange="$emit('openChange', $event)"
  />
</template>

<script setup>
import { computed } from "vue";
import {
  cascaderPathKeys,
  formatCascaderNodePath,
  formatCascaderNodePaths,
  formatCascaderPath,
} from "@/utils/customField";

const props = defineProps({
  nodes: { type: Array, default: () => [] },
  modelValue: { type: Array, default: () => [] },
  multiple: { type: Boolean, default: false },
  size: { type: String, default: "small" },
  disabled: { type: Boolean, default: false },
  forceOpen: { type: Boolean, default: false },
  placeholder: { type: String, default: "" },
});

const emit = defineEmits({
  "update:modelValue": (value) => true,
  change: () => true,
  openChange: (open) => true,
});

const treeData = computed(() => toTreeData(props.nodes));
const selectedValues = computed(() =>
  (props.modelValue || []).map((value) => ({
    value: String(value),
    label: formatCascaderNodePath(props.nodes, value),
  })),
);
const selectedValue = computed(() => {
  const stored = props.modelValue || [];
  const path =
    stored.length === 1
      ? cascaderPathKeys(props.nodes, stored[0])
      : stored.map(String);
  const value = path.at(-1);
  if (value === undefined || value === null || value === "") return undefined;
  return {
    value: String(value),
    label: formatCascaderPath(props.nodes, path),
  };
});
const treeValue = computed(() =>
  props.multiple ? selectedValues.value : selectedValue.value,
);
const selectedPathsText = computed(() =>
  formatCascaderNodePaths(props.nodes, props.modelValue || []),
);
// The select prop expects a render function, not a string.
const maxTagPlaceholder = computed(() =>
  props.multiple ? () => selectedPathsText.value : undefined,
);

function toTreeData(nodes) {
  return (nodes || []).map((node) => ({
    key: String(node.key),
    value: String(node.key),
    title: node.name,
    children: node.children?.length ? toTreeData(node.children) : undefined,
  }));
}

function onChange(selection) {
  if (props.multiple) {
    const items = Array.isArray(selection) ? selection : [];
    emit(
      "update:modelValue",
      items.map((item) => String(item?.value ?? item)),
    );
  } else {
    const key = selection?.value ?? selection;
    emit(
      "update:modelValue",
      key === undefined || key === null || key === ""
        ? []
        : cascaderPathKeys(props.nodes, key),
    );
  }
  emit("change");
}
</script>

<style scoped lang="less">
.custom-field-cascader {
  min-width: 0;

  :deep(.ant-select-selector),
  :deep(.ant-select-selection-overflow) {
    min-width: 0;
    max-width: 100%;
  }

  :deep(.ant-select-selection-overflow-item) {
    min-width: 0;
    max-width: calc(100% - 16px);
  }

  :deep(.ant-select-selection-placeholder) {
    width: calc(100% - 16px);
  }

  :deep(.ant-select-selection-placeholder),
  :deep(.ant-select-selection-item),
  :deep(.ant-select-selection-item-content) {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}
</style>

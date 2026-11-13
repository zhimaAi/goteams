<template>
  <div class="cascader-tree-nodes" :class="{ nested: depth > 1 }">
    <div v-for="(node, index) in nodes" :key="index" class="cascader-tree-node">
      <div class="cascader-tree-node__row">
        <a-input
          :value="node.name"
          :placeholder="$t('requirement_contentPlaceholder')"
          :maxlength="maxNameLength"
          @update:value="(value) => patch(index, { name: value })"
        />
        <a-button type="text" danger @click="removeAt(index)">
          <DeleteOutlined />
        </a-button>
      </div>
      <CascaderTreeNodes
        v-if="node.children?.length"
        :nodes="node.children"
        :depth="depth + 1"
        @update:nodes="(children) => patch(index, { children })"
      />
      <a-button
        v-if="depth < maxDepth"
        type="dashed"
        size="small"
        class="cascader-tree-node__add"
        @click="addChild(index)"
      >
        <PlusOutlined />
        {{
          $t("settings_fieldCascaderAddLevel", {
            level: $t(`settings_fieldCascaderLevel${depth + 1}`),
          })
        }}
      </a-button>
    </div>
  </div>
</template>

<script setup>
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons-vue";
import {
  CUSTOM_FIELD_CASCADER_MAX_DEPTH,
  CUSTOM_FIELD_CASCADER_NAME_MAX_LENGTH,
} from "@/constants/customField";

defineOptions({ name: "CascaderTreeNodes" });

const props = defineProps({
  nodes: { type: Array, default: () => [] },
  depth: { type: Number, default: 1 },
});

const emit = defineEmits({
  "update:nodes": (nodes) => true,
});

const maxDepth = CUSTOM_FIELD_CASCADER_MAX_DEPTH;
const maxNameLength = CUSTOM_FIELD_CASCADER_NAME_MAX_LENGTH;

function clone() {
  return props.nodes.map((node) => ({
    key: node.key || "",
    name: node.name,
    children: node.children ? [...node.children] : [],
  }));
}

function patch(index, patchValue) {
  const next = clone();
  next[index] = { ...next[index], ...patchValue };
  emit("update:nodes", next);
}

function removeAt(index) {
  const next = clone();
  next.splice(index, 1);
  emit("update:nodes", next);
}

function addChild(index) {
  const next = clone();
  next[index].children = [
    ...(next[index].children || []),
    { name: "", children: [] },
  ];
  emit("update:nodes", next);
}
</script>

<style scoped lang="less">
.cascader-tree-nodes.nested {
  margin: 8px 0 0 16px;
  padding-left: 16px;
  border-left: 1px solid var(--app-border);
}

.cascader-tree-node + .cascader-tree-node {
  margin-top: 8px;
}

.cascader-tree-node__row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.cascader-tree-node__add {
  margin-top: 8px;
}
</style>

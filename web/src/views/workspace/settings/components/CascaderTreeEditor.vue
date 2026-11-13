<!--
 * CascaderTreeEditor.vue
 * Nested name editor for cascader custom fields, max 4 levels.
 -->
<template>
  <div ref="editorRef" class="cascader-tree-editor">
    <div class="cascader-tree-editor__label">
      {{ $t("settings_fieldCascader") }}
    </div>
    <a-typography-text
      v-if="error"
      type="danger"
      class="cascader-tree-editor__error"
    >
      {{ error }}
    </a-typography-text>
    <CascaderTreeNodes
      :nodes="modelValue"
      :depth="1"
      @update:nodes="emit('update:modelValue', $event)"
    />
    <a-button type="dashed" size="small" @click="addRoot">
      <PlusOutlined />
      {{
        $t("settings_fieldCascaderAddLevel", {
          level: $t("settings_fieldCascaderLevel1"),
        })
      }}
    </a-button>
  </div>
</template>

<script setup>
import { ref } from "vue";
import { PlusOutlined } from "@ant-design/icons-vue";
import CascaderTreeNodes from "./CascaderTreeNodes.vue";

const props = defineProps({
  modelValue: { type: Array, default: () => [] },
  error: { type: String, default: "" },
});

const emit = defineEmits({
  "update:modelValue": (nodes) => true,
});

const editorRef = ref(null);

defineExpose({
  focus: () => editorRef.value?.querySelector("input")?.focus(),
});

function addRoot() {
  emit("update:modelValue", [...props.modelValue, { name: "", children: [] }]);
}
</script>

<style scoped lang="less">
.cascader-tree-editor__label {
  margin-bottom: 8px;
  color: rgba(0, 0, 0, 0.88);
}

.cascader-tree-editor__error {
  display: block;
  margin-bottom: 8px;
  font-size: 12px;
}
</style>

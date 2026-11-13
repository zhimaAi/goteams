<template>
  <div class="template-input">
    <a-textarea
      v-if="multiline"
      ref="inputRef"
      :value="modelValue"
      :rows="rows"
      :auto-size="autoSize"
      :placeholder="placeholder"
      :status="status"
      @input="onInput"
    />
    <a-input
      v-else
      ref="inputRef"
      :value="modelValue"
      :placeholder="placeholder"
      :status="status"
      @input="onInput"
    />
    <a-popover v-model:open="open" trigger="click" placement="bottomRight">
      <template #content>
        <div class="variable-panel">
          <a-input
            v-model:value="search"
            size="small"
            :placeholder="$t('automation_variableSearch')"
            allow-clear
          />
          <div class="variable-list">
            <button
              v-for="item in filteredVariables"
              :key="item.key"
              type="button"
              @click="insertVariable(item.key)"
            >
              <span>{{ groupLabel(item.group) }}</span>
              <strong>{{ variableLabel(item) }}</strong>
            </button>
            <a-empty
              v-if="!filteredVariables.length"
              :description="$t('common_noData')"
              :image="simpleImage"
            />
          </div>
        </div>
      </template>
      <button
        type="button"
        class="variable-button"
        :aria-label="$t('automation_insertVariable')"
      >
        { }
      </button>
    </a-popover>
  </div>
</template>

<script setup>
import { computed, nextTick, ref } from "vue";
import { Empty } from "ant-design-vue";
import { useI18n } from "vue-i18n";

const props = defineProps({
  modelValue: { type: String, default: "" },
  variables: { type: Array, default: () => [] },
  multiline: { type: Boolean, default: false },
  rows: { type: Number, default: 3 },
  autoSize: { type: [Boolean, Object], default: false },
  placeholder: { type: String, default: "" },
  status: { type: String, default: undefined },
});
const emit = defineEmits(["update:modelValue"]);
const { t } = useI18n();
const inputRef = ref();
const open = ref(false);
const search = ref("");
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;
const filteredVariables = computed(() => {
  const keyword = search.value.trim().toLowerCase();
  if (!keyword) return props.variables;
  return props.variables.filter((item) =>
    `${item.key} ${variableLabel(item)}`.toLowerCase().includes(keyword),
  );
});

function groupLabel(group) {
  return t(`automation_variableGroup_${group}`);
}
function variableLabel(item) {
  const special = {
    "rule.name": "automation_variableRuleName",
    "execution.id": "automation_variableExecutionId",
  };
  return special[item.key] ? t(special[item.key]) : item.label;
}
function nativeInput() {
  return inputRef.value?.resizableTextArea?.textArea || inputRef.value?.input;
}
function onInput(event) {
  const value = event.target.value;
  emit("update:modelValue", value);
  if (value.slice(0, event.target.selectionStart).endsWith("{{"))
    open.value = true;
}
// 在原生输入框当前光标处插入变量，并保持输入焦点。
function insertVariable(key) {
  const input = nativeInput();
  const start = input?.selectionStart ?? props.modelValue.length;
  const end = input?.selectionEnd ?? start;
  const token = `{{${key}}}`;
  emit(
    "update:modelValue",
    props.modelValue.slice(0, start) + token + props.modelValue.slice(end),
  );
  open.value = false;
  nextTick(() => {
    input?.focus();
    input?.setSelectionRange(start + token.length, start + token.length);
  });
}
</script>

<style scoped lang="less">
.template-input {
  position: relative;

  :deep(.ant-input) {
    padding-right: 42px;
  }
}
.variable-button {
  position: absolute;
  top: 5px;
  right: 7px;
  z-index: 2;
  width: 28px;
  height: 24px;
  padding: 0;
  color: #1677ff;
  font-weight: 700;
  font-family: monospace;
  background: #e6f4ff;
  border: 0;
  border-radius: 4px;
  cursor: pointer;
}
.variable-panel {
  width: 300px;
}
.variable-list {
  max-height: 300px;
  margin-top: 8px;
  overflow-y: auto;

  button {
    display: flex;
    width: 100%;
    padding: 8px;
    text-align: left;
    background: #fff;
    border: 0;
    border-radius: 4px;
    cursor: pointer;

    &:hover {
      background: #f5f7fa;
    }
    span {
      width: 74px;
      color: #8c8c8c;
      font-size: 12px;
    }
    strong {
      overflow: hidden;
      font-size: 13px;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }
}
</style>

<template>
  <a-tooltip
    :title="$t('editor_bold')"
    :placement="tooltipPlacement"
    :get-popup-container="getPopupContainer"
  >
    <a-button
      type="text"
      size="small"
      class="tool-button"
      @mousedown.prevent="$emit('command', 'bold')"
    >
      <BoldOutlined />
    </a-button>
  </a-tooltip>
  <a-tooltip
    :title="$t('editor_italic')"
    :placement="tooltipPlacement"
    :get-popup-container="getPopupContainer"
  >
    <a-button
      type="text"
      size="small"
      class="tool-button"
      @mousedown.prevent="$emit('command', 'italic')"
    >
      <ItalicOutlined />
    </a-button>
  </a-tooltip>
  <a-tooltip
    :title="$t('editor_strikethrough')"
    :placement="tooltipPlacement"
    :get-popup-container="getPopupContainer"
  >
    <a-button
      type="text"
      size="small"
      class="tool-button"
      @mousedown.prevent="$emit('command', 'strikethrough')"
    >
      <StrikethroughOutlined />
    </a-button>
  </a-tooltip>
  <a-tooltip
    :title="$t('editor_bulletList')"
    :placement="tooltipPlacement"
    :get-popup-container="getPopupContainer"
  >
    <a-button
      type="text"
      size="small"
      class="tool-button"
      @mousedown.prevent="$emit('command', 'bullet')"
    >
      <UnorderedListOutlined />
    </a-button>
  </a-tooltip>

  <span class="toolbar-divider" />

  <a-dropdown
    placement="bottomLeft"
    :trigger="['click']"
    :get-popup-container="getPopupContainer"
  >
    <a-button
      type="text"
      size="small"
      class="markdown-insert-button"
      @mousedown="$emit('remember-selection')"
    >
      {{ $t("editor_insert") }} <DownOutlined />
    </a-button>
    <template #overlay>
      <a-menu
        class="markdown-insert-menu"
        @click="$emit('command', $event.key)"
      >
        <a-menu-item key="heading1">H1</a-menu-item>
        <a-menu-item key="heading2">H2</a-menu-item>
        <a-menu-item key="heading3">H3</a-menu-item>
        <a-menu-divider />
        <a-menu-item key="ordered"
          ><OrderedListOutlined /> {{ $t("editor_orderedList") }}</a-menu-item
        >
        <a-menu-item key="task"
          ><CheckSquareOutlined /> {{ $t("editor_taskList") }}</a-menu-item
        >
        <a-menu-item key="quote"
          ><MessageOutlined /> {{ $t("editor_quote") }}</a-menu-item
        >
        <a-menu-item key="code"
          ><CodeOutlined /> {{ $t("editor_codeBlock") }}</a-menu-item
        >
        <a-menu-item key="table"
          ><TableOutlined /> {{ $t("editor_table") }}</a-menu-item
        >
        <a-menu-item key="link"
          ><LinkOutlined /> {{ $t("editor_link") }}</a-menu-item
        >
      </a-menu>
    </template>
  </a-dropdown>

  <a-tooltip
    v-if="allowUploads"
    :title="$t('editor_image')"
    :placement="tooltipPlacement"
    :get-popup-container="getPopupContainer"
  >
    <a-button
      type="text"
      size="small"
      class="tool-button"
      @mousedown="$emit('remember-selection')"
      @click="$emit('choose-image')"
    >
      <PictureOutlined />
    </a-button>
  </a-tooltip>
  <a-tooltip
    :title="
      previewVisible ? $t('editor_hidePreview') : $t('editor_showPreview')
    "
    :placement="tooltipPlacement"
    :get-popup-container="getPopupContainer"
  >
    <a-button
      type="text"
      size="small"
      class="tool-button"
      :class="{ active: previewVisible }"
      @mousedown.prevent="$emit('toggle-preview')"
    >
      <EyeOutlined v-if="previewVisible" />
      <EyeInvisibleOutlined v-else />
    </a-button>
  </a-tooltip>
  <a-popover
    placement="bottom"
    trigger="click"
    :get-popup-container="getPopupContainer"
  >
    <template #content>
      <div class="markdown-help-panel">
        <div>
          <code>**text**</code><span>{{ $t("editor_bold") }}</span>
        </div>
        <div>
          <code>*text*</code><span>{{ $t("editor_italic") }}</span>
        </div>
        <div>
          <code># text</code><span>{{ $t("editor_heading") }}</span>
        </div>
        <div>
          <code>- text</code><span>{{ $t("editor_bulletList") }}</span>
        </div>
        <div>
          <code>&gt; text</code><span>{{ $t("editor_quote") }}</span>
        </div>
        <div>
          <code>```</code><span>{{ $t("editor_codeBlock") }}</span>
        </div>
      </div>
    </template>
    <a-button
      type="text"
      size="small"
      class="tool-button"
      :aria-label="$t('editor_markdownHelp')"
      @mousedown="$emit('remember-selection')"
    >
      <QuestionCircleOutlined />
    </a-button>
  </a-popover>
</template>

<script setup>
import { useI18n } from "vue-i18n";
import {
  BoldOutlined,
  CheckSquareOutlined,
  CodeOutlined,
  DownOutlined,
  EyeInvisibleOutlined,
  EyeOutlined,
  ItalicOutlined,
  LinkOutlined,
  MessageOutlined,
  OrderedListOutlined,
  PictureOutlined,
  QuestionCircleOutlined,
  StrikethroughOutlined,
  TableOutlined,
  UnorderedListOutlined,
} from "@ant-design/icons-vue";

defineProps({
  previewVisible: { type: Boolean, required: true },
  allowUploads: { type: Boolean, default: true },
  tooltipPlacement: { type: String, default: "top" },
  getPopupContainer: { type: Function, required: true },
});
defineEmits([
  "command",
  "choose-image",
  "toggle-preview",
  "remember-selection",
]);
const { t: $t } = useI18n();
</script>

<style scoped lang="less">
.tool-button {
  width: 30px;
  min-width: 30px;
  height: 30px;
  padding: 0;
  color: #595959;
}
.tool-button.active {
  color: #1677ff;
  background: #e6f4ff;
}
.toolbar-divider {
  width: 1px;
  height: 18px;
  margin: 0 4px;
  background: #d9d9d9;
}
.markdown-insert-button {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 30px;
  padding: 0 8px;
  color: #595959;
}
.markdown-help-panel {
  width: 190px;
  color: #596579;
  font-size: 12px;
}
.markdown-help-panel > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  min-height: 28px;
}
.markdown-help-panel code {
  color: #2475fc;
  font-family: Consolas, "Courier New", monospace;
}
:deep(.markdown-insert-menu .ant-menu-item) {
  min-width: 168px;
}
:deep(.markdown-insert-menu .anticon) {
  margin-right: 7px;
}
</style>

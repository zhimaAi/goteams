<template>
  <OverflowToolbar :get-popup-container="getToolbarPopupContainer">
    <span class="toolbar-item" data-toolbar-item data-toolbar-order="1">
      <a-tooltip
        :title="$t('editor_formatPainter')"
        :placement="tooltipPlacement"
        :get-popup-container="getToolbarPopupContainer"
      >
        <a-button
          type="text"
          size="small"
          class="tool-button"
          :class="{ active: formatPainterActive }"
          :aria-label="$t('editor_formatPainter')"
          @mousedown.prevent
          @click="$emit('format-painter')"
        >
          <FormatPainterOutlined />
        </a-button>
      </a-tooltip>
    </span>
    <span class="toolbar-item" data-toolbar-item data-toolbar-order="2">
      <a-tooltip
        :title="$t('editor_clearFormat')"
        :placement="tooltipPlacement"
        :get-popup-container="getToolbarPopupContainer"
      >
        <a-button
          type="text"
          size="small"
          class="tool-button"
          :aria-label="$t('editor_clearFormat')"
          @mousedown.prevent
          @click="$emit('clear-format')"
        >
          <ClearOutlined />
        </a-button>
      </a-tooltip>
    </span>

    <span class="toolbar-item" data-toolbar-item data-toolbar-order="3">
      <a-dropdown
        v-model:open="insertMenuOpen"
        placement="bottomLeft"
        :trigger="['click']"
        :destroyPopupOnHide="true"
        :get-popup-container="getToolbarPopupContainer"
      >
        <a-button
          type="text"
          size="small"
          class="toolbar-menu-button"
          @mousedown.prevent
        >
          <PlusOutlined /> {{ $t("editor_insert") }} <DownOutlined />
        </a-button>
        <template #overlay>
          <a-menu
            class="rich-insert-menu"
            :getPopupContainer="getEditorPopupContainer"
            @click="handleInsertMenuClick"
          >
            <a-sub-menu key="table">
              <template #icon>
                <span class="insert-menu-icon"><TableOutlined /></span>
              </template>
              <template #title>{{ $t("editor_table") }}</template>
              <a-menu-item key="table-size-picker" class="table-size-menu-item">
                <div
                  class="table-size-panel"
                  @mousedown.prevent
                  @click.stop
                  @mouseleave="resetTablePreview"
                >
                  <div class="table-size-value">
                    {{ tablePreviewColumns }} × {{ tablePreviewRows }}
                  </div>
                  <div class="table-size-grid">
                    <button
                      v-for="cell in tablePickerCells"
                      :key="`${cell.row}-${cell.column}`"
                      type="button"
                      class="table-size-cell"
                      :class="{
                        active:
                          cell.row <= tablePreviewRows &&
                          cell.column <= tablePreviewColumns,
                      }"
                      :aria-label="
                        $t('editor_tableSizeAria', {
                          columns: cell.column,
                          rows: cell.row,
                        })
                      "
                      @mouseenter="previewTableSize(cell.row, cell.column)"
                      @click="selectTableSize(cell.row, cell.column)"
                    />
                  </div>
                  <button
                    type="button"
                    class="custom-table-size-button"
                    @click="openCustomTableDialog"
                  >
                    {{ $t("editor_customTableSize") }}
                  </button>
                </div>
              </a-menu-item>
            </a-sub-menu>
            <a-sub-menu key="image">
              <template #icon>
                <span class="insert-menu-icon"><PictureOutlined /></span>
              </template>
              <template #title>{{ $t("editor_insertImage") }}</template>
              <a-menu-item v-if="allowUploads" key="local-image">{{
                $t("editor_localImage")
              }}</a-menu-item>
              <a-menu-item key="network-image">{{
                $t("editor_networkImage")
              }}</a-menu-item>
            </a-sub-menu>
            <a-menu-item v-if="allowUploads" key="file">
              <template #icon>
                <span class="insert-menu-icon"><FileTextOutlined /></span>
              </template>
              {{ $t("editor_insertFile") }}
            </a-menu-item>
            <a-sub-menu key="video">
              <template #icon>
                <span class="insert-menu-icon"><VideoCameraOutlined /></span>
              </template>
              <template #title>{{ $t("editor_video") }}</template>
              <a-menu-item v-if="allowUploads" key="local-video">{{
                $t("editor_localVideo")
              }}</a-menu-item>
              <a-menu-item key="video-link">{{
                $t("editor_videoLink")
              }}</a-menu-item>
            </a-sub-menu>
            <a-menu-divider />
            <a-menu-item key="code-block">
              <template #icon>
                <span class="insert-menu-icon"><CodeOutlined /></span>
              </template>
              {{ $t("editor_codeBlock") }}
            </a-menu-item>
            <a-menu-item key="link">
              <template #icon>
                <span class="insert-menu-icon"><LinkOutlined /></span>
              </template>
              {{ $t("editor_link") }}
            </a-menu-item>
          </a-menu>
        </template>
      </a-dropdown>
    </span>

    <span class="toolbar-item" data-toolbar-item data-toolbar-order="4">
      <a-dropdown
        placement="bottomLeft"
        :trigger="['click']"
        :get-popup-container="getToolbarPopupContainer"
      >
        <a-button
          type="text"
          size="small"
          class="toolbar-menu-button"
          @mousedown.prevent
        >
          <AlignLeftOutlined /> {{ paragraphLabel }} <DownOutlined />
        </a-button>
        <template #overlay>
          <a-menu class="paragraph-menu" @click="handleParagraphClick">
            <a-menu-item key="paragraph">{{
              $t("editor_paragraph")
            }}</a-menu-item>
            <a-menu-divider />
            <a-menu-item v-for="level in 6" :key="`heading-${level}`">
              {{ $t(`editor_heading${level}`) }}
            </a-menu-item>
          </a-menu>
        </template>
      </a-dropdown>
    </span>

    <span class="toolbar-item" data-toolbar-item data-toolbar-order="5">
      <a-select
        class="font-size-select"
        size="small"
        :value="selectedFontSize"
        :options="fontSizeOptions"
        :aria-label="$t('editor_fontSize')"
        @change="$emit('font-size-change', $event)"
      />
    </span>
    <span class="toolbar-item" data-toolbar-item data-toolbar-order="6">
      <a-popover
        v-model:open="colorPickerOpen"
        trigger="click"
        placement="bottomLeft"
        overlay-class-name="font-color-popover"
        :get-popup-container="getEditorPopupContainer"
      >
        <template #content>
          <div class="font-color-panel" @mousedown.stop @click.stop>
            <button
              type="button"
              class="default-color-option"
              @click="selectFontColor(defaultFontColor)"
            >
              <span
                class="font-color-swatch"
                :style="{ background: defaultFontColor }"
              >
                <CheckOutlined v-if="selectedFontColor === defaultFontColor" />
              </span>
              {{ $t("editor_defaultColor") }}
            </button>
            <div class="font-color-grid">
              <button
                v-for="color in fontColorPalette"
                :key="color"
                type="button"
                class="font-color-swatch"
                :class="{ selected: selectedFontColor === color }"
                :style="{
                  background: color,
                  color: isLightFontColor(color) ? '#333' : '#fff',
                }"
                :title="color"
                :aria-label="color"
                @click="selectFontColor(color)"
              >
                <CheckOutlined v-if="selectedFontColor === color" />
              </button>
            </div>
            <div class="recent-color-section">
              <span>{{ $t("editor_recentColors") }}</span>
              <div class="font-color-grid recent">
                <button
                  v-for="color in recentFontColors"
                  :key="color"
                  type="button"
                  class="font-color-swatch"
                  :class="{ selected: selectedFontColor === color }"
                  :style="{
                    background: color,
                    color: isLightFontColor(color) ? '#333' : '#fff',
                  }"
                  :title="color"
                  :aria-label="color"
                  @click="selectFontColor(color)"
                >
                  <CheckOutlined v-if="selectedFontColor === color" />
                </button>
              </div>
            </div>
            <label class="custom-color-option">
              <span>{{ $t("editor_customColor") }}</span>
              <RightOutlined />
              <input
                type="color"
                :value="selectedFontColor"
                :aria-label="$t('editor_customColor')"
                @change="selectFontColor($event.target.value)"
              />
            </label>
          </div>
        </template>
        <a-button
          type="text"
          size="small"
          class="font-color-trigger"
          :style="{ '--font-color': selectedFontColor }"
          :aria-label="$t('editor_fontColor')"
          :title="$t('editor_fontColor')"
        >
          <FontColorsOutlined /><DownOutlined class="font-color-chevron" />
        </a-button>
      </a-popover>
    </span>

    <span
      class="toolbar-divider"
      data-toolbar-item
      data-toolbar-divider
      data-toolbar-order="7"
    >
    </span>
    <span
      v-for="(item, index) in inlineTools"
      :key="item.name"
      class="toolbar-item"
      data-toolbar-item
      :data-toolbar-order="8 + index"
    >
      <a-tooltip
        :title="$t(item.label)"
        :placement="tooltipPlacement"
        :get-popup-container="getToolbarPopupContainer"
      >
        <a-button
          type="text"
          size="small"
          class="tool-button"
          :class="{
            active: editor?.isActive(
              item.name === 'strike' ? 'strike' : item.name,
            ),
          }"
          @mousedown.prevent
          @click="toggleInline(item.name)"
        >
          <component :is="item.icon" />
        </a-button>
      </a-tooltip>
    </span>

    <span
      class="toolbar-divider"
      data-toolbar-item
      data-toolbar-divider
      data-toolbar-order="12"
    >
    </span>
    <span class="toolbar-item" data-toolbar-item data-toolbar-order="13">
      <a-tooltip
        :title="$t('editor_orderedList')"
        :placement="tooltipPlacement"
        :get-popup-container="getToolbarPopupContainer"
      >
        <a-button
          type="text"
          size="small"
          class="tool-button"
          :class="{ active: editor?.isActive('orderedList') }"
          @mousedown.prevent
          @click="editor?.chain().focus().toggleOrderedList().run()"
        >
          <OrderedListOutlined />
        </a-button>
      </a-tooltip>
    </span>
    <span class="toolbar-item" data-toolbar-item data-toolbar-order="14">
      <a-tooltip
        :title="$t('editor_bulletList')"
        :placement="tooltipPlacement"
        :get-popup-container="getToolbarPopupContainer"
      >
        <a-button
          type="text"
          size="small"
          class="tool-button"
          :class="{ active: editor?.isActive('bulletList') }"
          @mousedown.prevent
          @click="editor?.chain().focus().toggleBulletList().run()"
        >
          <UnorderedListOutlined />
        </a-button>
      </a-tooltip>
    </span>
  </OverflowToolbar>

  <a-modal
    v-model:open="customTableDialogOpen"
    :title="$t('editor_customTableSize')"
    :width="360"
    :ok-text="$t('common_confirm')"
    :cancel-text="$t('common_cancel')"
    :z-index="1400"
    @ok="confirmCustomTableSize"
  >
    <div class="custom-table-size-fields">
      <label>
        <span>{{ $t("editor_tableColumns") }}</span>
        <a-input-number v-model:value="customTableColumns" :min="1" :max="50" />
      </label>
      <label>
        <span>{{ $t("editor_tableRows") }}</span>
        <a-input-number v-model:value="customTableRows" :min="1" :max="50" />
      </label>
    </div>
  </a-modal>
</template>

<script setup>
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";
import {
  AlignLeftOutlined,
  BoldOutlined,
  CheckOutlined,
  ClearOutlined,
  CodeOutlined,
  DownOutlined,
  FileTextOutlined,
  FontColorsOutlined,
  FormatPainterOutlined,
  ItalicOutlined,
  LinkOutlined,
  OrderedListOutlined,
  PictureOutlined,
  PlusOutlined,
  RightOutlined,
  StrikethroughOutlined,
  TableOutlined,
  UnderlineOutlined,
  UnorderedListOutlined,
  VideoCameraOutlined,
} from "@ant-design/icons-vue";
import OverflowToolbar from "./OverflowToolbar.vue";

const props = defineProps({
  editor: { type: Object, default: null },
  selectedFontSize: { type: String, required: true },
  selectedFontColor: { type: String, required: true },
  selectedParagraph: { type: String, default: "paragraph" },
  formatPainterActive: { type: Boolean, default: false },
  allowUploads: { type: Boolean, default: true },
  tooltipPlacement: { type: String, default: "top" },
  getPopupContainer: { type: Function, required: true },
  getEditorPopupContainer: { type: Function, required: true },
});

const emit = defineEmits([
  "font-size-change",
  "font-color-change",
  "format-painter",
  "clear-format",
  "insert-table",
  "choose-image",
  "choose-network-image",
  "choose-file",
  "choose-video",
  "insert-video",
  "insert-code-block",
  "open-link",
  "paragraph-change",
]);
const { t: $t } = useI18n();
const colorPickerOpen = ref(false);
const insertMenuOpen = ref(false);
const customTableDialogOpen = ref(false);
const tablePreviewRows = ref(3);
const tablePreviewColumns = ref(3);
const customTableRows = ref(3);
const customTableColumns = ref(3);
const recentFontColors = ref(["#333333"]);
const defaultFontColor = "#333333";
const fontSizeOptions = [12, 14, 16, 18, 20, 24, 28, 32].map((size) => ({
  label: `${size}px`,
  value: `${size}px`,
}));
const fontColorPalette = [
  "#ffffff",
  "#f3f4f6",
  "#e5e7eb",
  "#d1d5db",
  "#9ca3af",
  "#6b7280",
  "#374151",
  "#17233c",
  "#000000",
  "#ff4d4f",
  "#fa8c16",
  "#fadb14",
  "#52c41a",
  "#13c2c2",
  "#1677ff",
  "#2f54eb",
  "#722ed1",
  "#c41d7f",
  "#ffd6d6",
  "#ffe2c2",
  "#fff1b8",
  "#d9f7be",
  "#b5f5ec",
  "#bae0ff",
  "#adc6ff",
  "#d3adf7",
  "#ffd6e7",
  "#ffaaa8",
  "#ffc069",
  "#ffe58f",
  "#95de64",
  "#5cdbd3",
  "#69b1ff",
  "#85a5ff",
  "#b37feb",
  "#f759ab",
  "#cf1322",
  "#d46b08",
  "#d4b106",
  "#389e0d",
  "#08979c",
  "#0958d9",
  "#1d39c4",
  "#531dab",
  "#9e1068",
  "#820014",
  "#873800",
  "#7c5914",
  "#135200",
  "#00474f",
  "#003eb3",
  "#061178",
  "#391085",
  "#780650",
];
const inlineTools = [
  { name: "bold", label: "editor_bold", icon: BoldOutlined },
  { name: "italic", label: "editor_italic", icon: ItalicOutlined },
  { name: "underline", label: "editor_underline", icon: UnderlineOutlined },
  {
    name: "strike",
    label: "editor_strikethrough",
    icon: StrikethroughOutlined,
  },
];
const tablePickerCells = Array.from({ length: 80 }, (_, index) => ({
  row: Math.floor(index / 10) + 1,
  column: (index % 10) + 1,
}));
const paragraphLabel = computed(() =>
  props.selectedParagraph === "paragraph"
    ? $t("editor_paragraph")
    : $t(`editor_heading${props.selectedParagraph.replace("heading-", "")}`),
);

function isLightFontColor(color) {
  const value = Number.parseInt(color.slice(1), 16);
  const red = (value >> 16) & 255;
  const green = (value >> 8) & 255;
  const blue = value & 255;
  return red * 0.299 + green * 0.587 + blue * 0.114 > 170;
}

function getToolbarPopupContainer(trigger) {
  if (trigger.closest(".overflow-toolbar-panel"))
    return props.getEditorPopupContainer();
  return props.getPopupContainer(trigger);
}

function selectFontColor(color) {
  recentFontColors.value = [
    color,
    ...recentFontColors.value.filter((item) => item !== color),
  ].slice(0, 9);
  colorPickerOpen.value = false;
  emit("font-color-change", color);
}

function toggleInline(name) {
  const chain = props.editor?.chain().focus();
  if (!chain) return;
  if (name === "strike") chain.toggleStrike().run();
  else if (name === "bold") chain.toggleBold().run();
  else if (name === "italic") chain.toggleItalic().run();
  else if (name === "underline") chain.toggleUnderline().run();
}

function closeInsertMenu() {
  insertMenuOpen.value = false;
}

function handleInsertMenuClick({ key }) {
  closeInsertMenu();
  if (key === "local-image") emit("choose-image");
  else if (key === "network-image") emit("choose-network-image");
  else if (key === "file") emit("choose-file");
  else if (key === "local-video") emit("choose-video");
  else if (key === "video-link") emit("insert-video");
  else if (key === "code-block") emit("insert-code-block");
  else if (key === "link") emit("open-link");
}

function previewTableSize(rows, columns) {
  tablePreviewRows.value = rows;
  tablePreviewColumns.value = columns;
}

function resetTablePreview() {
  tablePreviewRows.value = 3;
  tablePreviewColumns.value = 3;
}

function selectTableSize(rows, columns) {
  closeInsertMenu();
  emit("insert-table", { rows, columns });
  resetTablePreview();
}

function openCustomTableDialog() {
  customTableRows.value = tablePreviewRows.value;
  customTableColumns.value = tablePreviewColumns.value;
  closeInsertMenu();
  customTableDialogOpen.value = true;
}

function confirmCustomTableSize() {
  const rows = Math.min(50, Math.max(1, Number(customTableRows.value) || 1));
  const columns = Math.min(
    50,
    Math.max(1, Number(customTableColumns.value) || 1),
  );
  customTableDialogOpen.value = false;
  emit("insert-table", { rows, columns });
}

function handleParagraphClick({ key }) {
  emit("paragraph-change", key);
}
</script>

<style scoped lang="less">
:deep(.ant-dropdown-menu-submenu-title) {
  display: flex;
  align-items: center;
}
:deep(.ant-dropdown-menu-submenu-arrow) {
  display: flex;
  align-items: center;
}
.font-size-select {
  flex: 0 0 72px;
  width: 72px;
}
.toolbar-item {
  display: inline-flex;
  align-items: center;
  flex: 0 0 auto;
}
.font-size-select :deep(.ant-select-selector) {
  font-size: 12px;
}
.toolbar-menu-button {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 30px;
  padding: 0 8px;
  color: #595959;
}
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
  flex: 0 0 1px;
  background: #d9d9d9;
}
:deep(.rich-insert-menu .ant-menu-item),
:deep(.rich-insert-menu .ant-menu-submenu-title),
:deep(.paragraph-menu .ant-menu-item) {
  min-width: 172px;
}
.rich-insert-menu {
  --insert-menu-color: #334155;
}
:deep(.rich-insert-menu .ant-menu-item:not(.table-size-menu-item)),
:deep(.rich-insert-menu .ant-menu-submenu-title) {
  display: grid !important;
  align-items: center;
  height: 38px;
  grid-template-columns: 16px minmax(0, 1fr);
  column-gap: 10px;
  color: var(--insert-menu-color) !important;
  line-height: 20px;
}
:deep(.rich-insert-menu .ant-menu-title-content) {
  display: inline-flex;
  align-items: center;
  min-height: 20px;
  margin-inline-start: 0 !important;
  color: inherit;
  line-height: 20px;
}
.insert-menu-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  min-width: 16px;
  height: 16px;
  color: inherit;
  font-size: 16px;
  line-height: 1;
  vertical-align: middle;
}
.insert-menu-icon :deep(svg) {
  display: block;
  width: 16px;
  height: 16px;
}
.insert-menu-icon :deep(.anticon) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  color: inherit;
  line-height: 1;
  vertical-align: 0;
}
:deep(.rich-insert-menu .ant-menu-item-icon) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  min-width: 16px;
  height: 16px;
  margin-inline-end: 0 !important;
  color: inherit !important;
  font-size: 16px;
  line-height: 1;
  vertical-align: middle;
}
:deep(.rich-insert-menu .ant-menu-submenu-arrow) {
  color: inherit !important;
}
:deep(.rich-insert-menu .ant-menu-item:not(.table-size-menu-item):hover),
:deep(.rich-insert-menu .ant-menu-submenu-title:hover) {
  color: #1677ff !important;
}
:deep(.rich-insert-menu .table-size-menu-item) {
  height: auto;
  margin: 0;
  padding: 0;
  background: #fff;
  cursor: default;
  line-height: normal;
}
.table-size-panel {
  width: 202px;
  background: #fff;
  color: #334155;
}
.table-size-value {
  height: 34px;
  color: #596579;
  font-size: 13px;
  line-height: 34px;
  text-align: center;
}
.table-size-grid {
  display: grid;
  width: 181px;
  margin: 0 auto 10px;
  grid-template-columns: repeat(10, 18px);
  grid-template-rows: repeat(8, 18px);
  border-top: 1px solid #cfd7e3;
  border-left: 1px solid #cfd7e3;
}
.table-size-cell {
  width: 18px;
  height: 18px;
  padding: 0;
  border: 0;
  border-right: 1px solid #cfd7e3;
  border-bottom: 1px solid #cfd7e3;
  background: #fff;
  cursor: pointer;
}
.table-size-cell.active {
  border-color: #4d9be8;
  background: #91caff;
}
.custom-table-size-button {
  width: 100%;
  height: 36px;
  padding: 0;
  border: 0;
  border-top: 1px solid #e6eaf1;
  background: #fff;
  color: #596579;
  cursor: pointer;
}
.custom-table-size-button:hover {
  background: #f5f7fa;
  color: #1677ff;
}
.custom-table-size-fields {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  padding: 8px 0;
}
.custom-table-size-fields label {
  display: flex;
  flex-direction: column;
  gap: 8px;
  color: #334155;
}
.custom-table-size-fields :deep(.ant-input-number) {
  width: 100%;
}
.font-color-trigger {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 42px;
  width: 42px;
  height: 30px;
  padding: 0 4px;
  color: #595959;
}
.font-color-trigger::after {
  position: absolute;
  right: 20px;
  bottom: 3px;
  left: 6px;
  height: 3px;
  border: 1px solid rgba(0, 0, 0, 0.12);
  background: var(--font-color);
  content: "";
}
.font-color-chevron {
  margin-left: 5px;
  color: #8c8c8c;
  font-size: 9px;
}
:deep(.font-color-popover .ant-popover-inner) {
  padding: 0;
  border-radius: 4px;
}
.font-color-panel {
  width: 208px;
  color: #334155;
  font-size: 12px;
}
.default-color-option {
  display: flex;
  align-items: center;
  width: 100%;
  height: 36px;
  gap: 8px;
  padding: 0 12px;
  border: 0;
  border-bottom: 1px solid #e8ecf2;
  background: transparent;
  color: #334155;
  cursor: pointer;
  text-align: left;
}
.default-color-option:hover,
.custom-color-option:hover {
  background: #f5f7fa;
}
.font-color-grid {
  display: grid;
  grid-template-columns: repeat(9, 16px);
  gap: 5px;
  padding: 10px 12px;
}
.font-color-grid.recent {
  min-height: 26px;
  padding: 6px 12px 10px;
}
.font-color-swatch {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  padding: 0;
  border: 1px solid rgba(23, 35, 60, 0.12);
  border-radius: 2px;
  cursor: pointer;
  font-size: 10px;
}
.font-color-swatch:hover,
.font-color-swatch.selected {
  outline: 2px solid rgba(36, 117, 252, 0.45);
  outline-offset: 1px;
}
.recent-color-section {
  border-top: 1px solid #e8ecf2;
}
.recent-color-section > span {
  display: block;
  padding: 8px 12px 0;
  color: #7b8798;
}
.custom-color-option {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 38px;
  padding: 0 12px;
  overflow: hidden;
  border-top: 1px solid #e8ecf2;
  cursor: pointer;
}
.custom-color-option input {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  opacity: 0;
  cursor: pointer;
}
</style>

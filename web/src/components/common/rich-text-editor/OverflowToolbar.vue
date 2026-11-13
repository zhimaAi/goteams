<template>
  <div ref="toolbarRoot" class="overflow-toolbar">
    <div ref="primaryContainer" class="overflow-toolbar-primary">
      <slot />
    </div>
    <a-tooltip
      v-if="hasOverflow"
      :title="$t('editor_more')"
      :get-popup-container="getPopupContainer"
    >
      <a-button
        type="text"
        size="small"
        class="overflow-trigger"
        :class="{ active: overflowOpen }"
        :aria-label="$t('editor_more')"
        @mousedown.prevent
        @click="overflowOpen = !overflowOpen"
      >
        <EllipsisOutlined />
      </a-button>
    </a-tooltip>
    <div
      v-show="overflowOpen && hasOverflow"
      ref="overflowContainer"
      class="overflow-toolbar-panel"
    />
  </div>
</template>

<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { EllipsisOutlined } from "@ant-design/icons-vue";

defineProps({
  getPopupContainer: { type: Function, required: true },
});
const { t: $t } = useI18n();
const toolbarRoot = ref(null);
const primaryContainer = ref(null);
const overflowContainer = ref(null);
const hasOverflow = ref(false);
const overflowOpen = ref(false);
let resizeObserver;
let layoutFrame = 0;

function toolbarItems() {
  return [
    ...(primaryContainer.value?.children || []),
    ...(overflowContainer.value?.children || []),
  ]
    .filter((item) => item.hasAttribute("data-toolbar-item"))
    .sort(
      (left, right) =>
        Number(left.dataset.toolbarOrder) - Number(right.dataset.toolbarOrder),
    );
}

function layoutToolbar() {
  if (!toolbarRoot.value || !primaryContainer.value || !overflowContainer.value)
    return;
  const items = toolbarItems();
  items.forEach((item) => primaryContainer.value.append(item));

  const widths = items.map((item) => {
    const style = window.getComputedStyle(item);
    return (
      item.getBoundingClientRect().width +
      Number.parseFloat(style.marginLeft || "0") +
      Number.parseFloat(style.marginRight || "0")
    );
  });
  const requiredWidth =
    widths.reduce((total, width) => total + width, 0) +
    Math.max(0, items.length - 1) * 2;
  const rootWidth = toolbarRoot.value.clientWidth;
  if (requiredWidth <= rootWidth) {
    hasOverflow.value = false;
    overflowOpen.value = false;
    return;
  }

  hasOverflow.value = true;
  const availableWidth = Math.max(0, rootWidth - 32);
  let usedWidth = 0;
  let visibleCount = 0;
  for (let index = 0; index < items.length; index += 1) {
    const nextWidth = widths[index] + (visibleCount ? 2 : 0);
    if (usedWidth + nextWidth > availableWidth) break;
    usedWidth += nextWidth;
    visibleCount += 1;
  }
  while (
    visibleCount > 0 &&
    items[visibleCount - 1]?.hasAttribute("data-toolbar-divider")
  )
    visibleCount -= 1;
  items
    .slice(visibleCount)
    .forEach((item) => overflowContainer.value.append(item));
}

function scheduleLayout() {
  window.cancelAnimationFrame(layoutFrame);
  layoutFrame = window.requestAnimationFrame(layoutToolbar);
}

function handleOutsidePointer(event) {
  if (!overflowOpen.value || toolbarRoot.value?.contains(event.target)) return;
  if (
    event.target.closest(
      ".ant-dropdown, .ant-popover, .ant-select-dropdown, .ant-tooltip",
    )
  )
    return;
  overflowOpen.value = false;
}

function handleEscape(event) {
  if (event.key === "Escape") overflowOpen.value = false;
}

onMounted(async () => {
  await nextTick();
  resizeObserver = new ResizeObserver(scheduleLayout);
  resizeObserver.observe(toolbarRoot.value);
  document.addEventListener("mousedown", handleOutsidePointer);
  document.addEventListener("keydown", handleEscape);
  scheduleLayout();
});

onBeforeUnmount(() => {
  resizeObserver?.disconnect();
  window.cancelAnimationFrame(layoutFrame);
  document.removeEventListener("mousedown", handleOutsidePointer);
  document.removeEventListener("keydown", handleEscape);
});
</script>

<style scoped lang="less">
.overflow-toolbar {
  position: relative;
  display: flex;
  align-items: center;
  min-width: 30px;
  height: 30px;
  flex: 1 1 0;
}
.overflow-toolbar-primary,
.overflow-toolbar-panel {
  display: flex;
  align-items: center;
  gap: 2px;
}
.overflow-toolbar-primary {
  min-width: 0;
  height: 30px;
  overflow: hidden;
  flex: 1;
}
.overflow-trigger {
  width: 30px;
  min-width: 30px;
  height: 30px;
  padding: 0;
  flex: 0 0 30px;
  color: #595959;
}
.overflow-trigger.active {
  background: #e6f4ff;
  color: #1677ff;
}
.overflow-toolbar-panel {
  position: absolute;
  z-index: 20;
  top: 34px;
  right: 0;
  box-sizing: border-box;
  width: max-content;
  max-width: min(440px, calc(100vw - 32px));
  min-height: 38px;
  max-height: 124px;
  padding: 4px 6px;
  overflow: auto;
  flex-wrap: wrap;
  border: 1px solid #e2e8f0;
  border-radius: 4px;
  background: #fff;
  box-shadow: 0 6px 18px rgba(23, 35, 60, 0.14);
}
.overflow-toolbar-panel > :deep([data-toolbar-divider]:first-child) {
  display: none;
}
</style>

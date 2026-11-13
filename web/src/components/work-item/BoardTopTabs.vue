<template>
  <div
    class="board-top-tabs"
    :class="{ 'board-top-tabs--minimal': style === 'minimal' }"
  >
    <a-tabs
      :activeKey="activeKey"
      class="board-tabs"
      @change="(key) => emit('tab-change', key)"
    >
      <a-tab-pane v-for="item in tabs" :key="item.key" :tab="item.label" />
    </a-tabs>
    <BoardNavigationMenu
      :tabs="tabs"
      :all-apps="allApps"
      :available-count="availableCount"
      :more-label="moreLabel"
      :style="style"
      :active-key="activeKey"
      :can-edit="canEdit"
      :can-admin="canAdmin"
      :prevent-member-edit="preventMemberEdit"
      :saving="saving"
      @select="(key) => emit('navigation-select', key)"
      @order-change="(order) => emit('navigation-order-change', order)"
      @style-change="(value) => emit('navigation-style-change', value)"
      @apply-all="() => emit('apply-all')"
      @prevent-change="(value) => emit('prevent-change', value)"
      @app-settings="() => emit('app-settings')"
    />
  </div>
</template>

<script setup>
import BoardNavigationMenu from "./BoardNavigationMenu.vue";

defineProps({
  tabs: { type: Array, default: () => [] },
  allApps: { type: Array, default: () => [] },
  availableCount: { type: Number, default: 0 },
  activeKey: { type: String, default: undefined },
  moreLabel: { type: String, default: "" },
  style: { type: String, default: "simple" },
  canEdit: { type: Boolean, default: false },
  canAdmin: { type: Boolean, default: false },
  preventMemberEdit: { type: Boolean, default: false },
  saving: { type: Boolean, default: false },
});

const emit = defineEmits([
  "tab-change",
  "navigation-select",
  "navigation-order-change",
  "navigation-style-change",
  "apply-all",
  "prevent-change",
  "app-settings",
]);
</script>

<style scoped lang="less">
.board-top-tabs {
  display: contents;
}

.board-tabs {
  align-self: stretch;
  margin-left: 92px;
  margin-bottom: 0;

  :deep(.ant-tabs-nav) {
    height: 46px;
    margin-bottom: 0;
  }

  :deep(.ant-tabs-tab) {
    padding: 12px 12px;
    margin-left: 0 !important;
    font-size: 14px;
    line-height: 22px;
    letter-spacing: -0.0107em;

    &:hover .ant-tabs-tab-btn {
      color: #3157e2;
    }
  }

  :deep(.ant-tabs-tab-btn) {
    color: rgb(0 0 0 / 88%);
  }

  :deep(.ant-tabs-tab-active .ant-tabs-tab-btn) {
    color: #3157e2;
    font-weight: 400;
  }

  :deep(.ant-tabs-ink-bar) {
    height: 2px;
    background: #3157e2;
  }

  :deep(.ant-tabs-nav-operations) {
    display: none;
  }

  :deep(.ant-tabs-nav-list) {
    align-items: center;
    gap: 22px;
  }
}

.board-top-tabs:not(.board-top-tabs--minimal) .board-tabs :deep(.ant-tabs-tab) {
  height: 30px !important;
  padding: 4px 12px !important;
  margin: 0 !important;
  border-radius: 6px !important;
  line-height: 22px !important;
}

.board-top-tabs:not(.board-top-tabs--minimal)
  .board-tabs
  :deep(.ant-tabs-nav-list) {
  gap: 22px;
}

.board-top-tabs:not(.board-top-tabs--minimal)
  .board-tabs
  :deep(.ant-tabs-tab-active) {
  background: #e5efff !important;
}

.board-top-tabs:not(.board-top-tabs--minimal)
  .board-tabs
  :deep(.ant-tabs-tab-active .ant-tabs-tab-btn) {
  color: #3157e2;
}

.board-top-tabs:not(.board-top-tabs--minimal)
  .board-tabs
  :deep(.ant-tabs-ink-bar) {
  display: none;
}

@media (max-width: 768px) {
  .board-tabs {
    order: 3;
    width: 100%;
    margin-left: 0;
  }
}
</style>

<!--
 * ViewDropdown.vue
 * Board top-right view switcher: shows the current view and a dropdown with
 * personal/system view groups plus "new view" and "configure views" entries.
 * Callers owning a fixed builtin view set turn off grouped and showActions to
 * get a flat menu without the create/config entries.
 -->
<template>
  <a-dropdown trigger="click" placement="bottomRight">
    <a-button
      type="text"
      class="view-trigger"
      :class="{
        'view-trigger--plain': isAllView,
        'view-trigger-not-default': currentView && !currentView.is_default,
      }"
    >
      <EyeOutlined />
      <span>{{ $t("view_button") }}{{ showViewName ? ":" : "" }}</span>
      <span
        v-if="showViewName"
        class="view-trigger__name"
        :title="currentView.name"
        >{{ currentView.name }}</span
      >
    </a-button>
    <template #overlay>
      <a-menu
        class="view-menu"
        :selected-keys="selectedKeys"
        @click="handleMenuClick"
      >
        <template v-if="grouped">
          <template v-if="personalViews.length">
            <a-menu-item-group
              :title="`${$t('view_personal_group')} (${personalViews.length})`"
            >
              <a-menu-item
                v-for="view in personalViews"
                :key="`view-${view.id}`"
              >
                {{ view.name }}
              </a-menu-item>
            </a-menu-item-group>
          </template>
          <template v-if="systemViews.length">
            <a-menu-item-group
              :title="`${$t('view_system_group')} (${systemViews.length})`"
            >
              <a-menu-item v-for="view in systemViews" :key="`view-${view.id}`">
                {{ view.name }}
              </a-menu-item>
            </a-menu-item-group>
          </template>
        </template>
        <!-- Ungrouped callers list every enabled view in declaration order. -->
        <template v-else>
          <a-menu-item v-for="view in enabledViews" :key="`view-${view.id}`">
            {{ view.name }}
          </a-menu-item>
        </template>
        <a-menu-divider v-if="showActions" />
        <a-menu-item v-if="showActions" key="create" class="view-menu__action"
          ><PlusOutlined /> {{ $t("view_create") }}</a-menu-item
        >
        <a-menu-item v-if="showActions" key="config" class="view-menu__action"
          ><EditOutlined /> {{ $t("view_config_personal") }}</a-menu-item
        >
      </a-menu>
    </template>
  </a-dropdown>
</template>

<script setup>
import { computed } from "vue";
import { EyeOutlined, PlusOutlined, EditOutlined } from "@ant-design/icons-vue";

const props = defineProps({
  views: { type: Array, default: () => [] },
  currentViewId: { type: [String, Number], default: null },
  // Render the personal/system group titles; false lists every enabled view flat.
  grouped: { type: Boolean, default: true },
  // Render the divider plus the "new view" and "configure views" entries.
  showActions: { type: Boolean, default: true },
});

const emit = defineEmits({
  // Switch to a view, payload is the view id.
  select: (viewId) => true,
  // Open the create-view modal.
  create: () => true,
  // Open the view configuration modal.
  config: () => true,
});

const enabledViews = computed(() => props.views.filter((view) => view.enabled));
const personalViews = computed(() =>
  enabledViews.value.filter((view) => view.view_type === "personal"),
);
const systemViews = computed(() =>
  enabledViews.value.filter((view) => view.view_type === "system"),
);
const currentView = computed(
  () =>
    props.views.find(
      (view) => String(view.id) === String(props.currentViewId),
    ) || null,
);
const selectedKeys = computed(() =>
  props.currentViewId !== null ? [`view-${props.currentViewId}`] : [],
);
const isAllView = computed(
  () => !currentView.value || currentView.value.builtin_key === "all",
);
const showViewName = computed(
  () => Boolean(currentView.value) && !isAllView.value,
);

function handleMenuClick({ key }) {
  if (key === "create") {
    emit("create");
    return;
  }
  if (key === "config") {
    emit("config");
    return;
  }
  if (String(key).startsWith("view-")) {
    emit("select", String(key).slice(5));
  }
}
</script>

<style scoped lang="less">
.view-trigger {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  /* TAPD-style trigger: the whole button reads in primary blue. */
}
.view-trigger-not-default {
  background-color: #f0f6ff !important;
  color: #1677ff !important;
}
.view-trigger--plain {
  color: var(--app-text-secondary);
}
.view-trigger--plain:hover {
  color: #182b50 !important;
}
.view-trigger__name {
  /* Show at most ~7 CJK characters, then ellipsis (full name in tooltip). */
  max-width: 7em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.view-menu {
  min-width: 160px;
  max-height: 420px;
  overflow-y: auto;
}
.view-menu :deep(.ant-menu-item:not(.ant-menu-item-selected)) {
  color: #152646;
}
.view-menu :deep(.view-menu__action) {
  padding-left: 20px;
}
</style>

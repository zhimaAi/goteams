<template>
  <div v-if="projectEnded" class="ended-ribbon">
    <span>{{ $t("project_ended_tag") }}</span>
  </div>
  <div class="board-top">
    <img
      v-if="projectIcon"
      :src="projectIcon"
      alt=""
      class="project-icon-image"
    />
    <span
      v-else
      class="project-icon"
      :style="{ backgroundColor: projectColor, color: iconFgColor }"
    >
      {{ projectChar }}
    </span>
    <a-typography-title :level="4" class="project-name">{{
      workspaceName || $t("common_loading")
    }}</a-typography-title>
    <a-tooltip v-if="canViewProjectSettings" :title="$t('workspace_settings')">
      <a-button type="text" @click="openSettings">
        <SettingOutlined />
      </a-button>
    </a-tooltip>
    <BoardTopTabs
      :tabs="topNavigationTabs"
      :all-apps="navigationAllApps"
      :available-count="availableNavigationTabs.length"
      :more-label="moreNavigationLabel"
      :style="navigationStyle"
      :active-key="activeTab"
      :can-edit="navigationCanEdit"
      :can-admin="navigationIsAdmin"
      :prevent-member-edit="navigationPreventMemberEdit"
      :saving="navigationSaving"
      @tab-change="handleTabChange"
      @navigation-select="handleNavigationSelect"
      @navigation-order-change="handleNavigationOrderChange"
      @navigation-style-change="handleNavigationStyleChange"
      @apply-all="applyNavigationToAllMembers"
      @prevent-change="handlePreventNavigationEditChange"
      @app-settings="openAppSettings"
    />
    <!-- Spacer stays right after the tabs so the category chip is pushed to the right toolbar. -->
    <div class="board-spacer" />
    <a-button
      v-if="activeTab === 'dashboard'"
      type="text"
      class="search-toggle-button"
      :class="{
        'search-toggle-button--expanded': dashboardPanelRef?.searchExpanded,
      }"
      @click="dashboardPanelRef?.toggleSearch()"
    >
      <FilterOutlined /> {{ $t("common_search") }}
    </a-button>
    <div v-if="isWorkItemTab" class="board-top-actions">
      <ViewDropdown
        :views="currentTabViews"
        :current-view-id="currentViewId"
        @select="handleViewSelect"
        @create="openCreateView"
        @config="viewConfigOpen = true"
      />
      <a-button
        type="text"
        class="search-toggle-button"
        :class="{
          'search-toggle-button--expanded': currentSearchExpanded,
          'search-toggle-button--active': currentSearchHasValues,
        }"
        @click="toggleSearchBar"
      >
        <FilterOutlined /> {{ $t("search_button") }}
      </a-button>
      <a-tooltip :title="$t('workspace_listSettings')">
        <a-button
          type="text"
          class="toolbar-settings-button"
          :aria-label="$t('workspace_listSettings')"
          @click="openListSettings"
        >
          <SettingOutlined /> {{ $t("workspace_settingsShort") }}
          <span
            v-if="showListSettingsNotice"
            class="toolbar-settings-notice"
            aria-hidden="true"
          />
        </a-button>
      </a-tooltip>
    </div>
    <!-- Gantt toolbar teleport target: GanttPanel renders view/search/settings here so
           they sit on the tab row like the requirement/defect toolbars (kept via v-show). -->
    <div
      v-show="activeTab === 'gantt'"
      id="gantt-top-actions"
      class="board-top-actions"
    ></div>
    <!-- Iteration toolbar mirrors the work-item toolbar and delegates to IterationPanel. -->
    <div v-if="activeTab === 'iteration'" class="board-top-actions">
      <ViewDropdown
        :views="iterationPanelRef?.views"
        :current-view-id="iterationPanelRef?.currentViewId"
        @select="iterationPanelRef?.selectView($event)"
        @create="iterationPanelRef?.openCreateView()"
        @config="iterationPanelRef?.openViewConfig()"
      />
      <a-button
        type="text"
        class="search-toggle-button"
        :class="{
          'search-toggle-button--expanded': iterationPanelRef?.searchExpanded,
          'search-toggle-button--active': iterationPanelRef?.searchHasValues,
        }"
        @click="iterationPanelRef?.toggleSearch()"
      >
        <FilterOutlined /> {{ $t("search_button") }}
      </a-button>
      <a-tooltip :title="$t('workspace_listSettings')">
        <a-button
          type="text"
          class="toolbar-settings-button"
          :aria-label="$t('workspace_listSettings')"
          @click="iterationPanelRef?.openListSettings()"
        >
          <SettingOutlined /> {{ $t("workspace_settingsShort") }}
        </a-button>
      </a-tooltip>
    </div>
  </div>
</template>
<script setup>
import { ref, computed, watch } from "vue";
import { message } from "ant-design-vue";
import { SettingOutlined, FilterOutlined } from "@ant-design/icons-vue";
import BoardTopTabs from "./BoardTopTabs.vue";
import ViewDropdown from "./ViewDropdown.vue";
import {
  getWorkspaceNavigationSettings,
  saveMyWorkspaceNavigationSettings,
  saveWorkspaceNavigationSettings,
} from "@/api/workspaceNavigation";
const props = defineProps({ model: { type: Object, required: true } });
const {
  t,
  router,
  wsId,
  workItemAttributes,
  boardTabs,
  activeTab,
  boardTabTitle,
  visibleTopTabs,
  handleTabChange,
  workspaceName,
  projectEnded,
  projectIcon,
  projectColor,
  projectChar,
  iconFgColor,
  canViewProjectSettings,
  isWorkItemTab,
  currentTabViews,
  currentViewId,
  handleViewSelect,
  openCreateView,
  viewConfigOpen,
  currentSearchExpanded,
  currentSearchHasValues,
  toggleSearchBar,
  openListSettings,
  showListSettingsNotice,
} = props.model;
const dashboardPanelRef = computed(() => props.model.dashboardPanelRef);
const iterationPanelRef = computed(() => props.model.iterationPanelRef);
function openSettings() {
  router.push({ name: "WorkspaceSettings", params: { wsId: wsId.value } });
}
const navigationOrder = ref([]);
const navigationStyle = ref("simple");
const navigationPreventMemberEdit = ref(false);
const navigationIsAdmin = ref(false);
const navigationSaving = ref(false);
const availableNavigationTabs = computed(() => visibleTopTabs.value);
const navigationTabLabels = computed(() =>
  Object.fromEntries(
    boardTabs.map((tab) => [
      tab.key,
      tab.key === "requirement" || tab.key === "defect"
        ? workItemAttributes.displayName(tab.key)
        : t(boardTabTitle[tab.key] || "common_more"),
    ]),
  ),
);
const topNavigationKeys = computed(() => {
  const available = availableNavigationTabs.value;
  if (!navigationOrder.value.length) return available;
  return navigationOrder.value.filter((key) => available.includes(key));
});
const topNavigationTabs = computed(() =>
  topNavigationKeys.value.map((key) => ({
    key,
    label: navigationTabLabels.value[key],
  })),
);
const navigationAllApps = computed(() =>
  availableNavigationTabs.value.map((key) => ({
    key,
    label: navigationTabLabels.value[key],
  })),
);
const topActiveTab = computed(() =>
  topNavigationKeys.value.includes(activeTab.value)
    ? activeTab.value
    : undefined,
);
const moreNavigationLabel = computed(() => {
  const current = navigationAllApps.value.find(
    (item) => item.key === activeTab.value,
  );
  return current && !topNavigationKeys.value.includes(activeTab.value)
    ? current.label
    : t("common_more");
});
const navigationCanEdit = computed(
  () => navigationIsAdmin.value || !navigationPreventMemberEdit.value,
);

async function loadNavigationSettings() {
  try {
    const settings = await getWorkspaceNavigationSettings(wsId.value);
    navigationOrder.value = Array.isArray(settings?.main_nav)
      ? settings.main_nav
      : [...availableNavigationTabs.value];
    navigationStyle.value =
      settings?.style === "minimal" ? "minimal" : "simple";
    navigationPreventMemberEdit.value = settings?.prevent_member_edit === true;
    navigationIsAdmin.value = settings?.is_admin === true;
  } catch {
    navigationOrder.value = [...availableNavigationTabs.value];
  }
}

// Editing the issue tabs must retain the other workspace applications in main_nav.
function mergeNavigationOrder(order) {
  if (router.currentRoute.value.meta.workItemSurface !== "issue") return order;
  const remaining = [...order];
  const merged = navigationOrder.value.flatMap((key) =>
    availableNavigationTabs.value.includes(key)
      ? remaining.length
        ? [remaining.shift()]
        : []
      : [key],
  );
  return [...merged, ...remaining];
}

function navigationPayload(order = topNavigationKeys.value) {
  return {
    main_nav: mergeNavigationOrder(order),
    style: navigationStyle.value,
  };
}

async function persistNavigation(order, previousOrder) {
  if (!navigationCanEdit.value) return;
  navigationSaving.value = true;
  try {
    if (navigationIsAdmin.value) {
      await saveWorkspaceNavigationSettings(wsId.value, {
        ...navigationPayload(order),
        prevent_member_edit: navigationPreventMemberEdit.value,
        apply_to_all_members: false,
      });
    } else {
      await saveMyWorkspaceNavigationSettings(
        wsId.value,
        navigationPayload(order),
      );
    }
  } catch {
    navigationOrder.value = previousOrder;
  } finally {
    navigationSaving.value = false;
  }
}

function handleNavigationSelect(key) {
  if (!availableNavigationTabs.value.includes(key)) return;
  if (key === activeTab.value) return;
  handleTabChange(key);
}

// Navigate to the app market section of the project settings page.
function openAppSettings() {
  router.push({
    path: `/workspace/${wsId.value}/settings`,
    query: { section: "applications" },
  });
}

function handleNavigationOrderChange(order) {
  if (!navigationCanEdit.value) return;
  const previous = [...navigationOrder.value];
  navigationOrder.value = mergeNavigationOrder(order);
  void persistNavigation(order, previous);
}

function handleNavigationStyleChange(style) {
  if (style !== "simple" && style !== "minimal") return;
  const previous = navigationStyle.value;
  navigationStyle.value = style;
  const order = [...topNavigationKeys.value];
  navigationSaving.value = true;
  const save = navigationIsAdmin.value
    ? saveWorkspaceNavigationSettings(wsId.value, {
        ...navigationPayload(order),
        prevent_member_edit: navigationPreventMemberEdit.value,
        apply_to_all_members: false,
      })
    : saveMyWorkspaceNavigationSettings(wsId.value, navigationPayload(order));
  save
    .catch(() => {
      navigationStyle.value = previous;
    })
    .finally(() => {
      navigationSaving.value = false;
    });
}

async function applyNavigationToAllMembers() {
  if (!navigationIsAdmin.value) return;
  navigationSaving.value = true;
  try {
    await saveWorkspaceNavigationSettings(wsId.value, {
      ...navigationPayload([...topNavigationKeys.value]),
      prevent_member_edit: navigationPreventMemberEdit.value,
      apply_to_all_members: true,
    });
    message.success(t("common_saveSuccess"));
  } finally {
    navigationSaving.value = false;
  }
}

function handlePreventNavigationEditChange(value) {
  if (!navigationIsAdmin.value) return;
  const previous = navigationPreventMemberEdit.value;
  navigationPreventMemberEdit.value = Boolean(value);
  navigationSaving.value = true;
  saveWorkspaceNavigationSettings(wsId.value, {
    ...navigationPayload([...topNavigationKeys.value]),
    prevent_member_edit: navigationPreventMemberEdit.value,
    apply_to_all_members: false,
  })
    .catch(() => {
      navigationPreventMemberEdit.value = previous;
    })
    .finally(() => {
      navigationSaving.value = false;
    });
}

watch(wsId, loadNavigationSettings, { immediate: true });
</script>
<style scoped lang="less">
@import "./workItemBoard.less";
</style>

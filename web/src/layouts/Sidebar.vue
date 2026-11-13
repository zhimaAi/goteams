<template>
  <a-layout-sider
    :width="220"
    class="sidebar"
    :class="{ 'sidebar-dark': isDarkNav, 'sidebar-light': !isDarkNav }"
    :style="{ backgroundColor: siderBg, borderRightColor: siderBorder }"
  >
    <div class="sidebar-inner">
      <!-- Brand -->
      <RouterLink class="sidebar-logo" to="/">
        <img :src="sidebarLogo" class="app-logo" alt="" />
        <span class="logo-title">{{
          brandingStore.current.app_name || "GoTeams"
        }}</span>
      </RouterLink>

      <!-- Navigation menu -->
      <a-menu
        mode="inline"
        :theme="navTheme"
        :selectedKeys="selectedKeys"
        class="sidebar-menu"
        :class="{
          'has-community-entry': showUserCommunity,
          'has-enterprise-entry': showEnterpriseEntry,
          'has-license-reminder': licenseStore.needsReminder,
        }"
        @click="handleMenuClick"
      >
        <a-menu-item key="my-work">
          <RouterLink class="menu-link" to="/my-work" @click.stop>
            <img :src="myWorkIcon" class="nav-icon" alt="" />
            <span class="menu-link-label">{{ $t("menu_myWork") }}</span>
          </RouterLink>
        </a-menu-item>

        <a-menu-item v-if="canViewTeamWork" key="team-work">
          <RouterLink class="menu-link" to="/team-work" @click.stop>
            <img :src="teamWorkIcon" class="nav-icon" alt="" />
            <span class="menu-link-label">{{ $t("menu_teamWork") }}</span>
          </RouterLink>
        </a-menu-item>

        <a-menu-item key="pipelines">
          <RouterLink class="menu-link" to="/pipelines" @click.stop>
            <RobotOutlined class="menu-icon" />
            <span class="menu-link-label">{{ $t("menu_pipeline") }}</span>
          </RouterLink>
        </a-menu-item>

        <a-menu-item-group key="workspaces-group">
          <template #title>
            <div class="workspace-group-label">
              <span>{{ $t("workspace_sidebarTitle") }}</span>
              <div class="workspace-group-actions">
                <button
                  v-if="canCreateProject"
                  type="button"
                  class="workspace-group-add"
                  :class="{ disabled: authStore.isEnterpriseExpired }"
                  :disabled="authStore.isEnterpriseExpired"
                  :aria-label="$t('workspace_createProject')"
                  :title="
                    authStore.isEnterpriseExpired
                      ? $t('serviceExpired_title')
                      : ''
                  "
                  @click.stop="openCreateProject"
                >
                  <PlusOutlined />
                </button>
                <a-dropdown
                  :trigger="['hover', 'click']"
                  placement="bottomLeft"
                >
                  <button
                    type="button"
                    class="workspace-group-add"
                    :aria-label="$t('workspace_projectActions')"
                    @click.stop
                  >
                    <EllipsisOutlined />
                  </button>
                  <template #overlay>
                    <a-menu @click="handleWorkspaceAction">
                      <a-menu-item key="sort">
                        <HolderOutlined /> {{ $t("workspace_projectOrder") }}
                      </a-menu-item>
                    </a-menu>
                  </template>
                </a-dropdown>
              </div>
            </div>
          </template>

          <a-menu-item v-for="w in workspaces" :key="`ws-${w.id}`">
            <RouterLink
              class="menu-link"
              :to="workspaceEntryPath(w.id, w.workspace_type)"
              @click.stop
            >
              <img
                v-if="workspaceStore.iconUrls[w.id]"
                :src="workspaceStore.iconUrls[w.id]"
                alt=""
                class="ws-icon-image"
              />
              <span
                v-else
                class="ws-icon"
                :style="{
                  backgroundColor: w.color || '#0d68ff',
                  color: '#fff',
                }"
              >
                {{ w.name.trim().charAt(0) }}
              </span>
              <span class="menu-link-label">{{ w.name }}</span>
            </RouterLink>
          </a-menu-item>
        </a-menu-item-group>
      </a-menu>

      <!-- Bottom user area -->
      <div class="sidebar-footer">
        <div
          v-if="showEnterpriseEntry"
          class="community-entry"
          role="button"
          tabindex="0"
          @click="goEnterpriseAddMember"
          @keydown.enter="goEnterpriseAddMember"
        >
          <TeamOutlined class="community-entry-icon" />
          <span class="community-entry-label">{{
            $t("enterprise_management")
          }}</span>
          <span class="community-entry-tag">{{
            $t("workspace_addMember")
          }}</span>
          <button
            type="button"
            class="community-entry-close"
            :aria-label="$t('common_close')"
            :title="$t('common_close')"
            @click.stop="dismissEnterpriseEntry"
          >
            <CloseOutlined />
          </button>
        </div>
        <div
          v-if="showUserCommunity && !licenseStore.managementOnly"
          class="community-entry"
          role="button"
          tabindex="0"
          @click="communityModalOpen = true"
          @keydown.enter="communityModalOpen = true"
          @keydown.space.prevent="communityModalOpen = true"
        >
          <MessageOutlined class="community-entry-icon" />
          <span class="community-entry-label">{{
            $t("home_footer_group_title")
          }}</span>
          <button
            type="button"
            class="community-entry-close"
            :aria-label="$t('common_close')"
            :title="$t('common_close')"
            @click.stop="dismissCommunityEntry"
          >
            <CloseOutlined />
          </button>
        </div>

        <!-- License expiry reminder (tenant owner only, 15 days before expiry) -->
        <div
          v-if="licenseStore.needsReminder && authStore.isSuperAdmin()"
          class="license-reminder"
          role="button"
          tabindex="0"
          @click="licenseUpdateOpen = true"
          @keydown.enter="licenseUpdateOpen = true"
          @keydown.space.prevent="licenseUpdateOpen = true"
        >
          <SafetyCertificateOutlined class="license-reminder-icon" />
          <span class="license-reminder-label">
            {{ $t("license_reminder_days", { n: licenseStore.remainingDays }) }}
          </span>
        </div>

        <div class="footer-user-row">
          <a-dropdown
            :trigger="['click']"
            placement="topRight"
            @openChange="handleUserMenuOpenChange"
          >
            <a-space class="user-area">
              <a-avatar
                :size="32"
                :src="authStore.avatarUrl || undefined"
                :style="{
                  backgroundColor: authStore.avatarUrl ? undefined : '#3157e2',
                }"
              >
                <UserOutlined v-if="!authStore.avatarUrl" />
              </a-avatar>
              <span class="user-name" :style="{ color: textColor }">{{
                userName
              }}</span>
            </a-space>
            <template #overlay>
              <a-menu @click="handleUserMenuClick">
                <a-menu-item v-if="!licenseStore.managementOnly" key="profile">
                  <UserOutlined /> {{ $t("menu_profile") }}
                </a-menu-item>
                <a-menu-item v-if="canViewEnterpriseManagement" key="members">
                  <TeamOutlined /> {{ $t("enterprise_management") }}
                </a-menu-item>
                <a-menu-divider />
                <a-menu-item key="navTheme" class="no-hover-bg" @click.stop>
                  <div class="dropdown-row" @click.stop>
                    <span>{{ $t("common_navTheme") }}</span>
                    <a-segmented
                      :value="themeStore.mode"
                      size="small"
                      :options="navThemeOptions"
                      @change="handleNavThemeChange"
                    />
                  </div>
                </a-menu-item>
                <a-menu-item key="language" class="no-hover-bg" @click.stop>
                  <div class="dropdown-row" @click.stop>
                    <span>{{ $t("common_language") }}</span>
                    <a-segmented
                      :value="localeStore.currentLang"
                      size="small"
                      :options="langOptions"
                      @change="(v) => localeStore.setLang(v)"
                    />
                  </div>
                </a-menu-item>
                <a-menu-divider />
                <a-menu-item key="logout" danger>
                  <LogoutOutlined /> {{ $t("auth_logout") }}
                </a-menu-item>
              </a-menu>
            </template>
          </a-dropdown>
          <div class="notification-actions">
            <a-badge
              v-if="notificationStore.unreadCount > 0"
              :count="
                notificationStore.unreadCount > 99
                  ? '99+'
                  : notificationStore.unreadCount
              "
              :offset="[-2, 2]"
            >
              <button
                type="button"
                class="notification-button"
                :aria-label="$t('notification_title')"
                @click="openNotifications"
              >
                <BellOutlined />
              </button>
            </a-badge>
            <button
              v-else
              type="button"
              class="notification-button"
              :aria-label="$t('notification_title')"
              @click="openNotifications"
            >
              <BellOutlined />
            </button>
          </div>
        </div>
      </div>

      <!-- New Project Dialog -->
      <CreateWorkspaceModal v-model:open="createModalOpen" />
      <WorkspaceOrderModal
        v-model:open="orderModalOpen"
        :workspaces="workspaces"
        :icon-urls="workspaceStore.iconUrls"
        :can-apply-to-all="canManageProjects"
        :saving="orderSaving"
        @save="saveWorkspaceOrder"
      />
      <UserCommunityModal v-model:open="communityModalOpen" />

      <!-- License update dialog (shared with the license page; contains its own
           customer-service entry) -->
      <LicenseUpdateModal
        v-if="authStore.isSuperAdmin()"
        v-model:open="licenseUpdateOpen"
      />
    </div>
  </a-layout-sider>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, watch } from "vue";
import { useRouter, useRoute } from "vue-router";
import { useI18n } from "vue-i18n";
import { useLocaleStore } from "@/stores/locale";
import { useAuthStore } from "@/stores/auth";
import { useWorkspaceStore } from "@/stores/workspace";
import { useThemeStore } from "@/stores/theme";
import { useNotificationStore } from "@/stores/notifications";
import { sortEnterpriseProjects } from "@/api/workspace";
import { workspaceEntryPath } from "@/composables/useWorkspaceApplications";
import { PROJECT_ORDER_CHANGED_EVENT } from "@/utils/projectOrder";
import { message } from "ant-design-vue";
import {
  UserOutlined,
  TeamOutlined,
  PlusOutlined,
  EllipsisOutlined,
  HolderOutlined,
  LogoutOutlined,
  RobotOutlined,
  MessageOutlined,
  CloseOutlined,
  BellOutlined,
  SafetyCertificateOutlined,
} from "@ant-design/icons-vue";
import CreateWorkspaceModal from "@/components/layout/CreateWorkspaceModal.vue";
import WorkspaceOrderModal from "@/layouts/WorkspaceOrderModal.vue";
import UserCommunityModal from "@/components/layout/UserCommunityModal.vue";
import LicenseUpdateModal from "@/components/license/LicenseUpdateModal.vue";
import { useLicenseStore } from "@/stores/license";
import myWorkIcon from "@/image/left-nav/Icon (2).png";
import teamWorkIcon from "@/image/left-nav/Icon.png";
import appLogo from "@/image/logo.svg";
import { useBrandingStore } from "@/stores/branding";
import lightAppLogo from "@/image/logo.svg";

const COMMUNITY_ENTRY_DISMISSED_KEY = "goteams-community-entry-dismissed";
const ENTERPRISE_ENTRY_DISMISSED_KEY = "goteams-enterprise-entry-dismissed";

const { t: $t } = useI18n();
const localeStore = useLocaleStore();
const authStore = useAuthStore();
const brandingStore = useBrandingStore();
watch(
  () => authStore.token,
  (token) => {
    brandingStore.clearTenant();
    if (token) brandingStore.loadTenant();
  },
  { immediate: true },
);
const workspaceStore = useWorkspaceStore();
const themeStore = useThemeStore();
const notificationStore = useNotificationStore();
const licenseStore = useLicenseStore();

const router = useRouter();
const route = useRoute();

const createModalOpen = ref(false);
const orderModalOpen = ref(false);
const orderSaving = ref(false);
const communityModalOpen = ref(false);
const licenseUpdateOpen = ref(false);
const loggingOut = ref(false);
const showUserCommunity = ref(true);
const enterpriseEntryDismissed = ref(false);

const isDarkNav = computed(() => themeStore.isDark);
const sidebarLogo = computed(
  () =>
    brandingStore.current.logo_url ||
    (isDarkNav.value ? appLogo : lightAppLogo),
);
const navTheme = computed(() => (isDarkNav.value ? "dark" : "light"));
const siderBg = computed(() => (isDarkNav.value ? "#001829" : "#fff"));
const siderBorder = computed(() => (isDarkNav.value ? "#102b3e" : "#e8edf3"));
const textColor = computed(() =>
  isDarkNav.value ? "rgba(255, 255, 255, 0.85)" : "#344054",
);

const workspaceId = computed(() => route.params.wsId);
const workspaces = computed(() => workspaceStore.workspaces);
const canCreateProject = computed(() =>
  authStore.hasEnterprisePermission("project:create"),
);
const canManageProjects = computed(() =>
  authStore.hasEnterprisePermission("project:manage"),
);
const canViewTeamWork = computed(() =>
  [
    "team_work:view_today",
    "team_work:view_yesterday",
    "team_work:view_tomorrow",
    "team_work:view_custom_date",
    "team_work:view_unscheduled",
  ].some((permission) => authStore.hasEnterprisePermission(permission)),
);
const canViewEnterpriseManagement = computed(
  () =>
    licenseStore.managementOnly ||
    [
      "personalization:setting",
      "plan:view",
      "member:view",
      "member:view_role_list",
      "storage:view",
      "external_notification:manage",
    ].some((permission) => authStore.hasEnterprisePermission(permission)),
);
const showEnterpriseEntry = computed(
  () => canViewEnterpriseManagement.value && !enterpriseEntryDismissed.value,
);
onMounted(() => {
  if (licenseStore.managementOnly) return;
  workspaceStore.fetchWorkspaces().catch(() => {});
  authStore.loadAvatar().catch(() => {});
  notificationStore.startPolling();
});

onBeforeUnmount(() => notificationStore.stopPolling());

watch(
  workspaces,
  (list) => {
    list.forEach((workspace) => {
      if (workspace.icon) {
        workspaceStore.loadWorkspaceIcon(workspace.id).catch(() => {});
      }
    });
  },
  { immediate: true, deep: true },
);

const selectedKeys = computed(() => {
  const path = route.path;
  if (path.includes("/my-work")) return ["my-work"];
  if (path.includes("/team-work")) return ["team-work"];
  if (path.includes("/pipelines")) return ["pipelines"];
  if (path === "/notifications") return [];
  if (workspaceId.value) return [`ws-${workspaceId.value}`];
  return ["my-work"];
});

const userName = computed(
  () => authStore.userInfo?.name || $t("profile_defaultName"),
);

watch(
  () => authStore.userInfo?.id,
  () => {
    showUserCommunity.value =
      window.localStorage.getItem(getCommunityEntryStorageKey()) !== "1";
    enterpriseEntryDismissed.value =
      window.localStorage.getItem(getEnterpriseEntryStorageKey()) === "1";
  },
  { immediate: true },
);

// If no workspace is selected or the selected one is an invalid ID, navigate to the first automatically.
watch(
  () => workspaceStore.workspaces,
  (list) => {
    if (list.length > 0 && route.path.startsWith("/workspace/")) {
      const currentId = workspaceId.value;
      const isValid =
        currentId && currentId !== "0" && !currentId.startsWith("ws-");
      if (!isValid) {
        router.replace(workspaceEntryPath(list[0].id));
      }
    }
  },
  { immediate: true },
);
const navThemeOptions = computed(() => [
  { label: $t("common_darkTheme"), value: "dark" },
  { label: $t("common_lightTheme"), value: "light" },
]);
const langOptions = computed(() => [
  { label: $t("common_chinese"), value: "zh-CN" },
  { label: "EN", value: "en-US" },
]);

function handleMenuClick({ key }) {
  if (key === "my-work") {
    router.push("/my-work");
    return;
  }
  if (key === "team-work") {
    router.push("/team-work");
    return;
  }
  if (key === "pipelines") {
    router.push("/pipelines");
    return;
  }
  if (key.startsWith("ws-")) {
    const targetId = key.replace("ws-", "");
    router.push(workspaceEntryPath(targetId));
  }
}

async function handleUserMenuClick({ key }) {
  if (key === "profile") {
    router.push("/profile");
  }
  if (key === "members") {
    router.push("/members?tab=members");
  }
  if (key === "logout") {
    if (loggingOut.value) return;
    loggingOut.value = true;
    try {
      const result = await authStore.logout();
      if (result?.login_entry_url) {
        // Use the same tenant entry as the SSO settings, including its scope_id.
        window.location.replace(result.login_entry_url);
      } else {
        await router.push("/login");
      }
    } finally {
      loggingOut.value = false;
    }
  }
}

function handleUserMenuOpenChange(open) {
  if (open) {
    authStore.fetchUserInfo().catch(() => {});
  }
}

function handleNavThemeChange(val) {
  themeStore.setMode(val);
}

function getCommunityEntryStorageKey() {
  const userId = authStore.userInfo?.id;
  return userId
    ? `${COMMUNITY_ENTRY_DISMISSED_KEY}:${userId}`
    : COMMUNITY_ENTRY_DISMISSED_KEY;
}

function getEnterpriseEntryStorageKey() {
  const userId = authStore.userInfo?.id;
  return userId
    ? `${ENTERPRISE_ENTRY_DISMISSED_KEY}:${userId}`
    : ENTERPRISE_ENTRY_DISMISSED_KEY;
}

function goEnterpriseAddMember() {
  router.push("/members?tab=members&addMember=1");
}

function dismissCommunityEntry() {
  window.localStorage.setItem(getCommunityEntryStorageKey(), "1");
  showUserCommunity.value = false;
}

function dismissEnterpriseEntry() {
  window.localStorage.setItem(getEnterpriseEntryStorageKey(), "1");
  enterpriseEntryDismissed.value = true;
}

function openNotifications() {
  router.push("/notifications");
}

// Utility functions.
function openCreateProject() {
  if (authStore.isEnterpriseExpired) return;
  createModalOpen.value = true;
}

function handleWorkspaceAction({ key }) {
  if (key === "sort") orderModalOpen.value = true;
}

async function saveWorkspaceOrder(payload) {
  orderSaving.value = true;
  try {
    await sortEnterpriseProjects(payload);
    await workspaceStore.fetchWorkspaces(true);
    window.dispatchEvent(new Event(PROJECT_ORDER_CHANGED_EVENT));
    message.success($t("project_manage_sort_success"));
    orderModalOpen.value = false;
  } finally {
    orderSaving.value = false;
  }
}

function isLightColor(hex) {
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  return (0.299 * r + 0.587 * g + 0.114 * b) / 255 > 0.6;
}

function lightenColor(hex, ratio = 0.3) {
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  const mix = (c) => Math.round(c + (255 - c) * ratio);
  return `#${mix(r).toString(16).padStart(2, "0")}${mix(g).toString(16).padStart(2, "0")}${mix(b).toString(16).padStart(2, "0")}`;
}
</script>

<style scoped lang="less">
.sidebar {
  --sidebar-bg: #fff;
  --sidebar-panel: #f7f9fc;
  --sidebar-border: #e8edf3;
  --sidebar-text: #667085;
  --sidebar-text-active: #3157e2;
  --sidebar-group-text: #98a2b3;
  --sidebar-hover: #f2f5fb;
  --sidebar-selected: #e8eefb;
  --sidebar-logo-text: #182b50;
  --sidebar-logo-filter: none;
  --sidebar-icon-filter: brightness(0) saturate(100%);

  height: 100vh;
  position: fixed;
  left: 0;
  top: 0;
  bottom: 0;
  z-index: 10;
  border-right: 1px solid;
  overflow: hidden;
  background: var(--sidebar-bg);
}

.sidebar-dark {
  --sidebar-bg: #001829;
  --sidebar-panel: #12213c;
  --sidebar-border: #26364d;
  --sidebar-text: #fff;
  --sidebar-text-active: #fff;
  --sidebar-group-text: #728298;
  --sidebar-hover: rgba(255, 255, 255, 0.06);
  --sidebar-selected: #405069;
  --sidebar-logo-text: rgba(255, 255, 255, 0.9);
  --sidebar-logo-filter: brightness(0) invert(1);
  --sidebar-icon-filter: none;
}

.sidebar-inner {
  display: flex;
  flex-direction: column;
  height: 100%;
  position: relative;
}

.sidebar-logo {
  height: 64px;
  padding: 0 13px 0 16px;
  border-bottom: 1px solid var(--sidebar-border);
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  background: var(--sidebar-panel);
  text-decoration: none;
}

.app-logo {
  width: 24px;
  height: 24px;
  object-fit: contain;
  flex-shrink: 0;
}

.logo-title {
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--sidebar-logo-text);
  font-size: 16px;
  font-weight: 600;
  line-height: 1;
  white-space: nowrap;
}

.sidebar-menu {
  flex: 1;
  border-right: 0;
  overflow: auto;
  padding-bottom: 56px;
  padding-top: 3px;
  background: transparent;

  // Hide the scrollbar while keeping the element scrollable.
  scrollbar-width: none; // Firefox
  -ms-overflow-style: none; // IE/Edge (legacy)

  &::-webkit-scrollbar {
    width: 0;
    height: 0;
    display: none; // Chrome/Safari/Edge
  }

  &.has-community-entry,
  &.has-enterprise-entry {
    padding-bottom: 100px;
  }

  &.has-license-reminder {
    padding-bottom: 100px;
  }

  &.has-community-entry.has-license-reminder {
    padding-bottom: 144px;
  }

  &.has-community-entry.has-enterprise-entry {
    padding-bottom: 144px;
  }

  :deep(.ant-menu-item) {
    width: calc(100% - 24px);
    height: 44px;
    margin: 0 12px;
    border-radius: 8px;
    color: var(--sidebar-text);
    // color: var(--sidebar-text-active);
    line-height: 44px;
    transition:
      color 0.2s,
      background-color 0.2s;
  }

  :deep(.ant-menu-item:hover) {
    color: var(--sidebar-text-active);
  }

  :deep(.ant-menu-item-selected) {
    color: var(--sidebar-text-active);
    background: var(--sidebar-selected);
  }

  :deep(.ant-menu-item-selected::after) {
    display: none;
  }

  // The padding moves to the inner <a> so the whole row stays one link target.
  &:not(.ant-menu-inline-collapsed) {
    :deep(.ant-menu-item) {
      padding-inline: 0 !important;
    }
  }

  // `.ant-menu-title-content` carries no ant styling in 4.x, so it can host the full-row <a>.
  :deep(.ant-menu-title-content) {
    display: flex;
    min-width: 0;
  }

  :deep(.ant-menu-item-group-title) {
    height: 41px;
    padding: 7px 20px 4px !important;
    color: var(--sidebar-group-text);
    font-size: 12px;
    font-weight: 400;
    line-height: 20px;
    overflow: visible;
    text-overflow: clip;
    white-space: normal;
  }
}

// Full-row link: keeps ant's menu look while giving the item real <a> semantics.
.menu-link {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  height: 44px;
  padding: 0 16px;
  box-sizing: border-box;
  border-radius: 8px;
  color: inherit;
  text-decoration: none;
  font-size: 14px;
}

.menu-link-label {
  min-width: 0;
  overflow: hidden;
  line-height: 20px;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-left: 0 !important;
}

.menu-icon {
  flex-shrink: 0;
  font-size: 14px;
}

.nav-icon {
  display: block;
  width: 14px;
  height: 14px;
  flex-shrink: 0;
  object-fit: contain;
  filter: var(--sidebar-icon-filter);
}

:deep(.ant-menu-item:hover) .nav-icon,
:deep(.ant-menu-item-selected) .nav-icon {
  opacity: 1;
}

.workspace-group-label {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
}

.workspace-group-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}

.workspace-group-add {
  width: 24px;
  height: 24px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  cursor: pointer;
  transition:
    background-color 0.2s,
    opacity 0.2s;
  flex-shrink: 0;
  font-size: 16px;
  color: var(--sidebar-text);
  transform: rotate(90deg);
  transform-origin: center center;

  &:hover {
    background: var(--sidebar-hover);
    opacity: 1;
  }

  img {
    display: block;
    width: 14px;
    height: 14px;
    filter: var(--sidebar-icon-filter);
  }

  &.disabled {
    cursor: not-allowed;
    opacity: 0.3;
    pointer-events: none;
  }
}

.ws-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 700;
  line-height: 24px;
  flex-shrink: 0;
}

.ws-icon-image {
  width: 24px;
  height: 24px;
  border-radius: 6px;
  object-fit: cover;
  flex-shrink: 0;
}

.sidebar-footer {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  border-top: 1px solid;
  border-top-color: var(--sidebar-border);
  padding: 0;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  background: var(--sidebar-panel);
}

.community-entry {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  height: 44px;
  padding: 0 12px 0 20px;
  color: var(--sidebar-text);
  border-bottom: 1px solid var(--sidebar-border);
  cursor: pointer;
  outline: none;
  transition:
    color 0.2s,
    background-color 0.2s;

  &:hover,
  &:focus-visible,
  &:focus-within {
    color: var(--sidebar-text-active);
    background: var(--sidebar-hover);
  }

  &:hover .community-entry-close,
  &:focus-visible .community-entry-close,
  &:focus-within .community-entry-close {
    opacity: 1;
    visibility: visible;
  }
}

.community-entry-icon {
  flex-shrink: 0;
  font-size: 15px;
}

.community-entry-label {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  font-size: 14px;
  line-height: 20px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.community-entry-tag {
  flex-shrink: 0;
  margin-left: auto;
  padding: 1px 8px;
  border-radius: 4px;
  color: #fff;
  background: #0d68ff;
  font-size: 12px;
  line-height: 18px;
}

.community-entry-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  padding: 0;
  flex-shrink: 0;
  color: inherit;
  border: 0;
  border-radius: 4px;
  background: transparent;
  cursor: pointer;
  opacity: 0;
  visibility: hidden;
  transition:
    opacity 0.2s,
    background-color 0.2s;

  &:hover,
  &:focus-visible {
    background: var(--sidebar-selected);
    opacity: 1;
    visibility: visible;
  }
}

.license-reminder {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  min-height: 44px;
  padding: 6px 12px 6px 20px;
  color: #d46b08;
  border-bottom: 1px solid var(--sidebar-border);
  cursor: pointer;
  outline: none;
  transition:
    color 0.2s,
    background-color 0.2s;

  &:hover,
  &:focus-visible {
    background: var(--sidebar-hover);
  }
}

.license-reminder-icon {
  flex-shrink: 0;
  font-size: 15px;
}

.license-reminder-label {
  min-width: 0;
  flex: 1;
  font-size: 13px;
  line-height: 20px;
}

.footer-user-row {
  display: flex;
  align-items: center;
  padding-right: 10px;
}

.user-area {
  cursor: pointer;
  min-width: 0;
  flex: 1;
  height: 56px;
  padding: 12px 16px;
  box-sizing: border-box;
}

.user-name {
  font-size: 14px;
}

.notification-actions {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  gap: 6px;
}

.notification-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--sidebar-text);
  cursor: pointer;
  font-size: 17px;
}

.notification-button:hover {
  background: var(--sidebar-hover);
  color: var(--sidebar-text-active);
}

.dropdown-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 220px;

  span {
    margin-right: 8px;
  }
}

.user-menu-entry-tag {
  display: inline-block;
  margin-left: 6px;
  padding: 1px 8px;
  border-radius: 4px;
  color: #fff;
  background: #0d68ff;
  font-size: 12px;
  line-height: 18px;
  vertical-align: 1px;
}

:deep(.no-hover-bg) {
  &:hover {
    background: transparent !important;
  }
}
</style>

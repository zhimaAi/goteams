<template>
  <div class="settings-page">
    <div class="settings-left-column">
      <div class="settings-page-header">
        <div class="settings-title-row">
          <button
            type="button"
            class="settings-back-button"
            @click="handleBack"
          >
            <ArrowLeftOutlined />
            <span>{{ $t("common_back") }}</span>
          </button>
          <a-typography-title :level="3" class="settings-page-title">
            {{ $t("settings_title") }}
          </a-typography-title>
        </div>
        <a-dropdown v-if="canEndProject" :trigger="['click']">
          <a-button size="small"><MoreOutlined /></a-button>
          <template #overlay>
            <a-menu @click="handleToggleEnded">
              <a-menu-item key="toggleEnded">
                {{
                  isEnded
                    ? $t("project_settings_reopen")
                    : $t("project_settings_end")
                }}
              </a-menu-item>
            </a-menu>
          </template>
        </a-dropdown>
      </div>

      <nav class="settings-sidebar" :aria-label="$t('settings_title')">
        <a-skeleton
          v-if="applicationsLoading && !applicationsReady"
          active
          :title="false"
          :paragraph="{ rows: 3 }"
        />
        <template v-else>
          <section
            v-for="group in visibleGroups"
            :key="group.key"
            class="settings-menu-group"
          >
            <div class="settings-menu-heading">
              <component :is="group.icon" />
              <span>{{ $t(group.label) }}</span>
            </div>
            <button
              v-for="sec in group.sections"
              :key="sec.key"
              type="button"
              class="settings-menu-item"
              :class="{ active: activeSection === sec.key }"
              @click="selectSection(sec.key)"
            >
              {{ sec.menuName ? sec.menuName() : $t(sec.label) }}
            </button>
          </section>
        </template>
      </nav>
    </div>
    <div class="settings-content-body">
      <slot
        :section="activeSection"
        :ws-id="wsId"
        :select-section="selectSection"
        :on-application-state="handleApplicationState"
        :load-enabled-applications="loadEnabledApplications"
      />
    </div>
  </div>
</template>
<script setup>
import { computed, reactive, ref, watch, watchEffect } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { Modal, message } from "ant-design-vue";
import { useWorkItemTypeAttributes } from "@/composables/useWorkItemTypeAttributes";
import {
  AppstoreOutlined,
  ArrowLeftOutlined,
  InfoCircleOutlined,
  MoreOutlined,
  ShopOutlined,
} from "@ant-design/icons-vue";
import {
  getWorkspace,
  getWorkspaceMetadata,
  updateWorkspaceEnded,
} from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";
import { useWorkspaceStore } from "@/stores/workspace";
import {
  syncWorkspaceApplications,
  setWorkspaceApplicationEnabled,
  useWorkspaceApplications,
} from "@/composables/useWorkspaceApplications";

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const workspaceStore = useWorkspaceStore();
const activeSection = ref("");
const issueSurface = computed(() => route.meta.workItemSurface === "issue");
const endedAt = ref(0);
const wsId = computed(() => route.params.wsId);
const {
  applications: enabledApplications,
  workspaceType,
  ready: applicationsReady,
  loading: applicationsLoading,
} = useWorkspaceApplications(wsId);
// Menu names follow the per-workspace work item attribute settings, falling back
// to the legacy built-in labels until (or unless) the workspace configures them.
const workItemAttributes = useWorkItemTypeAttributes(wsId);
const typeMenuNames = reactive({ requirement: "", defect: "" });
watchEffect(() => {
  typeMenuNames.requirement =
    workItemAttributes.attributes.value.requirement?.name ||
    t("settings_requirement");
  typeMenuNames.defect =
    workItemAttributes.attributes.value.defect?.name || t("settings_defect");
});
const isEnded = computed(() => Number(endedAt.value) > 0);
const issueApplicationEnabled = computed(() =>
  enabledApplications.value.includes("issue"),
);
const planningAppEnabled = computed(() =>
  enabledApplications.value.includes("planning"),
);
const defectAppEnabled = computed(() =>
  enabledApplications.value.includes("defect"),
);

const hasAny = (keys) => keys.some((key) => authStore.hasPermission(key));
const canProjectInfo = computed(() =>
  authStore.hasPermission("project_info:edit"),
);
const canProjectMembers = computed(() =>
  hasAny([
    "project_member:add",
    "project_member:remove",
    "project_member:change_role",
  ]),
);
const canProjectRoles = computed(() =>
  hasAny(["project_role:add", "project_role:edit", "project_role:delete"]),
);
const canRequirementSettings = computed(() =>
  hasAny([
    "requirement_setting:field",
    "requirement_setting:status",
    "requirement_setting:priority",
    "requirement_setting:attribute",
  ]),
);
// Defect sits under "App Management" alongside "App Market" like planning: it is only
// visible when the defect app is enabled in the app market.
const canDefectSettings = computed(
  () =>
    defectAppEnabled.value &&
    hasAny([
      "defect_setting:field",
      "defect_setting:status",
      "defect_setting:priority",
      "defect_setting:attribute",
    ]),
);
// Planning's attribute fields/status/priority are all controlled by planning:edit.
// The planning section is only visible when the planning app is enabled in the app market.
const canPlanningSettings = computed(
  () => planningAppEnabled.value && authStore.hasPermission("planning:edit"),
);
const canSLA = computed(
  () => workspaceType.value === "issue" && hasAny(["sla:view", "sla:manage"]),
);
const canAutomation = computed(() =>
  hasAny(["automation:view", "automation:manage"]),
);
const canEndProject = computed(() => authStore.hasPermission("workspace:end"));
const canIssueInviteManage = computed(() =>
  authStore.hasPermission("ticket:manage"),
);

const groups = [
  {
    key: "basic",
    label: "settings_group_basic",
    icon: InfoCircleOutlined,
    sections: [
      { key: "info", visible: canProjectInfo, label: "settings_projectInfo" },
      {
        key: "members",
        visible: canProjectMembers,
        label: "settings_projectMembers",
      },
      {
        key: "roles",
        visible: canProjectRoles,
        label: "settings_projectRoles",
      },
    ],
  },
  {
    key: "builtin",
    label: "settings_group_builtin",
    icon: AppstoreOutlined,
    sections: [
      {
        key: "requirement",
        visible: canRequirementSettings,
        label: "settings_requirement",
        menuName: () => typeMenuNames.requirement,
      },
      {
        key: "automation",
        visible: canAutomation,
        label: "settings_automation",
      },
      { key: "sla", visible: canSLA, label: "sla_title" },
    ],
  },
  {
    key: "applications",
    label: "settings_group_applications",
    icon: ShopOutlined,
    sections: [
      {
        key: "applications",
        visible: computed(() =>
          hasAny(["application_market:view", "application_market:manage"]),
        ),
        label: "settings_apps",
      },
      {
        key: "issue-invite",
        visible: computed(
          () => issueApplicationEnabled.value && canIssueInviteManage.value,
        ),
        label: "app_issue_invite",
      },
      // Planning sits under "App Management" alongside "App Market" and is
      // only visible when the planning app is enabled in the app market.
      {
        key: "planning",
        visible: canPlanningSettings,
        label: "settings_planning",
      },
      // Defect shares the app switch rules of planning/issue-invite: it only shows up
      // while the defect app is enabled in the app market.
      {
        key: "defect",
        visible: canDefectSettings,
        label: "settings_defect",
        menuName: () => typeMenuNames.defect,
      },
    ],
  },
];
const visibleGroups = computed(() =>
  groups
    .map((group) => ({
      ...group,
      sections: group.sections.filter((sec) => sec.visible.value),
    }))
    .filter((group) => group.sections.length),
);
const menuSections = computed(() =>
  visibleGroups.value.flatMap((group) => group.sections),
);

// Turning the defect app off while its own settings page is open would leave the user on
// a section whose menu entry just disappeared, so send them back to the app market. This
// runs before the URL normalization below, otherwise that watcher would already have
// dropped them on the first visible section.
watch(defectAppEnabled, (enabled) => {
  if (!enabled && activeSection.value === "defect")
    selectSection("applications");
});

// The section lives in the URL query (?section=...); a query pointing at an
// invisible section falls back to the first visible one. URL normalization
// waits until the application metadata loads, otherwise a ?section=sla
// link is replaced by the fallback before the section becomes visible.
// The template detail route renders inside this layout with no ?section=;
// keep the invite section highlighted so the sidebar matches the page.
watchEffect(() => {
  if (issueSurface.value) {
    activeSection.value = "issue-invite";
    return;
  }
  if (applicationsLoading.value && !applicationsReady.value) {
    activeSection.value = "";
    return;
  }
  const querySection =
    typeof route.query.section === "string" ? route.query.section : "";
  const target = menuSections.value.some((sec) => sec.key === querySection)
    ? querySection
    : menuSections.value[0]?.key || "";
  if (!target) {
    activeSection.value = "";
    return;
  }
  if (target !== activeSection.value) activeSection.value = target;
  if (
    applicationsReady.value &&
    workspaceType.value &&
    target !== querySection
  ) {
    router.replace({ query: { ...route.query, section: target } });
  }
});

function handleBack() {
  if (issueSurface.value) {
    router.push({ name: "IssueInviteManage", params: { wsId: wsId.value } });
    return;
  }

  // Prefer real history (e.g. entered from the board); fall back to the board page
  // when the settings page was opened directly (no in-app history to return to).
  // if (window.history.state?.back) {
  //   router.back();
  //   return;
  // }
  router.push({ path: `/workspace/${wsId.value}/board` });
}

function selectSection(key) {
  if (key === "issue-invite") {
    router.push({ name: "IssueSettings", params: { wsId: wsId.value } });
    return;
  }
  if (issueSurface.value) {
    router.push({
      name: "WorkspaceSettings",
      params: { wsId: wsId.value },
      query: { section: key },
    });
    return;
  }
  if (key !== activeSection.value) {
    router.replace({ query: { ...route.query, section: key } });
  }
}

async function handleApplicationState({ appKey, enabled }) {
  if (appKey !== "issue") return;
  setWorkspaceApplicationEnabled(wsId.value, appKey, !!enabled);
  // Refresh because ticket permissions are hidden by the backend while the app is disabled.
  await authStore.refreshProjectPermissions(wsId.value);
}

// Refresh after app market changes; initial rendering uses the shared workspace cache.
let workspaceLoadSequence = 0;
async function loadEnabledApplications() {
  const sequence = ++workspaceLoadSequence;
  const currentWsId = wsId.value;
  try {
    const metadata = await getWorkspaceMetadata(currentWsId);
    if (sequence === workspaceLoadSequence && currentWsId === wsId.value) {
      syncWorkspaceApplications(
        currentWsId,
        metadata?.enabled_applications || [],
        metadata?.workspace?.workspace_type || "",
      );
    }
  } catch {
    // Keep the last confirmed switches; the request interceptor reports the failure.
  }
}

// Fetch endedAt only when the user can end the project (avoids unnecessary requests).
let endedLoadSequence = 0;
watch(
  [wsId, canEndProject],
  async ([currentWsId, canEnd]) => {
    const sequence = ++endedLoadSequence;
    if (!canEnd) {
      endedAt.value = 0;
      return;
    }
    try {
      const workspace = await getWorkspace(currentWsId);
      if (sequence === endedLoadSequence) {
        endedAt.value = Number(workspace?.ended_at || 0);
      }
    } catch {
      if (sequence === endedLoadSequence) endedAt.value = 0;
    }
  },
  { immediate: true },
);

function handleToggleEnded() {
  if (isEnded.value) {
    applyEnded(false);
    return;
  }
  Modal.confirm({
    title: t("project_settings_end"),
    content: t("project_settings_end_confirm"),
    okText: t("common_confirm"),
    cancelText: t("common_cancel"),
    onOk: () => applyEnded(true),
  });
}

// Ending a project only hides it from project lists, so stay on the settings page and refresh the sidebar.
async function applyEnded(ended) {
  await updateWorkspaceEnded(wsId.value, { ended });
  endedAt.value = ended ? Math.floor(Date.now() / 1000) : 0;
  message.success(
    ended
      ? t("project_manage_end_success")
      : t("project_manage_reopen_success"),
  );
  await workspaceStore.fetchWorkspaces(true);
}
</script>
<style scoped lang="less">
.settings-page {
  display: flex;
  flex-direction: row;
  flex: 1;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  background: #fff;
  padding-top: 12px;
}

.settings-left-column {
  display: flex;
  flex-direction: column;
  width: 212px;
  flex-shrink: 0;
  min-height: 0;
  border-right: 1px solid var(--app-border, #f0f0f0);
}

.settings-page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
  flex-shrink: 0;
  padding: 0 12px 0 12px;
  min-height: 38px;
}

.settings-title-row {
  display: flex;
  align-items: center;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.settings-back-button {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
  padding: 2px 6px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: #646a73;
  font: inherit;
  font-size: 13px;
  cursor: pointer;
  transition:
    background 0.15s,
    color 0.15s;

  &:hover {
    background: #f2f3f5;
    color: #172b4d;
  }
}

.settings-page-title {
  margin: 0 !important;
  overflow: hidden;
  color: var(--app-text, #1f1f1f);
  font-size: 16px !important;
  line-height: 24px !important;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.settings-sidebar {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 16px 10px 24px 0;
}

.settings-menu-group + .settings-menu-group {
  margin-top: 18px;
}

.settings-menu-heading {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 36px;
  padding: 0 12px;
  color: #172b4d;
  font-size: 14px;
  font-weight: 600;
}

.settings-menu-item {
  display: block;
  width: calc(100% - 16px);
  height: 40px;
  margin: 2px 0 2px 16px;
  padding: 0 14px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: #172b4d;
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition:
    background 0.15s,
    color 0.15s;

  &:hover {
    background: #f2f3f5;
  }

  &.active {
    background: #e6f4ff;
    color: #1677ff;
  }
}

.settings-content-body {
  flex: 1;
  min-width: 0;
  min-height: 0;
  padding: 0 16px 24px 30px;
  overflow-y: auto;
  overflow-x: hidden;
  background: #fff;
}

@media (max-width: 900px) {
  .settings-left-column {
    width: 180px;
  }

  .settings-content-body {
    padding-left: 20px;
  }
}
</style>

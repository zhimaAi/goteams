<template>
  <div class="app-market-panel">
    <a-spin :spinning="loading">
      <div class="market-head">
        <a-typography-title :level="5" class="market-title">
          {{ $t("settings_apps") }}
        </a-typography-title>
        <p class="market-desc">{{ $t("app_market_desc") }}</p>
      </div>
      <div class="app-market-list">
        <div
          v-for="app in apps"
          :key="app.app_key"
          class="app-card"
          :class="{ 'app-card--coming-soon': app.coming_soon }"
        >
          <!-- Top colored region: icon + name + switch (+ optional config button) -->
          <div class="app-card-top">
            <div class="app-icon">
              <component :is="app.icon" />
            </div>
            <h3 class="app-name">{{ app.name }}</h3>
            <div class="app-card-actions">
              <button
                v-if="
                  app.app_key === 'planning' &&
                  app.enabled &&
                  authStore.hasPermission('planning:edit')
                "
                type="button"
                class="app-config-btn"
                @click="goPlanningConfig"
              >
                {{ $t("app_market_config") }}
              </button>
              <button
                v-if="
                  app.app_key === 'defect' && app.enabled && canDefectManage
                "
                type="button"
                class="app-config-btn"
                @click="goDefectConfig"
              >
                {{ $t("app_market_config") }}
              </button>
              <button
                v-if="app.app_key === 'issue' && app.enabled && canIssueManage"
                type="button"
                class="app-config-btn"
                @click="$emit('configure', app.app_key)"
              >
                {{ $t("common_settings") }}
              </button>
              <!-- Coming-soon apps: the switch stays disabled visually, but an absolute
                   overlay on top captures clicks to show the "coming soon" toast. The
                   disabled antd switch itself does not bind click handlers. -->
              <div
                v-if="app.coming_soon"
                class="app-switch-coming-soon"
                :title="$t('app_coming_soon')"
                @click="notifyComingSoon"
              >
                <a-switch
                  :checked="false"
                  :un-checked-children="$t('common_off')"
                  disabled
                />
              </div>
              <a-switch
                v-else
                :checked="app.enabled"
                :checked-children="$t('common_on')"
                :un-checked-children="$t('common_off')"
                :loading="app.saving"
                :disabled="!canManage || app.confirming"
                @change="(checked) => handleToggle(app, checked)"
              />
            </div>
          </div>
          <!-- Bottom white region: description -->
          <p class="app-desc">{{ app.description }}</p>
        </div>
        <a-empty
          v-if="!loading && !apps.length"
          :description="$t('common_empty')"
        />
      </div>
    </a-spin>
  </div>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { message, Modal } from "ant-design-vue";
import {
  BarChartOutlined,
  BookOutlined,
  BugOutlined,
  DashboardOutlined,
  FormOutlined,
  ProfileOutlined,
  RetweetOutlined,
} from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import {
  listWorkspaceApplications,
  updateWorkspaceApplication,
} from "@/api/workspaceApplication";
import { useAuthStore } from "@/stores/auth";
import {
  setWorkspaceApplicationEnabled,
  syncWorkspaceApplications,
} from "@/composables/useWorkspaceApplications";
import { typeSettingPermission } from "@/utils/typePermission";

const props = defineProps({
  wsId: {
    type: [String, Number],
    required: true,
  },
});
const emit = defineEmits(["application-state", "configure", "changed"]);

const { t } = useI18n();
const authStore = useAuthStore();
const router = useRouter();
const canManage = computed(() =>
  authStore.hasPermission("application_market:manage"),
);
const canIssueManage = computed(() => authStore.hasPermission("ticket:manage"));
// Defect settings are split per action like requirements; offering "配置" without any of
// them would send the user to a settings page that hides the section again.
const canDefectManage = computed(() =>
  ["field", "status", "priority", "attribute"].some((action) =>
    authStore.hasPermission(typeSettingPermission("defect", action)),
  ),
);
const loading = ref(false);
const apps = ref([]);
let loadSequence = 0;

// App icon mapping (colors are unified to the Figma primary #3157E2).
const appMeta = {
  planning: { icon: ProfileOutlined },
  defect: { icon: BugOutlined },
  iteration: { icon: RetweetOutlined },
  gantt: { icon: BarChartOutlined },
  knowledge_base: { icon: BookOutlined },
  dashboard: { icon: DashboardOutlined },
  issue: { icon: FormOutlined },
};

const enableConfirmDescKeys = {
  issue: "app_market_enable_confirm_desc_issue",
};

const disableConfirmDescKeys = {
  issue: "app_market_disable_confirm_desc_issue",
};

async function loadApps() {
  const sequence = ++loadSequence;
  loading.value = true;
  try {
    const list = await listWorkspaceApplications(props.wsId);
    if (sequence !== loadSequence) return;
    apps.value = (list || []).map((item) => ({
      ...item,
      icon: appMeta[item.app_key]?.icon || BookOutlined,
      enabled: Number(item.enabled) === 1,
      coming_soon: !!item.coming_soon,
      saving: false,
      confirming: false,
    }));
    for (const item of apps.value) {
      emit("application-state", {
        appKey: item.app_key,
        enabled: item.enabled,
      });
    }
    // Share the loaded switches so other surfaces read this answer instead of refetching.
    syncWorkspaceApplications(
      props.wsId,
      apps.value.filter((item) => item.enabled).map((item) => item.app_key),
    );
  } catch {
    if (sequence === loadSequence) apps.value = [];
    // The unified interceptor already reports the failure.
  } finally {
    if (sequence === loadSequence) loading.value = false;
  }
}

// Coming-soon apps show a toast when clicked; the switch itself is disabled and swallows
// pointer events, so the surrounding overlay captures the click.
function notifyComingSoon() {
  message.info(t("app_coming_soon"));
}

function handleToggle(app, checked) {
  if (!canManage.value) return;
  // Coming-soon apps are disabled at the switch; this guard is a backend defense-in-depth.
  if (app.coming_soon) {
    message.info(t("app_coming_soon"));
    return;
  }
  const previous = app.enabled;
  app.enabled = checked;
  app.confirming = true;
  Modal.confirm({
    title: checked
      ? t("app_market_enable_confirm_title", { name: app.name })
      : t("app_market_disable_confirm_title", { name: app.name }),
    content: checked
      ? t(
          enableConfirmDescKeys[app.app_key] ||
            "app_market_enable_confirm_desc",
        )
      : t(
          disableConfirmDescKeys[app.app_key] ||
            "app_market_disable_confirm_desc",
        ),
    okText: t("common_confirm"),
    cancelText: t("common_cancel"),
    onOk: () => applyToggle(app, checked, previous),
    onCancel: () => {
      app.enabled = previous;
      app.confirming = false;
    },
  });
}

async function applyToggle(app, checked, previous) {
  if (!canManage.value) {
    app.enabled = previous;
    app.confirming = false;
    return;
  }
  app.saving = true;
  try {
    await updateWorkspaceApplication(props.wsId, app.app_key, {
      enabled: checked,
    });
    app.enabled = checked;
    // Publish the new switch so every surface gated by it updates without a refetch.
    setWorkspaceApplicationEnabled(props.wsId, app.app_key, checked);
    emit("application-state", { appKey: app.app_key, enabled: checked });
    message.success(
      checked
        ? t("app_market_enable_success", { name: app.name })
        : t("app_market_disable_success", { name: app.name }),
    );
    // Notify the parent so dependent UI (e.g. the settings sidebar menu) refreshes
    // without requiring a full page reload.
    emit("changed");
  } catch {
    app.enabled = previous;
    // The unified interceptor already reports the failure.
  } finally {
    app.saving = false;
    app.confirming = false;
  }
}

// Jump to the project settings > planning section to configure planning fields/status/priority.
function goPlanningConfig() {
  router.push(`/workspace/${props.wsId}/settings?section=planning`);
}

// Jump to the project settings > defect section (work item attribute configuration:
// name, status, priority, ...).
function goDefectConfig() {
  router.push(`/workspace/${props.wsId}/settings?section=defect`);
}

watch(() => props.wsId, loadApps, { immediate: true });
</script>

<style scoped lang="less">
.app-market-panel {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.market-head {
  margin-bottom: 20px;
}

.market-title {
  margin: 0 0 8px !important;
  font-size: 18px;
  font-weight: 600;
  color: #1f1f1f;
}

.market-desc {
  margin: 0;
  font-size: 13px;
  color: #8c8c8c;
}

.app-market-list {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
}

.app-card {
  display: flex;
  flex-direction: column;
  width: 274px;
  background: #fff;
  border: 1px solid #f0f0f0;
  border-radius: 12px;
  overflow: hidden;
  transition:
    box-shadow 0.2s,
    transform 0.2s;

  &:hover {
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
    transform: translateY(-1px);
  }
}

.app-card--coming-soon {
  opacity: 0.75;
}

.app-card-top {
  position: relative;
  display: flex;
  align-items: center;
  gap: 12px;
  height: 84px;
  padding: 0 16px;
  background: #e5efff;
}

.app-icon {
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  font-size: 20px;
  color: #fff;
  background: #3157e2;
}

.app-name {
  margin: 0;
  flex: 1;
  min-width: 0;
  font-size: 16px;
  font-weight: 600;
  color: #16243a;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.app-card-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.app-config-btn {
  padding: 1px 8px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: #3157e2;
  font-size: 14px;
  line-height: 22px;
  cursor: pointer;
  transition: background 0.15s;

  &:hover {
    background: rgba(49, 87, 226, 0.08);
  }
}

// Wrapper around a disabled coming-soon switch. The disabled antd switch does not
// bind click handlers, so an ::after overlay is absolutely positioned over the switch
// to guarantee click capture for the "coming soon" toast. cursor: not-allowed hints state.
.app-switch-coming-soon {
  position: relative;
  cursor: not-allowed;

  &::after {
    content: "";
    position: absolute;
    inset: 0;
  }
}

.app-desc {
  margin: 0;
  padding: 16px;
  font-size: 14px;
  line-height: 22px;
  color: #8c8c8c;
}
</style>

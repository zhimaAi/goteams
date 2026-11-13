<template>
  <a-empty v-if="!appEnabled" :description="$t('issue_invite_disabled')" />
  <section v-else class="issue-invite-panel">
    <div class="issue-invite-banner">
      <div class="issue-invite-banner__main">
        <span class="issue-invite-banner__icon">
          <LinkOutlined />
        </span>
        <div class="issue-invite-banner__texts">
          <div class="issue-invite-banner__head">
            <h3 class="issue-invite-banner__title">
              {{ $t("issue_invite_usage_title") }}
            </h3>
            <p class="issue-invite-banner__line">
              <i18n-t keypath="issue_invite_openid_hint">
                <template #code>
                  <span class="issue-invite-banner__code">openid=XX</span>
                </template>
              </i18n-t>
            </p>
          </div>
          <p class="issue-invite-banner__hint">
            <span class="issue-invite-banner__label">
              {{ $t("issue_invite_general_link_label") }}
            </span>
            <span>{{ $t("issue_invite_general_link_hint") }}</span>
            <span class="issue-invite-banner__divider">|</span>
            <span class="issue-invite-banner__label">
              {{ $t("issue_invite_template_link_label") }}
            </span>
            <span>{{ $t("issue_invite_template_link_hint") }}</span>
          </p>
        </div>
      </div>
      <div class="issue-invite-banner__actions">
        <a-button
          v-if="canManage"
          class="issue-invite-banner__settings"
          @click="openGeneralSettings"
        >
          <template #icon><SettingOutlined /></template>
          {{ $t("common_settings") }}
        </a-button>
        <a-button
          v-if="canCopyLink"
          type="primary"
          class="issue-invite-banner__copy"
          @click="copyGeneralLink"
        >
          <template #icon><ShareAltOutlined /></template>
          {{ $t("issue_invite_copy_general_link") }}
        </a-button>
      </div>
    </div>

    <h3 class="issue-invite-panel__title">
      {{ $t("issue_invite_template_list") }}
    </h3>
    <a-empty
      v-if="!templates.length"
      :description="$t('issue_invite_no_template')"
    />
    <div v-else class="issue-invite-panel__list">
      <div
        v-for="item in templates"
        :key="item.template_key"
        class="issue-invite-panel__row"
      >
        <WorkItemTypePill
          :type="item.type_key"
          :ws-id="wsId"
          size="large"
          class="issue-invite-type-pill"
        />
        <strong class="issue-invite-panel__name">{{ item.name }}</strong>
        <div class="issue-invite-panel__actions">
          <a-button
            v-if="canManage"
            class="issue-invite-panel__btn"
            @click="openSettings(item)"
          >
            <template #icon><SettingOutlined /></template>
            {{ $t("common_settings") }}
          </a-button>
          <a-button
            v-if="canCopyLink"
            class="issue-invite-panel__btn issue-invite-panel__btn--copy"
            :class="{ 'is-copied': copiedKey === item.template_key }"
            @click="copyTemplateLink(item)"
          >
            <template #icon><CopyOutlined /></template>
            {{
              copiedKey === item.template_key
                ? $t("common_copied")
                : $t("issue_invite_copy_link")
            }}
          </a-button>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { message } from "ant-design-vue";
import {
  CopyOutlined,
  LinkOutlined,
  SettingOutlined,
  ShareAltOutlined,
} from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import { useAuthStore } from "@/stores/auth";
import {
  getIssueInviteShare,
  listIssueInviteTemplates,
} from "@/api/issueInvite";
import WorkItemTypePill from "@/components/common/WorkItemTypePill.vue";
import { useWorkspaceApplications } from "@/composables/useWorkspaceApplications";

const route = useRoute();
const wsId = computed(() => route.params.wsId);
const { applications } = useWorkspaceApplications(wsId);
const appEnabled = computed(() => applications.value.includes("issue"));

const { t } = useI18n();
const router = useRouter();
const authStore = useAuthStore();
const templates = ref([]);
const generalUrl = ref("");
const inviteKey = ref("");
// Template key currently showing the "copied" state; cleared by a 2s timer.
const copiedKey = ref("");
const COPIED_RESET_MS = 2000;
let copiedTimer = null;

const canCopyLink = computed(() => authStore.hasPermission("ticket:copy_link"));
const canManage = computed(() => authStore.hasPermission("ticket:manage"));

let loadSeq = 0;

function markCopied(key) {
  copiedKey.value = key;
  clearTimeout(copiedTimer);
  copiedTimer = setTimeout(() => {
    copiedKey.value = "";
  }, COPIED_RESET_MS);
}

onBeforeUnmount(() => {
  clearTimeout(copiedTimer);
});

function currentOrigin() {
  return `${window.location.origin}${import.meta.env?.BASE_URL || "/"}`;
}

function buildInviteUrl(key) {
  if (!key) return "";
  return `${currentOrigin()}invite/i/${encodeURIComponent(key)}?open_mode=common`;
}

function buildTemplateUrl(key) {
  if (!inviteKey.value || !key) return "";
  return `${currentOrigin()}invite/i/${encodeURIComponent(inviteKey.value)}/${encodeURIComponent(key)}?open_mode=template`;
}

// Returns true when the text was written to the clipboard.
async function copyText(text) {
  if (!text) return false;
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
    } else {
      const textarea = document.createElement("textarea");
      textarea.value = text;
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      document.body.removeChild(textarea);
    }
    message.success(t("issue_invite_copy_success"));
    return true;
  } catch (error) {
    console.error("[issue-invite] copy failed", error);
    message.error(t("issue_invite_copy_failed"));
    return false;
  }
}

async function loadShareData() {
  const seq = ++loadSeq;
  if (!appEnabled.value) {
    templates.value = [];
    generalUrl.value = "";
    inviteKey.value = "";
    return;
  }
  try {
    const share = canCopyLink.value
      ? await getIssueInviteShare(wsId.value)
      : { templates: await listIssueInviteTemplates(wsId.value) };
    if (seq !== loadSeq) return;
    inviteKey.value = share?.invite_key || "";
    templates.value = (share?.templates || []).filter(
      (item) => Number(item.enabled) === 1,
    );
    generalUrl.value = buildInviteUrl(inviteKey.value);
  } catch (error) {
    if (seq === loadSeq) {
      templates.value = [];
      generalUrl.value = "";
    }
  }
}

function copyGeneralLink() {
  copyText(generalUrl.value);
}

async function copyTemplateLink(item) {
  const copied = await copyText(buildTemplateUrl(item.template_key));
  if (copied) markCopied(item.template_key);
}

function openSettings(item) {
  router.push({
    path: `/issues/${wsId.value}/settings/templates/${item.id}`,
  });
}

function openGeneralSettings() {
  router.push({
    path: `/issues/${wsId.value}/settings`,
  });
}

watch(() => [wsId.value, appEnabled.value, canCopyLink.value], loadShareData, {
  immediate: true,
});
</script>

<style scoped lang="less">
.issue-invite-panel {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  padding-top: 8px;
}

.issue-invite-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 32px;
  padding: 20px 24px;
  border: 1px solid #dce7fb;
  border-radius: 12px;
  background: linear-gradient(112deg, #f1f8ff 0%, #f3f6ff 52%, #eaf0ff 100%);
}

.issue-invite-banner__main {
  display: flex;
  align-items: center;
  gap: 14px;
  min-width: 0;
}

.issue-invite-banner__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 40px;
  height: 40px;
  border-radius: 12px;
  background: #fff;
  box-shadow: inset 0 0 0 1px rgba(47, 91, 234, 0.08);
  color: #3157e2;
  font-size: 18px;
}

.issue-invite-banner__texts {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.issue-invite-banner__head {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.issue-invite-banner__title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  line-height: 25.2px;
  letter-spacing: -0.0111em;
  color: #262626;
}

.issue-invite-banner__line {
  margin: 0;
  font-size: 14px;
  line-height: 22px;
  color: #3a4559;
}

.issue-invite-banner__code {
  display: inline-block;
  padding: 0 6px;
  border-radius: 6px;
  background: #fff;
  color: #2f5bea;
  font-family: Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 18px;
}

.issue-invite-banner__hint {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 0 4px;
  margin: 0;
  padding-top: 6px;
  font-size: 14px;
  line-height: 22px;
  color: #5c6780;
}

.issue-invite-banner__label {
  font-weight: 600;
  color: #344054;
}

.issue-invite-banner__divider {
  padding: 0 4px;
}

.issue-invite-banner__actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.issue-invite-banner__actions :deep(.ant-btn) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 40px;
  padding: 0 16px;
  border-radius: 10px;
  font-size: 14px;
  font-weight: 500;
}

.issue-invite-banner__actions :deep(.anticon) {
  font-size: 17px;
}

.issue-invite-banner__settings {
  background: #fff;
  border-color: #d7dce5;
  color: #344054;

  &:hover,
  &:focus {
    border-color: #3157e2;
    color: #3157e2;
  }
}

.issue-invite-banner__copy {
  background: #3157e2;
  border-color: #3157e2;

  &:hover,
  &:focus {
    background: #4a6ef0;
    border-color: #4a6ef0;
  }
}

.issue-invite-panel__title {
  margin: 26px 0 12px;
  font-size: 16px;
  font-weight: 600;
  line-height: 24px;
  color: #182235;
}

.issue-invite-panel__list {
  border: 1px solid #f0f0f0;
  border-radius: 12px;
  overflow: hidden;
}

.issue-invite-panel__row {
  display: flex;
  align-items: center;
  gap: 12px;
  height: 46px;
  padding: 0 16px;
  border-bottom: 1px solid #e4e6eb;

  &:last-child {
    border-bottom: none;
  }

  &:hover {
    background: #f5f9ff;
  }
}

// The extra element selector keeps the specificity above the child component's
// own `.work-item-type-pill.is-large` rule, so the Figma padding wins.
div.issue-invite-panel__row :deep(.work-item-type-pill.is-large) {
  min-width: 0;
  padding: 1px 6px;
}

.issue-invite-panel__name {
  min-width: 0;
  font-size: 14px;
  font-weight: 500;
  line-height: 22px;
  color: #262626;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.issue-invite-panel__actions {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-left: auto;
  flex-shrink: 0;
}

.issue-invite-panel__btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  height: 32px;
  padding: 0 10px;
  border: 1px solid transparent;
  border-radius: 9px;
  background: transparent;
  box-shadow: none;
  font-size: 14px;
  font-weight: 400;
  line-height: 21px;
  color: #595959;

  &:deep(.anticon) {
    font-size: 16px;
  }

  &:hover,
  &:focus {
    border-color: #d9d9d9;
    background: #fff;
    color: #595959;
  }
}

.issue-invite-panel__btn--copy {
  min-width: 101px;
  color: #3157e2;

  &:hover,
  &:focus {
    color: #3157e2;
  }

  &.is-copied {
    border-color: transparent;
    background: #e5f9f1;
    color: #21a665;
  }
}
</style>

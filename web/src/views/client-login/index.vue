<template>
  <div class="client-login-page">
    <div class="client-login-lang" @click="toggleLang">
      <span>{{ loginLang === "zh-CN" ? "中文" : "English" }}</span>
    </div>

    <section class="client-login-card">
      <p v-if="!state" class="client-login-error">
        {{ t("clientLogin_missingState") }}
      </p>
      <template v-else>
        <div class="client-login-avatar" aria-hidden="true">{{ initial }}</div>
        <p class="client-login-signed">{{ t("clientLogin_signedInAs") }}</p>
        <h1 class="client-login-name">{{ displayName }}</h1>

        <!-- 等待客户端完成授权：确认后页面不再自动关闭，轮询直到客户端兑换票据，
             给出明确成功/超时反馈，避免用户在客户端未授权前页面就消失。 -->
        <template v-if="connected">
          <div class="client-login-hint client-login-hint-success">
            <span class="client-login-hint-icon">✓</span>
            <div>
              <strong>{{ t("clientLogin_connectedTitle") }}</strong>
              <p>{{ t("clientLogin_connectedBody") }}</p>
            </div>
          </div>

          <a-button
            type="primary"
            class="client-login-confirm"
            block
            @click="handleBackToWorkspace"
          >
            {{ t("clientLogin_backToWorkspace") }}
          </a-button>
          <p class="client-login-opened">
            {{ t("clientLogin_closeHint") }}
          </p>
        </template>

        <!-- 超时：客户端未在时限内完成授权，可重试唤起。 -->
        <template v-else-if="timedOut">
          <div class="client-login-hint client-login-hint-warn">
            <span class="client-login-hint-icon">!</span>
            <div>
              <strong>{{ t("clientLogin_timeoutTitle") }}</strong>
              <p>{{ t("clientLogin_timeoutBody") }}</p>
            </div>
          </div>

          <a-button
            type="primary"
            class="client-login-confirm"
            block
            @click="handleRetryHandoff"
          >
            {{ t("clientLogin_retryHandoff") }}
          </a-button>
          <a-button
            class="client-login-switch"
            block
            @click="handleBackToWorkspace"
          >
            {{ t("clientLogin_backToWorkspace") }}
          </a-button>
        </template>

        <template v-else>
          <div class="client-login-hint">
            <span class="client-login-hint-icon">✓</span>
            <div>
              <strong>{{
                connecting
                  ? t("clientLogin_waitingTitle")
                  : t("clientLogin_hintTitle")
              }}</strong>
              <p>
                {{
                  connecting
                    ? t("clientLogin_waitingBody")
                    : t("clientLogin_hintBody", { name: displayName })
                }}
              </p>
            </div>
          </div>

          <a-button
            type="primary"
            class="client-login-confirm"
            :loading="connecting"
            block
            @click="connecting ? handleRetryHandoff() : handleConfirm()"
          >
            {{
              connecting
                ? t("clientLogin_openingClient")
                : t("clientLogin_confirm")
            }}
          </a-button>
          <a-button
            class="client-login-switch"
            block
            :disabled="connecting"
            @click="handleSwitchAccount"
          >
            {{ t("clientLogin_switchAccount") }}
          </a-button>
          <button
            type="button"
            class="client-login-cancel"
            :disabled="connecting"
            @click="handleCancel"
          >
            {{ t("clientLogin_cancel") }}
          </button>
        </template>
      </template>
    </section>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import { message } from "ant-design-vue";
import { useAuthStore } from "@/stores/auth";
import { useLocaleStore } from "@/stores/locale";
import {
  confirmClientLoginSession,
  getClientLoginSessionStatus,
} from "@/api/auth";

// 等待客户端完成票据兑换的轮询节奏与总时限：确认后页面保持打开，直到拿到
// exchanged（成功）/ expired（会话失效）或超时（客户端未唤起/未完成）。
const STATUS_POLL_MS = 1500;
const STATUS_TIMEOUT_MS = 3 * 60 * 1000;

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const localeStore = useLocaleStore();

// connecting：已确认并唤起客户端，正在等待授权结果。
// connected：客户端已完成授权，展示成功收尾。
// timedOut：超时未检测到客户端完成授权，可重试唤起。
const connecting = ref(false);
const connected = ref(false);
const timedOut = ref(false);
let statusTimer = 0;
const state = computed(() =>
  typeof route.query.state === "string" ? route.query.state.trim() : "",
);
const loginLang = computed(() => localeStore.currentLang);
const displayName = computed(
  () => authStore.userInfo?.name || authStore.userInfo?.username || "",
);
const initial = computed(
  () => displayName.value.slice(0, 1).toUpperCase() || "?",
);

function toggleLang() {
  localeStore.toggleLang();
}

function stopStatusPoll() {
  if (statusTimer) {
    window.clearTimeout(statusTimer);
    statusTimer = 0;
  }
}

async function pollStatusOnce() {
  try {
    const result = await getClientLoginSessionStatus(state.value);
    const status = result?.status || "";
    if (status === "exchanged") {
      connecting.value = false;
      connected.value = true;
      return;
    }
    if (status === "expired") {
      // 会话已失效：授权流程已不可完成，切到超时态给出重试/返回出口。
      connecting.value = false;
      timedOut.value = true;
      return;
    }
  } catch {
    // 单次轮询失败不打断等待，下一轮重试。
  }
  if (Date.now() - handoffAt >= STATUS_TIMEOUT_MS) {
    connecting.value = false;
    timedOut.value = true;
    return;
  }
  statusTimer = window.setTimeout(pollStatusOnce, STATUS_POLL_MS);
}

let handoffAt = 0;

function startWaitingForClient(callbackURL) {
  // 自定义协议不会让页面跳走，只是交给系统去唤起客户端；页面保持打开等待结果。
  window.location.href = callbackURL;
  handoffAt = Date.now();
  stopStatusPoll();
  statusTimer = window.setTimeout(pollStatusOnce, STATUS_POLL_MS);
}

async function handleConfirm() {
  if (!state.value || connecting.value) return;
  connecting.value = true;
  timedOut.value = false;
  try {
    const result = await confirmClientLoginSession(state.value);
    const callbackURL = result?.callback_url || "";
    if (!callbackURL.startsWith("teamsboard://")) {
      message.error(t("clientLogin_failed"));
      connecting.value = false;
      return;
    }
    startWaitingForClient(callbackURL);
  } catch {
    message.error(t("clientLogin_failed"));
    connecting.value = false;
  }
}

// 超时后重试：会话仍处于 confirmed（票据未消费），重新唤起客户端继续等待。
function handleRetryHandoff() {
  timedOut.value = false;
  connecting.value = true;
  handleConfirm();
}

function handleBackToWorkspace() {
  router.replace("/");
}

function handleSwitchAccount() {
  const redirect = `/client-login?state=${encodeURIComponent(state.value)}`;
  authStore.logout();
  router.replace({ path: "/login", query: { redirect } });
}

function handleCancel() {
  router.replace("/");
}

onBeforeUnmount(() => {
  stopStatusPoll();
});
</script>

<style scoped lang="less">
.client-login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f7f8fa;
}

.client-login-lang {
  position: absolute;
  top: 24px;
  right: 32px;
  color: #262626;
  cursor: pointer;
}

.client-login-card {
  width: 400px;
  max-width: calc(100vw - 32px);
  padding: 40px 32px 28px;
  border-radius: 16px;
  background: #fff;
  box-shadow: 0 4px 32px rgba(0, 0, 0, 0.08);
  text-align: center;
}

.client-login-avatar {
  width: 56px;
  height: 56px;
  margin: 0 auto 12px;
  border-radius: 50%;
  background: #eef2ff;
  color: #3157e2;
  font-size: 22px;
  font-weight: 600;
  line-height: 56px;
}

.client-login-signed {
  margin: 0;
  color: #8c8c8c;
  font-size: 13px;
  line-height: 20px;
}

.client-login-name {
  margin: 4px 0 20px;
  color: #0a0a0a;
  font-size: 28px;
  font-weight: 600;
  line-height: 36px;
  word-break: break-word;
}

.client-login-hint {
  display: flex;
  gap: 8px;
  margin-bottom: 24px;
  padding: 12px 14px;
  border-radius: 10px;
  background: #f7f8fa;
  text-align: left;
}

.client-login-hint-icon {
  flex: 0 0 18px;
  color: #3157e2;
  font-weight: 700;
}

.client-login-hint-success .client-login-hint-icon {
  color: #23a35a;
}

.client-login-hint-warn .client-login-hint-icon {
  color: #fa8c16;
}

.client-login-hint strong,
.client-login-hint p {
  margin: 0;
  font-size: 13px;
  line-height: 20px;
}

.client-login-hint strong {
  color: #262626;
}

.client-login-hint p {
  margin-top: 2px;
  color: #8c8c8c;
}

.client-login-confirm,
.client-login-switch {
  height: 40px;
  border-radius: 8px;
}

.client-login-switch {
  margin-top: 10px;
}

.client-login-cancel,
.client-login-error,
.client-login-opened {
  margin: 12px 0 0;
  border: 0;
  background: transparent;
  color: #8c8c8c;
  font-size: 13px;
  line-height: 20px;
}

.client-login-cancel {
  cursor: pointer;
}
</style>

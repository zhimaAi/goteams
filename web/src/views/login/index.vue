<template>
  <div class="login-page">
    <!-- Left brand showcase area -->
    <BrandPanel :branding="brandingStore.publicBranding" />

    <!-- Right card area -->
    <section class="login-form-side">
      <!-- Language switch -->
      <div class="login-lang-switch" @click="toggleLang">
        <img :src="iconGlobe" class="lang-icon" alt="" />
        <span class="lang-label">{{
          loginLang === "zh-CN" ? "中文" : "English"
        }}</span>
        <img :src="iconChevronDown" class="lang-arrow" alt="" />
      </div>

      <div class="login-card login-card-fixed">
        <h2 class="login-title">
          {{
            brandingStore.publicBranding.app_name
              ? lt2("personalization_login_title", {
                  name: brandingStore.publicBranding.app_name,
                })
              : lt("auth_loginWelcome")
          }}
        </h2>

        <a-button
          v-if="brandingStore.publicBranding.sso_url"
          type="primary"
          block
          :href="brandingStore.publicBranding.sso_url"
          class="enterprise-sso-button"
          >{{
            lt2("personalization_sso_login", {
              provider: brandingStore.publicBranding.sso_label,
            })
          }}</a-button
        >

        <!-- ========== Third-party Scan-code Login ========== -->
        <div v-if="loginMode !== 'password'" class="oauth-login">
          <a-spin
            v-if="oauthLoading || !providersLoaded"
            class="oauth-login-loading"
          />
          <template v-else-if="oauthAuthorize">
            <div class="scan-qrcode-box">
              <span class="scan-corner scan-corner-tl"></span>
              <span class="scan-corner scan-corner-tr"></span>
              <span class="scan-corner scan-corner-bl"></span>
              <span class="scan-corner scan-corner-br"></span>
              <div class="scan-qrcode-viewport">
                <OAuthQrcode
                  :provider="loginMode"
                  :authorize="oauthAuthorize"
                  :expires-in="oauthAuthorize.expires_in || 0"
                  :tip="oauthScanTip"
                  @error="handleOauthQrcodeError"
                  @ready="handleOauthQrcodeReady"
                  @refresh="refreshOauthQrcode"
                />
                <span
                  v-if="oauthQrcodeReady"
                  class="scan-qrcode-logo"
                  aria-hidden="true"
                >
                  <DingdingOutlined />
                </span>
              </div>
            </div>
          </template>
          <p v-else class="oauth-login-empty">
            {{ lt("auth_oauthQrcodeFailed") }}
          </p>
        </div>

        <!-- ========== Login Form ========== -->
        <a-form
          v-if="loginMode === 'password'"
          ref="loginFormRef"
          class="login-form"
          :model="loginForm"
          :rules="loginRules"
          layout="vertical"
          @finish="handleLogin"
        >
          <!-- Email -->
          <a-form-item name="username">
            <a-input
              v-model:value="loginForm.username"
              :placeholder="lt('auth_accountPlaceholder')"
              autocomplete="username"
              size="large"
            >
              <template #prefix>
                <img :src="iconUser" class="input-icon" alt="" />
              </template>
            </a-input>
          </a-form-item>

          <!-- Password -->
          <a-form-item name="password">
            <a-input-password
              v-model:value="loginForm.password"
              :placeholder="lt('auth_passwordPlaceholder')"
              :icon-render="passwordIcon"
              autocomplete="current-password"
              size="large"
            >
              <template #prefix>
                <img :src="iconLock" class="input-icon" alt="" />
              </template>
            </a-input-password>
          </a-form-item>

          <!-- Login button -->
          <a-button
            type="primary"
            html-type="submit"
            class="submit-button"
            :loading="loginLoading"
            block
          >
            {{ lt("auth_login") }}
          </a-button>
        </a-form>

        <!-- 登录方式切换（密码登录 / 扫码登录 互切） -->
        <div class="login-mode-area">
          <button
            v-if="loginMode === 'password' && scanProvider"
            type="button"
            class="mode-switch-btn"
            @click="switchToScan"
          >
            <DingdingOutlined class="mode-switch-icon mode-switch-dingtalk" />
            <span>{{ scanProviderLabel }}</span>
          </button>
          <button
            v-else-if="loginMode !== 'password'"
            type="button"
            class="mode-switch-btn"
            @click="switchLoginMode('password')"
          >
            <img :src="iconBackArrow" class="mode-switch-icon" alt="" />
            <span>{{ lt("auth_loginModePassword") }}</span>
          </button>
        </div>

        <!-- 第三方回跳：票据消费完成前盖住整个卡片，避免先闪出密码登录表单 -->
        <div v-if="oauthCallbackPending" class="login-callback-loading">
          <a-spin />
          <p class="login-callback-text">{{ lt("auth_oauthSigningIn") }}</p>
        </div>
      </div>
    </section>

    <!-- ========== Forgot Password Modal ========== -->
    <a-modal
      v-model:open="forgotModalOpen"
      :title="lt('auth_forgotPasswordTitle')"
      :footer="null"
      :maskClosable="false"
      width="420px"
      destroyOnClose
    >
      <a-steps :current="forgotStep" size="small" class="forgot-steps">
        <a-step :title="lt('auth_sendCode')" />
        <a-step :title="lt('auth_resetPassword')" />
      </a-steps>

      <!-- Step 1: send verification code -->
      <a-form
        v-if="forgotStep === 0"
        ref="forgotEmailFormRef"
        class="forgot-form"
        :model="forgotForm"
        :rules="forgotEmailRules"
        layout="vertical"
        @finish="handleSendForgotCode"
      >
        <a-form-item name="email">
          <a-input
            v-model:value="forgotForm.email"
            :placeholder="lt('auth_emailPlaceholder')"
            autocomplete="email"
            size="large"
          >
            <template #prefix>
              <img :src="iconEmail" class="input-icon" alt="" />
            </template>
          </a-input>
        </a-form-item>

        <a-button
          type="primary"
          html-type="submit"
          class="submit-button"
          :loading="forgotSendLoading"
          block
        >
          {{ lt("auth_sendCode") }}
        </a-button>
      </a-form>

      <!-- Step 2: verify code + new password -->
      <a-form
        v-if="forgotStep === 1"
        ref="forgotResetFormRef"
        class="forgot-form"
        :model="forgotForm"
        :rules="forgotResetRules"
        layout="vertical"
        @finish="handleResetPassword"
      >
        <a-form-item name="verifyCode">
          <a-input
            v-model:value="forgotForm.verifyCode"
            :placeholder="lt('auth_verifyCodePlaceholder')"
            :maxlength="6"
            size="large"
          />
        </a-form-item>

        <a-form-item name="newPassword">
          <a-input-password
            v-model:value="forgotForm.newPassword"
            :placeholder="lt('auth_newPassword')"
            :icon-render="passwordIcon"
            autocomplete="new-password"
            size="large"
          >
            <template #prefix>
              <img :src="iconLock" class="input-icon" alt="" />
            </template>
          </a-input-password>
        </a-form-item>

        <a-form-item name="confirmPassword">
          <a-input-password
            v-model:value="forgotForm.confirmPassword"
            :placeholder="lt('auth_confirmPasswordPlaceholder')"
            :icon-render="passwordIcon"
            autocomplete="new-password"
            size="large"
          >
            <template #prefix>
              <img :src="iconLock" class="input-icon" alt="" />
            </template>
          </a-input-password>
        </a-form-item>

        <a-button
          type="primary"
          html-type="submit"
          class="submit-button"
          :loading="forgotResetLoading"
          block
        >
          {{ lt("auth_resetConfirm") }}
        </a-button>
      </a-form>
    </a-modal>

    <!-- ========== Bind Account Modal (scanned account is not bound yet) ========== -->
    <a-modal
      v-model:open="bindModalOpen"
      :title="lt('auth_oauthBindTitle')"
      :footer="null"
      :maskClosable="false"
      width="440px"
      destroyOnClose
    >
      <p class="bind-desc">{{ bindDesc }}</p>

      <!-- Bind an existing account -->
      <a-form
        ref="bindFormRef"
        class="login-form"
        :model="bindForm"
        :rules="bindRules"
        layout="vertical"
        @finish="handleBindExisting"
      >
        <a-form-item name="username">
          <a-input
            v-model:value="bindForm.username"
            :placeholder="lt('auth_accountPlaceholder')"
            autocomplete="username"
            size="large"
          >
            <template #prefix>
              <img :src="iconEmail" class="input-icon" alt="" />
            </template>
          </a-input>
        </a-form-item>

        <a-form-item name="password">
          <a-input-password
            v-model:value="bindForm.password"
            :placeholder="lt('auth_passwordPlaceholder')"
            :icon-render="passwordIcon"
            autocomplete="current-password"
            size="large"
          >
            <template #prefix>
              <img :src="iconLock" class="input-icon" alt="" />
            </template>
          </a-input-password>
        </a-form-item>

        <a-button
          type="primary"
          html-type="submit"
          class="submit-button"
          :loading="bindLoading"
          block
        >
          {{ lt("auth_oauthBindAndLogin") }}
        </a-button>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup>
import { h, ref, reactive, computed, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { message, Modal } from "ant-design-vue";
import {
  EyeInvisibleOutlined,
  EyeOutlined,
  DingdingOutlined,
} from "@ant-design/icons-vue";
import { useAuthStore } from "@/stores/auth";
import { useLocaleStore } from "@/stores/locale";
import { login, sendVerifyCode, resetPassword } from "@/api/auth";
import {
  fetchOauthProviders,
  fetchOauthAuthorize,
  exchangeOauthTicket,
  bindOauthAccount,
} from "@/api/oauth";
import zhCNMessages from "@/locales/zh-CN";
import enUSMessages from "@/locales/en-US";
import BrandPanel from "@/components/common/BrandPanel.vue";
import { useBrandingStore } from "@/stores/branding";
import OAuthQrcode from "@/components/common/OAuthQrcode.vue";
import { takeLogoutReason } from "@/utils/logoutReason";
import { safeInternalPath } from "@/utils/internalRedirect";
import { getRememberedTab, setRememberedTab } from "@/utils/rememberedTab";
import iconEmail from "@/image/login/icon-email.svg";
import iconLock from "@/image/login/icon-lock.svg";
import iconUser from "@/image/login/icon-user.svg";
import iconBackArrow from "@/image/login/icon-back-arrow.svg";
import iconGlobe from "@/image/login/icon-globe.svg";
import iconChevronDown from "@/image/login/icon-chevron-down.svg";

const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();
const brandingStore = useBrandingStore();
const localeStore = useLocaleStore();

function postAuthDestination() {
  return (
    safeInternalPath(
      typeof route.query.redirect === "string" ? route.query.redirect : "",
    ) || "/my-work"
  );
}

// 第三方回跳会先清掉 query。登录完成前记住 TeamsBoard 带回的站内地址。
const authReturnPath = postAuthDestination();

// 登录页在登录前拿不到 userId，用固定的访客标识把「上次的登录方式」记在设备维度。
const LOGIN_TAB_USER = "guest";
const LOGIN_MODE_SCOPE = "auth-mode";

// 第三方回跳是整页加载，票据必须在首帧渲染之前就读出来，否则会先闪一次密码登录表单。
const oauthCallback = readOauthCallbackQuery();
const oauthCallbackPending = ref(!!oauthCallback);
// An explicit password entry must bypass remembered scan/register choices for this visit only.
const passwordEntry = route.query.mode === "password";

onMounted(async () => {
  // Surface why the user was signed out (e.g. account disabled by an admin). The 401 handler
  // could not show this itself: its redirect reloads the page and destroys any toast.
  const logoutReason = takeLogoutReason();
  if (logoutReason) {
    Modal.error({
      title: lt("auth_loginFailed"),
      content: logoutReason,
      okText: lt("common_confirm"),
    });
  }

  // ?token= 免登链接已在路由守卫（permission.js）中处理，登录页不再感知 token。
  await setupOauthLogin();
});

/**
 * Prepare third-party scan-code sign-in: load the enabled providers (they decide which scan
 * methods are shown) and then consume any result handed back by the provider callback.
 */
async function setupOauthLogin() {
  // 两个请求互不依赖：回跳票据自带平台标识，不必等 providers 列表，缩短「登录中」的停留。
  const providersRequest = fetchOauthProviders()
    .then((list) => list || [])
    .catch(() => {
      // A failed lookup only hides the third-party entries; password sign-in must still work.
      return [];
    });
  const callbackRequest = oauthCallback
    ? consumeOauthCallback(oauthCallback)
    : Promise.resolve(false);

  oauthProviders.value = await providersRequest;
  providersLoaded.value = true;

  if (await callbackRequest) {
    return; // 已登录并跳转后台，不再还原登录界面。
  }

  oauthCallbackPending.value = false;

  // 扫码失败或票据失效后仍停在登录页，补一张新二维码让用户原地重扫；
  // 绑定票据会打开弹窗，此时不需要二维码。
  if (oauthCallback && oauthCallback.bindTicket) {
    return;
  }
  await ensureAuthorizeForCurrentMode();
}

const MESSAGES = { "zh-CN": zhCNMessages, "en-US": enUSMessages };
const loginLang = computed(() => localeStore.currentLang);

function lt(key) {
  return MESSAGES[loginLang.value]?.[key] || MESSAGES["zh-CN"][key] || key;
}

/** i18n helper with template replacement */
function lt2(key, vars) {
  let tpl = lt(key);
  Object.entries(vars).forEach(([k, v]) => {
    tpl = tpl.replace(`{${k}}`, v);
  });
  return tpl;
}

function toggleLang() {
  localeStore.toggleLang();
}

const passwordIcon = (visible) =>
  h(visible ? EyeInvisibleOutlined : EyeOutlined);

// ========== Third-party Scan-code Login ==========
// Provider keys map to the label shown on the mode-switch button.
const OAUTH_PROVIDER_LABEL_KEYS = {
  dingtalk: "auth_loginModeDingtalk",
  feishu: "auth_loginModeFeishu",
};

// 登录方式的全部候选值：密码登录 + 已接入的第三方平台；用于校验「上次选择」的记忆值。
// 必须定义在 loginMode 之前：它在模块初始化时就被读取。
const LOGIN_MODE_KEYS = ["password", ...Object.keys(OAUTH_PROVIDER_LABEL_KEYS)];

// Callback failures come back as a reason code in the URL; map them to user-facing text.
const OAUTH_ERROR_KEYS = {
  provider_disabled: "auth_oauthErrorProviderDisabled",
  state_invalid: "auth_oauthErrorStateInvalid",
  code_invalid: "auth_oauthErrorCodeInvalid",
  ticket_invalid: "auth_oauthErrorTicketInvalid",
  bound_by_other: "auth_oauthErrorBoundByOther",
  server_error: "auth_oauthErrorServer",
  // 企业 SSO 回跳失败的原因码（provider=sso），与扫码链路共用同一套展示机制。
  sso_disabled: "sso_errorDisabled",
  sso_state_invalid: "sso_errorStateInvalid",
  sso_endpoint_unreachable: "sso_errorEndpointUnreachable",
  sso_auth_failed: "sso_errorAuthFailed",
  account_disabled: "sso_errorAccountDisabled",
  member_quota_exceeded: "sso_errorQuotaExceeded",
};

const oauthProviders = ref([]);
const providersLoaded = ref(false);
// 上次使用的登录方式；回跳票据带回的平台优先于记忆值。
const loginMode = ref(
  oauthCallback && LOGIN_MODE_KEYS.includes(oauthCallback.provider)
    ? oauthCallback.provider
    : passwordEntry
      ? "password"
      : getRememberedTab(
          LOGIN_TAB_USER,
          LOGIN_MODE_SCOPE,
          LOGIN_MODE_KEYS,
          "password",
        ),
);
const oauthAuthorize = ref(null);
const oauthLoading = ref(false);
const oauthQrcodeReady = ref(false);

function providerLabel(provider) {
  return lt(OAUTH_PROVIDER_LABEL_KEYS[provider] || "auth_loginModeDingtalk");
}

// 扫码登录指向第一个已启用的第三方平台；设计稿只展示单一的「扫码 / 密码」互切。
const scanProvider = computed(() => oauthProviders.value[0]?.provider || "");
const scanProviderLabel = computed(() =>
  scanProvider.value ? providerLabel(scanProvider.value) : "",
);

function switchToScan() {
  if (scanProvider.value) {
    switchLoginMode(scanProvider.value);
  }
}

const oauthScanTip = computed(() =>
  loginMode.value === "password"
    ? ""
    : lt2("auth_oauthScanLoginTip", { app: providerLabel(loginMode.value) }),
);

function oauthErrorMessage(reason) {
  return lt(OAUTH_ERROR_KEYS[reason] || "auth_oauthErrorServer");
}

/**
 * Fetch a scan-code authorize entry. Refreshing an expired QR code goes through the same path:
 * a new entry means a new state, so the code that already died cannot be scanned again.
 */
async function loadOauthAuthorize(mode) {
  oauthQrcodeReady.value = false;
  oauthAuthorize.value = null;
  oauthLoading.value = true;
  try {
    oauthAuthorize.value = await fetchOauthAuthorize(mode);
  } catch {
    // Keep the empty state: the user can switch methods and retry.
    oauthAuthorize.value = null;
  } finally {
    oauthLoading.value = false;
  }
}

function switchLoginMode(mode) {
  loginMode.value = mode;
  setRememberedTab(LOGIN_TAB_USER, LOGIN_MODE_SCOPE, mode);
  if (mode === "password") {
    oauthQrcodeReady.value = false;
    oauthAuthorize.value = null;
    return;
  }
  return loadOauthAuthorize(mode);
}

function refreshOauthQrcode() {
  return loadOauthAuthorize(loginMode.value);
}

function handleOauthQrcodeError() {
  oauthQrcodeReady.value = false;
  message.error(lt("auth_oauthQrcodeFailed"));
}

function handleOauthQrcodeReady() {
  oauthQrcodeReady.value = true;
}

/** 该第三方平台当前是否开启。 */
function isProviderEnabled(provider) {
  return oauthProviders.value.some((item) => item.provider === provider);
}

/**
 * 当前登录方式若还没有可用的授权入口，补拉一次（幂等）。
 * 记住的平台已被后台关闭时回退密码登录，但不改写记忆值——平台重新开启后仍能恢复。
 */
function ensureAuthorizeForCurrentMode() {
  if (!providersLoaded.value) {
    return Promise.resolve();
  }
  if (loginMode.value === "password") {
    return Promise.resolve();
  }
  if (!isProviderEnabled(loginMode.value)) {
    loginMode.value = "password";
    return Promise.resolve();
  }
  if (oauthAuthorize.value || oauthLoading.value) {
    return Promise.resolve();
  }
  return loadOauthAuthorize(loginMode.value);
}

/**
 * Read the provider callback result from the URL. Called synchronously before the first render:
 * a callback always arrives as a full page load, so waiting for it would flash the password form.
 */
function readOauthCallbackQuery() {
  const ticket =
    typeof route.query.oauth_ticket === "string"
      ? route.query.oauth_ticket
      : "";
  const bindTicket =
    typeof route.query.oauth_bind_ticket === "string"
      ? route.query.oauth_bind_ticket
      : "";
  const error =
    typeof route.query.oauth_error === "string" ? route.query.oauth_error : "";
  if (!ticket && !bindTicket && !error) {
    return null;
  }
  return {
    ticket,
    bindTicket,
    error,
    provider:
      typeof route.query.provider === "string" ? route.query.provider : "",
  };
}

/**
 * Consume what the provider callback left in the URL: a sign-in ticket, a bind ticket, or an
 * error reason. Returns true when the user is signed in and the page is already leaving.
 * The query is cleared first so a refresh cannot replay a one-time ticket.
 */
async function consumeOauthCallback(callback) {
  bindProvider.value = callback.provider;
  await router.replace("/login");

  if (callback.error) {
    Modal.error({
      title: lt("auth_loginFailed"),
      content: oauthErrorMessage(callback.error),
      okText: lt("common_confirm"),
    });
    return false;
  }

  if (callback.ticket) {
    return consumeOauthLoginTicket(callback.ticket);
  }

  bindTicket.value = callback.bindTicket;
  bindModalOpen.value = true;
  return false;
}

/** Exchange the one-time sign-in ticket for a session; returns true when the user is signed in. */
async function consumeOauthLoginTicket(ticket) {
  loginLoading.value = true;
  try {
    const res = await exchangeOauthTicket(ticket);
    await authStore.loginWithToken(res.token);
    message.success(lt("auth_loginSuccess"));
    router.push(authReturnPath);
    return true;
  } catch {
    Modal.error({
      title: lt("auth_loginFailed"),
      content: lt("auth_oauthErrorTicketInvalid"),
      okText: lt("common_confirm"),
    });
    return false;
  } finally {
    loginLoading.value = false;
  }
}

// ========== Login ==========
const loginFormRef = ref();
const loginLoading = ref(false);
const loginForm = reactive({
  username: "",
  password: "",
});

const loginRules = computed(() => ({
  username: [
    { required: true, message: lt("auth_emailRequired"), trigger: "blur" },
  ],
  password: [
    { required: true, message: lt("auth_passwordRequired"), trigger: "blur" },
    { min: 6, message: lt("auth_passwordMin"), trigger: "blur" },
  ],
}));

async function handleLogin() {
  loginLoading.value = true;
  try {
    await authStore.login({
      username: loginForm.username,
      password: loginForm.password,
    });
    message.success(lt("auth_loginSuccess"));
    router.push(authReturnPath);
  } catch (err) {
    const serverMessage = err?.response?.data?.message;
    Modal.error({
      title: lt("auth_loginFailed"),
      content: serverMessage || lt("auth_loginFailed"),
      okText: lt("common_confirm"),
    });
  } finally {
    loginLoading.value = false;
  }
}

// ========== Bind Account Dialog (scanned account is not bound yet) ==========
const bindModalOpen = ref(false);
const bindTicket = ref("");
const bindProvider = ref("");
const bindLoading = ref(false);
const bindFormRef = ref();

const bindForm = reactive({
  username: "",
  password: "",
});

const bindDesc = computed(() =>
  lt2("auth_oauthBindDesc", { app: providerLabel(bindProvider.value) }),
);

const bindRules = computed(() => ({
  username: [
    { required: true, message: lt("auth_emailRequired"), trigger: "blur" },
  ],
  password: [
    { required: true, message: lt("auth_passwordRequired"), trigger: "blur" },
    { min: 6, message: lt("auth_passwordMin"), trigger: "blur" },
  ],
}));

/** Sign in through the ticket returned by the bind endpoint. */
async function finishOauthLogin(result, successText) {
  await authStore.loginWithToken(result.token);
  message.success(successText);
  bindModalOpen.value = false;
  router.push(authReturnPath);
}

function showBindError(err) {
  const serverMessage = err?.response?.data?.message;
  Modal.error({
    title: lt("auth_oauthBindTitle"),
    content: serverMessage || err?.message || lt("common_requestFailed"),
    okText: lt("common_confirm"),
  });
}

async function handleBindExisting() {
  bindLoading.value = true;
  try {
    const res = await bindOauthAccount({
      ticket: bindTicket.value,
      username: bindForm.username,
      password: bindForm.password,
    });
    await finishOauthLogin(res, lt("auth_oauthBindSuccess"));
  } catch (err) {
    showBindError(err);
  } finally {
    bindLoading.value = false;
  }
}

// ========== Forgot Password ==========
const forgotModalOpen = ref(false);
const forgotStep = ref(0);
const forgotSendLoading = ref(false);
const forgotResetLoading = ref(false);
const forgotEmailFormRef = ref();
const forgotResetFormRef = ref();

const forgotForm = reactive({
  email: "",
  verifyCode: "",
  newPassword: "",
  confirmPassword: "",
});

const forgotEmailRules = computed(() => ({
  email: [
    { required: true, message: lt("auth_emailRequired"), trigger: "blur" },
    { type: "email", message: lt("auth_emailInvalid"), trigger: "blur" },
  ],
}));

const forgotResetRules = computed(() => ({
  verifyCode: [
    { required: true, message: lt("auth_codeRequired"), trigger: "blur" },
    { pattern: /^\d{6}$/, message: lt("auth_codeFormat"), trigger: "blur" },
  ],
  newPassword: [
    { required: true, message: lt("auth_passwordRequired"), trigger: "blur" },
    { min: 6, message: lt("auth_passwordMin"), trigger: "blur" },
    { min: 8, max: 32, message: lt("auth_passwordLen"), trigger: "blur" },
  ],
  confirmPassword: [
    {
      required: true,
      message: lt("auth_confirmPasswordRequired"),
      trigger: "blur",
    },
    {
      validator: (_rule, value) => {
        if (value !== forgotForm.newPassword) {
          return Promise.reject(lt("auth_passwordMismatch"));
        }
        return Promise.resolve();
      },
      trigger: "blur",
    },
  ],
}));

function openForgotPassword() {
  resetForgotState();
  forgotModalOpen.value = true;
}

function resetForgotState() {
  forgotStep.value = 0;
  forgotForm.email = "";
  forgotForm.verifyCode = "";
  forgotForm.newPassword = "";
  forgotForm.confirmPassword = "";
  forgotSendLoading.value = false;
  forgotResetLoading.value = false;
}

async function handleSendForgotCode() {
  forgotSendLoading.value = true;
  try {
    await sendVerifyCode(forgotForm.email);
    message.success(lt("auth_codeSent"));
    forgotStep.value = 1;
  } catch (err) {
    const serverMessage = err?.response?.data?.message;
    message.error(serverMessage || lt("common_requestFailed"));
  } finally {
    forgotSendLoading.value = false;
  }
}

async function handleResetPassword() {
  forgotResetLoading.value = true;
  try {
    await resetPassword({
      email: forgotForm.email,
      verifyCode: forgotForm.verifyCode,
      newPassword: forgotForm.newPassword,
    });
    message.success(lt("auth_resetSuccess"));
    forgotModalOpen.value = false;
    // Pre-fill email for convenience
    loginForm.username = forgotForm.email;
    resetForgotState();
  } catch (err) {
    const serverMessage = err?.response?.data?.message;
    message.error(serverMessage || lt("common_requestFailed"));
  } finally {
    forgotResetLoading.value = false;
  }
}
</script>

<style scoped lang="less">
.enterprise-sso-button {
  margin-bottom: 20px;
}
.login-page {
  display: flex;
  min-height: 100vh;
  background: #f7f8fb;
}

// ========== Left brand showcase area: extracted to components/common/BrandPanel.vue ==========

// ========== Right card area ==========
.login-form-side {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  min-width: 0;
}

.login-lang-switch {
  position: absolute;
  top: 16px;
  right: 16px;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 5px 12px;
  border: 1px solid #d9d9d9;
  border-radius: 6px;
  cursor: pointer;
  user-select: none;
  background: #fff;

  &:hover {
    border-color: #3157e2;
  }
}

.lang-icon {
  width: 16px;
  height: 16px;
}

.lang-label {
  font-size: 14px;
  color: #262626;
  line-height: 22px;
}

.lang-arrow {
  width: 16px;
  height: 16px;
}

// ========== Login card ==========
.login-card {
  position: relative;
  width: 500px;
  padding: 60px 60px 48px;
  background: #fff;
  border-radius: 24px;
  box-shadow: 0px 4px 32px 0px rgba(0, 0, 0, 0.08);
}

// 登录态（密码 / 扫码）固定 500x640，两种模式切换时卡片尺寸不变。
.login-card-fixed {
  height: 640px;
  display: flex;
  flex-direction: column;
  padding: 80px 60px 16px;
}

.login-card-fixed .login-title {
  margin-bottom: 54px;
}

// 第三方回跳后的中间态：票据消费完成前整卡只显示加载动画。
.login-callback-loading {
  position: absolute;
  inset: 0;
  z-index: 2;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  background: #fff;
  border-radius: 24px;
}

.login-callback-text {
  margin: 0;
  color: #8c8c8c;
  font-size: 14px;
  line-height: 22px;
}

.login-title {
  margin: 0 0 32px;
  font-size: 32px;
  font-weight: 400;
  line-height: 44px;
  color: #000;
  text-align: center;
}

// ========== 登录方式切换（密码登录 / 扫码登录） ==========
.login-mode-area {
  display: flex;
  justify-content: center;
  margin-top: 48px;
}

// 设计稿：圆角胶囊，描边 #F0F0F0，内含图标 + 文字（钉钉登录 / 密码登录）。
.mode-switch-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 14px;
  border: 1px solid #f0f0f0;
  border-radius: 32px;
  background: #fff;
  color: #595959;
  font-size: 14px;
  line-height: 22px;
  cursor: pointer;
  transition: all 0.2s ease;
  outline: none;

  &:hover {
    color: #3157e2;
    border-color: #3157e2;
  }
}

.mode-switch-icon {
  width: 16px;
  height: 16px;
  display: block;
}

.mode-switch-dingtalk {
  width: auto;
  height: auto;
  font-size: 18px;
  color: #0089ff;
}

// ========== 扫码登录二维码容器 ==========
// 设计稿：二维码约 208px，四角装饰框贴在二维码外侧。
// The SDK renders a 300px frame with a roughly 173px centered QR code.
// Give each wrapper an explicit height before centering and scaling the SDK frame.
.scan-qrcode-box {
  position: relative;
  width: 208px;
  height: 208px;
  margin: 0 auto;
  transform: translateY(4px);
}

.scan-qrcode-viewport {
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;

  :deep(.oauth-qrcode) {
    width: 100%;
    height: 100%;
  }

  :deep(.oauth-qrcode-frame) {
    width: 100%;
    height: 100%;
  }

  :deep(.oauth-qrcode-sdk) {
    position: absolute;
    top: 50%;
    left: 50%;
    width: 300px;
    height: 300px;
    margin: -150px 0 0 -150px;
    transform: scale(1.14);
    transform-origin: center;
  }

  // 设计稿扫码区不展示提示文案；过期遮罩仍在框内展示并可刷新
  :deep(.oauth-qrcode-tip) {
    display: none;
  }

  :deep(.oauth-qrcode-mask) {
    z-index: 3;
  }
}

.scan-qrcode-logo {
  position: absolute;
  top: 50%;
  left: 50%;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  color: #fff;
  font-size: 22px;
  background: #0089ff;
  border: 3px solid #fff;
  border-radius: 6px;
  transform: translate(-50%, -50%);
  pointer-events: none;
}

// 四角蓝色装饰框（设计稿节点 3755:37714~37717），位于二维码四角外侧
.scan-corner {
  position: absolute;
  width: 22px;
  height: 22px;
  border: 2px solid #3157e2;
}

.scan-corner-tl {
  top: -9px;
  left: -9px;
  border-right: none;
  border-bottom: none;
  border-top-left-radius: 6px;
}

.scan-corner-tr {
  top: -9px;
  right: -9px;
  border-left: none;
  border-bottom: none;
  border-top-right-radius: 6px;
}

.scan-corner-bl {
  bottom: -9px;
  left: -9px;
  border-right: none;
  border-top: none;
  border-bottom-left-radius: 6px;
}

.scan-corner-br {
  bottom: -9px;
  right: -9px;
  border-left: none;
  border-top: none;
  border-bottom-right-radius: 6px;
}

.oauth-login {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 240px;
}

.oauth-login-empty {
  margin: 0;
  color: #8c8c8c;
  font-size: 13px;
  line-height: 20px;
  text-align: center;
}

// ========== Bind account dialog ==========
.bind-desc {
  margin: 0 0 16px;
  color: #8c8c8c;
  font-size: 13px;
  line-height: 20px;
  text-align: center;
}

// ========== Form ==========
.login-form {
  :deep(.ant-form-item) {
    margin-bottom: 16px;
  }

  :deep(.ant-input-affix-wrapper) {
    padding: 12px 16px;
    border-color: #d9d9d9;
    border-radius: 6px;
    box-shadow: none;
    background: #fff;
  }

  :deep(.ant-input-affix-wrapper:hover),
  :deep(.ant-input-affix-wrapper-focused) {
    border-color: #3157e2;
  }

  :deep(.ant-input-affix-wrapper > .ant-input-prefix) {
    margin-right: 8px;
  }

  :deep(.ant-input) {
    font-size: 16px;
    line-height: 24px;
    color: #262626;
  }

  :deep(.ant-input::placeholder) {
    color: rgba(0, 0, 0, 0.25);
  }

  :deep(.ant-input-password-icon) {
    color: #8c8c8c;
  }
}

.login-card-fixed .login-form {
  :deep(.ant-form-item) {
    margin-bottom: 32px;
  }

  :deep(.ant-form-item:nth-child(2)) {
    margin-bottom: 8px;
  }

  :deep(.ant-input-affix-wrapper) {
    height: 48px;
    padding: 11px 16px;
  }
}

.input-icon {
  width: 16px;
  height: 16px;
  display: block;
}

.login-forgot {
  text-align: left;
  margin: 0 0 32px;
}

.forgot-link {
  font-size: 14px;
  color: #2475fc;
  line-height: 22px;
  cursor: pointer;

  &:hover {
    color: #3157e2;
  }
}

.submit-button {
  height: 48px;
  border-radius: 6px;
  font-size: 16px;
  font-weight: 400;
  line-height: 24px;
  background: #3157e2;
  border-color: #3157e2;
  box-shadow: none;

  &:hover,
  &:focus {
    background: #2548c4;
    border-color: #2548c4;
  }

  &:disabled,
  &.ant-btn-loading {
    background: #3157e2;
    border-color: #3157e2;
    opacity: 0.4;
  }
}

.login-card-fixed .submit-button {
  background: #a7b8ed;
  border-color: #a7b8ed;

  &:hover,
  &:focus {
    background: #8fa6e9;
    border-color: #8fa6e9;
  }
}

// ========== Forgot Password Modal ==========
.forgot-steps {
  margin-bottom: 24px;
}

.forgot-form {
  margin-top: 8px;

  :deep(.ant-form-item) {
    margin-bottom: 16px;
  }

  :deep(.ant-input-affix-wrapper) {
    padding: 12px 16px;
    border-color: #d9d9d9;
    border-radius: 6px;
    box-shadow: none;
    background: #fff;
  }

  :deep(.ant-input-affix-wrapper:hover),
  :deep(.ant-input-affix-wrapper-focused) {
    border-color: #3157e2;
  }

  :deep(.ant-input) {
    font-size: 16px;
    line-height: 24px;
    color: #262626;
  }

  :deep(.ant-input::placeholder) {
    color: rgba(0, 0, 0, 0.25);
  }
}

// ========== Responsive ==========
@media (max-width: 1100px) {
  .login-card {
    width: 440px;
    padding: 48px 48px 40px;
  }
}

@media (max-width: 800px) {
  .login-form-side {
    padding: 24px;
  }

  .login-card {
    width: 100%;
    max-width: 500px;
    padding: 48px 32px 40px;
  }

  .login-title {
    font-size: 26px;
    margin-bottom: 28px;
  }
}

@media (max-width: 480px) {
  .login-card {
    padding: 40px 24px 32px;
  }

  .login-title {
    font-size: 24px;
    margin-bottom: 24px;
  }
}
</style>

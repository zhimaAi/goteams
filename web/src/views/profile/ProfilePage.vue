<template>
  <div class="profile-page">
    <a-tabs
      v-model:activeKey="activeTab"
      class="profile-tabs"
      :class="{ 'is-wide': activeTab === 'apiTokens' }"
    >
      <template #leftExtra>
        <div class="profile-heading">
          <span class="profile-heading-icon"><IdcardOutlined /></span>
          <h1 class="profile-heading-title">{{ $t("profile_title") }}</h1>
        </div>
      </template>

      <a-tab-pane key="profile" :tab="$t('profile_tab_profile')">
        <section class="profile-card">
          <div class="profile-summary">
            <div class="avatar-wrap">
              <a-avatar
                :size="78"
                :src="authStore.avatarUrl || undefined"
                :style="{
                  backgroundColor: authStore.avatarUrl ? undefined : '#3157e2',
                }"
              >
                <UserOutlined
                  v-if="!authStore.avatarUrl"
                  class="avatar-placeholder"
                />
              </a-avatar>
              <a-upload
                class="avatar-upload"
                :show-upload-list="false"
                accept="image/png,image/jpeg"
                :before-upload="validateAvatar"
                :custom-request="handleAvatarUpload"
                :disabled="avatarUploading"
              >
                <span class="avatar-camera">
                  <LoadingOutlined v-if="avatarUploading" />
                  <CameraOutlined v-else />
                </span>
              </a-upload>
            </div>

            <div class="profile-identity">
              <h2 class="avatar-name">
                {{ authStore.userInfo?.name || $t("profile_defaultName") }}
              </h2>
              <span class="avatar-account">
                <UserOutlined />
                {{ authStore.userInfo?.username || "-" }}
              </span>
            </div>
          </div>

          <div class="profile-form-section">
            <a-form
              ref="profileFormRef"
              layout="vertical"
              :model="profileForm"
              :rules="profileRules"
              @finish="handleSaveProfile"
            >
              <a-form-item :label="$t('profile_username')" name="name">
                <a-input
                  v-model:value="profileForm.name"
                  class="profile-input"
                  :placeholder="$t('profile_namePlaceholder')"
                >
                  <template #prefix><UserOutlined /></template>
                </a-input>
              </a-form-item>

              <a-form-item :label="$t('profile_account')">
                <a-input
                  :value="authStore.userInfo?.username"
                  class="profile-input"
                  disabled
                >
                  <template #prefix><UserOutlined /></template>
                </a-input>
              </a-form-item>

              <a-form-item :label="$t('profile_email')">
                <div class="profile-email-row">
                  <a-input
                    :value="
                      authStore.userInfo?.email || $t('profile_emailUnbound')
                    "
                    class="profile-input"
                    disabled
                  >
                    <template #prefix><MailOutlined /></template>
                  </a-input>
                  <a-button
                    type="link"
                    class="email-bind-button"
                    @click="openEmailModal"
                  >
                    {{
                      authStore.userInfo?.email
                        ? $t("profile_changeEmail")
                        : $t("profile_bindEmail")
                    }}
                  </a-button>
                </div>
              </a-form-item>

              <div class="profile-actions">
                <div class="primary-actions">
                  <a-button type="primary" html-type="submit" :loading="saving">
                    {{ $t("profile_saveProfile") }}
                  </a-button>
                  <a-button @click="handleResetProfile">
                    {{ $t("common_reset") }}
                  </a-button>
                </div>
                <a-button
                  type="link"
                  class="password-button"
                  @click="passwordModalOpen = true"
                >
                  <LockOutlined />
                  {{ $t("profile_changePassword") }}
                </a-button>
              </div>
            </a-form>
          </div>
        </section>

        <!-- Third-party account binding -->
        <section class="profile-card oauth-card">
          <h3 class="oauth-card-title">
            {{ $t("profile_oauthSectionTitle") }}
          </h3>

          <div class="oauth-tip">
            <InfoCircleFilled class="oauth-tip-icon" />
            <span>{{ $t("profile_oauthTip") }}</span>
          </div>

          <div
            v-for="item in oauthBindings"
            :key="item.provider"
            class="oauth-row"
          >
            <img
              class="oauth-icon"
              :src="oauthIcon(item.provider)"
              :alt="oauthProviderName(item.provider)"
            />
            <div class="oauth-info">
              <div class="oauth-name-line">
                <span class="oauth-name">{{
                  oauthProviderName(item.provider)
                }}</span>
                <span v-if="item.bound" class="oauth-badge">{{
                  $t("profile_oauthBound")
                }}</span>
              </div>
              <p class="oauth-desc">
                {{
                  item.bound
                    ? oauthBoundText(item)
                    : oauthProviderDesc(item.provider)
                }}
              </p>
            </div>
            <div class="oauth-action">
              <a-button
                v-if="item.bound"
                type="link"
                danger
                @click="confirmUnbind(item)"
              >
                <DisconnectOutlined />
                {{ $t("profile_oauthUnbind") }}
              </a-button>
              <a-button
                v-else
                type="link"
                :disabled="!item.enabled"
                @click="startBind(item)"
              >
                <LinkOutlined />
                {{ $t("profile_oauthBind") }}
              </a-button>
            </div>
          </div>
        </section>
      </a-tab-pane>

      <a-tab-pane key="apiTokens" :tab="$t('profile_tab_apiTokens')">
        <ApiTokenPanel />
      </a-tab-pane>
    </a-tabs>

    <!-- Change Password Dialog -->
    <a-modal
      v-model:open="passwordModalOpen"
      :title="$t('profile_changePassword')"
      :ok-text="$t('profile_confirmChange')"
      :cancel-text="$t('common_cancel')"
      :confirm-loading="passwordChanging"
      :destroy-on-close="true"
      @ok="handleChangePassword"
      @cancel="handlePasswordCancel"
    >
      <a-form
        ref="passwordFormRef"
        layout="vertical"
        style="margin-top: 16px"
        :model="passwordForm"
        :rules="passwordRules"
      >
        <a-form-item :label="$t('profile_oldPassword')" name="oldPassword">
          <a-input-password
            v-model:value="passwordForm.oldPassword"
            :placeholder="$t('profile_oldPwdPlaceholder')"
            autocomplete="current-password"
          >
            <template #prefix><LockOutlined /></template>
          </a-input-password>
        </a-form-item>

        <a-form-item :label="$t('profile_newPassword')" name="newPassword">
          <a-input-password
            v-model:value="passwordForm.newPassword"
            :placeholder="$t('profile_newPwdPlaceholder')"
            autocomplete="new-password"
          >
            <template #prefix><LockOutlined /></template>
          </a-input-password>
        </a-form-item>

        <a-form-item
          :label="$t('profile_confirmPassword')"
          name="confirmPassword"
        >
          <a-input-password
            v-model:value="passwordForm.confirmPassword"
            :placeholder="$t('profile_confirmPwdPlaceholder')"
            autocomplete="new-password"
          >
            <template #prefix><LockOutlined /></template>
          </a-input-password>
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- Bind / change email dialog -->
    <a-modal
      v-model:open="emailModalOpen"
      :title="
        authStore.userInfo?.email
          ? $t('profile_changeEmail')
          : $t('profile_bindEmail')
      "
      :ok-text="$t('common_confirm')"
      :cancel-text="$t('common_cancel')"
      :confirm-loading="emailBinding"
      :destroy-on-close="true"
      @ok="handleBindEmail"
      @cancel="handleEmailCancel"
    >
      <a-form
        ref="emailFormRef"
        layout="vertical"
        style="margin-top: 16px"
        :model="emailForm"
        :rules="emailRules"
      >
        <a-form-item :label="$t('profile_email')" name="email">
          <a-input
            v-model:value="emailForm.email"
            :maxlength="255"
            :placeholder="$t('profile_emailPlaceholder')"
          >
            <template #prefix><MailOutlined /></template>
          </a-input>
        </a-form-item>

        <a-form-item :label="$t('profile_emailCode')" name="verifyCode">
          <a-space-compact style="width: 100%">
            <a-input
              v-model:value="emailForm.verifyCode"
              :maxlength="6"
              :placeholder="$t('profile_emailCodePlaceholder')"
            />
            <a-button
              :loading="codeSending"
              :disabled="codeCountdown > 0 || !emailForm.email.trim()"
              @click="handleSendEmailCode"
            >
              {{
                codeCountdown > 0
                  ? `${codeCountdown}s`
                  : $t("profile_sendEmailCode")
              }}
            </a-button>
          </a-space-compact>
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- Third-party Binding Dialog -->
    <a-modal
      v-model:open="oauthModalOpen"
      :title="oauthModalTitle"
      :footer="null"
      :maskClosable="false"
      width="400px"
      destroyOnClose
    >
      <div class="oauth-modal-body">
        <a-spin v-if="oauthModalLoading" />
        <OAuthQrcode
          v-else-if="oauthModalAuthorize"
          :provider="oauthModalProvider"
          :authorize="oauthModalAuthorize"
          :expires-in="oauthModalAuthorize.expires_in || 0"
          :tip="oauthModalTip"
          @error="handleOauthQrcodeError"
          @refresh="refreshOauthQrcode"
        />
        <p v-else class="oauth-modal-empty">
          {{ $t("profile_oauthBindFailed") }}
        </p>
      </div>
    </a-modal>
  </div>
</template>

<script setup>
import { ref, computed, reactive, onMounted, onUnmounted } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import {
  changePassword,
  uploadAvatar,
  sendBindEmailCode,
  bindEmail,
} from "@/api/auth";
import {
  fetchOauthBindings,
  fetchUserOauthAuthorize,
  unbindOauthAccount,
} from "@/api/oauth";
import ApiTokenPanel from "./ApiTokenPanel.vue";
import OAuthQrcode from "@/components/common/OAuthQrcode.vue";
import { message, Modal } from "ant-design-vue";
import {
  IdcardOutlined,
  UserOutlined,
  LockOutlined,
  MailOutlined,
  CameraOutlined,
  LoadingOutlined,
  InfoCircleFilled,
  LinkOutlined,
  DisconnectOutlined,
} from "@ant-design/icons-vue";
import iconFeishu from "@/image/login/icon-feishu.svg";
import iconDingtalk from "@/image/login/icon-dingtalk.svg";

const { t } = useI18n();
const authStore = useAuthStore();
const route = useRoute();
const router = useRouter();

const saving = ref(false);
const activeTab = ref("profile");
const avatarUploading = ref(false);
const passwordChanging = ref(false);
const passwordModalOpen = ref(false);
const profileFormRef = ref();
const passwordFormRef = ref();

const profileForm = reactive({ name: authStore.userInfo?.name || "" });
const passwordForm = reactive({
  oldPassword: "",
  newPassword: "",
  confirmPassword: "",
});

const emailModalOpen = ref(false);
const emailBinding = ref(false);
const emailFormRef = ref();
const emailForm = reactive({ email: "", verifyCode: "" });
const codeSending = ref(false);
const codeCountdown = ref(0);
let codeTimer = null;

const profileRules = computed(() => ({
  name: [
    {
      required: true,
      whitespace: true,
      message: t("profile_nameRequired"),
      trigger: "blur",
    },
    { max: 100, message: t("profile_nameTooLong"), trigger: "blur" },
  ],
}));
const passwordRules = computed(() => ({
  oldPassword: [
    { required: true, message: t("profile_oldPwdRequired"), trigger: "blur" },
  ],
  newPassword: [
    { required: true, message: t("profile_newPwdRequired"), trigger: "blur" },
    { validator: validateNewPassword, trigger: "blur" },
  ],
  confirmPassword: [
    {
      required: true,
      message: t("profile_confirmPwdRequired"),
      trigger: "blur",
    },
    { validator: validateConfirmPwd, trigger: "blur" },
  ],
}));
const emailRules = computed(() => ({
  email: [
    { required: true, message: t("profile_emailRequired"), trigger: "blur" },
    { type: "email", message: t("profile_emailInvalid"), trigger: "blur" },
  ],
  verifyCode: [
    {
      required: true,
      message: t("profile_emailCodeRequired"),
      trigger: "blur",
    },
  ],
}));

onMounted(async () => {
  profileForm.name = authStore.userInfo?.name || "";
  if (authStore.userInfo?.avatar) {
    authStore.loadAvatar().catch(() => {});
  }
  await loadOauthBindings();
  await handleOauthBindQuery();
});

// ========== Third-party account binding ==========
const OAUTH_NAME_KEYS = {
  dingtalk: "profile_oauthDingtalkName",
  feishu: "profile_oauthFeishuName",
};

const OAUTH_DESC_KEYS = {
  dingtalk: "profile_oauthDingtalkDesc",
  feishu: "profile_oauthFeishuDesc",
};

const oauthBindings = ref([]);
const oauthModalOpen = ref(false);
const oauthModalProvider = ref("");
const oauthModalAuthorize = ref(null);
const oauthModalLoading = ref(false);

function oauthProviderName(provider) {
  return t(OAUTH_NAME_KEYS[provider] || provider);
}

function oauthProviderDesc(provider) {
  return t(OAUTH_DESC_KEYS[provider] || provider);
}

function oauthIcon(provider) {
  return provider === "feishu" ? iconFeishu : iconDingtalk;
}

function oauthBoundText(item) {
  return t("profile_oauthBoundAccount", {
    name: item.nickname || oauthProviderName(item.provider),
  });
}

const oauthModalTitle = computed(() =>
  t("profile_oauthBindTitle", {
    app: oauthProviderName(oauthModalProvider.value),
  }),
);

const oauthModalTip = computed(() =>
  t("profile_oauthScanBindTip", {
    app: oauthProviderName(oauthModalProvider.value),
  }),
);

async function loadOauthBindings() {
  try {
    oauthBindings.value = (await fetchOauthBindings()) || [];
  } catch {
    // A failed lookup only hides the binding states; the rest of the page stays usable.
    oauthBindings.value = [];
  }
}

/**
 * Fetch a binding QR code entry. Refreshing an expired code reuses this path, so the dialog gets a
 * newly issued state and the dead code cannot be scanned again.
 */
async function loadOauthAuthorize(provider) {
  oauthModalAuthorize.value = null;
  oauthModalLoading.value = true;
  try {
    oauthModalAuthorize.value = await fetchUserOauthAuthorize(provider);
  } catch {
    oauthModalAuthorize.value = null;
  } finally {
    oauthModalLoading.value = false;
  }
}

async function startBind(item) {
  oauthModalProvider.value = item.provider;
  oauthModalOpen.value = true;
  await loadOauthAuthorize(item.provider);
}

function refreshOauthQrcode() {
  return loadOauthAuthorize(oauthModalProvider.value);
}

function handleOauthQrcodeError() {
  message.error(t("profile_oauthBindFailed"));
}

function confirmUnbind(item) {
  Modal.confirm({
    title: t("profile_oauthUnbindConfirmTitle", {
      app: oauthProviderName(item.provider),
    }),
    content: t("profile_oauthUnbindConfirm"),
    okText: t("common_confirm"),
    cancelText: t("common_cancel"),
    okButtonProps: { danger: true },
    onOk: async () => {
      try {
        await unbindOauthAccount(item.provider);
        message.success(t("profile_oauthUnbindSuccess"));
        await loadOauthBindings();
      } catch (err) {
        message.error(err?.message || t("common_requestFailed"));
      }
    },
  });
}

/** Surface the result of the binding callback, which redirects back here with a query param. */
async function handleOauthBindQuery() {
  const result =
    typeof route.query.oauth_bind === "string" ? route.query.oauth_bind : "";
  if (!result) return;
  const provider =
    typeof route.query.provider === "string" ? route.query.provider : "";
  await router.replace("/profile");

  if (result === "success") {
    message.success(
      t("profile_oauthBindSuccess", { app: oauthProviderName(provider) }),
    );
    await loadOauthBindings();
    return;
  }
  if (result === "conflict") {
    message.error(t("profile_oauthBindConflict"));
    return;
  }
  message.error(t("profile_oauthBindFailed"));
}

onUnmounted(() => clearInterval(codeTimer));

function validateNewPassword(_rule, value) {
  if (!value) return Promise.resolve();
  const valid =
    value.length >= 8 &&
    value.length <= 32 &&
    /^[a-zA-Z0-9_\-.!@#$%^&*]+$/.test(value);
  return valid
    ? Promise.resolve()
    : Promise.reject(new Error(t("profile_passwordFormat")));
}

function validateConfirmPwd(_rule, value) {
  if (!value || passwordForm.newPassword === value) return Promise.resolve();
  return Promise.reject(new Error(t("profile_passwordMismatch")));
}

function validateAvatar(file) {
  const isImage = file.type === "image/jpeg" || file.type === "image/png";
  if (!isImage) {
    message.error(t("workspace_imageRequired"));
    return false;
  }
  const isLt2M = file.size / 1024 / 1024 <= 2;
  if (!isLt2M) {
    message.error(t("workspace_imageTooLarge"));
    return false;
  }
  return true;
}

async function handleAvatarUpload({ file, onSuccess, onError }) {
  avatarUploading.value = true;
  try {
    const result = await uploadAvatar(file);
    await authStore.fetchUserInfo();
    authStore.setAvatarUrl(result?.avatar || "");
    message.success(t("profile_avatarUploadSuccess"));
    onSuccess(result);
  } catch (error) {
    onError(error);
  } finally {
    avatarUploading.value = false;
  }
}

async function handleSaveProfile() {
  saving.value = true;
  try {
    const user = await authStore.updateProfile({
      name: profileForm.name.trim(),
    });
    profileForm.name = user?.name || "";
    message.success(t("profile_profileUpdateSuccess"));
  } catch {
    // The request layer displays errors uniformly.
  } finally {
    saving.value = false;
  }
}

function handleResetProfile() {
  profileForm.name = authStore.userInfo?.name || "";
  profileFormRef.value?.clearValidate();
}

async function handleChangePassword() {
  try {
    await passwordFormRef.value?.validate();
  } catch {
    return;
  }
  passwordChanging.value = true;
  try {
    await changePassword({
      old_password: passwordForm.oldPassword,
      new_password: passwordForm.newPassword,
    });
    message.success(t("profile_passwordChangeSuccess"));
    passwordModalOpen.value = false;
    resetPasswordForm();
  } catch {
    // The request layer displays errors uniformly.
  } finally {
    passwordChanging.value = false;
  }
}

function handlePasswordCancel() {
  passwordModalOpen.value = false;
  resetPasswordForm();
}

function resetPasswordForm() {
  passwordForm.oldPassword = "";
  passwordForm.newPassword = "";
  passwordForm.confirmPassword = "";
  passwordFormRef.value?.clearValidate();
}

function openEmailModal() {
  resetEmailForm();
  emailModalOpen.value = true;
}

// The code is mailed to the address being bound, so an email only becomes bindable once its
// owner proves control by entering the code.
async function handleSendEmailCode() {
  try {
    await emailFormRef.value?.validateFields(["email"]);
  } catch {
    return;
  }
  codeSending.value = true;
  try {
    await sendBindEmailCode({ email: emailForm.email.trim() });
    message.success(t("profile_emailCodeSent"));
    codeCountdown.value = 60;
    clearInterval(codeTimer);
    codeTimer = setInterval(() => {
      codeCountdown.value -= 1;
      if (codeCountdown.value <= 0) clearInterval(codeTimer);
    }, 1000);
  } catch {
    // The request layer displays errors uniformly.
  } finally {
    codeSending.value = false;
  }
}

async function handleBindEmail() {
  try {
    await emailFormRef.value?.validate();
  } catch {
    return;
  }
  emailBinding.value = true;
  try {
    await bindEmail({
      email: emailForm.email.trim(),
      verifyCode: emailForm.verifyCode.trim(),
    });
    await authStore.fetchUserInfo();
    message.success(t("profile_emailBindSuccess"));
    emailModalOpen.value = false;
    resetEmailForm();
  } catch {
    // The request layer displays errors uniformly.
  } finally {
    emailBinding.value = false;
  }
}

function handleEmailCancel() {
  emailModalOpen.value = false;
  resetEmailForm();
}

function resetEmailForm() {
  emailForm.email = "";
  emailForm.verifyCode = "";
  emailFormRef.value?.clearValidate();
}
</script>

<style scoped lang="less">
.profile-page {
  --profile-page-bg: #f5f7fb;
  --profile-card-bg: #fff;
  --profile-summary-bg: #f2f5ff;
  --profile-border: #dfe5ef;
  --profile-muted: #65738d;
  --oauth-tip-bg: #eef4ff;
  --oauth-tip-border: #cfe0ff;
  --oauth-tip-text: #3157e2;
  --oauth-badge-bg: #e8f7ee;
  --oauth-badge-text: #12a150;

  flex: 1;
  min-height: 0;
  margin: -10px -24px -24px;
  // 页头行（图标 + 标题 + tab，nav 高 46px）与侧栏 logo 条（高 64px）垂直居中：
  // 46 / 2 + 9 = 32，落在 logo 条中心，故上内边距取 9px，不要改回 26px。
  padding: 0 32px 48px;
  overflow: auto;
  background: var(--profile-page-bg);
}

.profile-tabs {
  // nav 撑满整行（分隔线通栏），tab 内容与卡片同一列宽、居中
  :deep(.ant-tabs-content-holder) {
    width: min(100%, 720px);
    margin: 0 auto;
  }

  &.is-wide :deep(.ant-tabs-content-holder) {
    width: min(100%, 1080px);
  }

  // 页头行固定在滚动容器顶部，不随内容滚动；
  // top 取负值抵消 .profile-page 的 9px 上内边距，滚动后紧贴顶边。
  :deep(.ant-tabs-nav) {
    padding-top: 9px;
    position: sticky;
    top: 0;
    z-index: 10;
    background: var(--profile-page-bg);
  }
}

// 页头：图标方块 + 小标题，与 tab 同一行（对齐「我的工作」页）
.profile-heading {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-right: 56px;
}

.profile-heading-icon {
  flex-shrink: 0;
  width: 24px;
  height: 24px;
  border-radius: 6px;
  background: #0d68ff;
  color: #fff;
  font-size: 13px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.profile-heading-title {
  margin: 0;
  color: var(--app-text);
  font-size: 16px;
  font-weight: 400;
  line-height: 24px;
  white-space: nowrap;
}

.profile-card {
  overflow: hidden;
  border: 1px solid var(--profile-border);
  border-radius: 8px;
  background: var(--profile-card-bg);
}

.profile-summary {
  min-height: 137px;
  padding: 28px 32px;
  display: flex;
  align-items: center;
  gap: 22px;
  background: var(--profile-summary-bg);
  border-bottom: 1px solid var(--profile-border);
}

.profile-identity {
  min-width: 0;

  .avatar-name {
    margin: 0 0 6px;
    color: var(--app-text);
    font-size: 16px;
    font-weight: 600;
    line-height: 1.5;
  }

  .avatar-account {
    display: flex;
    align-items: center;
    gap: 7px;
    color: var(--profile-muted);
    font-size: 14px;
    line-height: 1.5;
    overflow-wrap: anywhere;
  }
}

.avatar-wrap {
  position: relative;
  flex: 0 0 78px;
  height: 78px;

  :deep(.ant-avatar) {
    border: 2px solid #fff;
    box-shadow: 0 4px 12px rgba(49, 87, 226, 0.18);
  }

  .avatar-placeholder {
    font-size: 34px;
  }

  :deep(.avatar-upload) {
    position: absolute;
    right: -1px;
    bottom: 0;
    line-height: 1;
  }

  .avatar-camera {
    width: 27px;
    height: 27px;
    border-radius: 50%;
    border: 2px solid #fff;
    background-color: #3157e2;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    box-shadow: 0 2px 6px rgba(49, 87, 226, 0.28);
    color: #fff;
    font-size: 13px;
  }
}

.profile-form-section {
  padding: 25px 32px 27px;

  :deep(.ant-form-item) {
    margin-bottom: 24px;
  }

  :deep(.ant-form-item-label) {
    padding-bottom: 8px;
  }

  :deep(.ant-form-item-label > label) {
    color: var(--app-text);
    font-weight: 600;
  }
}

.profile-input {
  min-height: 42px;

  :deep(.ant-input-prefix) {
    color: #9aa9bf;
  }
}

.profile-email-row {
  display: flex;
  align-items: center;
  gap: 10px;

  .profile-input {
    flex: 1;
    min-width: 0;
  }

  .email-bind-button {
    flex: 0 0 auto;
    height: auto;
    padding-inline: 0;
  }
}

.profile-actions {
  padding-top: 20px;
  border-top: 1px solid var(--profile-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;

  .primary-actions {
    display: flex;
    gap: 10px;
  }

  :deep(.ant-btn) {
    min-height: 40px;
    padding-inline: 20px;
  }

  .password-button {
    padding-inline: 0;
  }
}

:global(:root[data-theme="dark"]) .profile-page {
  --profile-page-bg: #141414;
  --profile-card-bg: #1f1f1f;
  --profile-summary-bg: #20283a;
  --profile-border: #30394a;
  --profile-muted: rgba(255, 255, 255, 0.55);
  --oauth-tip-bg: #1b2740;
  --oauth-tip-border: #2b3d63;
  --oauth-tip-text: #7fa2ff;
  --oauth-badge-bg: #14301f;
  --oauth-badge-text: #4ade80;
}

@media (max-width: 640px) {
  .profile-page {
    // 上内边距与桌面端一致，窄屏下页头行同样与侧栏 logo 条垂直居中
    padding: 0 16px 32px;
  }

  .profile-heading {
    margin-right: 24px;
  }

  .profile-summary {
    min-height: 120px;
    padding: 22px 20px;
    gap: 16px;
  }

  .profile-form-section {
    padding: 22px 20px 24px;
  }

  .profile-actions {
    align-items: stretch;
    flex-direction: column;

    .primary-actions {
      width: 100%;

      :deep(.ant-btn) {
        flex: 1;
      }
    }

    .password-button {
      align-self: flex-end;
    }
  }
}

@media (max-width: 480px) {
  .profile-actions .primary-actions {
    flex-direction: column;
  }
}

// ========== Third-party account binding ==========
.oauth-card {
  margin-top: 16px;
  padding: 24px 32px 28px;
}

.oauth-card-title {
  margin: 0 0 16px;
  font-size: 16px;
  font-weight: 600;
  line-height: 24px;
}

.oauth-tip {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border: 1px solid var(--oauth-tip-border);
  border-radius: 6px;
  background: var(--oauth-tip-bg);
  color: var(--oauth-tip-text);
  font-size: 13px;
  line-height: 20px;
}

.oauth-tip-icon {
  flex: none;
  font-size: 14px;
}

.oauth-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 0;
  border-bottom: 1px solid var(--profile-border);

  &:last-child {
    padding-bottom: 0;
    border-bottom: none;
  }
}

.oauth-icon {
  flex: none;
  width: 32px;
  height: 32px;
}

.oauth-info {
  flex: 1;
  min-width: 0;
}

.oauth-name-line {
  display: flex;
  align-items: center;
  gap: 8px;
}

.oauth-name {
  font-size: 14px;
  font-weight: 500;
  line-height: 22px;
}

.oauth-badge {
  padding: 0 6px;
  border-radius: 4px;
  background: var(--oauth-badge-bg);
  color: var(--oauth-badge-text);
  font-size: 12px;
  line-height: 18px;
}

.oauth-desc {
  margin: 2px 0 0;
  color: var(--profile-muted);
  font-size: 12px;
  line-height: 20px;
}

.oauth-action {
  flex: none;

  :deep(.ant-btn-link) {
    padding-inline: 0;
  }
}

.oauth-modal-body {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 340px;
}

// The dialog is teleported outside .profile-page, so the theme variables are not in scope here.
.oauth-modal-empty {
  margin: 0;
  color: #8c8c8c;
  font-size: 13px;
  text-align: center;
}
</style>

<template>
  <div class="invite-page">
    <!-- Left brand showcase (same as login) -->
    <BrandPanel />

    <!-- Right join card -->
    <section class="invite-form-side">
      <!-- Language switch -->
      <div class="invite-lang-switch" @click="localeStore.toggleLang()">
        <img :src="iconGlobe" class="lang-icon" alt="" />
        <span class="lang-label">{{
          lang === "zh-CN" ? "中文" : "English"
        }}</span>
        <img :src="iconChevronDown" class="lang-arrow" alt="" />
      </div>

      <!-- Expired result card -->
      <div v-if="expired" class="invite-card invite-expired-card">
        <a-result
          status="404"
          :title="lt('invite_expiredTitle')"
          :sub-title="lt('invite_expiredDesc')"
        />
      </div>

      <!-- Join form card -->
      <div v-else-if="loaded" class="invite-card">
        <!-- Countdown pill -->
        <div class="invite-countdown-pill">
          <ClockCircleOutlined />
          <span>{{ lt("invite_countdownLabel") }}</span>
          <span class="countdown-time">{{ countdownText }}</span>
        </div>

        <h2 class="invite-title">{{ inviteTitle }}</h2>

        <!-- Avatar upload -->
        <div class="invite-avatar-section">
          <div class="invite-avatar-wrap">
            <a-avatar
              :size="88"
              :src="avatarPreview || undefined"
              class="invite-avatar"
            >
              {{
                avatarPreview ? "" : (form.name || "").charAt(0).toUpperCase()
              }}
            </a-avatar>
            <a-upload
              :show-upload-list="false"
              accept="image/png,image/jpeg"
              :before-upload="validateAvatar"
              :custom-request="handleAvatarUpload"
              :disabled="avatarUploading"
            >
              <button
                type="button"
                class="invite-avatar-camera"
                :aria-label="lt('member_avatarUpload')"
              >
                <LoadingOutlined v-if="avatarUploading" />
                <CameraOutlined v-else />
              </button>
            </a-upload>
          </div>
          <p class="invite-avatar-hint">{{ lt("member_avatarHint") }}</p>
        </div>

        <a-form
          ref="formRef"
          class="invite-form"
          :model="form"
          :rules="rules"
          layout="vertical"
          @finish="handleSubmit"
        >
          <a-form-item :label="lt('member_name')" name="name">
            <a-input
              v-model:value="form.name"
              :placeholder="lt('member_namePlaceholder')"
              size="large"
            />
          </a-form-item>
          <a-form-item :label="lt('member_account')" name="username">
            <a-input
              v-model:value="form.username"
              :placeholder="lt('member_accountPlaceholder')"
              autocomplete="username"
              size="large"
            />
          </a-form-item>
          <a-form-item :label="lt('member_password')" name="password">
            <a-input-password
              v-model:value="form.password"
              :placeholder="lt('member_passwordPlaceholder')"
              autocomplete="new-password"
              size="large"
            />
          </a-form-item>
          <a-form-item
            :label="lt('invite_phone')"
            name="phone"
            :required="phoneRequired"
          >
            <a-input
              v-model:value="form.phone"
              :maxlength="11"
              :placeholder="lt('invite_phonePlaceholder')"
              size="large"
            />
          </a-form-item>
          <a-form-item
            :label="lt('member_email')"
            name="email"
            :required="emailRequired"
          >
            <a-input
              v-model:value="form.email"
              :maxlength="255"
              :placeholder="lt('member_emailPlaceholder')"
              size="large"
            />
          </a-form-item>
          <!-- The code is only required once an email is entered, so the field stays hidden until then. -->
          <a-form-item
            v-if="form.email.trim()"
            :label="lt('invite_emailCode')"
            name="verifyCode"
            required
          >
            <a-space-compact style="width: 100%">
              <a-input
                v-model:value="form.verifyCode"
                :maxlength="6"
                :placeholder="lt('invite_emailCodePlaceholder')"
                size="large"
              />
              <a-button
                size="large"
                :loading="codeSending"
                :disabled="codeCountdown > 0"
                @click="handleSendEmailCode"
              >
                {{
                  codeCountdown > 0
                    ? `${codeCountdown}s`
                    : lt("invite_sendEmailCode")
                }}
              </a-button>
            </a-space-compact>
          </a-form-item>

          <a-button
            type="primary"
            html-type="submit"
            class="submit-button"
            :loading="submitting"
            :disabled="avatarUploading"
            block
          >
            {{ lt2("invite_submitJoin", { role: roleName }) }}
          </a-button>
        </a-form>
      </div>
    </section>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { message } from "ant-design-vue";
import {
  CameraOutlined,
  ClockCircleOutlined,
  LoadingOutlined,
} from "@ant-design/icons-vue";
import {
  getInviteInfo,
  uploadInviteAvatar,
  sendInviteEmailCode,
  acceptInvite,
} from "@/api/invite";
import { useLocaleStore } from "@/stores/locale";
import zhCNMessages from "@/locales/zh-CN";
import enUSMessages from "@/locales/en-US";
import BrandPanel from "@/components/common/BrandPanel.vue";
import iconGlobe from "@/image/login/icon-globe.svg";
import iconChevronDown from "@/image/login/icon-chevron-down.svg";

const route = useRoute();
const router = useRouter();
const localeStore = useLocaleStore();

const MESSAGES = { "zh-CN": zhCNMessages, "en-US": enUSMessages };
const lang = computed(() => localeStore.currentLang);

function lt(key) {
  return MESSAGES[lang.value]?.[key] || MESSAGES["zh-CN"][key] || key;
}

function lt2(key, vars) {
  let tpl = lt(key);
  Object.entries(vars).forEach(([k, v]) => {
    tpl = tpl.replace(`{${k}}`, v);
  });
  return tpl;
}

const token = String(route.params.token || "");
const loaded = ref(false);
const expired = ref(false);
const roleName = ref("");
const inviterName = ref("");
const enterpriseName = ref("");
const phoneRequired = ref(false);
const emailRequired = ref(false);
const expiredAt = ref(0);
const countdownText = ref("00:00");
let countdownTimer = null;

const formRef = ref();
const submitting = ref(false);
const form = ref({
  name: "",
  username: "",
  password: "",
  phone: "",
  email: "",
  verifyCode: "",
});
const codeSending = ref(false);
const codeCountdown = ref(0);
let codeTimer = null;
const avatarPreview = ref("");
const avatarPath = ref("");
const avatarUploading = ref(false);

const inviteTitle = computed(() =>
  lt2("invite_joinTitle", {
    inviter: inviterName.value,
    enterprise: enterpriseName.value || lt("org_unnamedEnterprise"),
  }),
);

const rules = computed(() => ({
  name: [
    { required: true, message: lt("member_namePlaceholder"), trigger: "blur" },
  ],
  username: [
    {
      required: true,
      message: lt("member_accountPlaceholder"),
      trigger: "blur",
    },
  ],
  password: [
    {
      required: true,
      message: lt("member_passwordPlaceholder"),
      trigger: "blur",
    },
    { validator: validatePassword, trigger: "blur" },
  ],
  phone: [
    {
      required: phoneRequired.value,
      message: lt("invite_phonePlaceholder"),
      trigger: "blur",
    },
    {
      pattern: /^1\d{10}$/,
      message: lt("invite_phoneInvalid"),
      trigger: "blur",
    },
  ],
  email: [
    {
      required: emailRequired.value,
      message: lt("member_emailPlaceholder"),
      trigger: "blur",
    },
    {
      type: "email",
      message: lt("invite_emailInvalid"),
      trigger: "blur",
    },
  ],
  verifyCode: [
    {
      // Mark required so FormItem does not append a built-in required rule,
      // whose default message would duplicate the validator's message.
      required: true,
      validator: (_rule, value) => {
        if (!form.value.email.trim() || value) return Promise.resolve();
        return Promise.reject(new Error(lt("invite_emailCodePlaceholder")));
      },
      trigger: "blur",
    },
  ],
}));

onMounted(async () => {
  try {
    const info = await getInviteInfo(token);
    roleName.value = info?.role_name || "";
    inviterName.value = info?.inviter_name || "";
    enterpriseName.value = info?.enterprise_name || "";
    phoneRequired.value = Boolean(info?.phone_required);
    emailRequired.value = Boolean(info?.email_required);
    expiredAt.value = Number(info?.expired_at) || 0;
    loaded.value = true;
    startCountdown();
  } catch {
    expired.value = true;
  }
});

onUnmounted(() => {
  clearInterval(countdownTimer);
  clearInterval(codeTimer);
});

// The email code is sent by the public endpoint; the invitation token is the only credential.
async function handleSendEmailCode() {
  const email = form.value.email.trim();
  if (!email) {
    message.warning(lt("member_emailPlaceholder"));
    return;
  }
  codeSending.value = true;
  try {
    await sendInviteEmailCode(token, { email });
    message.success(lt("invite_emailCodeSent"));
    codeCountdown.value = 60;
    clearInterval(codeTimer);
    codeTimer = setInterval(() => {
      codeCountdown.value -= 1;
      if (codeCountdown.value <= 0) clearInterval(codeTimer);
    }, 1000);
  } catch {
    /* handled by interceptor */
  } finally {
    codeSending.value = false;
  }
}

// Password rules match the backend IsValidPassword: 8-32 chars, letters/digits/common special chars.
function validatePassword(_rule, value) {
  if (!value) return Promise.resolve();
  const valid =
    value.length >= 8 &&
    value.length <= 32 &&
    /^[a-zA-Z0-9_\-.!@#$%^&*]+$/.test(value);
  return valid
    ? Promise.resolve()
    : Promise.reject(new Error(lt("profile_passwordFormat")));
}

function startCountdown() {
  updateCountdown();
  clearInterval(countdownTimer);
  countdownTimer = setInterval(updateCountdown, 1000);
}

function updateCountdown() {
  const remain = expiredAt.value - Math.floor(Date.now() / 1000);
  if (remain <= 0) {
    clearInterval(countdownTimer);
    expired.value = true;
    return;
  }
  const minutes = String(Math.floor(remain / 60)).padStart(2, "0");
  const seconds = String(remain % 60).padStart(2, "0");
  countdownText.value = `${minutes}:${seconds}`;
}

function validateAvatar(file) {
  const isImage = file.type === "image/jpeg" || file.type === "image/png";
  if (!isImage) {
    message.error(lt("member_avatarInvalid"));
    return false;
  }
  if (file.size > 100 * 1024) {
    message.error(lt("member_avatarTooLarge"));
    return false;
  }
  if (avatarPreview.value.startsWith("blob:")) {
    URL.revokeObjectURL(avatarPreview.value);
  }
  avatarPreview.value = URL.createObjectURL(file);
  return true;
}

async function handleAvatarUpload({ file, onSuccess, onError }) {
  avatarUploading.value = true;
  try {
    const result = await uploadInviteAvatar(token, file);
    avatarPath.value = result?.url || "";
    onSuccess(result);
  } catch (error) {
    avatarPreview.value = "";
    avatarPath.value = "";
    onError(error);
  } finally {
    avatarUploading.value = false;
  }
}

async function handleSubmit() {
  submitting.value = true;
  try {
    await acceptInvite(token, {
      name: form.value.name.trim(),
      username: form.value.username.trim(),
      password: form.value.password,
      phone: form.value.phone.trim(),
      email: form.value.email.trim(),
      verifyCode: form.value.verifyCode.trim(),
      avatar: avatarPath.value,
    });
    message.success(lt("invite_joinSuccess"));
    router.push("/login");
  } catch (err) {
    // Error messages are uniformly toasted by the Axios interceptor; here only handle 404 (invite expired) by switching to the expired result page.
    if (err?.response?.status === 404) {
      expired.value = true;
    }
  } finally {
    submitting.value = false;
  }
}
</script>

<style scoped lang="less">
.invite-page {
  display: flex;
  min-height: 100vh;
  background: #f7f8fb;
}

.invite-form-side {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  min-width: 0;
  padding: 40px 24px;
}

.invite-lang-switch {
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

.lang-icon,
.lang-arrow {
  width: 16px;
  height: 16px;
}

.lang-label {
  font-size: 14px;
  color: #262626;
  line-height: 22px;
}

.invite-card {
  width: 500px;
  padding: 40px 60px 48px;
  background: #fff;
  border-radius: 24px;
  box-shadow: 0px 4px 32px 0px rgba(0, 0, 0, 0.08);
}

.invite-expired-card {
  padding: 48px 40px;
}

.invite-countdown-pill {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin: 0 auto 24px;
  padding: 8px 20px;
  width: fit-content;
  border-radius: 999px;
  color: #1677ff;
  background: #eef4ff;
  font-size: 14px;
  font-weight: 500;

  .countdown-time {
    font-family: "SFMono-Regular", Consolas, monospace;
    font-weight: 700;
  }
}

.invite-title {
  margin: 0 0 8px;
  overflow-wrap: anywhere;
  font-size: 26px;
  font-weight: 600;
  line-height: 36px;
  color: #000;
  text-align: center;
}

.invite-avatar-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  margin: 16px 0 8px;
}

.invite-avatar-wrap {
  position: relative;
  width: 88px;
  height: 88px;
  margin-bottom: 12px;
}

.invite-avatar {
  border: 3px solid #fff;
  color: #fff;
  background: #0d68ff;
  box-shadow: 0 4px 14px rgba(13, 104, 255, 0.15);
  font-size: 32px;
}

.invite-avatar-camera {
  position: absolute;
  right: 0;
  bottom: 0;
  display: flex;
  width: 32px;
  height: 32px;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 2px solid #fff;
  border-radius: 50%;
  color: #fff;
  background: #0d68ff;
  box-shadow: 0 3px 4px rgba(13, 104, 255, 0.3);
  cursor: pointer;
  font-size: 13px;
}

.invite-avatar-hint {
  margin: 0;
  color: rgba(0, 0, 0, 0.45);
  font-size: 13px;
  line-height: 20px;
}

.invite-form {
  margin-top: 8px;

  :deep(.ant-form-item) {
    margin-bottom: 16px;
  }

  :deep(.ant-input-affix-wrapper),
  :deep(.ant-input) {
    border-radius: 6px;
  }
}

.submit-button {
  height: 48px;
  margin-top: 8px;
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
}

@media (max-width: 800px) {
  .invite-card {
    width: 100%;
    max-width: 500px;
    padding: 40px 32px;
  }
}
</style>

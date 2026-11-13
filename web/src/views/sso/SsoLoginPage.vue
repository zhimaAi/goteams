<template>
  <div class="sso-login-page">
    <BrandPanel />
    <section class="sso-form-side">
      <button
        type="button"
        class="sso-lang-switch"
        @click="localeStore.toggleLang"
      >
        <img :src="iconGlobe" alt="" />
        <span>{{
          localeStore.currentLang === "zh-CN" ? "中文" : "English"
        }}</span>
        <img :src="iconChevronDown" alt="" />
      </button>
      <div class="sso-login-card" :aria-busy="loading || redirecting">
        <h2 class="sso-login-title">{{ t("sso_login_welcome") }}</h2>
        <div class="sso-entry-area">
          <a-spin v-if="loading || redirecting" />
          <button
            v-else-if="providerPresentation"
            type="button"
            class="sso-entry-button"
            :aria-label="t(providerPresentation.label)"
            @click="authorize"
          >
            <img :src="qrcodeBackground" class="sso-qrcode-background" alt="" />
            <span class="sso-entry-content">
              <img
                :src="providerPresentation.icon"
                class="sso-provider-icon"
                alt=""
              />
              <span class="sso-provider-label">{{
                t(providerPresentation.label)
              }}</span>
              <span class="sso-entry-link"
                >{{ t("sso_login_continue") }}<RightOutlined
              /></span>
            </span>
          </button>
        </div>
        <div class="sso-password-area">
          <button
            type="button"
            class="sso-password-button"
            @click="router.push('/login?mode=password')"
          >
            <img :src="iconBackArrow" alt="" />
            <span>{{ t("auth_loginModePassword") }}</span>
          </button>
        </div>
        <p class="sso-login-agreement">
          {{ t("auth_agreementPrefix") }}
          <a
            href="https://www.yuque.com/zhimaxiaoshiwangluo/vgyo1p/ahht4c4sut27tg2v?singleDoc#"
            target="_blank"
            rel="noopener noreferrer"
            >{{ t("auth_serviceAgreement") }}</a
          >
          {{ t("auth_agreementAnd") }}
          <a
            href="https://www.yuque.com/zhimaxiaoshiwangluo/vgyo1p/ph2vngl2oehvwqq4?singleDoc#"
            target="_blank"
            rel="noopener noreferrer"
            >{{ t("auth_privacyPolicy") }}</a
          >
        </p>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { getSsoEntry } from "@/api/sso";
import { useI18n } from "vue-i18n";
import { RightOutlined } from "@ant-design/icons-vue";
import { useLocaleStore } from "@/stores/locale";
import BrandPanel from "@/components/common/BrandPanel.vue";
import iconGlobe from "@/image/login/icon-globe.svg";
import iconChevronDown from "@/image/login/icon-chevron-down.svg";
import iconBackArrow from "@/image/login/icon-back-arrow.svg";
import dingtalkIcon from "@/image/sso/dingding.svg";
import feishuIcon from "@/image/sso/feishu.svg";
import qrcodeBackground from "@/image/sso/qrcode-background.svg";

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const localeStore = useLocaleStore();
const loading = ref(true);
const redirecting = ref(false);
const provider = ref("");
const scopeId =
  typeof route.query.scope_id === "string" ? route.query.scope_id : "";
const presentations = {
  dingtalk: { icon: dingtalkIcon, label: "auth_loginModeDingtalk" },
  feishu: { icon: feishuIcon, label: "auth_loginModeFeishu" },
};
const providerPresentation = computed(() => presentations[provider.value]);
let disposed = false;
onUnmounted(() => {
  disposed = true;
});

function authorize() {
  if (loading.value || redirecting.value || !scopeId) return;
  redirecting.value = true;
  // Keep authorization and tenant validation on the backend; replace prevents a back-button redirect loop.
  window.location.replace(
    `/api/sso/authorize?scope_id=${encodeURIComponent(scopeId)}`,
  );
}

onMounted(async () => {
  if (!scopeId) {
    loading.value = false;
    router.replace("/login");
    return;
  }
  let entry = null;
  try {
    entry = await getSsoEntry(scopeId);
  } catch {
    entry = null;
  }
  if (disposed) return;
  loading.value = false;
  if (!entry || !entry.enabled) {
    // Anonymous entry failures must not disclose configuration details.
    router.replace("/login");
    return;
  }
  provider.value = entry.provider;
  if (!providerPresentation.value) authorize();
});
</script>

<style scoped lang="less">
.sso-login-page {
  display: flex;
  min-height: 100vh;
  background: #f7f8fb;
}

.sso-form-side {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  min-width: 0;
}

.sso-lang-switch {
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
  background: #fff;
  color: #262626;
  font-size: 14px;
  line-height: 22px;
  img {
    width: 16px;
    height: 16px;
  }
  &:hover {
    border-color: #3157e2;
  }
}

.sso-login-card {
  width: 500px;
  height: 640px;
  display: flex;
  flex-direction: column;
  padding: 80px 60px 16px;
  background: #fff;
  border-radius: 24px;
  box-shadow: 0px 4px 32px 0px rgba(0, 0, 0, 0.08);
}

.sso-login-title {
  margin: 0 0 64px;
  font-size: 32px;
  font-weight: 400;
  line-height: 44px;
  color: #000;
  text-align: center;
}

.sso-entry-area {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 224px;
}

.sso-entry-button {
  position: relative;
  width: 224px;
  height: 224px;
  padding: 10px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  cursor: pointer;
  color: #262626;
  &:hover,
  &:focus-visible {
    box-shadow: 0 8px 24px rgba(49, 87, 226, 0.12);
  }
  @media (prefers-reduced-motion: no-preference) {
    transition:
      transform 0.22s ease,
      box-shadow 0.22s ease;
    .sso-entry-link :deep(.anticon) {
      transition: transform 0.22s ease;
    }
    &:hover,
    &:focus-visible {
      transform: translateY(-3px) scale(1.03);
      .sso-entry-link :deep(.anticon) {
        transform: translateX(3px);
      }
    }
  }
  &::before {
    content: "";
    position: absolute;
    inset: 0;
    border-radius: 6px;
    background:
      linear-gradient(#3157e2, #3157e2) left top / 22px 2px,
      linear-gradient(#3157e2, #3157e2) left top / 2px 22px,
      linear-gradient(#3157e2, #3157e2) right top / 22px 2px,
      linear-gradient(#3157e2, #3157e2) right top / 2px 22px,
      linear-gradient(#3157e2, #3157e2) left bottom / 22px 2px,
      linear-gradient(#3157e2, #3157e2) left bottom / 2px 22px,
      linear-gradient(#3157e2, #3157e2) right bottom / 22px 2px,
      linear-gradient(#3157e2, #3157e2) right bottom / 2px 22px;
    background-repeat: no-repeat;
    pointer-events: none;
  }
  &:focus-visible {
    outline: 2px solid #3157e2;
    outline-offset: 5px;
  }
}

.sso-qrcode-background {
  width: 100%;
  height: 100%;
  display: block;
}
.sso-entry-content {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
}
.sso-provider-icon {
  width: 32px;
  height: 32px;
}
.sso-provider-label {
  font-size: 16px;
  font-weight: 600;
  line-height: 24px;
}
.sso-entry-link {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 14px;
  line-height: 22px;
  color: #3157e2;
}

.sso-password-area {
  display: flex;
  justify-content: center;
  margin-top: auto;
  margin-bottom: 24px;
}
.sso-password-button {
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
  img {
    width: 16px;
    height: 16px;
  }
  &:hover {
    color: #3157e2;
    border-color: #3157e2;
  }
}
.sso-login-agreement {
  margin: 0;
  color: #8c8c8c;
  font-size: 14px;
  line-height: 22px;
  text-align: center;
  a {
    text-decoration: none;
    color: #2475fc;
  }
  a:hover {
    color: #3157e2;
  }
}
@media (max-width: 1100px) {
  .sso-login-card {
    width: 440px;
    padding: 48px 48px 40px;
  }
}
@media (max-width: 800px) {
  .sso-form-side {
    padding: 24px;
  }
  .sso-login-card {
    width: 100%;
    max-width: 500px;
    padding: 48px 32px 40px;
  }
  .sso-login-title {
    font-size: 26px;
    margin-bottom: 28px;
  }
}
@media (max-width: 480px) {
  .sso-login-card {
    padding: 40px 24px 32px;
  }
  .sso-login-title {
    font-size: 24px;
    margin-bottom: 24px;
  }
}
</style>

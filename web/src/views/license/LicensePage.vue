<template>
  <div class="license-page">
    <!-- Left brand showcase area (shared with the login page) -->
    <BrandPanel />

    <!-- Right card area -->
    <section class="license-form-side">
      <div class="license-lang-switch" @click="toggleLang">
        <img :src="iconGlobe" class="lang-icon" alt="" />
        <span class="lang-label">{{
          loginLang === "zh-CN" ? "中文" : "English"
        }}</span>
        <img :src="iconChevronDown" class="lang-arrow" alt="" />
      </div>

      <div class="license-card">
        <!-- Loading -->
        <div v-if="isLoading" class="card-center">
          <a-spin size="large" />
        </div>

        <!-- Status fetch failed: retry -->
        <div v-else-if="licenseStore.fetchFailed" class="card-center">
          <p class="state-text">{{ $t("license_status_load_failed") }}</p>
          <a-button
            type="primary"
            class="action-button"
            @click="licenseStore.fetchStatus()"
          >
            {{ $t("license_retry") }}
          </a-button>
        </div>

        <!-- Not activated: free private experience license -->
        <template v-else-if="showTrialForm">
          <h2 class="license-title">{{ $t("license_trial_title") }}</h2>
          <p class="license-subtitle">{{ $t("license_trial_subtitle") }}</p>
          <OnlineActivationForm @success="onActivated" />

          <div class="entry-links">
            <a class="entry-link" @click.prevent="offlineOpen = true">
              {{ $t("license_offline_entry") }}
            </a>
            <a-divider type="vertical" />
            <a class="entry-link" @click.prevent="contactOpen = true">
              {{ $t("license_contact_service") }}
            </a>
          </div>
        </template>

        <!-- Activated but blocked (expired / invalid / mismatch ...) -->
        <template v-else>
          <div class="card-center">
            <CloseCircleFilled class="blocked-icon" />
            <h2 class="license-title">{{ stateTitle }}</h2>
            <p v-if="licenseStore.reason" class="state-text">
              {{ $t(`license_reason_${licenseStore.reason}`) }}
            </p>
            <div class="blocked-info">
              <div v-if="licenseStore.corpName" class="info-row">
                <span class="info-label">{{ $t("license_success_corp") }}</span>
                <span class="info-value">{{ licenseStore.corpName }}</span>
              </div>
              <div v-if="licenseStore.expireAt" class="info-row">
                <span class="info-label">{{
                  $t("license_success_expire")
                }}</span>
                <span class="info-value">{{
                  formatLocalDate(licenseStore.expireAt)
                }}</span>
              </div>
            </div>
            <div class="blocked-actions">
              <a-button
                type="primary"
                class="action-button"
                @click="handleUpdate"
              >
                {{ $t("license_update_entry") }}
              </a-button>
              <a-button class="action-button" @click="contactOpen = true">
                {{ $t("license_contact_service") }}
              </a-button>
            </div>
          </div>
        </template>
      </div>
    </section>

    <!-- Offline activation (paste a full key issued by customer service) -->
    <OfflineActivationModal v-model:open="offlineOpen" @success="onActivated" />
    <ContactServiceModal v-model:open="contactOpen" />
    <LicenseUpdateModal
      v-if="hasToken"
      v-model:open="updateOpen"
      @success="onUpdated"
    />

    <!-- Activation success: shows the first-run admin credentials once -->
    <LicenseSuccessModal
      v-model:open="successOpen"
      :result="successResult"
      @confirm="goLogin"
    />
  </div>
</template>

<script setup>
import { computed, ref } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import BrandPanel from "@/components/common/BrandPanel.vue";
import OnlineActivationForm from "@/components/license/OnlineActivationForm.vue";
import OfflineActivationModal from "@/components/license/OfflineActivationModal.vue";
import LicenseUpdateModal from "@/components/license/LicenseUpdateModal.vue";
import ContactServiceModal from "@/components/license/ContactServiceModal.vue";
import LicenseSuccessModal from "@/components/license/LicenseSuccessModal.vue";
import { useLicenseStore } from "@/stores/license";
import { useAuthStore } from "@/stores/auth";
import { useLocaleStore } from "@/stores/locale";
import { LICENSE_STATE } from "@/constants/license";
import { formatLocalDate } from "@/utils/date";
import iconGlobe from "@/image/login/icon-globe.svg";
import iconChevronDown from "@/image/login/icon-chevron-down.svg";

const router = useRouter();
const { t } = useI18n();
const licenseStore = useLicenseStore();
const authStore = useAuthStore();
const localeStore = useLocaleStore();

licenseStore.fetchStatus();

const loginLang = computed(() => localeStore.currentLang);

function toggleLang() {
  localeStore.toggleLang();
}

const hasToken = computed(() => !!authStore.token);

function handleUpdate() {
  if (hasToken.value) {
    updateOpen.value = true;
    return;
  }
  router.push({ path: "/login", query: { redirect: "/members?tab=version" } });
}

const isLoading = computed(
  () => !licenseStore.status && !licenseStore.fetchFailed,
);

// The trial form only makes sense when no license has ever been installed;
// expired/invalid installs switch to the update view instead.
const showTrialForm = computed(
  () =>
    !licenseStore.state || licenseStore.state === LICENSE_STATE.NOT_ACTIVATED,
);

const stateTitle = computed(() => t(`license_state_${licenseStore.state}`));

// ---- Activation results ----
const offlineOpen = ref(false);
const contactOpen = ref(false);
const updateOpen = ref(false);
const successOpen = ref(false);
const successResult = ref(null);

async function onActivated(result) {
  offlineOpen.value = false;
  successResult.value = result || (await licenseStore.refreshAfterInstall());
  successOpen.value = true;
}

async function onUpdated() {
  updateOpen.value = false;
  await licenseStore.refreshAfterInstall();
}

function goLogin() {
  successOpen.value = false;
  router.push("/login");
}
</script>

<style scoped lang="less">
.license-page {
  display: flex;
  min-height: 100vh;
  background: #f7f8fb;
}

.license-form-side {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  min-width: 0;
  padding: 24px;
}

.license-lang-switch {
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

.license-card {
  width: 500px;
  padding: 60px 60px 48px;
  background: #fff;
  border-radius: 24px;
  box-shadow: 0px 4px 32px 0px rgba(0, 0, 0, 0.08);
}

.license-title {
  margin: 0 0 8px;
  font-size: 28px;
  font-weight: 400;
  line-height: 40px;
  color: #000;
  text-align: center;
}

.license-subtitle {
  margin: 0 0 28px;
  font-size: 14px;
  color: #667085;
  line-height: 22px;
  text-align: center;
}

.card-center {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 24px 0;
}

.blocked-icon {
  margin-bottom: 16px;
  font-size: 56px;
  color: #faad14;
}

.state-text {
  margin: 0 0 16px;
  font-size: 14px;
  line-height: 22px;
  color: #667085;
  text-align: center;
}

.blocked-info {
  width: 100%;
  margin-bottom: 24px;
  padding: 12px 16px;
  background: #f7f8fb;
  border-radius: 8px;
}

.info-row {
  display: flex;
  padding: 3px 0;
  font-size: 14px;
  line-height: 22px;
}

.info-label {
  flex-shrink: 0;
  width: 96px;
  color: #667085;
}

.info-value {
  color: #1f2937;
  word-break: break-all;
}

.blocked-actions {
  display: flex;
  gap: 12px;
  width: 100%;
}

.action-button {
  flex: 1;
  height: 44px;
  border-radius: 6px;
  font-size: 16px;

  &.ant-btn-primary {
    background: #3157e2;
    border-color: #3157e2;

    &:hover,
    &:focus {
      background: #2548c4;
      border-color: #2548c4;
    }
  }
}

.entry-links {
  margin-top: 24px;
  text-align: center;
}

.entry-link {
  color: #2475fc;
  font-size: 14px;
  cursor: pointer;

  &:hover {
    color: #3157e2;
  }
}

@media (max-width: 1100px) {
  .license-card {
    width: 440px;
    padding: 48px 48px 40px;
  }
}

@media (max-width: 800px) {
  .license-card {
    width: 100%;
    max-width: 500px;
    padding: 48px 32px 40px;
  }
}

@media (max-width: 480px) {
  .license-card {
    padding: 40px 24px 32px;
  }
}
</style>

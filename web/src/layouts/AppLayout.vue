<template>
  <a-layout class="workspace-layout">
    <Sidebar />
    <a-layout style="margin-left: 220px">
      <a-layout-content class="workspace-content">
        <ServiceExpiredPage v-if="showServiceExpiredPage" />
        <CapacityOverLimitPage v-else-if="authStore.capacityBlocked" />
        <slot v-else>
          <router-view v-slot="{ Component }">
            <KeepAlive :include="cachedViews">
              <component :is="Component" />
            </KeepAlive>
          </router-view>
        </slot>
      </a-layout-content>
    </a-layout>
  </a-layout>
</template>

<script setup>
import { computed, onMounted, onUnmounted, watch } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useLicenseStore } from "@/stores/license";
import { LICENSE_ROUTE_PATH } from "@/constants/license";
import ServiceExpiredPage from "@/views/service-expired/ServiceExpiredPage.vue";
import CapacityOverLimitPage from "@/views/storage-over-limit/CapacityOverLimitPage.vue";
import Sidebar from "./Sidebar.vue";

const authStore = useAuthStore();
const licenseStore = useLicenseStore();
const router = useRouter();

const cachedViews = ["MyWorkPage", "TeamWorkPage", "AgentPage"];

// 开源版的套餐到期字段是授权 claims 的投影，受限界面由授权门禁接管（固定停在企业设置版本页）。
const showServiceExpiredPage = computed(
  () => authStore.isEnterpriseExpired && !licenseStore.isCommercial,
);

onMounted(() => {
  // 确保企业版到期状态已加载（刷新后 initializeSession 已调用，此处作为兜底）
  if (!authStore.enterprisePlan) {
    authStore.fetchEnterprisePlanInfo();
  }
  // 开源版授权 60 秒轮询（仅页面可见时真正请求，卸载时必须清理）
  licenseStore.startPolling();
});

onUnmounted(() => {
  licenseStore.stopPolling();
});

// 轮询发现授权失效时进入受限续期页；未激活状态仍退到公共授权页。
watch(
  () => licenseStore.businessEnabled,
  (businessEnabled) => {
    if (
      businessEnabled ||
      licenseStore.fetchFailed ||
      !licenseStore.isCommercial
    )
      return;
    if (licenseStore.managementOnly) {
      if (
        router.currentRoute.value.path !== "/members" ||
        router.currentRoute.value.query.tab !== "version"
      ) {
        router.push({ path: "/members", query: { tab: "version" } });
      }
      return;
    }
    if (router.currentRoute.value.path === LICENSE_ROUTE_PATH) return;
    authStore.clearSession();
    router.push(LICENSE_ROUTE_PATH);
  },
);
</script>

<style scoped lang="less">
.workspace-layout {
  min-height: 100vh;
}

.workspace-content {
  padding: 10px 24px 10px 24px;
  height: 100vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  color: var(--app-text);
  background-color: var(--app-bg);
}
</style>

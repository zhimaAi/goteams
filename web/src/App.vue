<template>
  <a-config-provider :locale="antdLocale" :theme="themeConfig">
    <router-view />
    <QuotaExceededModal />
    <CapacityOverLimitModal />
  </a-config-provider>
</template>

<script setup>
import { computed } from "vue";
import { theme } from "ant-design-vue";
import zhCN from "ant-design-vue/es/locale/zh_CN";
import enUS from "ant-design-vue/es/locale/en_US";
import { useLocaleStore } from "@/stores/locale";
import { usePageTitle } from "@/composables/usePageTitle";
import QuotaExceededModal from "@/components/common/QuotaExceededModal.vue";
import CapacityOverLimitModal from "@/components/common/CapacityOverLimitModal.vue";

const localeStore = useLocaleStore();
const antdLocale = computed(() =>
  localeStore.currentLang === "zh-CN" ? zhCN : enUS,
);
// Initialize the global tab-title watcher (route / language / workspace).
usePageTitle();
const themeConfig = {
  algorithm: theme.defaultAlgorithm,
  token: { borderRadius: 6, colorPrimary: "#3157E2" },
};
</script>

<style>
/* Global base styles. */
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

html,
body,
#app {
  height: 100%;
  font-family:
    -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue",
    Arial, "Noto Sans", sans-serif, "Apple Color Emoji", "Segoe UI Emoji";
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

:root {
  color-scheme: light;
  --app-bg: #fff;
  --app-surface: #fff;
  --app-surface-muted: #fafafa;
  --app-border: #f0f0f0;
  --app-text: #182b50;
  --app-text-secondary: #8c95a8;
}

body {
  color: var(--app-text);
  background: var(--app-bg);
}

::-webkit-scrollbar {
  width: 6px;
  height: 6px;
}

::-webkit-scrollbar-thumb {
  background: var(--app-border);
  border-radius: 3px;
}
</style>

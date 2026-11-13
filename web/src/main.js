import { installSaveHotkeyInterceptor } from "./utils/saveHotkey";
import { createApp } from "vue";
import { createPinia } from "pinia";
import piniaPluginPersistedstate from "pinia-plugin-persistedstate";
import App from "./App.vue";
import router from "./router";
import "./router/permission"; // Register the route guard.
import { setupI18n } from "./locales";
import { useBrandingStore } from "./stores/branding";

// Ant Design Vue
import Antd from "ant-design-vue";
import "ant-design-vue/dist/reset.css";
// Shared markdown document-reference pill, used by conversations, logs and document bodies.
import "./styles/markdown-doc-reference.less";

installSaveHotkeyInterceptor();

async function bootstrap() {
  const app = createApp(App);

  // Pinia state management + persistence.
  const pinia = createPinia();
  pinia.use(piniaPluginPersistedstate);
  app.use(pinia);

  // Resolve the dedicated host before the first route guard runs.
  const brandingStore = useBrandingStore();
  await brandingStore.loadPublic();

  // Routes.
  app.use(router);

  // Internationalization (asynchronously load language packs).
  await setupI18n(app);

  // UI framework.
  app.use(Antd);

  // Mount.
  app.mount("#app");
}

bootstrap();

<template>
  <!-- Left brand showcase area (shared by login / invite pages) -->
  <section class="login-showcase">
    <div class="showcase-gradient"></div>
    <img :src="loginBg" class="showcase-bg" alt="" />

    <!-- Logo -->
    <div class="showcase-logo">
      <img :src="branding?.logo_url || logoIcon" class="logo-icon" alt="" />
      <span class="logo-text">{{ branding?.app_name || "GoTeams" }}</span>
    </div>

    <!-- Copy area -->
    <div
      v-if="branding?.login_slogan"
      class="showcase-body showcase-slogan"
      v-html="safeSlogan"
    ></div>
    <div v-else class="showcase-body">
      <h1 class="showcase-title">{{ lt("auth_loginMainTitle") }}</h1>
      <p class="showcase-subtitle">{{ lt("auth_loginSubtitle") }}</p>
      <p class="showcase-description">{{ lt("auth_loginDescription") }}</p>
    </div>

    <!-- Footer -->
    <p class="showcase-footer">{{ lt("auth_loginFooter") }}</p>
  </section>
</template>

<script setup>
import { computed } from "vue";
import DOMPurify from "dompurify";
import { useLocaleStore } from "@/stores/locale";
import zhCNMessages from "@/locales/zh-CN";
import enUSMessages from "@/locales/en-US";
import loginBg from "@/image/login/login-bg.png";
import logoIcon from "@/image/logo.svg";

const props = defineProps({ branding: { type: Object, default: null } });
const safeSlogan = computed(() =>
  DOMPurify.sanitize(props.branding?.login_slogan || "", {
    ALLOWED_TAGS: [
      "p",
      "br",
      "h1",
      "h2",
      "h3",
      "h4",
      "h5",
      "h6",
      "strong",
      "b",
      "em",
      "i",
      "u",
      "s",
      "span",
      "ul",
      "ol",
      "li",
      "blockquote",
    ],
    ALLOWED_ATTR: ["style"],
  }),
);

const MESSAGES = { "zh-CN": zhCNMessages, "en-US": enUSMessages };
const localeStore = useLocaleStore();
const lang = computed(() => localeStore.currentLang);

function lt(key) {
  return MESSAGES[lang.value]?.[key] || MESSAGES["zh-CN"][key] || key;
}
</script>

<style scoped lang="less">
.showcase-slogan {
  overflow-wrap: anywhere;
  :deep(p) {
    margin: 0 0 16px;
  }
}
.login-showcase {
  position: relative;
  width: 538px;
  min-height: 100vh;
  flex-shrink: 0;
  overflow: hidden;
  color: #fff;
  display: flex;
  flex-direction: column;
}

.showcase-bg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  z-index: 0;
}

.showcase-gradient {
  position: absolute;
  inset: 0;
  z-index: 1;
  background: linear-gradient(
    180deg,
    rgba(1, 9, 38, 0) 20%,
    rgba(1, 9, 38, 1) 100%
  );
  backdrop-filter: blur(2px);
}

.showcase-logo {
  position: absolute;
  top: 18px;
  left: 60px;
  z-index: 2;
  display: flex;
  align-items: center;
  gap: 8px;
}

.logo-icon {
  width: 28px;
  height: 28px;
  object-fit: contain;
  flex-shrink: 0;
}

.logo-text {
  overflow-wrap: anywhere;
  font-family: Inter, sans-serif;
  font-size: 16px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.85);
  letter-spacing: -0.0195em;
}

.showcase-body {
  position: relative;
  z-index: 2;
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: 60px;
  padding-top: 108px;
}

.showcase-title {
  margin: 0;
  font-size: 36px;
  font-weight: 600;
  line-height: 48px;
  letter-spacing: -0.0222em;
  color: #fff;
}

.showcase-subtitle {
  margin: 20px 0 0;
  font-size: 20px;
  font-weight: 600;
  line-height: 28px;
  color: #adbfff;
}

.showcase-description {
  margin: 40px 0 0;
  font-size: 16px;
  font-weight: 400;
  line-height: 28px;
  color: #e4e6eb;
}

.showcase-footer {
  position: relative;
  z-index: 2;
  margin: 0;
  padding: 0 60px 28px;
  font-size: 14px;
  font-weight: 400;
  letter-spacing: 0.08em;
  color: #7a8699;
}

// Responsive (kept consistent with the login page)
@media (max-width: 1100px) {
  .login-showcase {
    width: 380px;
  }

  .showcase-body {
    padding: 48px;
  }

  .showcase-title {
    font-size: 28px;
    line-height: 38px;
  }

  .showcase-subtitle {
    font-size: 18px;
  }
}

@media (max-width: 800px) {
  .login-showcase {
    display: none;
  }
}
</style>

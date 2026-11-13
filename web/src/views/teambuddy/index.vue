<template>
  <div class="teambuddy-page" :class="{ 'is-english': locale === 'en-US' }">
    <svg xmlns="http://www.w3.org/2000/svg" class="svg-defs" aria-hidden="true">
      <defs>
        <symbol id="tb-i-arrow" viewBox="0 0 24 24">
          <path d="M5 12h14M13 6l6 6-6 6" />
        </symbol>
        <symbol id="tb-i-down" viewBox="0 0 24 24">
          <path d="M12 3v12m-5-5 5 5 5-5M5 16v5h14v-5" />
        </symbol>

        <symbol id="tb-i-check" viewBox="0 0 24 24">
          <path d="m5 12 4 4L19 6" />
        </symbol>
        <symbol id="tb-i-code" viewBox="0 0 24 24">
          <path d="m8 6-6 6 6 6m8-12 6 6-6 6M14 3l-4 18" />
        </symbol>
        <symbol id="tb-i-folder" viewBox="0 0 24 24">
          <path d="M3 7V4h6l3 3h9v13H3z" />
        </symbol>
        <symbol id="tb-i-shield" viewBox="0 0 24 24">
          <path d="M12 2 3 6v6c0 5 9 10 9 10s9-5 9-10V6z" />
          <path d="m8 12 3 3 5-6" />
        </symbol>
      </defs>
    </svg>
    <a class="skip-link" href="#main-content">{{ $t("teambuddy_skip") }}</a>
    <header ref="siteHeader" class="site-header">
      <div class="header-inner section-wrap">
        <a class="brand" href="#top" :aria-label="$t('teambuddy_footer_home')"
          ><span class="brand-mark" aria-hidden="true"
            ><img
              src="@/image/teambuddy/teambuddy-logo.svg"
              width="32"
              height="32"
              alt="" /></span
          ><span>{{ $t("teambuddy_brand") }}</span></a
        >
        <nav :aria-label="$t('teambuddy_nav_label')">
          <a
            v-for="section in navigation"
            :key="section.id"
            :href="`#${section.id}`"
            :aria-current="
              activeSection === section.id ? 'location' : undefined
            "
            >{{ $t(section.labelKey) }}</a
          >
        </nav>
        <div class="header-actions">
          <div
            class="language"
            role="group"
            :aria-label="$t('teambuddy_language_label')"
          >
            <button
              class="language-button"
              type="button"
              lang="zh-CN"
              :aria-pressed="locale === 'zh-CN'"
              @click="localeStore.setLang('zh-CN')"
            >
              {{ $t("teambuddy_language_chinese") }}
            </button>
            <span class="language-divider" aria-hidden="true" />
            <button
              class="language-button"
              type="button"
              lang="en"
              aria-label="English"
              :aria-pressed="locale === 'en-US'"
              @click="localeStore.setLang('en-US')"
            >
              {{ $t("teambuddy_language_english") }}
            </button>
          </div>
          <a class="button button-primary button-header" href="#download"
            >{{ $t("teambuddy_nav_download")
            }}<svg class="icon" aria-hidden="true">
              <use href="#tb-i-down" /></svg
          ></a>
        </div>
      </div>
    </header>
    <main id="main-content">
      <section class="hero" id="top" aria-labelledby="hero-title">
        <div class="hero-grid section-wrap">
          <div class="hero-copy">
            <p
              v-motion
              :initial="motionInitial.rise"
              :enter="motionVisible(0)"
              class="eyebrow hero-eyebrow motion-reveal"
            >
              <span class="status-dot blue"></span
              >{{ $t("teambuddy_hero_badge") }}
            </p>
            <h1
              v-motion
              :initial="motionInitial.left"
              :enter="motionVisible(100)"
              class="motion-reveal"
              id="hero-title"
            >
              <span class="hero-firstline">{{
                $t("teambuddy_hero_intro")
              }}</span
              ><span class="hero-lastline">{{
                $t("teambuddy_hero_title")
              }}</span>
            </h1>
            <p
              v-motion
              :initial="motionInitial.rise"
              :enter="motionVisible(220)"
              class="hero-description motion-reveal"
            >
              {{ $t("teambuddy_hero_description")
              }}<span class="hero-term">{{ $t("teambuddy_hero_term") }}</span>
            </p>
            <div
              v-motion
              :initial="motionInitial.pop"
              :enter="motionVisible(360, true)"
              class="hero-actions motion-reveal"
            >
              <a class="button button-primary" href="#download"
                >{{ $t("teambuddy_hero_start")
                }}<svg class="icon" aria-hidden="true">
                  <use href="#tb-i-arrow"></use></svg></a
              ><span class="pending-wrap"
                ><a
                  class="button button-secondary"
                  href="https://github.com/zhimaAi/TeamBuddy"
                  target="_blank"
                  rel="noopener noreferrer"
                  ><svg class="icon" aria-hidden="true">
                    <use href="#tb-i-code" /></svg
                  >{{ $t("teambuddy_hero_source") }}</a
                ></span
              >
            </div>
          </div>
          <div
            v-motion
            :initial="motionInitial.scene"
            :enter="motionVisible(220)"
            class="hero-scene motion-reveal"
          >
            <div
              class="hero-visual"
              role="img"
              :aria-label="$t('teambuddy_hero_art')"
            ></div>
          </div>
        </div>
      </section>

      <section class="trust-section section-wrap" aria-labelledby="trust-title">
        <h2 id="trust-title" class="sr-only">{{ $t("teambuddy_brand") }}</h2>
        <div class="trust-grid">
          <article
            v-motion
            :initial="motionInitial.pop"
            :visible-once="motionVisible(0, true)"
            class="motion-reveal"
          >
            <svg class="icon" aria-hidden="true">
              <use href="#tb-i-code"></use>
            </svg>
            <h3>{{ $t("teambuddy_trust_source_title") }}</h3>
            <p>{{ $t("teambuddy_trust_source_description") }}</p>
          </article>
          <article
            v-motion
            :initial="motionInitial.pop"
            :visible-once="motionVisible(120, true)"
            class="motion-reveal"
          >
            <svg class="icon" aria-hidden="true">
              <use href="#tb-i-folder"></use>
            </svg>
            <h3>{{ $t("teambuddy_trust_local_title") }}</h3>
            <p>{{ $t("teambuddy_trust_local_description") }}</p>
          </article>
          <article
            v-motion
            :initial="motionInitial.pop"
            :visible-once="motionVisible(240, true)"
            class="motion-reveal"
          >
            <svg class="icon" aria-hidden="true">
              <use href="#tb-i-shield"></use>
            </svg>
            <h3>{{ $t("teambuddy_trust_private_title") }}</h3>
            <p>{{ $t("teambuddy_trust_private_description") }}</p>
          </article>
        </div>
      </section>

      <section
        class="showcase-section"
        id="showcase"
        aria-labelledby="showcase-title"
      >
        <div class="section-wrap">
          <div
            v-motion
            :initial="motionInitial.rise"
            :visible-once="motionVisible(0)"
            class="section-heading showcase-heading motion-reveal"
          >
            <h2 id="showcase-title">{{ $t("teambuddy_showcase_title") }}</h2>
          </div>
          <div
            v-motion
            :initial="motionInitial.fade"
            :visible-once="motionVisible(100)"
            ref="showcaseTabs"
            class="showcase-tabs motion-reveal"
            role="tablist"
            :aria-label="$t('teambuddy_showcase_label')"
          >
            <button
              v-for="(slide, index) in slides"
              :key="slide.id"
              type="button"
              role="tab"
              :id="`tb-tab-${index}`"
              :aria-controls="`tb-panel-${index}`"
              :aria-selected="activeSlide === index"
              :tabindex="activeSlide === index ? 0 : -1"
              @mouseenter="activeSlide = index"
              @click="activeSlide = index"
              @keydown="handleTabKey($event, index)"
            >
              {{ $t(slide.titleKey) }}
            </button>
            <span
              class="showcase-tab-indicator"
              :style="showcaseIndicatorStyle"
              aria-hidden="true"
            />
          </div>
          <div
            v-motion
            :initial="motionInitial.zoom"
            :visible-once="motionVisible(180)"
            class="showcase-gallery motion-reveal"
          >
            <Transition
              v-for="(slide, index) in slides"
              :key="slide.id"
              name="showcase"
            >
              <section
                v-show="activeSlide === index"
                :aria-hidden="activeSlide !== index"
                :inert="activeSlide !== index"
                class="showcase-panel"
                role="tabpanel"
                :id="`tb-panel-${index}`"
                :aria-labelledby="`tb-tab-${index}`"
                tabindex="0"
              >
                <img
                  class="showcase-screenshot"
                  :src="slide.image"
                  :alt="$t(slide.altKey)"
                  width="2880"
                  height="1800"
                  loading="lazy"
                  decoding="async"
                />
              </section>
            </Transition>
          </div>
        </div>
      </section>

      <section
        class="features-section section-wrap"
        id="features"
        aria-labelledby="features-title"
      >
        <div
          v-motion
          :initial="motionInitial.rise"
          :visible-once="motionVisible(0)"
          class="section-heading features-heading motion-reveal"
        >
          <div>
            <h2 id="features-title">{{ $t("teambuddy_features_title") }}</h2>
            <p>{{ $t("teambuddy_features_description") }}</p>
          </div>
        </div>
        <div class="feature-pair">
          <article class="feature-card" id="feature-board">
            <div
              v-motion
              :initial="motionInitial.left"
              :visible-once="motionVisible(0)"
              class="feature-copy motion-reveal"
            >
              <p class="eyebrow">
                <span class="eyebrow-line"></span
                >{{ $t("teambuddy_board_label") }}
              </p>
              <h3>{{ $t("teambuddy_board_title") }}</h3>
              <p class="feature-description">
                {{ $t("teambuddy_board_description") }}
              </p>
              <ul class="feature-bullets">
                <li>
                  <svg class="icon" aria-hidden="true">
                    <use href="#tb-i-check"></use></svg
                  ><span>{{ $t("teambuddy_board_bullet_personal") }}</span>
                </li>
                <li>
                  <svg class="icon" aria-hidden="true">
                    <use href="#tb-i-check"></use></svg
                  ><span>{{ $t("teambuddy_board_bullet_status") }}</span>
                </li>
                <li>
                  <svg class="icon" aria-hidden="true">
                    <use href="#tb-i-check"></use></svg
                  ><span>{{ $t("teambuddy_board_bullet_create") }}</span>
                </li>
              </ul>
            </div>
            <div
              v-motion
              :initial="motionInitial.right"
              :visible-once="motionVisible(140)"
              class="feature-visual feature-image-visual motion-reveal"
            >
              <img
                loading="lazy"
                decoding="async"
                class="feature-artwork"
                src="@/image/teambuddy/features/board-artwork-20261102.png"
                :alt="$t('teambuddy_board_art')"
                width="1200"
                height="1022"
              />
            </div>
          </article>
          <article class="feature-card" id="feature-chat">
            <div
              v-motion
              :initial="motionInitial.right"
              :visible-once="motionVisible(0)"
              class="feature-copy motion-reveal"
            >
              <p class="eyebrow">
                <span class="eyebrow-line"></span
                >{{ $t("teambuddy_chat_label") }}
              </p>
              <h3>{{ $t("teambuddy_chat_title") }}</h3>
              <p class="feature-description">
                {{ $t("teambuddy_chat_description") }}
              </p>
              <ul class="feature-bullets">
                <li>
                  <svg class="icon" aria-hidden="true">
                    <use href="#tb-i-check"></use></svg
                  ><span>{{ $t("teambuddy_chat_bullet_approval") }}</span>
                </li>
                <li>
                  <svg class="icon" aria-hidden="true">
                    <use href="#tb-i-check"></use></svg
                  ><span>{{ $t("teambuddy_chat_bullet_history") }}</span>
                </li>
                <li>
                  <svg class="icon" aria-hidden="true">
                    <use href="#tb-i-check"></use></svg
                  ><span>{{ $t("teambuddy_chat_bullet_messages") }}</span>
                </li>
              </ul>
            </div>
            <div
              v-motion
              :initial="motionInitial.left"
              :visible-once="motionVisible(140)"
              class="feature-visual feature-image-visual motion-reveal"
            >
              <img
                loading="lazy"
                decoding="async"
                class="feature-artwork"
                src="@/image/teambuddy/features/conversation-artwork-v2-20261102.png"
                :alt="$t('teambuddy_chat_art')"
                width="1200"
                height="1020"
              />
            </div>
          </article>
        </div>
      </section>

      <section
        class="wide-feature mobile-section"
        id="feature-mobile"
        aria-labelledby="mobile-title"
      >
        <div class="section-wrap wide-feature-grid">
          <div
            v-motion
            :initial="motionInitial.left"
            :visible-once="motionVisible(0)"
            class="feature-copy motion-reveal"
          >
            <p class="eyebrow">
              <span class="eyebrow-line"></span
              >{{ $t("teambuddy_mobile_label") }}
            </p>
            <h3 id="mobile-title">{{ $t("teambuddy_mobile_title") }}</h3>
            <p class="feature-description">
              {{ $t("teambuddy_mobile_description") }}
            </p>
            <ul class="feature-bullets">
              <li>
                <svg class="icon" aria-hidden="true">
                  <use href="#tb-i-check"></use></svg
                ><span>{{ $t("teambuddy_mobile_bullet_remote") }}</span>
              </li>
              <li>
                <svg class="icon" aria-hidden="true">
                  <use href="#tb-i-check"></use></svg
                ><span>{{ $t("teambuddy_mobile_bullet_channels") }}</span>
              </li>
              <li>
                <svg class="icon" aria-hidden="true">
                  <use href="#tb-i-check"></use></svg
                ><span>{{ $t("teambuddy_mobile_bullet_quick") }}</span>
              </li>
            </ul>
            <div class="channel-pills">
              <span>{{ $t("teambuddy_channel_wechat") }}</span
              ><span>{{ $t("teambuddy_channel_feishu") }}</span
              ><span>{{ $t("teambuddy_channel_dingtalk") }}</span
              ><span>{{ $t("teambuddy_channel_wecom") }}</span>
            </div>
          </div>
          <div
            v-motion
            :initial="motionInitial.pop"
            :visible-once="motionVisible(140, true)"
            class="mobile-visual feature-image-visual motion-reveal"
          >
            <img
              loading="lazy"
              decoding="async"
              class="feature-artwork"
              src="@/image/teambuddy/features/mobile-artwork-v3-20261102.png"
              :alt="$t('teambuddy_mobile_art')"
              width="1200"
              height="1020"
            />
          </div>
        </div>
      </section>

      <section
        class="wide-feature pipeline-section section-wrap"
        id="feature-pipeline"
      >
        <div class="wide-feature-grid">
          <div
            v-motion
            :initial="motionInitial.right"
            :visible-once="motionVisible(0)"
            class="feature-copy motion-reveal"
          >
            <p class="eyebrow">
              <span class="eyebrow-line"></span
              >{{ $t("teambuddy_pipeline_label") }}
            </p>
            <h3>{{ $t("teambuddy_pipeline_title") }}</h3>
            <p class="feature-description">
              {{ $t("teambuddy_pipeline_description") }}
            </p>
            <ul class="feature-bullets">
              <li>
                <svg class="icon" aria-hidden="true">
                  <use href="#tb-i-check"></use></svg
                ><span>{{ $t("teambuddy_pipeline_bullet_edit") }}</span>
              </li>
              <li>
                <svg class="icon" aria-hidden="true">
                  <use href="#tb-i-check"></use></svg
                ><span>{{ $t("teambuddy_pipeline_bullet_team") }}</span>
              </li>
              <li>
                <svg class="icon" aria-hidden="true">
                  <use href="#tb-i-check"></use></svg
                ><span>{{ $t("teambuddy_pipeline_bullet_config") }}</span>
              </li>
            </ul>
          </div>
          <div
            v-motion
            :initial="motionInitial.left"
            :visible-once="motionVisible(140)"
            class="pipeline-visual feature-image-visual motion-reveal"
          >
            <img
              loading="lazy"
              decoding="async"
              class="feature-artwork"
              src="@/image/teambuddy/features/pipeline-artwork-v2-20261102.png"
              :alt="$t('teambuddy_pipeline_art')"
              width="1200"
              height="1020"
            />
          </div>
        </div>
      </section>

      <section class="section-wrap last-features">
        <div class="feature-pair">
          <article class="feature-card" id="feature-project">
            <div
              v-motion
              :initial="motionInitial.left"
              :visible-once="motionVisible(0)"
              class="feature-copy motion-reveal"
            >
              <p class="eyebrow">
                <span class="eyebrow-line"></span
                >{{ $t("teambuddy_project_label") }}
              </p>
              <h3>{{ $t("teambuddy_project_title") }}</h3>
              <p class="feature-description">
                {{ $t("teambuddy_project_description") }}
              </p>
              <ul class="feature-bullets">
                <li>
                  <svg class="icon" aria-hidden="true">
                    <use href="#tb-i-check"></use></svg
                  ><span>{{ $t("teambuddy_project_bullet_directory") }}</span>
                </li>
                <li>
                  <svg class="icon" aria-hidden="true">
                    <use href="#tb-i-check"></use></svg
                  ><span>{{ $t("teambuddy_project_bullet_git") }}</span>
                </li>
                <li>
                  <svg class="icon" aria-hidden="true">
                    <use href="#tb-i-check"></use></svg
                  ><span>{{ $t("teambuddy_project_bullet_chips") }}</span>
                </li>
              </ul>
            </div>
            <div
              v-motion
              :initial="motionInitial.zoom"
              :visible-once="motionVisible(140)"
              class="feature-visual feature-image-visual motion-reveal"
            >
              <img
                loading="lazy"
                decoding="async"
                class="feature-artwork"
                src="@/image/teambuddy/features/project-artwork-20261102.png"
                :alt="$t('teambuddy_project_art')"
                width="1200"
                height="1020"
              />
            </div>
          </article>
          <article class="feature-card" id="feature-resource">
            <div
              v-motion
              :initial="motionInitial.right"
              :visible-once="motionVisible(0)"
              class="feature-copy motion-reveal"
            >
              <p class="eyebrow">
                <span class="eyebrow-line"></span
                >{{ $t("teambuddy_resource_label") }}
              </p>
              <h3>{{ $t("teambuddy_resource_title") }}</h3>
              <p class="feature-description">
                {{ $t("teambuddy_resource_description") }}
              </p>
              <ul class="feature-bullets">
                <li>
                  <svg class="icon" aria-hidden="true">
                    <use href="#tb-i-check"></use></svg
                  ><span>{{ $t("teambuddy_resource_bullet_commands") }}</span>
                </li>
                <li>
                  <svg class="icon" aria-hidden="true">
                    <use href="#tb-i-check"></use></svg
                  ><span>{{ $t("teambuddy_resource_bullet_knowledge") }}</span>
                </li>
                <li>
                  <svg class="icon" aria-hidden="true">
                    <use href="#tb-i-check"></use></svg
                  ><span>{{ $t("teambuddy_resource_bullet_settings") }}</span>
                </li>
              </ul>
            </div>
            <div
              v-motion
              :initial="motionInitial.left"
              :visible-once="motionVisible(140)"
              class="feature-visual feature-image-visual motion-reveal"
            >
              <img
                loading="lazy"
                decoding="async"
                class="feature-artwork feature-artwork-resource"
                src="@/image/teambuddy/features/resource-artwork-icons-shadow-v4-20261102.png"
                :alt="$t('teambuddy_resource_art')"
                width="1360"
                height="1156"
              />
            </div>
          </article>
        </div>
      </section>

      <section
        class="download-section"
        id="download"
        aria-labelledby="download-title"
      >
        <div class="section-wrap">
          <div class="download-hero">
            <div
              v-motion
              :initial="motionInitial.left"
              :visible-once="motionVisible(0)"
              class="download-copy motion-reveal"
            >
              <p class="eyebrow">{{ $t("teambuddy_brand") }}</p>
              <h2 id="download-title">{{ $t("teambuddy_download_title") }}</h2>
              <p>
                {{ $t("teambuddy_hero_description")
                }}<span class="download-term">{{
                  $t("teambuddy_hero_term")
                }}</span>
              </p>
            </div>
            <figure
              v-motion
              :initial="motionInitial.zoom"
              :visible-once="motionVisible(160)"
              class="download-art motion-reveal"
            >
              <img
                loading="lazy"
                decoding="async"
                src="@/image/teambuddy/download-devices-chat-v2-20261102.png"
                :alt="$t('teambuddy_download_art')"
                width="1536"
                height="1024"
              />
            </figure>
          </div>
          <div class="download-options" :aria-busy="loadingRelease">
            <article
              v-for="(item, index) in downloads"
              :key="item.id"
              v-motion
              :initial="motionInitial.pop"
              :visible-once="motionVisible(index * 110, true)"
              class="download-option motion-reveal"
            >
              <div class="platform-symbol">
                <img
                  :class="['platform-icon', `platform-icon-${item.platform}`]"
                  :src="item.platform === 'windows' ? windowsIcon : macosIcon"
                  width="52"
                  height="52"
                  alt=""
                  loading="lazy"
                />
              </div>
              <h3>{{ $t(item.titleKey) }}</h3>
              <p>{{ $t(item.detailKey) }}</p>
              <a v-if="item.url" class="button" :href="item.url"
                ><svg class="icon" aria-hidden="true">
                  <use href="#tb-i-down" /></svg
                >{{ $t(item.labelKey, { version: releaseVersion }) }}</a
              >
              <button v-else class="button" type="button" disabled>
                {{
                  $t(
                    loadingRelease
                      ? "teambuddy_download_loading"
                      : "teambuddy_download_unavailable",
                  )
                }}
              </button>
            </article>
          </div>
        </div>
      </section>
    </main>
    <footer
      v-motion
      :initial="motionInitial.fade"
      :visible-once="motionVisible()"
      class="site-footer section-wrap motion-reveal"
    >
      <a class="brand" href="#top" :aria-label="$t('teambuddy_footer_home')"
        ><span class="brand-mark" aria-hidden="true"
          ><img
            loading="lazy"
            decoding="async"
            src="@/image/teambuddy/teambuddy-logo.svg"
            width="32"
            height="32"
            alt="" /></span
        ><span>{{ $t("teambuddy_brand") }}</span></a
      >
      <p>{{ $t("teambuddy_footer_description") }}</p>
      <a href="#top" class="back-top"
        >{{ $t("teambuddy_footer_top")
        }}<svg class="icon" aria-hidden="true">
          <use href="#tb-i-arrow"></use></svg
      ></a>
    </footer>
  </div>
</template>
<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import {
  useEventListener,
  usePreferredReducedMotion,
  useResizeObserver,
} from "@vueuse/core";
import { useLocaleStore } from "@/stores/locale";
import { getLatestTeamBuddyRelease } from "@/api/teambuddy";
import windowsIcon from "@/image/teambuddy/icons/windows-user-20261102.svg";
import macosIcon from "@/image/teambuddy/icons/macos-user-20261102.svg";
import boardImage from "@/image/teambuddy/showcase/board.png";
import remoteImage from "@/image/teambuddy/showcase/remote.png";
import conversationImage from "@/image/teambuddy/showcase/conversation-latest-20261102.png";
import executionImage from "@/image/teambuddy/showcase/execution.png";
import expertsImage from "@/image/teambuddy/showcase/experts-20261102.png";
const { t: $t, locale } = useI18n();
const localeStore = useLocaleStore();
const siteHeader = ref(null);
const activeSection = ref("");
const navigation = [
  { id: "showcase", labelKey: "teambuddy_nav_showcase" },
  { id: "features", labelKey: "teambuddy_nav_features" },
  { id: "download", labelKey: "teambuddy_nav_download" },
];

function updateActiveSection() {
  if (!siteHeader.value) return;
  const offset = siteHeader.value.getBoundingClientRect().height + 24;
  let active = "";
  for (const section of navigation) {
    const element = document.getElementById(section.id);
    if (element && element.getBoundingClientRect().top <= offset)
      active = section.id;
  }
  if (
    window.scrollY > 0 &&
    window.scrollY + window.innerHeight >=
      document.documentElement.scrollHeight - 4
  )
    active = "download";
  activeSection.value = active;
}

useEventListener("scroll", updateActiveSection, { passive: true });
useEventListener("resize", updateActiveSection);
onMounted(updateActiveSection);
const activeSlide = ref(0);
const showcaseTabs = ref(null);
const showcaseIndicatorStyle = ref({});

function updateShowcaseIndicator() {
  const tab = showcaseTabs.value?.querySelector('[aria-selected="true"]');
  if (!tab) return;
  showcaseIndicatorStyle.value = {
    width: `${tab.offsetWidth}px`,
    transform: `translate(${tab.offsetLeft}px, ${tab.offsetTop + tab.offsetHeight - 2}px)`,
  };
}

watch([activeSlide, locale], updateShowcaseIndicator, { flush: "post" });
useResizeObserver(showcaseTabs, updateShowcaseIndicator);

const slides = [
  {
    id: "board",
    titleKey: "teambuddy_board_label",
    altKey: "teambuddy_board_alt",
    image: boardImage,
  },
  {
    id: "remote",
    titleKey: "teambuddy_remote_label",
    altKey: "teambuddy_remote_alt",
    image: remoteImage,
  },
  {
    id: "conversation",
    titleKey: "teambuddy_conversation_label",
    altKey: "teambuddy_conversation_alt",
    image: conversationImage,
  },
  {
    id: "execution",
    titleKey: "teambuddy_execution_label",
    altKey: "teambuddy_execution_alt",
    image: executionImage,
  },
  {
    id: "experts",
    titleKey: "teambuddy_experts_label",
    altKey: "teambuddy_experts_alt",
    image: expertsImage,
  },
];
const releaseVersion = ref("");
const releaseAssets = ref([]);
const loadingRelease = ref(false);
const releaseError = ref(false);
let releaseController;
const downloadPlatforms = [
  {
    id: "windows-x64",
    platform: "windows",
    titleKey: "teambuddy_platform_windows",
    detailKey: "teambuddy_platform_windows_x64",
    labelKey: "teambuddy_platform_windows_version",
    assetName: "TeamBuddy-windows-x64.exe",
  },
  {
    id: "windows-arm64",
    platform: "windows",
    titleKey: "teambuddy_platform_windows",
    detailKey: "teambuddy_platform_windows_arm64",
    labelKey: "teambuddy_platform_windows_arm64_action",
    assetName: "TeamBuddy-windows-arm64.exe",
  },
  {
    id: "macos-x64",
    platform: "macos",
    titleKey: "teambuddy_platform_macos",
    detailKey: "teambuddy_platform_intel",
    labelKey: "teambuddy_platform_intel_version",
    assetName: "TeamBuddy-macos-x64.dmg",
  },
  {
    id: "macos-arm64",
    platform: "macos",
    titleKey: "teambuddy_platform_macos",
    detailKey: "teambuddy_platform_apple",
    labelKey: "teambuddy_platform_apple_version",
    assetName: "TeamBuddy-macos-arm64.dmg",
  },
];
const downloads = computed(() =>
  downloadPlatforms.map((platform) => ({
    ...platform,
    url: releaseAssets.value.find((asset) => asset.name === platform.assetName)
      ?.url,
  })),
);

async function loadLatestRelease() {
  if (loadingRelease.value) return;
  loadingRelease.value = true;
  releaseError.value = false;
  releaseVersion.value = "";
  releaseAssets.value = [];
  releaseController = new AbortController();
  const timeout = setTimeout(() => releaseController.abort(), 10000);
  try {
    const release = await getLatestTeamBuddyRelease(releaseController.signal);
    releaseVersion.value = release.version;
    releaseAssets.value = release.assets;
  } catch {
    releaseError.value = true;
  } finally {
    clearTimeout(timeout);
    loadingRelease.value = false;
  }
}

onMounted(loadLatestRelease);
onBeforeUnmount(() => releaseController?.abort());
const reducedMotion = usePreferredReducedMotion();
const motionInitial = computed(() => {
  const profiles = {
    fade: { opacity: 0 },
    rise: { opacity: 0, y: 28 },
    left: { opacity: 0, x: -48 },
    right: { opacity: 0, x: 48 },
    zoom: { opacity: 0, y: 24, scale: 0.94 },
    pop: { opacity: 0, y: 32, scale: 0.9 },
    scene: { opacity: 0, x: 56, scale: 0.96 },
  };
  if (reducedMotion.value === "reduce") {
    return Object.fromEntries(
      Object.keys(profiles).map((key) => [
        key,
        { opacity: 1, x: 0, y: 0, scale: 1 },
      ]),
    );
  }
  return profiles;
});

function motionVisible(delay = 0, spring = false) {
  const transition =
    reducedMotion.value === "reduce"
      ? { duration: 0, delay: 0 }
      : spring
        ? { type: "spring", stiffness: 240, damping: 24, delay }
        : { duration: 650, delay };
  return { opacity: 1, x: 0, y: 0, scale: 1, transition };
}
function handleTabKey(event, index) {
  let next;
  if (event.key === "ArrowRight") next = (index + 1) % slides.length;
  else if (event.key === "ArrowLeft")
    next = (index + slides.length - 1) % slides.length;
  else if (event.key === "Home") next = 0;
  else if (event.key === "End") next = slides.length - 1;
  else return;
  event.preventDefault();
  activeSlide.value = next;
  event.currentTarget.parentElement
    .querySelectorAll('[role="tab"]')
    [next].focus();
}
</script>
<style scoped lang="less" src="./teambuddy.less"></style>

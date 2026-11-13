<template>
  <nav class="issue-invite-tabs" role="tablist">
    <div class="issue-invite-tabs__items">
      <button
        type="button"
        role="tab"
        class="issue-invite-tabs__item"
        :class="{ 'is-active': activeTab === 'create' }"
        :aria-selected="activeTab === 'create'"
        @click="switchTab('create')"
      >
        {{ $t("issue_invite_create_issue") }}
      </button>
      <button
        type="button"
        role="tab"
        class="issue-invite-tabs__item"
        :class="{ 'is-active': activeTab === 'history' }"
        :aria-selected="activeTab === 'history'"
        @click="switchTab('history')"
      >
        {{ $t("issue_invite_history") }}
      </button>
    </div>
  </nav>
  <main class="issue-invite-page">
    <!-- Shared tabs: visible on every view. -->

    <div
      v-if="state === 'loading' || state === 'detail-loading'"
      class="issue-invite-page__loading"
    >
      <a-spin />
    </div>
    <a-result
      v-else-if="state === 'invalid'"
      status="error"
      :title="$t('issue_invite_invalid')"
    />

    <!-- Detail view: dedicated route /invite/i/:key/history/:requirementId. -->
    <template v-else-if="isDetailView && detail">
      <!-- 改并重提复用新建表单，预填值来自详情的 edit_fields -->
      <IssueForm
        v-if="editing"
        :template="editing.template"
        :submitting="submitting"
        :invite-key="inviteKey"
        :openid="resolvedOpenid"
        :lang="currentLang"
        :initial-values="editing.initialValues"
        edit-mode
        @submit="resubmitIssue"
        @cancel="cancelEdit"
      />
      <IssueHistoryDetail
        v-else
        :detail="detail"
        :invite-key="inviteKey"
        :openid="resolvedOpenid"
        :lang="currentLang"
        :acting="acting"
        @recall="recallIssue"
        @urge="urgeIssue"
        @edit="startEdit"
      />
    </template>

    <!-- History list: dedicated route /invite/i/:key/history. -->
    <IssueHistoryList
      v-else-if="activeTab === 'history'"
      :items="history.items"
      :page="history.page"
      :page-size="history.page_size"
      :total="history.total"
      @select="openDetail"
      @page-change="loadHistory"
    />

    <!-- Picker view: /invite/i/:key (create tab). -->
    <IssueTemplatePicker
      v-else-if="!templateKey"
      :templates="templates"
      :guide-title="guideTitle"
      :guide-description="guideDescription"
      :theme-image="themeImage"
      @select="goCreate"
    />

    <!-- Form view: /invite/i/:key/:templateKey. -->
    <template v-else>
      <div class="issue-invite-form-head">
        <img
          v-if="selectedTemplate?.theme_image"
          :src="selectedTemplate.theme_image"
          class="issue-invite-form-head__theme"
          alt=""
        />
        <div
          v-if="
            selectedTemplate?.guide_title || selectedTemplate?.guide_description
          "
          class="issue-invite-form-head__card"
        >
          <h1
            v-if="selectedTemplate?.guide_title"
            class="issue-invite-form-head__title"
          >
            {{ selectedTemplate.guide_title }}
          </h1>
          <p
            v-if="selectedTemplate?.guide_description"
            class="issue-invite-form-head__desc"
          >
            {{ selectedTemplate.guide_description }}
          </p>
        </div>
      </div>
      <IssueForm
        :key="createDraftKey"
        :draft="createDraft"
        :template="selectedTemplate"
        :submitting="submitting"
        :invite-key="inviteKey"
        :openid="resolvedOpenid"
        :lang="currentLang"
        :link-values="linkValues"
        @submit="submitIssue"
      />
    </template>
  </main>
</template>

<script setup>
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch,
} from "vue";
import { useRoute, useRouter } from "vue-router";
import { message } from "ant-design-vue";
import { useI18n } from "vue-i18n";
import { useIssueOpenid } from "@/composables/useIssueOpenid";
import { useIssueInviteLang } from "@/composables/useIssueInviteLang";
import {
  getPublicIssueDetail,
  getPublicIssueInvite,
  getPublicIssueTemplate,
  listPublicIssueHistory,
  recallPublicIssue,
  resubmitPublicIssue,
  submitPublicIssue,
  urgePublicIssue,
} from "@/api/issueInvite";
import IssueForm from "./components/IssueForm.vue";
import IssueHistoryDetail from "./components/IssueHistoryDetail.vue";
import IssueHistoryList from "./components/IssueHistoryList.vue";
import IssueTemplatePicker from "./components/IssueTemplatePicker.vue";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const viewportMeta = document.querySelector('meta[name="viewport"]');
const originalViewport = viewportMeta.content;
const originalTouchAction = document.body.style.touchAction;
// Path params are the canonical form; legacy `?k=` / `?t=` links stay readable.
const inviteKey = computed(() =>
  String(route.params.key || route.query.k || ""),
);
const templateKey = computed(() =>
  String(route.params.templateKey || route.query.t || ""),
);
const requirementId = computed(() => String(route.params.requirementId || ""));
// The detail route has its own address under the history segment.
const isDetailView = computed(() => route.name === "IssueInviteHistoryDetail");
// The active tab is fully route-driven: history has its own address, and legacy
// `?tab=history` deep links on the picker route stay readable.
const isHistoryView = computed(
  () =>
    route.name === "IssueInviteHistory" ||
    route.name === "IssueInviteHistoryDetail" ||
    String(route.query.tab || "") === "history",
);
const activeTab = computed(() => (isHistoryView.value ? "history" : "create"));
const {
  resolvedOpenid,
  invalid: openidInvalid,
  initialize: initializeOpenid,
} = useIssueOpenid(route);
// `?Lang=zh-CN|en-US` decides the page language; it is forwarded to every public API call.
const { currentLang } = useIssueInviteLang(route);
const state = ref("loading");
const templates = ref([]);
const guideTitle = ref("");
const guideDescription = ref("");
const themeImage = ref("");
const selectedTemplate = ref(null);
// 从邀请链接上取到的参数值，按 field_key 归位后交给表单预填
const linkValues = ref({});
const submitting = ref(false);
const history = ref({ items: [], page: 1, page_size: 10, total: 0 });
const detail = ref(null);
// 撤回/催单请求进行中
const acting = ref(false);
// 编辑态：{ template, initialValues }；为 null 时看详情
const editing = ref(null);
// Drafts live only as long as this invitation page and never cross submitters or templates.
const drafts = reactive(new Map());
const createDraftKey = computed(() =>
  JSON.stringify([inviteKey.value, resolvedOpenid.value, templateKey.value]),
);
const createDraft = computed(() => {
  const key = createDraftKey.value;
  if (!drafts.has(key)) {
    drafts.set(key, reactive({ values: {}, files: [], uploading: false }));
  }
  return drafts.get(key);
});
let entryRequestSeq = 0;
let historyRequestSeq = 0;

const KEY_PATTERN = /^[A-Za-z0-9._-]{16}$/;
const TEMPLATE_KEY_PATTERN = /^[A-Za-z0-9._-]{1,64}$/;

function markInvalid() {
  state.value = "invalid";
}

async function loadPickerConfig(seq) {
  const config = await getPublicIssueInvite(
    inviteKey.value,
    resolvedOpenid.value,
    currentLang.value,
  );
  if (seq !== entryRequestSeq) return;
  templates.value = config?.templates || [];
  guideTitle.value = config?.guide_title || "";
  guideDescription.value = config?.guide_description || "";
  themeImage.value = config?.theme_image || "";
  if (!templates.value.length) {
    markInvalid();
    return;
  }
  state.value = "ready";
}

async function loadFormTemplate(seq) {
  if (!TEMPLATE_KEY_PATTERN.test(templateKey.value)) {
    markInvalid();
    return;
  }
  const template = await getPublicIssueTemplate(
    inviteKey.value,
    templateKey.value,
    resolvedOpenid.value,
    currentLang.value,
  );
  if (seq !== entryRequestSeq) return;
  selectedTemplate.value = template;
  linkValues.value = resolveLinkValues(selectedTemplate.value, route.query);
  rememberTemplateKey(templateKey.value);
  state.value = "form";
}

// 这几个 query 参数页面自己要用（邀请 key、模板 key、语言、openid、标签页），
// 模板里把字段参数名配成同名不会生效
const RESERVED_LINK_PARAMS = new Set([
  "k",
  "t",
  "Lang",
  "lang",
  "tab",
  "open_mode",
  "openid",
]);

// 链接传参：模板里配了参数名的字段，按参数名从链接上取值。
// 值保持原样交给表单，表单知道每个字段是什么控件、该按名称还是 key 认。
function resolveLinkValues(template, query) {
  const values = {};
  let schema = [];
  try {
    schema = JSON.parse(template?.fields_json || "[]");
  } catch (error) {
    return values;
  }
  for (const spec of schema) {
    const param = String(spec.link_param || "").trim();
    if (!param || RESERVED_LINK_PARAMS.has(param)) continue;
    const raw = query[param];
    if (raw === undefined || raw === null || raw === "") continue;
    values[spec.field_key] = raw;
  }
  return values;
}

async function loadDetailData(seq) {
  if (!requirementId.value) {
    markInvalid();
    return;
  }
  if (
    !KEY_PATTERN.test(inviteKey.value) ||
    openidInvalid.value ||
    !resolvedOpenid.value
  ) {
    markInvalid();
    return;
  }
  state.value = "detail-loading";
  try {
    const currentDetail = await getPublicIssueDetail(
      inviteKey.value,
      resolvedOpenid.value,
      parseInt(requirementId.value, 10),
      currentLang.value,
    );
    if (seq !== entryRequestSeq) return;
    detail.value = currentDetail;
    state.value = "ready";
  } catch (error) {
    if (seq === entryRequestSeq) markInvalid();
  }
}

async function loadEntry() {
  const seq = ++entryRequestSeq;
  if (
    !inviteKey.value ||
    !KEY_PATTERN.test(inviteKey.value) ||
    openidInvalid.value ||
    !resolvedOpenid.value
  ) {
    markInvalid();
    return;
  }

  state.value = "loading";
  resolveOpenMode();
  try {
    if (isDetailView.value) {
      await loadDetailData(seq);
    } else if (isHistoryView.value) {
      await loadHistory(1);
    } else if (templateKey.value) {
      await loadFormTemplate(seq);
    } else {
      await loadPickerConfig(seq);
    }
  } catch (error) {
    if (seq === entryRequestSeq) markInvalid();
  }
}

// Keep `?Lang=` on internal navigation so a refresh never loses the chosen language.
// The legacy `tab` param is stripped: tab state now lives in the route itself.
function withLang(query = {}) {
  const { tab: _ignored, ...base } = { ...route.query, ...query };
  return currentLang.value ? { ...base, Lang: currentLang.value } : base;
}

function goCreate(key) {
  if (!key || !inviteKey.value) return;
  router.push({
    name: "IssueInviteTemplate",
    params: { key: inviteKey.value, templateKey: key },
    query: withLang(),
  });
}

// Remember the last opened template per invite key (session-scoped) so the create
// tab can restore the form after the user visited history, instead of the picker.
const TEMPLATE_RECALL_KEY = (key) => `issue_invite_template:${key}`;

function recallTemplateKey() {
  try {
    return sessionStorage.getItem(TEMPLATE_RECALL_KEY(inviteKey.value)) || "";
  } catch (error) {
    return "";
  }
}

function rememberTemplateKey(key) {
  if (!key) return;
  try {
    sessionStorage.setItem(TEMPLATE_RECALL_KEY(inviteKey.value), key);
  } catch (error) {
    // Storage unavailable (e.g. privacy mode): the create tab just falls back to the picker.
  }
}

// The `?open_mode=` deep link decides what the create tab does after history:
// `template` restores the last opened template form, `common` always opens the picker.
const OPEN_MODE_TEMPLATE = "template";
const OPEN_MODE_COMMON = "common";
const OPEN_MODE_KEY = (key) => `issue_invite_mode:${key}`;
const openMode = ref(OPEN_MODE_COMMON);

function isOpenMode(value) {
  return value === OPEN_MODE_TEMPLATE || value === OPEN_MODE_COMMON;
}

function readOpenMode() {
  try {
    return sessionStorage.getItem(OPEN_MODE_KEY(inviteKey.value)) || "";
  } catch (error) {
    return "";
  }
}

function writeOpenMode(mode) {
  if (!isOpenMode(mode)) return;
  try {
    sessionStorage.setItem(OPEN_MODE_KEY(inviteKey.value), mode);
  } catch (error) {
    // Storage unavailable (e.g. privacy mode): fall back to the URL param or default.
  }
}

// The URL param always overwrites the cache; without it the cache decides; without
// either, initialize from the current route (template form => template, else common).
function resolveOpenMode() {
  const param = String(route.query.open_mode || "");
  if (isOpenMode(param)) {
    writeOpenMode(param);
    openMode.value = param;
    return;
  }
  const cached = readOpenMode();
  if (isOpenMode(cached)) {
    openMode.value = cached;
    return;
  }
  const init = templateKey.value ? OPEN_MODE_TEMPLATE : OPEN_MODE_COMMON;
  writeOpenMode(init);
  openMode.value = init;
}

// The two tabs map to dedicated routes; the current view never re-navigates to itself.
// The detail view is an exception: it shares the history tab, so clicking the tab there
// must navigate back to the history list instead of being swallowed.
function switchTab(tab) {
  if (tab !== "create" && tab !== "history") return;
  if (!inviteKey.value) return;
  if (activeTab.value === tab && !templateKey.value && !isDetailView.value)
    return;
  // Only in template mode does the create tab restore the last template form;
  // common mode always lands on the picker.
  if (
    tab === "create" &&
    !templateKey.value &&
    openMode.value === OPEN_MODE_TEMPLATE
  ) {
    const lastKey = recallTemplateKey();
    if (lastKey) {
      router.push({
        name: "IssueInviteTemplate",
        params: { key: inviteKey.value, templateKey: lastKey },
        query: withLang(),
      });
      return;
    }
  }
  router.push({
    name: tab === "history" ? "IssueInviteHistory" : "IssueInvite",
    params: { key: inviteKey.value },
    query: withLang(),
  });
}

async function submitIssue({ fields, fileIds }) {
  if (submitting.value) return;
  const submittedDraftKey = createDraftKey.value;
  submitting.value = true;
  try {
    await submitPublicIssue(
      inviteKey.value,
      selectedTemplate.value.template_key,
      resolvedOpenid.value,
      fields,
      currentLang.value,
      fileIds,
    );
    drafts.delete(submittedDraftKey);
    // After a successful submit, jump to the dedicated history route; the route
    // watcher loads the fresh record list.
    await router.push({
      name: "IssueInviteHistory",
      params: { key: inviteKey.value },
      query: withLang(),
    });
  } catch (error) {
    // Keep the form open so the user can correct and resubmit.
  } finally {
    submitting.value = false;
  }
}

async function loadHistory(page = 1) {
  const entrySeq = entryRequestSeq;
  const seq = ++historyRequestSeq;
  try {
    const data = await listPublicIssueHistory(
      inviteKey.value,
      resolvedOpenid.value,
      page,
      currentLang.value,
    );
    if (entrySeq !== entryRequestSeq || seq !== historyRequestSeq) return;
    history.value = {
      items: data?.items || [],
      page: data?.page || page,
      page_size: data?.page_size || 10,
      total: data?.total || 0,
    };
    state.value = "ready";
    // Reset after the list replaces the long form, including when changing pages.
    await nextTick();
    if (entrySeq === entryRequestSeq && seq === historyRequestSeq) {
      window.scrollTo({ top: 0, left: 0, behavior: "instant" });
    }
  } catch (error) {
    if (entrySeq === entryRequestSeq && seq === historyRequestSeq)
      markInvalid();
  }
}

function openDetail(requirementId) {
  if (!requirementId) return;
  router.push({
    name: "IssueInviteHistoryDetail",
    params: { key: inviteKey.value, requirementId: String(requirementId) },
    query: withLang(),
  });
}

// 撤回/催单都返回最新的详情，直接替换掉旧的，省一次请求
async function recallIssue() {
  if (acting.value || !detail.value) return;
  acting.value = true;
  try {
    detail.value = await recallPublicIssue(
      inviteKey.value,
      resolvedOpenid.value,
      detail.value.requirement_id,
      currentLang.value,
    );
    message.success(t("issue_invite_recall_success"));
  } catch (error) {
    // 失败原因由请求层提示
  } finally {
    acting.value = false;
  }
}

async function urgeIssue() {
  if (acting.value || !detail.value) return;
  acting.value = true;
  try {
    detail.value = await urgePublicIssue(
      inviteKey.value,
      resolvedOpenid.value,
      detail.value.requirement_id,
      currentLang.value,
    );
    message.success(t("issue_invite_urge_success"));
  } catch (error) {
    // 失败原因由请求层提示
  } finally {
    acting.value = false;
  }
}

// 编辑态要按模板重新渲染表单，所以先用详情里的 template_key 拉一次 schema
async function startEdit() {
  const current = detail.value;
  if (!current) return;
  // 模板被删掉的老工单拿不到 schema，改不了
  if (!current.template_key) {
    message.error(t("issue_invite_template_not_found"));
    return;
  }
  try {
    const template = await getPublicIssueTemplate(
      inviteKey.value,
      current.template_key,
      resolvedOpenid.value,
      currentLang.value,
    );
    const initialValues = {};
    for (const field of current.edit_fields || []) {
      initialValues[field.field_key] = field.value;
    }
    // 状态改完也走 edit_fields 的原始值预填，跟其它下拉一致
    editing.value = { template, initialValues };
  } catch (error) {
    // 失败原因由请求层提示
  }
}

function cancelEdit() {
  editing.value = null;
}

async function resubmitIssue({ fields, fileIds }) {
  if (submitting.value || !detail.value) return;
  submitting.value = true;
  try {
    detail.value = await resubmitPublicIssue(
      inviteKey.value,
      resolvedOpenid.value,
      detail.value.requirement_id,
      fields,
      fileIds,
      currentLang.value,
    );
    editing.value = null;
    message.success(t("issue_invite_resubmit_success"));
  } catch (error) {
    // 保留编辑态，用户改完还能再提交
  } finally {
    submitting.value = false;
  }
}

onMounted(() => {
  viewportMeta.content =
    "width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no";
  // Keep scrolling available while disabling page zoom, including portaled drawers.
  document.body.style.touchAction = "pan-x pan-y";
  initializeOpenid();
  loadEntry();
});

onBeforeUnmount(() => {
  entryRequestSeq += 1;
  viewportMeta.content = originalViewport;
  document.body.style.touchAction = originalTouchAction;
});

// If the user navigates between the picker, history and form routes (back/forward),
// reload the entry without keeping stale data around. The view flag is part of the
// key because picker and history share the same invite key and templateKey is empty on both.
watch(
  () =>
    `${inviteKey.value}/${templateKey.value}/${requirementId.value}/${route.name}/${route.query.open_mode || ""}/${route.query.openid || ""}`,
  () => {
    initializeOpenid();
    // Reset detail data when navigating away from detail route.
    if (!isDetailView.value) {
      detail.value = null;
    }
    // 换页/换工单后不该还停在上一张的编辑态
    editing.value = null;
    loadEntry();
  },
);
</script>

<style scoped lang="less">
// The public page renders as a centered column on PC (max 998px), full width on mobile.
:global(body) {
  background: #f5f6f8;
}

.issue-invite-page {
  width: 100%;
  max-width: 998px;
  margin: 0 auto;
  min-height: 100vh;
  background: #f5f6f8;
}

// The sticky tabs are 49px tall; fill the rest of the viewport so the loading
// spinner sits centered in the visible content area instead of the top-left corner.
.issue-invite-page__loading {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: calc(100vh - 49px);
}

// Mirrors the public catalog top area (`IssueTemplatePicker`): theme banner followed by
// a plain guide block on the page background, with the same type scale and spacing.
.issue-invite-form-head {
  padding: 16px 16px 0;

  // Theme banner: keep the uploaded aspect ratio instead of cropping it.
  &__theme {
    display: block;
    width: 100%;
    height: auto;
    margin: 0;
    border-radius: 16px;
    box-shadow: 0 14px 32px rgba(25, 43, 76, 0.06);
  }

  &__card {
    margin-top: 24px;
  }

  &__title {
    margin: 0;
    color: #172033;
    font-size: 16px;
    font-weight: 600;
    line-height: 24px;
  }

  &__desc {
    margin: 8px 0 0;
    color: #595959;
    font-size: 14px;
    line-height: 22px;
    white-space: pre-wrap;
  }
}

.issue-invite-tabs {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: stretch;
  background: #fff;
  border-bottom: 1px solid #f0f0f0;
  display: flex;
  &__items {
    max-width: 520px;
    margin: 0 auto;
    flex: 1;
    display: flex;
  }

  &__item {
    flex: 1;
    padding: 14px 8px 12px;
    background: transparent;
    border: 0;
    color: #4e5969;
    font-size: 15px;
    line-height: 22px;
    cursor: pointer;
    position: relative;
    transition: color 0.15s ease;

    &.is-active {
      color: #0d68ff;
      font-weight: 600;

      &::after {
        content: "";
        position: absolute;
        left: 50%;
        bottom: 0;
        width: 56px;
        height: 3px;
        background: #0d68ff;
        border-radius: 2px;
        transform: translateX(-50%);
      }
    }
  }
}
</style>

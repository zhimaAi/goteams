<template>
  <div class="issue-template-settings-page">
    <div class="issue-template-settings-page__header">
      <a-button
        type="link"
        class="issue-template-settings-page__back"
        @click="goBack"
      >
        <ArrowLeftOutlined />
        {{ $t("common_back") }}
      </a-button>
      <!-- 模板名称就在返回按钮右侧改：点一下变输入框，新建页直接就是输入框 -->
      <a-input
        v-if="isCreate || editingName"
        ref="nameInputRef"
        v-model:value="templateName"
        class="issue-template-settings-page__name-input"
        :maxlength="50"
        :placeholder="$t('issue_invite_template_name_placeholder')"
        @blur="editingName = false"
        @press-enter="editingName = false"
      />
      <span
        v-else
        class="issue-template-settings-page__title"
        @click="startEditName"
      >
        {{ templateName || $t("issue_invite_template_settings") }}
      </span>
      <a-tag
        v-if="template?.type_name"
        class="issue-template-settings-page__type"
        :class="template.type_key"
      >
        {{ template.type_name }}
      </a-tag>
      <a-button
        v-if="isCreate"
        type="primary"
        class="issue-template-settings-page__save"
        :loading="creating"
        @click="createTemplate"
      >
        {{ $t("common_save") }}
      </a-button>
    </div>

    <div class="issue-template-settings-page__body">
      <a-spin :spinning="loading">
        <a-empty
          v-if="!loading && !template"
          :description="$t('issue_invite_template_not_found')"
        />
        <IssueTemplateDetail
          v-else-if="template"
          ref="detailRef"
          v-model:name="templateName"
          :template="template"
          :ws-id="wsId"
          :invite-key="inviteKey"
          :can-copy-link="canCopyLink"
          :create="isCreate"
          :project-fields="projectFields"
          :status-options="statusOptions"
          :save="saveTemplate"
          @field-created="onFieldCreated"
        />
      </a-spin>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ArrowLeftOutlined } from "@ant-design/icons-vue";
import {
  getIssueInviteConfig,
  getIssueInviteTemplate,
} from "@/api/issueInvite";
import {
  listCustomFields,
  listFieldConfigs,
  listStatuses,
} from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";
import { useIssueInviteStore } from "@/stores/issueInvite";
import { message } from "ant-design-vue";
import { useI18n } from "vue-i18n";
import IssueTemplateDetail from "./IssueTemplateDetail.vue";

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const store = useIssueInviteStore();
const { t } = useI18n();
const template = ref(null);
const templateName = ref("");
const editingName = ref(false);
const nameInputRef = ref(null);
const inviteKey = ref("");
const loading = ref(false);
const creating = ref(false);
const projectFields = ref([]);
// 撤回后状态下拉框的候选状态，跟着模板类型走
const statusOptions = ref([]);
const wsId = computed(() => route.params.wsId);
const templateId = computed(() => Number(route.params.templateId));
const isCreate = computed(() => route.name === "IssueTemplateCreate");
const canCopyLink = computed(() => authStore.hasPermission("ticket:copy_link"));
const detailRef = ref(null);

let loadSeq = 0;

// 新建页还没有模板行，先给一份空壳，名称和字段由用户填
function blankTemplate() {
  return {
    id: 0,
    type_key: "requirement",
    name: "",
    description_mode: "rich-text",
    fields_json: "[]",
    theme_image: "",
    guide_title: "",
    guide_description: "",
    // 邀请文案开关默认关闭：新建模板进详情页时开关是关的，需手动开启才展示文案
    guide_enabled: 0,
    enabled: 1,
    recall_enabled: 0,
    recall_status_id: 0,
  };
}

// 新建页的字段列表就是项目属性字段里已开启的那些，顺序跟着属性字段走
async function loadProjectFields(typeKey) {
  try {
    const [configs, customs] = await Promise.all([
      listFieldConfigs(wsId.value, typeKey),
      listCustomFields(wsId.value, typeKey),
    ]);
    const items = [];
    for (const config of configs || []) {
      if (Number(config.enabled) !== 1) continue;
      items.push({ field_key: config.key, required: config.key === "title" });
      // 描述不在属性字段里，公开表单的工单内容固定要它，跟在标题后面
      if (config.key === "title") {
        items.push({ field_key: "description", required: true });
      }
    }
    // 标题被关掉时工单内容还是要填
    if (!items.some((item) => item.field_key === "description")) {
      items.unshift({ field_key: "description", required: true });
    }
    for (const custom of customs || []) {
      if (Number(custom.is_enabled) !== 1) continue;
      items.push({
        field_key: `custom:${custom.id}`,
        field_name: custom.name,
        field_type: custom.field_type,
      });
    }
    projectFields.value = items;
  } catch (error) {
    // The request interceptor already reports the error.
  }
}

async function loadStatusOptions(typeKey) {
  try {
    statusOptions.value = (await listStatuses(wsId.value, typeKey)) || [];
  } catch (error) {
    // The request interceptor already reports the error.
  }
}

// 新增自定义字段后要重拉一次字段列表：新建页取项目字段，编辑页重取模板
async function onFieldCreated() {
  if (isCreate.value) {
    await loadProjectFields("requirement");
    return;
  }
  try {
    // 先把没落盘的改动存掉，免得下面刷新时被覆盖
    await detailRef.value?.flushSave?.();
    const item = await getIssueInviteTemplate(wsId.value, templateId.value);
    if (item) template.value = item;
    await nextTick();
    detailRef.value?.refreshFields?.();
  } catch (error) {
    // The request interceptor already reports the error.
  }
}

async function loadTemplate() {
  const seq = ++loadSeq;
  loading.value = true;
  editingName.value = false;
  try {
    const config = await getIssueInviteConfig(wsId.value);
    if (seq !== loadSeq) return;
    inviteKey.value = config?.invite_key || "";
    if (isCreate.value) {
      template.value = blankTemplate();
      await loadProjectFields("requirement");
      await loadStatusOptions("requirement");
      return;
    }
    const item = await getIssueInviteTemplate(wsId.value, templateId.value);
    if (seq === loadSeq) template.value = item || null;
    await loadStatusOptions(item?.type_key || "requirement");
  } catch (error) {
    if (seq === loadSeq)
      template.value = isCreate.value ? blankTemplate() : null;
  } finally {
    if (seq === loadSeq) loading.value = false;
  }
}

// 点名字开始改名，输入框一出现就聚焦
async function startEditName() {
  editingName.value = true;
  await nextTick();
  nameInputRef.value?.focus?.();
}

// Flush any pending autosave before leaving so the list reflects the latest edits.
async function goBack() {
  try {
    await detailRef.value?.flushSave?.();
  } catch (error) {
    // The request interceptor already reported the failure; leave anyway.
  }
  router.push({
    name: "IssueSettings",
    params: { wsId: wsId.value },
  });
}

// Autosave is triggered by the detail component; update the local copy with the
// server-normalized response so navigation away and back shows fresh data.
async function saveTemplate(payload) {
  if (!template.value) return;
  const item = await store.saveTemplate(wsId.value, template.value.id, payload);
  template.value = item;
  return item;
}

// 新建必须由按钮触发：名称是必填，空着提交会被后端拒绝
async function createTemplate() {
  if (creating.value) return;
  const payload = await detailRef.value?.buildPayload?.();
  if (!payload) return;
  if (!payload.name.trim()) {
    message.warning(t("issue_invite_template_name_required"));
    return;
  }
  creating.value = true;
  try {
    const item = await store.createTemplate(wsId.value, payload);
    message.success(t("issue_invite_create_success"));
    router.replace({
      path: `/issues/${wsId.value}/settings/templates/${item.id}`,
    });
  } catch (error) {
    // The request interceptor already reports the error.
  } finally {
    creating.value = false;
  }
}

watch([wsId, templateId, isCreate], loadTemplate, { immediate: true });
</script>

<style scoped lang="less">
.issue-template-settings-page {
  display: flex;
  flex-direction: column;
  flex: 1;
  // Fill the settings content body so only __body scrolls and the header stays put;
  // the parent (.settings-content-body) is a block box with a resolved height.
  height: 100%;
  min-height: 0;
}

.issue-template-settings-page__header {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  // Keeps the header pinned to the top of the nearest scrollport even if the
  // outer container happens to scroll (e.g. narrow viewports).
  position: sticky;
  top: 0;
  z-index: 2;
  background: #fff;
  // 10px (content padding-top) + 44px/2 = 32px, matching the sidebar logo's vertical center.
  min-height: 44px;
}

.issue-template-settings-page__back {
  padding-left: 0;
  padding-right: 8px;
}

.issue-template-settings-page__title {
  color: var(--app-text, #1f1f1f);
  font-size: 18px;
  font-weight: 600;
  line-height: 24px;
  cursor: text;
}

// 改名用的输入框跟标题同样大小，别一看就像换了个控件
.issue-template-settings-page__name-input {
  width: 260px;
  font-size: 18px;
  font-weight: 600;
}

.issue-template-settings-page__type {
  margin: 0;
  border-radius: 3px;

  &.requirement {
    color: #0d68ff;
    background: #e6f4ff;
    border-color: #e6f4ff;
  }

  &.defect {
    color: #f85e5e;
    background: #fff1f0;
    border-color: #fff1f0;
  }
}

// 新建页的保存按钮靠右，标题占满中间
.issue-template-settings-page__save {
  margin-left: 6px;
}

.issue-template-settings-page__body {
  flex: 1;
  min-height: 0;
  padding: 8px 24px 24px 8px;
  overflow-y: auto;
  background: #f7f8fa;
  border-radius: 8px;
}
</style>

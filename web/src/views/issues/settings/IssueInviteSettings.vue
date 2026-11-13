<template>
  <a-empty v-if="!appEnabled" :description="$t('issue_invite_disabled')" />
  <div v-else class="issue-invite-settings">
    <div class="issue-invite-settings__main">
      <section class="issue-invite-settings__banner">
        <div class="issue-invite-settings__banner-text">
          <strong>{{ $t("issue_invite_link_title") }}</strong>
          <p>{{ $t("issue_invite_openid_hint", { code: "openid=XX" }) }}</p>
        </div>
        <a-button
          v-if="canCopyLink"
          type="primary"
          class="issue-invite-settings__copy"
          @click="copyCurrentLink"
        >
          {{ $t("issue_invite_copy_link") }}
        </a-button>
      </section>

      <section class="issue-invite-settings__card">
        <header class="issue-invite-settings__list-head">
          <h3 class="issue-invite-settings__section-title">
            {{ $t("issue_invite_copy_text") }}
          </h3>
          <div class="issue-invite-settings__head-actions">
            <a-button
              type="link"
              size="small"
              :disabled="!configEnabled"
              @click="copyTextModalOpen = true"
            >
              {{ $t("issue_invite_copy_text_edit") }}
            </a-button>
            <a-switch
              :checked="configEnabled"
              :checked-children="$t('common_on')"
              :un-checked-children="$t('common_off')"
              @change="toggleConfigEnabled"
            />
          </div>
        </header>
      </section>

      <section class="issue-invite-settings__card">
        <header class="issue-invite-settings__list-head">
          <h3 class="issue-invite-settings__section-title">
            {{ $t("issue_invite_template_list") }}
            <a-tooltip :title="$t('issue_invite_template_list_hint')">
              <QuestionCircleOutlined class="issue-invite-settings__help" />
            </a-tooltip>
          </h3>
          <a-button type="primary" @click="openCreatePage">
            {{ $t("issue_invite_add_template") }}
          </a-button>
        </header>
        <a-table
          ref="tableRef"
          :data-source="templates"
          :columns="columns"
          :pagination="false"
          :loading="store.loading"
          row-key="id"
          :row-class-name="rowClassName"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'sort'">
              <a-tooltip :title="$t('issue_invite_drag_sort_hint')">
                <span class="issue-invite-settings__drag-handle">
                  <HolderOutlined />
                </span>
              </a-tooltip>
            </template>
            <template v-else-if="column.key === 'type'">
              <WorkItemTypePill
                :type="record.type_key"
                :ws-id="props.wsId"
                class="issue-invite-settings__type-pill"
              />
            </template>
            <template v-else-if="column.key === 'enabled'">
              <a-switch
                :checked="Number(record.enabled) === 1"
                :checked-children="$t('common_on')"
                :un-checked-children="$t('common_off')"
                @change="(value) => toggleTemplate(record, value)"
              />
            </template>
            <template v-else-if="column.key === 'action'">
              <a-button
                type="link"
                size="small"
                @click="openTemplateSettings(record)"
              >
                {{ $t("common_settings") }}
              </a-button>
              <a-button
                danger
                type="link"
                size="small"
                @click="removeTemplate(record)"
              >
                {{ $t("common_remove") }}
              </a-button>
            </template>
          </template>
          <template #emptyText>
            <a-empty :description="$t('issue_invite_no_template')" />
          </template>
        </a-table>
      </section>
    </div>

    <IssuePhonePreview
      :ws-id="props.wsId"
      :templates="templates"
      :guide-title="guideTitle"
      :guide-description="guideDescription"
      :theme-image="themeImage"
      :config-enabled="configEnabled"
    />

    <IssueInviteCopyTextModal
      v-model:open="copyTextModalOpen"
      v-model:guide-title="guideTitle"
      v-model:guide-description="guideDescription"
      v-model:theme-image="themeImage"
      :ws-id="props.wsId"
      @confirm="flushSave"
      @upload="handleCopyTextUpload"
    />
  </div>
</template>

<script setup>
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
} from "vue";
import { useRouter } from "vue-router";
import { message, Modal } from "ant-design-vue";
import { HolderOutlined, QuestionCircleOutlined } from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import { storeToRefs } from "pinia";
import Sortable from "sortablejs";
import { useAuthStore } from "@/stores/auth";
import { useIssueInviteStore } from "@/stores/issueInvite";
import WorkItemTypePill from "@/components/common/WorkItemTypePill.vue";
import IssuePhonePreview from "./IssuePhonePreview.vue";
import IssueInviteCopyTextModal from "./IssueInviteCopyTextModal.vue";

const props = defineProps({
  wsId: { type: [String, Number], required: true },
  appEnabled: { type: Boolean, required: true },
});

const { t } = useI18n();
const authStore = useAuthStore();
const store = useIssueInviteStore();
const router = useRouter();
const { templates } = storeToRefs(store);
const canCopyLink = computed(() => authStore.hasPermission("ticket:copy_link"));

const guideTitle = ref("");
const guideDescription = ref("");
const themeImage = ref("");
const configEnabled = ref(true);
const copyTextModalOpen = ref(false);
const dirty = ref(false);
const saving = ref(false);
let saveTimer = null;
let syncingConfig = false;
let lastSavedToastAt = 0;

const columns = [
  { key: "sort", title: "", width: 44 },
  { key: "type", title: t("issue_invite_type"), width: 90 },
  { key: "name", title: t("issue_invite_template_name"), dataIndex: "name" },
  { key: "enabled", title: t("issue_invite_enabled"), width: 80 },
  { key: "action", title: t("issue_invite_action"), width: 130 },
];

const tableRef = ref(null);
const ordering = ref(false);
let sortableInstance = null;

function rowClassName() {
  return "issue-invite-settings__row";
}

function setupSortable() {
  const tbody = tableRef.value?.$el?.querySelector(".ant-table-tbody");
  if (!tbody) return;
  sortableInstance?.destroy();
  sortableInstance = Sortable.create(tbody, {
    handle: ".issue-invite-settings__drag-handle",
    draggable: ".issue-invite-settings__row",
    animation: 150,
    ghostClass: "issue-invite-settings__row-ghost",
    onEnd: handleSortEnd,
  });
}

async function handleSortEnd({ oldDraggableIndex, newDraggableIndex }) {
  if (ordering.value || oldDraggableIndex === newDraggableIndex) return;
  const previous = [...templates.value];
  const next = [...previous];
  const [moved] = next.splice(oldDraggableIndex, 1);
  next.splice(newDraggableIndex, 0, moved);
  templates.value = next;
  ordering.value = true;
  try {
    await store.reorderTemplates(
      props.wsId,
      next.map((item) => item.id),
    );
  } catch (error) {
    templates.value = previous;
  } finally {
    ordering.value = false;
  }
}

function currentOrigin() {
  return `${window.location.origin}${import.meta.env?.BASE_URL || "/"}`;
}

function buildInviteUrl(key) {
  if (!key) return "";
  return `${currentOrigin()}invite/i/${encodeURIComponent(key)}?open_mode=common`;
}

async function copyText(text) {
  if (!text) return;
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
    } else {
      const textarea = document.createElement("textarea");
      textarea.value = text;
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      document.body.removeChild(textarea);
    }
    message.success(t("issue_invite_copy_success"));
  } catch (error) {
    message.error(t("issue_invite_copy_failed"));
  }
}

async function copyCurrentLink() {
  const key = store.config?.invite_key || store.inviteKey;
  await copyText(buildInviteUrl(key));
}

async function saveConfigNow() {
  if (saving.value || !dirty.value) return;
  saving.value = true;
  try {
    await store.saveConfig(props.wsId, {
      guide_title: guideTitle.value.trim(),
      guide_description: guideDescription.value.trim(),
      theme_image: themeImage.value,
    });
    dirty.value = false;
    // A light toast, throttled so rapid autosaves don't spam.
    const now = Date.now();
    if (now - lastSavedToastAt > 3000) {
      lastSavedToastAt = now;
      message.success(t("issue_invite_saved"));
    }
  } catch (error) {
    message.error(t("issue_invite_config_save_failed"));
  } finally {
    saving.value = false;
  }
}

function scheduleSave() {
  if (syncingConfig) return;
  dirty.value = true;
  clearTimeout(saveTimer);
  saveTimer = setTimeout(() => {
    saveConfigNow();
  }, 800);
}

function flushSave() {
  clearTimeout(saveTimer);
  saveConfigNow();
}

// Uploading a theme image is an immediate action: persist it right away using the
// current form values so it survives a quick close or refresh.
function handleCopyTextUpload(url) {
  themeImage.value = url;
  dirty.value = true;
  clearTimeout(saveTimer);
  saveConfigNow();
}

// 新增走独立的新建页，模板的字段显示与链接提取在那里配置
function openCreatePage() {
  router.push({
    path: `/issues/${props.wsId}/settings/templates/new`,
  });
}

function openTemplateSettings(item) {
  router.push({
    path: `/issues/${props.wsId}/settings/templates/${item.id}`,
  });
}

async function toggleTemplate(item, enabled) {
  const previous = Number(item.enabled) === 1;
  try {
    await store.saveTemplate(props.wsId, item.id, {
      ...item,
      enabled: enabled ? 1 : 0,
    });
    message.success(
      t(
        enabled
          ? "issue_invite_enable_success"
          : "issue_invite_disable_success",
      ),
    );
  } catch (error) {
    item.enabled = previous ? 1 : 0;
  }
}

async function toggleConfigEnabled(value) {
  const previous = configEnabled.value;
  configEnabled.value = value;
  try {
    await store.saveConfig(props.wsId, {
      enabled: value ? 1 : 0,
    });
    message.success(
      t(
        value
          ? "issue_invite_config_enable_success"
          : "issue_invite_config_disable_success",
      ),
    );
    // 开启邀请文案功能时自动弹出邀请文案弹窗
    if (value) copyTextModalOpen.value = true;
  } catch (error) {
    configEnabled.value = previous;
  }
}

function removeTemplate(item) {
  Modal.confirm({
    title: t("issue_invite_remove_confirm_title"),
    content: t("issue_invite_remove_confirm_content", { name: item.name }),
    okText: t("common_confirm"),
    cancelText: t("common_cancel"),
    async onOk() {
      await store.removeTemplate(props.wsId, item.id);
      message.success(t("issue_invite_remove_success"));
    },
  });
}

// Sortable attaches to the antd-rendered tbody; rebuild whenever the row list changes
// (initial load, optimistic reorder, import/remove) since rows are keyed re-renders.
onMounted(async () => {
  await nextTick();
  setupSortable();
});

watch(templates, async () => {
  await nextTick();
  setupSortable();
});

watch(
  () => [props.wsId, props.appEnabled],
  ([currentWsId, enabled]) => {
    if (enabled) {
      store.loadConfiguration(currentWsId).catch(() => {
        // The application may have been disabled after the metadata snapshot;
        // clear workspace-specific state so stale data is never shown.
        store.reset();
      });
    } else {
      store.reset();
    }
  },
  { immediate: true },
);

// Sync the editable fields with the store config; store changes never mark the form dirty.
watch(
  () => store.config,
  (cfg) => {
    syncingConfig = true;
    guideTitle.value = cfg?.guide_title || "";
    guideDescription.value = cfg?.guide_description || "";
    themeImage.value = cfg?.theme_image || "";
    configEnabled.value = Number(cfg?.enabled ?? 1) === 1;
    syncingConfig = false;
  },
  { immediate: true },
);

watch(
  [guideTitle, guideDescription, themeImage],
  () => {
    scheduleSave();
  },
  // Sync flush keeps the syncingConfig guard effective: store.config assignments happen
  // inside its own sync watcher, so the guard must still be set when this runs, or a
  // page load would fire a spurious autosave.
  { flush: "sync" },
);

onBeforeUnmount(() => {
  sortableInstance?.destroy();
  sortableInstance = null;
  clearTimeout(saveTimer);
  if (dirty.value && !saving.value) {
    saveConfigNow();
  }
});
</script>

<style scoped lang="less">
.issue-invite-settings {
  display: flex;
  gap: 24px;
  align-items: flex-start;

  &__main {
    flex: 1;
    min-width: 0;
  }

  &__banner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 16px 20px;
    background: #e6f4ff;
    border-radius: 8px;
  }

  &__banner-text {
    color: #1f1f1f;
    font-size: 14px;

    strong {
      font-weight: 600;
    }

    p {
      margin: 4px 0 0;
      color: #5f5f5f;
      font-size: 12px;
    }
  }

  &__copy {
    flex: 0 0 auto;
    background: #0d68ff;
    border-color: #0d68ff;
    border-radius: 6px;
  }

  &__card {
    margin-top: 16px;
    padding: 16px 20px;
    background: #fff;
    border: 1px solid #f0f0f0;
    border-radius: 8px;
  }

  &__section-title {
    margin: 0;
    color: #1f1f1f;
    font-size: 15px;
    font-weight: 600;
  }

  &__help {
    margin-left: 4px;
    color: #c1c1c1;
    font-size: 13px;
  }

  &__list-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 12px;
  }

  &__head-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  &__drag-handle {
    display: inline-flex;
    color: rgba(0, 0, 0, 0.35);
    font-size: 17px;
    cursor: grab;

    &:active {
      cursor: grabbing;
    }
  }

  &__type-pill {
    display: inline-block;
    height: auto;
    padding: 1px 6px;
    border-radius: 3px;
    color: #fff;
    font-size: 12px;
    line-height: 18px;
  }
}
// The sortable ghost is the antd-rendered row itself, which carries no scoped attr.
:deep(.issue-invite-settings__row-ghost) {
  opacity: 0.5;
  background: #e6f4ff;
}
</style>

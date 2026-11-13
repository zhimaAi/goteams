<template>
  <div class="api-token-panel">
    <a-alert
      v-if="hintVisible"
      class="token-hint"
      type="warning"
      show-icon
      closable
      @close="hintVisible = false"
    >
      <template #message>
        <p class="hint-line">{{ $t("api_token_hint") }}</p>
        <p class="hint-line">
          <i18n-t keypath="api_token_hintSkill" tag="span">
            <template #skill><span class="hint-skill">skill</span></template>
          </i18n-t>
        </p>
        <p class="hint-line">
          <i18n-t keypath="api_token_hintDoc" tag="span">
            <template #doc>
              <span class="hint-doc">{{ $t("api_token_doc") }}</span>
            </template>
          </i18n-t>
        </p>
      </template>
    </a-alert>

    <section class="token-card">
      <header class="token-card-head">
        <h3 class="token-card-title">{{ $t("api_token_listTitle") }}</h3>
        <div class="token-head-actions">
          <a-button :loading="docDownloading" @click="handleDownloadDoc">
            <FileTextOutlined />
            {{ $t("api_token_doc") }}
          </a-button>
          <a-tooltip
            v-if="limitReached"
            :title="$t('api_token_limitTip', { count: maxTokens })"
          >
            <span class="create-btn-wrap">
              <a-button type="primary" disabled>
                <PlusOutlined />
                {{ $t("api_token_create") }}
              </a-button>
            </span>
          </a-tooltip>
          <a-button v-else type="primary" @click="openEditor(null)">
            <PlusOutlined />
            {{ $t("api_token_create") }}
          </a-button>
        </div>
      </header>

      <a-spin :spinning="loading">
        <a-empty
          v-if="!loading && !tokens.length"
          :description="$t('api_token_empty')"
        />
        <div v-else class="token-list">
          <article v-for="record in tokens" :key="record.id" class="token-item">
            <!-- key row -->
            <div class="token-item-top">
              <span class="token-label">{{ $t("api_token_keyLabel") }}</span>
              <code class="token-key-text">{{ record.token }}</code>
              <a-tooltip :title="$t('common_copy')">
                <CopyOutlined
                  class="token-key-copy"
                  @click="copyToken(record)"
                />
              </a-tooltip>

              <span class="token-actions">
                <a-switch
                  :checked="record.enabled === 1"
                  :checked-children="$t('common_on')"
                  :un-checked-children="$t('common_off')"
                  :loading="togglingId === record.id"
                  class="token-switch"
                  @change="(checked) => handleToggle(record, checked)"
                />
                <a-button
                  type="link"
                  size="small"
                  class="token-action-btn"
                  @click="handleDownloadSkill(record)"
                >
                  <DownloadOutlined />
                  {{ $t("api_token_downloadSkill") }}
                </a-button>
                <a-button
                  type="link"
                  size="small"
                  class="token-action-btn"
                  :loading="installLoading"
                  @click="handleInstallSkill(record)"
                >
                  <ImportOutlined />
                  {{ $t("api_token_installSkill") }}
                </a-button>
                <a-button
                  type="link"
                  size="small"
                  class="token-action-btn"
                  @click="openEditor(record)"
                >
                  <EditOutlined />
                  {{ $t("api_token_edit") }}
                </a-button>
                <a-popconfirm
                  :title="$t('api_token_deleteConfirm')"
                  :ok-text="$t('common_confirm')"
                  :cancel-text="$t('common_cancel')"
                  @confirm="handleDelete(record)"
                >
                  <a-button
                    type="link"
                    size="small"
                    danger
                    class="token-action-btn"
                  >
                    <DeleteOutlined />
                    {{ $t("common_delete") }}
                  </a-button>
                </a-popconfirm>
              </span>
            </div>

            <!-- remark row -->
            <p class="token-meta">
              <span class="meta-label">{{ $t("api_token_nameLabel") }}</span>
              <span class="meta-value">{{ record.name || "-" }}</span>
            </p>

            <!-- usage row -->
            <p class="token-meta token-usage-row">
              <span class="usage-group">
                <span class="meta-label">{{ $t("api_token_usageToday") }}</span>
                <b class="meta-number">{{ record.today_calls ?? 0 }}</b>
              </span>
              <span class="usage-group">
                <span class="meta-label">{{
                  $t("api_token_usageYesterday")
                }}</span>
                <b class="meta-number">{{ record.yesterday_calls ?? 0 }}</b>
              </span>
              <span class="usage-group">
                <span class="meta-label">{{
                  $t("api_token_usageCumulative")
                }}</span>
                <b class="meta-number">{{ record.cumulative_calls ?? 0 }}</b>
              </span>
            </p>

            <!-- permissions row -->
            <div class="token-meta token-perms-row">
              <span class="meta-label">{{
                $t("api_token_permissionsLabel")
              }}</span>
              <a-tag
                v-for="item in record.permissions"
                :key="item.key"
                color="processing"
                class="perm-tag"
              >
                {{ item.name }}
              </a-tag>
            </div>
          </article>
        </div>
      </a-spin>
    </section>

    <!-- editor modal: name + permissions -->
    <a-modal
      v-model:open="editorOpen"
      :title="editingId ? $t('api_token_editTitle') : $t('api_token_create')"
      :ok-text="$t('common_save')"
      :cancel-text="$t('common_cancel')"
      :confirm-loading="saving"
      width="520px"
      @ok="handleSave"
    >
      <a-form layout="vertical" class="editor-form">
        <a-form-item :label="$t('api_token_name')">
          <a-input
            v-model:value="editorName"
            :maxlength="100"
            :placeholder="$t('api_token_namePlaceholder')"
          />
        </a-form-item>

        <a-form-item
          :label="$t('api_token_permissions')"
          :validate-status="permError ? 'error' : ''"
          :help="permError ? $t('api_token_permRequired') : ''"
          required
        >
          <a-checkbox-group
            v-model:value="editorPermissions"
            class="permission-group"
            :options="permissionOptions"
          />
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- install skill modal: temporary link, countdown, copy-for-AI text -->
    <a-modal
      v-model:open="installOpen"
      :title="$t('api_token_installTitle')"
      :footer="null"
      :destroy-on-close="true"
      width="520px"
      @cancel="handleInstallClose"
    >
      <div class="install-body">
        <a-alert
          class="install-validity"
          :type="installExpired ? 'warning' : 'info'"
          show-icon
          :message="
            installExpired
              ? $t('api_token_installLinkExpired')
              : $t('api_token_installLinkValid', { time: installCountdown })
          "
        />
        <p class="install-tip">{{ $t("api_token_installTip") }}</p>
        <div class="install-code">{{ installText }}</div>
        <div class="install-actions">
          <a-button
            type="primary"
            :disabled="installExpired"
            @click="copyInstallText"
          >
            <CopyOutlined />
            {{ $t("api_token_installCopy") }}
          </a-button>
        </div>
      </div>
    </a-modal>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import {
  CopyOutlined,
  DeleteOutlined,
  DownloadOutlined,
  EditOutlined,
  FileTextOutlined,
  ImportOutlined,
  PlusOutlined,
} from "@ant-design/icons-vue";
import {
  createApiToken,
  createApiTokenSkillLink,
  deleteApiToken,
  downloadApiTokenDoc,
  downloadApiTokenSkill,
  listApiTokenPermissions,
  listApiTokens,
  setApiTokenEnabled,
  updateApiToken,
} from "@/api/apiToken";
const { t } = useI18n();

const maxTokens = 10; // 与后端 business.ApiTokenMaxPerUser 保持一致
const hintVisible = ref(true);
const tokens = ref([]);
const permissions = ref([]);
const loading = ref(false);
const togglingId = ref(0);
const docDownloading = ref(false);

const editorOpen = ref(false);
const saving = ref(false);
const editingId = ref(0);
const editorName = ref("");
const editorPermissions = ref([]);
const permError = ref(false);

// Temporary install link: the backend keeps it in Redis for five minutes and allows at most
// ten downloads, so the modal only needs to show the remaining time.
const installOpen = ref(false);
const installUrl = ref("");
const installRemain = ref(0);
const installLoading = ref(false);
let installTimer = null;

const installExpired = computed(() => installRemain.value <= 0);

const installCountdown = computed(() => {
  const remain = Math.max(installRemain.value, 0);
  const minutes = String(Math.floor(remain / 60)).padStart(2, "0");
  const seconds = String(remain % 60).padStart(2, "0");
  return `${minutes}:${seconds}`;
});

const installText = computed(() =>
  t("api_token_installText", { url: installUrl.value }),
);

const limitReached = computed(() => tokens.value.length >= maxTokens);

const permissionOptions = computed(() =>
  permissions.value.map((item) => ({ label: item.name, value: item.key })),
);

onMounted(async () => {
  await Promise.all([fetchTokens(), fetchPermissions()]);
});

onBeforeUnmount(stopInstallCountdown);

async function fetchTokens() {
  loading.value = true;
  try {
    tokens.value = (await listApiTokens()) || [];
  } catch {
    // The request layer displays errors uniformly.
  } finally {
    loading.value = false;
  }
}

async function fetchPermissions() {
  try {
    permissions.value = (await listApiTokenPermissions()) || [];
  } catch {
    // The request layer displays errors uniformly.
  }
}

function openEditor(record) {
  editingId.value = record?.id || 0;
  editorName.value = record?.name || "";
  editorPermissions.value = (record?.permissions || []).map((item) => item.key);
  permError.value = false;
  editorOpen.value = true;
}

async function handleSave() {
  if (!editorPermissions.value.length) {
    permError.value = true;
    return;
  }
  permError.value = false;
  saving.value = true;
  const payload = {
    name: editorName.value.trim(),
    permissions: editorPermissions.value,
  };
  try {
    if (editingId.value) {
      await updateApiToken(editingId.value, payload);
      message.success(t("api_token_updated"));
    } else {
      await createApiToken(payload);
      message.success(t("api_token_created"));
    }
    editorOpen.value = false;
    await fetchTokens();
  } catch {
    // The request layer displays errors uniformly.
  } finally {
    saving.value = false;
  }
}

async function handleToggle(record, checked) {
  togglingId.value = record.id;
  try {
    await setApiTokenEnabled(record.id, checked ? 1 : 0);
  } catch {
    // The request layer displays errors uniformly.
  } finally {
    togglingId.value = 0;
    await fetchTokens();
  }
}

async function handleDelete(record) {
  try {
    await deleteApiToken(record.id);
    message.success(t("api_token_deleted"));
  } catch {
    // The request layer displays errors uniformly.
  } finally {
    await fetchTokens();
  }
}

// navigator.clipboard only exists in a secure context (HTTPS or localhost). Environments
// served over plain HTTP, such as the dev*.applnk.cn hosts, expose no clipboard API at all,
// so fall back to a hidden textarea plus execCommand instead of failing silently.
async function copyText(text) {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.left = "-9999px";
  textarea.style.opacity = "0";
  document.body.appendChild(textarea);
  try {
    textarea.select();
    if (!document.execCommand("copy")) {
      throw new Error("execCommand copy failed");
    }
  } finally {
    textarea.remove();
  }
}

async function copyToken(record) {
  try {
    await copyText(record.token);
    message.success(t("common_copied"));
  } catch {
    message.warning(t("api_token_copyFallback"));
  }
}

async function handleDownloadSkill(record) {
  try {
    const response = await downloadApiTokenSkill(record.id);
    saveResponseFile(response, "goteams-api.zip");
  } catch {
    // The blob interceptor shows a generic toast on 404/500.
  }
}

// The install link is generated on demand: it lives in Redis for five minutes and allows
// at most ten downloads, so every click gets a fresh one.
async function handleInstallSkill(record) {
  // One click, one link. The backend keeps a single link per token, but concurrent
  // create calls are not serialised, so a double click could leave a second live link
  // behind. Disabling the button while the request is in flight closes that window.
  if (installLoading.value) {
    return;
  }
  installLoading.value = true;
  try {
    const data = await createApiTokenSkillLink(record.id);
    installUrl.value = data?.url || "";
    installRemain.value = data?.expire_seconds || 0;
    installOpen.value = true;
    startInstallCountdown();
  } catch {
    // The request layer displays errors uniformly.
  } finally {
    installLoading.value = false;
  }
}

function startInstallCountdown() {
  stopInstallCountdown();
  installTimer = window.setInterval(() => {
    installRemain.value -= 1;
    if (installRemain.value <= 0) {
      stopInstallCountdown();
    }
  }, 1000);
}

function stopInstallCountdown() {
  if (installTimer !== null) {
    window.clearInterval(installTimer);
    installTimer = null;
  }
}

function handleInstallClose() {
  installOpen.value = false;
  stopInstallCountdown();
}

async function copyInstallText() {
  try {
    await copyText(installText.value);
    message.success(t("common_copied"));
  } catch {
    message.warning(t("api_token_copyFallback"));
  }
}

// The reference covers every endpoint and carries no key, so it needs no token selected.
async function handleDownloadDoc() {
  docDownloading.value = true;
  try {
    const response = await downloadApiTokenDoc();
    saveResponseFile(response, "goteams-api-doc.md");
  } catch {
    // The blob interceptor shows a generic toast on 404/500.
  } finally {
    docDownloading.value = false;
  }
}

function saveResponseFile(response, fallbackName) {
  const disposition = response.headers?.["content-disposition"] || "";
  const match = disposition.match(/filename\*=UTF-8''([^;]+)/i);
  const filename = match ? decodeURIComponent(match[1]) : fallbackName;
  const url = URL.createObjectURL(response.data);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
}
</script>

<style scoped lang="less">
.token-hint {
  margin-bottom: 16px;

  .hint-line {
    margin: 0;
    line-height: 1.7;
  }

  .hint-skill,
  .hint-doc {
    color: #3157e2;
  }
}

.create-btn-wrap {
  display: inline-block;
}

.token-card {
  padding: 20px 24px 24px;
  border: 1px solid var(--profile-border);
  border-radius: 8px;
  background: var(--profile-card-bg);
}

.token-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--profile-border);
  margin-bottom: 16px;
}

.token-card-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--profile-muted);
}

.token-head-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.token-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.token-item {
  padding: 16px 20px;
  border: 1px solid #f0f0f0;
  border-radius: 8px;
  background: var(--profile-card-bg);
}

.token-item-top {
  display: flex;
  align-items: center;
  gap: 12px;
}

.token-label {
  font-weight: 600;
  font-size: 14px;
  color: #1f1f1f;
  white-space: nowrap;
}

.token-key-text {
  display: inline-block;
  padding: 4px 10px;
  background: #f5f5f5;
  border-radius: 4px;
  font-family: Consolas, Monaco, "Courier New", monospace;
  font-size: 13px;
  color: #262626;
  word-break: break-all;
}

.token-key-copy {
  font-size: 15px;
  color: #595959;
  cursor: pointer;
  transition: color 0.2s;

  &:hover {
    color: #3157e2;
  }
}

.token-actions {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-left: auto;
  flex-shrink: 0;
}

.token-switch {
  flex-shrink: 0;
}

.token-action-btn {
  padding: 0;
  height: auto;
  font-size: 13px;
}

.token-meta {
  margin: 10px 0 0;
  font-size: 13px;
  line-height: 1.6;
}

.meta-label {
  color: #8c8c8c;
}

.meta-value {
  color: #595959;
}

.token-usage-row {
  display: flex;
  gap: 24px;

  .usage-group {
    white-space: nowrap;
  }

  .meta-number {
    color: #1f1f1f;
    font-weight: 700;
  }
}

.token-perms-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px 6px;

  .perm-tag {
    margin-inline-end: 0;
  }
}

.install-body {
  .install-validity {
    margin-bottom: 16px;
  }

  .install-tip {
    margin: 0 0 12px;
    font-size: 13px;
    line-height: 1.7;
    color: #595959;
  }

  .install-code {
    padding: 12px 14px;
    border-radius: 6px;
    background: #f5f5f5;
    font-size: 13px;
    line-height: 1.7;
    color: #262626;
    word-break: break-all;
  }

  .install-actions {
    display: flex;
    justify-content: flex-end;
    margin-top: 16px;
  }
}

.editor-form {
  margin-top: 16px;
}

.permission-group {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
</style>

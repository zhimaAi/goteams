<!--
 * SsoTab.vue
 * 企业 SSO 登录：开关卡常驻，未开启时只给功能简介，开启走弹窗引导；已开启后系统地址并入开关卡，下方平铺成员同步卡。
 * 平台清单与字段形态由后端 provider_meta 下发；钉钉配置按后台操作分步引导。
 -->
<template>
  <div class="sso-tab">
    <a-spin :spinning="loading">
      <!-- 开关卡是页面唯一的开启入口，未开启时也只保留它和功能简介 -->
      <div class="sso-card sso-card--switch">
        <div class="sso-switch-head">
          <div class="sso-switch-main">
            <div class="sso-switch-title">{{ $t("sso_card_title") }}</div>
            <div class="sso-switch-status">
              <i18n-t v-if="savedEnabled" keypath="sso_status_on" tag="span">
                <template #provider>
                  <span class="sso-provider-name">{{
                    currentProviderLabel
                  }}</span>
                </template>
              </i18n-t>
              <template v-else>{{ $t("sso_status_off") }}</template>
            </div>
          </div>
          <div class="sso-switch-actions">
            <!-- 配置入口只在已开启后出现：未开启时点开关本身就是开启引导 -->
            <a-button v-if="savedEnabled" @click="openEnableDialog">
              <template #icon><EditOutlined /></template>
              {{ $t("sso_configure") }}
            </a-button>
            <a-switch
              :checked="savedEnabled"
              :disabled="loading"
              :loading="saving"
              @change="onToggle"
            />
          </div>
        </div>

        <div v-if="savedEnabled" class="sso-url-section">
          <div class="sso-url-row">
            <div class="sso-url-label">{{ $t("sso_entry_url") }}</div>
            <div class="sso-url-field">
              <a-input
                :value="config?.login_entry_url || $t('sso_url_unavailable')"
                readonly
              />
              <a-button
                :disabled="!config?.login_entry_url"
                @click="copy(config.login_entry_url)"
              >
                <template #icon><CopyOutlined /></template>
                {{ $t("common_copy") }}
              </a-button>
            </div>
          </div>
        </div>
      </div>

      <div v-if="!savedEnabled" class="sso-card sso-card--intro">
        <div class="sso-intro-title">{{ $t("sso_intro_section") }}</div>
        <div class="sso-intro-body">
          <p class="sso-intro-text">{{ $t("sso_intro_content") }}</p>
          <p class="sso-intro-text sso-intro-action">
            {{ $t("sso_intro_action") }}
          </p>
        </div>
      </div>

      <template v-else>
        <div class="sso-card sso-card--sync">
          <div class="sso-sync-head">
            <div class="sso-sync-title">{{ $t("sso_sync_section") }}</div>
          </div>

          <div class="sso-sync-body">
            <!-- 周期与执行动作同排；编辑项自动保存，同步前先写入最新配置。 -->
            <div class="sso-sync-row">
              <span class="sso-sync-label">{{ $t("sso_sync_period") }}</span>
              <a-select
                v-model:value="syncForm.auto_sync_period"
                :options="syncPeriodOptions"
                class="sso-field sso-field--period"
                @change="queueSyncSave"
              />
              <a-select
                v-if="syncForm.auto_sync_period === 'weekly'"
                v-model:value="syncForm.weekly_day"
                :options="weeklyDayOptions"
                :aria-label="$t('sso_sync_weekly_day')"
                class="sso-field sso-field--day"
                @change="queueSyncSave"
              />
              <div class="sso-sync-actions">
                <span v-if="syncSaveStatus" class="sso-tip" role="status">{{
                  $t(`sso_sync_autosave_${syncSaveStatus}`)
                }}</span>
                <a-button
                  type="primary"
                  :loading="syncRunning"
                  :disabled="syncRunning"
                  @click="runSyncNow"
                >
                  <template #icon><SyncOutlined /></template>
                  {{ $t("sso_sync_now") }}
                </a-button>
              </div>
            </div>
            <div class="sso-tip">{{ $t("sso_sync_enable_tip") }}</div>

            <!-- SCIM 端点与字段映射只对 OIDC/OAuth2.0 有意义；钉钉/飞书用各自通讯录同步 -->
            <a-form v-if="meta.support_scim" layout="vertical" class="sso-form">
              <a-form-item :label="$t('sso_scim_base_url')">
                <a-input
                  v-model:value="syncForm.scim_base_url"
                  :maxlength="500"
                  @change="queueSyncSave"
                />
                <div class="sso-tip">{{ $t("sso_scim_base_url_tip") }}</div>
              </a-form-item>

              <a-form-item :label="$t('sso_scim_token')">
                <a-input-password
                  v-model:value="syncForm.scim_token"
                  :maxlength="500"
                  class="sso-field"
                  autocomplete="off"
                  @change="queueSyncSave"
                />
                <div class="sso-tip">{{ $t("sso_scim_token_tip") }}</div>
              </a-form-item>

              <!-- 逐字段行编辑：左列本地字段，右列 IdP 取值路径；留空由后端回落平台默认映射 -->
              <div class="sso-tip sso-mapping-tip">
                {{ $t("sso_field_mapping_tip") }}
              </div>

              <div class="sso-mapping-group">
                <div class="sso-mapping-group-title">
                  {{ $t("sso_user_field_mapping") }}
                </div>
                <div
                  v-for="row in userMappingFields"
                  :key="row.key"
                  class="sso-mapping-row"
                >
                  <span class="sso-mapping-label">{{ $t(row.label_key) }}</span>
                  <a-input
                    v-model:value="syncForm.user_mapping[row.key]"
                    :maxlength="200"
                    class="sso-mapping-input"
                    @change="queueSyncSave"
                  />
                </div>
              </div>

              <div class="sso-mapping-group">
                <div class="sso-mapping-group-title">
                  {{ $t("sso_dept_field_mapping") }}
                </div>
                <div
                  v-for="row in deptMappingFields"
                  :key="row.key"
                  class="sso-mapping-row"
                >
                  <span class="sso-mapping-label">{{ $t(row.label_key) }}</span>
                  <a-input
                    v-model:value="syncForm.dept_mapping[row.key]"
                    :maxlength="200"
                    class="sso-mapping-input"
                    @change="queueSyncSave"
                  />
                </div>
              </div>
            </a-form>
            <div v-else class="sso-tip">{{ $t("sso_sync_vendor_note") }}</div>

            <div class="sso-sync-result">
              <span class="sso-sync-result-item">
                <span class="sso-sync-result-label"
                  >{{ $t("sso_sync_last_at") }}：</span
                >
                <span class="sso-sync-result-value">{{ lastSyncedLabel }}</span>
              </span>
              <a-tag v-if="syncStatus" :color="syncStatusColor">{{
                syncStatusLabel
              }}</a-tag>
              <span class="sso-sync-result-item">
                <span class="sso-sync-result-label"
                  >{{ $t("sso_sync_count") }}：</span
                >
                <span class="sso-sync-result-value">{{ syncLastCount }}</span>
              </span>
              <span v-if="syncFailedCount > 0" class="sso-sync-result-item">
                <span class="sso-sync-result-label"
                  >{{ $t("sso_sync_failed_count") }}：</span
                >
                <span class="sso-sync-result-value">{{ syncFailedCount }}</span>
              </span>
            </div>
            <div v-if="syncLastError" class="sso-tip sso-sync-error">
              {{ syncLastError }}
            </div>
          </div>
        </div>
      </template>
    </a-spin>

    <!-- 首次开启：选平台 + 填配置，测试通过才置为已开启 -->
    <a-modal
      v-model:open="enableOpen"
      :title="null"
      :width="640"
      :mask-closable="false"
      :keyboard="false"
      :body-style="{
        maxHeight: '68vh',
        overflowY: 'auto',
        paddingRight: '10px',
      }"
    >
      <SsoConfigForm
        v-model:form="dialogForm"
        :providers="providers"
        :step="dingtalkStep"
        :callback-url="dingtalkCallbackUrl"
        @provider-change="onDialogProviderChange"
        @copy-callback="copy(dingtalkCallbackUrl)"
      />

      <template #footer>
        <a-button :disabled="saving" @click="enableOpen = false">
          {{ $t("common_cancel") }}
        </a-button>
        <a-button
          v-if="dialogForm.provider === 'dingtalk' && dingtalkStep > 0"
          :disabled="saving || callbackLoading"
          @click="dingtalkStep--"
        >
          {{ $t("sso_dingtalk_previous") }}
        </a-button>
        <a-button
          v-if="dialogForm.provider === 'dingtalk' && dingtalkStep < 2"
          type="primary"
          :loading="callbackLoading"
          @click="nextDingtalkStep"
        >
          {{ $t("sso_dingtalk_next") }}
        </a-button>
        <a-button
          v-else
          type="primary"
          :loading="saving"
          :disabled="dialogForm.provider === 'dingtalk' && !dingtalkCallbackUrl"
          @click="saveEnable"
        >
          {{
            $t(
              dialogForm.provider === "dingtalk"
                ? "sso_dingtalk_finish"
                : "sso_enable_dialog_ok",
            )
          }}
        </a-button>
      </template>
    </a-modal>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { Modal, message } from "ant-design-vue";
import {
  CopyOutlined,
  EditOutlined,
  SyncOutlined,
} from "@ant-design/icons-vue";
import SsoConfigForm from "./SsoConfigForm.vue";
import {
  getSsoConfig,
  saveSsoConfig,
  testSsoConnection,
  saveSsoSyncConfig,
  runSsoSync,
} from "@/api/sso";
import { writeClipboardText } from "@/utils/workItemClipboard";
import { formatLocalDate } from "@/utils/date";

const { t } = useI18n();

// 字段映射的本地字段清单与默认取值路径，与后端 ssoScimUserDefaults / ssoScimDeptDefaults 一致（SCIM 标准字段）。
// 界面预填默认值，管理员按自己 IdP 的实际字段名修改即可；后端不支持的本地字段（如部门头像）没有映射键，不出现在界面上。
const userMappingFields = [
  { key: "external_id", label_key: "sso_map_user_id", default: "id" },
  { key: "username", label_key: "sso_map_username", default: "userName" },
  { key: "name", label_key: "sso_map_nickname", default: "displayName" },
  {
    key: "phone",
    label_key: "sso_map_phone",
    default: "phoneNumbers[0].value",
  },
  { key: "avatar", label_key: "sso_map_avatar", default: "photos[0].value" },
  { key: "email", label_key: "sso_map_email", default: "emails[0].value" },
  { key: "job_title", label_key: "sso_map_job_title", default: "title" },
  {
    key: "employee_no",
    label_key: "sso_map_employee_no",
    default: "employeeNumber",
  },
  {
    key: "department",
    label_key: "sso_map_department",
    default: "memberships[*].value",
  },
];

// parent_external_id 后端默认为空：SCIM 组没有标准的上级字段，需按 IdP 实际字段填写。
const deptMappingFields = [
  { key: "external_id", label_key: "sso_map_dept_id", default: "id" },
  { key: "name", label_key: "sso_map_dept_name", default: "displayName" },
  {
    key: "parent_external_id",
    label_key: "sso_map_dept_parent_id",
    default: "",
  },
];

function emptyMapping(fields) {
  return Object.fromEntries(
    fields.map((field) => [field.key, field.default || ""]),
  );
}

const loading = ref(false);
const saving = ref(false);
const syncSaveStatus = ref("");
const syncRunning = ref(false);
let syncSaveTimer;
let syncSavePromise;
let syncSaveQueued = false;
const providers = ref([]);
// config 是后端已保存的投影；form 是编辑中的副本。两者分开才能判断「密钥有没有被编辑」。
const config = ref(null);
const form = ref(emptyForm());
const syncForm = ref(emptySyncForm());

// 首次开启弹窗：表单放独立副本，取消后不污染页面上已保存的内容。
const enableOpen = ref(false);
const dialogForm = ref(emptyForm());
const dingtalkStep = ref(0);
const dingtalkCallbackUrl = ref("");
const callbackLoading = ref(false);

function emptyForm() {
  return {
    enabled: false,
    provider: "",
    app_key: "",
    app_secret: "",
    agent_id: "",
    issuer_url: "",
    authorize_url: "",
    token_url: "",
    userinfo_url: "",
    scopes: "",
    external_id_field: "",
  };
}

function emptySyncForm() {
  return {
    auto_sync_period: "none",
    weekly_day: 1,
    scim_base_url: "",
    scim_token: "",
    user_mapping: emptyMapping(userMappingFields),
    dept_mapping: emptyMapping(deptMappingFields),
  };
}

// 页面形态读已保存的开启状态，不读编辑中的 form.enabled：卡片显隐反映的是服务端事实。
const savedEnabled = computed(() => Boolean(config.value?.enabled));

const providerOptions = computed(() =>
  providers.value.map((item) => ({ value: item.key, label: item.label })),
);

// 状态行里的已开启平台名。provider_meta 未加载完或缺失该平台时回退平台 key，
// 避免渲染出「已开启 · 」这种断句。
const currentProviderLabel = computed(
  () =>
    providers.value.find((item) => item.key === config.value?.provider)
      ?.label ||
    config.value?.provider ||
    "",
);

const meta = computed(
  () =>
    providers.value.find((item) => item.key === form.value.provider) || {
      app_key_label: t("sso_field_app_key"),
      app_secret_label: t("sso_field_app_secret"),
    },
);

// 平台默认授权域来自后端 provider_meta.scope_default，与各平台运行时的兜底常量同源，前端不另抄一份。
function defaultScopesFor(providerKey) {
  return (
    providers.value.find((item) => item.key === providerKey)?.scope_default ||
    ""
  );
}

// 可选周期由后端按环境下发（测试环境比生产多一个「每 5 分钟」），文案也已翻译好；
// 接口异常时回落到线上四项目，避免下拉整个空掉。
const syncPeriodOptions = computed(() => {
  const options = config.value?.sync?.period_options;
  if (Array.isArray(options) && options.length) {
    return options;
  }
  return [
    { value: "none", label: t("sso_sync_period_none") },
    { value: "hourly", label: t("sso_sync_period_hourly") },
    { value: "daily", label: t("sso_sync_period_daily") },
    { value: "weekly", label: t("sso_sync_period_weekly") },
  ];
});

const weeklyDayOptions = computed(() =>
  [1, 2, 3, 4, 5, 6, 7].map((day) => ({
    value: day,
    label: t(`sso_sync_weekday_${day}`),
  })),
);

// 同步结果展示都读 config.sync（后端已保存投影 + running），不读编辑中的 syncForm。
const syncStatus = computed(() => config.value?.sync?.last_sync_status || "");
const syncStatusLabel = computed(() => {
  switch (syncStatus.value) {
    case "success":
      return t("sso_sync_status_success");
    case "partial":
      return t("sso_sync_status_partial");
    case "failed":
      return t("sso_sync_status_failed");
    default:
      return "";
  }
});
const syncStatusColor = computed(() => {
  if (syncStatus.value === "success") return "green";
  if (syncStatus.value === "partial") return "orange";
  return "red";
});
const syncLastCount = computed(
  () => config.value?.sync?.last_synced_count ?? 0,
);
const syncFailedCount = computed(
  () => config.value?.sync?.last_failed_count ?? 0,
);
const syncLastError = computed(() => config.value?.sync?.last_error || "");
const lastSyncedLabel = computed(() => {
  const at = config.value?.sync?.last_synced_at ?? 0;
  return at ? formatLocalDate(at, true) : t("sso_sync_never");
});

onMounted(fetch);
onUnmounted(() => {
  if (syncSaveQueued) void flushSyncSave();
});

async function fetch() {
  loading.value = true;
  try {
    const res = await getSsoConfig();
    applyConfig(res);
  } catch {
    // GET 的失败（网络错误等）拦截器已经提示过，这里只负责收起 loading。
  } finally {
    loading.value = false;
  }
}

function applyConfig(res) {
  config.value = res;
  if (res?.provider_meta?.length) providers.value = res.provider_meta;
  form.value = {
    enabled: Boolean(res?.enabled),
    provider: res?.provider || providerOptions.value[0]?.value || "",
    app_key: res?.app_key || "",
    app_secret: res?.app_secret || "",
    agent_id: res?.agent_id || "",
    issuer_url: res?.issuer_url || "",
    authorize_url: res?.authorize_url || "",
    token_url: res?.token_url || "",
    userinfo_url: res?.userinfo_url || "",
    // 库里没存过 scope 时按平台默认授权域预填，管理员看得见实际生效值（与后端兜底同源）。
    scopes: res?.scopes || defaultScopesFor(res?.provider),
    external_id_field: res?.external_id_field || "",
  };
  applySyncConfig(res?.sync);
}

// 同步段回填表单。字段映射后端回的是 JSON 对象，按固定字段清单投影到各行；scim_token 明文回显（眼睛按钮查看原值）。
function applySyncConfig(sync) {
  syncSaveStatus.value = "";
  if (!sync) {
    syncForm.value = emptySyncForm();
    syncRunning.value = false;
    return;
  }
  syncForm.value = {
    auto_sync_period: sync.auto_sync_period || "none",
    weekly_day: sync.weekly_day || 1,
    scim_base_url: sync.scim_base_url || "",
    scim_token: sync.scim_token || "",
    user_mapping: pickMapping(sync.user_field_mapping, userMappingFields),
    dept_mapping: pickMapping(sync.dept_field_mapping, deptMappingFields),
  };
  syncRunning.value = Boolean(sync.running);
}

function pickMapping(value, fields) {
  const source =
    value && typeof value === "object" && !Array.isArray(value) ? value : {};
  return Object.fromEntries(
    fields.map((field) => [
      field.key,
      typeof source[field.key] === "string" && source[field.key].trim() !== ""
        ? source[field.key]
        : field.default || "",
    ]),
  );
}

// 开关是「先落配置再变开启态」：打开只负责拉起弹窗，关掉要先确认再写库。
function onToggle(checked) {
  if (checked) {
    openEnableDialog();
    return;
  }
  Modal.confirm({
    title: t("sso_disable_confirm_title"),
    content: t("sso_disable_confirm_content"),
    okText: t("common_confirm"),
    cancelText: t("common_cancel"),
    onOk: disableSso,
  });
}

function openEnableDialog() {
  dialogForm.value = { ...form.value, enabled: true };
  dingtalkStep.value = 0;
  dingtalkCallbackUrl.value = "";
  enableOpen.value = true;
}

async function nextDingtalkStep() {
  if (dingtalkStep.value === 0) {
    dingtalkStep.value = 1;
    return;
  }
  if (
    !dialogForm.value.app_key?.trim() ||
    !dialogForm.value.app_secret?.trim()
  ) {
    message.error(t("sso_test_fail_config_invalid"));
    return;
  }
  callbackLoading.value = true;
  try {
    const preview = await getSsoConfig("dingtalk");
    if (
      !enableOpen.value ||
      dialogForm.value.provider !== "dingtalk" ||
      dingtalkStep.value !== 1
    )
      return;
    dingtalkCallbackUrl.value = preview?.login_callback_url || "";
    dingtalkStep.value = 2;
  } catch {
    // The request interceptor shows the fetch error; keep the credentials step open.
  } finally {
    callbackLoading.value = false;
  }
}

// 平台一旦保存过，切换会清空该租户的全部身份绑定，所以确认放在保存动作上（见 confirmProviderSwitchOnSave）；
// 改表单本身不弹，切换后即时把表单重置成新平台。
//
// 切换平台必须清空全部平台专属字段（应用标识、密钥、端点、Scope、唯一标识字段）：
// 各平台的凭证与端点互不通用，留着上一平台的值，标签一换就会被当成新平台的配置展示与提交。
// Scope 直接预填新平台的默认授权域，管理员不必知道每个平台该填什么。
function onDialogProviderChange(next) {
  dingtalkStep.value = 0;
  dingtalkCallbackUrl.value = "";
  dialogForm.value = {
    ...emptyForm(),
    enabled: dialogForm.value.enabled,
    provider: next,
    scopes: defaultScopesFor(next),
  };
}

// 切换登录平台的确认放在保存动作上：后端在 provider 变化时会清空该租户的全部身份绑定，
// 改表单不弹、落库前弹一次；取消即整体不保存，留在弹窗里继续编辑。
// 判据取「已保存的 provider」与表单值对比：从未保存过配置时 provider 为空，没有绑定可清，不弹。
// 已知未覆盖：app_key 变化同样会清空绑定（后端判据是 provider 或 app_key），此处不弹确认。
function confirmProviderSwitchOnSave() {
  const saved = config.value?.provider || "";
  const next = dialogForm.value.provider;
  if (!saved || saved === next) return Promise.resolve(true);
  return new Promise((resolve) => {
    Modal.confirm({
      title: t("sso_provider_switch_title"),
      content: t("sso_provider_switch_content"),
      okText: t("common_confirm"),
      cancelText: t("common_cancel"),
      onOk: () => resolve(true),
      onCancel: () => resolve(false),
    });
  });
}

// buildPayload 从编辑中的表单构造提交体。密钥已明文回显在输入框（眼睛按钮），
// 提交时原样带上——清空即表示删除密钥（后端空串=清空）。保存与连接测试都走这一份，避免两处逻辑漂移。
function buildPayload(source) {
  return { ...source };
}

// 开启与改配置共用「测试并保存」：先做一次连接预检，通过才把配置连同 enabled=1 一起落库，
// 不通过就留在弹窗里让管理员改配置。后端在 enabled=1 时同样会拒收缺必需凭证的请求，前端不预判必填项。
async function saveEnable() {
  if (!(await confirmProviderSwitchOnSave())) return;
  saving.value = true;
  try {
    const payload = buildPayload(dialogForm.value);
    const tested = await testSsoConnection(payload);
    if (!tested?.ok) {
      message.error(t(testFailKey(tested?.reason)));
      return;
    }
    const res = await saveSsoConfig({ ...payload, enabled: true });
    applyConfig(res);
    enableOpen.value = false;
    message.success(t("sso_enabled_success"));
  } catch (err) {
    message.error(err?.message || t("common_requestFailed"));
  } finally {
    saving.value = false;
  }
}

// 关闭只改 enabled，但必须按当前表单整份回传：后端合并对非密钥字段是「空串即覆盖」，
// 少传一个字段就会把已保存的配置清空。
async function disableSso() {
  saving.value = true;
  try {
    const res = await saveSsoConfig({
      ...buildPayload(form.value),
      enabled: false,
    });
    applyConfig(res);
    message.success(t("sso_disabled_success"));
  } catch (err) {
    message.error(err?.message || t("common_requestFailed"));
  } finally {
    saving.value = false;
  }
}

// testFailKey 把后端归一原因映射到提示文案；未知原因按通用失败处理，不猜语义。
function testFailKey(reason) {
  switch (reason) {
    case "sso_config_invalid":
      return "sso_test_fail_config_invalid";
    case "sso_endpoint_unreachable":
      return "sso_test_fail_endpoint";
    case "sso_auth_failed":
      return "sso_test_fail_auth";
    default:
      return "sso_test_fail_server";
  }
}

// buildSyncPayload 从编辑中的同步表单构造提交体。字段映射只提交非空行（后端缺失键回落默认值）；
// scim_token 已明文回显在输入框，提交时原样带上——清空即表示删除密钥（后端空串=清空）。
function buildSyncPayload() {
  return {
    enabled: true,
    auto_sync_period: syncForm.value.auto_sync_period,
    weekly_day: syncForm.value.weekly_day,
    scim_base_url: syncForm.value.scim_base_url,
    scim_token: syncForm.value.scim_token,
    user_field_mapping: toMappingJson(syncForm.value.user_mapping),
    dept_field_mapping: toMappingJson(syncForm.value.dept_mapping),
  };
}

function toMappingJson(mapping) {
  const entries = Object.entries(mapping || {}).filter(
    ([, path]) => typeof path === "string" && path.trim() !== "",
  );
  if (!entries.length) return "";
  return JSON.stringify(
    Object.fromEntries(entries.map(([key, path]) => [key, path.trim()])),
  );
}

function queueSyncSave() {
  syncSaveQueued = true;
  syncSaveStatus.value = "saving";
  clearTimeout(syncSaveTimer);
  syncSaveTimer = setTimeout(() => void flushSyncSave(), 500);
}

async function flushSyncSave(force = false) {
  clearTimeout(syncSaveTimer);
  if (force) syncSaveQueued = true;
  if (syncSavePromise) return syncSavePromise;
  if (!syncSaveQueued) return syncSaveStatus.value !== "failed";
  syncSaveStatus.value = "saving";
  syncSavePromise = (async () => {
    while (syncSaveQueued) {
      syncSaveQueued = false;
      try {
        await saveSsoSyncConfig(buildSyncPayload());
      } catch (err) {
        message.error(err?.message || t("common_requestFailed"));
        return false;
      }
    }
    return true;
  })();
  const saved = await syncSavePromise;
  syncSavePromise = null;
  syncSaveStatus.value = saved ? "saved" : "failed";
  if (syncSaveQueued) return flushSyncSave();
  return saved;
}

// 手动同步前先写入编辑中的配置；尚未保存过的租户也需要先创建调度任务行。
async function runSyncNow() {
  if (syncRunning.value) return;
  syncRunning.value = true;
  try {
    if (!(await flushSyncSave(true))) return;
    reportSyncResult(await runSsoSync());
  } catch (err) {
    message.error(err?.message || t("common_requestFailed"));
  } finally {
    syncRunning.value = false;
    await refreshSyncResult();
  }
}

// 结果提示按本次执行的状态分档：接口回的就是刚写进库的 last_sync_*，与结果区同源。
// 执行失败只给一句可读提示，具体原因留给结果区展示。
function reportSyncResult(res) {
  if (!res?.done) {
    message.info(t("sso_sync_running"));
    return;
  }
  if (res.status === "failed") {
    message.error(t("sso_sync_failed_tip"));
    return;
  }
  if (res.failed_count > 0) {
    message.warning(
      t("sso_sync_done_partial", {
        count: res.synced_count,
        failed: res.failed_count,
      }),
    );
    return;
  }
  message.success(t("sso_sync_done_count", { count: res.synced_count }));
}

// 只重读整份配置以刷新同步结果：不调 applyConfig，避免覆盖正在编辑的同步表单。
async function refreshSyncResult() {
  try {
    config.value = await getSsoConfig();
  } catch {
    // 结果刷新失败不额外提示：本次同步已经结束，页面可手动刷新。
  }
}

async function copy(value) {
  if (!value) return;
  try {
    await writeClipboardText(value);
    message.success(t("common_copied"));
  } catch {
    message.error(t("sso_copy_failed"));
  }
}
</script>

<style scoped lang="less">
@import "@/styles/variables.less";

// tabpane 定高（MemberPage 的 ant-tabs 链），内容超高时由本组件负责滚动，
// 否则同步结果等底部内容会被裁切且页面无法滚动。
.sso-tab {
  height: 100%;
  overflow-y: auto;
  padding: 16px 0;
}

.sso-card {
  margin-bottom: 24px;
  padding: 20px 24px;
  background: #fff;
  border: 1px solid #f0f0f0;
  border-radius: 8px;
}

// 开关卡：上半是标题与开关，下半（已开启时）平铺系统地址，中间用分隔线断开
.sso-card--switch {
  padding: 0;
  overflow: hidden;
}

.sso-switch-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 20px 24px;
}

// 开关右侧的「配置」按钮与开关同排，按钮在开关左侧
.sso-switch-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.sso-switch-title {
  font-size: 16px;
  font-weight: 600;
}

.sso-switch-status {
  margin-top: 8px;
  color: rgba(0, 0, 0, 0.45);
  font-size: 13px;
  line-height: 20px;
}

// 状态行里的已开启平台名：用主色强调，与前面的状态词区分
.sso-provider-name {
  color: @primary-color;
}

// 功能简介：标题带浅灰底，与正文区分开
.sso-card--intro {
  padding: 0;
  overflow: hidden;
}

.sso-intro-title {
  padding: 20px 24px;
  background: #fafafa;
  border-bottom: 1px solid #f0f0f0;
  font-size: 15px;
  font-weight: 600;
}

.sso-intro-body {
  padding: 20px 24px;
}

.sso-intro-text {
  margin: 0;
  color: rgba(0, 0, 0, 0.45);
  font-size: 13px;
  line-height: 22px;
}

.sso-intro-action {
  margin-top: 16px;
}

// 同步卡：灰底标题栏放标题与开关，正文放周期、SCIM 字段与同步结果
.sso-card--sync {
  padding: 0;
  overflow: hidden;
}

.sso-sync-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 20px 24px;
  background: #fafafa;
  border-bottom: 1px solid #f0f0f0;
}

.sso-sync-title {
  font-size: 15px;
  font-weight: 600;
}

.sso-sync-body {
  padding: 20px 24px;
}

// 周期与执行动作同排，动作靠右
.sso-sync-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.sso-sync-label {
  color: rgba(0, 0, 0, 0.65);
  font-size: 13px;
}

.sso-sync-actions {
  display: flex;
  gap: 12px;
  margin-left: auto;
}

// 同步结果行：标签浅、数值深，项之间拉开距离
.sso-sync-result {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0 24px;
  margin-top: 16px;
  font-size: 13px;
}

.sso-sync-result-label {
  color: rgba(0, 0, 0, 0.45);
}

.sso-sync-result-value {
  color: rgba(0, 0, 0, 0.85);
}

.sso-form {
  margin-top: 16px;
}

.sso-field {
  max-width: 420px;
}

.sso-field--period {
  width: 190px;
}

.sso-field--day {
  width: 130px;
}

.sso-tip {
  margin-top: 4px;
  color: rgba(0, 0, 0, 0.45);
  font-size: 12px;
  line-height: 20px;
}

// 字段映射：左侧字段说明固定宽度，右侧单个输入框
.sso-mapping-tip {
  margin-bottom: 12px;
}

.sso-mapping-group {
  margin-bottom: 16px;
}

.sso-mapping-group-title {
  margin-bottom: 8px;
  color: rgba(0, 0, 0, 0.85);
  font-size: 13px;
  font-weight: 500;
}

.sso-mapping-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.sso-mapping-label {
  flex: none;
  width: 160px;
  color: rgba(0, 0, 0, 0.65);
  font-size: 13px;
}

.sso-mapping-input {
  min-width: 0;
  flex: 1;
  max-width: 420px;
}

// 系统地址区：标签在上，只读输入框与复制按钮同排
.sso-url-section {
  padding: 20px 24px;
  border-top: 1px solid #f0f0f0;
}

.sso-url-row {
  margin-bottom: 16px;
}

.sso-url-label {
  margin-bottom: 8px;
  color: rgba(0, 0, 0, 0.45);
  font-size: 13px;
}

.sso-url-field {
  display: flex;
  gap: 8px;
}

.sso-url-field :deep(.ant-input[readonly]) {
  background: #f5f5f5;
}
</style>

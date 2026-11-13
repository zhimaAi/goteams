<!--
 * SsoConfigForm.vue
 * SSO 登录配置表单：登录平台选择与该平台的凭证字段。
 * 「已开启后的配置卡片」与「首次开启的引导弹窗」复用同一份表单，避免两处字段漂移。
 * 平台清单与字段形态由后端 provider_meta 下发；钉钉配置按后台操作分步引导。
 -->
<template>
  <a-form layout="vertical" class="sso-form">
    <a-form-item :label="$t('sso_provider')" required>
      <a-segmented
        :value="form.provider"
        :options="providerOptions"
        @change="emit('provider-change', $event)"
      />
    </a-form-item>

    <a-steps v-if="isDingtalk" :current="step" size="small" class="sso-steps">
      <a-step :title="$t('sso_dingtalk_step_permissions')" />
      <a-step :title="$t('sso_dingtalk_step_credentials')" />
      <a-step :title="$t('sso_dingtalk_step_callback')" />
    </a-steps>

    <!-- 权限清单紧随平台选择：管理员选完平台就要去第三方后台开权限，再回来填凭证。
         表格列与厂商后台一致（名称 + 权限标识 + 备注），管理员是拿报错里的 code 回后台搜的。 -->
    <div
      v-if="meta.permissions.length && (!isDingtalk || step === 0)"
      class="sso-perm"
    >
      <div class="sso-perm-title">
        {{ $t("sso_perm_required_title") }}
        <a
          v-if="isDingtalk"
          :href="dingtalkConsoleUrl"
          target="_blank"
          rel="noopener noreferrer"
          >{{ $t("sso_dingtalk_console") }}</a
        >
      </div>
      <table class="sso-perm-table">
        <thead>
          <tr>
            <th>{{ $t("sso_perm_col_name") }}</th>
            <th v-if="showPermType">{{ $t("sso_perm_col_type") }}</th>
            <th>{{ $t("sso_perm_col_remark") }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in meta.permissions" :key="item.name">
            <td>
              <div>{{ item.name }}</div>
              <!-- 钉钉的两条字段权限没有公开的权限点 code，只有中文名 -->
              <span v-if="item.code" class="sso-perm-code">{{
                item.code
              }}</span>
            </td>
            <td v-if="showPermType">{{ item.type }}</td>
            <td class="sso-perm-remark">{{ item.remark }}</td>
          </tr>
        </tbody>
      </table>
      <!-- 数据范围是权限表之外的配置项，逐行重复说不清，单独一行提示 -->
      <div v-if="meta.permission_scope" class="sso-perm-scope">
        {{ meta.permission_scope }}
      </div>
      <button
        v-if="isDingtalk"
        type="button"
        class="sso-step-image"
        :aria-label="
          $t('sso_dingtalk_view_image', {
            step: $t('sso_dingtalk_step_permissions'),
          })
        "
        @click="previewStepImage(dingtalkStepOne)"
      >
        <img
          :src="dingtalkStepOne"
          :alt="$t('sso_dingtalk_step_permissions')"
        />
      </button>
    </div>

    <template v-if="!isDingtalk || step === 1">
      <div v-if="isDingtalk" class="sso-step-heading">
        {{ $t("sso_dingtalk_credentials_tip") }}
        <a
          :href="dingtalkConsoleUrl"
          target="_blank"
          rel="noopener noreferrer"
          >{{ $t("sso_dingtalk_console") }}</a
        >
      </div>
      <a-form-item :label="meta.app_key_label" required>
        <a-input v-model:value="form.app_key" :maxlength="200" />
      </a-form-item>

      <a-form-item :label="meta.app_secret_label" required>
        <a-input-password
          v-model:value="form.app_secret"
          :maxlength="500"
          autocomplete="off"
        />
        <div class="sso-tip">{{ $t("sso_client_secret_tip") }}</div>
      </a-form-item>

      <a-form-item
        v-if="meta.need_agent_id"
        :label="$t('sso_agent_id')"
        required
      >
        <a-input v-model:value="form.agent_id" :maxlength="100" />
      </a-form-item>

      <a-form-item
        v-if="form.provider === 'oidc'"
        :label="$t('sso_issuer_url')"
        required
      >
        <a-input v-model:value="form.issuer_url" :maxlength="500" />
        <div class="sso-tip">{{ $t("sso_issuer_url_tip") }}</div>
      </a-form-item>

      <template v-if="meta.need_endpoints">
        <a-form-item :label="$t('sso_authorize_url')" required>
          <a-input v-model:value="form.authorize_url" :maxlength="500" />
        </a-form-item>
        <a-form-item :label="$t('sso_token_url')" required>
          <a-input v-model:value="form.token_url" :maxlength="500" />
        </a-form-item>
        <a-form-item :label="$t('sso_userinfo_url')" required>
          <a-input v-model:value="form.userinfo_url" :maxlength="500" />
        </a-form-item>
        <a-form-item :label="$t('sso_external_id_field')" required>
          <a-input v-model:value="form.external_id_field" :maxlength="100" />
          <div class="sso-tip">{{ $t("sso_external_id_field_tip") }}</div>
        </a-form-item>
      </template>

      <a-form-item
        v-if="meta.need_endpoints || form.provider === 'oidc'"
        :label="$t('sso_scopes')"
        required
      >
        <a-input v-model:value="form.scopes" :maxlength="255" />
        <div class="sso-tip">{{ $t("sso_scopes_tip") }}</div>
      </a-form-item>
      <button
        v-if="isDingtalk"
        type="button"
        class="sso-step-image"
        :aria-label="
          $t('sso_dingtalk_view_image', {
            step: $t('sso_dingtalk_step_credentials'),
          })
        "
        @click="previewStepImage(dingtalkStepTwo)"
      >
        <img
          :src="dingtalkStepTwo"
          :alt="$t('sso_dingtalk_step_credentials')"
        />
      </button>
    </template>

    <div v-if="isDingtalk && step === 2" class="sso-callback-step">
      <div class="sso-step-heading">
        {{ $t("sso_dingtalk_callback_tip") }}
        <a
          :href="dingtalkConsoleUrl"
          target="_blank"
          rel="noopener noreferrer"
          >{{ $t("sso_dingtalk_console") }}</a
        >
      </div>
      <a-form-item :label="$t('sso_callback_url')">
        <div class="sso-callback-field">
          <a-input :value="callbackUrl || $t('sso_url_unavailable')" readonly />
          <a-button :disabled="!callbackUrl" @click="emit('copy-callback')">
            {{ $t("common_copy") }}
          </a-button>
        </div>
      </a-form-item>
      <button
        type="button"
        class="sso-step-image"
        :aria-label="
          $t('sso_dingtalk_view_image', {
            step: $t('sso_dingtalk_step_callback'),
          })
        "
        @click="previewStepImage(dingtalkStepThree)"
      >
        <img :src="dingtalkStepThree" :alt="$t('sso_dingtalk_step_callback')" />
      </button>
    </div>
  </a-form>
</template>

<script setup>
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { openImageViewer } from "@/utils/imageViewer";
import dingtalkStepOne from "@/image/sso/1.png";
import dingtalkStepTwo from "@/image/sso/2.png";
import dingtalkStepThree from "@/image/sso/3.png";

const props = defineProps({
  // providers 是后端下发的平台元数据清单（provider_meta）
  providers: { type: Array, default: () => [] },
  step: { type: Number, default: 0 },
  callbackUrl: { type: String, default: "" },
});

const emit = defineEmits(["provider-change", "copy-callback"]);

const form = defineModel("form", { type: Object, required: true });

const { t } = useI18n();
const isDingtalk = computed(() => form.value.provider === "dingtalk");
const dingtalkConsoleUrl =
  "https://open-dev.dingtalk.com/fe/app?hash=%23%2Fcorp%2Fapp#/corp/app";

function previewStepImage(source) {
  openImageViewer({ sources: [source] });
}

const providerOptions = computed(() =>
  props.providers.map((item) => ({ value: item.key, label: item.label })),
);

// 未匹配到平台元数据时退回通用标签，避免切换过程中标签闪成空白。
// permissions 两条路径都必须落到数组：模板里按数组渲染（`meta.permissions.length`），
// 一旦是 undefined，抛错发生在渲染阶段，整份表单会停在切换前的那一帧且后续点击都不再响应。
const meta = computed(() => {
  const matched = props.providers.find(
    (item) => item.key === form.value.provider,
  );
  if (!matched) {
    return {
      app_key_label: t("sso_field_app_key"),
      app_secret_label: t("sso_field_app_secret"),
      permissions: [],
    };
  }
  return { ...matched, permissions: matched.permissions || [] };
});

// 钉钉后台的权限表是「权限信息 / 接口 / 权限点code」，没有「权限类型」这一列。
// 该平台整列都为空时收起，不留一列空白。
const showPermType = computed(() =>
  meta.value.permissions.some((item) => item.type),
);
</script>

<style scoped lang="less">
@import "@/styles/variables.less";

// 平台选择承担 tab 语义：选中项用主色实底 + 白字，覆盖 antd 默认的白色滑块。
// thumb 是切换动画期间的滑块，必须与选中项同色，否则动画过程会闪出白底。
.sso-form :deep(.ant-segmented-item-selected),
.sso-form :deep(.ant-segmented-thumb) {
  background-color: @primary-color;
  box-shadow: none;
}

.sso-form :deep(.ant-segmented-item-selected) {
  color: #fff;
}

.sso-tip {
  margin-top: 4px;
  color: rgba(0, 0, 0, 0.45);
  font-size: 12px;
  line-height: 20px;
}

.sso-steps {
  margin-bottom: 16px;
}

.sso-step-heading {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
  font-size: 13px;
  font-weight: 500;
}

.sso-step-heading a,
.sso-perm-title a {
  font-weight: 400;
  white-space: nowrap;
}

.sso-step-image {
  display: block;
  width: 100%;
  margin-top: 16px;
  padding: 0;
  border: 1px solid #f0f0f0;
  background: transparent;
  cursor: zoom-in;

  img {
    display: block;
    width: 100%;
    height: auto;
  }
}

.sso-callback-field {
  display: flex;
  gap: 8px;
}

// 权限表：与平台选择拉开距离，字号与 .sso-tip 一致
.sso-perm {
  margin-bottom: 24px;
  padding: 12px;
  border: 1px solid #f0f0f0;
  border-radius: 6px;
}

.sso-perm-title {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
  font-size: 13px;
  font-weight: 500;
}

.sso-perm-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
  line-height: 20px;
  /* 权限标识是不含空格的 ASCII 长串（如 qyapi_get_department_member），
     窄弹窗下必须允许词内断行，否则会把弹窗撑宽 */
  overflow-wrap: break-word;

  th,
  td {
    padding: 6px 8px;
    text-align: left;
    vertical-align: top;
    border-bottom: 1px solid #f0f0f0;
  }

  th {
    color: rgba(0, 0, 0, 0.45);
    font-weight: 400;
    background-color: #fafafa;
  }

  // 名称列定宽，备注列才有稳定的可读宽度；否则长 code 会把名称列撑到占满整行。
  th:first-child {
    width: 46%;
  }

  tr:last-child td {
    border-bottom: none;
  }
}

.sso-perm-code {
  color: rgba(0, 0, 0, 0.45);
}

.sso-perm-remark {
  color: rgba(0, 0, 0, 0.65);
}

.sso-perm-scope {
  margin-top: 8px;
  color: rgba(0, 0, 0, 0.45);
  font-size: 12px;
  line-height: 20px;
}
</style>

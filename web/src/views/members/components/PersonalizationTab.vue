<template>
  <a-spin :spinning="loading" wrapper-class-name="personalization-scroll">
    <div class="personalization-panel">
      <a-alert
        :type="editable ? 'success' : 'info'"
        show-icon
        :message="
          $t(
            editable
              ? 'personalization_unlocked'
              : 'personalization_enterprise_only',
          )
        "
        :description="$t('personalization_description')"
      >
        <template v-if="!editable" #action>
          <a-button type="primary" @click="$emit('upgrade')">{{
            $t("personalization_upgrade")
          }}</a-button>
        </template>
      </a-alert>
      <a-form
        ref="formRef"
        :model="form"
        :rules="rules"
        :label-col="{ style: { width: '140px' } }"
        :disabled="!editable || loading || saving"
        class="personalization-form"
        @finish="save"
      >
        <a-form-item :label="$t('personalization_logo')">
          <div class="logo-row">
            <a-upload
              accept="image/png,image/jpeg"
              :show-upload-list="false"
              :before-upload="uploadLogo"
              :disabled="!editable || saving || uploading"
            >
              <a-button
                class="logo-upload"
                :loading="uploading"
                :aria-label="$t('personalization_logo')"
              >
                <img :src="logoURL || defaultLogo" alt="" />
              </a-button>
            </a-upload>
            <div class="field-tip">{{ $t("personalization_logo_tip") }}</div>
          </div>
        </a-form-item>
        <a-form-item name="app_name" :label="$t('personalization_app_name')">
          <a-input
            v-model:value="form.app_name"
            :maxlength="100"
            placeholder="GoTeams"
          />
        </a-form-item>
        <a-form-item name="domain_prefix" :label="$t('personalization_domain')">
          <a-input
            v-model:value="form.domain_prefix"
            :addon-before="domainScheme"
            :addon-after="domainSuffix"
            :maxlength="63"
            :disabled="!editable || !domainSuffix || saving"
            placeholder="your-company"
            class="domain-input"
          />
          <div class="field-tip">
            {{
              $t(
                domainSuffix
                  ? "personalization_domain_tip"
                  : "personalization_domain_unavailable",
              )
            }}
          </div>
          <a
            v-if="accessURL"
            :href="accessURL"
            target="_blank"
            rel="noopener noreferrer"
            >{{ accessURL }}</a
          >
        </a-form-item>
        <a-form-item :label="$t('personalization_slogan')">
          <RichTextEditor
            :key="editorRevision"
            v-model="form.login_slogan"
            :read-only="!editable || saving"
            :default-editing="true"
            :plain="true"
            :min-height="160"
            :allow-uploads="false"
            :allow-markdown="false"
            :placeholder="$t('personalization_slogan_placeholder')"
          />
        </a-form-item>
        <div class="personalization-actions">
          <a-button
            :disabled="!editable || saving || uploading"
            @click="restoreDefaults"
            >{{ $t("personalization_restore") }}</a-button
          >
          <a-button
            type="primary"
            html-type="submit"
            :loading="saving"
            :disabled="!editable || uploading || loading"
            >{{ $t("personalization_save") }}</a-button
          >
        </div>
      </a-form>
    </div>
  </a-spin>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from "vue";
import { message } from "ant-design-vue";
import { useI18n } from "vue-i18n";
import RichTextEditor from "@/components/common/RichTextEditor.vue";
import defaultLogo from "@/image/logo.svg";
import {
  getPersonalization,
  savePersonalization,
  uploadPersonalizationLogo,
} from "@/api/personalization";
import { useBrandingStore } from "@/stores/branding";

defineEmits(["upgrade"]);
const { t } = useI18n();
const brandingStore = useBrandingStore();
const formRef = ref();
const loading = ref(true);
const saving = ref(false);
const uploading = ref(false);
const editable = ref(false);
const logoURL = ref("");
const domainSuffix = ref("");
const domainScheme = ref("");
const accessURL = ref("");
const editorRevision = ref(0);
const form = reactive({
  app_name: "",
  logo: "",
  domain_prefix: "",
  login_slogan: "",
});
const rules = computed(() => ({
  domain_prefix: [
    {
      validator: (_, value) =>
        !value || /^[a-z0-9][a-z0-9-]{5,61}[a-z0-9]$/.test(value)
          ? Promise.resolve()
          : Promise.reject(t("personalization_domain_tip")),
      trigger: "blur",
    },
  ],
}));

function apply(data) {
  form.app_name = data.app_name;
  form.logo = data.logo;
  form.domain_prefix = data.domain_prefix;
  form.login_slogan = data.login_slogan;
  logoURL.value = data.logo_url;
  domainSuffix.value = data.domain_suffix;
  domainScheme.value = data.domain_scheme;
  accessURL.value = data.access_url;
  editable.value = data.editable;
  editorRevision.value += 1;
}

onMounted(async () => {
  try {
    apply(await getPersonalization());
  } catch {
    /* The API interceptor reports the failure; editing stays disabled. */
  } finally {
    loading.value = false;
  }
});

async function uploadLogo(file) {
  if (
    !["image/png", "image/jpeg"].includes(file.type) ||
    file.size > 2 * 1024 * 1024
  ) {
    message.error(t("personalization_logo_tip"));
    return false;
  }
  uploading.value = true;
  try {
    const result = await uploadPersonalizationLogo(file);
    form.logo = result.logo;
    logoURL.value = result.logo_url;
  } catch {
    /* The API interceptor reports upload failures. */
  } finally {
    uploading.value = false;
  }
  return false;
}

function restoreDefaults() {
  Object.assign(form, {
    app_name: "",
    logo: "",
    domain_prefix: "",
    login_slogan: "",
  });
  logoURL.value = "";
  editorRevision.value += 1;
  formRef.value?.clearValidate();
}

async function save() {
  if (saving.value || uploading.value || !editable.value) return;
  saving.value = true;
  try {
    apply(await savePersonalization({ ...form }));
    await brandingStore.loadTenant();
    message.success(t("common_saveSuccess"));
  } catch {
    /* The API interceptor reports save failures. */
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped lang="less">
.personalization-scroll {
  height: 100%;
  min-height: 0;
  overflow-y: auto;
}
.personalization-panel {
  padding: 20px 0;
}
.personalization-form {
  margin-top: 20px;
  padding: 30px;
  border: 1px solid var(--ant-color-border-secondary, #e8edf3);
  border-radius: 12px;
}
.logo-row {
  display: flex;
  align-items: center;
  gap: 20px;
}
.logo-upload {
  width: 100px;
  height: 100px;
  padding: 12px;
}
.logo-upload img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}
.field-tip {
  color: var(--ant-color-text-secondary, #8c8c8c);
  font-size: 13px;
  line-height: 24px;
}
.domain-input {
  max-width: 560px;
}
.personalization-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
@media (max-width: 640px) {
  .personalization-form {
    padding: 16px;
  }
  .logo-row {
    flex-wrap: wrap;
  }
}
</style>

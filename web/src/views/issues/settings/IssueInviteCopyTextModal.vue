<template>
  <a-modal
    v-model:open="open"
    :title="$t('issue_invite_copy_text')"
    :width="480"
    :mask-closable="false"
  >
    <div class="issue-invite-copy-text-modal__field">
      <label class="issue-invite-copy-text-modal__label">{{
        $t("issue_invite_theme_image")
      }}</label>
      <a-upload
        :show-upload-list="false"
        accept="image/jpeg,image/png,image/gif,image/webp"
        :before-upload="validateThemeImage"
        :custom-request="handleThemeImageUpload"
        :disabled="themeImageUploading"
      >
        <div class="issue-invite-copy-text-modal__upload-box">
          <img
            v-if="themeImage"
            :src="themeImage"
            class="issue-invite-copy-text-modal__upload-preview"
            alt=""
          />
          <template v-else>
            <LoadingOutlined v-if="themeImageUploading" />
            <PlusOutlined v-else />
            <span>{{ $t("issue_invite_theme_image_upload") }}</span>
          </template>
        </div>
      </a-upload>
      <p class="issue-invite-copy-text-modal__hint">
        {{ $t("issue_invite_theme_image_hint") }}
      </p>
    </div>

    <div class="issue-invite-copy-text-modal__field">
      <label class="issue-invite-copy-text-modal__label">{{
        $t("issue_invite_guide_title")
      }}</label>
      <a-input
        v-model:value="guideTitle"
        :maxlength="20"
        :placeholder="$t('issue_invite_guide_title_placeholder')"
        show-count
      />
    </div>

    <div class="issue-invite-copy-text-modal__field">
      <label class="issue-invite-copy-text-modal__label">{{
        $t("issue_invite_guide_description")
      }}</label>
      <a-textarea
        v-model:value="guideDescription"
        :maxlength="300"
        :rows="4"
        :placeholder="$t('issue_invite_guide_description_placeholder')"
        show-count
      />
      <p class="issue-invite-copy-text-modal__hint">
        {{ $t("issue_invite_guide_description_hint") }}
      </p>
    </div>

    <template #footer>
      <a-button @click="open = false">{{ $t("common_cancel") }}</a-button>
      <a-button type="primary" @click="handleConfirm">{{
        $t("common_confirm")
      }}</a-button>
    </template>
  </a-modal>
</template>

<script setup>
import { ref } from "vue";
import { message } from "ant-design-vue";
import { LoadingOutlined, PlusOutlined } from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import { useIssueInviteStore } from "@/stores/issueInvite";

const props = defineProps({
  wsId: { type: [String, Number], required: true },
});

const open = defineModel("open", { type: Boolean, default: false });
const guideTitle = defineModel("guideTitle", { type: String, default: "" });
const guideDescription = defineModel("guideDescription", {
  type: String,
  default: "",
});
const themeImage = defineModel("themeImage", { type: String, default: "" });

const { t } = useI18n();
const store = useIssueInviteStore();
const themeImageUploading = ref(false);

const emit = defineEmits(["confirm", "upload"]);

function handleConfirm() {
  emit("confirm");
  open.value = false;
}

function validateThemeImage(file) {
  const ext = String(file.name || "")
    .split(".")
    .pop()
    .toLowerCase();
  if (!["jpg", "jpeg", "png", "gif", "webp"].includes(ext)) {
    message.error(t("issue_invite_theme_image_type_invalid"));
    return false;
  }
  if (file.size > 500 * 1024) {
    message.error(t("issue_invite_theme_image_too_large"));
    return false;
  }
  return true;
}

async function handleThemeImageUpload({ file, onSuccess, onError }) {
  themeImageUploading.value = true;
  try {
    const url = await store.uploadThemeImage(props.wsId, file);
    // Hand the URL back so the parent persists it with the current form values,
    // avoiding any dependency on the debounce or ref round-trip timing.
    emit("upload", url);
    themeImage.value = url;
    onSuccess(url);
  } catch (error) {
    onError(error);
  } finally {
    themeImageUploading.value = false;
  }
}
</script>

<style scoped lang="less">
.issue-invite-copy-text-modal {
  &__field {
    margin-top: 16px;
  }

  &__label {
    display: block;
    margin-bottom: 8px;
    color: #262626;
    font-size: 14px;
  }

  &__upload-box {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 4px;
    width: 120px;
    height: 80px;
    border: 1px dashed #d9d9d9;
    border-radius: 6px;
    background: #fafafa;
    color: #8b8b8b;
    font-size: 12px;
    cursor: pointer;

    &:hover {
      border-color: #0d68ff;
      color: #0d68ff;
    }
  }

  &__upload-preview {
    width: 100%;
    height: 100%;
    object-fit: cover;
    border-radius: 6px;
  }

  &__hint {
    margin: 8px 0 0;
    color: #8b8b8b;
    font-size: 12px;
  }
}
</style>

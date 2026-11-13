<template>
  <a-modal
    v-model:open="visible"
    :title="title"
    :width="type === 'link' ? 650 : 520"
    :ok-text="$t('common_confirm')"
    :cancel-text="$t('common_cancel')"
    :confirm-loading="submitting"
    @ok="submit"
  >
    <p class="kb-modal-desc" v-if="type === 'link'">
      {{ $t("kb_linkModalDesc") }}
    </p>

    <a-form layout="vertical">
      <a-form-item v-if="type !== 'upload'" :label="fieldTitleLabel">
        <a-input
          v-model:value="form.title"
          :maxlength="200"
          :placeholder="fieldTitlePlaceholder"
          @press-enter="submit"
        />
      </a-form-item>

      <a-form-item v-if="type === 'link'" :label="$t('kb_fieldUrl')">
        <a-input
          v-model:value="form.url"
          :placeholder="$t('kb_fieldUrlPlaceholder')"
          @press-enter="submit"
        />
      </a-form-item>

      <a-form-item v-if="type === 'upload'" :label="$t('kb_fieldFile')">
        <a-upload-dragger
          :file-list="fileList"
          :before-upload="onBeforeUpload"
          :multiple="true"
          :show-upload-list="true"
        >
          <p class="ant-upload-drag-icon"><InboxOutlined /></p>
          <p class="ant-upload-text">{{ $t("kb_uploadDragText") }}</p>
          <p class="ant-upload-hint">{{ $t("kb_uploadHint") }}</p>
        </a-upload-dragger>
      </a-form-item>
    </a-form>
  </a-modal>
</template>

<script setup>
import { computed, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import DOMPurify from "dompurify";
import { InboxOutlined } from "@ant-design/icons-vue";
import {
  createKnowledgeFolder,
  createKnowledgeDocument,
  createKnowledgeDocumentFromFile,
  createKnowledgeLink,
} from "@/api/knowledgeBase";
import { useKnowledgeStore } from "@/stores/knowledge";

const props = defineProps({
  open: { type: Boolean, default: false },
  // 'folder' | 'document' | 'link' | 'upload'
  type: { type: String, default: "folder" },
  parentId: { type: Number, default: 0 },
  wsId: { type: [Number, String], default: 0 },
});

const emit = defineEmits(["update:open", "created"]);

const { t: $t } = useI18n();
const store = useKnowledgeStore();
const submitting = ref(false);
const fileList = ref([]);
const form = reactive({ title: "", url: "" });

const visible = computed({
  get: () => props.open,
  set: (value) => emit("update:open", value),
});

const title = computed(() => {
  switch (props.type) {
    case "document":
      return $t("kb_newDocument");
    case "link":
      return $t("kb_newLink");
    case "upload":
      return $t("kb_uploadFile");
    default:
      return $t("kb_newFolder");
  }
});
const fieldTitleLabel = computed(() =>
  props.type === "folder" ? $t("kb_fieldName") : $t("kb_fieldTitle"),
);
const fieldTitlePlaceholder = computed(() =>
  props.type === "folder"
    ? $t("kb_fieldNamePlaceholder")
    : $t("kb_fieldTitlePlaceholder"),
);

watch(
  () => props.open,
  (open) => {
    if (open) {
      form.title = "";
      form.url = "";
      fileList.value = [];
    }
  },
);

function onBeforeUpload(file) {
  if (file.size > 200 * 1024 * 1024) {
    message.warning($t("kb_uploadTooLarge"));
    return false;
  }
  fileList.value = [...fileList.value, file];
  return false;
}

// docx 上传后默认转为结构化富文本内容；转换失败时回退为文件态上传。
async function uploadFile(file) {
  if (!/\.docx$/i.test(file.name)) {
    return createKnowledgeDocumentFromFile(props.wsId, props.parentId, file);
  }
  try {
    const mammothModule = await import("mammoth");
    const mammoth = mammothModule.default || mammothModule;
    const arrayBuffer = await file.arrayBuffer();
    const result = await mammoth.convertToHtml(
      { arrayBuffer },
      { externalFileAccess: false },
    );
    const content = DOMPurify.sanitize(result.value, {
      USE_PROFILES: { html: true },
    });
    const title = (file.name.replace(/\.docx$/i, "").trim() || file.name).slice(
      0,
      200,
    );
    return createKnowledgeDocument(props.wsId, {
      parent_id: props.parentId,
      title,
      content,
      content_mode: "rich-text",
    });
  } catch {
    return createKnowledgeDocumentFromFile(props.wsId, props.parentId, file);
  }
}

async function submit() {
  if (submitting.value) return;
  if (props.type === "upload") {
    if (!fileList.value.length) {
      message.warning($t("kb_uploadEmpty"));
      return;
    }
    submitting.value = true;
    try {
      const created = [];
      for (const file of fileList.value) {
        const item = await uploadFile(file);
        created.push(item);
        store.upsertItem(item);
      }
      message.success($t("kb_uploadSuccess"));
      emit("created", created);
      visible.value = false;
    } catch {
      // error toast handled by the request interceptor
    } finally {
      submitting.value = false;
    }
    return;
  }

  const titleValue = form.title.trim();
  if (props.type !== "link" && !titleValue) {
    message.warning($t("kb_titleRequired"));
    return;
  }
  if (props.type === "link") {
    const url = form.url.trim();
    if (!/^https?:\/\/.+/i.test(url)) {
      message.warning($t("kb_urlInvalid"));
      return;
    }
  }

  submitting.value = true;
  try {
    let item;
    if (props.type === "folder") {
      item = await createKnowledgeFolder(props.wsId, {
        parent_id: props.parentId,
        title: titleValue,
      });
    } else if (props.type === "document") {
      item = await createKnowledgeDocument(props.wsId, {
        parent_id: props.parentId,
        title: titleValue,
        content: "",
        content_mode: "rich-text",
      });
    } else {
      item = await createKnowledgeLink(props.wsId, {
        parent_id: props.parentId,
        title: titleValue,
        source_url: form.url.trim(),
      });
    }
    store.upsertItem(item);
    emit("created", item);
    visible.value = false;
  } catch {
    // error toast handled by the request interceptor
  } finally {
    submitting.value = false;
  }
}
</script>

<style scoped lang="less">
.kb-modal-desc {
  margin-bottom: 20px;
  font-size: 14px;
  color: #8c8c8c;
}
</style>

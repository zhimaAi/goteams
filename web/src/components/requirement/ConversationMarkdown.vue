<template>
  <div class="conversation-markdown">
    <div ref="body" @click="onImageClick" v-html="html"></div>
    <a-modal
      :open="Boolean(preview)"
      :footer="null"
      width="min(90vw, 1000px)"
      centered
      destroy-on-close
      @update:open="preview = null"
    >
      <img
        v-if="preview"
        class="conversation-full-image"
        :src="preview.src"
        :alt="preview.alt"
      />
    </a-modal>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import {
  getPipelineDocument,
  getPipelineDocumentLink,
} from "@/api/pipelineDocuments";
import { renderConversationMarkdown } from "./conversationMarkdown";

const props = defineProps({
  content: { type: String, default: "" },
  documents: { type: Array, default: () => [] },
  sourcePath: { type: String, default: "" },
});
const { t } = useI18n();
const body = ref(null);
const preview = ref(null);
let controller;
let version = 0;

const html = computed(() =>
  renderConversationMarkdown(props.content, props.documents, {
    sourcePath: props.sourcePath,
    loading: t("common_loading"),
    notSynced: t("pipeline_documents_image_not_synced"),
    preview: t("pipeline_documents_image_preview"),
    retry: t("pipeline_documents_image_retry"),
  }),
);

function clearImages() {
  controller?.abort();
  preview.value = null;
  body.value
    ?.querySelectorAll(".markdown-document-image img")
    .forEach((img) => {
      img.removeAttribute("src");
    });
}

async function loadImage(card, currentVersion, signal) {
  const id = Number(card.dataset.imageDocumentId);
  // Internal HTML attributes are not an authority to access another task's document.
  if (!props.documents.some((doc) => Number(doc.document_id) === id)) return;
  if (card.dataset.loading === String(currentVersion) || signal.aborted) return;
  const image = card.querySelector("img");
  const button = card.querySelector(".markdown-image-preview");
  const status = card.querySelector(".markdown-image-status");
  const retry = card.querySelector(".markdown-image-retry");
  if (!image || !button || !status || !retry) return;
  card.dataset.loading = String(currentVersion);
  button.disabled = true;
  image.hidden = true;
  retry.hidden = true;
  status.hidden = false;
  status.textContent = t("common_loading");
  try {
    const result = await getPipelineDocumentLink(id, "inline", {
      signal,
      silent: true,
    });
    if (signal.aborted || currentVersion !== version || !card.isConnected)
      return;
    const url = new URL(result?.link || "", window.location.href);
    if (
      !result?.link ||
      !/^https?:$/.test(url.protocol) ||
      url.username ||
      url.password
    ) {
      throw new Error("Invalid image link");
    }
    image.src = url.href;
    await image.decode();
    if (signal.aborted || currentVersion !== version || !card.isConnected)
      return;
    image.hidden = false;
    status.hidden = true;
    button.disabled = false;
  } catch (error) {
    if (signal.aborted || currentVersion !== version || !card.isConnected)
      return;
    image.removeAttribute("src");
    let state = "";
    if (error.response?.status === 409) {
      try {
        const metadata = await getPipelineDocument(id, {
          signal,
          silent: true,
        });
        state = metadata?.storage_state || "";
      } catch {
        /* The image remains unavailable when metadata access fails. */
      }
    }
    if (signal.aborted || currentVersion !== version || !card.isConnected)
      return;
    status.textContent = t(
      state === "pending"
        ? "pipeline_documents_image_pending"
        : "pipeline_documents_image_unavailable",
    );
    retry.hidden = false;
  } finally {
    if (card.dataset.loading === String(currentVersion))
      delete card.dataset.loading;
  }
}

watch(
  [html, () => props.documents],
  async () => {
    const currentVersion = ++version;
    clearImages();
    controller = new AbortController();
    const signal = controller.signal;
    await nextTick();
    if (signal.aborted || currentVersion !== version) return;
    const cards = [
      ...(body.value?.querySelectorAll(".markdown-document-image") || []),
    ];
    let next = 0;
    await Promise.all(
      Array.from({ length: Math.min(4, cards.length) }, async () => {
        while (next < cards.length && !signal.aborted) {
          await loadImage(cards[next++], currentVersion, signal);
        }
      }),
    );
  },
  { immediate: true },
);

async function onImageClick(event) {
  const card = event.target.closest?.(".markdown-document-image");
  if (!card) return;
  event.preventDefault();
  event.stopPropagation();
  if (event.target.closest(".markdown-image-retry")) {
    if (controller) await loadImage(card, version, controller.signal);
    return;
  }
  const img = card.querySelector("img");
  if (img && !img.hidden && img.naturalWidth) {
    preview.value = { src: img.src, alt: img.alt };
  }
}

onBeforeUnmount(() => {
  version += 1;
  clearImages();
});
</script>

<style scoped lang="less">
.conversation-markdown :deep(.markdown-document-image) {
  display: inline-flex;
  flex-direction: column;
  max-width: 100%;
  overflow: hidden;
  border: 1px solid var(--ant-color-border, #d9d9d9);
  border-radius: 8px;
  vertical-align: middle;
}
.conversation-markdown :deep(.markdown-image-preview) {
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 120px;
  min-height: 80px;
  padding: 0;
  border: 0;
  background: var(--ant-color-fill-alter, #fafafa);
  cursor: zoom-in;
}
.conversation-markdown :deep(.markdown-image-preview:disabled) {
  cursor: default;
}
.conversation-markdown :deep(.markdown-document-image img:not([hidden])) {
  display: block;
  max-width: 100%;
  max-height: 240px;
  object-fit: contain;
}
.conversation-markdown :deep(.markdown-image-status) {
  padding: 16px;
  color: var(--ant-color-text-secondary, #595959);
  font-size: 12px;
}
.conversation-markdown :deep(.markdown-image-retry) {
  min-height: 32px;
  border: 0;
  background: var(--ant-color-bg-container, #fff);
  color: var(--ant-color-primary, #1677ff);
  cursor: pointer;
}
.conversation-markdown :deep(.markdown-document-image button:focus-visible) {
  outline: 2px solid var(--ant-color-primary, #1677ff);
  outline-offset: -2px;
}
.conversation-full-image {
  display: block;
  max-width: 100%;
  max-height: 80vh;
  margin: auto;
  object-fit: contain;
}
</style>

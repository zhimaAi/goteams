<!--
 * KbUploadedFileViewer.vue
 * 知识库本地上传文件预览；docx 在浏览器转换，图片使用鉴权 Blob 预览，其他格式降级为下载态。
 -->
<template>
  <div class="kb-uploaded-file-viewer">
    <div v-if="canPreview && loading" class="kb-file-status">
      <a-spin />
      <span>{{ $t('kb_filePreviewLoading') }}</span>
    </div>

    <div v-else-if="isDocx && previewHtml" class="kb-docx-preview-wrap">
      <Teleport v-if="canEdit" to="#kb-doc-header-extras">
        <a-button size="small" class="kb-edit-btn" @click="handleEditClick">
          <EditOutlined />
          {{ $t('kb_editDocx') }}
        </a-button>
      </Teleport>
      <div class="kb-docx-preview" v-html="previewHtml" />
    </div>

    <div v-else-if="isImage && imagePreviewUrl" class="kb-image-preview">
      <a-image
        :src="imagePreviewUrl"
        :alt="item.source_file_name || item.title"
        @error="handleImagePreviewError"
      />
    </div>

    <div v-else class="kb-file-status">
      <div class="kb-file-icon"><FileUnknownOutlined /></div>
      <h2 class="kb-file-name">{{ item.source_file_name || item.title }}</h2>
      <p class="kb-file-hint">
        {{
          previewFailed
            ? $t('kb_filePreviewFailed')
            : $t('kb_fileCannotPreview')
        }}
      </p>
      <div class="kb-file-actions">
        <a-button v-if="canEdit" type="primary" size="large" @click="handleEditClick">
          <EditOutlined />
          {{ $t('kb_editDocx') }}
        </a-button>
        <a-button type="primary" size="large" @click="$emit('download')">
          <DownloadOutlined />
          {{ $t('kb_downloadWithSize', { size: formattedSize }) }}
        </a-button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import DOMPurify from 'dompurify'
import { DownloadOutlined, EditOutlined, FileUnknownOutlined } from '@ant-design/icons-vue'
import {
  convertKnowledgeDocument,
  downloadKnowledgeDocument,
} from '@/api/knowledgeBase'
import { useKnowledgeStore } from '@/stores/knowledge'

const props = defineProps({
  item: { type: Object, required: true },
  wsId: { type: [Number, String], default: 0 },
})

const emit = defineEmits({
  download: () => true,
  converted: (item) => !!item,
})

const { t: $t } = useI18n()
const store = useKnowledgeStore()
const loading = ref(false)
const previewHtml = ref('')
const imagePreviewUrl = ref('')
const previewFailed = ref(false)
let previewRequest = 0

const extension = computed(() =>
  String(props.item.source_extension || '').toLowerCase(),
)
const isDocx = computed(() => extension.value === 'docx')
const canEdit = computed(
  () => isDocx.value && props.item.content_mode === 'file' && store.canEditItem(props.item),
)
const isImage = computed(() => {
  const mimeType = String(props.item.source_mime_type || '').toLowerCase()
  return (
    mimeType.startsWith('image/') ||
    ['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg', 'ico', 'avif'].includes(
      extension.value,
    )
  )
})
const canPreview = computed(() => isDocx.value || isImage.value)
const formattedSize = computed(() =>
  formatFileSize(props.item.source_file_size),
)

watch(
  () => [props.item.id, extension.value],
  () => {
    previewRequest += 1
    loading.value = false
    releaseImagePreviewUrl()
    previewHtml.value = ''
    previewFailed.value = false
    if (isDocx.value) void loadDocxPreview()
    else if (isImage.value) void loadImagePreview()
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  previewRequest += 1
  releaseImagePreviewUrl()
})

async function loadDocxPreview() {
  const requestId = ++previewRequest
  loading.value = true
  try {
    const [response, mammothModule] = await Promise.all([
      downloadKnowledgeDocument(props.wsId, props.item.id),
      import('mammoth'),
    ])
    const mammoth = mammothModule.default || mammothModule
    const arrayBuffer = await response.data.arrayBuffer()
    const result = await mammoth.convertToHtml(
      { arrayBuffer },
      { externalFileAccess: false },
    )
    if (requestId !== previewRequest) return
    previewHtml.value = sanitizeDocxHtml(result.value)
    previewFailed.value = false
  } catch {
    if (requestId !== previewRequest) return
    previewFailed.value = true
  } finally {
    if (requestId === previewRequest) loading.value = false
  }
}

function sanitizeDocxHtml(value) {
  return DOMPurify.sanitize(value, {
    USE_PROFILES: { html: true },
  })
}

async function loadImagePreview() {
  const requestId = ++previewRequest
  loading.value = true
  try {
    const response = await downloadKnowledgeDocument(props.wsId, props.item.id)
    if (requestId !== previewRequest) return
    imagePreviewUrl.value = URL.createObjectURL(response.data)
    previewFailed.value = false
  } catch {
    if (requestId !== previewRequest) return
    previewFailed.value = true
  } finally {
    if (requestId === previewRequest) loading.value = false
  }
}

// 新版上传已默认转为结构化内容，此处仅历史文件态文档直接转换，不再弹窗确认。
function handleEditClick() {
  convertToDocument()
}

async function convertToDocument() {
  try {
    let html = previewHtml.value
    if (!html) {
      const [response, mammothModule] = await Promise.all([
        downloadKnowledgeDocument(props.wsId, props.item.id),
        import('mammoth'),
      ])
      const mammoth = mammothModule.default || mammothModule
      const arrayBuffer = await response.data.arrayBuffer()
      const result = await mammoth.convertToHtml(
        { arrayBuffer },
        { externalFileAccess: false },
      )
      html = sanitizeDocxHtml(result.value)
    }
    const item = await convertKnowledgeDocument(props.wsId, props.item.id, {
      revision: props.item.revision,
      title: props.item.title,
      content: html,
    })
    message.success($t('kb_convertDocxSuccess'))
    emit('converted', item)
  } catch (error) {
    if (error?.response?.status === 409) {
      message.error($t('kb_convertDocxConflict'))
    } else {
      message.error($t('kb_convertFailed'))
    }
  }
}

function handleImagePreviewError() {
  releaseImagePreviewUrl()
  previewFailed.value = true
}

function releaseImagePreviewUrl() {
  if (!imagePreviewUrl.value) return
  URL.revokeObjectURL(imagePreviewUrl.value)
  imagePreviewUrl.value = ''
}

function formatFileSize(value) {
  const size = Number(value) || 0
  if (size < 1024) return `${size}B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)}KB`
  return `${(size / 1024 / 1024).toFixed(1)}MB`
}
</script>

<style scoped lang="less">
.kb-uploaded-file-viewer {
  flex: 1;
  min-height: 100%;
  background: #fff;
  overflow-y: auto;
}

.kb-file-status {
  min-height: calc(100vh - 176px);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: #8c8c8c;
  gap: 16px;
}

.kb-file-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  border-radius: 12px;
  color: #1677ff;
  background: #e6f4ff;
  font-size: 32px;
}

.kb-file-name {
  max-width: 640px;
  margin: 0;
  color: #262626;
  font-size: 20px;
  overflow-wrap: anywhere;
}

.kb-file-hint {
  margin: -8px 0 8px;
}

.kb-file-actions {
  display: flex;
  gap: 12px;
}

.kb-docx-preview-wrap {
  display: flex;
  flex-direction: column;
}

.kb-edit-btn {
  border-color: #d9d9d9;
}

.kb-docx-preview {
  width: min(920px, calc(100% - 64px));
  min-height: calc(100vh - 152px);
  margin: 24px auto;
  padding: 64px 72px;
  color: #262626;
  background: #fff;
  border: 1px solid #f0f0f0;
  box-shadow: 0 2px 16px rgb(0 0 0 / 6%);
  line-height: 1.75;
  overflow-wrap: anywhere;
}

.kb-docx-preview :deep(img) {
  max-width: 100%;
  height: auto;
}

.kb-docx-preview :deep(table) {
  width: 100%;
  border-collapse: collapse;
}

.kb-docx-preview :deep(td),
.kb-docx-preview :deep(th) {
  padding: 8px;
  border: 1px solid #d9d9d9;
}

.kb-image-preview {
  min-height: calc(100vh - 176px);
  padding: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f5f5;
}

.kb-image-preview :deep(.ant-image) {
  display: flex;
  max-width: 100%;
  max-height: calc(100vh - 240px);
}

.kb-image-preview :deep(img) {
  max-width: 100%;
  max-height: calc(100vh - 240px);
  object-fit: contain;
}
</style>

<template>
  <a-modal
    v-model:open="visible"
    :title="$t('kb_history')"
    :width="860"
    :footer="null"
    @after-close="reset"
  >
    <div class="kb-history">
      <div class="kb-history-list">
        <div
          v-for="version in versions"
          :key="version.id"
          class="kb-version-item"
          :class="{ active: selectedVersion?.id === version.id }"
          @click="selectVersion(version)"
        >
          <div class="kb-version-title">
            <span class="kb-version-no">v{{ version.version_no }}</span>
            <span class="kb-version-tag" :class="`is-${version.version_type}`">
              {{ versionTypeText(version.version_type) }}
            </span>
          </div>
          <div class="kb-version-meta">
            {{ version.created_by_name }} · {{ formatTime(version.created_at) }}
          </div>
          <div v-if="version.description" class="kb-version-desc">
            {{ version.description }}
          </div>
        </div>
        <div v-if="!loading && !versions.length" class="kb-history-empty">
          {{ $t('kb_historyEmpty') }}
        </div>
      </div>

      <div class="kb-history-detail">
        <div v-if="!selectedVersion" class="kb-history-detail-empty">
          {{ $t('kb_historySelectHint') }}
        </div>
        <template v-else>
          <div class="kb-history-detail-header">
            <h3 class="kb-history-detail-title">{{ selectedVersion.title }}</h3>
            <div class="kb-history-detail-actions">
              <a-button size="small" @click="copyVersionLink">
                <PaperClipOutlined /> {{ $t('kb_copyTitleLink') }}
              </a-button>
              <a-button size="small" type="primary" danger @click="confirmRestore">
                <RollbackOutlined /> {{ $t('kb_restoreVersion') }}
              </a-button>
            </div>
          </div>
          <div class="kb-history-detail-body">
            <div
              v-if="selectedVersion.content_mode === 'markdown'"
              class="kb-history-rendered"
              v-html="selectedMarkdownHtml"
            />
            <RichTextEditor
              v-else
              class="kb-history-readonly-editor"
              :model-value="selectedVersion.content || ''"
              :content-mode="selectedVersion.content_mode"
              :read-only="true"
              :plain="true"
              :min-height="0"
            />
          </div>
        </template>
      </div>
    </div>
  </a-modal>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { message, Modal } from 'ant-design-vue'
import { PaperClipOutlined, RollbackOutlined } from '@ant-design/icons-vue'
import {
  getKnowledgeVersion,
  listKnowledgeVersions,
  restoreKnowledgeVersion,
} from '@/api/knowledgeBase'
import { useKnowledgeStore } from '@/stores/knowledge'
import { formatLocalDate } from '@/utils/date'
import RichTextEditor from '@/components/common/RichTextEditor.vue'
import { markdownToHtml } from '@/components/common/rich-text-editor/markdownContent'

const props = defineProps({
  open: { type: Boolean, default: false },
  wsId: { type: [Number, String], default: 0 },
  documentId: { type: [Number, String], default: 0 },
  // Current optimistic revision used for the restore lock.
  revision: { type: Number, default: 0 },
})

const emit = defineEmits(['update:open', 'restored'])

const { t: $t } = useI18n()
const store = useKnowledgeStore()

const versions = ref([])
const selectedVersion = ref(null)
const loading = ref(false)
const detailLoading = ref(false)

const visible = computed({
  get: () => props.open,
  set: (value) => emit('update:open', value),
})

const selectedMarkdownHtml = computed(() =>
  markdownToHtml(selectedVersion.value?.content || ''),
)

watch(
  () => props.open,
  (open) => {
    if (open && props.documentId) loadList()
  },
)

async function loadList() {
  loading.value = true
  selectedVersion.value = null
  try {
    const res = await listKnowledgeVersions(props.wsId, props.documentId, {
      page: 1,
      size: 100,
    })
    versions.value = res?.items || []
    if (versions.value.length) selectVersion(versions.value[0])
  } catch {
    versions.value = []
  } finally {
    loading.value = false
  }
}

async function selectVersion(version) {
  selectedVersion.value = version
  if (version.content !== undefined) return
  detailLoading.value = true
  try {
    const detail = await getKnowledgeVersion(
      props.wsId,
      props.documentId,
      version.id,
    )
    if (detail) selectedVersion.value = detail
  } catch {
    // error toast handled by the request interceptor
  } finally {
    detailLoading.value = false
  }
}

function confirmRestore() {
  Modal.confirm({
    title: $t('kb_restoreConfirmTitle'),
    content: $t('kb_restoreConfirmDesc'),
    okText: $t('kb_restoreVersion'),
    cancelText: $t('common_cancel'),
    okButtonProps: { danger: true },
    onOk: () => restore(),
  })
}

async function restore() {
  try {
    const restored = await restoreKnowledgeVersion(
      props.wsId,
      props.documentId,
      selectedVersion.value.id,
      { revision: props.revision },
    )
    message.success($t('kb_restoreSuccess'))
    emit('restored', restored)
    visible.value = false
  } catch (error) {
    if (error?.response?.status === 409) {
      message.error($t('kb_restoreConflict'))
      emit('restored', null) // let the view reload the document
    }
  }
}

function copyVersionLink() {
  const url = `${window.location.origin}/workspace/${props.wsId}/board?tab=knowledge&kbFile=${props.documentId}`
  copyText(url)
}

function copyText(text) {
  if (navigator.clipboard?.writeText) {
    navigator.clipboard.writeText(text).then(
      () => message.success($t('common_copied')),
      () => fallbackCopy(text),
    )
  } else {
    fallbackCopy(text)
  }
}

function fallbackCopy(text) {
  const textarea = document.createElement('textarea')
  textarea.value = text
  document.body.appendChild(textarea)
  textarea.select()
  try {
    document.execCommand('copy')
    message.success($t('common_copied'))
  } catch {
    // clipboard unavailable
  }
  document.body.removeChild(textarea)
}

function versionTypeText(type) {
  switch (type) {
    case 'create':
      return $t('kb_versionTypeCreate')
    case 'convert':
      return $t('kb_versionTypeConvert')
    case 'restore':
      return $t('kb_versionTypeRestore')
    default:
      return $t('kb_versionTypeManual')
  }
}

function formatTime(timestamp) {
  return timestamp ? formatLocalDate(timestamp, true).slice(0, 16) : ''
}

function reset() {
  versions.value = []
  selectedVersion.value = null
}
</script>

<style scoped lang="less">
.kb-history {
  display: flex;
  gap: 16px;
  height: 520px;
}

.kb-history-list {
  width: 260px;
  flex-shrink: 0;
  overflow-y: auto;
  border-right: 1px solid #f0f0f0;
  padding-right: 12px;
}

.kb-version-item {
  padding: 10px 12px;
  border-radius: 6px;
  cursor: pointer;
  margin-bottom: 4px;

  &:hover {
    background: #f5f5f5;
  }

  &.active {
    background: #e6f4ff;
  }
}

.kb-version-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.kb-version-no {
  font-weight: 600;
  font-size: 14px;
  color: #262626;
}

.kb-version-tag {
  font-size: 12px;
  padding: 0 6px;
  border-radius: 4px;
  background: #f0f0f0;
  color: #595959;

  &.is-restore {
    background: #fff7e6;
    color: #d46b08;
  }
}

.kb-version-meta {
  margin-top: 4px;
  font-size: 12px;
  color: #8c8c8c;
}

.kb-version-desc {
  margin-top: 4px;
  font-size: 12px;
  color: #595959;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.kb-history-empty,
.kb-history-detail-empty {
  margin-top: 80px;
  text-align: center;
  color: #8c8c8c;
  font-size: 14px;
}

.kb-history-detail {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.kb-history-readonly-editor {
  width: 100%;

  :deep(.editor-shell) {
    border: 0;
    border-radius: 0;
    overflow: visible;
  }

  :deep(.editor-body),
  :deep(.rich-text-content) {
    min-height: 0;
  }

  :deep(.editor-body) {
    overflow: visible;
  }

  :deep(.rich-text-content) {
    padding: 0;
  }
}

.kb-history-detail-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid #f0f0f0;
}

.kb-history-detail-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #1f1f1f;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.kb-history-detail-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.kb-history-detail-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px 4px;
}

.kb-history-rendered {
  line-height: 1.7;
  color: #262626;
  font-size: 14px;

  :deep(img) {
    max-width: 100%;
  }
}
</style>

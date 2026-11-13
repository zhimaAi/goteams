<template>
  <div class="kb-doc-viewer">
    <template v-if="!item">
      <a-empty :description="$t('kb_selectDocHint')" class="kb-empty" />
    </template>

    <template v-else-if="item.item_type === 'folder'">
      <a-empty :description="$t('kb_folderEmptyHint')" class="kb-empty" />
    </template>

    <template v-else-if="item.item_type === 'link'">
      <div class="kb-link-preview">
        <iframe class="kb-link-frame" :src="item.source_url" />
      </div>
    </template>

    <template v-else-if="item.source_type === 'upload' && item.content_mode === 'file'">
      <KbUploadedFileViewer
        :item="item"
        :ws-id="wsId"
        @download="$emit('download', item)"
        @converted="onConverted"
      />
    </template>

    <template v-else>
      <!-- Preview mode -->
      <div v-if="!editing" class="kb-doc-preview" @dblclick="onPreviewDblClick">
        <div
          v-if="item.content_mode === 'markdown'"
          class="kb-rendered-content"
          v-html="markdownHtml"
        />
        <RichTextEditor
          v-else
          class="kb-readonly-editor"
          :model-value="item.content || ''"
          :content-mode="item.content_mode"
          :read-only="true"
          :plain="true"
          :min-height="0"
        />
      </div>

      <!-- Edit mode -->
      <RichTextEditor
        v-else
        :key="`editor-${item.id}-${sessionId}`"
        ref="editorRef"
        :model-value="editorContent"
        :content-mode="editorMode"
        :placeholder="$t('kb_editorPlaceholder')"
        :default-editing="true"
        :plain="true"
        :fill="true"
        :min-height="360"
        :ws-id="wsId"
        :image-uploader="imageUploader"
        :file-uploader="fileUploader"
        :video-uploader="videoUploader"
        @update:model-value="onContentChange"
        @update:content-mode="onContentModeChange"
      />

      <!-- Edit actions are rendered in the document header. -->
      <Teleport v-if="editing" to="#kb-doc-edit-actions">
        <div class="kb-edit-actions">
          <span class="kb-save-status" :class="`is-${saveStatus}`">
            <LoadingOutlined v-if="saveStatus === 'saving'" spin />
            <ExclamationCircleFilled v-else-if="saveStatus === 'error'" />
            <WarningFilled v-else-if="saveStatus === 'conflict'" />
            <CheckCircleFilled v-else-if="saveStatus === 'saved'" />
            {{ saveStatusText }}
          </span>
          <a-button
            v-if="saveStatus === 'conflict'"
            size="small"
            type="primary"
            danger
            @click="conflictOpen = true"
          >
            {{ $t('kb_resolveConflict') }}
          </a-button>
          <a-button
            v-if="saveStatus === 'error'"
            size="small"
            @click="autosave.resume()"
          >
            {{ $t('kb_retrySave') }}
          </a-button>
          <a-button
            type="primary"
            :loading="finishing"
            :disabled="finishing || copySaving"
            @click="finishEdit"
          >
            {{ $t('common_save') }}
          </a-button>
          <a-button
            :loading="copySaving"
            :disabled="finishing || copySaving"
            @click="saveAsCopy"
          >
            {{ $t('kb_saveAsCopy') }}
          </a-button>
          <a-button
            :disabled="finishing || copySaving"
            @click="cancelEdit"
          >
            {{ $t('common_cancel') }}
          </a-button>
        </div>
      </Teleport>

      <!-- Draft restore modal -->
      <a-modal
        v-model:open="draftOpen"
        :title="$t('kb_draftFound')"
        :ok-text="$t('kb_draftRestore')"
        :cancel-text="$t('kb_draftDiscard')"
        :width="480"
        @ok="restoreDraft"
        @cancel="discardDraft"
      >
        <p class="kb-modal-text">{{ draftText }}</p>
      </a-modal>

      <!-- Conflict modal -->
      <a-modal
        v-model:open="conflictOpen"
        :title="$t('kb_conflictTitle')"
        :footer="null"
        :width="520"
      >
        <p class="kb-modal-text">
          {{ $t('kb_conflictDesc') }}
          <template v-if="autosave.conflict">
            （{{ $t('kb_conflictServerTime') }}
            {{ formatTime(autosave.conflict.updated_at) }}）
          </template>
        </p>
        <div class="kb-conflict-actions">
          <a-button type="primary" @click="loadServerContent">
            {{ $t('kb_conflictLoadServer') }}
          </a-button>
          <a-button @click="saveAsCopy">
            {{ $t('kb_conflictSaveAsCopy') }}
          </a-button>
          <a-button @click="conflictOpen = false">
            {{ $t('kb_conflictLater') }}
          </a-button>
        </div>
      </a-modal>
    </template>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import {
  CheckCircleFilled,
  ExclamationCircleFilled,
  LoadingOutlined,
  WarningFilled,
} from '@ant-design/icons-vue'
import RichTextEditor from '@/components/common/RichTextEditor.vue'
import KbUploadedFileViewer from './KbUploadedFileViewer.vue'
import { markdownToHtml } from '@/components/common/rich-text-editor/markdownContent'
import {
  createKnowledgeDocument,
  createKnowledgeVersion,
  getKnowledgeItem,
  uploadKnowledgeFile,
  uploadKnowledgeImage,
  uploadKnowledgeVideo,
} from '@/api/knowledgeBase'
import { useKnowledgeStore } from '@/stores/knowledge'
import { useAuthStore } from '@/stores/auth'
import { useKnowledgeAutosave } from '@/composables/useKnowledgeAutosave'
import { useKnowledgeDraft } from '@/composables/useKnowledgeDraft'
import { formatLocalDate } from '@/utils/date'

const props = defineProps({
  item: { type: Object, default: null },
  wsId: { type: [Number, String], default: 0 },
  editing: { type: Boolean, default: false },
})

const emit = defineEmits(['done', 'cancel', 'edit', 'switch-document', 'title-changed', 'download', 'converted'])

const { t: $t } = useI18n()
const store = useKnowledgeStore()
const authStore = useAuthStore()

const autosave = useKnowledgeAutosave(props.wsId)
const draft = useKnowledgeDraft({
  userId: authStore.userInfo?.id || 0,
  wsId: props.wsId,
})

const editorRef = ref(null)
const editorContent = ref('')
const editorMode = ref('rich-text')
const sessionId = ref('')
const draftOpen = ref(false)
const conflictOpen = ref(false)
const finishing = ref(false)
const copySaving = ref(false)
const pendingDraft = ref(null)
const draftText = computed(() => {
  if (!pendingDraft.value) return ''
  const base = pendingDraft.value.baseRevision
  const current = props.item?.revision ?? 0
  if (Number(base) < Number(current)) return $t('kb_draftStaleText')
  return $t('kb_draftRestoreText')
})

const markdownHtml = computed(() => markdownToHtml(props.item?.content || ''))

const saveStatus = computed(() => autosave.status.value)
const saveStatusText = computed(() => {
  switch (autosave.status.value) {
    case 'saving':
      return $t('kb_saveSaving')
    case 'unsaved':
      return $t('kb_saveUnsaved')
    case 'error':
      return $t('kb_saveError')
    case 'conflict':
      return $t('kb_saveConflict')
    default:
      return $t('kb_saveSaved')
  }
})

function newSessionId() {
  return (crypto.randomUUID?.() || `kb-${Date.now()}-${Math.random().toString(16).slice(2)}`)
}

function getPayload() {
  return {
    title: props.item?.title || '',
    content: editorContent.value,
    contentMode: editorMode.value,
  }
}

// 预览界面双击进入编辑态（权限校验与头部编辑按钮一致）。
function onPreviewDblClick() {
  if (props.editing || !props.item || !store.canEditItem(props.item)) return
  emit('edit')
}

function getDraftRecord(synced) {
  return draft.buildRecord({
    documentId: props.item.id,
    title: props.item.title,
    content: editorContent.value,
    contentMode: editorMode.value,
    baseRevision: autosave.revision.value,
    serverHash: autosave.serverHash.value,
    synced,
  })
}

function syncCurrentItem(overrides = {}) {
  store.upsertItem({
    ...props.item,
    content: editorContent.value,
    content_mode: editorMode.value,
    revision: autosave.revision.value,
    content_hash: autosave.serverHash.value,
    updated_at: autosave.updatedAt.value || props.item.updated_at,
    updated_by: autosave.updatedBy.value || props.item.updated_by,
    ...overrides,
  })
}

function onContentChange(value) {
  editorContent.value = value
  autosave.markDirty(getPayload)
  draft.schedule(props.item.id, () => getDraftRecord(false))
}

function onContentModeChange(mode) {
  editorMode.value = mode
  autosave.markDirty(getPayload)
  draft.schedule(props.item.id, () => getDraftRecord(false))
}

function imageUploader(file) {
  return uploadKnowledgeImage(props.wsId, props.item.id, file).then((res) => ({
    url: res.url,
    name: res.original_name,
    size: res.file_size,
  }))
}

function fileUploader(file) {
  return uploadKnowledgeFile(props.wsId, props.item.id, file).then((res) => ({
    url: res.url,
    name: res.original_name,
    size: res.file_size,
  }))
}

function videoUploader(file) {
  return uploadKnowledgeVideo(props.wsId, props.item.id, file).then((res) => ({
    url: res.url,
    name: res.original_name,
    size: res.file_size,
  }))
}

// ---- Draft restore ----
async function checkDraft() {
  if (!props.item) return
  pendingDraft.value = null
  const record = await draft.read(props.item.id)
  if (!record) return
  if (record.synced && record.serverHash === props.item.content_hash) {
    // Local draft matches the server state; the server content wins silently.
    return
  }
  if (!record.synced) {
    pendingDraft.value = record
    draftOpen.value = true
  }
}

function restoreDraft() {
  const record = pendingDraft.value
  if (record) {
    editorContent.value = record.content || ''
    editorMode.value = record.contentMode === 'markdown' ? 'markdown' : 'rich-text'
    autosave.start(props.item.id, sessionId.value, props.item.revision, props.item.content_hash)
    autosave.markDirty(getPayload)
  }
  pendingDraft.value = null
  draftOpen.value = false
}

function discardDraft() {
  draft.remove(props.item.id).catch(() => {})
  pendingDraft.value = null
  draftOpen.value = false
}

// ---- Autosave side effects ----
watch(
  () => autosave.status.value,
  async (status) => {
    if (!props.item) return
    if (status === 'saved') {
      // Mark the draft synced with the confirmed revision/hash.
      draft.schedule(props.item.id, () =>
        getDraftRecord(true),
      )
      await draft.flush(props.item.id)
      syncCurrentItem()
    } else if (status === 'conflict') {
      draft.schedule(props.item.id, () => getDraftRecord(false))
      await draft.flush(props.item.id)
      conflictOpen.value = true
    } else if (status === 'error') {
      draft.schedule(props.item.id, () => getDraftRecord(false))
    }
  },
)

// ---- Edit lifecycle ----
// The uploaded file was converted into an editable document: refresh the store item and let the
// parent flip into edit mode; the editing watch below initializes the session once.
function onConverted(item) {
  store.upsertItem(item)
  emit('converted', item)
}

function initEditSession() {
  sessionId.value = newSessionId()
  editorContent.value = props.item.content || ''
  editorMode.value = props.item.content_mode === 'markdown' ? 'markdown' : 'rich-text'
  autosave.start(props.item.id, sessionId.value, props.item.revision, props.item.content_hash)
  void checkDraft()
}

// Fires on mount (immediate), on entering edit mode, and on document switches.
watch(
  [() => props.editing, () => props.item?.id],
  ([editing]) => {
    if (editing && props.item) initEditSession()
  },
  { immediate: true },
)

// Leaving the page while editing must not lose the last keystrokes.
onBeforeUnmount(() => {
  autosave.stop()
  if (props.item) void draft.flush(props.item.id)
})

async function finishEdit() {
  if (finishing.value || copySaving.value) return
  finishing.value = true
  try {
    try {
      await draft.flush(props.item.id)
    } catch {
      // Draft write failure must not block the server save.
    }
    const versionSaved = await saveCurrentVersion()
    if (!versionSaved) return
    autosave.stop()
    syncCurrentItem()
    emit('done')
  } finally {
    finishing.value = false
  }
}

// ---- Conflict resolution ----
async function loadServerContent() {
  try {
    const fresh = await getKnowledgeItem(props.wsId, props.item.id)
    if (!fresh) return
    await draft.remove(props.item.id)
    store.upsertItem(fresh)
    editorContent.value = fresh.content || ''
    editorMode.value = fresh.content_mode === 'markdown' ? 'markdown' : 'rich-text'
    autosave.start(fresh.id, sessionId.value, fresh.revision, fresh.content_hash)
    conflictOpen.value = false
  } catch {
    // error toast handled by the request interceptor
  }
}

async function saveAsCopy() {
  if (copySaving.value || finishing.value) return
  copySaving.value = true
  try {
    const payload = getPayload()
    const item = await createKnowledgeDocument(props.wsId, {
      parent_id: props.item.parent_id,
      title: payload.title,
      content: payload.content,
      content_mode: payload.contentMode,
      edit_session_id: newSessionId(),
    })
    await draft.remove(props.item.id)
    store.upsertItem(item)
    conflictOpen.value = false
    draftOpen.value = false
    message.success($t('kb_saveAsCopySuccess'))
    emit('switch-document', item)
  } catch {
    // error toast handled by the request interceptor
  } finally {
    copySaving.value = false
  }
}

function cancelEdit() {
  if (finishing.value || copySaving.value) return
  autosave.stop()
  conflictOpen.value = false
  draftOpen.value = false
  emit('cancel')
}

async function saveCurrentVersion() {
  if (saveStatus.value === 'conflict') {
    conflictOpen.value = true
    return false
  }
  try {
    const saved = await autosave.flush()
    if (!saved) {
      if (saveStatus.value === 'conflict') conflictOpen.value = true
      return false
    }
    const version = await createKnowledgeVersion(props.wsId, props.item.id, {
      revision: autosave.revision.value,
      description: '',
      edit_session_id: sessionId.value,
    })
    syncCurrentItem(
      version?.version_no ? { version_seq: version.version_no } : {},
    )
    if (version?.created) message.success($t('kb_versionCreated'))
    else message.success($t('common_saveSuccess'))
    return true
  } catch {
    // error toast handled by the request interceptor
    return false
  }
}

function formatTime(timestamp) {
  return timestamp ? formatLocalDate(timestamp, true).slice(0, 16) : ''
}
</script>

<style scoped lang="less">
.kb-doc-viewer {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: #fff;
}

.kb-empty {
  margin: 120px auto;
}

.kb-doc-preview {
  flex: 1;
  overflow-y: auto;
  padding: 24px 32px 48px;
}

.kb-rendered-content {
  width: 100%;
  max-width: none;
  margin: 0;
  line-height: 1.7;
  color: #262626;
  font-size: 14px;

  :deep(img) {
    max-width: 100%;
  }

  :deep(a) {
    color: #1677ff;
  }
}

.kb-readonly-editor {
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

:deep(.rich-text-editor.is-editable .rich-text-content) {
  box-sizing: border-box;
  width: 100%;
  max-width: none;
  margin: 0;
  padding: 28px 32px 48px;
}

.kb-link-preview {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.kb-link-frame {
  flex: 1;
  width: 100%;
  border: 0;
  background: #fff;
}

.kb-edit-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.kb-save-status {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  color: #8c8c8c;

  &.is-saving {
    color: #1677ff;
  }

  &.is-error {
    color: #ff4d4f;
  }

  &.is-conflict {
    color: #fa8c16;
  }

  &.is-saved {
    color: #21a665;
  }
}

.kb-modal-text {
  margin: 0;
  font-size: 14px;
  color: #595959;
  line-height: 1.7;
}

.kb-conflict-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 24px;
}
</style>

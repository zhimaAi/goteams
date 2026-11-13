<template>
  <div class="kb-doc-header">
    <div class="kb-doc-header-left">
      <a-button
        type="text"
        size="small"
        class="kb-star-btn"
        :class="{ starred: item?.favorite }"
        :aria-label="item?.favorite ? $t('kb_unfavorite') : $t('kb_favorite')"
        @click="onFavorite"
      >
        <StarFilled v-if="item?.favorite" />
        <StarOutlined v-else />
      </a-button>

      <template v-if="item">
        <input
          v-if="editingTitle && !editing"
          ref="titleInput"
          v-model="titleValue"
          class="kb-title-input"
          :maxlength="200"
          @keydown.enter="commitTitle"
          @keydown.esc="cancelTitle"
          @blur="commitTitle"
        />
        <h1 v-else class="kb-doc-title" :title="item.title" @click="startTitleEdit">
          {{ item.title }}
        </h1>
      </template>
    </div>

    <div class="kb-doc-header-right">
      <div
        v-if="editing"
        id="kb-doc-edit-actions"
        class="kb-doc-edit-actions"
      />

      <span v-if="!editing && item?.updated_at" class="kb-doc-meta">
        {{ $t('kb_updatedAt', { time: formatTime(item.updated_at) }) }}
      </span>

      <div v-if="!editing && item" id="kb-doc-header-extras" class="kb-doc-header-extras" />

      <a-button
        v-if="!editing && isLink"
        size="small"
        class="kb-open-btn"
        :href="item.source_url"
        target="_blank"
        rel="noopener noreferrer"
      >
        {{ $t('kb_open') }}
      </a-button>

      <template v-if="!editing && isDocument">
        <a-button
          v-if="canEditItem && item.content_mode !== 'file'"
          type="primary"
          class="kb-edit-btn"
          @click="$emit('edit')"
        >
          <EditOutlined /> {{ $t('common_edit') }}
        </a-button>
      </template>

      <a-dropdown v-if="item && !editing" trigger="click" placement="bottomRight">
        <a-button type="text" size="small" class="kb-header-more" :aria-label="$t('common_more')">
          <EllipsisOutlined />
        </a-button>
        <template #overlay>
          <a-menu class="kb-more-menu" @click="onMenuClick">
            <a-menu-item key="download" v-if="isDocument">
              <DownloadOutlined /> {{ $t('kb_download') }}
            </a-menu-item>
            <a-menu-item key="copy-link">
              <LinkOutlined /> {{ $t('kb_copyLink') }}
            </a-menu-item>
            <a-menu-item key="copy-title-link" v-if="item.item_type !== 'folder'">
              <PaperClipOutlined /> {{ $t('kb_copyTitleLink') }}
            </a-menu-item>
            <a-menu-item key="history" v-if="isDocument && item.content_mode !== 'file'">
              <HistoryOutlined /> {{ $t('kb_history') }}
            </a-menu-item>
            <a-menu-divider v-if="canEditItem" />
            <a-menu-item key="delete" danger v-if="canEditItem">
              <DeleteOutlined />
              {{
                item.item_type === 'folder'
                  ? $t('kb_deleteFolder')
                  : $t('common_delete')
              }}
            </a-menu-item>
          </a-menu>
        </template>
      </a-dropdown>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import {
  DeleteOutlined,
  DownloadOutlined,
  EditOutlined,
  EllipsisOutlined,
  HistoryOutlined,
  LinkOutlined,
  PaperClipOutlined,
  StarFilled,
  StarOutlined,
} from '@ant-design/icons-vue'
import { useKnowledgeStore } from '@/stores/knowledge'
import { formatFriendlyLocalDate } from '@/utils/date'

const props = defineProps({
  item: { type: Object, default: null },
  editing: { type: Boolean, default: false },
})

const emit = defineEmits([
  'edit',
  'edit-link',
  'favorite',
  'delete',
  'copy-link',
  'copy-title-link',
  'download',
  'history',
  'title-changed',
])

const { t: $t } = useI18n()
const store = useKnowledgeStore()
const editingTitle = ref(false)
const titleValue = ref('')
const titleInput = ref(null)

const isDocument = computed(() => props.item?.item_type === 'document')
const isLink = computed(() => props.item?.item_type === 'link')
const canEditItem = computed(() =>
  props.item ? store.canEditItem(props.item) : false,
)

function startTitleEdit() {
  if (props.editing || !canEditItem.value) return
  // 链接的名称和链接一起编辑，统一走弹窗。
  if (isLink.value) {
    emit('edit-link', props.item)
    return
  }
  editingTitle.value = true
  titleValue.value = props.item.title
  nextTick(() => {
    titleInput.value?.focus()
    titleInput.value?.select()
  })
}

async function commitTitle() {
  if (!editingTitle.value) return
  editingTitle.value = false
  const title = titleValue.value.trim()
  if (!title || title === props.item.title) return
  try {
    await store.renameItem(props.item.id, title)
    message.success($t('kb_renameSuccess'))
    emit('title-changed', title)
  } catch {
    // error toast handled by the request interceptor
  }
}

function cancelTitle() {
  editingTitle.value = false
}

function onFavorite() {
  store.toggleFavorite(props.item.id).catch(() => {})
}

function onMenuClick({ key }) {
  emit(key, props.item)
}

function formatTime(timestamp) {
  return timestamp
    ? formatFriendlyLocalDate(timestamp, $t('common_today'), $t('common_yesterday'))
    : ''
}

defineExpose({ startTitleEdit })
</script>

<style scoped lang="less">
.kb-doc-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 56px;
  padding-right: 24px;
  border-bottom: 1px solid #f0f0f0;
  flex-shrink: 0;
  gap: 12px;
}

.kb-doc-header-left,
.kb-doc-header-right {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.kb-doc-header-right {
  margin-left: auto;
  flex-shrink: 0;
}

.kb-doc-edit-actions {
  display: flex;
  align-items: center;
  min-height: 32px;
}

.kb-star-btn {
  color: #8c8c8c;
  flex-shrink: 0;

  &.starred {
    color: #fadb14;
  }
}

.kb-doc-title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: #172033;
  cursor: pointer;
  max-width: 420px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  line-height: 1.3;
}

.kb-title-input {
  height: 30px;
  padding: 0 10px;
  font-size: 18px;
  font-weight: 600;
  border: 1px solid #1677ff;
  border-radius: 6px;
  outline: none;
  width: 320px;
}

.kb-doc-meta {
  font-size: 13px;
  color: #8c8c8c;
  white-space: nowrap;
}

.kb-doc-header-extras {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.kb-open-btn {
  border-color: #d9d9d9;
}

.kb-header-more {
  color: #595959;
}

.kb-more-menu {
  min-width: 176px;
  padding: 4px 0;
  border-radius: 8px;

  :deep(.ant-dropdown-menu-item) {
    height: 32px;
    padding: 0 12px;
    font-size: 14px;
  }

  :deep(.ant-dropdown-menu-item-danger) {
    color: #ff4d4f;
  }
}
</style>

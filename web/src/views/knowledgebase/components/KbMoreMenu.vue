<template>
  <a-dropdown :trigger="['click']" placement="bottomRight" @click.stop>
    <a-button
      type="text"
      size="small"
      class="kb-more-trigger"
      :aria-label="$t('common_more')"
      @click.stop
    >
      <EllipsisOutlined />
    </a-button>
    <template #overlay>
      <a-menu class="kb-more-menu" @click="onClick">
        <a-menu-item key="open-new">
          <ExportOutlined /> {{ $t('kb_openInNewTab') }}
        </a-menu-item>
        <a-menu-item key="copy-link">
          <LinkOutlined /> {{ $t('kb_copyLink') }}
        </a-menu-item>
        <a-menu-item key="copy-title-link">
          <PaperClipOutlined /> {{ $t('kb_copyTitleLink') }}
        </a-menu-item>
        <a-menu-item key="rename" v-if="canRename">
          <EditOutlined /> {{ item.item_type === 'link' ? $t('common_edit') : $t('common_rename') }}
        </a-menu-item>
        <a-menu-item key="move-to" v-if="canRename">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M1.5 3.5C1.5 2.94772 1.94772 2.5 2.5 2.5H5.5L6.5 3.5H11.5C12.0523 3.5 12.5 3.94772 12.5 4.5V10.5C12.5 11.0523 12.0523 11.5 11.5 11.5H2.5C1.94772 11.5 1.5 11.0523 1.5 10.5V3.5Z" stroke="currentColor" stroke-width="1.2" fill="none"/>
            <path d="M5.5 7H9.5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" fill="none"/>
            <path d="M8.5 5.5L10.5 7L8.5 8.5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round" fill="none"/>
          </svg>
          {{ $t('kb_moveTo') }}
        </a-menu-item>
        <a-menu-item key="favorite">
          <StarFilled v-if="item.favorite" class="kb-star-filled" />
          <StarOutlined v-else />
          {{ item.favorite ? $t('kb_unfavorite') : $t('kb_favorite') }}
        </a-menu-item>
        <a-menu-divider />
        <a-menu-item key="delete" danger v-if="canDelete">
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
</template>

<script setup>
import { computed } from 'vue'
import {
  DeleteOutlined,
  EditOutlined,
  EllipsisOutlined,
  ExportOutlined,
  LinkOutlined,
  PaperClipOutlined,
  StarFilled,
  StarOutlined,
} from '@ant-design/icons-vue'

const props = defineProps({
  item: { type: Object, required: true },
  canEdit: { type: Boolean, default: false },
  canManage: { type: Boolean, default: false },
})

const emit = defineEmits([
  'rename',
  'delete',
  'favorite',
  'copy-link',
  'copy-title-link',
  'open-new',
  'move-to',
])

// 创建者可管理自己的文件夹/文档/链接；kb:manage 可管理全部节点。
const canRename = computed(() => props.canEdit || props.canManage)
const canDelete = canRename

function onClick({ key, domEvent }) {
  domEvent?.stopPropagation()
  emit(key, props.item)
}
</script>

<style scoped lang="less">
.kb-more-trigger {
  width: 24px;
  height: 24px;
  padding: 0;
  color: #595959;
  opacity: 0;
  transition: opacity 0.2s;

  :deep(.ant-btn-icon) {
    font-size: 14px;
  }
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

.kb-star-filled {
  color: #fadb14;
}
</style>

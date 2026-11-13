<template>
  <aside class="kb-sidebar">
    <KbCreateMenu v-if="store.hasCreate" @select="onCreateClick">
      <a-button block class="kb-new-btn">
        <PlusOutlined /> {{ $t('kb_new') }}
      </a-button>
    </KbCreateMenu>

    <div
      class="kb-home-entry"
      :class="{ active: store.viewState === 'home' }"
      @click="goHome"
    >
      <KbIcon name="home" class="kb-home-icon" />
      <span>{{ $t('kb_home') }}</span>
    </div>

    <div class="kb-directory-toolbar">
      <span class="kb-toolbar-label">
        <KbIcon name="directory" class="kb-directory-icon" /> {{ $t('kb_directory') }}
      </span>
      <div class="kb-toolbar-actions">
        <a-tooltip :title="$t(allFoldersExpanded ? 'kb_collapseAll' : 'kb_expandAll')">
          <a-button
            type="text"
            size="small"
            class="kb-toolbar-btn"
            @click="toggleAllFolders"
          >
            <KbIcon :name="allFoldersExpanded ? 'collapse-all' : 'expand-all'" />
          </a-button>
        </a-tooltip>
        <a-tooltip :title="$t('common_refresh')">
          <a-button
            type="text"
            size="small"
            class="kb-toolbar-btn"
            :disabled="store.loading"
            @click="refreshTree"
          >
            <LoadingOutlined v-if="store.loading" spin />
            <SyncOutlined v-else />
          </a-button>
        </a-tooltip>
        <a-dropdown trigger="click" placement="bottomRight">
          <a-button
            type="text"
            size="small"
            class="kb-toolbar-btn"
            :aria-label="$t('kb_sort')"
          >
            <KbIcon name="sort" />
          </a-button>
          <template #overlay>
            <a-menu :selected-keys="[store.sortKey]" @click="onSortClick">
              <a-menu-item v-for="opt in KB_SORT_KEYS" :key="opt.key">
                {{ $t(opt.labelKey) }}
              </a-menu-item>
            </a-menu>
          </template>
        </a-dropdown>
        <a-button
          type="text"
          size="small"
          class="kb-toolbar-btn kb-toolbar-btn--always"
          :aria-label="$t('common_search')"
          @click="openSearch"
        >
          <SearchOutlined />
        </a-button>
      </div>
    </div>

    <KbDirectoryTree
      :children="sortedTree"
      @select="onSelect"
      @toggle="onToggle"
      @create="onCreateUnder"
      @more="onMore"
    />

    <div v-if="!store.loading && !store.tree.length" class="kb-tree-empty">
      {{ $t('kb_treeEmpty') }}
    </div>
  </aside>
</template>

<script setup>
import { computed } from 'vue'
import { message } from 'ant-design-vue'
import {
  LoadingOutlined,
  PlusOutlined,
  SearchOutlined,
  SyncOutlined,
} from '@ant-design/icons-vue'
import { useI18n } from 'vue-i18n'
import { useKnowledgeStore, KB_SORT_KEYS } from '@/stores/knowledge'
import KbDirectoryTree from './KbDirectoryTree.vue'
import KbCreateMenu from './KbCreateMenu.vue'
import KbIcon from './KbIcon.vue'

const emit = defineEmits(['create', 'search', 'open-item', 'delete-item', 'rename-item', 'copy-link', 'copy-title-link', 'open-new', 'move-to'])

const { t: $t } = useI18n()
const store = useKnowledgeStore()
const sortedTree = computed(() => store.applySort(store.tree))
const allFoldersExpanded = computed(() => {
  const folderIds = store.items
    .filter((item) => item.item_type === 'folder')
    .map((item) => Number(item.id))
  return folderIds.length > 0 && folderIds.every((id) => store.expandedFolderIds.includes(id))
})

function goHome() {
  store.goHome()
}

function openSearch() {
  emit('search')
}

function onCreateClick(type) {
  emit('create', { type, parentId: 0 })
}

function onCreateUnder({ type, folder }) {
  emit('create', { type, parentId: Number(folder.id) })
}

function onSelect(node) {
  emit('open-item', node)
}

function onToggle(node) {
  store.toggleFolder(node.id)
}

function onSortClick({ key }) {
  store.sortKey = key
  store.persistSort()
}

function toggleAllFolders() {
  if (allFoldersExpanded.value) store.collapseAll()
  else store.expandAll()
}

async function refreshTree() {
  try {
    await store.refresh()
    message.success($t('kb_refreshSuccess'))
  } catch {
    // The unified interceptor already reports the failure.
  }
}

function onMore(payload) {
  const eventByAction = {
    delete: 'delete-item',
    rename: 'rename-item',
  }
  emit(eventByAction[payload.action] || payload.action, payload.node)
}
</script>

<style scoped lang="less">
.kb-sidebar {
  display: flex;
  flex-direction: column;
  gap: 8px;
  height: 100%;
  padding: 12px;
  background: #f8f9fb;
  border-right: 1px solid #f0f0f0;
  overflow: hidden;
}

.kb-new-btn {
  border-radius: 6px;
  flex-shrink: 0;
  background: #fff;
  border-color: #d9d9d9;
  color: #1f1f1f;

  &:hover,
  &:focus {
    background: #fff;
  }
}

.kb-home-entry {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 36px;
  padding: 0 10px;
  border-radius: 6px;
  cursor: pointer;
  color: #262626;
  flex-shrink: 0;

  .kb-home-icon {
    color: #8c8c8c;
  }

  &:hover {
    background: #f0f0f0;
  }

  &.active {
    height: 36px;
    background: #e6f4ff;
    color: #3157e2;

    .kb-home-icon {
      color: #3157e2;
    }
  }
}

.kb-directory-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 36px;
  padding: 0 10px;
  flex-shrink: 0;
}

.kb-toolbar-label {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 14px;
  font-weight: 500;
  color: #1f1f1f;

  .kb-directory-icon {
    color: #8c8c8c;
  }
}

.kb-toolbar-actions {
  display: flex;
  align-items: center;
  gap: 2px;
}

.kb-toolbar-btn {
  width: 26px;
  height: 26px;
  padding: 0;
  color: #4e5969;
  opacity: 0;
  transition: opacity 0.15s;

  .kb-sidebar:hover & {
    opacity: 1;
  }

  &--always {
    opacity: 1;
  }

  &:hover {
    background: #f2f3f5;
    color: #165dff;
  }
}

.kb-tree-empty {
  padding: 24px 8px;
  text-align: center;
  font-size: 13px;
  color: #8c8c8c;
}

</style>

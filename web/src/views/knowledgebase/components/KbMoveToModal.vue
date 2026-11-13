<template>
  <a-modal
    v-model:open="visible"
    :title="$t('kb_moveToModalTitle')"
    :width="480"
    :footer="null"
    :destroy-on-close="true"
    class="kb-move-modal"
    @after-close="onAfterClose"
  >
    <div class="kb-move-tree">
      <div
        class="kb-move-node"
        :class="{ selected: selectedFolderId === ROOT_ID, disabled: isRootDisabled }"
        style="padding-left: 12px"
        @click="isRootDisabled ? null : onSelectRoot()"
      >
        <span class="kb-move-caret-placeholder" />
        <HomeOutlined class="kb-move-home-icon" />
        <span class="kb-move-folder-name">{{ $t('kb_rootDir') }}</span>
      </div>

      <div
        v-for="node in folderTree"
        :key="node.id"
        class="kb-move-node"
        :class="{
          selected: selectedFolderId === Number(node.id),
          disabled: node._disabled || disabledFolderId === Number(node.id),
        }"
        :style="{ paddingLeft: `${12 + node._depth * 20}px` }"
        @click="onSelectNodeClick(node)"
      >
        <button
          v-if="node.item_type === 'folder'"
          type="button"
          class="kb-move-caret"
          :class="{ expanded: isExpanded(node.id) }"
          @click.stop="toggleExpand(node)"
        >
          <RightOutlined />
        </button>
        <span v-else class="kb-move-caret-placeholder" />

        <FolderFilled v-if="node.item_type === 'folder'" class="kb-move-folder-icon" />
        <component
          :is="node._icon"
          v-else-if="node.item_type === 'document'"
          class="kb-move-doc-icon"
        />
        <LinkOutlined v-else class="kb-move-link-icon" />

        <span class="kb-move-folder-name" :title="node.title">
          {{ node.title }}
        </span>
      </div>

      <div v-if="!folderTree.length" class="kb-move-empty">
        {{ $t('kb_treeEmpty') }}
      </div>
    </div>

    <div class="kb-move-footer">
      <a-button @click="visible = false">
        {{ $t('common_cancel') }}
      </a-button>
      <a-button
        type="primary"
        :disabled="!selectedFolderId"
        :loading="moving"
        @click="onMove"
      >
        {{ $t('kb_moveToConfirm') }}
      </a-button>
    </div>
  </a-modal>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { message } from 'ant-design-vue'
import {
  FolderFilled,
  HomeOutlined,
  LinkOutlined,
  RightOutlined,
} from '@ant-design/icons-vue'
import { useI18n } from 'vue-i18n'
import { useKnowledgeStore } from '@/stores/knowledge'
import { kbDocFileIcon } from '@/utils/kbFileIcon'

const ROOT_ID = -1

const props = defineProps({
  open: { type: Boolean, default: false },
  item: { type: Object, default: null },
  wsId: { type: [Number, String], default: 0 },
})

const emit = defineEmits(['update:open', 'moved'])

const { t: $t } = useI18n()
const store = useKnowledgeStore()
const expandedIds = ref(new Set())
const selectedFolderId = ref(0)
const moving = ref(false)

const visible = computed({
  get: () => props.open,
  set: (value) => emit('update:open', value),
})

// The folder where the item currently resides (disabled in the tree).
const disabledFolderId = computed(() => {
  if (!props.item) return 0
  return Number(props.item.parent_id) || 0
})

// Root is disabled when the item is already at root level.
const isRootDisabled = computed(() => {
  return !!props.item && Number(props.item.parent_id) === 0
})

// Collect all descendant ids of the item being moved.
function collectDescendantIds(itemId) {
  const ids = new Set()
  const id = Number(itemId)
  ids.add(id)
  const queue = [id]
  while (queue.length) {
    const parentId = queue.shift()
    for (const item of store.items) {
      const childId = Number(item.id)
      if (Number(item.parent_id) !== parentId || ids.has(childId)) continue
      ids.add(childId)
      queue.push(childId)
    }
  }
  return ids
}

// Build a flat list of all items with depth, excluding the moved item and its descendants.
// Folders are selectable; documents and links are shown disabled for context.
const folderTree = computed(() => {
  if (!props.item) return []
  const excludeIds = collectDescendantIds(props.item.id)
  // Build parent->children map.
  const childrenMap = {}
  for (const item of store.items) {
    const id = Number(item.id)
    if (excludeIds.has(id)) continue
    const pid = Number(item.parent_id)
    if (!childrenMap[pid]) childrenMap[pid] = []
    const docIcon = item.item_type === 'document' ? kbDocFileIcon(item) : {}
    childrenMap[pid].push({
      ...item,
      _depth: 0,
      _disabled: item.item_type !== 'folder',
      _icon: docIcon.icon || null,
    })
  }
  // Sort each level.
  for (const pid of Object.keys(childrenMap)) {
    childrenMap[pid] = store.applySort(childrenMap[pid])
  }
  // Flatten with DFS, respecting expanded state.
  const result = []
  function walk(nodes, depth) {
    for (const node of nodes) {
      node._depth = depth
      result.push(node)
      const children = childrenMap[Number(node.id)]
      if (children && children.length && expandedIds.value.has(Number(node.id))) {
        walk(children, depth + 1)
      }
    }
  }
  walk(childrenMap[0] || [], 0)
  return result
})

function isExpanded(id) {
  return expandedIds.value.has(Number(id))
}

function toggleExpand(node) {
  const id = Number(node.id)
  const next = new Set(expandedIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  expandedIds.value = next
}

function onSelectNodeClick(node) {
  if (node._disabled || disabledFolderId.value === Number(node.id)) return
  selectedFolderId.value = Number(node.id)
}

function onSelectRoot() {
  selectedFolderId.value = ROOT_ID
}

async function onMove() {
  if (moving.value || !selectedFolderId.value || !props.item) return
  moving.value = true
  try {
    const parentId = selectedFolderId.value === ROOT_ID ? 0 : selectedFolderId.value
    await store.moveItem(props.item.id, parentId, -1)
    message.success($t('kb_moveSuccess'))
    emit('moved', { itemId: props.item.id, folderId: parentId })
    visible.value = false
  } catch {
    // error toast handled by the request interceptor
  } finally {
    moving.value = false
  }
}

function onAfterClose() {
  selectedFolderId.value = 0
  expandedIds.value = new Set()
  moving.value = false
}

// Expand root folders when modal opens.
watch(
  () => props.open,
  (open) => {
    if (open && props.item) {
      selectedFolderId.value = 0
      // Expand root-level folders and their immediate children.
      const rootFolders = store.items.filter(
        (it) => it.item_type === 'folder' && Number(it.parent_id) === 0,
      )
      const ids = new Set()
      for (const f of rootFolders) {
        ids.add(Number(f.id))
        const children = store.items.filter(
          (it) => it.item_type === 'folder' && Number(it.parent_id) === Number(f.id),
        )
        for (const c of children) ids.add(Number(c.id))
      }
      expandedIds.value = ids
    }
  },
)
</script>

<style scoped lang="less">
.kb-move-tree {
  max-height: 400px;
  overflow-y: auto;
  margin-bottom: 16px;
}

.kb-move-node {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 36px;
  padding-right: 12px;
  border-radius: 6px;
  cursor: pointer;
  color: #262626;
  user-select: none;

  &:hover {
    background: #f0f0f0;
  }

  &.selected {
    background: #e6f4ff;
    color: #1677ff;

    .kb-move-folder-icon {
      color: #faad14;
    }
  }

  &.disabled {
    cursor: not-allowed;
    color: #bfbfbf;

    .kb-move-folder-icon {
      color: #d9d9d9;
    }

    &:hover {
      background: transparent;
    }
  }
}

.kb-move-caret {
  width: 18px;
  height: 18px;
  flex: 0 0 18px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 0;
  color: #8b95a7;
  background: transparent;
  font-size: 10px;
  cursor: pointer;
  transition: transform 0.15s;

  &.expanded {
    transform: rotate(90deg);
  }
}

.kb-move-caret-placeholder {
  width: 18px;
  height: 18px;
  flex: 0 0 18px;
}

.kb-move-home-icon {
  font-size: 15px;
  color: #8c8c8c;
  flex-shrink: 0;
}

.kb-move-folder-icon {
  font-size: 15px;
  color: #8c8c8c;
  flex-shrink: 0;
}

.kb-move-doc-icon,
.kb-move-link-icon {
  font-size: 15px;
  flex-shrink: 0;
}

.kb-move-folder-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
}

.kb-move-empty {
  padding: 24px 8px;
  text-align: center;
  font-size: 13px;
  color: #8c8c8c;
}

.kb-move-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding-top: 12px;
  border-top: 1px solid #f0f0f0;
}
</style>
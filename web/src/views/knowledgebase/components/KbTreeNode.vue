<template>
  <div class="kb-tree-node">
    <div
      class="kb-tree-row"
      :class="{
        selected: isSelected,
        'is-folder': isFolder,
        'kb-row-draggable': canEdit,
      }"
      :data-kb-item-id="node.id"
      :style="{ paddingLeft: `${8 + depth * 16}px` }"
      @click="onClick"
    >
      <button
        v-if="isFolder"
        type="button"
        class="kb-node-caret"
        :aria-label="$t(isExpanded ? 'common_collapse' : 'common_expand')"
        @click.stop="onToggle"
      >
        <KbIcon :name="isExpanded ? 'caret-expanded' : 'caret-collapsed'" />
      </button>
      <span v-else class="kb-node-caret-placeholder" />

      <span class="kb-node-icon">
        <FolderFilled v-if="isFolder" class="kb-folder-icon" />
        <component
          :is="docFileIcon.icon"
          v-else-if="node.item_type === 'document'"
          class="kb-doc-icon"
          :class="docFileIcon.modifier"
        />
        <LinkOutlined v-else class="kb-link-icon" />
      </span>

      <input
        v-if="renaming"
        ref="renameInput"
        v-model="renameValue"
        class="kb-rename-input"
        :maxlength="200"
        @click.stop
        @keydown.enter="commitRename"
        @keydown.esc="cancelRename"
        @blur="commitRename"
      />
      <span v-else class="kb-node-title" :title="node.title">{{ node.title }}</span>

      <!-- Folder aggregate badge: right-aligned, replaced by the action buttons on hover. -->
      <span v-if="isFolder" class="kb-node-count">{{ childCount }}</span>

      <span v-if="!renaming" class="kb-node-actions">
        <KbCreateMenu
          v-if="isFolder && canCreate"
          placement="bottomRight"
          :show-folder="false"
          @select="onCreate"
        >
          <a-button
            type="text"
            size="small"
            class="kb-node-action"
            :aria-label="$t('kb_createUnderFolder')"
            @click.stop
          >
            <PlusOutlined />
          </a-button>
        </KbCreateMenu>
        <KbMoreMenu
          :item="node"
          :can-edit="canEdit"
          :can-manage="canManage"
          @rename="onRename"
          @delete="onDelete"
          @favorite="onFavorite"
          @copy-link="onCopyLink"
          @copy-title-link="onCopyTitleLink"
          @open-new="onOpenNew"
          @move-to="onMoveTo"
        />
      </span>
    </div>

    <div v-if="isFolder" class="kb-tree-children">
      <KbTreeNode
        v-for="child in sortedChildren"
        :key="child.id"
        :node="child"
        :depth="depth + 1"
        @select="$emit('select', $event)"
        @toggle="$emit('toggle', $event)"
        @create="$emit('create', $event)"
        @more="$emit('more', $event)"
      />
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import {
  FolderFilled,
  LinkOutlined,
  PlusOutlined,
} from '@ant-design/icons-vue'
import { useKnowledgeStore } from '@/stores/knowledge'
import { kbDocFileIcon } from '@/utils/kbFileIcon'
import KbMoreMenu from './KbMoreMenu.vue'
import KbCreateMenu from './KbCreateMenu.vue'
import KbIcon from './KbIcon.vue'

const props = defineProps({
  node: { type: Object, required: true },
  depth: { type: Number, default: 0 },
})

const emit = defineEmits(['select', 'toggle', 'create', 'more'])

const { t: $t } = useI18n()
const store = useKnowledgeStore()
const renaming = ref(false)
const renameValue = ref('')
const renameInput = ref(null)

const isFolder = computed(() => props.node.item_type === 'folder')
const docFileIcon = computed(() => kbDocFileIcon(props.node))
const isExpanded = computed(() =>
  store.expandedFolderIds.includes(Number(props.node.id)),
)
const isSelected = computed(() => {
  const id = Number(props.node.id)
  return isFolder.value
    ? id === Number(store.selectedFolderId)
    : id === Number(store.selectedItemId)
})
const canCreate = computed(() => store.hasCreate)
const canEdit = computed(() => store.canEditItem(props.node))
const canManage = computed(() => store.hasManage)
// Live count of direct children for the folder, computed from the flat item list.
const childCount = computed(() =>
  isFolder.value ? store.childCountOf(props.node.id) : 0,
)
const sortedChildren = computed(() =>
  isExpanded.value ? store.applySort(props.node.children || []) : [],
)

function onClick() {
  emit('select', props.node)
}

function onToggle() {
  emit('toggle', props.node)
}

function onCreate(type) {
  emit('create', { type, folder: props.node })
}

function startRename() {
  renaming.value = true
  renameValue.value = props.node.title
  nextTick(() => {
    renameInput.value?.focus()
    renameInput.value?.select()
  })
}

async function commitRename() {
  if (!renaming.value) return
  renaming.value = false
  const title = renameValue.value.trim()
  if (!title || title === props.node.title) return
  try {
    await store.renameItem(props.node.id, title)
    message.success($t('kb_renameSuccess'))
  } catch {
    // error toast handled by the request interceptor
  }
}

function cancelRename() {
  renaming.value = false
}

function onRename() {
  if (isFolder.value || props.node.item_type === 'link') {
    emit('more', { action: 'rename', node: props.node })
  } else {
    startRename()
  }
}

async function onDelete() {
  emit('more', { action: 'delete', node: props.node })
}

function onFavorite() {
  store.toggleFavorite(props.node.id).catch(() => {})
}

function onCopyLink() {
  emit('more', { action: 'copy-link', node: props.node })
}

function onCopyTitleLink() {
  emit('more', { action: 'copy-title-link', node: props.node })
}

function onOpenNew() {
  emit('more', { action: 'open-new', node: props.node })
}

function onMoveTo() {
  emit('more', { action: 'move-to', node: props.node })
}

watch(
  () => props.node.title,
  () => {
    if (renaming.value) renameValue.value = props.node.title
  },
)
</script>

<style scoped lang="less">
.kb-tree-row {
  position: relative;
  display: flex;
  align-items: center;
  gap: 6px;
  height: 32px;
  padding-right: 6px;
  border-radius: 6px;
  cursor: pointer;
  color: #262626;
  user-select: none;
  transition: background 0.12s ease;

  &:hover {
    background: rgba(22, 119, 255, 6%);

    // Hover the row: hide the folder count badge so the icon actions (+ / more)
    // have room to show. The badge reappears when the row is no longer hovered.
    .kb-node-count {
      display: none;
    }

    .kb-node-actions {
      opacity: 1;
      pointer-events: auto;
    }

    .kb-node-action,
    :deep(.kb-more-trigger) {
      opacity: 1;
    }
  }

  &.selected {
    background: #e6f4ff;
    color: #1677ff;

    .kb-node-actions {
      opacity: 1;
      pointer-events: auto;
    }

    .kb-node-action,
    :deep(.kb-more-trigger) {
      opacity: 1;
    }
  }

  &.kb-row-draggable {
    cursor: grab;

    &:active {
      cursor: grabbing;
    }
  }
}

.kb-node-caret,
.kb-node-caret-placeholder {
  width: 18px;
  height: 18px;
  flex: 0 0 18px;
}

.kb-node-caret {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 0;
  color: #262626;
  background: transparent;
  cursor: pointer;

  :deep(.kb-svg-icon) {
    width: 16px;
    height: 16px;
  }
}

.kb-node-icon {
  display: flex;
  align-items: center;
  font-size: 15px;
  flex-shrink: 0;
}

// Folder icon is uniform orange regardless of expansion state (per design).
.kb-folder-icon {
  color: #faad14;
}

// File-type icon glyphs come from antd Filled but their fill is `currentColor`,
// so each modifier drives the visible hue via CSS color. Values are pixel-sampled
// from the Figma design (page_dongtai.png tree area).
.kb-doc-icon {
  color: #8c8c8c;
}

.kb-doc-icon--word {
  color: #4a8dff;
}

.kb-doc-icon--excel {
  color: #00c090;
}

.kb-doc-icon--csv {
  color: #00bdad;
}

.kb-doc-icon--ppt {
  color: #fc7541;
}

.kb-doc-icon--pdf {
  color: #e94848;
}

.kb-doc-icon--ofd {
  color: #356ffe;
}

.kb-doc-icon--image {
  color: #36d2ad;
}

.kb-doc-icon--markdown {
  color: #20b2aa;
}

.kb-doc-icon--zip {
  color: #ff5562;
}

.kb-doc-icon--video {
  color: #627cfe;
}

.kb-doc-icon--txt {
  color: #f9a006;
}

.kb-doc-icon--html {
  color: #7a8699;
}

.kb-doc-icon--unknown {
  color: #d0d8e1;
}

.kb-link-icon {
  color: #8c8c8c;
}

.kb-node-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
  line-height: 1.2;
}

// Right-aligned folder child count badge, shown only for folder rows.
// It sits at the far right of the row (margin-left: auto) and is hidden on hover
// so the absolutely positioned action buttons (+ / more) can take its place.
.kb-node-count {
  flex-shrink: 0;
  margin-left: auto;
  margin-right: 4px;
  font-size: 12px;
  color: #8c8c8c;
  font-variant-numeric: tabular-nums;
}

// Action buttons are absolutely positioned at the row's right edge, so they never
// push the count badge away from the far right in the default state.
.kb-node-actions {
  position: absolute;
  right: 6px;
  top: 50%;
  transform: translateY(-50%);
  display: flex;
  align-items: center;
  gap: 0;
  flex-shrink: 0;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.15s ease;
}

.kb-node-action {
  width: 24px;
  height: 24px;
  padding: 0;
  color: #595959;
  transition: background 0.15s ease;

  :deep(.ant-btn-icon) {
    font-size: 14px;
  }
}

.kb-rename-input {
  flex: 1;
  min-width: 0;
  height: 26px;
  padding: 0 8px;
  font-size: 14px;
  border: 1px solid #1677ff;
  border-radius: 4px;
  outline: none;
}

.kb-tree-children {
  position: relative;
  margin-left: 12px;
}
</style>
<template>
  <div class="kb-view">
    <div
      v-if="!store.sidebarCollapsed"
      class="kb-sidebar-wrap"
      :style="{ width: `${store.sidebarWidth}px` }"
    >
      <KbSidebar
        @create="openCreate"
        @search="searchOpen = true"
        @open-item="handleSelect"
        @delete-item="confirmDelete"
        @rename-item="openRenameItem"
        @copy-link="copyDeepLink"
        @copy-title-link="copyTitleDeepLink"
        @open-new="openInNewTab"
        @move-to="openMoveToModal"
      />
    </div>

    <button
      type="button"
      class="kb-resizer"
      :class="{ collapsed: store.sidebarCollapsed }"
      :aria-label="
        store.sidebarCollapsed ? $t('kb_expandSidebar') : $t('kb_collapseSidebar')
      "
      :title="
        store.sidebarCollapsed ? $t('kb_expandSidebar') : $t('kb_collapseSidebar')
      "
      @pointerdown="startResize"
      @click="handleResizerClick"
    >
      <span class="kb-resizer-arrow">
        <RightOutlined v-if="store.sidebarCollapsed" />
        <LeftOutlined v-else />
      </span>
    </button>

    <!-- 整条分隔边都可横向拖动调整左侧树宽度（折叠时隐藏，避免挡住左侧菜单） -->
    <div
      v-if="!store.sidebarCollapsed"
      class="kb-resizer-zone"
      :style="{ left: `${store.sidebarWidth - 7}px` }"
      @pointerdown="startResize"
    ></div>

    <main class="kb-main">
      <KbHomeView
        v-if="store.viewState === 'home'"
        :ws-id="wsId"
        @open-item="openById"
        @open-folder="openFolderByRef"
      />
      <template v-else>
        <KbDocHeader
          :item="store.selectedItem"
          :editing="editing"
          @edit="startEdit"
          @edit-link="openRenameItem"
          @delete="confirmDelete"
          @copy-link="copyDeepLink"
          @copy-title-link="copyTitleDeepLink"
          @history="historyOpen = true"
          @download="downloadDocument"
        />
        <KbDocViewer
          :key="`viewer-${store.selectedItemId}`"
          :item="store.selectedItem"
          :ws-id="wsId"
          :editing="editing"
          @edit="startEdit"
          @done="finishEdit"
          @cancel="cancelEdit"
          @switch-document="switchDocument"
          @download="downloadDocument"
          @converted="startEdit"
        />
      </template>
    </main>

    <KbCreateModal
      v-model:open="createOpen"
      :type="createType"
      :parent-id="createParentId"
      :ws-id="wsId"
      @created="onCreated"
    />

    <KbSearchModal
      v-model:open="searchOpen"
      :ws-id="wsId"
      @open-item="onSearchOpen"
    />

    <KbHistoryModal
      v-model:open="historyOpen"
      :ws-id="wsId"
      :document-id="store.selectedItemId"
      :revision="store.selectedItem?.revision || 0"
      @restored="onRestored"
    />

    <a-modal
      v-model:open="renameFolderOpen"
      :title="renameModalTitle"
      :ok-text="$t('common_confirm')"
      :cancel-text="$t('common_cancel')"
      :confirm-loading="renameFolderSaving"
      @ok="submitFolderRename"
      @after-close="resetFolderRename"
    >
      <a-form layout="vertical">
        <a-form-item :label="renameFieldLabel">
          <a-input
            v-model:value="renameFolderTitle"
            :maxlength="200"
            :placeholder="
              renameFolderItem?.item_type === 'link'
                ? $t('kb_fieldTitlePlaceholder')
                : $t('kb_fieldNamePlaceholder')
            "
            @press-enter="submitFolderRename"
          />
        </a-form-item>
        <a-form-item
          v-if="renameFolderItem?.item_type === 'link'"
          :label="$t('kb_fieldUrl')"
        >
          <a-input
            v-model:value="renameFolderUrl"
            :placeholder="$t('kb_fieldUrlPlaceholder')"
            @press-enter="submitFolderRename"
          />
        </a-form-item>
      </a-form>
    </a-modal>

    <KbMoveToModal
      v-model:open="moveToOpen"
      :item="moveToItem"
      :ws-id="wsId"
      @moved="onMovedTo"
    />
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { message, Modal } from 'ant-design-vue'
import {
  LeftOutlined,
  RightOutlined,
} from '@ant-design/icons-vue'
import { useKnowledgeStore } from '@/stores/knowledge'
import { useAuthStore } from '@/stores/auth'
import {
  downloadKnowledgeDocument,
  getKnowledgeItem,
} from '@/api/knowledgeBase'
import KbSidebar from './components/KbSidebar.vue'
import KbDocHeader from './components/KbDocHeader.vue'
import KbDocViewer from './components/KbDocViewer.vue'
import KbHomeView from './components/KbHomeView.vue'
import KbCreateModal from './components/KbCreateModal.vue'
import KbSearchModal from './components/KbSearchModal.vue'
import KbHistoryModal from './components/KbHistoryModal.vue'
import KbMoveToModal from './components/KbMoveToModal.vue'

const props = defineProps({
  wsId: { type: [Number, String], default: 0 },
})

const { t: $t } = useI18n()
const store = useKnowledgeStore()
const authStore = useAuthStore()
const route = useRoute()

const editing = ref(false)
const createOpen = ref(false)
const createType = ref('folder')
const createParentId = ref(0)
const searchOpen = ref(false)
const historyOpen = ref(false)
const renameFolderOpen = ref(false)
const renameFolderItem = ref(null)
const renameFolderTitle = ref('')
const renameFolderUrl = ref('')
const renameFolderSaving = ref(false)
const moveToOpen = ref(false)
const moveToItem = ref(null)

onMounted(async () => {
  store.init(authStore.userInfo?.id, props.wsId)
  await store.fetchTree()
  await handleDeepLink()
})

async function handleDeepLink() {
  const { kbFile, kbFolder, kbCollapsed } = route.query
  if (String(kbCollapsed || '') === '1') store.sidebarCollapsed = true
  if (kbFile) {
    await openItemById(Number(kbFile))
  } else if (kbFolder) {
    await openFolderById(Number(kbFolder))
  }
}

// ---- Tree interactions ----
function handleSelect(node) {
  if (node.item_type === 'link') {
    // 链接点击不进入选中态，直接新开浏览器页卡。
    openLinkInNewTab(node.id, node.source_url)
    return
  }
  if (node.item_type === 'folder') {
    openFolderById(node.id)
  } else {
    openItemById(node.id)
  }
}

function selectFolderNode(node) {
  store.selectFolder(Number(node.id))
  expandAncestors(node)
}

function expandAncestors(node) {
  const chain = []
  const visited = new Set([Number(node.id)])
  let parentId = Number(node.parent_id)
  while (parentId && !visited.has(parentId)) {
    visited.add(parentId)
    const parent = store.items.find((item) => Number(item.id) === parentId)
    if (!parent) break
    chain.unshift(parent)
    parentId = Number(parent.parent_id)
  }
  chain.forEach((item) => store.expandFolder(item.id))
}

// 打开文件夹时进入文件夹空态页，并自动打开其中第一个文档（预览态）。
async function openFolderById(folderId) {
  const folder = store.items.find((item) => Number(item.id) === Number(folderId))
  if (!folder) {
    message.error($t('kb_itemNotFound'))
    return
  }
  selectFolderNode(folder)
  store.expandFolder(folder.id)
  try {
    editing.value = false
    const first = findFirstDocumentIn(folder.id)
    if (first) {
      await openItemById(first.id)
      return
    }
    await store.openItem(folder.id)
  } catch {
    store.goHome()
  }
}

async function openItemById(itemId) {
  try {
    const cached = store.items.find((it) => Number(it.id) === Number(itemId))
    if (cached?.item_type === 'link') {
      openLinkInNewTab(cached.id, cached.source_url)
      return
    }
    const item = await store.openItem(itemId)
    if (!item) {
      message.error($t('kb_itemNotFound'))
      store.goHome()
      return
    }
    if (item.item_type === 'link') {
      openLinkInNewTab(item.id, item.source_url)
      return
    }
    editing.value = false
    if (item.item_type === 'folder') {
      store.selectFolder(Number(item.id))
      expandAncestors(item)
      return
    }
    store.clearSelectedFolder()
    if (item.parent_id) {
      const parent = store.items.find(
        (it) => Number(it.id) === Number(item.parent_id),
      )
      if (parent) store.expandFolder(parent.id)
    }
  } catch {
    store.goHome()
  }
}

// 在扁平 items 上递归查找文件夹内（含子文件夹）的第一个文档。
function findFirstDocumentIn(folderId, visited = new Set()) {
  const id = Number(folderId)
  if (visited.has(id)) return null
  visited.add(id)
  const children = store.items
    .filter((item) => Number(item.parent_id) === id)
    .sort((a, b) =>
      Number(a.created_at) - Number(b.created_at) || Number(a.id) - Number(b.id),
    )
  for (const child of children) {
    if (child.item_type === 'document') return child
    if (child.item_type === 'folder') {
      const found = findFirstDocumentIn(child.id, visited)
      if (found) return found
    }
  }
  return null
}

function openById({ id, item_type }) {
  if (item_type === 'link') {
    openLinkInNewTab(id)
    return
  }
  if (item_type === 'folder') openFolderById(id)
  else openItemById(id)
}

function openFolderByRef({ id }) {
  openFolderById(id)
}

function onSearchOpen({ type, item }) {
  if (type === 'folder') openFolderById(item.id)
  else openItemById(item.id)
}

// ---- Create ----
function openCreate({ type, parentId }) {
  createType.value = type || 'folder'
  createParentId.value = parentId || 0
  createOpen.value = true
}

async function onCreated(item) {
  if (Array.isArray(item)) {
    const first = item[0]
    if (first) {
      await openItemById(first.id)
      await scrollTreeItemIntoView(first.id)
      enterEditIfEditable(first)
    }
    return
  }
  if (item.item_type === 'folder') {
    if (item.parent_id) store.expandFolder(Number(item.parent_id))
    selectFolderNode(item)
    return
  }
  // 新建的可编辑文档（含上传后自动转结构化的 docx/txt/md）直接进入编辑状态。
  await openItemById(item.id)
  await scrollTreeItemIntoView(item.id)
  enterEditIfEditable(item)
}

function enterEditIfEditable(item) {
  if (item.item_type === 'document' && item.content_mode !== 'file') {
    editing.value = true
  }
}

async function scrollTreeItemIntoView(itemId) {
  await nextTick()
  const row = document.querySelector(`[data-kb-item-id="${Number(itemId)}"]`)
  row?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
}

const renameModalTitle = computed(() =>
  renameFolderItem.value?.item_type === 'link'
    ? $t('kb_editLink')
    : $t('kb_renameFolder'),
)
const renameFieldLabel = computed(() =>
  renameFolderItem.value?.item_type === 'link' ? $t('kb_fieldTitle') : $t('kb_fieldName'),
)

function openRenameItem(item) {
  renameFolderItem.value = item
  renameFolderTitle.value = item?.title || ''
  renameFolderUrl.value = item?.source_url || ''
  renameFolderOpen.value = true
}

function openMoveToModal(item) {
  moveToItem.value = item
  moveToOpen.value = true
}

function onMovedTo() {
  moveToItem.value = null
}

async function submitFolderRename() {
  if (renameFolderSaving.value) return
  const title = renameFolderTitle.value.trim()
  if (!title) {
    message.warning($t('kb_nameRequired'))
    return
  }
  const isLink = renameFolderItem.value?.item_type === 'link'
  const url = isLink ? renameFolderUrl.value.trim() : ''
  if (isLink && !/^https?:\/\/.+/i.test(url)) {
    message.warning($t('kb_urlInvalid'))
    return
  }
  const unchanged =
    !renameFolderItem.value ||
    (title === renameFolderItem.value.title &&
      (!isLink || url === (renameFolderItem.value.source_url || '')))
  if (unchanged) {
    renameFolderOpen.value = false
    return
  }
  renameFolderSaving.value = true
  try {
    await store.renameItem(renameFolderItem.value.id, title, url)
    message.success(isLink ? $t('kb_linkSaved') : $t('kb_renameSuccess'))
    renameFolderOpen.value = false
  } catch {
    // The unified interceptor already reports the failure.
  } finally {
    renameFolderSaving.value = false
  }
}

function resetFolderRename() {
  renameFolderItem.value = null
  renameFolderTitle.value = ''
  renameFolderUrl.value = ''
  renameFolderSaving.value = false
}

// ---- Edit lifecycle ----
function startEdit() {
  editing.value = true
}

async function finishEdit() {
  editing.value = false
  if (store.selectedItemId) {
    try {
      const fresh = await getKnowledgeItem(props.wsId, store.selectedItemId)
      if (fresh) store.upsertItem(fresh)
    } catch {
      // keep the local item; the tree refresh will reconcile
    }
  }
  store.refresh()
}

async function cancelEdit() {
  editing.value = false
  if (store.selectedItemId) {
    try {
      const fresh = await getKnowledgeItem(props.wsId, store.selectedItemId)
      if (fresh) store.upsertItem(fresh)
    } catch {
      // Keep the current server-backed item if reloading fails.
    }
  }
  store.refresh()
}

async function switchDocument(item) {
  editing.value = true
  await openItemById(item.id)
  editing.value = true
}

// ---- Delete ----
function confirmDelete(item) {
  const isFolder = item.item_type === 'folder'
  Modal.confirm({
    title: isFolder ? $t('kb_deleteFolderConfirmTitle') : $t('kb_deleteConfirmTitle'),
    content: isFolder ? $t('kb_deleteFolderConfirmDesc') : $t('kb_deleteConfirmDesc'),
    okText: $t('common_delete'),
    cancelText: $t('common_cancel'),
    okButtonProps: { danger: true },
    onOk: async () => {
      try {
        await store.removeItem(item.id)
        editing.value = false
        historyOpen.value = false
        message.success($t('kb_deleteSuccess'))
        store.refresh()
      } catch {
        // error toast handled by the request interceptor
      }
    },
  })
}

// ---- Share / open ----
// 链接点击不进入选中态，直接新开浏览器页卡；URL 缺失时兜底拉取详情。
async function openLinkInNewTab(itemId, sourceUrl = '') {
  let url = sourceUrl
  if (!url) {
    const cached = store.items.find((it) => Number(it.id) === Number(itemId))
    url = cached?.source_url || ''
  }
  if (!url) {
    try {
      const detail = await getKnowledgeItem(props.wsId, itemId)
      url = detail?.source_url || ''
    } catch {
      // error toast handled by the request interceptor
    }
  }
  if (!url) {
    message.warning($t('kb_linkUrlMissing'))
    return
  }
  window.open(url, '_blank', 'noopener,noreferrer')
}

function itemDeepLink(item) {
  const param = item.item_type === 'folder' ? 'kbFolder' : 'kbFile'
  return `${window.location.origin}/workspace/${props.wsId}/board?tab=knowledge&${param}=${item.id}&kbCollapsed=1`
}

function copyDeepLink(item) {
  copyText(itemDeepLink(item))
}

function copyTitleDeepLink(item) {
  copyText(`${item.title} ${itemDeepLink(item)}`)
}

function openInNewTab(item) {
  // 链接直接打开外部地址；其余节点打开知识库内页。
  if (item.item_type === 'link' && item.source_url) {
    window.open(item.source_url, '_blank', 'noopener,noreferrer')
    return
  }
  window.open(itemDeepLink(item), '_blank', 'noopener,noreferrer')
}

async function downloadDocument(item) {
  if (!item || item.item_type !== 'document') return
  try {
    const response = await downloadKnowledgeDocument(props.wsId, item.id)
    const url = URL.createObjectURL(response.data)
    const link = document.createElement('a')
    link.href = url
    link.download = knowledgeDownloadName(item, response.headers['content-disposition'])
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)
  } catch {
    // error toast handled by the request interceptor
  }
}

function knowledgeDownloadName(item, disposition = '') {
  const encoded = disposition.match(/filename\*=utf-8''([^;]+)/i)?.[1]
  if (encoded) {
    try {
      return decodeURIComponent(encoded)
    } catch {
      // Fall back to the item metadata when a proxy rewrites the header.
    }
  }
  const plain = disposition.match(/filename="?([^";]+)"?/i)?.[1]
  if (plain) return plain
  if (item.source_type === 'upload' && item.source_file_name)
    return item.source_file_name
  const title = (item.title || 'document').replace(/[\\/:*?"<>|]/g, '_')
  return title.toLowerCase().endsWith('.docx') ? title : `${title}.docx`
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

// ---- Restore from history ----
async function onRestored(restored) {
  if (restored) {
    try {
      const fresh = await getKnowledgeItem(props.wsId, store.selectedItemId)
      if (fresh) store.upsertItem(fresh)
    } catch {
      // ignore
    }
  } else {
    try {
      const fresh = await getKnowledgeItem(props.wsId, store.selectedItemId)
      if (fresh) store.upsertItem(fresh)
    } catch {
      // ignore
    }
  }
  editing.value = false
  store.refresh()
}

// ---- Resizer ----
// 横向拖动分隔条调整左侧树宽度；宽度在拖拽结束时写入 localStorage（见
// store.persistSidebarWidth），下次进入时由 store.init() 恢复。使用 Pointer
// Events 同时覆盖鼠标与触屏，拖动期间禁用文本选中并保持 col-resize 光标。
let resizeState = null
let didResize = false

function startResize(event) {
  if (store.sidebarCollapsed) return
  didResize = false
  resizeState = { startX: event.clientX, startWidth: store.sidebarWidth }
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
  document.addEventListener('pointermove', onResize)
  document.addEventListener('pointerup', stopResize)
  document.addEventListener('pointercancel', stopResize)
  event.preventDefault()
}

function onResize(event) {
  if (!resizeState) return
  if (Math.abs(event.clientX - resizeState.startX) > 2) didResize = true
  const next = Math.min(
    420,
    Math.max(220, resizeState.startWidth + event.clientX - resizeState.startX),
  )
  store.sidebarWidth = next
}

function handleResizerClick() {
  if (didResize) {
    didResize = false
    return
  }
  store.sidebarCollapsed = !store.sidebarCollapsed
}

function stopResize() {
  if (!resizeState) return
  resizeState = null
  store.persistSidebarWidth()
  document.removeEventListener('pointermove', onResize)
  document.removeEventListener('pointerup', stopResize)
  document.removeEventListener('pointercancel', stopResize)
  document.body.style.cursor = ''
  document.body.style.userSelect = ''
}

onBeforeUnmount(() => {
  document.removeEventListener('pointermove', onResize)
  document.removeEventListener('pointerup', stopResize)
  document.removeEventListener('pointercancel', stopResize)
  document.body.style.cursor = ''
  document.body.style.userSelect = ''
})
</script>

<style scoped lang="less">
.kb-view {
  position: relative;
  display: flex;
  flex: 1;
  min-height: 0;
  /* 子元素各自控制溢出；visible 才能让折叠按钮负边距后不被裁剪 */
  overflow: visible;
}

.kb-sidebar-wrap {
  flex-shrink: 0;
  height: 100%;
  overflow: hidden;
}

.kb-resizer {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 44px;
  margin: auto 0;
  flex-shrink: 0;
  padding: 0;
  z-index: 6;
  border: 1px solid #f0f0f0;
  border-left: 0;
  border-radius: 0 7px 7px 0;
  color: #8c8c8c;
  background: #fff;
  box-shadow: 2px 0 5px rgb(0 0 0 / 7%);
  cursor: col-resize;
  touch-action: none;
  transition:
    color 0.15s ease,
    background 0.15s ease;

  &:hover {
    color: #1677ff;
    background: color-mix(in srgb, #1677ff 6%, #fff);
  }

  &.collapsed {
    border-left: 1px solid #f0f0f0;
    /* 抵消 .workspace-content 的 padding-left，让按钮贴到主菜单边缘 */
    margin-left: -24px;
  }
}

.kb-resizer-arrow {
  font-size: 10px;
  pointer-events: none;
}

// 覆盖在分隔边上的全高拖拽区，让整条分隔线都能横向拖动调整左侧树宽度。
// 不占布局空间；折叠时通过 v-if 隐藏，避免挡住左侧主菜单。
.kb-resizer-zone {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 14px;
  cursor: col-resize;
  touch-action: none;
  z-index: 5;
}

.kb-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  background: #fff;
  overflow: hidden;
}
</style>

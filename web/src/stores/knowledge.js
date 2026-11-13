import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  getKnowledgeTree,
  getKnowledgeItem,
  moveKnowledgeItem,
  renameKnowledgeItem,
  deleteKnowledgeItem,
  setKnowledgeFavorite,
  unsetKnowledgeFavorite,
} from '@/api/knowledgeBase'
import { useAuthStore } from '@/stores/auth'

// Sidebar sort options (applied to siblings within each folder).
export const KB_SORT_KEYS = [
  { key: 'manual', labelKey: 'kb_sort_manual' },
  { key: 'updated_desc', labelKey: 'kb_sort_updated_desc' },
  { key: 'updated_asc', labelKey: 'kb_sort_updated_asc' },
  { key: 'created_desc', labelKey: 'kb_sort_created_desc' },
  { key: 'created_asc', labelKey: 'kb_sort_created_asc' },
  { key: 'name_asc', labelKey: 'kb_sort_name_asc' },
  { key: 'name_desc', labelKey: 'kb_sort_name_desc' },
]

function storageKey(userId, wsId, name) {
  return `goteams:kb:${name}:${userId || ''}:${wsId}`
}

function readStored(key, fallback) {
  try {
    const value = localStorage.getItem(key)
    return value === null ? fallback : value
  } catch {
    return fallback
  }
}

function writeStored(key, value) {
  try {
    localStorage.setItem(key, value)
  } catch {
    /* storage may be unavailable */
  }
}

function clampSidebarWidth(value) {
  return Math.min(420, Math.max(220, Number(value) || 360))
}

function buildTree(items, parentId = 0) {
  return items
    .filter((item) => Number(item.parent_id) === Number(parentId))
    .map((item) => ({
      ...item,
      children: buildTree(items, item.id),
    }))
}

export const useKnowledgeStore = defineStore('knowledge', () => {
  // ---- State ----
  const wsId = ref('')
  const items = ref([])
  const loading = ref(false)
  const viewState = ref('home') // 'home' | 'doc'
  const selectedItemId = ref(0)
  const selectedFolderId = ref(0)
  const expandedFolderIds = ref([])
  const sidebarWidth = ref(360)
  const sidebarCollapsed = ref(false)
  const sortKey = ref('manual')
  const editSessionId = ref('')

  // ---- Getters ----
  const tree = computed(() => buildTree(items.value))
  const selectedItem = computed(() =>
    items.value.find((item) => Number(item.id) === Number(selectedItemId.value)),
  )
  const selectedFolder = computed(() =>
    items.value.find((item) => Number(item.id) === Number(selectedFolderId.value)),
  )
  const favoriteItemIds = computed(() =>
    new Set(items.value.filter((item) => item.favorite).map((item) => Number(item.id))),
  )
  const authStore = useAuthStore()
  const permissionsUnavailable = computed(
    () => !authStore.sessionInitialized && !authStore.userInfo?.id,
  )
  const hasManage = computed(
    () => permissionsUnavailable.value || authStore.hasPermission('kb:manage'),
  )
  const hasCreate = computed(
    () => hasManage.value || permissionsUnavailable.value || authStore.hasPermission('kb:create'),
  )

  // 创建者拥有自己创建节点的完整对象权限；kb:manage 可管理全部节点。
  function canEditItem(item) {
    if (hasManage.value) return true
    return Number(item.creator_id) === Number(authStore.userInfo?.id)
  }

  // ---- Actions ----
  function init(userId, targetWsId) {
    if (wsId.value === String(targetWsId) && items.value.length) return
    wsId.value = String(targetWsId)
    items.value = []
    expandedFolderIds.value = []
    selectedItemId.value = 0
    selectedFolderId.value = 0
    viewState.value = 'home'
    editSessionId.value = ''
    sidebarWidth.value = clampSidebarWidth(
      readStored(storageKey(userId, wsId.value, 'sidebar-width'), '360'),
    )
    sortKey.value =
      readStored(storageKey(userId, wsId.value, 'sort'), 'manual') || 'manual'
  }

  function persistSort() {
    writeStored(storageKey(authStore.userInfo?.id, wsId.value, 'sort'), sortKey.value)
  }

  function persistSidebarWidth() {
    writeStored(
      storageKey(authStore.userInfo?.id, wsId.value, 'sidebar-width'),
      String(sidebarWidth.value),
    )
  }

  async function fetchTree() {
    if (!wsId.value) return
    loading.value = true
    try {
      const treeItems = (await getKnowledgeTree(wsId.value)) || []
      const currentById = new Map(
        items.value.map((item) => [Number(item.id), item]),
      )
      // Tree responses intentionally omit document bodies. Preserve detail-only
      // fields for already opened items so a background refresh cannot blank
      // the selected document preview.
      items.value = treeItems.map((item) => ({
        ...(currentById.get(Number(item.id)) || {}),
        ...item,
      }))
    } finally {
      loading.value = false
    }
  }

  async function openItem(itemId) {
    const item = await getKnowledgeItem(wsId.value, itemId)
    if (!item) return null
    const index = items.value.findIndex((it) => Number(it.id) === Number(item.id))
    if (index >= 0) items.value[index] = item
    else items.value.push(item)
    selectedItemId.value = Number(item.id)
    viewState.value = 'doc'
    return item
  }

  function goHome() {
    selectedItemId.value = 0
    selectedFolderId.value = 0
    viewState.value = 'home'
  }

  function selectFolder(folderId) {
    selectedFolderId.value = folderId
  }

  function clearSelectedFolder() {
    selectedFolderId.value = 0
  }

  function toggleFolder(folderId) {
    const id = Number(folderId)
    if (expandedFolderIds.value.includes(id)) {
      expandedFolderIds.value = expandedFolderIds.value.filter((v) => v !== id)
    } else {
      expandedFolderIds.value = [...expandedFolderIds.value, id]
    }
  }

  function expandFolder(folderId) {
    const id = Number(folderId)
    if (!expandedFolderIds.value.includes(id)) {
      expandedFolderIds.value = [...expandedFolderIds.value, id]
    }
  }

  function expandAll() {
    expandedFolderIds.value = items.value
      .filter((item) => item.item_type === 'folder')
      .map((item) => Number(item.id))
  }

  function collapseAll() {
    expandedFolderIds.value = []
  }

  async function refresh() {
    await fetchTree()
  }

  async function renameItem(itemId, title, sourceUrl) {
    await renameKnowledgeItem(wsId.value, itemId, title, sourceUrl)
    const item = items.value.find((it) => Number(it.id) === Number(itemId))
    if (item) {
      item.title = title
      if (sourceUrl) item.source_url = sourceUrl
    }
  }

  // 本地重排 sort_order（与后端 MoveKnowledgeItem 的密集重编号一致）：先移除
  // 原父级，再按 prevId 插入目标父级并对全部兄弟按 1 起始重新编号，让切换
  // manual 排序后的首次渲染位置与最终结果一致，避免刷新前按旧 sort_order
  // 渲染导致文件先跳到列表最上方再跳回落点的抖动。
  function applyLocalMoveOrder(itemId, parentId, prevId) {
    const item = items.value.find((it) => Number(it.id) === Number(itemId))
    if (!item) return
    const oldParent = Number(item.parent_id)
    const siblings = (parent) =>
      items.value
        .filter(
          (it) =>
            Number(it.parent_id) === parent && Number(it.id) !== Number(itemId),
        )
        .sort(
          (a, b) =>
            Number(a.sort_order) - Number(b.sort_order) ||
            Number(a.id) - Number(b.id),
        )
    const targetList = siblings(parentId)
    let insertAt = targetList.length
    if (prevId === 0) insertAt = 0
    else if (prevId > 0) {
      const i = targetList.findIndex((it) => Number(it.id) === Number(prevId))
      if (i >= 0) insertAt = i + 1
    }
    item.parent_id = parentId
    if (oldParent !== Number(parentId)) {
      siblings(oldParent).forEach((it, i) => {
        if (Number(it.sort_order) !== i + 1) it.sort_order = i + 1
      })
    }
    targetList.splice(insertAt, 0, item)
    targetList.forEach((it, i) => {
      if (Number(it.sort_order) !== i + 1) it.sort_order = i + 1
    })
  }

  // prevId 为落点：插入到该兄弟节点之后（0=置顶，-1=追加到末尾）。
  async function moveItem(itemId, parentId, prevId = -1) {
    await moveKnowledgeItem(wsId.value, itemId, parentId, prevId)
    applyLocalMoveOrder(itemId, parentId, prevId)
    // 拖拽排序后进入手动排序模式，并重新拉树拿到最新 sort_order。
    sortKey.value = 'manual'
    persistSort()
    await refresh()
  }

  async function removeItem(itemId) {
    const removedIds = new Set([Number(itemId)])
    const queue = [Number(itemId)]
    while (queue.length) {
      const parentId = queue.shift()
      for (const item of items.value) {
        const id = Number(item.id)
        if (Number(item.parent_id) !== parentId || removedIds.has(id)) continue
        removedIds.add(id)
        queue.push(id)
      }
    }
    await deleteKnowledgeItem(wsId.value, itemId)
    items.value = items.value.filter((it) => !removedIds.has(Number(it.id)))
    expandedFolderIds.value = expandedFolderIds.value.filter((id) => !removedIds.has(Number(id)))
    if (removedIds.has(Number(selectedItemId.value))) goHome()
    else if (removedIds.has(Number(selectedFolderId.value))) clearSelectedFolder()
  }

  async function toggleFavorite(itemId) {
    const item = items.value.find((it) => Number(it.id) === Number(itemId))
    if (!item) return
    const next = !item.favorite
    if (next) await setKnowledgeFavorite(wsId.value, itemId)
    else await unsetKnowledgeFavorite(wsId.value, itemId)
    item.favorite = next
  }

  function upsertItem(item) {
    const index = items.value.findIndex((it) => Number(it.id) === Number(item.id))
    if (index >= 0) items.value[index] = item
    else items.value.push(item)
  }

  // childCounts maps parent folder id → number of direct children. Recomputed whenever
  // items change so the tree sidebar can display the live count badge next to each folder.
  const childCounts = computed(() => {
    const map = new Map()
    for (const item of items.value) {
      const parent = Number(item.parent_id)
      map.set(parent, (map.get(parent) || 0) + 1)
    }
    return map
  })

  // Returns the number of direct children for a folder (root is parent_id === 0).
  // Counts both folders and documents since the sidebar shows a single aggregate badge.
  function childCountOf(parentId) {
    return childCounts.value.get(Number(parentId)) || 0
  }

  function applySort(itemsToSort) {
    const key = sortKey.value
    const list = [...itemsToSort]
    list.sort((a, b) => {
      switch (key) {
        case 'manual':
          return (
            Number(a.sort_order) - Number(b.sort_order) ||
            Number(a.id) - Number(b.id)
          )
        case 'updated_desc':
          return Number(b.updated_at) - Number(a.updated_at)
        case 'updated_asc':
          return Number(a.updated_at) - Number(b.updated_at)
        case 'created_asc':
          return Number(a.created_at) - Number(b.created_at)
        case 'name_asc':
          return a.title.localeCompare(b.title, 'zh-CN')
        case 'name_desc':
          return b.title.localeCompare(a.title, 'zh-CN')
        case 'created_desc':
        default:
          return Number(b.created_at) - Number(a.created_at)
      }
    })
    return list
  }

  return {
    wsId,
    items,
    loading,
    viewState,
    selectedItemId,
    selectedFolderId,
    expandedFolderIds,
    sidebarWidth,
    sidebarCollapsed,
    sortKey,
    editSessionId,
    tree,
    selectedItem,
    selectedFolder,
    favoriteItemIds,
    hasCreate,
    hasManage,
    canEditItem,
    init,
    persistSort,
    persistSidebarWidth,
    fetchTree,
    openItem,
    goHome,
    selectFolder,
    clearSelectedFolder,
    toggleFolder,
    expandFolder,
    expandAll,
    collapseAll,
    refresh,
    renameItem,
    moveItem,
    removeItem,
    toggleFavorite,
    upsertItem,
    applySort,
    childCountOf,
  }
})

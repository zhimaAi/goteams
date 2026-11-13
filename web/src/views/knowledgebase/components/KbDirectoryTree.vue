<template>
  <div ref="treeEl" class="kb-tree" @mousedown="onTreeMousedown">
    <KbTreeNode
      v-for="node in children"
      :key="node.id"
      :node="node"
      :depth="0"
      @select="onSelect"
      @toggle="onToggle"
      @create="onCreate"
      @more="onMore"
    />
  </div>
</template>

<script setup>
import { onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { useKnowledgeStore } from '@/stores/knowledge'
import KbTreeNode from './KbTreeNode.vue'

defineProps({
  children: { type: Array, default: () => [] },
})

const emit = defineEmits(['select', 'toggle', 'create', 'more'])

const { t: $t } = useI18n()
const store = useKnowledgeStore()
const treeEl = ref(null)

const HOLD_MS = 300
const TIP_MS = 150
const MOVE_THRESHOLD = 5
const EXPAND_MS = 500

// 长按拖拽状态（非响应式，仅本次拖拽过程使用）。
let pressTimer = 0
let tipTimer = 0
let pending = null // { item, row, startX, startY }
let dragging = false
let dragItem = null
let ghost = null
let dropLine = null
let cursorX = 0
let cursorY = 0
let scrollTimer = 0
let suppressClick = false
let expandTimer = 0
let expandFolderId = 0

function findItem(itemId) {
  return store.items.find((it) => Number(it.id) === Number(itemId)) || null
}

function onTreeMousedown(e) {
  if (e.button !== 0) return
  if (e.target.closest('button, input, a, .kb-node-actions')) return
  const row = e.target.closest('.kb-tree-row')
  if (!row) return
  const item = findItem(Number(row.dataset.kbItemId))
  if (!item || !store.canEditItem(item)) return
  cursorX = e.clientX
  cursorY = e.clientY
  pending = { item, row, startX: e.clientX, startY: e.clientY }
  row.style.cursor = 'grabbing'
  pressTimer = window.setTimeout(beginDrag, HOLD_MS)
  tipTimer = window.setTimeout(() => row.classList.add('hold-tip'), TIP_MS)
  window.addEventListener('mousemove', onWindowMousemove)
  window.addEventListener('mouseup', onWindowMouseup)
  window.addEventListener('keydown', onWindowKeydown)
  window.addEventListener('blur', onWindowBlur)
}

function onWindowMousemove(e) {
  cursorX = e.clientX
  cursorY = e.clientY
  if (dragging) {
    moveGhost()
    updateDropTarget()
    return
  }
  if (!pending) return
  const dx = e.clientX - pending.startX
  const dy = e.clientY - pending.startY
  // 按下后移动超过阈值立即开始拖拽（无需等满长按时长），避免手势被静默取消。
  if (Math.hypot(dx, dy) > MOVE_THRESHOLD) beginDrag()
}

function beginDrag() {
  if (!pending || dragging) return
  window.clearTimeout(pressTimer)
  window.clearTimeout(tipTimer)
  dragItem = pending.item
  dragging = true
  suppressClick = true
  pending.row.classList.remove('hold-tip')
  ghost = document.createElement('div')
  ghost.className = 'kb-drag-ghost'
  ghost.textContent = dragItem.title
  document.body.appendChild(ghost)
  document.body.style.cursor = 'grabbing'
  document.body.style.userSelect = 'none'
  pending.row.classList.add('drag-source')
  dropLine = document.createElement('div')
  dropLine.className = 'kb-drop-line'
  treeEl.value?.appendChild(dropLine)
  moveGhost()
  scrollTimer = window.setInterval(autoScrollTree, 120)
}

function moveGhost() {
  if (!ghost) return
  ghost.style.left = `${cursorX}px`
  ghost.style.top = `${cursorY}px`
}

// 更新放置目标：文件行上/下半区分别插到该文件前/后；树内空白区追加到根目录末尾。
// 拖动文件悬停到文件夹行任意位置均视为“移入文件夹”（整行落点，仅高亮不显示横线）；
// 拖动文件夹仍为中部移入、上下边缘同级排序；悬停未展开文件夹会延时自动展开。
// 插入线：根目录同级插入为全宽横线，文件夹内部插入为从行内容缩进处起的满宽横线（截掉左侧）。
// 返回 { folderId, prevId } | null；prevId：-1=追加到末尾，>0=插入到该兄弟之后，0=置顶。
function updateDropTarget() {
  const el = treeEl.value
  if (!el) return null
  const rows = el.querySelectorAll('.kb-tree-row')
  let hitRow = null
  for (const row of rows) {
    const rect = row.getBoundingClientRect()
    const hit =
      cursorX >= rect.left &&
      cursorX <= rect.right &&
      cursorY >= rect.top &&
      cursorY <= rect.bottom
    if (!hit) continue
    hitRow = row
    break
  }
  for (const row of rows) row.classList.remove('drag-target')
  const treeRect = el.getBoundingClientRect()
  const insideTree =
    cursorX >= treeRect.left &&
    cursorX <= treeRect.right &&
    cursorY >= treeRect.top &&
    cursorY <= treeRect.bottom

  let result = null
  if (hitRow) {
    const targetItem = findItem(Number(hitRow.dataset.kbItemId))
    const rect = hitRow.getBoundingClientRect()
    const ratio = (cursorY - rect.top) / rect.height
    const isFolder = targetItem?.item_type === 'folder'
    // 拖动文件时文件夹整行都是“移入”落点，避免空文件夹只有中部窄区间可落。
    const draggingFile = dragItem?.item_type !== 'folder'
    const moveIntoFolder =
      isFolder && (draggingFile || (ratio >= 0.25 && ratio <= 0.75))
    if (!targetItem || Number(targetItem.id) === Number(dragItem?.id)) {
      clearExpandTimer()
      hideDropLine()
    } else if (moveIntoFolder) {
      // 悬停文件夹仅整行高亮提示“移入”，不显示横线。
      hitRow.classList.add('drag-target')
      hideDropLine()
      result = { folderId: Number(targetItem.id), prevId: -1 }
      scheduleExpand(targetItem.id)
    } else {
      clearExpandTimer()
      const insertBefore = ratio < 0.5
      result = siblingDropTarget(targetItem, insertBefore)
      const lineTop =
        (insertBefore ? rect.top : rect.bottom) -
        treeRect.top +
        el.scrollTop -
        1
      // 根目录同级插入用全宽横线；文件夹内部插入用从缩进处起的满宽横线。
      if (Number(targetItem.parent_id) === 0) {
        useFullDropLine()
        moveDropLine(lineTop)
      } else {
        showIndentDropLine(hitRow, el, treeRect, lineTop)
      }
    }
  } else if (insideTree) {
    clearExpandTimer()
    useFullDropLine()
    moveDropLine(el.scrollHeight - 14)
    result = { folderId: 0, prevId: -1 }
  } else {
    clearExpandTimer()
    hideDropLine()
  }
  return result
}

// 将“插到目标前面”换算为后端使用的 prevId；计算时排除当前拖动项。
function siblingDropTarget(targetItem, insertBefore) {
  const folderId = Number(targetItem.parent_id)
  if (!insertBefore) {
    return { folderId, prevId: Number(targetItem.id) }
  }
  const siblingIds = store.items
    .filter(
      (item) =>
        Number(item.parent_id) === folderId &&
        Number(item.id) !== Number(dragItem?.id),
    )
    .sort(
      (a, b) =>
        Number(a.sort_order) - Number(b.sort_order) ||
        Number(a.id) - Number(b.id),
    )
    .map((item) => Number(item.id))
  const targetIndex = siblingIds.indexOf(Number(targetItem.id))
  return {
    folderId,
    prevId: targetIndex <= 0 ? 0 : siblingIds[targetIndex - 1],
  }
}

function moveDropLine(top) {
  if (!dropLine) return
  dropLine.style.top = `${top}px`
  dropLine.style.display = 'block'
}

function hideDropLine() {
  if (dropLine) dropLine.style.display = 'none'
}

// 恢复为同级插入/根目录追加的全宽横线，并清除缩进横线残留的 inline left。
function useFullDropLine() {
  if (!dropLine) return
  dropLine.className = 'kb-drop-line'
  dropLine.style.left = ''
}

// 文件夹内部的插入横线：左侧从行内容缩进处起（截掉左侧），延伸到右边缘，
// 与根目录从容器左侧开始的全宽横线区分，提示落点在文件夹内部。
function showIndentDropLine(row, el, treeRect, top) {
  if (!dropLine) return
  const rowRect = row.getBoundingClientRect()
  const paddingLeft = Number.parseInt(row.style.paddingLeft, 10) || 8
  dropLine.className = 'kb-drop-line'
  dropLine.style.left = `${rowRect.left - treeRect.left + paddingLeft + 45}px`
  dropLine.style.top = `${top}px`
  dropLine.style.display = 'block'
}

// 悬停未展开文件夹延时自动展开，帮助用户看清落点；移出目标或拖拽结束立即取消。
function scheduleExpand(folderId) {
  const id = Number(folderId)
  if (expandFolderId === id && expandTimer) return
  window.clearTimeout(expandTimer)
  expandTimer = 0
  expandFolderId = id
  if (store.expandedFolderIds.includes(id)) return
  expandTimer = window.setTimeout(() => {
    expandTimer = 0
    store.expandFolder(id)
  }, EXPAND_MS)
}

function clearExpandTimer() {
  window.clearTimeout(expandTimer)
  expandTimer = 0
  expandFolderId = 0
}

function autoScrollTree() {
  const el = treeEl.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  if (cursorY < rect.top + 24) el.scrollTop -= 24
  else if (cursorY > rect.bottom - 24) el.scrollTop += 24
}

async function onWindowMouseup(e) {
  // 直接用 mouseup 事件坐标，避免依赖最后一次 mousemove。
  cursorX = e.clientX
  cursorY = e.clientY
  if (!dragging) {
    cancelPending()
    return
  }
  const target = updateDropTarget()
  const item = dragItem
  cleanupDrag()
  if (!target || !item) return
  const folderId = target.folderId ?? 0
  const prevId = target.prevId ?? -1
  // 拖回自身行无实际移动，跳过请求。
  if (prevId === Number(item.id)) return
  // 落点即当前位置时跳过请求（与后端同文件夹无变化判断一致）。
  if (isSamePosition(item, folderId, prevId)) return
  try {
    await store.moveItem(item.id, folderId, prevId)
    message.success($t('kb_moveSuccess'))
  } catch {
    // error toast handled by the request interceptor
  }
}

// 判断落点与当前所在位置是否一致：按 sort_order 升序、id 升序取兄弟列表，
// 计算出插入位与当前位相同且父级未变即视为无操作（与后端判断一致）。
function isSamePosition(item, folderId, prevId) {
  if (Number(item.parent_id) !== Number(folderId)) return false
  const siblings = store.items
    .filter((it) => Number(it.parent_id) === Number(folderId))
    .sort(
      (a, b) =>
        Number(a.sort_order) - Number(b.sort_order) ||
        Number(a.id) - Number(b.id),
    )
    .map((it) => Number(it.id))
  const curIdx = siblings.findIndex((id) => id === Number(item.id))
  if (curIdx < 0) return false
  const withoutItem = siblings.filter((id) => id !== Number(item.id))
  let insertAt = withoutItem.length
  if (prevId === 0) insertAt = 0
  else if (prevId > 0) {
    const i = withoutItem.findIndex((id) => id === Number(prevId))
    insertAt = i < 0 ? withoutItem.length : i + 1
  }
  return curIdx === insertAt
}

function onWindowKeydown(e) {
  if (e.key !== 'Escape') return
  if (dragging) suppressClick = true
  cancelDrag()
}

function onWindowBlur() {
  cancelDrag()
}

function cancelPending() {
  window.clearTimeout(pressTimer)
  window.clearTimeout(tipTimer)
  if (pending) {
    pending.row.classList.remove('hold-tip')
    pending.row.style.cursor = ''
  }
  pending = null
  removeListeners()
}

function cancelDrag() {
  if (dragging) cleanupDrag()
  else cancelPending()
}

function cleanupDrag() {
  window.clearTimeout(pressTimer)
  window.clearTimeout(tipTimer)
  window.clearInterval(scrollTimer)
  scrollTimer = 0
  clearExpandTimer()
  if (pending) {
    pending.row.classList.remove('hold-tip')
    pending.row.style.cursor = ''
  }
  pending = null
  dragging = false
  dragItem = null
  ghost?.remove()
  ghost = null
  dropLine?.remove()
  dropLine = null
  document.body.style.cursor = ''
  document.body.style.userSelect = ''
  treeEl.value
    ?.querySelectorAll('.drag-target, .drag-source, .hold-tip')
    .forEach((row) =>
      row.classList.remove('drag-target', 'drag-source', 'hold-tip'),
    )
  removeListeners()
}

function removeListeners() {
  window.removeEventListener('mousemove', onWindowMousemove)
  window.removeEventListener('mouseup', onWindowMouseup)
  window.removeEventListener('keydown', onWindowKeydown)
  window.removeEventListener('blur', onWindowBlur)
}

// 拖拽结束后抑制紧随其后的 click，避免误触发行选中。
function onWindowClickCapture(e) {
  if (!suppressClick) return
  suppressClick = false
  e.stopPropagation()
  e.preventDefault()
}
window.addEventListener('click', onWindowClickCapture, true)

onBeforeUnmount(() => {
  window.removeEventListener('click', onWindowClickCapture, true)
  cleanupDrag()
})

function onSelect(node) {
  emit('select', node)
}
function onToggle(node) {
  emit('toggle', node)
}
function onCreate(node) {
  emit('create', node)
}
function onMore(payload) {
  emit('more', payload)
}
</script>

<style scoped lang="less">
.kb-tree {
  position: relative;
  flex: 1;
  overflow-y: auto;
  padding: 0 0 12px;

  // 长按前 150ms 的行提示，让用户感知长按已生效。
  :deep(.kb-tree-row.hold-tip) {
    background: #e6f4ff;
  }

  :deep(.kb-tree-row.drag-source) {
    opacity: 0.4;
  }

  // 文件夹中部为“移入”落点，上下边缘仍用于同级排序。
  :deep(.kb-tree-row.drag-target) {
    position: relative;
    background: #e6f4ff;
    box-shadow: inset 0 0 0 1px #1677ff;
  }

  // 放置位置横线：指示同级插入位置或根目录末尾。
  :deep(.kb-drop-line) {
    position: absolute;
    left: 8px;
    right: 8px;
    height: 2px;
    border-radius: 1px;
    background: #1677ff;
    box-shadow: 0 1px 2px rgb(22 119 255 / 40%);
    pointer-events: none;
    display: none;
    z-index: 3;
  }
}
</style>

<style lang="less">
.kb-drag-ghost {
  position: fixed;
  left: 0;
  top: 0;
  z-index: 2000;
  max-width: 240px;
  padding: 4px 10px;
  border-radius: 4px;
  background: #1677ff;
  color: #fff;
  font-size: 12px;
  line-height: 18px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  box-shadow: 0 2px 8px rgb(0 0 0 / 18%);
  pointer-events: none;
  transform: translate(10px, 10px);
}
</style>

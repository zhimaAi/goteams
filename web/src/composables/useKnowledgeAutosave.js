import { ref } from 'vue'
import { saveKnowledgeDocumentContent } from '@/api/knowledgeBase'

const SAVE_DEBOUNCE = 2000
const SAVE_MAX_WAIT = 10000

/**
 * Server autosave scheduler for one document:
 * 2s debounce / 10s maxWait, one in-flight request at a time.
 * A 409 stops the timer and switches to the `conflict` state until the user
 * resolves it (load server content / save as copy / keep local draft).
 */
export function useKnowledgeAutosave(wsId) {
  const status = ref('saved') // saved | unsaved | saving | error | conflict
  const revision = ref(0)
  const serverHash = ref('')
  const updatedAt = ref(0)
  const updatedBy = ref(0)
  const conflict = ref(null) // { current_revision, updated_at, updated_by }
  const lastError = ref(null)

  let documentId = 0
  let editSessionId = ''
  let getPayload = null
  let dirty = false
  let inFlightPromise = null
  let paused = false
  let debounceTimer = null
  let maxWaitTimer = null

  function clearTimers() {
    clearTimeout(debounceTimer)
    clearTimeout(maxWaitTimer)
    debounceTimer = null
    maxWaitTimer = null
  }

  function start(docId, sessionId, baseRevision, baseHash) {
    documentId = docId
    editSessionId = sessionId
    revision.value = baseRevision
    serverHash.value = baseHash
    updatedAt.value = 0
    updatedBy.value = 0
    conflict.value = null
    lastError.value = null
    getPayload = null
    dirty = false
    inFlightPromise = null
    paused = false
    clearTimers()
    status.value = 'saved'
  }

  // Track a new edit. getPayload must return { title, content, contentMode }
  // and is evaluated lazily at send time so the latest state is always sent.
  function markDirty(payloadGetter) {
    getPayload = payloadGetter
    if (paused || !documentId) return
    dirty = true
    if (status.value !== 'saving') status.value = 'unsaved'
    clearTimeout(debounceTimer)
    debounceTimer = setTimeout(() => void flush(), SAVE_DEBOUNCE)
    if (!maxWaitTimer) {
      maxWaitTimer = setTimeout(() => void flush(), SAVE_MAX_WAIT)
    }
  }

  async function flush() {
    if (paused || !documentId) return false
    if (inFlightPromise) return inFlightPromise

    const drainPromise = (async () => {
      // Drain edits serially. The loop exits when content is clean, editing is
      // paused, or a request reaches an error/conflict state.
      while (!paused && documentId) {
        if (!dirty) return status.value === 'saved'
        const payload = getPayload ? getPayload() : null
        if (!payload) return false
        clearTimers()
        dirty = false
        status.value = 'saving'
        try {
          const res = await saveKnowledgeDocumentContent(wsId, documentId, {
            revision: revision.value,
            title: payload.title,
            content: payload.content,
            content_mode: payload.contentMode,
            edit_session_id: editSessionId,
          })
          revision.value = res.revision
          serverHash.value = res.content_hash
          updatedAt.value = res.updated_at
          updatedBy.value = res.updated_by
          status.value = dirty ? 'unsaved' : 'saved'
        } catch (error) {
          if (error?.response?.status === 409) {
            const data = error.response.data?.data || {}
            conflict.value = {
              current_revision: data.current_revision,
              updated_at: data.updated_at,
              updated_by: data.updated_by,
            }
            // Stop overwriting the server; keep the local content as a draft.
            status.value = 'conflict'
            paused = true
            dirty = true
          } else {
            lastError.value = error
            status.value = 'error'
            dirty = true
          }
          return false
        }
      }
      return false
    })()
    inFlightPromise = drainPromise
    try {
      return await drainPromise
    } finally {
      if (inFlightPromise === drainPromise) inFlightPromise = null
    }
  }

  // Resume autosave after the user resolved a conflict / error.
  function resume() {
    conflict.value = null
    lastError.value = null
    paused = false
    if (dirty) {
      clearTimeout(debounceTimer)
      debounceTimer = setTimeout(() => void flush(), SAVE_DEBOUNCE)
    } else {
      status.value = 'saved'
    }
  }

  // End editing: cancel timers but keep dirty state so a later start() resets.
  function stop() {
    clearTimers()
    paused = true
  }

  return {
    status,
    revision,
    serverHash,
    updatedAt,
    updatedBy,
    conflict,
    lastError,
    start,
    markDirty,
    flush,
    resume,
    stop,
  }
}

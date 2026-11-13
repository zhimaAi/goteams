import localforage from 'localforage'

// Device-local draft store. Writes go through a single promise queue so that
// switching documents (flush before switch) can never interleave A's content
// into B's record.
const DRAFT_STORE = localforage.createInstance({
  name: 'goteams',
  storeName: 'knowledge_document_drafts',
})

const DRAFT_SCHEMA_VERSION = 1
const WRITE_DEBOUNCE = 300
const WRITE_MAX_WAIT = 2000
const UNSYNCED_TTL = 30 * 24 * 60 * 60 * 1000
const SYNCED_TTL = 3 * 24 * 60 * 60 * 1000

let opQueue = Promise.resolve()

function enqueue(task) {
  const run = opQueue.then(task, task)
  opQueue = run.catch(() => {})
  return run
}

/**
 * Draft persistence for one user + workspace.
 * schedule(documentId, getRecord) debounces the write (300ms, max 2s);
 * flush(documentId) forces an immediate durable write; read() restores it.
 */
export function useKnowledgeDraft({ userId, wsId }) {
  const pending = new Map() // documentId -> () => record
  const timers = new Map() // documentId -> { debounce, maxWait }

  function key(documentId) {
    return `kb-draft-${userId}-${wsId}-${documentId}`
  }

  function clearTimers(documentId) {
    const state = timers.get(documentId)
    if (state) {
      clearTimeout(state.debounce)
      clearTimeout(state.maxWait)
      timers.delete(documentId)
    }
  }

  async function writeNow(documentId) {
    const getRecord = pending.get(documentId)
    clearTimers(documentId)
    pending.delete(documentId)
    if (!getRecord) return
    const record = getRecord()
    if (!record) return
    record.savedAt = Date.now()
    return enqueue(() => DRAFT_STORE.setItem(key(documentId), record))
  }

  function schedule(documentId, getRecord) {
    pending.set(documentId, getRecord)
    let state = timers.get(documentId)
    if (!state) {
      state = { debounce: null, maxWait: null }
      timers.set(documentId, state)
    }
    clearTimeout(state.debounce)
    state.debounce = setTimeout(() => writeNow(documentId), WRITE_DEBOUNCE)
    if (!state.maxWait) {
      state.maxWait = setTimeout(() => writeNow(documentId), WRITE_MAX_WAIT)
    }
  }

  async function flush(documentId) {
    await writeNow(documentId)
    await opQueue
  }

  async function read(documentId) {
    return enqueue(async () => {
      const record = await DRAFT_STORE.getItem(key(documentId))
      if (!record) return null
      const ttl = record.synced ? SYNCED_TTL : UNSYNCED_TTL
      if (Date.now() - (record.savedAt || 0) > ttl) {
        await DRAFT_STORE.removeItem(key(documentId))
        return null
      }
      return record
    })
  }

  async function remove(documentId) {
    clearTimers(documentId)
    pending.delete(documentId)
    return enqueue(() => DRAFT_STORE.removeItem(key(documentId)))
  }

  function buildRecord({ documentId, title, content, contentMode, baseRevision, serverHash, synced }) {
    return {
      schemaVersion: DRAFT_SCHEMA_VERSION,
      key: key(documentId),
      userId: String(userId),
      workspaceId: String(wsId),
      documentId,
      title,
      content,
      contentMode,
      baseRevision,
      serverHash,
      savedAt: Date.now(),
      synced: !!synced,
    }
  }

  return { schedule, flush, read, remove, buildRecord }
}

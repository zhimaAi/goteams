import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getUnreadNotificationCount, markAllNotificationsRead, markNotificationsRead } from '@/api/notification'

export const useNotificationStore = defineStore('notifications', () => {
  const unreadCount = ref(0)
  let pollingTimer = null
  let loadingPromise = null

  async function refreshCount() {
    if (loadingPromise) return loadingPromise
    loadingPromise = getUnreadNotificationCount()
      .then((data) => { unreadCount.value = Number(data?.count || 0); return unreadCount.value })
      .catch(() => unreadCount.value)
      .finally(() => { loadingPromise = null })
    return loadingPromise
  }

  function startPolling() {
    if (pollingTimer) return
    void refreshCount()
    pollingTimer = window.setInterval(() => { void refreshCount() }, 15000)
  }

  function stopPolling() {
    if (pollingTimer) window.clearInterval(pollingTimer)
    pollingTimer = null
  }

  async function markRead(ids) {
    const normalized = [...new Set((ids || []).map(Number).filter(Boolean))]
    if (!normalized.length) return
    await markNotificationsRead(normalized)
    await refreshCount()
  }

  async function markAllRead() {
    await markAllNotificationsRead()
    unreadCount.value = 0
  }

  function clear() {
    unreadCount.value = 0
    stopPolling()
    loadingPromise = null
  }

  return { unreadCount, refreshCount, startPolling, stopPolling, markRead, markAllRead, clear }
})

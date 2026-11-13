import { request } from './request'

export function listNotifications(params = {}) {
  return request({ url: '/api/notifications', method: 'GET', params })
}

export function getUnreadNotificationCount() {
  return request({ url: '/api/notifications/unread-count', method: 'GET', silent: true })
}

export function markNotificationsRead(ids) {
  return request({ url: '/api/notifications/read', method: 'POST', data: { ids } })
}

export function markAllNotificationsRead() {
  return request({ url: '/api/notifications/read-all', method: 'POST' })
}

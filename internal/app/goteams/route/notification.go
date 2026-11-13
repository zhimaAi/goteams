package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegNotificationRoutes registers authenticated notification APIs.
func RegNotificationRoutes(r lib_web.Route) {
	r[http.MethodGet][`/api/notifications`] = wrapAction(withAuth(), controller.ListNotifications)
	r[http.MethodGet][`/api/notifications/unread-count`] = wrapAction(withAuth(), controller.UnreadNotificationCount)
	r[http.MethodPost][`/api/notifications/read`] = wrapAction(withAuth(), controller.MarkNotificationsRead)
	r[http.MethodPost][`/api/notifications/read-all`] = wrapAction(withAuth(), controller.MarkAllNotificationsRead)
}

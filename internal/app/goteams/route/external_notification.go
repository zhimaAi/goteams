package route

import (
	"net/http"

	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
)

// RegExternalNotificationRoutes registers enterprise notification strategy routes.
// Strategies are configured once per enterprise and push for every project.
func RegExternalNotificationRoutes(r lib_web.Route) {
	r[http.MethodGet][`/api/enterprise/external-notification/user-mappings`] = wrapAction(withPerm("external_notification:manage"), controller.ListNotificationUserMappings)
	r[http.MethodPut][`/api/enterprise/external-notification/user-mappings`] = wrapAction(withPerm("external_notification:manage"), controller.SaveNotificationUserMappings)
	prefix := `/api/enterprise/external-notification/strategies`
	r[http.MethodGet][prefix] = wrapAction(withPerm("external_notification:manage"), controller.ListExternalNotificationStrategies)
	r[http.MethodPost][prefix] = wrapAction(withPerm("external_notification:manage"), controller.CreateExternalNotificationStrategy)
	r[http.MethodPut][prefix+`/:strategyId`] = wrapAction(withPerm("external_notification:manage"), controller.UpdateExternalNotificationStrategy)
	r[http.MethodPut][prefix+`/:strategyId/status`] = wrapAction(withPerm("external_notification:manage"), controller.UpdateExternalNotificationStrategyStatus)
	r[http.MethodPost][prefix+`/:strategyId/test`] = wrapAction(withPerm("external_notification:manage"), controller.TestExternalNotificationStrategy)
	r[http.MethodDelete][prefix+`/:strategyId`] = wrapAction(withPerm("external_notification:manage"), controller.DeleteExternalNotificationStrategy)
}

// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"net/http"
	"strconv"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/controller"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
	"goteams/internal/pkg/lib_web"

	"github.com/gin-gonic/gin"
)

// RegDashboardRoutes registers project dashboard routes behind the dashboard application switch.
func RegDashboardRoutes(r lib_web.Route) {
	prefix := `/api/workspaces/:wsId/dashboards`
	view := withDashboard(define.PermissionDashboardView)
	create := withDashboard(define.PermissionDashboardCreate)

	r[http.MethodGet][prefix] = wrapAction(view, controller.ListDashboards)
	r[http.MethodPost][prefix] = wrapAction(create, controller.CreateDashboard)
	r[http.MethodPut][prefix+`/order`] = wrapAction(view, controller.ReorderDashboards)
	r[http.MethodGet][prefix+`/:dashboardId`] = wrapAction(view, controller.GetDashboard)
	r[http.MethodPut][prefix+`/:dashboardId`] = wrapAction(view, controller.UpdateDashboard)
	r[http.MethodDelete][prefix+`/:dashboardId`] = wrapAction(view, controller.DeleteDashboard)
	r[http.MethodGet][prefix+`/:dashboardId/permission`] = wrapAction(view, controller.GetDashboardPermission)
	r[http.MethodPut][prefix+`/:dashboardId/permission`] = wrapAction(view, controller.ReplaceDashboardPermission)
	r[http.MethodPost][prefix+`/:dashboardId/save-as`] = wrapAction(create, controller.SaveDashboardAs)
	r[http.MethodPut][prefix+`/:dashboardId/layout`] = wrapAction(view, controller.ReplaceDashboardLayout)
	r[http.MethodPost][prefix+`/:dashboardId/cards`] = wrapAction(view, controller.CreateDashboardCard)
	r[http.MethodPut][prefix+`/:dashboardId/cards/:cardId`] = wrapAction(view, controller.UpdateDashboardCard)
	r[http.MethodDelete][prefix+`/:dashboardId/cards/:cardId`] = wrapAction(view, controller.DeleteDashboardCard)
	r[http.MethodPost][prefix+`/:dashboardId/cards/:cardId/copy`] = wrapAction(view, controller.CopyDashboardCard)
	r[http.MethodPost][prefix+`/:dashboardId/charts/query`] = wrapAction(view, controller.QueryDashboardCharts)
	r[http.MethodPost][prefix+`/:dashboardId/cards/:cardId/copy-many`] = wrapAction(view, controller.CopyDashboardCards)
	r[http.MethodPost][prefix+`/:dashboardId/metrics/query`] = wrapAction(view, controller.QueryDashboardMetrics)
	r[http.MethodGet][prefix+`/:dashboardId/sla-options`] = wrapAction(view, controller.GetDashboardSLAOptions)
	r[http.MethodGet][prefix+`/:dashboardId/user-state`] = wrapAction(view, controller.GetDashboardUserState)
	r[http.MethodPut][prefix+`/:dashboardId/user-state`] = wrapAction(view, controller.UpdateDashboardUserState)
	r[http.MethodPost][prefix+`/:dashboardId/uploads/image`] = wrapAction(view, controller.UploadDashboardImage)
	r[http.MethodPost][prefix+`/:dashboardId/uploads/file`] = wrapAction(view, controller.UploadDashboardFile)
	r[http.MethodPost][prefix+`/:dashboardId/uploads/video`] = wrapAction(view, controller.UploadDashboardVideo)

	announcementPrefix := `/api/workspaces/:wsId/dashboard-announcement`
	r[http.MethodGet][announcementPrefix] = wrapAction(view, controller.GetDashboardAnnouncement)
	r[http.MethodPut][announcementPrefix] = wrapAction(view, controller.UpdateDashboardAnnouncement)
}

// dashboardFeatureMiddleware rejects dashboard requests while the application is disabled.
func dashboardFeatureMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := lib_i18n.GetLang(c)
		workspaceId, err := strconv.Atoi(c.Param(`wsId`))
		if err != nil || workspaceId <= 0 {
			c.JSON(http.StatusBadRequest, define.Response{
				Code:    http.StatusBadRequest,
				Message: lib_i18n.Show(lang, define.MsgParamInvalid),
			})
			c.Abort()
			return
		}
		enabled, err := business.IsWorkspaceApplicationEnabled(c.GetInt(`adminId`), workspaceId, define.AppKeyDashboard)
		if err != nil {
			c.JSON(http.StatusInternalServerError, define.Response{
				Code:    http.StatusInternalServerError,
				Message: lib_i18n.Show(lang, define.MsgError),
			})
			c.Abort()
			return
		}
		if !enabled {
			c.JSON(http.StatusForbidden, define.Response{
				Code:    http.StatusForbidden,
				Message: lib_i18n.Show(lang, define.MsgDashboardFeatureDisabled),
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// withDashboard applies authentication, the dashboard application switch, and one project permission.
func withDashboard(permKey string) gin.HandlerFunc {
	return chainMiddleware(common.AuthMiddleware(), dashboardFeatureMiddleware(), business.PermissionMiddleware(permKey))
}

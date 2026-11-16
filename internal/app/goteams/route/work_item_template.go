// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"net/http"

	"goteams/internal/app/goteams/controller"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_web"

	"github.com/gin-gonic/gin"
)

var workItemTemplateWritePerm = map[string]string{
	define.TypeKeyRequirement: `requirement_setting:field`,
	define.TypeKeyDefect:      `defect_setting:field`,
}

func templateManagementRead(use, manage gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Query(`manage`) == `1` {
			manage(c)
			return
		}
		use(c)
	}
}

// RegWorkItemTemplateRoutes registers work-item template routes.
func RegWorkItemTemplateRoutes(r lib_web.Route) {
	prefix := `/api/workspaces/:wsId/work-item-templates`
	write := withTypeScopedPerm(workItemTemplateWritePerm, `requirement_setting:field`)

	read := templateManagementRead(withWorkspaceMember(), write)
	r[http.MethodGet][`/api/workspaces/:wsId/roles/template-options`] = wrapAction(
		withAnyPerm(workItemTemplateWritePerm[define.TypeKeyRequirement], workItemTemplateWritePerm[define.TypeKeyDefect], define.PermissionTicketManage),
		controller.ListTemplateRoleOptions,
	)
	r[http.MethodGet][prefix] = wrapAction(read, controller.ListWorkItemTemplates)
	r[http.MethodPost][prefix] = wrapAction(write, controller.CreateWorkItemTemplate)
	r[http.MethodPut][prefix+`/settings`] = wrapAction(write, controller.UpdateWorkItemTemplateSettings)
	r[http.MethodGet][prefix+`/display-settings`] = wrapAction(withWorkspaceMember(), controller.GetWorkItemDisplaySettings)
	r[http.MethodPut][prefix+`/display-settings`] = wrapAction(write, controller.UpdateWorkItemDisplaySettings)
	r[http.MethodGet][prefix+`/pool`] = wrapAction(write, controller.ListWorkItemTemplatePool)
	r[http.MethodPost][prefix+`/link`] = wrapAction(write, controller.LinkWorkItemTemplates)
	r[http.MethodDelete][prefix+`/pool/:id`] = wrapAction(write, controller.DeleteWorkItemTemplatePool)
	r[http.MethodGet][prefix+`/:id`] = wrapAction(read, controller.GetWorkItemTemplate)
	r[http.MethodPut][prefix+`/:id`] = wrapAction(write, controller.UpdateWorkItemTemplate)
	r[http.MethodPatch][prefix+`/:id/visibility`] = wrapAction(write, controller.UpdateWorkItemTemplateVisibility)
	r[http.MethodDelete][prefix+`/:id`] = wrapAction(write, controller.DeleteWorkItemTemplate)
	r[http.MethodDelete][prefix+`/:id/link`] = wrapAction(write, controller.UnlinkWorkItemTemplate)
}

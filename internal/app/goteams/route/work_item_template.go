// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

var workItemTemplateWritePerm = map[string]string{
	define.TypeKeyRequirement: `requirement_setting:field`,
	define.TypeKeyDefect:      `defect_setting:field`,
}

// RegWorkItemTemplateRoutes registers work-item template routes.
func RegWorkItemTemplateRoutes(r lib_web.Route) {
	prefix := `/api/workspaces/:wsId/work-item-templates`
	write := withTypeScopedPerm(workItemTemplateWritePerm, `requirement_setting:field`)

	r[http.MethodGet][prefix] = wrapAction(withWorkspaceMember(), controller.ListWorkItemTemplates)
	r[http.MethodPost][prefix] = wrapAction(write, controller.CreateWorkItemTemplate)
	r[http.MethodPut][prefix+`/settings`] = wrapAction(write, controller.UpdateWorkItemTemplateSettings)
	r[http.MethodGet][prefix+`/display-settings`] = wrapAction(withWorkspaceMember(), controller.GetWorkItemDisplaySettings)
	r[http.MethodPut][prefix+`/display-settings`] = wrapAction(write, controller.UpdateWorkItemDisplaySettings)
	r[http.MethodGet][prefix+`/pool`] = wrapAction(write, controller.ListWorkItemTemplatePool)
	r[http.MethodPost][prefix+`/link`] = wrapAction(write, controller.LinkWorkItemTemplates)
	r[http.MethodDelete][prefix+`/pool/:id`] = wrapAction(write, controller.DeleteWorkItemTemplatePool)
	r[http.MethodGet][prefix+`/:id`] = wrapAction(withWorkspaceMember(), controller.GetWorkItemTemplate)
	r[http.MethodPut][prefix+`/:id`] = wrapAction(write, controller.UpdateWorkItemTemplate)
	r[http.MethodDelete][prefix+`/:id`] = wrapAction(write, controller.DeleteWorkItemTemplate)
	r[http.MethodDelete][prefix+`/:id/link`] = wrapAction(write, controller.UnlinkWorkItemTemplate)
}

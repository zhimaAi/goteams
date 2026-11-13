// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegWorkItemViewRoutes registers requirement/defect list view routes.
// The list route dispatches the view permission by ?type=; write routes only
// require any module view permission here — ownership and the system-view
// manage permission are enforced in the controller because the target module
// comes from the body or the stored view row.
func RegWorkItemViewRoutes(r lib_web.Route) {
	prefix := `/api/workspaces/:wsId/views`
	scoped := map[string]string{
		define.TypeKeyRequirement: `requirement:view`,
		define.TypeKeyDefect:      `defect:view`,
		define.TypeKeyIteration:   `version:view`,
		define.TypeKeyGantt:       `gantt:view`,
	}
	viewPermissions := []string{`requirement:view`, `defect:view`, `version:view`, `gantt:view`}

	r[http.MethodGet][prefix] = wrapAction(withTypeScopedPerm(scoped, viewPermissions...), controller.ListWorkItemViews)
	r[http.MethodPost][prefix] = wrapAction(withAnyPerm(viewPermissions...), controller.CreateWorkItemView)
	r[http.MethodPut][prefix+`/:viewId`] = wrapAction(withAnyPerm(viewPermissions...), controller.UpdateWorkItemView)
	r[http.MethodDelete][prefix+`/:viewId`] = wrapAction(withAnyPerm(viewPermissions...), controller.DeleteWorkItemView)
	r[http.MethodPut][`/api/workspaces/:wsId/view-settings`] = wrapAction(withAnyPerm(viewPermissions...), controller.SaveUserViewSettings)
}

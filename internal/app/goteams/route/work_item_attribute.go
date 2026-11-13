// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// planningAttributePerm maps the work item attribute config permission; planning has no such tab.
var planningAttributePerm = map[string]string{}

// RegWorkItemAttributeRoutes registers work item attribute read/save routes (requirement/defect only).
func RegWorkItemAttributeRoutes(r lib_web.Route) {
	prefix := `/api/workspaces/:wsId/work-item-attributes`

	// Any workspace member may read the config so names/icons render for everyone.
	r[http.MethodGet][prefix] = wrapAction(withWorkspaceMember(), controller.GetWorkItemAttributes)
	r[http.MethodPut][prefix] = wrapAction(withTypeScopedPerm(planningAttributePerm,
		"requirement_setting:attribute", "defect_setting:attribute"), controller.SaveWorkItemAttribute)
}

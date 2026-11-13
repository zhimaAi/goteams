// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// planningFieldPerm maps field config permission: planning uses planning:edit, requirement/defect keep OR semantics.
var planningFieldPerm = map[string]string{define.TypeKeyPlanning: `planning:edit`}

// RegFieldRoutes registers field config + custom field routes (login + permission required).
func RegFieldRoutes(r lib_web.Route) {
	prefix := `/api/workspaces/:wsId/field-configs`
	cfPrefix := `/api/workspaces/:wsId/custom-fields`
	orderPath := `/api/workspaces/:wsId/fields/order`

	// Built-in field configuration (attribute field switches).
	r[http.MethodGet][prefix] = wrapAction(withWorkspaceMember(), controller.ListFieldConfigs)
	r[http.MethodPut][prefix] = wrapAction(withTypeScopedPerm(planningFieldPerm,
		"requirement_setting:field", "defect_setting:field"), controller.SaveFieldConfigs)
	r[http.MethodPut][prefix+`/reset`] = wrapAction(withTypeScopedPerm(planningFieldPerm,
		"requirement_setting:field", "defect_setting:field"), controller.ResetFieldConfigs)
	r[http.MethodPut][orderPath] = wrapAction(withTypeScopedPerm(planningFieldPerm,
		"requirement_setting:field", "defect_setting:field"), controller.SaveFieldOrder)

	// Custom field CRUD.
	r[http.MethodGet][cfPrefix] = wrapAction(withWorkspaceMember(), controller.ListCustomFields)
	r[http.MethodGet][cfPrefix+`/enterprise-roles`] = wrapAction(withTypeScopedPerm(planningFieldPerm,
		"requirement_setting:field", "defect_setting:field"), controller.ListCustomFieldEnterpriseRoles)
	r[http.MethodPost][cfPrefix] = wrapAction(withTypeScopedPerm(planningFieldPerm,
		"requirement_setting:field", "defect_setting:field"), controller.CreateCustomField)
	r[http.MethodPut][cfPrefix+`/:fieldId`] = wrapAction(withTypeScopedPerm(planningFieldPerm,
		"requirement_setting:field", "defect_setting:field"), controller.UpdateCustomField)
	r[http.MethodDelete][cfPrefix+`/:fieldId`] = wrapAction(withTypeScopedPerm(planningFieldPerm,
		"requirement_setting:field", "defect_setting:field"), controller.DeleteCustomField)
}

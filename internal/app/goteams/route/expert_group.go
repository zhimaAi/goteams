// Copyright © 2026 Wuhan Sesame Small Customer Service Network Co., Ltd.

package route

import (
	"net/http"

	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
)

// RegExpertGroupRoutes registers expert group management routes.
// Permission model mirrors pipelines: viewing requires pipeline:view, management requires
// pipeline:manage (需求 1896 决策：专家团沿用流水线 CanManage 权限模型).
func RegExpertGroupRoutes(r lib_web.Route) {
	r[http.MethodGet][`/api/expert-groups`] = wrapAction(withPerm(`pipeline:view`), controller.ListExpertGroups)
	r[http.MethodGet][`/api/expert-groups/:expertGroupId`] = wrapAction(withPerm(`pipeline:view`), controller.GetExpertGroup)
	r[http.MethodPost][`/api/expert-groups`] = wrapAction(withPerm(`pipeline:manage`), controller.CreateExpertGroup)
	r[http.MethodPut][`/api/expert-groups/:expertGroupId`] = wrapAction(withPerm(`pipeline:manage`), controller.UpdateExpertGroup)
	r[http.MethodDelete][`/api/expert-groups/:expertGroupId`] = wrapAction(withPerm(`pipeline:manage`), controller.DeleteExpertGroup)
	r[http.MethodPost][`/api/expert-groups/:expertGroupId/avatar`] = wrapAction(withPerm(`pipeline:manage`), controller.UploadExpertGroupAvatar)
}

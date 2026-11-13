// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegCommentRoutes registers comment routes (auth + permission check).
func RegCommentRoutes(r lib_web.Route) {
	prefix := `/api/workspaces/:wsId/requirements/:reqId/comments`

	r[http.MethodGet][prefix] = wrapAction(withAnyPerm("requirement:view", "defect:view"), controller.ListComments)
	r[http.MethodPost][prefix] = wrapAction(withAnyPerm("requirement:edit", "defect:edit"), controller.CreateComment)
	r[http.MethodPut][prefix+`/:commentId`] = wrapAction(withAnyPerm("requirement:edit", "defect:edit"), controller.UpdateComment)
	r[http.MethodDelete][prefix+`/:commentId`] = wrapAction(withAnyPerm("requirement:edit", "defect:edit"), controller.DeleteComment)
}

// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegAttachmentRoutes registers requirement attachment routes.
func RegAttachmentRoutes(r lib_web.Route) {
	prefix := `/api/workspaces/:wsId/requirements/:reqId/attachments`
	r[http.MethodGet][prefix] = wrapAction(withAnyPerm(`requirement:view`, `defect:view`), controller.ListAttachments)
	r[http.MethodPost][prefix] = wrapAction(withAnyPerm(`requirement:edit`, `defect:edit`), controller.UploadAttachments)
	r[http.MethodGet][prefix+`/:attachmentId/download`] = wrapAction(withAnyPerm(`requirement:view`, `defect:view`), controller.DownloadAttachment)
	r[http.MethodDelete][prefix+`/:attachmentId`] = wrapAction(withAnyPerm(`requirement:edit`, `defect:edit`), controller.DeleteAttachment)
}

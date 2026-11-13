package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegStorageRoutes registers enterprise-level storage management routes.
func RegStorageRoutes(r lib_web.Route) {
	// Storage file list (paginated).
	r[http.MethodGet][`/api/enterprise/files`] = wrapAction(withPerm("storage:view"), controller.ListEnterpriseFiles)

	// Download file.
	r[http.MethodGet][`/api/enterprise/files/:fileId/download`] = wrapAction(withPerm("storage:download"), controller.DownloadEnterpriseFile)

	// Delete single file.
	r[http.MethodDelete][`/api/enterprise/files/:fileId`] = wrapAction(withPerm("storage:delete"), controller.DeleteEnterpriseFile)

	// Batch delete files.
	r[http.MethodPost][`/api/enterprise/files/batch-delete`] = wrapAction(withPerm("storage:delete"), controller.BatchDeleteEnterpriseFiles)
}

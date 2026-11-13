// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"net/http"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/controller"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
	"goteams/internal/pkg/lib_web"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

// RegKnowledgeBaseRoutes registers knowledge base routes (feature gated by the workspace application).
func RegKnowledgeBaseRoutes(r lib_web.Route) {
	// Tree / detail / search.
	r[http.MethodGet][`/api/workspaces/:wsId/kb/tree`] = wrapAction(withKb(`kb:view`), controller.GetKnowledgeTree)
	r[http.MethodGet][`/api/workspaces/:wsId/kb/items/:itemId`] = wrapAction(withKb(`kb:view`), controller.GetKnowledgeItemDetail)
	r[http.MethodGet][`/api/workspaces/:wsId/kb/search`] = wrapAction(withKb(`kb:view`), controller.SearchKnowledgeItems)

	// Create / rename / delete / copy.
	r[http.MethodPost][`/api/workspaces/:wsId/kb/folders`] = wrapAction(withAnyKb(`kb:create`, `kb:manage`), controller.CreateKnowledgeFolder)
	r[http.MethodPost][`/api/workspaces/:wsId/kb/documents`] = wrapAction(withAnyKb(`kb:create`, `kb:manage`), controller.CreateKnowledgeDocument)
	r[http.MethodPost][`/api/workspaces/:wsId/kb/documents/from-file`] = wrapAction(withAnyKb(`kb:create`, `kb:manage`), controller.CreateKnowledgeDocumentFromFile)
	r[http.MethodPost][`/api/workspaces/:wsId/kb/documents/from-text`] = wrapAction(withAnyKb(`kb:create`, `kb:manage`), controller.CreateKnowledgeDocumentFromFile)
	r[http.MethodPost][`/api/workspaces/:wsId/kb/links`] = wrapAction(withAnyKb(`kb:create`, `kb:manage`), controller.CreateKnowledgeLink)
	r[http.MethodPut][`/api/workspaces/:wsId/kb/items/:itemId`] = wrapAction(withKb(`kb:view`), controller.RenameKnowledgeItem)
	r[http.MethodPost][`/api/workspaces/:wsId/kb/items/:itemId/move`] = wrapAction(withKb(`kb:view`), controller.MoveKnowledgeItem)
	r[http.MethodDelete][`/api/workspaces/:wsId/kb/items/:itemId`] = wrapAction(withKb(`kb:view`), controller.DeleteKnowledgeItem)
	r[http.MethodPost][`/api/workspaces/:wsId/kb/documents/:documentId/copy`] = wrapAction(withAnyKb(`kb:create`, `kb:manage`), controller.CopyKnowledgeDocument)
	r[http.MethodGet][`/api/workspaces/:wsId/kb/documents/:documentId/download`] = wrapAction(withKb(`kb:view`), controller.DownloadKnowledgeDocument)

	// Autosave / convert.
	r[http.MethodPut][`/api/workspaces/:wsId/kb/documents/:documentId/content`] = wrapAction(withKb(`kb:view`), controller.SaveKnowledgeDocumentContent)
	r[http.MethodPost][`/api/workspaces/:wsId/kb/documents/:documentId/convert`] = wrapAction(withKb(`kb:view`), controller.ConvertKnowledgeDocument)

	// Versions.
	r[http.MethodGet][`/api/workspaces/:wsId/kb/documents/:documentId/versions`] = wrapAction(withKb(`kb:view`), controller.GetKnowledgeVersions)
	r[http.MethodGet][`/api/workspaces/:wsId/kb/documents/:documentId/versions/:versionId`] = wrapAction(withKb(`kb:view`), controller.GetKnowledgeVersionDetail)
	r[http.MethodPost][`/api/workspaces/:wsId/kb/documents/:documentId/versions`] = wrapAction(withKb(`kb:view`), controller.CreateKnowledgeVersion)
	r[http.MethodPost][`/api/workspaces/:wsId/kb/documents/:documentId/versions/:versionId/restore`] = wrapAction(withKb(`kb:view`), controller.RestoreKnowledgeVersion)

	// Favorites.
	r[http.MethodPut][`/api/workspaces/:wsId/kb/items/:itemId/favorite`] = wrapAction(withKb(`kb:view`), controller.SetKnowledgeFavorite)
	r[http.MethodDelete][`/api/workspaces/:wsId/kb/items/:itemId/favorite`] = wrapAction(withKb(`kb:view`), controller.UnsetKnowledgeFavorite)

	// Home feed.
	r[http.MethodGet][`/api/workspaces/:wsId/kb/home/activities`] = wrapAction(withKb(`kb:view`), controller.GetKnowledgeHomeActivities)
	r[http.MethodGet][`/api/workspaces/:wsId/kb/home/favorites`] = wrapAction(withKb(`kb:view`), controller.GetKnowledgeHomeFavorites)
	r[http.MethodGet][`/api/workspaces/:wsId/kb/home/created-by-me`] = wrapAction(withKb(`kb:view`), controller.GetKnowledgeHomeCreatedByMe)

	// Attachments.
	r[http.MethodPost][`/api/workspaces/:wsId/kb/documents/:documentId/attachments/images`] = wrapAction(withKb(`kb:view`), controller.UploadKnowledgeImage)
	r[http.MethodPost][`/api/workspaces/:wsId/kb/documents/:documentId/attachments/files`] = wrapAction(withKb(`kb:view`), controller.UploadKnowledgeFile)
	r[http.MethodPost][`/api/workspaces/:wsId/kb/documents/:documentId/attachments/videos`] = wrapAction(withKb(`kb:view`), controller.UploadKnowledgeVideo)
	r[http.MethodGet][`/api/workspaces/:wsId/kb/attachments/:attachmentId/content`] = wrapAction(withKb(`kb:view`), controller.GetKnowledgeAttachmentContent)
}

// kbFeatureMiddleware blocks access when the knowledge base application is disabled in the workspace.
func kbFeatureMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := lib_i18n.GetLang(c)
		adminId := c.GetInt(`adminId`)
		wsId := c.Param(`wsId`)
		enabled, err := business.IsWorkspaceApplicationEnabled(adminId, cast.ToInt(wsId), define.AppKeyKnowledgeBase)
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
				Message: lib_i18n.Show(lang, define.MsgFeatureDisabled),
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// withKb applies authentication, the feature gate, and the requested permission.
// kb:manage implies kb:view for every knowledge-base read and object operation.
func withKb(permKey string) gin.HandlerFunc {
	if permKey == `kb:view` {
		return chainMiddleware(common.AuthMiddleware(), kbFeatureMiddleware(), business.PermissionAnyMiddleware(`kb:view`, `kb:manage`))
	}
	return chainMiddleware(common.AuthMiddleware(), kbFeatureMiddleware(), business.PermissionMiddleware(permKey))
}

// withAnyKb applies authentication, the knowledge-base feature gate, and OR permission checks.
func withAnyKb(permKeys ...string) gin.HandlerFunc {
	return chainMiddleware(common.AuthMiddleware(), kbFeatureMiddleware(), business.PermissionAnyMiddleware(permKeys...))
}

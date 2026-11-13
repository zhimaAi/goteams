// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"net/http"

	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/controller"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_web"

	"github.com/gin-gonic/gin"
)

// RegOpenApiV1Routes 注册供 AI Agent 调用的开放接口：凭个人 API token 鉴权，
// 与运营后台的 /api/open/*（验签）互不影响。每条路由先按 token 权限点拦截，
// 再由 handler 叠加项目成员与角色权限校验。
func RegOpenApiV1Routes(r lib_web.Route) {
	myWork := withApiToken(define.ApiPermMyWorkList, controller.ListApiMyWork)
	workspaces := withApiToken(define.ApiPermWorkspaceList, controller.ListApiWorkspaces)
	listItems := withApiToken(define.ApiPermWorkItemList, controller.ListApiWorkspaceWorkItems)
	viewItem := withApiToken(define.ApiPermWorkItemView, controller.GetApiWorkItem)
	create := withApiToken(define.ApiPermWorkItemCreate, controller.CreateApiWorkItem)
	update := withApiToken(define.ApiPermWorkItemUpdate, controller.UpdateApiWorkItem)
	comment := withApiToken(define.ApiPermWorkItemComment, controller.CreateApiWorkItemComment)
	file := withApiToken(define.ApiPermWorkItemView, controller.DownloadApiWorkItemFile)
	// 素材上传同时服务新建与修改，只给其中一个功能的 Key 也应该能传素材。
	assetScopes := []string{define.ApiPermWorkItemCreate, define.ApiPermWorkItemUpdate}
	uploadImage := withApiTokenAny(assetScopes, controller.UploadApiWorkItemImage)
	uploadFile := withApiTokenAny(assetScopes, controller.UploadApiWorkItemFile)
	uploadVideo := withApiTokenAny(assetScopes, controller.UploadApiWorkItemVideo)

	r[http.MethodGet][`/api/v1/my-work`] = myWork
	r[http.MethodGet][`/api/v1/workspaces`] = workspaces
	r[http.MethodGet][`/api/v1/workspaces/:wsId/work-items`] = listItems
	r[http.MethodGet][`/api/v1/workspaces/:wsId/work-items/:itemId`] = viewItem
	r[http.MethodPost][`/api/v1/workspaces/:wsId/work-items`] = create
	r[http.MethodPatch][`/api/v1/workspaces/:wsId/work-items/:itemId`] = update
	r[http.MethodPost][`/api/v1/workspaces/:wsId/work-items/:itemId/comments`] = comment
	r[http.MethodGet][`/api/v1/workspaces/:wsId/work-items/:itemId/files/download`] = file
	r[http.MethodPost][`/api/v1/workspaces/:wsId/assets/image`] = uploadImage
	r[http.MethodPost][`/api/v1/workspaces/:wsId/assets/file`] = uploadFile
	r[http.MethodPost][`/api/v1/workspaces/:wsId/assets/video`] = uploadVideo
}

// withApiToken 把鉴权、权限点拦截、业务 handler 与成功统计编进同一个 action。
//
// 统计必须在 action 之后：本项目用 wrapAction 把中间件和 handler 合成一个 gin handler，
// 中间件里调 c.Next() 时后面已无待执行 handler，状态码恒为默认 200，写在里面会把失败也计成成功。
func withApiToken(scope string, action lib_web.Action) lib_web.Action {
	return withApiTokenAny([]string{scope}, action)
}

// withApiTokenAny 与 withApiToken 相同，只是权限点满足任意一个即可。
func withApiTokenAny(scopes []string, action lib_web.Action) lib_web.Action {
	middlewares := []gin.HandlerFunc{common.ApiTokenAuthMiddleware(), common.ApiPermAnyMiddleware(scopes...)}
	return func(c *gin.Context) {
		for _, middleware := range middlewares {
			middleware(c)
			if c.IsAborted() {
				return
			}
		}
		action(c)
		common.ApiTokenStatSuccess(c)
	}
}

// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/controller"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_web"
	"net/http"
)

// RegApiTokenRoutes 注册个人设置里的 API token 管理接口。
// 全部只操作当前用户自己的 token，登录即可，不走项目 RBAC。
func RegApiTokenRoutes(r lib_web.Route) {
	auth := withAuth()

	r[http.MethodGet][`/api/user/api-tokens`] = wrapAction(auth, controller.ListApiTokens)
	r[http.MethodPost][`/api/user/api-tokens`] = wrapAction(auth, controller.CreateApiToken)
	r[http.MethodGet][`/api/user/api-tokens/permissions`] = wrapAction(auth, controller.GetApiTokenPermissions)
	// 接口文档描述全部开放接口且不含密钥，与具体 token 无关，所以不挂在 :tokenId 下。
	r[http.MethodGet][`/api/user/api-tokens/doc`] = wrapAction(auth, controller.DownloadApiTokenDoc)
	r[http.MethodPut][`/api/user/api-tokens/:tokenId`] = wrapAction(auth, controller.UpdateApiToken)
	r[http.MethodPut][`/api/user/api-tokens/:tokenId/enabled`] = wrapAction(auth, controller.SetApiTokenEnabled)
	r[http.MethodDelete][`/api/user/api-tokens/:tokenId`] = wrapAction(auth, controller.DeleteApiToken)
	r[http.MethodGet][`/api/user/api-tokens/:tokenId/skill`] = wrapAction(auth, controller.DownloadApiTokenSkill)
	// 临时安装链接：登录态生成，凭链接免登录下载（链接本身就是凭据，5 分钟过期且只能用一次）。
	r[http.MethodPost][`/api/user/api-tokens/:tokenId/skill/link`] = wrapAction(auth, controller.CreateApiTokenSkillLink)
	r[http.MethodGet][define.ApiTokenSkillLinkPath+`:linkKey`] = controller.DownloadApiTokenSkillByLink
}

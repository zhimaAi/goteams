// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/controller"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
	"goteams/internal/pkg/lib_web"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

// Route is the global route table — read by lib_web.InitGin and registered with the Gin engine.
var Route lib_web.Route

func init() {
	// Step 1: Initialize route map (all HTTP methods + special routes).
	Route = make(map[string]map[string]lib_web.Action)
	Route[http.MethodGet] = make(map[string]lib_web.Action)
	Route[http.MethodPost] = make(map[string]lib_web.Action)
	Route[http.MethodPut] = make(map[string]lib_web.Action)
	Route[http.MethodPatch] = make(map[string]lib_web.Action)
	Route[http.MethodDelete] = make(map[string]lib_web.Action)
	Route[lib_web.NoMethod] = make(map[string]lib_web.Action)
	Route[lib_web.NoRoute] = make(map[string]lib_web.Action)

	// Step 2: Register routes by domain.
	RegAuthRoutes(Route)
	RegOauthRoutes(Route)
	RegLicenseRoutes(Route)
	RegUserRoutes(Route)
	RegEnterpriseRoutes(Route)
	RegPlanRoutes(Route)
	RegWorkspaceRoutes(Route)
	RegRequirementRoutes(Route)
	RegWorkItemViewRoutes(Route)
	RegTeamWorkRoutes(Route)
	RegVersionRoutes(Route)
	RegWorkspaceMemberRoutes(Route)
	RegCommentRoutes(Route)
	RegFieldRoutes(Route)
	RegWorkItemTemplateRoutes(Route)
	RegAttachmentRoutes(Route)
	RegWorkflowRoutes(Route)
	RegStorageRoutes(Route)
	RegAutomationRoutes(Route)
	RegSLARoutes(Route)
	RegClientRoutes(Route)
	RegWorkItemAttributeRoutes(Route)
	RegPlanningRoutes(Route)
	RegPipelineRoutes(Route)
	RegExpertGroupRoutes(Route)
	RegClientRoutes(Route)
	RegWorkspaceApplicationRoutes(Route)
	RegWorkspaceNavigationRoutes(Route)
	RegDashboardRoutes(Route)
	RegIssueInviteRoutes(Route)
	RegKnowledgeBaseRoutes(Route)
	RegExternalNotificationRoutes(Route)
	RegNotificationRoutes(Route)
	RegApiTokenRoutes(Route)
	RegSsoRoutes(Route)
	RegPersonalizationRoutes(Route)
	RegOpenApiV1Routes(Route)
	RegSkillRoutes(Route)

	// Step 3: Ping + 404/405.
	Route[http.MethodGet][`/api/ping`] = controller.Ping
	Route[http.MethodGet][`/api/site/region`] = controller.SiteRegion
	Route[lib_web.NoMethod][`/`] = controller.NoMethod
	Route[lib_web.NoRoute][`/`] = controller.NoRoute

	// Step 4: Mirror every /api/* route as a /mobile/* alias for the mobile client.
	// Handlers (with their wrapped middleware chains) are shared, so aliases stay in
	// sync automatically; each alias additionally stamps require_source=mobile.
	// Collect first to avoid mutating the map while ranging it.
	for method, actions := range Route {
		if method == lib_web.NoRoute || method == lib_web.NoMethod {
			continue
		}
		type alias struct {
			path   string
			action lib_web.Action
		}
		var aliases []alias
		for path, action := range actions {
			if rest, ok := strings.CutPrefix(path, `/api/`); ok {
				aliases = append(aliases, alias{path: `/mobile/` + rest, action: withMobileSource(action)})
			}
		}
		for _, a := range aliases {
			actions[a.path] = a.action
		}
	}
}

// ======================== Middleware Utility Functions ========================

// wrapAction wraps a single Action with a middleware chain.
func wrapAction(middleware gin.HandlerFunc, action lib_web.Action) lib_web.Action {
	return func(c *gin.Context) {
		middleware(c)
		if !c.IsAborted() {
			action(c)
		}
	}
}

// withMobileSource stamps /mobile/* alias requests with require_source=mobile
// (overwriting any client-supplied value) before the shared handler runs, so
// controllers can tell mobile-origin traffic apart. It must run before any code
// reads the query, since gin caches the parsed URL query on first access.
func withMobileSource(action lib_web.Action) lib_web.Action {
	return func(c *gin.Context) {
		query := c.Request.URL.Query()
		query.Set(`require_source`, `mobile`)
		c.Request.URL.RawQuery = query.Encode()
		action(c)
	}
}

// chainMiddleware chains multiple middlewares into one.
func chainMiddleware(middlewares ...gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, mw := range middlewares {
			mw(c)
			if c.IsAborted() {
				return
			}
		}
	}
}

// withPerm is a shortcut: authentication + permission check (permission key driven).
// Usage: withPerm("requirement:create")
func withPerm(permKey string) gin.HandlerFunc {
	return chainMiddleware(common.AuthMiddleware(), business.PermissionMiddleware(permKey))
}

// withAnyPerm is a shortcut: authentication + OR permission check.
func withAnyPerm(permKeys ...string) gin.HandlerFunc {
	return chainMiddleware(common.AuthMiddleware(), business.PermissionAnyMiddleware(permKeys...))
}

// withWorkspaceMemberAndAnyPerm 在工作项详情请求来自非工作空间成员时返回带项目名的提示；
// 已是成员但缺少权限时沿用通用无权限响应。
func withWorkspaceMemberAndAnyPerm(permKeys ...string) gin.HandlerFunc {
	return chainMiddleware(common.AuthMiddleware(), func(c *gin.Context) {
		adminID, userID := c.GetInt(`adminId`), c.GetInt(`userId`)
		workspaceID := c.Param(`wsId`)
		if !business.IsTenantOwner(adminID, userID) && !business.IsWorkspaceMember(adminID, userID, workspaceID) {
			lang := lib_i18n.GetLang(c)
			workspace, err := business.GetWorkspaceAccess(business.WorkspaceAccessModel{
				AdminID: adminID, WorkspaceID: cast.ToInt(workspaceID), UserID: userID,
			})
			message := lib_i18n.Show(lang, define.MsgForbidden)
			if err == nil {
				message = lib_i18n.Show(lang, `workspace_access_forbidden`, workspace.WorkspaceName)
			}
			c.JSON(http.StatusForbidden, define.Response{Code: http.StatusForbidden, Message: message})
			c.Abort()
			return
		}
		business.PermissionAnyMiddleware(permKeys...)(c)
	})
}

// withAllPerm is a shortcut: authentication + AND permission check (every key required).
func withAllPerm(permKeys ...string) gin.HandlerFunc {
	middlewares := make([]gin.HandlerFunc, 0, len(permKeys)+1)
	middlewares = append(middlewares, common.AuthMiddleware())
	for _, key := range permKeys {
		middlewares = append(middlewares, business.PermissionMiddleware(key))
	}
	return chainMiddleware(middlewares...)
}

// withTypeScopedPerm dispatches permissions by the ?type= param: matched scoped work-item types use
// their single permission, while unmatched types fall back to OR semantics, preserving requirement/defect behavior.
func withTypeScopedPerm(scoped map[string]string, fallback ...string) gin.HandlerFunc {
	return chainMiddleware(common.AuthMiddleware(), func(c *gin.Context) {
		typeKey := c.DefaultQuery(`type`, define.TypeKeyRequirement)
		if permKey, ok := scoped[typeKey]; ok {
			business.PermissionMiddleware(permKey)(c)
			return
		}
		business.PermissionAnyMiddleware(fallback...)(c)
	})
}

// withActionTargetPerm checks the source project edit permission from query parameters.
// The action-target endpoint has no :wsId route parameter, so the regular permission middleware
// cannot resolve its project scope.
func withActionTargetPerm() gin.HandlerFunc {
	return chainMiddleware(common.AuthMiddleware(), func(c *gin.Context) {
		sourceWsID, err := strconv.Atoi(c.Query(`sourceWsId`))
		typeKey := c.Query(`type`)
		action := c.Query(`action`)
		var permKey string
		sourceType := typeKey
		if action == `convert` {
			switch typeKey {
			case define.TypeKeyRequirement:
				sourceType = define.TypeKeyDefect
			case define.TypeKeyDefect:
				sourceType = define.TypeKeyRequirement
			}
		}
		switch sourceType {
		case define.TypeKeyRequirement:
			permKey = `requirement:edit`
		case define.TypeKeyDefect:
			permKey = `defect:edit`
		default:
			c.Next()
			return
		}
		if err != nil || sourceWsID <= 0 {
			c.Next()
			return
		}
		if !business.HasPermission(c.GetInt(`adminId`), c.GetInt(`userId`), strconv.Itoa(sourceWsID), permKey) {
			c.JSON(http.StatusForbidden, define.Response{
				Code:    http.StatusForbidden,
				Message: lib_i18n.Show(lib_i18n.GetLang(c), define.MsgForbidden),
			})
			c.Abort()
			return
		}
		c.Next()
	})
}

// withAuth is a shortcut: authentication only.
func withAuth() gin.HandlerFunc {
	return common.AuthMiddleware()
}

// withWorkspaceMember is a shortcut: authentication + workspace membership check.
func withWorkspaceMember() gin.HandlerFunc {
	return chainMiddleware(common.AuthMiddleware(), workspaceMemberMiddleware())
}

// workspaceMemberMiddleware checks whether the current user belongs to the workspace.
func workspaceMemberMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := lib_i18n.GetLang(c)
		userId := c.GetInt(`userId`)
		adminId := c.GetInt(`adminId`)
		wsId := c.Param(`wsId`)

		if !business.IsTenantOwner(adminId, userId) && !business.IsWorkspaceMember(adminId, userId, wsId) {
			c.JSON(http.StatusForbidden, define.Response{
				Code:    http.StatusForbidden,
				Message: lib_i18n.Show(lang, define.MsgForbidden),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

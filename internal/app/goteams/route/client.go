// Copyright © 2026 Wuhan Sesame Small Customer Service Network Co., Ltd.

package route

import (
	"net/http"

	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/controller"
	"goteams/internal/pkg/lib_web"

	"github.com/gin-gonic/gin"
)

// withClientAuth is the client-specific auth chain: auth + version check + device credential check (with tenant consistency).
func withClientAuth() gin.HandlerFunc {
	return chainMiddleware(
		common.AuthMiddleware(),
		common.ClientVersionMiddleware(),
		common.DeviceCredentialMiddleware(),
	)
}

// RegClientRoutes registers client-specific REST API routes.
func RegClientRoutes(r lib_web.Route) {
	// Browser login handshake: the desktop has no credentials yet, so create/exchange stay public.
	// Confirm uses the web user's existing session from the system browser.
	r[http.MethodPost][`/api/client/login-sessions`] = controller.CreateClientLoginSession
	r[http.MethodPost][`/api/client/login-sessions/confirm`] = wrapAction(withAuth(), controller.ConfirmClientLoginSession)
	r[http.MethodPost][`/api/client/login-sessions/exchange`] = controller.ExchangeClientLoginTicket

	// Confirmation-page poll: the signed-in web user waits for the desktop to
	// finish the ticket exchange before showing the success state.
	r[http.MethodGet][`/api/client/login-sessions/:state/status`] = wrapAction(withAuth(), controller.GetClientLoginSessionStatus)

	// Client-specific APIs (require device credential + JWT).
	clientAuth := withClientAuth()
	r[http.MethodGet][`/api/client/compatibility`] = wrapAction(clientAuth, controller.GetClientCompatibility)
	r[http.MethodGet][`/api/client/me`] = wrapAction(clientAuth, controller.GetClientMe)
	r[http.MethodGet][`/api/client/work-items`] = wrapAction(clientAuth, controller.GetClientWorkItems)
	r[http.MethodGet][`/api/client/pipelines`] = wrapAction(clientAuth, controller.GetClientPipelines)
	r[http.MethodGet][`/api/client/expert-groups`] = wrapAction(clientAuth, controller.GetClientExpertGroups)
	r[http.MethodGet][`/api/client/pipelines/:pipelineId/snapshot`] = wrapAction(clientAuth, controller.GetClientPipelineSnapshot)
	r[http.MethodGet][`/api/client/pipelines/:pipelineId/execution-authorization`] = wrapAction(clientAuth, controller.GetClientExecutionAuthorization)

	// Incremental task result sync (REST, replaces the removed WebSocket channel).
	r[http.MethodPost][`/api/client/pipeline-tasks/sync`] = wrapAction(clientAuth, controller.PushPipelineTaskSync)
	// Step-level session sync: the client pushes each step's full conversation projection of cloud tasks.
	r[http.MethodPost][`/api/client/pipeline-tasks/sessions`] = wrapAction(clientAuth, controller.PushPipelineTaskSession)
	// 产出文档上报：接口只落暂存盘并入队，对象存储写入由后台 worker 完成。
	r[http.MethodPost][`/api/client/pipeline-tasks/documents`] = wrapAction(clientAuth, controller.PushPipelineTaskDocument)
	r[http.MethodDelete][`/api/client/pipeline-tasks/:localTaskUuid`] = wrapAction(clientAuth, controller.DeleteClientPipelineTask)

	// Device registration (only JWT required; client no longer needs a bootstrap key).
	r[http.MethodPost][`/api/client/devices/register`] = wrapAction(withAuth(), controller.RegisterClientDevice)

	// 客户端下载（仅需 JWT，客户端通用不绑定租户）
	// 下载地址直接由列表接口从配置文件 [client] 段下发，客户端自行访问，无需服务端中转。
	r[http.MethodGet][`/api/client/releases`] = wrapAction(withAuth(), controller.ListClientReleases)
}

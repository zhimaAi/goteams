// Copyright © 2026 Wuhan Sesame Small Customer Service Network Co., Ltd.

package route

import (
	"errors"
	"net/http"
	"strconv"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/controller"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
	"goteams/internal/pkg/lib_web"

	"github.com/gin-gonic/gin"
	"github.com/zhimaAi/go_tools/logs"
)

// RegPipelineRoutes registers pipeline management and task routes.
func RegPipelineRoutes(r lib_web.Route) {
	// Pipeline CRUD
	r[http.MethodGet][`/api/pipelines`] = wrapAction(withPerm("pipeline:view"), controller.ListPipelines)
	r[http.MethodGet][`/api/pipelines/:pipelineId`] = wrapAction(withPerm("pipeline:view"), controller.GetPipeline)
	r[http.MethodPost][`/api/pipelines`] = wrapAction(withPerm("pipeline:manage"), controller.CreatePipeline)
	r[http.MethodPut][`/api/pipelines/:pipelineId`] = wrapAction(withPerm("pipeline:manage"), controller.UpdatePipeline)
	r[http.MethodPut][`/api/pipelines/:pipelineId/workflow`] = wrapAction(withPerm("pipeline:manage"), controller.UpdatePipelineWorkflow)
	r[http.MethodDelete][`/api/pipelines/:pipelineId`] = wrapAction(withPerm("pipeline:manage"), controller.DeletePipeline)
	r[http.MethodPost][`/api/pipelines/:pipelineId/icon`] = wrapAction(withPerm("pipeline:manage"), controller.UploadPipelineIcon)

	// Pipeline tasks
	r[http.MethodGet][`/api/pipeline-tasks`] = wrapAction(withPerm("pipeline:view"), controller.ListPipelineTasks)
	r[http.MethodGet][`/api/pipeline-tasks/:taskId`] = wrapAction(withPerm("pipeline:view"), controller.GetPipelineTaskDetail)

	// 产出文档：清单、详情正文、2 小时签名链接。权限跟随工作项查看权限，与 Agent tab
	// 的沟通记录一致——这些文件在详情抽屉里就是任务详情的一部分，不该要求流水线菜单权限。
	r[http.MethodGet][`/api/pipeline-tasks/:taskId/documents`] = wrapAction(withPipelineDocumentPerm(true), controller.ListPipelineTaskDocuments)
	r[http.MethodGet][`/api/pipeline-documents/:documentId`] = wrapAction(withPipelineDocumentPerm(false), controller.GetPipelineDocument)
	r[http.MethodGet][`/api/pipeline-documents/:documentId/link`] = wrapAction(withPipelineDocumentPerm(false), controller.GetPipelineDocumentLink)
}

// withPipelineDocumentPerm 用「工作项查看权限」（requirement:view / defect:view）守卫产出文档读取接口。
//
// 三个接口按 taskId / documentId 寻址，路径里没有 wsId，withAnyPerm 这类基于路由参数的中间件
// 取不到工作空间角色，所以先反查文档所属工作空间（admin_id 隔离），再按该空间的工作项权限判定。
// 不存在统一回 404，避免把「任务/文档是否存在」暴露给无权用户。
func withPipelineDocumentPerm(byTask bool) gin.HandlerFunc {
	return chainMiddleware(common.AuthMiddleware(), func(c *gin.Context) {
		lang := lib_i18n.GetLang(c)
		adminId := c.GetInt(`adminId`)
		userId := c.GetInt(`userId`)

		paramKey := `documentId`
		if byTask {
			paramKey = `taskId`
		}
		resourceId, err := strconv.ParseInt(c.Param(paramKey), 10, 64)
		if err != nil || resourceId <= 0 {
			c.JSON(http.StatusBadRequest, define.Response{
				Code:    http.StatusBadRequest,
				Message: lib_i18n.Show(lang, define.MsgParamInvalid),
			})
			c.Abort()
			return
		}

		var workspaceId int
		if byTask {
			workspaceId, err = business.PipelineTaskWorkspaceId(adminId, resourceId)
		} else {
			workspaceId, err = business.PipelineDocumentWorkspaceId(adminId, resourceId)
		}
		if err != nil {
			if isPipelineResourceMissing(err) {
				message := define.MsgPipelineDocumentNotFound
				if byTask {
					message = define.MsgPipelineTaskNotFound
				}
				c.JSON(http.StatusNotFound, define.Response{
					Code:    http.StatusNotFound,
					Message: lib_i18n.Show(lang, message),
				})
			} else {
				logs.Error("resolve pipeline document workspace failed, task_scoped:%v id:%d err:%s", byTask, resourceId, err.Error())
				c.JSON(http.StatusInternalServerError, define.Response{
					Code:    http.StatusInternalServerError,
					Message: lib_i18n.Show(lang, define.MsgError),
				})
			}
			c.Abort()
			return
		}

		if !business.CanViewPipelineDocuments(adminId, userId, workspaceId) {
			c.JSON(http.StatusForbidden, define.Response{
				Code:    http.StatusForbidden,
				Message: lib_i18n.Show(lang, define.MsgForbidden),
			})
			c.Abort()
			return
		}

		c.Next()
	})
}

// isPipelineResourceMissing 区分「资源不存在」与查询失败，前者对外回 404。
func isPipelineResourceMissing(err error) bool {
	return errors.Is(err, business.ErrPipelineTaskNotFound) ||
		errors.Is(err, business.ErrPipelineDocumentNotFound)
}

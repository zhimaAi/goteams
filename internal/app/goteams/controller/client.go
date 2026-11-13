// Copyright © 2026 Wuhan Sesame Small Customer Service Network Co., Ltd.

package controller

import (
	"errors"
	"net/http"
	"strings"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

// ======================== Client-only REST API ========================

// GetClientCompatibility returns client compatibility info.
func GetClientCompatibility(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))

	result, err := business.GetClientCompatibility(adminId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `client.compatibility`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    result,
	})
}

// GetClientMe returns the current client user info.
func GetClientMe(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))

	result, err := business.GetClientMe(adminId, userId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `client.me`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    result,
	})
}

// GetClientWorkItems returns the importable work item list.
func GetClientWorkItems(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))

	keyword := c.Query(`keyword`)
	page := cast.ToInt(c.Query(`page`))
	size := cast.ToInt(c.Query(`size`))

	list, total, err := business.GetClientWorkItems(adminId, userId, keyword, page, size)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `client.work_items`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: map[string]interface{}{
			`list`:  list,
			`total`: total,
			`page`:  page,
			`size`:  size,
		},
	})
}

// GetClientPipelines returns the pipeline list available to the current user.
func GetClientPipelines(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))

	list, err := business.GetClientPipelines(adminId, userId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `client.pipelines`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    list,
	})
}

// GetClientPipelineSnapshot returns the immutable pipeline creation snapshot.
func GetClientPipelineSnapshot(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))
	pipelineId, err := positiveParam(c, `pipelineId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `client.snapshot.param`, err)
		return
	}

	result, err := business.GetPipelineSnapshot(adminId, pipelineId, userId)
	if err != nil {
		if errors.Is(err, business.ErrPipelineNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgPipelineNotFound))
			return
		}
		if errors.Is(err, business.ErrPipelineUseForbidden) {
			abortForbidden(c, lib_i18n.Show(lang, define.MsgPipelineUseForbidden))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `client.snapshot`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    result,
	})
}

// GetClientExecutionAuthorization confirms permissions before execution.
func GetClientExecutionAuthorization(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))
	pipelineId, err := positiveParam(c, `pipelineId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `client.auth.param`, err)
		return
	}

	result, err := business.CheckExecutionAuthorization(adminId, pipelineId, userId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `client.auth`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    result,
	})
}

// ======================== Client pipeline task sync (REST, replaces WebSocket) ========================

// PushPipelineTaskSync accepts the client's incremental task sync push and upserts it into the cloud.
// Steps and conversations are merged incrementally by their stable keys; the deleted lists are applied first.
func PushPipelineTaskSync(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))

	var req define.PipelineTaskSnapshot
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `client.task_sync.bind`, err)
		return
	}
	// Device identity comes only from the authenticated request headers; client payload overrides are rejected.
	req.DeviceUUID = c.GetString(`deviceUUID`)

	result, err := business.SaveTaskSnapshot(adminId, userId, req)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `client.task_sync.save`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    result,
	})
}

// PushPipelineTaskSession accepts the client's step-level session projection (created or
// updated) and upserts it into the cloud session table. When the task projection
// is not synced yet the push is rejected so the client retries it in a later round.
func PushPipelineTaskSession(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))

	var req define.PipelineTaskSessionSync
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `client.session_sync.bind`, err)
		return
	}

	result, err := business.SaveTaskSession(adminId, userId, req)
	if err != nil {
		// The task projection has not been synced yet: respond 404 instead of a parameter
		// error so the client records a retryable failure and re-pushes in a later round.
		if errors.Is(err, business.ErrPipelineTaskSessionTargetMissing) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgPipelineTaskNotFound))
			return
		}
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `client.session_sync.save`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    result,
	})
}

// DeleteClientPipelineTask deletes the cloud task projection by the client task uuid.
// Missing tasks are treated as already deleted (idempotent).
func DeleteClientPipelineTask(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))
	localTaskUUID := c.Param(`localTaskUuid`)
	if strings.TrimSpace(localTaskUUID) == `` {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `client.task_delete.param`, errInvalidID)
		return
	}

	if err := business.DeleteTaskByLocalUUID(adminId, userId, localTaskUUID); err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `client.task_delete`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    map[string]interface{}{`local_task_uuid`: localTaskUUID, `result`: `deleted`},
	})
}

// RegisterClientDevice registers the client device.
// Shared by all clients; the bootstrap key is no longer required.
func RegisterClientDevice(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))

	var req define.RegisterDeviceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `client.register.bind`, err)
		return
	}

	if err := validate.ValidateDeviceRegister(req.DeviceUUID, req.OS); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `client.register.validate`, err)
		return
	}

	result, err := business.RegisterDevice(adminId, userId, req)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `client.register`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    result,
	})
}

// ======================== 客户端下载 API ========================

// ListClientReleases 返回可下载的客户端版本列表。
// 客户端通用，不区分租户。
func ListClientReleases(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	platform := c.Query(`platform`)
	arch := c.Query(`arch`)

	list, err := business.ListClientReleases(platform, arch)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `client.releases`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: map[string]interface{}{
			`list`: list,
		},
	})
}

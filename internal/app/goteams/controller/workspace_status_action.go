package controller

import (
	"errors"
	"net/http"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
)

type statusActionSwitchRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}

type statusActionNameRequest struct {
	Name string `json:"name"`
}

type statusActionMappingRequest struct {
	SourceStatusId int `json:"source_status_id"`
	ActionId       int `json:"action_id"`
	TargetStatusId int `json:"target_status_id"`
}

type statusActionNameResponse struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type statusActionMappingResponse struct {
	Id             int `json:"id"`
	SourceStatusId int `json:"source_status_id"`
	ActionId       int `json:"action_id"`
	TargetStatusId int `json:"target_status_id"`
}

type statusActionsResponse struct {
	Enabled  bool                          `json:"enabled"`
	Actions  []statusActionNameResponse    `json:"actions"`
	Mappings []statusActionMappingResponse `json:"mappings"`
}

type statusActionItemResponse struct {
	Id             int    `json:"id"`
	Name           string `json:"name"`
	SourceStatusId int    `json:"source_status_id"`
	TargetStatusId int    `json:"target_status_id"`
}

type statusActionsMetadataResponse struct {
	Enabled bool                       `json:"enabled"`
	Items   []statusActionItemResponse `json:"items"`
}

func toStatusActionsMetadataResponse(model *business.WorkspaceStatusActionsModel) *statusActionsMetadataResponse {
	if model == nil {
		return nil
	}
	items := make([]statusActionItemResponse, 0, len(model.Mappings))
	names := make(map[int]string, len(model.Actions))
	for _, action := range model.Actions {
		names[action.Id] = action.Name
	}
	for _, mapping := range model.Mappings {
		items = append(items, statusActionItemResponse{
			Id: mapping.Id, Name: names[mapping.ActionId],
			SourceStatusId: mapping.SourceStatusId, TargetStatusId: mapping.TargetStatusId,
		})
	}
	return &statusActionsMetadataResponse{Enabled: model.Enabled, Items: items}
}

func respondStatusActions(c *gin.Context, model business.WorkspaceStatusActionsModel, err error) {
	lang := lib_i18n.GetLang(c)
	switch {
	case errors.Is(err, business.ErrStatusActionWorkspace), errors.Is(err, business.ErrStatusActionInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgStatusActionInvalid), "status_action", err)
	case errors.Is(err, business.ErrStatusActionConflict):
		abortConflict(c, lib_i18n.Show(lang, define.MsgStatusActionConfigConflict))
	case errors.Is(err, business.ErrStatusActionNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgStatusActionNotFound))
	case err != nil:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), "status_action", err)
	default:
		response := statusActionsResponse{
			Enabled: model.Enabled, Actions: make([]statusActionNameResponse, 0, len(model.Actions)),
			Mappings: make([]statusActionMappingResponse, 0, len(model.Mappings)),
		}
		for _, item := range model.Actions {
			response.Actions = append(response.Actions, statusActionNameResponse(item))
		}
		for _, item := range model.Mappings {
			response.Mappings = append(response.Mappings, statusActionMappingResponse(item))
		}
		c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response})
	}
}

// GetWorkspaceStatusActions 读取工单状态操作配置。
func GetWorkspaceStatusActions(c *gin.Context) {
	wsId, err := positiveParam(c, "wsId")
	if err != nil {
		respondStatusActions(c, business.WorkspaceStatusActionsModel{}, business.ErrStatusActionInvalid)
		return
	}
	model, err := business.GetWorkspaceStatusActions(business.WorkspaceStatusActionsModel{AdminId: c.GetInt("adminId"), WorkspaceId: wsId}, true)
	respondStatusActions(c, model, err)
}

// SetWorkspaceStatusActionsEnabled 保存工单状态操作开关。
func SetWorkspaceStatusActionsEnabled(c *gin.Context) {
	var req statusActionSwitchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondStatusActions(c, business.WorkspaceStatusActionsModel{}, business.ErrStatusActionInvalid)
		return
	}
	saveWorkspaceStatusActions(c, business.WorkspaceStatusActionChangeModel{Enabled: req.Enabled})
}

// SaveWorkspaceStatusAction 新增、改名或删除操作名称。
func SaveWorkspaceStatusAction(c *gin.Context) {
	id, valid := statusActionRecordId(c, "actionId")
	if !valid {
		return
	}
	remove := c.Request.Method == http.MethodDelete
	var req statusActionNameRequest
	if !remove {
		if err := c.ShouldBindJSON(&req); err != nil {
			respondStatusActions(c, business.WorkspaceStatusActionsModel{}, business.ErrStatusActionInvalid)
			return
		}
	}
	saveWorkspaceStatusActions(c, business.WorkspaceStatusActionChangeModel{
		Delete: remove, Action: &business.WorkspaceStatusActionModel{Id: id, Name: req.Name},
	})
}

// SaveWorkspaceStatusActionMapping 新增、修改或删除一条状态操作关系。
func SaveWorkspaceStatusActionMapping(c *gin.Context) {
	id, valid := statusActionRecordId(c, "mappingId")
	if !valid {
		return
	}
	remove := c.Request.Method == http.MethodDelete
	var req statusActionMappingRequest
	if !remove {
		if err := c.ShouldBindJSON(&req); err != nil {
			respondStatusActions(c, business.WorkspaceStatusActionsModel{}, business.ErrStatusActionInvalid)
			return
		}
	}
	saveWorkspaceStatusActions(c, business.WorkspaceStatusActionChangeModel{
		Delete: remove, Mapping: &business.WorkspaceStatusActionMappingModel{
			Id: id, SourceStatusId: req.SourceStatusId, ActionId: req.ActionId, TargetStatusId: req.TargetStatusId,
		},
	})
}

func statusActionRecordId(c *gin.Context, param string) (int, bool) {
	if c.Request.Method == http.MethodPost {
		return 0, true
	}
	id, err := positiveParam(c, param)
	if err != nil {
		respondStatusActions(c, business.WorkspaceStatusActionsModel{}, business.ErrStatusActionInvalid)
	}
	return id, err == nil
}

func saveWorkspaceStatusActions(c *gin.Context, change business.WorkspaceStatusActionChangeModel) {
	wsId, err := positiveParam(c, "wsId")
	if err != nil {
		respondStatusActions(c, business.WorkspaceStatusActionsModel{}, business.ErrStatusActionInvalid)
		return
	}
	change.AdminId, change.WorkspaceId = c.GetInt("adminId"), wsId
	model, err := business.UpdateWorkspaceStatusActions(change)
	respondStatusActions(c, model, err)
}

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
)

type workspaceAccessResponse struct {
	WorkspaceName string `json:"workspace_name"`
	IsMember      bool   `json:"is_member"`
	RequestStatus string `json:"request_status"`
}

type requestWorkspaceAccessRequest struct {
	Reason string `json:"reason"`
}

type requestWorkspaceAccessResponse struct {
	Status string `json:"status"`
}

// GetWorkspaceAccess returns only the project data needed by the access request page.
func GetWorkspaceAccess(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsID, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workspace.access.id`, err)
		return
	}
	model, err := business.GetWorkspaceAccess(business.WorkspaceAccessModel{
		AdminID: c.GetInt(`adminId`), WorkspaceID: wsID, UserID: c.GetInt(`userId`),
	})
	if errors.Is(err, business.ErrWorkspaceNotFound) {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgWorkspaceNotFound))
		return
	}
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workspace.access.get`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: workspaceAccessResponse{WorkspaceName: model.WorkspaceName, IsMember: model.IsMember, RequestStatus: model.RequestStatus}})
}

// RequestWorkspaceAccess submits a membership request for the current user.
func RequestWorkspaceAccess(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsID, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workspace.access.id`, err)
		return
	}
	var req requestWorkspaceAccessRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `workspace.access.bind`, err)
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if err = validate.ValidateWorkspaceAccessReason(req.Reason); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `workspace.access.reason`, err)
		return
	}
	err = business.RequestWorkspaceAccess(business.WorkspaceAccessModel{
		AdminID: c.GetInt(`adminId`), WorkspaceID: wsID, UserID: c.GetInt(`userId`),
	}, req.Reason)
	switch {
	case errors.Is(err, business.ErrWorkspaceNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgWorkspaceNotFound))
	case errors.Is(err, business.ErrWorkspaceAccessPending):
		abortConflict(c, lib_i18n.Show(lang, define.MsgWorkspaceAccessPending))
	case errors.Is(err, business.ErrWorkspaceAccessAlreadyMember):
		abortConflict(c, lib_i18n.Show(lang, define.MsgWorkspaceAccessAlreadyMember))
	case err != nil:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `workspace.access.request`, err)
	default:
		c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
			Data: requestWorkspaceAccessResponse{Status: `pending`}})
	}
}

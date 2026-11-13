// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

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

// ======================== Member Invitation (link invite) ========================

type createInvitationRequest struct {
	InviteLimit   int   `json:"invite_limit" binding:"required"`
	ExpireMinutes int   `json:"expire_minutes" binding:"required"`
	RoleId        int   `json:"role_id" binding:"required,gt=0"`
	PhoneRequired bool  `json:"phone_required"`
	EmailRequired bool  `json:"email_required"`
	DepartmentIds []int `json:"department_ids"`
}

type createInvitationResponse struct {
	Token          string `json:"token"`
	RoleId         string `json:"role_id"`
	InviterName    string `json:"inviter_name"`
	EnterpriseName string `json:"enterprise_name"`
	InviteLimit    int    `json:"invite_limit"`
	AcceptedCount  int    `json:"accepted_count"`
	PhoneRequired  bool   `json:"phone_required"`
	EmailRequired  bool   `json:"email_required"`
	ExpiredAt      string `json:"expired_at"`
}

type getInvitationResponse struct {
	Token          string `json:"token"`
	RoleId         string `json:"role_id"`
	InviterName    string `json:"inviter_name"`
	EnterpriseName string `json:"enterprise_name"`
	InviteLimit    int    `json:"invite_limit"`
	AcceptedCount  int    `json:"accepted_count"`
	PhoneRequired  bool   `json:"phone_required"`
	EmailRequired  bool   `json:"email_required"`
	ExpiredAt      string `json:"expired_at"`
}

// CreateInvitation generates a new invitation link (stops the previous active one).
func CreateInvitation(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req createInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `invitation.create.bind`, err)
		return
	}
	if req.InviteLimit < 1 || req.InviteLimit > 9999 || req.ExpireMinutes < 1 || req.ExpireMinutes > 10080 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `invitation.create.validate`, errors.New(`invalid invitation limit or expire minutes`))
		return
	}

	model, err := business.CreateInvitation(business.CreateInvitationModel{
		AdminId: c.GetInt(`adminId`), CreatedBy: c.GetInt(`userId`),
		RoleId: req.RoleId, InviteLimit: req.InviteLimit, ExpireMinutes: req.ExpireMinutes,
		PhoneRequired: req.PhoneRequired, EmailRequired: req.EmailRequired, DepartmentIds: req.DepartmentIds,
	})
	if err != nil {
		if errors.Is(err, business.ErrEnterpriseRoleNotFound) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `invitation.create.role`, err)
			return
		}
		if errors.Is(err, business.ErrDepartmentInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgDepartmentInvalid), `invitation.create.department`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `invitation.create`, err)
		return
	}
	info, err := business.GetInvitationInfoByToken(model.Token)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `invitation.create.info`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: createInvitationResponse{
			Token: model.Token, RoleId: cast.ToString(model.RoleId), InviterName: info.InviterName,
			EnterpriseName: info.EnterpriseName, InviteLimit: model.InviteLimit,
			AcceptedCount: 0, PhoneRequired: model.PhoneRequired, EmailRequired: model.EmailRequired, ExpiredAt: cast.ToString(model.ExpiredAt),
		},
	})
}

// GetInvitation returns the tenant's current active invitation (Data is null when none).
func GetInvitation(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	model, found, err := business.GetActiveInvitation(c.GetInt(`adminId`))
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `invitation.get`, err)
		return
	}
	resp := define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)}
	if found {
		info, infoErr := business.GetInvitationInfoByToken(model.Token)
		if infoErr != nil {
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `invitation.get.info`, infoErr)
			return
		}
		resp.Data = getInvitationResponse{
			Token: model.Token, RoleId: cast.ToString(model.RoleId), InviterName: info.InviterName,
			EnterpriseName: info.EnterpriseName, InviteLimit: model.InviteLimit,
			AcceptedCount: model.AcceptedCount, PhoneRequired: model.PhoneRequired, EmailRequired: model.EmailRequired, ExpiredAt: cast.ToString(model.ExpiredAt),
		}
	}
	c.JSON(http.StatusOK, resp)
}

// StopInvitation stops the tenant's active invitation.
func StopInvitation(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	if err := business.StopInvitation(c.GetInt(`adminId`)); err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `invitation.stop`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// ======================== Public invite endpoints (token authenticated) ========================

type inviteInfoResponse struct {
	RoleName       string `json:"role_name"`
	InviterName    string `json:"inviter_name"`
	EnterpriseName string `json:"enterprise_name"`
	PhoneRequired  bool   `json:"phone_required"`
	EmailRequired  bool   `json:"email_required"`
	ExpiredAt      string `json:"expired_at"`
}

// GetInviteInfo returns invitation info for the public accept page.
func GetInviteInfo(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	model, err := business.GetInvitationInfoByToken(c.Param(`token`))
	if err != nil {
		respondInviteError(c, lang, `invite.info`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: inviteInfoResponse{
			RoleName:       lib_i18n.ShowStored(lang, model.RoleName),
			InviterName:    model.InviterName,
			EnterpriseName: model.EnterpriseName,
			PhoneRequired:  model.PhoneRequired,
			EmailRequired:  model.EmailRequired,
			ExpiredAt:      cast.ToString(model.ExpiredAt),
		},
	})
}

type uploadInviteAvatarResponse struct {
	Url string `json:"url"`
}

// UploadInviteAvatar uploads a pending avatar for an invitee (invitation token acts as auth).
func UploadInviteAvatar(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	invitation, err := business.GetInvitationByToken(c.Param(`token`))
	if err != nil {
		respondInviteError(c, lang, `invite.avatar.token`, err)
		return
	}
	file, err := c.FormFile(`file`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgAvatarInvalid), `invite.avatar.form_file`, err)
		return
	}
	avatar, err := business.SavePendingAvatar(invitation.AdminId, file)
	if err != nil {
		switch {
		case errors.Is(err, business.ErrAvatarInvalid):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgAvatarInvalid), `invite.avatar.invalid`, err)
		case errors.Is(err, business.ErrAvatarTooLarge):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgAvatarTooLarge), `invite.avatar.too_large`, err)
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `invite.avatar.save`, err)
		}
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    uploadInviteAvatarResponse{Url: avatar},
	})
}

// sendInviteEmailCodeRequest describes the invite-join email verification code request body.
type sendInviteEmailCodeRequest struct {
	Email string `json:"email" binding:"required"`
}

// SendInviteEmailCode mails a verification code to an invitee (the invitation token is the auth).
func SendInviteEmailCode(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	if _, err := business.GetInvitationByToken(c.Param(`token`)); err != nil {
		respondInviteError(c, lang, `invite.email_code.token`, err)
		return
	}
	var req sendInviteEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `invite.email_code.bind`, err)
		return
	}
	if err := validate.ValidateMemberEmail(req.Email); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `invite.email_code.validate`, err)
		return
	}
	if err := business.SendInviteEmailCode(req.Email, lang); err != nil {
		switch {
		case errors.Is(err, business.ErrUserEmailExists):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgEmailExists), `invite.email_code.email_exists`, err)
		case errors.Is(err, business.ErrVerifyCodeFrequent):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `invite.email_code.frequent`, err)
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgEmailSendFailed), `invite.email_code.send`, err)
		}
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgVerifyCodeSent),
	})
}

type acceptInviteRequest struct {
	Name       string `json:"name" binding:"required"`
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	Phone      string `json:"phone"`
	Email      string `json:"email"`
	VerifyCode string `json:"verifyCode"`
	Avatar     string `json:"avatar"`
}

// AcceptInvite creates a tenant member from the public invitation page.
func AcceptInvite(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req acceptInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `invite.accept.bind`, err)
		return
	}
	if err := validateAcceptInviteRequest(req); err != nil {
		// FieldError 显示字段级文案；电话号码哨兵错误单独映射
		msg := fieldErrorMessage(err, lang, define.MsgParamInvalid)
		if errors.Is(err, validate.ErrInvitePhoneInvalid) {
			msg = lib_i18n.Show(lang, `invite_phone_invalid`)
		}
		abortBadRequest(c, msg, `invite.accept.validate`, err)
		return
	}

	_, err := business.AcceptInvitation(business.AcceptInvitationModel{
		Token: c.Param(`token`), Username: req.Username, Password: req.Password,
		Name: req.Name, Phone: strings.TrimSpace(req.Phone), Email: req.Email, VerifyCode: req.VerifyCode,
		Avatar: req.Avatar,
	})
	if err != nil {
		switch {
		case errors.Is(err, business.ErrInvitationInvalid):
			respondInviteError(c, lang, `invite.accept.invalid`, err)
		case errors.Is(err, business.ErrInvitePhoneRequired):
			abortBadRequest(c, lib_i18n.Show(lang, `invite_phone_required`), `invite.accept.phone_required`, err)
		case errors.Is(err, business.ErrInviteEmailRequired):
			abortBadRequest(c, lib_i18n.Show(lang, `invite_email_required`), `invite.accept.email_required`, err)
		case errors.Is(err, business.ErrVerifyCodeInvalid):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgVerifyCodeInvalid), `invite.accept.code_invalid`, err)
		case errors.Is(err, business.ErrUserEmailExists):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgEmailExists), `invite.accept.email_exists`, err)
		case errors.Is(err, business.ErrUserUsernameExists):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgUserExist), `invite.accept.username_exists`, err)
		case errors.Is(err, business.ErrAvatarInvalid):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgAvatarInvalid), `invite.accept.avatar`, err)
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `invite.accept`, err)
		}
		return
	}

	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

func validateAcceptInviteRequest(req acceptInviteRequest) error {
	if err := validate.ValidateAccount("username", req.Username); err != nil {
		return err
	}
	if err := validate.ValidateProfile(strings.TrimSpace(req.Name)); err != nil {
		return err
	}
	if err := validate.IsValidPassword(req.Password); err != nil {
		return err
	}
	// Validate format when the phone number is non-empty; whether it is required is decided by invitation settings, checked in the business layer.
	if phone := strings.TrimSpace(req.Phone); phone != `` {
		return validate.ValidateInvitePhone(phone)
	}
	return validate.ValidateMemberEmail(req.Email)
}

// respondInviteError maps an invitation business error to the unified "invitation invalid" 404 response.
func respondInviteError(c *gin.Context, lang string, stage string, err error) {
	if errors.Is(err, business.ErrInvitationInvalid) {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgInvitationInvalid))
		return
	}
	abortInternal(c, lib_i18n.Show(lang, define.MsgError), stage, err)
}

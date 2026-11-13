// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
)

// Commercial license endpoints. Public activation endpoints sit behind the global license
// gate whitelist and must reject when a valid license is already installed; owner endpoints
// stay reachable while the license just expired so the owner can update the key.

// ======================== Response DTOs ========================

type licenseQuotasResponse struct {
	MemberLimit                   int64 `json:"member_limit"`
	ProjectLimit                  int64 `json:"project_limit"`
	CustomFieldPerProjectLimit    int64 `json:"custom_field_per_project_limit"`
	AutomationRulePerProjectLimit int64 `json:"automation_rule_per_project_limit"`
	AutomationExecLimit           int64 `json:"automation_exec_limit"`
	AgentLimit                    int64 `json:"agent_limit"`
	StorageLimit                  int64 `json:"storage_limit"`
}

type licenseStatusResponse struct {
	Edition        string                 `json:"edition"`
	BuildVersion   string                 `json:"build_version"`
	LicenseRequire bool                   `json:"license_required"`
	Status         string                 `json:"status"`
	Reason         string                 `json:"reason"`
	CanLogin       bool                   `json:"can_login"`
	BusinessEnable bool                   `json:"business_enabled"`
	DeviceCode     string                 `json:"device_code"`
	CorpName       string                 `json:"corp_name,omitempty"`
	ActivationMode string                 `json:"activation_mode,omitempty"`
	LicenseKind    string                 `json:"license_kind,omitempty"`
	LicenseNo      string                 `json:"license_no,omitempty"`
	ExpireAt       int64                  `json:"expire_at,omitempty"`
	RemainingDays  int                    `json:"remaining_days,omitempty"`
	Revision       int                    `json:"revision,omitempty"`
	Quotas         *licenseQuotasResponse `json:"quotas,omitempty"`
}

type licenseActivateOnlineRequest struct {
	CorpName   string `json:"corp_name"`
	Phone      string `json:"phone"`
	Email      string `json:"email"`
	VerifyCode string `json:"verify_code"`
	RequestId  string `json:"request_id"`
}

type licenseActivateResponse struct {
	LicenseNo         string                 `json:"license_no"`
	CorpName          string                 `json:"corp_name"`
	ActivationMode    string                 `json:"activation_mode"`
	ExpireAt          int64                  `json:"expire_at"`
	Revision          int                    `json:"revision"`
	State             string                 `json:"state"`
	Quotas            *licenseQuotasResponse `json:"quotas,omitempty"`
	BootstrapUsername string                 `json:"bootstrap_username,omitempty"`
	BootstrapPassword string                 `json:"bootstrap_password,omitempty"`
}

type licenseInstallRequest struct {
	LicenseKey string `json:"license_key"`
}

type licenseRefreshResponse struct {
	Status string `json:"status"`
	State  string `json:"state"`
}

// ======================== Shared helpers ========================

// toLicenseQuotasResponse maps verified claim quotas for response payloads.
func toLicenseQuotasResponse(quotas common.LicenseQuotaClaims) *licenseQuotasResponse {
	return &licenseQuotasResponse{
		MemberLimit:                   quotas.MemberLimit,
		ProjectLimit:                  quotas.ProjectLimit,
		CustomFieldPerProjectLimit:    quotas.CustomFieldPerProjectLimit,
		AutomationRulePerProjectLimit: quotas.AutomationRulePerProjectLimit,
		AutomationExecLimit:           quotas.AutomationExecLimit,
		AgentLimit:                    quotas.AgentLimit,
		StorageLimit:                  quotas.StorageLimit,
	}
}

// remainingDaysUntil converts a future expiry into whole remaining days (0 once expired).
func remainingDaysUntil(expireAt int64) int {
	remaining := expireAt - time.Now().Unix()
	if remaining <= 0 {
		return 0
	}
	return int((remaining + 86399) / 86400)
}

// ensureActivationAllowed returns true when the request may proceed; otherwise it already
// wrote the 403 response. Public activation endpoints must refuse while a valid license is
// installed so an unauthenticated visitor cannot replace it.
func ensureActivationAllowed(c *gin.Context) bool {
	if define.LicenseStateCanLogin(business.GetLicenseSnapshot().State) {
		abortForbidden(c, lib_i18n.Show(lib_i18n.GetLang(c), define.MsgLicenseAlreadyActive))
		return false
	}
	return true
}

// writeLicenseActivateResponse renders the shared activation/install success payload.
func writeLicenseActivateResponse(c *gin.Context, lang string, messageKey string, result business.InstallLicenseResult, bootstrap business.OnlineActivateResult, hasBootstrap bool) {
	resp := licenseActivateResponse{
		LicenseNo:         result.LicenseNo,
		CorpName:          result.CorpName,
		ActivationMode:    result.ActivationMode,
		ExpireAt:          result.ExpireAt,
		Revision:          result.Revision,
		State:             result.State,
		Quotas:            toLicenseQuotasResponse(result.Quotas),
		BootstrapUsername: bootstrap.BootstrapUsername,
		BootstrapPassword: bootstrap.BootstrapPassword,
	}
	if !hasBootstrap {
		resp.BootstrapUsername = ``
		resp.BootstrapPassword = ``
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, messageKey),
		Data:    resp,
	})
}

// licenseOpsUserMessage resolves the user-presentable message for an online-license ops error.
// GoTeams-side errors carry an i18n key; operations-backend messages are curated terminal-facing
// copy by contract and pass through verbatim so actionable reasons (e.g. a missing verify code)
// reach the user instead of a generic outage notice.
func licenseOpsUserMessage(lang string, err error) string {
	var opsErr *business.LicenseOpsError
	if errors.As(err, &opsErr) {
		if opsErr.Key != `` {
			return lib_i18n.Show(lang, opsErr.Key)
		}
		if opsErr.Msg != `` {
			return opsErr.Msg
		}
	}
	return lib_i18n.Show(lang, define.MsgLicenseOnlineFailed)
}

// mapLicenseInstallError converts install failures into the controlled response with the
// matching blocking state, so the frontend can guide the user without leaking internals.
// Parse failures are split into user-actionable causes (format / environment / signature /
// claims) while the raw cause still goes to the server log for troubleshooting.
func mapLicenseInstallError(c *gin.Context, lang, stage string, err error) bool {
	var licenseErr *business.LicenseError
	var opsErr *business.LicenseOpsError
	switch {
	case errors.As(err, &opsErr):
		abortBadRequest(c, licenseOpsUserMessage(lang, err), stage, err)
		return true
	case errors.Is(err, business.ErrLicenseExpired):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgLicenseExpired), stage, err)
		return true
	case errors.Is(err, business.ErrLicenseDeviceMismatch):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgLicenseDeviceMismatch), stage, err)
		return true
	case errors.Is(err, business.ErrLicenseClockInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgLicenseClockInvalid), stage, err)
		return true
	case errors.Is(err, business.ErrLicenseNotYetValid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgLicenseKeyInvalid), stage, err)
		return true
	case errors.Is(err, business.ErrLicenseRevisionRegressed):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgLicenseKeyInvalid), stage, err)
		return true
	case errors.Is(err, common.ErrLicenseKeyFormat),
		errors.Is(err, common.ErrLicenseKeyAlg),
		errors.Is(err, common.ErrLicenseKeyTyp):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgLicenseKeyFormat), stage, err)
		return true
	case errors.Is(err, common.ErrLicenseKeyUnknownKID):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgLicenseKeyEnvMismatch), stage, err)
		return true
	case errors.Is(err, common.ErrLicenseKeySignature):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgLicenseKeySignature), stage, err)
		return true
	case errors.Is(err, common.ErrLicenseClaims):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgLicenseClaimsInvalid), stage, err)
		return true
	case errors.Is(err, business.ErrLicenseRootTenantMissing),
		errors.Is(err, business.ErrLicenseConfigError):
		abortInternal(c, lib_i18n.Show(lang, define.MsgLicenseConfigError), stage, err)
		return true
	case errors.As(err, &licenseErr):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgLicenseKeyInvalid), stage, err)
		return true
	}
	return false
}

// ======================== Public endpoints (whitelisted) ========================

// LicenseStatus returns the sanitized license status, device code and verified quotas.
func LicenseStatus(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	snapshot := business.GetLicenseSnapshot()
	resp := licenseStatusResponse{
		Edition:        define.BuildEdition,
		BuildVersion:   define.BuildVersion,
		LicenseRequire: true,
		Status:         snapshot.State,
		Reason:         define.LicenseReasonForState(snapshot.State),
		CanLogin:       define.LicenseStateAllowsManagementLogin(snapshot.State),
		BusinessEnable: define.LicenseStateCanLogin(snapshot.State),
		DeviceCode:     snapshot.DeviceCode,
	}
	if snapshot.Parsed != nil {
		resp.CorpName = snapshot.Effective.CorpName
		resp.ActivationMode = snapshot.Effective.ActivationMode
		resp.LicenseKind = snapshot.Effective.LicenseKind
		resp.LicenseNo = snapshot.Effective.LicenseNo
		resp.ExpireAt = snapshot.Effective.ExpireAt
		resp.RemainingDays = remainingDaysUntil(snapshot.Effective.ExpireAt)
		resp.Revision = snapshot.Effective.Revision
		resp.Quotas = toLicenseQuotasResponse(snapshot.Effective.Quotas)
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: resp})
}

// LicenseSendVerifyCode forwards the online-trial email verification-code request.
func LicenseSendVerifyCode(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgLicenseRequestInvalid), `license.verifycode.bind`, err)
		return
	}
	email := validate.NormalizeEmail(req.Email)
	if err := validate.ValidateTrialEmail(email); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgLicenseRequestInvalid), `license.verifycode.validate`, err)
		return
	}
	if err := business.ForwardTrialVerifyCode(email); err != nil {
		if errors.Is(err, business.ErrLicenseConfigError) {
			abortInternal(c, lib_i18n.Show(lang, define.MsgLicenseConfigError), `license.verifycode.forward`, err)
			return
		}
		abortBadRequest(c, licenseOpsUserMessage(lang, err), `license.verifycode.forward`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgLicenseVerifyCodeSent)})
}

// LicenseActivateOnline requests, verifies and installs the free private experience license.
func LicenseActivateOnline(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	if !ensureActivationAllowed(c) {
		return
	}
	var req licenseActivateOnlineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgLicenseRequestInvalid), `license.activate.bind`, err)
		return
	}
	req.CorpName = strings.TrimSpace(req.CorpName)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Email = validate.NormalizeEmail(req.Email)
	req.VerifyCode = strings.TrimSpace(req.VerifyCode)
	req.RequestId = strings.TrimSpace(req.RequestId)
	if err := validate.ValidateTrialCorpName(req.CorpName); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgLicenseRequestInvalid), `license.activate.validate`, err)
		return
	}
	if err := validate.ValidateTrialPhone(req.Phone); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgLicenseRequestInvalid), `license.activate.validate`, err)
		return
	}
	if err := validate.ValidateTrialEmail(req.Email); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgLicenseRequestInvalid), `license.activate.validate`, err)
		return
	}
	if err := validate.ValidateVerifyCode(req.VerifyCode); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgLicenseRequestInvalid), `license.activate.validate`, err)
		return
	}
	if err := validate.ValidateRequestId(req.RequestId); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgLicenseRequestInvalid), `license.activate.validate`, err)
		return
	}
	result, err := business.ActivateLicenseOnline(business.OnlineActivateModel{
		CorpName: req.CorpName, Phone: req.Phone, Email: req.Email,
		VerifyCode: req.VerifyCode, RequestId: req.RequestId,
	})
	if err != nil {
		if mapLicenseInstallError(c, lang, `license.activate.online`, err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `license.activate.online`, err)
		return
	}
	writeLicenseActivateResponse(c, lang, define.MsgLicenseActivateSuccess, result.Install, result, result.FirstActivation)
}

// LicenseActivateOffline verifies and installs a pasted offline license key.
func LicenseActivateOffline(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	if !ensureActivationAllowed(c) {
		return
	}
	var req licenseInstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgLicenseRequestInvalid), `license.activate.offline.bind`, err)
		return
	}
	req.LicenseKey = strings.TrimSpace(req.LicenseKey)
	if err := validate.ValidateLicenseKeyInput(req.LicenseKey); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgLicenseRequestInvalid), `license.activate.offline.validate`, err)
		return
	}
	result, err := business.InstallLicenseKey(req.LicenseKey)
	if err != nil {
		if mapLicenseInstallError(c, lang, `license.activate.offline`, err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `license.activate.offline`, err)
		return
	}
	writeLicenseActivateResponse(c, lang, define.MsgLicenseActivateSuccess, result, business.OnlineActivateResult{}, false)
}

// ======================== Owner endpoints (session + owner) ========================

// LicenseInstall installs an owner-provided full key while the license is valid or expired.
func LicenseInstall(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := c.GetInt(`adminId`)
	userId := c.GetInt(`userId`)
	if !business.IsTenantOwner(adminId, userId) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}
	var req licenseInstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgLicenseRequestInvalid), `license.install.bind`, err)
		return
	}
	req.LicenseKey = strings.TrimSpace(req.LicenseKey)
	if err := validate.ValidateLicenseKeyInput(req.LicenseKey); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgLicenseRequestInvalid), `license.install.validate`, err)
		return
	}
	result, err := business.InstallLicenseKey(req.LicenseKey)
	if err != nil {
		if mapLicenseInstallError(c, lang, `license.install`, err) {
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `license.install`, err)
		return
	}
	writeLicenseActivateResponse(c, lang, define.MsgLicenseUpdateSuccess, result, business.OnlineActivateResult{}, false)
}

// LicenseRefreshOnline triggers an on-demand online re-verification for the owner.
func LicenseRefreshOnline(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := c.GetInt(`adminId`)
	userId := c.GetInt(`userId`)
	if !business.IsTenantOwner(adminId, userId) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}
	if err := business.RefreshLicenseOnline(); err != nil {
		if errors.Is(err, business.ErrLicenseConfigError) {
			abortInternal(c, lib_i18n.Show(lang, define.MsgLicenseConfigError), `license.refresh`, err)
			return
		}
		abortBadRequest(c, licenseOpsUserMessage(lang, err), `license.refresh`, err)
		return
	}
	snapshot := business.GetLicenseSnapshot()
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgLicenseUpdateSuccess),
		Data:    licenseRefreshResponse{Status: snapshot.State, State: snapshot.State},
	})
}

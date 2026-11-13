// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"
	"goteams/internal/pkg/lib_storage"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
)

// ======================== Public Endpoints ========================

// Ping health check.
func Ping(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    `pong`,
	})
}

// loginRequest describes the login request body.
type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// loginResponse describes the login response data.
type loginResponse struct {
	Id         string `json:"id"`
	AdminId    string `json:"admin_id"`
	Username   string `json:"username"`
	Name       string `json:"name"`
	Avatar     string `json:"avatar"`
	RoleId     string `json:"role_id"`
	IsOwner    bool   `json:"is_owner"`
	FirstLogin string `json:"first_login"`
	TelNumber  string `json:"tel_number"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	Token      string `json:"token"`
}

// refreshTokenResponse describes the refresh-token response data.
type refreshTokenResponse struct {
	Id         string `json:"id"`
	AdminId    string `json:"admin_id"`
	Username   string `json:"username"`
	Name       string `json:"name"`
	Avatar     string `json:"avatar"`
	RoleId     string `json:"role_id"`
	IsOwner    bool   `json:"is_owner"`
	FirstLogin string `json:"first_login"`
	TelNumber  string `json:"tel_number"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	Token      string `json:"token"`
}

// toLoginResponse converts the login model into the response data.
func toLoginResponse(model business.LoginModel) loginResponse {
	user := model.User
	return loginResponse{
		Id:         strconv.Itoa(user.Id),
		AdminId:    strconv.Itoa(user.AdminId),
		Username:   user.Username,
		Name:       user.Name,
		Avatar:     user.Avatar,
		RoleId:     strconv.Itoa(user.RoleId),
		IsOwner:    user.IsOwner,
		FirstLogin: user.FirstLogin,
		TelNumber:  user.TelNumber,
		Status:     strconv.Itoa(user.Status),
		CreatedAt:  strconv.FormatInt(user.CreatedAt, 10),
		UpdatedAt:  strconv.FormatInt(user.UpdatedAt, 10),
		Token:      model.Token,
	}
}

// toRefreshTokenResponse converts the refresh-token model into the response data.
func toRefreshTokenResponse(model business.RefreshTokenModel) refreshTokenResponse {
	user := model.User
	return refreshTokenResponse{
		Id:         strconv.Itoa(user.Id),
		AdminId:    strconv.Itoa(user.AdminId),
		Username:   user.Username,
		Name:       user.Name,
		Avatar:     user.Avatar,
		RoleId:     strconv.Itoa(user.RoleId),
		IsOwner:    user.IsOwner,
		FirstLogin: user.FirstLogin,
		TelNumber:  user.TelNumber,
		Status:     strconv.Itoa(user.Status),
		CreatedAt:  strconv.FormatInt(user.CreatedAt, 10),
		UpdatedAt:  strconv.FormatInt(user.UpdatedAt, 10),
		Token:      model.Token,
	}
}

// Login user login.
func Login(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), `auth.login.bind`, err)
		return
	}
	req.Username = validate.NormalizeEmail(req.Username)

	// Validate parameters via validate layer.
	if err := validate.ValidateLogin(validate.LoginParams{
		Username: req.Username,
		Password: req.Password,
	}); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `auth.login.validate`, err)
		return
	}

	model, err := business.Login(business.LoginModel{
		Username: req.Username,
		Password: req.Password,
		Ip:       c.ClientIP(),
		Ua:       c.GetHeader(`User-Agent`),
	})
	if err != nil {
		if errors.Is(err, business.ErrLoginWrong) {
			c.JSON(http.StatusUnauthorized, define.Response{
				Code:    http.StatusUnauthorized,
				Message: lib_i18n.Show(lang, define.MsgLoginWrong),
			})
			return
		}
		if errors.Is(err, business.ErrAccountDisabled) {
			c.JSON(http.StatusUnauthorized, define.Response{
				Code:    http.StatusUnauthorized,
				Message: lib_i18n.Show(lang, define.MsgAccountDisabled),
			})
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `auth.login`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toLoginResponse(model),
	})
}

type registerRequest struct {
	Email      string `json:"email"`
	Username   string `json:"username"`
	Password   string `json:"password" binding:"required"`
	Name       string `json:"name"`
	VerifyCode string `json:"verifyCode"`
}

type registerResponse struct {
	Id         string `json:"id"`
	AdminId    string `json:"admin_id"`
	Username   string `json:"username"`
	Name       string `json:"name"`
	Avatar     string `json:"avatar"`
	RoleId     string `json:"role_id"`
	IsOwner    bool   `json:"is_owner"`
	FirstLogin string `json:"first_login"`
	TelNumber  string `json:"tel_number"`
	Remark     string `json:"remark"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	Token      string `json:"token"`
}

// Register 邮箱注册租户管理员。
func Register(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), `auth.register.bind`, err)
		return
	}
	if strings.TrimSpace(req.Email) == `` {
		req.Email = req.Username
	}
	req.Email = validate.NormalizeEmail(req.Email)
	if err := validate.ValidateRegister(validate.RegisterParams{Username: req.Email, Password: req.Password, Name: req.Name}); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `auth.register.validate`, err)
		return
	}
	if err := business.VerifyRegisterCode(req.Email, req.VerifyCode); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgVerifyCodeInvalid), `auth.register.verify_code`, err)
		return
	}
	result, err := business.RegisterTenant(business.RegisterTenantModel{
		Username: req.Email, Password: req.Password, Name: req.Name, Lang: lang,
	})
	if err != nil {
		if errors.Is(err, business.ErrUserAlreadyExists) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgUserExist), `auth.register.user_exists`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `auth.register.register_tenant`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: toRegisterResponse(result),
	})
}

func toRegisterResponse(model business.RegisterTenantModel) registerResponse {
	return registerResponse{
		Id: strconv.Itoa(model.Id), AdminId: strconv.Itoa(model.AdminId), Username: model.Username,
		Name: model.Name, Avatar: model.Avatar, RoleId: strconv.Itoa(model.RoleId),
		IsOwner: model.IsOwner, FirstLogin: model.FirstLogin, TelNumber: model.TelNumber, Remark: model.Remark,
		Status: strconv.Itoa(model.Status), CreatedAt: strconv.Itoa(model.CreatedAt),
		UpdatedAt: strconv.Itoa(model.UpdatedAt), Token: model.Token,
	}
}

// SendVerifyCode 私有化版不提供匿名邮件找回密码。
func SendVerifyCode(c *gin.Context) {
	abortForbidden(c, lib_i18n.Show(lib_i18n.GetLang(c), define.MsgForbidden))
}

// SendRegisterVerifyCode 发送注册邮箱验证码。
func SendRegisterVerifyCode(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req struct {
		Email    string `json:"email"`
		Username string `json:"username"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), `auth.send_register_code.bind`, err)
		return
	}
	if req.Email == `` {
		req.Email = req.Username
	}
	req.Email = validate.NormalizeEmail(req.Email)
	if err := validate.ValidateForgotPassword(req.Email); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `auth.send_register_code.validate`, err)
		return
	}
	if err := business.SendRegisterVerifyCode(req.Email, lang); err != nil {
		switch {
		case errors.Is(err, business.ErrUserAlreadyExists):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgUserExist), `auth.send_register_code.exists`, err)
		case errors.Is(err, business.ErrVerifyCodeFrequent):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `auth.send_register_code.frequent`, err)
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgEmailSendFailed), `auth.send_register_code.send`, err)
		}
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgVerifyCodeSent)})
}

// ForgotPassword 私有化版不提供匿名邮件找回密码。
func ForgotPassword(c *gin.Context) {
	abortForbidden(c, lib_i18n.Show(lib_i18n.GetLang(c), define.MsgForbidden))
}

// ResetPassword 私有化版不接受匿名邮箱验证码重置密码。
func ResetPassword(c *gin.Context) {
	abortForbidden(c, lib_i18n.Show(lib_i18n.GetLang(c), define.MsgForbidden))
}

func sendPasswordVerifyCode(c *gin.Context, logKey string) {
	lang := lib_i18n.GetLang(c)
	var req struct {
		Email    string `json:"email"`
		Username string `json:"username"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), logKey+`.bind`, err)
		return
	}
	if req.Email == `` {
		req.Email = req.Username
	}
	req.Email = validate.NormalizeEmail(req.Email)
	if err := validate.ValidateForgotPassword(req.Email); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), logKey+`.validate`, err)
		return
	}
	if err := business.SendPasswordVerifyCode(req.Email, lang); err != nil {
		switch {
		case errors.Is(err, business.ErrUserNotExist):
			c.JSON(http.StatusNotFound, define.Response{Code: http.StatusNotFound, Message: lib_i18n.Show(lang, define.MsgUserNotFound)})
		case errors.Is(err, business.ErrVerifyCodeFrequent):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), logKey+`.frequent`, err)
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgEmailSendFailed), logKey+`.send`, err)
		}
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgVerifyCodeSent)})
}

// GetCaptcha returns a slider captcha (disabled in current version).
func GetCaptcha(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	c.JSON(http.StatusServiceUnavailable, define.Response{
		Code:    http.StatusServiceUnavailable,
		Message: lib_i18n.Show(lang, define.MsgFeatureDisabled),
	})
}

// CheckCaptcha verifies a slider captcha (disabled in current version).
func CheckCaptcha(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	c.JSON(http.StatusServiceUnavailable, define.Response{
		Code:    http.StatusServiceUnavailable,
		Message: lib_i18n.Show(lang, define.MsgFeatureDisabled),
	})
}

// ======================== User Endpoints ========================

// GetProfile returns the current user's profile.
func GetProfile(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))

	user, err := business.GetUserInfo(adminId, userId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `auth.get_profile.get_user_info`, err)
		return
	}
	if len(user) == 0 {
		c.JSON(http.StatusNotFound, define.Response{
			Code:    http.StatusNotFound,
			Message: lib_i18n.Show(lang, define.MsgUserNotFound),
		})
		return
	}

	// Remove sensitive fields.
	delete(user, `password`)

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    user,
	})
}

// UpdateProfile updates the user's profile.
func UpdateProfile(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))

	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), `auth.update_profile.bind`, err)
		return
	}

	// Validate via validate layer.
	if err := validate.ValidateProfile(req.Name); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `auth.update_profile.validate`, err)
		return
	}

	if err := business.UpdateUserProfile(adminId, userId, req.Name); err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `auth.update_profile.update_user_profile`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// sendBindEmailCodeRequest describes the bind-email verification code request body.
type sendBindEmailCodeRequest struct {
	Email string `json:"email" binding:"required"`
}

// SendBindEmailCode mails a verification code to the address the current user wants to bind.
func SendBindEmailCode(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req sendBindEmailCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), `auth.send_bind_email_code.bind`, err)
		return
	}
	if err := validate.ValidateMemberEmail(req.Email); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `auth.send_bind_email_code.validate`, err)
		return
	}
	if err := business.SendBindEmailCode(c.GetInt(`userId`), req.Email, lang); err != nil {
		switch {
		case errors.Is(err, business.ErrUserEmailExists):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgEmailExists), `auth.send_bind_email_code.email_exists`, err)
		case errors.Is(err, business.ErrVerifyCodeFrequent):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `auth.send_bind_email_code.frequent`, err)
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgEmailSendFailed), `auth.send_bind_email_code.send`, err)
		}
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgVerifyCodeSent),
	})
}

// bindEmailRequest describes the bind/change email request body.
type bindEmailRequest struct {
	Email      string `json:"email" binding:"required"`
	VerifyCode string `json:"verifyCode" binding:"required"`
}

// BindEmail binds or rebinds the current user's email once the verification code passes.
func BindEmail(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req bindEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), `auth.bind_email.bind`, err)
		return
	}
	if err := validate.ValidateMemberEmail(req.Email); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `auth.bind_email.validate`, err)
		return
	}
	if err := business.BindUserEmail(c.GetInt(`adminId`), c.GetInt(`userId`), req.Email, req.VerifyCode); err != nil {
		switch {
		case errors.Is(err, business.ErrVerifyCodeInvalid):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgVerifyCodeInvalid), `auth.bind_email.code_invalid`, err)
		case errors.Is(err, business.ErrUserEmailExists):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgEmailExists), `auth.bind_email.email_exists`, err)
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `auth.bind_email`, err)
		}
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// GetAvatar returns the avatar file.
// By default returns the current logged-in user's avatar; the optional query parameter userId is used to view other members' avatars within the same tenant,
// GetUserAvatarPath already enforces tenant isolation by admin_id, so it is safe.
func GetAvatar(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := c.GetInt(`userId`)
	if q := c.Query(`userId`); q != `` {
		userId = cast.ToInt(q)
	}
	path, err := business.GetUserAvatarPath(c.GetInt(`adminId`), userId)
	if err != nil {
		if errors.Is(err, business.ErrAvatarNotFound) {
			abortNotFound(c, lib_i18n.Show(lang, define.MsgAvatarNotFound))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `auth.get_avatar.get_path`, err)
		return
	}
	c.Header(`Cache-Control`, `no-store`)
	if err := lib_storage.M.Serve(path, c); err != nil {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgAvatarNotFound))
	}
}

// UploadAvatar uploads and updates the current user's avatar.
func UploadAvatar(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	file, err := c.FormFile(`file`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgAvatarInvalid), `auth.upload_avatar.form_file`, err)
		return
	}
	avatar, err := business.SaveUserAvatar(c.GetInt(`adminId`), c.GetInt(`userId`), file)
	if err != nil {
		switch {
		case errors.Is(err, business.ErrAvatarInvalid):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgAvatarInvalid), `auth.upload_avatar.invalid`, err)
		case errors.Is(err, business.ErrAvatarTooLarge):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgAvatarTooLarge), `auth.upload_avatar.too_large`, err)
		case errors.Is(err, business.ErrAvatarNotFound):
			abortNotFound(c, lib_i18n.Show(lang, define.MsgUserNotFound))
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `auth.upload_avatar.save`, err)
		}
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    gin.H{`avatar`: avatar},
	})
}

// changePasswordRequest describes the change-own-password request body.
type changePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// ChangePassword verifies the old password and then updates the current user's password.
func ChangePassword(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), `auth.change_password.bind`, err)
		return
	}
	if err := validate.ValidatePassword(req.OldPassword, req.NewPassword); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `auth.change_password.validate`, err)
		return
	}

	err := business.ChangeOwnPassword(business.ChangeOwnPasswordModel{
		AdminId:      c.GetInt(`adminId`),
		UserId:       c.GetInt(`userId`),
		OldPassword:  req.OldPassword,
		NewPassword:  req.NewPassword,
		SessionToken: c.GetString(`sessionToken`),
	})
	if err != nil {
		switch {
		case errors.Is(err, business.ErrUserNotExist):
			abortNotFound(c, lib_i18n.Show(lang, define.MsgUserNotFound))
		case errors.Is(err, business.ErrPasswordWrong):
			c.JSON(http.StatusBadRequest, define.Response{
				Code:    http.StatusBadRequest,
				Message: lib_i18n.Show(lang, define.MsgPasswordWrong),
			})
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `auth.change_password`, err)
		}
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

// RefreshToken returns the current user's info and session token (sliding renewal happens in the auth middleware).
func RefreshToken(c *gin.Context) {
	lang := lib_i18n.GetLang(c)

	model, err := business.RefreshLoginToken(business.RefreshTokenModel{
		UserId: c.GetInt(`userId`),
		Token:  c.GetString(`sessionToken`),
	})
	if err != nil {
		if errors.Is(err, business.ErrAccountDisabled) {
			c.JSON(http.StatusUnauthorized, define.Response{
				Code:    http.StatusUnauthorized,
				Message: lib_i18n.Show(lang, define.MsgAccountDisabled),
			})
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `auth.refresh_token`, err)
		return
	}

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toRefreshTokenResponse(model),
	})
}

type logoutResponse struct {
	LoginEntryUrl string `json:"login_entry_url"`
}

// Logout deletes the current session so the token is invalidated immediately.
func Logout(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	common.DeleteSession(c.GetString(`sessionToken`))
	var data logoutResponse
	// 退出只读取入口投影，普通成员无需 SSO 配置权限，也不应触发配置行物化。
	if business.SsoEntry(c.GetInt(`adminId`), lang).Enabled {
		entryUrl, err := business.SsoLoginEntryUrl(c.GetInt(`adminId`))
		if err != nil {
			// 入口生成失败不能阻止退出；客户端沿用普通登录入口。
			logs.Error("logout sso entry failed: admin_id=%d, err=%s", c.GetInt(`adminId`), err.Error())
		} else {
			data.LoginEntryUrl = entryUrl
		}
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    data,
	})
}

// GetMyPermissions returns the current user's permission list in a workspace (for frontend menu/button visibility).
func GetMyPermissions(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	userId := cast.ToInt(c.GetInt(`userId`))
	adminId := cast.ToInt(c.GetInt(`adminId`))
	wsId := c.Query(`wsId`)

	groups := business.GetUserPermissionGroups(adminId, userId, wsId, lang)

	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    groups,
	})
}

// ======================== 404/405 ========================

func NoRoute(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	c.JSON(http.StatusNotFound, define.Response{
		Code:    http.StatusNotFound,
		Message: lib_i18n.Show(lang, define.MsgRouteNotFound),
	})
}

// NoMethod responds with 405 Method Not Allowed for unsupported HTTP methods.
func NoMethod(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	c.JSON(http.StatusMethodNotAllowed, define.Response{
		Code:    http.StatusMethodNotAllowed,
		Message: lib_i18n.Show(lang, define.MsgMethodNotAllowed),
	})
}

// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
	"github.com/zhimaAi/go_tools/logs"
)

// ======================== 扫码入口 ========================

// oauthProviderItem 描述登录页需要的一个第三方登录入口。
type oauthProviderItem struct {
	Provider string `json:"provider"`
	ClientId string `json:"client_id"`
	SdkURL   string `json:"sdk_url"`
}

// GetOauthProviders 返回已开启的第三方登录平台，供登录页决定展示哪些扫码入口。
func GetOauthProviders(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	items := make([]oauthProviderItem, 0, len(business.OauthProviders()))
	for _, provider := range business.EnabledOauthProviders() {
		items = append(items, oauthProviderItem{
			Provider: provider.Key(),
			ClientId: provider.ClientId(),
			SdkURL:   provider.SdkURL(),
		})
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    items,
	})
}

// oauthAuthorizeResponse 描述一次扫码授权入口，前端据此初始化二维码 SDK。
type oauthAuthorizeResponse struct {
	Provider     string `json:"provider"`
	Scene        string `json:"scene"`
	ClientId     string `json:"client_id"`
	SdkURL       string `json:"sdk_url"`
	AuthorizeURL string `json:"authorize_url"`
	RedirectUri  string `json:"redirect_uri"`
	State        string `json:"state"`
	// ExpiresIn 是二维码有效期（秒），前端据此在过期时展示刷新蒙层。
	ExpiresIn int `json:"expires_in"`
}

// toOauthAuthorizeResponse 把业务模型转换成响应结构。
func toOauthAuthorizeResponse(model business.OauthAuthorizeModel) oauthAuthorizeResponse {
	return oauthAuthorizeResponse{
		Provider:     model.Provider,
		Scene:        model.Scene,
		ClientId:     model.ClientId,
		SdkURL:       model.SdkURL,
		AuthorizeURL: model.AuthorizeURL,
		RedirectUri:  model.RedirectUri,
		State:        model.State,
		ExpiresIn:    model.ExpiresIn,
	}
}

// GetOauthAuthorize 生成登录页的扫码授权入口。
func GetOauthAuthorize(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	providerKey := strings.TrimSpace(c.Query(`provider`))
	if err := validate.ValidateOauthProvider(providerKey); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgOauthProviderUnknown), `oauth.authorize.validate`, err)
		return
	}

	model, err := business.BuildOauthAuthorize(providerKey, define.OauthSceneLogin, 0)
	if err != nil {
		respondOauthAuthorizeError(c, lang, `oauth.authorize`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toOauthAuthorizeResponse(model),
	})
}

// GetUserOauthAuthorize 生成个人设置里的绑定扫码授权入口（需登录）。
func GetUserOauthAuthorize(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	providerKey := strings.TrimSpace(c.Query(`provider`))
	if err := validate.ValidateOauthProvider(providerKey); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgOauthProviderUnknown), `oauth.user_authorize.validate`, err)
		return
	}

	model, err := business.BuildOauthAuthorize(providerKey, define.OauthSceneBind, c.GetInt(`userId`))
	if err != nil {
		respondOauthAuthorizeError(c, lang, `oauth.user_authorize`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toOauthAuthorizeResponse(model),
	})
}

// respondOauthAuthorizeError 把生成扫码入口时的错误映射为对外状态码。
func respondOauthAuthorizeError(c *gin.Context, lang string, stage string, err error) {
	if errors.Is(err, business.ErrOauthProviderDisabled) {
		c.JSON(http.StatusServiceUnavailable, define.Response{
			Code:    http.StatusServiceUnavailable,
			Message: lib_i18n.Show(lang, define.MsgOauthProviderDisabled),
		})
		return
	}
	abortInternal(c, lib_i18n.Show(lang, define.MsgError), stage, err)
}

// ======================== 回调 ========================

// DingtalkCallback 处理钉钉扫码授权回调（公开接口，第三方后台登记的地址）。
func DingtalkCallback(c *gin.Context) {
	oauthCallback(c, define.OauthProviderDingtalk)
}

// FeishuCallback 处理飞书扫码授权回调（公开接口，第三方后台登记的地址）。
func FeishuCallback(c *gin.Context) {
	oauthCallback(c, define.OauthProviderFeishu)
}

// oauthCallback 统一处理回调：成功或失败都 302 回前端，由前端按 query 展示结果。
// 浏览器直接跳转到本地址，返回 JSON 用户看不到，因此这里不做 JSON 响应。
func oauthCallback(c *gin.Context, providerKey string) {
	provider, ok := business.OauthProviderByKey(providerKey)
	if !ok {
		c.Redirect(http.StatusFound, oauthErrorPath(``, define.OauthErrorProviderDisabled))
		return
	}

	result, err := business.HandleOauthCallback(providerKey, c.Query(provider.CodeQueryKey()), c.Query(`state`))
	if err != nil {
		// The callback query carries the authorization code, which is a credential: log the reason
		// without the request context that logRequestError would dump.
		logs.Error("oauth callback failed: provider=%s, err=%s", providerKey, err.Error())
		c.Redirect(http.StatusFound, oauthErrorPath(result.Scene, oauthErrorReason(err)))
		return
	}
	c.Redirect(http.StatusFound, oauthSiteBase(c)+result.Path)
}

// oauthErrorReason 把回调错误映射为前端可识别的错误原因。
func oauthErrorReason(err error) string {
	switch {
	case errors.Is(err, business.ErrOauthProviderDisabled):
		return define.OauthErrorProviderDisabled
	case errors.Is(err, business.ErrOauthStateInvalid):
		return define.OauthErrorStateInvalid
	case errors.Is(err, business.ErrOauthCodeInvalid):
		return define.OauthErrorCodeInvalid
	case errors.Is(err, business.ErrOauthTicketInvalid):
		return define.OauthErrorTicketInvalid
	case errors.Is(err, business.ErrOauthBoundByOther), errors.Is(err, business.ErrOauthAlreadyBound):
		return define.OauthErrorBoundByOther
	default:
		return define.OauthErrorServer
	}
}

// oauthErrorPath 拼装失败回跳路径：绑定场景回个人设置页，其余回登录页。
func oauthErrorPath(scene string, reason string) string {
	if scene == define.OauthSceneBind {
		return `/profile?` + business.OauthQueryBindResult + `=failed&` + business.OauthQueryError + `=` + reason
	}
	return `/login?` + business.OauthQueryError + `=` + reason
}

// oauthSiteBase 返回拼装绝对回跳地址用的站点前缀：优先用配置，未配置时按当前请求推导。
func oauthSiteBase(c *gin.Context) string {
	if base := strings.TrimRight(strings.TrimSpace(define.Config.WebService[`site_url`]), `/`); base != `` {
		return base
	}
	scheme := `http`
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader(`X-Forwarded-Proto`), `https`) {
		scheme = `https`
	}
	return scheme + `://` + c.Request.Host
}

// ======================== 换取登录态与绑定 ========================

// ExchangeOauthTicket 用登录凭据换取登录态，返回与密码登录一致的响应。
func ExchangeOauthTicket(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req struct {
		Ticket string `json:"ticket" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), `oauth.exchange.bind`, err)
		return
	}
	if err := validate.ValidateOauthTicket(req.Ticket); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgOauthTicketInvalid), `oauth.exchange.validate`, err)
		return
	}

	model, err := business.ExchangeOauthLoginTicket(req.Ticket, c.ClientIP(), c.GetHeader(`User-Agent`))
	if err != nil {
		respondOauthLoginError(c, lang, `oauth.exchange`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toLoginResponse(model),
	})
}

// BindOauthAccount 用账号密码把扫码得到的第三方账号绑定到已有账号，并直接登录。
func BindOauthAccount(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req struct {
		Ticket   string `json:"ticket" binding:"required"`
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), `oauth.bind.bind`, err)
		return
	}
	if err := validate.ValidateOauthTicket(req.Ticket); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgOauthTicketInvalid), `oauth.bind.ticket`, err)
		return
	}
	req.Username = validate.NormalizeEmail(req.Username)
	if err := validate.ValidateLogin(validate.LoginParams{Username: req.Username, Password: req.Password}); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `oauth.bind.validate`, err)
		return
	}

	model, err := business.BindOauthAccount(req.Ticket, req.Username, req.Password, c.ClientIP(), c.GetHeader(`User-Agent`))
	if err != nil {
		respondOauthLoginError(c, lang, `oauth.bind`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toLoginResponse(model),
	})
}

// RegisterOauthAccount 注册新租户并绑定扫码得到的第三方账号，注册成功即登录。
func RegisterOauthAccount(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req struct {
		Ticket     string `json:"ticket" binding:"required"`
		Name       string `json:"name" binding:"required"`
		Email      string `json:"email" binding:"required"`
		VerifyCode string `json:"verifyCode" binding:"required"`
		Password   string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), `oauth.register.bind`, err)
		return
	}
	if err := validate.ValidateOauthTicket(req.Ticket); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgOauthTicketInvalid), `oauth.register.ticket`, err)
		return
	}
	req.Email = validate.NormalizeEmail(req.Email)
	if err := validate.ValidateRegister(validate.RegisterParams{
		Username: req.Email, Password: req.Password, Name: req.Name,
	}); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `oauth.register.validate`, err)
		return
	}

	model, err := business.RegisterAndBindOauthAccount(req.Ticket, req.Name, req.Email, req.VerifyCode, req.Password, lang)
	if err != nil {
		respondOauthRegisterError(c, lang, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toRegisterResponse(model),
	})
}

// respondOauthRegisterError 把注册绑定的错误映射为对外状态码。
func respondOauthRegisterError(c *gin.Context, lang string, err error) {
	switch {
	case errors.Is(err, business.ErrOauthTicketInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgOauthTicketInvalid), `oauth.register.ticket_invalid`, err)
	case errors.Is(err, business.ErrVerifyCodeInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgVerifyCodeInvalid), `oauth.register.verify_code`, err)
	case errors.Is(err, business.ErrUserAlreadyExists):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgUserExist), `oauth.register.user_exists`, err)
	case errors.Is(err, business.ErrOauthBoundByOther), errors.Is(err, business.ErrOauthAlreadyBound):
		respondLoggedError(c, http.StatusConflict, http.StatusConflict,
			lib_i18n.Show(lang, define.MsgOauthBoundByOther), `oauth.register.bound`, err)
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `oauth.register`, err)
	}
}

// respondOauthLoginError 把第三方登录相关的业务错误映射为对外状态码。
func respondOauthLoginError(c *gin.Context, lang string, stage string, err error) {
	switch {
	case errors.Is(err, business.ErrOauthTicketInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgOauthTicketInvalid), stage+`.ticket`, err)
	case errors.Is(err, business.ErrOauthBoundByOther):
		respondLoggedError(c, http.StatusConflict, http.StatusConflict,
			lib_i18n.Show(lang, define.MsgOauthBoundByOther), stage+`.bound_other`, err)
	case errors.Is(err, business.ErrOauthAlreadyBound):
		respondLoggedError(c, http.StatusConflict, http.StatusConflict,
			lib_i18n.Show(lang, define.MsgOauthAlreadyBound), stage+`.already_bound`, err)
	case errors.Is(err, business.ErrLoginWrong):
		c.JSON(http.StatusUnauthorized, define.Response{
			Code:    http.StatusUnauthorized,
			Message: lib_i18n.Show(lang, define.MsgLoginWrong),
		})
	case errors.Is(err, business.ErrAccountDisabled):
		c.JSON(http.StatusUnauthorized, define.Response{
			Code:    http.StatusUnauthorized,
			Message: lib_i18n.Show(lang, define.MsgAccountDisabled),
		})
	case errors.Is(err, business.ErrUserNotExist):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgUserNotFound))
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), stage, err)
	}
}

// ======================== 个人设置 ========================

// oauthBindingItem 描述个人设置里的一个第三方账号绑定项。
type oauthBindingItem struct {
	Provider string `json:"provider"`
	Enabled  bool   `json:"enabled"`
	Bound    bool   `json:"bound"`
	Nickname string `json:"nickname"`
	BoundAt  string `json:"bound_at"`
}

// GetUserOauthBindings 返回当前用户的第三方账号绑定状态。
func GetUserOauthBindings(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	list, err := business.GetUserOauthBindingList(c.GetInt(`userId`))
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `oauth.bindings`, err)
		return
	}

	items := make([]oauthBindingItem, 0, len(list))
	for _, item := range list {
		items = append(items, oauthBindingItem{
			Provider: item.Provider,
			Enabled:  item.Enabled,
			Bound:    item.Bound,
			Nickname: item.Nickname,
			BoundAt:  strconv.FormatInt(item.BoundAt, 10),
		})
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    items,
	})
}

// DeleteUserOauthBinding 解除当前账号在指定平台的第三方账号绑定。
func DeleteUserOauthBinding(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	providerKey := strings.TrimSpace(c.Param(`provider`))
	if err := validate.ValidateOauthProvider(providerKey); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgOauthProviderUnknown), `oauth.unbind.validate`, err)
		return
	}

	if err := business.UnbindOauthAccount(c.GetInt(`adminId`), c.GetInt(`userId`), providerKey); err != nil {
		switch {
		case errors.Is(err, business.ErrOauthNotBound):
			abortNotFound(c, lib_i18n.Show(lang, define.MsgOauthNotBound))
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `oauth.unbind`, err)
		}
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

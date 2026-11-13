// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"net/http"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
	"github.com/zhimaAi/go_tools/logs"
)

// ======================== 企业 SSO 公开入口与回调 ========================

// SsoEntry 返回企业登录入口的最小展示信息（公开接口）。
//
// 一切不满足都回 enabled=false 且不带原因：这是匿名可读的接口，
// 回显「哪家企业开了 SSO / 缺哪一项配置」等于给探测者送信息。
func SsoEntry(c *gin.Context) {
	adminId, err := business.DecodeSsoScopeId(c.Query(`scope_id`))
	lang := lib_i18n.GetLang(c)
	data := business.SsoEntryModel{}
	if err == nil {
		data = business.SsoEntry(adminId, lang)
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    data,
	})
}

// SsoAuthorize 校验入口并 302 到 IdP 授权地址（公开接口）。
//
// 浏览器直接跳转，返回 JSON 用户看不到，因此失败也只能 302 回登录页带 oauth_error。
func SsoAuthorize(c *gin.Context) {
	target, err := business.SsoAuthorizeUrl(c.Query(`scope_id`))
	if err != nil {
		logs.Error("sso authorize failed: err=%s", err.Error())
		c.Redirect(http.StatusFound, oauthSiteBase(c)+ssoLoginErrorPath(ssoErrorReason(err)))
		return
	}
	c.Redirect(http.StatusFound, target)
}

// SsoCallback 处理 IdP 登录回调（公开接口，地址里的 key 是租户回调标识）。
func SsoCallback(c *gin.Context) {
	ssoCallbackHandler(c, false)
}

// SsoBindCallback 处理 IdP 绑定回调（公开接口，本期无发起入口，端点真实可用但不被触发）。
func SsoBindCallback(c *gin.Context) {
	ssoCallbackHandler(c, true)
}

func ssoCallbackHandler(c *gin.Context, bind bool) {
	providerKey := c.Param(`provider`)
	callbackKey := c.Param(`key`)
	code := c.Query(`code`)
	if code == `` {
		// 厂商在用户取消或 IdP 拒绝时只回 error，不回 code。
		logs.Error("sso callback without code: provider=%s, idp_error=%s", providerKey, c.Query(`error`))
		c.Redirect(http.StatusFound, oauthSiteBase(c)+ssoLoginErrorPath(define.SsoErrorAuthFailed))
		return
	}

	result, err := business.HandleSsoCallback(providerKey, callbackKey, code, c.Query(`state`), bind)
	if err != nil {
		// 回调 query 带着授权码，属凭据：只记 provider 与原因，不记请求上下文。
		logs.Error("sso callback failed: provider=%s, bind=%t, err=%s", providerKey, bind, err.Error())
		c.Redirect(http.StatusFound, ssoPersonalizationBase(c, result.AdminId)+ssoCallbackErrorPath(result.Scene, err))
		return
	}
	c.Redirect(http.StatusFound, ssoPersonalizationBase(c, result.AdminId)+result.Path)
}

// ssoCallbackErrorPath 决定失败回跳到哪个页面：绑定场景回个人设置，其余回登录页。
func ssoCallbackErrorPath(scene string, err error) string {
	reason := ssoErrorReason(err)
	if scene == define.SsoSceneBind {
		return `/profile?` + business.OauthQueryBindResult + `=failed&` + business.OauthQueryError + `=` + reason
	}
	return ssoLoginErrorPath(reason)
}

func ssoLoginErrorPath(reason string) string {
	return `/login?` + business.OauthQueryError + `=` + reason
}

// ssoErrorReason 把业务错误映射为前端可识别的错误原因取值。
// 未识别的一律归 server_error：内部错误文本不进浏览器地址。
func ssoErrorReason(err error) string {
	var quotaErr *business.QuotaExceededError
	switch {
	case errors.Is(err, business.ErrSsoDisabled), errors.Is(err, business.ErrSsoProviderInvalid),
		errors.Is(err, business.ErrSsoConfigIncomplete):
		return define.SsoErrorDisabled
	case errors.Is(err, business.ErrSsoStateInvalid):
		return define.SsoErrorStateInvalid
	case errors.Is(err, business.ErrSsoEndpointUnreachable):
		return define.SsoErrorEndpointGone
	case errors.Is(err, business.ErrSsoAuthFailed):
		return define.SsoErrorAuthFailed
	case errors.Is(err, business.ErrAccountDisabled), errors.Is(err, business.ErrUserNotExist):
		return define.SsoErrorAccountDisabled
	case errors.As(err, &quotaErr):
		return define.SsoErrorQuotaExceeded
	default:
		return define.SsoErrorServer
	}
}

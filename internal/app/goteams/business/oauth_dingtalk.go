// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"net/http"
	"strings"

	"goteams/internal/app/goteams/define"

	"github.com/zhimaAi/go_tools/logs"
)

// 钉钉扫码登录使用的固定厂商地址，参数含义见钉钉开放平台「扫码登录第三方网站」。
const (
	dingtalkSdkURL        = `https://g.alicdn.com/dingding/h5-dingtalk-login/0.21.0/ddlogin.js`
	dingtalkAuthorizeURL  = `https://login.dingtalk.com/oauth2/auth`
	dingtalkUserTokenURL  = `https://api.dingtalk.com/v1.0/oauth2/userAccessToken`
	dingtalkUserInfoURL   = `https://api.dingtalk.com/v1.0/contact/users/me`
	dingtalkAccessTokenHd = `x-acs-dingtalk-access-token`
)

// dingtalkProvider 实现钉钉扫码登录。
type dingtalkProvider struct{}

// Key 返回钉钉的平台标识。
func (dingtalkProvider) Key() string {
	return define.OauthProviderDingtalk
}

// Enabled 判断钉钉是否已开启且凭证配置完整。
func (dingtalkProvider) Enabled() bool {
	if !oauthProviderSwitchOn(define.Config.Dingtalk) {
		return false
	}
	return strings.TrimSpace(define.Config.Dingtalk[`app_key`]) != `` &&
		strings.TrimSpace(define.Config.Dingtalk[`app_secret`]) != ``
}

// ClientId 返回钉钉应用的 AppKey。
func (dingtalkProvider) ClientId() string {
	return strings.TrimSpace(define.Config.Dingtalk[`app_key`])
}

// SdkURL 返回钉钉二维码 SDK 的脚本地址。
func (dingtalkProvider) SdkURL() string {
	return dingtalkSdkURL
}

// CodeQueryKey 返回回调地址上授权码的参数名，钉钉使用 authCode。
func (dingtalkProvider) CodeQueryKey() string {
	return `authCode`
}

// AuthorizeURL 拼装钉钉授权地址，作为二维码内容。
func (dingtalkProvider) AuthorizeURL(redirectUri string, state string) string {
	query := make([]string, 0, 6)
	query = append(query, `redirect_uri=`+oauthEscape(redirectUri))
	query = append(query, `response_type=code`)
	query = append(query, `client_id=`+oauthEscape((dingtalkProvider{}).ClientId()))
	query = append(query, `scope=openid`)
	query = append(query, `state=`+oauthEscape(state))
	query = append(query, `prompt=consent`)
	return dingtalkAuthorizeURL + `?` + strings.Join(query, `&`)
}

// dingtalkUserTokenResponse 是钉钉换取用户令牌的响应。
type dingtalkUserTokenResponse struct {
	AccessToken string `json:"accessToken"`
	ExpireIn    int    `json:"expireIn"`
}

// dingtalkUserInfoResponse 是钉钉获取用户个人信息的响应。
type dingtalkUserInfoResponse struct {
	Nick      string `json:"nick"`
	AvatarUrl string `json:"avatarUrl"`
	OpenId    string `json:"openId"`
	UnionId   string `json:"unionId"`
}

// ExchangeCode 用授权码换取钉钉用户身份。
// 先换取用户级 accessToken，再按 unionId=me 拉取当前授权人的个人信息。
func (dingtalkProvider) ExchangeCode(code string, redirectUri string) (OauthUserInfo, error) {
	if strings.TrimSpace(code) == `` {
		return OauthUserInfo{}, ErrOauthCodeInvalid
	}

	tokenBody := map[string]string{
		`clientId`:     (dingtalkProvider{}).ClientId(),
		`clientSecret`: strings.TrimSpace(define.Config.Dingtalk[`app_secret`]),
		`code`:         code,
		`grantType`:    `authorization_code`,
	}
	var token dingtalkUserTokenResponse
	if err := oauthCallJSON(http.MethodPost, dingtalkUserTokenURL, nil, tokenBody, &token); err != nil {
		logs.Error("exchange dingtalk user token failed: %s", err.Error())
		return OauthUserInfo{}, ErrOauthCodeInvalid
	}
	if token.AccessToken == `` {
		logs.Error("exchange dingtalk user token failed: empty access token")
		return OauthUserInfo{}, ErrOauthCodeInvalid
	}

	var info dingtalkUserInfoResponse
	headers := map[string]string{dingtalkAccessTokenHd: token.AccessToken}
	if err := oauthCallJSON(http.MethodGet, dingtalkUserInfoURL, headers, nil, &info); err != nil {
		logs.Error("fetch dingtalk user info failed: %s", err.Error())
		return OauthUserInfo{}, ErrOauthCodeInvalid
	}
	if info.UnionId == `` {
		logs.Error("fetch dingtalk user info failed: empty union id")
		return OauthUserInfo{}, ErrOauthCodeInvalid
	}

	return OauthUserInfo{
		UnionId:  info.UnionId,
		OpenId:   info.OpenId,
		Nickname: info.Nick,
		Avatar:   info.AvatarUrl,
	}, nil
}

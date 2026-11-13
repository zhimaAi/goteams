// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"net/http"
	"strings"

	"goteams/internal/app/goteams/define"

	"github.com/zhimaAi/go_tools/logs"
)

// 飞书扫码登录使用的固定厂商地址，参数含义见飞书开放平台「网页应用登录 / 二维码 SDK 接入文档」。
// 二维码 SDK 只支持旧版登录流程，因此换取令牌仍走 v2 接口。
const (
	feishuSdkURL       = `https://lf-package-cn.feishucdn.com/obj/feishu-static/lark/passport/qrcode/LarkSSOSDKWebQRCode-1.0.3.js`
	feishuAuthorizeURL = `https://passport.feishu.cn/suite/passport/oauth/authorize`
	feishuUserTokenURL = `https://open.feishu.cn/open-apis/authen/v2/oauth/token`
	feishuUserInfoURL  = `https://open.feishu.cn/open-apis/authen/v1/user_info`
)

// feishuProvider 实现飞书扫码登录。
type feishuProvider struct{}

// Key 返回飞书的平台标识。
func (feishuProvider) Key() string {
	return define.OauthProviderFeishu
}

// Enabled 判断飞书是否已开启且凭证配置完整。
func (feishuProvider) Enabled() bool {
	if !oauthProviderSwitchOn(define.Config.Feishu) {
		return false
	}
	return strings.TrimSpace(define.Config.Feishu[`app_id`]) != `` &&
		strings.TrimSpace(define.Config.Feishu[`app_secret`]) != ``
}

// ClientId 返回飞书应用的 App ID。
func (feishuProvider) ClientId() string {
	return strings.TrimSpace(define.Config.Feishu[`app_id`])
}

// SdkURL 返回飞书二维码 SDK 的脚本地址。
func (feishuProvider) SdkURL() string {
	return feishuSdkURL
}

// CodeQueryKey 返回回调地址上授权码的参数名，飞书使用 code。
func (feishuProvider) CodeQueryKey() string {
	return `code`
}

// AuthorizeURL 拼装飞书授权地址，前端把二维码 SDK 回传的 tmp_code 拼到该地址后跳转。
func (feishuProvider) AuthorizeURL(redirectUri string, state string) string {
	query := make([]string, 0, 4)
	query = append(query, `client_id=`+oauthEscape((feishuProvider{}).ClientId()))
	query = append(query, `redirect_uri=`+oauthEscape(redirectUri))
	query = append(query, `response_type=code`)
	query = append(query, `state=`+oauthEscape(state))
	return feishuAuthorizeURL + `?` + strings.Join(query, `&`)
}

// feishuUserTokenResponse 是飞书换取 user_access_token 的响应。
// 注意：v2 接口的字段直接位于响应体顶层，不在 data 里。
type feishuUserTokenResponse struct {
	Code         int    `json:"code"`
	AccessToken  string `json:"access_token"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
	TokenType    string `json:"token_type"`
	ExpiresInSec int    `json:"expires_in"`
}

// feishuUserInfoResponse 是飞书获取用户信息的响应，用户字段包在 data 里。
type feishuUserInfoResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Name      string `json:"name"`
		AvatarUrl string `json:"avatar_url"`
		OpenId    string `json:"open_id"`
		UnionId   string `json:"union_id"`
	} `json:"data"`
}

// ExchangeCode 用授权码换取飞书用户身份。
// 先换取 user_access_token，再拉取用户信息。
func (feishuProvider) ExchangeCode(code string, redirectUri string) (OauthUserInfo, error) {
	if strings.TrimSpace(code) == `` {
		return OauthUserInfo{}, ErrOauthCodeInvalid
	}

	tokenBody := map[string]string{
		`grant_type`:    `authorization_code`,
		`client_id`:     (feishuProvider{}).ClientId(),
		`client_secret`: strings.TrimSpace(define.Config.Feishu[`app_secret`]),
		`code`:          code,
		`redirect_uri`:  redirectUri,
	}
	var token feishuUserTokenResponse
	if err := oauthCallJSON(http.MethodPost, feishuUserTokenURL, nil, tokenBody, &token); err != nil {
		logs.Error("exchange feishu user token failed: %s", err.Error())
		return OauthUserInfo{}, ErrOauthCodeInvalid
	}
	if token.Code != 0 || token.AccessToken == `` {
		// 飞书的错误码与错误描述对排查配置问题有用，且不含令牌。
		logs.Error("exchange feishu user token failed: code=%d, error=%s", token.Code, token.Error)
		return OauthUserInfo{}, ErrOauthCodeInvalid
	}

	var info feishuUserInfoResponse
	headers := map[string]string{`Authorization`: `Bearer ` + token.AccessToken}
	if err := oauthCallJSON(http.MethodGet, feishuUserInfoURL, headers, nil, &info); err != nil {
		logs.Error("fetch feishu user info failed: %s", err.Error())
		return OauthUserInfo{}, ErrOauthCodeInvalid
	}
	if info.Code != 0 || info.Data.UnionId == `` {
		logs.Error("fetch feishu user info failed: code=%d, msg=%s", info.Code, info.Msg)
		return OauthUserInfo{}, ErrOauthCodeInvalid
	}

	return OauthUserInfo{
		UnionId:  info.Data.UnionId,
		OpenId:   info.Data.OpenId,
		Nickname: info.Data.Name,
		Avatar:   info.Data.AvatarUrl,
	}, nil
}

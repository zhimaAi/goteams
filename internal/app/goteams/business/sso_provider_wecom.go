// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"goteams/internal/app/goteams/define"

	"github.com/zhimaAi/go_tools/logs"
)

// 企业微信的固定厂商地址。
//
// 授权用的是 wwlogin（浏览器扫码登录），不是 open.weixin.qq.com 的「微信内网页授权」——
// 后者在 PC 浏览器里会提示「请在微信客户端打开链接」。本系统是 PC Web 场景，只能用前者。
const (
	ssoWecomAuthorizeURL  = `https://login.work.weixin.qq.com/wwlogin/sso/login`
	ssoWecomTokenURL      = `https://qyapi.weixin.qq.com/cgi-bin/gettoken`
	ssoWecomUserInfoURL   = `https://qyapi.weixin.qq.com/cgi-bin/auth/getuserinfo`
	ssoWecomUserDetailURL = `https://qyapi.weixin.qq.com/cgi-bin/user/get`
	// 成员同步用的通讯录接口：department/list 一次给全量部门树，user/list 按部门给成员详情。
	ssoWecomDeptListURL = `https://qyapi.weixin.qq.com/cgi-bin/department/list`
	ssoWecomUserListURL = `https://qyapi.weixin.qq.com/cgi-bin/user/list`
)

// wecomSsoProvider 实现企业微信企业 SSO 登录。
//
// 身份取 auth/getuserinfo 的 userid，与成员同步（user/list）同命名空间。
// corpsecret 换到的应用 access_token 用完即弃，不落库。
type wecomSsoProvider struct{}

func (wecomSsoProvider) Key() string { return define.SsoProviderWecom }

// TestConnect 用 CorpId + CorpSecret 换一次应用 access_token，验到凭证有效性这一层（SsoTestLevelCredentials）。
func (p wecomSsoProvider) TestConnect(config SsoConfigModel) (string, error) {
	if _, err := ssoWecomAccessToken(config.AppKey, config.AppSecret); err != nil {
		return ``, err
	}
	return SsoTestLevelCredentials, nil
}

func (p wecomSsoProvider) AuthorizeURL(config SsoConfigModel, state SsoAuthzState) (string, error) {
	query := url.Values{}
	query.Set(`login_type`, `CorpApp`)
	query.Set(`appid`, config.AppKey)
	query.Set(`agentid`, config.AgentId)
	query.Set(`redirect_uri`, state.RedirectUri)
	query.Set(`state`, state.State)
	return ssoWecomAuthorizeURL + `?` + query.Encode(), nil
}

// ssoWecomErrorResponse 是企业微信所有接口的公共错误字段。
type ssoWecomErrorResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// ssoWecomTokenResponse 是应用 access_token 响应。
type ssoWecomTokenResponse struct {
	ssoWecomErrorResponse
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// ssoWecomAuthResponse 是授权码换身份的响应：外部联系人只有 external_userid，没有 userid。
type ssoWecomAuthResponse struct {
	ssoWecomErrorResponse
	UserId         string `json:"userid"`
	ExternalUserId string `json:"external_userid"`
}

// ssoWecomUserDetailResponse 是通讯录成员详情，字段多于本系统需要的部分由 json 自动忽略。
type ssoWecomUserDetailResponse struct {
	ssoWecomErrorResponse
	UserId   string `json:"userid"`
	Name     string `json:"name"`
	Mobile   string `json:"mobile"`
	Email    string `json:"email"`
	Position string `json:"position"`
	Avatar   string `json:"avatar"`
}

func (p wecomSsoProvider) ExchangeCode(config SsoConfigModel, code string, state SsoAuthzState) (SsoUserInfo, error) {
	accessToken, err := ssoWecomAccessToken(config.AppKey, config.AppSecret)
	if err != nil {
		return SsoUserInfo{}, err
	}

	var auth ssoWecomAuthResponse
	// 企微接口统一把 access_token 放 query，凭证与授权码都不进请求体。
	target := fmt.Sprintf(`%s?access_token=%s&code=%s`, ssoWecomUserInfoURL,
		url.QueryEscape(accessToken), url.QueryEscape(code))
	if err = ssoVendorCallJSON(http.MethodGet, target, nil, nil, &auth); err != nil {
		logs.Error("sso wecom get userinfo by code failed: %s", err.Error())
		return SsoUserInfo{}, fmt.Errorf("%w: wecom auth endpoint", ErrSsoAuthFailed)
	}
	if auth.ErrCode != 0 || auth.UserId == `` {
		// 外部联系人（微信用户）拿不到 userid，不是企业成员，按认证失败处理。
		logs.Error("sso wecom auth rejected: errcode=%d, errmsg=%s, external=%t",
			auth.ErrCode, auth.ErrMsg, auth.ExternalUserId != ``)
		return SsoUserInfo{}, fmt.Errorf("%w: wecom identity rejected", ErrSsoAuthFailed)
	}

	info := SsoUserInfo{ExternalId: auth.UserId}
	var detail ssoWecomUserDetailResponse
	detailUrl := fmt.Sprintf(`%s?access_token=%s&userid=%s`, ssoWecomUserDetailURL,
		url.QueryEscape(accessToken), url.QueryEscape(auth.UserId))
	if err = ssoVendorCallJSON(http.MethodGet, detailUrl, nil, nil, &detail); err != nil {
		// 详情接口失败只影响姓名/邮箱等资料，身份已经拿到：降级为「只有标识」的登录，不拒绝。
		// 应用未开通通讯录读取权限时就会走到这里，管理员在成员同步里补齐权限后资料自然完整。
		logs.Error("sso wecom fetch user detail failed: err=%s", err.Error())
		return info, nil
	}
	if detail.ErrCode != 0 {
		logs.Error("sso wecom user detail rejected: errcode=%d, errmsg=%s", detail.ErrCode, detail.ErrMsg)
		return info, nil
	}
	info.Name = strings.TrimSpace(detail.Name)
	info.Phone = strings.TrimSpace(detail.Mobile)
	info.Email = strings.TrimSpace(detail.Email)
	info.JobTitle = strings.TrimSpace(detail.Position)
	info.Avatar = strings.TrimSpace(detail.Avatar)
	return info, nil
}

// ssoWecomAccessToken 用 CorpId + CorpSecret 换应用级 access_token。
func ssoWecomAccessToken(corpId string, corpSecret string) (string, error) {
	target := fmt.Sprintf(`%s?corpid=%s&corpsecret=%s`, ssoWecomTokenURL,
		url.QueryEscape(corpId), url.QueryEscape(corpSecret))
	var token ssoWecomTokenResponse
	if err := ssoVendorCallJSON(http.MethodGet, target, nil, nil, &token); err != nil {
		logs.Error("sso wecom fetch access token failed: %s", err.Error())
		return ``, fmt.Errorf("%w: wecom token endpoint", ErrSsoAuthFailed)
	}
	if token.ErrCode != 0 || token.AccessToken == `` {
		logs.Error("sso wecom fetch access token rejected: errcode=%d, errmsg=%s", token.ErrCode, token.ErrMsg)
		return ``, fmt.Errorf("%w: wecom token rejected", ErrSsoAuthFailed)
	}
	return token.AccessToken, nil
}

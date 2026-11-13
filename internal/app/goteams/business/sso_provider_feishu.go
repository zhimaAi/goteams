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

// 飞书网页应用登录（SSO）的固定厂商地址。授权域名是 accounts.feishu.cn，
// 与 2398 扫码登录用的 passport.feishu.cn 不是同一条链路：那条是二维码 SDK 的旧流程。
const (
	ssoFeishuAuthorizeURL = `https://accounts.feishu.cn/open-apis/authen/v1/authorize`
	ssoFeishuTokenURL     = `https://open.feishu.cn/open-apis/authen/v2/oauth/token`
	ssoFeishuUserInfoURL  = `https://open.feishu.cn/open-apis/authen/v1/user_info`
	// 成员同步用的通讯录接口（contact/v3，收 tenant_access_token）。
	// 子部门列表的部门 id 是路径参数（官方现行文档为 /departments/{department_id}/children，
	// 没有 parent_department_id 查询参数），所以这里只存前缀，由调用方拼部门 id。
	ssoFeishuDepartmentsURL = `https://open.feishu.cn/open-apis/contact/v3/departments`
	ssoFeishuUsersURL       = `https://open.feishu.cn/open-apis/contact/v3/users/find_by_department`
	// ssoFeishuRootDepartment 是企业根部门 id。children 不返回根部门本身，但根部门的直属成员
	// 只能用 department_id=0 查（官方 FAQ「如何获取根部门下的员工信息」）。
	ssoFeishuRootDepartment = `0`
)

// feishuSsoProvider 实现飞书企业 SSO 登录。
//
// 身份取 user_info 的 open_id，与成员同步（contact/v3/users）用的是同一命名空间，
// 因此「先同步建号、本人再登录」不会建出第二个账号。
type feishuSsoProvider struct{}

func (feishuSsoProvider) Key() string { return define.SsoProviderFeishu }

// ssoFeishuTenantTokenURL 是自建应用换取 tenant_access_token 的内部端点。
const ssoFeishuTenantTokenURL = `https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal`

// ssoFeishuTenantTokenResponse 是 tenant_access_token 的响应。
type ssoFeishuTenantTokenResponse struct {
	Code        int    `json:"code"`
	Msg         string `json:"msg"`
	AccessToken string `json:"tenant_access_token"`
}

// TestConnect 用应用凭证换一次 tenant_access_token，验到凭证有效性这一层（SsoTestLevelCredentials）。
func (p feishuSsoProvider) TestConnect(config SsoConfigModel) (string, error) {
	if _, err := ssoFeishuTenantToken(config.AppKey, config.AppSecret); err != nil {
		return ``, err
	}
	return SsoTestLevelCredentials, nil
}

// ssoFeishuTenantToken 用 AppId + AppSecret 换 tenant_access_token。
func ssoFeishuTenantToken(appId string, appSecret string) (string, error) {
	body := map[string]string{`app_id`: appId, `app_secret`: appSecret}
	var token ssoFeishuTenantTokenResponse
	if err := ssoVendorCallJSON(http.MethodPost, ssoFeishuTenantTokenURL, nil, body, &token); err != nil {
		logs.Error("sso feishu fetch tenant token failed: %s", err.Error())
		return ``, fmt.Errorf("%w: feishu token endpoint", ErrSsoAuthFailed)
	}
	if token.Code != 0 || token.AccessToken == `` {
		logs.Error("sso feishu fetch tenant token rejected: code=%d, msg=%s", token.Code, token.Msg)
		return ``, fmt.Errorf("%w: feishu tenant token rejected", ErrSsoAuthFailed)
	}
	return token.AccessToken, nil
}

// AuthorizeURL 拼飞书授权地址。
//
// scope 只在管理员显式配置时才带上，不设兜底值：登录链路本身不需要任何 scope——
// 授权码换到的 user_access_token 调用 /authen/v1/user_info 的权限要求是「无」，
// open_id / name / avatar_url 开箱即可读。也不能填占位值，飞书授权页会校验 scope
// 是否为有效权限点，填一个它不认识的值会直接拒绝授权（错误码 20043，
// 报「xxx 有误，请修改后重试」）。需要 refresh_token 之类的额外授权时由管理员在
// 配置里填 offline_access 等有效权限点。
func (p feishuSsoProvider) AuthorizeURL(config SsoConfigModel, state SsoAuthzState) (string, error) {
	query := url.Values{}
	query.Set(`client_id`, config.AppKey)
	query.Set(`redirect_uri`, state.RedirectUri)
	query.Set(`response_type`, `code`)
	query.Set(`state`, state.State)
	if scopes := strings.TrimSpace(config.Scopes); scopes != `` {
		query.Set(`scope`, scopes)
	}
	return ssoFeishuAuthorizeURL + `?` + query.Encode(), nil
}

// ssoFeishuTokenResponse 是 v2 换取 user_access_token 的响应。字段在顶层，不包在 data 里。
type ssoFeishuTokenResponse struct {
	Code        int    `json:"code"`
	Msg         string `json:"msg"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
	AccessToken string `json:"access_token"`
}

// ssoFeishuUserInfoResponse 是用户信息响应；资料字段包在 data 里。
type ssoFeishuUserInfoResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		OpenId    string `json:"open_id"`
		UnionId   string `json:"union_id"`
		Name      string `json:"name"`
		AvatarUrl string `json:"avatar_url"`
	} `json:"data"`
}

func (p feishuSsoProvider) ExchangeCode(config SsoConfigModel, code string, state SsoAuthzState) (SsoUserInfo, error) {
	body := map[string]string{
		`grant_type`:    `authorization_code`,
		`client_id`:     config.AppKey,
		`client_secret`: config.AppSecret,
		`code`:          code,
		`redirect_uri`:  state.RedirectUri,
	}
	var token ssoFeishuTokenResponse
	// 飞书 v2 令牌端点收 JSON 体，与标准 OAuth2 的表单编码不同，因此不用 SsoExchangeCodeForToken。
	if err := ssoVendorCallJSON(http.MethodPost, ssoFeishuTokenURL, nil, body, &token); err != nil {
		logs.Error("sso feishu exchange token failed: %s", err.Error())
		return SsoUserInfo{}, fmt.Errorf("%w: feishu token endpoint", ErrSsoAuthFailed)
	}
	if token.Code != 0 || token.AccessToken == `` {
		logs.Error("sso feishu exchange token failed: code=%d, error=%s", token.Code, token.Error)
		return SsoUserInfo{}, fmt.Errorf("%w: feishu token rejected", ErrSsoAuthFailed)
	}

	var resp ssoFeishuUserInfoResponse
	headers := map[string]string{`Authorization`: `Bearer ` + token.AccessToken}
	if err := ssoVendorCallJSON(http.MethodGet, ssoFeishuUserInfoURL, headers, nil, &resp); err != nil {
		logs.Error("sso feishu fetch user info failed: %s", err.Error())
		return SsoUserInfo{}, fmt.Errorf("%w: feishu user info endpoint", ErrSsoAuthFailed)
	}
	if resp.Code != 0 || resp.Data.OpenId == `` {
		logs.Error("sso feishu fetch user info failed: code=%d, msg=%s", resp.Code, resp.Msg)
		return SsoUserInfo{}, fmt.Errorf("%w: feishu user info rejected", ErrSsoAuthFailed)
	}
	return SsoUserInfo{
		ExternalId: resp.Data.OpenId,
		Name:       resp.Data.Name,
		Avatar:     strings.TrimSpace(resp.Data.AvatarUrl),
	}, nil
}

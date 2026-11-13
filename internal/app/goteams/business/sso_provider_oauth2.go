// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"fmt"
	"net/url"
	"strings"

	"goteams/internal/app/goteams/define"
)

// oauth2Provider 实现通用 OAuth2.0：authorize / token / userinfo 三段端点全部由租户配置，
// 身份值取 userinfo 里配置的 external_id_field。
//
// 与 OIDC 的区别只有两点：没有 discovery（端点是配置项），也没有可验签的 id_token。
// 因此 userinfo 端点就是身份的唯一来源，地址必须走受控客户端。
type oauth2Provider struct{}

func (oauth2Provider) Key() string { return define.SsoProviderOauth2 }

// ssoOauth2DefaultScopes 是未填 scope 时的兜底，也是界面切到本平台时的预填值。
// 带 openid：部分 OAuth2.0 服务要求 scope 含 openid 才放行授权，少一个就 400。
const ssoOauth2DefaultScopes = `openid profile email`

// TestConnect 逐个探测已填的授权/令牌/用户信息端点是否可达。
// 探测不带凭据，端点回 4xx 也算可达；只能验到端点这一层（SsoTestLevelEndpoints）。
func (p oauth2Provider) TestConnect(config SsoConfigModel) (string, error) {
	endpoints := []string{config.AuthorizeUrl, config.TokenUrl, config.UserInfoUrl}
	checked := 0
	for _, endpoint := range endpoints {
		endpoint = strings.TrimSpace(endpoint)
		if endpoint == `` {
			continue
		}
		if err := ssoProbeEndpoint(endpoint, nil); err != nil {
			return ``, err
		}
		checked++
	}
	if checked == 0 {
		return ``, fmt.Errorf("%w: no oauth2 endpoint configured", ErrSsoConfigIncomplete)
	}
	return SsoTestLevelEndpoints, nil
}

func (p oauth2Provider) AuthorizeURL(config SsoConfigModel, state SsoAuthzState) (string, error) {
	if strings.TrimSpace(config.AuthorizeUrl) == `` {
		return ``, fmt.Errorf("%w: authorize_url is required", ErrSsoConfigIncomplete)
	}
	query := url.Values{}
	query.Set(`response_type`, `code`)
	query.Set(`client_id`, config.AppKey)
	query.Set(`redirect_uri`, state.RedirectUri)
	query.Set(`scope`, ssoScopeString(config.Scopes, ssoOauth2DefaultScopes))
	query.Set(`state`, state.State)
	if state.CodeVerifier != `` {
		query.Set(`code_challenge`, ssoCodeChallenge(state.CodeVerifier))
		query.Set(`code_challenge_method`, `S256`)
	}
	return ssoAppendQuery(config.AuthorizeUrl, query.Encode()), nil
}

func (p oauth2Provider) ExchangeCode(config SsoConfigModel, code string, state SsoAuthzState) (SsoUserInfo, error) {
	if strings.TrimSpace(config.TokenUrl) == `` || strings.TrimSpace(config.UserInfoUrl) == `` {
		return SsoUserInfo{}, fmt.Errorf("%w: token_url and userinfo_url are required", ErrSsoConfigIncomplete)
	}
	token, err := SsoExchangeCodeForToken(config.TokenUrl, config.AppKey, config.AppSecret, state, code)
	if err != nil {
		return SsoUserInfo{}, err
	}
	fields, err := SsoFetchUserInfo(config.UserInfoUrl, token.AccessToken)
	if err != nil {
		return SsoUserInfo{}, err
	}
	field := strings.TrimSpace(config.ExternalIdField)
	if field == `` {
		field = `sub`
	}
	info := SsoUserInfo{
		ExternalId: ssoStringField(fields, field),
		Username:   ssoStringField(fields, `preferred_username`, `username`, `user_name`),
		Email:      ssoStringField(fields, `email`),
		Name:       ssoStringField(fields, `name`, `display_name`),
		Phone:      ssoStringField(fields, `phone_number`, `mobile`),
		Avatar:     ssoStringField(fields, `picture`, `avatar`),
	}
	if info.ExternalId == `` {
		// 不回显响应内容：租户自配端点的响应可能带内部信息，只报「该字段取不到值」。
		return SsoUserInfo{}, fmt.Errorf("%w: userinfo has no %s", ErrSsoAuthFailed, field)
	}
	return info, nil
}

// ssoAppendQuery 把 query 拼到配置里给的端点上，保留它可能自带的参数。
func ssoAppendQuery(endpoint string, query string) string {
	endpoint = strings.TrimSpace(endpoint)
	if query == `` {
		return endpoint
	}
	if strings.Contains(endpoint, `?`) {
		return endpoint + `&` + query
	}
	return endpoint + `?` + query
}

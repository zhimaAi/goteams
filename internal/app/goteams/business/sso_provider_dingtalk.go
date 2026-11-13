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

// 钉钉企业内部应用登录的固定厂商地址。
//
// 授权与用户级令牌走新版 api.dingtalk.com；unionid→userid 的换算只有旧版 topapi 提供，
// 它收应用级 access_token（放 query），与用户级令牌不是一回事，两把令牌都用完即弃。
const (
	ssoDingtalkAuthorizeURL = `https://login.dingtalk.com/oauth2/auth`
	ssoDingtalkUserTokenURL = `https://api.dingtalk.com/v1.0/oauth2/userAccessToken`
	ssoDingtalkUserInfoURL  = `https://api.dingtalk.com/v1.0/contact/users/me`
	ssoDingtalkAppTokenURL  = `https://api.dingtalk.com/v1.0/oauth2/accessToken`
	// unionid → userid 只有旧版 topapi 一个地址，官方文档给的就是 /topapi/user/getbyunionid，没有 v2。
	// 写成 /topapi/v2/... 会被网关按路径映射成 dingtalk.oapi.v2.user.getbyunionid 并判成
	// 「不合法ApiName」（errcode=22），整个登录链路会卡在这一步。
	ssoDingtalkByUnionIdURL = `https://oapi.dingtalk.com/topapi/user/getbyunionid`
	// 成员同步用的通讯录接口（旧版 topapi，收应用级 access_token 放 query）。
	ssoDingtalkDeptListURL    = `https://oapi.dingtalk.com/topapi/v2/department/listsub`
	ssoDingtalkUserListURL    = `https://oapi.dingtalk.com/topapi/v2/user/list`
	dingtalkAccessTokenHeader = `x-acs-dingtalk-access-token`
	// ssoDingtalkRootDeptId 是钉钉的根部门 id。listsub 不返回根部门本身，但根部门的直属成员
	// 只能用 dept_id=1 查（官方文档：根部门该参数传 1，且只返回该部门直属员工、不含子部门）。
	ssoDingtalkRootDeptId = 1
	// ssoDingtalkContactTypeEmployee 是 getbyunionid 返回里「企业内部员工」的 contact_type 取值。
	// 另一个取值 1 是「企业外部联系人」——不是员工，但同样能换到 userid，必须挡掉。
	ssoDingtalkContactTypeEmployee = `0`
)

// dingtalkSsoProvider 实现钉钉企业内部应用 SSO 登录。
//
// 身份取通讯录 userid（由 unionId 换算），与成员同步取的是同一命名空间。
// 只用 unionId 会让「同步先建的号」和「本人登录」对不上，因此这步换算不能省。
type dingtalkSsoProvider struct{}

func (dingtalkSsoProvider) Key() string { return define.SsoProviderDingtalk }

// ssoDingtalkDefaultScopes 是未填 scope 时的兜底：corpid 才能让钉钉把授权人认到企业上下文里。
const ssoDingtalkDefaultScopes = `openid corpid`

// TestConnect 用企业凭证换一次应用级令牌，验到凭证有效性这一层（SsoTestLevelCredentials）。
func (p dingtalkSsoProvider) TestConnect(config SsoConfigModel) (string, error) {
	if _, err := ssoDingtalkAppToken(config.AppKey, config.AppSecret); err != nil {
		return ``, err
	}
	return SsoTestLevelCredentials, nil
}

func (p dingtalkSsoProvider) AuthorizeURL(config SsoConfigModel, state SsoAuthzState) (string, error) {
	query := url.Values{}
	query.Set(`redirect_uri`, state.RedirectUri)
	query.Set(`response_type`, `code`)
	query.Set(`client_id`, config.AppKey)
	query.Set(`scope`, ssoScopeString(config.Scopes, ssoDingtalkDefaultScopes))
	query.Set(`state`, state.State)
	query.Set(`prompt`, `consent`)
	return ssoDingtalkAuthorizeURL + `?` + query.Encode(), nil
}

// ssoDingtalkTokenResponse 是两版令牌接口共用的响应形态（用户级与应用级字段名一致）。
type ssoDingtalkTokenResponse struct {
	AccessToken string `json:"accessToken"`
	ExpireIn    int    `json:"expireIn"`
}

// ssoDingtalkUserInfoResponse 是 /contact/users/me 的响应，只有个人维度字段，没有企业 userid。
type ssoDingtalkUserInfoResponse struct {
	Nick      string `json:"nick"`
	AvatarUrl string `json:"avatarUrl"`
	Mobile    string `json:"mobile"`
	OpenId    string `json:"openId"`
	UnionId   string `json:"unionId"`
	Email     string `json:"email"`
}

// ssoDingtalkTopApiResponse 是旧版 topapi 的公共信封；result 由各方接口自行解释。
type ssoDingtalkTopApiResponse struct {
	ErrCode int            `json:"errcode"`
	ErrMsg  string         `json:"errmsg"`
	Result  map[string]any `json:"result"`
}

func (p dingtalkSsoProvider) ExchangeCode(config SsoConfigModel, code string, state SsoAuthzState) (SsoUserInfo, error) {
	userToken, err := ssoDingtalkUserToken(config, code)
	if err != nil {
		return SsoUserInfo{}, err
	}

	var me ssoDingtalkUserInfoResponse
	headers := map[string]string{dingtalkAccessTokenHeader: userToken}
	if err = ssoVendorCallJSON(http.MethodGet, ssoDingtalkUserInfoURL, headers, nil, &me); err != nil {
		logs.Error("sso dingtalk fetch user info failed: %s", err.Error())
		return SsoUserInfo{}, fmt.Errorf("%w: dingtalk user info endpoint", ErrSsoAuthFailed)
	}
	if me.UnionId == `` {
		return SsoUserInfo{}, fmt.Errorf("%w: dingtalk user info has no union id", ErrSsoAuthFailed)
	}

	appToken, err := ssoDingtalkAppToken(config.AppKey, config.AppSecret)
	if err != nil {
		return SsoUserInfo{}, err
	}
	userid, err := ssoDingtalkUseridByUnionId(appToken, me.UnionId)
	if err != nil {
		return SsoUserInfo{}, err
	}

	return SsoUserInfo{
		ExternalId: userid,
		Name:       strings.TrimSpace(me.Nick),
		Phone:      strings.TrimSpace(me.Mobile),
		Email:      strings.TrimSpace(me.Email),
		Avatar:     strings.TrimSpace(me.AvatarUrl),
	}, nil
}

// ssoDingtalkUserToken 用授权码换用户级令牌；该接口不校验 redirect_uri，因此不需要事务里的回调地址。
func ssoDingtalkUserToken(config SsoConfigModel, code string) (string, error) {
	body := map[string]string{
		`clientId`:     config.AppKey,
		`clientSecret`: config.AppSecret,
		`code`:         code,
		`grantType`:    `authorization_code`,
	}
	var token ssoDingtalkTokenResponse
	if err := ssoVendorCallJSON(http.MethodPost, ssoDingtalkUserTokenURL, nil, body, &token); err != nil {
		logs.Error("sso dingtalk exchange user token failed: %s", err.Error())
		return ``, fmt.Errorf("%w: dingtalk token endpoint", ErrSsoAuthFailed)
	}
	if token.AccessToken == `` {
		return ``, fmt.Errorf("%w: dingtalk user token is empty", ErrSsoAuthFailed)
	}
	return token.AccessToken, nil
}

// ssoDingtalkAppToken 用企业内部应用凭证换应用级令牌（通讯录接口需要它）。
func ssoDingtalkAppToken(appKey string, appSecret string) (string, error) {
	body := map[string]string{`appKey`: appKey, `appSecret`: appSecret}
	var token ssoDingtalkTokenResponse
	if err := ssoVendorCallJSON(http.MethodPost, ssoDingtalkAppTokenURL, nil, body, &token); err != nil {
		logs.Error("sso dingtalk fetch app token failed: %s", err.Error())
		return ``, fmt.Errorf("%w: dingtalk app token endpoint", ErrSsoAuthFailed)
	}
	if token.AccessToken == `` {
		return ``, fmt.Errorf("%w: dingtalk app token is empty", ErrSsoAuthFailed)
	}
	return token.AccessToken, nil
}

// ssoDingtalkUseridByUnionId 把 unionId 换算成企业内通讯录 userid。
// 换算不出来即「这个人不是该企业成员」，直接拒绝登录，不退到 unionId 建号（那会和同步对不上）。
// 换算得出来也要看 contact_type：钉钉对「企业外部联系人」同样返回 userid，那类身份不是员工。
func ssoDingtalkUseridByUnionId(appToken string, unionId string) (string, error) {
	target := ssoDingtalkByUnionIdURL + `?access_token=` + url.QueryEscape(appToken)
	var resp ssoDingtalkTopApiResponse
	if err := ssoVendorCallJSON(http.MethodPost, target, nil, map[string]string{`unionid`: unionId}, &resp); err != nil {
		logs.Error("sso dingtalk convert unionid failed: %s", err.Error())
		return ``, fmt.Errorf("%w: dingtalk userid convert endpoint", ErrSsoAuthFailed)
	}
	if resp.ErrCode != 0 {
		logs.Error("sso dingtalk convert unionid rejected: errcode=%d, errmsg=%s", resp.ErrCode, resp.ErrMsg)
		return ``, fmt.Errorf("%w: dingtalk userid convert rejected", ErrSsoAuthFailed)
	}
	// 读不到或读到别的取值都当非员工。这道校验是「必须是本企业员工才能登录」的唯一落点，
	// 不能按放行兜底；出错时把 result 打进日志，缺字段与外部联系人才分得开。
	contactType := ssoStringField(resp.Result, `contact_type`)
	if contactType != ssoDingtalkContactTypeEmployee {
		logs.Error("sso dingtalk user is not an enterprise employee: contact_type=%q, result=%v",
			contactType, resp.Result)
		return ``, fmt.Errorf("%w: dingtalk user is not an enterprise employee", ErrSsoAuthFailed)
	}
	userid := ssoStringField(resp.Result, `userid`)
	if userid == `` {
		return ``, fmt.Errorf("%w: dingtalk unionid has no userid in this enterprise", ErrSsoAuthFailed)
	}
	return userid, nil
}

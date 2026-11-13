// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/zhimaAi/go_tools/logs"
)

// SsoProviderMeta 是某个平台的字段形态。平台清单与标签属于业务契约，由后端下发，
// 前端硬编码会与后端校验脱节（新增平台要改两处）。
type SsoProviderMeta struct {
	Key                 string `json:"key"`
	Label               string `json:"label"`
	AppKeyLabel         string `json:"app_key_label"`
	AppSecretLabel      string `json:"app_secret_label"`
	NeedAgentId         bool   `json:"need_agent_id"`
	NeedTrustedDomain   bool   `json:"need_trusted_domain"`
	NeedEndpoints       bool   `json:"need_endpoints"`
	NeedExternalIdField bool   `json:"need_external_id_field"`
	SupportScim         bool   `json:"support_scim"`
	// ScopeDefault 是该平台未填 scope 时运行时实际使用的兜底授权域（与各 sso_provider_*.go 的默认常量同源）。
	// 下发给前端预填，管理员不必去猜每个平台该填什么。
	ScopeDefault string `json:"scope_default"`
	// Permissions 是该平台必须在第三方后台开通的权限点，已按请求语言翻译。
	// 由 ssoProviderPermissions 的清单派生，前端渲染成表格。
	// 不能带 omitempty：无法枚举权限点的平台（OIDC / OAuth2.0）要回空数组而不是缺字段，
	// 缺字段会让前端读到 undefined（nil 切片配合无 omitempty 又会序列化成 null）。
	Permissions []SsoPermission `json:"permissions"`
	// PermissionScope 是权限表下方的数据范围说明（飞书「全部成员」/ 钉钉「全部员工」）。
	// 它是权限范围的配置项而不是权限点，混进表格会让一行备注同时讲两件事。
	PermissionScope string `json:"permission_scope"`
}

// SsoPermission 是一个厂商后台权限点，四个字段对应控制台的列。
// 控制台列口径并不一致：飞书是「权限名称 + scope + 权限类型」，钉钉是「权限信息 + 权限点code」，
// 因此钉钉行的 Type 留空，前端在该平台所有行都没有类型时收起这一列。
type SsoPermission struct {
	Name   string `json:"name"`
	Code   string `json:"code"`
	Type   string `json:"type"`
	Remark string `json:"remark"`
}

// ssoPermissionSpec 是 SsoPermission 的未翻译形态：code 是厂商的固定标识、不随语言变化，
// 其余三列走 i18n key。
type ssoPermissionSpec struct {
	nameKey   string
	code      string
	typeKey   string
	remarkKey string
}

// ssoProviderMetaTable 逐平台描述必填项。SupportScim=false 的平台走平台自带通讯录接口而不是 SCIM。
var ssoProviderMetaTable = map[string]SsoProviderMeta{
	define.SsoProviderOidc: {
		Key: define.SsoProviderOidc, Label: "sso_provider_oidc",
		AppKeyLabel: "sso_field_client_id", AppSecretLabel: "sso_field_client_secret",
		NeedTrustedDomain: true, SupportScim: true, ScopeDefault: ssoOidcDefaultScopes,
	},
	define.SsoProviderOauth2: {
		Key: define.SsoProviderOauth2, Label: "sso_provider_oauth2",
		AppKeyLabel: "sso_field_client_id", AppSecretLabel: "sso_field_client_secret",
		NeedTrustedDomain: true, NeedEndpoints: true, NeedExternalIdField: true, SupportScim: true,
		ScopeDefault: ssoOauth2DefaultScopes,
	},
	define.SsoProviderDingtalk: {
		Key: define.SsoProviderDingtalk, Label: "sso_provider_dingtalk",
		AppKeyLabel: "sso_field_client_id", AppSecretLabel: "sso_field_client_secret",
		PermissionScope: "sso_perm_dingtalk_scope_note", ScopeDefault: ssoDingtalkDefaultScopes,
	},
	// 企业微信本期不开放，条目一并注释；恢复时连同 define.SsoProviders 与 ssoProviders 一起加回。
	// define.SsoProviderWecom: {
	// 	Key: define.SsoProviderWecom, Label: "sso_provider_wecom",
	// 	AppKeyLabel: "sso_field_wecom_corp_id", AppSecretLabel: "sso_field_app_secret",
	// 	NeedAgentId: true,
	// },
	define.SsoProviderFeishu: {
		Key: define.SsoProviderFeishu, Label: "sso_provider_feishu",
		AppKeyLabel: "sso_field_feishu_app_id", AppSecretLabel: "sso_field_app_secret",
		PermissionScope: "sso_perm_feishu_scope_note",
	},
}

// ssoProviderPermissions 逐平台列出必须在第三方后台开通的权限点（按展示顺序）。
// 只覆盖走厂商通讯录的平台：OIDC / OAuth2.0 的权限点由各 IdP 自行定义，无法预先枚举。
//
// 名称与 code 必须逐字对齐厂商控制台：管理员是拿报错里的 code 回后台搜的（钉钉 errcode=88 报
// requiredScopes=[qyapi_get_member]，飞书报 scope 名），文案换成我们自己归纳的说法就对不上。
// 顺序按实际配置流程排：登录所需 → 通讯录同步所需 → 可选字段。
var ssoProviderPermissions = map[string][]ssoPermissionSpec{
	define.SsoProviderDingtalk: {
		{
			nameKey: `sso_perm_dingtalk_base_name`, code: `qyapi_base`,
			remarkKey: `sso_perm_dingtalk_base_remark`,
		},
		{
			nameKey: `sso_perm_dingtalk_profile_read_name`, code: `Contact.User.Read`,
			remarkKey: `sso_perm_dingtalk_profile_read_remark`,
		},
		{
			nameKey: `sso_perm_dingtalk_user_read_name`, code: `qyapi_get_member`,
			remarkKey: `sso_perm_dingtalk_user_read_remark`,
		},
		{
			nameKey: `sso_perm_dingtalk_member_read_name`, code: `qyapi_get_department_member`,
			remarkKey: `sso_perm_dingtalk_member_read_remark`,
		},
		{
			nameKey: `sso_perm_dingtalk_dept_read_name`, code: `qyapi_get_department_list`,
			remarkKey: `sso_perm_dingtalk_dept_read_remark`,
		},
		// 两条字段权限没有公开的权限点 code（官方只在接口文档里按中文名说明），Code 留空。
		{
			nameKey: `sso_perm_dingtalk_mobile_name`, remarkKey: `sso_perm_dingtalk_mobile_remark`,
		},
		{
			nameKey: `sso_perm_dingtalk_email_name`, remarkKey: `sso_perm_dingtalk_email_remark`,
		},
	},
	define.SsoProviderFeishu: {
		{
			nameKey: `sso_perm_feishu_contact_name`, code: `contact:contact.base:readonly`,
			typeKey: `sso_perm_type_app_identity`, remarkKey: `sso_perm_feishu_contact_remark`,
		},
		{
			nameKey: `sso_perm_feishu_dept_org_name`, code: `contact:department.organize:readonly`,
			typeKey: `sso_perm_type_app_identity`, remarkKey: `sso_perm_feishu_dept_org_remark`,
		},
		{
			nameKey: `sso_perm_feishu_user_base_name`, code: `contact:user.base:readonly`,
			typeKey: `sso_perm_type_app_identity`, remarkKey: `sso_perm_feishu_user_base_remark`,
		},
		{
			nameKey: `sso_perm_feishu_user_org_name`, code: `contact:user.department:readonly`,
			typeKey: `sso_perm_type_app_identity`, remarkKey: `sso_perm_feishu_user_org_remark`,
		},
		{
			nameKey: `sso_perm_feishu_user_mobile_name`, code: `contact:user.phone:readonly`,
			typeKey: `sso_perm_type_app_identity`, remarkKey: `sso_perm_feishu_user_mobile_remark`,
		},
	},
}

// SsoProviderMetas 按 define.SsoProviders 的顺序下发元数据，i18n key 按请求语言翻译后出参。
func SsoProviderMetas(lang string) []SsoProviderMeta {
	list := make([]SsoProviderMeta, 0, len(define.SsoProviders))
	for _, key := range define.SsoProviders {
		meta := ssoProviderMetaTable[key]
		meta.Label = lib_i18n.Show(lang, meta.Label)
		meta.AppKeyLabel = lib_i18n.Show(lang, meta.AppKeyLabel)
		meta.AppSecretLabel = lib_i18n.Show(lang, meta.AppSecretLabel)
		meta.PermissionScope = ssoPermText(lang, meta.PermissionScope)
		meta.Permissions = ssoProviderPermissionsOf(lang, key)
		list = append(list, meta)
	}
	return list
}

// ssoProviderPermissionsOf 把一个平台的权限点清单翻译成出参；没有清单的平台返回空切片。
// 必须返回非 nil 的空切片：Permissions 不带 omitempty，nil 会序列化成 null，
// 前端按数组渲染（`meta.permissions.length`）时同样会抛错。
func ssoProviderPermissionsOf(lang string, key string) []SsoPermission {
	specs := ssoProviderPermissions[key]
	permissions := make([]SsoPermission, 0, len(specs))
	for _, spec := range specs {
		permissions = append(permissions, SsoPermission{
			Name:   lib_i18n.Show(lang, spec.nameKey),
			Code:   spec.code,
			Type:   ssoPermText(lang, spec.typeKey),
			Remark: lib_i18n.Show(lang, spec.remarkKey),
		})
	}
	return permissions
}

// ssoPermText 翻译可选的权限列。钉钉控制台只有「权限信息 / 接口 / 权限点code」，没有
// 「权限类型」这一列，对应 key 为空；空 key 直接回空串，不去 Show() 一个不存在的键。
func ssoPermText(lang string, key string) string {
	if key == `` {
		return ``
	}
	return lib_i18n.Show(lang, key)
}

// SsoProviderSupports 判定平台标识是否已知，供入参校验与回调路由使用。
func SsoProviderSupports(provider string) bool {
	_, ok := ssoProviderMetaTable[provider]
	return ok
}

// SsoAuthzState 是从 Redis 消费出来的授权事务上下文，贯穿 authorize → callback → exchange。
//
// nonce 比对 OIDC id_token，code_verifier 是 PKCE 的一次性暗号，redirect_uri 必须与授权时逐字符一致；
// 三者只随 state 一次性消费，都不出现在业务代码里可改的位置。
type SsoAuthzState struct {
	// State 是本次授权的单次凭据，即 Redis 键后缀，也是回给 IdP 的 OAuth state 参数。
	// provider 必须把它原样带进授权地址，否则回调侧取不回这份事务上下文。
	State        string `json:"state"`
	AdminId      int    `json:"admin_id"`
	Provider     string `json:"provider"`
	Scene        string `json:"scene"`
	Nonce        string `json:"nonce"`
	CodeVerifier string `json:"code_verifier"`
	RedirectUri  string `json:"redirect_uri"`
	// UserId 仅 scene=bind 有值：本期没有发起绑定的入口，端点已就绪但不被触发。
	UserId int `json:"user_id"`
}

// SsoUserInfo 是平台换取到的身份信息。
// ExternalId 是身份匹配的唯一依据；其余字段只是资料，永不参与匹配。
type SsoUserInfo struct {
	ExternalId            string
	Username              string
	Email                 string
	Name                  string
	Phone                 string
	JobTitle              string
	EmployeeNo            string
	Avatar                string
	DepartmentExternalIds []string
}

// SsoProvider 描述一个企业 SSO 认证平台的授权与换取身份行为。
//
// 成员/部门拉取不在这里：它需要的凭证、分页与树形结构和授权完全不同，由 sso_sync.go 单独分发。
type SsoProvider interface {
	// Key 返回平台标识，与 sso_configs.provider 取值一致。
	Key() string
	// AuthorizeURL 拼授权地址。OIDC 的 discovery 是网络操作，因此必须能返回错误。
	AuthorizeURL(config SsoConfigModel, state SsoAuthzState) (string, error)
	// ExchangeCode 用授权码换取身份；verifier / 预期 nonce / redirect_uri 全部取自已消费的 state。
	ExchangeCode(config SsoConfigModel, code string, state SsoAuthzState) (SsoUserInfo, error)
	// TestConnect 预检「不需要用户授权码」就能验的部分，返回实际验到了哪一层。
	// 授权码换 token 那半段没有码可测，界面不得把它显示成「登录验证通过」。
	TestConnect(config SsoConfigModel) (string, error)
}

// 连接测试实际验到的层级：OIDC / OAuth2 只能验端点，三家厂商能验到凭证有效性。
const (
	SsoTestLevelEndpoints   = `endpoints`
	SsoTestLevelCredentials = `credentials`
)

// ssoProviders 是平台实现注册表，顺序与 define.SsoProviders 一致。
var ssoProviders = map[string]SsoProvider{
	define.SsoProviderOidc:     oidcProvider{},
	define.SsoProviderOauth2:   oauth2Provider{},
	define.SsoProviderDingtalk: dingtalkSsoProvider{},
	// 企业微信本期不开放，注册项一并注释；恢复时连同 define.SsoProviders 与 meta 表一起加回。
	// define.SsoProviderWecom:  wecomSsoProvider{},
	define.SsoProviderFeishu: feishuSsoProvider{},
}

// SsoProviderByKey 按平台标识取实现。
func SsoProviderByKey(key string) (SsoProvider, bool) {
	provider, ok := ssoProviders[key]
	return provider, ok
}

// ======================== 平台共用的授权码流程构件 ========================

// ErrSsoAuthFailed 表示授权码换取身份失败：凭证不对、码已用过/过期、或响应缺少身份字段。
// 对外只回这一类归一化原因（define.SsoErrorAuthFailed），不回显第三方响应内容。
var ErrSsoAuthFailed = errors.New("sso authorization failed")

// ssoCodeVerifierBytes 是 PKCE code_verifier 的随机字节数；RFC 7636 允许 43~128 字符，
// 32 字节经 base64url 恰好 43 字符，落在下限上。
const ssoCodeVerifierBytes = 32

// ssoScopeString 用平台默认 scope 兜底：管理员留空时不能发一个没有 scope 的授权请求，
// 否则 OIDC 侧不会返回 id_token。
func ssoScopeString(raw string, fallback string) string {
	if scopes := strings.TrimSpace(raw); scopes != `` {
		return scopes
	}
	return fallback
}

// NewSsoCodeVerifier 生成一次性 PKCE 暗号。不支持 PKCE 的平台用不上它，
// 但生成成本是一次随机读，因此统一生成、不做按平台分支。
func NewSsoCodeVerifier() (string, error) {
	buf := make([]byte, ssoCodeVerifierBytes)
	if _, err := rand.Read(buf); err != nil {
		logs.Error("generate sso code verifier failed: %s", err.Error())
		return ``, err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// ssoCodeChallenge 按 RFC 7636 的 S256 算法折算 code_challenge。
func ssoCodeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// SsoTokenResponse 是 authorization code 换取令牌的响应；三家厂商的错误码形态各不相同，
// 因此这里只放标准 OAuth2 字段，厂商差异由各自 provider 实现处理。
type SsoTokenResponse struct {
	AccessToken string `json:"access_token"`
	IdToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
	Error       string `json:"error"`
}

// SsoExchangeCodeForToken 用授权码换 access_token（OIDC 还含 id_token）。
//
// token 端点必须表单编码，凭据放表单体而不是 Basic 头；redirect_uri 与 code_verifier 都取自
// 本次授权事务，与授权时逐字符一致，否则标准 IdP 直接拒绝。target 是租户自配或由 discovery
// 派生的地址，两者都属外部输入，必须走受控客户端。
func SsoExchangeCodeForToken(target string, clientId string, clientSecret string, state SsoAuthzState, code string) (SsoTokenResponse, error) {
	form := url.Values{}
	form.Set(`grant_type`, `authorization_code`)
	form.Set(`code`, code)
	form.Set(`redirect_uri`, state.RedirectUri)
	form.Set(`client_id`, clientId)
	form.Set(`client_secret`, clientSecret)
	if state.CodeVerifier != `` {
		form.Set(`code_verifier`, state.CodeVerifier)
	}
	var token SsoTokenResponse
	if err := ssoCallForm(target, nil, form, &token); err != nil {
		return token, err
	}
	if token.AccessToken == `` && token.IdToken == `` {
		return token, fmt.Errorf("%w: token response has no access_token", ErrSsoAuthFailed)
	}
	return token, nil
}

// SsoFetchUserInfo 用 access_token 拉 userinfo。
// 返回通用 map 由调用方按平台取字段：OIDC 的 sub 比对与 OAuth2 的可配置标识都基于同一份响应。
func SsoFetchUserInfo(target string, accessToken string) (map[string]any, error) {
	var fields map[string]any
	headers := map[string]string{`Authorization`: `Bearer ` + accessToken}
	if err := ssoCallJSON(http.MethodGet, target, headers, nil, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

// ssoStringField 按白名单键读第三方响应里的字符串字段。
// 只认标量：部分 IdP 的 name/email 是嵌套对象或数组，cast 到字符串会 panic，
// 因此这里显式跳过非字符串/非数字的值，宁可读空也不回显整个响应体。
func ssoStringField(fields map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := fields[key].(type) {
		case string:
			if text := strings.TrimSpace(value); text != `` {
				return text
			}
		case float64:
			return strconv.FormatFloat(value, 'f', -1, 64)
		case bool:
			return strconv.FormatBool(value)
		}
	}
	return ``
}

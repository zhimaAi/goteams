// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package define

// ======================== 企业 SSO 常量 ========================

// SSO 平台标识。一个租户同一时刻只有一个平台生效，sso_configs.provider 是其唯一来源。
const (
	SsoProviderOidc     = `oidc`
	SsoProviderOauth2   = `oauth2`
	SsoProviderDingtalk = `dingtalk`
	// SsoProviderWecom 本期不开放。常量保留是因为企微实现（sso_provider_wecom.go）与
	// 企微同步代码仍在引用它；「不开放」由它不出现在 SsoProviders 清单里表达。
	SsoProviderWecom  = `wecom`
	SsoProviderFeishu = `feishu`
)

// SsoProviders 是平台清单的唯一真相源；provider_meta 与入参枚举都由它派生，前端不硬编码。
// 企业微信本期注释掉：实现代码都还在，只是不再进入清单，因此界面不展示、不可选、也过不了入参校验。
// 要重新开放，把它加回下面这一行即可（同时恢复 sso_provider.go 的 meta 与注册表两条目）。
var SsoProviders = []string{
	SsoProviderOidc, SsoProviderOauth2, SsoProviderDingtalk, SsoProviderFeishu,
	// SsoProviderWecom,
}

// SSO 授权事务的场景。bind 本期不实现，仅保留取值以免回调口径漂移。
const (
	SsoSceneLogin = `login`
	SsoSceneBind  = `bind`
)

// 成员同步的自动周期；none 表示该行不参与定时调度。
const (
	SsoSyncPeriodNone   = `none`
	SsoSyncPeriodHourly = `hourly`
	SsoSyncPeriodDaily  = `daily`
	SsoSyncPeriodWeekly = `weekly`
	// SsoSyncPeriodFiveMin 是测试环境专属的短周期，只为联调时不用等满一小时就能跑完一轮自动同步。
	// 它不进生产清单：线上按 5 分钟跑一次全量通讯录同步，既压 IdP 也压自己的库。
	SsoSyncPeriodFiveMin = `every_5min`
)

// SsoSyncPeriods 是生产环境 auto_sync_period 的合法取值。
var SsoSyncPeriods = []string{SsoSyncPeriodNone, SsoSyncPeriodHourly, SsoSyncPeriodDaily, SsoSyncPeriodWeekly}

// SsoSyncPeriodsDev 是测试环境（IsDev）的合法取值，比生产多一个每 5 分钟。
var SsoSyncPeriodsDev = []string{SsoSyncPeriodNone, SsoSyncPeriodHourly, SsoSyncPeriodFiveMin,
	SsoSyncPeriodDaily, SsoSyncPeriodWeekly}

// SsoSyncPeriodsForEnv 按当前环境返回合法周期清单，顺序即界面下拉的展示顺序。
// 入参校验与下拉选项都取它，避免出现「界面不给选、接口却能存」的错位。
func SsoSyncPeriodsForEnv() []string {
	if IsDev {
		return SsoSyncPeriodsDev
	}
	return SsoSyncPeriods
}

// SsoSyncTaskKey 是成员同步在 scheduled_tasks 里的任务类型。
const SsoSyncTaskKey = `sso_member_sync`

// SsoSyncLastErrorLimit 与 sso_sync_configs.last_error 的 VARCHAR(500) 对齐，写回前按此截断。
const SsoSyncLastErrorLimit = 500

// SsoCallbackKeyRunes 是随机回调标识的字符数：24 字节随机值经 base64.RawURLEncoding 恰好 32 字符，
// 且不含 '/'，不会破坏 /api/sso/callback/{provider}/{key} 的路径匹配。
const SsoCallbackKeyRunes = 32

// 成员同步失败结果的取值，落在 sso_sync_configs.last_sync_status。
const (
	SsoSyncStatusSuccess = `success`
	SsoSyncStatusPartial = `partial`
	SsoSyncStatusFailed  = `failed`
)

// SSO 回跳到登录页时 oauth_error 的取值。浏览器直接跳转，失败只能靠 query 传给前端，
// 前端按取值翻文案，因此这些字符串本身是对外契约，不随内部错误重命名。
const (
	SsoErrorDisabled        = `sso_disabled`
	SsoErrorStateInvalid    = `sso_state_invalid`
	SsoErrorAuthFailed      = `sso_auth_failed`
	SsoErrorEndpointGone    = `sso_endpoint_unreachable`
	SsoErrorAccountDisabled = `account_disabled`
	SsoErrorQuotaExceeded   = `member_quota_exceeded`
	SsoErrorServer          = `server_error`
)

// SsoErrorConfigInvalid 只用于连接预检接口的失败原因：必填字段未填齐或平台不支持。
// 它不进登录回跳的 oauth_error 契约，因此单列在此，避免和上面的对外集合混为一谈。
const SsoErrorConfigInvalid = `sso_config_invalid`

// 回跳 query 里 provider 参数的取值：SSO 登录复用 2398 的登录凭据链路，
// 用这个固定值把「企业 SSO」和「钉钉/飞书扫码」区分开，前端据此决定提示文案。
const SsoTicketProviderFlag = `sso`

// SSO 对外文案 key。
const (
	MsgSsoConfigInvalid   = `sso_config_invalid`
	MsgSsoEntryUrlMissing = `sso_entry_url_missing`
	MsgSsoCallbackInvalid = `sso_callback_invalid`
	MsgSsoAuthFailed      = `sso_auth_failed`
)

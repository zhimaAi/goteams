// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
)

// ======================== 企业 SSO 配置（租户级，权限 sso:setting） ========================

// ssoConfigResponse 是 GET/PUT /api/enterprise/sso 的对外形状。
// 登录密钥（AppSecret）与 SCIM Token 同口径：按产品要求明文回显供管理端查看（眼睛按钮），清空并保存即删除。
// trusted_domain 由 site_url 派生，不单独存列。
type ssoConfigResponse struct {
	Enabled          bool                       `json:"enabled"`
	Provider         string                     `json:"provider"`
	AppKey           string                     `json:"app_key"`
	AppSecret        string                     `json:"app_secret"`
	AgentId          string                     `json:"agent_id"`
	TrustedDomain    string                     `json:"trusted_domain"`
	IssuerUrl        string                     `json:"issuer_url"`
	AuthorizeUrl     string                     `json:"authorize_url"`
	TokenUrl         string                     `json:"token_url"`
	UserInfoUrl      string                     `json:"userinfo_url"`
	Scopes           string                     `json:"scopes"`
	ExternalIdField  string                     `json:"external_id_field"`
	LoginEntryUrl    string                     `json:"login_entry_url"`
	LoginCallbackUrl string                     `json:"login_callback_url"`
	BindCallbackUrl  string                     `json:"bind_callback_url"`
	Sync             ssoSyncResponse            `json:"sync"`
	ProviderMeta     []business.SsoProviderMeta `json:"provider_meta"`
}

type ssoSyncResponse struct {
	Enabled             bool            `json:"enabled"`
	AutoSyncPeriod      string          `json:"auto_sync_period"`
	WeeklyDay           int             `json:"weekly_day"`
	ScimBaseUrl         string          `json:"scim_base_url"`
	ScimToken           string          `json:"scim_token"`
	ScimTokenConfigured bool            `json:"scim_token_configured"`
	UserFieldMapping    json.RawMessage `json:"user_field_mapping"`
	DeptFieldMapping    json.RawMessage `json:"dept_field_mapping"`
	Running             bool            `json:"running"`
	LastSyncedAt        int             `json:"last_synced_at"`
	LastSyncedCount     int             `json:"last_synced_count"`
	LastSyncStatus      string          `json:"last_sync_status"`
	LastFailedCount     int             `json:"last_failed_count"`
	LastError           string          `json:"last_error"`
	// PeriodOptions 是当前环境下可选的自动周期（value + 已翻译文案），由后端按环境下发：
	// 测试环境比生产多一个每 5 分钟，前端不硬编码清单。
	PeriodOptions []business.SsoSyncPeriodMeta `json:"period_options"`
}

// ssoSyncRunResponse 是手动同步的出参。Done=false 只表示领不到租约（已有同步在跑）；
// 执行本身的失败仍回 done=true，原因落在 status/last_error 上——与定时同步共用同一份结果记录，
// 前端因此能直接把「本次同步了多少人」提示出来，不必再读一次配置。
type ssoSyncRunResponse struct {
	Done        bool   `json:"done"`
	Status      string `json:"status"`
	SyncedCount int    `json:"synced_count"`
	FailedCount int    `json:"failed_count"`
	LastError   string `json:"last_error"`
}

// saveSsoConfigRequest 是保存登录配置的入参。AppSecret 用指针实现三态契约：
// 缺省=保留、空串=清空、非空=覆盖；密钥已明文回显，管理端提交时通常整份带上，空串即管理员主动清空。
type saveSsoConfigRequest struct {
	Enabled         bool    `json:"enabled"`
	Provider        string  `json:"provider" binding:"required"`
	AppKey          string  `json:"app_key"`
	AppSecret       *string `json:"app_secret"`
	AgentId         string  `json:"agent_id"`
	IssuerUrl       string  `json:"issuer_url"`
	AuthorizeUrl    string  `json:"authorize_url"`
	TokenUrl        string  `json:"token_url"`
	UserInfoUrl     string  `json:"userinfo_url"`
	Scopes          string  `json:"scopes"`
	ExternalIdField string  `json:"external_id_field"`
}

// saveSsoSyncConfigRequest 是保存同步配置的入参，ScimToken 同样是三态指针。
type saveSsoSyncConfigRequest struct {
	Enabled          bool    `json:"enabled"`
	AutoSyncPeriod   string  `json:"auto_sync_period" binding:"required"`
	WeeklyDay        int     `json:"weekly_day"`
	ScimBaseUrl      string  `json:"scim_base_url"`
	ScimToken        *string `json:"scim_token"`
	UserFieldMapping string  `json:"user_field_mapping"`
	DeptFieldMapping string  `json:"dept_field_mapping"`
}

// GetSsoConfig 一次返回登录配置、同步配置、平台元数据与入口/回调地址。
// provider_preview 只影响回调地址里的平台段，用于下拉切换时预览未保存平台的地址，不写库。
func GetSsoConfig(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := c.GetInt(`adminId`)
	provider := c.Query(`provider_preview`)
	if provider != `` && !business.SsoProviderSupports(provider) {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `sso.config.get.preview`, errors.New(`invalid provider_preview`))
		return
	}
	if err := respondSsoConfig(c, lang, adminId, provider); err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `sso.config.get`, err)
	}
}

// SaveSsoConfig 保存登录配置；保存成功后回读，出参与 GET 完全同形。
func SaveSsoConfig(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := c.GetInt(`adminId`)
	var req saveSsoConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `sso.config.save.bind`, err)
		return
	}
	if err := validateSsoConfigRequest(req); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `sso.config.save.validate`, err)
		return
	}
	if _, err := business.SaveSsoConfig(business.SaveSsoConfigModel{
		AdminId: adminId, Enabled: req.Enabled, Provider: req.Provider, AppKey: req.AppKey,
		AppSecret: req.AppSecret, AgentId: req.AgentId, IssuerUrl: req.IssuerUrl,
		AuthorizeUrl: req.AuthorizeUrl, TokenUrl: req.TokenUrl, UserInfoUrl: req.UserInfoUrl,
		Scopes: req.Scopes, ExternalIdField: req.ExternalIdField,
	}); err != nil {
		respondSsoConfigError(c, lang, `sso.config.save`, err)
		return
	}
	if err := respondSsoConfig(c, lang, adminId, ``); err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `sso.config.save.readback`, err)
	}
}

// SaveSsoSyncConfig 保存同步配置，并把周期同步到定时任务表。
func SaveSsoSyncConfig(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := c.GetInt(`adminId`)
	var req saveSsoSyncConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `sso.sync.save.bind`, err)
		return
	}
	if err := validateSsoSyncRequest(req); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `sso.sync.save.validate`, err)
		return
	}
	// 同步开关已移除；旧客户端提交 enabled=false 也按开启保存。
	if _, err := business.SaveSsoSyncConfig(business.SaveSsoSyncConfigModel{
		AdminId: adminId, Enabled: true, AutoSyncPeriod: req.AutoSyncPeriod, WeeklyDay: req.WeeklyDay,
		ScimBaseUrl: req.ScimBaseUrl, ScimToken: req.ScimToken,
		UserFieldMapping: req.UserFieldMapping, DeptFieldMapping: req.DeptFieldMapping,
	}); err != nil {
		respondSsoConfigError(c, lang, `sso.sync.save`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// TestSsoConnection 用「表单里当前填的、尚未保存」的配置做一次连接预检。
// 复用保存请求的入参形状与三态密钥语义：密钥明文回显后管理端整份提交；省略 app_secret 时预检走已存的那把。
// 只验不需要授权码的部分，成功回 {ok:true, level}，失败回 {ok:false, reason}，reason 复用登录链路的对外归一码。
func TestSsoConnection(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := c.GetInt(`adminId`)
	var req saveSsoConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `sso.config.test.bind`, err)
		return
	}
	if err := validateSsoConfigRequest(req); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `sso.config.test.validate`, err)
		return
	}
	saved, err := business.GetSsoConfig(adminId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `sso.config.test.load`, err)
		return
	}
	merged := business.MergeSsoConfig(saved, business.SaveSsoConfigModel{
		AdminId: adminId, Enabled: req.Enabled, Provider: req.Provider, AppKey: req.AppKey,
		AppSecret: req.AppSecret, AgentId: req.AgentId, IssuerUrl: req.IssuerUrl,
		AuthorizeUrl: req.AuthorizeUrl, TokenUrl: req.TokenUrl, UserInfoUrl: req.UserInfoUrl,
		Scopes: req.Scopes, ExternalIdField: req.ExternalIdField,
	})
	level, testErr := business.SsoTest(merged)
	if testErr != nil {
		c.JSON(http.StatusOK, define.Response{
			Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
			Data: map[string]any{`ok`: false, `reason`: ssoTestReason(testErr)},
		})
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: map[string]any{`ok`: true, `level`: level},
	})
}

// RunSsoSync 手动触发一次成员同步：在请求内同步执行完再返回，并把本次执行结果一并带回。
// 领不到租约（已有同步在跑）回 done=false；执行本身失败不回错误码——失败原因落在 status/last_error 上，
// 与定时同步共用同一份结果记录。
func RunSsoSync(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := c.GetInt(`adminId`)
	err := business.TriggerSsoSync(adminId)
	switch {
	case err == nil:
		respondSsoSyncRun(c, lang, adminId)
	case errors.Is(err, business.ErrSchedulerTaskBusy):
		c.JSON(http.StatusOK, define.Response{
			Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
			Data: ssoSyncRunResponse{Done: false},
		})
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `sso.sync.run`, err)
	}
}

// respondSsoSyncRun 读回本次同步刚写进 sso_sync_configs 的结果。执行失败也走这里：
// 结果已经落库，接口只把它投影给前端，不另造一条错误通道。
func respondSsoSyncRun(c *gin.Context, lang string, adminId int) {
	syncConfig, err := business.GetSsoSyncConfig(adminId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `sso.sync.run`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: ssoSyncRunResponse{
			Done: true, Status: syncConfig.LastSyncStatus, SyncedCount: syncConfig.LastSyncedCount,
			FailedCount: syncConfig.LastFailedCount, LastError: syncConfig.LastError,
		},
	})
}

// ssoTestReason 把连接预检的错误映射成对外归一码：配置问题与网络/凭证问题分开，// 界面据此提示「先补全配置」还是「端点不可达/凭证无效」。
func ssoTestReason(err error) string {
	switch {
	case errors.Is(err, business.ErrSsoProviderInvalid), errors.Is(err, business.ErrSsoConfigIncomplete):
		return define.SsoErrorConfigInvalid
	default:
		return ssoErrorReason(err)
	}
}

// respondSsoConfig 读取当前有效配置并写出响应。previewProvider 非空时只替换回调地址里的平台段。
func respondSsoConfig(c *gin.Context, lang string, adminId int, previewProvider string) error {
	config, err := business.GetSsoConfig(adminId)
	if err != nil {
		return err
	}
	syncConfig, err := business.GetSsoSyncConfig(adminId)
	if err != nil {
		return err
	}
	running, err := business.IsSsoSyncRunning(adminId)
	if err != nil {
		return err
	}
	entryUrl, err := business.SsoLoginEntryUrl(adminId)
	if err != nil {
		return err
	}
	provider := config.Provider
	if previewProvider != `` {
		provider = previewProvider
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: ssoConfigResponse{
			Enabled: config.Enabled, Provider: config.Provider, AppKey: config.AppKey,
			AppSecret: config.AppSecret, AgentId: config.AgentId,
			TrustedDomain: business.SsoSiteUrl(), IssuerUrl: config.IssuerUrl,
			AuthorizeUrl: config.AuthorizeUrl, TokenUrl: config.TokenUrl, UserInfoUrl: config.UserInfoUrl,
			Scopes: config.Scopes, ExternalIdField: config.ExternalIdField, LoginEntryUrl: entryUrl,
			LoginCallbackUrl: business.SsoCallbackUrl(provider, config.CallbackKey, false),
			BindCallbackUrl:  business.SsoCallbackUrl(provider, config.CallbackKey, true),
			Sync:             toSsoSyncResponse(syncConfig, running, lang),
			ProviderMeta:     business.SsoProviderMetas(lang),
		},
	})
	return nil
}

func toSsoSyncResponse(model business.SsoSyncConfigModel, running bool, lang string) ssoSyncResponse {
	return ssoSyncResponse{
		Enabled: model.Enabled, AutoSyncPeriod: model.AutoSyncPeriod, WeeklyDay: model.WeeklyDay,
		ScimBaseUrl: model.ScimBaseUrl, ScimToken: model.ScimToken,
		ScimTokenConfigured: model.ScimToken != ``,
		UserFieldMapping:    ssoFieldMappingJson(model.UserFieldMapping),
		DeptFieldMapping:    ssoFieldMappingJson(model.DeptFieldMapping),
		Running:             running, LastSyncedAt: model.LastSyncedAt, LastSyncedCount: model.LastSyncedCount,
		LastSyncStatus: model.LastSyncStatus, LastFailedCount: model.LastFailedCount, LastError: model.LastError,
		PeriodOptions: business.SsoSyncPeriodMetas(lang),
	}
}

// ssoFieldMappingJson 把库里存的映射 JSON 原样出参；空或非法时回 {}。
// 兜底是必要的：整个响应会因一个 RawMessage 非法而序列化失败，不能让脏数据放大成接口 500。
func ssoFieldMappingJson(raw string) json.RawMessage {
	if json.Valid([]byte(raw)) {
		return json.RawMessage(raw)
	}
	return json.RawMessage(`{}`)
}

func validateSsoConfigRequest(req saveSsoConfigRequest) error {
	if err := validate.MaxLen(`app_key`, req.AppKey, 200); err != nil {
		return err
	}
	if req.AppSecret != nil {
		if err := validate.MaxLen(`app_secret`, *req.AppSecret, 500); err != nil {
			return err
		}
	}
	if err := validate.MaxLen(`agent_id`, req.AgentId, 100); err != nil {
		return err
	}
	if err := validate.MaxLen(`scopes`, req.Scopes, 255); err != nil {
		return err
	}
	if err := validate.MaxLen(`external_id_field`, req.ExternalIdField, 100); err != nil {
		return err
	}
	for _, item := range []struct{ field, value string }{
		{`issuer_url`, req.IssuerUrl}, {`authorize_url`, req.AuthorizeUrl},
		{`token_url`, req.TokenUrl}, {`userinfo_url`, req.UserInfoUrl},
	} {
		if err := validate.ValidateSsoUrl(item.field, item.value, 500); err != nil {
			return err
		}
	}
	return nil
}

func validateSsoSyncRequest(req saveSsoSyncConfigRequest) error {
	// 与 business.ValidateSsoSyncConfig 同源：周期清单按环境取，测试环境才认每 5 分钟。
	if err := validate.IsInList(`auto_sync_period`, req.AutoSyncPeriod, define.SsoSyncPeriodsForEnv()); err != nil {
		return err
	}
	if req.AutoSyncPeriod == define.SsoSyncPeriodWeekly && (req.WeeklyDay < 1 || req.WeeklyDay > 7) {
		return &validate.FieldError{Field: `weekly_day`, Key: define.MsgParamInvalid}
	}
	if err := validate.ValidateSsoUrl(`scim_base_url`, req.ScimBaseUrl, 500); err != nil {
		return err
	}
	if req.ScimToken != nil {
		if err := validate.MaxLen(`scim_token`, *req.ScimToken, 500); err != nil {
			return err
		}
	}
	if err := validate.ValidateSsoFieldMapping(`user_field_mapping`, req.UserFieldMapping); err != nil {
		return err
	}
	return validate.ValidateSsoFieldMapping(`dept_field_mapping`, req.DeptFieldMapping)
}

// respondSsoConfigError 把配置校验与写入错误映射成对外文案。
func respondSsoConfigError(c *gin.Context, lang string, stage string, err error) {
	switch {
	case errors.Is(err, business.ErrSsoProviderInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgSsoConfigInvalid), stage, err)
	case errors.Is(err, business.ErrSsoConfigIncomplete):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgSsoConfigInvalid), stage, err)
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), stage, err)
	}
}

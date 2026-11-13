// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_redis"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// 第三方扫码登录相关的业务错误，controller 据此映射对外状态码。
var (
	// ErrOauthProviderDisabled 表示该平台未开启或凭证配置不完整。
	ErrOauthProviderDisabled = errors.New(`oauth provider disabled`)
	// ErrOauthStateInvalid 表示 state 缺失、已过期或已被消费。
	ErrOauthStateInvalid = errors.New(`oauth state invalid`)
	// ErrOauthCodeInvalid 表示第三方授权码无效或换取用户身份失败。
	ErrOauthCodeInvalid = errors.New(`oauth authorization code invalid`)
	// ErrOauthTicketInvalid 表示票据缺失、已过期或已被消费。
	ErrOauthTicketInvalid = errors.New(`oauth ticket invalid`)
	// ErrOauthBoundByOther 表示该第三方账号已绑定到其他 GoTeams 账号。
	ErrOauthBoundByOther = errors.New(`oauth account bound by another user`)
	// ErrOauthAlreadyBound 表示当前 GoTeams 账号已绑定同平台的第三方账号。
	ErrOauthAlreadyBound = errors.New(`oauth provider already bound`)
	// ErrOauthNotBound 表示当前 GoTeams 账号未绑定该平台。
	ErrOauthNotBound = errors.New(`oauth provider not bound`)
)

// Redis 键前缀与默认有效期；有效期可被 [oauth] 分节覆盖。
const (
	oauthStateKeyPrefix       = `goteams:oauth:state:`
	oauthLoginTicketKeyPrefix = `goteams:oauth:login_ticket:`
	oauthBindTicketKeyPrefix  = `goteams:oauth:bind_ticket:`

	oauthDefaultStateTTL       = 600
	oauthDefaultBindTicketTTL  = 600
	oauthDefaultLoginTicketTTL = 120

	// oauthQrcodeTTL 是第三方二维码的有效期（秒），钉钉与飞书一致，不随环境调整。
	// 前端据此在二维码过期时盖蒙层并给刷新入口；取值口径见 docs/do/2398_钉钉飞书扫码登录.md。
	oauthQrcodeTTL = 300

	// oauthHTTPTimeout 是访问第三方开放平台的单次请求超时。
	oauthHTTPTimeout = 10 * time.Second
	// oauthNicknameLimit 与 oauthAvatarLimit 对应 user_oauth_bindings 的列长度上限。
	oauthNicknameLimit = 100
	oauthAvatarLimit   = 500
	// oauthErrorTextLimit 限制错误日志里记录的错误码与描述长度。
	oauthErrorTextLimit = 200
)

// OauthUserInfo 是第三方平台返回的用户身份信息。
type OauthUserInfo struct {
	// UnionId 是同一开发者账号下的用户唯一标识，扫码登录据此定位绑定关系。
	UnionId string
	// OpenId 是应用内的用户标识，仅作留存展示。
	OpenId string
	// Nickname 是第三方昵称，仅用于个人设置展示。
	Nickname string
	// Avatar 是第三方头像地址，仅用于个人设置展示。
	Avatar string
}

// OauthProvider 描述一个第三方扫码登录平台。
type OauthProvider interface {
	// Key 返回平台标识，取值与 user_oauth_bindings.provider 一致。
	Key() string
	// Enabled 表示该平台已开启且凭证配置完整。
	Enabled() bool
	// ClientId 返回前端初始化二维码 SDK 所需的应用标识（钉钉 AppKey / 飞书 App ID）。
	// 该值同时作为 user_oauth_bindings.app_key 落库，换应用后旧绑定自动失效。
	ClientId() string
	// SdkURL 返回官方二维码 SDK 的脚本地址，由前端动态加载。
	SdkURL() string
	// CodeQueryKey 返回回调地址上授权码的参数名（钉钉为 authCode，飞书为 code）。
	CodeQueryKey() string
	// AuthorizeURL 返回二维码指向的授权地址。
	AuthorizeURL(redirectUri string, state string) string
	// ExchangeCode 用授权码换取用户身份。
	ExchangeCode(code string, redirectUri string) (OauthUserInfo, error)
}

// OauthProviders 返回全部已注册平台（含未开启的），顺序固定，供界面按序展示。
func OauthProviders() []OauthProvider {
	return []OauthProvider{dingtalkProvider{}, feishuProvider{}}
}

// OauthProviderByKey 按平台标识取平台实现。
func OauthProviderByKey(key string) (OauthProvider, bool) {
	for _, provider := range OauthProviders() {
		if provider.Key() == key {
			return provider, true
		}
	}
	return nil, false
}

// EnabledOauthProviders 返回已开启的平台，供登录页决定展示哪些扫码入口。
func EnabledOauthProviders() []OauthProvider {
	enabled := make([]OauthProvider, 0, len(OauthProviders()))
	for _, provider := range OauthProviders() {
		if provider.Enabled() {
			enabled = append(enabled, provider)
		}
	}
	return enabled
}

// oauthConfigInt 读取 [oauth] 分节的整型配置，缺失或非法时回退默认值。
func oauthConfigInt(key string, fallback int) int {
	value := cast.ToInt(strings.TrimSpace(define.Config.Oauth[key]))
	if value <= 0 {
		return fallback
	}
	return value
}

// oauthProviderSwitchOn 判断某个 provider 分节的 enable 开关是否打开。
func oauthProviderSwitchOn(section map[string]string) bool {
	return strings.EqualFold(strings.TrimSpace(section[`enable`]), `true`)
}

// oauthAppKey 返回某平台当前配置的应用标识（钉钉 AppKey / 飞书 App ID），与 user_oauth_bindings.app_key 对齐。
// 绑定表的所有读写都按它过滤：换应用后旧行不再命中，等于自动失效。
func oauthAppKey(providerKey string) string {
	provider, ok := OauthProviderByKey(providerKey)
	if !ok {
		return ``
	}
	return provider.ClientId()
}

// oauthStateData 是二维码一次性凭据承载的上下文，回调时据此判断场景与归属用户。
type oauthStateData struct {
	Provider string `json:"provider"`
	Scene    string `json:"scene"`
	// UserId 仅在 scene 为 bind 时有值，表示发起绑定的已登录用户。
	UserId int `json:"user_id"`
}

// oauthLoginTicketData 是已绑定账号时的登录凭据，前端一次性换取登录态。
type oauthLoginTicketData struct {
	UserId int `json:"user_id"`
}

// oauthBindTicketData 是未绑定账号时的绑定凭据，携带第三方身份供绑定或注册时落库。
type oauthBindTicketData struct {
	Provider string `json:"provider"`
	UnionId  string `json:"union_id"`
	OpenId   string `json:"open_id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

// NewOauthState 生成随机 state 并写入 Redis，回调时一次性消费。
func NewOauthState(provider string, scene string, userId int) (string, error) {
	state, err := randomOauthToken(16)
	if err != nil {
		return ``, err
	}
	raw, err := json.Marshal(oauthStateData{Provider: provider, Scene: scene, UserId: userId})
	if err != nil {
		logs.Error("marshal oauth state failed: %s", err.Error())
		return ``, err
	}
	ttl := time.Duration(oauthConfigInt(`state_ttl`, oauthDefaultStateTTL)) * time.Second
	if err = define.Redis.Set(context.Background(), oauthStateKeyPrefix+state, raw, ttl).Err(); err != nil {
		logs.Error("save oauth state failed: %s", err.Error())
		return ``, err
	}
	return state, nil
}

// ConsumeOauthState 原子取出并删除 state；不存在、已过期或已被消费时返回 ErrOauthStateInvalid。
func ConsumeOauthState(state string) (oauthStateData, error) {
	if len(state) == 0 {
		return oauthStateData{}, ErrOauthStateInvalid
	}
	raw, exists, err := lib_redis.Take(define.Redis, oauthStateKeyPrefix+state)
	if err != nil {
		logs.Error("consume oauth state failed: %s", err.Error())
		return oauthStateData{}, err
	}
	if !exists {
		return oauthStateData{}, ErrOauthStateInvalid
	}
	var data oauthStateData
	if err = json.Unmarshal([]byte(raw), &data); err != nil {
		logs.Error("unmarshal oauth state failed: %s", err.Error())
		return oauthStateData{}, ErrOauthStateInvalid
	}
	return data, nil
}

// NewOauthLoginTicket 为已绑定账号的扫码结果签发一次性登录凭据。
func NewOauthLoginTicket(userId int) (string, error) {
	raw, err := json.Marshal(oauthLoginTicketData{UserId: userId})
	if err != nil {
		logs.Error("marshal oauth login ticket failed: %s", err.Error())
		return ``, err
	}
	ticket, err := randomOauthToken(24)
	if err != nil {
		return ``, err
	}
	ttl := time.Duration(oauthConfigInt(`login_ticket_ttl`, oauthDefaultLoginTicketTTL)) * time.Second
	if err = define.Redis.Set(context.Background(), oauthLoginTicketKeyPrefix+ticket, raw, ttl).Err(); err != nil {
		logs.Error("save oauth login ticket failed: %s", err.Error())
		return ``, err
	}
	return ticket, nil
}

// ConsumeOauthLoginTicket 原子取出并删除登录凭据，返回其对应的用户 ID。
func ConsumeOauthLoginTicket(ticket string) (int, error) {
	if len(ticket) == 0 {
		return 0, ErrOauthTicketInvalid
	}
	raw, exists, err := lib_redis.Take(define.Redis, oauthLoginTicketKeyPrefix+ticket)
	if err != nil {
		logs.Error("consume oauth login ticket failed: %s", err.Error())
		return 0, err
	}
	if !exists {
		return 0, ErrOauthTicketInvalid
	}
	var data oauthLoginTicketData
	if err = json.Unmarshal([]byte(raw), &data); err != nil || data.UserId <= 0 {
		logs.Error("unmarshal oauth login ticket failed: %s", err.Error())
		return 0, ErrOauthTicketInvalid
	}
	return data.UserId, nil
}

// NewOauthBindTicket 为未绑定账号的扫码结果签发一次性绑定凭据。
func NewOauthBindTicket(provider string, info OauthUserInfo) (string, error) {
	raw, err := json.Marshal(oauthBindTicketData{
		Provider: provider, UnionId: info.UnionId, OpenId: info.OpenId,
		Nickname: info.Nickname, Avatar: info.Avatar,
	})
	if err != nil {
		logs.Error("marshal oauth bind ticket failed: %s", err.Error())
		return ``, err
	}
	ticket, err := randomOauthToken(24)
	if err != nil {
		return ``, err
	}
	ttl := time.Duration(oauthConfigInt(`bind_ticket_ttl`, oauthDefaultBindTicketTTL)) * time.Second
	if err = define.Redis.Set(context.Background(), oauthBindTicketKeyPrefix+ticket, raw, ttl).Err(); err != nil {
		logs.Error("save oauth bind ticket failed: %s", err.Error())
		return ``, err
	}
	return ticket, nil
}

// ConsumeOauthBindTicket 原子取出并删除绑定凭据。
func ConsumeOauthBindTicket(ticket string) (oauthBindTicketData, error) {
	if len(ticket) == 0 {
		return oauthBindTicketData{}, ErrOauthTicketInvalid
	}
	raw, exists, err := lib_redis.Take(define.Redis, oauthBindTicketKeyPrefix+ticket)
	if err != nil {
		logs.Error("consume oauth bind ticket failed: %s", err.Error())
		return oauthBindTicketData{}, err
	}
	if !exists {
		return oauthBindTicketData{}, ErrOauthTicketInvalid
	}
	var data oauthBindTicketData
	if err = json.Unmarshal([]byte(raw), &data); err != nil || data.UnionId == `` {
		logs.Error("unmarshal oauth bind ticket failed: %s", err.Error())
		return oauthBindTicketData{}, ErrOauthTicketInvalid
	}
	return data, nil
}

// GetOauthBindingByUnionId 按 (provider, app_key, union_id) 查询绑定，未绑定时返回空 map。
// app_key 取当前配置；换过应用的旧行不再命中，等于自动失效。
func GetOauthBindingByUnionId(provider string, unionId string) (msql.Params, error) {
	m := msql.Model(define.TableUserOauthBindings, define.Postgres)
	row, err := m.Where2([][]string{
		{`provider`, provider},
		{`app_key`, oauthAppKey(provider)},
		{`union_id`, unionId},
	}).Find()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	return row, nil
}

// GetUserOauthBindings 查询某用户的全部绑定，key 为平台标识。
func GetUserOauthBindings(userId int) (map[string]msql.Params, error) {
	m := msql.Model(define.TableUserOauthBindings, define.Postgres)
	rows, err := m.Where(`user_id`, cast.ToString(userId)).Select()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	bindings := make(map[string]msql.Params, len(rows))
	for _, row := range rows {
		// 只认当前应用下的绑定；换过 app_key 的旧行自动失效，不参与展示与解绑。
		provider, ok := OauthProviderByKey(row[`provider`])
		if !ok || !strings.EqualFold(strings.TrimSpace(row[`app_key`]), provider.ClientId()) {
			continue
		}
		bindings[row[`provider`]] = row
	}
	return bindings, nil
}

// CreateOauthBinding 建立绑定关系（记录当前应用 app_key）。
// 已绑到其他账号返回 ErrOauthBoundByOther，当前账号已绑同平台的同应用返回 ErrOauthAlreadyBound。
func CreateOauthBinding(adminId int, userId int, provider string, info OauthUserInfo) error {
	bound, err := GetOauthBindingByUnionId(provider, info.UnionId)
	if err != nil {
		return err
	}
	if len(bound) > 0 {
		if cast.ToInt(bound[`user_id`]) != userId {
			return ErrOauthBoundByOther
		}
		return ErrOauthAlreadyBound
	}

	m := msql.Model(define.TableUserOauthBindings, define.Postgres)
	existing, err := m.Where2([][]string{
		{`user_id`, cast.ToString(userId)},
		{`provider`, provider},
		{`app_key`, oauthAppKey(provider)},
	}).Find()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return err
	}
	if len(existing) > 0 {
		return ErrOauthAlreadyBound
	}

	now := tool.Time2Int()
	_, err = m.Insert(msql.Datas{
		`admin_id`:   adminId,
		`user_id`:    userId,
		`provider`:   provider,
		`app_key`:    oauthAppKey(provider),
		`open_id`:    info.OpenId,
		`union_id`:   info.UnionId,
		`nickname`:   truncateOauthText(info.Nickname, oauthNicknameLimit),
		`avatar`:     truncateOauthText(info.Avatar, oauthAvatarLimit),
		`created_at`: now,
		`updated_at`: now,
	}, `id`)
	if err != nil {
		// 并发下唯一索引兜底：落到这里说明另一个请求刚建立了同样的绑定。
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == `23505` {
			return ErrOauthBoundByOther
		}
		logs.Error("create oauth binding failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return err
	}
	return nil
}

// DeleteOauthBinding 解除绑定，未绑定时返回 ErrOauthNotBound。
// 条件带上 admin_id + user_id + provider + app_key，避免过宽的删除范围；换过应用的旧行不会被当前配置删到。
func DeleteOauthBinding(adminId int, userId int, provider string) error {
	m := msql.Model(define.TableUserOauthBindings, define.Postgres)
	affected, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`user_id`, cast.ToString(userId)},
		{`provider`, provider},
		{`app_key`, oauthAppKey(provider)},
	}).Delete()
	if err != nil {
		logs.Error("delete oauth binding failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return err
	}
	if affected == 0 {
		return ErrOauthNotBound
	}
	return nil
}

// randomOauthToken 生成指定字节数的随机十六进制串，用于 state 与票据。
func randomOauthToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		logs.Error("generate oauth token failed: %s", err.Error())
		return ``, err
	}
	return hex.EncodeToString(buf), nil
}

// truncateOauthText 按字符截断第三方返回的展示文本，避免超出列长度导致写入失败。
func truncateOauthText(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}

// oauthErrorDetail 是第三方错误响应里可以安全记录的部分。
// 钉钉用 code + message，飞书用 code + msg；两家的 code 都可能是数字或字符串。
type oauthErrorDetail struct {
	Code    any    `json:"code"`
	Message string `json:"message"`
	Msg     string `json:"msg"`
}

// oauthErrorSummary 从错误响应体中提取错误码与描述，取不到时返回空串。
// 只按白名单读这三个字段：完整响应体可能回显请求参数（含 clientSecret），不落日志。
// 缺权限时钉钉会把缺失的权限点放在 accessdenieddetail.requiredScopes 里，因此 message
// 之外不再单独取值，靠描述文本足以定位。
func oauthErrorSummary(raw []byte) string {
	var detail oauthErrorDetail
	if err := json.Unmarshal(raw, &detail); err != nil {
		return ``
	}
	code := truncateOauthText(cast.ToString(detail.Code), oauthErrorTextLimit)
	message := detail.Message
	if message == `` {
		message = detail.Msg
	}
	message = truncateOauthText(message, oauthErrorTextLimit)
	if code == `` {
		return message
	}
	if message == `` {
		return code
	}
	return code + `: ` + message
}

// oauthHTTPClient 返回访问第三方开放平台用的客户端。
// 目标地址是固定的厂商域名，不由用户输入决定，因此不做出站地址校验。
func oauthHTTPClient() *http.Client {
	return &http.Client{Timeout: oauthHTTPTimeout}
}

// oauthCallJSON 发起一次 JSON 请求并把响应体解析到 out。
// 调用方只传固定的厂商地址，不拼接用户输入。
func oauthCallJSON(method string, requestURL string, headers map[string]string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf(`marshal oauth request: %w`, err)
		}
		reader = bytes.NewReader(raw)
	}

	request, err := http.NewRequest(method, requestURL, reader)
	if err != nil {
		return fmt.Errorf(`build oauth request: %w`, err)
	}
	if body != nil {
		request.Header.Set(`Content-Type`, `application/json; charset=utf-8`)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := oauthHTTPClient().Do(request)
	if err != nil {
		return fmt.Errorf(`call oauth endpoint: %w`, err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			logs.Error("close oauth response failed: %s", closeErr.Error())
		}
	}()

	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return fmt.Errorf(`read oauth response: %w`, err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		// 不落完整响应体（可能回显请求参数），只按白名单取厂商的错误码与描述。
		if detail := oauthErrorSummary(raw); detail != `` {
			return fmt.Errorf(`oauth endpoint returned status %d: %s`, response.StatusCode, detail)
		}
		return fmt.Errorf(`oauth endpoint returned status %d`, response.StatusCode)
	}
	if err = json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf(`unmarshal oauth response: %w`, err)
	}
	return nil
}

// oauthEscape 对 query 参数做 URL 编码。
func oauthEscape(value string) string {
	return url.QueryEscape(value)
}

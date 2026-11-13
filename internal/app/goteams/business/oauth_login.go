// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"errors"
	"strings"

	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// 前端回跳路径上用于表达回调结果的 query 参数名。
const (
	// OauthQueryLoginTicket 已绑定账号时的一次性登录凭据参数。
	OauthQueryLoginTicket = `oauth_ticket`
	// OauthQueryBindTicket 未绑定账号时的一次性绑定凭据参数。
	OauthQueryBindTicket = `oauth_bind_ticket`
	// OauthQueryError 回调失败时的错误原因参数，取值见 define.OauthErrorXxx。
	OauthQueryError = `oauth_error`
	// OauthQueryBindResult 个人设置绑定完成后的结果参数，取值为 success / conflict。
	OauthQueryBindResult = `oauth_bind`
	// OauthQueryProvider 回跳时带回的平台标识参数。
	OauthQueryProvider = `provider`
)

// OauthAuthorizeModel 描述一次扫码授权入口，供前端初始化二维码 SDK。
type OauthAuthorizeModel struct {
	// Provider 是平台标识。
	Provider string
	// Scene 是使用场景，login=登录页，bind=个人设置。
	Scene string
	// ClientId 是前端初始化 SDK 所需的应用标识。
	ClientId string
	// SdkURL 是官方二维码 SDK 的脚本地址。
	SdkURL string
	// AuthorizeURL 是二维码指向的授权地址。
	AuthorizeURL string
	// RedirectUri 是在第三方后台登记的回调地址，前端需把它原样传给 SDK。
	RedirectUri string
	// State 是本次授权的随机凭据，前端需把它原样传给 SDK。
	State string
	// ExpiresIn 是二维码的有效期（秒），前端据此在过期时展示刷新蒙层。
	ExpiresIn int
}

// OauthBindingModel 描述个人设置里的一个第三方账号绑定项。
type OauthBindingModel struct {
	// Provider 是平台标识。
	Provider string
	// Enabled 表示该平台是否已开启；未开启时前端只展示不可用状态。
	Enabled bool
	// Bound 表示当前用户是否已绑定该平台。
	Bound bool
	// Nickname 是绑定时取到的第三方昵称，未绑定时为空。
	Nickname string
	// BoundAt 是绑定时间（Unix 时间戳），未绑定时为 0。
	BoundAt int64
}

// OauthRedirectUri 返回在第三方后台登记的本站回调地址；站点地址未配置时返回空串。
func OauthRedirectUri(providerKey string) string {
	base := strings.TrimRight(strings.TrimSpace(define.Config.WebService[`site_url`]), `/`)
	if base == `` {
		return ``
	}
	return base + define.OauthCallbackPath(providerKey)
}

// BuildOauthAuthorize 生成扫码授权入口。
// scene 为 bind 时必须带已登录用户 ID，否则无法把绑定结果落到具体账号。
func BuildOauthAuthorize(providerKey string, scene string, userId int) (OauthAuthorizeModel, error) {
	provider, ok := OauthProviderByKey(providerKey)
	if !ok || !provider.Enabled() {
		return OauthAuthorizeModel{}, ErrOauthProviderDisabled
	}
	if scene == define.OauthSceneBind && userId <= 0 {
		return OauthAuthorizeModel{}, ErrOauthStateInvalid
	}

	redirectUri := OauthRedirectUri(providerKey)
	if redirectUri == `` {
		logs.Error("build oauth authorize failed: webservice site_url is not configured")
		return OauthAuthorizeModel{}, ErrOauthProviderDisabled
	}

	state, err := NewOauthState(providerKey, scene, userId)
	if err != nil {
		return OauthAuthorizeModel{}, err
	}
	return OauthAuthorizeModel{
		Provider:     providerKey,
		Scene:        scene,
		ClientId:     provider.ClientId(),
		SdkURL:       provider.SdkURL(),
		AuthorizeURL: provider.AuthorizeURL(redirectUri, state),
		RedirectUri:  redirectUri,
		State:        state,
		ExpiresIn:    oauthQrcodeTTL,
	}, nil
}

// OauthCallbackResult 描述回调处理结果。
type OauthCallbackResult struct {
	// Scene 是本次回调的场景，用于决定失败时回跳哪个页面；state 校验失败时为空串，按登录场景处理。
	Scene string
	// Path 是成功时前端回跳路径（相对站点根，含 query）。
	Path string
}

// HandleOauthCallback 处理第三方回调：校验 state、换取身份，并返回前端回跳路径。
// 返回的路径是相对站点根的路径（含 query），由 controller 拼成绝对地址后 302。
// 已绑定账号回跳登录页并带上登录凭据；未绑定回跳登录页并带上绑定凭据；个人设置绑定回跳个人设置页。
func HandleOauthCallback(providerKey string, code string, state string) (OauthCallbackResult, error) {
	provider, ok := OauthProviderByKey(providerKey)
	if !ok || !provider.Enabled() {
		return OauthCallbackResult{}, ErrOauthProviderDisabled
	}

	stateData, err := ConsumeOauthState(state)
	if err != nil {
		return OauthCallbackResult{}, err
	}
	result := OauthCallbackResult{Scene: stateData.Scene}
	// state 由本站签发，必须与回调的平台一致，避免拿 A 平台的 state 走 B 平台的回调。
	if stateData.Provider != providerKey {
		return result, ErrOauthStateInvalid
	}

	info, err := provider.ExchangeCode(code, OauthRedirectUri(providerKey))
	if err != nil {
		return result, err
	}
	if stateData.Scene == define.OauthSceneBind {
		result.Path, err = handleOauthBindCallback(providerKey, info, stateData.UserId)
		return result, err
	}
	result.Path, err = handleOauthLoginCallback(providerKey, info)
	return result, err
}

// handleOauthLoginCallback 处理登录页的扫码结果。
// 回跳路径带回 provider，前端据此在绑定弹窗里显示正确的应用名。
func handleOauthLoginCallback(providerKey string, info OauthUserInfo) (string, error) {
	bound, err := GetOauthBindingByUnionId(providerKey, info.UnionId)
	if err != nil {
		return ``, err
	}
	if len(bound) == 0 {
		ticket, ticketErr := NewOauthBindTicket(providerKey, info)
		if ticketErr != nil {
			return ``, ticketErr
		}
		return `/login?` + OauthQueryBindTicket + `=` + ticket + `&` + OauthQueryProvider + `=` + providerKey, nil
	}

	ticket, err := NewOauthLoginTicket(cast.ToInt(bound[`user_id`]))
	if err != nil {
		return ``, err
	}
	return `/login?` + OauthQueryLoginTicket + `=` + ticket + `&` + OauthQueryProvider + `=` + providerKey, nil
}

// handleOauthBindCallback 处理个人设置里的绑定结果。
func handleOauthBindCallback(providerKey string, info OauthUserInfo, userId int) (string, error) {
	adminId, err := oauthUserAdminId(userId)
	if err != nil {
		return ``, err
	}
	if err = CreateOauthBinding(adminId, userId, providerKey, info); err != nil {
		if errors.Is(err, ErrOauthBoundByOther) || errors.Is(err, ErrOauthAlreadyBound) {
			return `/profile?` + OauthQueryBindResult + `=conflict&` + OauthQueryProvider + `=` + providerKey, nil
		}
		return ``, err
	}
	return `/profile?` + OauthQueryBindResult + `=success&` + OauthQueryProvider + `=` + providerKey, nil
}

// ExchangeOauthLoginTicket 用登录凭据换取登录态，返回与密码登录一致的模型。
func ExchangeOauthLoginTicket(ticket string, ip string, ua string) (LoginModel, error) {
	userId, err := ConsumeOauthLoginTicket(ticket)
	if err != nil {
		return LoginModel{}, err
	}
	return oauthLoginByUserId(userId, ip, ua)
}

// BindOauthAccount 在未绑定场景下用账号密码绑定已有账号，并直接登录。
// 绑定失败时撤销刚创建的会话，避免"没绑定成功却已登录"。
func BindOauthAccount(ticket string, username string, password string, ip string, ua string) (LoginModel, error) {
	ticketData, err := ConsumeOauthBindTicket(ticket)
	if err != nil {
		return LoginModel{}, err
	}

	login, err := Login(LoginModel{
		Username: validate.NormalizeEmail(username), Password: password, Ip: ip, Ua: ua,
	})
	if err != nil {
		return LoginModel{}, err
	}

	if err = CreateOauthBinding(login.User.AdminId, login.User.Id, ticketData.Provider, oauthInfoFromBindTicket(ticketData)); err != nil {
		common.DeleteSession(login.Token)
		return LoginModel{}, err
	}
	return login, nil
}

// RegisterAndBindOauthAccount 在未绑定场景下注册新租户，并绑定当前第三方账号。
// 注册成功后绑定失败不会回滚租户（用户已可凭密码登录），但会返回错误让前端提示重新绑定。
func RegisterAndBindOauthAccount(ticket string, name string, email string, verifyCode string, password string, lang string) (RegisterTenantModel, error) {
	ticketData, err := ConsumeOauthBindTicket(ticket)
	if err != nil {
		return RegisterTenantModel{}, err
	}

	email = validate.NormalizeEmail(email)
	if err = VerifyRegisterCode(email, verifyCode); err != nil {
		return RegisterTenantModel{}, err
	}

	model, err := RegisterTenant(RegisterTenantModel{
		Username: email, Password: password, Name: name, Lang: lang,
	})
	if err != nil {
		return RegisterTenantModel{}, err
	}

	if err = CreateOauthBinding(model.AdminId, model.Id, ticketData.Provider, oauthInfoFromBindTicket(ticketData)); err != nil {
		logs.Error("bind oauth account after register failed: user_id=%d, provider=%s", model.Id, ticketData.Provider)
		return RegisterTenantModel{}, err
	}
	return model, nil
}

// UnbindOauthAccount 解除当前账号在某平台的绑定。
func UnbindOauthAccount(adminId int, userId int, providerKey string) error {
	if _, ok := OauthProviderByKey(providerKey); !ok {
		return ErrOauthProviderDisabled
	}
	return DeleteOauthBinding(adminId, userId, providerKey)
}

// GetUserOauthBindingList 返回个人设置展示用的绑定列表，顺序与平台注册顺序一致。
func GetUserOauthBindingList(userId int) ([]OauthBindingModel, error) {
	bindings, err := GetUserOauthBindings(userId)
	if err != nil {
		return nil, err
	}

	list := make([]OauthBindingModel, 0, len(OauthProviders()))
	for _, provider := range OauthProviders() {
		item := OauthBindingModel{Provider: provider.Key(), Enabled: provider.Enabled()}
		if row, ok := bindings[provider.Key()]; ok {
			item.Bound = true
			item.Nickname = row[`nickname`]
			item.BoundAt = cast.ToInt64(row[`created_at`])
		}
		list = append(list, item)
	}
	return list, nil
}

// oauthInfoFromBindTicket 把绑定凭据里的第三方身份还原成待落库的用户信息。
func oauthInfoFromBindTicket(ticket oauthBindTicketData) OauthUserInfo {
	return OauthUserInfo{
		UnionId: ticket.UnionId, OpenId: ticket.OpenId,
		Nickname: ticket.Nickname, Avatar: ticket.Avatar,
	}
}

// oauthLoginByUserId 为已通过第三方校验的用户创建会话，返回与密码登录一致的模型。
func oauthLoginByUserId(userId int, ip string, ua string) (LoginModel, error) {
	m := msql.Model(define.TableUsers, define.Postgres)
	row, err := m.Field(userTokenFields).Where2([][]string{
		{`id`, cast.ToString(userId)},
		{`deleted_at`, `0`},
	}).Find()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return LoginModel{}, err
	}
	if len(row) == 0 {
		return LoginModel{}, ErrUserNotExist
	}
	if cast.ToInt(row[`status`]) == define.MemberStatusDisabled {
		return LoginModel{}, ErrAccountDisabled
	}

	user, err := toUserModel(row)
	if err != nil {
		return LoginModel{}, err
	}
	token, err := common.CreateSession(common.SessionData{
		UserId: user.Id, AdminId: user.AdminId, Ip: ip, Ua: ua,
	})
	if err != nil {
		return LoginModel{}, err
	}
	return LoginModel{User: user, Token: token}, nil
}

// oauthUserAdminId 查询用户的租户 ID；用户不存在或已删除返回 ErrUserNotExist，已禁用返回 ErrAccountDisabled。
func oauthUserAdminId(userId int) (int, error) {
	if userId <= 0 {
		return 0, ErrUserNotExist
	}
	m := msql.Model(define.TableUsers, define.Postgres)
	row, err := m.Field(`admin_id,status`).Where2([][]string{
		{`id`, cast.ToString(userId)},
		{`deleted_at`, `0`},
	}).Find()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return 0, err
	}
	if len(row) == 0 {
		return 0, ErrUserNotExist
	}
	if cast.ToInt(row[`status`]) == define.MemberStatusDisabled {
		return 0, ErrAccountDisabled
	}
	return cast.ToInt(row[`admin_id`]), nil
}

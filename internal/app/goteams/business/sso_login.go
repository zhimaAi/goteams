// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
	"goteams/internal/pkg/lib_redis"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// SSO 登录链路的业务错误。controller 据此映射回跳 query 里的 oauth_error 取值，
// 因此这里区分「对用户可解释」的几类，其余都归到服务端错误。
var (
	// ErrSsoDisabled 表示入口不可用：scope_id 无效、租户未开启、或配置不完整。
	// 三者共用一个对外原因，免得匿名请求靠它探测哪家企业开了 SSO。
	ErrSsoDisabled = errors.New("sso is disabled or not configured")
	// ErrSsoStateInvalid 表示 state 缺失、已过期、已被消费，或与回调地址不属于同一租户。
	ErrSsoStateInvalid = errors.New("sso state invalid")
)

// Redis 键前缀与有效期。state 独立于 2398 扫码登录的前缀，两套链路互不消费对方的凭据。
const (
	ssoStateKeyPrefix  = `goteams:sso:state:`
	ssoDefaultStateTTL = 600
	// ssoStateTokenBytes 是 state 的随机字节数；hex 出 32 个字符，够抗穷举。
	ssoStateTokenBytes = 16
	// ssoNonceBytes 是 OIDC nonce 的随机字节数。
	ssoNonceBytes = 16
)

// JIT 建号参数。username 由大写字母 + 数字组成、去掉易混字符，落库前会被转小写。
const (
	ssoJitUsernameLen     = 16
	ssoJitUsernameRetries = 3
	ssoJitCharset         = `ABCDEFGHJKLMNPQRSTUVWXYZ23456789`
	// ssoJitPasswordBytes 只影响 bcrypt 输入长度，账号本身不走密码登录。
	ssoJitPasswordBytes = 32
)

// 第三方资料写入前的按列截断上限，与 user_sso_bindings / users 的列长度一致。
const (
	ssoExternalIdLimit    = 200
	ssoFieldUsernameLimit = 255
	ssoFieldEmailLimit    = 255
	ssoFieldJobTitleLimit = 100
	ssoFieldEmpNoLimit    = 100
	ssoFieldNameLimit     = 100
	ssoFieldPhoneLimit    = 20
	ssoFieldAvatarLimit   = 500
)

// SsoEntryModel 是登录页决定「是否渲染企业登录入口」所需的最小信息。
// 不返回任何凭证配置字段：这是一个匿名可读的公开接口。
type SsoEntryModel struct {
	Enabled       bool   `json:"enabled"`
	Provider      string `json:"provider"`
	ProviderLabel string `json:"provider_label"`
}

// SsoEntry 判定企业登录入口是否展示。
//
// fail-soft：解密失败、租户不存在、未开启、配置不完整、甚至数据库读失败，一律回 enabled=false。
// 入口只影响登录页多一个按钮，把它做成错误没有意义，反而会把正常登录也拖挂。
func SsoEntry(adminId int, lang string) SsoEntryModel {
	if adminId <= 0 {
		return SsoEntryModel{}
	}
	config, exists, err := ssoLoadConfig(adminId)
	if err != nil || !exists || !config.Enabled {
		return SsoEntryModel{}
	}
	if err = ValidateSsoConfig(config); err != nil {
		return SsoEntryModel{}
	}
	meta, ok := ssoProviderMetaTable[config.Provider]
	if !ok {
		return SsoEntryModel{}
	}
	return SsoEntryModel{
		Enabled: true, Provider: config.Provider,
		ProviderLabel: lib_i18n.Show(lang, meta.Label),
	}
}

// SsoAuthorizeUrl 校验入口并生成跳转到 IdP 的授权地址。
//
// scope_id 解密失败与「该租户没开 SSO」都回 ErrSsoDisabled：调用方据此 302 回登录页报错，
// 不必区分——两者对访问者的表现本来就该一样。
func SsoAuthorizeUrl(scopeId string) (string, error) {
	adminId, err := DecodeSsoScopeId(scopeId)
	if err != nil {
		return ``, ErrSsoDisabled
	}
	config, exists, err := ssoLoadConfig(adminId)
	if err != nil {
		return ``, err
	}
	if !exists || !config.Enabled {
		return ``, ErrSsoDisabled
	}
	if err = ValidateSsoConfig(config); err != nil {
		logs.Error("sso authorize rejected: config incomplete: admin_id=%d, err=%s", adminId, err.Error())
		return ``, ErrSsoDisabled
	}
	provider, ok := SsoProviderByKey(config.Provider)
	if !ok {
		return ``, ErrSsoProviderInvalid
	}
	state, err := NewSsoState(config, define.SsoSceneLogin, 0)
	if err != nil {
		return ``, err
	}
	return provider.AuthorizeURL(config, state)
}

// NewSsoState 生成一次授权事务的上下文并落到 Redis。
//
// nonce / code_verifier / redirect_uri 只存在这里，随 state 一次性消费，不出现在 URL 上；
// redirect_uri 必须与授权时逐字符一致，所以不能等回调时再拼一遍。
func NewSsoState(config SsoConfigModel, scene string, userId int) (SsoAuthzState, error) {
	redirectUri := SsoCallbackUrl(config.Provider, config.CallbackKey, scene == define.SsoSceneBind)
	if redirectUri == `` {
		logs.Error("build sso state failed: webservice site_url is not configured")
		return SsoAuthzState{}, ErrSsoDisabled
	}
	stateToken, err := randomOauthToken(ssoStateTokenBytes)
	if err != nil {
		return SsoAuthzState{}, err
	}
	nonce, err := randomOauthToken(ssoNonceBytes)
	if err != nil {
		return SsoAuthzState{}, err
	}
	state := SsoAuthzState{
		State: stateToken, AdminId: config.AdminId, Provider: config.Provider,
		Scene: scene, Nonce: nonce, RedirectUri: redirectUri, UserId: userId,
	}
	// 不支持 PKCE 的平台（三家厂商）用不上 verifier，但生成成本只是一次随机读，
	// 统一生成比按平台分支更少出错。
	if config.Provider == define.SsoProviderOidc || config.Provider == define.SsoProviderOauth2 {
		verifier, verifierErr := NewSsoCodeVerifier()
		if verifierErr != nil {
			return SsoAuthzState{}, verifierErr
		}
		state.CodeVerifier = verifier
	}

	raw, err := json.Marshal(state)
	if err != nil {
		logs.Error("marshal sso state failed: %s", err.Error())
		return SsoAuthzState{}, err
	}
	ttl := time.Duration(ssoConfigInt(`state_ttl`, ssoDefaultStateTTL)) * time.Second
	if err = define.Redis.Set(context.Background(), ssoStateKeyPrefix+stateToken, raw, ttl).Err(); err != nil {
		logs.Error("save sso state failed: %s", err.Error())
		return SsoAuthzState{}, err
	}
	return state, nil
}

// ConsumeSsoState 原子取出并删除 state。重复回调（用户刷新 IdP 回跳页）第二次就取不到，
// 这是「一次授权只能换一个身份」的保证。
func ConsumeSsoState(state string) (SsoAuthzState, error) {
	if state == `` {
		return SsoAuthzState{}, ErrSsoStateInvalid
	}
	raw, exists, err := lib_redis.Take(define.Redis, ssoStateKeyPrefix+state)
	if err != nil {
		logs.Error("consume sso state failed: %s", err.Error())
		return SsoAuthzState{}, err
	}
	if !exists {
		return SsoAuthzState{}, ErrSsoStateInvalid
	}
	var data SsoAuthzState
	if err = json.Unmarshal([]byte(raw), &data); err != nil || data.AdminId <= 0 {
		logs.Error("unmarshal sso state failed: err=%v", err)
		return SsoAuthzState{}, ErrSsoStateInvalid
	}
	return data, nil
}

// SsoCallbackResult 描述回调处理结果，供 controller 拼 302 目标。
type SsoCallbackResult struct {
	AdminId int
	// Scene 是本次回调的场景；state 校验失败时为空串，按登录场景回跳。
	Scene string
	// Path 是成功时前端回跳路径（相对站点根，含 query）。
	Path string
}

// HandleSsoCallback 处理 IdP 回调：定位租户、交叉校验 state、换取身份、匹配或 JIT，返回回跳路径。
//
// 只记 provider 与原因，不记 query——回调 query 里带着授权码。
// bind 参数对应 /bind 端点：本期没有发起绑定的入口，端点真实可用但不被触发。
func HandleSsoCallback(providerKey string, callbackKey string, code string, stateToken string, bind bool) (SsoCallbackResult, error) {
	config, exists, err := GetSsoConfigByCallbackKey(callbackKey)
	if err != nil {
		return SsoCallbackResult{}, err
	}
	if !exists {
		return SsoCallbackResult{}, ErrSsoDisabled
	}
	state, err := ConsumeSsoState(stateToken)
	result := SsoCallbackResult{Scene: state.Scene, AdminId: config.AdminId}
	if err != nil {
		return result, err
	}
	// 租户锚点是路径里的 callback_key，state 里的 admin_id 只是第二道交叉校验。
	// 两者不一致就是「A 租户的 state 配到 B 租户的地址」，一律拒绝。
	if state.AdminId != config.AdminId || state.Provider != providerKey {
		return result, ErrSsoStateInvalid
	}
	if (state.Scene == define.SsoSceneBind) != bind {
		return result, ErrSsoStateInvalid
	}
	// 在途授权不续命：管理员在用户扫码期间关掉 SSO，这里必须拒掉。
	if !config.Enabled {
		return result, ErrSsoDisabled
	}
	if err = ValidateSsoConfig(config); err != nil {
		return result, ErrSsoDisabled
	}
	provider, ok := SsoProviderByKey(providerKey)
	if !ok {
		return result, ErrSsoProviderInvalid
	}
	info, err := provider.ExchangeCode(config, code, state)
	if err != nil {
		logs.Error("sso callback exchange failed: provider=%s, admin_id=%d, err=%s", providerKey, config.AdminId, err.Error())
		return result, err
	}
	info = ssoTrimUserInfo(info)
	if info.ExternalId == `` {
		return result, fmt.Errorf("%w: empty external id", ErrSsoAuthFailed)
	}

	if bind {
		result.Path, err = handleSsoBindCallback(config, info, state.UserId)
		return result, err
	}
	result.Path, err = handleSsoLoginCallback(config, info)
	return result, err
}

// handleSsoLoginCallback 已绑定直接登录；未绑定 JIT 建号后登录。两条都签发一次性登录凭据，
// 会话由前端拿凭据走既有的 exchange 接口创建——SSO 不另开一条建会话的路。
func handleSsoLoginCallback(config SsoConfigModel, info SsoUserInfo) (string, error) {
	userId, err := SsoResolveUser(config, info)
	if err != nil {
		return ``, err
	}
	ticket, err := NewOauthLoginTicket(userId)
	if err != nil {
		return ``, err
	}
	return `/login?` + OauthQueryLoginTicket + `=` + ticket + `&` + OauthQueryProvider + `=` + define.SsoTicketProviderFlag, nil
}

// handleSsoBindCallback 把 IdP 身份绑到发起绑定的登录用户上。
func handleSsoBindCallback(config SsoConfigModel, info SsoUserInfo, userId int) (string, error) {
	result := `&` + OauthQueryProvider + `=` + define.SsoTicketProviderFlag
	if userId <= 0 {
		return `/profile?` + OauthQueryBindResult + `=conflict` + result, nil
	}
	adminId, err := oauthUserAdminId(userId)
	if err != nil {
		return ``, err
	}
	if err = CreateSsoBinding(adminId, userId, config, info); err != nil {
		if errors.Is(err, ErrOauthBoundByOther) || errors.Is(err, ErrOauthAlreadyBound) {
			return `/profile?` + OauthQueryBindResult + `=conflict` + result, nil
		}
		return ``, err
	}
	return `/profile?` + OauthQueryBindResult + `=success` + result, nil
}

// SsoResolveUser 按「绑定表 external_id → JIT」定位成员，返回可登录的 user_id。
//
// 刻意不按邮箱等资料认领既有账号：邮箱由 IdP 侧维护、可被改写，拿它匹配等于留一条账号接管路径。
// 代价是成员在首次 SSO 登录前已有本地账号时，会 JIT 出第二个账号，而不是自动认领旧账号。
func SsoResolveUser(config SsoConfigModel, info SsoUserInfo) (int, error) {
	userId, found, err := GetSsoBindingUserId(config.AdminId, config.Provider, config.AppKey, info.ExternalId)
	if err != nil {
		return 0, err
	}
	if found {
		// 命中绑定后必须复查成员状态：绑定行在成员软删除后仍保留，
		// 所以「已删除成员再次登录」只会在这里被拒，不会重新建号。
		return userId, assertSsoUserActive(config.AdminId, userId)
	}
	return ssoJitProvision(config, info)
}

// assertSsoUserActive 复查成员仍存在且未被禁用。
func assertSsoUserActive(adminId int, userId int) error {
	m := msql.Model(define.TableUsers, define.Postgres)
	row, err := m.Field(`id,status`).Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(userId)},
		{`deleted_at`, `0`},
	}).Find()
	if err != nil {
		logs.Error("check sso member failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return err
	}
	if len(row) == 0 {
		return ErrUserNotExist
	}
	if cast.ToInt(row[`status`]) == define.MemberStatusDisabled {
		return ErrAccountDisabled
	}
	return nil
}

// GetSsoBindingUserId 按租户 + 平台 + 应用 + 外部标识查绑定成员。
func GetSsoBindingUserId(adminId int, provider string, appKey string, externalId string) (int, bool, error) {
	m := msql.Model(define.TableUserSsoBindings, define.Postgres)
	value, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`provider`, provider},
		{`app_key`, appKey},
		{`external_id`, externalId},
	}).Value(`user_id`)
	if err != nil {
		logs.Error("get sso binding failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return 0, false, err
	}
	userId := cast.ToInt(value)
	if userId <= 0 {
		return 0, false, nil
	}
	return userId, true, nil
}

// CreateSsoBinding 为已登录用户建立绑定（个人设置绑定入口用）。
// 同一成员在同一应用下只允许一条绑定，撞唯一索引时按「已绑定/已被他人绑定」回业务错误。
func CreateSsoBinding(adminId int, userId int, config SsoConfigModel, info SsoUserInfo) error {
	existing, found, err := GetSsoBindingUserId(adminId, config.Provider, config.AppKey, info.ExternalId)
	if err != nil {
		return err
	}
	if found {
		if existing == userId {
			return ErrOauthAlreadyBound
		}
		return ErrOauthBoundByOther
	}
	if err = assertSsoUserActive(adminId, userId); err != nil {
		return err
	}
	m := msql.Model(define.TableUserSsoBindings, define.Postgres)
	bound, err := m.Where2([][]string{
		{`user_id`, cast.ToString(userId)},
		{`provider`, config.Provider},
		{`app_key`, config.AppKey},
	}).Value(`id`)
	if err != nil {
		logs.Error("query sso binding by user failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return err
	}
	if cast.ToInt(bound) > 0 {
		return ErrOauthAlreadyBound
	}
	return insertSsoBindingTx(nil, adminId, userId, config.Provider, config.AppKey, info)
}

// insertSsoBindingTx 写入绑定行；tx 为 nil 时走自动提交连接。
func insertSsoBindingTx(tx *sql.Tx, adminId int, userId int, provider string, appKey string, info SsoUserInfo) error {
	now := tool.Time2Int()
	_, err := msql.RawExec(define.Postgres, `INSERT INTO user_sso_bindings
		(admin_id,user_id,provider,app_key,external_id,username,email,job_title,employee_no,nickname,avatar,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12)`, tx,
		adminId, userId, provider, appKey, info.ExternalId, info.Username, info.Email,
		info.JobTitle, info.EmployeeNo, info.Name, info.Avatar, now)
	if err != nil {
		logs.Error("insert sso binding failed: admin_id=%d, provider=%s, err=%s", adminId, provider, err.Error())
	}
	return err
}

// ssoJitProvision 首次登录建号。重试对象只有随机 username 的碰撞，不是绑定冲突——
// 唯一冲突会把事务打成 aborted 状态，同事务内不能继续写，所以每次重试都开新事务。
func ssoJitProvision(config SsoConfigModel, info SsoUserInfo) (int, error) {
	if err := EnsureDefaultPlan(config.AdminId); err != nil {
		return 0, err
	}
	roleId, err := enterprisePresetRoleId(config.AdminId, `perm_role_enterprise_member`)
	if err != nil {
		return 0, err
	}
	for attempt := 0; attempt < ssoJitUsernameRetries; attempt++ {
		username, err := NewSsoJitUsername()
		if err != nil {
			return 0, err
		}
		userId, err := ssoJitAttempt(config, info, username, roleId)
		if err == nil {
			return userId, nil
		}
		if isUserUsernameConflict(err) {
			logs.Error("sso jit username collision: admin_id=%d, attempt=%d", config.AdminId, attempt+1)
			continue
		}
		return 0, err
	}
	logs.Error("sso jit create failed: username collisions exceeded %d retries, admin_id=%d",
		ssoJitUsernameRetries, config.AdminId)
	return 0, errors.New(`sso jit username collision after retries`)
}

// ssoJitAttempt 在单个事务里完成「配额锁 → 重查绑定 → 建号 → 补头像 → 写绑定」。
func ssoJitAttempt(config SsoConfigModel, info SsoUserInfo, username string, roleId int) (int, error) {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin sso jit tx failed: %s", err.Error())
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	// 咨询锁只串行化配额计数，不会自动重读身份，所以取锁后必须再查一次绑定：
	// 同一外部身份并发登录时，后拿到锁的一方如果直接建号就会撞唯一索引。
	if err = CheckMemberQuotaTx(tx, config.AdminId); err != nil {
		return 0, err
	}
	userId, found, err := getSsoBindingUserIdTx(tx, config.AdminId, config.Provider, config.AppKey, info.ExternalId)
	if err != nil {
		return 0, err
	}
	if found {
		if err = tx.Commit(); err != nil {
			logs.Error("commit sso jit tx failed: %s", err.Error())
			return 0, err
		}
		return userId, nil
	}

	password, err := randomOauthToken(ssoJitPasswordBytes)
	if err != nil {
		return 0, err
	}
	name := info.Name
	if name == `` {
		name = username
	}
	userId, err = createUserTx(tx, CreateUserModel{
		Username: username, Password: password, Name: name, Phone: info.Phone,
		AdminId: config.AdminId, RoleId: roleId,
	}, 0)
	if err != nil {
		return 0, err
	}
	// createUserTx 的列清单里没有 avatar，头像必须在同一事务内补一条 UPDATE。
	if info.Avatar != `` {
		if _, err = msql.RawExec(define.Postgres, `UPDATE users SET avatar=$1, updated_at=$2 WHERE id=$3`,
			tx, info.Avatar, tool.Time2Int(), userId); err != nil {
			logs.Error("set sso jit avatar failed: %s", err.Error())
			return 0, err
		}
	}
	if err = insertSsoBindingTx(tx, config.AdminId, userId, config.Provider, config.AppKey, info); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit sso jit tx failed: %s", err.Error())
		return 0, err
	}
	return userId, nil
}

// getSsoBindingUserIdTx 在事务内查绑定，返回 (user_id, 是否存在, 错误)。
func getSsoBindingUserIdTx(tx *sql.Tx, adminId int, provider string, appKey string, externalId string) (int, bool, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT user_id FROM user_sso_bindings
		WHERE admin_id=$1 AND provider=$2 AND app_key=$3 AND external_id=$4 LIMIT 1`, tx,
		adminId, provider, appKey, externalId)
	if err != nil {
		logs.Error("query sso binding in tx failed: %s", err.Error())
		return 0, false, err
	}
	if len(rows) == 0 {
		return 0, false, nil
	}
	return cast.ToInt(rows[0][`user_id`]), true, nil
}

// NewSsoJitUsername 生成 16 位随机账号（大写字母 + 数字，去掉 0/O/1/I）。
// createUserTx 落库前会转小写，因此重试判重要按小写值比对——这里只负责生成。
func NewSsoJitUsername() (string, error) {
	charset := big.NewInt(int64(len(ssoJitCharset)))
	builder := strings.Builder{}
	builder.Grow(ssoJitUsernameLen)
	for i := 0; i < ssoJitUsernameLen; i++ {
		index, err := rand.Int(rand.Reader, charset)
		if err != nil {
			logs.Error("generate sso username failed: %s", err.Error())
			return ``, err
		}
		builder.WriteByte(ssoJitCharset[index.Int64()])
	}
	return builder.String(), nil
}

// ssoTrimUserInfo 按列长度截断第三方资料，避免超出长度导致整条写入失败。
// 姓名优先取 Name，缺失时退到 Username：users.name 是界面展示的主力字段。
func ssoTrimUserInfo(info SsoUserInfo) SsoUserInfo {
	info.ExternalId = truncateOauthText(info.ExternalId, ssoExternalIdLimit)
	info.Username = truncateOauthText(info.Username, ssoFieldUsernameLimit)
	info.Email = truncateOauthText(info.Email, ssoFieldEmailLimit)
	info.JobTitle = truncateOauthText(info.JobTitle, ssoFieldJobTitleLimit)
	info.EmployeeNo = truncateOauthText(info.EmployeeNo, ssoFieldEmpNoLimit)
	name := info.Name
	if name == `` {
		name = info.Username
	}
	info.Name = truncateOauthText(name, ssoFieldNameLimit)
	info.Phone = truncateOauthText(info.Phone, ssoFieldPhoneLimit)
	info.Avatar = truncateOauthText(info.Avatar, ssoFieldAvatarLimit)
	return info
}

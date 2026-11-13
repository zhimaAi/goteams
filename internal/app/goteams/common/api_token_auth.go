// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package common

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// ApiTokenPrefix 是 API token 明文前缀，用于区分登录会话 token 和运营开放接口签名。
const ApiTokenPrefix = `gt_`

// Context keys injected by ApiTokenAuthMiddleware.
const (
	CtxApiTokenId    = `apiTokenId`
	CtxApiTokenPerms = `apiTokenPerms`
	CtxApiTokenDay   = `apiTokenDay`
)

// GenerateApiToken 生成一枚 API token，返回明文、展示用前缀和入库的 sha256。
// 明文只在创建响应里出现一次，服务端只留 hash，遗失无法找回。
func GenerateApiToken() (plain, prefix, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		logs.Error("generate api token failed: %s", err.Error())
		return ``, ``, ``, err
	}
	plain = ApiTokenPrefix + base64.RawURLEncoding.EncodeToString(buf)
	prefix = plain
	if len(prefix) > 16 {
		prefix = prefix[:16]
	}
	return plain, prefix, ApiTokenHash(plain), nil
}

// ApiTokenHash 返回明文 token 的 sha256 hex，即 api_tokens.token_hash 的取值。
func ApiTokenHash(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// ApiTokenAuthMiddleware 校验 `Authorization: Bearer gt_xxx`，只做身份判定与上下文注入。
// 校验通过后注入 adminId / userId，开放 API 可直接复用既有 business 逻辑。
// 租户日额度不在这里扣：见 ApiQuotaCharge。
func ApiTokenAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := lib_i18n.GetLang(c)

		plain := strings.TrimSpace(strings.TrimPrefix(c.GetHeader(`Authorization`), `Bearer `))
		if !strings.HasPrefix(plain, ApiTokenPrefix) {
			abortApiToken(c, http.StatusUnauthorized, lib_i18n.Show(lang, define.MsgApiTokenInvalid))
			return
		}

		row, err := msql.Model(define.TableApiTokens, define.Postgres).
			Field(`id, admin_id, user_id, enabled, permissions`).
			Where(`token_hash`, ApiTokenHash(plain)).Find()
		if err != nil {
			logs.Error("find api token failed: err=%s", err.Error())
			c.JSON(http.StatusInternalServerError, define.Response{
				Code:    define.CodeError,
				Message: lib_i18n.Show(lang, define.MsgError),
			})
			c.Abort()
			return
		}
		if len(row) == 0 {
			abortApiToken(c, http.StatusUnauthorized, lib_i18n.Show(lang, define.MsgApiTokenInvalid))
			return
		}
		if cast.ToInt(row[`enabled`]) != 1 {
			abortApiToken(c, http.StatusForbidden, lib_i18n.Show(lang, define.MsgApiTokenDisabled))
			return
		}

		adminId, userId, tokenId := cast.ToInt(row[`admin_id`]), cast.ToInt(row[`user_id`]), cast.ToInt(row[`id`])

		// 凭据挂在人身上：归属成员被禁用或移出租户后，api_tokens 行本身不会消失，
		// 必须在这里单独兜底，否则 Key 会继续以「已停用 / 已移除」的身份调用。
		// 登录态靠禁用时撤销 Redis 会话兜底，API Key 没有会话可撤，只能每次查一次 users。
		ownerActive, err := apiTokenOwnerActive(adminId, userId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, define.Response{
				Code:    define.CodeError,
				Message: lib_i18n.Show(lang, define.MsgError),
			})
			c.Abort()
			return
		}
		if !ownerActive {
			abortApiToken(c, http.StatusUnauthorized, lib_i18n.Show(lang, define.MsgApiTokenOwnerInactive))
			return
		}

		c.Set(`adminId`, adminId)
		c.Set(`userId`, userId)
		c.Set(CtxApiTokenId, tokenId)
		c.Set(CtxApiTokenPerms, ParseTextArray(row[`permissions`]))
		c.Set(CtxApiTokenDay, ApiQuotaDay(time.Now()))
	}
}

// apiTokenOwnerActive 报告 API token 归属用户当前是否可用：users 行还在（未被移出租户）且未禁用。
// status 口径与 Login / RefreshLoginToken 一致：0 视为禁用，因此拿不到行或 status=0 都判为不可用。
func apiTokenOwnerActive(adminId, userId int) (bool, error) {
	if adminId <= 0 || userId <= 0 {
		return false, nil
	}
	m := msql.Model(define.TableUsers, define.Postgres)
	row, err := m.Where2([][]string{
		{`id`, cast.ToString(userId)},
		{`admin_id`, cast.ToString(adminId)},
		{`deleted_at`, `0`},
	}).Field(`status`).Find()
	if err != nil {
		logs.Error("check api token owner failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return false, err
	}
	if len(row) == 0 {
		return false, nil
	}
	return cast.ToInt(row[`status`]) != define.MemberStatusDisabled, nil
}

// ApiQuotaCharge 在权限校验通过后累计调用量，私有化不设每日套餐额度。
func ApiQuotaCharge(c *gin.Context) bool {
	lang := lib_i18n.GetLang(c)
	adminId, day := c.GetInt(`adminId`), c.GetInt(CtxApiTokenDay)
	allowed, err := ApiQuotaConsume(adminId, day)
	if err != nil {
		c.JSON(http.StatusInternalServerError, define.Response{
			Code:    define.CodeError,
			Message: lib_i18n.Show(lang, define.MsgError),
		})
		c.Abort()
		return false
	}
	if !allowed {
		abortApiToken(c, http.StatusTooManyRequests, lib_i18n.Show(lang, define.MsgApiQuotaExceeded))
		return false
	}
	return true
}

// ApiTokenStatSuccess 在业务 handler 写回响应之后累加调用统计：只有成功调用才计数，
// 失败重试不应污染「今日触发」。必须由路由组合器在 action 之后调用——
// 在中间件里用 c.Next() 判断状态码是错的，见 ApiTokenAuthMiddleware 的说明。
func ApiTokenStatSuccess(c *gin.Context) {
	if apiTokenStatCounts(c.Writer.Status()) {
		ApiTokenStatAdd(c.GetInt(CtxApiTokenId), c.GetInt(CtxApiTokenDay))
	}
}

// apiTokenStatCounts 报告状态码是否算一次成功调用。单独成函数是因为 ApiTokenStatAdd 直接写库，
// 「失败不计统计」这条计费口径否则无法被测试观察。
func apiTokenStatCounts(status int) bool {
	return status < http.StatusBadRequest
}

// ApiPermMiddleware 校验当前 API token 是否具备指定权限点，用于路由级拦截。
func ApiPermMiddleware(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if HasApiTokenPerm(c, perm) {
			c.Next()
			return
		}
		c.JSON(http.StatusForbidden, define.Response{
			Code:    http.StatusForbidden,
			Message: lib_i18n.Show(lib_i18n.GetLang(c), define.MsgApiPermissionDenied),
		})
		c.Abort()
	}
}

// ApiPermAnyMiddleware 校验当前 API token 具备其中任意一个权限点；一个动作被多个功能共用时使用（例如上传工作项素材）。
func ApiPermAnyMiddleware(perms ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if HasAnyApiTokenPerm(c, perms) {
			c.Next()
			return
		}
		c.JSON(http.StatusForbidden, define.Response{
			Code:    http.StatusForbidden,
			Message: lib_i18n.Show(lib_i18n.GetLang(c), define.MsgApiPermissionDenied),
		})
		c.Abort()
	}
}

// HasAnyApiTokenPerm 报告当前 API token 是否具备其中任意一个权限点。
func HasAnyApiTokenPerm(c *gin.Context, perms []string) bool {
	for _, perm := range perms {
		if HasApiTokenPerm(c, perm) {
			return true
		}
	}
	return false
}

// HasApiTokenPerm 报告当前 API token 是否具备指定权限点。
func HasApiTokenPerm(c *gin.Context, perm string) bool {
	value, ok := c.Get(CtxApiTokenPerms)
	if !ok {
		return false
	}
	perms, ok := value.([]string)
	if !ok {
		return false
	}
	for _, item := range perms {
		if item == perm {
			return true
		}
	}
	return false
}

// abortApiToken 返回统一结构的鉴权失败响应。
func abortApiToken(c *gin.Context, status int, message string) {
	c.JSON(status, define.Response{Code: status, Message: message})
	c.Abort()
}

// ParseTextArray 解析 Postgres TEXT[] 的文本形态 `{a,b,c}`，空数组和空串都返回 nil。
// 权限点取值固定为 `域:操作`，不含逗号和引号，无需处理转义。
func ParseTextArray(raw string) []string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, `{`)
	raw = strings.TrimSuffix(raw, `}`)
	if raw == `` {
		return nil
	}
	parts := strings.Split(raw, `,`)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.Trim(part, `"`); part != `` {
			out = append(out, part)
		}
	}
	return out
}

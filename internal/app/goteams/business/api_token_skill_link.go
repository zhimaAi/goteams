// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_redis"

	"github.com/go-redis/redis/v8"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/tool"
)

// ErrApiTokenSkillLinkGone 表示临时安装链接不存在、已过期，或下载次数已用完。
var ErrApiTokenSkillLinkGone = errors.New(`api token skill link gone`)

// apiTokenSkillLinkTTL 是临时链接的有效期，与写入 Redis 时使用的 TTL 同源。
var apiTokenSkillLinkTTL = time.Duration(define.ApiTokenSkillLinkTTLSeconds) * time.Second

// ApiTokenSkillLinkModel 是临时链接背后的下载配置：凭它即可重新渲染出与该 token 一致的安装包。
// 因此链接一旦泄漏等于泄漏安装包，只能靠短有效期与有效期内限次下载收窄暴露窗口。
type ApiTokenSkillLinkModel struct {
	AdminId int    `json:"admin_id"`
	UserId  int    `json:"user_id"`
	TokenId int    `json:"token_id"`
	SiteUrl string `json:"site_url"`
	Lang    string `json:"lang"`
}

// CreateApiTokenSkillLink 为指定 token 生成临时安装链接的 key，并返回有效期（秒）。
// 只有 token 归属者本人能生成；同一 token 同时只保留一个有效链接，重新生成会让旧链接立即失效。
func CreateApiTokenSkillLink(adminId, userId, tokenId int, siteUrl, lang string) (string, int, error) {
	if !apiTokenOwned(adminId, userId, tokenId) {
		return ``, 0, ErrApiTokenNotFound
	}
	key, err := generateApiTokenSkillLinkKey()
	if err != nil {
		return ``, 0, err
	}
	payload, err := tool.JsonEncode(ApiTokenSkillLinkModel{
		AdminId: adminId,
		UserId:  userId,
		TokenId: tokenId,
		SiteUrl: siteUrl,
		Lang:    lang,
	})
	if err != nil {
		logs.Error("encode api token skill link failed: token_id=%d, err=%s", tokenId, err.Error())
		return ``, 0, err
	}
	// 先撤掉该 token 上已有的链接，再写新的：索引后写，读到它就一定指向当前有效的链接。
	apiTokenSkillDropPreviousLink(adminId, userId, tokenId)
	linkRedisKey := apiTokenSkillLinkKey(key)
	if err = lib_redis.Set(define.Redis, linkRedisKey, payload, apiTokenSkillLinkTTL); err != nil {
		logs.Error("save api token skill link failed: token_id=%d, err=%s", tokenId, err.Error())
		return ``, 0, err
	}
	// 下载计数键与链接同时创建、同一 TTL：两者的生命周期绑在一起，
	// 就不会出现「链接还在、计数先过期」导致名额被重置的情况。
	countKey := apiTokenSkillLinkCountKey(key)
	if err = lib_redis.Set(define.Redis, countKey, 0, apiTokenSkillLinkTTL); err != nil {
		logs.Error("save api token skill link count failed: token_id=%d, err=%s", tokenId, err.Error())
		_ = lib_redis.Del(define.Redis, linkRedisKey)
		return ``, 0, err
	}
	indexKey := apiTokenSkillLinkTokenKey(adminId, userId, tokenId)
	if err = lib_redis.Set(define.Redis, indexKey, key, apiTokenSkillLinkTTL); err != nil {
		logs.Error("save api token skill link index failed: token_id=%d, err=%s", tokenId, err.Error())
		_ = lib_redis.Del(define.Redis, linkRedisKey, countKey)
		return ``, 0, err
	}
	return key, define.ApiTokenSkillLinkTTLSeconds, nil
}

// UseApiTokenSkillLink 取回临时链接的下载配置，并占用一次下载名额。
// 每个链接在有效期内最多下载 define.ApiTokenSkillLinkMaxDownloads 次，超出后链接作废。
// 名额用 Redis 的 INCR 发放：INCR 是原子的，并发请求各自拿到不同序号，序号超上限的直接拒绝，
// 所以并发也发不出超额名额（不用 GETDEL：本项目部署的是 Redis 5.0，而它需要 6.2+）。
func UseApiTokenSkillLink(linkKey string) (ApiTokenSkillLinkModel, error) {
	linkKey = strings.TrimSpace(linkKey)
	if linkKey == `` {
		return ApiTokenSkillLinkModel{}, ErrApiTokenSkillLinkGone
	}
	key := apiTokenSkillLinkKey(linkKey)
	raw, err := lib_redis.Get(define.Redis, key)
	if err != nil && !errors.Is(err, redis.Nil) {
		// 读取故障与「链接不存在」对外同为失效（这是公开接口，不该泄漏内部状态），
		// 但故障要留痕，否则 Redis 挂了会表现为「所有链接都过期」而没人发现。
		logs.Error("read api token skill link failed: key=%s, err=%s", key, err.Error())
	}
	if err != nil || raw == `` {
		return ApiTokenSkillLinkModel{}, ErrApiTokenSkillLinkGone
	}
	used, err := lib_redis.Incr(define.Redis, apiTokenSkillLinkCountKey(linkKey))
	if err != nil {
		logs.Error("count api token skill link download failed: key=%s, err=%s", key, err.Error())
		return ApiTokenSkillLinkModel{}, ErrApiTokenSkillLinkGone
	}
	if used > define.ApiTokenSkillLinkMaxDownloads {
		// 名额已用完：链接连同计数一起清掉，不再对外提供。
		apiTokenSkillDropLink(linkKey)
		return ApiTokenSkillLinkModel{}, ErrApiTokenSkillLinkGone
	}
	var model ApiTokenSkillLinkModel
	if err = tool.JsonDecodeUseNumber(raw, &model); err != nil {
		logs.Error("decode api token skill link failed: err=%s", err.Error())
		return ApiTokenSkillLinkModel{}, ErrApiTokenSkillLinkGone
	}
	return model, nil
}

// DropApiTokenSkillLink 删除某个 token 当前有效的临时链接，供 token 被删除时顺手清理。
func DropApiTokenSkillLink(adminId, userId, tokenId int) {
	apiTokenSkillDropPreviousLink(adminId, userId, tokenId)
}

// apiTokenSkillDropPreviousLink 删除该 token 当前有效的临时链接（若有），连同下载计数与反查索引。
func apiTokenSkillDropPreviousLink(adminId, userId, tokenId int) {
	indexKey := apiTokenSkillLinkTokenKey(adminId, userId, tokenId)
	previous, err := lib_redis.Get(define.Redis, indexKey)
	if err != nil || previous == `` {
		return
	}
	if err = lib_redis.Del(define.Redis, apiTokenSkillLinkKey(previous), apiTokenSkillLinkCountKey(previous), indexKey); err != nil {
		logs.Error("drop previous api token skill link failed: err=%s", err.Error())
	}
}

// apiTokenSkillDropLink 删除一条临时链接及其下载计数，供名额用完时作废该链接。
func apiTokenSkillDropLink(linkKey string) {
	if err := lib_redis.Del(define.Redis, apiTokenSkillLinkKey(linkKey), apiTokenSkillLinkCountKey(linkKey)); err != nil {
		logs.Error("drop api token skill link failed: key=%s, err=%s", linkKey, err.Error())
	}
}

// generateApiTokenSkillLinkKey 生成 32 位 URL 安全的随机 key（字母、数字、`-`、`_`）。
func generateApiTokenSkillLinkKey() (string, error) {
	buf := make([]byte, define.ApiTokenSkillLinkKeyBytes)
	if _, err := rand.Read(buf); err != nil {
		logs.Error("generate api token skill link key failed: err=%s", err.Error())
		return ``, err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// apiTokenSkillLinkKey 是临时链接 key 在 Redis 里的完整键名。
func apiTokenSkillLinkKey(linkKey string) string {
	return define.CacheKeyApiTokenSkillLink + `:` + linkKey
}

// apiTokenSkillLinkCountKey 是临时链接下载计数键名，与链接同生命周期。
func apiTokenSkillLinkCountKey(linkKey string) string {
	return define.CacheKeyApiTokenSkillLinkCount + `:` + linkKey
}

// apiTokenSkillLinkTokenKey 是 token 维度的反查索引键名，保证同一个 token 只有一个有效链接。
func apiTokenSkillLinkTokenKey(adminId, userId, tokenId int) string {
	return strings.Join([]string{
		define.CacheKeyApiTokenSkillLinkToken,
		strconv.Itoa(adminId),
		strconv.Itoa(userId),
		strconv.Itoa(tokenId),
	}, `:`)
}

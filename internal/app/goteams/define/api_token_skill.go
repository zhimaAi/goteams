// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package define

// API token skill 安装包的临时下载链接：链接里的 key 只存在 Redis，有效期 5 分钟，
// 有效期内最多下载 ApiTokenSkillLinkMaxDownloads 次。
// 链接本身不携带任何凭据，key 又是 32 位随机串，外部无法通过列举拿到有效链接。
const (
	// CacheKeyApiTokenSkillLink 是「临时链接 key → 下载配置」的缓存键前缀。
	CacheKeyApiTokenSkillLink = `goteams:api-token:skill-link`
	// CacheKeyApiTokenSkillLinkToken 是「token → 当前有效临时链接 key」的反查索引键前缀，
	// 用来保证同一个 token 同时只有一个有效链接：重新生成时旧链接立即失效。
	CacheKeyApiTokenSkillLinkToken = `goteams:api-token:skill-link-token`
	// CacheKeyApiTokenSkillLinkCount 是「临时链接 key → 已下载次数」的计数键前缀。
	// 单独一个键是因为计数要用原子的 INCR，不能和下载配置挤在同一个键里。
	CacheKeyApiTokenSkillLinkCount = `goteams:api-token:skill-link-count`
	// ApiTokenSkillLinkTTLSeconds 是临时链接的有效期（秒）。
	ApiTokenSkillLinkTTLSeconds = 300
	// ApiTokenSkillLinkMaxDownloads 是单个链接在有效期内允许的下载次数。
	ApiTokenSkillLinkMaxDownloads = 10
	// ApiTokenSkillLinkKeyBytes 是临时链接 key 的随机字节数：24 字节 base64url 编码后正好 32 个字符。
	ApiTokenSkillLinkKeyBytes = 24
	// ApiTokenSkillLinkPath 是临时安装链接的路径前缀，与 route 里注册的公开路由成对维护。
	ApiTokenSkillLinkPath = `/api/public/skill-links/`
)

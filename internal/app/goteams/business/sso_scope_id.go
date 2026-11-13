// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"sync"

	"goteams/internal/app/goteams/define"

	"github.com/zhimaAi/go_tools/logs"
)

// ErrSsoScopeKeyInvalid 表示 [sso] scope_id_key 缺失或长度非法。
// 它不阻断服务启动，只让入口地址生成为空；配置接口据此提示，绝不降级成明文 adminId。
var ErrSsoScopeKeyInvalid = errors.New("sso scope_id_key is missing or invalid")

// ssoScopeKeyRunes 是 AES-256 需要的密钥字节数，对应 INI 里 64 位十六进制字符。
const ssoScopeKeyRunes = 64

// ssoScopeKeyCache 缓存解析后的密钥。INI 只在启动时读一次，这里存在的意义是把
// 「长度非法」这条错误路径的重复解析与重复日志挡掉，而不是热更新配置。
var ssoScopeKeyCache struct {
	sync.Mutex
	raw   string
	key   []byte
	valid bool
}

// ssoScopeKey 读取并校验入口加密密钥。
func ssoScopeKey() ([]byte, error) {
	raw := strings.TrimSpace(define.Config.Sso[`scope_id_key`])
	ssoScopeKeyCache.Lock()
	defer ssoScopeKeyCache.Unlock()
	if ssoScopeKeyCache.raw == raw && ssoScopeKeyCache.valid {
		return ssoScopeKeyCache.key, nil
	}
	if len(raw) != ssoScopeKeyRunes {
		return nil, ErrSsoScopeKeyInvalid
	}
	key, err := hex.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, ErrSsoScopeKeyInvalid
	}
	ssoScopeKeyCache.raw, ssoScopeKeyCache.key, ssoScopeKeyCache.valid = raw, key, true
	return key, nil
}

// EncodeSsoScopeId 把租户编号加密成入口参数。每次都用新的随机 nonce，所以同一租户
// 两次得到的 scope_id 不同——多个地址同时有效是设计结果，前端不得缓存或比较它。
func EncodeSsoScopeId(adminId int) (string, error) {
	key, err := ssoScopeKey()
	if err != nil {
		return ``, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		logs.Error("sso scope_id cipher error: %s", err.Error())
		return ``, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		logs.Error("sso scope_id gcm error: %s", err.Error())
		return ``, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		logs.Error("sso scope_id nonce error: %s", err.Error())
		return ``, err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(strconv.Itoa(adminId)), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// DecodeSsoScopeId 解出入口参数声明的租户编号。它只回答「你属于哪个租户」，不是鉴权：
// 真正的准入是入口处的「租户存在 + enabled=1 + 凭证完整」。改一个字符就会因认证标签失败。
func DecodeSsoScopeId(scopeId string) (int, error) {
	key, err := ssoScopeKey()
	if err != nil {
		return 0, err
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(scopeId))
	if err != nil {
		return 0, ErrSsoScopeKeyInvalid
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return 0, err
	}
	if len(raw) < gcm.NonceSize() {
		return 0, ErrSsoScopeKeyInvalid
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return 0, ErrSsoScopeKeyInvalid
	}
	adminId, err := strconv.Atoi(string(plain))
	if err != nil || adminId <= 0 {
		return 0, ErrSsoScopeKeyInvalid
	}
	return adminId, nil
}

// NewSsoCallbackKey 生成回调地址里的租户标识：24 字节随机 → 32 个 URL 安全字符。
// 它只在配置行首次物化时写入，此后恒定，PUT 保存不接受也不修改它。
//
// 刻意不加密 adminId：确定性加密等价于对低熵编号做可比对的指纹，回调地址要交给第三方 IdP，
// 拿到一条地址就能逐个枚举反推企业编号。随机值同时满足「恒定可登记」与「不可反推」。
func NewSsoCallbackKey() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		logs.Error("generate sso callback key failed: %s", err.Error())
		return ``, err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

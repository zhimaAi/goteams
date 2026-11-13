// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package common

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"goteams/internal/app/goteams/define"

	"github.com/go-redis/redis/v8"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/tool"
)

const (
	sessionKeyPrefix   = `goteams:session:`
	sessionIndexPrefix = `goteams:session_index:`
)

// ErrSessionInvalid indicates a missing, expired or malformed session.
var ErrSessionInvalid = errors.New(`session invalid`)

// SessionData describes the server-side login session stored in Redis.
type SessionData struct {
	UserId    int    `json:"user_id"`
	AdminId   int    `json:"admin_id"`
	CreatedAt int    `json:"created_at"`
	Ip        string `json:"ip"`
	Ua        string `json:"ua"`
}

// sessionTTL returns the idle timeout (sliding window) from config.
func sessionTTL() time.Duration {
	return time.Duration(cast.ToInt(define.Config.Session[`ttl`])) * time.Second
}

// sessionMaxLife returns the absolute session lifetime in seconds from config.
func sessionMaxLife() int {
	return cast.ToInt(define.Config.Session[`max_life`])
}

// CreateSession stores a new session in Redis and returns the opaque token.
func CreateSession(data SessionData) (string, error) {
	return CreateSessionWithTTL(data, sessionTTL())
}

// CreateSessionWithTTL 使用自定义初始有效期创建会话（用于运营短期登录链接等场景）。
// 会话一旦被使用，后续请求仍按标准滑动窗口续期。
func CreateSessionWithTTL(data SessionData, ttl time.Duration) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		logs.Error("generate session token failed: %s", err.Error())
		return ``, err
	}
	token := hex.EncodeToString(buf)

	data.CreatedAt = tool.Time2Int()
	raw, err := json.Marshal(data)
	if err != nil {
		logs.Error("marshal session failed: %s", err.Error())
		return ``, err
	}

	ctx := context.Background()
	indexKey := sessionIndexPrefix + cast.ToString(data.UserId)
	pipe := define.Redis.Pipeline()
	pipe.Set(ctx, sessionKeyPrefix+token, raw, ttl)
	pipe.ZAdd(ctx, indexKey, &redis.Z{Score: float64(data.CreatedAt), Member: token})
	// The index only needs to outlive the longest possible session (absolute lifetime).
	pipe.Expire(ctx, indexKey, time.Duration(sessionMaxLife())*time.Second)
	if _, err = pipe.Exec(ctx); err != nil {
		logs.Error("create session failed: %s", err.Error())
		return ``, err
	}

	return token, nil
}

// GetSession validates a session token, enforces the absolute lifetime and slides the idle TTL.
func GetSession(token string) (SessionData, error) {
	if len(token) == 0 {
		return SessionData{}, ErrSessionInvalid
	}

	ctx := context.Background()
	raw, err := define.Redis.Get(ctx, sessionKeyPrefix+token).Result()
	if errors.Is(err, redis.Nil) {
		return SessionData{}, ErrSessionInvalid
	}
	if err != nil {
		logs.Error("get session failed: %s", err.Error())
		return SessionData{}, err
	}

	var data SessionData
	if err = json.Unmarshal([]byte(raw), &data); err != nil || data.UserId <= 0 {
		logs.Error("unmarshal session failed: %s", raw)
		return SessionData{}, ErrSessionInvalid
	}

	// Absolute lifetime gate: even active sessions must re-login after max_life.
	if tool.Time2Int()-data.CreatedAt > sessionMaxLife() {
		DeleteSession(token)
		return SessionData{}, ErrSessionInvalid
	}

	// Sliding expiration: any authenticated request restarts the idle window.
	if err = define.Redis.Expire(ctx, sessionKeyPrefix+token, sessionTTL()).Err(); err != nil {
		logs.Error("renew session ttl failed: %s", err.Error())
	}

	return data, nil
}

// DeleteSession removes a single session and its index entry (used by logout).
func DeleteSession(token string) {
	if len(token) == 0 {
		return
	}
	ctx := context.Background()

	raw, err := define.Redis.Get(ctx, sessionKeyPrefix+token).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		logs.Error("delete session read failed: %s", err.Error())
	}

	pipe := define.Redis.Pipeline()
	pipe.Del(ctx, sessionKeyPrefix+token)
	var data SessionData
	if json.Unmarshal([]byte(raw), &data) == nil && data.UserId > 0 {
		pipe.ZRem(ctx, sessionIndexPrefix+cast.ToString(data.UserId), token)
	}
	if _, err = pipe.Exec(ctx); err != nil {
		logs.Error("delete session failed: %s", err.Error())
	}
}

// DeleteUserSessions removes all sessions of a user, optionally keeping one token (empty keepToken removes all).
// Deletion failures are logged only: business operations must not be rolled back by session cleanup.
func DeleteUserSessions(userId int, keepToken string) {
	ctx := context.Background()
	indexKey := sessionIndexPrefix + cast.ToString(userId)

	tokens, err := define.Redis.ZRange(ctx, indexKey, 0, -1).Result()
	if err != nil {
		logs.Error("list user sessions failed: %s", err.Error())
		return
	}

	pipe := define.Redis.Pipeline()
	for _, token := range tokens {
		if token == keepToken {
			continue
		}
		pipe.Del(ctx, sessionKeyPrefix+token)
		pipe.ZRem(ctx, indexKey, token)
	}
	if _, err = pipe.Exec(ctx); err != nil {
		logs.Error("delete user sessions failed: %s", err.Error())
	}
}


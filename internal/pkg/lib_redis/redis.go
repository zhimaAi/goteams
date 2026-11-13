// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package lib_redis

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/tool"
)

// CacheBuildHandler is the cache build interface — callback to build data when cache misses.
// References wensou's lib_redis encapsulation for cache penetration protection.
type CacheBuildHandler interface {
	GetCacheKey() string
	GetCacheData() (any, time.Duration, error)
}

// GetCacheWithBuild reads from cache with penetration protection.
//   - cache: Redis client
//   - handler: cache build interface
//   - result: deserialization target (pointer)
//
// Flow: 1. Check cache → 2. Hit → return directly → 3. Miss → callback handler to build → 4. Write to cache → 5. Return
func GetCacheWithBuild(cache *redis.Client, handler CacheBuildHandler, result any) error {
	cacheKey := handler.GetCacheKey()
	if len(cacheKey) == 0 {
		return errors.New(`cache key is empty`)
	}

	// 1. Try reading from Redis.
	jsonStr, err := cache.Get(context.Background(), cacheKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		logs.Error("redis get error: %s", err.Error())
		return err
	}

	// 2. Cache hit.
	if len(jsonStr) > 0 {
		if jsonStr == `-1` { // Sentinel value: no data in database.
			return nil
		}
		return tool.JsonDecodeUseNumber(jsonStr, result)
	}

	// 3. Cache miss → callback to build data.
	data, ttl, err := handler.GetCacheData()
	if err != nil {
		logs.Error("build cache data error: %s", err.Error())
		return err
	}

	jsonStr, err = tool.JsonEncode(data)
	if err != nil {
		logs.Error("json encode error: %s", err.Error())
		return err
	}

	// 4. Write sentinel value to prevent penetration when no data.
	if data == nil || len(jsonStr) == 0 || jsonStr == `{}` || jsonStr == `[]` {
		jsonStr = `-1`
	}

	_, err = cache.Set(context.Background(), cacheKey, jsonStr, ttl).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		logs.Error("redis set error: %s", err.Error())
		return err
	}

	// 5. Return empty for sentinel value.
	if jsonStr == `-1` {
		return nil
	}
	return tool.JsonDecodeUseNumber(jsonStr, result)
}

// DelCacheData deletes cache data (called after data modification).
func DelCacheData(cache *redis.Client, handler CacheBuildHandler) {
	cacheKey := handler.GetCacheKey()
	if len(cacheKey) == 0 {
		return
	}
	_, err := cache.Del(context.Background(), cacheKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		logs.Error("redis del error: %s", err.Error())
	}
}

// AddLock is a distributed lock based on SetNX (returns true if lock acquired).
func AddLock(cache *redis.Client, key string, ttl time.Duration) bool {
	ok, err := cache.SetNX(context.Background(), key, time.Now().Unix(), ttl).Result()
	if err != nil {
		logs.Error("redis lock error: %s", err.Error())
		return false
	}
	return ok
}

// UnLock releases a distributed lock.
func UnLock(cache *redis.Client, key string) {
	_, err := cache.Del(context.Background(), key).Result()
	if err != nil {
		logs.Error("redis unlock error: %s", err.Error())
	}
}

// Get basic get.
func Get(cache *redis.Client, key string) (string, error) {
	return cache.Get(context.Background(), key).Result()
}

// Set basic set.
func Set(cache *redis.Client, key string, value any, ttl time.Duration) error {
	return cache.Set(context.Background(), key, value, ttl).Err()
}

// Del basic delete.
func Del(cache *redis.Client, keys ...string) error {
	return cache.Del(context.Background(), keys...).Err()
}

// Incr increments the integer stored at key and returns the new value.
// The increment is atomic, so it is the cheapest way to hand out a limited number of
// slots to concurrent callers: each caller gets a distinct sequence number and the ones
// above the limit can be rejected. A missing key is treated as 0.
func Incr(cache *redis.Client, key string) (int64, error) {
	return cache.Incr(context.Background(), key).Result()
}

// takeScript returns the value of one key and deletes it inside a single server-side step,
// so two concurrent callers can never observe the same value.
var takeScript = redis.NewScript(`local value = redis.call('GET', KEYS[1])
if value == false then
	return nil
end
redis.call('DEL', KEYS[1])
return value`)

// Take atomically reads and deletes the value at key — a one-shot consume, used for
// credentials that must not be replayed (OAuth state, one-time tickets).
// The second return value reports whether the key existed; a missing key is not an error.
// A Lua script is used instead of the GETDEL command because GETDEL requires Redis 6.2+,
// while Lua scripts are supported since Redis 2.6.
func Take(cache *redis.Client, key string) (string, bool, error) {
	value, err := takeScript.Run(context.Background(), cache, []string{key}).Result()
	if errors.Is(err, redis.Nil) {
		return ``, false, nil
	}
	if err != nil {
		return ``, false, err
	}
	text, ok := value.(string)
	if !ok {
		return ``, false, nil
	}
	return text, true, nil
}

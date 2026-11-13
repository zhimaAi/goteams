// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package initialize

import (
	"context"
	"goteams/internal/app/goteams/define"
	"strconv"

	"github.com/go-redis/redis/v8"
	"github.com/zhimaAi/go_tools/logs"
)

// initRedis initializes the Redis connection.
func initRedis() {
	server := define.Config.Redis["host"] + `:` + define.Config.Redis["port"]
	database := 0
	if value := define.Config.Redis["db"]; value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			panic("invalid Redis database")
		}
		database = parsed
	}
	option := &redis.Options{
		Addr:     server,
		Password: define.Config.Redis["password"],
		DB:       database,
	}

	define.Redis = redis.NewClient(option)

	// Verify connection.
	if _, err := define.Redis.Ping(context.Background()).Result(); err != nil {
		logs.Error(err.Error())
		panic(`Redis Ping Error`)
	}

	logs.Info(`redis connected: %s`, server)
}

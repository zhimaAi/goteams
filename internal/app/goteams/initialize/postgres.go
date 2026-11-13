// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package initialize

import (
	"fmt"
	"goteams/internal/app/goteams/define"

	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// initPostgres initializes the PostgreSQL connection.
// Uses zhimaAi/go_tools/msql as the ORM (consistent with wensou).
func initPostgres() {
	conn := postgresConnString()

	// Register database connection.
	if err := msql.RegisterDataBase(define.Postgres, conn, define.Postgres); err != nil {
		logs.Error(err.Error())
		panic(`Postgres Register Error`)
	}

	// 授权安装会把完整Key作为SQL参数，私有化禁止参数展开调试日志。
	if err := msql.SetDebug(define.Postgres, false); err != nil {
		logs.Error(err.Error())
	}

	logs.Info(`postgres connected: %s:%s/%s`,
		define.Config.Postgres["host"],
		define.Config.Postgres["port"],
		define.Config.Postgres["dbname"])
}

func postgresConnString() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		define.Config.Postgres["host"], define.Config.Postgres["port"],
		define.Config.Postgres["user"], define.Config.Postgres["password"],
		define.Config.Postgres["dbname"], define.Config.Postgres["sslmode"])
}

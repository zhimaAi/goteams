// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package initialize

import (
	"goteams/internal/app/goteams/database"
	"goteams/internal/app/goteams/define"

	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// initMigrations runs the database schema migration before preset data initialization.
func initMigrations() {
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		logs.Error(err.Error())
		panic(`get migration database error`)
	}

	logs.Info(`start database migrations`)
	if err = database.RunMigrations(db); err != nil {
		logs.Error(err.Error())
		panic(`run database migrations error`)
	}
}

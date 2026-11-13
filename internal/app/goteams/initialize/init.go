// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package initialize

import (
	"flag"
	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"

	"github.com/zhimaAi/go_tools/logs"
)

const usage = `false is pro, true is dev, default run dev`

// Initialize initializes all components in order.
// Call chain: config → postgres → redis → gin
func Initialize() {
	// Parse runtime environment flag.
	flag.BoolVar(&define.IsDev, `IsDev`, true, usage)
	flag.Parse()

	// Enable terminal output in both dev and production to ensure docker logs shows all logs.
	logs.SetTerminal(true)
	if define.IsDev {
		logs.Info(`current run dev: dev`)
	} else {
		logs.Info(`current run dev: pro`)
	}

	// Initialize config.
	initConfig()
	logs.Info(`initialize config finish`)

	// Initialize PostgreSQL.
	initPostgres()
	logs.Info(`initialize postgres finish`)

	// Before business queries and seed data initialization, ensure the database schema is migrated to the latest version.
	initMigrations()
	logs.Info(`database migrations finish`)

	// Initialize Redis.
	initRedis()
	logs.Info(`initialize redis finish`)

	// Commercial license bootstrap: device identity, default admin and license snapshot.
	// Runs after migrations, before Gin and background workers; failures degrade to config_error.
	business.InitCommercialLicense()
	logs.Info(`initialize commercial license finish`)

	// Initialize Gin HTTP server (non-blocking, only creates the server instance).
	initGin()
	logs.Info(`initialize gin finish`)

	// Template creation reads the committed offline package; checking it here surfaces a missing
	// or unreadable package at boot instead of on the first "create from template" request. A
	// failure is logged by the check itself and does not stop the process.
	if err := business.CheckTemplatePackage(); err == nil {
		logs.Info(`initialize template package finish`)
	}

	// start the persistent automation executor only after migrations and the HTTP service are ready.
	business.StartAutomationWorkers()
	logs.Info(`initialize automation workers finish`)

	// 产出文档的异步上传 worker：接口只入队，这里负责把暂存文件写进对象存储。
	business.StartDocumentUploadWorkers()
	logs.Info(`initialize document upload workers finish`)

	// start the SLA aging evaluator after migrations and the HTTP service are ready.
	business.StartSLAWorkers()
	logs.Info(`initialize sla workers finish`)

	// 定时任务基础组件同样依赖迁移完成，且必须晚于注册 handler 的装配步骤。
	business.RegisterScheduledTask(define.SsoSyncTaskKey, business.SsoSyncHandler)
	business.RegisterScheduledTask(define.TenantRequirementTotalsTaskKey, business.TenantRequirementTotalsHandler)
	business.RegisterScheduledTask(business.LicenseUsageTaskKey, business.LicenseUsageReportHandler)
	if err := business.EnsureLicenseUsageReportSchedule(); err != nil {
		panic(err)
	}
	business.StartScheduler()
	logs.Info(`initialize scheduler finish`)

	// start the 6-hour license online re-verification loop (trial mandatory-online mode).
	business.StartLicenseRefreshLoop()
	logs.Info(`initialize license refresh loop finish`)
}

// InitScriptBase 是命令行脚本的最小启动：参数 → 配置 → 数据库 → 迁移 → Redis，
// 不启动 HTTP 服务与后台 worker。步骤顺序与 Initialize 一致。
func InitScriptBase() {
	flag.BoolVar(&define.IsDev, `IsDev`, true, usage)
	flag.Parse()
	logs.SetTerminal(true)
	initConfig()
	initPostgres()
	initMigrations()
	initRedis()
}

// Shutdown stops the background executors for graceful main-process exit.
func Shutdown() {
	business.StopScheduler()
	business.StopDocumentUploadWorkers()
	business.StopAutomationWorkers()
	business.StopSLAWorkers()
	business.StopLicenseRefreshLoop()
}

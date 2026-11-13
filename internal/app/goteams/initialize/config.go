// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package initialize

import (
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_storage"

	"github.com/zhimaAi/go_tools/logs"
)

// initConfig loads the configuration from the build-tagged source (external INI file, or
// the embedded template of the commercial offline build) and reads every section.
// applyDeploymentEnv afterwards lets GOTEAMS_* environment variables override single keys.
func initConfig() {
	config, err := openConfig()
	if err != nil {
		logs.Error(err.Error())
		panic(`read config file error`)
	}

	// Read each section configuration.
	define.Config.WebService, err = config.GetSection(`webservice`)
	if err != nil {
		logs.Error(err.Error())
		panic(`read config webservice error`)
	}
	applyDeploymentEnv(`webservice`, define.Config.WebService)

	define.Config.Session, err = config.GetSection(`session`)
	if err != nil {
		logs.Error(err.Error())
		panic(`read config session error`)
	}
	applyDeploymentEnv(`session`, define.Config.Session)

	define.Config.Redis, err = config.GetSection(`redis`)
	if err != nil {
		logs.Error(err.Error())
		panic(`read config redis error`)
	}
	applyDeploymentEnv(`redis`, define.Config.Redis)

	define.Config.Postgres, err = config.GetSection(`postgres`)
	if err != nil {
		logs.Error(err.Error())
		panic(`read config postgres error`)
	}
	applyDeploymentEnv(`postgres`, define.Config.Postgres)

	// oss-s3 is optional; when missing or failing to load, the system automatically falls back to local storage.
	ossS3, err := config.GetSection(`oss-s3`)
	if err != nil {
		logs.Error(`read config oss-s3 section error: %s`, err.Error())
	} else {
		applyDeploymentEnv(`oss-s3`, ossS3)
		define.Config.OssS3 = ossS3
		if ossS3[`enable`] == `true` {
			if validateErr := lib_storage.ValidateConfig(ossS3); validateErr != nil {
				logs.Error(`validate config oss-s3 section error: %s`, validateErr.Error())
				panic(`validate config oss-s3 section error`)
			}
		}
	}

	// Commercial builds never register cloud operations open APIs. Keep the map empty so
	// accidentally wiring an old handler cannot inherit a packaged shared secret.
	define.Config.OpenApi = map[string]string{}

	define.Config.Email, err = config.GetSection(`email`)
	if err != nil {
		logs.Error(err.Error())
		panic(`read config email error`)
	}
	applyDeploymentEnv(`email`, define.Config.Email)

	// client section is optional; defaults are used when missing.
	define.Config.Client, err = config.GetSection(`client`)
	if err != nil {
		logs.Error(`read config client section error: %s`, err.Error())
		define.Config.Client = map[string]string{}
	} else {
		applyDeploymentEnv(`client`, define.Config.Client)
	}

	// 未配置主密钥时保留明文存储；私有化包额外支持部署环境变量和格式校验。
	define.Config.SessionEncryption, err = config.GetSection(`session-encryption`)
	if err != nil {
		logs.Error(`read config session-encryption section error: %s`, err.Error())
		define.Config.SessionEncryption = map[string]string{}
	}
	if err = configureSessionEncryption(define.Config.SessionEncryption); err != nil {
		logs.Error(`validate config session-encryption section error: %s`, err.Error())
		panic(`validate config session-encryption section error`)
	}

	// template section is optional; defaults are used when missing.
	define.Config.Template, err = config.GetSection(`template`)
	if err != nil {
		logs.Error(`read config template section error: %s`, err.Error())
		define.Config.Template = map[string]string{}
	}

	// oauth section is optional; third-party scan-code login stays disabled when it is missing.
	define.Config.Oauth, err = config.GetSection(`oauth`)
	if err != nil {
		logs.Error(`read config oauth section error: %s`, err.Error())
		define.Config.Oauth = map[string]string{}
	}

	// dingtalk section is optional; the DingTalk entry stays hidden when it is missing.
	define.Config.Dingtalk, err = config.GetSection(`dingtalk`)
	if err != nil {
		logs.Error(`read config dingtalk section error: %s`, err.Error())
		define.Config.Dingtalk = map[string]string{}
	}

	// feishu section is optional; the Feishu entry stays hidden when it is missing.
	define.Config.Feishu, err = config.GetSection(`feishu`)
	if err != nil {
		logs.Error(`read config feishu section error: %s`, err.Error())
		define.Config.Feishu = map[string]string{}
	}

	// scheduler 段可选；缺失时定时任务组件按内置默认并发运行。
	define.Config.Scheduler, err = config.GetSection(`scheduler`)
	if err != nil {
		logs.Error(`read config scheduler section error: %s`, err.Error())
		define.Config.Scheduler = map[string]string{}
	}

	// sso 段可选；scope_id_key 缺失时入口地址为空，不影响服务启动（见 business/sso_scope_id.go）。
	define.Config.Sso, err = config.GetSection(`sso`)
	if err != nil {
		logs.Error(`read config sso section error: %s`, err.Error())
		define.Config.Sso = map[string]string{}
	}

	// Development INIs intentionally omit commercial-only sections. File builds use the
	// build-tagged defaults; embedded commercial builds return nil and remain fail-closed.
	define.AccountConfig, err = config.GetSection(`account`)
	if err != nil {
		define.AccountConfig = defaultAccountSection()
		if define.AccountConfig == nil {
			logs.Error(`read config account section error: %s`, err.Error())
			panic(`read config account error`)
		}
		logs.Info(`config account section missing; using development defaults`)
	}
	applyDeploymentEnv(`account`, define.AccountConfig)

	define.LicenseConfig, err = config.GetSection(`license`)
	if err != nil {
		define.LicenseConfig = defaultLicenseSection()
		if define.LicenseConfig == nil {
			logs.Error(`read config license section error: %s`, err.Error())
			panic(`read config license error`)
		}
		logs.Info(`config license section missing; using development defaults`)
	}
	applyDeploymentEnv(`license`, define.LicenseConfig)
}

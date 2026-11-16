// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package initialize

import (
	"fmt"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_storage"
	"os"
	"runtime"
	"strings"

	"github.com/Unknwon/goconfig"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/tool"
)

// initConfig loads the INI configuration file.
//   - Production: configs/goteams/config_pro.ini
//   - Development (container): use the DEV env var (e.g. goteams-dev) and take the segment after '-' to build the config,
//     e.g. DEV=goteams-dev → configs/goteams/config_dev.ini
//   - Local development (Windows/Mac): configs/goteams/config_loc.ini
func initConfig() {
	file := `configs/goteams/config_pro.ini`
	if define.IsDev {
		if tool.InArrayString(runtime.GOOS, []string{`windows`, `darwin`}) {
			define.IsLoc = true
			file = `configs/goteams/config_loc.ini`
		} else {
			// Container environment: take the segment after '-' from the DEV env var, e.g. goteams-dev → dev.
			dev := `dev`
			if devEnv := os.Getenv(`DEV`); devEnv != `` {
				parts := strings.Split(devEnv, `-`)
				if len(parts) >= 2 {
					dev = parts[1]
				}
			}
			file = fmt.Sprintf(`configs/goteams/config_%s.ini`, dev)
		}
	}

	logs.Info(`read config file: %s`, file)
	config, err := goconfig.LoadConfigFile(file)
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

	define.Config.Session, err = config.GetSection(`session`)
	if err != nil {
		logs.Error(err.Error())
		panic(`read config session error`)
	}

	define.Config.Redis, err = config.GetSection(`redis`)
	if err != nil {
		logs.Error(err.Error())
		panic(`read config redis error`)
	}

	define.Config.Postgres, err = config.GetSection(`postgres`)
	if err != nil {
		logs.Error(err.Error())
		panic(`read config postgres error`)
	}

	// oss-s3 is optional; when missing or failing to load, the system automatically falls back to local storage.
	ossS3, err := config.GetSection(`oss-s3`)
	if err != nil {
		logs.Error(`read config oss-s3 section error: %s`, err.Error())
	} else {
		define.Config.OssS3 = ossS3
		if ossS3[`enable`] == `true` {
			if validateErr := lib_storage.ValidateConfig(ossS3); validateErr != nil {
				logs.Error(`validate config oss-s3 section error: %s`, validateErr.Error())
				panic(`validate config oss-s3 section error`)
			}
		}
	}

	define.Config.OpenApi = map[string]string{}

	define.Config.Email, err = config.GetSection(`email`)
	if err != nil {
		logs.Error(err.Error())
		panic(`read config email error`)
	}

	// client section is optional; defaults are used when missing.
	define.Config.Client, err = config.GetSection(`client`)
	if err != nil {
		logs.Error(`read config client section error: %s`, err.Error())
		define.Config.Client = map[string]string{}
	}

	// session-encryption section is optional; when missing, session-log envelope
	// encryption is disabled and the cloud degrades to storing plaintext in OSS.
	define.Config.SessionEncryption, err = config.GetSection(`session-encryption`)
	if err != nil {
		logs.Error(`read config session-encryption section error: %s`, err.Error())
		define.Config.SessionEncryption = map[string]string{}
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

	// account 段可选；缺失时使用内置默认管理员，首次启动自动创建。
	define.AccountConfig, err = config.GetSection(`account`)
	if err != nil {
		define.AccountConfig = map[string]string{
			`admin_username`: `admin`,
			`admin_password`: `admin123456`,
			`admin_name`:     `Admin`,
		}
		logs.Info(`config account section missing; using development defaults`)
	}

	// license 段可选；缺失时使用内置默认授权配置。
	define.LicenseConfig, err = config.GetSection(`license`)
	if err != nil {
		define.LicenseConfig = map[string]string{
			`activation_url`:           ``,
			`request_timeout_seconds`:  `10`,
			`refresh_interval_seconds`: `21600`,
			`contact_qr_url`:           `/assets/customer-service-qr.png`,
			`instance_id_file`:         `data/license/instance_id`,
		}
		logs.Info(`config license section missing; using development defaults`)
	}
}

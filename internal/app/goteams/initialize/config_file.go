// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

//go:build !commercial_embed

package initialize

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"goteams/internal/app/goteams/define"

	"github.com/Unknwon/goconfig"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/tool"
)

// openConfig resolves the configuration source for development/standard builds:
//   - Production: configs/goteams/config_pro.ini
//   - Development (container): use the DEV env var (e.g. goteams-dev) and take the segment
//     after '-' to build the config, e.g. DEV=goteams-dev → configs/goteams/config_dev.ini
//   - Local development (Windows/Mac): configs/goteams/config_loc.ini
func openConfig() (*goconfig.ConfigFile, error) {
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
	return goconfig.LoadConfigFile(file)
}

// applyDeploymentEnv is a no-op outside the commercial embedded build: plain deployments
// read their full configuration from the INI file only.
func applyDeploymentEnv(section string, values map[string]string) {}

// configureSessionEncryption 保留普通构建原有的 INI 配置和加密行为。
func configureSessionEncryption(values map[string]string) error { return nil }

// defaultAccountSection returns the built-in development defaults for a dev INI without
// [account]; values mirror the environment-specific commercial embedded templates.
func defaultAccountSection() map[string]string {
	return map[string]string{
		`admin_username`: `admin`,
		`admin_password`: `admin123456`,
		`admin_name`:     `Admin`,
	}
}

// defaultLicenseSection returns the built-in development defaults for a dev INI without
// [license]; values mirror the environment-specific commercial embedded templates.
func defaultLicenseSection() map[string]string {
	return map[string]string{
		`activation_url`:           ``,
		`request_timeout_seconds`:  `10`,
		`refresh_interval_seconds`: `21600`,
		`contact_qr_url`:           `/assets/customer-service-qr.png`,
		`instance_id_file`:         `data/license/instance_id`,
	}
}

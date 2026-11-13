// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

//go:build commercial_embed

package initialize

import (
	"encoding/base64"
	"errors"
	"os"
	"strings"

	"goteams/configs/goteams"

	"github.com/Unknwon/goconfig"
	"github.com/zhimaAi/go_tools/logs"
)

// openConfig resolves the configuration source for the commercial offline build: the
// external config_pro.ini keeps priority when an operator mounts one (legacy installs),
// otherwise the INI embedded in the binary is used. Deployment values are never baked in;
// they arrive through GOTEAMS_* environment variables applied by applyDeploymentEnv.
func openConfig() (*goconfig.ConfigFile, error) {
	if _, err := os.Stat(externalConfigFile); err == nil {
		logs.Info(`read config file: %s (external overrides embedded)`, externalConfigFile)
		return goconfig.LoadConfigFile(externalConfigFile)
	}
	logs.Info(`read embedded config (commercial build, no config file on disk)`)
	return goconfig.LoadFromData(goteamsconfig.ConfigIni)
}

// deploymentEnvPrefix marks environment variables that override single INI keys.
const deploymentEnvPrefix = `GOTEAMS_`

// applyDeploymentEnv overlays GOTEAMS_<SECTION>_<KEY> environment variables (upper case,
// '-' and '.' normalized to '_') onto already-parsed sections. Only keys present in the
// embedded/external INI can be overridden, and empty variables are ignored.
func applyDeploymentEnv(section string, values map[string]string) {
	normalized := strings.NewReplacer(`-`, `_`, `.`, `_`).Replace(section)
	for key := range values {
		envName := deploymentEnvPrefix + strings.ToUpper(normalized+`_`+key)
		if v := strings.TrimSpace(os.Getenv(envName)); v != `` {
			values[key] = v
		}
	}
}

// configureSessionEncryption 仅对私有化包开放主密钥覆盖，填错时拒绝启动，避免静默落成明文。
func configureSessionEncryption(values map[string]string) error {
	// 旧外置 INI 可能没有该键，也要允许客户通过环境变量开启。
	if _, exists := values[`master_key`]; !exists {
		values[`master_key`] = ``
	}
	applyDeploymentEnv(`session-encryption`, values)
	raw := strings.TrimSpace(values[`master_key`])
	raw = strings.TrimSpace(strings.Trim(raw, `"`))
	if raw == `` {
		return nil
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return errors.New(`session-encryption master_key must be base64-encoded 32 bytes`)
	}
	return nil
}

// externalConfigFile is the optional operator-provided config checked before the embedded INI.
const externalConfigFile = `configs/goteams/config_pro.ini`

// defaultAccountSection and defaultLicenseSection return nil: the commercial build never
// falls back to implicit defaults, a missing [account]/[license] section aborts startup.
func defaultAccountSection() map[string]string { return nil }

// defaultLicenseSection returns nil, see defaultAccountSection.
func defaultLicenseSection() map[string]string { return nil }

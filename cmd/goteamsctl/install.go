// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	goteamsconfig "goteams/configs/goteams"
)

// cmdInstall performs a fresh commercial installation; re-running on an installed root is
// rejected with a hint to use upgrade instead of acting as an overwrite install.
func cmdInstall(root string, options ctlOptions) error {
	pkgDir, err := exeDir()
	if err != nil {
		return err
	}
	fmt.Println(`[precheck] verifying platform and package integrity ...`)
	if err = requirePlatform(); err != nil {
		return err
	}
	// Integrity comes first: the bundled Docker runtime and the payload compose file are only
	// usable once every whitelisted hash has been checked.
	manifest, err := verifyPackage(pkgDir)
	if err != nil {
		return err
	}
	fmt.Printf("[precheck] package %s (%s) verified\n", manifest.Name, manifest.Version)
	if err = checkDiskSpace(existingAncestor(root), 10); err != nil {
		return err
	}
	if err = checkMemory(2); err != nil {
		return err
	}
	if err = requireHostDMI(filepath.Join(pkgDir, `payload`, `docker`, `docker-compose.yml`)); err != nil {
		return err
	}
	if err = ensureDockerRuntime(pkgDir, manifest, options.assumeYes); err != nil {
		return err
	}

	// Interactive sizing (skipped with --yes). Only operator-facing values are prompted;
	// internal ports stay at their fixed defaults (still overridable through config/.env).
	webPort := 8080
	corePort := 8081
	pgPort := 15432
	redisPort := 16379
	timezone := `Asia/Shanghai`
	siteURL := defaultSiteURL(webPort)
	if !options.assumeYes {
		fmt.Println(`answer the prompts (Enter accepts the default):`)
		timezone = promptLine(`timezone`, timezone)
		webPort, err = promptPort(`web port (site)`, webPort)
		if err != nil {
			return err
		}
		siteURL, err = normalizeSiteURL(promptLine(`site url (address used in generated links)`, defaultSiteURL(webPort)))
		if err != nil {
			return err
		}
		if !confirm(`proceed with installation`, false) {
			return errors.New(`installation aborted`)
		}
	} else {
		for _, p := range []int{webPort, corePort, pgPort, redisPort} {
			if err = checkPortFree(p); err != nil {
				return fmt.Errorf(`--yes needs free ports: %w`, err)
			}
		}
	}

	// Generated secrets: instance_id is per-install; postgres/redis/open-api credentials are
	// fixed values committed in the selected config_embed_<environment>.ini and Compose
	// (overridable per install through the .env GOTEAMS_* keys).
	instanceID := randomHex(16)

	fmt.Println(`[dirs] creating install layout ...`)
	for _, dir := range []string{
		filepath.Join(root, `releases`),
		filepath.Join(root, `config`),
		filepath.Join(root, `data`, `uploads`),
		filepath.Join(root, `data`, `license`),
		filepath.Join(root, `logs`),
		filepath.Join(root, `backups`),
	} {
		if err = os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	fmt.Println(`[config] writing .env (application config is embedded in the binary) ...`)
	releaseDir := filepath.Join(root, `releases`, manifest.Version)
	envContent := renderEnvContent(root, releaseDir, map[string]string{
		`TIMEZONE`:      timezone,
		`WEB_PORT`:      itoa(webPort),
		`CORE_PORT`:     itoa(corePort),
		`POSTGRES_USER`: `goteams`,
		`POSTGRES_DB`:   `goteams`,
		`POSTGRES_PORT`: itoa(pgPort),
		`REDIS_PORT`:    itoa(redisPort),
		`SITE_URL`:      siteURL,
	})
	if err = writeFileAtomic(composeEnvPath(root), []byte(envContent), 0o600); err != nil {
		return err
	}
	if err = writeFileAtomic(filepath.Join(root, `data`, `license`, `instance_id`), []byte(instanceID), 0o600); err != nil {
		return err
	}

	// The payload must land before pulling: docker compose resolves -f releases/<version>/
	// docker/docker-compose.yml, which only exists once the payload has been copied.
	fmt.Println(`[release] copying payload ...`)
	if err = copyPackagePayload(pkgDir, root, manifest.Version); err != nil {
		return err
	}

	if err = loadOrPullImages(pkgDir, func() error {
		return runCompose(root, releaseDir, `pull`)
	}); err != nil {
		return fmt.Errorf(`images: %w`, err)
	}

	if err = switchCurrentSymlink(root, releaseDir); err != nil {
		return err
	}

	if err = saveState(root, map[string]any{
		`edition`:          `commercial`,
		`version`:          manifest.Version,
		`previous_version`: ``,
		`installed_at`:     time.Now().Unix(),
		`updated_at`:       time.Now().Unix(),
	}); err != nil {
		return err
	}

	fmt.Println(`[services] starting stack ...`)
	if err = startStack(root, releaseDir); err != nil {
		return err
	}

	fmt.Println(`[verify] final checks ...`)
	printStatusSummary(root)
	fmt.Println()
	fmt.Printf("installation complete: http://127.0.0.1:%d\n", webPort)
	fmt.Printf("site url (carried by generated links): %s\n", siteURL)
	fmt.Println(`change it anytime with: goteamsctl set-site-url <url>`)
	adminUser, adminPass := embeddedAdminAccount()
	fmt.Printf("default admin: %s / %s  (environment-embedded credential, forced password change on first login)\n", adminUser, adminPass)
	fmt.Println(`license: not activated — open the site to activate a free private experience license (default expiry 2099-01-01) or paste an offline key.`)
	fmt.Println(`storage defaults / 默认存储: local uploads, session encryption off / 本地存储，会话加密未开启。`)
	fmt.Printf("optional OSS and encryption / 可选 OSS 与加密: edit %s\n", composeEnvPath(root))
	fmt.Printf("configuration guide / 配置指南: %s (sections 3.3–3.5)\n", filepath.Join(root, `current`, `docs`, `license_install.md`))
	return nil
}

// cmdInstallDocker runs only the Docker runtime step of a fresh installation so an operator can
// prepare a host beforehand. Package integrity is verified first: the bundled binaries must never
// run out of an unverified tree.
func cmdInstallDocker(options ctlOptions) error {
	pkgDir, err := exeDir()
	if err != nil {
		return err
	}
	if err = requirePlatform(); err != nil {
		return err
	}
	manifest, err := verifyPackage(pkgDir)
	if err != nil {
		return err
	}
	fmt.Printf("[precheck] package %s (%s) verified\n", manifest.Name, manifest.Version)
	return ensureDockerRuntime(pkgDir, manifest, options.assumeYes)
}

// embeddedAdminAccount reads the default administrator credentials from the
// environment-specific INI embedded in goteamsctl at build time, so the installer
// prints the same credential the goteams binary will seed on first start.
func embeddedAdminAccount() (username, password string) {
	for _, entry := range parseIniEntries(string(goteamsconfig.ConfigIni)) {
		if entry.Section != `account` {
			continue
		}
		switch entry.Key {
		case `admin_username`:
			username = entry.Value
		case `admin_password`:
			password = entry.Value
		}
	}
	return username, password
}

// existingAncestor returns the closest existing ancestor of path (disk prechecks need an
// existing directory because the install root is created later).
func existingAncestor(path string) string {
	target := path
	for {
		if _, err := os.Stat(target); err == nil {
			return target
		}
		parent := filepath.Dir(target)
		if parent == target {
			return path
		}
		target = parent
	}
}

// copyPackagePayload copies whitelisted payload files into releases/<version> and extracts
// the web archive. Only manifest-listed files are touched.
func copyPackagePayload(pkgDir, root, version string) error {
	releaseDir := filepath.Join(root, `releases`, version)
	for _, rel := range []string{
		`payload/bin/goteams`,
		`payload/docker/docker-compose.yml`,
		`payload/docker/nginx/web.conf`,
		`payload/docker/start-web.sh`,
		`payload/docs/license_install.md`,
	} {
		src := filepath.Join(pkgDir, filepath.FromSlash(rel))
		dst := filepath.Join(releaseDir, filepath.FromSlash(strings.TrimPrefix(rel, `payload/`)))
		if err := copyFile(src, dst); err != nil {
			return err
		}
	}
	if err := os.Chmod(filepath.Join(releaseDir, `bin`, `goteams`), 0o755); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Join(releaseDir, `docker`, `start-web.sh`), 0o755); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Join(releaseDir, `docs`, `license_install.md`), 0o644); err != nil {
		return err
	}
	return extractTarGzSafe(filepath.Join(pkgDir, `payload`, `web`, `dist.tar.gz`), filepath.Join(releaseDir, `web`, `dist`))
}

// copyFile copies one file creating parent directories.
func copyFile(src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, raw, 0o755)
}

// switchCurrentSymlink atomically repoints <root>/current at releaseDir.
func switchCurrentSymlink(root, releaseDir string) error {
	link := filepath.Join(root, `current`)
	tmp := filepath.Join(root, `.current-new`)
	_ = os.Remove(tmp)
	if err := os.Symlink(releaseDir, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, link)
}

// renderEnvContent builds the installer-maintained .env: compose plumbing (root, release,
// ports, database identifiers) plus the optional public site address. Application defaults
// still live in the environment-specific binary; the GOTEAMS_* keys here only override
// single embedded INI values (empty values are ignored by applyDeploymentEnv). The license
// activation URL stays embedded and is not written here.
func renderEnvContent(root, releaseDir string, values map[string]string) string {
	var b strings.Builder
	b.WriteString(`# GoTeams commercial runtime env (written by goteamsctl, mode 0600).
# Application defaults (license activation URL, credentials) are embedded in the
# environment-specific goteams binary; GOTEAMS_* keys below override single INI values.

# install root and current release (maintained by goteamsctl, do not edit)
GOTEAMS_ROOT=` + root + `
GOTEAMS_RELEASE=` + releaseDir + `

# compose plumbing ports and database identifiers
`)
	for _, key := range []string{
		`TIMEZONE`, `WEB_PORT`, `CORE_PORT`,
		`POSTGRES_USER`, `POSTGRES_DB`, `POSTGRES_PORT`, `REDIS_PORT`,
	} {
		b.WriteString(key + `=` + values[key] + "\n")
	}
	// Public site address used in server-generated links ([webservice] site_url). Skipped when
	// empty so an upgrade never appends a blank key; change it with "goteamsctl set-site-url".
	if siteURL := strings.TrimSpace(values[`SITE_URL`]); siteURL != `` {
		b.WriteString("\n# public site address carried by generated links ([webservice] site_url)\n")
		b.WriteString(envSiteURLKey + `=` + siteURL + "\n")
	}
	// 示例始终保持注释，不自动开启 OSS，也不生成、写入或覆盖客户主密钥。
	b.WriteString(`
# Optional object storage / 可选对象存储（默认关闭，启用前填齐客户自己的配置）
# GOTEAMS_OSS_S3_ENABLE=true
# GOTEAMS_OSS_S3_ENDPOINT=oss.example.com
# GOTEAMS_OSS_S3_ACCESS_KEY=
# GOTEAMS_OSS_S3_SECRET_KEY=
# GOTEAMS_OSS_S3_BUCKET=goteams
# GOTEAMS_OSS_S3_USE_SSL=true
# GOTEAMS_OSS_S3_URL_STYLE=path
# GOTEAMS_OSS_S3_CUSTOM_DOMAIN=
# GOTEAMS_OSS_S3_PUBLIC_ENDPOINT=

# Optional session encryption / 可选会话正文存储加密（默认不配置）
# Generate a separate key per instance / 每个实例自行生成: openssl rand -base64 32
# Back up the key; never replace or delete it / 主密钥须备份，启用后勿随意更换或删除
# GOTEAMS_SESSION_ENCRYPTION_MASTER_KEY=

# Recreate goteams after editing / 修改后重建后端，docker compose restart 不会重读环境变量
`)
	b.WriteString("# Guide / 配置指南: " + filepath.Join(root, `current`, `docs`, `license_install.md`) + " (3.3–3.5)\n")
	return b.String()
}

// itoa is a small int->string helper keeping template maps tidy.
func itoa(v int) string {
	return fmt.Sprintf(`%d`, v)
}

// exeDir returns the directory holding the goteamsctl executable (the package root).
func exeDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return ``, err
	}
	return filepath.Dir(exe), nil
}

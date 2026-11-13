// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// cmdUpgrade performs the in-place upgrade: verify → prepare new release (no downtime) →
// backup → switch artifacts → start → verify. Customer config/data/license never move.
func cmdUpgrade(root string) error {
	state, err := loadState(root)
	if err != nil {
		return fmt.Errorf(`read state.json: %w`, err)
	}
	oldVersion := str(state, `version`)
	if oldVersion == `` {
		return errors.New(`state.json has no version`)
	}

	pkgDir, err := exeDir()
	if err != nil {
		return err
	}
	fmt.Println(`[precheck] verifying package integrity ...`)
	if err = requirePlatform(); err != nil {
		return err
	}
	if err = requireDocker(); err != nil {
		return err
	}
	manifest, err := verifyPackage(pkgDir)
	if err != nil {
		return err
	}
	if cmp, err := compareSemver(manifest.Version, oldVersion); err != nil || cmp <= 0 {
		return fmt.Errorf(`upgrade requires a version greater than %s (got %s)`, oldVersion, manifest.Version)
	}
	if manifest.MinUpgradeVersion != `` {
		if cmp, err := compareSemver(oldVersion, manifest.MinUpgradeVersion); err != nil || cmp < 0 {
			return fmt.Errorf(`this package can only upgrade from %s or later (installed %s)`, manifest.MinUpgradeVersion, oldVersion)
		}
	}

	oldRelease := filepath.Join(root, `releases`, oldVersion)
	newRelease := filepath.Join(root, `releases`, manifest.Version)
	if _, err = os.Stat(newRelease); err == nil {
		return fmt.Errorf(`release %s already exists on disk`, manifest.Version)
	}

	// Old identity snapshot for the post-upgrade device-code comparison (best effort).
	oldStatus, statusErr := fetchLicenseStatus(root)
	oldDeviceCode := ``
	if statusErr == nil {
		oldDeviceCode = str(oldStatus, `device_code`)
	} else {
		fmt.Println(`[warn] old service unreachable, skipping device-code comparison`)
	}

	fmt.Println(`[release] copying payload and merging configuration ...`)
	if err = copyPackagePayload(pkgDir, root, manifest.Version); err != nil {
		return err
	}
	if err = mergeConfiguration(root, newRelease, manifest.Version); err != nil {
		return err
	}

	// Compose precheck with a temporary env pointing at the new release (no downtime yet).
	precheckEnv, err := composeReleaseEnvPath(root, newRelease)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(precheckEnv) }()
	precheckArgs := []string{`compose`, `--project-name`, composeProjectName, `--env-file`, precheckEnv,
		`-f`, filepath.Join(newRelease, `docker`, `docker-compose.yml`), `config`, `--quiet`}
	if err = runCommand(`docker`, precheckArgs...); err != nil {
		return fmt.Errorf(`compose precheck failed: %w`, err)
	}

	fmt.Println(`[images] preparing images for the new release ...`)
	if err = loadOrPullImages(pkgDir, func() error {
		pullArgs := append([]string{`compose`, `--project-name`, composeProjectName, `--env-file`, precheckEnv,
			`-f`, filepath.Join(newRelease, `docker`, `docker-compose.yml`)}, `pull`)
		return runCommand(`docker`, pullArgs...)
	}); err != nil {
		return fmt.Errorf(`images: %w`, err)
	}

	fmt.Println(`[services] stopping web + goteams (database stays up for the backup) ...`)
	if err = runCompose(root, oldRelease, `stop`, `web`, `goteams`); err != nil {
		return err
	}

	backupDir := filepath.Join(root, `backups`, time.Now().Format(`20060102150405`))
	fmt.Printf("[backup] pg_dump into %s ...\n", backupDir)
	if err = backupDatabase(root, oldRelease, backupDir); err != nil {
		return err
	}

	fmt.Println(`[services] stopping remaining services ...`)
	if err = runCompose(root, oldRelease, `stop`); err != nil {
		return err
	}

	// Switch artifacts: current symlink + .env GOTEAMS_RELEASE. Config/data/license untouched.
	if err = switchCurrentSymlink(root, newRelease); err != nil {
		return err
	}
	if err = updateEnvRelease(composeEnvPath(root), newRelease); err != nil {
		return err
	}

	fmt.Println(`[services] starting upgraded stack ...`)
	if err = startStack(root, newRelease); err != nil {
		// A failed start never switches back automatically; see upgradeStartFailure.
		return upgradeStartFailure(root, oldRelease, backupDir, err)
	}

	if err = verifyAfterUpgrade(root, oldDeviceCode); err != nil {
		return err
	}

	if err = saveState(root, map[string]any{
		`edition`:          `commercial`,
		`version`:          manifest.Version,
		`previous_version`: oldVersion,
		`installed_at`:     state[`installed_at`],
		`updated_at`:       time.Now().Unix(),
	}); err != nil {
		return err
	}
	printStatusSummary(root)
	fmt.Printf("upgrade complete: %s -> %s (previous release kept at %s)\n", oldVersion, manifest.Version, oldRelease)
	return nil
}

// upgradeStartFailure reports a failed start. The upgrade never rolls back automatically: the
// new release stays active, the previous release and the database backup are both kept, and the
// operator decides the next step. The migration version is only reported best-effort.
func upgradeStartFailure(root, oldRelease, backupDir string, cause error) error {
	fmt.Fprintln(os.Stderr, `error: upgraded stack failed to start: `+cause.Error())
	if newGoose, gooseErr := fetchGooseVersion(root); gooseErr == nil {
		fmt.Fprintf(os.Stderr, "[info] current database migration version: goose %d\n", newGoose)
	} else {
		fmt.Fprintln(os.Stderr, `[info] cannot read the database migration version now: `+gooseErr.Error())
	}
	fmt.Fprintf(os.Stderr, `[info] automatic rollback is disabled by design; the new release stays active and no data was touched.
[info] previous release kept at: %s
[info] database backup:         %s
[info] next: inspect "goteamsctl logs", then fix and run "goteamsctl start"; or restore the backup
[info]       above and only then switch back with "goteamsctl rollback-artifacts %s".
`, oldRelease, backupDir, filepath.Base(oldRelease))
	return fmt.Errorf(`upgraded stack failed to start (no automatic rollback); previous release %s and backup %s kept for manual recovery`, filepath.Base(oldRelease), backupDir)
}

// verifyAfterUpgrade checks device-code stability (must be identical); the backend remains
// the authority on license and quota consistency.
func verifyAfterUpgrade(root, oldDeviceCode string) error {
	status, err := fetchLicenseStatus(root)
	if err != nil {
		return fmt.Errorf(`post-upgrade license status unavailable: %w`, err)
	}
	if oldDeviceCode != `` {
		if newCode := str(status, `device_code`); newCode != oldDeviceCode {
			return fmt.Errorf(`device code changed after upgrade (%s -> %s); upgrade is invalid`, oldDeviceCode, newCode)
		}
	}
	return nil
}

// backupDatabase dumps the database into backupDir and refuses to continue on an empty dump.
func backupDatabase(root, releaseDir, backupDir string) error {
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return err
	}
	env, err := readEnvFile(composeEnvPath(root))
	if err != nil {
		return err
	}
	dumpPath := filepath.Join(backupDir, `pg_dump.sql`)
	dumpFile, err := os.OpenFile(dumpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	dumpArgs := []string{`compose`, `--project-name`, composeProjectName, `--env-file`, composeEnvPath(root),
		`-f`, filepath.Join(releaseDir, `docker`, `docker-compose.yml`),
		`exec`, `-T`, `postgres`, `pg_dump`, `-U`, env[`POSTGRES_USER`], env[`POSTGRES_DB`]}
	cmd := execCommand(`docker`, dumpArgs...)
	cmd.Stdout = dumpFile
	runErr := cmd.Run()
	closeErr := dumpFile.Close()
	if runErr != nil {
		return fmt.Errorf(`pg_dump failed: %w`, runErr)
	}
	if closeErr != nil {
		return closeErr
	}
	info, err := os.Stat(dumpPath)
	if err != nil {
		return err
	}
	if info.Size() < 1024 {
		return fmt.Errorf(`database backup suspiciously small (%d bytes), refusing to continue`, info.Size())
	}
	return nil
}

// mergeConfiguration upgrades the host-side files. A legacy install still carrying a
// config_pro.ini has every key migrated into GOTEAMS_* .env overrides (customer values
// are never lost) and the INI is renamed out of the way; the .env itself is merged with
// the current plumbing keys without overwriting existing values.
func mergeConfiguration(root, newRelease, version string) error {
	if err := migrateLegacyIni(root, version); err != nil {
		return err
	}

	envPath := composeEnvPath(root)
	oldEnvRaw, err := os.ReadFile(envPath)
	if err != nil {
		return err
	}
	oldEnvMap, err := readEnvFile(envPath)
	if err != nil {
		return err
	}
	renderedEnv := renderEnvContent(root, newRelease, map[string]string{
		`TIMEZONE`:      oldEnvMap[`TIMEZONE`],
		`WEB_PORT`:      oldEnvMap[`WEB_PORT`],
		`CORE_PORT`:     oldEnvMap[`CORE_PORT`],
		`POSTGRES_USER`: orDefault(oldEnvMap[`POSTGRES_USER`], `goteams`),
		`POSTGRES_DB`:   orDefault(oldEnvMap[`POSTGRES_DB`], `goteams`),
		`POSTGRES_PORT`: orDefault(oldEnvMap[`POSTGRES_PORT`], `15432`),
		`REDIS_PORT`:    orDefault(oldEnvMap[`REDIS_PORT`], `16379`),
	})
	mergedEnv, err := mergeEnvIntoOld(string(oldEnvRaw), renderedEnv, version)
	if err != nil {
		return err
	}
	return mustWriteFile(envPath, mergedEnv, 0o600)
}

// migrateLegacyIni moves a pre-embedded-config install forward: every non-empty key of the
// on-disk config_pro.ini becomes a GOTEAMS_<SECTION>_<KEY> override appended to .env (keys
// already present are kept), then the INI is renamed aside. A missing INI (current layout)
// is not an error.
func migrateLegacyIni(root, version string) error {
	iniPath := filepath.Join(root, `config`, `configs`, `goteams`, `config_pro.ini`)
	rawIni, err := os.ReadFile(iniPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	envPath := composeEnvPath(root)
	oldEnv, err := readEnvFile(envPath)
	if err != nil {
		return err
	}
	normalize := strings.NewReplacer(`-`, `_`, `.`, `_`)
	var overrides strings.Builder
	for _, e := range parseIniEntries(string(rawIni)) {
		if e.Section == `` || e.Key == `` || strings.TrimSpace(e.Value) == `` {
			continue
		}
		name := `GOTEAMS_` + strings.ToUpper(normalize.Replace(e.Section+`_`+e.Key))
		if _, exists := oldEnv[name]; exists {
			continue
		}
		overrides.WriteString(name + `=` + e.Value + "\n")
	}
	if overrides.Len() > 0 {
		rawEnv, readErr := os.ReadFile(envPath)
		if readErr != nil {
			return readErr
		}
		merged, mergeErr := mergeEnvIntoOld(string(rawEnv), overrides.String(), version)
		if mergeErr != nil {
			return mergeErr
		}
		if err = mustWriteFile(envPath, merged, 0o600); err != nil {
			return err
		}
	}
	// POSIX rename replaces an existing leftover from a previous migration; the content is
	// the same file, so no customer data can be lost here.
	return os.Rename(iniPath, iniPath+`.migrated`)
}

// updateEnvRelease rewrites only the GOTEAMS_RELEASE line of the .env file.
func updateEnvRelease(envPath, releaseDir string) error {
	raw, err := os.ReadFile(envPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), `GOTEAMS_RELEASE=`) {
			lines[i] = `GOTEAMS_RELEASE=` + releaseDir
		}
	}
	return writeFileAtomic(envPath, []byte(strings.Join(lines, "\n")), 0o600)
}

// cmdRollbackArtifacts repoints current at an existing release and restarts; nothing is
// deleted and no database operation is performed.
func cmdRollbackArtifacts(root, version string, options ctlOptions) error {
	if !validVersionName(version) {
		return fmt.Errorf(`invalid version name %q`, version)
	}
	releaseDir := filepath.Join(root, `releases`, version)
	if _, err := os.Stat(filepath.Join(releaseDir, `docker`, `docker-compose.yml`)); err != nil {
		return fmt.Errorf(`release %s is not a valid artifact directory`, version)
	}
	if !confirm(`switch artifacts to `+version+` and restart services?`, options.assumeYes) {
		return errors.New(`aborted`)
	}
	if err := switchCurrentSymlink(root, releaseDir); err != nil {
		return err
	}
	if err := updateEnvRelease(composeEnvPath(root), releaseDir); err != nil {
		return err
	}
	if err := startStack(root, releaseDir); err != nil {
		return err
	}
	state, err := loadState(root)
	if err == nil {
		state[`previous_version`] = str(state, `version`)
		state[`version`] = version
		state[`updated_at`] = time.Now().Unix()
		_ = saveState(root, state)
	}
	fmt.Println(`artifacts switched to ` + version)
	return nil
}

// validVersionName only accepts release directory names (defense against path traversal).
func validVersionName(version string) bool {
	if version == `` || strings.Contains(version, `/`) || strings.Contains(version, `\`) || strings.Contains(version, `..`) {
		return false
	}
	return strings.HasPrefix(version, `v`) && len(version) < 64
}

// str reads a string key from a status map.
func str(m map[string]any, key string) string {
	if m == nil {
		return ``
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ``
}

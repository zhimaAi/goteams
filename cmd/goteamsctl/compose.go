// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// composeEnvPath returns the fixed .env path of an installed instance.
func composeEnvPath(root string) string {
	return filepath.Join(root, `config`, `.env`)
}

// composeReleaseEnvPath writes a temporary copy of .env with GOTEAMS_RELEASE overridden to
// releaseDir; used for prechecks so the live .env keeps pointing at the old release.
func composeReleaseEnvPath(root, releaseDir string) (string, error) {
	raw, err := os.ReadFile(composeEnvPath(root))
	if err != nil {
		return ``, err
	}
	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), `GOTEAMS_RELEASE=`) {
			lines[i] = `GOTEAMS_RELEASE=` + releaseDir
		}
	}
	tmp, err := os.CreateTemp(``, `.env-precheck-*`)
	if err != nil {
		return ``, err
	}
	tmpName := tmp.Name()
	if _, err = tmp.WriteString(strings.Join(lines, "\n")); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return ``, err
	}
	if err = tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return ``, err
	}
	return tmpName, nil
}

// runCompose executes a docker compose subcommand with the mandatory project name and env
// file, streaming output, checking the exit code and forwarding SIGINT/SIGTERM.
func runCompose(root, releaseDir string, args ...string) error {
	full := []string{`compose`, `--project-name`, composeProjectName, `--env-file`, composeEnvPath(root), `-f`, filepath.Join(releaseDir, `docker`, `docker-compose.yml`)}
	full = append(full, args...)
	return runCommand(`docker`, full...)
}

// offlineImageBundle returns the path of the image archive shipped in offline packages, or
// an empty string for the standard (online) variant where the file does not exist.
func offlineImageBundle(pkgDir string) string {
	p := filepath.Join(pkgDir, `payload`, `images`, `images.tar.gz`)
	if _, err := os.Stat(p); err != nil {
		return ``
	}
	return p
}

// loadOrPullImages provisions the compose images. Offline packages carry a docker-save
// archive already integrity-verified by verifyPackage, loaded with docker load (no network);
// standard packages fall back to the caller's registry pull.
func loadOrPullImages(pkgDir string, pullFn func() error) error {
	if bundle := offlineImageBundle(pkgDir); bundle != `` {
		fmt.Println(`[images] loading bundled docker images (offline package) ...`)
		return runCommand(`docker`, `load`, `-i`, bundle)
	}
	return pullFn()
}

// runCommand executes a subprocess with streamed stdio, exit code checking and signal
// forwarding so Ctrl+C / SIGTERM reach the child instead of killing the CLI silently.
func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		return err
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	for {
		select {
		case sig := <-sigCh:
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		case err := <-done:
			signal.Stop(sigCh)
			if err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					return fmt.Errorf(`%s exited with code %d`, name, exitErr.ExitCode())
				}
				return err
			}
			return nil
		}
	}
}

// runCommandOutput runs a subprocess capturing stdout (used for psql/pg_dump plumbing).
func runCommandOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	return string(out), err
}

// execCommand builds a raw exec.Cmd for callers needing custom stdio plumbing.
func execCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

// requireDocker verifies the Docker Engine and Compose V2 are usable.
func requireDocker() error {
	if err := exec.Command(`docker`, `version`, `--format`, `{{.Server.Version}}`).Run(); err != nil {
		return errors.New(`docker engine is not reachable (installed and running?)`)
	}
	if err := exec.Command(`docker`, `compose`, `version`, `--short`).Run(); err != nil {
		return errors.New(`docker compose v2 is required`)
	}
	return nil
}

// requirePlatform enforces the supported runtime: linux amd64.
func requirePlatform() error {
	if runtime.GOOS != `linux` || runtime.GOARCH != `amd64` {
		return fmt.Errorf(`commercial package requires linux/amd64, running on %s/%s`, runtime.GOOS, runtime.GOARCH)
	}
	return nil
}

// requireHostDMI refuses to run on hosts without DMI (WSL and similar non-native Linux
// environments): the commercial compose mounts /sys/class/dmi/id read-only for the device
// code, and such containers fail with an obscure docker mount error. A compose file without
// the mount (local test workaround) skips the check.
func requireHostDMI(composeFile string) error {
	if _, err := os.Stat(`/sys/class/dmi/id`); err == nil {
		return nil
	}
	if raw, err := os.ReadFile(composeFile); err == nil && !strings.Contains(string(raw), `/sys/class/dmi/id`) {
		return nil
	}
	return errors.New(`this host does not expose /sys/class/dmi (hardware identity): WSL and other non-native Linux environments are not supported; install on a real Linux server or virtual machine`)
}

// checkPortFree verifies no process is listening on the given TCP port.
func checkPortFree(port int) error {
	ln, err := net.Listen(`tcp`, `:`+strconv.Itoa(port))
	if err != nil {
		return fmt.Errorf(`port %d is already in use`, port)
	}
	return ln.Close()
}

// promptPort asks for a free TCP port with a default; retries on conflicts.
func promptPort(label string, def int) (int, error) {
	for attempt := 0; attempt < 5; attempt++ {
		line := promptLine(label, strconv.Itoa(def))
		port, err := strconv.Atoi(line)
		if err != nil || port < 1 || port > 65535 {
			fmt.Println(`invalid port, please retry`)
			continue
		}
		if err = checkPortFree(port); err != nil {
			fmt.Println(err.Error() + `, please retry`)
			continue
		}
		return port, nil
	}
	return 0, errors.New(`too many invalid port attempts`)
}

// serviceCoreURL builds the local backend URL from the .env CORE_PORT.
func serviceCoreURL(root string) (string, error) {
	env, err := readEnvFile(composeEnvPath(root))
	if err != nil {
		return ``, err
	}
	port := strings.TrimSpace(env[`CORE_PORT`])
	if port == `` {
		return ``, errors.New(`CORE_PORT missing in .env`)
	}
	return `http://127.0.0.1:` + port, nil
}

// serviceWebURL builds the local web URL from the .env WEB_PORT.
func serviceWebURL(root string) (string, error) {
	env, err := readEnvFile(composeEnvPath(root))
	if err != nil {
		return ``, err
	}
	port := strings.TrimSpace(env[`WEB_PORT`])
	if port == `` {
		return ``, errors.New(`WEB_PORT missing in .env`)
	}
	return `http://127.0.0.1:` + port, nil
}

// readEnvFile parses a KEY=VALUE .env file.
func readEnvFile(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	env := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == `` || strings.HasPrefix(line, `#`) {
			continue
		}
		parts := strings.SplitN(line, `=`, 2)
		if len(parts) != 2 {
			continue
		}
		env[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return env, nil
}

// startStack brings the stack up in the documented order: databases first (wait healthy),
// then goteams (runs migrations), then web once the backend answers.
func startStack(root, releaseDir string) error {
	if err := requireHostDMI(filepath.Join(releaseDir, `docker`, `docker-compose.yml`)); err != nil {
		return err
	}
	fmt.Println(`[1/4] starting postgres + redis (waiting until healthy) ...`)
	if err := runCompose(root, releaseDir, `up`, `-d`, `--wait`, `postgres`, `redis`); err != nil {
		return fmt.Errorf(`start databases: %w`, err)
	}
	fmt.Println(`[2/4] starting goteams (runs database migrations) ...`)
	if err := runCompose(root, releaseDir, `up`, `-d`, `goteams`); err != nil {
		return fmt.Errorf(`start goteams: %w`, err)
	}
	coreURL, err := serviceCoreURL(root)
	if err != nil {
		return err
	}
	if err = waitHTTPReady(coreURL+`/api/ping`, 5*time.Minute); err != nil {
		return fmt.Errorf(`goteams did not become ready: %w`, err)
	}
	fmt.Println(`[3/4] goteams is ready, starting web ...`)
	if err = runCompose(root, releaseDir, `up`, `-d`, `web`); err != nil {
		return fmt.Errorf(`start web: %w`, err)
	}
	webURL, err := serviceWebURL(root)
	if err != nil {
		return err
	}
	if err = waitHTTPReady(webURL, 2*time.Minute); err != nil {
		return fmt.Errorf(`web did not become ready: %w`, err)
	}
	fmt.Println(`[4/4] web is ready`)
	return nil
}

// fetchLicenseStatus returns the /api/system/license/status data map (best effort).
func fetchLicenseStatus(root string) (map[string]any, error) {
	coreURL, err := serviceCoreURL(root)
	if err != nil {
		return nil, err
	}
	return httpGetJSON(coreURL+`/api/system/license/status`, 10*time.Second)
}

// fetchGooseVersion returns max(goose_db_version.version_id) via psql (0 when unknown).
func fetchGooseVersion(root string) (int64, error) {
	env, err := readEnvFile(composeEnvPath(root))
	if err != nil {
		return 0, err
	}
	releaseDir := strings.TrimSpace(env[`GOTEAMS_RELEASE`])
	if releaseDir == `` {
		return 0, errors.New(`GOTEAMS_RELEASE missing in .env`)
	}
	out, err := runCommandOutput(`docker`, `compose`, `--project-name`, composeProjectName,
		`--env-file`, composeEnvPath(root), `-f`, filepath.Join(releaseDir, `docker`, `docker-compose.yml`),
		`exec`, `-T`, `postgres`, `psql`, `-U`, env[`POSTGRES_USER`], `-d`, env[`POSTGRES_DB`],
		`-tAc`, `SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version`)
	if err != nil {
		return 0, err
	}
	var v int64
	if _, err = fmt.Sscanf(strings.TrimSpace(out), `%d`, &v); err != nil {
		return 0, fmt.Errorf(`unexpected goose version output %q`, strings.TrimSpace(out))
	}
	return v, nil
}

// printStatusSummary renders the license status JSON for operators.
func printStatusSummary(root string) {
	status, err := fetchLicenseStatus(root)
	if err != nil {
		fmt.Println(`license status unavailable: ` + err.Error())
		return
	}
	raw, _ := jsonIndent(status)
	fmt.Println(`license status: ` + raw)
}

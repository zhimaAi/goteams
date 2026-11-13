// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// dockerBinDir receives the static engine binaries (never /usr/bin, which belongs to
	// the host package manager).
	dockerBinDir = `/usr/local/bin`
	// dockerComposePluginDir is searched by the docker CLI for v2 plugins.
	dockerComposePluginDir = `/usr/local/lib/docker/cli-plugins`
	// systemdUnitDir is the administrator-owned unit directory.
	systemdUnitDir = `/etc/systemd/system`
	// dockerRuntimeWait bounds how long the engine is polled after the services start.
	dockerRuntimeWait = 3 * time.Minute
)

// dockerProbe describes what the host already provides.
type dockerProbe struct {
	binaryPath  string
	engineOK    bool
	engineVer   string
	composeOK   bool
	composeVer  string
	unitPresent bool
}

// ensureDockerRuntime makes the host able to run the stack before a fresh installation:
// an existing Docker is used untouched, a Compose-plugin-only gap is filled additively, and
// a host with no Docker at all gets the bundled runtime. Any half-installed state fails
// closed with the manual commands to repair it. Only install and the explicit install-docker
// subcommand call this, so start/stop/status/upgrade can never change a host's Docker.
func ensureDockerRuntime(pkgDir string, manifest *packageManifest, assumeYes bool) error {
	probe := probeDocker()
	if probe.engineOK && probe.composeOK {
		fmt.Printf(`[docker] engine %s + compose %s detected; bundled runtime install skipped`+"\n", probe.engineVer, probe.composeVer)
		return nil
	}

	runtime := newDockerRuntimePaths(pkgDir, manifest)

	if probe.engineOK && !probe.composeOK {
		// Healthy engine, missing CLI plugin: installing one file under
		// /usr/local/lib/docker/cli-plugins is additive and cannot touch the distro engine.
		if !fileExists(runtime.composePlugin) {
			return runtimeUnavailable(`docker compose v2 is missing and the package carries no Compose plugin; install the plugin so that "docker compose version" works`)
		}
		if err := requireRootForHostWrite(); err != nil {
			return err
		}
		if !confirm(`install the bundled Compose `+runtime.composeVersion+` plugin into `+dockerComposePluginDir, assumeYes) {
			return runtimeUnavailable(`the operator declined the bundled Compose plugin install`)
		}
		if err := installComposePlugin(runtime.composePlugin); err != nil {
			return err
		}
		if err := verifyComposePlugin(); err != nil {
			return err
		}
		fmt.Printf("[docker] Compose %s plugin installed\n", runtime.composeVersion)
		return nil
	}

	if probe.binaryPath != `` || probe.unitPresent {
		return dockerBrokenGuidance(probe)
	}
	if runtime.staticTarball == `` || !fileExists(runtime.composePlugin) {
		return runtimeUnavailable(`docker is not installed and this package carries no bundled runtime under ` + dockerRuntimeRelDir)
	}
	return installBundledDockerRuntime(runtime, assumeYes)
}

// installBundledDockerRuntime installs the packaged engine, plugin and units, then starts
// the services and waits until the daemon answers.
func installBundledDockerRuntime(runtime dockerRuntimePaths, assumeYes bool) error {
	if err := requireDockerInstallPrerequisites(); err != nil {
		return err
	}
	fmt.Printf("[docker] no Docker on this host; the package bundles Docker Engine %s + Compose %s\n", runtime.engineVersion, runtime.composeVersion)
	if !confirm(`install the bundled Docker runtime into `+dockerBinDir+`, `+dockerComposePluginDir+` and `+systemdUnitDir, assumeYes) {
		return runtimeUnavailable(`docker is required and the bundled runtime install was declined; install Docker manually and re-run`)
	}

	fmt.Println(`[docker] installing engine binaries ...`)
	if err := installEngineBinaries(runtime.staticTarball); err != nil {
		return err
	}
	if err := installComposePlugin(runtime.composePlugin); err != nil {
		return err
	}
	fmt.Println(`[docker] installing systemd units ...`)
	if err := installSystemdUnits(runtime.pkgDir); err != nil {
		return err
	}
	fmt.Println(`[docker] starting containerd + docker ...`)
	if err := startDockerServices(); err != nil {
		return dockerStartGuidance(err)
	}
	if err := waitForDockerEngine(dockerRuntimeWait); err != nil {
		return dockerStartGuidance(err)
	}
	if err := verifyComposePlugin(); err != nil {
		return err
	}
	probe := probeDocker()
	fmt.Printf(`[docker] Docker Engine %s + Compose %s are ready`+"\n", probe.engineVer, probe.composeVer)
	return nil
}

// probeDocker collects the host state without failing: every check is best effort so the
// caller can decide which branch applies.
func probeDocker() dockerProbe {
	probe := dockerProbe{}
	probe.binaryPath, _ = exec.LookPath(`docker`)
	if probe.binaryPath == `` {
		for _, unit := range dockerRuntimeUnits {
			if hostUnitExists(unit) {
				probe.unitPresent = true
				break
			}
		}
		return probe
	}
	if out, err := runQuiet(`docker`, `version`, `--format`, `{{.Server.Version}}`); err == nil {
		probe.engineOK = true
		probe.engineVer = firstOutputLine(out)
	}
	if out, err := runQuiet(`docker`, `compose`, `version`, `--short`); err == nil {
		probe.composeOK = true
		probe.composeVer = firstOutputLine(out)
	}
	return probe
}

// requireRootForHostWrite guards every write into system-owned directories.
func requireRootForHostWrite() error {
	if os.Geteuid() != 0 {
		return runtimeUnavailable(`writing system directories requires root: run "sudo ./goteamsctl install"`)
	}
	return nil
}

// requireDockerInstallPrerequisites refuses to attempt a runtime install on a host where it
// cannot work, before anything is written to disk.
func requireDockerInstallPrerequisites() error {
	if err := requireRootForHostWrite(); err != nil {
		return err
	}
	if _, err := os.Stat(`/run/systemd/system`); err != nil {
		return runtimeUnavailable(`the bundled Docker runtime is installed through systemd, but this host has no running systemd (PID 1); install Docker with your own init setup`)
	}
	if _, err := exec.LookPath(`systemctl`); err != nil {
		return runtimeUnavailable(`systemctl is not available; cannot manage the Docker services`)
	}
	// The static engine ships no firewall userspace tooling: without one of these, dockerd
	// cannot set up bridge networking and fails later with a confusing error.
	if !hasAnyTool(`iptables`, `nft`) {
		return runtimeUnavailable(`Docker needs iptables or nft on the host (not part of the static bundle): install it first, e.g. "yum install -y iptables" or "apt-get install -y iptables"`)
	}
	return nil
}

// installEngineBinaries extracts the static tarball into a private temporary directory,
// accepts only the whitelisted entries, then moves them to /usr/local/bin without ever
// replacing an existing host binary.
func installEngineBinaries(tarball string) error {
	// Extract on the destination filesystem: /tmp is often a size-limited tmpfs and a
	// cross-device rename would fail on the final move.
	tmpDir, err := os.MkdirTemp(dockerBinDir, `.tmp-docker-runtime-`)
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	extracted, err := extractStaticEngineTarball(tarball, tmpDir)
	if err != nil {
		return err
	}
	var blocked []string
	for _, name := range extracted {
		if fileExists(filepath.Join(dockerBinDir, name)) {
			blocked = append(blocked, filepath.Join(dockerBinDir, name))
		}
	}
	if len(blocked) > 0 {
		return fmt.Errorf(`refusing to overwrite existing host binaries: %s`, strings.Join(blocked, `, `))
	}
	for _, name := range extracted {
		if err = installFile(filepath.Join(tmpDir, name), filepath.Join(dockerBinDir, name), 0o755); err != nil {
			return fmt.Errorf(`install docker binary %s: %w`, name, err)
		}
	}
	return nil
}

// installComposePlugin places the Compose V2 CLI plugin.
func installComposePlugin(src string) error {
	if err := os.MkdirAll(dockerComposePluginDir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(dockerComposePluginDir, `docker-compose`)
	if fileExists(dst) {
		return fmt.Errorf(`refusing to overwrite existing %s`, dst)
	}
	if err := installFile(src, dst, 0o755); err != nil {
		return fmt.Errorf(`install compose plugin: %w`, err)
	}
	return nil
}

// installSystemdUnits copies the bundled units. An existing unit aborts the install instead
// of silently layering our unit on top of an unknown Docker setup.
func installSystemdUnits(pkgDir string) error {
	for _, unit := range dockerRuntimeUnits {
		src := filepath.Join(pkgDir, filepath.FromSlash(dockerRuntimeRelDir), `systemd`, unit)
		raw, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf(`read bundled unit %s: %w`, unit, err)
		}
		dst := filepath.Join(systemdUnitDir, unit)
		if fileExists(dst) {
			return fmt.Errorf(`refusing to overwrite existing %s`, dst)
		}
		if err = writeFileAtomic(dst, raw, 0o644); err != nil {
			return fmt.Errorf(`install unit %s: %w`, unit, err)
		}
	}
	return runCommand(`systemctl`, `daemon-reload`)
}

// startDockerServices creates the socket group, enables and starts containerd + docker.
func startDockerServices() error {
	if _, err := exec.LookPath(`groupadd`); err != nil {
		return runtimeUnavailable(`groupadd is required to create the "docker" group used by docker.socket`)
	}
	if err := runCommand(`groupadd`, `-f`, `docker`); err != nil {
		return fmt.Errorf(`create docker group: %w`, err)
	}
	for _, args := range [][]string{
		{`enable`, `--now`, `containerd.service`},
		{`enable`, `--now`, `docker.socket`, `docker.service`},
	} {
		if err := runCommand(`systemctl`, args...); err != nil {
			return fmt.Errorf(`systemctl %s: %w`, strings.Join(args, ` `), err)
		}
	}
	return nil
}

// waitForDockerEngine polls the daemon until it answers or the deadline passes.
func waitForDockerEngine(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if out, err := runQuiet(`docker`, `version`, `--format`, `{{.Server.Version}}`); err == nil {
			fmt.Printf(`[docker] engine is up (%s)`+"\n", firstOutputLine(out))
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf(`docker daemon did not become ready within %s`, timeout)
}

// verifyComposePlugin checks the plugin is callable through the docker CLI.
func verifyComposePlugin() error {
	if out, err := runQuiet(`docker`, `compose`, `version`, `--short`); err == nil {
		fmt.Printf(`[docker] docker compose %s is available`+"\n", firstOutputLine(out))
		return nil
	}
	return runtimeUnavailable(fmt.Sprintf(`"docker compose" still does not work after installing the plugin into %s; check that the docker CLI searches that directory`, dockerComposePluginDir))
}

// hostUnitExists reports whether systemd already owns a unit of this name in any load path.
func hostUnitExists(unit string) bool {
	for _, dir := range []string{systemdUnitDir, `/run/systemd/system`, `/usr/lib/systemd/system`, `/lib/systemd/system`} {
		if fileExists(filepath.Join(dir, unit)) {
			return true
		}
	}
	return false
}

// runQuiet executes a command capturing combined output for probes (no streaming, no prompts).
func runQuiet(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf(`%s %s: %w`, name, strings.Join(args, ` `), err)
	}
	return string(out), nil
}

// hasAnyTool reports whether at least one of the named executables is in PATH.
func hasAnyTool(names ...string) bool {
	for _, name := range names {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	return false
}

// firstOutputLine trims command output down to its first line for display.
func firstOutputLine(out string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	return strings.TrimSpace(line)
}

// runtimeUnavailable builds the consistently prefixed error used for every decision the
// installer refuses to take on the operator's behalf.
func runtimeUnavailable(advice string) error {
	return fmt.Errorf(`docker runtime unavailable: %s`, advice)
}

// dockerBrokenGuidance fails closed on a host that has a partial Docker setup: the engine
// binary or a docker unit exists but the daemon or Compose plugin does not work, so guessing
// here could break a customer-managed installation.
func dockerBrokenGuidance(probe dockerProbe) error {
	fmt.Println(`[docker] a Docker installation was detected but is not usable:`)
	fmt.Printf(`         binary=%q engine=%v compose=%v unit present=%v`+"\n", probe.binaryPath, probe.engineOK, probe.composeOK, probe.unitPresent)
	fmt.Println(`         the bundled runtime is only installed on hosts without Docker, so nothing was changed.`)
	fmt.Println(`[docker] fix the existing installation, then re-run the command:`)
	fmt.Println(`         docker info`)
	fmt.Println(`         systemctl status docker containerd docker.socket`)
	fmt.Println(`         journalctl -u docker --no-pager -n 50`)
	return errors.New(`docker engine is present but not usable (see the [docker] lines above)`)
}

// dockerStartGuidance reports a failed service start with the commands that show why.
func dockerStartGuidance(cause error) error {
	fmt.Println(`[docker] bundled runtime was installed but the engine did not come up:`)
	fmt.Println(`         systemctl status docker --no-pager`)
	fmt.Println(`         journalctl -u docker -u containerd --no-pager -n 80`)
	fmt.Println(`         binaries live in ` + dockerBinDir + `, units in ` + systemdUnitDir + `; remove them to undo`)
	return fmt.Errorf(`bundled docker runtime failed to start: %w`, cause)
}

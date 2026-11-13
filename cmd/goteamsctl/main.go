// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

// goteamsctl is the commercial-package control tool (Linux amd64). It is shipped inside the
// commercial package and drives install/upgrade/service lifecycle against a fixed install root.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"goteams/internal/app/goteams/define"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultRoot        = `/opt/goteams`
	composeProjectName = `goteams-commercial`
	stateFileName      = `state.json`
	markerFileName     = `state.json`
)

var stdinReader = bufio.NewReader(os.Stdin)

func usage() {
	fmt.Print(`GoTeams commercial control tool

Usage:
  goteamsctl install   [--root /opt/goteams] [--yes]
  goteamsctl set-site-url [url] [--root /opt/goteams]
  goteamsctl start     [--root /opt/goteams]
  goteamsctl stop      [--root /opt/goteams]
  goteamsctl restart   [--root /opt/goteams]
  goteamsctl status    [--root /opt/goteams]
  goteamsctl logs      [--root /opt/goteams] [service]
  goteamsctl upgrade   [--root /opt/goteams]
  goteamsctl install-docker   [--yes]
  goteamsctl rollback-artifacts <version> [--root /opt/goteams]
  goteamsctl reset-admin-password [--root /opt/goteams]
  goteamsctl uninstall [--root /opt/goteams]
  goteamsctl version

Notes:
  License activation is not part of install or upgrade: activate from the web UI
  (free private experience license, default expiry 2099-01-01, or offline key), at any time after the services are up.
  The public site address (used in server-generated links such as automation templates
  and API docs) is asked during install and stored in <root>/config/.env as
  GOTEAMS_WEBSERVICE_SITE_URL; change it later with "goteamsctl set-site-url <url>".
  Invitation links (members / issue forms) follow the browser origin instead and are
  unaffected by this value.
  Offline packages carry the docker images (payload/images/images.tar.gz) and are
  installed/upgraded without network access; standard packages pull from the registry.
  Both variants carry a Docker runtime installer (payload/runtime/): install uses it only on a
  host without Docker at all, and an existing-but-broken Docker is never modified — the tool
  fails closed and prints the repair commands. Run "install-docker" to prepare a host first.
`)
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	command := args[0]
	if command == `version` || command == `-version` || command == `--version` {
		fmt.Printf("goteamsctl %s edition=%s commit=%s\n", define.BuildVersion, define.BuildEdition, define.BuildCommit)
		return
	}
	if command == `help` || command == `-h` || command == `--help` {
		usage()
		return
	}

	options, err := parseFlags(args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, `error: `+err.Error())
		os.Exit(2)
	}

	var root string
	// install-docker prepares a bare host: there is no managed instance to resolve yet, so it
	// deliberately skips the root marker checks.
	if command != `install-docker` {
		if root, err = resolveRoot(options.root, command == `install`); err != nil {
			fmt.Fprintln(os.Stderr, `error: `+err.Error())
			os.Exit(2)
		}
	}

	var runErr error
	switch command {
	case `install`:
		runErr = cmdInstall(root, options)
	case `set-site-url`:
		runErr = cmdSetSiteURL(root, options.service, options)
	case `install-docker`:
		runErr = cmdInstallDocker(options)
	case `start`:
		runErr = cmdStart(root)
	case `stop`:
		runErr = cmdStop(root)
	case `restart`:
		runErr = cmdRestart(root)
	case `status`:
		runErr = cmdStatus(root)
	case `logs`:
		runErr = cmdLogs(root, options.service)
	case `upgrade`:
		runErr = cmdUpgrade(root)
	case `rollback-artifacts`:
		if options.service == `` {
			runErr = errors.New(`rollback-artifacts requires a version argument`)
			break
		}
		runErr = cmdRollbackArtifacts(root, options.service, options)
	case `reset-admin-password`:
		runErr = cmdResetAdminPassword(root)
	case `uninstall`:
		runErr = cmdUninstall(root)
	default:
		usage()
		os.Exit(2)
	}
	if runErr != nil {
		fmt.Fprintln(os.Stderr, `error: `+runErr.Error())
		os.Exit(1)
	}
}

// ctlOptions carries parsed command line flags.
type ctlOptions struct {
	root      string
	service   string
	assumeYes bool
}

// parseFlags parses the shared flag set; a license key is never a CLI argument.
func parseFlags(args []string) (ctlOptions, error) {
	options := ctlOptions{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == `--root`:
			i++
			if i >= len(args) {
				return options, errors.New(`--root requires a value`)
			}
			options.root = args[i]
		case arg == `--yes` || arg == `-y`:
			options.assumeYes = true
		case strings.HasPrefix(arg, `-`):
			return options, fmt.Errorf(`unknown flag: %s`, arg)
		default:
			if options.service == `` {
				options.service = arg
			}
		}
	}
	return options, nil
}

// resolveRoot validates the install root: absolute, not a dangerous path, and (outside
// install) holding the state.json marker proving it is a managed instance.
func resolveRoot(explicit string, forInstall bool) (string, error) {
	root := strings.TrimSpace(explicit)
	if root == `` {
		root = defaultRoot
	}
	root = filepath.Clean(root)
	if !filepath.IsAbs(root) {
		return ``, fmt.Errorf(`--root must be an absolute path, got %q`, root)
	}
	if root == `/` || root == `/opt` || root == `/usr` || root == `/etc` || root == `/var` || root == `/home` || root == `/tmp` || root == `/root` {
		return ``, fmt.Errorf(`refusing to operate on system path %q`, root)
	}
	statePath := filepath.Join(root, markerFileName)
	if forInstall {
		if _, err := os.Stat(statePath); err == nil {
			return ``, fmt.Errorf(`%s already looks installed; use upgrade instead`, root)
		}
		return root, nil
	}
	if _, err := os.Stat(statePath); err != nil {
		return ``, fmt.Errorf(`%s is not an installed instance; run "goteamsctl install" first (missing %s)`, root, statePath)
	}
	return root, nil
}

// promptLine reads a line from stdin with a default value.
func promptLine(label, def string) string {
	if def != `` {
		fmt.Printf(`%s [%s]: `, label, def)
	} else {
		fmt.Printf(`%s: `, label)
	}
	line, _ := stdinReader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == `` {
		return def
	}
	return line
}

// confirm asks a yes/no question; assumeYes short-circuits it.
func confirm(label string, assumeYes bool) bool {
	if assumeYes {
		return true
	}
	line := promptLine(label+` [y/N]`, ``)
	return strings.EqualFold(line, `y`) || strings.EqualFold(line, `yes`)
}

// loadState reads state.json from the install root.
func loadState(root string) (map[string]any, error) {
	return readJSONFile(filepath.Join(root, stateFileName))
}

// saveState writes state.json atomically (no secrets inside).
func saveState(root string, state map[string]any) error {
	return writeJSONFileAtomic(filepath.Join(root, stateFileName), state)
}

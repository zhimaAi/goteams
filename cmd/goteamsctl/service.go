// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// allowedLogServices limits `goteamsctl logs` targets to compose service names.
var allowedLogServices = map[string]bool{
	``: true, `goteams`: true, `web`: true, `postgres`: true, `redis`: true,
}

// currentReleaseDir resolves the active release directory from .env GOTEAMS_RELEASE and
// verifies it still holds a compose file (guards against a half-finished upgrade).
func currentReleaseDir(root string) (string, error) {
	env, err := readEnvFile(composeEnvPath(root))
	if err != nil {
		return ``, err
	}
	releaseDir := strings.TrimSpace(env[`GOTEAMS_RELEASE`])
	if releaseDir == `` {
		return ``, errors.New(`GOTEAMS_RELEASE missing in .env`)
	}
	if _, err = os.Stat(filepath.Join(releaseDir, `docker`, `docker-compose.yml`)); err != nil {
		return ``, fmt.Errorf(`active release %s is not a valid release directory`, releaseDir)
	}
	return releaseDir, nil
}

// cmdStart brings the full stack up in the documented order (databases → goteams → web).
func cmdStart(root string) error {
	if err := requireDocker(); err != nil {
		return err
	}
	releaseDir, err := currentReleaseDir(root)
	if err != nil {
		return err
	}
	if err = startStack(root, releaseDir); err != nil {
		return err
	}
	printStatusSummary(root)
	return nil
}

// cmdStop stops every service of the stack (data volumes are kept).
func cmdStop(root string) error {
	if err := requireDocker(); err != nil {
		return err
	}
	releaseDir, err := currentReleaseDir(root)
	if err != nil {
		return err
	}
	if err = runCompose(root, releaseDir, `stop`); err != nil {
		return err
	}
	fmt.Println(`all services stopped (data volumes kept)`)
	return nil
}

// cmdRestart stops and then starts the stack in the documented order.
func cmdRestart(root string) error {
	if err := cmdStop(root); err != nil {
		return err
	}
	return cmdStart(root)
}

// cmdStatus prints compose ps, the installed version and the license status summary.
func cmdStatus(root string) error {
	if err := requireDocker(); err != nil {
		return err
	}
	releaseDir, err := currentReleaseDir(root)
	if err != nil {
		return err
	}
	if err = runCompose(root, releaseDir, `ps`); err != nil {
		return err
	}
	if state, stateErr := loadState(root); stateErr == nil {
		fmt.Printf("installed version: %s (previous: %s)\n", str(state, `version`), str(state, `previous_version`))
	}
	printStatusSummary(root)
	return nil
}

// cmdLogs tails compose logs (follow, last 200 lines); Ctrl+C is forwarded to docker.
func cmdLogs(root, service string) error {
	service = strings.TrimSpace(service)
	if !allowedLogServices[service] {
		return fmt.Errorf(`unknown service %q (allowed: goteams, web, postgres, redis)`, service)
	}
	if err := requireDocker(); err != nil {
		return err
	}
	releaseDir, err := currentReleaseDir(root)
	if err != nil {
		return err
	}
	args := []string{`logs`, `-f`, `--tail=200`}
	if service != `` {
		args = append(args, service)
	}
	return runCompose(root, releaseDir, args...)
}

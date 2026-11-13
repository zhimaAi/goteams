// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// uninstallConfirmPhrase must be typed verbatim (case sensitive) to confirm an uninstall.
const uninstallConfirmPhrase = `UNINSTALL-GOTEAMS`

// printRed writes a line wrapped in ANSI red; uninstall warnings must be impossible to miss.
func printRed(format string, args ...any) {
	fmt.Printf("\033[31m"+format+"\033[0m\n", args...)
}

// cmdUninstall removes the whole managed instance: containers/networks/named data volumes
// through compose down -v, then the install root (releases, data, logs, backups, config).
// It is irreversible: the red typed-phrase confirmation is mandatory and --yes never
// bypasses it. If compose down fails, disk contents stay untouched (services first).
func cmdUninstall(root string) error {
	if err := requireDocker(); err != nil {
		return err
	}
	state, _ := loadState(root)
	version := str(state, `version`)
	if version == `` {
		version = `unknown`
	}
	releaseDir, releaseErr := currentReleaseDir(root)

	printRed(`==================================================================`)
	printRed(`DANGER: uninstalling GoTeams PERMANENTLY DELETES ALL DATA:`)
	printRed(`  install root : %s`, root)
	printRed(`  version      : %s`, version)
	printRed(`  - all containers, networks and data volumes (postgres, redis)`)
	printRed(`  - the whole install root: releases/, data/ (uploads, license),`)
	printRed(`    logs/, backups/, config/ and state.json`)
	printRed(`  THIS CANNOT BE UNDONE. Back up anything you need first.`)
	printRed(`==================================================================`)

	fmt.Print(`type ` + "\033[31m" + uninstallConfirmPhrase + "\033[0m" + ` (in red above) to confirm: `)
	line, _ := stdinReader.ReadString('\n')
	if strings.TrimSpace(line) != uninstallConfirmPhrase {
		return errors.New(`uninstall aborted: confirmation phrase did not match`)
	}

	if releaseErr == nil {
		fmt.Println(`[services] removing containers, networks and data volumes ...`)
		if err := runCompose(root, releaseDir, `down`, `-v`, `--remove-orphans`); err != nil {
			return fmt.Errorf(`compose down failed (nothing was deleted on disk): %w`, err)
		}
	} else {
		fmt.Println(`[warn] cannot resolve the active release (` + releaseErr.Error() + `);`)
		fmt.Println(`[warn] skipping compose down — remove leftover containers manually if any`)
	}

	marker := filepath.Join(root, stateFileName)
	if _, err := os.Stat(marker); err != nil {
		return fmt.Errorf(`refusing to delete: %s is missing (not a managed instance)`, marker)
	}
	fmt.Printf("[disk] removing %s ...\n", root)
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf(`remove install root: %w`, err)
	}
	fmt.Println(`uninstall complete: services removed and data deleted`)
	return nil
}

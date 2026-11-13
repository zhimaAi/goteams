// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// cmdResetAdminPassword regenerates the default administrator password: a fresh random
// secret is bcrypt-hashed locally and applied to the database through the postgres container.
// The plaintext is printed once; it is never written to disk, logs or command arguments.
func cmdResetAdminPassword(root string) error {
	if err := requireDocker(); err != nil {
		return err
	}
	env, err := readEnvFile(composeEnvPath(root))
	if err != nil {
		return err
	}
	// New installs embed admin_username=admin; a legacy migrated .env may override it.
	username := strings.TrimSpace(env[`GOTEAMS_ACCOUNT_ADMIN_USERNAME`])
	if username == `` {
		username = `admin`
	}
	releaseDir := strings.TrimSpace(env[`GOTEAMS_RELEASE`])
	if releaseDir == `` {
		return errors.New(`GOTEAMS_RELEASE missing in .env`)
	}

	password := randomString(16)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	hashText := string(hash)
	// Defense in depth for SQL quoting: neither value may carry a single quote.
	if strings.Contains(hashText, `'`) || strings.Contains(username, `'`) {
		return errors.New(`unsafe characters in password hash or username, aborting`)
	}

	if !confirm(`reset the password of the default administrator `+username+`?`, false) {
		return errors.New(`aborted`)
	}

	sql := fmt.Sprintf(`UPDATE users SET password='%s', updated_at=%d WHERE username='%s' AND id=admin_id AND deleted_at=0`,
		hashText, time.Now().Unix(), username)
	out, err := runCommandOutput(`docker`, `compose`, `--project-name`, composeProjectName,
		`--env-file`, composeEnvPath(root), `-f`, filepath.Join(releaseDir, `docker`, `docker-compose.yml`),
		`exec`, `-T`, `postgres`, `psql`, `-U`, env[`POSTGRES_USER`], `-d`, env[`POSTGRES_DB`],
		`-v`, `ON_ERROR_STOP=1`, `-c`, sql)
	if err != nil {
		return fmt.Errorf(`apply password update: %w`, err)
	}
	if !strings.Contains(out, `UPDATE 1`) {
		return fmt.Errorf(`no default administrator row matched username %q`, username)
	}
	fmt.Println(`password reset successfully for ` + username)
	fmt.Println(`new password: ` + password)
	fmt.Println(`store it now; it will not be shown again`)
	return nil
}

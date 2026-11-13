// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// envSiteURLKey is the .env override for the embedded [webservice] site_url. The goteams
// binary applies GOTEAMS_WEBSERVICE_SITE_URL on top of its embedded INI, and compose passes
// it through, so editing the .env is enough to change the public site address.
const envSiteURLKey = `GOTEAMS_WEBSERVICE_SITE_URL`

// defaultSiteURL is the install-time default: the embedded INI value adapted to the chosen
// web port. Local-only links keep working after install; operators should replace it with
// the externally reachable address that generated links (automation templates, API docs)
// should carry.
func defaultSiteURL(webPort int) string {
	return fmt.Sprintf(`http://127.0.0.1:%d`, webPort)
}

// defaultSiteURLForEnv derives the default from an existing .env WEB_PORT (falling back to
// the compose default 8080).
func defaultSiteURLForEnv(env map[string]string) string {
	port := 8080
	if raw := strings.TrimSpace(env[`WEB_PORT`]); raw != `` {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			port = n
		}
	}
	return defaultSiteURL(port)
}

// normalizeSiteURL validates and canonicalizes a site URL: http/https scheme, a host, no
// query or fragment, and no trailing slash (the backend joins paths with a single slash).
func normalizeSiteURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == `` {
		return ``, errors.New(`site url is empty`)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return ``, fmt.Errorf(`invalid site url %q: %w`, raw, err)
	}
	if parsed.Scheme != `http` && parsed.Scheme != `https` {
		return ``, fmt.Errorf(`site url must start with http:// or https://, got %q`, raw)
	}
	if parsed.Host == `` {
		return ``, fmt.Errorf(`site url is missing a host, got %q`, raw)
	}
	if parsed.RawQuery != `` || parsed.Fragment != `` || parsed.Path != `` && strings.TrimRight(parsed.Path, `/`) != `` {
		return ``, fmt.Errorf(`site url must be a bare origin without path, query or fragment, got %q`, raw)
	}
	return strings.TrimRight(value, `/`), nil
}

// setEnvKey replaces the value of KEY in the .env, keeping the line position; a missing key
// is appended. Only the installer-maintained layout is touched, never other lines.
func setEnvKey(envPath, key, value string) error {
	raw, err := os.ReadFile(envPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(raw), "\n")
	prefix := key + `=`
	for i, line := range lines {
		body := strings.TrimSpace(line)
		if body == `` || strings.HasPrefix(body, `#`) {
			continue
		}
		if strings.HasPrefix(body, prefix) {
			lines[i] = prefix + value
			return writeFileAtomic(envPath, []byte(strings.Join(lines, "\n")), 0o600)
		}
	}
	content := strings.TrimRight(string(raw), "\n")
	if content != `` {
		content += "\n"
	}
	content += prefix + value + "\n"
	return writeFileAtomic(envPath, []byte(content), 0o600)
}

// cmdSetSiteURL updates the public site address in .env and recreates the goteams container
// so the goteams binary picks up the GOTEAMS_WEBSERVICE_SITE_URL override. The web frontend
// is untouched: invitation links follow the browser origin, not this value.
func cmdSetSiteURL(root, rawURL string, options ctlOptions) error {
	envPath := composeEnvPath(root)
	env, err := readEnvFile(envPath)
	if err != nil {
		return err
	}

	value := strings.TrimSpace(rawURL)
	if value == `` {
		if options.assumeYes {
			return errors.New(`set-site-url requires a URL argument when --yes is set`)
		}
		def := strings.TrimSpace(env[envSiteURLKey])
		if def == `` {
			def = defaultSiteURLForEnv(env)
		}
		value = promptLine(`site url (address used in generated links)`, def)
	}

	normalized, err := normalizeSiteURL(value)
	if err != nil {
		return err
	}
	if err = setEnvKey(envPath, envSiteURLKey, normalized); err != nil {
		return err
	}
	fmt.Printf("site url set to %s (%s)\n", normalized, envPath)

	if err = requireDocker(); err != nil {
		fmt.Println(`[warn] docker is unavailable, .env updated but services were not restarted: ` + err.Error())
		return nil
	}
	releaseDir, err := currentReleaseDir(root)
	if err != nil {
		return err
	}
	fmt.Println(`[services] recreating goteams so the new site url takes effect ...`)
	if err = runCompose(root, releaseDir, `up`, `-d`, `goteams`); err != nil {
		return fmt.Errorf(`recreate goteams: %w`, err)
	}
	fmt.Println(`site url updated (goteams recreated)`)
	return nil
}

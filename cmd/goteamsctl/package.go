// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// packageManifest mirrors manifest.json shipped in the commercial package.
type packageManifest struct {
	Name                 string          `json:"name"`
	Version              string          `json:"version"`
	Edition              string          `json:"edition"`
	Arch                 string          `json:"arch"`
	MinUpgradeVersion    string          `json:"min_upgrade_version"`
	DockerEngineVersion  string          `json:"docker_engine_version"`
	DockerComposeVersion string          `json:"docker_compose_version"`
	Images               []string        `json:"images"`
	Files                []manifestEntry `json:"files"`
}

// manifestEntry is one whitelisted payload file with its expected SHA-256.
type manifestEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// licenseClaims is a display-only parse of the key claims payload (signature verification is
// the backend's job; the CLI only reads fields to show and cross-check the status output).
type licenseClaims struct {
	LicenseNo string         `json:"license_no"`
	Revision  int            `json:"revision"`
	ExpireAt  int64          `json:"expire_at"`
	Quotas    map[string]any `json:"quotas"`
}

// verifyPackage validates the unpacked package at pkgDir: SHA256SUMS entries, manifest
// whitelist equality and the goteams binary edition/version identity.
func verifyPackage(pkgDir string) (*packageManifest, error) {
	manifestPath := filepath.Join(pkgDir, `manifest.json`)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf(`read manifest.json: %w`, err)
	}
	var manifest packageManifest
	if err = json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf(`parse manifest.json: %w`, err)
	}
	if manifest.Name != `goteams-commercial` || manifest.Edition != `commercial` || manifest.Arch != `linux-amd64` {
		return nil, errors.New(`manifest is not a linux-amd64 commercial package`)
	}
	if !strings.HasPrefix(manifest.Version, `v`) {
		return nil, fmt.Errorf(`invalid manifest version %q`, manifest.Version)
	}
	if _, err = parseSemver(manifest.Version); err != nil {
		return nil, fmt.Errorf(`invalid manifest version: %w`, err)
	}

	sums, err := parseSHA256Sums(filepath.Join(pkgDir, `SHA256SUMS`))
	if err != nil {
		return nil, err
	}
	manifestPaths := make(map[string]string, len(manifest.Files))
	for _, entry := range manifest.Files {
		manifestPaths[filepath.ToSlash(filepath.Clean(entry.Path))] = entry.SHA256
	}
	// SHA256SUMS and manifest.files must describe exactly the same whitelist.
	if len(sums) != len(manifestPaths) {
		return nil, errors.New(`SHA256SUMS and manifest file lists differ`)
	}
	for path, sum := range sums {
		if _, ok := manifestPaths[path]; !ok {
			return nil, fmt.Errorf(`SHA256SUMS lists non-whitelisted file %s`, path)
		}
		if manifestPaths[path] != sum {
			return nil, fmt.Errorf(`manifest and SHA256SUMS disagree for %s`, path)
		}
	}
	// A manifest that declares a bundled Docker runtime must whitelist every file the runtime
	// installer reads, otherwise the install would fail halfway through on a live host.
	if manifest.DockerEngineVersion != `` || manifest.DockerComposeVersion != `` {
		if manifest.DockerEngineVersion == `` || manifest.DockerComposeVersion == `` {
			return nil, errors.New(`manifest declares a partial docker runtime (engine and compose versions are both required)`)
		}
		runtimePaths := dockerRuntimeManifestPaths(manifest.DockerEngineVersion)
		if len(runtimePaths) == 0 {
			return nil, fmt.Errorf(`invalid docker_engine_version %q`, manifest.DockerEngineVersion)
		}
		for _, rel := range runtimePaths {
			if _, ok := manifestPaths[rel]; !ok {
				return nil, fmt.Errorf(`bundled docker runtime is not whitelisted: %s`, rel)
			}
		}
	}
	for _, entry := range manifest.Files {
		full := filepath.Join(pkgDir, filepath.FromSlash(entry.Path))
		got, err := fileSHA256(full)
		if err != nil {
			return nil, fmt.Errorf(`hash %s: %w`, entry.Path, err)
		}
		if got != entry.SHA256 {
			return nil, fmt.Errorf(`sha256 mismatch for %s`, entry.Path)
		}
	}
	// Every payload file must be whitelisted (no extra files smuggled into payload/).
	payloadDir := filepath.Join(pkgDir, `payload`)
	err = filepath.Walk(payloadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(pkgDir, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if _, ok := manifestPaths[rel]; !ok {
			return fmt.Errorf(`payload contains non-whitelisted file %s`, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// The shipped goteams binary must self-identify as the commercial build of this version.
	binPath := filepath.Join(pkgDir, `payload`, `bin`, `goteams`)
	out, err := exec.Command(binPath, `-version`).Output()
	if err != nil {
		return nil, fmt.Errorf(`run payload/bin/goteams -version: %w`, err)
	}
	line := strings.TrimSpace(string(out))
	if !strings.Contains(line, `edition=commercial`) {
		return nil, fmt.Errorf(`goteams binary is not commercial: %s`, line)
	}
	if !strings.HasPrefix(line, `goteams `+manifest.Version+` `) {
		return nil, fmt.Errorf(`goteams binary version mismatch: %s (manifest %s)`, line, manifest.Version)
	}
	return &manifest, nil
}

// parseSHA256Sums parses "<sha256>  <path>" lines.
func parseSHA256Sums(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(`read SHA256SUMS: %w`, err)
	}
	sums := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == `` || strings.HasPrefix(line, `#`) {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 || len(parts[0]) != 64 {
			return nil, fmt.Errorf(`bad SHA256SUMS line: %q`, line)
		}
		sums[parts[1]] = parts[0]
	}
	if len(sums) == 0 {
		return nil, errors.New(`SHA256SUMS is empty`)
	}
	return sums, nil
}

// fileSHA256 returns the hex sha256 of a file.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return ``, err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return ``, err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// parseLicenseClaims decodes the claims segment of a GTLIC1 key without verifying the
// signature (backend verification stays authoritative).
func parseLicenseClaims(key string) (*licenseClaims, error) {
	parts := strings.Split(strings.TrimSpace(key), `.`)
	if len(parts) != 4 || parts[0] != `GTLIC1` {
		return nil, errors.New(`key is not a GTLIC1 license key`)
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf(`decode claims: %w`, err)
	}
	var claims licenseClaims
	if err = json.Unmarshal(raw, &claims); err != nil {
		return nil, fmt.Errorf(`parse claims: %w`, err)
	}
	return &claims, nil
}

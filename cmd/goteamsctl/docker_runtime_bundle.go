// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

// Platform independent half of the bundled Docker runtime: the package layout contract that
// the manifest check relies on, plus the strict extraction of the upstream static tarball.

package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// dockerRuntimeRelDir is the package directory carrying the bundled Docker runtime.
const dockerRuntimeRelDir = `payload/runtime`

// dockerRuntimeBinaries is the closed set of files accepted from the bundled static
// tarball; any other entry (or a missing one) aborts the installation.
var dockerRuntimeBinaries = []string{
	`docker`, `dockerd`, `docker-proxy`, `docker-init`,
	`containerd`, `containerd-shim-runc-v2`, `ctr`, `runc`,
}

// dockerRuntimeUnits are the systemd units shipped next to the binaries. The names must
// match the packaging whitelist in scripts/build-commercial-package.ps1.
var dockerRuntimeUnits = []string{
	`containerd.service`, `docker.service`, `docker.socket`,
}

// dockerRuntimeManifestPaths lists every package-relative path a package must whitelist when
// its manifest declares a bundled Docker runtime. A malformed engine version yields no paths,
// which makes the manifest check fail instead of joining attacker-controlled text into a path.
func dockerRuntimeManifestPaths(engineVersion string) []string {
	if !validRuntimeVersion(engineVersion) {
		return nil
	}
	paths := []string{
		dockerRuntimeRelDir + `/docker-` + engineVersion + `.tgz`,
		dockerRuntimeRelDir + `/docker-compose`,
	}
	for _, unit := range dockerRuntimeUnits {
		paths = append(paths, dockerRuntimeRelDir+`/systemd/`+unit)
	}
	return paths
}

// validRuntimeVersion accepts a plain version token (digits, letters, dot, hyphen) and
// rejects anything that could escape a directory or contain a glob.
func validRuntimeVersion(version string) bool {
	if version == `` || len(version) > 32 || strings.IndexAny(version, `/\`) >= 0 {
		return false
	}
	for _, ch := range version {
		switch {
		case ch >= '0' && ch <= '9', ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch == '.', ch == '-':
		default:
			return false
		}
	}
	return version != `.` && version != `..`
}

// dockerRuntimePaths points at the bundled runtime inside a verified package.
type dockerRuntimePaths struct {
	pkgDir         string
	engineVersion  string
	composeVersion string
	staticTarball  string
	composePlugin  string
}

// newDockerRuntimePaths resolves the bundled runtime of a package whose manifest and hashes
// have already been verified. The manifest versions are display-only; every byte that gets
// installed is the whitelisted file behind them.
func newDockerRuntimePaths(pkgDir string, manifest *packageManifest) dockerRuntimePaths {
	return dockerRuntimePaths{
		pkgDir:         pkgDir,
		engineVersion:  orDefault(manifest.DockerEngineVersion, `bundled`),
		composeVersion: orDefault(manifest.DockerComposeVersion, `bundled`),
		staticTarball:  bundledEngineTarball(pkgDir),
		composePlugin:  filepath.Join(pkgDir, filepath.FromSlash(dockerRuntimeRelDir), `docker-compose`),
	}
}

// bundledEngineTarball returns the single payload/runtime/docker-<version>.tgz of a package
// (empty when absent or ambiguous).
func bundledEngineTarball(pkgDir string) string {
	matches, err := filepath.Glob(filepath.Join(pkgDir, filepath.FromSlash(dockerRuntimeRelDir), `docker-*.tgz`))
	if err != nil || len(matches) != 1 {
		return ``
	}
	return matches[0]
}

// extractStaticEngineTarball unpacks the upstream layout (every member below a single "docker/"
// directory) and returns the extracted member names. Symlinks, devices, traversal and unknown
// names are rejected, so a tampered archive can only ever write whitelisted file names.
func extractStaticEngineTarball(tarball, destDir string) ([]string, error) {
	f, err := os.Open(tarball)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf(`open gzip %s: %w`, tarball, err)
	}
	defer func() { _ = gz.Close() }()

	reader := tar.NewReader(gz)
	var names []string
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf(`read tar %s: %w`, tarball, err)
		}
		member := path.Clean(filepath.ToSlash(header.Name))
		if header.Typeflag == tar.TypeDir {
			if member == `docker` {
				continue
			}
			return nil, fmt.Errorf(`unexpected directory %q in %s`, header.Name, tarball)
		}
		if header.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf(`reject non-regular entry %q in %s`, header.Name, tarball)
		}
		base := path.Base(member)
		if !strings.HasPrefix(member, `docker/`) || !containsString(dockerRuntimeBinaries, base) {
			return nil, fmt.Errorf(`reject unexpected entry %q in %s`, header.Name, tarball)
		}
		if err = installReader(reader, filepath.Join(destDir, base), 0o755); err != nil {
			return nil, err
		}
		names = append(names, base)
	}
	if len(names) != len(dockerRuntimeBinaries) {
		return nil, fmt.Errorf(`bundled docker tarball has %d binaries, expected %d (%s)`, len(names), len(dockerRuntimeBinaries), strings.Join(dockerRuntimeBinaries, `, `))
	}
	return names, nil
}

// installFile copies src to dst with the given mode, streaming (the engine binaries are tens
// of megabytes, so they must not be buffered).
func installFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	return installReader(in, dst, mode)
}

// installReader writes a reader into a temporary file under the destination directory, applies
// the mode and renames it into place: the target name only ever exists with its final bytes and
// permissions, and a live inode is never written through.
func installReader(reader io.Reader, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), `.tmp-docker-*`)
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err = io.Copy(tmp, reader); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, dst)
}

// fileExists reports whether target exists (any type).
func fileExists(target string) bool {
	_, err := os.Stat(target)
	return err == nil
}

// containsString reports whether list holds value.
func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

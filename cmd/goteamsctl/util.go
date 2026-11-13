// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// readJSONFile loads a JSON document from disk.
func readJSONFile(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err = json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf(`parse %s: %w`, path, err)
	}
	return data, nil
}

// writeJSONFileAtomic writes pretty JSON via temp file + rename.
func writeJSONFileAtomic(path string, data any) error {
	raw, err := json.MarshalIndent(data, ``, `  `)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, raw, 0o644)
}

// writeFileAtomic writes content via temp file + rename with the given mode.
func writeFileAtomic(path string, content []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, `.tmp-ctl-*`)
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()
	if _, err = tmp.Write(content); err != nil {
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
	if err = os.Chmod(tmpName, mode); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// randomString returns a cryptographically random alphanumeric string.
func randomString(n int) string {
	const alphabet = `ABCDEFGHJKMNPQRSTVWXYZabcdefghjkmnpqrstvwxyz23456789`
	out := make([]byte, n)
	for i := range out {
		v, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			panic(`crypto rand failed: ` + err.Error())
		}
		out[i] = alphabet[v.Int64()]
	}
	return string(out)
}

// randomHex returns a cryptographically random hex string.
func randomHex(nBytes int) string {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		panic(`crypto rand failed: ` + err.Error())
	}
	return hex.EncodeToString(buf)
}

// extractTarGzSafe extracts a tar.gz archive into dest, rejecting entries that escape dest
// (../ segments, absolute paths, symlinks) and skipping everything but regular files/dirs.
func extractTarGzSafe(archivePath, dest string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf(`open gzip %s: %w`, archivePath, err)
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf(`read tar %s: %w`, archivePath, err)
		}
		name := header.Name
		if name == `` || strings.Contains(name, `\`) || strings.HasPrefix(name, `/`) {
			return fmt.Errorf(`reject unsafe archive entry %q`, name)
		}
		cleaned := filepath.Clean(filepath.Join(destAbs, name))
		if !strings.HasPrefix(cleaned, destAbs+string(filepath.Separator)) {
			return fmt.Errorf(`reject archive entry %q escaping %s`, name, destAbs)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(cleaned, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err = os.MkdirAll(filepath.Dir(cleaned), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(cleaned, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err = io.Copy(out, tr); err != nil {
				_ = out.Close()
				return err
			}
			if err = out.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf(`reject archive link entry %q`, name)
		default:
			// ignore other entry types (devices, fifos, ...)
		}
	}
}

// compareSemver compares vX.Y.Z strings numerically; returns -1/0/1.
func compareSemver(a, b string) (int, error) {
	pa, err := parseSemver(a)
	if err != nil {
		return 0, err
	}
	pb, err := parseSemver(b)
	if err != nil {
		return 0, err
	}
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1, nil
		}
		if pa[i] > pb[i] {
			return 1, nil
		}
	}
	return 0, nil
}

// parseSemver strictly parses ^v?\d+\.\d+\.\d+$.
func parseSemver(v string) ([3]int, error) {
	var out [3]int
	v = strings.TrimPrefix(v, `v`)
	parts := strings.Split(v, `.`)
	if len(parts) != 3 {
		return out, fmt.Errorf(`invalid semver %q`, v)
	}
	for i, p := range parts {
		n := 0
		if p == `` {
			return out, fmt.Errorf(`invalid semver %q`, v)
		}
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				return out, fmt.Errorf(`invalid semver %q`, v)
			}
			n = n*10 + int(ch-'0')
		}
		out[i] = n
	}
	return out, nil
}

// httpGetJSON performs a GET and decodes {code, message, data}.
func httpGetJSON(url string, timeout time.Duration) (map[string]any, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var payload struct {
		Code    int            `json:"code"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf(`decode %s: %w`, url, err)
	}
	if payload.Code != 200 {
		return nil, fmt.Errorf(`%s returned code=%d message=%s`, url, payload.Code, payload.Message)
	}
	return payload.Data, nil
}

// jsonIndent pretty-prints a value, falling back to the raw representation.
func jsonIndent(v any) (string, error) {
	raw, err := json.MarshalIndent(v, ``, `  `)
	if err != nil {
		return fmt.Sprintf(`%v`, v), err
	}
	return string(raw), nil
}

// waitHTTPReady polls the URL until it returns any HTTP response (any status code counts:
// the license gate may legitimately answer 403 while services are healthy).
func waitHTTPReady(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 3 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf(`timeout waiting for %s`, url)
}

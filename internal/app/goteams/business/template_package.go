// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	templateassets "goteams/assets/template"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_storage"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// Template package errors. Callers use errors.Is to tell a missing package from a workspace
// outside the package.
var (
	// ErrTemplatePackageMissing means the package file is absent, corrupt, or fails a
	// consistency check (manifest/workspaces/data cross validation, asset hashes, limits).
	ErrTemplatePackageMissing = errors.New(`template package missing`)
	// ErrTemplatePackageWorkspace means the requested template project is not part of
	// the package's active workspace set.
	ErrTemplatePackageWorkspace = errors.New(`template package workspace not found`)
	// ErrTemplateExportForbidden means the caller is not the configured template account.
	ErrTemplateExportForbidden = errors.New(`template export forbidden`)
	// ErrTemplatePackageTooLarge means the export exceeds the package or entry size limit.
	ErrTemplatePackageTooLarge = errors.New(`template package too large`)
)

// Package entry names and directories. The writer and the reader share them so a written
// package always satisfies the reader's whitelist.
const (
	templateManifestEntry   = `manifest.json`
	templateWorkspacesEntry = `workspaces.json`
	templatePoolEntry       = `work_item_templates.json`
	templateDataDir         = `data`
	templateAssetsDir       = `assets`
	templateAssetIndexEntry = `assets/index.json`
	// templateDevMarkerEntry marks a package produced outside the production config. Only the
	// offline export tool writes it; the reader ignores it, and scripts/publish.sh refuses to
	// deploy a package that carries it.
	templateDevMarkerEntry    = `.dev`
	templatePackageMode       = 0644
	templatePackageRetryDelay = 30 * time.Second
)

// Package limits and path: define holds the shipped defaults, the variables are injectable so
// tests can point the loader at a fixture without a new ini key.
var (
	templatePackageFile          = define.TemplatePackagePath
	templatePackageMaxBytes      = int64(define.TemplatePackageMaxBytes)
	templatePackageMaxEntryBytes = int64(define.TemplatePackageMaxEntryBytes)
	// templatePackageMaxTotalBytes bounds the unpacked payload: a package is an untrusted
	// input, so a small gzip must not expand without a ceiling. It is an abuse guard, not the
	// package size limit, hence the multiple of the shipped limit.
	templatePackageMaxTotalBytes = int64(define.TemplatePackageMaxBytes) * 8
)

// templatePackageMaxEntries bounds the entry count. Every entry is buffered in full before the
// byte total gets near the ceiling, so a package of many tiny entries is stopped by this limit
// rather than by templatePackageMaxTotalBytes. Counted generously: a package of a few hundred
// template projects with all their assets stays far below it.
const templatePackageMaxEntries = 1 << 16

// Asset key rules: the business segment names a lib_storage namespace (ObjectKey concatenates
// it raw into the object key), the extension and the content hash keep the entry name safe.
var (
	templateAssetExtPattern = regexp.MustCompile(`^[a-z0-9]{1,20}$`)
	templateAssetHexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	// templateAssetNamePattern matches the entry base name, i.e. the truncated content hash.
	templateAssetNamePattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
	// templateAssetRefPattern matches one asset reference inside a JSON value or content
	// string; the index decides whether a match is really a reference.
	templateAssetRefPattern = regexp.MustCompile(`assets/[0-9a-f]{32}\.[a-z0-9]{1,20}`)
	templateAssetBusinesses = map[string]bool{
		lib_storage.BusinessEnterprise:    true,
		lib_storage.BusinessPipeline:      true,
		lib_storage.BusinessWorkspace:     true,
		lib_storage.BusinessRequirement:   true,
		lib_storage.BusinessKnowledgeBase: true,
	}
)

// templatePackageManifest is manifest.json: the self-description of the package.
type templatePackageManifest struct {
	GeneratedAt       int64 `json:"generated_at"`
	TemplateAdminID   int   `json:"template_admin_id"`
	WorkspaceIDs      []int `json:"workspace_ids"`
	AssetCount        int   `json:"asset_count"`
	MissingAssetCount int   `json:"missing_asset_count"`
}

// templateAssetIndexRow is one assets/index.json value, keyed by the package path of the asset.
// That path is the only reference package rows and content carry: the archive is self-contained,
// so no value in it points at the template account's storage.
type templateAssetIndexRow struct {
	Business    string `json:"business"`
	Ext         string `json:"ext"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
	StoredName  string `json:"stored_name"`
	Sha256      string `json:"sha256"`
}

// TemplateAsset is the file metadata of a package asset, matching what the destination
// tenant's enterprise file ledger records for a migrated object.
type TemplateAsset struct {
	Business    string
	Ext         string
	FileName    string
	StoredName  string
	ContentType string
	FileSize    int64
}

// TemplatePackage is a loaded, validated, immutable package. Requests hold the instance they
// loaded, so replacing the file on disk never mixes two package versions inside one copy.
type TemplatePackage struct {
	templateAdminID int
	workspaces      []msql.Params // ended_at = 0, sort_order asc, id desc
	workspaceIDs    []int         // same order as workspaces
	allowedWsIDs    map[int]bool  // the only workspace ids a source may be built for
	sources         map[int]map[string][]msql.Params
	poolTemplates   map[int]msql.Params
	assets          map[string]TemplateAsset // asset ref (package path) -> metadata
	assetSha        map[string]string        // asset ref -> sha256
	assetData       map[string][]byte        // sha256 -> bytes
}

// Workspaces returns the package's active template projects, ordered like the template
// account's own list (sort_order asc, id desc). The returned slice is a copy.
func (p *TemplatePackage) Workspaces() []msql.Params {
	return copyTemplateRows(p.workspaces)
}

// WorkspaceIDs returns the active template project ids in Workspaces order.
func (p *TemplatePackage) WorkspaceIDs() []int {
	ids := make([]int, len(p.workspaceIDs))
	copy(ids, p.workspaceIDs)
	return ids
}

// TemplateAdminID returns the admin id the package was exported from.
func (p *TemplatePackage) TemplateAdminID() int {
	return p.templateAdminID
}

// WorkspaceIcon returns the package path of one project's avatar and whether the package carries
// its bytes. A project without an avatar and one whose avatar the export could not read both
// answer false, and the caller then falls back to the letter avatar instead of a dead URL.
func (p *TemplatePackage) WorkspaceIcon(wsID int) (string, bool) {
	if !p.allowedWsIDs[wsID] {
		return ``, false
	}
	for _, row := range p.workspaces {
		if cast.ToInt(row[`id`]) != wsID {
			continue
		}
		ref := row[`icon`]
		if _, carried := p.assets[ref]; !carried {
			return ``, false
		}
		return ref, true
	}
	return ``, false
}

// NewSource returns a reader for one template project. The id must be part of the package's
// active workspace set; the check lives here so no caller can read outside the package by
// passing an arbitrary workspace id.
func (p *TemplatePackage) NewSource(wsID int) (*TemplateSource, error) {
	if !p.allowedWsIDs[wsID] {
		return nil, fmt.Errorf(`%w: workspace %d`, ErrTemplatePackageWorkspace, wsID)
	}
	return &TemplateSource{pkg: p, wsID: wsID}, nil
}

// TemplateSource reads one template project's rows and assets out of a loaded package.
type TemplateSource struct {
	pkg  *TemplatePackage
	wsID int // always a member of the package's allowed workspace set
}

// Rows returns the package rows of one source key. An unknown key or a missing file is an
// error: an empty result would silently copy an empty project. The returned slice and its
// maps are fresh copies, so a caller may rewrite intermediate structures (parent_id
// remapping) without touching the cached package.
func (s *TemplateSource) Rows(key string) ([]msql.Params, error) {
	if _, ok := templateTableByName(key); !ok {
		return nil, fmt.Errorf(`%w: unknown source %q`, ErrTemplatePackageMissing, key)
	}
	rows, ok := s.pkg.sources[s.wsID][key]
	if !ok {
		return nil, fmt.Errorf(`%w: source %q missing for workspace %d`, ErrTemplatePackageMissing, key, s.wsID)
	}
	return copyTemplateRows(rows), nil
}

// PoolTemplate returns one tenant-level pool template carried by the package. The returned
// map is a copy.
func (s *TemplateSource) PoolTemplate(templateID int) (msql.Params, bool) {
	row, ok := s.pkg.poolTemplates[templateID]
	if !ok {
		return nil, false
	}
	return copyTemplateRow(row), true
}

// HasAsset reports whether the package carries the asset a row or content refers to by its
// package path. The index is the only authority an offline package offers: whether the same
// value would parse as an object of the current storage deployment is a different question and
// must not decide whether the asset migrates.
func (s *TemplateSource) HasAsset(ref string) bool {
	_, ok := s.pkg.assets[ref]
	return ok
}

// ReadAsset returns the bytes of one package asset and its file metadata. ref is the package
// path found in package rows; ok is false for references the package does not carry.
func (s *TemplateSource) ReadAsset(ref string) ([]byte, TemplateAsset, bool) {
	asset, ok := s.pkg.assets[ref]
	if !ok {
		return nil, TemplateAsset{}, false
	}
	sha, ok := s.pkg.assetSha[ref]
	if !ok {
		return nil, TemplateAsset{}, false
	}
	data, ok := s.pkg.assetData[sha]
	if !ok {
		return nil, TemplateAsset{}, false
	}
	return data, asset, true
}

// ======================== Loading ========================

type templatePackageCacheKey struct {
	path  string
	mtime int64
	size  int64
}

type templatePackageCacheEntry struct {
	valid   bool
	key     templatePackageCacheKey
	pkg     *TemplatePackage
	err     error
	retryAt time.Time
}

// templatePackageCache keeps the last load (success or failure) keyed by path, mtime and size.
// Caching failures too keeps a missing or unreadable package from re-reading (and re-logging)
// on every project creation; a changed file always invalidates the entry.
var templatePackageCache struct {
	mu    sync.RWMutex
	entry templatePackageCacheEntry
}

// LoadTemplatePackage loads, validates and caches the template package.
func LoadTemplatePackage() (*TemplatePackage, error) {
	file := templatePackagePath()
	key := templatePackageCacheKey{path: file}
	if info, err := os.Stat(file); err == nil {
		key.mtime = info.ModTime().UnixNano()
		key.size = info.Size()
	}
	if pkg, err, ok := lookupTemplatePackageCache(key); ok {
		return pkg, err
	}
	pkg, err := readTemplatePackage(file)
	storeTemplatePackageCache(key, pkg, err)
	return pkg, err
}

// CheckTemplatePackage validates the template package at process start. A package this
// deployment cannot read is logged, never fatal: creating a project still succeeds on the seed
// configuration, but the template list stays empty until the file is in place.
func CheckTemplatePackage() error {
	pkg, err := LoadTemplatePackage()
	if err != nil {
		logTemplatePackageUnavailable(`startup check`, err)
		return err
	}
	logs.Info(`template package ready: admin %d workspaces %d assets %d`,
		pkg.TemplateAdminID(), len(pkg.workspaceIDs), len(pkg.assets))
	return nil
}

// templatePackagePath resolves the package location against the process working directory.
func templatePackagePath() string {
	if filepath.IsAbs(templatePackageFile) {
		return templatePackageFile
	}
	return filepath.Join(define.AppRoot, templatePackageFile)
}

func lookupTemplatePackageCache(key templatePackageCacheKey) (*TemplatePackage, error, bool) {
	templatePackageCache.mu.RLock()
	defer templatePackageCache.mu.RUnlock()
	entry := templatePackageCache.entry
	if !entry.valid || entry.key != key {
		return nil, nil, false
	}
	if entry.err == nil {
		return entry.pkg, nil, true
	}
	if time.Now().Before(entry.retryAt) {
		return nil, entry.err, true
	}
	return nil, nil, false
}

func storeTemplatePackageCache(key templatePackageCacheKey, pkg *TemplatePackage, err error) {
	entry := templatePackageCacheEntry{valid: true, key: key, pkg: pkg, err: err}
	if err != nil {
		entry.retryAt = time.Now().Add(templatePackageRetryDelay)
	}
	templatePackageCache.mu.Lock()
	templatePackageCache.entry = entry
	templatePackageCache.mu.Unlock()
}

// resetTemplatePackageCache drops the cached entry; tests use it to isolate cases.
func resetTemplatePackageCache() {
	templatePackageCache.mu.Lock()
	templatePackageCache.entry = templatePackageCacheEntry{}
	templatePackageCache.mu.Unlock()
}

// readTemplatePackage reads the whole archive into memory, rejects anything outside the
// package format, and cross-validates manifest, workspaces and data directories. A file at the
// configured path wins when present (so a build/test can point at a fixture); when it is absent
// the copy embedded in the binary is used, so a deployment that ships no assets directory (the
// commercial package) still carries the default template projects.
func readTemplatePackage(file string) (*TemplatePackage, error) {
	raw, err := templatePackageBytes(file)
	if err != nil {
		return nil, err
	}
	return parseTemplatePackage(raw)
}

// templatePackageBytes returns the package bytes: the on-disk file when it exists, otherwise the
// embedded default. A file that exists but cannot be read is an error, never a silent fallback.
func templatePackageBytes(file string) ([]byte, error) {
	raw, err := os.ReadFile(file)
	if err == nil {
		if size := int64(len(raw)); size <= 0 || size > templatePackageMaxBytes {
			return nil, fmt.Errorf(`%w: package size %d is outside the limit`, ErrTemplatePackageMissing, size)
		}
		return raw, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf(`%w: read %s: %v`, ErrTemplatePackageMissing, file, err)
	}
	if size := int64(len(templateassets.Package)); size > 0 && size <= templatePackageMaxBytes {
		logs.Info(`template package: no file at %s, using the package embedded in the binary`, file)
		return templateassets.Package, nil
	}
	return nil, fmt.Errorf(`%w: stat %s: %v`, ErrTemplatePackageMissing, file, err)
}

// parseTemplatePackage decodes the archive bytes; it is separate from the file read so tests
// and the export self-check can validate bytes without touching the filesystem.
func parseTemplatePackage(raw []byte) (*TemplatePackage, error) {
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf(`%w: gzip: %v`, ErrTemplatePackageMissing, err)
	}
	defer func() { _ = gz.Close() }()

	var (
		manifestRaw   []byte
		workspacesRaw []byte
		poolRaw       []byte
		indexRaw      []byte
		sourceJSON    = make(map[int]map[string][]byte)
		assetFiles    = make(map[string][]byte)
		seen          = make(map[string]bool)
		total         int64
	)
	reader := tar.NewReader(gz)
	entries := 0
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf(`%w: tar: %v`, ErrTemplatePackageMissing, err)
		}
		if entries++; entries > templatePackageMaxEntries {
			return nil, fmt.Errorf(`%w: entry count exceeds the limit`, ErrTemplatePackageMissing)
		}
		name := header.Name
		if !validTemplateEntryName(name) {
			return nil, fmt.Errorf(`%w: rejected entry name %q`, ErrTemplatePackageMissing, name)
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return nil, fmt.Errorf(`%w: entry %q is not a regular file`, ErrTemplatePackageMissing, name)
		}
		if header.Linkname != `` {
			return nil, fmt.Errorf(`%w: entry %q carries a link target`, ErrTemplatePackageMissing, name)
		}
		if seen[name] {
			return nil, fmt.Errorf(`%w: duplicate entry %q`, ErrTemplatePackageMissing, name)
		}
		seen[name] = true
		if header.Size < 0 || header.Size > templatePackageMaxEntryBytes {
			return nil, fmt.Errorf(`%w: entry %q size %d is outside the limit`, ErrTemplatePackageMissing, name, header.Size)
		}
		data, err := io.ReadAll(io.LimitReader(reader, templatePackageMaxEntryBytes+1))
		if err != nil {
			return nil, fmt.Errorf(`%w: read entry %q: %v`, ErrTemplatePackageMissing, name, err)
		}
		if int64(len(data)) > templatePackageMaxEntryBytes {
			return nil, fmt.Errorf(`%w: entry %q exceeds the entry limit`, ErrTemplatePackageMissing, name)
		}
		total += int64(len(data))
		if total > templatePackageMaxTotalBytes {
			return nil, fmt.Errorf(`%w: unpacked size exceeds the limit`, ErrTemplatePackageMissing)
		}
		kind, wsID, key := classifyTemplateEntry(name)
		switch kind {
		case templateEntryManifest:
			manifestRaw = data
		case templateEntryWorkspaces:
			workspacesRaw = data
		case templateEntryPoolTemplates:
			poolRaw = data
		case templateEntrySource:
			if sourceJSON[wsID] == nil {
				sourceJSON[wsID] = make(map[string][]byte)
			}
			sourceJSON[wsID][key] = data
		case templateEntryAssetIndex:
			indexRaw = data
		case templateEntryAssetFile:
			assetFiles[name] = data
		case templateEntryDevMarker:
			// A dev-export marker: no package data, deliberately ignored so a package exported
			// outside production can still be loaded and verified locally.
		case templateEntryInvalid:
			return nil, fmt.Errorf(`%w: entry %q is not part of the package format`, ErrTemplatePackageMissing, name)
		}
	}
	if manifestRaw == nil {
		return nil, fmt.Errorf(`%w: %s is missing`, ErrTemplatePackageMissing, templateManifestEntry)
	}
	if workspacesRaw == nil {
		return nil, fmt.Errorf(`%w: %s is missing`, ErrTemplatePackageMissing, templateWorkspacesEntry)
	}
	if indexRaw == nil {
		return nil, fmt.Errorf(`%w: %s is missing`, ErrTemplatePackageMissing, templateAssetIndexEntry)
	}

	manifest, err := decodeTemplateManifest(manifestRaw)
	if err != nil {
		return nil, err
	}
	var workspaces []msql.Params
	if err := json.Unmarshal(workspacesRaw, &workspaces); err != nil {
		return nil, fmt.Errorf(`%w: %s is not valid JSON: %v`, ErrTemplatePackageMissing, templateWorkspacesEntry, err)
	}
	poolRows, err := decodeTemplatePoolTemplates(poolRaw)
	if err != nil {
		return nil, err
	}
	index, err := decodeTemplateAssetIndex(indexRaw)
	if err != nil {
		return nil, err
	}
	sources, err := decodeTemplateSources(sourceJSON)
	if err != nil {
		return nil, err
	}
	assets, assetSha, assetData, err := validateTemplateAssets(index, assetFiles)
	if err != nil {
		return nil, err
	}
	return buildTemplatePackage(manifest, workspaces, sources, poolRows, assets, assetSha, assetData)
}

// decodeTemplateManifest parses the manifest.
func decodeTemplateManifest(raw []byte) (templatePackageManifest, error) {
	var manifest templatePackageManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return manifest, fmt.Errorf(`%w: %s is not valid JSON: %v`, ErrTemplatePackageMissing, templateManifestEntry, err)
	}
	if manifest.TemplateAdminID <= 0 {
		return manifest, fmt.Errorf(`%w: %s carries no template admin id`, ErrTemplatePackageMissing, templateManifestEntry)
	}
	return manifest, nil
}

// decodeTemplatePoolTemplates parses work_item_templates.json; an absent file means a package
// without pool templates.
func decodeTemplatePoolTemplates(raw []byte) ([]msql.Params, error) {
	if raw == nil {
		return nil, nil
	}
	var rows []msql.Params
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf(`%w: %s is not valid JSON: %v`, ErrTemplatePackageMissing, templatePoolEntry, err)
	}
	return rows, nil
}

// decodeTemplateAssetIndex parses assets/index.json into its keyed form.
func decodeTemplateAssetIndex(raw []byte) (map[string]templateAssetIndexRow, error) {
	var index map[string]templateAssetIndexRow
	if err := json.Unmarshal(raw, &index); err != nil {
		return nil, fmt.Errorf(`%w: %s is not valid JSON: %v`, ErrTemplatePackageMissing, templateAssetIndexEntry, err)
	}
	return index, nil
}

// decodeTemplateSources parses every data/<workspace_id>/<key>.json file. Each project
// directory must carry exactly the known sources: a missing file would silently copy an empty
// project, an unknown one means the package was not produced by this format.
func decodeTemplateSources(raw map[int]map[string][]byte) (map[int]map[string][]msql.Params, error) {
	sources := make(map[int]map[string][]msql.Params, len(raw))
	for wsID, files := range raw {
		for key := range files {
			if _, ok := templateTableByName(key); !ok {
				return nil, fmt.Errorf(`%w: unknown source %q under data/%d`, ErrTemplatePackageMissing, key, wsID)
			}
		}
		decoded := make(map[string][]msql.Params, len(templateTables))
		for _, table := range templateTables {
			data, ok := files[table.Name]
			if !ok {
				return nil, fmt.Errorf(`%w: source %q missing for workspace %d`, ErrTemplatePackageMissing, table.Name, wsID)
			}
			var rows []msql.Params
			if err := json.Unmarshal(data, &rows); err != nil {
				return nil, fmt.Errorf(`%w: data/%d/%s.json is not valid JSON: %v`, ErrTemplatePackageMissing, wsID, table.Name, err)
			}
			decoded[table.Name] = rows
		}
		sources[wsID] = decoded
	}
	return sources, nil
}

// validateTemplateAssets checks the index against the asset entries: reference shape, hashes and
// the business namespace. An index row that cannot be used (foreign business, malformed ext) is
// dropped with a warning instead of corrupting the package — ReadAsset then reports it as
// unavailable, exactly like an object missing on the template side today.
func validateTemplateAssets(index map[string]templateAssetIndexRow, files map[string][]byte) (
	map[string]TemplateAsset, map[string]string, map[string][]byte, error) {
	assets := make(map[string]TemplateAsset, len(index))
	assetSha := make(map[string]string, len(index))
	assetData := make(map[string][]byte)
	referenced := make(map[string]bool, len(index))
	for ref, entry := range index {
		if kind, _, _ := classifyTemplateEntry(ref); kind != templateEntryAssetFile {
			return nil, nil, nil, fmt.Errorf(`%w: %s key %q is not an asset path`,
				ErrTemplatePackageMissing, templateAssetIndexEntry, ref)
		}
		referenced[ref] = true
		if !templateAssetBusinesses[entry.Business] || !templateAssetExtPattern.MatchString(entry.Ext) {
			logs.Warning(`template package: asset [%s] business [%s] ext [%s] outside the storage whitelist, ignored`,
				ref, entry.Business, entry.Ext)
			continue
		}
		if !templateAssetHexPattern.MatchString(entry.Sha256) {
			return nil, nil, nil, fmt.Errorf(`%w: asset %q carries an invalid sha256`, ErrTemplatePackageMissing, ref)
		}
		if ref != templateAssetEntryPath(entry.Sha256, entry.Ext) {
			return nil, nil, nil, fmt.Errorf(`%w: asset %q does not match its content hash and extension`,
				ErrTemplatePackageMissing, ref)
		}
		data, ok := files[ref]
		if !ok {
			return nil, nil, nil, fmt.Errorf(`%w: asset %q entry is missing`, ErrTemplatePackageMissing, ref)
		}
		if sha := templateSha256Hex(data); sha != entry.Sha256 {
			return nil, nil, nil, fmt.Errorf(`%w: asset %q content does not match its hash`, ErrTemplatePackageMissing, ref)
		}
		assets[ref] = TemplateAsset{
			Business:    entry.Business,
			Ext:         entry.Ext,
			FileName:    entry.FileName,
			StoredName:  entry.StoredName,
			ContentType: entry.ContentType,
			FileSize:    entry.FileSize,
		}
		assetSha[ref] = entry.Sha256
		assetData[entry.Sha256] = data
	}
	for name := range files {
		if !referenced[name] {
			return nil, nil, nil, fmt.Errorf(`%w: asset entry %q is not referenced by the index`, ErrTemplatePackageMissing, name)
		}
	}
	return assets, assetSha, assetData, nil
}

// buildTemplatePackage assembles the package and cross-validates its three sources of truth:
// manifest.workspace_ids, workspaces[].id (ended_at = 0) and the data/ directories.
func buildTemplatePackage(manifest templatePackageManifest, workspaces []msql.Params,
	sources map[int]map[string][]msql.Params, poolRows []msql.Params,
	assets map[string]TemplateAsset, assetSha map[string]string, assetData map[string][]byte) (*TemplatePackage, error) {
	pkg := &TemplatePackage{
		templateAdminID: manifest.TemplateAdminID,
		allowedWsIDs:    make(map[int]bool),
		sources:         make(map[int]map[string][]msql.Params),
		poolTemplates:   make(map[int]msql.Params),
		assets:          assets,
		assetSha:        assetSha,
		assetData:       assetData,
	}
	active := make([]msql.Params, 0, len(workspaces))
	seenWorkspace := make(map[int]bool, len(workspaces))
	for _, row := range workspaces {
		wsID := cast.ToInt(row[`id`])
		if wsID <= 0 {
			return nil, fmt.Errorf(`%w: workspace row without id`, ErrTemplatePackageMissing)
		}
		if seenWorkspace[wsID] {
			return nil, fmt.Errorf(`%w: duplicate workspace %d`, ErrTemplatePackageMissing, wsID)
		}
		seenWorkspace[wsID] = true
		if cast.ToInt(row[`admin_id`]) != manifest.TemplateAdminID {
			return nil, fmt.Errorf(`%w: workspace %d belongs to another admin`, ErrTemplatePackageMissing, wsID)
		}
		if cast.ToInt(row[`ended_at`]) != 0 {
			continue
		}
		// The list endpoint converts every active row with toWorkspaceModel. Doing the same
		// conversion here means a row it cannot use fails the load, so a bad package yields an
		// empty list instead of a failing request.
		if _, err := toWorkspaceModel(row); err != nil {
			return nil, fmt.Errorf(`%w: workspace %d row is unusable: %v`, ErrTemplatePackageMissing, wsID, err)
		}
		active = append(active, row)
		pkg.allowedWsIDs[wsID] = true
	}
	// Ended projects never take part: only active rows may appear in the manifest and must
	// have a data directory.
	manifestIDs := make(map[int]bool, len(manifest.WorkspaceIDs))
	for _, wsID := range manifest.WorkspaceIDs {
		if wsID <= 0 || manifestIDs[wsID] {
			return nil, fmt.Errorf(`%w: manifest workspace ids are malformed`, ErrTemplatePackageMissing)
		}
		if !pkg.allowedWsIDs[wsID] {
			return nil, fmt.Errorf(`%w: manifest lists inactive workspace %d`, ErrTemplatePackageMissing, wsID)
		}
		manifestIDs[wsID] = true
	}
	if len(manifestIDs) != len(pkg.allowedWsIDs) {
		return nil, fmt.Errorf(`%w: manifest workspace ids do not match %s`, ErrTemplatePackageMissing, templateWorkspacesEntry)
	}
	for wsID := range sources {
		if !manifestIDs[wsID] {
			return nil, fmt.Errorf(`%w: data/%d is outside the manifest workspace ids`, ErrTemplatePackageMissing, wsID)
		}
		pkg.sources[wsID] = sources[wsID]
	}
	for wsID := range manifestIDs {
		if _, ok := pkg.sources[wsID]; !ok {
			return nil, fmt.Errorf(`%w: data/%d directory is missing`, ErrTemplatePackageMissing, wsID)
		}
	}
	// Listing order is the template account's own order and is verified rather than trusted.
	sort.SliceStable(active, func(i, j int) bool {
		return templateWorkspaceOrderLess(active[i], active[j])
	})
	pkg.workspaces = active
	pkg.workspaceIDs = make([]int, 0, len(active))
	for _, row := range active {
		pkg.workspaceIDs = append(pkg.workspaceIDs, cast.ToInt(row[`id`]))
	}
	for _, row := range poolRows {
		templateID := cast.ToInt(row[`id`])
		if templateID <= 0 {
			return nil, fmt.Errorf(`%w: pool template row without id`, ErrTemplatePackageMissing)
		}
		pkg.poolTemplates[templateID] = row
	}
	return pkg, nil
}

// templateWorkspaceOrderLess orders workspace rows like the template account's project list:
// manual sort_order first, newest id before older ones.
func templateWorkspaceOrderLess(left, right msql.Params) bool {
	leftOrder := cast.ToInt(left[`sort_order`])
	rightOrder := cast.ToInt(right[`sort_order`])
	if leftOrder != rightOrder {
		return leftOrder < rightOrder
	}
	return cast.ToInt(left[`id`]) > cast.ToInt(right[`id`])
}

// ======================== Entry names ========================

type templateEntryKind int

const (
	templateEntryInvalid templateEntryKind = iota
	templateEntryManifest
	templateEntryWorkspaces
	templateEntryPoolTemplates
	templateEntrySource
	templateEntryAssetIndex
	templateEntryAssetFile
	templateEntryDevMarker
)

// validTemplateEntryName rejects absolute paths, path traversal, links and Windows drive
// prefixes before any entry is interpreted (tar slip).
func validTemplateEntryName(name string) bool {
	if name == `` || strings.HasPrefix(name, `/`) || strings.HasPrefix(name, `\`) {
		return false
	}
	if strings.Contains(name, `\`) || strings.Contains(name, `:`) {
		return false
	}
	if strings.Contains(name, `..`) {
		return false
	}
	if path.IsAbs(name) || filepath.IsAbs(name) {
		return false
	}
	if path.Clean(name) != name {
		return false
	}
	return true
}

// classifyTemplateEntry maps an entry name to its role. Only the exact names of the format are
// accepted; data file base names are validated against the source list by the caller.
func classifyTemplateEntry(name string) (templateEntryKind, int, string) {
	switch name {
	case templateManifestEntry:
		return templateEntryManifest, 0, ``
	case templateWorkspacesEntry:
		return templateEntryWorkspaces, 0, ``
	case templatePoolEntry:
		return templateEntryPoolTemplates, 0, ``
	case templateAssetIndexEntry:
		return templateEntryAssetIndex, 0, ``
	case templateDevMarkerEntry:
		return templateEntryDevMarker, 0, ``
	}
	if rest, ok := strings.CutPrefix(name, templateDataDir+`/`); ok {
		parts := strings.Split(rest, `/`)
		if len(parts) != 2 {
			return templateEntryInvalid, 0, ``
		}
		wsID, ok := parseTemplateWorkspaceSegment(parts[0])
		if !ok {
			return templateEntryInvalid, 0, ``
		}
		key, ok := strings.CutSuffix(parts[1], `.json`)
		if !ok || key == `` {
			return templateEntryInvalid, 0, ``
		}
		return templateEntrySource, wsID, key
	}
	if rest, ok := strings.CutPrefix(name, templateAssetsDir+`/`); ok {
		parts := strings.Split(rest, `/`)
		if len(parts) != 1 || parts[0] == `` {
			return templateEntryInvalid, 0, ``
		}
		base, ext, ok := strings.Cut(parts[0], `.`)
		if !ok || !templateAssetNamePattern.MatchString(base) {
			return templateEntryInvalid, 0, ``
		}
		if !templateAssetExtPattern.MatchString(ext) {
			return templateEntryInvalid, 0, ``
		}
		return templateEntryAssetFile, 0, ``
	}
	return templateEntryInvalid, 0, ``
}

// parseTemplateWorkspaceSegment accepts the canonical decimal form of a positive workspace id,
// i.e. exactly what templateDataEntry writes. Anything else would alias an existing directory
// (data/007 vs data/7) while staying a distinct entry name, so the duplicate check would miss
// it and one source file would silently replace another.
func parseTemplateWorkspaceSegment(segment string) (int, bool) {
	value, err := strconv.Atoi(segment)
	if err != nil || value <= 0 || strconv.Itoa(value) != segment {
		return 0, false
	}
	return value, true
}

// templateDataEntry is the package path of one source file.
func templateDataEntry(wsID int, key string) string {
	return templateDataDir + `/` + strconv.Itoa(wsID) + `/` + key + `.json`
}

// templateAssetEntryPath is the package path of one asset: the content hash names the file,
// so identical bytes are stored once and a name always maps to exactly one content.
func templateAssetEntryPath(sha, ext string) string {
	return templateAssetsDir + `/` + sha[:32] + `.` + ext
}

// templateSha256Hex returns the lowercase hex sha256 of data.
func templateSha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ======================== Row helpers ========================

// copyTemplateRows deep-copies rows: the cached package is immutable, so callers may restructure
// what they read (parent id remapping, deleted keys) without corrupting the cache.
func copyTemplateRows(rows []msql.Params) []msql.Params {
	if rows == nil {
		return nil
	}
	out := make([]msql.Params, len(rows))
	for i, row := range rows {
		out[i] = copyTemplateRow(row)
	}
	return out
}

func copyTemplateRow(row msql.Params) msql.Params {
	if row == nil {
		return nil
	}
	out := make(msql.Params, len(row))
	for key, value := range row {
		out[key] = value
	}
	return out
}

// ======================== Writing ========================

// templatePackageWriter assembles a package in memory using the same entry rules the reader
// validates, keeping a written package and a readable package one format.
type templatePackageWriter struct {
	buffer bytes.Buffer
	gzip   *gzip.Writer
	tar    *tar.Writer
	now    int64
}

// newTemplatePackageWriter starts a package whose entries carry the given timestamp.
func newTemplatePackageWriter(now int64) *templatePackageWriter {
	writer := &templatePackageWriter{now: now}
	writer.gzip = gzip.NewWriter(&writer.buffer)
	writer.tar = tar.NewWriter(writer.gzip)
	return writer
}

// writeJSON marshals value and appends it as one entry.
func (w *templatePackageWriter) writeJSON(name string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf(`marshal template package entry %s: %w`, name, err)
	}
	return w.writeEntry(name, data)
}

// writeEntry appends one regular file. The size limit is enforced here so an oversized entry
// fails the export instead of producing a package the reader would reject.
func (w *templatePackageWriter) writeEntry(name string, data []byte) error {
	if !validTemplateEntryName(name) {
		return fmt.Errorf(`template package: invalid entry name %q`, name)
	}
	if int64(len(data)) > templatePackageMaxEntryBytes {
		return fmt.Errorf(`%w: entry %s is %d bytes`, ErrTemplatePackageTooLarge, name, len(data))
	}
	header := &tar.Header{
		Name:     name,
		Mode:     templatePackageMode,
		Size:     int64(len(data)),
		ModTime:  time.Unix(w.now, 0),
		Typeflag: tar.TypeReg,
	}
	if err := w.tar.WriteHeader(header); err != nil {
		return fmt.Errorf(`write template package entry %s: %w`, name, err)
	}
	if _, err := w.tar.Write(data); err != nil {
		return fmt.Errorf(`write template package entry %s: %w`, name, err)
	}
	return nil
}

// finish closes the archive and returns the compressed bytes, refusing to hand back a package
// above the size limit.
func (w *templatePackageWriter) finish() ([]byte, error) {
	if err := w.tar.Close(); err != nil {
		return nil, fmt.Errorf(`close template package: %w`, err)
	}
	if err := w.gzip.Close(); err != nil {
		return nil, fmt.Errorf(`close template package: %w`, err)
	}
	if data := w.buffer.Bytes(); int64(len(data)) > templatePackageMaxBytes {
		return nil, fmt.Errorf(`%w: package is %d bytes`, ErrTemplatePackageTooLarge, len(data))
	}
	return w.buffer.Bytes(), nil
}

// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package lib_storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"goteams/internal/app/goteams/define"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/tool"
)

// Business identifier used as the leading directory segment of the object key (business/adminID/date/random.ext).
const (
	BusinessEnterprise      = `enterprise`
	BusinessPersonalization = `personalization`
	BusinessPipeline        = `pipeline`
	BusinessExpertGroup     = `expert_group`
	BusinessWorkspace       = `workspace`
	BusinessRequirement     = `requirement`
	BusinessKnowledgeBase   = `knowledge_base`
	BusinessDashboard       = `dashboard`
	// BusinessSessionLog is the object-key namespace for encrypted client session logs (需求 1896 优化点3).
	BusinessSessionLog = `session-logs`
	BusinessTaskDocs   = `task-docs`
)

// IsObjectKeyShape reports whether key has the shape of an object this system wrote: at least four
// segments with a business directory first (current {business}/{adminID}/{date}/{rand}.{ext}) or
// second (the layout written before the swap). A host serving more than storage can hand back
// paths that parse but are not keys, so callers matching a URL by host alone check this first.
func IsObjectKeyShape(key string) bool {
	parts := strings.Split(key, `/`)
	if len(parts) < 4 || parts[3] == `` {
		return false
	}
	// Upload is the only writer of object keys and every caller passes a Business* constant, so a
	// key whose first two segments are none of them did not come from this system.
	return isBusinessSegment(parts[0]) || isBusinessSegment(parts[1])
}

// isBusinessSegment reports whether seg is one of the business directories an object key can use.
func isBusinessSegment(seg string) bool {
	switch seg {
	case BusinessEnterprise, BusinessPersonalization, BusinessPipeline, BusinessWorkspace,
		BusinessRequirement, BusinessKnowledgeBase, BusinessDashboard, BusinessTaskDocs:
		return true
	}
	return false
}

// Local upload root directory (relative to AppRoot, used for disk read/write).
const uploadRoot = `data/uploads`

// urlPrefix is the accessible URL prefix for local storage values (leading slash, resolvable under the current domain).
// Stored value format: "/uploads/{business}/{adminID}/{date}/{rand}.{ext}"; the actual disk path is data/uploads/{business}/...
const urlPrefix = `/uploads/`

// Backend identifier (used only to distinguish storage location when parsing stored_path; no longer encoded into the stored value).
const (
	backendLocal = `local`
	backendOss   = `oss`
)

// OSS URL styles.
const (
	urlStylePath          = `path`
	urlStyleVirtualHosted = `virtual_hosted`
	urlStyleCNAME         = `cname`
)

// Storage errors.
var (
	// ErrNotFound indicates the stored object does not exist.
	ErrNotFound = errors.New(`storage object not found`)
	// ErrNotOwned indicates that a stored path is outside the caller's tenant/business namespace.
	ErrNotOwned = errors.New(`storage object not owned`)
)

// ossWriteTimeout bounds a single object write. A single upload may be 200MB, and on a slow link the
// previous 30s expired before the object was written (context deadline exceeded). It is aligned with
// the frontend upload timeouts and the proxy timeouts of the nginx upload locations.
const ossWriteTimeout = 10 * time.Minute

// Manager is the storage manager: uploads to oss-s3 (S3-compatible object storage) when configured, otherwise to local disk.
type Manager struct {
	mu     sync.Mutex
	client *minio.Client
}

// M is the global storage manager instance.
var M = &Manager{}

// enabled reports whether oss-s3 is configured (with required parameters present).
func (m *Manager) enabled() bool {
	cfg := define.Config.OssS3
	if cfg == nil {
		return false
	}
	return cfg[`enable`] == `true` && cfg[`endpoint`] != `` && cfg[`bucket`] != ``
}

func (m *Manager) bucket() string {
	return define.Config.OssS3[`bucket`]
}

// ValidateConfig validates the enabled oss-s3 URL configuration.
func ValidateConfig(cfg map[string]string) error {
	style := normalizedURLStyle(cfg[`url_style`])
	switch style {
	case urlStylePath, urlStyleVirtualHosted:
	case urlStyleCNAME:
		if _, err := configuredHost(cfg[`custom_domain`]); err != nil {
			return fmt.Errorf(`invalid oss-s3 custom_domain: %w`, err)
		}
	default:
		return fmt.Errorf(`invalid oss-s3 url_style %q`, cfg[`url_style`])
	}
	if _, err := configuredHost(cfg[`endpoint`]); err != nil {
		return fmt.Errorf(`invalid oss-s3 endpoint: %w`, err)
	}
	if strings.TrimSpace(cfg[`bucket`]) == `` {
		return errors.New(`invalid oss-s3 bucket: value is required`)
	}
	// public_endpoint is optional; validate format when configured
	if pubEP := strings.TrimSpace(cfg[`public_endpoint`]); pubEP != `` {
		if _, err := configuredHost(pubEP); err != nil {
			return fmt.Errorf(`invalid oss-s3 public_endpoint: %w`, err)
		}
	}
	return nil
}

// objectURL builds the object access URL based on the url_style configuration.
func (m *Manager) objectURL(key string) (string, error) {
	cfg := define.Config.OssS3
	scheme := `https`
	if cfg[`use_ssl`] != `true` {
		scheme = `http`
	}
	// Prefer public_endpoint (external access address) for URL generation; fall back to endpoint when not configured
	rawEndpoint := strings.TrimSpace(cfg[`public_endpoint`])
	if rawEndpoint == `` {
		rawEndpoint = cfg[`endpoint`]
	}
	endpoint, err := configuredHost(rawEndpoint)
	if err != nil {
		return ``, err
	}

	switch normalizedURLStyle(cfg[`url_style`]) {
	case urlStylePath:
		return buildObjectURL(scheme, endpoint, m.bucket(), key), nil
	case urlStyleVirtualHosted:
		return buildObjectURL(scheme, m.bucket()+`.`+endpoint, key), nil
	case urlStyleCNAME:
		customDomain, err := configuredHost(cfg[`custom_domain`])
		if err != nil {
			return ``, err
		}
		return buildObjectURL(scheme, customDomain, key), nil
	default:
		return ``, fmt.Errorf(`invalid oss-s3 url_style %q`, cfg[`url_style`])
	}
}

// normalizedURLStyle returns the normalized URL style; an empty config keeps the legacy path style.
func normalizedURLStyle(style string) string {
	style = strings.ToLower(strings.TrimSpace(style))
	if style == `` {
		return urlStylePath
	}
	return style
}

// configuredHost validates and returns an endpoint or custom domain without scheme/path.
func configuredHost(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == `` {
		return ``, errors.New(`value is required`)
	}
	if strings.Contains(value, `://`) {
		return ``, errors.New(`scheme must be configured with use_ssl`)
	}
	u, err := url.Parse(`//` + value)
	if err != nil || u.Host == `` || u.Path != `` || u.RawQuery != `` || u.Fragment != `` {
		return ``, errors.New(`must contain only a host and optional port`)
	}
	return u.Host, nil
}

// buildObjectURL joins and escapes the object URL path segments.
func buildObjectURL(scheme, host string, pathSegments ...string) string {
	var segments []string
	for _, segment := range pathSegments {
		segment = strings.Trim(segment, `/`)
		if segment != `` {
			segments = append(segments, segment)
		}
	}
	u := &url.URL{Scheme: scheme, Host: host, Path: `/` + strings.Join(segments, `/`)}
	return u.String()
}

// getClient lazily initializes and returns the minio client (internally locked).
func (m *Manager) getClient() (*minio.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.client != nil {
		return m.client, nil
	}
	cfg := define.Config.OssS3
	useSSL := cfg[`use_ssl`] == `true`
	client, err := minio.New(cfg[`endpoint`], &minio.Options{
		Creds:  credentials.NewStaticV4(cfg[`access_key`], cfg[`secret_key`], ``),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}
	m.client = client
	return m.client, nil
}

// ObjectKey generates a unified object key: {business}/{adminID}/{YYYYMMDD}/{random}.{ext}
func ObjectKey(adminID int, business, ext string) string {
	if ext != `` && !strings.HasPrefix(ext, `.`) {
		ext = `.` + ext
	}
	return fmt.Sprintf(`%s/%d/%s/%s%s`, business, adminID, time.Now().Format(`20060102`), tool.Random(24), ext)
}

// DocumentObjectKey 生成流水线产出文档的对象 key：task-docs/{adminID}/{taskID}/{docKey}.{ext}。
// 同一文档内容更新覆盖同一 key（对象 PUT 是原子的，读侧不会读到半写文件），
// 历史版本不入库，需要留痕时由租户在对象存储侧自行开启版本控制。
func DocumentObjectKey(adminID int, taskID int64, docKey, ext string) string {
	if ext != `` && !strings.HasPrefix(ext, `.`) {
		ext = `.` + ext
	}
	return fmt.Sprintf(`%s/%d/%d/%s%s`, BusinessTaskDocs, adminID, taskID, docKey, ext)
}

// UploadRaw stores bytes under a caller-supplied, tenant-scoped object key.
// mediaType 是对象的内容类型；留空按二进制流存，浏览器不会执行它。
// Callers that version encrypted content must use a fresh key per version so a
// failed or stale database update cannot overwrite the currently referenced bytes.
// Returns the stored identifier (full OSS URL when configured, else a /uploads/ path).
func (m *Manager) UploadRaw(key string, content []byte, mediaType string) (string, error) {
	if key == `` {
		return ``, errors.New(`empty object key`)
	}
	if !m.enabled() {
		return m.uploadLocal(key, bytes.NewReader(content))
	}
	if err := ValidateConfig(define.Config.OssS3); err != nil {
		return ``, err
	}
	client, err := m.getClient()
	if err != nil {
		return ``, err
	}
	if mediaType == `` {
		mediaType = `application/octet-stream`
	}
	options := minio.PutObjectOptions{ContentType: mediaType}
	// 未知类型（含 html / svg 等可执行内容）按附件下发，避免在存储域被浏览器内联执行。
	if mediaType == `application/octet-stream` {
		options.ContentDisposition = `attachment`
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err = client.PutObject(ctx, m.bucket(), key, bytes.NewReader(content), int64(len(content)), options); err != nil {
		return ``, fmt.Errorf(`oss upload raw failed: %w`, err)
	}
	return m.objectURL(key)
}

// readerSize returns how many bytes are still readable from r, without consuming any of them.
// It reports -1 when r cannot provide a length, such as a pipe or a socket.
//
// The value selects the minio-go upload strategy: an unknown length (-1) forces
// putObjectMultipartStreamNoLength, which sends parts strictly one after another, while a length of
// at least 16MiB lets putObjectMultipartStreamFromReadAt send them in parallel. A single file may be
// up to 200MB, so the real length has to be reported rather than guessed.
//
// Callers pass a *multipart.File or a *bytes.Reader, both of which implement io.Seeker. The -1
// fallback is kept anyway so an unmeasurable reader degrades to a slow upload instead of a failure.
func readerSize(r io.Reader) (int64, error) {
	seeker, ok := r.(io.Seeker)
	if !ok {
		return -1, nil
	}
	start, err := seeker.Seek(0, io.SeekCurrent)
	if err != nil {
		return -1, nil
	}
	end, err := seeker.Seek(0, io.SeekEnd)
	if err != nil {
		// Genuinely unseekable (pipe, socket, stdin). A failed Seek leaves the position untouched,
		// so the caller can still hand the reader over with an unknown length.
		return -1, nil
	}
	// The rewind is mandatory: minio-go reads from the current position, so a failed rewind would
	// make it read from EOF and silently upload an empty object.
	if _, err = seeker.Seek(start, io.SeekStart); err != nil {
		return 0, fmt.Errorf(`rewind upload reader: %w`, err)
	}
	return end - start, nil
}

// Upload stores a file. When oss-s3 is configured (enabled) it uploads to remote storage and returns a full accessible URL;
// if the upload fails the error is returned directly (no silent fallback to local disk). When oss-s3 is not configured,
// it writes to local storage and returns a prefixed relative URL.
// The return value is stored directly in the business table's stored_path: OSS -> "http(s)://{endpoint}/{bucket}/{key}",
// local -> "/uploads/{business}/{adminID}/{date}/{rand}.{ext}".
// Convention: values starting with http(s):// are OSS; everything else is treated as a local path (possibly with the "/uploads/" prefix);
// the backend type is not encoded in the stored value.
func (m *Manager) Upload(adminID int, business, ext string, r io.Reader) (string, error) {
	key := ObjectKey(adminID, business, ext)
	if !m.enabled() {
		// OSS not configured: store on local disk and return the prefixed relative URL.
		return m.uploadLocal(key, r)
	}
	if err := ValidateConfig(define.Config.OssS3); err != nil {
		return ``, err
	}
	// OSS is configured: upload to remote storage only; any failure returns an error instead of silently falling back to local disk.
	client, err := m.getClient()
	if err != nil {
		return ``, err
	}
	// Hand the caller's reader straight to minio-go instead of buffering the whole object: a single
	// file may be 200MB and that buffer is held for the entire write, so concurrent large uploads
	// would pile up gigabytes of heap. The length comes from the reader itself, otherwise minio-go
	// falls back to sending parts one after another.
	size, err := readerSize(r)
	if err != nil {
		return ``, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), ossWriteTimeout)
	defer cancel()
	options := minio.PutObjectOptions{ContentType: contentType(ext)}
	// 新增格式只作为附件下载，避免 HTML、SVG 等内容在存储域下执行。
	if options.ContentType == `application/octet-stream` {
		options.ContentDisposition = `attachment`
	}
	if _, err = client.PutObject(ctx, m.bucket(), key, r, size, options); err != nil {
		return ``, fmt.Errorf(`oss upload failed: %w`, err)
	}
	stored, err := m.objectURL(key)
	if err != nil {
		// Best-effort cleanup of the orphaned object when URL generation fails.
		if removeErr := client.RemoveObject(ctx, m.bucket(), key, minio.RemoveObjectOptions{}); removeErr != nil {
			logs.Warning(`remove object after URL generation failure: %s`, removeErr.Error())
		}
		return ``, err
	}
	return stored, nil
}

// uploadLocal writes the content to local disk and returns the prefixed relative URL (used when oss-s3 is not configured).
func (m *Manager) uploadLocal(key string, r io.Reader) (string, error) {
	full := filepath.Join(define.AppRoot, uploadRoot, key)
	// Directories and files must be readable/accessible to other users (nginx worker): local uploads are served statically by nginx,
	// goteams writes as root while the nginx container reads as the nginx user; overly strict permissions would cause 403 Forbidden.
	dir := filepath.Dir(full)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ``, err
	}
	// Legacy parent directories may have been created 0750 by old code (no x for others), and MkdirAll does not fix existing directories,
	// so we walk up from the target dir ensuring each is accessible, avoiding nginx being denied at some parent level.
	for d := dir; ; d = filepath.Dir(d) {
		if d == define.AppRoot {
			break
		}
		_ = os.Chmod(d, 0755)
		if d == filepath.Dir(d) {
			break
		}
	}
	f, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return ``, err
	}
	if _, err = io.Copy(f, r); err != nil {
		_ = f.Close()
		_ = os.Remove(full)
		return ``, err
	}
	if err = f.Close(); err != nil {
		return ``, err
	}
	// Belt and suspenders: explicitly fix the file permission (guard against umask) to ensure nginx can read it.
	_ = os.Chmod(full, 0644)
	return urlPrefix + key, nil
}

// ParseStored parses the stored identifier, supporting path, virtual_hosted, cname, and legacy path URLs.
func ParseStored(stored string) (backend, key string, err error) {
	if stored == `` {
		return ``, ``, errors.New(`invalid stored path`)
	}
	if strings.HasPrefix(stored, `http://`) || strings.HasPrefix(stored, `https://`) {
		u, e := url.Parse(stored)
		if e != nil {
			return ``, ``, e
		}
		keyPath := strings.TrimPrefix(u.Path, `/`)
		if keyPath == `` {
			return ``, ``, errors.New(`invalid oss url`)
		}
		cfg := define.Config.OssS3
		bucket := strings.TrimSpace(cfg[`bucket`])
		endpoint, _ := configuredHost(cfg[`endpoint`])
		publicEndpoint, _ := configuredHost(cfg[`public_endpoint`])
		customDomain, _ := configuredHost(cfg[`custom_domain`])
		host := u.Host

		// virtual_hosted URL (also matches public_endpoint generated addresses).
		if bucket != `` {
			virtualPrefix := bucket + `.`
			if len(host) > len(virtualPrefix) && strings.EqualFold(host[:len(virtualPrefix)], virtualPrefix) {
				hostSuffix := host[len(virtualPrefix):]
				if (endpoint != `` && endpointHostsEqual(hostSuffix, endpoint)) ||
					(publicEndpoint != `` && endpointHostsEqual(hostSuffix, publicEndpoint)) {
					return backendOss, keyPath, nil
				}
			}
		}
		// path and legacy path URL (also matches public_endpoint generated addresses).
		if bucket != `` {
			prefix := bucket + `/`
			if strings.HasPrefix(keyPath, prefix) && len(keyPath) > len(prefix) {
				if (endpoint != `` && endpointHostsEqual(host, endpoint)) ||
					(publicEndpoint != `` && endpointHostsEqual(host, publicEndpoint)) {
					return backendOss, keyPath[len(prefix):], nil
				}
			}
		}
		// For a cname URL the path is the object key directly.
		if customDomain != `` && strings.EqualFold(host, customDomain) {
			return backendOss, keyPath, nil
		}
		return ``, ``, errors.New(`invalid oss url`)
	}
	key = strings.TrimPrefix(stored, urlPrefix)
	if key == `` || strings.HasPrefix(key, `/`) || filepath.IsAbs(key) || strings.ContainsRune(key, '\x00') {
		return ``, ``, errors.New(`invalid stored path`)
	}
	cleaned := filepath.Clean(key)
	if cleaned == `.` || cleaned == `..` || strings.HasPrefix(cleaned, `..`+string(filepath.Separator)) {
		return ``, ``, errors.New(`invalid stored path`)
	}
	// Generated object keys never contain navigation segments. Rejecting a non-canonical key
	// keeps every disk operation below uploadRoot, including Copy's source read.
	if filepath.ToSlash(cleaned) != filepath.ToSlash(key) {
		return ``, ``, errors.New(`invalid stored path`)
	}
	return backendLocal, cleaned, nil
}

// endpointHostsEqual compares the endpoint in a URL with the configured endpoint.
func endpointHostsEqual(actual, configured string) bool {
	return strings.EqualFold(actual, configured)
}

// IsOwned checks whether the stored_path belongs to the given tenant and business (security check to prevent unauthorized access).
// Both key layouts are accepted: {business}/{adminID}/ (current) and {adminID}/{business}/ (objects written before the swap).
func (m *Manager) IsOwned(adminID int, business, stored string) bool {
	_, key, err := ParseStored(stored)
	if err != nil {
		return false
	}
	// ParseStored hands back the key with the OS separator, so normalize it before matching prefixes.
	key = filepath.ToSlash(key)
	return strings.HasPrefix(key, fmt.Sprintf(`%s/%d/`, business, adminID)) ||
		strings.HasPrefix(key, fmt.Sprintf(`%d/%s/`, adminID, business))
}

// Serve streams the stored object back to the HTTP client (the frontend need not know the backend type).
func (m *Manager) Serve(stored string, c *gin.Context) error {
	backend, key, err := ParseStored(stored)
	if err != nil {
		return ErrNotFound
	}
	downloadOnly := contentType(filepath.Ext(key)) == `application/octet-stream`
	if backend == backendOss {
		client, err := m.getClient()
		if err != nil {
			return err
		}
		ctx := c.Request.Context()
		obj, err := client.GetObject(ctx, m.bucket(), key, minio.GetObjectOptions{})
		if err != nil {
			return err
		}
		defer obj.Close()
		stat, err := obj.Stat()
		if err != nil {
			return ErrNotFound
		}
		if downloadOnly {
			c.Header(`Content-Type`, `application/octet-stream`)
			c.Header(`Content-Disposition`, `attachment`)
			c.Header(`X-Content-Type-Options`, `nosniff`)
		} else {
			c.Header(`Content-Type`, stat.ContentType)
		}
		c.Header(`Cache-Control`, `no-store`)
		http.ServeContent(c.Writer, c.Request, filepath.Base(key), time.Now(), obj)
		return nil
	}
	// Local
	full := filepath.Join(define.AppRoot, uploadRoot, key)
	if _, err := os.Stat(full); err != nil {
		return ErrNotFound
	}
	if downloadOnly {
		c.Header(`Content-Type`, `application/octet-stream`)
		c.Header(`Content-Disposition`, `attachment`)
		c.Header(`X-Content-Type-Options`, `nosniff`)
	}
	c.Header(`Cache-Control`, `no-store`)
	c.File(full)
	return nil
}

// Delete removes a stored object (minio or local).
func (m *Manager) Delete(stored string) error {
	if stored == `` {
		return nil
	}
	backend, key, err := ParseStored(stored)
	if err != nil {
		// Unknown format (e.g. legacy data) is ignored silently.
		return nil
	}
	if backend == backendOss {
		client, err := m.getClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return client.RemoveObject(ctx, m.bucket(), key, minio.RemoveObjectOptions{})
	}
	full := filepath.Join(define.AppRoot, uploadRoot, key)
	if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Exists reports whether the stored object exists.
func (m *Manager) Exists(stored string) bool {
	backend, key, err := ParseStored(stored)
	if err != nil {
		return false
	}
	if backend == backendOss {
		client, err := m.getClient()
		if err != nil {
			return false
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err = client.StatObject(ctx, m.bucket(), key, minio.StatObjectOptions{})
		return err == nil
	}
	full := filepath.Join(define.AppRoot, uploadRoot, key)
	info, err := os.Stat(full)
	return err == nil && info.Mode().IsRegular()
}

// Read returns the full content of a stored object (used for cross-namespace migration, e.g. claiming pending avatars).
func (m *Manager) Read(stored string) ([]byte, error) {
	backend, key, err := ParseStored(stored)
	if err != nil {
		return nil, err
	}
	if backend == backendOss {
		client, err := m.getClient()
		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		obj, err := client.GetObject(ctx, m.bucket(), key, minio.GetObjectOptions{})
		if err != nil {
			return nil, err
		}
		defer obj.Close()
		return io.ReadAll(obj)
	}
	return os.ReadFile(filepath.Join(define.AppRoot, uploadRoot, key))
}

// Copy duplicates one managed local object into a new tenant/business namespace without
// deleting the source. The sourceBusiness check is mandatory so a caller that only knows a
// ledger row cannot turn an arbitrary tenant-local URL into a copied file.
func (m *Manager) Copy(stored string, adminID int, sourceBusiness, newBusiness, ext string) (string, error) {
	if adminID <= 0 || sourceBusiness == `` || newBusiness == `` || !m.IsOwned(adminID, sourceBusiness, stored) {
		return ``, ErrNotOwned
	}
	_, key, err := ParseStored(stored)
	if err != nil {
		return ``, err
	}
	source, err := os.Open(filepath.Join(define.AppRoot, uploadRoot, key))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ``, ErrNotFound
		}
		return ``, err
	}
	defer func() { _ = source.Close() }()
	return m.Upload(adminID, newBusiness, ext, source)
}

// Move migrates a stored object to a new business namespace (read, re-upload, then delete the old object) and returns the new stored_path.
func (m *Manager) Move(stored string, adminID int, newBusiness, ext string) (string, error) {
	data, err := m.Read(stored)
	if err != nil {
		return ``, err
	}
	newStored, err := m.Upload(adminID, newBusiness, ext, bytes.NewReader(data))
	if err != nil {
		return ``, err
	}
	if err = m.Delete(stored); err != nil {
		logs.Warning(`Move remove source failed: %s`, err.Error())
	}
	return newStored, nil
}

// MediaTypeForExt 按现有白名单给出对象的内容类型：只有图片、PDF、Office、纯文本和音视频走真实
// MIME，其余（含 html / svg / js）一律二进制流，由调用方配合 Content-Disposition 强制下载。
func MediaTypeForExt(ext string) string {
	return contentType(ext)
}

// contentType infers the MIME type from the extension (used for minio/HTTP responses).
func contentType(ext string) string {
	ext = `.` + strings.TrimPrefix(strings.ToLower(ext), `.`)
	switch strings.ToLower(ext) {
	case `.jpg`, `.jpeg`:
		return `image/jpeg`
	case `.png`:
		return `image/png`
	case `.gif`:
		return `image/gif`
	case `.webp`:
		return `image/webp`
	case `.pdf`:
		return `application/pdf`
	case `.doc`:
		return `application/msword`
	case `.docx`:
		return `application/vnd.openxmlformats-officedocument.wordprocessingml.document`
	case `.xls`:
		return `application/vnd.ms-excel`
	case `.xlsx`:
		return `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`
	case `.ppt`:
		return `application/vnd.ms-powerpoint`
	case `.pptx`:
		return `application/vnd.openxmlformats-officedocument.presentationml.presentation`
	case `.txt`:
		return `text/plain; charset=utf-8`
	case `.csv`:
		return `text/csv`
	case `.md`:
		return `text/markdown; charset=utf-8`
	case `.zip`:
		return `application/zip`
	case `.rar`:
		return `application/vnd.rar`
	case `.7z`:
		return `application/x-7z-compressed`
	case `.mp4`:
		return `video/mp4`
	case `.webm`:
		return `video/webm`
	default:
		return `application/octet-stream`
	}
}

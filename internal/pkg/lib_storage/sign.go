// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package lib_storage

import (
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"goteams/internal/app/goteams/define"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/zhimaAi/go_tools/logs"
)

// 签名有效期。纯本地计算，签多长都不额外花钱，只决定链接泄露后的可用窗口
const signedURLTTL = 130 * time.Minute

// DocumentSignedURLTTL 是产出文档查看/下载链接的有效期，产品要求 2 小时
const DocumentSignedURLTTL = 120 * time.Minute

// 签名用的 client。服务端读写 oss 走内网 endpoint，签名要签成外网地址，所以单独建一个
var signClient = struct {
	sync.Mutex
	once   bool
	client *minio.Client
	hosts  []string
}{}

var ossURLPattern = regexp.MustCompile(`https?://[^\s"'<>\\()\[\]{}，。；：！？、（）【】《》“”‘’]+`)
var jsonStringPattern = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
var htmlAmpersands = strings.NewReplacer(`&amp;`, `&`, `&#38;`, `&`, `&#x26;`, `&`, `&#X26;`, `&`)

// SignEnabled reports whether dedicated public signing credentials are configured.
func SignEnabled() bool {
	if !M.enabled() {
		return false
	}
	cfg := define.Config.OssS3
	if strings.TrimSpace(cfg[`aliyun_url_sign`]) != `1` ||
		strings.TrimSpace(cfg[`aliyun_pub_access_key`]) == `` ||
		strings.TrimSpace(cfg[`aliyun_pub_secret_key`]) == `` {
		return false
	}
	// cname 用的是自定义域名，地址里不带 bucket 段，minio 拼不出这个形态，先不签
	return normalizedURLStyle(define.Config.OssS3[`url_style`]) != urlStyleCNAME
}

// prepareSign 第一次调用时建签名 client，并算出本系统 oss 地址会出现哪些 host
func prepareSign() (*minio.Client, []string) {
	signClient.Lock()
	defer signClient.Unlock()
	if signClient.once {
		return signClient.client, signClient.hosts
	}
	signClient.once = true
	cfg := define.Config.OssS3
	// 签名地址要对外能访问：优先用 public_endpoint（生产是内网 endpoint + 公网 public_endpoint）
	rawHost := strings.TrimSpace(cfg[`public_endpoint`])
	if rawHost == `` {
		rawHost = cfg[`endpoint`]
	}
	endpoint, err := configuredHost(rawHost)
	if err != nil {
		logs.Error(`oss sign endpoint invalid: %s`, err.Error())
		return nil, nil
	}
	lookup := minio.BucketLookupPath
	if normalizedURLStyle(cfg[`url_style`]) == urlStyleVirtualHosted {
		lookup = minio.BucketLookupDNS
	}
	// 显式给 region：不给的话 minio 签之前会先发一次 GetBucketLocation，
	// 而生产的内网机器不一定能访问公网 endpoint
	client, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(strings.TrimSpace(cfg[`aliyun_pub_access_key`]), strings.TrimSpace(cfg[`aliyun_pub_secret_key`]), ``),
		Secure:       cfg[`use_ssl`] == `true`,
		Region:       ossRegion(cfg[`endpoint`]),
		BucketLookup: lookup,
	})
	if err != nil {
		logs.Error(`oss sign client init failed: %s`, err.Error())
		return nil, nil
	}
	signClient.client = client
	signClient.hosts = ossHosts()
	return client, signClient.hosts
}

// ossRegion 从 endpoint 推 region：oss-cn-hongkong-internal.aliyuncs.com -> oss-cn-hongkong
func ossRegion(endpoint string) string {
	host, err := configuredHost(endpoint)
	if err != nil || !strings.HasSuffix(host, `.aliyuncs.com`) {
		return ``
	}
	head := strings.SplitN(host, `.`, 2)[0]
	return strings.TrimSuffix(head, `-internal`)
}

// ossHosts 列出本系统 oss 地址可能用到的 host，用来判断文本里哪些地址该换
func ossHosts() []string {
	cfg := define.Config.OssS3
	bucket := strings.TrimSpace(cfg[`bucket`])
	virtualHosted := normalizedURLStyle(cfg[`url_style`]) == urlStyleVirtualHosted
	hosts := make([]string, 0, 6)
	for _, raw := range []string{cfg[`endpoint`], cfg[`public_endpoint`], cfg[`custom_domain`]} {
		host, err := configuredHost(raw)
		if err != nil {
			continue
		}
		if virtualHosted && bucket != `` {
			hosts = append(hosts, bucket+`.`+host)
		}
		hosts = append(hosts, host)
	}
	return hosts
}

// SignedURL 把存储值换成带签名的临时地址。不是本系统的 oss 对象、或者签名失败，都原样返回。
// 入参允许是已经签过的地址：解析时只看路径，所以能重新签一遍，旧签名过期也能恢复
func SignedURL(stored string) string {
	return SignedURLWithTTL(stored, signedURLTTL, nil)
}

// SignedURLWithTTL 用指定有效期签一个可访问地址；extra 里的查询参数会一起参与签名
// （例如用 response-content-disposition 控制下载行为）。本地存储没有签名，原样返回相对地址。
func SignedURLWithTTL(stored string, ttl time.Duration, extra url.Values) string {
	if stored == `` || !SignEnabled() {
		return stored
	}
	backend, key, err := ParseStored(stored)
	if err != nil || backend != backendOss {
		return stored
	}
	client, _ := prepareSign()
	if client == nil {
		return stored
	}
	permanent, err := url.Parse(unsignedOSSURL(stored))
	if err != nil {
		return stored
	}
	query, err := url.ParseQuery(permanent.RawQuery)
	if err != nil {
		return stored
	}
	for name, values := range extra {
		for _, value := range values {
			query.Set(name, value)
		}
	}
	u, err := client.PresignedGetObject(context.Background(), M.bucket(), key, ttl, query)
	if err != nil {
		logs.Error(`oss presign failed: key=%s, err=%s`, key, err.Error())
		return stored
	}
	u.Fragment = permanent.Fragment
	return u.String()
}

// RewriteOSSURLs refreshes managed OSS URLs in text or JSON without retaining old signatures.
func RewriteOSSURLs(text string) string {
	if text == `` || !SignEnabled() {
		return text
	}
	return rewriteOSSURLs(text, SignedURL)
}

// StripOSSURLSignatures removes temporary credentials even when response signing is disabled.
func StripOSSURLSignatures(text string) string {
	return rewriteOSSURLs(text, unsignedOSSURL)
}

func unsignedOSSURL(stored string) string {
	backend, _, err := ParseStored(stored)
	if err != nil || backend != backendOss {
		return stored
	}
	u, err := url.Parse(stored)
	if err != nil || u.RawQuery == `` {
		return stored
	}
	parts := strings.Split(u.RawQuery, `&`)
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		name, _, _ := strings.Cut(part, `=`)
		name, err = url.QueryUnescape(name)
		if err != nil {
			return stored
		}
		switch strings.ToLower(name) {
		case `x-amz-algorithm`, `x-amz-credential`, `x-amz-date`, `x-amz-expires`,
			`x-amz-signedheaders`, `x-amz-signature`, `x-amz-security-token`, `x-amz-content-sha256`,
			`ossaccesskeyid`, `expires`, `signature`, `security-token`,
			`x-oss-signature-version`, `x-oss-credential`, `x-oss-date`, `x-oss-expires`,
			`x-oss-additional-headers`, `x-oss-signature`, `x-oss-security-token`:
			continue
		}
		kept = append(kept, part)
	}
	if len(kept) == len(parts) {
		return stored
	}
	u.RawQuery = strings.Join(kept, `&`)
	u.ForceQuery = false
	return u.String()
}

func rewriteOSSURLs(text string, rewrite func(string) string) string {
	if text == `` {
		return text
	}
	// Decode JSON strings first: escaped ampersands otherwise split a signature mid-URL.
	if json.Valid([]byte(text)) {
		var out strings.Builder
		last := 0
		for _, span := range jsonStringPattern.FindAllStringIndex(text, -1) {
			if strings.HasPrefix(strings.TrimLeft(text[span[1]:], " \t\r\n"), `:`) {
				continue
			}
			var value string
			if err := json.Unmarshal([]byte(text[span[0]:span[1]]), &value); err != nil {
				return text
			}
			replaced := rewriteOSSURLs(value, rewrite)
			if replaced == value {
				continue
			}
			encoded, err := json.Marshal(replaced)
			if err != nil {
				return text
			}
			out.WriteString(text[last:span[0]])
			out.Write(encoded)
			last = span[1]
		}
		if last == 0 {
			return text
		}
		out.WriteString(text[last:])
		return out.String()
	}
	if !strings.Contains(text, `://`) || !containsAny(strings.ToLower(text), ossHosts()) {
		return text
	}
	return ossURLPattern.ReplaceAllStringFunc(text, func(raw string) string {
		candidate := strings.TrimRight(raw, `.,!?`)
		decoded := htmlAmpersands.Replace(candidate)
		replaced := rewrite(decoded)
		if replaced == decoded {
			return raw
		}
		if decoded != candidate {
			replaced = strings.ReplaceAll(replaced, `&`, `&amp;`)
		}
		return replaced + raw[len(candidate):]
	})
}

// containsAny 文本里有没有出现这些 host 中的任意一个
func containsAny(text string, hosts []string) bool {
	for _, host := range hosts {
		if strings.Contains(text, strings.ToLower(host)) {
			return true
		}
	}
	return false
}

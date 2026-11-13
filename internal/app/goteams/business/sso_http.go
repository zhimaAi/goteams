// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"goteams/internal/app/goteams/define"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
)

// ErrSsoEndpointUnreachable 表示租户自配端点不可达、被出站限制拒绝或返回非法响应。
// 对外只回这一类归一化原因，不透传内网响应内容。
var ErrSsoEndpointUnreachable = errors.New("sso endpoint unreachable")

const (
	// ssoHTTPTimeout 是一次 SSO 出站请求的超时，与第三方扫码登录同口径。
	ssoHTTPTimeout = 10 * time.Second
	// ssoDialTimeout 是建连超时；握手慢不等于端点慢，单独收紧以便快速失败。
	ssoDialTimeout = 5 * time.Second
	// ssoResponseLimit 限制读取的响应体大小：IdP 响应是外部输入，不能让它撑爆内存。
	ssoResponseLimit = 1 << 20
)

// ssoCallJSON 用受控客户端发起一次 JSON 请求并把响应解析到 out。
// target 可以是租户自配地址，因此必须走 ssoTrustedClient()，不能换成普通 client。
func ssoCallJSON(method string, target string, headers map[string]string, body any, out any) error {
	raw, err := ssoRequestBody(body)
	if err != nil {
		return err
	}
	return ssoDoRequest(ssoBuildRequest(method, target, headers, `application/json; charset=utf-8`, raw), out)
}

// ssoCallForm 以 x-www-form-urlencoded 发起请求（token 端点必须表单编码），响应解析到 out。
// 客户端认证方式定死为凭据进表单体，不用 Basic 头，避免各家实现口径不一。
func ssoCallForm(target string, headers map[string]string, form url.Values, out any) error {
	request := ssoBuildRequest(http.MethodPost, target, headers, `application/x-www-form-urlencoded`,
		strings.NewReader(form.Encode()))
	return ssoDoRequest(request, out)
}

// ssoProbeEndpoint 只回答「这个地址有没有 HTTP 响应」，用于连接测试。
//
// 只有连接层失败（DNS 解析不到、超时、被出站限制拒绝）才算不可达；4xx/5xx 都算可达——
// 探测不带凭据，端点回 401 恰恰说明它活着。不跟随重定向：跟随就等于再发一次外部可控请求，
// 而「可达」这个结论第一跳就能给出。
func ssoProbeEndpoint(target string, headers map[string]string) error {
	request := ssoBuildRequest(http.MethodGet, target, headers, ``, nil)
	if request == nil {
		return fmt.Errorf("%w: invalid url", ErrSsoEndpointUnreachable)
	}
	if err := ssoCheckRequestUrl(request.URL); err != nil {
		return err
	}
	client := ssoTrustedClient()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrSsoEndpointUnreachable, err.Error())
	}
	if closeErr := response.Body.Close(); closeErr != nil {
		logs.Error("close sso probe response failed: %s", closeErr.Error())
	}
	return nil
}

// ssoRequestBody 把请求体序列化成 reader；body 为 nil 时返回 nil（无请求体）。
func ssoRequestBody(body any) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal sso request: %w", err)
	}
	return bytes.NewReader(raw), nil
}

func ssoBuildRequest(method string, target string, headers map[string]string, contentType string, body io.Reader) *http.Request {
	request, err := http.NewRequest(method, target, body)
	if err != nil {
		return nil
	}
	if contentType != `` && body != nil {
		request.Header.Set(`Content-Type`, contentType)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	return request
}

func ssoDoRequest(request *http.Request, out any) error {
	if request == nil {
		return fmt.Errorf("%w: invalid request", ErrSsoEndpointUnreachable)
	}
	if err := ssoCheckRequestUrl(request.URL); err != nil {
		return err
	}
	return ssoSend(ssoTrustedClient(), request, out)
}

// ssoVendorCallJSON 访问写死的厂商域名（钉钉 / 企微 / 飞书）。
// 地址不由租户输入决定，所以用普通超时客户端，不受私网限制约束。
func ssoVendorCallJSON(method string, target string, headers map[string]string, body any, out any) error {
	raw, err := ssoRequestBody(body)
	if err != nil {
		return err
	}
	request := ssoBuildRequest(method, target, headers, `application/json; charset=utf-8`, raw)
	if request == nil {
		return fmt.Errorf("%w: invalid vendor request", ErrSsoEndpointUnreachable)
	}
	return ssoSend(&http.Client{Timeout: ssoHTTPTimeout}, request, out)
}

func ssoSend(client *http.Client, request *http.Request, out any) error {
	response, err := client.Do(request)
	if err != nil {
		// 拨号阶段的拒绝（内网地址）也归一化成同一个原因，调用方无法从这里读出内网拓扑。
		return fmt.Errorf("%w: %s", ErrSsoEndpointUnreachable, err.Error())
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			logs.Error("close sso response failed: %s", closeErr.Error())
		}
	}()

	raw, err := io.ReadAll(io.LimitReader(response.Body, ssoResponseLimit))
	if err != nil {
		return fmt.Errorf("%w: read response: %s", ErrSsoEndpointUnreachable, err.Error())
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		// 不落完整响应体（可能回显 client_secret），只按白名单取错误码与描述写日志。
		logs.Error("sso endpoint returned status: host=%s, status=%d, detail=%s",
			request.URL.Host, response.StatusCode, oauthErrorSummary(raw))
		return fmt.Errorf("%w: status %d", ErrSsoEndpointUnreachable, response.StatusCode)
	}
	if err = json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%w: %s", ErrSsoEndpointUnreachable, err.Error())
	}
	return nil
}

// ssoCheckRequestUrl 校验请求地址本身的形态：scheme 与主机名。
// 真正的 IP 限制在建连那一刻执行，这里只挡明显非法的输入。
func ssoCheckRequestUrl(target *url.URL) error {
	if target == nil {
		return fmt.Errorf("%w: empty url", ErrSsoEndpointUnreachable)
	}
	switch target.Scheme {
	case `https`:
		return nil
	case `http`:
		// 联调环境（本地 Dex / mock IdP）允许 http；生产强制 https。
		if define.IsDev {
			return nil
		}
		return fmt.Errorf("%w: https is required", ErrSsoEndpointUnreachable)
	default:
		return fmt.Errorf("%w: unsupported scheme %s", ErrSsoEndpointUnreachable, target.Scheme)
	}
}

// ssoTrustedClient 返回访问「租户自配端点」的受控客户端。
//
// 这条链路上的地址全部是外部输入，SSRF 护栏必须在这里：拨号时校验真实 IP（DNS 重绑定绕不过）、
// 逐跳重定向校验、禁止跨 host 跳转。discovery 派生出的端点同样走这个客户端。
func ssoTrustedClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   ssoDialTimeout,
		KeepAlive: 30 * time.Second,
		Control:   ssoControlConn,
	}
	transport := &http.Transport{
		DialContext:         dialer.DialContext,
		TLSHandshakeTimeout: ssoDialTimeout,
		// 不沿用代理：走代理时 Control 看到的是代理的地址，护栏会失效。
		Proxy: nil,
		TLSClientConfig: &tls.Config{
			// 联调用自签证书的本地 IdP；生产不放宽。
			InsecureSkipVerify: define.IsDev,
		},
	}
	return &http.Client{
		Timeout:   ssoHTTPTimeout,
		Transport: transport,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) == 0 || strings.ToLower(request.URL.Hostname()) != strings.ToLower(via[0].URL.Hostname()) {
				return errors.New(`sso redirect to another host is not allowed`)
			}
			return ssoCheckRequestUrl(request.URL)
		},
	}
}

// ssoControlConn 在实际建连前校验对端 IP。
// Control 拿到的是解析后的真实地址，所以这里挡的是「最终连到哪儿」，不是「DNS 说要到哪儿」。
func ssoControlConn(network string, address string, conn syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if !ssoIpAllowed(net.ParseIP(host)) {
		return fmt.Errorf("%w: address not allowed", ErrSsoEndpointUnreachable)
	}
	return nil
}

// ssoIpAllowed 判定出站目标 IP 是否放行：默认拒绝全部回环 / 私网 / 链路本地 / CGNAT / ULA，
// 由部署级 [sso] allow_cidrs 精确放行内网 IdP 网段。联调环境直接放行，否则本地 Dex 连不上。
func ssoIpAllowed(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if define.IsDev {
		return true
	}
	if !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return ssoIpInAllowList(ip)
	}
	// CGNAT(100.64/10) 与 ULA(fc00::/7) 都不属于 IsPrivate 的口径，单独补上。
	if isCgnatOrUla(ip) {
		return ssoIpInAllowList(ip)
	}
	return true
}

func isCgnatOrUla(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 100 && ip4[1]&0xC0 == 64
	}
	return len(ip) == net.IPv6len && ip[0]&0xFE == 0xFC
}

// ssoIpInAllowList 比对部署级白名单；空白名单即全部受限地址保持拒绝。
func ssoIpInAllowList(ip net.IP) bool {
	for _, cidr := range ssoAllowCidrs() {
		if _, block, err := net.ParseCIDR(cidr); err == nil && block.Contains(ip) {
			return true
		}
	}
	return false
}

// ssoAllowCidrs 读取 [sso] allow_cidrs（逗号分隔 CIDR）。每次出站都重读，不缓存解析结果。
func ssoAllowCidrs() []string {
	raw := strings.TrimSpace(define.Config.Sso[`allow_cidrs`])
	if raw == `` {
		return nil
	}
	parts := strings.Split(raw, `,`)
	list := make([]string, 0, len(parts))
	for _, item := range parts {
		if item = strings.TrimSpace(item); item != `` {
			list = append(list, item)
		}
	}
	return list
}

// ssoConfigInt 读取 [sso] 分节的整型参数，缺失或非法时回退默认值。
func ssoConfigInt(key string, fallback int) int {
	value := cast.ToInt(strings.TrimSpace(define.Config.Sso[key]))
	if value <= 0 {
		return fallback
	}
	return value
}

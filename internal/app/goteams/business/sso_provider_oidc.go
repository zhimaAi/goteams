// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"

	"github.com/zhimaAi/go_tools/logs"
)

// oidcProvider 实现标准 OIDC：discovery + authorization code + PKCE + id_token 校验。
//
// 全部端点都来自租户自配 issuer 的 discovery 响应，属外部可控输入，因此每一次出站都用受控客户端
// （ssoCallJSON / ssoCallForm），discovery 派生出的地址与 issuer 同等对待。
type oidcProvider struct{}

func (oidcProvider) Key() string { return define.SsoProviderOidc }

// ssoOidcDefaultScopes 是管理员未填 scope 时的兜底：没有 openid 就没有 id_token，整条链路失效。
const ssoOidcDefaultScopes = `openid profile email`

// ssoOidcClockLeeway 容忍的时钟偏移（秒）。IdP 与本机时钟差几十秒是常态，
// 不留余量会让「刚签发的 token 判成过期」。
const ssoOidcClockLeeway int64 = 120

// ssoOidcDiscovery 只声明本系统用到的端点；其余字段由 json 解码自动忽略。
type ssoOidcDiscovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
	JwksUri               string `json:"jwks_uri"`
}

// ssoOidcDiscover 拉 discovery。ponytail: 每次授权都重新拉取，不缓存 —— 一次 SSO 登录多一个
// 往返而已；真出现 IdP 限流再加带 TTL 的 issuer→doc 缓存。
func ssoOidcDiscover(issuer string) (ssoOidcDiscovery, error) {
	var doc ssoOidcDiscovery
	base := strings.TrimRight(strings.TrimSpace(issuer), `/`)
	if base == `` {
		return doc, fmt.Errorf("%w: issuer_url is empty", ErrSsoEndpointUnreachable)
	}
	if err := ssoCallJSON(http.MethodGet, base+`/.well-known/openid-configuration`, nil, nil, &doc); err != nil {
		return doc, err
	}
	if doc.AuthorizationEndpoint == `` || doc.TokenEndpoint == `` || doc.JwksUri == `` {
		return doc, fmt.Errorf("%w: discovery is missing required endpoints", ErrSsoEndpointUnreachable)
	}
	return doc, nil
}

// TestConnect 验 discovery 可用且 jwks 至少有一把公钥。
// 没有授权码换不了 token，因此只能验到端点这一层（SsoTestLevelEndpoints）。
func (p oidcProvider) TestConnect(config SsoConfigModel) (string, error) {
	doc, err := ssoOidcDiscover(config.IssuerUrl)
	if err != nil {
		return ``, err
	}
	var jwks ssoJwks
	if err = ssoCallJSON(http.MethodGet, doc.JwksUri, nil, nil, &jwks); err != nil {
		return ``, err
	}
	if len(jwks.Keys) == 0 {
		return ``, fmt.Errorf("%w: jwks is empty", ErrSsoEndpointUnreachable)
	}
	return SsoTestLevelEndpoints, nil
}

func (p oidcProvider) AuthorizeURL(config SsoConfigModel, state SsoAuthzState) (string, error) {
	doc, err := ssoOidcDiscover(config.IssuerUrl)
	if err != nil {
		return ``, err
	}
	query := url.Values{}
	query.Set(`response_type`, `code`)
	query.Set(`client_id`, config.AppKey)
	query.Set(`redirect_uri`, state.RedirectUri)
	query.Set(`scope`, ssoScopeString(config.Scopes, ssoOidcDefaultScopes))
	query.Set(`state`, state.State)
	query.Set(`nonce`, state.Nonce)
	if state.CodeVerifier != `` {
		query.Set(`code_challenge`, ssoCodeChallenge(state.CodeVerifier))
		query.Set(`code_challenge_method`, `S256`)
	}
	return doc.AuthorizationEndpoint + `?` + query.Encode(), nil
}

func (p oidcProvider) ExchangeCode(config SsoConfigModel, code string, state SsoAuthzState) (SsoUserInfo, error) {
	doc, err := ssoOidcDiscover(config.IssuerUrl)
	if err != nil {
		return SsoUserInfo{}, err
	}
	token, err := SsoExchangeCodeForToken(doc.TokenEndpoint, config.AppKey, config.AppSecret, state, code)
	if err != nil {
		return SsoUserInfo{}, err
	}
	if token.IdToken == `` {
		// 能拿到 access_token 却没有 id_token，多半是 scope 漏了 openid。
		return SsoUserInfo{}, fmt.Errorf("%w: no id_token in response", ErrSsoAuthFailed)
	}
	claims, err := ssoVerifyIdToken(token.IdToken, doc.JwksUri, config.IssuerUrl, config.AppKey, state.Nonce)
	if err != nil {
		return SsoUserInfo{}, err
	}
	info := SsoUserInfo{
		ExternalId: claims.Subject,
		Username:   ssoStringField(claims.raw, `preferred_username`, `upn`),
		Email:      claims.Email,
		Name:       ssoStringField(claims.raw, `name`),
		Phone:      ssoStringField(claims.raw, `phone_number`),
		Avatar:     ssoStringField(claims.raw, `picture`),
	}
	if doc.UserInfoEndpoint != `` && token.AccessToken != `` {
		// userinfo 只用来补 id_token 里没有的资料；sub 不一致说明两个端点不是同一个人，直接拒绝。
		fields, fetchErr := SsoFetchUserInfo(doc.UserInfoEndpoint, token.AccessToken)
		if fetchErr != nil {
			logs.Error("sso oidc userinfo fetch failed: issuer=%s, err=%s", config.IssuerUrl, fetchErr.Error())
		} else if sub := ssoStringField(fields, `sub`); sub != `` && sub != info.ExternalId {
			return SsoUserInfo{}, fmt.Errorf("%w: userinfo sub mismatch", ErrSsoAuthFailed)
		} else {
			ssoFillMissingProfile(&info, fields)
		}
	}
	if info.ExternalId == `` {
		return SsoUserInfo{}, fmt.Errorf("%w: id_token has no sub", ErrSsoAuthFailed)
	}
	return info, nil
}

// ssoFillMissingProfile 用 userinfo 补齐 id_token 里缺失的资料字段。
// 只填空，不覆盖：id_token 是验签过的，可信度更高。
func ssoFillMissingProfile(info *SsoUserInfo, fields map[string]any) {
	if info.Username == `` {
		info.Username = ssoStringField(fields, `preferred_username`, `upn`)
	}
	if info.Email == `` {
		info.Email = ssoStringField(fields, `email`)
	}
	if info.Name == `` {
		info.Name = ssoStringField(fields, `name`)
	}
	if info.Phone == `` {
		info.Phone = ssoStringField(fields, `phone_number`)
	}
	if info.Avatar == `` {
		info.Avatar = ssoStringField(fields, `picture`)
	}
}

// ssoIdTokenClaims 是校验通过后要用的声明；raw 保留原始 payload，供上层按平台取扩展字段。
type ssoIdTokenClaims struct {
	Issuer   string      `json:"iss"`
	Audience ssoAudience `json:"aud"`
	Subject  string      `json:"sub"`
	Nonce    string      `json:"nonce"`
	Exp      int64       `json:"exp"`
	Iat      int64       `json:"iat"`
	Email    string      `json:"email"`
	raw      map[string]any
}

// ssoAudience 处理 aud 既可能是字符串也可能是数组的两种合法写法。
type ssoAudience []string

func (a *ssoAudience) UnmarshalJSON(raw []byte) error {
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		*a = ssoAudience{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err != nil {
		return errors.New(`invalid aud claim`)
	}
	*a = many
	return nil
}

func (a ssoAudience) contains(want string) bool {
	for _, item := range a {
		if item == want {
			return true
		}
	}
	return false
}

// ssoVerifyIdToken 校验 id_token 的签名与标准声明（iss/aud/exp/nonce）。
//
// 只接受 RS256 与 ES256：验签实现按 header 里的 kty 分派，alg=none 或 HMAC 类算法都拿不到一个
// 能通过校验的签名，因此不存在算法混淆降级的空间。
func ssoVerifyIdToken(idToken string, jwksUri string, issuer string, clientId string, nonce string) (ssoIdTokenClaims, error) {
	var claims ssoIdTokenClaims
	headerRaw, payloadRaw, signature, err := ssoSplitJwt(idToken)
	if err != nil {
		return claims, err
	}
	var header ssoJwtHeader
	if err = json.Unmarshal(headerRaw, &header); err != nil {
		return claims, fmt.Errorf("%w: id_token header: %s", ErrSsoAuthFailed, err.Error())
	}
	if header.Alg != `RS256` && header.Alg != `ES256` {
		return claims, fmt.Errorf("%w: unsupported id_token alg %s", ErrSsoAuthFailed, header.Alg)
	}

	var jwks ssoJwks
	if err = ssoCallJSON(http.MethodGet, jwksUri, nil, nil, &jwks); err != nil {
		return claims, err
	}
	signingInput := strings.Join([]string{
		base64.RawURLEncoding.EncodeToString(headerRaw),
		base64.RawURLEncoding.EncodeToString(payloadRaw),
	}, `.`)
	if err = ssoVerifyJwtSignature(header, signingInput, signature, jwks); err != nil {
		return claims, err
	}
	if err = json.Unmarshal(payloadRaw, &claims); err != nil {
		return claims, fmt.Errorf("%w: id_token payload: %s", ErrSsoAuthFailed, err.Error())
	}
	if err = json.Unmarshal(payloadRaw, &claims.raw); err != nil {
		return claims, fmt.Errorf("%w: id_token payload: %s", ErrSsoAuthFailed, err.Error())
	}
	if err = ssoCheckIdTokenClaims(claims, issuer, clientId, nonce); err != nil {
		return claims, err
	}
	return claims, nil
}

// ssoCheckIdTokenClaims 逐条比对标准声明。iss 允许配置里带或不带结尾斜杠，
// 其余都是精确比对：这里放宽任何一项都等于放弃校验。
func ssoCheckIdTokenClaims(claims ssoIdTokenClaims, issuer string, clientId string, nonce string) error {
	expected := strings.TrimRight(strings.TrimSpace(issuer), `/`)
	got := strings.TrimRight(strings.TrimSpace(claims.Issuer), `/`)
	if expected != `` && got != expected {
		return fmt.Errorf("%w: id_token issuer mismatch", ErrSsoAuthFailed)
	}
	if !claims.Audience.contains(clientId) {
		return fmt.Errorf("%w: id_token audience mismatch", ErrSsoAuthFailed)
	}
	now := time.Now().Unix()
	if claims.Exp != 0 && now > claims.Exp+ssoOidcClockLeeway {
		return fmt.Errorf("%w: id_token expired", ErrSsoAuthFailed)
	}
	if claims.Nonce == `` || claims.Nonce != nonce {
		return fmt.Errorf("%w: id_token nonce mismatch", ErrSsoAuthFailed)
	}
	if claims.Subject == `` {
		return fmt.Errorf("%w: id_token has no sub", ErrSsoAuthFailed)
	}
	return nil
}

// ssoSplitJwt 拆出 header / payload / 签名的原始字节；三段都是 base64url。
func ssoSplitJwt(token string) ([]byte, []byte, []byte, error) {
	parts := strings.Split(token, `.`)
	if len(parts) != 3 {
		return nil, nil, nil, fmt.Errorf("%w: id_token is not a JWS", ErrSsoAuthFailed)
	}
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w: id_token header encoding: %s", ErrSsoAuthFailed, err.Error())
	}
	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w: id_token payload encoding: %s", ErrSsoAuthFailed, err.Error())
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w: id_token signature encoding: %s", ErrSsoAuthFailed, err.Error())
	}
	return headerRaw, payloadRaw, signature, nil
}

// ssoJwtHeader 是 JWS 头里参与验签决策的两个字段。
type ssoJwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

// ssoVerifyJwtSignature 用 JWKS 里的公钥验证签名。kid 存在时只认那一把，
// 不存在时逐把试（部分 IdP 不在 header 写 kid），任一把通过即算有效。
func ssoVerifyJwtSignature(header ssoJwtHeader, signingInput string, signature []byte, jwks ssoJwks) error {
	input := []byte(signingInput)
	var lastErr error
	for _, key := range jwks.Keys {
		if header.Kid != `` && key.Kid != header.Kid {
			continue
		}
		var err error
		switch header.Alg {
		case `RS256`:
			err = ssoVerifyRs256(key, input, signature)
		case `ES256`:
			err = ssoVerifyEs256(key, input, signature)
		}
		if err == nil {
			return nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no matching key in jwks for kid %s", header.Kid)
	}
	return fmt.Errorf("%w: id_token signature: %s", ErrSsoAuthFailed, lastErr.Error())
}

func ssoVerifyRs256(key ssoJwk, input []byte, signature []byte) error {
	pub, err := key.rsaPublicKey()
	if err != nil {
		return err
	}
	sum := sha256.Sum256(input)
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], signature)
}

func ssoVerifyEs256(key ssoJwk, input []byte, signature []byte) error {
	pub, err := key.ecdsaPublicKey()
	if err != nil {
		return err
	}
	// ES256 的 JWS 签名是固定 64 字节的 r‖s，不是 DER。
	if len(signature) != 64 {
		return errors.New(`es256 signature length`)
	}
	sum := sha256.Sum256(input)
	return ssoVerifyEcdsa(pub, sum[:], signature[:32], signature[32:])
}

// ssoVerifyEcdsa 把 JWS 的定长 r‖s 交给 ecdsa 校验。
func ssoVerifyEcdsa(pub *ecdsa.PublicKey, hash []byte, rRaw []byte, sRaw []byte) error {
	if !ecdsa.Verify(pub, hash, new(big.Int).SetBytes(rRaw), new(big.Int).SetBytes(sRaw)) {
		return errors.New(`invalid ecdsa signature`)
	}
	return nil
}

// ssoJwks 是 RFC 7517 的密钥集。
type ssoJwks struct {
	Keys []ssoJwk `json:"keys"`
}

type ssoJwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func (k ssoJwk) rsaPublicKey() (*rsa.PublicKey, error) {
	if k.Kty != `RSA` {
		return nil, errors.New(`jwk is not an RSA key`)
	}
	modulus, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil || len(modulus) == 0 {
		return nil, errors.New(`invalid jwk RSA modulus`)
	}
	exponent, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil || len(exponent) == 0 {
		return nil, errors.New(`invalid jwk RSA exponent`)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: int(new(big.Int).SetBytes(exponent).Int64())}, nil
}

func (k ssoJwk) ecdsaPublicKey() (*ecdsa.PublicKey, error) {
	if k.Kty != `EC` {
		return nil, errors.New(`jwk is not an EC key`)
	}
	// 本系统只声明支持 ES256，即 P-256；其余曲线直接拒。
	if k.Crv != `P-256` {
		return nil, fmt.Errorf(`unsupported EC curve %s`, k.Crv)
	}
	x, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, errors.New(`invalid jwk EC x`)
	}
	y, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, errors.New(`invalid jwk EC y`)
	}
	curve := elliptic.P256()
	// 不在曲线上的公钥会被 ecdsa 自己拒掉（Go 的 Verify 会校验点合法性），这里不重复实现。
	return &ecdsa.PublicKey{Curve: curve, X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}, nil
}
